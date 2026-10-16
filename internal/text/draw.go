package text

import (
	"image"
	"math"

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
	if dpi <= 0 || len(b.Lines) == 0 || math.IsNaN(x0+y0) || math.IsInf(x0+y0, 0) {
		return
	}
	sf, _ := font(st)
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
	fb, _ := fallback(st) // знак вне гарнитуры — из Montserrat, как в measure
	var buf, fbBuf sfnt.Buffer
	main, alt := glyphSrc{sf, &buf}, glyphSrc{fb, &fbBuf}
	for _, l := range b.Lines {
		if l.Text == "" {
			continue
		}
		bx, by := ox+l.X*pxmm, oy+l.Y*pxmm
		walkGlyphs(main, alt, l.Text, unit, func(_ rune, src glyphSrc, g sfnt.GlyphIndex, pos, _ fixed.Int26_6) {
			// контур — в px кегля, позиция — в единицах основного шрифта
			if segs, err := src.f.LoadGlyph(src.b, g, ppem, nil); err == nil {
				p.glyph(segs, bx+float64(pos)/64/upem*ppemPx, by)
			}
		})
	}
}
