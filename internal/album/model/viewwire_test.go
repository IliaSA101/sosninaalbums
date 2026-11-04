package model

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "перезаписать testdata/parity/viewdelta.json")

// wv — маленький вид: развороты с ID и фото, шапка с rev.
func wv(rev int, spreads ...SpreadView) View {
	return View{Product: ProductAlbum, Rev: rev, Spreads: spreads, Folders: []string{}, FillFrom: []int{}}
}

func sv(id string, photos ...string) SpreadView {
	return SpreadView{ID: id, Photos: photos, W: 600, H: 300, Mid: 300}
}

// decode — JSON вида в карту (как его видит фронт).
func decode(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// merge — то же, что фронт (base/core/viewdelta.ts mergeView): шапка дельты, развороты по order —
// из дельты, иначе из prev; разворота нет нигде — nil (фронт просит GetView).
func merge(prev, d map[string]any) map[string]any {
	old := map[string]any{}
	for _, s := range anyList(prev["spreads"]) {
		old[s.(map[string]any)["id"].(string)] = s
	}
	fresh := map[string]any{}
	for _, s := range anyList(d["spreads"]) {
		fresh[s.(map[string]any)["id"].(string)] = s
	}
	out := map[string]any{}
	for k, v := range d {
		if k != "base" && k != "order" {
			out[k] = v
		}
	}
	spreads := []any{}
	for _, id := range anyList(d["order"]) {
		s, ok := fresh[id.(string)]
		if !ok {
			s, ok = old[id.(string)]
		}
		if !ok {
			return nil
		}
		spreads = append(spreads, s)
	}
	out["spreads"] = spreads
	return out
}

func anyList(v any) []any {
	l, _ := v.([]any)
	return l
}

// full — полный вид cur с номером seq, как его ждёт фронт после слияния.
func full(t *testing.T, cur View, seq int) map[string]any {
	t.Helper()
	cur.Seq = seq
	m := decode(t, cur)
	if m["spreads"] == nil {
		m["spreads"] = []any{}
	}
	return m
}

// TestWireJSONShape — форма JSON: полный вид, дельта, пустая дельта, альбом без разворотов.
func TestWireJSONShape(t *testing.T) {
	prev := wv(1, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"))
	prev.Seq = 7
	m := decode(t, prev)
	if m["seq"] != 7.0 || m["base"] != nil || m["order"] != nil || len(anyList(m["spreads"])) != 3 {
		t.Fatalf("полный вид: seq %v base %v order %v spreads %d", m["seq"], m["base"], m["order"], len(anyList(m["spreads"])))
	}

	cur := wv(2, sv("a", "p1"), sv("b", "p2", "p4"), sv("c", "p3"))
	w, same := Wire(&prev, cur, 7, 8)
	if same || w.Fresh() != 1 || len(w.Spreads) != 3 {
		t.Fatalf("дельта: same %v, изменено %d, в Go разворотов %d (ждали все 3)", same, w.Fresh(), len(w.Spreads))
	}
	m = decode(t, w)
	got := anyList(m["spreads"])
	if m["seq"] != 8.0 || m["base"] != 7.0 || len(got) != 1 || got[0].(map[string]any)["id"] != "b" {
		t.Fatalf("дельта: %v", m)
	}
	if !reflect.DeepEqual(m["order"], []any{"a", "b", "c"}) {
		t.Fatalf("order %v", m["order"])
	}

	e := Unchanged(cur, 8)
	m = decode(t, e)
	if m["seq"] != 8.0 || m["base"] != 8.0 || len(anyList(m["spreads"])) != 0 || m["spreads"] == nil || len(anyList(m["order"])) != 3 {
		t.Fatalf("пустая дельта: %v", m)
	}

	// Все развороты удалены: order и spreads — пустые списки, а не пропуск поля и не null.
	w, _ = Wire(&prev, wv(3), 8, 9)
	m = decode(t, w)
	if o, ok := m["order"].([]any); !ok || len(o) != 0 {
		t.Fatalf("order пустого альбома: %#v", m["order"])
	}
	if s, ok := m["spreads"].([]any); !ok || len(s) != 0 {
		t.Fatalf("spreads пустого альбома: %#v", m["spreads"])
	}
	// Дельта, собранная не через Wire (без Order и fresh), — тоже списки, а не null.
	m = decode(t, View{Seq: 3, Base: 2, Spreads: []SpreadView{sv("a")}})
	if o, ok := m["order"].([]any); !ok || len(o) != 0 || len(anyList(m["spreads"])) != 0 {
		t.Fatalf("дельта без Wire: order %#v, spreads %#v", m["order"], m["spreads"])
	}
}

// TestWireFresh — что идёт в дельту: по развороту на каждый случай сразу, порядок, шапка.
func TestWireFresh(t *testing.T) {
	prev := wv(1, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"), sv("d", "p4"))
	freshIDs := func(w View) []string {
		var out []string
		for i := range w.Spreads {
			if w.fresh[i] {
				out = append(out, w.Spreads[i].ID)
			}
		}
		return out
	}
	cases := []struct {
		name  string
		cur   View
		fresh []string
		same  bool
	}{
		{"равный", wv(1, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"), sv("d", "p4")), nil, true},
		{"изменён, удалён, добавлен", wv(2, sv("a", "p1", "p9"), sv("c", "p3"), sv("d", "p4"), sv("e")), []string{"a", "e"}, false},
		{"только порядок", wv(2, sv("b", "p2"), sv("a", "p1"), sv("c", "p3"), sv("d", "p4")), nil, false},
		{"только шапка", wv(2, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"), sv("d", "p4")), nil, false},
		{"все изменены", wv(2, sv("a"), sv("b"), sv("c"), sv("d")), []string{"a", "b", "c", "d"}, false},
	}
	for _, c := range cases {
		w, same := Wire(&prev, c.cur, 1, 2)
		if same != c.same || !slices.Equal(freshIDs(w), c.fresh) || w.Fresh() != len(c.fresh) {
			t.Errorf("%s: same %v, изменены %v; ждали %v, %v", c.name, same, freshIDs(w), c.same, c.fresh)
		}
		if same != SameView(prev, c.cur) {
			t.Errorf("%s: SameView расходится с Wire", c.name)
		}
		if got := merge(full(t, prev, 1), decode(t, w)); !reflect.DeepEqual(got, full(t, c.cur, 2)) {
			t.Errorf("%s: слияние не дало полный вид", c.name)
		}
	}
	// Без прежнего вида — все развороты новые; пустой вид — ни одного.
	if w, same := Wire(nil, prev, 1, 2); same || w.Fresh() != 4 {
		t.Errorf("без prev: same %v, изменено %d", same, w.Fresh())
	}
	if w, same := Wire(nil, View{}, 1, 2); same || w.Fresh() != 0 || len(w.Order) != 0 {
		t.Errorf("пустой без prev: same %v, изменено %d, order %v", same, w.Fresh(), w.Order)
	}
	// Номера цепочки на равенство не влияют.
	a, b := prev, prev
	a.Seq, b.Seq, b.Base, b.Order = 3, 9, 4, []string{"x"}
	if !SameView(a, b) {
		t.Error("SameView учёл номера цепочки")
	}
}

// deltaCase — вектор сверки Go ↔ TS (frontend base/core/viewdelta.ts): prev — вид фронта,
// delta — что прислал Go, want — полный вид после слияния (seq — дельты, без base и order);
// null — слить нельзя (разворота из order нет ни в дельте, ни в prev): фронт просит GetView.
type deltaCase struct {
	Name  string          `json:"name"`
	Prev  json.RawMessage `json:"prev"`
	Delta json.RawMessage `json:"delta"`
	Want  json.RawMessage `json:"want"`
}

func deltaCases(t *testing.T) []deltaCase {
	raw := func(v any) json.RawMessage {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	prev := wv(4, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"))
	prev.Seq = 10
	mk := func(name string, cur View) deltaCase {
		w, _ := Wire(&prev, cur, prev.Seq, prev.Seq+1)
		return deltaCase{Name: name, Prev: raw(prev), Delta: raw(w), Want: raw(full(t, cur, w.Seq))}
	}
	cover := wv(5, sv("a", "p1"), sv("b", "p2"), sv("c", "p3"))
	cover.Cover = &CoverView{Sheet: sv("cover", "p7"), N: 3}
	fonts := wv(5, sv("a", "p1"), sv("b", "p2"), sv("c", "p3")) // шапка с albumFonts (082)
	fonts.AlbumFonts = []FontUse{{ID: "my:0123456789abcdef", Name: "Lato"}, {ID: "my:fedcba9876543210"}}
	fill := wv(5, sv("a", "p1"), sv("b", "p2"), sv("c", "p3")) // шапка с числами «Заполнить» (132): развороты те же
	fill.FillFrom, fill.CoverFill = []int{3, 2, 1}, 4
	other := wv(4, sv("x", "p1"))
	other.Seq = 10
	foreign, _ := Wire(&other, wv(5, sv("x", "p1"), sv("y")), 10, 11)
	return []deltaCase{
		mk("один изменён", wv(5, sv("a", "p1"), sv("b", "p2", "p8"), sv("c", "p3"))),
		mk("удалён", wv(5, sv("a", "p1"), sv("c", "p3"))),
		mk("добавлен в середину", wv(5, sv("a", "p1"), sv("n"), sv("b", "p2"), sv("c", "p3"))),
		mk("порядок", wv(5, sv("c", "p3"), sv("a", "p1"), sv("b", "p2"))),
		mk("только шапка (обложка)", cover),
		mk("только шапка (свои шрифты)", fonts),
		mk("только шапка (числа заполнения)", fill),
		mk("все удалены", wv(5)),
		{Name: "пустая дельта", Prev: raw(prev), Delta: raw(Unchanged(prev, prev.Seq)), Want: raw(full(t, prev, prev.Seq))},
		{Name: "дельта к чужому виду", Prev: raw(prev), Delta: raw(foreign), Want: json.RawMessage("null")},
	}
}

// TestViewDeltaParity — векторы testdata/parity/viewdelta.json: Go-слияние (то же, что у фронта)
// даёт want; файл совпадает с тем, что пишет Go (перезапись: go test ./internal/album/model -run
// ViewDeltaParity -update).
func TestViewDeltaParity(t *testing.T) {
	cases := deltaCases(t)
	for _, c := range cases {
		var prev, d, want map[string]any
		for _, x := range []struct {
			raw json.RawMessage
			to  *map[string]any
		}{{c.Prev, &prev}, {c.Delta, &d}, {c.Want, &want}} {
			if err := json.Unmarshal(x.raw, x.to); err != nil {
				t.Fatal(err)
			}
		}
		if got := merge(prev, d); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: слияние %v, ждали %v", c.Name, got, want)
		}
	}
	data, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	path := filepath.Join("..", "..", "..", "testdata", "parity", "viewdelta.json")
	if *update {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n")), data) {
		t.Fatal("testdata/parity/viewdelta.json устарел: go test ./internal/album/model -run ViewDeltaParity -update")
	}
}
