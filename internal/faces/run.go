package faces

import (
	"fmt"
	"math"
)

// memTrack считает живые байты тензоров прогона: пик — то, что держит сеть, когда тензор
// отдают сразу после последнего потребителя. Прототип держал все тензоры до конца (53 МБ на
// кадр 640×448); TestDetectPeakMem следит, чтобы утечка не вернулась.
type memTrack struct{ live, peak int }

func (m *memTrack) alloc(n int) []float32 {
	m.live += 4 * n
	m.peak = max(m.peak, m.live)
	return make([]float32, n)
}

func (m *memTrack) free(d []float32) { m.live -= 4 * len(d) }

// run гоняет сеть на входе CHW (3 канала BGR 0…255, стороны кратны 32). Вход переходит во
// владение сети и отдаётся после первого потребителя. Результат — выходы голов по именам.
func (n *net) run(in []float32, h, w, workers int, tr *memTrack) (map[string]headOut, error) {
	if h < 32 || w < 32 || h%32 != 0 || w%32 != 0 || len(in) != 3*h*w {
		return nil, fmt.Errorf("faces: вход %dx%d не кратен 32", h, w)
	}
	tr.live += 4 * len(in)
	tr.peak = max(tr.peak, tr.live)
	vals := make([]tensor, n.nvals)
	vals[n.input] = tensor{3, h, w, in}
	refs := append([]int(nil), n.uses...)
	outs := make(map[string]headOut, len(n.heads))
	for _, s := range n.steps {
		if s.op == opHead {
			x := vals[s.in[0]]
			if x.c != s.headC {
				return nil, fmt.Errorf("faces: у головы %q %d каналов, ждали %d", s.name, x.c, s.headC)
			}
			outs[s.name] = head(x, s, tr)
		} else {
			o, err := n.exec(s, vals, tr, workers)
			if err != nil {
				return nil, err
			}
			vals[s.out] = o
		}
		for _, id := range s.in[:inputs(s)] {
			if refs[id]--; refs[id] == 0 {
				tr.free(vals[id].d)
				vals[id] = tensor{}
			}
		}
	}
	return outs, nil
}

// inputs — сколько входов у шага.
func inputs(s step) int {
	if s.op == opAdd {
		return 2
	}
	return 1
}

// exec считает один шаг, кроме головы.
func (n *net) exec(s step, vals []tensor, tr *memTrack, workers int) (tensor, error) {
	x := vals[s.in[0]]
	switch s.op {
	case opConv:
		return s.conv.run(x, tr, workers)
	case opMaxPool:
		return maxPool2(x, tr), nil
	case opResize:
		return up2(x, tr), nil
	case opAdd:
		y := vals[s.in[1]]
		if x.c != y.c || x.h != y.h || x.w != y.w {
			return tensor{}, fmt.Errorf("faces: Add складывает %dx%dx%d и %dx%dx%d", x.c, x.h, x.w, y.c, y.h, y.w)
		}
		o := tensor{x.c, x.h, x.w, tr.alloc(len(x.d))}
		for i := range o.d {
			o.d[i] = x.d[i] + y.d[i]
		}
		return o, nil
	}
	return tensor{}, fmt.Errorf("faces: операция %d", s.op)
}

// maxPool2 — максимум по окнам 2×2 с шагом 2.
func maxPool2(x tensor, tr *memTrack) tensor {
	h, w := x.h/2, x.w/2
	o := tensor{x.c, h, w, tr.alloc(x.c * h * w)}
	for c := range x.c {
		for r := range h {
			a := x.d[c*x.h*x.w+2*r*x.w:]
			b := a[x.w:]
			d := o.d[c*h*w+r*w : c*h*w+(r+1)*w]
			for k := range d {
				d[k] = max(a[2*k], a[2*k+1], b[2*k], b[2*k+1])
			}
		}
	}
	return o
}

// up2 — увеличение вдвое ближайшим соседом.
func up2(x tensor, tr *memTrack) tensor {
	h, w := x.h*2, x.w*2
	o := tensor{x.c, h, w, tr.alloc(x.c * h * w)}
	for c := range x.c {
		for r := range h {
			src := x.d[c*x.h*x.w+(r/2)*x.w : c*x.h*x.w+(r/2+1)*x.w]
			d := o.d[c*h*w+r*w : c*h*w+(r+1)*w]
			for k := range d {
				d[k] = src[k/2]
			}
		}
	}
	return o
}

// head переводит CHW в [позиция][канал] (Transpose 0 2 3 1 + Reshape) и при нужде берёт сигмоиду.
func head(x tensor, s step, tr *memTrack) headOut {
	n := x.h * x.w
	o := headOut{n: n, c: x.c, d: tr.alloc(len(x.d))}
	for c := range x.c {
		src := x.d[c*n : (c+1)*n]
		for i, v := range src {
			o.d[i*x.c+c] = v
		}
	}
	if s.sigmoid {
		for i, v := range o.d {
			o.d[i] = float32(1 / (1 + math.Exp(-float64(v))))
		}
	}
	return o
}
