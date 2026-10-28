package faces

import (
	"math"
	"math/rand"
	"testing"
)

// refConv — свёртка по определению (NCHW, группы, шаг, отступ): эталон для всех путей.
func refConv(c *convSpec, x tensor) tensor {
	ho, wo := c.outSize(x.h), c.outSize(x.w)
	o := tensor{c.co, ho, wo, make([]float32, c.co*ho*wo)}
	for oc := range c.co {
		for r := range ho {
			for col := range wo {
				acc := refPixel(c, x, oc, r, col)
				if c.relu && acc < 0 {
					acc = 0
				}
				o.d[(oc*ho+r)*wo+col] = float32(acc)
			}
		}
	}
	return o
}

// refPixel — один выходной пиксель: смещение плюс сумма по ядру и каналам группы.
func refPixel(c *convSpec, x tensor, oc, r, col int) float64 {
	gi := oc / (c.co / c.group)
	var acc float64
	if c.b != nil {
		acc = float64(c.b[oc])
	}
	for ic := range c.cig {
		for ky := range c.k {
			for kx := range c.k {
				iy, ix := r*c.stride-c.pad+ky, col*c.stride-c.pad+kx
				if iy < 0 || iy >= x.h || ix < 0 || ix >= x.w {
					continue
				}
				acc += float64(c.w[((oc*c.cig+ic)*c.k+ky)*c.k+kx]) * float64(x.d[((gi*c.cig+ic)*x.h+iy)*x.w+ix])
			}
		}
	}
	return acc
}

func randTensor(rng *rand.Rand, c, h, w int) tensor {
	t := tensor{c, h, w, make([]float32, c*h*w)}
	for i := range t.d {
		t.d[i] = rng.Float32()*4 - 2
	}
	return t
}

type convCfg struct{ ci, co, k, stride, pad, group int }

// TestConvPaths: быстрые пути (1×1, поканальная 3×3) и общий путь совпадают со свёрткой по
// определению: размеры от 1×1 и 2×2 до нескольких десятков, шаг 1 и 2, группы, смещение и без,
// ReLU; число потоков результат не меняет до бита.
func TestConvPaths(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	cfgs := []convCfg{
		{5, 7, 1, 1, 0, 1},  // поточечная, остаток после блока из четырёх каналов
		{8, 8, 1, 1, 0, 1},  // ровно два блока
		{3, 3, 1, 1, 0, 1},  // меньше блока
		{6, 6, 3, 1, 1, 6},  // поканальная
		{4, 8, 3, 1, 1, 4},  // поканальная с множителем
		{3, 16, 3, 2, 1, 1}, // первая свёртка сети
		{4, 8, 3, 1, 1, 2},  // группы
		{4, 4, 3, 2, 1, 4},  // поканальная с шагом 2 — общий путь
		{3, 5, 3, 1, 0, 1},  // без отступа
		{2, 2, 5, 2, 2, 1},  // ядро 5
	}
	for _, cf := range cfgs {
		for _, hw := range [][2]int{{1, 1}, {2, 2}, {3, 2}, {7, 5}, {16, 16}, {33, 9}} {
			if hw[0]+2*cf.pad >= cf.k && hw[1]+2*cf.pad >= cf.k {
				checkConv(t, rng, cf, hw[0], hw[1])
			}
		}
	}
}

func randConv(rng *rand.Rand, cf convCfg) *convSpec {
	c := &convSpec{co: cf.co, cig: cf.ci / cf.group, k: cf.k, stride: cf.stride, pad: cf.pad, group: cf.group}
	c.w = make([]float32, c.co*c.cig*c.k*c.k)
	for i := range c.w {
		c.w[i] = rng.Float32() - 0.5
	}
	if rng.Intn(3) > 0 {
		c.b = make([]float32, c.co)
		for i := range c.b {
			c.b[i] = rng.Float32() - 0.5
		}
	}
	c.relu = rng.Intn(2) == 0
	c.prepare()
	return c
}

