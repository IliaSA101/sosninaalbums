package layout

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Положения «Фото через корешок» (владелец 2026-10-17): нет · только широкие · любые фото.
var (
	foldNone = Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3}
	foldWide = Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}
	foldAny  = Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}
)

// across — фото w×h, у которого слева от корешка доля left ширины.
func across(p Params, w, h, left float64) Rect {
	return Rect{X: p.PageW - left*w, Y: 10, W: w, H: h}
}

// TestFoldOK — правило сгиба: «владелец 2026-10-17» (через корешок только кадры шире высоты
// в 1,2 раза и не меньше 20 % на каждой странице; «любые» — всё; «нет» — ничего).
func TestFoldOK(t *testing.T) {
	sheet := foldWide
	sheet.Sheet = true
	tall := across(foldWide, 120, 180, 0.5)
	cases := []struct {
		name string
		x    Rect
		a    float64
		want [3]bool // нет, широкие, любые
	}{
		{"слева от корешка", Rect{20, 20, 200, 100}, 0, [3]bool{true, true, true}},
		{"справа вплотную", Rect{300, 20, 200, 100}, 0, [3]bool{true, true, true}},
		{"касание < 0,01 мм", Rect{300 - 200 + 0.005, 20, 200, 300 - 20}, 0, [3]bool{true, true, true}},
		{"заход 0,02 мм", Rect{300 - 0.02, 20, 30, 20}, 0, [3]bool{false, false, true}},
		{"вертикальное по центру", tall, 0, [3]bool{false, false, true}},
		{"рамка 1,19", across(foldWide, 119, 100, 0.5), 0, [3]bool{false, false, true}},
		{"рамка 1,2", across(foldWide, 120, 100, 0.5), 0, [3]bool{false, true, true}},
		{"рамка 2, кадр 1,19", across(foldWide, 200, 100, 0.5), 1.19, [3]bool{false, false, true}},
		{"рамка 2, кадр 1,2", across(foldWide, 200, 100, 0.5), 1.2, [3]bool{false, true, true}},
		{"рамка 1,19, кадр 3", across(foldWide, 119, 100, 0.5), 3, [3]bool{false, false, true}},
		{"доля 19 %", across(foldWide, 300, 100, 0.19), 0, [3]bool{false, false, true}},
		{"доля 20 %", across(foldWide, 300, 100, 0.2), 0, [3]bool{false, true, true}},
		{"доля справа 19 %", across(foldWide, 300, 100, 0.81), 0, [3]bool{false, false, true}},
		{"доля справа 20 %", across(foldWide, 300, 100, 0.8), 0, [3]bool{false, true, true}},
	}
	for _, c := range cases {
		for i, p := range []Params{foldNone, foldWide, foldAny} {
			if got := FoldOK1(c.x, c.a, p); got != c.want[i] {
				t.Errorf("%s, положение %d: FoldOK1 = %v, ждали %v", c.name, i, got, c.want[i])
			}
			if got := FoldOK([]Rect{c.x}, []float64{c.a}, p); got != c.want[i] {
				t.Errorf("%s, положение %d: FoldOK = %v, ждали %v", c.name, i, got, c.want[i])
			}
		}
		// У листа корешка нет: проходит всё.
		if !FoldOK1(c.x, c.a, sheet) {
			t.Errorf("%s: лист не пропустил", c.name)
		}
	}

	// «В край» на весь разворот 40×30: рамка 4:3 проходит по рамке, квадратный кадр — нет (№13).
	v := Params{PageW: 200, PageH: 300, Layflat: true}
	full := []Rect{{0, 0, 400, 300}}
	if !FoldOK(full, nil, v) {
		t.Error("B[full:0] 40×30 по рамке: не прошло")
	}
	if FoldOK(full, []float64{1}, v) {
		t.Error("B[full:0] 1:1 на 40×30: кадр уже 1,2, а прошло")
	}
	if !FoldOK(full, []float64{1.5}, v) {
		t.Error("B[full:0] 3:2 на 40×30: не прошло")
	}

	// Несколько фото: одно узкое на сгибе портит всю раскладку; пустая — проходит.
	ok := Rect{20, 20, 100, 100}
	if FoldOK([]Rect{ok, tall, ok}, nil, foldWide) || !FoldOK([]Rect{ok, tall, ok}, nil, foldAny) {
		t.Error("три фото с вертикальным на сгибе: правило не по всем фото")
	}
	if !FoldOK([]Rect{ok, across(foldWide, 240, 100, 0.5)}, nil, foldWide) {
		t.Error("два фото, широкое на сгибе: не прошло")
	}
	if !FoldOK(nil, nil, foldWide) || !FoldOK([]Rect{}, nil, foldNone) {
		t.Error("пустая раскладка не прошла")
	}
}

