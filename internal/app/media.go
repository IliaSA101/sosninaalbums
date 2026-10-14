package app

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// maxSkippedLog — сколько путей нечитаемых файлов писать в лог.
const maxSkippedLog = 20

// readColor — imgfile.ReadColor; подменяется в тестах (/media не должен читать недостающие).
var readColor = imgfile.ReadColor

// rescan запускает чтение папок проекта и сразу возвращается. Прежнее сканирование и его прогрев
// миниатюр отменяются; из ленты уходят фото, чьих папок больше нет (при смене проекта лента пустеет).
// Дальше интерфейс получает события `scan` (ход) и `library` (список порциями).
func (a *App) rescan() {
	defer guard("rescan")
	ctx, cancel := context.WithCancel(a.stopCtx)

	a.mu.Lock()
	// Список папок и поколение — под одним замком: два rescan подряд (OnFileDrop и Undo)
	// не могут поменяться местами, и новое поколение всегда с новым списком.
	folders := a.editor().Folders()
	if a.scanCancel != nil {
		a.scanCancel()
	}
	stopWarm := a.warm != nil
	if stopWarm {
		a.warm()
		a.warm = nil
	}
	a.scanCancel = cancel
	a.scanGen++
	gen := a.scanGen
	// Новые срезы, а не правка на месте: прежний уже мог уйти в GetState или в событие.
	kept := make([]library.Photo, 0, len(a.photos))
	for _, p := range a.photos {
		if slices.ContainsFunc(folders, func(f string) bool { return library.Under(p.Path, f) }) {
			kept = append(kept, p)
		}
	}
	a.photos, a.byID = kept, byIDMap(kept)
	a.scan = library.ScanProgress{Active: true, Walking: true}
	pr := a.scan
	a.mu.Unlock()

	if stopWarm {
		// Прогрев отменён: полоска «Готовлю миниатюры» сбрасывается (0/0 — прогрева нет).
		a.emitMu.Lock()
		a.emit("thumbs", 0, 0)
		a.emitMu.Unlock()
	}
	a.publish(gen, kept, pr)
	a.refreshMissing()
	a.bg.Go(func() {
		defer guard("сканирование")
		var fin library.ScanProgress
		photos, skipped, err := a.scanner.Scan(ctx, folders, func(ph []library.Photo, cur library.ScanProgress, final bool) {
			if final {
				fin = cur
			}
			a.publish(gen, ph, cur)
		})
		if err != nil {
			return // отменено: уже идёт новое сканирование
		}
		log.Printf("папок %d: %d фото (обход %v, чтение %v, из памяти %d), пропущено %d", len(folders), len(photos),
			fin.WalkTime.Round(time.Millisecond), fin.ReadTime.Round(time.Millisecond), fin.Memo, len(skipped))
		for i, s := range skipped {
			if i >= maxSkippedLog {
				log.Printf("  ... и ещё %d (всего не читается %d)", len(skipped)-i, len(skipped))
				break
			}
			log.Printf("  не читается: %s", filepath.Base(s))
		}
		a.warmThumbs(gen, photos)
	})
}

func byIDMap(photos []library.Photo) map[string]library.Photo {
	m := make(map[string]library.Photo, len(photos))
	for _, p := range photos {
		m[p.ID] = p
	}
	return m
}

