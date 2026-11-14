package edit

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Подборки фото (083): правки Editor с рассылкой вида. ID фото не из ленты пропускает m.infos —
// путь от интерфейса в файл не попадает.

// NewBin — новая подборка в конце View.bins с фото ленты ids, один шаг.
func (m *Methods) NewBin(name string, ids []string) (v model.View, err error) {
	defer m.guardEdit("NewBin", &v, &err)
	return m.changedOrErr(m.editor().NewBin(name, m.infos(ids)))
}

// RenameBin — новое имя подборки; пустое или то же — холостая.
func (m *Methods) RenameBin(id, name string) (v model.View, err error) {
	defer m.guardEdit("RenameBin", &v, &err)
	return m.changedOrErr(m.editor().RenameBin(id, name))
}

// DeleteBin — подборка и её членство у всех фото, один шаг; фото остаются в ленте и альбоме.
func (m *Methods) DeleteBin(id string) (v model.View) {
	defer m.guardEdit("DeleteBin", &v, nil)
	return m.changed(m.editor().DeleteBin(id))
}

// SetBin — фото ленты ids в подборку (in) или из неё; добавить или убрать решает интерфейс.
func (m *Methods) SetBin(id string, ids []string, in bool) (v model.View, err error) {
	defer m.guardEdit("SetBin", &v, &err)
	return m.changedOrErr(m.editor().SetBin(id, m.infos(ids), in))
}
