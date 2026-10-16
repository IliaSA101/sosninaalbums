package layout

// Геометрия по ключу: области разворота, оценка страниц, Build. Грамматика ключа,
// разбор и преобразования записи — keyparse.go, корни 051 (G, L/A, Q) — shapes.go.

import (
	"math"
)

// geom — области разворота при заданных Params.
type geom struct {
	spread Rect    // весь разворот без полей
	pages  [2]Rect // левая и правая страница без полей
	full   Rect    // весь разворот в край
	bleed  [2]Rect // страницы в край
	spine  float64
	gap    float64
}

func newGeom(p Params) geom {
	left := Rect{p.Margin, p.Margin, p.PageW - 2*p.Margin, p.PageH - 2*p.Margin}
	right := left
	right.X += p.PageW
	spine := p.PageW
	if p.Sheet {
		spine = 0 // у листа корешка нет: score без штрафа за сгиб
	}
	return geom{
		spread: Rect{p.Margin, p.Margin, 2*p.PageW - 2*p.Margin, p.PageH - 2*p.Margin},
		pages:  [2]Rect{left, right},
		full:   Rect{0, 0, 2 * p.PageW, p.PageH},
		bleed:  [2]Rect{{0, 0, p.PageW, p.PageH}, {p.PageW, 0, p.PageW, p.PageH}},
		spine:  spine,
		gap:    p.Gap,
	}
}

// fitTree вписывает дерево в box по центру; false — не помещается или фото < 1 мм.
func fitTree(t *node, box Rect, gap float64, out []Rect) (float64, bool) {
	return fitTreeAt(t, box, gap, out, 0.5)
}

// fitTreeAt — то же, но по горизонтали блок стоит на доле ax свободного места:
// 0 — у левого края box, 1 — у правого.
func fitTreeAt(t *node, box Rect, gap float64, out []Rect, ax float64) (float64, bool) {
	return fitTreeXY(t, box, gap, out, ax, 0.5)
}

// fitTreeXY — то же, и по вертикали блок стоит на доле ay: 0 — у верхнего края box.
// При ay = 0.5 числа те же, что у прежнего «/2»: умножение на 0.5 точное, а явное
// приведение не даёт сливать его со сложением (FMA на arm64).
func fitTreeXY(t *node, box Rect, gap float64, out []Rect, ax, ay float64) (float64, bool) {
	w := box.W
	h := t.a*w + t.b
	if h > box.H {
		h = box.H
		w = (h - t.b) / t.a
	}
	if w <= 0 || h <= 0 {
		return 0, false
	}
	x := box.X + (box.W-w)*ax
	y := box.Y + float64((box.H-h)*ay)
	return h, t.place(x, y, w, h, gap, out)
}

const (
	maxCrop = 0.3 // обрезка «в край», не больше
	// smallScale — блок «поменьше»: вписывается в область страницы, уменьшенную
	// до этой доли по каждой стороне; одно фото выходит ровно такой доли от вписанного.
	smallScale = 0.65
)

// pageGeo — то, что от страницы нужно оценке разворота.
type pageGeo struct {
	h     float64 // высота блока (у страницы в край — как у блока во всю высоту)
	depth int
	bleed bool
	crop  float64
}

// cropOf — доля кадра, срезанная, чтобы фото с пропорцией a заполнило r.
func cropOf(r Rect, a float64) float64 {
	ra := r.W / r.H
	return 1 - math.Min(ra, a)/math.Max(ra, a)
}

// bleedPenalty — штраф раскладки «в край»: сама обрезка и уход от полей.
func bleedPenalty(crop float64) float64 { return 0.1 + 1.5*crop }

// page раскладывает страницу side ключа с модификатором mod в out.
// false — фото меньше 1 мм или обрезка «в край» больше maxCrop.
func (g geom) page(side int, mod byte, kt *keyTree, aspects []float64, out []Rect) (pageGeo, bool) {
	if mod == 'E' {
		r := g.bleed[side]
		c := cropOf(r, aspects[kt.leaf])
		if c > maxCrop {
			return pageGeo{}, false
		}
		out[kt.leaf] = r
		return pageGeo{h: g.spread.H, bleed: true, crop: c}, true
	}
	box, ax, ay := g.pages[side], 0.5, 0.5
	switch mod {
	case 'S':
		box = smallBox(box)
	case 'O': // внешний край: у левой страницы слева, у правой справа
		ax = float64(side)
	case 'I':
		ax = float64(1 - side)
	case 'U':
		ay = 0
	case 'D':
		ay = 1
	}
	h, depth, ok := g.fitRoot(kt, aspects, box, ax, ay, out)
	return pageGeo{h: h, depth: depth}, ok
}

