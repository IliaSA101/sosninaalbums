package gen

import (
	"hash/fnv"
	"math"
	"strconv"
)

// rng — splitmix64: свой ГСЧ, а не math/rand — его поток может смениться с версией Go, а
// выпущенный узор не должен меняться (решение №90).
type rng struct{ s uint64 }

func (r *rng) next() uint64 {
	r.s += 0x9e3779b97f4a7c15
	z := r.s
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// float — [0, 1) с 53 битами.
func (r *rng) float() float64 { return float64(r.next()>>11) / (1 << 53) }

// rangeF — [a, b).
func (r *rng) rangeF(a, b float64) float64 { return a + (b-a)*r.float() }

// intn — [0, n), n > 0.
func (r *rng) intn(n int) int { return min(int(r.float()*float64(n)), n-1) }

// chance — истина с вероятностью p.
func (r *rng) chance(p float64) bool { return r.float() < p }

// cellRand — ГСЧ клетки (ix, iy) узора kind: мотив клетки зависит только от сида, ключа и
// номера клетки — не от размера области, dpi и того, какие клетки видны.
func cellRand(seed uint32, kind string, ix, iy int) rng {
	h := fnv.New64a()
	_, _ = h.Write([]byte(kind + "|" + strconv.Itoa(ix) + "," + strconv.Itoa(iy)))
	r := rng{s: h.Sum64() ^ uint64(seed)<<32}
	r.next() // FNV близких клеток различается в младших битах — перемешать
	return r
}

// cells — клетки шага step (мм), задевающие область w×h мм с полем margin вокруг.
func cells(w, h, step, margin float64, fn func(ix, iy int)) {
	x0, x1 := int(math.Floor(-margin/step)), int(math.Floor((w+margin)/step))
	y0, y1 := int(math.Floor(-margin/step)), int(math.Floor((h+margin)/step))
	for iy := y0; iy <= y1; iy++ {
		for ix := x0; ix <= x1; ix++ {
			fn(ix, iy)
		}
	}
}
