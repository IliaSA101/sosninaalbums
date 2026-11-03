package text

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"log"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Строки своих шрифтов несут DX и рисуются по нему (161 G2).

// emMM — кегль в мм: DX в мм делится на него, чтобы сравнивать с единицами шрифта.
func emMM(pt float64) float64 { return pt * 25.4 / 72 }

// customStyle регистрирует встроенный файл как свою гарнитуру и отдаёт стиль на 100 pt.
func customStyle(t *testing.T, file string) Style {
	t.Helper()
	fc, b := face(t, file, false, false)
	name := "Свой " + file
	setCustom(t, []CustomFamily{{ID: FamilyID(name), Name: name, Faces: []CustomFace{fc}}}, loader(b))
	return Style{Font: FamilyID(name), Size: 100, Color: "#000000", Align: "left"}
}

// dxOf — DX первой строки текста s свёрнутого в одну строку.
func dxOf(t *testing.T, s string, st Style) []float64 {
	t.Helper()
	b := Layout(s, st, 5000)
	if len(b.Lines) != 1 {
		t.Fatalf("%q: строк %d", s, len(b.Lines))
	}
	return b.Lines[0].DX
}

// perMille — сдвиг перед знаком i в тысячных em.
func perMille(dx []float64, i int, pt float64) float64 { return dx[i] / emMM(pt) * 1000 }

func TestGPOSKernCyrl(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	for _, c := range []struct {
		s    string
		want float64
	}{{"То", -49}, {"'А", -49}, {".у", -46}} {
		dx := dxOf(t, c.s, st)
		if got := perMille(dx, 1, st.Size); math.Abs(got-c.want) > 0.1 {
			t.Errorf("«%s»: %.2f/1000 em, ждали %.0f (dx %v)", c.s, got, c.want, dx)
		}
	}
}

// Цифры и знаки берут скрипт соседа слева, в начале абзаца — справа.
func TestGPOSKernDigitsScript(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	cases := []struct {
		s    string
		i    int
		want float64
	}{
		{".у", 1, -46},  // начало абзаца: знак берёт cyrl от «у»
		{"а.у", 2, -46}, // слева кириллица
		{"a.у", 2, 0},   // слева латиница: «.» и «у» в разных скриптах
	}
	for _, c := range cases {
		if got := perMille(dxOf(t, c.s, st), c.i, st.Size); math.Abs(got-c.want) > 0.1 {
			t.Errorf("«%s»: %.2f, ждали %.0f", c.s, got, c.want)
		}
	}
}

func TestGPOSKernCrossScript(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	if dx := dxOf(t, "ТV", st); dx[1] != 0 {
		t.Errorf("«ТV» кернится: %v", dx)
	}
	rc := customStyle(t, "RobotoCondensed-Regular.ttf")
	if dx := dxOf(t, "V Т", rc); slices.ContainsFunc(dx, func(v float64) bool { return v != 0 }) {
		t.Errorf("«V Т» кернится: %v", dx)
	}
}

// Шрифт без GPOS (таблица kern): DX не пуст и равен sfnt.Kern.
func TestGPOSNoTableUsesSfnt(t *testing.T) {
	st := customStyle(t, "RuslanDisplay-Regular.ttf")
	m, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	sf := m["RuslanDisplay-Regular.ttf"]
	var buf sfnt.Buffer
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6
	for _, a := range "АБВГДЕЖЗИКЛМНОПРСТУФХЦЧШЩЭЮЯабвгдежзиклмнопрстуфхцчшщъыьэюя" {
		for _, b := range "АБВГДЕЖЗИКЛМНОПРСТУФХЦЧШЩЭЮЯабвгдежзиклмнопрстуфхцчшщъыьэюя" {
			ga, _ := sf.GlyphIndex(&buf, a)
			gb, _ := sf.GlyphIndex(&buf, b)
			k, err := sf.Kern(&buf, ga, gb, unit, xfont.HintingNone)
			if err != nil || k == 0 {
				continue
			}
			want := float64(k) / 64 / float64(sf.UnitsPerEm()) * emMM(st.Size)
			if dx := dxOf(t, string([]rune{a, b}), st); len(dx) != 2 || math.Abs(dx[1]-want) > 1e-3 {
				t.Fatalf("«%c%c»: dx %v, sfnt.Kern %.4f мм", a, b, dx, want)
			}
			return
		}
	}
	t.Fatal("в Ruslan Display нет кириллической пары с кернингом kern")
}

