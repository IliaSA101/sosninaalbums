package text

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// Гарнитуры без ₽ (решение №66): у них знак берётся из Montserrat.
// Тот же список — в fonts/README.md; полноту проверяет TestFallbackRubleList.
var noRuble = []string{"marck", "forum", "neucha", "playfair", "tenorsans", "ruslan", "underdog"}

// missing — знаки вне гарнитуры (₽ из noRuble и noGlyph): id → знаки.
func missing() map[string]string {
	m := map[string]string{}
	for id, s := range noGlyph {
		m[id] += s
	}
	for _, id := range noRuble {
		m[id] += "₽"
	}
	return m
}

func blank(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	return img
}

// Список «нет ₽» не устарел и полон: у гарнитур из него знака нет ни в одном
// начертании, у остальных он есть во всех.
func TestFallbackRubleList(t *testing.T) {
	m, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	var b sfnt.Buffer
	for _, f := range Families() {
		listed := slices.Contains(noRuble, f.ID)
		for _, fc := range f.Faces {
			g, _ := m[fc.File].GlyphIndex(&b, '₽')
			switch {
			case listed && g != 0:
				t.Errorf("%s: ₽ есть, уберите %s из noRuble", fc.File, f.ID)
			case !listed && g == 0:
				t.Errorf("%s: ₽ нет, добавьте %s в noRuble и README", fc.File, f.ID)
			}
		}
	}
}

func TestFallbackStyle(t *testing.T) {
	cases := []struct {
		st   Style
		want string
	}{
		{Style{Font: "marck", Bold: true}, "Montserrat-Regular.ttf"}, // жирного нет — Clamped снял
		{Style{Font: "playfair", Bold: true, Italic: true}, "Montserrat-BoldItalic.ttf"},
		{Style{Font: "playfair", Bold: true}, "Montserrat-Bold.ttf"},
		{Style{Font: "playfair", Italic: true}, "Montserrat-Italic.ttf"},
		{Style{Font: "neucha"}, "Montserrat-Regular.ttf"},
		{Style{Font: "нет-такой"}, ""}, // неизвестная — Montserrat, запасного нет
		{Style{Font: DefaultFont, Bold: true}, ""},
	}
	for _, c := range cases {
		sf, file := fallback(c.st.Clamped())
		if file != c.want || (sf == nil) != (c.want == "") {
			t.Errorf("%+v: файл %q (шрифт nil=%v), ждали %q", c.st, file, sf == nil, c.want)
		}
	}
}

// Знак вне гарнитуры (₽, № у Alice/Prata) стоит той же ширины, что в Montserrat
// того же начертания, и при другом UnitsPerEm (Neucha 1024): масштаб запасного
// знака верный. Проверяются все начертания каждой такой гарнитуры.
func TestFallbackGlyph(t *testing.T) {
	const tol = 1e-4 // мм; точность 1/64 единицы шрифта
	n := 0
	for id, runes := range missing() {
		for _, fc := range family(id).Faces {
			main := Style{Font: id, Size: 20, Bold: fc.Bold, Italic: fc.Italic}
			ref := Style{Font: DefaultFont, Size: 20, Bold: fc.Bold, Italic: fc.Italic}
			for _, r := range runes {
				n++
				got, want := Layout(string(r), main, 100), Layout(string(r), ref, 100)
				if len(got.Lines) != 1 || got.Lines[0].Text != string(r) {
					t.Errorf("%s %q: знак пропал: %+v", fc.File, r, got.Lines)
					continue
				}
				if want.Lines[0].W <= 0 {
					t.Fatalf("%+v: у Montserrat нет %q", ref, r)
				}
				if d := math.Abs(got.Lines[0].W - want.Lines[0].W); d > tol {
					t.Errorf("%s: ширина %q %.6f, в Montserrat %.6f", fc.File, r, got.Lines[0].W, want.Lines[0].W)
				}
			}
		}
	}
	if n < len(noRuble)+len(noGlyph) {
		t.Errorf("проверено знаков %d", n)
	}
	// знака нет и в запасном — пропускается, как прежде
	if b := Layout("中", testStyle, 100); b.Lines[0].Text != "" || b.Lines[0].W != 0 {
		t.Errorf("знак без глифа нигде: %+v", b.Lines)
	}
	if b := Layout("a中b", Style{Font: "marck", Size: 14}, 100); b.Lines[0].Text != "ab" {
		t.Errorf("текст со знаком без глифа: %q", b.Lines[0].Text)
	}
}

