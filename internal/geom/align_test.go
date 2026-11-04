package geom

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// spreadSheet — разворот 200×100 мм, поле 10: две страницы встык.
func spreadSheet() Sheet {
	return Sheet{W: 200, H: 100, Margin: 10, Pages: []layout.Rect{{X: 0, Y: 0, W: 100, H: 100}, {X: 100, Y: 0, W: 100, H: 100}}}
}

// coverSheet — обложка: две сторонки с корешком между ними.
func coverSheet() Sheet {
	return Sheet{W: 200, H: 100, Margin: 10, Pages: []layout.Rect{{X: 0, Y: 0, W: 90, H: 100}, {X: 110, Y: 0, W: 90, H: 100}}}
}

func closeTo(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestSheetPageAt(t *testing.T) {
	one := Sheet{W: 100, H: 100, Pages: []layout.Rect{{X: 0, Y: 0, W: 100, H: 100}}}
	cases := []struct {
		name string
		st   Sheet
		x    float64
		want int
	}{
		{"левая страница", spreadSheet(), 50, 0},
		{"правая страница", spreadSheet(), 150, 1},
		{"шов — уже правая", spreadSheet(), 100, 1},
		{"левый край листа", spreadSheet(), 0, 0},
		{"правый край листа — ближайшая", spreadSheet(), 200, 1},
		{"левее листа", spreadSheet(), -50, 0},
		{"правее листа", spreadSheet(), 999, 1},
		{"корешок ближе к правой", coverSheet(), 106, 1},
		{"корешок ближе к левой", coverSheet(), 94, 0},
		{"корешок ровно посередине — первая", coverSheet(), 100, 0},
		{"одна страница", one, 50, 0},
		{"одна страница, вне листа", one, 500, 0},
		{"без страниц", Sheet{W: 100, H: 100}, 50, 0},
		{"NaN — первая", spreadSheet(), math.NaN(), 0},
	}
	for _, c := range cases {
		if got := c.st.PageAt(c.x); got != c.want {
			t.Errorf("%s: PageAt(%v) = %d, ждали %d", c.name, c.x, got, c.want)
		}
	}
}

func shifts(t *testing.T, st Sheet, rects []layout.Rect, op, to string) ([][2]float64, bool) {
	t.Helper()
	out, over := AlignShifts(st, rects, op, to)
	if len(out) != len(rects) {
		t.Fatalf("сдвигов %d на %d рамок", len(out), len(rects))
	}
	return out, over
}

// «По странице»: каждая рамка встаёт в поле своей страницы (по центру рамки).
func TestAlignByPageTwoAreas(t *testing.T) {
	left := layout.Rect{X: 20, Y: 20, W: 20, H: 20}
	right := layout.Rect{X: 120, Y: 30, W: 20, H: 20}
	rects := []layout.Rect{left, right}
	for name, st := range map[string]Sheet{"разворот": spreadSheet(), "обложка": coverSheet()} {
		right0 := st.Pages[1].X
		for _, c := range []struct {
			op      string
			l, r    [2]float64
			comment string
		}{
			{"left", [2]float64{-10, 0}, [2]float64{right0 + 10 - 120, 0}, "к левому полю страницы"},
			{"right", [2]float64{st.Pages[0].W - 10 - 40, 0}, [2]float64{right0 + st.Pages[1].W - 10 - 140, 0}, "к правому полю страницы"},
			{"hcenter", [2]float64{st.Pages[0].W/2 - 30, 0}, [2]float64{right0 + st.Pages[1].W/2 - 130, 0}, "по центру страницы"},
			{"top", [2]float64{0, -10}, [2]float64{0, -20}, "к верхнему полю"},
			{"bottom", [2]float64{0, 100 - 10 - 40}, [2]float64{0, 100 - 10 - 50}, "к нижнему полю"},
			{"vcenter", [2]float64{0, 50 - 30}, [2]float64{0, 50 - 40}, "по середине"},
		} {
			out, over := shifts(t, st, rects, c.op, AlignPage)
			if over || !closeTo(out[0][0], c.l[0]) || !closeTo(out[0][1], c.l[1]) || !closeTo(out[1][0], c.r[0]) || !closeTo(out[1][1], c.r[1]) {
				t.Errorf("%s/%s (%s): %v over=%v, ждали %v %v", name, c.op, c.comment, out, over, c.l, c.r)
			}
		}
	}
}

// Лист с одной страницей и лист без страниц ведут себя одинаково: поле — весь лист.
func TestAlignByPageSingleArea(t *testing.T) {
	r := []layout.Rect{{X: 120, Y: 20, W: 20, H: 20}}
	one := Sheet{W: 200, H: 100, Margin: 10, Pages: []layout.Rect{{X: 0, Y: 0, W: 200, H: 100}}}
	none := Sheet{W: 200, H: 100, Margin: 10}
	for name, st := range map[string]Sheet{"одна": one, "без страниц": none} {
		if out, _ := shifts(t, st, r, "left", AlignPage); !closeTo(out[0][0], 10-120) || out[0][1] != 0 {
			t.Errorf("%s: left %v", name, out)
		}
		if out, _ := shifts(t, st, r, "hcenter", AlignPage); !closeTo(out[0][0], 100-130) {
			t.Errorf("%s: hcenter %v", name, out)
		}
	}
	// Опора «по развороту» не зависит от страниц; «по выделению» — от union.
	two := []layout.Rect{{X: 20, Y: 20, W: 20, H: 20}, {X: 120, Y: 60, W: 20, H: 20}}
	if out, _ := shifts(t, spreadSheet(), two, "left", AlignSpread); !closeTo(out[0][0], -10) || !closeTo(out[1][0], -110) {
		t.Errorf("по развороту: %v", out)
	}
	if out, _ := shifts(t, spreadSheet(), two, "left", AlignSel); !closeTo(out[0][0], 0) || !closeTo(out[1][0], -100) {
		t.Errorf("по выделению: %v", out)
	}
}

// Распределение «по странице»: группы по страницам независимы, одиночная рамка стоит.
func TestDistributeByPage(t *testing.T) {
	rects := []layout.Rect{
		{X: 20, Y: 10, W: 20, H: 10}, {X: 50, Y: 10, W: 20, H: 10}, // левая: поле 10..90, зазор 40
		{X: 130, Y: 10, W: 20, H: 10}, // правая: одна — не двигается
	}
	out, over := shifts(t, spreadSheet(), rects, "distH", AlignPage)
	if over || !closeTo(out[0][0], -10) || !closeTo(out[1][0], 20) || out[2] != [2]float64{} {
		t.Fatalf("две страницы: %v over=%v", out, over)
	}
	// Распределение по вертикали «по выделению»: крайние стоят, середина — посередине.
	v := []layout.Rect{{X: 0, Y: 0, W: 10, H: 10}, {X: 0, Y: 5, W: 10, H: 10}, {X: 0, Y: 50, W: 10, H: 10}}
	if out, over = shifts(t, spreadSheet(), v, "distV", AlignSel); over || out[0][1] != 0 || !closeTo(out[2][1], 0) || !closeTo(out[1][1], 20) {
		t.Fatalf("distV sel: %v over=%v", out, over)
	}
}

// Не помещаются: over, первая у начала поля, последняя у конца, остальные равным шагом;
// крайние шире поля — все центры в середине поля; повтор холостой.
func TestDistributeOverflow(t *testing.T) {
	st := spreadSheet() // поле левой страницы 10..90
	three := []layout.Rect{{X: 15, Y: 0, W: 40, H: 10}, {X: 30, Y: 0, W: 40, H: 10}, {X: 45, Y: 0, W: 40, H: 10}}
	out, over := shifts(t, st, three, "distH", AlignPage)
	if !over {
		t.Fatal("три по 40 в поле 80 помещаются?")
	}
	for i, wantCentre := range []float64{30, 50, 70} {
		if c := three[i].X + out[i][0] + 20; !closeTo(c, wantCentre) {
			t.Errorf("центр %d: %.2f, ждали %.2f", i, c, wantCentre)
		}
	}
	wide := []layout.Rect{{X: 0, Y: 0, W: 100, H: 10}, {X: 10, Y: 0, W: 100, H: 10}}
	out, over = shifts(t, st, wide, "distH", AlignPage)
	if !over {
		t.Fatal("широкие помещаются?")
	}
	for i, r := range wide {
		if c := r.X + out[i][0] + 50; !closeTo(c, 50) {
			t.Errorf("широкая %d: центр %.2f, ждали 50", i, c)
		}
	}
	moved := make([]layout.Rect, len(wide))
	for i, r := range wide {
		moved[i] = layout.Rect{X: r.X + out[i][0], Y: r.Y, W: r.W, H: r.H}
	}
	if again, _ := shifts(t, st, moved, "distH", AlignPage); again[0] != [2]float64{} || again[1] != [2]float64{} {
		t.Errorf("повтор не холостой: %v", again)
	}
}

// Вырожденные размеры: пустой набор, нулевой лист, нулевые рамки — без паники и NaN.
func TestAlignDegenerate(t *testing.T) {
	if out, over := shifts(t, spreadSheet(), nil, "distH", AlignPage); len(out) != 0 || over {
		t.Fatalf("пустой набор: %v %v", out, over)
	}
	if out, _ := shifts(t, spreadSheet(), nil, "left", AlignSel); len(out) != 0 {
		t.Fatalf("пустой набор, left: %v", out)
	}
	zero := Sheet{}
	rects := []layout.Rect{{}, {X: 5, Y: 5}, {X: 5, Y: 5, W: 10, H: 10}}
	for _, op := range []string{"left", "right", "hcenter", "top", "bottom", "vcenter", "distH", "distV"} {
		for _, to := range []string{AlignSel, AlignPage, AlignSpread} {
			out, _ := shifts(t, zero, rects, op, to)
			for _, d := range out {
				if math.IsNaN(d[0]) || math.IsNaN(d[1]) || math.IsInf(d[0], 0) || math.IsInf(d[1], 0) {
					t.Fatalf("%s/%s: %v", op, to, out)
				}
			}
		}
	}
	// Поле шире листа (поле больше половины стороны) — отрицательная ширина не ломает расчёт.
	tiny := Sheet{W: 10, H: 10, Margin: 20}
	if out, _ := shifts(t, tiny, rects[2:], "left", AlignSpread); math.IsNaN(out[0][0]) {
		t.Fatalf("поле больше листа: %v", out)
	}
}
