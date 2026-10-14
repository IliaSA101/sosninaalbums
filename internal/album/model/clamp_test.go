package model

import (
	"log"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Строки из файла в log.txt — обрезанные (paths.LogQ): мегабайтный вид разворота или тип
// изделия не раздувают лог, перевод строки не подделывает запись.
func TestLogFileStringsClipped(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	huge := "x\n" + strings.Repeat("я", 1<<20)
	p := New()
	p.Spreads = []*Spread{{ID: "a", Kind: huge}}
	p.FixVignettes()
	NormProduct(huge)
	var l PanicLog
	l.Safely(huge, func() { panic("сбой") })
	out := buf.String()
	if p.Spreads[0].Kind != "" || strings.Count(out, "ПАНИКА") != 1 || strings.Count(out, `"x\nяя`) != 3 {
		t.Fatalf("лог: %.300q", out)
	}
	if first, _, _ := strings.Cut(out, "goroutine"); len(first) > 1000 {
		t.Fatalf("строки из файла не обрезаны: %d байт", len(first))
	}
}

func TestClampFrame(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []struct {
		in   *Frame
		want *Frame
	}{
		{nil, nil},
		{&Frame{S: 1}, nil},
		{&Frame{S: nan, X: inf, Y: -inf}, nil},
		{&Frame{S: 0.5, X: 0.0001}, nil}, // шум округления — тоже сброс
		{&Frame{S: 10, X: -3, Y: 3}, &Frame{S: 4, X: -1, Y: 1}},
		{&Frame{S: 1.23456, X: 0.12345, Y: nan}, &Frame{S: 1.235, X: 0.123}},
	} {
		got := ClampFrame(c.in)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("ClampFrame(%+v) = %+v, ждали %+v", c.in, got, c.want)
		}
	}
}

func TestClampColors(t *testing.T) {
	if c, ok := ClampColor("#ABCdef"); !ok || c != "#abcdef" {
		t.Fatalf("цвет: %q", c)
	}
	for _, bad := range []string{"", "#abc", "red", "#gggggg", "#1234567", "abcdef1", "#12345\x00"} {
		if c, ok := ClampColor(bad); ok || c != "" {
			t.Errorf("цвет %q пропущен", bad)
		}
	}
	if st := ClampStyle(TextStyle{Color: "#FFF"}); st.Color != text.DefaultColor || st.Font != text.DefaultFont {
		t.Fatalf("стиль: %+v", st)
	}
	p := New()
	p.Settings.Bg = "#FFF"
	p.Spreads = []*Spread{{ID: "s", BgL: "#AA0000", BgR: "url(x)"}}
	p.ClampDesign()
	if p.Settings.Bg != "" || p.Spreads[0].BgL != "#aa0000" || p.Spreads[0].BgR != "" {
		t.Fatalf("фон: %q %q %q", p.Settings.Bg, p.Spreads[0].BgL, p.Spreads[0].BgR)
	}
}

func TestClampTexts(t *testing.T) {
	p := New()
	nan := math.NaN()
	long := strings.Repeat("я", 1500)
	many := strings.Repeat("строка\r\n", 70)
	blocks := []*TextBlock{
		nil,
		{ID: "c0ffee000001", Text: "  \n\t", W: 100}, // пустой — выброшен
		{ID: "c0ffee000001", Text: "a\u202eb\x07c\u2028d", W: nan, X: nan, Y: math.Inf(-1)},
		{ID: "c0ffee000001", Text: long, W: 5, X: 1e6, Y: 1e6},   // повтор ID
		{ID: "C0FFEE000002", Text: many, W: 1e6, X: -10, Y: -10}, // ID не в нижнем регистре
		{ID: "x", Text: "ok", W: 100, Style: TextStyle{Font: "marck", Bold: true, Size: 1e9, Align: "?"}},
	}
	for range MaxTexts + 10 { // сверх MaxTexts — лишние отброшены
		blocks = append(blocks, &TextBlock{ID: NewID(), Text: "t", W: 50})
	}
	p.Spreads = []*Spread{{ID: "s", Texts: blocks}}
	long2 := strings.Repeat("ж", 400)
	p.Photos["p"] = PhotoRef{Path: "/p.jpg", Caption: &Caption{Text: long2, Style: TextStyle{Size: nan}}}
	p.Photos["q"] = PhotoRef{Path: "/q.jpg", Caption: &Caption{Text: " \r\n "}}
	p.ClampDesign()
	p.FitTexts()
	ts := p.Spreads[0].Texts
	if len(ts) != MaxTexts {
		t.Fatalf("блоков %d", len(ts))
	}
	seen := map[string]bool{}
	sw, ph := 2*p.Settings.PageW, p.Settings.PageH
	for i, b := range ts {
		h := text.Cached(b.Text, b.Style, b.W).H
		if !IsTextID(b.ID) || seen[b.ID] || b.W < MinTextW || b.W > sw || b.X < 0 || b.X+b.W > sw+1e-9 || b.Y < 0 || b.Y+h > ph+1e-9 && b.Y != 0 {
			t.Fatalf("блок %d: %+v (h %.1f)", i, b, h)
		}
		seen[b.ID] = true
		for _, r := range b.Text {
			if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
				t.Fatalf("блок %d: управляющий знак %U", i, r)
			}
		}
	}
	if ts[0].Text != "abc\nd" || ts[0].W != 0.4*p.Settings.PageW || ts[0].X != 0 || ts[0].Y != 0 {
		t.Fatalf("NaN и чистка: %+v", ts[0])
	}
	if n := len([]rune(ts[1].Text)); n != MaxTextRunes || ts[1].ID == "c0ffee000001" {
		t.Fatalf("длина %d, id %s", n, ts[1].ID)
	}
	if n := strings.Count(ts[2].Text, "\n") + 1; n != MaxTextLines || strings.Contains(ts[2].Text, "\r") {
		t.Fatalf("строк %d", n)
	}
	if st := ts[3].Style; st.Bold || st.Size != text.MaxSize || st.Align != "center" {
		t.Fatalf("стиль: %+v", st)
	}
	if c := p.Photos["p"].Caption; c == nil || len([]rune(c.Text)) != MaxCaptionRunes || c.Style.Size != text.DefaultSize {
		t.Fatalf("подпись: %+v", c)
	}
	if p.Photos["q"].Caption != nil {
		t.Fatal("пустая подпись должна сниматься")
	}
	// Второй проход ничего не меняет: холостая правка остаётся холостой.
	before := Marshal(p)
	p.ClampDesign()
	p.FitTexts()
	if string(before) != string(Marshal(p)) {
		t.Fatal("зажим не идемпотентен")
	}
}

