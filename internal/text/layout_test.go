package text

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

var testStyle = Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}

func TestLayoutWrap(t *testing.T) {
	txt := "Иван и Мария приглашают вас на свою свадьбу Wedding day AV To"
	w := 40.0
	b := Layout(txt, testStyle, w)
	if len(b.Lines) < 3 {
		t.Fatalf("мало строк: %d", len(b.Lines))
	}
	var words []string
	for _, l := range b.Lines {
		if l.W > w+1e-6 {
			t.Errorf("строка %q шире блока: %.2f > %.0f", l.Text, l.W, w)
		}
		if strings.HasSuffix(l.Text, " ") {
			t.Errorf("хвостовой пробел: %q", l.Text)
		}
		words = append(words, strings.Fields(l.Text)...)
	}
	if got, want := strings.Join(words, " "), txt; got != want {
		t.Errorf("слова потеряны: %q", got)
	}
	// шире — не больше строк; монотонность
	if wide := Layout(txt, testStyle, 400); len(wide.Lines) != 1 {
		t.Errorf("в 400 мм ждали 1 строку, %d", len(wide.Lines))
	}
	// высота и базовые линии
	size := 14 * 25.4 / 72
	lh := 1.2 * size
	if d := b.Lines[1].Y - b.Lines[0].Y; math.Abs(d-lh) > 1e-9 {
		t.Errorf("шаг строк %.4f, ждали %.4f", d, lh)
	}
	if b.H <= float64(len(b.Lines)-1)*lh {
		t.Errorf("H=%.2f меньше шагов строк", b.H)
	}
	// абзацы и пустой абзац
	p := Layout("а\n\nб", testStyle, 100)
	if len(p.Lines) != 3 || p.Lines[1].Text != "" {
		t.Errorf("абзацы: %+v", p.Lines)
	}
	// слово длиннее блока рвётся по знакам
	long := Layout(strings.Repeat("Ш", 30), testStyle, 20)
	if len(long.Lines) < 2 {
		t.Errorf("длинное слово не порвано")
	}
	for _, l := range long.Lines {
		if l.W > 20+1e-6 || l.Text == "" {
			t.Errorf("кусок %q ширина %.2f", l.Text, l.W)
		}
	}
	if got := strings.Join(lineTexts(long), ""); got != strings.Repeat("Ш", 30) {
		t.Errorf("знаки потеряны: %q", got)
	}
	// пустой и нулевая ширина не падают
	if e := Layout("", testStyle, 50); len(e.Lines) != 1 {
		t.Errorf("пустой текст: %d строк", len(e.Lines))
	}
	_ = Layout("текст", testStyle, 0)
	_ = Layout("текст", testStyle, math.NaN())
}

func lineTexts(b Box) []string {
	var out []string
	for _, l := range b.Lines {
		out = append(out, l.Text)
	}
	return out
}

func TestLayoutKerning(t *testing.T) {
	// пара AV кернится: ширина «AV» меньше суммы ширин «A» и «V»
	a, v, av := Layout("A", testStyle, 500).Lines[0].W, Layout("V", testStyle, 500).Lines[0].W, Layout("AV", testStyle, 500).Lines[0].W
	if !(av < a+v-1e-6) {
		t.Errorf("кернинга нет: AV=%.4f A+V=%.4f", av, a+v)
	}
}

func TestLayoutAlign(t *testing.T) {
	w := 100.0
	for _, c := range []struct {
		align string
		want  func(lw float64) float64
	}{
		{"left", func(float64) float64 { return 0 }},
		{"center", func(lw float64) float64 { return (w - lw) / 2 }},
		{"right", func(lw float64) float64 { return w - lw }},
	} {
		st := testStyle
		st.Align = c.align
		b := Layout("Привет", st, w)
		l := b.Lines[0]
		if math.Abs(l.X-c.want(l.W)) > 1e-9 {
			t.Errorf("%s: x=%.3f", c.align, l.X)
		}
	}
}

func TestLayoutMissingGlyphsSkipped(t *testing.T) {
	st := Style{Font: "marck", Size: 14, Align: "left"}
	b := Layout("5中", st, 100) // нет ни в Marck, ни в запасном Montserrat
	if b.Lines[0].Text != "5" {
		t.Errorf("текст %q, ждали «5» (中 нет ни в одном шрифте)", b.Lines[0].Text)
	}
	if ref := Layout("5", st, 100); math.Abs(ref.Lines[0].W-b.Lines[0].W) > 1e-9 {
		t.Error("пропущенный знак измерен")
	}
}

func TestCleanText(t *testing.T) {
	in := "a" + string(rune(13)) + string(rune(10)) + "b" + string(rune(9)) + "c" + string(rune(0)) + "d" + string(rune(0x200b)) +
		"e" + string(rune(0x2028)) + "f" + string([]byte{0xff}) + "g"
	got := CleanText(in)
	if got != "a\nb cdefg" {
		t.Errorf("%q", got)
	}
	if n := len([]rune(CleanText(strings.Repeat("я", MaxRunes+100)))); n != MaxRunes {
		t.Errorf("длина %d", n)
	}
}

func TestCachedBounded(t *testing.T) {
	for i := 0; i < cacheSize+200; i++ {
		Cached(fmt.Sprintf("текст %d", i), testStyle, 50)
	}
	cache.Lock()
	n, m := cache.lru.Len(), len(cache.m)
	cache.Unlock()
	if n > cacheSize || m != n {
		t.Errorf("кэш %d/%d, предел %d", n, m, cacheSize)
	}
	a := Cached("копия", testStyle, 50.004)
	a.Lines[0].Text = "испорчено"
	if b := Cached("копия", testStyle, 50); b.Lines[0].Text != "копия" {
		t.Error("кэш отдал не копию")
	}
	if w := Layout("копия", testStyle, 50); w.Lines[0].W != Cached("копия", testStyle, 50).Lines[0].W {
		t.Error("Cached и Layout разошлись")
	}
}

func BenchmarkLayout1000(b *testing.B) {
	txt := strings.Repeat("Свадебный альбом AV Wedding To день ", 28)[:1000]
	for _, f := range []string{"montserrat", "cormorant"} {
		b.Run(f, func(b *testing.B) {
			st := Style{Font: f, Size: 14, Align: "left"}
			for i := 0; i < b.N; i++ {
				Layout(txt, st, 120)
			}
		})
	}
}
