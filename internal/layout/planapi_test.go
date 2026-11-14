package layout

import (
	"slices"
	"testing"
)

// Узкий вход для layout/plan (planapi.go): обёртки дают то же, что неэкспортные функции.
// Сверку самой группировки с прежним путём ведёт пакет plan.

func TestNewGeomMatches(t *testing.T) {
	sheet := Params{PageW: 102, PageH: 304, Margin: 8, Gap: 3, Layflat: true, Sheet: true}
	for _, p := range append(slices.Clone(invariantParams), sheet) {
		g, w := NewGeom(p), newGeom(p)
		if g.Spread != w.spread || g.Pages != w.pages || g.Full != w.full || g.Spine != w.spine {
			t.Errorf("%+v: NewGeom %+v, newGeom %+v", p, g, w)
		}
	}
	if g := NewGeom(sheet); g.Spine != 0 {
		t.Errorf("у листа корешок %v", g.Spine)
	}
}

// TestBreakDPCuts — таблица BreakDP даёт те же границы рядов, что lineBreaks; нет места — false.
func TestBreakDPCuts(t *testing.T) {
	asp := []float64{L, P, P, L, S, L, P, L, L, P}
	g := newGeom(album30)
	for _, r := range []int{1, 2, 3, 4} {
		want := lineBreaks(asp, 0, len(asp), g.pages[0], album30.Gap, r, false)
		tab, ok := BreakDP(asp, 0, len(asp), g.pages[0], album30.Gap, r, false)
		if !ok || tab.M != len(asp) {
			t.Fatalf("r=%d: таблица %+v", r, tab)
		}
		got := make([]int, r+1)
		for k, j := r, tab.M; k > 0; k-- {
			got[k] = j
			j = tab.From[k*(tab.M+1)+j]
		}
		if !slices.Equal(got, want) {
			t.Errorf("r=%d: границы %v, lineBreaks %v", r, got, want)
		}
	}
	if tab, ok := BreakDP(asp, 0, len(asp), Rect{W: 100, H: 0}, 3, 2, false); ok {
		t.Errorf("нулевая высота: таблица %+v, ждали false", tab)
	}
}

func TestScoreWrappers(t *testing.T) {
	asp := []float64{L, P, P, L, S, L, P, L}
	g := NewGeom(album30)
	for _, r := range Layouts(asp, album30, 6) {
		got := Score(r.Rects, g.Spread, g.Spine, 3)
		if want := score(r.Rects, g.Spread, g.Spine, 3); got != want {
			t.Errorf("%s: Score %v, score %v", r.Key, got, want)
		}
		got = PairScore(r.Rects, g, 3, 100, 120)
		if want := pairScore(r.Rects, g.g, 3, 100, 120, false); got != want {
			t.Errorf("%s: PairScore %v, pairScore %v", r.Key, got, want)
		}
	}
}

// TestOnReset — ResetCache зовёт все подписки (матрицы plan сбрасываются вместе с кэшем).
func TestOnReset(t *testing.T) {
	resetMu.Lock()
	saved := resetHooks
	resetHooks = nil // подписки plan не считаем; прежние вернёт Cleanup
	resetMu.Unlock()
	t.Cleanup(func() {
		resetMu.Lock()
		resetHooks = saved
		resetMu.Unlock()
	})
	a, b := 0, 0
	OnReset(func() { a++ })
	OnReset(func() { b++ })
	ResetCache()
	resetCache()
	if a != 2 || b != 2 {
		t.Errorf("подписки сработали %d и %d раз, ждали по 2", a, b)
	}
}

// TestRefPath — прежний путь (эталон тестов plan): окно считается, повтор даёт то же,
// деревья рядов вписываются в box.
func TestRefPath(t *testing.T) {
	asp := []float64{L, P, P, L, S, L, P, L, L, P, L, L}
	for _, p := range invariantParams {
		s := RefQuickScore(asp, p)
		if s != RefQuickScore(asp, p) || s <= 0 {
			t.Errorf("layflat=%t: RefQuickScore %v нестабилен или неположителен", p.Layflat, s)
		}
		box := NewGeom(p).Pages[0]
		fits := RefRows(asp, p.Gap)(2, 5, box)
		if len(fits) == 0 {
			t.Fatalf("layflat=%t: деревьев рядов нет", p.Layflat)
		}
		for i, f := range fits {
			if !f.Ok || len(f.Rects) != 5 || f.H <= 0 || f.Depth < 1 {
				t.Errorf("layflat=%t дерево %d: %+v", p.Layflat, i, f)
			}
		}
	}
}
