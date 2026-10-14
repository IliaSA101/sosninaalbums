package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Резервные копии альбома (спека 019 E). a.backups == nil — копий нет, остальное работает.

// BackupInfo — строка «Резервных копий». Time — unix-миллисекунды (местное время из имени).
type BackupInfo struct {
	Name    string `json:"name"`
	Time    int64  `json:"time"`
	Size    int64  `json:"size"`
	Spreads int    `json:"spreads"`
	Photos  int    `json:"photos"`
}

const backupNotice = "Это резервная копия — сохраните её через «Сохранить как»"

// startBackups — хранилище копий и чистка старых в фоне (startup).
func (a *App) startBackups() {
	root := backup.DefaultRoot()
	if root == "" {
		return
	}
	a.backups = backup.New(root)
	a.pruneBackups()
}

// pruneBackups — общая чистка копий в фоне: при старте и каждые несколько записей за сеанс.
func (a *App) pruneBackups() {
	if a.stopped.Load() {
		return // выход: closeBackground уже ждёт фон
	}
	a.bg.Go(func() {
		defer guard("резервные копии")
		if err := a.backups.Prune(); err != nil {
			log.Printf("резервные копии, чистка: %v", paths.NoPath(err))
		}
	})
}

// backupKey — папка копий альбома path ("" — черновик).
func backupKey(path string) string {
	if path == "" {
		return backup.DraftKey
	}
	return backup.Key(path)
}

// backupBefore — копия прежнего файла перед его перезаписью: force — без учёта «раз в 10 минут».
// Сбой копии не мешает сохранению: только лог (с ключом, без пути).
func (a *App) backupBefore(path, key string, force bool) {
	a.backupKeep(path, key, force, "")
}

// backupKeep — backupBefore, только чистка не удаляет копию keep ("" — любую).
func (a *App) backupKeep(path, key string, force bool, keep string) {
	// Пропавший сетевой диск: чтение ждало бы таймаута SMB, а следом его ждёт и сама запись.
	if a.backups == nil || !reachable(path) {
		return
	}
	var err error
	switch {
	case keep != "":
		err = a.backups.ForceKeep(path, key, keep)
	case force:
		err = a.backups.Force(path, key)
	default:
		err = a.backups.Before(path, key)
	}
	if err != nil {
		log.Printf("резервная копия %s: %s", key, paths.NoPath(err))
	}
	if a.backups.PruneDue() {
		a.pruneBackups()
	}
}

// backupDraft — копия черновика перед тем, как его удалят или затрут чужим содержимым;
// пустой черновик не копируем.
func (a *App) backupDraft() { a.backupDraftKeep("") }

// backupDraftKeep — backupDraft, чистка не удаляет копию keep.
func (a *App) backupDraftKeep(keep string) {
	if a.backups == nil || a.draftNewer.Load() || !backup.HasContent(a.draftPath()) {
		return
	}
	a.backupKeep(a.draftPath(), backup.DraftKey, true, keep)
}

// Backups — копии текущего альбома (или черновика), новые первыми.
func (a *App) Backups() (list []BackupInfo, err error) {
	defer guardErr("Backups", &err)
	list = []BackupInfo{}
	if a.backups == nil {
		return list, nil
	}
	ents, err := a.backups.List(backupKey(a.editor().Path()))
	if err != nil {
		return list, err
	}
	for _, e := range ents {
		list = append(list, BackupInfo{Name: e.Name, Time: e.Time.UnixMilli(), Size: e.Size, Spreads: e.Spreads, Photos: e.Photos})
	}
	return list, nil
}

// RestoreBackup заменяет альбом содержимым копии name одним шагом отмены. Имя — только из
// Backups: путь строится по ключу текущего альбома. Перед заменой текущий файл уходит в копию,
// а сама name при этой чистке не удаляется: иначе после Ctrl+Z её версии нигде не было бы.
func (a *App) RestoreBackup(name string) (v model.View, err error) {
	defer a.guardEdit("RestoreBackup", &v, &err)
	ed := a.editor()
	path := ed.Path()
	key := backupKey(path)
	file, ferr := a.backups.File(key, name)
	if ferr != nil {
		return ed.View(), backup.ErrNotFound
	}
	base := "" // черновик: только абсолютные пути
	if path != "" {
		if abs, aerr := filepath.Abs(path); aerr == nil {
			base = filepath.Dir(abs) // альбом переехал вместе с фото — относительные пути найдут их
		}
	}
	q, lerr := album.LoadBase(file, base)
	switch {
	case errors.Is(lerr, salbum.ErrNewer):
		return ed.View(), errors.New("копия записана более новой версией программы")
	case errors.Is(lerr, os.ErrNotExist):
		return ed.View(), backup.ErrNotFound
	case lerr != nil:
		return ed.View(), lerr
	}
	if path != "" {
		a.backupKeep(path, key, true, name)
	} else {
		a.backupDraftKeep(name)
	}
	before := ed.Folders()
	v, err = ed.Restore(q)
	if err != nil {
		return v, err
	}
	v = a.changed(v)
	// Недостающие при смене набора фото без смены папок пересчитывает changed.
	if !slices.Equal(before, v.Folders) {
		a.rescan()
	}
	return v, nil
}

// ShowBackups открывает папку копий текущего альбома в Проводнике; её нет — корень копий.
func (a *App) ShowBackups() error {
	if a.backups == nil {
		return errors.New("папка резервных копий недоступна")
	}
	dir := a.backups.Dir(backupKey(a.editor().Path()))
	if dir == "" || !isDir(dir) {
		dir = a.backups.Root
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return revealPath(dir, false)
}

// openBackup открывает копию несохранённым альбомом: папку копий программа сама не перезаписывает,
// LastProject и «Последние» не меняются. Прежний черновик — в копию: автосохранение его затрёт.
func (a *App) openBackup(path string) (State, error) {
	p, err := album.LoadBase(path, "")
	if err != nil {
		return a.GetState(), err
	}
	ed, err := album.NewEditor(p, "")
	if err != nil {
		return a.GetState(), err
	}
	a.flushSave()
	a.backupDraft()
	a.ed.Store(ed)
	a.rescan()
	st := a.emitState()
	a.notice(backupNotice)
	return st, nil
}

// checkSaveTarget — «Сохранить как» в папку копий запрещено: её чистит Prune.
func (a *App) checkSaveTarget(path string) error {
	if a.backups.Inside(path) {
		return fmt.Errorf("сохраните альбом в другую папку: здесь резервные копии")
	}
	return nil
}

// rememberProject — альбом стал текущим файлом: в конфиг и папку диалога.
func (a *App) rememberProject(path string) {
	a.updateConfig(func(c *prefs.Config) { c.LastProject = path; c.Recent = prefs.PushRecent(c.Recent, path) })
	a.rememberDir(prefs.DirProject, filepath.Dir(path))
}
