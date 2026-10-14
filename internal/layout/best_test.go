package layout

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// refBlocks — прежний путь: все деревья, оценка, sort.SliceStable по убыванию
// (оценка в порядке o: точная или tieQ).
func refBlocks(o order, asp []float64, box Rect, gap, spine float64) []candidate {
	cs, _ := blockCandidates(asp, box, gap)
	for i := range cs {
		cs[i].Score = score(cs[i].Rects, box, spine, cs[i].depth)
	}
	sort.SliceStable(cs, func(i, j int) bool { return o.q(cs[i].Score) > o.q(cs[j].Score) })
	return cs
}

// refLess — «a раньше b» в выдаче порядка o: оценка по убыванию, в tieOrder ничья — ключ.
func refLess(o order, a, b Result) bool {
	qa, qb := o.q(a.Score), o.q(b.Score)
	switch {
	case qa > qb:
		return true
	case qa < qb, o == legacyOrder:
		return false
	}
	return a.Key < b.Key
}

// refLayouts — Layouts до G1 в порядке o: блок на разворот целиком, в порядке перебора
// (страницы — через topBlocks, его сверяет TestBestBlocksSame).
func refLayouts(o order, asp []float64, p Params, limit int) []Result {
	g := newGeom(p)
	var all []Result
	if p.Layflat {
		cs, _ := blockCandidates(asp, g.spread, p.Gap)
		for _, c := range cs {
			c.Score = score(c.Rects, g.spread, g.spine, c.depth)
			all = append(all, c.Result)
		}
	}
	n := len(asp)
	for k := 0; k <= n; k++ {
		ls := topBlocks(o, asp[:k], g.pages[0], p.Gap, perSide)
		rs := topBlocks(o, asp[k:], g.pages[1], p.Gap, perSide)
		for i := range rs {
			rs[i].Key = mapLeaves(rs[i].Key, func(j int) string { return strconv.Itoa(j + k) })
		}
		for _, l := range ls {
			for _, r := range rs {
				rects := append(append([]Rect{}, l.Rects...), r.Rects...)
				s := pairScore(rects, g, max(l.depth, r.depth), l.height, r.height, k == 0 || k == n)
				all = append(all, Result{rects, s, "P[" + l.Key + "|" + r.Key + "]"})
			}
		}
	}
	if n == 1 {
		if p.Layflat {
			all = appendBleed(all, asp, []Rect{g.full}, g, "B[full:0]")
		}
		all = appendBleed(all, asp, []Rect{g.bleed[1]}, g, "B[right:0]")
	}
	if n == 2 {
		all = appendBleed(all, asp, []Rect{g.bleed[0], g.bleed[1]}, g, "B[pages:0,1]")
	}
	sort.SliceStable(all, func(i, j int) bool { return refLess(o, all[i], all[j]) })
	seen := map[string]bool{}
	var out []Result
	for _, r := range all {
		if !seen[r.Key] {
			seen[r.Key] = true
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	return out
}

func sameBits(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) }

func sameResult(a, b Result) bool {
	if a.Key != b.Key || !sameBits(a.Score, b.Score) || len(a.Rects) != len(b.Rects) {
		return false
	}
	for i, r := range a.Rects {
		q := b.Rects[i]
		if !sameBits(r.X, q.X) || !sameBits(r.Y, q.Y) || !sameBits(r.W, q.W) || !sameBits(r.H, q.H) {
			return false
		}
	}
	return true
}

// Случаи вне корпуса эталона: NaN в оценках от NaN, 0 и Inf в пропорциях (сортировка
// с NaN зависит от всего списка — bestBlocks не отсекает) и limit 0 (Layouts отдаёт все).
var bestCases = []struct {
	asp   []float64
	limit int
}{
	{[]float64{L, math.NaN(), P, L}, 24},
	{[]float64{P, L, math.NaN()}, 5},
	{[]float64{math.NaN(), math.NaN(), L, P, S, L, P, L, P}, 24},
	{[]float64{math.Inf(1), L, P, 0, L}, 24},
	{[]float64{L, 0, P, L, math.Inf(1), P}, 24},
	{[]float64{1e-300, L, P, 1e300, P}, 24},
	{[]float64{L, P, L, P, S}, 0},
	{[]float64{P, L, S, P, L, P, L, P, L}, 0},
}

// orders — оба порядка: tieOrder (с 0.10) и legacyOrder (миграция альбомов до 0.10).
var orders = []order{tieOrder, legacyOrder}

func TestBestBlocksSame(t *testing.T) {
	g := newGeom(album30)
	nans := 0
	for _, o := range orders {
		for ci, c := range bestCases {
			ref := refBlocks(o, c.asp, g.spread, album30.Gap, g.spine)
			nan := slices.ContainsFunc(ref, func(r candidate) bool { return math.IsNaN(r.Score) })
			if nan {
				nans++
			}
			for _, keep := range []int{-1, 1, 4, 24} {
				got, _ := bestBlocksIn(o, c.asp, g.spread, album30.Gap, g.spine, keep, false)
				want := ref
				if keep >= 0 && !nan {
					want = want[:min(len(want), keep)]
				}
				if len(got) != len(want) {
					t.Fatalf("порядок %t случай %d keep %d: %d блоков, ждали %d", o, ci, keep, len(got), len(want))
				}
				for i := range got {
					if !sameResult(got[i].Result, want[i].Result) || got[i].depth != want[i].depth || !sameBits(got[i].height, want[i].height) {
						t.Fatalf("порядок %t случай %d keep %d: блок %d %q, ждали %q", o, ci, keep, i, got[i].Key, want[i].Key)
					}
				}
			}
		}
	}
	if nans == 0 {
		t.Fatal("ни в одном случае нет NaN в оценках — ветка NaN не проверена")
	}
}

// TestLayoutsSameAsBefore — Layouts и LegacyLayouts совпадают с полным перебором в своём
// порядке и на точных ничьих (tieSets): отсев блоков не меняет выдачу.
func TestLayoutsSameAsBefore(t *testing.T) {
	cases := slices.Clone(bestCases)
	few, geoms := 8, goldenGeoms
	if testkit.Race {
		few, geoms = 5, goldenGeoms[:1]
	}
	for _, asp := range tieSets[:few] { // до 8 фото: и деревья, и ряды; больше — долго
		cases = append(cases, struct {
			asp   []float64
			limit int
		}{asp, 2})
	}
	for _, o := range orders {
		for ci, c := range cases {
			for _, p := range geoms {
				full := refLayouts(o, c.asp, p, 0) // выдача limit — его префикс
				for _, limit := range []int{c.limit, 1, 4, 24, 0} {
					want := full
					if limit > 0 {
						want = full[:min(len(full), limit)]
					}
					sameRows(t, fmt.Sprintf("порядок %t случай %d limit %d", o, ci, limit), layouts(c.asp, p, limit, o), want)
				}
			}
		}
	}
}

// sameRows — got побитово совпадает с want (what — где, для сообщения).
func sameRows(t *testing.T, what string, got, want []Result) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d раскладок, ждали %d", what, len(got), len(want))
	}
	for i := range got {
		if !sameResult(got[i], want[i]) {
			t.Fatalf("%s: ряд %d %q, ждали %q", what, i, got[i].Key, want[i].Key)
		}
	}
}