// checkConv сверяет один вариант с определением и один поток с четырьмя — до бита.
func checkConv(t *testing.T, rng *rand.Rand, cf convCfg, h, w int) {
	t.Helper()
	c := randConv(rng, cf)
	x := randTensor(rng, cf.ci, h, w)
	want := refConv(c, x)
	one, err := c.run(x, &memTrack{}, 1)
	if err != nil {
		t.Fatalf("%+v %dx%d: %v", cf, h, w, err)
	}
	four, err := c.run(x, &memTrack{}, 4)
	if err != nil {
		t.Fatalf("%+v %dx%d: %v", cf, h, w, err)
	}
	if one.c != want.c || one.h != want.h || one.w != want.w {
		t.Fatalf("%+v %dx%d: форма %dx%dx%d, ждали %dx%dx%d", cf, h, w, one.c, one.h, one.w, want.c, want.h, want.w)
	}
	for i := range want.d {
		if d := math.Abs(float64(one.d[i] - want.d[i])); d > 1e-4 {
			t.Fatalf("%+v %dx%d: элемент %d: %v, ждали %v", cf, h, w, i, one.d[i], want.d[i])
		}
		if math.Float32bits(one.d[i]) != math.Float32bits(four.d[i]) {
			t.Fatalf("%+v %dx%d: 4 потока дали другой бит в %d", cf, h, w, i)
		}
	}
}

func TestConvRejectsBadInput(t *testing.T) {
	c := &convSpec{co: 2, cig: 3, k: 3, stride: 1, pad: 0, group: 1, w: make([]float32, 2*3*9)}
	c.prepare()
	if _, err := c.run(tensor{2, 8, 8, make([]float32, 128)}, &memTrack{}, 1); err == nil {
		t.Error("число каналов не сверено")
	}
	if _, err := c.run(tensor{3, 2, 2, make([]float32, 12)}, &memTrack{}, 1); err == nil {
		t.Error("вход меньше ядра принят")
	}
}

func TestFloorDiv(t *testing.T) {
	for _, c := range [][3]int{{7, 2, 3}, {-1, 2, -1}, {-2, 2, -1}, {-3, 2, -2}, {0, 3, 0}, {5, 1, 5}, {-5, 1, -5}} {
		if got := floorDiv(c[0], c[1]); got != c[2] {
			t.Errorf("floorDiv(%d,%d) = %d, ждали %d", c[0], c[1], got, c[2])
		}
	}
}

// TestOpsSmall: MaxPool, Resize и Add на малых тензорах по определению.
func TestOpsSmall(t *testing.T) {
	x := tensor{2, 2, 4, []float32{1, 5, 2, 0, 3, 4, 9, 1, -1, -2, -3, -4, 0, 0, 7, 8}}
	mp := maxPool2(x, &memTrack{})
	if want := []float32{5, 9, 0, 8}; mp.c != 2 || mp.h != 1 || mp.w != 2 || !eq(mp.d, want) {
		t.Errorf("maxPool2 = %+v, ждали %v", mp, want)
	}
	up := up2(tensor{1, 1, 2, []float32{3, 4}}, &memTrack{})
	if want := []float32{3, 3, 4, 4, 3, 3, 4, 4}; up.h != 2 || up.w != 4 || !eq(up.d, want) {
		t.Errorf("up2 = %+v, ждали %v", up, want)
	}
	vals := []tensor{{1, 1, 2, []float32{1, 2}}, {1, 1, 2, []float32{10, 20}}}
	sum, err := (&net{}).exec(step{op: opAdd, in: [2]int{0, 1}}, vals, &memTrack{}, 1)
	if err != nil || !eq(sum.d, []float32{11, 22}) {
		t.Errorf("Add = %+v, %v", sum, err)
	}
	vals[1] = tensor{1, 1, 3, []float32{1, 2, 3}}
	if _, err := (&net{}).exec(step{op: opAdd, in: [2]int{0, 1}}, vals, &memTrack{}, 1); err == nil {
		t.Error("Add разных форм принят")
	}
	h := head(tensor{2, 1, 2, []float32{0, 1, 2, 3}}, step{sigmoid: true}, &memTrack{})
	// [позиция][канал]: позиция 0 — (0, 2), позиция 1 — (1, 3), затем сигмоида
	sg := func(v float64) float32 { return float32(1 / (1 + math.Exp(-v))) }
	if h.n != 2 || h.c != 2 || !eq(h.d, []float32{sg(0), sg(2), sg(1), sg(3)}) {
		t.Errorf("head = %+v", h)
	}
}

func eq(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
