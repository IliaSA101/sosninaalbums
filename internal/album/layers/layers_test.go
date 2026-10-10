package layers_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// textIDsOf — ID текстов первого разворота снизу вверх.
func textIDsOf(e *album.Editor) []string {
	var out []string
	for _, t := range edtest.P(e).Spreads[0].Texts {
		out = append(out, t.ID)
	}
	return out
}

func TestArrangeLayersUpOverlap(t *testing.T) {
	// A и B перекрываются, C далеко, D перекрывает B, но не A.
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(60, 60, 100, 100, "b"),
		edtest.Gp(300, 10, 50, 50, "c"), edtest.Gp(120, 120, 100, 100, "d"),
	})
	g := edtest.PlaceIDsOf(e)
	a, b, c, d := g[0], g[1], g[2], g[3]
	e.ArrangeLayers("s1", "up", []string{a})
	edtest.MustGrid(t, e)
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{b, a, c, d}) {
		t.Fatalf("up A: %v", got)
	}
	// Выше A есть только C и D, с ними нет перекрытия: стоит.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ArrangeLayers("s1", "up", []string{a})
	edtest.Same(t, e, rev, undo, "up без перекрытия выше")
	// down зеркально: D встаёт под B (ближайший перекрывающийся ниже).
	e.ArrangeLayers("s1", "down", []string{d})
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{d, b, a, c}) {
		t.Fatalf("down D: %v", got)
	}
	// front / back сохраняют взаимный порядок выделенных.
	e.ArrangeLayers("s1", "front", []string{a, b})
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{d, c, b, a}) {
		t.Fatalf("front: %v", got)
	}
	e.ArrangeLayers("s1", "back", []string{a, b})
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{b, a, d, c}) {
		t.Fatalf("back: %v", got)
	}
	// Лента = порядок мест.
	edtest.MustGrid(t, e)
	e.Undo()
	edtest.MustGrid(t, e)
}

func TestArrangeLayersNoOverlapNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 50, "a"), edtest.Gp(100, 10, 50, 50, "b"), edtest.Gp(200, 10, 50, 50, "c")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	g := edtest.PlaceIDsOf(e)
	e.ArrangeLayers("s1", "up", []string{g[0]})
	e.ArrangeLayers("s1", "down", []string{g[2]})
	e.ArrangeLayers("s1", "up", []string{g[0], g[1]})
	e.ArrangeLayers("s1", "nope", []string{g[0]})
	e.ArrangeLayers("s1", "up", []string{"нет такого"})
	e.ArrangeLayers("нет", "up", []string{g[0]})
	e.ArrangeLayers("s1", "front", []string{g[2]}) // уже наверху
	edtest.Same(t, e, rev, undo, "без перекрытий")
	// Касание краем — не перекрытие.
	e2 := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 50, "a"), edtest.Gp(60, 10, 50, 50, "b")})
	rev, undo = edtest.Rev(e2), edtest.UndoLen(e2)
	e2.ArrangeLayers("s1", "up", []string{edtest.PlaceIDsOf(e2)[0]})
	edtest.Same(t, e2, rev, undo, "касание")
}

func TestArrangeLayersTexts(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 50, 50, "a")})
	t1, t2, t3 := edtest.AddText(t, e, "s1", 0), edtest.AddText(t, e, "s1", 0), edtest.AddText(t, e, "s1", 1)
	// Первые два на одном месте, третий на другой странице — с ними не пересекается.
	e.ArrangeLayers("s1", "up", []string{t1})
	if got := textIDsOf(e); !slices.Equal(got, []string{t2, t1, t3}) {
		t.Fatalf("up t1: %v", got)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ArrangeLayers("s1", "up", []string{t1})
	e.ArrangeLayers("s1", "up", []string{t3})
	edtest.Same(t, e, rev, undo, "нет перекрытия выше")
	e.ArrangeLayers("s1", "back", []string{t3})
	if got := textIDsOf(e); !slices.Equal(got, []string{t3, t2, t1}) {
		t.Fatalf("back t3: %v", got)
	}
	e.ArrangeLayers("s1", "down", []string{t1})
	if got := textIDsOf(e); !slices.Equal(got, []string{t3, t1, t2}) {
		t.Fatalf("down t1: %v", got)
	}
	if len(edtest.P(e).Spreads[0].Grid) != 1 {
		t.Fatal("места изменились")
	}
}

func TestArrangeLayersAutoNoop(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, op := range []string{"front", "up", "down", "back"} {
		e.ArrangeLayers(id, op, []string{"a", "b"})
	}
	edtest.Same(t, e, rev, undo, "места «Авто»")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("правка закрепила «Авто»")
	}
	// Тексты у «Авто» работают, разворот остаётся «Авто».
	t1, t2 := edtest.AddText(t, e, id, 0), edtest.AddText(t, e, id, 0)
	e.ArrangeLayers(id, "up", []string{t1})
	if got := textIDsOf(e); !slices.Equal(got, []string{t2, t1}) {
		t.Fatalf("тексты: %v", got)
	}
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("тексты закрепили «Авто»")
	}
}

