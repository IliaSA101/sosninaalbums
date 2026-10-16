package spreads

// Спека 041, доправка: внутренности выбора (pickRun, ctxOf), которые снаружи не добыть —
// выбор, совпавший по виду с текущим, и опустевшие соседи.

import (
	"cmp"
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// autoSpread — разворот «Авто» из n горизонтальных фото в проекте p.
func autoSpread(p *model.Project, name string, n int) *model.Spread {
	s := &model.Spread{ID: name}
	for i := range n {
		id := fmt.Sprint(name, i)
		p.Photos[id] = model.PhotoRef{W: 300, H: 200}
		s.Photos = append(s.Photos, id)
	}
	return s
}

// TestPickRunSameLookKeepsKey: выбранный вид совпал с текущим — ключ не пишется (иначе K → ""
// без видимой перемены даёт rev, шаг отмены и тост «Раскладки изменены»); другой вид — пишется.
// Цепочка из двух разворотов: у первого вид тот же, у второго другой.
func TestPickRunSameLookKeepsKey(t *testing.T) {
	p := model.New()
	a, b := autoSpread(p, "a", 3), autoSpread(p, "b", 3)
	p.Spreads = []*model.Spread{a, b}
	prm := p.Params()
	row := layout.Cached(p.Aspects(a.Photos), prm, model.RowLen(3))
	best, other := row[0], row[1]
	a.Layout, b.Layout = other.Key, other.Key
	curLook := variety.Describe(other, prm).Look
	same := variety.Cand{Result: best, Info: variety.Describe(best, prm), InRow: true}
	same.Look = curLook // тот же вид, что у текущего K, но ключ — лучшая ("")
	diff := same
	diff.Look = "иной вид"
	pickRun(p, 0, 1, variety.Normal, nil, true, [][]variety.Cand{{same}, {diff}}, nil)
	if a.Layout != other.Key {
		t.Fatalf("тот же вид: ключ %q, был %q", a.Layout, other.Key)
	}
	if b.Layout != "" {
		t.Fatalf("другой вид: ключ %q, ждали лучшую (\"\")", b.Layout)
	}
	// сборка (cur = false) пишет как прежде: у собранных текущего вида ещё нет
	a.Layout = other.Key
	pickRun(p, 0, 0, variety.Normal, nil, false, [][]variety.Cand{{same}}, nil)
	if a.Layout != "" {
		t.Fatalf("без «Разнообразить» ключ %q, ждали лучшую", a.Layout)
	}
}

// TestShownNoView: у разворота, которому нечего показать (нет фото, сетка, виньетка, фото сверх
// предела раскладки), вида нет — такой сосед ритм прерывает.
func TestShownNoView(t *testing.T) {
	p := model.New()
	grid := autoSpread(p, "g", 3)
	grid.Grid = []*model.Place{{ID: "pl", W: 50, H: 50}}
	vig := autoSpread(p, "v", 3)
	vig.Kind = model.KindVignette
	for name, s := range map[string]*model.Spread{
		"пустой": {ID: "e"}, "сетка": grid, "виньетка": vig, "сверх предела": autoSpread(p, "x", model.MaxPhotos+6),
	} {
		if in, ok := shown(p, s); ok || in.Look != "" {
			t.Errorf("%s: вид %q, ok=%v", name, in.Look, ok)
		}
	}
	if _, ok := shown(p, autoSpread(p, "ok", 3)); !ok {
		t.Error("обычный разворот «Авто» без вида")
	}
}

// TestCtxOfSkipsDead: опустевшие правкой развороты (dead) — не соседи ни слева, ни справа и не
// рвут счёт разворотов без воздуха; без dead они видны как «не Авто».
func TestCtxOfSkipsDead(t *testing.T) {
	p := model.New()
	a, b, n, f := autoSpread(p, "a", 3), autoSpread(p, "b", 4), autoSpread(p, "n", 3), autoSpread(p, "f", 5)
	e1, e2 := &model.Spread{ID: "e1"}, &model.Spread{ID: "e2"}
	p.Spreads = []*model.Spread{a, b, e1, n, e2, f}
	dead := func(s *model.Spread) bool { return s == e1 || s == e2 }
	ia, _ := shown(p, a)
	ib, _ := shown(p, b)
	if_, _ := shown(p, f)
	if ia.Family == variety.Air || ib.Family == variety.Air {
		t.Fatal("заготовка: воздух у соседей, счёт Since не проверить")
	}

	got := ctxOf(p, 3, 3, dead)
	if !slices.Equal(got.Prev, []string{ia.Look, ib.Look}) || got.Family != ib.Family || got.Next != if_.Look || got.Since != 2 {
		t.Fatalf("с dead: %+v, ждали Prev [a b], Next f, Since 2", got)
	}
	// те же развороты без dead: пустой сосед «не Авто» — вида нет, ритм прерван
	raw := ctxOf(p, 3, 3, nil)
	if !slices.Equal(raw.Prev, []string{ib.Look, ""}) || raw.Next != "" || raw.Since != 0 {
		t.Fatalf("без dead: %+v", raw)
	}
	// цепочка в начале: слева живых нет — Prev пуст, Since 0
	if first := ctxOf(p, 0, 0, dead); len(first.Prev) != 0 || first.Since != 0 {
		t.Fatalf("начало: %+v", first)
	}
	// все слева мертвы — как начало
	if all := ctxOf(p, 3, 3, func(s *model.Spread) bool { return s == a || s == b || s == e1 }); len(all.Prev) != 0 || all.Since != 0 {
		t.Fatalf("все слева мертвы: %+v", all)
	}
}

// TestCtxOfForms: Ctx.Forms — формы не больше FormsWindow «Авто»-соседей перед цепочкой, последняя —
// ближняя; сетка или пустой сосед счёт форм обрывает, а у начала альбома форм нет.
func TestCtxOfForms(t *testing.T) {
	p := model.New()
	for i := range 9 {
		p.Spreads = append(p.Spreads, autoSpread(p, fmt.Sprint("s", i), 2+i%5*3))
	}
	p.Spreads = append(p.Spreads, autoSpread(p, "target", 4))
	at := len(p.Spreads) - 1
	want := func(from, to int) []forms.Form { // формы соседей from..to-1, ближний последним
		var out []forms.Form
		for _, s := range p.Spreads[from:to] {
			in, ok := shown(p, s)
			if !ok {
				t.Fatalf("заготовка: у %s нет вида", s.ID)
			}
			out = append(out, in.Form)
		}
		return out
	}
	all := want(at-variety.FormsWindow, at)
	if len(slices.Compact(slices.Clone(all))) < 2 {
		t.Fatalf("заготовка: формы соседей одинаковы %v, порядок не проверить", all)
	}
	if got := ctxOf(p, at, at, nil).Forms; !slices.Equal(got, all) {
		t.Fatalf("окно: %v, ждали %v (последние %d, ближняя последней)", got, all, variety.FormsWindow)
	}
	grid := p.Spreads[at-3]
	grid.Grid = []*model.Place{{ID: "pl", W: 50, H: 50}}
	if got, w := ctxOf(p, at, at, nil).Forms, want(at-2, at); !slices.Equal(got, w) {
		t.Fatalf("сетка рвёт счёт: %v, ждали %v", got, w)
	}
	if got := ctxOf(p, 0, 0, nil).Forms; len(got) != 0 {
		t.Fatalf("начало альбома: формы %v", got)
	}
	if got := ctxOf(p, 2, 2, nil).Forms; len(got) != 2 {
		t.Fatalf("два соседа слева: формы %v", got)
	}
}

// neighborCand — кандидат пула из настоящей раскладки r с заданным видом и оценкой: «Плотно»,
// без главного (семейство одно на всех — штраф семейства соседа выбор не решает).
func neighborCand(r layout.Result, prm layout.Params, look string, score float64) variety.Cand {
	c := variety.Cand{Result: r, Info: variety.Describe(r, prm), InRow: true}
	c.Look, c.Score, c.Fair, c.Family, c.Big, c.Heroes = look, score, score, variety.Dense, -1, 0
	return c
}

// TestVaryNeighborKey (ревью 041 G3): «Разнообразить» одного разворота, у соседа слева тот же
// ключ, что у лучшей, при другом виде, — ключ меняется на равную другую схему (Ctx.PrevKey); у
// соседа другой ключ — лучшая. Сосед-сетка ключа не даёт.
func TestVaryNeighborKey(t *testing.T) {
	p := model.New()
	a, b := autoSpread(p, "a", 3), autoSpread(p, "b", 3)
	p.Spreads = []*model.Spread{a, b}
	prm := p.Params()
	row := layout.Cached(p.Aspects(b.Photos), prm, model.RowLen(3))
	k0, k1 := row[0].Key, row[1].Key
	for _, c := range []struct {
		name, prev, want string
	}{
		{"у соседа ключ лучшей", "", k1},
		{"у соседа другой ключ", k1, ""},
	} {
		a.Layout, b.Layout = c.prev, ""
		pool := []variety.Cand{neighborCand(row[0], prm, "X", 1.0), neighborCand(row[1], prm, "Y", 0.99)}
		pickRun(p, 1, 1, variety.Normal, nil, true, [][]variety.Cand{pool}, nil)
		if b.Layout != c.want {
			t.Errorf("%s: ключ %q, ждали %q", c.name, b.Layout, c.want)
		}
		if got := ctxOf(p, 1, 1, nil).PrevKey; got != cmp.Or(c.prev, k0) {
			t.Errorf("%s: PrevKey %q", c.name, got)
		}
	}
	a.Grid = []*model.Place{{ID: "pl", W: 50, H: 50}}
	if got := ctxOf(p, 1, 1, nil).PrevKey; got != "" {
		t.Errorf("сосед-сетка: PrevKey %q", got)
	}
}

// TestVaryAfterHeroNeighbor (ревью 041 G3): раскладка с главным фото «даром» (сырая потеря ≤ 0.02)
// не ставится сразу после соседа с главным (Ctx.Hero), после соседа без главного — ставится.
func TestVaryAfterHeroNeighbor(t *testing.T) {
	p := model.New()
	a, b := autoSpread(p, "a", 3), autoSpread(p, "b", 3)
	p.Spreads = []*model.Spread{a, b}
	prm := p.Params()
	row := layout.Cached(p.Aspects(b.Photos), prm, model.RowLen(3))
	for _, c := range []struct {
		name, prev string
		hero       bool
		want       string
	}{
		{"сосед с главным", "H(0,V(1,2))", true, ""},
		{"сосед без главного", "H(0,1,2)", false, row[1].Key},
	} {
		a.Layout, b.Layout = c.prev, ""
		ia, _ := shown(p, a)
		if (ia.Heroes > 0) != c.hero {
			t.Fatalf("%s: заготовка — главных у соседа %d", c.name, ia.Heroes)
		}
		// лучшая повторяет вид соседа (штраф), раскладка с главным — «даром»: на 0.01 хуже
		best := neighborCand(row[0], prm, ia.Look, 1.0)
		feat := neighborCand(row[1], prm, "F", 0.99)
		feat.InRow, feat.Family, feat.Big, feat.Heroes, feat.Kind = false, variety.Hero, 0, 1, variety.KindPage
		pickRun(p, 1, 1, variety.Normal, nil, true, [][]variety.Cand{{best, feat}}, nil)
		if b.Layout != c.want {
			t.Errorf("%s: ключ %q, ждали %q", c.name, b.Layout, c.want)
		}
		if got := ctxOf(p, 1, 1, nil).Hero; got != c.hero {
			t.Errorf("%s: Ctx.Hero %v", c.name, got)
		}
	}
}

// TestAvgPhotos: среднее фото считается по живым разворотам «Авто» с фото; сетка, пустой и убранный
// правкой (dead) не входят; таких нет вовсе — 0, а не деление на ноль.
func TestAvgPhotos(t *testing.T) {
	p := model.New()
	a, b, c := autoSpread(p, "a", 3), autoSpread(p, "b", 4), autoSpread(p, "c", 8)
	g := autoSpread(p, "g", 6)
	g.Grid = []*model.Place{{}}
	e := &model.Spread{ID: "e"}
	p.Spreads = []*model.Spread{a, b, c, g, e}
	if got := avgPhotos(p, func(s *model.Spread) bool { return s != c }); got != 3.5 {
		t.Fatalf("без c: %v, ждали 3.5 (сетка и пустой не считаются)", got)
	}
	if got := avgPhotos(p, func(*model.Spread) bool { return true }); got != 5 {
		t.Fatalf("все живые: %v, ждали 5", got)
	}
	if got := avgPhotos(p, func(s *model.Spread) bool { return s == g || s == e }); got != 0 {
		t.Fatalf("только сетка и пустой: %v, ждали 0", got)
	}
	if got := avgPhotos(model.New(), func(*model.Spread) bool { return true }); got != 0 {
		t.Fatalf("пустой проект: %v, ждали 0", got)
	}
}
