package app

import (
	"encoding/json"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Копировать и вставить (спека 099, design §1, §6): буфер — снимок модели в App, интерфейс видит
// только сводку ClipInfo (числа и ID листа, без путей). Методы здесь, а не в app/edit: тип
// результата — пакета app и есть состояние (edit-methods.md).

// clipBuf — буфер обмена альбома. Снимок привязан к Editor, при котором снят: после смены альбома
// (retire) указатель чужой, и буфер считается пустым — пути альбома A в альбом B не попадают.
type clipBuf struct {
	mu sync.Mutex
	ed *album.Editor
	c  *model.Clip
}

// set кладёт снимок, снятый в ed.
func (b *clipBuf) set(ed *album.Editor, c *model.Clip) {
	b.mu.Lock()
	b.ed, b.c = ed, c
	b.mu.Unlock()
}

// get — снимок для ed; буфер другого Editor сбрасывается (память не держит чужой альбом).
// Снимок после set не меняется, поэтому вставка читает его без замка.
func (b *clipBuf) get(ed *album.Editor) *model.Clip {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ed != ed {
		b.ed, b.c = nil, nil
		return nil
	}
	return b.c
}

// ClipResult — вид после «Вырезать» и сводка буфера.
type ClipResult struct {
	View model.View     `json:"view"`
	Clip model.ClipInfo `json:"clip"`
}

// PasteResult — итог вставки и дубля: что выделить (Refs, Texts, Stickers), Repeat — фото, уже
// стоящие в альбоме (правки нет, UI спрашивает и зовёт ещё раз), Froze — «Авто» стал «Ручной».
type PasteResult struct {
	View     model.View     `json:"view"`
	Refs     []string       `json:"refs"`
	Texts    []string       `json:"texts"`
	Stickers []string       `json:"stickers"`
	Repeat   []string       `json:"repeat"`
	Froze    bool           `json:"froze"`
	Clip     model.ClipInfo `json:"clip"`
}

// MarshalJSON — срезы всегда списки: UI не проверяет на null.
func (r PasteResult) MarshalJSON() ([]byte, error) {
	type plain PasteResult // без метода: иначе рекурсия
	q := plain(r)
	q.Refs, q.Texts, q.Stickers, q.Repeat = orEmpty(q.Refs), orEmpty(q.Texts), orEmpty(q.Stickers), orEmpty(q.Repeat)
	return json.Marshal(q)
}

// clipInfo — сводка буфера для ed (пустая у чужого Editor).
func (a *App) clipInfo(ed *album.Editor) model.ClipInfo { return a.clip.get(ed).Info() }

// ClipState — сводка буфера после смены альбома и при старте. Только чтение.
func (a *App) ClipState() model.ClipInfo {
	defer guard("ClipState")
	return a.clipInfo(a.editor())
}

// CopyItems — выделенное листа sheet в буфер (Ctrl+C). Нечего копировать — буфер прежний.
// Проект не меняется: ни шага, ни события.
func (a *App) CopyItems(sheet string, sel model.ClipSel) model.ClipInfo {
	defer guard("CopyItems")
	ed := a.editor()
	if c, ok := ed.TakeClip(sheet, sel); ok {
		a.clip.set(ed, &c)
	}
	return a.clipInfo(ed)
}

// CutItems — снимок в буфер и удаление выделенного одним шагом отмены (Ctrl+X). Отмена после
// вырезания возвращает элементы, буфер остаётся (как системный). Нечего — холостая, буфер прежний.
func (a *App) CutItems(sheet string, sel model.ClipSel) (r ClipResult, err error) {
	defer a.guardEdit("CutItems", &r.View, &err)
	ed := a.editor()
	v, c, ok, err := ed.CutClip(sheet, sel)
	if r.View, err = a.changedOrErr(v, err); err != nil {
		r.Clip = a.clipInfo(ed)
		return r, err
	}
	if ok {
		a.clip.set(ed, &c)
	}
	r.Clip = a.clipInfo(ed)
	return r, nil
}

// PasteItems — вставка буфера (Ctrl+V, «Вставить» меню). Пустой буфер и буфер чужого альбома —
// холостая. Repeat вопроса 098 без правки: вид прежний, события и автосохранения нет.
func (a *App) PasteItems(r model.PasteReq) (res PasteResult, err error) {
	defer a.guardEdit("PasteItems", &res.View, &err)
	ed := a.editor()
	v, out, err := ed.PasteClip(a.clip.get(ed), r)
	return a.pasteResult(ed, v, out, err)
}

// DuplicateItems — дубль выделенного на том же листе со сдвигом одним шагом (Ctrl+D). Буфер не
// трогается; фото — копии сразу, без вопроса (0.23-q2), поэтому параметра repeat нет.
func (a *App) DuplicateItems(sheet string, sel model.ClipSel) (res PasteResult, err error) {
	defer a.guardEdit("DuplicateItems", &res.View, &err)
	ed := a.editor()
	v, out, err := ed.DuplicateClip(sheet, sel)
	return a.pasteResult(ed, v, out, err)
}

// pasteResult собирает ответ вставки: правка идёт в цепочку видов, вопрос (out.Repeat) — нет.
func (a *App) pasteResult(ed *album.Editor, v model.View, out model.PasteOut, err error) (PasteResult, error) {
	r := PasteResult{Clip: a.clipInfo(ed)}
	if err == nil && len(out.Repeat) > 0 {
		r.View, r.Repeat = a.out(v), out.Repeat
		return r, nil
	}
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Refs, r.Texts, r.Stickers, r.Froze = out.Refs, out.Texts, out.Stickers, out.Froze
	return r, nil
}
