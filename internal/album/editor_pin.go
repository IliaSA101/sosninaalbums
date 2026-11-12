package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Отпечаток при выгрузке по ссылке (167): печать пути (uidAt) решений не принимает — файл,
// перенесённый или открытый по другому пути, остаётся тем же проектом, а «тот же ли это альбом»
// спрашивает окно (экран moved).

// NeedFileUID — нужно ли записать отпечаток в файл перед выгрузкой: альбом всё ещё path с
// отпечатком uid (иначе ErrUIDChanged), а в файле его нет (альбом до 0.32, Restore или Ctrl+Z
// сменили отпечаток).
func (e *Editor) NeedFileUID(path, uid string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return false, err
	}
	return e.diskUID != uid, nil
}

// SaveFileUID — запись отпечатка uid в файл (NeedFileUID) со сверкой под тем же замком. Отпечаток
// уже в файле — ничего не пишет.
func (e *Editor) SaveFileUID(path, uid string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return "", err
	}
	if e.diskUID == uid {
		return uid, nil
	}
	return uid, e.save("")
}

// SaveFresh — «выгрузить как новый»: альбому выдаётся новый отпечаток без привязок (привязки лежат
// по отпечатку и остаются прежнему), печать пути ставится, файл пишется. Прежний отпечаток и его
// запись о ссылке не трогаются. Запись не удалась — отпечаток прежний. Сверка path и uid — под замком.
func (e *Editor) SaveFresh(path, uid string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return "", err
	}
	nu, undo := e.issue(e.path)
	if err := e.save(""); err != nil {
		undo()
		return "", err
	}
	return nu, nil
}

// SaveOver — «Сохранить как» в path: решение «тот же проект или другой» и запись под одним замком
// (иначе Restore или Ctrl+Z между сверкой и записью положили бы другой проект поверх файла).
// Тот же путь, что у альбома, — обычная запись. Другой путь: у файла-цели на диске тот же
// отпечаток (копия → «Сохранить как» поверх оригинала, 167-25) — отпечаток и привязки
// сохраняются; иначе новый отпечаток без привязок. fresh — выдан новый отпечаток.
func (e *Editor) SaveOver(path string) (fresh bool, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.path != "" && paths.SamePath(e.path, path) {
		return false, e.save(path)
	}
	if uid := e.p.UID; uid != "" && salbum.FileUID(path) == uid {
		return false, e.save(path)
	}
	_, undo := e.issue(path)
	if err := e.save(path); err != nil {
		undo()
		return false, err
	}
	return true, nil
}
