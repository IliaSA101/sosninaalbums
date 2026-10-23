package app

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Фото альбома на диске: чтение папок и миниатюры (/media — media.go, недостающие — missing.go,
// перепривязка и показ в Проводнике — relink.go, сверка папок по фокусу — foldercheck.go).

// rescan запускает чтение папок проекта и сразу возвращается. Прежнее сканирование и его прогрев
// миниатюр отменяются; из ленты уходят фото, чьих папок больше нет (при смене проекта лента пустеет).
// Дальше интерфейс получает события `scan` (ход) и `library` (список порциями).
func (a *App) rescan() { a.rescanWith(scanLoud) }

// scanMode — видно ли чтение папок: громкое — «Обновить все», смена папок, запуск; тихое — сверка
// папок (foldercheck.go, 088): ход виден только от quietShowRead новых файлов.
type scanMode int

const (
	scanLoud scanMode = iota
	scanQuiet
)

// quietShowRead — от стольких новых файлов тихое чтение показывает ход: это около секунды чтения,
// и без полосы фотограф не поймёт, что лента ещё растёт (088-3).
const quietShowRead = 50

// rescanWith — rescan в режиме mode. Тихое при идущем чтении не начинается: идущее и так свежее.
// Папки недоступных томов (088-4) не обходятся в обоих режимах, их фото остаются в ленте.
func (a *App) rescanWith(mode scanMode) {
	defer guard("rescan")
	quiet := mode == scanQuiet

	// Список папок и поколение — одним куском под rescanMu: два rescan подряд (OnFileDrop и Undo)
	// не могут поменяться местами, и новое поколение всегда с новым списком. Folders — до a.mu:
	// замок редактора держат правки (и снимок прогрева), а a.mu ждут publish, infos и /media.
	a.rescanMu.Lock()
	src := a.sources()
	a.mu.Lock()
	if quiet && a.scanning {
		a.mu.Unlock()
		a.rescanMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	if a.scanCancel != nil {
		a.scanCancel()
	}
	stopWarm := a.warm != nil
	if stopWarm {
		a.warm()
		a.warm = nil
	}
	// Тихое без прерванного прогрева греет только новые фото (088-7): прежние уже в кэше. Карту
	// не копируем: реестр всегда заменяется целиком, а не правится на месте.
	var seen map[string]library.Photo
	if quiet && !stopWarm {
		seen = a.byID
	}
	a.scanCancel = cancel
	a.scanGen++
	gen := a.scanGen
	a.scanning = true
	// Новые срезы, а не правка на месте: прежний уже мог уйти в GetState или в событие.
	kept := keptPhotos(a.photos, src)
	next := byIDMap(kept)
	a.mediaSt.forget(a.byID, next) // плитки прежнего альбома из очереди браузера — не промах
	a.photos, a.byID = kept, next
	if !quiet {
		a.scan = library.ScanProgress{Active: true, Walking: true}
	}
	pr := a.scan
	a.mu.Unlock()
	a.rescanMu.Unlock()

	if stopWarm {
		// Прогрев отменён: полоска «Готовлю миниатюры» сбрасывается (0/0 — прогрева нет).
		a.emitMu.Lock()
		a.emit("thumbs", 0, 0)
		a.emitMu.Unlock()
	}
	if quiet {
		a.refreshMissingQuiet()
	} else {
		a.publish(gen, kept, pr)
		a.refreshMissing()
	}
	a.bg.Go(func() {
		defer guard("сканирование")
		defer a.scanDone(gen)
		a.readFolders(ctx, gen, src, kept, quiet, seen)
	})
}

// readFolders — фоновая часть rescanWith: проба томов, чтение, лог и прогрев миниатюр.
func (a *App) readFolders(ctx context.Context, gen int, src library.Sources, kept []library.Photo,
	quiet bool, seen map[string]library.Photo) {
	// Проба тома — до 1,5 с на папку, поэтому здесь, а не до возврата rescan.
	live, dead := splitLive(src.Folders)
	liveFiles, deadFiles := splitLive(src.Files)
	deadKept := photosUnder(kept, dead, deadFiles)
	show, decided := !quiet, !quiet
	var fin library.ScanProgress
	photos, skipped, err := a.scanner.Scan(ctx, library.Sources{Folders: live, Files: liveFiles}, func(ph []library.Photo, cur library.ScanProgress, final bool) {
		if final {
			fin = cur
		}
		if ph != nil && len(deadKept) > 0 {
			ph = library.MergeNatural(ph, deadKept)
		}
		if !decided && !cur.Walking {
			// Первый отчёт после обхода: Found − Read — файлы, которых нет в памяти сканера.
			decided, show = true, cur.Found-cur.Read >= quietShowRead
		}
		if show {
			a.publish(gen, ph, cur)
		} else if ph != nil {
			a.publishList(gen, ph)
		}
	})
	if err != nil {
		return // отменено: уже идёт новое сканирование
	}
	log.Print(scanLogLine(src, len(photos), fin, len(skipped)))
	album.LogMem("чтение папок")
	if n := len(dead) + len(deadFiles); n > 0 {
		log.Printf("  на недоступном диске источников: %d — не читались", n)
	}
	if len(skipped) > 0 {
		log.Printf("  не читается: %s", skippedSummary(skipped))
	}
	if seen != nil {
		// Новый срез, не DeleteFunc: photos уже ушёл в реестр и событие финальным списком.
		var fresh []library.Photo
		for _, p := range photos {
			if _, ok := seen[p.ID]; !ok {
				fresh = append(fresh, p)
			}
		}
		if len(fresh) == 0 {
			return // новых нет — прогрева и событий thumbs тоже
		}
		photos = fresh
	}
	a.warmThumbs(gen, photos)
}

// scanDone снимает «идёт чтение» своего поколения; новое поколение ставит его само.
func (a *App) scanDone(gen int) {
	a.mu.Lock()
	if gen == a.scanGen {
		a.scanning = false
	}
	a.mu.Unlock()
}

// splitLive делит папки на доступные и папки томов, которые не отвечают (reachable).
func splitLive(folders []string) (live, dead []string) {
	for _, f := range folders {
		if reachable(f) {
			live = append(live, f)
		} else {
			dead = append(dead, f)
		}
	}
	return live, dead
}

// photosUnder — фото из photos, лежащие в одной из папок dirs или это один из файлов files, в прежнем порядке.
func photosUnder(photos []library.Photo, dirs, files []string) []library.Photo {
	if len(dirs)+len(files) == 0 {
		return nil
	}
	return keptPhotos(photos, library.Sources{Folders: dirs, Files: files})
}

// keptPhotos — фото из photos, которые читают источники src (папка над фото или сам файл), в прежнем
// порядке. Новый срез: прежний мог уйти в GetState или в событие.
func keptPhotos(photos []library.Photo, src library.Sources) []library.Photo {
	under := paths.UnderAny(src.Folders)
	files := make(map[string]bool, len(src.Files))
	for _, f := range src.Files {
		files[model.LooseKey(f)] = true
	}
	kept := make([]library.Photo, 0, len(photos))
	for _, p := range photos {
		if under(p.Path) || len(files) > 0 && files[model.LooseKey(p.Path)] {
			kept = append(kept, p)
		}
	}
	return kept
}

// sources — источники ленты проекта: папки и отдельные фото.
func (a *App) sources() library.Sources {
	folders, files := a.editor().Sources()
	return library.Sources{Folders: folders, Files: files}
}

// scanLogLine — итог чтения для log.txt: без путей, «отдельных» только когда они есть.
func scanLogLine(src library.Sources, photos int, fin library.ScanProgress, skipped int) string {
	s := fmt.Sprintf("папок %d: %d фото (обход %v, чтение %v, из памяти %d), пропущено %d", len(src.Folders), photos,
		fin.WalkTime.Round(time.Millisecond), fin.ReadTime.Round(time.Millisecond), fin.Memo, skipped)
	if n := len(src.Files); n > 0 {
		s += fmt.Sprintf(", отдельных %d", n)
	}
	return s
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
	a.send(gen, photos, &pr)
}

// publishList — publish без хода: тихое чтение (088-3) меняет ленту, а полосу не трогает.
func (a *App) publishList(gen int, photos []library.Photo) { a.send(gen, photos, nil) }

func (a *App) send(gen int, photos []library.Photo, pr *library.ScanProgress) {
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
	if pr != nil {
		a.scan = *pr
	}
	a.mu.Unlock()
	if photos != nil {
		a.emit("library", photos)
	}
	if pr != nil {
		a.emit("scan", *pr)
	}
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
	cancelled0 := a.mediaCancelled() // до a.warm: тест ждёт его, чтобы снять запросы во время прогрева
	a.mu.Lock()
	if gen != a.scanGen {
		a.mu.Unlock()
		return
	}
	// Чтение кончилось вместе с выдачей a.warm одним куском: сверка папок (088) не проскочит
	// между ними и не решит, что прогрева нет, — иначе он потерял бы прежние фото.
	a.scanning = false
	ctx, cancel := context.WithCancel(a.stopCtx)
	a.warm = cancel
	a.mu.Unlock()
	defer cancel()
	if a.layouts.wait(ctx); ctx.Err() != nil {
		return // отменён, пока ждал раскладки: и не начинаем
	}
	t0 := time.Now()
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
	if ctx.Err() == nil && len(photos) > 0 { // отменённый прогрев строки не пишет
		log.Print(thumbsLogLine(len(photos), time.Since(t0), a.mediaCancelled()-cancelled0))
		album.LogMem("прогрев миниатюр")
	}
	// Закончил без отмены — прогрева больше нет: тихое чтение (088-7) не прервёт его и греет одни
	// новые. Не отменён — значит, a.warm всё ещё наш: новое поколение отменило бы его.
	a.mu.Lock()
	if ctx.Err() == nil {
		a.warm = nil
	}
	a.mu.Unlock()
}

// thumbsLogLine — строка журнала конца прогрева (089 §8 п. 2, разбор — scripts/e2e/report.py); снятые
// клиентом за прогрев запросы окна — хвостом через запятую и только когда были (090 G2).
func thumbsLogLine(n int, d time.Duration, cancelled int64) string {
	s := fmt.Sprintf("миниатюры: %d за %v", n, d.Round(time.Millisecond))
	if cancelled > 0 {
		s += fmt.Sprintf(", снято запросов %d", cancelled)
	}
	return s
}

func (a *App) infos(ids []string) []model.PhotoInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]model.PhotoInfo, 0, len(ids))
	for _, id := range ids {
		p, ok := a.byID[id]
		if !ok && model.IsCopyReq(id) { // запрос копии (098): сведения файла, ID — прежний запрос
			p, ok = a.byID[model.FileOf(id)]
		}
		if ok {
			info := model.PhotoInfo{ID: id, Path: p.Path, W: p.W, H: p.H, Size: p.Size, Taken: p.Taken, Sig: a.sigOf(p)}
			if p.Xmp != nil { // отметки — только из сканера: ReadInfo их нарочно не читает
				info.Xmp = *p.Xmp
			}
			out = append(out, info)
		}
	}
	return out
}
