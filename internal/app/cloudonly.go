package app

import (
	"maps"
	"runtime"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Фото в облаке без готовой копии (160-M1, доправка 6): файл — заглушка OneDrive «только в сети», а миниатюры нет.
// Открыть его значит скачать, поэтому плитка получает состояние «не читается» с причиной (State.Unread и событие
// `unread`, как у не прочитанных при сканировании, 117) и не просит /media/t|p — ни запросов, ни 404 в консоли.

// cloudStub — файл path лежит в облаке (заглушка с данными не на диске). Шов: тест подменяет его целиком на любой ОС,
// а в проде вне Windows заглушек нет — Stat не нужен. Только сведения файла, данные не читаются.
var cloudStub = func(path string) bool {
	if runtime.GOOS != "windows" {
		return false
	}
	st, err := library.StatLocal(path)
	return err == nil && imgfile.Dehydrated(st)
}

// cloudNoCopy — path заглушка облака, и ни миниатюры, ни превью для неё в кэше нет (Cache.Cached по любому цвету).
func (a *App) cloudNoCopy(path string) bool {
	if path == "" || !cloudStub(path) {
		return false
	}
	if a.cache == nil {
		return true
	}
	ph := library.Photo{Path: path}
	_, has := a.cache.CachedAny(ph, false)
	return !has
}

// cloudScan проверяет refs (ID → путь) и ещё фото из a.cloud вне refs (путь — из реестра ленты): им нужна перепроверка,
// иначе скачанный файл остался бы «в облаке». Отдаёт проверенные пути и найденные заглушки без копии (ID → причина).
// Диска касается вне замков.
func (a *App) cloudScan(refs map[string]string, gone []string) (found, checked map[string]string) {
	checked = refs
	cloned := false
	a.mu.Lock()
	for id := range a.cloud {
		if _, in := refs[id]; in {
			continue
		}
		if p, ok := a.byID[model.BaseID(id)]; ok {
			if !cloned {
				checked, cloned = maps.Clone(refs), true // refs — карта вызывающего, дополнять её нельзя
			}
			checked[id] = p.Path
		}
	}
	a.mu.Unlock()
	found = map[string]string{}
	for id, p := range checked {
		if _, missing := slices.BinarySearch(gone, id); !missing && a.cloudNoCopy(p) { // пропавшее — не «в облаке»
			found[id] = library.WhyCloudOnly
		}
	}
	return found, checked
}

// pathNowLocked — нынешний путь фото id: проект (missRefs) или реестр ленты. Под a.mu.
func (a *App) pathNowLocked(id string) (string, bool) {
	if p, ok := a.missRefs[id]; ok {
		return p, true
	}
	p, ok := a.byID[model.BaseID(id)] // копия фото (-N) — тот же файл
	return p.Path, ok
}

// applyCloudLocked сливает итог cloudScan в a.cloud: проверенные фото решает свежий итог (found), остальные остаются,
// пока фото есть в проекте или ленте; найденное принимается, если путь фото с проверки не сменился (как mergeMissing).
// Отдаёт, изменилась ли карта. Карта всегда новая: прежняя могла уйти в событие. Под a.mu.
func (a *App) applyCloudLocked(found, checked map[string]string) bool {
	var next map[string]string
	for id, why := range a.cloud {
		if _, again := checked[id]; again {
			continue
		}
		if _, ok := a.pathNowLocked(id); ok {
			if next == nil {
				next = map[string]string{}
			}
			next[id] = why
		}
	}
	for id, why := range found {
		if p, ok := a.pathNowLocked(id); ok && p == checked[id] {
			if next == nil {
				next = map[string]string{}
			}
			next[id] = why
		}
	}
	if maps.Equal(next, a.cloud) {
		return false
	}
	a.cloud = next
	return true
}

// pruneCloudLocked убирает записи фото, которых нет ни в проекте (refs), ни в ленте. Под a.mu.
func (a *App) pruneCloudLocked(refs map[string]string) bool {
	var next map[string]string
	for id, why := range a.cloud {
		_, inRefs := refs[id]
		_, inRibbon := a.byID[model.BaseID(id)]
		if inRefs || inRibbon {
			if next == nil {
				next = map[string]string{}
			}
			next[id] = why
		}
	}
	if len(next) == len(a.cloud) {
		return false
	}
	a.cloud = next
	return true
}

// noteCloud запоминает, что фото id лежит в облаке без копии (запрос миниатюры это увидел раньше сверки файлов) и
// шлёт `unread`; запись уже есть — тишина.
func (a *App) noteCloud(id string) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if a.stopped.Load() || a.cloud[id] == library.WhyCloudOnly {
		a.mu.Unlock()
		return
	}
	next := maps.Clone(a.cloud)
	if next == nil {
		next = map[string]string{}
	}
	next[id] = library.WhyCloudOnly
	a.cloud = next
	all := a.unreadAllLocked()
	a.mu.Unlock()
	a.emit("unread", all)
}

// clearCloud снимает запись после отданной копии: файл скачали, миниатюра есть.
func (a *App) clearCloud(id string) {
	a.mu.Lock()
	_, has := a.cloud[id]
	a.mu.Unlock()
	if !has {
		return
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if _, has = a.cloud[id]; !has || a.stopped.Load() {
		a.mu.Unlock()
		return
	}
	next := maps.Clone(a.cloud)
	delete(next, id)
	a.cloud = next
	all := a.unreadAllLocked()
	a.mu.Unlock()
	a.emit("unread", all)
}
