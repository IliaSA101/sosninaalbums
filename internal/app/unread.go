package app

import (
	"maps"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Отдельные файлы, которые найдены, но не читаются (117): окно показывает «не читается» с причиной,
// а не «нет на диске». Пропуски в папках сюда не входят — они только в журнале.

// unreadOf — ID фото отдельного файла → причина, по пропускам skipped среди отдельных файлов src.
// Ключ — тот же library.IDFor пути, что отдаёт вид (LooseFileView.Photo). Пустой результат — не nil.
func unreadOf(src library.Sources, skipped []library.Skip) map[string]string {
	loose := make(map[string]bool, len(src.Files))
	for _, f := range src.Files {
		loose[model.LooseKey(f)] = true
	}
	out := map[string]string{}
	for _, s := range skipped {
		if loose[model.LooseKey(s.Path)] {
			out[library.IDFor(s.Path)] = s.Why
		}
	}
	return out
}

// prunedUnread — запись остаётся, пока её файл среди отдельных файлов src: сменился альбом — чужие
// причины в State не попадают, а у прежнего чтения до его конца ничего не мигает. Под a.mu.
func prunedUnread(cur map[string]string, src library.Sources) map[string]string {
	if len(cur) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, f := range src.Files {
		if why, ok := cur[library.IDFor(f)]; ok {
			out[library.IDFor(f)] = why
		}
	}
	return out
}

// publishUnread кладёт итог чтения в a.unread и шлёт событие `unread` (тот же вид, что State.Unread)
// после каждого окончания чтения, и пустое тоже: иначе после замены файла запись осталась бы в окне.
// Итог устаревшего чтения (другое поколение) отбрасывается.
func (a *App) publishUnread(gen int, src library.Sources, skipped []library.Skip) {
	next := unreadOf(src, skipped)
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if gen != a.scanGen || a.stopped.Load() {
		a.mu.Unlock()
		return
	}
	a.unread = next // новая карта: прежняя уже могла уйти в событие
	all := a.unreadAllLocked()
	a.mu.Unlock()
	a.emit("unread", all)
}

// unreadAllLocked — причины чтения (a.unread) и миниатюр (a.badThumb) одной картой для State и
// события `unread`; причина чтения сильнее. Всегда новая карта (может уйти в событие). Под a.mu.
func (a *App) unreadAllLocked() map[string]string {
	if len(a.badThumb) == 0 {
		return maps.Clone(a.unread)
	}
	out := maps.Clone(a.badThumb)
	maps.Copy(out, a.unread)
	return out
}

// badThumbMax — предел запомненных причин миниатюр; дальше карта начинается заново.
const badThumbMax = 4096

// noteBadThumb запоминает, что оригинал фото id не декодируется (усечён, повреждён): заголовок
// прочёлся, фото в ленте есть, а миниатюры нет. Окно показывает «файл повреждён» тем же текстом,
// что у не прочитанных при чтении (событие `unread`), а не «нет файла». Та же причина — без события.
func (a *App) noteBadThumb(id, why string) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if a.stopped.Load() || a.badThumb[id] == why {
		a.mu.Unlock()
		return
	}
	if a.badThumb == nil || len(a.badThumb) >= badThumbMax {
		a.badThumb = map[string]string{}
	}
	a.badThumb[id] = why
	all := a.unreadAllLocked()
	a.mu.Unlock()
	a.emit("unread", all)
}

// clearBadThumb снимает причину после удачной миниатюры (файл перезаписан и читается).
func (a *App) clearBadThumb(id string) {
	a.mu.Lock()
	_, bad := a.badThumb[id]
	a.mu.Unlock()
	if !bad {
		return
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if _, bad = a.badThumb[id]; !bad || a.stopped.Load() {
		a.mu.Unlock()
		return
	}
	delete(a.badThumb, id)
	all := a.unreadAllLocked()
	a.mu.Unlock()
	a.emit("unread", all)
}

// prunedBadThumb — запись остаётся, пока фото в реестре byID или среди отдельных файлов src: сменился
// альбом — чужие причины не остаются. Под a.mu.
func prunedBadThumb(cur map[string]string, byID map[string]library.Photo, src library.Sources) map[string]string {
	if len(cur) == 0 {
		return nil
	}
	loose := make(map[string]bool, len(src.Files))
	for _, f := range src.Files {
		loose[library.IDFor(f)] = true
	}
	out := map[string]string{}
	for id, why := range cur {
		if _, ok := byID[id]; ok || loose[id] {
			out[id] = why
		}
	}
	return out
}