// Кадр по умолчанию: без Frame в PhotoRef — S = 1 без сдвига (проверка ушла из
// TestFramesParity при переносе в geom).
func TestDefaultFrame(t *testing.T) {
	if (PhotoRef{}).FrameOrDefault() != (Frame{S: 1}) {
		t.Fatal("кадр по умолчанию")
	}
}

// Безопасная зона (028 А): 0 — «без зоны» и сохраняется, NaN/Inf — по умолчанию 8,
// отрицательная — 0, больше четверти меньшей стороны страницы — зажим.
func TestClampSettingsSafe(t *testing.T) {
	if d := DefaultSettings(); d.Safe != DefaultSafe || DefaultSafe != 8 {
		t.Fatalf("по умолчанию %v (DefaultSafe %v)", d.Safe, DefaultSafe)
	}
	cases := []struct {
		name         string
		pw, ph, safe float64
		want         float64
	}{
		{"8", 300, 300, 8, 8},
		{"0", 300, 300, 0, 0},
		{"дробное", 300, 300, 12.5, 12.5},
		{"NaN", 300, 300, math.NaN(), 8},
		{"+Inf", 300, 300, math.Inf(1), 8},
		{"-Inf", 300, 300, math.Inf(-1), 8},
		{"< 0", 300, 300, -3, 0},
		{"> ¼ меньшей стороны", 300, 200, 60, 50},
		{"ровно ¼", 200, 300, 50, 50},
	}
	for _, c := range cases {
		s := DefaultSettings()
		s.PageW, s.PageH, s.Safe = c.pw, c.ph, c.safe
		if got := ClampSettings(s).Safe; got != c.want {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}

// Числа обложки (032 §4): NaN/Inf — «Свои размеры», пределы, таблица ≤ 64 строк по
// возрастанию без повторов, неизвестный пресет — custom; зажим идемпотентен.
func TestClampCoverSpec(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	var long []SpineStep
	for i := range 70 {
		long = append(long, SpineStep{Upto: 70 - i, Mm: float64(i)})
	}
	got := ClampCoverSpec(CoverSpec{Preset: "чужой", W: 20, H: -5, Wrap: nan, Hinge: 99, A: inf, B: -inf, Steps: long})
	if got.Preset != CoverCustom || got.W != 50 || got.H != 0 || got.Wrap != 15 || got.Hinge != 20 || got.A != 10 || got.B != 0 {
		t.Fatalf("числа: %+v", got)
	}
	if len(got.Steps) != MaxSpineSteps {
		t.Fatalf("строк %d", len(got.Steps))
	}
	for i := 1; i < len(got.Steps); i++ {
		if got.Steps[i].Upto <= got.Steps[i-1].Upto {
			t.Fatalf("таблица не по возрастанию: %v", got.Steps[i-1:i+1])
		}
	}
	dup := ClampCoverSpec(CoverSpec{Preset: "fsfera-card", W: 1e9, Steps: []SpineStep{{5, 8}, {3, nan}, {5, 9}, {0, 500}, {MaxSpreads + 5, 6}}})
	want := []SpineStep{{1, 100}, {3, 10}, {5, 8}, {MaxSpreads, 6}}
	if dup.Preset != "fsfera-card" || dup.W != 1000 || !slices.Equal(dup.Steps, want) {
		t.Fatalf("повторы и пределы: %+v", dup)
	}
	for _, c := range append([]CoverPreset{{Spec: got}, {Spec: dup}}, CoverPresets()...) {
		if again := ClampCoverSpec(c.Spec); !reflect.DeepEqual(again, c.Spec) {
			t.Errorf("%s: зажим не идемпотентен или портит пресет: %+v → %+v", c.ID, c.Spec, again)
		}
	}
	if (CoverSpec{}).Steps != nil || ClampCoverSpec(CoverSpec{Steps: []SpineStep{}}).Steps != nil {
		t.Fatal("пустая таблица не nil")
	}
}

// Variety из UI зажимается в 0…2: чужое число не включает несуществующий уровень (041).
func TestVarietyClamp(t *testing.T) {
	for in, want := range map[Variety]Variety{-5: VarietyLess, -1: VarietyLess, 0: VarietyLess, 1: VarietyNormal, 2: VarietyMore, 3: VarietyMore, 1 << 40: VarietyMore} {
		if got := in.Clamp(); got != want {
			t.Fatalf("Variety(%d).Clamp() = %d, ждали %d", in, got, want)
		}
	}
	if (AutoOpts{}).Variety != VarietyLess {
		t.Fatal("нулевые AutoOpts — не «Меньше»")
	}
}
