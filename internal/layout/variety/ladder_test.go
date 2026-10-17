package variety

// 055 «Лесенка честно и чаще»: место в ряду «Авто», вторая лесенка другого направления, 4 фото на
// разворот, чередование L/A в выборе и лесенка вне ритма воздуха.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

type ladderSet struct {
	name string
	as   []float64
}

// ladderSets — 22 набора 3–5 фото: одинаковые V и H, смеси, окна stock (разбор A §3).
func ladderSets(t *testing.T) []ladderSet {
	st := testkit.StockAspects(t)
	var out []ladderSet
	for _, k := range []int{3, 4, 5} {
		out = append(out, ladderSet{fmt.Sprintf("V%d", k), sameAspects(k, aspP)}, ladderSet{fmt.Sprintf("H%d", k), sameAspects(k, aspL)})
	}
	out = append(out,
		ladderSet{"VHV", []float64{aspP, aspL, aspP}},
		ladderSet{"HVH", []float64{aspL, aspP, aspL}},
		ladderSet{"VVHV", []float64{aspP, aspP, aspL, aspP}},
		ladderSet{"HHVH", []float64{aspL, aspL, aspP, aspL}},
		ladderSet{"VHVHV", []float64{aspP, aspL, aspP, aspL, aspP}},
		ladderSet{"смесь4", mixedAspects(4, 4)},
		ladderSet{"смесь5", mixedAspects(5, 5)})
	for _, k := range []int{3, 4, 5} {
		for s := 0; s+k <= len(st); s += 37 {
			out = append(out, ladderSet{fmt.Sprintf("stock%d@%d", k, s), st[s : s+k]})
		}
	}
	return out
}

// ladderAt — позиции лесенок в ряду.
func ladderAt(row []forms.Cand) []int {
	var at []int
	for i, c := range row {
		if c.Form == forms.Ladder {
			at = append(at, i)
		}
	}
	return at
}

// TestRowLadderNotFirst: на 3–5 фото × 3 геометрии лесенка не на месте 0 и 1 ряда (не первая форма:
// пустая страница и пустота углов теперь в Q), есть в ≥ 85 % рядов; где две — направления разные.
func TestRowLadderNotFirst(t *testing.T) {
	ResetCache()
	rows, with := 0, 0
	for _, g := range []struct {
		name string
		p    layout.Params
	}{{"30x30", geomSq}, {"30x20", geomWide}, {"без layflat", geomFlat}} {
		for _, s := range ladderSets(t) {
			row := Row(s.as, g.p, rowLen(len(s.as)))
			at := ladderAt(row)
			rows++
			t.Logf("%s %-10s %v", g.name, s.name, at)
			if len(at) == 0 {
				continue
			}
			with++
			if at[0] < 2 {
				t.Errorf("%s %s: лесенка %q на месте %d", g.name, s.name, row[at[0]].Key, at[0])
			}
			if len(at) > 2 {
				t.Errorf("%s %s: лесенок %d", g.name, s.name, len(at))
			}
			if len(at) == 2 && forms.LadderDir(row[at[0]].Key) == forms.LadderDir(row[at[1]].Key) {
				t.Errorf("%s %s: обе лесенки одного направления: %q, %q", g.name, s.name, row[at[0]].Key, row[at[1]].Key)
			}
		}
	}
	if with*100 < rows*85 {
		t.Errorf("лесенка в %d из %d рядов", with, rows)
	}
	t.Logf("лесенка в %d из %d рядов", with, rows)
}

// TestLadderSpreadFour (страж BACKLOG, разборы A и B): 4 вертикальных на разворот 30×30 и 30×20 —
// в Gen обе лесенки на разворот, мельчайшая сторона не ниже пола 0.18, корешок ни одно фото не
// пересекает; в ряду есть лесенка на разворот, первая лесенка на месте 3…9, вторая — другого
// направления. 3 и 5 вертикальных на разворот 30×30 — нет (среднее фото на сгибе).
func TestLadderSpreadFour(t *testing.T) {
	ResetCache()
	as := sameAspects(4, aspP)
	for _, g := range []struct {
		name string
		p    layout.Params
	}{{"30x30", geomSq}, {"30x20", geomWide}} {
		row := Row(as, g.p, rowLen(len(as)))
		gen := map[string]forms.Cand{}
		for _, c := range forms.Gen(as, g.p) {
			gen[c.Key] = c
		}
		for _, key := range []string{"L(0,1,2,3)", "A(0,1,2,3)"} {
			c, ok := gen[key]
			if !ok || c.Form != forms.Ladder {
				t.Errorf("%s: в Gen нет лесенки %s", g.name, key)
				continue
			}
			checkSpreadLadder(t, g.name, c, g.p, row[0].Q)
		}
		at := ladderAt(row)
		if len(at) != 2 {
			t.Fatalf("%s: лесенок в ряду %d, ждали 2", g.name, len(at))
		}
		if at[0] < 3 || at[0] > 9 {
			t.Errorf("%s: первая лесенка на месте %d", g.name, at[0])
		}
		if forms.LadderDir(row[at[0]].Key) == forms.LadderDir(row[at[1]].Key) {
			t.Errorf("%s: лесенки одного направления: %q, %q", g.name, row[at[0]].Key, row[at[1]].Key)
		}
		if !slices.ContainsFunc(at, func(i int) bool { return !layout.IsPaged(row[i].Key) }) {
			t.Errorf("%s: в ряду нет лесенки на разворот: %q, %q", g.name, row[at[0]].Key, row[at[1]].Key)
		}
	}
	for _, k := range []int{3, 5} {
		for _, c := range forms.Gen(sameAspects(k, aspP), geomSq) {
			if c.Form == forms.Ladder && !layout.IsPaged(c.Key) {
				t.Errorf("%d вертикальных: лесенка на разворот %q (фото на сгибе)", k, c.Key)
			}
		}
	}
}