// refUnique — первые keep различных ключей полного списка ref (keep < 0 — все); в tieOrder
// и все равные последнему (см. bestBlocksIn).
func refUnique(o order, ref []candidate, keep int) []Result {
	var out []Result
	seen := map[string]bool{}
	for _, r := range ref {
		if keep >= 0 && len(out) >= keep && (o == legacyOrder || len(out) == 0 || o.q(out[len(out)-1].Score) != o.q(r.Score)) {
			break
		}
		if !seen[r.Key] {
			seen[r.Key] = true
			out = append(out, r.Result)
		}
	}
	return out
}

// bestBlocks(unique=true) — те же первые keep различных ключей, что у полного прежнего
// списка (в tieOrder — с хвостом ничьих). При NaN в оценках отдаёт все блоки без отсева.
func TestBestBlocksUniqueSame(t *testing.T) {
	g := newGeom(album30)
	tails := 0
	for _, o := range orders {
		for ci, asp := range append(slices.Clone(tieSets), bestAsps()...) {
			ref := refBlocks(o, asp, g.spread, album30.Gap, g.spine)
			nan := slices.ContainsFunc(ref, func(r candidate) bool { return math.IsNaN(r.Score) })
			for _, keep := range []int{-1, 1, 4, 24} {
				cs, _ := bestBlocksIn(o, asp, g.spread, album30.Gap, g.spine, keep, true)
				got := make([]Result, len(cs))
				for i, c := range cs {
					got[i] = c.Result
				}
				var want []Result
				if nan {
					for _, r := range ref {
						want = append(want, r.Result)
					}
				} else {
					want = refUnique(o, ref, keep)
				}
				if keep >= 0 && len(want) > keep {
					tails++
				}
				sameRows(t, fmt.Sprintf("порядок %t случай %d keep %d", o, ci, keep), got, want)
			}
		}
	}
	if tails == 0 {
		t.Fatal("ни разу не было хвоста ничьих — ветка не проверена")
	}
}

