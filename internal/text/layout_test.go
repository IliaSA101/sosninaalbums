package text

import (
	"encoding/binary"
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

// Одиночный завершающий \n строки не даёт (как HTML/contenteditable редактора).
func TestLayoutTrailingNewline(t *testing.T) {
	st := Style{Font: "greatvibes", Size: 80, Color: "#222222", Align: "left"}
	for _, c := range []struct {
		s    string
		want int
	}{
		{"", 1},
		{"а", 1},
		{"а\n", 1},
		{"а\n\n", 2},
		{"а\n\n\n", 3},
		{"\n", 1},
		{"\n\n", 2},
		{"a\nb", 2},
		{"a\nb\n", 2},
		{"a\n\nb", 3},
		{"а\r\n", 1},
	} {
		if n := len(Layout(c.s, st, 200).Lines); n != c.want {
			t.Errorf("%q: строк %d, нужно %d", c.s, n, c.want)
		}
	}
	h := func(s string) float64 { return Layout(s, st, 200).H }
	if a, b := h("Сава и Вася\n"), h("Сава и Вася"); a != b {
		t.Errorf("H с \\n в конце %.3f, без %.3f", a, b)
	}
	if !(h("Сава и Вася\n\n") > h("Сава и Вася")) {
		t.Error("пустая строка после Enter должна расти рамку")
	}
	// кэш и прямой расчёт согласованы
	if a, b := Cached("Сава и Вася\n", st, 200).H, h("Сава и Вася"); a != b {
		t.Errorf("Cached H %.3f, нужно %.3f", a, b)
	}
}

// sfntTables читает hhea, OS/2 и head прямо из байтов файла, без sfnt: проверка
// не зависит от разбора, которым пользуется Layout.
func sfntTables(t *testing.T, b []byte) (hheaAsc, hheaDesc, winAsc, winDesc, upem float64) {
	t.Helper()
	be := binary.BigEndian
	if len(b) < 12 {
		t.Fatal("файл шрифта короче заголовка")
	}
	tab := map[string][]byte{}
	for i, n := 0, int(be.Uint16(b[4:])); i < n; i++ {
		r := b[12+16*i:]
		off, ln := int(be.Uint32(r[8:])), int(be.Uint32(r[12:]))
		tab[string(r[:4])] = b[off : off+ln]
	}
	head, hhea, os2 := tab["head"], tab["hhea"], tab["OS/2"]
	if head == nil || hhea == nil || os2 == nil {
		t.Fatal("в шрифте нет head, hhea или OS/2")
	}
	return float64(int16(be.Uint16(hhea[4:]))), -float64(int16(be.Uint16(hhea[6:]))),
		float64(be.Uint16(os2[74:])), float64(be.Uint16(os2[76:])), float64(be.Uint16(head[18:]))
}

// Модель печати: первая база на hhea ascent, шаг 1,2 кегля, H = asc + desc +
// (n−1)·шаг. На ней держится редактор (156): он ставит строки по этим числам,
// так что смена модели сдвинет печать готовых альбомов.
func TestLayoutBaselineModel(t *testing.T) {
	sizeMM := 80 * 25.4 / 72
	for _, id := range []string{"badscript", "pacifico", "montserrat", "neucha"} {
		asc, desc, winAsc := hheaMM(t, id, sizeMM)
		if id == "neucha" && math.Abs(winAsc-asc) < 1 {
			t.Fatalf("neucha: win (%.2f мм) и hhea (%.2f мм) неразличимы, проверка вырождена", winAsc, asc)
		}
		if id == "badscript" && math.Abs(asc-37.37) > 0.01 {
			t.Errorf("badscript: asc %.3f мм, в спеке 37,37", asc)
		}
		st := Style{Font: id, Size: 80, Color: "#222222", Align: "left"}
		for _, n := range []int{1, 3} {
			// строки разделены \n, ширина с запасом: переносов нет
			box := Layout(strings.Repeat("Аб\n", n-1)+"Аб", st, 400)
			if len(box.Lines) != n {
				t.Fatalf("%s n=%d: строк %d", id, n, len(box.Lines))
			}
			checkModel(t, fmt.Sprintf("%s n=%d", id, n), box, asc, desc, 1.2*sizeMM)
		}
	}
}

// hheaMM — hhea ascent и descent обычного начертания id и win ascent, мм при кегле sizeMM.
func hheaMM(t *testing.T, id string, sizeMM float64) (asc, desc, winAsc float64) {
	t.Helper()
	fam := builtinFamily(id)
	if fam == nil {
		t.Fatalf("%s: нет среди встроенных", id)
	}
	var file string
	for _, fc := range fam.Faces {
		if !fc.Bold && !fc.Italic {
			file = fc.File
		}
	}
	raw, ok := FontFile(file)
	if !ok {
		t.Fatalf("%s: файла %q нет", id, file)
	}
	ha, hd, wa, _, upem := sfntTables(t, raw)
	return ha / upem * sizeMM, hd / upem * sizeMM, wa / upem * sizeMM
}

// checkModel — строки box стоят на asc + i·step, высота = asc + desc + (n−1)·step.
func checkModel(t *testing.T, label string, box Box, asc, desc, step float64) {
	t.Helper()
	const tol = 1e-6
	for i, l := range box.Lines {
		if want := asc + float64(i)*step; math.Abs(l.Y-want) > tol {
			t.Errorf("%s строка %d: Y %.6f, нужно %.6f (hhea)", label, i, l.Y, want)
		}
	}
	if want := asc + desc + float64(len(box.Lines)-1)*step; math.Abs(box.H-want) > tol {
		t.Errorf("%s: H %.6f, нужно %.6f", label, box.H, want)
	}
}
