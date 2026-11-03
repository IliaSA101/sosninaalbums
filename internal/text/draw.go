package text

import (
	"image"
	"math"
	"unicode/utf8"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Draw рисует блок b на dst: (x0, y0) — левый верхний угол блока в мм от
// верхнего левого угла dst, deg — поворот блока вокруг этого угла по часовой
// (NaN/Inf — без поворота), dpi — разрешение холста. Строки берутся из Layout
// с тем же стилем, а глифы ставятся тем же обходом walkGlyphs, что в measure, так что
// на бумаге строки той же ширины и с теми же переносами, что на экране.
// opentype.Face сюда не годится: его Kern почти нулевой (design §5).
func Draw(dst *image.RGBA, b Box, st Style, x0, y0, deg float64, dpi int) {
	st = st.Clamped()
	if c := usableCustom(st.Font); c != nil {
		sf, _, d := fontData(st)
		fb, _ := fallback(st)
		if guarded(func() { drawWith(dst, b, st, x0, y0, deg, dpi, sf, fb, d) }) {
			return
		}
		// Уже нарисованные глифы остаются: после Inspect (все глифы и пробная
		// отрисовка) такого не ждём, главное — не уронить выгрузку.
		c.markBroken("отрисовка")
	}
	sf, _ := font(st)
	fb, _ := fallback(st)
	drawWith(dst, b, st, x0, y0, deg, dpi, sf, fb, nil)
}

// drawWith — Draw заданным шрифтом sf и запасным fb (может быть nil); st уже Clamped.
// d — данные своего начертания: знаки ставятся по Line.DX (161), nil — встроенный шрифт.
func drawWith(dst *image.RGBA, b Box, st Style, x0, y0, deg float64, dpi int, sf, fb *sfnt.Font, d *faceData) {
	if dpi <= 0 || len(b.Lines) == 0 || math.IsNaN(x0+y0) || math.IsInf(x0+y0, 0) {
		return
	}
	if sf == nil {
		return
	}
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		deg = 0
	}
	pxmm := float64(dpi) / 25.4
	ppemPx := st.Size / 72 * float64(dpi) // кегль в px
	ppem := fixed.Int26_6(math.Round(ppemPx * 64))
	upem := float64(sf.UnitsPerEm())
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6 // масштаб measure: 1 единица шрифта = 64
	col, _ := parseColor(st.Color)
	o := dst.Bounds().Min
	ox, oy := float64(o.X)+x0*pxmm, float64(o.Y)+y0*pxmm
	p := newPen(dst, image.NewUniform(col), deg, ox, oy)
	// fb — знак вне гарнитуры, из Montserrat, как в measure
	var buf, fbBuf sfnt.Buffer
	main, alt := glyphSrc{sf, &buf}, glyphSrc{fb, &fbBuf}
	for _, l := range b.Lines {
		if l.Text == "" {
			continue
		}
		bx, by := ox+l.X*pxmm, oy+l.Y*pxmm
		if d != nil {
			gl, xs := placeCustom(main, alt, d, l, unit, pxmm, ppemPx)
			for i, g := range gl {
				if segs, err := g.src.f.LoadGlyph(g.src.b, g.g, ppem, nil); err == nil {
					p.glyph(segs, bx+xs[i], by)
				}
			}
			continue
		}
		walkGlyphs(main, alt, l.Text, unit, func(_ rune, src glyphSrc, g sfnt.GlyphIndex, pos, _ fixed.Int26_6) {
			// контур — в px кегля, позиция — в единицах основного шрифта
			if segs, err := src.f.LoadGlyph(src.b, g, ppem, nil); err == nil {
				p.glyph(segs, bx+float64(pos)/64/upem*ppemPx, by)
			}
		})
	}
}

// placedGlyph — глиф строки и шрифт, из которого он взят.
type placedGlyph struct {
	src glyphSrc
	g   sfnt.GlyphIndex
}

// glyphXs — x каждого знака от начала строки в px: ширины adv (px) предыдущих знаков плюс сумма
// DX до знака включительно (мм → px). adv — по одному на кодовую точку Text, DX не короче adv.
func glyphXs(l Line, adv []float64, pxPerMM float64) []float64 {
	xs := make([]float64, len(adv))
	var pos, kern float64
	for i, a := range adv {
		kern += l.DX[i]
		xs[i] = pos + kern*pxPerMM
		pos += a
	}
	return xs
}

// placeCustom — глифы строки своего шрифта и их x (px) от начала строки. Строка из Layout несёт
// DX — ставим по нему, как экран. Бокс собран мимо Layout (DX нет или не сходится по длине) —
// кернинг считается по самой строке (161-2: запасной путь).
func placeCustom(main, alt glyphSrc, d *faceData, l Line, unit fixed.Int26_6, pxmm, ppemPx float64) ([]placedGlyph, []float64) {
	upem := float64(main.f.UnitsPerEm())
	px := func(f fixed.Int26_6) float64 { return float64(f) / 64 / upem * ppemPx }
	var gl []placedGlyph
	var adv []float64
	for _, r := range l.Text {
		src, g, ok := pickGlyph(main, alt, r)
		if !ok {
			continue
		}
		a, err := src.f.GlyphAdvance(src.b, g, unit, xfont.HintingNone)
		if err != nil {
			continue
		}
		gl, adv = append(gl, placedGlyph{src, g}), append(adv, px(a))
	}
	if n := utf8.RuneCountInString(l.Text); len(l.DX) == n && len(gl) == n {
		return gl, glyphXs(l, adv, pxmm)
	}
	p := measureCustom(main.f, alt.f, d, l.Text)
	xs := make([]float64, len(p.x))
	for i, x := range p.x {
		xs[i] = px(x)
	}
	return gl[:min(len(gl), len(xs))], xs
}