func bestAsps() [][]float64 {
	out := make([][]float64, len(bestCases))
	for i, c := range bestCases {
		out[i] = c.asp
	}
	return out
}

// tieSets — до 16 фото с точными ничьими: одинаковые пропорции и чередования дают
// симметричные раскладки с равной оценкой.
var tieSets = [][]float64{
	{L, L}, {P, P, P}, {S, S, S, S}, {L, P, L, P}, {L, L, L, L, L}, {P, L, P, L, P, L},
	{S, S, S, S, S, S, S}, {P, P, P, P, P, P, P, P}, {L, P, L, P, L, P, L, P, L, P},
	{S, S, S, S, S, S, S, S, S, S, S, S}, {L, L, L, L, L, L, L, L, L, L, L, L, L, L, L, L},
}

// ulpNoise — пропорции, сдвинутые на несколько последних битов: так FMA на arm64 сдвигает
// биты оценок. Детерминированно по seed.
func ulpNoise(asp []float64, seed uint64) []float64 {
	rng := splitmix64(seed)
	out := slices.Clone(asp)
	for i := range out {
		for range rng.next() % 5 {
			if rng.next()%2 == 0 {
				out[i] = math.Nextafter(out[i], math.Inf(1))
			} else {
				out[i] = math.Nextafter(out[i], 0)
			}
		}
	}
	return out
}

// tieRows — ключи рядов, которые видят «Авто» и лента: порядок ленты, перестановки
// и ряды страниц первого постраничного ключа.
func tieRows(asp []float64, p Params) []string {
	var keys []string
	add := func(rs []Result) {
		for _, r := range rs {
			keys = append(keys, r.Key)
		}
		keys = append(keys, "|")
	}
	ls := Layouts(asp, p, 24)
	add(ls)
	re, _ := Rearranged(asp, p, 24)
	add(re)
	for _, r := range ls {
		if IsPaged(r.Key) {
			for side := range 2 {
				in, pr, _, _ := PageChoices(r.Key, asp, p, side, 24)
				add(in)
				add(pr)
			}
			break
		}
	}
	return keys
}

// TestTiesNoise — шум в последних битах пропорций (как FMA в оценках) не меняет ни
// первую раскладку, ни порядок рядов до 16 фото: ничьи решает ключ, а не биты (решение 95).
func TestTiesNoise(t *testing.T) {
	sets := slices.Clone(tieSets)
	step, geoms := 3, goldenGeoms
	if testkit.Race {
		step, geoms = 6, goldenGeoms[:1] // чистый счёт: остальное — в обычном прогоне
	}
	for n := 1; n <= goldenMaxN; n += step {
		sets = append(sets, goldenAspects(7, n, n%3))
	}
	for _, p := range geoms {
		for si, asp := range sets {
			want := tieRows(asp, p)
			for seed := range uint64(3) {
				if got := tieRows(ulpNoise(asp, seed*100+uint64(si)), p); !slices.Equal(got, want) {
					t.Errorf("%+v набор %d (%d фото) шум %d: ряды разошлись\n%v\n%v", p, si, len(asp), seed, got, want)
					break
				}
			}
		}
	}
}
