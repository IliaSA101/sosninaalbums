package variety

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ряд «Авто» на 17–40 фото (спека 072, design §6): формы не мельче мозаики, «Ряды», «С главным».

// manyGeoms — 4 геометрии 072: 30×30 «любые», 30×30 «только широкие», без layflat, 30×20.
func manyGeoms() []struct {
	name string
	p    layout.Params
} {
	return []struct {
		name string
		p    layout.Params
	}{{"30x30", geomSq}, {"30x30W", wideFold(geomSq)}, {"flat", geomFlat}, {"30x20", geomWide}}
}

// manyRowSets — 8 наборов n фото: смесь, 2 серии, гориз., верт., stock с 0, 30, 60.
func manyRowSets(st []float64, n int) []struct {
	name string
	as   []float64
} {
	return []struct {
		name string
		as   []float64
	}{
		{"смесь", mixedAspects(n, uint64(n))}, {"серия", seriesAspects(n, 0.65, uint64(n))},
		{"серия+7", seriesAspects(n, 0.65, uint64(n+7))}, {"гориз", sameAspects(n, aspL)},
		{"верт", sameAspects(n, aspP)}, {"stock0", st[0:n]}, {"stock30", st[30 : 30+n]}, {"stock60", st[60 : 60+n]},
	}
}

// formsIn8 — разных форм в первых 8 плитках ряда и их имена.
func formsIn8(row []forms.Cand) (int, string) {
	seen := map[forms.Form]bool{}
	var names []string
	for _, c := range row[:min(8, len(row))] {
		seen[c.Form] = true
		names = append(names, string(c.Form))
	}
	return len(seen), strings.Join(names, " ")
}

// TestRowManyForms (критерий 072): первые 8 «Авто» на 17–40 фото × 4 геометрии × 8 наборов — на 25 и
// 28 фото ≥ 3 форм во всех 32 наборах, на 30 — в ≥ 28; ≥ 4 форм на 25 — в ≥ 28 (серия 25 — ≥ 3 на
// всех геометриях входит в «все 32»). На прочих числах — только счёт (-v): пороги спеки — 25–30.
func TestRowManyForms(t *testing.T) {
	t.Cleanup(ResetCache)
	st := testkit.StockAspects(t)
	want3 := map[int]int{25: 32, 28: 32, 30: 28}
	for _, n := range []int{17, 20, 22, 25, 28, 30, 35, 40} {
		n3, n4, sum, total := 0, 0, 0, 0
		for _, g := range manyGeoms() {
			for _, s := range manyRowSets(st, n) {
				got, names := formsIn8(Row(s.as, g.p, rowLen(n)))
				total++
				sum += got
				if got >= 3 {
					n3++
				}
				if got >= 4 {
					n4++
				}
				if testing.Verbose() {
					t.Logf("%d фото %s/%s: %d форм — %s", n, g.name, s.name, got, names)
				}
				if want3[n] == 32 && got < 3 {
					t.Errorf("%d фото %s/%s: %d форм — %s", n, g.name, s.name, got, names)
				}
			}
		}
		t.Logf("%d фото: ср %.1f, ≥ 3 форм %d/%d, ≥ 4 — %d", n, float64(sum)/float64(total), n3, total, n4)
		if w, ok := want3[n]; ok && n3 < w {
			t.Errorf("%d фото: ≥ 3 форм в %d наборах из %d, нужно %d", n, n3, total, w)
		}
		if n == 25 && n4 < 28 {
			t.Errorf("25 фото: ≥ 4 форм в %d наборах из %d, нужно 28", n4, total)
		}
	}
}

// sideNoHeroes — мельчайшая сторона фото; у «С главным» и «Две крупных» — без главных.
func sideNoHeroes(c forms.Cand) float64 {
	h := forms.Heroes(c.Rects)
	skip := c.Form == forms.Hero || c.Form == forms.Pair
	m := math.Inf(1)
	for i, r := range c.Rects {
		if skip && (h.N >= 1 && h.Idx[0] == i || h.N == 2 && h.Idx[1] == i) {
			continue
		}
		m = math.Min(m, math.Min(r.W, r.H))
	}
	return m
}