// publish принимает отчёт сканирования поколения gen: обновляет реестр фото и ход, шлёт события
// `library` (если список пришёл) и `scan`. Устаревшее поколение молчит. emitMu сериализует отправку,
// чтобы старый список не обогнал новый; a.mu под отправкой не держим — /media не ждёт IPC.
func (a *App) publish(gen int, photos []library.Photo, pr library.ScanProgress) {
	var byID map[string]library.Photo
	if photos != nil {
		byID = byIDMap(photos)
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if gen != a.scanGen {
		a.mu.Unlock()
		return
	}
	if photos != nil {
		a.photos, a.byID = photos, byID
	}
	a.scan = pr
	a.mu.Unlock()
	if photos != nil {
		a.emit("library", photos)
	}
	a.emit("scan", pr)
}

// warmThumbs запускает прогрев миниатюр после чтения папок, если поколение ещё актуально.
func (a *App) warmThumbs(gen int, photos []library.Photo) {
	if a.cache == nil {
		return
	}
	a.mu.Lock()
	if gen != a.scanGen {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	a.warm = cancel
	a.mu.Unlock()
	var lastMu sync.Mutex // колбэк зовут все воркеры прогрева
	last := time.Time{}
	a.cache.Warm(ctx, photos, func(done, total int) {
		lastMu.Lock()
		due := done == total || time.Since(last) > 200*time.Millisecond
		if due {
			last = time.Now()
		}
		lastMu.Unlock()
		if !due {
			return
		}
		// Устаревшее поколение молчит: иначе его хвост перебьёт сброс полоски после rescan.
		a.emitMu.Lock()
		defer a.emitMu.Unlock()
		a.mu.Lock()
		cur := gen == a.scanGen
		a.mu.Unlock()
		if cur {
			a.emit("thumbs", done, total)
		}
	})
}

func (a *App) infos(ids []string) []model.PhotoInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]model.PhotoInfo, 0, len(ids))
	for _, id := range ids {
		if p, ok := a.byID[id]; ok {
			info := model.PhotoInfo{ID: p.ID, Path: p.Path, W: p.W, H: p.H, Size: p.Size}
			if p.Xmp != nil { // отметки — только из сканера: ReadInfo их нарочно не читает
				info.Xmp = *p.Xmp
			}
			out = append(out, info)
		}
	}
	return out
}

// media отдаёт миниатюры: /media/t/<id> и /media/p/<id>; шрифты — /media/font/<файл>.
func (a *App) media(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		defer guard("media")
		if name, ok := strings.CutPrefix(r.URL.Path, "/media/font/"); ok {
			serveFont(w, r, name)
			return
		}
		if r.URL.Path == "/media/cover-guide" {
			a.serveCoverGuide(w, r)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
		if len(parts) != 2 || a.cache == nil {
			http.NotFound(w, r)
			return
		}
		a.mu.Lock()
		p, ok := a.byID[parts[1]]
		_, gone := slices.BinarySearch(a.missing, parts[1]) // missingOf отдаёт по порядку
		a.mu.Unlock()
		if !ok && gone {
			// Файла нет (или том не отвечает): не читаем, иначе обработчик ждёт таймаута SMB.
			http.NotFound(w, r)
			return
		}
		if !ok {
			// Фото из проекта, чья папка сейчас не открыта.
			if ref, found := a.editor().Ref(parts[1]); found && reachable(ref.Path) {
				col, prof := readColor(ref.Path) // от цвета зависит ключ кэша и пересчёт
				p, ok = library.Photo{ID: parts[1], Path: ref.Path, Name: filepath.Base(ref.Path), Color: col, Profile: prof}, true
			}
		}
		if !ok {
			// Путь запроса в лог не пишем: промахов бывают сотни подряд.
			logRare(a.mediaMiss.Add(1), "media: фото не найдено")
			http.NotFound(w, r)
			return
		}
		file, err := a.cache.File(p, parts[0] == "p")
		if err != nil {
			logRare(a.mediaMiss.Add(1), "превью %s: %v", filepath.Base(p.Path), paths.NoPath(err))
			http.NotFound(w, r)
			return
		}
		// Тип задаём сами: копия бывает PNG (прозрачные картинки), а реестровый тип Windows ненадёжен.
		ct := "image/jpeg"
		if strings.EqualFold(filepath.Ext(file), ".png") {
			ct = "image/png"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		http.ServeFile(w, r, file) //nolint:gosec // file — путь из кэша миниатюр по ID из реестра, не из запроса
	})
}

// serveFont отдаёт встроенный шрифт. Имя ищется в белом списке text.FontFile:
// путь из запроса в файловую систему не попадает. Не immutable: файлы могут
// смениться с версией, а кэш WebView2 переживает обновление.
func serveFont(w http.ResponseWriter, r *http.Request, name string) {
	data, ok := text.FontFile(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