// Запасной знак участвует в ширине и переносах: «1 000 ₽» длиннее «1 000»,
// строка из одних ₽ рвётся по знакам, а не исчезает.
func TestFallbackWrap(t *testing.T) {
	for _, id := range noRuble {
		st := Style{Font: id, Size: 20, Align: "left"}
		with, without := Layout("1 000 ₽", st, 200), Layout("1 000", st, 200)
		if !strings.HasSuffix(with.Lines[0].Text, "₽") || with.Lines[0].W <= without.Lines[0].W {
			t.Errorf("%s: ₽ не учтён в строке: %+v / %+v", id, with.Lines, without.Lines)
		}
		wide := Layout("₽₽₽₽₽₽₽₽", st, 200).Lines[0].W
		b := Layout("₽₽₽₽₽₽₽₽", st, wide/3)
		if len(b.Lines) < 3 {
			t.Errorf("%s: строка из ₽ не перенесена: %+v", id, b.Lines)
		}
		var got string
		for _, l := range b.Lines {
			got += l.Text
			if l.W > wide/3+1e-6 {
				t.Errorf("%s: строка %q шире блока", id, l.Text)
			}
		}
		if got != "₽₽₽₽₽₽₽₽" {
			t.Errorf("%s: знаки потеряны: %q", id, got)
		}
	}
}

// На стыке шрифтов кернинга нет: знак вне гарнитуры стоит сразу за шириной
// соседа, следующий — сразу за ним. Перебор всех букв: кернинг чужой пары
// (номер глифа одного шрифта в таблице другого) где-нибудь да не нулевой.
func TestFallbackNoKernAtJoin(t *testing.T) {
	for id, runes := range missing() {
		for _, fc := range family(id).Faces {
			st := Style{Font: id, Bold: fc.Bold, Italic: fc.Italic}.Clamped()
			sf, _ := font(st)
			fb, _ := fallback(st)
			for _, r := range runes {
				for _, c := range needRunes {
					if strings.ContainsRune(runes, c) {
						continue // сам вне гарнитуры: тройка целиком из Montserrat
					}
					s := string([]rune{c, r, c})
					p := measure(sf, fb, s)
					if len(p.r) != 3 {
						t.Fatalf("%s %q: знаков %d", fc.File, s, len(p.r))
					}
					if p.x[1] != p.x[0]+p.adv[0] || p.x[2] != p.x[1]+p.adv[1] {
						t.Errorf("%s %q: на стыке шрифтов кернинг: x=%v adv=%v", fc.File, s, p.x, p.adv)
					}
				}
			}
		}
	}
}

// Draw ставит глифы там же, где measure: строки «c₽c» целиком и те же глифы
// по одному в позициях из measure рисуются одинаково.
func TestFallbackDrawMatchesMeasure(t *testing.T) {
	const dpi = 150
	for id, runes := range missing() {
		for _, fc := range family(id).Faces {
			st := Style{Font: id, Size: 24, Color: "#000000", Align: "left", Bold: fc.Bold, Italic: fc.Italic}.Clamped()
			whole, parts := joinRows(st, runes)
			w, h := 240, int((whole.Lines[len(whole.Lines)-1].Y+24)/25.4*dpi) // 40 мм
			a, b := blank(w, h), blank(w, h)
			Draw(a, whole, st, 2, 0, 0, dpi)
			Draw(b, parts, st, 2, 0, 0, dpi)
			bad := 0
			for i := range a.Pix {
				if d := int(a.Pix[i]) - int(b.Pix[i]); d > 8 || d < -8 {
					bad++
				}
			}
			if bad > 0 {
				t.Errorf("%s: Draw поставил глифы не там, где measure (%d байт отличаются)", fc.File, bad)
			}
		}
	}
}

// joinRows — строки «c₽c» по одной на ряд (whole) и те же глифы отдельными
// строками в позициях из measure (parts); ряд 12 мм.
func joinRows(st Style, runes string) (whole, parts Box) {
	const rowMM = 12.0
	sf, _ := font(st)
	fb, _ := fallback(st)
	size, upem := st.Size*25.4/72, float64(sf.UnitsPerEm())
	for _, r := range runes {
		for _, c := range needRunes {
			if strings.ContainsRune(runes, c) {
				continue
			}
			s := string([]rune{c, r, c})
			y := rowMM * float64(len(whole.Lines)+1)
			whole.Lines = append(whole.Lines, Line{Text: s, Y: y})
			p := measure(sf, fb, s)
			for k, g := range p.r {
				parts.Lines = append(parts.Lines, Line{Text: string(g), X: float64(p.x[k]) / 64 * size / upem, Y: y})
			}
		}
	}
	return whole, parts
}

