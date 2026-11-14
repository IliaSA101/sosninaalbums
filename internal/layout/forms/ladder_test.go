package forms

// 055: честная Q лесенки (пустота полосы ступеней) и штраф пустой страницы; LadderDir, MirrorLadder.

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// qBefore — Q по формуле до 055: без пустоты ступеней и пустых страниц.
func qBefore(t *testing.T, r layout.Result, f Form, p layout.Params) float64 {
	t.Helper()
	env, share, _, _ := envelope(r.Key, f, r.Rects, p)
	return env - spreadTerm(r.Rects, Heroes(r.Rects)) - spineTerm(r.Rects, p) - depthW*float64(depthOf(r.Key)) - bandTerm(f, share)
}

func mustBuild(t *testing.T, key string, as []float64, p layout.Params) layout.Result {
	t.Helper()
	r, ok := layout.Build(key, as, p)
	if !ok {
		t.Fatalf("%s не строится", key)
	}
	return r
}

// TestLadderQualityHonest: лесенка 3 вертикальных на странице с пустой второй — ниже мозаики и
// ниже 0.8 (было 0.985: место 1 ряда); лесенка без пустой страницы — прежняя Q − 1/12 при любом
// числе фото и геометрии (полоса ступеней h/(h+s) = 2/3); у прочих видов Q прежняя.
func TestLadderQualityHonest(t *testing.T) {
	v3 := same(3, aspP)
	lad := mustBuild(t, "P[-|A(0,1,2)]", v3, geomSq)
	q, _ := Quality(lad, Ladder, geomSq)
	best := layout.Layouts(v3, geomSq, 1)[0]
	mq, _ := Quality(best, Of(best.Key, best.Rects, geomSq), geomSq)
	if q >= mq || q >= 0.8 {
		t.Errorf("3V лесенка с пустой страницей: Q %.3f, мозаика %.3f", q, mq)
	}
	for _, c := range []struct {
		key string
		as  []float64
		p   layout.Params
	}{
		{"A(0,1,2,3)", same(4, aspP), geomSq},
		{"L(0,1,2,3)", same(4, aspP), geomWide},
		{"A(0,1,2)", same(3, aspL), geomSq},
		{"L(0,1,2,3,4)", same(5, aspL), geomSq},
		{"P[0|A(1,2,3)]", same(4, aspP), geomSq},
		{"P[L(0,1,2)|H(3,4)]", mixed(5, 5), geomFlat},
	} {
		r := mustBuild(t, c.key, c.as, c.p)
		if Of(r.Key, r.Rects, c.p) != Ladder {
			t.Fatalf("%s: не лесенка", c.key)
		}
		q, _ := Quality(r, Ladder, c.p)
		if want := qBefore(t, r, Ladder, c.p) - 1.0/12; math.Abs(q-want) > 1e-9 {
			t.Errorf("%s: Q %.12f, ждали прежняя − 1/12 = %.12f", c.key, q, want)
		}
	}
	for _, c := range []struct {
		key string
		f   Form
		as  []float64
	}{
		{"H(0,1,2)", Strip, same(3, aspL)},
		{"G(H(0,1),H(2,3))", Grid, same(4, aspL)},
		{"P[0|H(1,2)]", Asym, same(3, aspL)},
		{"H(0,1,2)", Ladder, same(3, aspL)}, // вид Ladder без L/A в ключе — углов нет
	} {
		r := mustBuild(t, c.key, c.as, geomSq)
		q, _ := Quality(r, c.f, geomSq)
		if want := qBefore(t, r, c.f, geomSq); q != want {
			t.Errorf("%s (%s): Q %.12f, прежняя %.12f", c.key, c.f, q, want)
		}
	}
}

// TestEmptyPageTerm: пустая страница постраничного ключа — ровно −emptyPageW у любого вида (две
// пустых не бывает: Build их не строит); у «в край» Q — оценка layout, как была.
func TestEmptyPageTerm(t *testing.T) {
	for _, c := range []struct {
		key string
		f   Form
		as  []float64
		air float64 // пустота ступеней лесенки
	}{
		{"P[-|A(0,1,2)]", Ladder, same(3, aspP), 1.0 / 3},
		{"P[L(0,1,2)|-]", Ladder, same(3, aspP), 1.0 / 3},
		{"P[-|H(0,1)]", Air, same(2, aspL), 0},
		{"P[V(0,1)|-]", Asym, same(2, aspL), 0},
	} {
		r := mustBuild(t, c.key, c.as, geomSq)
		q, _ := Quality(r, c.f, geomSq)
		if want := qBefore(t, r, c.f, geomSq) - ladderAirW*c.air - emptyPageW; math.Abs(q-want) > 1e-9 {
			t.Errorf("%s: Q %.12f, ждали %.12f", c.key, q, want)
		}
	}
	// та же лесенка с заполненной второй страницей — без штрафа
	full := mustBuild(t, "P[0|A(1,2,3)]", same(4, aspP), geomSq)
	if _, _, _, empty := envelope(full.Key, Ladder, full.Rects, geomSq); empty != 0 {
		t.Errorf("%s: пустых страниц %d", full.Key, empty)
	}
	bleeds := 0
	for _, g := range []layout.Params{geomSq, geomFlat} {
		for _, key := range []string{"B[right:0]", "B[pages:0,1]"} {
			r, ok := layout.Build(key, same(len(leavesOfKey(key)), 1), g)
			if !ok {
				continue
			}
			bleeds++
			if q, ok := Quality(r, Bleed, g); !ok || q != r.Score {
				t.Errorf("в край %s: Q %.6f, оценка layout %.6f", r.Key, q, r.Score)
			}
		}
	}
	if bleeds == 0 {
		t.Fatal("ни одного ключа «в край»: тест их не проверяет")
	}
}

// TestLadderDir: направление и зеркало по ключу; без лесенки — 0 и ключ как есть.
func TestLadderDir(t *testing.T) {
	for _, c := range []struct {
		key, mirror string
		dir         byte
	}{
		{"L(0,1,2)", "A(0,1,2)", 'L'},
		{"A(0,1,2,3)", "L(0,1,2,3)", 'A'},
		{"P[-|A(0,1,2)]", "P[-|L(0,1,2)]", 'A'},
		{"P[S(L(0,1,2))|H(3,4)]", "P[S(A(0,1,2))|H(3,4)]", 'L'},
		{"P[0|H(1,2)]", "P[0|H(1,2)]", 0},
		{"QU[0|1]", "QU[0|1]", 0},
		{"B[right:0]", "B[right:0]", 0},
		{"", "", 0},
		{"L", "L", 0},
	} {
		if got := LadderDir(c.key); got != c.dir {
			t.Errorf("LadderDir(%q) = %q, ждали %q", c.key, got, c.dir)
		}
		if got := MirrorLadder(c.key); got != c.mirror {
			t.Errorf("MirrorLadder(%q) = %q, ждали %q", c.key, got, c.mirror)
		}
		if MirrorLadder(MirrorLadder(c.key)) != c.key {
			t.Errorf("MirrorLadder дважды не вернул %q", c.key)
		}
	}
}
