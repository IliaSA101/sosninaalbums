package faces

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// convSpec — свёртка с группами; relu сливается из следующего узла Relu.
type convSpec struct {
	w, b                 []float32
	co, cig, k, stride   int
	pad, group           int
	relu                 bool
	pointwise, depthwise bool
}

// conv компилирует узел Conv: ядро квадратное, шаг 1 или 2, отступы равные, без расширения.
func (b *builder) conv(nd onnxNode) error {
	if len(nd.in) < 2 || len(nd.in) > 3 || len(nd.out) != 1 {
		return fmt.Errorf("%w: Conv", errNet)
	}
	wt := b.g.tensors[nd.in[1]]
	if wt == nil || len(wt.dims) != 4 || len(wt.f) == 0 {
		return fmt.Errorf("%w: Conv веса", errNet)
	}
	c := &convSpec{w: wt.f, co: int(wt.dims[0]), cig: int(wt.dims[1]), k: int(wt.dims[2]), stride: 1, group: 1}
	if wt.dims[2] != wt.dims[3] || c.k < 1 || c.k > 7 {
		return fmt.Errorf("%w: Conv ядро %dx%d", errNet, wt.dims[2], wt.dims[3])
	}
	if s := nd.ints["strides"]; len(s) == 2 && s[0] == s[1] && (s[0] == 1 || s[0] == 2) {
		c.stride = int(s[0])
	} else if len(s) != 0 {
		return fmt.Errorf("%w: Conv strides", errNet)
	}
	if p := nd.ints["pads"]; len(p) == 4 && p[0] == p[1] && p[1] == p[2] && p[2] == p[3] && p[0] >= 0 && p[0] < 4 {
		c.pad = int(p[0])
	} else if len(p) != 0 {
		return fmt.Errorf("%w: Conv pads", errNet)
	}
	if d := nd.ints["dilations"]; len(d) != 0 && !intsAre(d, 1, 1) {
		return fmt.Errorf("%w: Conv dilations", errNet)
	}
	if g := nd.ints["group"]; len(g) == 1 && g[0] >= 1 && g[0] <= 1024 {
		c.group = int(g[0])
	} else if len(g) != 0 {
		return fmt.Errorf("%w: Conv group", errNet)
	}
	if c.co < 1 || c.cig < 1 || c.co%c.group != 0 || len(wt.f) != c.co*c.cig*c.k*c.k {
		return fmt.Errorf("%w: Conv размеры", errNet)
	}
	if len(nd.in) == 3 {
		bt := b.g.tensors[nd.in[2]]
		if bt == nil || len(bt.f) != c.co {
			return fmt.Errorf("%w: Conv смещение", errNet)
		}
		c.b = bt.f
	}
	c.prepare()
	b.n.steps = append(b.n.steps, step{op: opConv, in: [2]int{b.id(nd.in[0])}, out: b.id(nd.out[0]), conv: c})
	return nil
}

// prepare выбирает быстрые пути: поточечная 1×1 и поканальная 3×3 с шагом 1.
func (c *convSpec) prepare() {
	c.pointwise = c.k == 1 && c.stride == 1 && c.pad == 0
	c.depthwise = c.k == 3 && c.stride == 1 && c.pad == 1 && c.cig == 1
}

// outSize — размер выхода по одной оси.
func (c *convSpec) outSize(n int) int { return (n+2*c.pad-c.k)/c.stride + 1 }

// run считает свёртку. Выходные каналы делятся между workers потоками: каждый канал считается
// одним потоком в одном порядке, поэтому результат не зависит от числа потоков.
func (c *convSpec) run(x tensor, tr *memTrack, workers int) (tensor, error) {
	if x.c != c.cig*c.group {
		return tensor{}, fmt.Errorf("faces: свёртка ждёт %d каналов, пришло %d", c.cig*c.group, x.c)
	}
	if x.h+2*c.pad < c.k || x.w+2*c.pad < c.k {
		return tensor{}, fmt.Errorf("faces: вход %dx%d меньше ядра", x.h, x.w)
	}
	ho, wo := c.outSize(x.h), c.outSize(x.w)
	o := tensor{c.co, ho, wo, tr.alloc(c.co * ho * wo)}
	parallel(workers, c.co, func(oc int) { c.plane(x, o, oc) })
	return o, nil
}

// plane считает один выходной канал.
func (c *convSpec) plane(x, o tensor, oc int) {
	ohw := o.h * o.w
	dst := o.d[oc*ohw : (oc+1)*ohw]
	bias := float32(0)
	if c.b != nil {
		bias = c.b[oc]
	}
	gi := oc / (c.co / c.group)
	switch {
	case c.depthwise && x.w >= 2:
		src := x.d[gi*x.h*x.w : (gi+1)*x.h*x.w]
		dw3x3(dst, src, c.w[oc*9:oc*9+9], bias, x.h, x.w)
	case c.pointwise:
		c.pw(dst, x, oc, gi, bias)
	default:
		c.general(dst, x, o, oc, gi, bias)
	}
	if c.relu {
		for i, v := range dst {
			if v < 0 {
				dst[i] = 0
			}
		}
	}
}

