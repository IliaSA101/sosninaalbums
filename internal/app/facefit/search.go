package facefit

import (
	"context"
	"errors"
	"io/fs"
	"runtime"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Поиск лиц для подгона (спека 130, design §6.3): горутины читают превью фото, ищут лицо и
// выбирают одно; точки кэшируются. Правок проекта здесь нет — только заполнение снимка hits.

const (
	// faceEvery — не чаще этого шлётся faces:progress (последнее — всегда).
	faceEvery = 100 * time.Millisecond
	// faceSimilar — второе лицо не меньше этой доли высоты самого крупного: «несколько лиц».
	faceSimilar = 0.7
)

// faceWorkers — потоков поиска: min(4, ядра − 1), не меньше одного и не больше мест (0.27-130-10).
func faceWorkers(places int) int {
	return max(1, min(4, runtime.NumCPU()-1, places))
}

// faceTick — ход поиска: faces:progress с (done, total), не чаще раза в faceEvery, последний —
// всегда. Одна горутина шлёт за раз: числа уходят по возрастанию.
type faceTick struct {
	mu          sync.Mutex
	done, total int
	last        time.Time
	emit        func(done, total int)
}

// begin — первое событие (0, total): кнопка сразу показывает «Лица 0 / N».
func (t *faceTick) begin() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.last = time.Now()
	t.emit(0, t.total)
}

// step — ещё одно место готово.
func (t *faceTick) step() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.done++
	if now := time.Now(); t.done == t.total || now.Sub(t.last) >= faceEvery {
		t.last = now
		t.emit(t.done, t.total)
	}
}

// findFaces заполняет лица в hits (Box, Eye, Many, Missing). Ошибка — поиск прерван: программа
// закрывается или открыт другой альбом; тогда правки не будет.
func (f *Fitter) findFaces(ed *album.Editor, hits []model.FaceHit) error {
	if len(hits) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(f.h.Ctx())
	defer cancel()
	tick := &faceTick{total: len(hits), emit: func(done, total int) { f.h.Emit("faces:progress", done, total) }}
	tick.begin()
	jobs := make(chan int)
	var wg sync.WaitGroup
	started := 0
	for range faceWorkers(len(hits)) {
		wg.Add(1)
		if !f.h.Go(func() {
			defer wg.Done()
			for i := range jobs {
				if f.h.Editor() != ed { // закрыли альбом: остальным искать незачем
					cancel()
					continue
				}
				f.Job(ctx, &hits[i])
				tick.step()
			}
		}) {
			wg.Done() // фон закрыт: программа выходит
			break
		}
		started++
	}
	if started == 0 {
		return ctx.Err()
	}
feed:
	for i := range hits {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	if f.h.Editor() != ed {
		return ErrMoved
	}
	return ctx.Err()
}

// Job ищет лицо на фото места h. Нет файла, он не читается или поиск упал — Missing («нет
// фото», кадр прежний); в журнале строка без пути. Фото в облаке (заглушка OneDrive) не скачивается: берётся
// готовая копия своего размера, а без неё Missing вместе с Cloud — причина «только в облаке».
// Экспорт для тестов App (рабочие потоки зовут её отсюда же).
func (f *Fitter) Job(ctx context.Context, h *model.FaceHit) {
	h.Missing = true // пока не прочитали: отказ или паника оставляют «нет фото»
	defer appkit.Guard("faceJob")
	p, ok, gone, cloud := f.h.Photo(h.Photo)
	// mediaPhoto для фото ленты путь не проверяет (он из сканера): вторая линия границы путей.
	if !ok || gone || paths.CheckLocalPath(p.Path) != nil {
		return
	}
	st, err := library.StatLocal(p.Path)
	if err != nil {
		f.missed(err)
		return
	}
	key := faceKey{path: p.Path, size: st.Size(), mtime: st.ModTime().UnixNano(), color: p.Color,
		turn: h.Turn, version: faces.Version}
	found, hit := f.cache.get(key)
	if !hit {
		if found, err = f.detectPhoto(ctx, h.Photo, p, cloud, h.Turn); err != nil {
			if errors.Is(err, library.ErrCloudOnly) {
				h.Cloud = true
			} else if ctx.Err() == nil {
				f.missed(err)
			}
			return
		}
		f.cache.put(key, found)
	}
	h.Missing = false
	h.Box, h.Eye, h.Many = pickFace(found)
}

// detectPhoto — лица на превью фото, повёрнутом на turn четвертей. Превью — копия кэша миниатюр
// (EXIF-поворот в ней уже применён, как на холсте); читается через imgfile: заголовок, предел
// размера и бюджет памяти до декода. id — ID фото места, cloud — фото проекта в облаке (photoCopy).
func (f *Fitter) detectPhoto(ctx context.Context, id string, p library.Photo, cloud bool, turn int) ([]faces.Face, error) {
	file, err := f.h.Copy(ctx, id, p, cloud)
	if err != nil {
		return nil, err
	}
	img, release, err := imgfile.OpenOrientedCtx(ctx, file)
	if err != nil {
		return nil, err
	}
	defer release()
	detect := f.Detect
	if detect == nil {
		detect = faces.Detect
	}
	return detect(faces.Turn(img, turn)), nil
}

// missed пишет в журнал, почему фото осталось без поиска; нет файла — не сбой.
func (f *Fitter) missed(err error) {
	if !errors.Is(err, fs.ErrNotExist) {
		f.h.Rare(f.miss.Add(1), "лица: фото не прочитано: %v", paths.NoPath(err))
	}
}

// pickFace выбирает лицо (design §3): самое крупное по высоте рамки; если есть ещё лица не меньше
// 70 % его высоты — это «несколько лиц», берётся ближайшее к центру фото. Рамка и середина глаз —
// в долях картинки после четвертей. Лиц нет — пустая рамка.
func pickFace(fs []faces.Face) (box layout.Rect, eye geom.Pt, many bool) {
	top := 0.0
	for _, f := range fs {
		top = max(top, f.Box.H)
	}
	if top <= 0 {
		return layout.Rect{}, geom.Pt{}, false
	}
	best, bestD, n := -1, 0.0, 0
	for i, f := range fs {
		if f.Box.H < faceSimilar*top {
			continue
		}
		n++
		dx, dy := f.Box.X+f.Box.W/2-0.5, f.Box.Y+f.Box.H/2-0.5
		if d := dx*dx + dy*dy; best < 0 || d < bestD {
			best, bestD = i, d
		}
	}
	f := fs[best]
	e := f.EyeLine()
	return layout.Rect{X: f.Box.X, Y: f.Box.Y, W: f.Box.W, H: f.Box.H}, geom.Pt{X: e.X, Y: e.Y}, n > 1
}