func TestArrangeLayersMixed(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(60, 60, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	t1, t2 := edtest.AddText(t, e, "s1", 0), edtest.AddText(t, e, "s1", 0)
	// Места и тексты в одном выделении — каждое в своём стеке, мусор пропускается.
	e.ArrangeLayers("s1", "back", []string{t2, g[1], "нет такого"})
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{g[1], g[0]}) {
		t.Fatalf("места: %v", got)
	}
	if got := textIDsOf(e); !slices.Equal(got, []string{t2, t1}) {
		t.Fatalf("тексты: %v", got)
	}
	// ref фото работает как ID места.
	e.ArrangeLayers("s1", "front", []string{"b"})
	if got := edtest.PlaceIDsOf(e); !slices.Equal(got, []string{g[0], g[1]}) {
		t.Fatalf("ref фото: %v", got)
	}
	// Выделенная пара: каждое встаёт над своим ближайшим невыделенным.
	e2 := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(20, 20, 100, 100, "b"), edtest.Gp(30, 30, 100, 100, "c"), edtest.Gp(40, 40, 100, 100, "d")})
	h := edtest.PlaceIDsOf(e2)
	e2.ArrangeLayers("s1", "up", []string{h[0], h[1]})
	if got := edtest.PlaceIDsOf(e2); !slices.Equal(got, []string{h[2], h[0], h[1], h[3]}) {
		t.Fatalf("up пары: %v", got)
	}
	edtest.MustGrid(t, e2)
}

// Общий порядок слоёв (спека 010, design §4).

func TestOrderDefaultNil(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(60, 60, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	t1, t2 := edtest.TextAt(t, e, "s1", 50, 50), edtest.TextAt(t, e, "s1", 55, 55)
	s := edtest.P(e).Spreads[0]
	if s.Order != nil {
		t.Fatalf("новые тексты: Order %v", s.Order)
	}
	// Перестановка внутри текстов и внутри мест — порядок по умолчанию.
	e.ArrangeLayers("s1", "down", []string{t2})
	e.ArrangeLayers("s1", "up", []string{"a"})
	edtest.MustGrid(t, e)
	if s := edtest.P(e).Spreads[0]; s.Order != nil || !slices.Equal(edtest.StackOf(e), []string{g[1], g[0], t2, t1}) {
		t.Fatalf("Order %v, стек %v", s.Order, edtest.StackOf(e))
	}
	if bytes.Contains(model.Marshal(edtest.P(e)), []byte(`"order"`)) {
		t.Fatalf("поле order без «текста под фото»:\n%s", model.Marshal(edtest.P(e)))
	}
}

func TestOrderSyncAddDelete(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(150, 10, 100, 100, "b")})
	g := edtest.PlaceIDsOf(e)
	a, b := g[0], g[1]
	t1 := edtest.TextAt(t, e, "s1", 20, 20)
	e.ArrangeLayers("s1", "back", []string{t1})
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, a, b}) {
		t.Fatalf("back текста: %v", got)
	}
	// Новое место — над предшествующим ему в Grid (AddPlace — в конец).
	if _, err := e.AddPlace("s1", 1); err != nil {
		t.Fatal(err)
	}
	c := edtest.PlaceIDsOf(e)[2]
	// Деление: половина встаёт сразу над делимым.
	if _, err := e.ArrangePlaces("s1", "splitH", []string{a}); err != nil {
		t.Fatal(err)
	}
	a2 := edtest.PlaceIDsOf(e)[1]
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, a, a2, b, c}) {
		t.Fatalf("после AddPlace и деления: %v", got)
	}
	e.DeletePlaces("s1", []string{b})
	t2 := edtest.AddText(t, e, "s1", 1) // новый текст — наверх
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, a, a2, c, t2}) {
		t.Fatalf("после удаления места и нового текста: %v", got)
	}
	edtest.MustGrid(t, e)
	// Ушёл текст под фото — порядок по умолчанию, Order = nil.
	e.DeleteText(t1)
	if got := edtest.P(e).Spreads[0].Order; got != nil {
		t.Fatalf("после удаления текста: %v", got)
	}
	edtest.MustGrid(t, e)

	// Починка напрямую: чужие и повторы — вон; первое место без Order — под первым
	// местом стека, следующее — сразу над предшествующим в Grid, текст — наверх.
	pl := func(id string) *model.Place { return &model.Place{ID: id} }
	s := &model.Spread{Grid: []*model.Place{pl("p1"), pl("p2"), pl("p3"), pl("p4")},
		Texts: []*model.TextBlock{{ID: "t1"}, {ID: "t2"}},
		Order: []string{"p2", "t1", "нет", "p4", "p2"}}
	if got, want := s.Stack(), []string{"p1", "p2", "p3", "t1", "p4", "t2"}; !slices.Equal(got, want) {
		t.Fatalf("stack %v, ждали %v", got, want)
	}
	// Мест в Order нет — недостающие места вниз.
	s.Order = []string{"t2", "t1"}
	if got, want := s.Stack(), []string{"p1", "p2", "p3", "p4", "t2", "t1"}; !slices.Equal(got, want) {
		t.Fatalf("stack %v, ждали %v", got, want)
	}
}

