package layout

// Геометрия корней ключа из спеки 051: ряды одной высоты G, лесенка L/A, лист «поменьше»
// m (place в layout.go), страницы одной высоты Q. Разбор — keyparse.go. Все размеры в мм,
// фото без обрезки, пропорции точные. Числа (kappa, шаг лесенки h/2) входят в формат v9:
// после выпуска менять только новой буквой или версией.

// kappa — фото m<i> от высоты его ряда: ячейка h × kappa·h·a, фото kappa·h по центру.
const kappa = 0.5

// fitRoot вписывает корень в box: по горизонтали на доле ax свободного места, по вертикали
// на доле ay (0 — у верхнего края). Высота блока и глубина для оценки; false — не
// помещается или фото меньше 1 мм. Дерево H/V с ax, ay = 0.5 — ровно прежний fitTree.
func (g geom) fitRoot(kt *keyTree, aspects []float64, box Rect, ax, ay float64, out []Rect) (float64, int, bool) {
	switch kt.op {
	case 'G':
		return fitRows(kt, aspects, box, g.gap, ax, ay, out)
	case 'L', 'A':
		return fitLadder(kt, aspects, box, g.gap, ax, ay, out)
	case 'Z':
		return fitScatter(kt, aspects, box, g.gap, out)
	}
	t := kt.node(aspects, g.gap)
	h, ok := fitTreeXY(t, box, g.gap, out, ax, ay)
	return h, t.depth, ok
}

// whole — корень на весь разворот: по центру, поменьше (S), вверху (U) или внизу (D).
func (g geom) whole(k parsedKey, aspects []float64, rects []Rect, key string) (Result, bool) {
	box, ay := g.spread, 0.5
	switch k.mod {
	case 'S':
		box = smallBox(box)
	case 'U':
		ay = 0
	case 'D':
		ay = 1
	}
	_, depth, ok := g.fitRoot(k.tree, aspects, box, 0.5, ay, rects)
	if !ok {
		return Result{}, false
	}
	return Result{Rects: rects, Score: score(rects, g.spread, g.spine, depth), Key: key}, true
}

// rowLeaves — фото ряда G: одно или дети H.
func rowLeaves(r *keyTree) []*keyTree {
	if r.leaf >= 0 {
		return []*keyTree{r}
	}
	return r.kids
}

// fitRows — G: у всех фото одна высота h (у V(H…,H…) высоты рядов разные). Ряд шириной
// h·Σa + gap·(len−1), ряды через gap, каждый по центру блока; h — наибольшая, при которой
// блок помещается в box. При одной пропорции всех фото — ровная сетка.
func fitRows(kt *keyTree, aspects []float64, box Rect, gap, ax, ay float64, out []Rect) (float64, int, bool) {
	m := float64(len(kt.kids))
	h := (box.H - gap*(m-1)) / m
	depth := 1
	sums := make([]float64, len(kt.kids))
	for i, r := range kt.kids {
		ls := rowLeaves(r)
		for _, l := range ls {
			sums[i] += aspects[l.leaf]
		}
		if len(ls) > 1 {
			depth = 2 // как V(H…, …)
		}
		h = min(h, (box.W-gap*float64(len(ls)-1))/sums[i])
	}
	if !(h >= 1) { // и NaN
		return 0, 0, false
	}
	bw := 0.0
	for i, r := range kt.kids {
		bw = max(bw, h*sums[i]+gap*float64(len(rowLeaves(r))-1))
	}
	bh := h*m + gap*(m-1)
	x0 := box.X + float64((box.W-bw)*ax)
	y := box.Y + float64((box.H-bh)*ay)
	for i, r := range kt.kids {
		ls := rowLeaves(r)
		x := x0 + (bw-(h*sums[i]+gap*float64(len(ls)-1)))/2
		for _, l := range ls {
			w := h * aspects[l.leaf]
			if w < 1 {
				return 0, 0, false
			}
			out[l.leaf] = Rect{x, y, w, h}
			x += w + gap
		}
		y += h + gap
	}
	return bh, depth, true
}

// fitLadder — лесенка: 3–5 фото одной высоты h в ряд через gap, каждое следующее ниже (L)
// или выше (A) на h/2. По x фото не перекрываются — пересечений нет.
func fitLadder(kt *keyTree, aspects []float64, box Rect, gap, ax, ay float64, out []Rect) (float64, int, bool) {
	k := float64(len(kt.kids))
	sum := 0.0
	for _, l := range kt.kids {
		sum += aspects[l.leaf]
	}
	steps := 1 + (k-1)/2 // высота блока в h
	h := min(box.H/steps, (box.W-gap*(k-1))/sum)
	if !(h >= 1) {
		return 0, 0, false
	}
	bw, bh := h*sum+gap*(k-1), h*steps
	x := box.X + float64((box.W-bw)*ax)
	y0 := box.Y + float64((box.H-bh)*ay)
	for i, l := range kt.kids {
		step := i
		if kt.op == 'A' {
			step = len(kt.kids) - 1 - i
		}
		w := h * aspects[l.leaf]
		if w < 1 {
			return 0, 0, false
		}
		out[l.leaf] = Rect{x, y0 + float64(step)*h/2, w, h}
		x += w + gap
	}
	return bh, 1, true
}

// pairQ — Q[l|r]: каждый корень вписан в свою страницу по центру её ширины, больший уменьшен
// до высоты меньшего; по высоте оба по центру, вверху (U) или внизу (D). Блоки на всю ширину
// страницы (ленты, ряды) сходятся у корешка сами; узкий блок у корешка оставлял пустой внешний
// край страницы (решение 0.13 №39). Оценка — как у страниц одной высоты (pairScore).
func (g geom) pairQ(k parsedKey, aspects []float64, out []Rect, key string) (Result, bool) {
	var hs [2]float64
	var ok bool
	for side, kt := range k.pages {
		if hs[side], _, ok = g.fitRoot(kt, aspects, g.pages[side], 0.5, 0.5, out); !ok {
			return Result{}, false
		}
	}
	h, depth := min(hs[0], hs[1]), 0
	ay := 0.5
	switch k.mod {
	case 'U':
		ay = 0
	case 'D':
		ay = 1
	}
	for side, kt := range k.pages {
		// В полосе высоты h блок выходит ровно h: его высота при этой ширине не меньше h.
		pg := g.pages[side]
		box := Rect{pg.X, pg.Y + float64((pg.H-h)*ay), pg.W, h}
		var d int
		if _, d, ok = g.fitRoot(kt, aspects, box, 0.5, 0.5, out); !ok {
			return Result{}, false
		}
		depth = max(depth, d)
	}
	return Result{Rects: out, Score: pairScore(out, g, depth, h, h, false), Key: key}, true
}
