package model

import (
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
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

// TestClampFoldAny (064): FoldAny держится только при layflat и не у листа; новый альбом —
// «Только широкие»; Params передаёт положение (у листа — без FoldAny); зажим идемпотентен.
func TestClampFoldAny(t *testing.T) {
	if d := DefaultSettings(); !d.Layflat || d.FoldAny {
		t.Fatalf("по умолчанию layflat %v, foldAny %v — ждали «Только широкие»", d.Layflat, d.FoldAny)
	}
	cases := []struct {
		layflat, foldAny, sheet bool
		wantFlat, wantAny       bool
	}{
		{false, false, false, false, false},
		{false, true, false, false, false},
		{true, false, false, true, false},
		{true, true, false, true, true},
		{true, true, true, true, false},
		{false, true, true, true, false},
	}
	for _, c := range cases {
		s := DefaultSettings()
		s.Layflat, s.FoldAny, s.Sheet = c.layflat, c.foldAny, c.sheet
		got := ClampSettings(s)
		if got.Layflat != c.wantFlat || got.FoldAny != c.wantAny || ClampSettings(got) != got {
			t.Errorf("%+v: layflat %v, foldAny %v (ждали %v, %v)", c, got.Layflat, got.FoldAny, c.wantFlat, c.wantAny)
		}
		p := New()
		p.Settings = got
		if prm := p.Params(); prm.FoldAny != got.FoldAny || prm.Layflat != got.Layflat {
			t.Errorf("%+v: Params layflat %v, foldAny %v", c, prm.Layflat, prm.FoldAny)
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
	// Безопасные линии (047): NaN/Inf и минус — 0 (Settings.Safe), сверху 30, шаг 0,01.
	for _, c := range [][4]float64{{nan, 0, 0, 0}, {-1, inf, 0, 0}, {1e9, -inf, 30, 0}, {8.004, 12.006, 8, 12.01}, {0, 1e9, 0, 30}} {
		got := ClampCoverSpec(CoverSpec{Preset: "fsfera-thin", Safe: c[0], SpineSafe: c[1]})
		if got.Safe != c[2] || got.SpineSafe != c[3] {
			t.Errorf("линии %v/%v → %v/%v, ждали %v/%v", c[0], c[1], got.Safe, got.SpineSafe, c[2], c[3])
		}
		if again := ClampCoverSpec(got); again.Safe != got.Safe || again.SpineSafe != got.SpineSafe {
			t.Errorf("зажим линий не идемпотентен: %+v → %+v", got, again)
		}
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

// AutoOpts из UI (065): Variety в 0…2, семейств не больше 16 строк (nil остаётся nil, короткий
// список — как есть); остальные поля не трогаются.
func TestAutoOptsClamp(t *testing.T) {
	long := make([]string, 40)
	for i := range long {
		long[i] = strconv.Itoa(i)
	}
	got := AutoOpts{Spreads: 7, MaxPer: 9, Variety: 9, Families: long}.Clamp()
	if got.Variety != VarietyMore || got.Spreads != 7 || got.MaxPer != 9 || !slices.Equal(got.Families, long[:16]) {
		t.Fatalf("Clamp: %+v", got)
	}
	// 078: уровни похожих и ч/б в 0…3, вариант в 1…MaxVariant, возвращённых — первые 10 000.
	keep := make([]string, maxKeepArgs+5)
	for _, c := range []struct{ in, sim, gray, variant int }{{0, 0, 0, 1}, {-3, 0, 0, 1}, {2, 2, 2, 2}, {7, 3, 3, 7}, {1 << 40, 3, 3, MaxVariant}} {
		got := AutoOpts{Similar: c.in, Gray: c.in, Variant: c.in, Keep: keep, Again: true}.Clamp()
		if got.Similar != c.sim || got.Gray != c.gray || got.Variant != c.variant || len(got.Keep) != maxKeepArgs || !got.Again {
			t.Fatalf("Clamp(%d): %+v", c.in, got)
		}
	}
	if (AutoOpts{}).Clamp().Keep != nil || len(AutoOpts{Keep: keep[:3]}.Clamp().Keep) != 3 {
		t.Fatal("Clamp: короткий и пустой Keep")
	}
	if ClampFamilies(nil) != nil || !slices.Equal(ClampFamilies([]string{"dense", "hero"}), []string{"dense", "hero"}) ||
		len(ClampFamilies(long[:16])) != 16 || len(ClampFamilies([]string{})) != 0 {
		t.Fatal("ClampFamilies")
	}
}

// randomTexts — проект с блоками у краёв и за краями: углы, выравнивания, «по тексту».
func randomTexts(r *rand.Rand, n int) *Project {
	p := New()
	words := []string{"Текст", "Иван и Мария", "AVATAR WAVE", "очень длинная строка про свадьбу в саду у реки", "раз\nдва\nтри"}
	aligns := []string{"left", "center", "right"}
	s := &Spread{ID: NewID()}
	for range n {
		st := plainStyle
		st.Align = aligns[r.IntN(3)]
		st.Size = 6 + r.Float64()*60
		tb := &TextBlock{ID: NewID(), X: r.Float64()*700 - 50, Y: r.Float64()*400 - 50, W: r.Float64() * 700,
			Rot: geom.NormDeg(r.Float64()*400 - 200), Auto: r.IntN(2) == 0, Text: words[r.IntN(len(words))], Style: st}
		if r.IntN(4) == 0 {
			tb.Rot = 0
		}
		s.Texts = append(s.Texts, tb)
		if len(s.Texts) == MaxTexts {
			p.Spreads = append(p.Spreads, s)
			s = &Spread{ID: NewID()}
		}
	}
	if len(s.Texts) > 0 {
		p.Spreads = append(p.Spreads, s)
	}
	return p
}

func TestFitTextsIdempotent(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 11))
	for i := range 20 {
		p := randomTexts(r, 40)
		if i%2 == 1 {
			p.Settings.PageW, p.Settings.PageH = 50+r.Float64()*300, 50+r.Float64()*300
		}
		p.FitTexts()
		once := Marshal(p)
		p.FitTexts()
		if string(once) != string(Marshal(p)) {
			t.Fatalf("прогон %d: второй fitTexts поменял проект", i)
		}
		sw, ph := 2*p.Settings.PageW, p.Settings.PageH
		for _, s := range p.Spreads {
			for _, tb := range s.Texts {
				a := BoxOf(tb).AABB()
				if a.W <= sw && (a.X < -1e-6 || a.X+a.W > sw+1e-6) || a.W > sw && math.Abs(a.X) > 0.01 {
					t.Fatalf("прогон %d: блок за разворотом по x: %+v, aabb %+v", i, *tb, a)
				}
				if a.H <= ph && (a.Y < -1e-6 || a.Y+a.H > ph+1e-6) {
					t.Fatalf("прогон %d: блок за разворотом по y: %+v, aabb %+v", i, *tb, a)
				}
			}
		}
	}
}

func TestFitTextsRotated(t *testing.T) {
	p := New()
	tb := &TextBlock{ID: NewID(), X: 590, Y: -10, W: 100, Rot: 30, Text: "Текст у края", Style: plainStyle}
	p.Spreads = []*Spread{{ID: NewID(), Texts: []*TextBlock{tb}}}
	p.FitTexts()
	a := BoxOf(tb).AABB()
	if a.X < 0 || a.X+a.W > 600+1e-9 || a.Y < 0 || a.X+a.W < 600-0.011 || a.Y > 0.011 || tb.Rot != 30 || tb.W != 100 {
		t.Fatalf("повёрнутый у края: %+v, aabb %+v", *tb, a)
	}
	if math.Abs(tb.X*100-math.Round(tb.X*100)) > 1e-6 {
		t.Fatalf("x не округлён до сотых: %v", tb.X)
	}
}

// BenchmarkFitTexts2000 — fitTexts на 2000 блоках (MaxTotalTexts) с тёплым кэшем; цель ≤ 20 мс.
func BenchmarkFitTexts2000(b *testing.B) {
	p := randomTexts(rand.New(rand.NewPCG(1, 2)), MaxTotalTexts)
	p.FitTexts()
	b.ResetTimer()
	for b.Loop() {
		p.FitTexts()
	}
}

// Ширина «по тексту» — наименьшая, при которой текст встаёт одной строкой.
func TestAutoW(t *testing.T) {
	tb := &TextBlock{Text: "Анна и Пётр AVATAR", Style: plainStyle}
	w := AutoW(nil, tb, 1000)
	if n := len(text.Cached(tb.Text, tb.Style, w).Lines); n != 1 {
		t.Fatalf("в ширину %v — %d строк", w, n)
	}
	if nat := text.NaturalWidth(tb.Text, tb.Style); w < nat || w > nat+0.02+1e-9 {
		t.Fatalf("ширина %v при естественной %v", w, nat)
	}
	if w = AutoW(nil, tb, 20); w != 20 {
		t.Fatalf("предел: %v", w)
	}
	if got := New().TextMaxW(nil); got != 284 {
		t.Fatalf("textMaxW %v", got)
	}
}

// Форма места (044): белый список форм, скругление 0…50 целым, у эллипса 0; зажим один
// во всех входах — места файла, коробки шаблонов, виньетка и её стиль.
func TestClampPlaceShape(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		shape     string
		round     float64
		wantShape string
		wantRound float64
	}{
		{"", 0, "", 0},
		{"oval", 0, "oval", 0},
		{"oval", 30, "oval", 0}, // у круга скругления нет
		{"ellipse", 10, "", 10}, // чужая форма — прямоугольник
		{"heart", 0, "", 0},
		{"", nan, "", 0},
		{"", inf, "", 0},
		{"", -inf, "", 0},
		{"", -5, "", 0},
		{"", 70, "", 50},
		{"", 12.4, "", 12},
		{"", 12.5, "", 13},
		{"", 50, "", 50},
	}
	for _, c := range cases {
		s, r := ClampShape(c.shape, c.round)
		if s != c.wantShape || r != c.wantRound {
			t.Errorf("ClampShape(%q, %v) = %q, %v; ждали %q, %v", c.shape, c.round, s, r, c.wantShape, c.wantRound)
		}
		if s2, r2 := ClampShape(s, r); s2 != s || r2 != r {
			t.Errorf("не идемпотентна: %q %v → %q %v", s, r, s2, r2)
		}
	}
	// Места файла: NaN в round место не выбрасывает (в отличие от NaN в рамке).
	p := New()
	p.Photos["a"] = PhotoRef{Path: "a.jpg", W: 3000, H: 2000}
	s := &Spread{ID: "s", Photos: []string{"a"}, Grid: []*Place{
		{ID: "000000000001", X: 10, Y: 10, W: 50, H: 50, Photo: "a", Shape: "oval", Round: 20},
		{ID: "000000000002", X: 70, Y: 10, W: 50, H: 50, Round: nan},
		{ID: "000000000003", X: 130, Y: 10, W: 50, H: 50, Shape: "star", Round: 70},
		{ID: "000000000004", X: 190, Y: 10, W: 50, H: 50, Round: 12.4},
	}}
	p.Spreads = []*Spread{s}
	p.ClampGrid()
	got := make([]string, 0, len(s.Grid))
	for _, pl := range s.Grid {
		got = append(got, pl.Shape+":"+strconv.FormatFloat(pl.Round, 'g', -1, 64))
	}
	if want := []string{"oval:0", ":0", ":50", ":12"}; !slices.Equal(got, want) {
		t.Fatalf("места файла: %v, ждали %v", got, want)
	}
	// Коробки шаблонов, виньетка и стиль своего шаблона виньетки.
	boxes := ClampBoxes([]Box{{W: 0.5, H: 0.5, Shape: "oval", Round: 5}, {W: 0.5, H: 0.5, Shape: "x", Round: 99}, {W: 0.5, H: 0.5, Round: nan}})
	if len(boxes) != 3 || boxes[0].Shape != "oval" || boxes[0].Round != 0 || boxes[1].Shape != "" || boxes[1].Round != 50 || boxes[2].Round != 0 {
		t.Fatalf("ClampBoxes: %+v", boxes)
	}
	v := ClampVignette(Vignette{Tpl: "head-t-left", Pupils: 3, Shape: "oval", Round: 10})
	if v.Shape != "oval" || v.Round != 0 {
		t.Fatalf("ClampVignette: %+v", v)
	}
	if v = ClampVignette(Vignette{Tpl: "head-t-left", Pupils: 3, Shape: "x", Round: 10.6}); v.Shape != "" || v.Round != 11 {
		t.Fatalf("ClampVignette: %+v", v)
	}
	tpl := ClampTemplates([]Template{{ID: "0123456789ab", Name: "в", Scope: TplSpread, Kind: TplVignette, Boxes: []Box{{W: 0.2, H: 0.3}},
		Vig: &VigTpl{VigGeo: VigGeo{Roles: []string{RolePupil}, Pages: 2, Aspect: 2, Ratio: "1:1"}, Style: VigStyle{Shape: "oval", Round: 25}}}})
	if len(tpl) != 1 || tpl[0].Vig.Style.Shape != "oval" || tpl[0].Vig.Style.Round != 0 {
		t.Fatalf("стиль виньетки: %+v", tpl)
	}
	// Радиус — % меньшей стороны; Outline места.
	pl := Place{W: 100, H: 60, Round: 10}
	if o := pl.Outline(); o.Oval || o.Radius != 6 {
		t.Fatalf("Outline: %+v", o)
	}
	if o := (&Place{W: 40, H: 80, Shape: ShapeOval}).Outline(); !o.Oval || o.Radius != 0 {
		t.Fatalf("Outline круга: %+v", o)
	}
}

// «Скруглённое» (076) — те же 10 %, что «Скруглённые» виньетки: VIG_ROUND во фронте читается
// из исходника, чтобы два числа не разошлись. Ключ правки в файл не попадает: из файла
// "rounded" — чужая форма (прямоугольник со своим скруглением).
func TestRoundedPct(t *testing.T) {
	if RoundedPct != 10 || RoundedPct > MaxRound {
		t.Fatalf("RoundedPct = %v", RoundedPct)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "frontend", "src", "panels", "vignette", "vigform.ts"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`export const VIG_ROUND = (\d+)`).FindSubmatch(src)
	if m == nil || string(m[1]) != strconv.Itoa(RoundedPct) {
		t.Fatalf("VIG_ROUND во фронте: %q, RoundedPct %d", m, RoundedPct)
	}
	if s, r := ClampShape(ShapeRounded, 25); s != "" || r != 25 {
		t.Fatalf("ClampShape(rounded) = %q, %v", s, r)
	}
}

// Узор (048): прозрачность 0..1 с шагом 0.01, NaN и ±Inf — 0; зажим отдаёт копию (Inks не
// общий с исходником); nil — nil.
func TestClampPattern(t *testing.T) {
	if ClampPattern(nil) != nil || (*Pattern)(nil).Clone() != nil {
		t.Fatal("nil не nil")
	}
	for in, want := range map[float64]float64{
		0: 0, 0.126: 0.13, 1: 1, -1: 0, 2: 1, math.NaN(): 0, math.Inf(1): 0, math.Inf(-1): 0,
	} {
		if got := ClampPattern(&Pattern{Kind: "dots", Fade: in}).Fade; got != want {
			t.Errorf("fade %v → %v, ждали %v", in, got, want)
		}
	}
	src := &Pattern{Kind: "dots", Inks: []string{"#aabbcc"}}
	c := ClampPattern(src)
	c.Inks[0] = "#000000"
	if c == src || src.Inks[0] != "#aabbcc" {
		t.Fatal("зажим правит исходник")
	}
	if cl := src.Clone(); cl == src || !reflect.DeepEqual(cl, src) {
		t.Fatalf("копия: %+v", cl)
	} else if cl.Inks[0] = "#000000"; src.Inks[0] != "#aabbcc" {
		t.Fatal("копия делит цвета с исходником")
	}
	// Ключ не по грамматике — узор отбрасывается (решение №91).
	for _, kind := range []string{"", "Dots", "dots!", "мой", strings.Repeat("a", 33)} {
		if got := ClampPattern(&Pattern{Kind: kind, Fade: 0.5}); got != nil {
			t.Errorf("ключ %q: %+v, ждали nil", kind, got)
		}
	}
	// Остальное — gen.Clamp: цвета, масштаб, seed.
	for _, tc := range []struct {
		name string
		in   Pattern
		want Pattern
	}{
		{"цвета нижним регистром, лишние срезаны", Pattern{Kind: "dots", Inks: []string{"#AABBCC", "#112233"}},
			Pattern{Kind: "dots", Inks: []string{"#aabbcc"}}},
		{"плохой цвет — цвет генератора", Pattern{Kind: "bands", Inks: []string{"red", "#000000"}},
			Pattern{Kind: "bands", Inks: []string{"#e5d5c1", "#000000"}}},
		{"плохой цвет в хвосте — по умолчанию, не хранится", Pattern{Kind: "dots", Inks: []string{"red"}},
			Pattern{Kind: "dots"}},
		{"цвета по умолчанию не хранятся", Pattern{Kind: "dots", Inks: []string{"#D9B779"}}, Pattern{Kind: "dots"}},
		{"масштаб: шаг 0.01", Pattern{Kind: "dots", Scale: 1.234}, Pattern{Kind: "dots", Scale: 1.23}},
		{"масштаб сверх предела", Pattern{Kind: "dots", Scale: 9}, Pattern{Kind: "dots", Scale: 4}},
		{"масштаб ниже предела", Pattern{Kind: "dots", Scale: 0.01}, Pattern{Kind: "dots", Scale: 0.25}},
		{"масштаб 1 — по умолчанию", Pattern{Kind: "dots", Scale: 1}, Pattern{Kind: "dots"}},
		{"масштаб NaN", Pattern{Kind: "dots", Scale: math.NaN()}, Pattern{Kind: "dots"}},
		{"seed у неслучайного обнулён", Pattern{Kind: "dots", Seed: 7}, Pattern{Kind: "dots"}},
		{"seed у случайного остаётся", Pattern{Kind: "dots-scatter", Seed: 7}, Pattern{Kind: "dots-scatter", Seed: 7}},
		{"неизвестный ключ хранится как есть", Pattern{Kind: "zz-new", Inks: []string{"#010203"}, Scale: 2, Seed: 9, Fade: 0.3},
			Pattern{Kind: "zz-new", Inks: []string{"#010203"}, Scale: 2, Seed: 9, Fade: 0.3}},
		{"неизвестный: цвета до первого плохого", Pattern{Kind: "zz-new", Inks: []string{"#010203", "x", "#040506"}},
			Pattern{Kind: "zz-new", Inks: []string{"#010203"}}},
	} {
		got := ClampPattern(&tc.in)
		if got == nil || !reflect.DeepEqual(*got, tc.want) {
			t.Errorf("%s: %+v, ждали %+v", tc.name, got, tc.want)
		}
	}
	// Зажатый узор — неподвижная точка.
	once := ClampPattern(&Pattern{Kind: "dots-scatter", Inks: []string{"#ABCDEF", "bad"}, Scale: 3.333, Seed: 5, Fade: 0.777})
	if twice := ClampPattern(once); !reflect.DeepEqual(once, twice) {
		t.Errorf("зажим не идемпотентен: %+v → %+v", once, twice)
	}
}

// Узоры листа при чтении: плохой ключ выпадает из слота, pat вытесняет patL/R.
func TestClampPatternsSlots(t *testing.T) {
	s := &Spread{Pat: &Pattern{Kind: "BAD"}, PatL: &Pattern{Kind: "dots"}, PatR: &Pattern{Kind: "zz-new"}}
	clampPatterns(s)
	if s.Pat != nil || s.PatL == nil || s.PatR == nil || s.PatR.Kind != "zz-new" {
		t.Fatalf("плохой pat: %+v %+v %+v", s.Pat, s.PatL, s.PatR)
	}
	s = &Spread{Pat: &Pattern{Kind: "dots"}, PatL: &Pattern{Kind: "glow"}, PatR: &Pattern{Kind: "glow"}}
	clampPatterns(s)
	if s.Pat == nil || s.PatL != nil || s.PatR != nil {
		t.Fatalf("pat и patL/R: %+v %+v %+v", s.Pat, s.PatL, s.PatR)
	}
}

// SplitPat: узор листа уходит странице напротив; без узора листа и не у страницы — ничего.
func TestSplitPat(t *testing.T) {
	dots, star := &Pattern{Kind: "dots"}, &Pattern{Kind: "stars"}
	for _, c := range []struct {
		scope   string
		in      Spread
		s, l, r *Pattern
	}{
		{ScopeLeft, Spread{Pat: dots}, nil, nil, dots},
		{ScopeRight, Spread{Pat: dots}, nil, dots, nil},
		{ScopeSpread, Spread{Pat: dots}, dots, nil, nil},
		{"top", Spread{Pat: dots}, dots, nil, nil},
		{ScopeLeft, Spread{PatR: star}, nil, nil, star},
		{ScopeLeft, Spread{}, nil, nil, nil},
	} {
		s := c.in
		s.SplitPat(c.scope)
		if s.Pat != c.s || s.PatL != c.l || s.PatR != c.r {
			t.Fatalf("%s: %v %v %v", c.scope, s.Pat, s.PatL, s.PatR)
		}
	}
}

// 049: {Предмет} — зажим (одна строка ≤ MaxPersonRunes, пробельный — пусто, только предмет —
// не nil), запас знаков, подстановка у учителя и ученика, имя из файла с предметом.
func TestPersonSubject(t *testing.T) {
	long := strings.Repeat("я", MaxPersonRunes+5)
	if c := ClampPerson(Person{Last: "Иванова", Subject: " Мате\nматика "}); c == nil || c.Subject != "Мате матика" {
		t.Fatalf("строка предмета: %+v", c)
	}
	if c := ClampPerson(Person{Subject: long}); c == nil || utf8.RuneCountInString(c.Subject) != MaxPersonRunes || c.Named() {
		t.Fatalf("длинный предмет без имени: %+v", c)
	}
	if ClampPerson(Person{Subject: " \t "}) != nil || (Person{Subject: "Химия"}).Named() || !(Person{Middle: "И"}).Named() {
		t.Fatal("пробельный предмет — пустое имя; предмет — не имя")
	}
	if n := (Person{Last: "аб", Subject: "вгд"}).runes(); n != 5 {
		t.Fatalf("запас знаков без предмета: %d", n)
	}
	b := textBudget{runes: 4}
	if clampPersonIn(&Person{Last: "аб", Subject: "вгд"}, &b) != nil {
		t.Fatal("предмет сверх запаса знаков не списан")
	}
	teacher := &Person{Last: "Сидорова", First: "Ольга", Middle: "Ивановна", Subject: "Математика"}
	pupil := &Person{Last: "Козлов", First: "Иван"}
	vars := Vars{Title: "Школа № 5", Subtitle: "11 «А»", Lead: "Мария Ивановна"}
	for _, c := range []struct {
		in, want string
		ctx      FieldCtx
	}{
		{StaffCaption, "Сидорова\nОльга Ивановна\nМатематика", FieldCtx{Person: teacher, Mode: FieldPrint}},
		{StaffCaption, "Козлов\nИван ", FieldCtx{Person: pupil, Mode: FieldPrint}}, // пустая строка предмета снята
		{StaffCaption, "Козлов\nИван [Отчество]\n[Предмет]", FieldCtx{Person: pupil}},
		{StaffCaption, "[Фамилия]\n[Имя] [Отчество]\n[Предмет]", FieldCtx{Person: teacher, Mode: FieldStub}},
		{"{предмет}|{ПРЕДМЁТ}", "Математика|Математика", FieldCtx{Person: teacher}},
		{"{Школа}|{Класс}|{Учитель}|{Заголовок}|{Подзаголовок}|{Руководитель}",
			"Школа № 5|11 «А»|Мария Ивановна|Школа № 5|11 «А»|Мария Ивановна", FieldCtx{Vars: vars, Person: teacher}},
	} {
		if got, _ := Expand(c.in, c.ctx); got != c.want {
			t.Errorf("%q (режим %d): %q, ждали %q", c.in, c.ctx.Mode, got, c.want)
		}
	}
	if _, miss := Expand(StaffCaption, FieldCtx{Person: pupil, Mode: FieldPrint}); !slices.Equal(miss, []string{"Отчество", "Предмет"}) {
		t.Fatalf("пустые поля: %v", miss)
	}
	// Имя из файла не даёт предмета; свой только предмет — к имени из файла.
	if p := PersonFromFile(`D:\x\Иванова Мария Петровна.jpg`); p == nil || p.Subject != "" {
		t.Fatalf("из файла: %+v", p)
	}
	r := PhotoRef{Path: `D:\x\Иванова Мария.jpg`, Person: &Person{Subject: "Химия"}}
	if who, file := r.Name(); who == nil || !file || who.Last != "Иванова" || who.Subject != "Химия" || r.Person.Last != "" {
		t.Fatalf("предмет к имени из файла: %+v %v", who, file)
	}
	r.Path = `D:\x\DSC_0001.jpg`
	if who, file := r.Name(); who == nil || file || who.Subject != "Химия" {
		t.Fatalf("предмет без имени: %+v %v", who, file)
	}
	if v := NameView(r); v.Subject != "Химия" || v.FromFile || v.Teacher {
		t.Fatalf("карточка: %+v", v)
	}
}
