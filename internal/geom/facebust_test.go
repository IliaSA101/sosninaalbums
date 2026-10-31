package geom

import (
	"math"
	"slices"
	"testing"
)

// Силуэт под лицо (спека 130, silhouette.md, решение 0.27-130-26).

// bustNear — точка path[i] равна (x, y) с точностью до 1e-3 мм.
func bustNear(t *testing.T, b Bust, i int, x, y float64) {
	t.Helper()
	p := b.Path[i]
	if math.Abs(p[0]-x) > 1e-3 || math.Abs(p[1]-y) > 1e-3 {
		t.Errorf("точка %d: %.4f, %.4f; ждали %.4f, %.4f", i, p[0], p[1], x, y)
	}
}

// bustMinMaxY — верхняя и нижняя точка контура.
func bustMinMaxY(b Bust) (lo, hi float64) {
	lo, hi = math.Inf(1), math.Inf(-1)
	for _, p := range b.Path {
		lo, hi = min(lo, p[1]), max(hi, p[1])
	}
	return lo, hi
}

// TestFaceBustShape: окно 47,5 × 63,3 мм, норма 0,35 / 0,32 — числа выписаны отдельным расчётом
// (F = 22,155; глаза 20,256; середина 23,75), форма — симметрия, макушка, низ, конечность.
func TestFaceBustShape(t *testing.T) {
	const w, h = 47.5, 63.3
	b := FaceBust(w, h, DefaultFaceNorm)
	if got := len(b.Path); got != 85 {
		t.Fatalf("точек %d, ждали 85 (голова 49, шея 2, плечи 2×16, низ 2)", got)
	}
	if math.Abs(b.EyeY-0.32*h) > 1e-9 {
		t.Errorf("линия глаз %.4f, ждали %.4f", b.EyeY, 0.32*h)
	}
	bustNear(t, b, 0, 29.5103, 31.5735)  // шея справа, конец дуги головы
	bustNear(t, b, 24, 23.75, 3.6398)    // макушка: глаза − 0,75 F
	bustNear(t, b, 48, 17.9897, 31.5735) // шея слева
	bustNear(t, b, 49, 17.9897, 36.6507) // основание шеи слева: глаза + 0,74 F
	bustNear(t, b, 57, -0.6528, 38.9868) // середина левого плеча
	bustNear(t, b, 65, -8.3747, 44.6265) // край левого плеча: ±1,45 F, +1,10 F
	bustNear(t, b, 66, -8.3747, h)       // низ окна
	bustNear(t, b, 67, 55.8747, h)
	bustNear(t, b, 68, 55.8747, 44.6265)
	bustNear(t, b, 76, 48.1528, 38.9868)
	bustNear(t, b, 84, 29.5103, 36.6507)

	// Макушка выше всего, низ — ровно низ окна; полная ширина плеч 2,9 F.
	lo, hi := bustMinMaxY(b)
	if math.Abs(lo-(b.EyeY-0.75*0.35*h)) > 1e-9 || hi != h {
		t.Errorf("контур по Y: %.4f…%.4f, ждали %.4f…%.4f", lo, hi, b.EyeY-0.75*0.35*h, h)
	}
	// Симметрия: у каждой точки есть зеркальная относительно w/2.
	for i, p := range b.Path {
		m := [2]float64{w - p[0], p[1]}
		if !slices.ContainsFunc(b.Path, func(q [2]float64) bool {
			return math.Abs(q[0]-m[0]) < 1e-9 && math.Abs(q[1]-m[1]) < 1e-9
		}) {
			t.Fatalf("точка %d %v без зеркальной", i, p)
		}
		if !finite(p[0], p[1]) {
			t.Fatalf("точка %d не число: %v", i, p)
		}
	}
	// Замыкает вызывающий: последняя точка не равна первой.
	if b.Path[0] == b.Path[len(b.Path)-1] {
		t.Error("контур замкнут точкой: замыкает вызывающий")
	}
}