func TestLineDXCustomOnly(t *testing.T) {
	for _, l := range Layout("Тот Тото то", Style{Font: "lora", Size: 20}, 300).Lines {
		if l.DX != nil {
			t.Errorf("встроенный шрифт несёт DX: %v", l.DX)
		}
	}
	st := customStyle(t, "Lora-Regular.ttf")
	st.Size = 20
	box := Layout("Тото Тото Тото Тото Тото\n\nТото", st, 40)
	if len(box.Lines) < 4 {
		t.Fatalf("строк %d", len(box.Lines))
	}
	for _, l := range box.Lines {
		switch {
		case l.Text == "" && l.DX != nil:
			t.Errorf("у пустой строки DX %v", l.DX)
		case l.Text != "" && (len(l.DX) != utf8.RuneCountInString(l.Text) || l.DX[0] != 0):
			t.Errorf("строка %q: DX %v", l.Text, l.DX)
		}
	}
}

// Вне BMP знаки без глифа выпадают из Text — длина DX считается по рунам Text; U+0306 и U+0301
// занимают своё место. (𝐀 и U+FE0F: ни в одном встроенном шрифте нет глифа — см. отчёт.)
func TestLineDXRunes(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	st.Size = 40
	box := Layout("То̆То́ 😀 То", st, 5000)
	l := box.Lines[0]
	if strings.Contains(l.Text, "😀") {
		t.Fatalf("эмодзи без глифа остался в строке: %q", l.Text)
	}
	if n := utf8.RuneCountInString(l.Text); len(l.DX) != n || n < 8 {
		t.Fatalf("рун %d, DX %d: %q", n, len(l.DX), l.Text)
	}
}

func TestCachedCopiesDX(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	a := Cached("Тото", st, 300)
	want := slices.Clone(a.Lines[0].DX)
	a.Lines[0].DX[1] = 99
	b := Cached("Тото", st, 300)
	if !slices.Equal(b.Lines[0].DX, want) {
		t.Fatalf("кэш делит DX с вызывающим: %v, ждали %v", b.Lines[0].DX, want)
	}
}

// Знак из Montserrat (₽ у Playfair): на стыке с основным шрифтом кернинга нет, между знаками
// запасного шрифта — кернинг Montserrat (sfnt), как в 0.35.
func TestLineDXFallbackRun(t *testing.T) {
	st := customStyle(t, "PlayfairDisplay-Regular.ttf")
	st.Size = 40
	dx := dxOf(t, "Т₽о", st)
	if len(dx) != 3 || dx[1] != 0 || dx[2] != 0 {
		t.Errorf("кернинг на стыке с запасным шрифтом: %v", dx)
	}
	ref := Style{Font: DefaultFont, Size: 40}
	got, want := dxOf(t, "₽₽", st), Layout("₽₽", ref, 5000).Lines[0].W
	if w := Layout("₽₽", st, 5000).Lines[0].W; len(got) != 2 || math.Abs(w-want) > 1e-3 {
		t.Errorf("пара запасных знаков: ширина %.4f, у Montserrat %.4f (dx %v)", w, want, got)
	}
}

