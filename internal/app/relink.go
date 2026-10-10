package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

// refreshMissing ставит пересчёт недостающих в фон: os.Stat тысяч фото (или мёртвый сетевой том)
// не держит startup, Undo и rescan. Снимок refs берётся сейчас, отчёт устаревшего поколения
// отбрасывается; событие `missing` — под emitMu, как library и scan.
func (a *App) refreshMissing() {
	a.mu.Lock()
	a.missGen++
	gen := a.missGen
	a.mu.Unlock()
	refs := a.editor().Refs()
	a.missPending.Add(1)
	a.bg.Go(func() {
		defer a.missPending.Add(-1)
		defer guard("missing")
		ids := missingOf(refs, a.exists)
		a.emitMu.Lock()
		defer a.emitMu.Unlock()
		a.mu.Lock()
		if gen != a.missGen || a.stopped.Load() {
			a.mu.Unlock()
			return
		}
		a.missing = ids
		a.mu.Unlock()
		a.emit("missing", ids)
	})
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