// TestFaceBustNorms: линия глаз и размер идут за нормой и окном, форма — от F; окно любой
// пропорции (широкое, узкое, мелкое), норма «как это лицо» с краёв.
func TestFaceBustNorms(t *testing.T) {
	cases := []struct {
		name string
		w, h float64
		n    FaceNorm
	}{
		{"норма класса", 47.5, 63.3, FaceNorm{Size: 0.31, Eye: 0.33}},
		{"широкое окно", 100, 40, DefaultFaceNorm},
		{"узкое окно", 10, 80, DefaultFaceNorm},
		{"крупное лицо", 50, 60, FaceNorm{Size: 1, Eye: 0.5}},
		{"глаза у верха", 50, 60, FaceNorm{Size: 0.2, Eye: 0}},
		{"глаза за низом", 50, 60, FaceNorm{Size: 0.2, Eye: 1.5}},
		{"крохотное окно", 0.001, 0.001, DefaultFaceNorm},
	}
	for _, c := range cases {
		b := FaceBust(c.w, c.h, c.n)
		if len(b.Path) != 85 {
			t.Errorf("%s: точек %d", c.name, len(b.Path))
			continue
		}
		f := c.n.Size * c.h
		lo, hi := bustMinMaxY(b)
		if math.Abs(b.EyeY-c.n.Eye*c.h) > 1e-9 || math.Abs(lo-(b.EyeY-0.75*f)) > 1e-9*max(1, c.h) {
			t.Errorf("%s: глаза %.4f, макушка %.4f; F = %.4f", c.name, b.EyeY, lo, f)
		}
		// Низ не выше края плеча: линия не идёт вверх и при глазах у самого низа.
		if want := max(c.h, b.EyeY+1.10*f); math.Abs(hi-want) > 1e-9*max(1, c.h) {
			t.Errorf("%s: низ %.4f, ждали %.4f", c.name, hi, want)
		}
		// Дуга головы: точка i и 48−i — зеркальная пара относительно w/2.
		for i := 1; i < 24; i++ {
			a, z := b.Path[i], b.Path[48-i]
			if math.Abs(a[0]+z[0]-c.w) > 1e-9*max(1, c.w) || math.Abs(a[1]-z[1]) > 1e-9*max(1, c.h) {
				t.Fatalf("%s: голова несимметрична в точке %d: %v и %v", c.name, i, a, z)
			}
		}
	}
}

// TestFaceBustBad: негодный вход — пустой силуэт без паники.
func TestFaceBustBad(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	ok := DefaultFaceNorm
	cases := []struct {
		name string
		w, h float64
		n    FaceNorm
	}{
		{"w = 0", 0, 60, ok}, {"w < 0", -1, 60, ok}, {"h = 0", 50, 0, ok}, {"h < 0", 50, -1, ok},
		{"w NaN", nan, 60, ok}, {"h NaN", 50, nan, ok}, {"w Inf", inf, 60, ok}, {"h Inf", 50, inf, ok},
		{"размер 0", 50, 60, FaceNorm{Size: 0, Eye: 0.3}},
		{"размер < 0", 50, 60, FaceNorm{Size: -0.3, Eye: 0.3}},
		{"размер NaN", 50, 60, FaceNorm{Size: nan, Eye: 0.3}},
		{"глаза NaN", 50, 60, FaceNorm{Size: 0.3, Eye: nan}},
		{"глаза Inf", 50, 60, FaceNorm{Size: 0.3, Eye: inf}},
		{"размер огромный", 1e300, 1e300, FaceNorm{Size: 1e300, Eye: 0.3}},
		{"всё нули", 0, 0, FaceNorm{}},
	}
	for _, c := range cases {
		if b := FaceBust(c.w, c.h, c.n); b.Path != nil || b.EyeY != 0 {
			t.Errorf("%s: силуэт не пуст: %d точек, глаза %v", c.name, len(b.Path), b.EyeY)
		}
	}
}
