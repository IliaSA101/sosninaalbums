package colorspace

// OKLab (Бьёрн Оттоссон, общественное достояние): перцептивно ровное пространство, в нём
// расстояние между точками близко к тому, как глаз различает цвета. Нужно «С фото» окна
// выбора цвета (спека 109): главные цвета считаются по расстоянию в OKLab, а не в RGB.

import (
	"fmt"
	"image"
	"math"
)

// Lab — цвет в OKLab: L 0..1 (светлота), A и B около -0,4..0,4.
type Lab struct{ L, A, B float32 }

// srgbLin — sRGB 0..1 → линейный свет.
func srgbLin(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// linSRGB — линейный свет → sRGB 0..1 (зажим в 0..1).
func linSRGB(c float64) float64 {
	c = math.Max(0, math.Min(1, c))
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// toLab — sRGB 0..1 (не обязательно целые 8 бит: Sample усредняет по ячейке) → OKLab.
func toLab(r, g, b float64) Lab {
	r, g, b = srgbLin(r), srgbLin(g), srgbLin(b)
	l := math.Cbrt(0.4122214708*r + 0.5363325363*g + 0.0514459929*b)
	m := math.Cbrt(0.2119034982*r + 0.6806995451*g + 0.1073969566*b)
	s := math.Cbrt(0.0883024619*r + 0.2817188376*g + 0.6299787005*b)
	return Lab{
		L: float32(0.2104542553*l + 0.7936177850*m - 0.0040720468*s),
		A: float32(1.9779984951*l - 2.4285922050*m + 0.4505937099*s),
		B: float32(0.0259040371*l + 0.7827717662*m - 0.8086757660*s),
	}
}

// ToLab переводит sRGB в OKLab.
func ToLab(r, g, b uint8) Lab {
	return toLab(float64(r)/255, float64(g)/255, float64(b)/255)
}

// Hex возвращает цвет как "#rrggbb" (sRGB; вне гаммы зажимается в 0..255).
func (c Lab) Hex() string {
	L, a, b := float64(c.L), float64(c.A), float64(c.B)
	l := L + 0.3963377774*a + 0.2158037573*b
	m := L - 0.1055613458*a - 0.0638541728*b
	s := L - 0.0894841775*a - 1.2914855480*b
	l, m, s = l*l*l, m*m*m, s*s*s
	r := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	g := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	bl := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s
	ch := func(v float64) int { return int(math.Round(linSRGB(v) * 255)) }
	return fmt.Sprintf("#%02x%02x%02x", ch(r), ch(g), ch(bl))
}

// dist2 — квадрат расстояния между цветами.
func dist2(a, b Lab) float32 {
	dl, da, db := a.L-b.L, a.A-b.A, a.B-b.B
	return dl*dl + da*da + db*db
}

// sampleStep — шаг по пикселям внутри ячейки: на миниатюре 400 px этого хватает для среднего.
const sampleStep = 2

// Sample делит картинку на сетку n×n и возвращает средний цвет каждой ячейки (n² точек, по
// строкам). Пустая картинка или n < 1 — nil. Ячейка меньше пикселя берёт ближайший пиксель.
func Sample(img image.Image, n int) []Lab {
	if img == nil || n < 1 {
		return nil
	}
	bd := img.Bounds()
	if bd.Empty() {
		return nil
	}
	out := make([]Lab, 0, n*n)
	for j := range n {
		y0, y1 := cell(bd.Min.Y, bd.Dy(), j, n)
		for i := range n {
			x0, x1 := cell(bd.Min.X, bd.Dx(), i, n)
			var sr, sg, sb, k float64
			for y := y0; y < y1; y += sampleStep {
				for x := x0; x < x1; x += sampleStep {
					r, g, b, _ := img.At(x, y).RGBA()
					sr += float64(r)
					sg += float64(g)
					sb += float64(b)
					k++
				}
			}
			const full = 65535
			out = append(out, toLab(sr/k/full, sg/k/full, sb/k/full))
		}
	}
	return out
}

// cell — границы i-й из n ячеек отрезка [min, min+size); пустая ячейка растёт до одного пикселя.
func cell(minv, size, i, n int) (from, to int) {
	from = minv + i*size/n
	to = minv + (i+1)*size/n
	if to <= from {
		to = from + 1
	}
	if to > minv+size {
		from, to = minv+size-1, minv+size
	}
	return from, to
}
