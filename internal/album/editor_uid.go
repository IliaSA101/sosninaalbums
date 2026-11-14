package album

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Отпечаток проекта (146 Т4, решение 0.32-146-T4-9; model/uid.go). Он лежит в Project и потому
// в снимках отмены (Ctrl+Z после Restore чужой копии возвращает и прежний), но правкой не
// является: выдача и смена идут мимо change, без шага отмены, rev и dirty — как SyncOwnNames.

// Печать (model.UIDTag) ставится при выдаче отпечатка («Сохранить как» другим проектом, сбор
// collect.Save, «выгрузить как новый») и переносится записью как есть. Что запись о ссылке своя,
// решает путь в записи по uid (167-12), а не печать: она остаётся в файле для старых версий.

// ErrUIDChanged — альбом сменился с тех пор, как снят снимок для выгрузки (другой файл,
// «Сохранить как», восстановление копии): писать отпечаток старого снимка нельзя.
var ErrUIDChanged = errors.New("альбом сменился — начните выгрузку ещё раз")

// ensureUID выдаёт отпечаток проекту без годного (новый, файл до 0.32, копия без поля).
func ensureUID(p *model.Project) {
	if !model.GoodUID(p.UID) {
		p.UID = model.NewUID()
	}
}

// Ident — путь файла и отпечаток одним снимком (под одним замком).
func (e *Editor) Ident() (path, uid string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path, e.p.UID
}

// SnapshotAt — Snapshot вместе с путём файла и привязками (167: Clone их теряет, они поле вне JSON)
// под одним замком: снимок для выгрузки (shareJob), путь записи о ссылке и её хозяин — от одного
// альбома, а не от двух соседних.
func (e *Editor) SnapshotAt() (path string, p *model.Project, v model.View, refs []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path, e.p.Clone(), e.printView(), slices.Clone(e.refs[e.p.UID])
}

// keepSeal запоминает печать отпечатка q из файла (NewEditor, Restore) как есть; негодные — нет.
// Печать этого отпечатка, годная для пути альбома, не заменяется: копия со старой печатью (файл
// переносили) не лишает альбом своей. Под e.mu (или до выдачи Editor).
func (e *Editor) keepSeal(q *model.Project) {
	s := q.UIDAt()
	if !model.GoodUID(q.UID) || !model.GoodUIDTag(s) {
		return
	}
	if old := e.seals[q.UID]; old != "" && e.path != "" && old == model.UIDTag(q.UID, e.path) {
		return
	}
	e.seals[q.UID] = s
}

// stamp кладёт в проект печать его отпечатка для записи в файл (как есть, без сверки с путём).
// Поле вне JSON: снимки отмены и холостую правку не трогает. Под e.mu.
func (e *Editor) stamp() {
	e.p.SetUIDAt(e.seals[e.p.UID])
	e.p.SetShareRefs(e.refs[e.p.UID]) // привязки (167) едут тем же путём
}

// issue выдаёт альбому новый отпечаток с печатью пути path (в проекте и истории, как swapUID) и
// возвращает отмену выдачи — на случай, если запись не удалась. Под e.mu.
func (e *Editor) issue(path string) (uid string, undo func()) {
	old, uid := e.p.UID, model.NewUID()
	e.swapUID(old, uid)
	e.seals[uid] = model.UIDTag(uid, path)
	return uid, func() {
		e.swapUID(uid, old)
		delete(e.seals, uid)
	}
}

// swapUID меняет отпечаток from на to в проекте, снимках отмены и возврата и снимке пересборки.
// Снимки с другим отпечатком (до Restore чужой копии) остаются своими. Длина одинакова — вес
// шагов не меняется. Под e.mu.
func (e *Editor) swapUID(from, to string) {
	swap := func(p *model.Project) {
		if p != nil && p.UID == from {
			p.UID = to
		}
	}
	swap(e.p)
	for i := range e.undo {
		swap(e.undo[i].P)
	}
	for i := range e.redo {
		swap(e.redo[i].P)
	}
	swap(e.auto.base)
}

// sameIdent — альбом всё ещё path с отпечатком uid. Под e.mu.
func (e *Editor) sameIdent(path, uid string) error {
	if path == "" || e.path == "" || !paths.SamePath(e.path, path) || e.p.UID != uid {
		return ErrUIDChanged
	}
	return nil
}
