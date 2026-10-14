package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Перенос проекта и перепривязка недостающих фото (спека 006, §8).

// DriveHint — подсказка «Диск E: → F:». From пуст — подсказки нет.
type DriveHint struct {
	From  string `json:"from"`  // буква пропавшего диска, «E»
	To    string `json:"to"`    // буква, на которой фото нашлись
	Found int    `json:"found"` // сколько пропавших фото с диска From лежит там же на To
	Total int    `json:"total"` // сколько пропавших фото было на диске From
}

// RelinkResult — итог поиска в папке: сколько привязано, сколько осталось
// недостающими и сколько нашлось в нескольких равноценных местах (не привязаны).
type RelinkResult struct {
	View      model.View `json:"view"`
	Found     int        `json:"found"`
	Missing   int        `json:"missing"`
	Ambiguous int        `json:"ambiguous"`
}

// missingOf — ID фото, чьих файлов нет (exists — false), по порядку ID.
func missingOf(refs map[string]model.PhotoRef, exists func(string) bool) []string {
	return library.MissingOf(refs, func(r model.PhotoRef) string { return r.Path }, exists)
}

// reachable — osx.Reachable; подменяется в тестах (мёртвый сетевой том).
var reachable = osx.Reachable

// fileExists — обычный файл на месте; том, который не отвечает (отключённый сетевой диск),
// — «нет файла» без ожидания таймаута SMB.
func fileExists(path string) bool {
	if !reachable(path) {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// refreshMissing — полный пересчёт недостающих (открытие, rescan) в фоне: os.Stat тысяч фото
// (или мёртвый сетевой том) не держит startup, Undo и rescan. Отчёт прежнего полного пересчёта
// отбрасывается; событие `missing` — под emitMu, как library и scan.
func (a *App) refreshMissing() {
	a.missSyncMu.Lock()
	defer a.missSyncMu.Unlock()
	refs := a.snapRefs(a.editor())
	a.mu.Lock()
	a.missGen++
	gen := a.missGen
	a.missRefs = refs
	a.mu.Unlock()
	a.checkMissing(gen, refs, false)
}

// checkMissing — фоновая проверка фото refs (ID → путь) поколения gen. Итог сливается с уже
// известным списком (mergeMissing). quiet — событие только при смене списка (после правки).
func (a *App) checkMissing(gen int, refs map[string]string, quiet bool) {
	a.missPending.Add(1)
	a.bg.Go(func() {
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
	})
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
// сканирования, и интерфейсу имя для списка «Показать» больше взять негде. Без os.Stat.
func (a *App) MissingNames() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	m := make(map[string]string, len(a.missing))
	for _, id := range a.missing {
		if p, ok := a.missRefs[id]; ok {
			m[id] = filepath.Base(p)
		}
	}
	return m
}

// SuggestDrive предлагает смену буквы диска: пропавшие фото на самой частой
// пропавшей букве ищутся на остальных существующих дисках. Только чтение.
func (a *App) SuggestDrive() DriveHint {
	defer guard("SuggestDrive")
	refs := a.editor().Refs()
	var gone []string
	for _, id := range missingOf(refs, a.exists) {
		gone = append(gone, refs[id].Path)
	}
	// Буквы дисков тоже через Reachable: отключённая сетевая буква иначе держит подсказку.
	return DriveHint(library.SuggestDrive(gone, func(p string) bool {
		if !reachable(p) {
			return false
		}
		_, err := os.Stat(p)
		return err == nil
	}))
}

// RelinkDrive меняет букву диска у недостающих фото и папок, если файлы нашлись
// на новой. Буквы A–Z (допускается «E:»); ошибка — только для неверных букв.
func (a *App) RelinkDrive(from, to string) (model.View, error) {
	ed := a.editor()
	f, ok1 := paths.DriveLetter(from)
	t, ok2 := paths.DriveLetter(to)
	if !ok1 || !ok2 || f == t {
		return ed.View(), fmt.Errorf("нужны две разные буквы диска")
	}
	v := a.changed(ed.RelinkDrive(f, t))
	a.rescan()
	return v, nil
}

// RelinkFolder спрашивает папку и ищет в ней недостающие фото.
func (a *App) RelinkFolder() (RelinkResult, error) {
	dir, err := wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Где лежат недостающие фото",
		DefaultDirectory: a.dialogDir(prefs.DirPhotos),
	})
	if err != nil || dir == "" {
		return RelinkResult{View: a.editor().View()}, err
	}
	res, err := a.RelinkFolderPath(dir)
	if err == nil {
		a.rememberDir(prefs.DirPhotos, filepath.Dir(dir))
	}
	return res, err
}

// RelinkFolderPath ищет недостающие фото в папке dir (локальной) по имени и
// размеру файла и перепривязывает найденные. Не нашлось — проект не меняется.
func (a *App) RelinkFolderPath(dir string) (res RelinkResult, rerr error) {
	defer guardErr("RelinkFolderPath", &rerr)
	ed := a.editor()
	res = RelinkResult{View: ed.View()}
	if err := paths.CheckLocalPath(dir); err != nil {
		return res, err
	}
	dir = filepath.Clean(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return res, fmt.Errorf("не папка: %s", dir)
	}
	refs := ed.Refs()
	ids := missingOf(refs, a.exists) // синхронно: ответ нужен сразу, вызов — от действия пользователя
	if len(ids) == 0 {
		return res, nil
	}
	want := make([]library.Want, len(ids))
	for i, id := range ids {
		r := refs[id]
		want[i] = library.Want{ID: id, Old: r.Path, Size: r.Size, W: r.W, H: r.H}
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	found, err := library.FindByName(ctx, dir, want, 0)
	if err != nil {
		return res, err
	}
	res.Found, res.Ambiguous = len(found.Paths), len(found.Ambiguous)
	res.Missing = len(ids) - res.Found
	if res.Found > 0 {
		res.View = a.changed(ed.Relink(found.Paths, dir))
		a.rescan()
	}
	return res, nil
}
