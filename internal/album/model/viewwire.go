package model

import (
	"encoding/json"
	"reflect"
)

// Дельта вида (075): по мосту Wails идёт не весь вид (1,2 МБ на 200 разворотах), а шапка и
// изменённые развороты. Номера цепочки ведёт App (internal/app/wire.go), здесь — форма и
// сравнение. Контракт с фронтом — docs/specs/0.19/075-polish/design.md §1.3, сверка —
// testdata/parity/viewdelta.json.

// MarshalJSON — полный вид (Base == 0) как есть; дельта — spreads только изменённые, order
// всегда списком: у альбома без разворотов поле с omitempty пропало бы, а фронт его ждёт.
func (v View) MarshalJSON() ([]byte, error) {
	type plain View // без метода: иначе рекурсия
	if v.Base == 0 {
		return json.Marshal(plain(v))
	}
	q := plain(v)
	q.Spreads = make([]SpreadView, 0, len(v.fresh))
	for i, f := range v.fresh {
		if f && i < len(v.Spreads) {
			q.Spreads = append(q.Spreads, v.Spreads[i])
		}
	}
	order := v.Order
	if order == nil {
		order = []string{}
	}
	return json.Marshal(struct {
		plain
		Order []string `json:"order"` // мельче вложенного: перекрывает его omitempty
	}{q, order})
}

// Wire — дельта cur к prev (вид с номером base), сама с номером seq: в spreads пойдут
// развороты, которых нет в prev или чей готовый вид изменился. Сравниваются виды, а не модель
// (075-2): вид разворота зависит и от настроек, и от разметки, и от плана виньеток — перечень
// «влияющего» легко забыть пополнить. prev == nil — все развороты новые. same — шапка, порядок
// и все развороты как у prev: правки, видимой интерфейсу, нет.
func Wire(prev *View, cur View, base, seq int) (w View, same bool) {
	w = cur
	w.Seq, w.Base = seq, base
	w.Order = make([]string, len(cur.Spreads))
	w.fresh = make([]bool, len(cur.Spreads))
	old := map[string]int{}
	if prev != nil {
		for i := range prev.Spreads {
			old[prev.Spreads[i].ID] = i
		}
	}
	same = prev != nil && len(prev.Spreads) == len(cur.Spreads)
	for i := range cur.Spreads {
		id := cur.Spreads[i].ID
		w.Order[i] = id
		j, ok := old[id]
		w.fresh[i] = !ok || !reflect.DeepEqual(prev.Spreads[j], cur.Spreads[i])
		same = same && !w.fresh[i] && j == i
	}
	return w, same && sameHead(*prev, cur)
}

// SameView — виды равны для интерфейса: шапка и развороты (номера цепочки не в счёт).
func SameView(a, b View) bool {
	_, same := Wire(&a, b, 0, 0)
	return same
}

// Unchanged — пустая дельта к v с его же номером seq (холостая правка, обогнанный вид): фронт,
// у которого уже вид seq, её пропускает.
func Unchanged(v View, seq int) View {
	w, _ := Wire(nil, v, seq, seq)
	w.fresh = nil
	return w
}

// Fresh — сколько разворотов дельта несёт в spreads (у полного вида — 0).
func (v View) Fresh() int {
	n := 0
	for _, f := range v.fresh {
		if f {
			n++
		}
	}
	return n
}

// sameHead — всё, кроме разворотов и номеров цепочки, совпадает.
func sameHead(a, b View) bool {
	a.Spreads, a.Seq, a.Base, a.Order, a.fresh = nil, 0, 0, nil, nil
	b.Spreads, b.Seq, b.Base, b.Order, b.fresh = nil, 0, 0, nil, nil
	return reflect.DeepEqual(a, b)
}