// TestCacheKeyFold — FoldAny различает записи кэша только там, где действует.
func TestCacheKeyFold(t *testing.T) {
	asp := []float64{1.5, 2.0 / 3}
	if CacheKey(asp, foldWide, 2) == CacheKey(asp, foldAny, 2) {
		t.Error("широкие и любые делят запись кэша")
	}
	none := foldNone
	none.FoldAny = true
	if CacheKey(asp, foldNone, 2) != CacheKey(asp, none, 2) {
		t.Error("без layflat FoldAny разделил записи")
	}
	sheet, sheetAny := foldWide, foldAny
	sheet.Sheet, sheetAny.Sheet = true, true
	if CacheKey(asp, sheet, 2) != CacheKey(asp, sheetAny, 2) {
		t.Error("у листа FoldAny разделил записи")
	}
}

// foldRows — все ряды пакета, которые видит подбор: «Авто» (Cached, на многих — весь ManyCap),
// «С перестановкой», ряды страниц первых постраничных ключей (и «в край»).
func foldRows(asp []float64, p Params) map[string][]Result {
	out := map[string][]Result{"auto": Cached(asp, p, 24)}
	if len(asp) >= ManyFrom {
		out["auto48"] = Cached(asp, p, ManyCap)
	}
	out["rearranged"], _ = CachedRearranged(asp, p, 24)
	n := 0
	for _, r := range out["auto"] {
		if !IsPaged(r.Key) {
			continue
		}
		if n++; n > 4 {
			break
		}
		for side := range 2 {
			in, re, _, ok := CachedPages(r.Key, asp, p, side, 24)
			if ok {
				out[fmt.Sprintf("%s/%d", r.Key, side)] = in
				out[fmt.Sprintf("%s/%d/re", r.Key, side)] = re
			}
		}
	}
	return out
}

// TestFoldRuleAllPaths — сторож правила сгиба в пакете layout: «владелец 2026-10-17» (правило
// одно для всех путей подбора). Корпус 1–64 фото: при «только широкие» каждый результат
// Cached, CachedRearranged, CachedPages проходит FoldOK, при «нет» ни одно фото не
// пересекает корешок. Чтобы сторож не был пустым, при «любые» на том же корпусе есть
// результаты вне правила — их и отсёк фильтр. Формы, «Варианты», featured и план — свои
// сторожа в forms/variety/plan (064 G2).
func TestFoldRuleAllPaths(t *testing.T) {
	// Квадрат 30×30, книжная 20×30 с промежутком 0 (разворот 4:3 — «в край» по кадру), 40×30.
	geos := []foldGeo{newFoldGeo(300, 300, 12, 3), newFoldGeo(200, 300, 5, 0), newFoldGeo(400, 300, 12, 3)}
	step := 1
	if testkit.Lite() {
		step = 5
	}
	var outside atomic.Int64 // результатов «любые» вне правила «широкие»
	var wg sync.WaitGroup
	for gi, g := range geos {
		for n := 1; n <= MaxPhotos; n += step {
			sets := []int{n % 3, 4} // 4 — все вертикальные (случай владельца)
			if n <= 4 {
				sets = []int{0, 1, 2, 4}
			}
			for _, set := range sets {
				wg.Go(func() {
					name := fmt.Sprintf("геометрия %d, n=%d, набор %d", gi, n, set)
					outside.Add(g.check(t, name, manyAspects(uint64(gi)+1, n, set)))
				})
			}
		}
	}
	wg.Wait()
	if outside.Load() == 0 {
		t.Error("при «любые» нет ни одного результата вне правила: сторож ничего не проверяет")
	}
}

// foldGeo — одна геометрия в трёх положениях «Фото через корешок».
type foldGeo struct{ wide, none, all Params }

func newFoldGeo(w, h, m, g float64) foldGeo {
	p := Params{PageW: w, PageH: h, Margin: m, Gap: g, Layflat: true}
	none, all := p, p
	none.Layflat, all.FoldAny = false, true
	return foldGeo{p, none, all}
}

