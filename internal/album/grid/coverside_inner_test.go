package grid

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// innerCover — проект с обложкой «тонкие листы» и одним местом на лицевой.
func innerCover(t *testing.T) *model.Project {
	t.Helper()
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Grid: []*model.Place{{ID: "c0ffee0000c1", X: 400, Y: 100, W: 120, H: 80}}}}
	if len(p.CoverSides()) != 2 {
		t.Fatal("нет сторонок")
	}
	return p
}

// Сбой layout (паника под recover): ряда нет, форма не узнана даже при памяти, Step холостой, места целы.
func TestCoverGridLayoutPanic(t *testing.T) {
	old := layoutsImpl
	layoutsImpl = func([]float64, layout.Params, int) []layout.Result { panic("тест-паника") }
	t.Cleanup(func() { layoutsImpl = old })
	p := innerCover(t)
	c, _ := coverSideOf(p, model.ScopeRight)
	last := [2][]model.Box{nil, {{X: 0.1, Y: 0.1, W: 0.3, H: 0.3}}}
	if st := CoverSideStates(p, last)[1]; st.Known || st.Alone || st.Places != 1 {
		t.Errorf("состояние при сбое layout: %+v", st)
	}
	before := model.Marshal(p)
	if shape, err := StepCoverGrid(p, model.ScopeRight, 1, last); shape != nil || err != nil || string(model.Marshal(p)) != string(before) {
		t.Errorf("Step при сбое layout: %v, %v", shape, err)
	}
	if len(c.places) != 1 {
		t.Errorf("мест %d", len(c.places))
	}
}

// Нечисловой прямоугольник из layout не превращается в потерянное место: зажим выбрасывает долю — Step отказывается.
func TestStepCoverGridDroppedBox(t *testing.T) {
	old := layoutsImpl
	nan := layout.Result{Rects: []layout.Rect{{X: math.NaN(), W: 10, H: 10}}, Key: "nan"}
	layoutsImpl = func([]float64, layout.Params, int) []layout.Result { return []layout.Result{nan} }
	t.Cleanup(func() { layoutsImpl = old })
	p := innerCover(t)
	c, _ := coverSideOf(p, model.ScopeRight)
	var last [2][]model.Box
	last[1] = fracOf([]layout.Rect{{X: c.places[0].X - c.box.X, Y: c.places[0].Y - c.box.Y, W: c.places[0].W, H: c.places[0].H}}, c.box)
	if !CoverSideStates(p, last)[1].Known {
		t.Fatal("подготовка: форма по памяти не узнана")
	}
	before := model.Marshal(p)
	if shape, err := StepCoverGrid(p, model.ScopeRight, 1, last); shape != nil || err != nil || string(model.Marshal(p)) != string(before) {
		t.Errorf("Step с нечисловой раскладкой: %v, %v", shape, err)
	}
}

// Ряд «По фото» схлопывает формы в допуске sameTol (0.26-127-21): остаётся первая из близких;
// отличие по одной из x, y, w, h больше допуска — разные формы; раскладка не на все места отброшена.
func TestByPhotoRowCollapsesSame(t *testing.T) {
	base := []layout.Rect{{X: 10, Y: 10, W: 100, H: 80}, {X: 120, Y: 10, W: 100, H: 80}}
	shift := func(d float64, f func(r *layout.Rect, d float64)) []layout.Rect {
		out := append([]layout.Rect(nil), base...)
		f(&out[1], d)
		return out
	}
	x := func(r *layout.Rect, d float64) { r.X += d }
	w := func(r *layout.Rect, d float64) { r.W += d }
	fake := []layout.Result{
		{Key: "a", Rects: base},
		{Key: "a-near", Rects: shift(0.05, x)},     // в допуске: схлопнулась
		{Key: "a-near-w", Rects: shift(-0.05, w)},  // в допуске по ширине: схлопнулась
		{Key: "far-x", Rects: shift(0.5, x)},       // дальше допуска по x
		{Key: "far-w", Rects: shift(0.5, w)},       // дальше допуска по w
		{Key: "far-x-near", Rects: shift(0.52, x)}, // близка к far-x, но та сравнивается с оставленными
		{Key: "one", Rects: base[:1]},              // не на все места
	}
	old := layoutsImpl
	layoutsImpl = func([]float64, layout.Params, int) []layout.Result { return fake }
	t.Cleanup(func() { layoutsImpl = old })
	row := byPhotoRow([]float64{1.25, 1.25}, layout.Params{PageW: 150, PageH: 100, Sheet: true})
	var keys []string
	for _, r := range row {
		keys = append(keys, r.Key)
	}
	if want := []string{"a", "far-x", "far-w"}; !slices.Equal(keys, want) {
		t.Errorf("ряд %v, ждали %v", keys, want)
	}
}
