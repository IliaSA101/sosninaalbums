package sheetcut

import "image"

// minRefDist2 — квадрат расстояния опорного цвета от белого, ниже которого предмет «белый на белом»:
// мягкий край не строим, иначе белое крыло растворилось бы в фоне.
const minRefDist2 = 12 * 12

// softEdge строит мягкий край в полосе шириной edge px у границы фона: альфа — по расстоянию цвета
// от белого, цвет восстанавливается (un-premultiply от белого), чтобы на тёмной подложке не было
// белого ореола. Опорный цвет предмета берётся с более глубоких соседей (их цвет уже восстановлен),
// поэтому светлый предмет (серебро) не получает ложной прозрачности, как при «цвет в альфу».
func (b *bgWork) softEdge(edge int) {
	band := b.bandOrder(edge)
	var nb [4]int
	for k := len(band) - 1; k >= 0; k-- {
		i := band[k]
		n := b.neighbors(i, &nb)
		ref, ok := b.deeperColor(i, nb[:n])
		b.blendPixel(i, ref, ok)
	}
}

// bandOrder — пиксели полосы (не фон, до edge шагов от фона) в порядке возрастания глубины;
// глубина пикселя хранится только на время обхода.
func (b *bgWork) bandOrder(edge int) []int {
	dist := make([]uint8, len(b.state))
	var q []int
	var nb [4]int
	for i, s := range b.state {
		if s == stBG {
			continue
		}
		n := b.neighbors(i, &nb)
		for _, p := range nb[:n] {
			if b.state[p] == stBG {
				dist[i] = 1
				q = append(q, i)
				break
			}
		}
	}
	for k := 0; k < len(q); k++ {
		i := q[k]
		if int(dist[i]) >= edge {
			continue
		}
		n := b.neighbors(i, &nb)
		for _, p := range nb[:n] {
			if b.state[p] != stBG && dist[p] == 0 {
				dist[p] = dist[i] + 1
				q = append(q, p)
			}
		}
	}
	b.depth = dist
	return q
}

// deeperColor — средний цвет соседей глубже пикселя (глубина 0 у не попавших в полосу = самые глубокие).
func (b *bgWork) deeperColor(i int, nbs []int) (ref [3]int, ok bool) {
	var sum [3]int
	cnt := 0
	d := b.depth[i]
	for _, p := range nbs {
		if b.state[p] == stBG {
			continue
		}
		if b.depth[p] == 0 || b.depth[p] > d {
			for c := 0; c < 3; c++ {
				sum[c] += int(b.pix[p*4+c])
			}
			cnt++
		}
	}
	if cnt == 0 {
		return ref, false
	}
	return [3]int{sum[0] / cnt, sum[1] / cnt, sum[2] / cnt}, true
}

// blendPixel считает альфу и цвет пикселя полосы: C = a·F + (1−a)·белый.
func (b *bgWork) blendPixel(i int, ref [3]int, ok bool) {
	p := b.pix[i*4 : i*4+4]
	var v, wc [3]int // v — белый минус опорный цвет, wc — белый минус цвет пикселя
	var dot, len2 int
	for c := 0; c < 3; c++ {
		v[c], wc[c] = 255-ref[c], 255-int(p[c])
		dot += wc[c] * v[c]
		len2 += v[c] * v[c]
	}
	var a float64
	switch {
	case !ok:
		a = float64(max(wc[0], wc[1], wc[2])) / 255 // «цвет в альфу»: опоры нет
	case len2 < minRefDist2:
		return // белое на белом: оставить как есть, альфа 255
	default:
		a = float64(dot) / float64(len2)
	}
	switch {
	case a >= 0.98:
		return
	case a < 0.5/255:
		copy(p, []uint8{0, 0, 0, 0})
		return
	}
	unmatte(p, a)
	p[3] = uint8(a*255 + 0.5)
}

// unmatte восстанавливает цвет p (RGB) пикселя, смешанного с белым при покрытии a (0 < a ≤ 1):
// C' = (C − (1−a)·255)/a, зажато в 0..255. Общая для своего снятия фона и defringe rembg.
func unmatte(p []uint8, a float64) {
	for c := 0; c < 3; c++ {
		f := (float64(p[c]) - (1-a)*255) / a
		p[c] = uint8(min(255, max(0, f+0.5)))
	}
}

// defringe убирает светлую кайму у листа, снятого с белого фона внешним алгоритмом: у пикселей
// с 0 < альфа < 255 цвет восстанавливается от белого; альфа 0 и 255 не трогаются.
func defringe(img *image.NRGBA) {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		if a := img.Pix[i+3]; a > 0 && a < 255 {
			unmatte(img.Pix[i:i+3], float64(a)/255)
		}
	}
}
