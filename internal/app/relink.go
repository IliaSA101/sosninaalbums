package app

import (
	"context"
	"errors"
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
	if err != nil {
		return RelinkResult{View: a.editor().View()}, err
	}
	if dir == "" {
		return RelinkResult{View: a.out(a.editor().View())}, nil
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
		res.View = a.out(res.View) // ответ без ошибки — в цепочку видов (075)
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
	if res.Found == 0 {
		res.View = a.out(res.View)
		return res, nil
	}
	res.View = a.changed(ed.Relink(found.Paths, dir))
	a.rescan()
	return res, nil
}

// ShowPhotoInExplorer показывает файл фото альбома в Проводнике. Путь берётся из
// проекта по ID, от интерфейса не принимается (как ShowInExplorer). Файла нет —
// открывается его папка; нет и её — ошибка.
func (a *App) ShowPhotoInExplorer(id string) error {
	r, ok := a.editor().Ref(id)
	if !ok {
		return errors.New("фото нет в альбоме")
	}
	path := filepath.Clean(r.Path)
	if err := paths.CheckLocalPath(path); err != nil {
		return errors.New("путь к фото недоступен")
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return revealPath(path, true)
	}
	if dir := filepath.Dir(path); isDir(dir) {
		return revealPath(dir, false)
	}
	return errors.New("файл не найден")
}

// revealPath — запуск файлового менеджера; в тестах подменяется, чтобы не стартовать процесс.
// file — выделить файл в его папке, иначе открыть папку. Ждать завершения не нужно.
var revealPath = osx.Reveal

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
