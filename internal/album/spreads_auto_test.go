package album_test

// Склейка и разрез разворота через ручной (103, design §2.2): «Вернуть авто» + склейка/разрез —
// одна правка, один шаг отмены; отказ до «Вернуть авто» не оставляет следов.

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// twoSpreads — два разворота: первый a, b, c (AutoEd), второй d, e; каждый можно закрепить.
func twoSpreads(t *testing.T) (e *album.Editor, s0, s1 string) {
	t.Helper()
	e, s0 = edtest.AutoEd(t)
	v := edtest.NewSpread(e, -1, edtest.Photos("d", "e"))
	return e, s0, v.Spreads[1].ID
}

// manual закрепляет разворот id (места фото, Grid не пуст).
func manual(t *testing.T, e *album.Editor, id string) {
	t.Helper()
	v := e.SetGrid(id, true)
	for _, sv := range v.Spreads {
		if sv.ID == id && !sv.Grid {
			t.Fatalf("%s не закрепился", id)
		}
	}
}

func isGrid(e *album.Editor, id string) bool {
	_, s := edtest.P(e).Find(id)
	return s != nil && len(s.Grid) > 0
}

// TestMergeSpreadsAutoOneStep: ручной + «Авто», «Авто» + ручной, оба ручные — склейка в один
// «Авто»-разворот, один шаг отмены; Undo возвращает места и содержимое, круг Save → Load держит.
func TestMergeSpreadsAutoOneStep(t *testing.T) {
	cases := []struct {
		name   string
		f0, f1 bool
	}{{"ручной+авто", true, false}, {"авто+ручной", false, true}, {"оба ручные", true, true}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { mergeOneStep(t, c.f0, c.f1) })
	}
}

