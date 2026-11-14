package packprep

import (
	"image"
	"math"
	"runtime"
	"sync"
)

const (
	lanczosA   = 3  // окно Lanczos-3
	weightBits = 16 // веса — целые 1<<16: плоский цвет остаётся плоским без округлений
)

// kernel — Lanczos-3: sinc(x)·sinc(x/3) при |x| < 3.
func kernel(x float64) float64 {
	x = math.Abs(x)
	if x < 1e-9 {
		return 1
	}
	if x >= lanczosA {
		return 0
	}
	px := math.Pi * x
	// float64(): запрещает слияние умножения со сложением (FMA) — веса, а с ними байты, не зависят от платформы
	return float64(math.Sin(px) / px * (math.Sin(px/lanczosA) / (px / lanczosA)))
}

// taps — веса для одного выходного пикселя: первый исходный индекс и целые веса (сумма ровно 1<<weightBits).
type taps struct {
	first int
	w     []int32
}

// makeTaps считает веса сжатия/растяжения оси из src в dst px: центр выходного пикселя — в
// исходных координатах, при уменьшении окно растягивается на масштаб. Края — по зажиму индексов.
func makeTaps(src, dst int) []taps {
	scale := float64(src) / float64(dst)
	support := float64(lanczosA) * math.Max(1, scale)
	step := math.Min(1, 1/scale) // сжатие ядра при уменьшении
	out := make([]taps, dst)
	for i := range out {
		center := float64((float64(i)+0.5)*scale) - 0.5
		lo := int(math.Ceil(center - support))
		hi := int(math.Floor(center + support))
		f := make([]float64, 0, hi-lo+1)
		sum := 0.0
		for j := lo; j <= hi; j++ {
			v := kernel((float64(j) - center) * step)
			f = append(f, v)
			sum += v
		}
		out[i] = quantize(f, sum, lo, src)
	}
	return out
}

// quantize приводит веса к целым с суммой ровно 1<<weightBits и сворачивает индексы за краем в край.
func quantize(f []float64, sum float64, lo, src int) taps {
	n := len(f)
	w := make([]int32, n)
	var acc int32
	big := 0
	for k, v := range f {
		w[k] = int32(math.Round(float64(v/sum) * (1 << weightBits)))
		acc += w[k]
		if w[k] > w[big] {
			big = k
		}
	}
	w[big] += (1 << weightBits) - acc // остаток округления — на самый большой вес
	first := min(max(lo, 0), src-1)
	last := min(max(lo+n-1, 0), src-1)
	folded := make([]int32, last-first+1)
	for k, wk := range w {
		j := min(max(lo+k, 0), src-1)
		folded[j-first] += wk
	}
	return taps{first: first, w: folded}
}

// Lanczos масштабирует непрозрачную картинку до w×h (Lanczos-3, два прохода, целые веса).
// Альфа не учитывается и на выходе 255: функция для фонов. Результат детерминирован и не
// зависит от числа ядер — строки считаются независимо.
func Lanczos(src *image.NRGBA, w, h int) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if w < 1 || h < 1 || sw < 1 || sh < 1 {
		return image.NewNRGBA(image.Rect(0, 0, max(w, 0), max(h, 0)))
	}
	tx, ty := makeTaps(sw, w), makeTaps(sh, h)
	mid := image.NewNRGBA(image.Rect(0, 0, w, sh))
	parallelRows(sh, func(y int) {
		in := src.Pix[y*src.Stride : y*src.Stride+sw*4]
		out := mid.Pix[y*mid.Stride : y*mid.Stride+w*4]
		for x, t := range tx {
			var r, g, b int64
			for k, wk := range t.w {
				p := in[(t.first+k)*4:]
				r += int64(wk) * int64(p[0])
				g += int64(wk) * int64(p[1])
				b += int64(wk) * int64(p[2])
			}
			o := out[x*4:]
			o[0], o[1], o[2], o[3] = clampShift(r), clampShift(g), clampShift(b), 255
		}
	})
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	parallelRows(h, func(y int) {
		t := ty[y]
		out := dst.Pix[y*dst.Stride : y*dst.Stride+w*4]
		for x := range w {
			var r, g, b int64
			for k, wk := range t.w {
				p := mid.Pix[(t.first+k)*mid.Stride+x*4:]
				r += int64(wk) * int64(p[0])
				g += int64(wk) * int64(p[1])
				b += int64(wk) * int64(p[2])
			}
			o := out[x*4:]
			o[0], o[1], o[2], o[3] = clampShift(r), clampShift(g), clampShift(b), 255
		}
	})
	return dst
}

// clampShift — округление суммы с весами 1<<weightBits к байту.
func clampShift(v int64) uint8 {
	v = (v + 1<<(weightBits-1)) >> weightBits
	return uint8(min(max(v, 0), 255))
}

// parallelRows вызывает fn для каждой строки 0…n−1 на всех ядрах.
func parallelRows(n int, fn func(y int)) {
	workers := min(runtime.GOMAXPROCS(0), n)
	var wg sync.WaitGroup
	next := make(chan int, workers)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for y := range next {
				fn(y)
			}
		}()
	}
	for y := range n {
		next <- y
	}
	close(next)
	wg.Wait()
}
