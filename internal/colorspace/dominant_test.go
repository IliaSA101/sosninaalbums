package colorspace

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// hexLab — "#rrggbb" в OKLab.
func hexLab(t testing.TB, h string) Lab {
	t.Helper()
	var r, g, b uint8
	if _, err := fmt.Sscanf(h, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("не цвет %q: %v", h, err)
	}
	return ToLab(r, g, b)
}

func deltaE(a, b Lab) float64 { return math.Sqrt(float64(dist2(a, b))) }

// repeat — n одинаковых точек.
func repeat(c Lab, n int) []Lab {
	out := make([]Lab, n)
	for i := range out {
		out[i] = c
	}
	return out
}

// Все 4096 цветов сетки 16³ (с краями 0 и 255) проходят круг sRGB → OKLab → sRGB без потерь.
func TestLabRoundTrip(t *testing.T) {
	steps := []uint8{0, 17, 34, 51, 68, 85, 102, 119, 136, 153, 170, 187, 204, 221, 238, 255}
	n := 0
	for _, r := range steps {
		for _, g := range steps {
			for _, b := range steps {
				want := fmt.Sprintf("#%02x%02x%02x", r, g, b)
				if got := ToLab(r, g, b).Hex(); got != want {
					t.Fatalf("%s → %s", want, got)
				}
				n++
			}
		}
	}
	if n != 4096 {
		t.Fatalf("сетка из %d цветов", n)
	}
	// Вне гаммы Hex зажимает, а не падает и не уходит за 00..ff.
	if got := (Lab{L: 1.5, A: 0.5, B: -0.5}).Hex(); len(got) != 7 {
		t.Errorf("вне гаммы: %q", got)
	}
	if got := (Lab{L: -1}).Hex(); got != "#000000" {
		t.Errorf("отрицательная светлота: %q", got)
	}
}

func TestDominantEmpty(t *testing.T) {
	for _, pts := range [][]Lab{nil, {}} {
		got := Dominant(pts, 8)
		if got == nil || len(got) != 0 {
			t.Errorf("пустой ввод: %#v, ждали пустой не-nil", got)
		}
	}
	if got := Dominant(repeat(ToLab(1, 2, 3), 10), 0); got == nil || len(got) != 0 {
		t.Errorf("limit 0: %#v", got)
	}
	if got := Dominant(repeat(ToLab(1, 2, 3), 10), -3); len(got) != 0 {
		t.Errorf("limit < 0: %#v", got)
	}
}

// Один цвет (в том числе из одной точки) — один ответ, тот же цвет.
func TestDominantSolid(t *testing.T) {
	for _, n := range []int{1, 2, 15, 16, 17, 1024} {
		got := Dominant(repeat(ToLab(200, 120, 90), n), 8)
		if !slices.Equal(got, []string{"#c8785a"}) {
			t.Errorf("%d точек: %v", n, got)
		}
	}
}

// Красная и синяя половины — оба цвета, по 50 %, расстояние до исходных < 0,02.
func TestDominantTwoHalves(t *testing.T) {
	red, blue := ToLab(220, 20, 20), ToLab(20, 40, 220)
	pts := append(repeat(red, 500), repeat(blue, 500)...)
	got := Dominant(pts, 8)
	if len(got) != 2 {
		t.Fatalf("ждали 2 цвета, получили %v", got)
	}
	for _, want := range []Lab{red, blue} {
		ok := slices.ContainsFunc(got, func(h string) bool { return deltaE(hexLab(t, h), want) < 0.02 })
		if !ok {
			t.Errorf("нет цвета рядом с %v среди %v", want, got)
		}
	}
	// Больший кластер — первым.
	if got := Dominant(append(repeat(red, 700), repeat(blue, 300)...), 8); deltaE(hexLab(t, got[0]), red) > 0.02 {
		t.Errorf("первым должен идти красный (70 %%): %v", got)
	}
}

// pseudoPts — детерминированные случайные точки в кубе RGB.
func pseudoPts(n int, seed uint64) []Lab {
	rnd := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	pts := make([]Lab, n)
	for i := range pts {
		pts[i] = ToLab(uint8(rnd.IntN(256)), uint8(rnd.IntN(256)), uint8(rnd.IntN(256)))
	}
	return pts
}

func TestDominantDeterministic(t *testing.T) {
	pts := pseudoPts(2000, 7)
	first := Dominant(pts, 8)
	for range 3 {
		if again := Dominant(pts, 8); !slices.Equal(first, again) {
			t.Fatalf("ответ меняется: %v / %v", first, again)
		}
	}
	// Входной срез не портится.
	cp := slices.Clone(pts)
	Dominant(pts, 8)
	if !slices.Equal(cp, pts) {
		t.Error("Dominant изменил входные точки")
	}
}

