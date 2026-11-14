package text

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"runtime"
	"strings"
	"testing"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func whiteCanvas(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

// bearings — левый отступ первого знака и правый последнего (мм): чернила строки
// идут от x+lsb до x+W−rsb, а W из Layout считается по ширинам знаков.
func bearings(t *testing.T, st Style, s string) (lsb, rsb float64) {
	t.Helper()
	sf, _ := font(st.Clamped())
	if sf == nil {
		t.Fatal("шрифт не загружен")
	}
	var buf sfnt.Buffer
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6
	mm := func(f fixed.Int26_6) float64 { return float64(f) / 64 * st.Size * 25.4 / 72 / float64(sf.UnitsPerEm()) }
	glyph := func(r rune) (fixed.Rectangle26_6, fixed.Int26_6) {
		g, err := sf.GlyphIndex(&buf, r)
		if err != nil || g == 0 {
			t.Fatalf("нет глифа %q", r)
		}
		gb, adv, err := sf.GlyphBounds(&buf, g, unit, xfont.HintingNone)
		if err != nil {
			t.Fatal(err)
		}
		return gb, adv
	}
	rs := []rune(s)
	gb, _ := glyph(rs[0])
	lsb = mm(gb.Min.X)
	gb, adv := glyph(rs[len(rs)-1])
	rsb = mm(adv - gb.Max.X)
	return lsb, rsb
}

// Ширина нарисованной строки — та же, что считает Layout (с кернингом).
// Прежний Drawer давал +2.6 мм на «AVATAR»: его Kern почти нулевой.
func TestDrawKerningMatchesLayout(t *testing.T) {
	const dpi = 300
	mmpx := func(mm float64) float64 { return mm / 25.4 * dpi }
	st := Style{Font: "cormorant", Size: 28, Color: "#000000", Align: "left", Italic: true}
	for _, s := range []string{"Анна и Пётр AVATAR", "AVATAR WAVE"} {
		b := Layout(s, st, 200)
		if len(b.Lines) != 1 {
			t.Fatalf("%q: строк %d, нужна одна", s, len(b.Lines))
		}
		x0, y0 := 10.0, 5.0
		img := whiteCanvas(int(mmpx(230)), int(mmpx(25)))
		Draw(img, b, st, x0, y0, 0, dpi)
		ink := inkBounds(img)
		lsb, rsb := bearings(t, st, s)
		wantL, wantR := x0+b.Lines[0].X+lsb, x0+b.Lines[0].X+b.Lines[0].W-rsb
		gotL, gotR := float64(ink.Min.X)/dpi*25.4, float64(ink.Max.X)/dpi*25.4
		if math.Abs(gotL-wantL) > 0.2 || math.Abs(gotR-wantR) > 0.2 {
			t.Errorf("%q: чернила %.2f..%.2f мм, по Layout %.2f..%.2f мм", s, gotL, gotR, wantL, wantR)
		}
	}
}

// Все чернила повёрнутого блока — внутри повёрнутой рамки.
func TestDrawRotatedInk(t *testing.T) {
	const dpi = 300
	pxmm := float64(dpi) / 25.4
	st := Style{Font: "cormorant", Size: 24, Color: "#000000", Align: "center"}
	b := Layout("Иван и Мария\nAVATAR WAVE, 12 июля", st, 70)
	x0, y0 := 80.0, 80.0
	for _, deg := range []float64{0, 30, 90, -135} {
		img := whiteCanvas(int(160*pxmm), int(160*pxmm))
		Draw(img, b, st, x0, y0, deg, dpi)
		sin, cos := math.Sincos(-deg * math.Pi / 180)
		tol := 1.0
		n, bad := 0, 0
		for y := range img.Rect.Dy() {
			for x := range img.Rect.Dx() {
				if img.RGBAAt(x, y).R >= 200 {
					continue
				}
				n++
				dx, dy := (float64(x)+0.5)/pxmm-x0, (float64(y)+0.5)/pxmm-y0
				u, v := dx*cos-dy*sin, dx*sin+dy*cos
				if u < -tol || u > b.W+tol || v < -tol || v > b.H+tol {
					bad++
				}
			}
		}
		if n == 0 {
			t.Errorf("%v°: ничего не нарисовано", deg)
		}
		if bad > 0 {
			t.Errorf("%v°: %d px чернил вне рамки", deg, bad)
		}
	}
}

// Блок у края и за краем холста: без паники, за холстом (dst — часть большего
// изображения) ничего не появляется.
func TestDrawEdgeClip(t *testing.T) {
	const dpi = 300
	pxmm := float64(dpi) / 25.4
	st := Style{Font: "marck", Size: 60, Color: "#000000", Align: "left"}
	b := Layout("Жизнь\nпрекрасна", st, 100)
	big := whiteCanvas(1200, 800)
	in := image.Rect(200, 200, 1000, 600)
	dst := big.SubImage(in).(*image.RGBA)
	wmm, hmm := float64(in.Dx())/pxmm, float64(in.Dy())/pxmm
	for _, p := range [][2]float64{{-3, -3}, {wmm - 20, hmm - 10}, {-80, hmm / 2}, {wmm + 50, -50}} {
		Draw(dst, b, st, p[0], p[1], 20, dpi)
	}
	inside := false
	for y := range big.Rect.Dy() {
		for x := range big.Rect.Dx() {
			if big.RGBAAt(x, y).R >= 200 {
				continue
			}
			if !image.Pt(x, y).In(in) {
				t.Fatalf("чернила за холстом в (%d, %d)", x, y)
			}
			inside = true
		}
	}
	if !inside {
		t.Error("у края ничего не нарисовано")
	}
}

// TestDrawMaskClipped — аудит 082: маска глифа не больше холста. «Ж» 200 pt при 300 dpi —
// ~800 px; без обрезки маска и растеризатор берут мегабайты на холсте 100×100.
func TestDrawMaskClipped(t *testing.T) {
	const dpi, side = 300, 100
	st := Style{Font: DefaultFont, Size: MaxSize, Color: "#000000", Align: "left"}
	b := Layout("Ж", st, 500)
	dst := whiteCanvas(side, side)
	Draw(dst, b, st, -5, -30, 0, dpi) // прогрев: кэш шрифта и разбор файла — не в счёт
	if !hasInk(dst) {
		t.Fatal("глиф не лёг на холст — проверять нечего")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	Draw(dst, b, st, -5, -30, 0, dpi)
	runtime.ReadMemStats(&after)
	// маска 1 байт и растеризатор 4 байта на px холста, плюс мелочь строки
	if got, lim := after.TotalAlloc-before.TotalAlloc, uint64(side*side*5+64<<10); got > lim {
		t.Errorf("отрисовка выделила %d байт, предел %d", got, lim)
	}
	// то же напрямую: маска пера не больше холста, хотя глиф намного больше
	sf, _ := font(st)
	var buf sfnt.Buffer
	g, _ := sf.GlyphIndex(&buf, 'Ж')
	segs, err := sf.LoadGlyph(&buf, g, fixed.I(833), nil) // 200 pt при 300 dpi
	if err != nil {
		t.Fatal(err)
	}
	p := newPen(dst, image.Black, 15, 0, 0)
	p.glyph(segs, -50, 600)
	if n := cap(p.mask.Pix); n == 0 || n > side*side {
		t.Errorf("маска %d байт, холст %d px", n, side*side)
	}
}

func hasInk(img *image.RGBA) bool {
	for i := 0; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 128 {
			return true
		}
	}
	return false
}

// TestDrawClippedSame — обрезанная по холсту маска даёт те же пиксели, что целая: холст —
// окно большого изображения, глиф (с поворотом) режут все четыре края.
func TestDrawClippedSame(t *testing.T) {
	const dpi = 300
	pxmm := float64(dpi) / 25.4
	st := Style{Font: "marck", Size: 120, Color: "#202020", Align: "left"}
	b := Layout("Жёлудь", st, 300)
	full := whiteCanvas(1400, 700)
	Draw(full, b, st, 10, 10, 12, dpi)
	for _, win := range []image.Rectangle{image.Rect(150, 120, 420, 330), image.Rect(0, 0, 90, 700), image.Rect(700, 300, 1400, 700)} {
		part := whiteCanvas(1400, 700).SubImage(win).(*image.RGBA)
		Draw(part, b, st, 10-float64(win.Min.X)/pxmm, 10-float64(win.Min.Y)/pxmm, 12, dpi)
		ink, worst := 0, 0
		for y := win.Min.Y; y < win.Max.Y; y++ {
			for x := win.Min.X; x < win.Max.X; x++ {
				a, c := int(full.RGBAAt(x, y).R), int(part.RGBAAt(x, y).R)
				if a < 200 {
					ink++
				}
				worst = max(worst, abs(a-c))
			}
		}
		if ink == 0 {
			t.Errorf("%v: окно без чернил — тест ничего не режет", win)
		}
		// до 512 px vector считает в фиксированной точке, крупнее — в плавающей: расхождение
		// в пару уровней ожидаемо, ошибка сдвига начала дала бы сотни
		if worst > 4 {
			t.Errorf("%v: обрезанная маска расходится на %d", win, worst)
		}
	}
}

func BenchmarkDrawRotated(b *testing.B) {
	const dpi = 300
	st := Style{Font: "cormorant", Size: 28, Color: "#333333", Align: "center"}
	s := "Мы встретились осенью, и с тех пор каждый день — праздник. Спасибо всем, кто был рядом!"
	box := Layout(strings.Join([]string{s, s, s}, "\n"), st, 130) // 9 строк
	img := whiteCanvas(7000, 3600)
	b.ResetTimer()
	for range b.N {
		Draw(img, box, st, 60, 120, 30, dpi)
	}
	b.ReportMetric(float64(len(box.Lines)), "lines")
}
