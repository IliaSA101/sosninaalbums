package spreads_test

// Свой кадр фото (098 design §3.1): все пути «Авто» видят фото в пропорциях своего кадра —
// правило сгиба, листание, галерея, ключ из интерфейса, закрепление; цена — холодные ряды кэша.

import (
	"fmt"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ownEd — «Авто»-разворот из фото ph, у фото asp — свой кадр одной правкой (для замера: правило
// сгиба здесь ни при чём, KeepLayouts держит прежний ключ).
func ownEd(t testing.TB, ph []model.PhotoInfo, asp map[string]float64) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	id := edtest.NewSpread(e, -1, ph).Spreads[0].ID
	edtest.Change(e, func(p *model.Project) {
		for k, a := range asp {
			r := p.Photos[k]
			r.Asp = a
			p.Photos[k] = r
		}
	})
	return e, id
}

// cropEd — свой кадр путём Марины: разворот из ph с ключом key ("" — лучшая), место фото ph[0]
// стало box (разворот закрепился), «Вернуть авто» с галочкой.
func cropEd(t *testing.T, ph []model.PhotoInfo, key string, box model.PlaceBox) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	id := edtest.NewSpread(e, -1, ph).Spreads[0].ID
	if key != "" && e.SetLayout(id, key).Spreads[0].Layout != key {
		t.Fatalf("ключ %q не встал", key)
	}
	box.ID = ph[0].ID
	e.SetPlaces(id, []model.PlaceBox{box}, "")
	edtest.MustGrid(t, e)
	if _, err := e.BackToAuto(id, "", true); err != nil {
		t.Fatal(err)
	}
	if edtest.P(e).Photos[ph[0].ID].Asp == 0 {
		t.Fatal("свой кадр не встал")
	}
	return e, id
}

// onSpine — ячейка r лежит на корешке разворота.
func onSpine(r layout.Rect, prm layout.Params) bool {
	return r.X < prm.PageW-0.01 && r.X+r.W > prm.PageW+0.01
}

// allRows — все раскладки галереи разворота (ряды разворота и страниц).
func allRows(c model.Choices) [][]layout.Rect {
	var out [][]layout.Rect
	for _, rows := range []*model.ChoiceRows{&c.Whole, c.Left, c.Right} {
		if rows == nil {
			continue
		}
		for _, row := range [][]model.Choice{rows.InOrder, rows.Rearranged, rows.Extra} {
			for _, ch := range row {
				out = append(out, ch.Rects)
			}
		}
	}
	return out
}

// TestFoldRuleOwnCrop: широкое фото с узким своим кадром (4 : 5) через сгиб не идёт ни в одном пути
// «Авто» — и по ключу, сохранённому до закрепления (V(0,1): оба фото на корешке); узкое со своим
// кадром 1,8 — идёт (у целого 2 : 3 такой раскладки нет).
func TestFoldRuleOwnCrop(t *testing.T) {
	const cross = "V(0,1)"
	w := whPhotos("w", [2]int{300, 200}, [2]int{300, 200})
	e, id := cropEd(t, w, cross, model.PlaceBox{X: 50, Y: 50, W: 100, H: 125})
	p := edtest.P(e)
	prm, asp := p.Params(), p.Aspects(p.Spreads[0].Photos)
	if c, ok := layout.Build(cross, p.PhotoAspects(p.Spreads[0].Photos), prm); !ok || !onSpine(c.Rects[0], prm) {
		t.Fatalf("%s у целых 3:2 должен класть фото на корешок — тест не про правило", cross)
	}
	check := func(what string, rects []layout.Rect) {
		t.Helper()
		if !layout.FoldOK(rects, asp, prm) || onSpine(rects[0], prm) {
			t.Fatalf("%s: свой кадр 4 : 5 через сгиб: %+v", what, rects[0])
		}
	}
	check("«Вернуть авто»", e.View().Spreads[0].Rects)
	for i, rects := range allRows(e.LayoutChoices(id)) {
		check(fmt.Sprint("галерея ", i), rects)
	}
	for i := range 6 {
		check(fmt.Sprint("‹ › ", i), e.StepLayout(id, "spread", 1).Spreads[0].Rects)
	}
	// Ключ целых фото из интерфейса (галерея до своего кадра) — отказ без шага.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetLayout(id, cross)
	edtest.Same(t, e, rev, undo, "ключ через сгиб")
	// Закрепление: место — в пропорциях своего кадра, картинка та же.
	edtest.FreezeAt(t, e, 0)
	pl := edtest.P(e).Spreads[0].Grid[0]
	if r := pl.W / pl.H; pl.Photo != "w0" || r < 0.8*0.999 || r > 0.8*1.001 || onSpine(layout.Rect{X: pl.X, W: pl.W}, prm) {
		t.Fatalf("место после закрепления: %+v", *pl)
	}

	e, id = cropEd(t, whPhotos("v", [2]int{200, 300}), "", model.PlaceBox{X: 50, Y: 50, W: 180, H: 100})
	for _, rects := range allRows(e.LayoutChoices(id)) {
		if onSpine(rects[0], prm) {
			return
		}
	}
	t.Fatal("свой кадр 1,8: в галерее нет раскладки через сгиб")
}

// stepOwnCrop — ‹ › × 5 по развороту из n фото, у каждого второго свой кадр, на холодном кэше
// раскладок; время пяти шагов.
func stepOwnCrop(tb testing.TB, n int) time.Duration {
	tb.Helper()
	ph := mixedPhotos("o", n)
	asp := map[string]float64{}
	for i := 0; i < n; i += 2 {
		asp[ph[i].ID] = []float64{1.7, 0.8}[i/2%2]
	}
	e, id := ownEd(tb, ph, asp)
	layout.ResetCache()
	variety.ResetCache()
	start := time.Now()
	for range 5 {
		e.StepLayout(id, "spread", 1)
	}
	return time.Since(start)
}

// TestStepLayoutOwnCropTime: листание разворота со своими кадрами на холодном кэше — в пределе
// (design §3.3: тот же порядок, что поворот фото на 90°).
func TestStepLayoutOwnCropTime(t *testing.T) {
	testkit.CostTest(t)
	for _, c := range []struct {
		n    int
		base time.Duration
	}{{5, 300 * time.Millisecond}, {16, 2 * time.Second}} {
		if c.n > 5 && testkit.Lite() {
			continue
		}
		if d := testkit.Fastest(3, func() { stepOwnCrop(t, c.n) }); d > testkit.Limit(c.base) {
			t.Fatalf("%d фото: ‹ › × 5 за %v", c.n, d)
		}
	}
}

// BenchmarkStepLayoutOwnCrop — ‹ › × 5 на холодном кэше, 5 и 16 фото со своими кадрами.
func BenchmarkStepLayoutOwnCrop(b *testing.B) {
	for _, n := range []int{5, 16} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			var sum time.Duration
			for b.Loop() {
				sum += stepOwnCrop(b, n)
			}
			b.ReportMetric(float64(sum.Milliseconds())/float64(b.N), "мс/5шагов")
		})
	}
}
