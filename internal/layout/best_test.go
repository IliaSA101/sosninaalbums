package layout

import (
	"math"
	"slices"
	"sort"
	"strconv"
	"testing"
)

// refBlocks — прежний путь: все деревья, оценка, sort.SliceStable по убыванию.
func refBlocks(asp []float64, box Rect, gap, spine float64) []candidate {
	cs, _ := blockCandidates(asp, box, gap)
	for i := range cs {
		cs[i].Score = score(cs[i].Rects, box, spine, cs[i].depth)
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].Score > cs[j].Score })
	return cs
}

// refLayouts — Layouts до G1: блок на разворот целиком, в порядке перебора
// (страницы — через topBlocks, его сверяет TestBestBlocksSame).
func refLayouts(asp []float64, p Params, limit int) []Result {
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
		ls := topBlocks(asp[:k], g.pages[0], p.Gap, perSide)
		rs := topBlocks(asp[k:], g.pages[1], p.Gap, perSide)
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
	sort.SliceStable(all, func(i, j int) bool { return all[i].Score > all[j].Score })
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

func TestBestBlocksSame(t *testing.T) {
	g := newGeom(album30)
	nans := 0
	for ci, c := range bestCases {
		ref := refBlocks(c.asp, g.spread, album30.Gap, g.spine)
		nan := slices.ContainsFunc(ref, func(r candidate) bool { return math.IsNaN(r.Score) })
		if nan {
			nans++
		}
		for _, keep := range []int{-1, 1, 4, 24} {
			got, _ := bestBlocks(c.asp, g.spread, album30.Gap, g.spine, keep, false)
			want := ref
			if keep >= 0 && !nan {
				want = want[:min(len(want), keep)]
			}
			if len(got) != len(want) {
				t.Fatalf("случай %d keep %d: %d блоков, ждали %d", ci, keep, len(got), len(want))
			}
			for i := range got {
				if !sameResult(got[i].Result, want[i].Result) || got[i].depth != want[i].depth || !sameBits(got[i].height, want[i].height) {
					t.Fatalf("случай %d keep %d: блок %d %q, ждали %q", ci, keep, i, got[i].Key, want[i].Key)
				}
			}
		}
	}
	if nans == 0 {
		t.Fatal("ни в одном случае нет NaN в оценках — ветка NaN не проверена")
	}
}

func TestLayoutsSameAsBefore(t *testing.T) {
	for ci, c := range bestCases {
		for _, p := range goldenGeoms {
			for _, limit := range []int{c.limit, 1, 4, 24, 0} {
				got, want := Layouts(c.asp, p, limit), refLayouts(c.asp, p, limit)
				if len(got) != len(want) {
					t.Fatalf("случай %d limit %d: %d раскладок, ждали %d", ci, limit, len(got), len(want))
				}
				for i := range got {
					if !sameResult(got[i], want[i]) {
						t.Fatalf("случай %d limit %d: ряд %d %q, ждали %q", ci, limit, i, got[i].Key, want[i].Key)
					}
				}
			}
		}
	}
}

// bestBlocks(unique=true) — те же первые keep различных ключей, что topUnique над
// полным прежним списком. При NaN в оценках отдаёт все блоки без отсева (см. bestBlocks).
func TestBestBlocksUniqueSame(t *testing.T) {
	g := newGeom(album30)
	for ci, c := range bestCases {
		ref := refBlocks(c.asp, g.spread, album30.Gap, g.spine)
		nan := slices.ContainsFunc(ref, func(r candidate) bool { return math.IsNaN(r.Score) })
		for _, keep := range []int{-1, 1, 4, 24} {
			got, _ := bestBlocks(c.asp, g.spread, album30.Gap, g.spine, keep, true)
			var want []Result
			if nan {
				for _, r := range ref {
					want = append(want, r.Result)
				}
			} else {
				all := make([]Result, len(ref))
				for i, r := range ref {
					all[i] = r.Result
				}
				want = topUnique(all, max(keep, 0))
			}
			if len(got) != len(want) {
				t.Fatalf("случай %d keep %d: %d блоков, ждали %d", ci, keep, len(got), len(want))
			}
			for i := range got {
				if !sameResult(got[i].Result, want[i]) {
					t.Fatalf("случай %d keep %d: блок %d %q, ждали %q", ci, keep, i, got[i].Key, want[i].Key)
				}
			}
		}
	}
}