// mergeOneStep — склейка пары, где первый ручной (f0) и/или второй (f1): состав, шаг, Save → Load, Undo.
func mergeOneStep(t *testing.T, f0, f1 bool) {
	t.Helper()
	e, s0, s1 := twoSpreads(t)
	if f0 {
		manual(t, e, s0)
	}
	if f1 {
		manual(t, e, s1)
	}
	before := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, err := e.MergeSpreadsAuto(s0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 1 || len(v.Spreads[0].Photos) != 5 || v.Spreads[0].Grid {
		t.Fatalf("после склейки: %d разворотов, %+v", len(v.Spreads), v.Spreads[0].Photos)
	}
	if edtest.UndoLen(e) != undo+1 || edtest.Rev(e) != rev+1 {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	q := edtest.SaveLoad(t, e)
	if len(q.Spreads) != 1 || len(q.Spreads[0].Photos) != 5 || len(q.Spreads[0].Grid) > 0 {
		t.Fatalf("Save → Load: %d разворотов, %v", len(q.Spreads), q.Spreads[0].Photos)
	}
	e.Undo()
	if after := model.Marshal(edtest.P(e)); !bytes.Equal(before, after) {
		t.Fatalf("Undo вернул другой проект:\n%s\n%s", before, after)
	}
	if isGrid(e, s0) != f0 || isGrid(e, s1) != f1 || edtest.Rev(e) != rev+2 {
		t.Fatalf("Undo: места s0 %v, s1 %v", isGrid(e, s0), isGrid(e, s1))
	}
}

// TestMergeSpreadsAutoNoGrid: оба «Авто» — то же, что MergeSpreads (один шаг, без «Вернуть авто»).
func TestMergeSpreadsAutoNoGrid(t *testing.T) {
	e, s0, _ := twoSpreads(t)
	undo := edtest.UndoLen(e)
	v, err := e.MergeSpreadsAuto(s0, false)
	if err != nil || len(v.Spreads) != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("склейка «Авто»: %v, %d разворотов, шагов %d", err, len(v.Spreads), edtest.UndoLen(e)-undo)
	}
}

// TestMergeSpreadsAutoKeep: с галочкой свой кадр — пропорции окна места (Asp), без неё — снят.
func TestMergeSpreadsAutoKeep(t *testing.T) {
	for _, keep := range []bool{true, false} {
		e, id, _ := marina(t)
		edtest.NewSpread(e, -1, edtest.Photos("d", "e"))
		if aspOf(e, "a") != 0 {
			t.Fatalf("keep %v: до склейки Asp %v", keep, aspOf(e, "a"))
		}
		if _, err := e.MergeSpreadsAuto(id, keep); err != nil {
			t.Fatal(err)
		}
		if got := aspOf(e, "a") != 0; got != keep {
			t.Fatalf("keep %v: Asp %v", keep, aspOf(e, "a"))
		}
		if len(edtest.P(e).Spreads) != 1 {
			t.Fatalf("keep %v: %d разворотов", keep, len(edtest.P(e).Spreads))
		}
		e.Undo()
		if aspOf(e, "a") != 0 || !isGrid(e, id) {
			t.Fatalf("keep %v: Undo не вернул ручной разворот без кадра", keep)
		}
	}
}

// fillTexts — n пустых блоков текста на страницу разворота.
func fillTexts(t *testing.T, e *album.Editor, id string, n int) {
	t.Helper()
	for range n {
		edtest.AddText(t, e, id, 0)
	}
}

// TestMergeSpreadsAutoRefused: виньетка, >64 фото, >64 надписей, последний, неизвестный id —
// холостая до «Вернуть авто»: rev и шаги прежние, места целы.
func TestMergeSpreadsAutoRefused(t *testing.T) {
	big := func(n int) []model.PhotoInfo {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = "p" + strconv.Itoa(i)
		}
		return edtest.Photos(ids...)
	}
	cases := []struct {
		name  string
		setup func(t *testing.T, e *album.Editor) string
	}{
		{"фото сверх предела", func(t *testing.T, e *album.Editor) string {
			a := edtest.NewSpread(e, -1, big(40)).Spreads[0].ID
			edtest.NewSpread(e, -1, edtest.Photos("q0", "q1", "q2", "q3", "q4", "q5", "q6", "q7", "q8", "q9",
				"q10", "q11", "q12", "q13", "q14", "q15", "q16", "q17", "q18", "q19", "q20", "q21", "q22", "q23",
				"q24", "q25", "q26", "q27", "q28", "q29"))
			manual(t, e, a)
			return a
		}},
		{"надписей сверх предела", func(t *testing.T, e *album.Editor) string {
			a := edtest.NewSpread(e, -1, edtest.Photos("a1", "a2")).Spreads[0].ID
			b := edtest.NewSpread(e, -1, edtest.Photos("b1", "b2")).Spreads[1].ID
			fillTexts(t, e, a, model.MaxTexts/2+1)
			fillTexts(t, e, b, model.MaxTexts/2)
			manual(t, e, a)
			return a
		}},
		{"последний", func(t *testing.T, e *album.Editor) string {
			edtest.NewSpread(e, -1, edtest.Photos("a1", "a2"))
			b := edtest.NewSpread(e, -1, edtest.Photos("b1", "b2")).Spreads[1].ID
			manual(t, e, b)
			return b
		}},
		{"виньетка следом", func(t *testing.T, e *album.Editor) string {
			a := edtest.NewSpread(e, -1, edtest.Photos("a1", "a2")).Spreads[0].ID
			if _, err := e.AddVignette(-1); err != nil {
				t.Fatal(err)
			}
			manual(t, e, a)
			return a
		}},
		{"виньетка первой", func(t *testing.T, e *album.Editor) string {
			v, err := e.AddVignette(0)
			if err != nil {
				t.Fatal(err)
			}
			b := edtest.NewSpread(e, -1, edtest.Photos("b1", "b2")).Spreads[1].ID
			manual(t, e, b)
			return v.Spreads[0].ID
		}},
		{"неизвестный id", func(t *testing.T, e *album.Editor) string {
			a := edtest.NewSpread(e, -1, edtest.Photos("a1", "a2")).Spreads[0].ID
			edtest.NewSpread(e, -1, edtest.Photos("b1", "b2"))
			manual(t, e, a)
			return "нет"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := edtest.Open(t, model.New(), "")
			id := c.setup(t, e)
			before := model.Marshal(edtest.P(e))
			rev, undo := edtest.Rev(e), edtest.UndoLen(e)
			v, err := e.MergeSpreadsAuto(id, true)
			if err != nil {
				t.Fatal(err)
			}
			edtest.Same(t, e, rev, undo, c.name)
			if v.Rev != rev || !bytes.Equal(before, model.Marshal(edtest.P(e))) {
				t.Fatalf("%s: проект изменился", c.name)
			}
		})
	}
}

