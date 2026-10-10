package text

import (
	"image"
	"math"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Draw рисует блок b на dst: (x0, y0) — левый верхний угол блока в мм от
// верхнего левого угла dst, deg — поворот блока вокруг этого угла по часовой
// (NaN/Inf — без поворота), dpi — разрешение холста. Строки берутся из Layout
// с тем же стилем, а глифы ставятся с тем же кернингом, что в measure, так что
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
	var buf sfnt.Buffer
	for _, l := range b.Lines {
		if l.Text == "" {
			continue
		}
		bx, by := ox+l.X*pxmm, oy+l.Y*pxmm
		var prev sfnt.GlyphIndex
		var pos fixed.Int26_6 // в единицах шрифта ×64, как в measure
		first := true
		for _, r := range l.Text {
			g, err := sf.GlyphIndex(&buf, r)
			if err != nil || g == 0 {
				continue
			}
			adv, err := sf.GlyphAdvance(&buf, g, unit, xfont.HintingNone)
			if err != nil {
				continue
			}
			if !first {
				if k, err := sf.Kern(&buf, prev, g, unit, xfont.HintingNone); err == nil {
					pos += k
				}
			}
			first = false
			if segs, err := sf.LoadGlyph(&buf, g, ppem, nil); err == nil {
				p.glyph(segs, bx+float64(pos)/64/upem*ppemPx, by)
			}
			pos += adv
			prev = g
		}
	}
}