// TestRowManyNotSmaller (072-2, 072-8): у каждой формы, вставленной в ряд на 17–40 фото, мельчайшая
// сторона (у «С главным» и «Две крупных» — без главных) ≥ manySideSlack той же у Row[0] (в квантах) —
// или форма держит свой пол вида без послабления (Quality): порог мозаики полы снижает, но не
// поднимает (у серии 25 мозаика 50 мм, «Ряды» 44 мм при поле вида 42 — в ряду, design §1). Хоть раз
// вставлена форма, прошедшая только по порогу мозаики; среди вставленных — «Ряды», «С главным
// фото», «Две крупных».
func TestRowManyNotSmaller(t *testing.T) {
	t.Cleanup(ResetCache)
	st := testkit.StockAspects(t)
	seen := map[forms.Form]int{}
	relaxed := 0
	for _, n := range []int{17, 20, 25, 28, 30, 35, 40} {
		for _, g := range manyGeoms() {
			for _, s := range manyRowSets(st, n) {
				e := rowOf(s.as, g.p, rowLen(n))
				floor := manySideSlack * sideNoHeroes(forms.Cand{Result: e.row[0].Result})
				for _, c := range e.row[1:] {
					if !e.gen[c.Key] {
						continue
					}
					seen[c.Form]++
					_, own := forms.Quality(c.Result, c.Form, g.p)
					if !own {
						relaxed++
					}
					if side := sideNoHeroes(c); quanta(side) < quanta(floor) && !own {
						t.Errorf("%d фото %s/%s: %s %q — сторона %.1f мм < %.1f", n, g.name, s.name, c.Form, c.Key, side, floor)
					}
				}
			}
		}
	}
	t.Logf("вставленные формы: %v, по порогу мозаики — %d", seen, relaxed)
	if seen[forms.Rows] == 0 || seen[forms.Hero] == 0 || seen[forms.Pair] == 0 || relaxed == 0 {
		t.Errorf("нет «Рядов», «С главным», «Две крупных» или форм по порогу мозаики (%d): %v", relaxed, seen)
	}
}

// TestRowManyFirstSame: Row[0] на многих фото — по-прежнему лучшая раскладка layout (Layout "" и
// «Меньше» не меняются), до ManyFrom formRelax — 0, с ManyFrom — manySideSlack мозаики.
func TestRowManyFirstSame(t *testing.T) {
	t.Cleanup(ResetCache)
	st := testkit.StockAspects(t)
	for _, n := range []int{16, 17, 30, 40} {
		as := st[:n]
		base := layout.Cached(as, geomSq, rowLen(n))
		if row := Row(as, geomSq, rowLen(n)); row[0].Key != base[0].Key {
			t.Errorf("%d фото: Row[0] %q, у layout %q", n, row[0].Key, base[0].Key)
		}
		relax := formRelax(as, base[0])
		mos := sideNoHeroes(forms.Cand{Result: base[0]})
		if n < layout.ManyFrom && relax != 0 || n >= layout.ManyFrom && relax != manySideSlack*mos {
			t.Errorf("%d фото: formRelax %.2f при мозаике %.2f", n, relax, mos)
		}
	}
	if formRelax(st[:20], layout.Result{}) != 0 {
		t.Error("formRelax пустой раскладки не 0")
	}
}

// BenchmarkRow30 — холодный ряд «Авто» на stock 30 вместе с layout (design §5: ≤ 9 мс).
func BenchmarkRow30(b *testing.B) {
	st := testkit.StockAspects(b)[:30]
	for b.Loop() {
		ResetCache()
		layout.ResetCache()
		Row(st, geomSq, rowLen(30))
	}
}

// pickNonMosaic — сколько разворотов из подряд идущих по per фото stock ×2 (не больше 6) выбор
// «Обычное» ставит не мозаикой.
func pickNonMosaic(t *testing.T, per int) (int, string) {
	st := testkit.StockAspects(t)
	all := append(append([]float64{}, st...), st...)
	var sp []Spread
	for i := 0; i+per <= len(all) && len(sp) < 6; i += per {
		sp = append(sp, Spread{Pool: corpusPool(all[i:i+per], geomSq), Star: -1})
	}
	n := 0
	var names []string
	for i, k := range Pick(sp, geomSq, Normal, Ctx{Avg: float64(per)}) {
		f := sp[i].Pool[k].Form
		if f != forms.Mosaic {
			n++
		}
		names = append(names, string(f))
	}
	return n, strings.Join(names, " ")
}

// TestPickManyForms (критерий 072): автосборка «Обычное» на stock ×2 по 30 фото на 6 разворотов —
// не мозаикой ≥ 3 (в 0.17 — 1), по 25 — ≥ 4 (в 0.17 — 3).
func TestPickManyForms(t *testing.T) {
	t.Cleanup(ResetCache)
	for _, c := range []struct{ per, want int }{{30, 3}, {25, 4}} {
		got, names := pickNonMosaic(t, c.per)
		t.Logf("по %d фото: не мозаик %d — %s", c.per, got, names)
		if got < c.want {
			t.Errorf("по %d фото: не мозаик %d < %d (%s)", c.per, got, c.want, names)
		}
	}
}