// checkSpreadLadder — лесенка на разворот: стороны не ниже пола 0.18, корешок не пересекает, Q в
// допуске ряда от Q лучшей q0.
func checkSpreadLadder(t *testing.T, name string, c forms.Cand, p layout.Params, q0 float64) {
	t.Helper()
	for _, r := range c.Rects {
		if min(r.W, r.H) < 0.18*p.PageH {
			t.Errorf("%s %s: сторона %.1f мм ниже пола", name, c.Key, min(r.W, r.H))
		}
		if r.X < p.PageW-0.01 && r.X+r.W > p.PageW+0.01 {
			t.Errorf("%s %s: фото %+v пересекает корешок", name, c.Key, r)
		}
	}
	if quanta(c.Q) < quanta(q0-rowQSlack) {
		t.Errorf("%s %s: Q %.3f ниже допуска ряда (Q[0] %.3f)", name, c.Key, c.Q, q0)
	}
}

// ladderCand — синтетическая лесенка направления dir: вид у каждого разворота свой, чтобы выбор
// решали направление и ключ, а не повтор вида.
func ladderCand(dir string, n int) Cand {
	c := cand(dir+"(0,1,2)", fmt.Sprint(dir, n), 1.0, true)
	c.Formed, c.Form, c.Q, c.Family = true, forms.Ladder, 0.9, Figure
	return c
}

// TestPickLadderAlternates: из равных L и A выбор чередует направление — без прошлой лесенки L
// (по ходу чтения), дальше A, L…; Ctx.Ladder продолжает чередование цепочки перед собой.
func TestPickLadderAlternates(t *testing.T) {
	chain := func(n int) []Spread {
		out := make([]Spread, n)
		for i := range out {
			out[i] = Spread{Pool: []Cand{ladderCand("A", i), ladderCand("L", i)}, Star: -1}
		}
		return out
	}
	dirs := func(c Ctx, lv Level) string {
		in := chain(4)
		var b []byte
		for i, k := range Pick(in, geomSq, lv, c) {
			b = append(b, forms.LadderDir(in[i].Pool[k].Key))
		}
		return string(b)
	}
	for _, c := range []struct {
		name string
		ctx  Ctx
		lv   Level
		want string
	}{
		{"без прошлой — сначала L", Ctx{}, Normal, "LALA"},
		{"«Больше» — так же", Ctx{}, More, "LALA"},
		{"перед цепочкой L — сначала A", Ctx{Ladder: 'L'}, Normal, "ALAL"},
		{"перед цепочкой A — сначала L", Ctx{Ladder: 'A'}, Normal, "LALA"},
		{"«Меньше» — лучшая", Ctx{Ladder: 'L'}, Less, "AAAA"},
	} {
		if got := dirs(c.ctx, c.lv); got != c.want {
			t.Errorf("%s: %s, ждали %s", c.name, got, c.want)
		}
	}
	// не лесенка направления не имеет и чередование не сдвигает
	mixed := []Spread{{Pool: []Cand{cand("H(0,1,2)", "h", 1.0, true)}, Star: -1}, chain(1)[0]}
	if got := Pick(mixed, geomSq, Normal, Ctx{}); mixed[1].Pool[got[1]].Key != "L(0,1,2)" {
		t.Errorf("после не-лесенки выбрана %q", mixed[1].Pool[got[1]].Key)
	}
}

// TestPickLadderNotAir: настоящая лесенка (пустая страница, заполнение < airFill) — семейство
// Figure, а не воздух: «воздух не по ритму» её не штрафует, счёт разворотов без воздуха она не
// обнуляет. Без лесенки заполнение то же — воздух (страж того, что дело в форме).
func TestPickLadderNotAir(t *testing.T) {
	as := sameAspects(3, aspP)
	r, ok := layout.Build("P[-|A(0,1,2)]", as, geomSq)
	if !ok {
		t.Fatal("лесенка не строится")
	}
	in := Describe(r, geomSq)
	if in.Form != forms.Ladder || in.Family != Figure || in.Fill >= airFill {
		t.Fatalf("лесенка: форма %s, семейство %s, заполнение %.2f", in.Form, in.Family, in.Fill)
	}
	x := Cand{Result: r, Info: in, InRow: true, Formed: true}
	n := neighbors{}
	air := x
	air.Family = Air
	if d := n.penalty(&air) - n.penalty(&x); d != penAirOffBeat {
		t.Errorf("разница штрафа лесенки и воздуха %.2f, ждали %.2f", d, penAirOffBeat)
	}
	st := newChain(Ctx{Since: 3})
	st.push(&x)
	if st.since != 4 || st.ladder != 'A' {
		t.Errorf("после лесенки since %d (ждали 4), направление %q", st.since, st.ladder)
	}
	st.push(&air)
	if st.since != 0 {
		t.Errorf("после воздуха since %d", st.since)
	}
}