// «Вернуть авто» — Order = nil, тексты над фото.
func TestOrderAutoNil(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(150, 10, 100, 100, "b")})
	t1, t2 := edtest.TextAt(t, e, "s1", 20, 20), edtest.TextAt(t, e, "s1", 30, 30)
	e.ArrangeLayers("s1", "back", []string{t2})
	if edtest.P(e).Spreads[0].Order == nil {
		t.Fatal("текст под фото не записан")
	}
	v := e.SetGrid("s1", false)
	s, sv := edtest.P(e).Spreads[0], v.Spreads[0]
	if s.Grid != nil || s.Order != nil || !slices.Equal(textIDsOf(e), []string{t2, t1}) {
		t.Fatalf("«Авто»: Grid %v, Order %v, тексты %v", s.Grid, s.Order, textIDsOf(e))
	}
	top := 0
	for _, f := range sv.Frames {
		top = max(top, f.Z)
	}
	if len(sv.Frames) != 2 || top == 0 || sv.Texts[0].Z <= top || sv.Texts[1].Z <= sv.Texts[0].Z {
		t.Fatalf("z: ячейки до %d, тексты %d, %d", top, sv.Texts[0].Z, sv.Texts[1].Z)
	}
}

// Order — в derived, не в empty(): разворот, где удалили всё сразу, удаляется,
// хотя Order до syncOrder ещё не пуст.
func TestOrderEmptySpreadDeleted(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")}, []string{"x"})
	t1 := edtest.TextAt(t, e, "s1", 20, 20)
	e.ArrangeLayers("s1", "back", []string{t1})
	if edtest.P(e).Spreads[0].Order == nil {
		t.Fatal("текст под фото не записан")
	}
	e.DeleteSelection("s1", []string{"a"}, edtest.PlaceIDsOf(e), []string{t1})
	if len(edtest.P(e).Spreads) != 1 || edtest.P(e).Spreads[0].ID != "s2" {
		t.Fatalf("развороты: %d, первый %s", len(edtest.P(e).Spreads), edtest.P(e).Spreads[0].ID)
	}
}

func TestArrangeLayersTextUnderPhoto(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 120, 120, "a"), edtest.Gp(200, 10, 60, 60, "b")})
	g := edtest.PlaceIDsOf(e)
	a, b := g[0], g[1]
	t1 := edtest.TextAt(t, e, "s1", 40, 40) // перекрывает только a
	// down: под ближайшее перекрывающееся ниже — под a, b неперекрытое стоит.
	e.ArrangeLayers("s1", "down", []string{t1})
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, a, b}) {
		t.Fatalf("down: %v", got)
	}
	edtest.MustGrid(t, e)
	sv := e.View().Spreads[0]
	if sv.Texts[0].Z != 1 || sv.Places[0].Z != 2 || sv.Frames[0].Z != 2 || sv.Places[1].Z != 3 {
		t.Fatalf("z: текст %d, места %d %d, ячейка %d", sv.Texts[0].Z, sv.Places[0].Z, sv.Places[1].Z, sv.Frames[0].Z)
	}
	// up: сразу над a; b не перекрыт — остаётся выше.
	e.ArrangeLayers("s1", "up", []string{t1})
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{a, t1, b}) {
		t.Fatalf("up: %v", got)
	}
	// ArrangePlaces front/back — тот же стек: «наверх» — и над текстом.
	if _, err := e.ArrangePlaces("s1", "front", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{t1, b, a}) || !slices.Equal(edtest.PlaceIDsOf(e), []string{b, a}) {
		t.Fatalf("ArrangePlaces front: %v, места %v", got, edtest.PlaceIDsOf(e))
	}
	if _, err := e.ArrangePlaces("s1", "back", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{a, t1, b}) {
		t.Fatalf("ArrangePlaces back: %v", got)
	}
	// Текст наверх — порядок по умолчанию, Order = nil.
	e.ArrangeLayers("s1", "front", []string{t1})
	if got := edtest.StackOf(e); edtest.P(e).Spreads[0].Order != nil || !slices.Equal(got, []string{a, b, t1}) {
		t.Fatalf("front текста: Order %v, стек %v", edtest.P(e).Spreads[0].Order, got)
	}
	edtest.MustGrid(t, e)
	e.Undo()
	if got := edtest.P(e).Spreads[0].Order; !slices.Equal(got, []string{a, t1, b}) {
		t.Fatalf("undo: %v", got)
	}
}
