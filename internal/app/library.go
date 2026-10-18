package app

import (
	"context"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Фото альбома на диске: чтение папок и миниатюры (/media — media.go, недостающие — missing.go,
// перепривязка и показ в Проводнике — relink.go).

// rescan запускает чтение папок проекта и сразу возвращается. Прежнее сканирование и его прогрев
// миниатюр отменяются; из ленты уходят фото, чьих папок больше нет (при смене проекта лента пустеет).
// Дальше интерфейс получает события `scan` (ход) и `library` (список порциями).
func (a *App) rescan() {
	defer guard("rescan")
	ctx, cancel := context.WithCancel(a.stopCtx)

	// Список папок и поколение — одним куском под rescanMu: два rescan подряд (OnFileDrop и Undo)
	// не могут поменяться местами, и новое поколение всегда с новым списком. Folders — до a.mu:
	// замок редактора держат правки (и снимок прогрева), а a.mu ждут publish, infos и /media.
	a.rescanMu.Lock()
	folders := a.editor().Folders()
	a.mu.Lock()
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
	a.rescanMu.Unlock()

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
		if len(skipped) > 0 {
			log.Printf("  не читается: %s", skippedSummary(skipped))
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
// Прогрев раскладок открытия (warmEditor) идёт первым (073-11): прогрев миниатюр ждёт его до
// старта, а начатый — встаёт после текущего фото каждого воркера. Видимые миниатюры ленты от
// этого не ждут: их отдаёт /media своим cache.File по запросу, а пауза лишь освобождает им
// места семафора кэша.
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
	if a.layouts.wait(ctx); ctx.Err() != nil {
		return // отменён, пока ждал раскладки: и не начинаем
	}
	var lastMu sync.Mutex // колбэк зовут все воркеры прогрева
	last := time.Time{}
	a.cache.Warm(ctx, photos, func(done, total int) {
		// Колбэк — после каждого фото, до следующего: здесь воркер и встаёт на паузу.
		if a.layouts.wait(ctx); ctx.Err() != nil {
			return // отменённый прогрев молчит
		}
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
			info := model.PhotoInfo{ID: p.ID, Path: p.Path, W: p.W, H: p.H, Size: p.Size, Taken: p.Taken}
			if p.Xmp != nil { // отметки — только из сканера: ReadInfo их нарочно не читает
				info.Xmp = *p.Xmp
			}
			out = append(out, info)
		}
	}
	return out
}
