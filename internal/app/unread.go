package app

import (
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
	a.mu.Unlock()
	a.emit("unread", next)
}
