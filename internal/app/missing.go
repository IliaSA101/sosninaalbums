package app

import (
	"path/filepath"
	"slices"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Недостающие фото альбома: проверка файлов, список, имена.

// missingOf — ID фото фотографа, чьих файлов нет (exists — false), по порядку ID. Свои из
// библиотеки (Lib, 048) не входят: в папке фотографа и на другом диске их не найти —
// они находятся сами, когда тот же файл загружен в библиотеку (RelinkLib).
func missingOf(refs map[string]model.PhotoRef, exists func(string) bool) []string {
	ids := library.MissingOf(refs, func(r model.PhotoRef) string { return r.Path },
		func(p string) bool { return p == "" || exists(p) }) // пустой путь — только у своих
	return slices.DeleteFunc(ids, func(id string) bool { return refs[id].Lib != "" })
}

// reachable — osx.Reachable; подменяется в тестах (мёртвый сетевой том).
var reachable = osx.Reachable

// fileExists — обычный файл на месте; том, который не отвечает (отключённый сетевой диск),
// — «нет файла» без ожидания таймаута SMB. Путь, который не проходит CheckLocalPath (фото
// «CON.jpg» с Windows 11 на Windows 10, 073-10), — тоже «нет»: Stat открыл бы устройство.
// Ссылка на нелокальный путь (\\host\share) — тоже «нет»: StatLocal не идёт по ней на диск.
func fileExists(path string) bool {
	if paths.CheckLocalPath(path) != nil || !reachable(path) {
		return false
	}
	st, err := library.StatLocal(path)
	return err == nil && st.Mode().IsRegular()
}

// refreshMissing — полный пересчёт недостающих (открытие, rescan) в фоне: os.Stat тысяч фото
// (или мёртвый сетевой том) не держит startup, Undo и rescan. Отчёт прежнего полного пересчёта
// отбрасывается; событие `missing` — под emitMu, как library и scan.
func (a *App) refreshMissing() { a.refreshMissingWith(false) }

// refreshMissingQuiet — тот же полный пересчёт для сверки папок (088): событие `missing` только
// при смене списка — без изменений интерфейс ничего не получает.
func (a *App) refreshMissingQuiet() { a.refreshMissingWith(true) }

func (a *App) refreshMissingWith(quiet bool) {
	a.missSyncMu.Lock()
	defer a.missSyncMu.Unlock()
	refs := a.snapRefs(a.editor())
	a.mu.Lock()
	a.missGen++
	gen := a.missGen
	a.missRefs = refs
	a.mu.Unlock()
	a.checkMissing(gen, refs, quiet)
}

// checkMissing — фоновая проверка фото refs (ID → путь) поколения gen. Итог сливается с уже
// известным списком (mergeMissing). quiet — событие только при смене списка (после правки, сверка).
func (a *App) checkMissing(gen int, refs map[string]string, quiet bool) {
	a.missPending.Add(1)
	if !a.goBG(func() {
		defer a.missPending.Add(-1)
		defer guard("missing")
		gone := library.MissingOf(refs, func(p string) string { return p }, a.exists)
		a.emitMu.Lock()
		defer a.emitMu.Unlock()
		a.mu.Lock()
		// Начался полный пересчёт — он проверит и эти фото, его итог свежее.
		if gen != a.missGen || a.stopped.Load() {
			a.mu.Unlock()
			return
		}
		ids := mergeMissing(a.missing, a.missRefs, refs, gone)
		same := slices.Equal(ids, a.missing)
		if !same {
			a.missing = ids // новый срез: прежний уже мог уйти в GetState или в событие
		}
		a.mu.Unlock()
		if !quiet || !same {
			a.emit("missing", ids)
		}
	}) {
		a.missPending.Add(-1) // фон закрыт: проверка не началась
	}
}

// mergeMissing — новый список недостающих по порядку ID: из прежнего cur уходят фото, которых
// нет в альбоме (now), и проверенные заново; из проверенных (checked, пропали gone) входят те,
// чей путь в альбоме тот же, что при проверке. Фото, убранное или переехавшее, пока шла проверка,
// её итогом не возвращается: его путь сверит своя проверка.
func mergeMissing(cur []string, now, checked map[string]string, gone []string) []string {
	out := make([]string, 0, len(cur)+len(gone))
	for _, id := range cur {
		p, ok := now[id]
		if !ok {
			continue
		}
		if q, ok := checked[id]; ok && q == p {
			continue // решает свежая проверка ниже
		}
		out = append(out, id)
	}
	for _, id := range gone {
		if p, ok := now[id]; ok && p == checked[id] {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// editorPaths — ID → путь фото альбома: по нему правка видит, сменился ли набор фото.
func editorPaths(ed *album.Editor) map[string]string {
	refs := ed.Refs()
	m := make(map[string]string, len(refs))
	for id, r := range refs {
		m[id] = r.Path
	}
	return m
}

// syncMissing — после правки (changed): набор фото альбома мог смениться. Убывшие снимаются
// сразу без os.Stat; в фон уходят только новые и переехавшие фото, итог сливается с известными.
// Набор прежний — ничего, и событие `missing` не шлётся. Снимок и запись — под missSyncMu:
// при параллельных правках снимок, прочитанный раньше, раньше и ложится.
func (a *App) syncMissing(rev int) {
	defer guard("syncMissing")
	a.missSyncMu.Lock()
	defer a.missSyncMu.Unlock()
	ed := a.editor()
	wed := weak.Make(ed)
	if a.missEd == wed && a.missRev == rev {
		return // холостая правка или тот же rev: refs не копируем
	}
	a.missEd, a.missRev = wed, rev
	refs := a.snapRefs(ed)
	gen, fresh := a.dropMissing(refs)
	if fresh != nil {
		a.checkMissing(gen, fresh, true)
	}
}

// dropMissing кладёт набор фото refs в missRefs и сразу снимает убывшие из списка; отдаёт
// поколение и новые или переехавшие фото (nil — таких нет). emitMu — снаружи mu, как в
// checkMissing: событие не обгонит отчёт фоновой проверки.
func (a *App) dropMissing(refs map[string]string) (gen int, fresh map[string]string) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	for id, p := range refs {
		if q, ok := a.missRefs[id]; !ok || q != p {
			if fresh == nil {
				fresh = map[string]string{}
			}
			fresh[id] = p
		}
	}
	gen = a.missGen
	if fresh == nil && len(refs) == len(a.missRefs) {
		a.mu.Unlock()
		return gen, nil
	}
	a.missRefs = refs
	kept := make([]string, 0, len(a.missing))
	for _, id := range a.missing {
		if _, ok := refs[id]; ok {
			kept = append(kept, id)
		}
	}
	same := len(kept) == len(a.missing)
	if !same {
		a.missing = kept // новый срез: прежний уже мог уйти в GetState или в событие
	}
	a.mu.Unlock()
	if !same {
		a.emit("missing", kept)
	}
	return gen, fresh
}

// MissingNames — имена файлов недостающих фото (ID → имя): пропавшего файла нет в ленте
// сканирования, и интерфейсу имя для списка «Показать» больше взять негде. Без os.Stat. Своего
// (Lib) нет: это плитка библиотеки, у него пустой путь или имя файла-ключа, а не имя фото.
func (a *App) MissingNames() map[string]string {
	refs := a.editor().Refs() // до a.mu: замки редактора и App не вкладываются
	a.mu.Lock()
	defer a.mu.Unlock()
	m := make(map[string]string, len(a.missing))
	for _, id := range a.missing {
		if refs[id].Lib != "" {
			continue
		}
		if p, ok := a.missRefs[id]; ok {
			m[id] = filepath.Base(p)
		}
	}
	return m
}