// Обычный текст (весь набор букв) измеряется так же, с запасным шрифтом и без.
func TestFallbackNoChange(t *testing.T) {
	all := needRunes + " 0123456789 .,!?-—«»"
	for _, f := range families {
		// знаки, которых в гарнитуре нет (noGlyph), рисует запасной шрифт — их в проверку не берём
		s := strings.Map(func(r rune) rune {
			if strings.ContainsRune(noGlyph[f.ID], r) {
				return -1
			}
			return r
		}, all)
		for _, fc := range f.Faces {
			st := Style{Font: f.ID, Bold: fc.Bold, Italic: fc.Italic}.Clamped()
			sf, _ := font(st)
			fb, _ := fallback(st)
			if f.ID != DefaultFont && fb == nil {
				t.Fatalf("%s: запасного шрифта нет", fc.File)
			}
			if f.ID == DefaultFont && fb != nil {
				t.Errorf("%s: у Montserrat запасной шрифт", fc.File)
			}
			a, b := measure(sf, fb, s), measure(sf, nil, s)
			if len([]rune(s)) != len(a.r) { // знаков вне гарнитуры в наборе быть не должно
				t.Errorf("%s: часть набора выпала", fc.File)
			}
			if !reflect.DeepEqual(a, b) {
				t.Errorf("%s: запасной шрифт изменил обычный текст", fc.File)
			}
		}
	}
}

// У Alice и Prata нет «№» (noGlyph): знак рисуется запасным шрифтом, не пропадает.
func TestFallbackNoGlyphFamilies(t *testing.T) {
	for id, runes := range noGlyph {
		for _, r := range runes {
			st := Style{Font: id, Size: 20, Align: "left"}
			b := Layout(string(r), st, 100)
			if len(b.Lines) != 1 || b.Lines[0].Text != string(r) || b.Lines[0].W <= 0 {
				t.Errorf("%s: %q не нарисуется запасным шрифтом: %+v", id, r, b.Lines)
			}
		}
	}
}

// Draw рисует знак вне гарнитуры (₽, №): чернила есть, размер как у Montserrat,
// краска стоит на месте знака из Layout, ничего не выходит за ширину строки.
func TestFallbackDraw(t *testing.T) {
	const dpi = 300
	mmpx := func(mm float64) float64 { return mm / 25.4 * dpi }
	one := func(id string, r rune) image.Rectangle {
		st := Style{Font: id, Size: 40, Color: "#000000", Align: "left"}
		img := blank(int(mmpx(60)), int(mmpx(30)))
		Draw(img, Layout(string(r), st, 50), st, 5, 5, 0, dpi)
		return inkBounds(img)
	}
	for id, runes := range missing() {
		for _, r := range runes {
			ref := one(DefaultFont, r)
			if ref.Empty() {
				t.Fatalf("Montserrat не нарисовала %q", r)
			}
			ink := one(id, r)
			if ink.Empty() {
				t.Errorf("%s: %q не нарисован", id, r)
				continue
			}
			if abs(ink.Dx()-ref.Dx()) > 1 || abs(ink.Dy()-ref.Dy()) > 1 { // субпиксель базовой линии
				t.Errorf("%s: %q %dx%d px, в Montserrat %dx%d", id, r, ink.Dx(), ink.Dy(), ref.Dx(), ref.Dy())
			}
			// знак посреди строки: краска в его колонках, между соседями
			st := Style{Font: id, Size: 20, Color: "#000000", Align: "left"}
			sf, _ := font(st.Clamped())
			fb, _ := fallback(st.Clamped())
			p := measure(sf, fb, "1 000 "+string(r)+" 1")
			mm := func(k int) float64 {
				return float64(p.x[k]) / 64 * st.Clamped().Size * 25.4 / 72 / float64(sf.UnitsPerEm())
			}
			from, to := mm(6), mm(7)
			img := blank(int(mmpx(120)), int(mmpx(30)))
			Draw(img, Layout("1 000 "+string(r)+" 1", st, 100), st, 5, 5, 0, dpi)
			if !inkIn(img, int(mmpx(5+from+(to-from)/4)), int(mmpx(5+to-(to-from)/4))) {
				t.Errorf("%s: на месте %q (%.2f..%.2f мм) нет краски", id, r, from, to)
			}
			// знак в конце строки: правый край чернил там, где кончается строка
			b := Layout("1 000 "+string(r), st, 100)
			img = blank(int(mmpx(120)), int(mmpx(30)))
			Draw(img, b, st, 5, 5, 0, dpi)
			right := inkBounds(img).Max.X
			if float64(right) > mmpx(5+b.Lines[0].W)+mmpx(1) || float64(right) < mmpx(5+b.Lines[0].W)-mmpx(3) {
				t.Errorf("%s: правый край чернил %d px, строка кончается на %.0f px", id, right, mmpx(5+b.Lines[0].W))
			}
		}
	}
}

// inkIn — есть ли нечисто-белые пиксели в колонках [x0, x1).
func inkIn(img *image.RGBA, x0, x1 int) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := max(x0, b.Min.X); x < min(x1, b.Max.X); x++ {
			if img.RGBAAt(x, y).R < 200 {
				return true
			}
		}
	}
	return false
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