// TestSplitSpreadAutoIndex: ручной разворот режется по at; at вне 1..len−1 и виньетка — холостая;
// Undo одним шагом возвращает ручной разворот; Save → Load держит оба «Авто»-разворота.
func TestSplitSpreadAutoIndex(t *testing.T) {
	e, id := edtest.AutoEd(t)
	manual(t, e, id)
	before := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, at := range []int{-1, 0, 3, 4, 1 << 30} {
		if _, err := e.SplitSpreadAuto(id, at, true); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "at "+strconv.Itoa(at))
	}
	if _, err := e.SplitSpreadAuto("нет", 1, true); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "неизвестный id")

	v, err := e.SplitSpreadAuto(id, 1, true)
	if err != nil || len(v.Spreads) != 2 {
		t.Fatalf("разрез: %v, %d разворотов", err, len(v.Spreads))
	}
	if len(v.Spreads[0].Photos) != 1 || len(v.Spreads[1].Photos) != 2 || v.Spreads[0].Grid || v.Spreads[1].Grid ||
		edtest.UndoLen(e) != undo+1 {
		t.Fatalf("разрез: %v | %v, шагов %d", v.Spreads[0].Photos, v.Spreads[1].Photos, edtest.UndoLen(e)-undo)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	if q := edtest.SaveLoad(t, e); len(q.Spreads) != 2 || len(q.Spreads[0].Grid)+len(q.Spreads[1].Grid) != 0 {
		t.Fatalf("Save → Load: %d разворотов", len(q.Spreads))
	}
	e.Undo()
	if !bytes.Equal(before, model.Marshal(edtest.P(e))) || !isGrid(e, id) {
		t.Fatal("Undo не вернул ручной разворот")
	}

	// «Авто» режется как прежним SplitSpread: без «Вернуть авто», одним шагом.
	edtest.Change(e, func(p *model.Project) { p.Unfreeze(p.Spreads[0]) })
	undo = edtest.UndoLen(e)
	if v, err = e.SplitSpreadAuto(id, 2, false); err != nil || len(v.Spreads) != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("разрез «Авто»: %v, %d разворотов, шагов %d", err, len(v.Spreads), edtest.UndoLen(e)-undo)
	}

	// Виньетка не режется.
	vv, err := e.AddVignette(-1)
	if err != nil {
		t.Fatal(err)
	}
	vid := vv.Spreads[len(vv.Spreads)-1].ID
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SplitSpreadAuto(vid, 1, true); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "виньетка")
}

// TestSplitSpreadAutoLimit: на 200 разворотах разрез ручного — ошибка предела, откат всего:
// места целы, шага нет; на 199 — проходит.
func TestSplitSpreadAutoLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxSpreads - 1 {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#ffffff"})
		}
	})
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	id := v.Spreads[len(v.Spreads)-1].ID
	manual(t, e, id)
	if len(edtest.P(e).Spreads) != model.MaxSpreads {
		t.Fatalf("заготовка: %d разворотов", len(edtest.P(e).Spreads))
	}
	before := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SplitSpreadAuto(id, 1, true); err == nil {
		t.Fatal("разрез сверх MaxSpreads прошёл")
	}
	edtest.Same(t, e, rev, undo, "разрез сверх MaxSpreads")
	if !isGrid(e, id) || !bytes.Equal(before, model.Marshal(edtest.P(e))) {
		t.Fatal("откат оставил следы «Вернуть авто»")
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads = p.Spreads[1:] })
	if v, err := e.SplitSpreadAuto(id, 1, true); err != nil || len(v.Spreads) != model.MaxSpreads {
		t.Fatalf("разрез до предела: %v, %d разворотов", err, len(v.Spreads))
	}
}