// pw — свёртка 1×1: dst = bias + Σ w·канал. Каналы берутся по четыре, чтобы dst читался и
// писался вчетверо реже.
func (c *convSpec) pw(dst []float32, x tensor, oc, gi int, bias float32) {
	hw := x.h * x.w
	for i := range dst {
		dst[i] = bias
	}
	wr := c.w[oc*c.cig : (oc+1)*c.cig]
	ch := func(ic int) []float32 { return x.d[(gi*c.cig+ic)*hw : (gi*c.cig+ic+1)*hw] }
	ic := 0
	for ; ic+4 <= c.cig; ic += 4 {
		s0, s1, s2, s3 := ch(ic), ch(ic+1), ch(ic+2), ch(ic+3)
		w0, w1, w2, w3 := wr[ic], wr[ic+1], wr[ic+2], wr[ic+3]
		for i := range dst {
			dst[i] += w0*s0[i] + w1*s1[i] + w2*s2[i] + w3*s3[i]
		}
	}
	for ; ic < c.cig; ic++ {
		s, w := ch(ic), wr[ic]
		for i := range dst {
			dst[i] += w * s[i]
		}
	}
}

// general — любая свёртка из допустимых: по ядру и строкам, с обрезкой по краям.
func (c *convSpec) general(dst []float32, x, o tensor, oc, gi int, bias float32) {
	hw := x.h * x.w
	for i := range dst {
		dst[i] = bias
	}
	k, st, pd := c.k, c.stride, c.pad
	for ic := range c.cig {
		src := x.d[(gi*c.cig+ic)*hw : (gi*c.cig+ic+1)*hw]
		for ky := range k {
			for kx := range k {
				wv := c.w[((oc*c.cig+ic)*k+ky)*k+kx]
				// столбцы выхода, для которых входной столбец лежит внутри: c*st-pd+kx в [0, x.w)
				c0 := max(0, -floorDiv(kx-pd, st))
				c1 := min(o.w, floorDiv(x.w-1+pd-kx, st)+1)
				for r := range o.h {
					iy := r*st - pd + ky
					if iy < 0 || iy >= x.h {
						continue
					}
					row := src[iy*x.w : (iy+1)*x.w]
					drow := dst[r*o.w : (r+1)*o.w]
					for cc := c0; cc < c1; cc++ {
						drow[cc] += wv * row[cc*st-pd+kx]
					}
				}
			}
		}
	}
}

// dw3x3 — поканальная свёртка 3×3, шаг 1, отступ 1: девять слагаемых на пиксель за один проход.
func dw3x3(dst, src, w []float32, bias float32, h, wd int) {
	zero := make([]float32, wd)
	for r := range h {
		a, b, c := zero, src[r*wd:(r+1)*wd], zero
		if r > 0 {
			a = src[(r-1)*wd : r*wd]
		}
		if r+1 < h {
			c = src[(r+1)*wd : (r+2)*wd]
		}
		d := dst[r*wd : (r+1)*wd]
		d[0] = bias + w[1]*a[0] + w[2]*a[1] + w[4]*b[0] + w[5]*b[1] + w[7]*c[0] + w[8]*c[1]
		for x := 1; x < wd-1; x++ {
			d[x] = bias +
				w[0]*a[x-1] + w[1]*a[x] + w[2]*a[x+1] +
				w[3]*b[x-1] + w[4]*b[x] + w[5]*b[x+1] +
				w[6]*c[x-1] + w[7]*c[x] + w[8]*c[x+1]
		}
		e := wd - 1
		d[e] = bias + w[0]*a[e-1] + w[1]*a[e] + w[3]*b[e-1] + w[4]*b[e] + w[6]*c[e-1] + w[7]*c[e]
	}
}

// parallel зовёт f(0..n-1): в один поток по порядку или по workers горутинам. Каналы независимы,
// поэтому порядок их выполнения на результат не влияет.
func parallel(workers, n int, f func(i int)) {
	if workers <= 1 || n <= 1 {
		for i := range n {
			f(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(workers, n) {
		wg.Go(func() {
			for {
				i := int(next.Add(1)) - 1
				if i >= n {
					return
				}
				f(i)
			}
		})
	}
	wg.Wait()
}

// floorDiv — деление с округлением вниз (в Go `/` округляет к нулю, а отрицательный числитель
// на краю входа дал бы лишний столбец).
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
