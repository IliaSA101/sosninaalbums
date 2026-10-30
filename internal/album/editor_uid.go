package album

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Отпечаток проекта (146 Т4, решение 0.32-146-T4-9; model/uid.go). Он лежит в Project и потому
// в снимках отмены (Ctrl+Z после Restore чужой копии возвращает и прежний), но правкой не
// является: выдача и смена идут мимо change, без шага отмены, rev и dirty — как SyncOwnNames.

// Печать (model.UIDTag) ставится только при выдаче отпечатка: «Сохранить как» другим проектом
// (SaveNew), сбор (collect.Save), выгрузка альбома без своей печати (SaveUID). Обычная запись
// переносит печать файла как есть: открытие и запись по другому пути (буква USB-диска, облачная
// папка на втором ПК) отпечаток не меняют. Сверка с путём — при выгрузке и показе ссылки
// (ShareIdent, SnapshotAt): печать не этого пути — запись о ссылке пути чужая.

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

// ShareIdent — Ident для записи о ссылке: отпечаток — только при печати этого пути (иначе "",
// и запись пути для альбома чужая: копия шаблона, файл с другого пути).
func (e *Editor) ShareIdent() (path, uid string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.sealed() {
		return e.path, ""
	}
	return e.path, e.p.UID
}

// SnapshotAt — Snapshot вместе с путём файла и сверкой печати под одним замком: снимок для
// выгрузки (shareJob), путь записи о ссылке и её хозяин — от одного альбома, а не от двух соседних.
func (e *Editor) SnapshotAt() (path string, p *model.Project, v model.View, sealed bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path, e.p.Clone(), e.printView(), e.sealed()
}

// sealed — печать отпечатка альбома — его пути: отпечаток выдан или прочитан по этому пути. Нет
// печати (файл ранней сборки 0.32, новый отпечаток), путь в другом написании (8.3, символическая
// ссылка) — false: лишний вопрос о новой ссылке вместо страниц в чужую ссылку. Под e.mu.
func (e *Editor) sealed() bool {
	s := e.seals[e.p.UID]
	return e.path != "" && s != "" && s == model.UIDTag(e.p.UID, e.path)
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
func (e *Editor) stamp() { e.p.SetUIDAt(e.seals[e.p.UID]) }

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

// SaveNew — «Сохранить как» другим проектом (новый альбом, другой путь): новый отпечаток с
// печатью path в проекте и в истории, затем Save. Иначе копия, легшая поверх файла другого
// альбома, прошла бы его сверку. Запись не удалась — отпечаток прежний.
func (e *Editor) SaveNew(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, undo := e.issue(path)
	if err := e.save(path); err != nil {
		undo()
		return err
	}
	return nil
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

// NeedUID — нужно ли записать отпечаток в файл перед выгрузкой: альбом всё ещё path с
// отпечатком uid (иначе ErrUIDChanged), а в файле его нет (альбом до 0.32 не сохранялся с
// открытия, Restore или Ctrl+Z сменили отпечаток) или его печать не этого пути (SaveUID выдаст
// новый).
func (e *Editor) NeedUID(path, uid string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return false, err
	}
	return e.diskUID != uid || !e.sealed(), nil
}

// SaveUID — запись перед выгрузкой со сверкой под тем же замком (NeedUID): печать отпечатка не
// этого пути (копия шаблона, файл с другого пути, новый отпечаток) — новый отпечаток с печатью
// path; отпечаток своей печати, которого нет в файле, — Save. Возвращает отпечаток альбома после
// записи: он идёт в запись о ссылке. Запись не удалась — отпечаток прежний.
func (e *Editor) SaveUID(path, uid string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return "", err
	}
	if e.sealed() {
		if e.diskUID == uid {
			return uid, nil
		}
		return uid, e.save("")
	}
	nu, undo := e.issue(e.path)
	if err := e.save(""); err != nil {
		undo()
		return "", err
	}
	return nu, nil
}

// sameIdent — альбом всё ещё path с отпечатком uid. Под e.mu.
func (e *Editor) sameIdent(path, uid string) error {
	if path == "" || e.path == "" || !paths.SamePath(e.path, path) || e.p.UID != uid {
		return ErrUIDChanged
	}
	return nil
}