// glyphXs = позиции measure ± 0,001 мм; бокс без DX кернится по самой строке.
func TestDrawUsesLineDX(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	st.Size = 30
	box := Layout("ТАТЬЯНА и ТИМОФЕЙ 12.10.2026 То .у", st, 5000)
	l := box.Lines[0]
	sf, _, d := fontData(st.Clamped())
	if d == nil {
		t.Fatal("нет данных начертания")
	}
	p := measureCustom(sf, nil, d, l.Text)
	const pxmm = 10.0
	var buf sfnt.Buffer
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6
	var adv []float64
	for _, r := range l.Text {
		g, _ := sf.GlyphIndex(&buf, r)
		a, _ := sf.GlyphAdvance(&buf, g, unit, xfont.HintingNone)
		adv = append(adv, float64(a)/64/float64(sf.UnitsPerEm())*emMM(st.Size)*pxmm)
	}
	xs := glyphXs(l, adv, pxmm)
	for i := range xs {
		want := float64(p.x[i]) / 64 / float64(sf.UnitsPerEm()) * emMM(st.Size)
		if math.Abs(xs[i]/pxmm-want) > 0.001 {
			t.Fatalf("знак %d: %.4f мм, measure %.4f мм", i, xs[i]/pxmm, want)
		}
	}
	// бокс без DX: те же позиции по самой строке
	bare := l
	bare.DX = nil
	_, xs2 := placeCustom(glyphSrc{sf, &buf}, glyphSrc{}, d, bare, unit, pxmm, emMM(st.Size)*pxmm)
	for i := range xs2 {
		if math.Abs(xs2[i]-xs[i]) > 0.01 {
			t.Fatalf("без DX знак %d: %.4f px, с DX %.4f", i, xs2[i], xs[i])
		}
	}
}

// Пиксельный: Lora 60 pt «ТоТо» — правый край чернил с DX левее, чем при обнулённых, на ΣDX ± 1 px.
func TestDrawKernShifts(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	st.Size = 60
	box := Layout("ТоТо", st, 5000)
	sum := 0.0
	for _, v := range box.Lines[0].DX {
		sum += v
	}
	if sum >= 0 {
		t.Fatalf("ΣDX %.4f: ждали отрицательную", sum)
	}
	const dpi = 300
	render := func(b Box) image.Rectangle {
		img := image.NewRGBA(image.Rect(0, 0, 1500, 500))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		Draw(img, b, st, 5, 5, 0, dpi)
		return inkBounds(img)
	}
	zero := box
	zero.Lines = slices.Clone(box.Lines)
	zero.Lines[0].DX = make([]float64, len(box.Lines[0].DX))
	with, without := render(box), render(zero)
	if shift := float64(without.Max.X-with.Max.X) - (-sum * dpi / 25.4); math.Abs(shift) > 1 {
		t.Errorf("край чернил сдвинут на %d px, ждали %.1f", without.Max.X-with.Max.X, -sum*dpi/25.4)
	}
}

// Отказ GPOS: одна строка журнала с текстом причины, поколение +1 один раз, текст рисуется.
func TestKernOffLogsOnce(t *testing.T) {
	st := customStyle(t, "Lora-Regular.ttf")
	st.Size = 30
	_, _, d := fontData(st.Clamped())
	var logs bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prev) })
	gen := Gen()
	d.disableKern(errGPOS)
	d.disableKern(errGPOS)
	if strings.Count(logs.String(), "кернинг GPOS не прочитан") != 1 || Gen() != gen+1 {
		t.Fatalf("журнал %q, поколение %d → %d", logs.String(), gen, Gen())
	}
	dx := dxOf(t, "То", st)
	if len(dx) != 2 || dx[1] != 0 {
		t.Errorf("после отказа ждали DX без кернинга: %v", dx)
	}
}

// Импорт идёт GPOS-путём под своим сроком: злая таблица не отклоняет файл и не вешает разбор.
func TestInspectRunsGPOS(t *testing.T) {
	lora := builtinBytes(t, "Lora-Regular.ttf")
	orig := trialKern
	t.Cleanup(func() { trialKern = orig })
	var calls int
	var last error
	trialKern = func(sf *sfnt.Font, gpos []byte) error {
		calls++
		last = orig(sf, gpos)
		return last
	}
	if _, err := Inspect(lora); err != nil || calls != 1 || last != nil {
		t.Fatalf("настоящий GPOS: ошибка %v, вызовов %d, отказ GPOS %v", err, calls, last)
	}
	evil := withTables(t, lora, map[string][]byte{"GPOS": bombFeatures(64).build().b})
	start := time.Now()
	_, err := Inspect(evil)
	if err != nil || calls != 2 || !errors.Is(last, errGPOSBudget) {
		t.Fatalf("злой GPOS: ошибка %v, вызовов %d, отказ GPOS %v", err, calls, last)
	}
	if el, limit := time.Since(start), testkit.Limit(5*time.Second); el > limit {
		t.Errorf("разбор шёл %s (> %s)", el, limit)
	}
}