// Двадцать разных цветов по 5 %: не больше 8, все различимы (ΔE ≥ порога, с запасом на округление
// до 8 бит), а max меньше 8 урезает ещё.
func TestDominantAtMost8Distinct(t *testing.T) {
	var pts []Lab
	for i := range 20 {
		h := float64(i) / 20
		r, g, b := hsv(h, 0.8, 0.9)
		pts = append(pts, repeat(ToLab(r, g, b), 50)...)
	}
	got := Dominant(pts, 8)
	if len(got) == 0 || len(got) > 8 {
		t.Fatalf("ждали 1..8 цветов, получили %d: %v", len(got), got)
	}
	for i, a := range got {
		for _, b := range got[i+1:] {
			if d := deltaE(hexLab(t, a), hexLab(t, b)); d < minDeltaE-0.005 {
				t.Errorf("%s и %s различимы слабо: ΔE %.3f", a, b, d)
			}
		}
	}
	if three := Dominant(pts, 3); len(three) != 3 || !slices.Equal(three, got[:3]) {
		t.Errorf("limit 3: %v, ждали начало %v", three, got)
	}
	// Похожие оттенки одного цвета (ΔE < 0,09) схлопываются в один.
	var near []Lab
	for i := range 10 {
		near = append(near, repeat(ToLab(uint8(100+i), 100, 100), 100)...)
	}
	if got := Dominant(near, 8); len(got) != 1 {
		t.Errorf("близкие оттенки: %v", got)
	}
}

// hsv — h, s, v в 0..1 → sRGB 8 бит.
func hsv(h, s, v float64) (r, g, b uint8) {
	i := int(h * 6)
	f := h*6 - float64(i)
	p, q, tt := v*(1-s), v*(1-f*s), v*(1-(1-f)*s)
	var rf, gf, bf float64
	switch i % 6 {
	case 0:
		rf, gf, bf = v, tt, p
	case 1:
		rf, gf, bf = q, v, p
	case 2:
		rf, gf, bf = p, v, tt
	case 3:
		rf, gf, bf = p, q, v
	case 4:
		rf, gf, bf = tt, p, v
	default:
		rf, gf, bf = v, p, q
	}
	return uint8(math.Round(rf * 255)), uint8(math.Round(gf * 255)), uint8(math.Round(bf * 255))
}

// Цвет меньше 1 % точек в ответ не попадает, ровно 1 % и больше — попадает.
func TestDominantDropsUnder1Percent(t *testing.T) {
	grey, red := ToLab(128, 128, 128), ToLab(230, 20, 20)
	cases := []struct {
		red  int
		want int
	}{{5, 1}, {9, 1}, {10, 2}, {20, 2}}
	for _, c := range cases {
		pts := append(repeat(grey, 1000-c.red), repeat(red, c.red)...)
		if got := Dominant(pts, 8); len(got) != c.want {
			t.Errorf("красных %d из 1000: %v, ждали %d цвет(а)", c.red, got, c.want)
		}
	}
}

func TestDominantCost(t *testing.T) {
	pts := pseudoPts(65536, 11)
	d := testkit.Fastest(3, func() { Dominant(pts, 8) })
	t.Logf("65 536 точек: %v", d)
	if lim := testkit.Limit(60 * time.Millisecond); d > lim {
		t.Errorf("65 536 точек: %v > %v", d, lim)
	}
}

// Sample: сетка n×n по строкам, средние по ячейкам; края — пустая картинка, ячейка меньше пикселя,
// картинка не с нуля.
func TestSampleGrid(t *testing.T) {
	// Левая половина красная, правая синяя.
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 64 {
			if x < 32 {
				img.Set(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.Set(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	pts := Sample(img, 4)
	if len(pts) != 16 {
		t.Fatalf("точек %d, ждали 16", len(pts))
	}
	for i, p := range pts {
		want := "#ff0000"
		if i%4 >= 2 {
			want = "#0000ff"
		}
		if got := p.Hex(); got != want {
			t.Errorf("ячейка %d: %s, ждали %s", i, got, want)
		}
	}
	// Ячейка меньше пикселя: 2×2 → сетка 5×5 всё равно 25 точек без деления на ноль.
	small := image.NewRGBA(image.Rect(0, 0, 2, 2))
	small.Set(1, 1, color.RGBA{G: 255, A: 255})
	if got := Sample(small, 5); len(got) != 25 {
		t.Errorf("мелкая картинка: %d точек", len(got))
	}
	// Картинка не с нуля (SubImage) читается по своим границам.
	sub := img.SubImage(image.Rect(32, 0, 64, 32))
	for _, p := range Sample(sub, 3) {
		if p.Hex() != "#0000ff" {
			t.Fatalf("подкартинка: %s", p.Hex())
		}
	}
	// Пустое.
	if Sample(nil, 4) != nil || Sample(img, 0) != nil || Sample(image.NewRGBA(image.Rect(0, 0, 0, 0)), 4) != nil {
		t.Error("пустой ввод должен давать nil")
	}
}
