package gen

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Холст, точка и ГСЧ живут в paint; псевдонимы оставляют сигнатуры рисовальщиков короткими.
type (
	canvas = paint.Canvas
	pt     = paint.Pt
	rng    = paint.Rand
)

// maxAreaMM — сторона области узора (мм), сверх которой не рисуется ничего: обложка — до
// 2×1000 мм + корешок, а мусорный pxPerMM дал бы миллионы клеток мотива.
const maxAreaMM = 2500

// DrawPattern кладёт узор s (draw.Over, прозрачный вне мотива) в область area холста dst:
// pxPerMM — масштаб печати или превью, fade — прозрачность 0..1 (как Backdrop.Fade). Мотив
// привязан к углу area в мм. Неизвестный ключ или стикер — ничего.
func DrawPattern(dst *image.RGBA, area image.Rectangle, s Spec, pxPerMM, fade float64) {
	g, ok := find(s.Kind)
	if !ok || g.art || dst == nil || !(pxPerMM > 0) || math.IsInf(pxPerMM, 0) {
		return
	}
	w, h := float64(area.Dx())/pxPerMM, float64(area.Dy())/pxPerMM
	if w > maxAreaMM || h > maxAreaMM {
		return
	}
	m := f64.Aff3{pxPerMM, 0, float64(area.Min.X), 0, pxPerMM, float64(area.Min.Y)}
	withCanvas(dst, area, m, w, h, fade, func(c *canvas) {
		rs, _ := Resolve(s)
		g.draw(c, rs)
	})
}

// DrawArt кладёт процедурный стикер s: m переводит рамку (0..1)² в px холста — сдвиг, масштаб,
// поворот, зеркало; рисуется вектором, без пересэмплирования. Неизвестный ключ или узор — ничего.
func DrawArt(dst *image.RGBA, m f64.Aff3, s Spec, fade float64) {
	g, ok := find(s.Kind)
	if !ok || !g.art || dst == nil {
		return
	}
	for _, v := range m {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return
		}
	}
	if m[0]*m[4]-m[1]*m[3] == 0 {
		return
	}
	withCanvas(dst, dst.Rect, m, 1, 1, fade, func(c *canvas) {
		rs, _ := Resolve(s)
		g.draw(c, rs)
	})
}

// withCanvas зовёт fn на холсте области. При прозрачности рисунок идёт непрозрачным в свой слой и
// ложится одним проходом с альфой 1 − fade — как превью /media/gen с opacity на холсте UI:
// гаси каждая фигура отдельно, перекрытия темнели бы и печать расходилась с экраном.
func withCanvas(dst *image.RGBA, area image.Rectangle, m f64.Aff3, w, h, fade float64, fn func(c *canvas)) {
	c := paint.New(dst, area, m, w, h, fade)
	if c == nil {
		return
	}
	if c.Alpha >= 1 {
		fn(c)
		return
	}
	a := c.Alpha
	layer := image.NewRGBA(c.Clip) // те же координаты, что у холста: m не меняется
	c.Dst, c.Alpha = layer, 1
	fn(c)
	paint.OverAlpha(dst, layer, a)
}

// inks — цвета Spec после Resolve.
func inks(s Spec) []color.NRGBA {
	out := make([]color.NRGBA, len(s.Inks))
	for i, v := range s.Inks {
		out[i] = paint.RGB(v)
	}
	return out
}