// check — ряды asp при «широкие» проходят FoldOK и «Авто» не пуст, при «нет» корешок не
// пересечён; возвращает, сколько результатов «любые» правило «широкие» не пропустило бы.
func (g foldGeo) check(t *testing.T, name string, asp []float64) int64 {
	for row, rs := range foldRows(asp, g.wide) {
		if row == "auto" && len(rs) == 0 {
			t.Errorf("%s: ряд «Авто» пуст", name)
		}
		for _, r := range rs {
			if !FoldOK(r.Rects, asp, g.wide) {
				t.Errorf("%s, %s: %s — фото на сгибе вне правила", name, row, r.Key)
			}
		}
	}
	for row, rs := range foldRows(asp, g.none) {
		for _, r := range rs {
			if !FoldOK(r.Rects, nil, g.none) {
				t.Errorf("%s, %s: %s — фото через корешок без layflat", name, row, r.Key)
			}
		}
	}
	var out int64
	for _, rs := range foldRows(asp, g.all) {
		for _, r := range rs {
			if !FoldOK(r.Rects, asp, g.wide) {
				out++
			}
		}
	}
	return out
}

// TestLayoutsFoldRule — три вертикальных 2:3 на 30×30, «владелец 2026-10-17»: при «только
// широкие» в ряду нет ни ряда H(0,1,2) (до 0.15 — «Авто» №1), ни столбца V(0,1,2); при
// «любые» оба на месте, ряд прежний. «В край» на весь разворот 40×30 — по кадру (№13).
func TestLayoutsFoldRule(t *testing.T) {
	ppp := []float64{P, P, P}
	all := rowKeys(Layouts(ppp, album30, 24))
	wide := Layouts(ppp, album30wide, 24)
	if r := Layouts(ppp, album30, 1); len(r) == 0 || r[0].Key != "H(0,1,2)" {
		t.Errorf("«любые»: «Авто» %v, ждали H(0,1,2) — как до 0.15", r)
	}
	for _, k := range []string{"H(0,1,2)", "V(0,1,2)"} {
		if _, ok := all[k]; !ok {
			t.Errorf("«любые»: %s нет в ряду", k)
		}
		if _, ok := rowKeys(wide)[k]; ok {
			t.Errorf("«только широкие»: %s в ряду", k)
		}
	}
	if len(wide) == 0 || !IsPaged(wide[0].Key) {
		t.Errorf("«только широкие»: «Авто» %v, ждали постраничную", wide[:min(1, len(wide))])
	}
	for _, r := range wide {
		if !FoldOK(r.Rects, ppp, album30wide) {
			t.Errorf("«только широкие»: %s вне правила", r.Key)
		}
	}

	// B[full:0] на развороте 40×30 (рамка 4:3): квадратный кадр — центр на сгибе.
	v, vAny := Params{PageW: 200, PageH: 300, Margin: 5, Layflat: true}, Params{PageW: 200, PageH: 300, Margin: 5, Layflat: true, FoldAny: true}
	full := func(a float64, p Params) bool {
		_, ok := rowKeys(Layouts([]float64{a}, p, 24))["B[full:0]"]
		return ok
	}
	if !full(1, vAny) || full(1, v) || !full(1.5, v) {
		t.Errorf("B[full:0] 40×30: 1:1 «любые» %v, 1:1 «широкие» %v, 3:2 «широкие» %v; ждали true, false, true",
			full(1, vAny), full(1, v), full(1.5, v))
	}
	// На 60×30 рамка 2:1: «в край» берёт кадры от 1,4 (обрезка ≤ 30 %) — правило проходят.
	if !full(1.5, album30wide) || full(1, album30) {
		t.Error("B[full:0] на 60×30: 3:2 нет в ряду «широкие» или 1:1 построен с обрезкой 50 %")
	}
}

// TestFoldAnyLadderOnSpread — лесенка L(0,1,2) из трёх вертикальных на весь разворот:
// «владелец 2026-10-17» — при «любые» она допустима, при «только широкие» — нет. Build правило
// не проверяет: стоящий ключ (свёрстанное) строится в обоих положениях (decisions 0.15 №1).
// Сама лесенка в ряду «Вариантов» — сторож variety (064 G2).
func TestFoldAnyLadderOnSpread(t *testing.T) {
	ppp := []float64{P, P, P}
	for _, key := range []string{"L(0,1,2)", "H(0,1,2)", "V(0,1,2)"} {
		a, okA := Build(key, ppp, album30)
		w, okW := Build(key, ppp, album30wide)
		if !okA || !okW {
			t.Fatalf("%s: Build «любые» %v, «широкие» %v — стоящий ключ обязан строиться", key, okA, okW)
		}
		if !slices.Equal(a.Rects, w.Rects) || a.Score != w.Score {
			t.Errorf("%s: FoldAny поменял Build", key)
		}
		if !FoldOK(a.Rects, ppp, album30) || FoldOK(w.Rects, ppp, album30wide) {
			t.Errorf("%s: FoldOK «любые» %v, «широкие» %v; ждали true, false", key,
				FoldOK(a.Rects, ppp, album30), FoldOK(w.Rects, ppp, album30wide))
		}
	}
}