// smallBox — область «поменьше»: доля smallScale по каждой стороне, по центру.
func smallBox(box Rect) Rect {
	d := 1 - smallScale
	return Rect{box.X + box.W*d/2, box.Y + box.H*d/2, box.W * smallScale, box.H * smallScale}
}

// pagedScore — оценка постраничного разворота с модификаторами страниц.
// Без модификаторов «в край» совпадает с pairScore.
func pagedScore(rects []Rect, g geom, pg [2]pageGeo, empty bool) float64 {
	s := pairScore(rects, g, max(pg[0].depth, pg[1].depth), pg[0].h, pg[1].h, empty)
	if pg[0].bleed || pg[1].bleed {
		s -= bleedPenalty(math.Max(pg[0].crop, pg[1].crop))
	}
	return s
}

// pairScore — оценка постраничного разворота: та же формула, что в Layouts.
func pairScore(rects []Rect, g geom, depth int, hl, hr float64, empty bool) float64 {
	s := score(rects, g.spread, g.spine, depth)
	if !empty {
		// Блоки на двух страницах лучше смотрятся на одной высоте.
		s -= 0.3 * math.Abs(hl-hr) / g.spread.H
	} else {
		s -= 0.05 // пустая страница
	}
	return s
}

// bleedResult — фото в край: rects[i] — ячейка фото i. false — обрезка больше 30 %.
func bleedResult(rects []Rect, aspects []float64, g geom, key string) (Result, bool) {
	crop := 0.0
	for i, r := range rects {
		crop = math.Max(crop, cropOf(r, aspects[i]))
	}
	if crop > maxCrop {
		return Result{}, false
	}
	s := score(rects, g.spread, g.spine, 0) - bleedPenalty(crop)
	return Result{rects, s, key}, true
}

// Build строит раскладку по ключу. false: ключ не разобрать или он не канонический,
// листья не 0…n−1 по разу, фото меньше 1 мм, без layflat блок на весь разворот
// или фото через корешок, у листа (Sheet) страницы и Q, обрезка «в край» больше 30 %.
// Score — та же формула, что в Layouts; у Q — как у страниц одной высоты (pairScore).
// U/D без заметного сдвига строятся (вид как по центру, как у O/I): порог minShift —
// дело генераторов, иначе смена формата молча сбрасывала бы выбранную раскладку.
func Build(key string, aspects []float64, p Params) (Result, bool) {
	n := len(aspects)
	if n == 0 || n > MaxPhotos || p.PageW <= 0 || p.PageH <= 0 {
		return Result{}, false
	}
	for _, a := range aspects {
		if !(a > 0) || math.IsInf(a, 0) {
			return Result{}, false
		}
	}
	k, ok := parseKey(key)
	if !ok || !leavesOK(k.leaves(), n) || p.Sheet && (IsPaged(key) || k.kind == 'Q') {
		return Result{}, false
	}
	// Одна раскладка — один ключ (у страниц «в край» без обычной второй страницы — B[…]).
	if k.String() != key {
		return Result{}, false
	}
	g := newGeom(p)
	rects := make([]Rect, n)
	switch k.kind {
	case 'T':
		if !p.Layflat {
			return Result{}, false
		}
		return g.whole(k, aspects, rects, key)
	case 'Q':
		return g.pairQ(k, aspects, rects, key)
	case 'P':
		var pg [2]pageGeo
		empty := false
		for side, kt := range k.pages {
			if kt == nil {
				empty = true
				continue
			}
			if pg[side], ok = g.page(side, k.mods[side], kt, aspects, rects); !ok {
				return Result{}, false
			}
		}
		return Result{rects, pagedScore(rects, g, pg, empty), key}, true
	}
	switch {
	case k.bleed == "full" && p.Layflat:
		rects[k.idx[0]] = g.full
	case k.bleed == "right":
		rects[k.idx[0]] = g.bleed[1]
	case k.bleed == "pages":
		rects[k.idx[0]] = g.bleed[0]
		rects[k.idx[1]] = g.bleed[1]
	default:
		return Result{}, false
	}
	return bleedResult(rects, aspects, g, key)
}
