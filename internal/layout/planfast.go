package layout

// Быстрая матрица группировки (027): оценка окна из многих фото та же, что у прежнего
// quickScore через manyGen (страницы рядами r*±1 с разрезом ⌊k/2⌋, при layflat — ряды на
// весь разворот), но без узлов, карт и копий: на 5000 фото таких окон сотни тысяч.
// Числа те же до бита: коэффициенты и раскладка повторяют compose и place операция в
// операцию, разрез — тот же breakDP, оценка — те же pairScore и score (сверка — planfast_test).

import (
	"fmt"
	"math"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// rowTree — дерево рядов над отрезком: границы рядов cuts[at : at+r+1] (смещения от
// начала отрезка); r = 0 — дерева нет (lineBreaks дал nil).
type rowTree struct{ at, r int }

// segRows — деревья lineNodes(…, 0, 1, false) над отрезками одного начала длиной m0…:
// trees[(m−m0)·3 + d+1] — ряды r*+d, d = −1, 0, 1.
type segRows struct {
	m0    int
	trees []rowTree
	cuts  []int
}

func (s *segRows) tree(m, d int) []int {
	t := s.trees[(m-s.m0)*3+d+1]
	if t.r == 0 {
		return nil
	}
	return s.cuts[t.at : t.at+t.r+1]
}

// planWork — буферы одной горутины: всё переиспользуется от окна к окну.
type planWork struct {
	pre        []float64 // префиксные суммы пропорций отрезка
	center     []int     // r* по длине отрезка
	need       []int     // до какой длины нужен DP с r рядами
	ra, rb     []float64 // h = a·w + b рядов
	single     []bool    // ряд из одного фото — лист
	comb       []Rect    // окно целиком: левая и правая страница или весь разворот
	right      [3][]Rect
	rightH     [3]float64
	rightDepth [3]int
	spread     segRows
}

func newPlanWork() *planWork {
	w := &planWork{
		pre: make([]float64, MaxPhotos+1), center: make([]int, MaxPhotos+1), need: make([]int, MaxPhotos+1),
		ra: make([]float64, MaxPhotos), rb: make([]float64, MaxPhotos), single: make([]bool, MaxPhotos),
		comb: make([]Rect, MaxPhotos),
	}
	for i := range w.right {
		w.right[i] = make([]Rect, MaxPhotos)
	}
	return w
}

// rows заполняет out деревьями рядов над [s, s+m) для m в m0…m1 в box (как lineNodes
// с d0 = 0, d1 = 1, без столбцов). DP разреза (breakDP, с тем же правилом ничьих) один
// на все m при данном r: клетки таблицы от m не зависят, m лишь ограничивает, какие читаются.
func (w *planWork) rows(asp []float64, s, m0, m1 int, box Rect, gap float64, out *segRows) {
	out.m0, out.trees, out.cuts = m0, out.trees[:0], out.cuts[:0]
	if m1 < m0 {
		return
	}
	for range 3 * (m1 - m0 + 1) {
		out.trees = append(out.trees, rowTree{})
	}
	wd, ht := box.W, box.H
	w.pre[0] = 0
	for i := range m1 {
		w.pre[i+1] = w.pre[i] + asp[s+i]
	}
	clear(w.need[:m1+1])
	for m := max(m0, 2); m <= m1; m++ {
		// lineCenter: та же сумма в том же порядке, что pre[m].
		c := int(math.Round(math.Sqrt(w.pre[m] * ht / wd)))
		w.center[m] = c
		for d := -1; d <= 1; d++ {
			if r := c + d; lineOK(r, m, wd, ht, gap) {
				w.need[r] = m
			}
		}
	}
	if m0 <= 1 && m1 >= 1 {
		// Одно фото — один лист, как в lineNodes.
		out.trees[(1-m0)*3+1] = rowTree{len(out.cuts), 1}
		out.cuts = append(out.cuts, 0, 1)
	}
	for r := 1; r <= m1; r++ {
		L := w.need[r]
		if L == 0 {
			continue
		}
		// Таблица до самой длинной нужной длины: клетки те же, что у lineBreaks на каждую m.
		t := breakDP(asp, s, s+L, box, gap, r, false)
		stride := t.m + 1
		for m := max(m0, 2, r); m <= L; m++ {
			d := r - w.center[m]
			if d < -1 || d > 1 || !lineOK(r, m, wd, ht, gap) {
				continue
			}
			at := len(out.cuts)
			for range r + 1 {
				out.cuts = append(out.cuts, 0)
			}
			cuts := out.cuts[at:]
			for k, j := r, m; k > 0; k-- {
				cuts[k] = j
				j = t.from[k*stride+j]
			}
			out.trees[(m-m0)*3+d+1] = rowTree{at, r}
		}
	}
}

// lineOK — те же условия, при которых lineBreaks не отдаёт nil.
func lineOK(r, m int, wd, ht, gap float64) bool {
	want := (ht - float64(r-1)*gap) / float64(r)
	return r >= 1 && r <= m && want > 0 && wd > 0
}

// fit — fitTree(linesTree(cuts, false), box, gap, out) без узлов: высота блока, глубина
// дерева, false — не помещается или фото < 1 мм. asp и out — от начала отрезка.
func (w *planWork) fit(asp []float64, cuts []int, box Rect, gap float64, out []Rect) (float64, int, bool) {
	r := len(cuts) - 1
	ra, rb, single := w.ra[:r], w.rb[:r], w.single[:r]
	var lb float64 // b листа
	for q := range r {
		a0, b0 := cuts[q], cuts[q+1]
		if b0-a0 == 1 {
			ra[q], rb[q], single[q] = 1/asp[a0], lb, true
			continue
		}
		// compose(true, листья): все одной высоты.
		g := gap * float64(b0-a0-1)
		var inv, sb float64
		for x := a0; x < b0; x++ {
			la := 1 / asp[x]
			inv += 1 / la
			sb += lb / la
		}
		a := 1 / inv
		ra[q], rb[q], single[q] = a, a*(sb-g), false
	}
	var ta, tb float64
	depth := 0
	if r == 1 {
		ta, tb = ra[0], rb[0]
		if !single[0] {
			depth = 1
		}
	} else {
		// compose(false, ряды): ширина общая, высоты складываются.
		g := gap * float64(r-1)
		for q := range r {
			ta += ra[q]
			tb += rb[q]
			if single[q] {
				depth = max(depth, 1)
			} else {
				depth = max(depth, 2)
			}
		}
		tb += g
	}
	// fitTreeAt(…, 0.5)
	wd := box.W
	h := ta*wd + tb
	if h > box.H {
		h = box.H
		wd = (h - tb) / ta
	}
	if wd <= 0 || h <= 0 {
		return 0, 0, false
	}
	x := box.X + (box.W-wd)*0.5
	y := box.Y + (box.H-h)/2
	if r == 1 {
		return h, depth, w.placeRow(asp, cuts, 0, x, y, wd, h, gap, out)
	}
	if wd < 1 || h < 1 {
		return h, depth, false
	}
	for q := range r {
		kh := ra[q]*wd + rb[q]
		if !w.placeRow(asp, cuts, q, x, y, wd, kh, gap, out) {
			return h, depth, false
		}
		y += kh + gap
	}
	return h, depth, true
}

// placeRow — node.place для ряда q (или листа, если в нём одно фото).
func (w *planWork) placeRow(asp []float64, cuts []int, q int, x, y, wd, h, gap float64, out []Rect) bool {
	if wd < 1 || h < 1 {
		return false
	}
	a0, b0 := cuts[q], cuts[q+1]
	if w.single[q] {
		out[a0] = Rect{x, y, wd, h}
		return true
	}
	var lb float64
	for i := a0; i < b0; i++ {
		kw := (h - lb) / (1 / asp[i])
		if kw < 1 || h < 1 {
			return false
		}
		out[i] = Rect{x, y, kw, h}
		x += kw + gap
	}
	return true
}

// window — оценка окна [i, i+k) из многих фото, как у прежнего quickScore: пары страниц
// (левая — в порядке рядов, к каждой все правые), затем ряды на весь разворот; лучшая —
// первая из наибольших. pages[s] — ряды страниц, начинающихся с s; spread — ряды
// разворота от i (nil без layflat). pages nil — страниц нет (лист 031). −1 — окно не раскладывается.
func (w *planWork) window(asp []float64, i, k int, pages []segRows, spread *segRows, g geom, gap float64) float64 {
	best, found := -1.0, false
	if pages != nil {
		best, found = w.pairs(asp, i, k, pages, g, gap)
	}
	if spread != nil {
		for d := -1; d <= 1; d++ {
			c := spread.tree(k, d)
			if c == nil {
				continue
			}
			if _, dep, ok := w.fit(asp[i:i+k], c, g.spread, gap, w.comb); ok {
				if s := score(w.comb[:k], g.spread, g.spine, dep); !found || s > best {
					best, found = s, true
				}
			}
		}
	}
	return best
}

// pairs — лучшая pairScore по парам страниц окна (manyGen.pairs); found = false — пар нет.
func (w *planWork) pairs(asp []float64, i, k int, pages []segRows, g geom, gap float64) (best float64, found bool) {
	h := k / 2
	best = -1
	var rok [3]bool
	for d := -1; d <= 1; d++ {
		rok[d+1] = false
		if c := pages[i+h].tree(k-h, d); c != nil {
			rok[d+1], w.rightH[d+1], w.rightDepth[d+1] = w.fitOK(asp[i+h:i+k], c, g.pages[1], gap, w.right[d+1])
		}
	}
	for dl := -1; dl <= 1; dl++ {
		c := pages[i].tree(h, dl)
		if c == nil {
			continue
		}
		hl, depl, ok := w.fit(asp[i:i+h], c, g.pages[0], gap, w.comb)
		if !ok {
			continue
		}
		for dr := range 3 {
			if !rok[dr] {
				continue
			}
			copy(w.comb[h:k], w.right[dr][:k-h])
			s := pairScore(w.comb[:k], g, max(depl, w.rightDepth[dr]), hl, w.rightH[dr], false)
			if !found || s > best {
				best, found = s, true
			}
		}
	}
	return best, found
}

// fitOK — fit с признаком первым (для записи в массивы правой страницы).
func (w *planWork) fitOK(asp []float64, cuts []int, box Rect, gap float64, out []Rect) (bool, float64, int) {
	h, depth, ok := w.fit(asp, cuts, box, gap, out)
	return ok, h, depth
}

// windowCols считает столбцы from…to матрицы: cols[k][i] = k · оценка окна [i, i+k).
// До maxFullTree фото — настоящая оценка (Cached), больше — window. Сначала ряды страниц
// для всех начал (правая страница окна начинается не там, где окно), затем окна по
// началам: ряды разворота от начала окна — один DP на все длины.
func windowCols(aspects []float64, p Params, from, to int) [][]float64 {
	n := len(aspects)
	out := make([][]float64, to-from+1)
	for k := from; k <= to; k++ {
		out[k-from] = make([]float64, max(0, n-k+1))
	}
	g := newGeom(p)
	kmin := max(from, maxFullTree+1)
	var pages []segRows
	if kmin <= to && !p.Sheet {
		pages = make([]segRows, n)
		planEach(n, func(w *planWork, s int) {
			w.rows(aspects, s, kmin/2, min((to+1)/2, n-s), g.pages[0], p.Gap, &pages[s])
		})
	}
	planEach(n, func(w *planWork, i int) {
		for k := from; k <= min(to, maxFullTree, n-i); k++ {
			// Вес по числу фото: качество «на фото», а не «на разворот»,
			// иначе выгодно дробить на много мелких разворотов.
			out[k-from][i] = float64(k) * smallScore(aspects[i:i+k], p)
		}
		if kmin > min(to, n-i) {
			return
		}
		var spread *segRows
		if p.Layflat {
			spread = &w.spread
			w.rows(aspects, i, kmin, min(to, n-i), g.spread, p.Gap, spread)
		}
		for k := kmin; k <= min(to, n-i); k++ {
			out[k-from][i] = float64(k) * w.window(aspects, i, k, pages, spread, g, p.Gap)
		}
	})
	return out
}

// smallScore — оценка лучшей раскладки окна до maxFullTree фото (Cached), −1 — нет раскладки.
func smallScore(asp []float64, p Params) float64 {
	if res := Cached(asp, p, 1); len(res) > 0 {
		return res[0].Score
	}
	return -1
}

// planEach зовёт f(w, 0…n−1) на planWorkers горутинах, у каждой свои буферы.
// Начала раздаются порциями: окна у начал примерно равные. Паника воркера не роняет
// процесс: остальные бросают работу, а первая паника повторяется в вызывающей горутине
// после Wait (try откатит правку, guard покажет ошибку).
func planEach(n int, f func(w *planWork, i int)) {
	const chunk = 16
	var next atomic.Int64
	var wg sync.WaitGroup
	var once sync.Once
	var fail any
	for range min(planWorkers, (n+chunk-1)/chunk) {
		wg.Go(func() {
			defer func() {
				if r := recover(); r != nil {
					once.Do(func() { fail = fmt.Sprintf("%v\n[воркер раскладки]\n%s", r, debug.Stack()) })
					next.Store(int64(n)) // остальным больше нечего брать
				}
			}()
			w := newPlanWork()
			for {
				lo := int(next.Add(chunk)) - chunk
				if lo >= n {
					return
				}
				for i := lo; i < min(lo+chunk, n); i++ {
					f(w, i)
				}
			}
		})
	}
	wg.Wait()
	if fail != nil {
		panic(fail)
	}
}
