package album

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Привязки к выгруженным альбомам (167, model/sharerefs.go). Лежат в Editor картой по отпечатку,
// как печати (seals), а не в Project: Clone идёт через JSON и поля вне его теряет, а поле в JSON
// сделало бы привязку правкой (шаг отмены, «изменён»). По отпечатку они сами следуют за отменой
// и Restore: Ctrl+Z после Restore чужой копии возвращает и её привязки. В файл идут через stamp.
// Привязка — не правка: ни шага отмены, ни rev, ни «изменён»; ложится в файл с ближайшей записью
// автосохранением (App перезапускает его таймер).

// ErrBadRef — привязка негодна (не 32 знака hex).
var ErrBadRef = errors.New("привязка к ссылке негодна")

// keepRefs запоминает привязки отпечатка q из файла (NewEditor, Restore); уже известные Editor
// остаются первыми, из файла добавляются недостающие, всего не больше model.MaxShareRefs. Под
// e.mu (или до выдачи Editor).
func (e *Editor) keepRefs(q *model.Project) {
	if !model.GoodUID(q.UID) {
		return
	}
	merged := e.refs[q.UID]
	for _, r := range q.ShareRefs() {
		if len(merged) < model.MaxShareRefs && !slices.Contains(merged, r) {
			merged = append(slices.Clone(merged), r)
		}
	}
	if len(merged) > 0 {
		e.refs[q.UID] = merged
	}
}

// SetRef кладёт привязку ref первой для альбома path с отпечатком uid (после выгрузки, «Обновить»):
// сверка под замком — иначе ErrUIDChanged, альбом сменился. Лишнее сверх model.MaxShareRefs
// отбрасывается. Без шага отмены, rev и «изменён»; запись — ближайшим автосохранением.
func (e *Editor) SetRef(path, uid, ref string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.sameIdent(path, uid); err != nil {
		return err
	}
	if !model.GoodLinkRef(ref) {
		return ErrBadRef
	}
	e.refs[uid] = model.AddLinkRef(e.refs[uid], ref)
	return nil
}

// KeepRefs кладёт привязки refs отпечатка uid без сверки пути: при открытии файла своего пути,
// если привязки записи в файле нет (167-24). Решает вызывающий — чужой путь (копия, шаблон)
// KeepRefs не зовёт. Привязка по отпечатку действует, только пока он текущий. Негодные
// пропускаются; без шага отмены и rev.
func (e *Editor) KeepRefs(uid string, refs ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !model.GoodUID(uid) {
		return
	}
	cur := e.refs[uid]
	for i := len(refs) - 1; i >= 0; i-- { // первая из refs остаётся первой
		cur = model.AddLinkRef(cur, refs[i])
	}
	if len(cur) > 0 {
		e.refs[uid] = cur
	}
}

// ShareRefs — привязки текущего альбома (копия): новая первой.
func (e *Editor) ShareRefs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.refs[e.p.UID])
}

// NewTemplateEditor — редактор нового альбома без файла по проекту p из шаблона («Новый альбом из
// шаблона…», 167-11): новый отпечаток, привязок и печати нет — ссылка шаблона к нему не переходит.
// p после Load/Decode; отпечаток, печать и привязки в нём сбрасываются (p принадлежит вызывающему и
// дальше не используется).
func NewTemplateEditor(p *model.Project) (*Editor, error) {
	p.UID = ""
	p.SetUIDAt("")
	p.SetShareRefs(nil)
	return NewEditor(p, "")
}
