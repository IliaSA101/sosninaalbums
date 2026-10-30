package app

import (
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Цепочка видов (075, design §1.2): каждый вид, отданный интерфейсу ответом или событием,
// получает номер seq; после правки уходит дельта к последнему отданному — шапка и изменённые
// развороты. Фронт применяет дельту только к виду с seq == base, иначе просит GetView.

// wireState — последний отданный вид; поля под a.mu.
type wireState struct {
	sent   model.View
	seq    int                        // счётчик на весь процесс, только растёт
	sentEd weak.Pointer[album.Editor] // чей вид в sent: другой Editor — полный вид
}

// outLocked — вид правки в цепочку; emit — слать ли событие view. Под a.mu.
func (a *App) outLocked(v model.View) (w model.View, emit bool) {
	ws := &a.wire
	ed := a.editor()
	if foreign(v, ed) {
		// Поздний вид прежнего альбома (правка началась до открытия, 087-3): его rev с нынешним
		// не сравним — в цепочку и событием не идёт, вызывающему — вид нынешнего.
		return a.fullLocked(ed.View()), false
	}
	switch {
	case ws.sentEd.Value() != ed:
		// Первый вид или другой альбом: дельте не к чему относиться — полный.
		ws.seq++
		ws.sent, ws.sentEd = v, weak.Make(ed)
		v.Seq = ws.seq
		return v, true
	case v.Rev < ws.sent.Rev:
		// Правку обогнала более новая (параллельные вызовы Wails): старый вид не перекрывает новый.
		return model.Unchanged(ws.sent, ws.seq), false
	}
	w, same := model.Wire(&ws.sent, v, ws.seq, ws.seq+1)
	if same {
		return model.Unchanged(ws.sent, ws.seq), false // холостая правка: событие не нужно
	}
	ws.seq++
	ws.sent = v
	return w, true
}

// foreign — вид построен не Editor ed (прежним альбомом). Вид без номера считается своим.
func foreign(v model.View, ed *album.Editor) bool {
	return v.Src() != 0 && v.Src() != ed.ID()
}

// fullLocked — полный вид в цепочку (открытие, GetState, GetView). Равный отданному номер не
// сдвигает: иначе два клиента wails dev, просящие GetView, гоняли бы друг друга. Под a.mu.
func (a *App) fullLocked(v model.View) model.View {
	ws := &a.wire
	ed := a.editor()
	if ws.sentEd.Value() == ed && ws.seq > 0 {
		if v.Rev < ws.sent.Rev {
			v = ws.sent // вид старше отданного: отдаём новый
		}
		if model.SameView(ws.sent, v) {
			v.Seq = ws.seq
			return v
		}
	}
	ws.seq++
	ws.sent, ws.sentEd = v, weak.Make(ed)
	v.Seq = ws.seq
	return v
}

// out — вид без правки (сохранение, холостой ответ) в цепочку; событие — если вид сменился
// (Ctrl+S снял «*»: другие клиенты тоже должны это увидеть). Полный вид первым в цепочке
// не рассылается: новый альбом всем шлёт emitState, а здесь правки нет.
func (a *App) out(v model.View) model.View {
	a.mu.Lock()
	defer a.mu.Unlock()
	w, emit := a.outLocked(v)
	if emit && w.Base > 0 {
		a.emit("view", w)
	}
	return w
}

// GetView — полный вид для интерфейса: фронт просит его, когда дельта пришла не к его виду
// (второй клиент wails dev, гонка). Дешевле GetState: без настроек, «Последних» и ленты.
func (a *App) GetView() model.View {
	select {
	case <-a.ready:
	case <-a.stopCtx.Done():
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.fullLocked(a.editor().View())
}
