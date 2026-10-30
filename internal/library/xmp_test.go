package library

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const (
	xmpHead = `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">`
	xmpTail = `</rdf:RDF></x:xmpmeta>`
	nsDecl  = `xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:xmpDM="http://ns.adobe.com/xmp/1.0/DynamicMedia/"`
)

// xmpAttrs — пакет с отметками атрибутами rdf:Description.
func xmpAttrs(attrs string) []byte {
	return []byte(xmpHead + `<rdf:Description rdf:about="" ` + nsDecl + ` ` + attrs + `/>` + xmpTail)
}

func readXMPFile(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "xmp", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseXMPFiles(t *testing.T) {
	cases := []struct {
		file string
		want Marks
		f    markFields
	}{
		{"lr-classic-en.xmp", Marks{Rating: 4, Label: "red"}, fieldRating | fieldLabel},
		{"lr-classic-ru.xmp", Marks{Rating: 3, Label: "green"}, fieldRating | fieldLabel},
		{"lr-classic-pick.xmp", Marks{Rating: 5, Label: "purple", Flag: 1}, fieldRating | fieldLabel | fieldFlag},
		{"bridge-reject.xmp", Marks{Label: "green", Flag: -1}, fieldRating | fieldLabel | fieldFlag | fieldRejectByRating},
		{"elements.xmp", Marks{Rating: 2, Label: "blue", Flag: -1}, fieldRating | fieldLabel | fieldFlag},
		{"custom-label.xmp", Marks{Rating: 1, Label: "other", LabelText: "Клиенту"}, fieldRating | fieldLabel},
		{"foreign-prefix.xmp", Marks{Rating: 3, Flag: -1}, fieldRating | fieldFlag},
	}
	for _, c := range cases {
		m, f, err := parseXMP(readXMPFile(t, c.file))
		if err != nil || m != c.want || f != c.f {
			t.Errorf("%s: %+v %b %v, want %+v %b", c.file, m, f, err, c.want, c.f)
		}
	}
	for _, name := range []string{"broken.xmp", "doctype.xmp"} {
		m, f, err := parseXMP(readXMPFile(t, name))
		if err == nil || m != (Marks{}) || f != 0 {
			t.Errorf("%s: %+v %b %v, want нули и ошибку", name, m, f, err)
		}
	}
}

func TestParseXMPAttributes(t *testing.T) {
	m, f, err := parseXMP(xmpAttrs(`xmp:Rating="4" xmp:Label="Red"`))
	if err != nil || m != (Marks{Rating: 4, Label: "red"}) || f != fieldRating|fieldLabel {
		t.Fatalf("%+v %b %v", m, f, err)
	}
}

func TestParseXMPElements(t *testing.T) {
	p := xmpHead + `<rdf:Description rdf:about="" ` + nsDecl + `>` +
		`<xmp:Rating>4</xmp:Rating><xmp:Label>Red</xmp:Label></rdf:Description>` + xmpTail
	m, f, err := parseXMP([]byte(p))
	if err != nil || m != (Marks{Rating: 4, Label: "red"}) || f != fieldRating|fieldLabel {
		t.Fatalf("%+v %b %v", m, f, err)
	}
	// Текст вложенных элементов значением не считается.
	p = xmpHead + `<rdf:Description ` + nsDecl + `><xmp:Label><rdf:Alt><rdf:li>Red</rdf:li></rdf:Alt></xmp:Label></rdf:Description>` + xmpTail
	if m, _, err := parseXMP([]byte(p)); err != nil || m.Label != "" {
		t.Fatalf("вложенный текст: %+v %v", m, err)
	}
}

func TestParseXMPNamespace(t *testing.T) {
	cases := []struct {
		name, p string
		rating  int
	}{
		{"xap", xmpHead + `<rdf:Description xmlns:xap="http://ns.adobe.com/xap/1.0/" xap:Rating="2"/>` + xmpTail, 2},
		{"foo", xmpHead + `<rdf:Description xmlns:foo="http://ns.adobe.com/xap/1.0/" foo:Rating="3"/>` + xmpTail, 3},
		{"элемент foo", xmpHead + `<rdf:Description xmlns:foo="http://ns.adobe.com/xap/1.0/"><foo:Rating>5</foo:Rating></rdf:Description>` + xmpTail, 5},
		{"чужой URI", xmpHead + `<rdf:Description xmlns:xmp="http://example.com/" xmp:Rating="4"/>` + xmpTail, 0},
		{"без объявления", xmpHead + `<rdf:Description xmp:Rating="4"/>` + xmpTail, 0},
		{"по умолчанию", xmpHead + `<Rating xmlns="http://ns.adobe.com/xap/1.0/">1</Rating>` + xmpTail, 1},
	}
	for _, c := range cases {
		m, _, err := parseXMP([]byte(c.p))
		if err != nil || m.Rating != c.rating {
			t.Errorf("%s: %+v %v, want Rating %d", c.name, m, err, c.rating)
		}
	}
}

func TestParseXMPLabels(t *testing.T) {
	long := strings.Repeat("я", 100)
	cases := []struct{ in, color, text string }{
		{"Red", "red", ""},
		{"yellow", "yellow", ""},
		{"  GREEN ", "green", ""},
		{"Blue", "blue", ""},
		{"Purple", "purple", ""},
		{"Красный", "red", ""},
		{"Желтый", "yellow", ""},
		{"Жёлтый", "yellow", ""},
		{"ЗЕЛЁНЫЙ", "green", ""},
		{"синий", "blue", ""},
		{"Фиолетовый", "purple", ""},
		{"Select", "red", ""},
		{"Second", "yellow", ""},
		{"Approved", "green", ""},
		{"Review", "blue", ""},
		{"To Do", "purple", ""},
		{"To Delete", "red", ""},
		{"Color Correction Needed", "yellow", ""},
		{"Good to Use", "green", ""},
		{"Retouching Needed", "blue", ""},
		{"To Print", "purple", ""},
		{"Клиенту", "other", "Клиенту"},
		{" Клиенту ", "other", "Клиенту"},
		{long, "other", strings.Repeat("я", 64)},
		{"a&#x9;b&#x7f;c&#x85;d&#x202e;e", "other", "abcde"},
		{"", "", ""},
		{"   ", "", ""},
	}
	for _, c := range cases {
		m, f, err := parseXMP(xmpAttrs(`xmp:Label="` + c.in + `"`))
		if err != nil || m.Label != c.color || m.LabelText != c.text || f != fieldLabel {
			t.Errorf("%q: %+v %b %v, want %q %q", c.in, m, f, err, c.color, c.text)
		}
	}
	// Неверный UTF-8 и пробел нулевой ширины не проходят в текст метки.
	if got := cleanLabel(string(rune(0x200b)) + "ab\xffc"); got != "abc" {
		t.Errorf("неверный UTF-8 и форматирующие: %q", got)
	}
	if c, k := labelColor("Ёлка"); c != "other" || k != "Ёлка" {
		t.Errorf("labelColor сохраняет исходный текст: %q %q", c, k)
	}
}

func TestParseXMPReject(t *testing.T) {
	cases := []struct {
		attrs string
		want  Marks
		f     markFields
	}{
		{`xmp:Rating="-1"`, Marks{Flag: -1}, fieldRating | fieldFlag | fieldRejectByRating},
		{`xmp:Rating="3.0"`, Marks{Rating: 3}, fieldRating},
		{`xmp:Rating=" 5 "`, Marks{Rating: 5}, fieldRating},
		{`xmp:Rating="0"`, Marks{}, fieldRating},
		{`xmp:Rating="7"`, Marks{}, 0},
		{`xmp:Rating="abc"`, Marks{}, 0},
		{`xmp:Rating="NaN"`, Marks{}, 0},
		{`xmp:Rating="-2"`, Marks{}, 0},
		{`xmp:Rating="-1" xmpDM:pick="1"`, Marks{Flag: 1}, fieldRating | fieldFlag},
		{`xmp:Rating="-1" xmpDM:pick="0"`, Marks{}, fieldRating | fieldFlag},
		{`xmp:Rating="-1" xmpDM:pick="x"`, Marks{Flag: -1}, fieldRating | fieldFlag | fieldRejectByRating},
	}
	for _, c := range cases {
		m, f, err := parseXMP(xmpAttrs(c.attrs))
		if err != nil || m != c.want || f != c.f {
			t.Errorf("%s: %+v %b %v, want %+v %b", c.attrs, m, f, err, c.want, c.f)
		}
	}
}

func TestParseXMPPick(t *testing.T) {
	cases := []struct {
		attrs string
		flag  int
		f     markFields
	}{
		{`xmpDM:pick="1"`, 1, fieldFlag},
		{`xmpDM:pick="-1"`, -1, fieldFlag},
		{`xmpDM:pick="0"`, 0, fieldFlag},
		{`xmpDM:pick="2"`, 0, 0},
		{`xmpDM:good="true"`, 0, 0},
		{`xmpDM:good="false" xmpDM:pick="1"`, 1, fieldFlag},
	}
	for _, c := range cases {
		m, f, err := parseXMP(xmpAttrs(c.attrs))
		if err != nil || m.Flag != c.flag || f != c.f {
			t.Errorf("%s: %+v %b %v, want flag %d %b", c.attrs, m, f, err, c.flag, c.f)
		}
	}
}

func TestParseXMPHostile(t *testing.T) {
	deep := strings.Repeat("<a>", 1000) + `<b xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5"/>` + strings.Repeat("</a>", 1000)
	wide := xmpHead + strings.Repeat("<a/>", 200_000) + `<b xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5"/>` + xmpTail
	full := string(xmpAttrs(`xmp:Rating="5" xmp:Label="Red"`))
	// 1 МБ атрибутов в одном теге: без ошибки и без отметок, разбор линейный (design §1.5).
	var attrs strings.Builder
	attrs.WriteString(xmpHead + `<rdf:Description `)
	for i := 0; attrs.Len() < maxXMPBytes-100; i++ {
		fmt.Fprintf(&attrs, `a%d="Rating" `, i)
	}
	attrs.WriteString(`/>` + xmpTail)
	anyErr := errors.New("любая ошибка")
	cases := map[string]struct {
		p   string
		err error // nil — без ошибки, anyErr — любая
	}{
		"doctype":    {`<!DOCTYPE x><x xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5"/>`, errXMPDirective},
		"entity":     {`<!DOCTYPE x [<!ENTITY e "Red">]><x xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5" xmp:Label="&e;"/>`, errXMPDirective},
		"внешняя":    {`<x xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5" xmp:Label="&ext;"/>`, anyErr},
		"глубина":    {deep, errXMPDepth},
		"ширина":     {wide, errXMPTokens},
		"атрибуты":   {attrs.String(), nil},
		"обрезан":    {full[:len(full)-20], anyErr},
		"мусор":      {"\x00\xff<<<Rating>>>&&&Label<pick", anyErr},
		"пусто":      {"", nil},
		"переросток": {string(xmpAttrs(`xmp:Rating="5"`)) + strings.Repeat(" ", maxXMPBytes), errXMPSize},
	}
	// Время — только защита от зависания и квадратичного разбора (пределы срабатывают
	// за миллисекунды, обычно всё ~0,07 с). Абсолютный порог падал при параллельных
	// прогонах (1,68 с), поэтому порог масштабируется (testkit.Limit), а берётся
	// лучший из трёх прогонов: всплеск нагрузки портит один, квадратичный разбор — все.
	limit := testkit.Limit(500 * time.Millisecond)
	for name, c := range cases {
		var m Marks
		var f markFields
		var err error
		d := testkit.Fastest(3, func() { m, f, err = parseXMP([]byte(c.p)) })
		if d > limit {
			t.Errorf("%s: %v, дольше %v", name, d, limit)
		}
		if m != (Marks{}) || f != 0 {
			t.Errorf("%s: %+v %b, want нули", name, m, f)
		}
		switch {
		case c.err == nil && err != nil, c.err != nil && err == nil:
			t.Errorf("%s: ошибка %v, ждали %v", name, err, c.err)
		case c.err != nil && c.err != anyErr && !errors.Is(err, c.err):
			t.Errorf("%s: ошибка %v, ждали %v", name, err, c.err)
		}
	}
}

// Текст ошибки encoding/xml несёт имена из файла: в log.txt он обрезан и экранирован.
func TestParseXMPErrorQuoted(t *testing.T) {
	long := strings.Repeat("щ", 50_000)
	p := xmpHead + `<rdf:Description ` + nsDecl + ` xmp:Rating="1"><` + long + `></b>` + xmpTail
	_, _, err := parseXMP([]byte(p))
	if err == nil {
		t.Fatal("нет ошибки")
	}
	if s := err.Error(); utf8.RuneCountInString(s) > 300 || !strings.Contains(s, "щщщ") {
		t.Errorf("текст ошибки %d знаков: %.100s", utf8.RuneCountInString(s), s)
	}
	// Перевод строки и смена направления письма не попадают в лог как есть.
	rlo := string(rune(0x202e))
	if s := quoteErr(errors.New("a\nb" + rlo + "c")); strings.ContainsAny(s, "\r\n"+rlo) {
		t.Errorf("не экранировано: %s", s)
	}
}

// Значение длиннее maxXMPText и вложенная структура — поля нет, атрибутом и элементом одинаково.
func TestParseXMPLongValue(t *testing.T) {
	elem := func(prop, v string) []byte {
		return []byte(xmpHead + `<rdf:Description ` + nsDecl + `><xmp:` + prop + `>` + v + `</xmp:` + prop + `></rdf:Description>` + xmpTail)
	}
	cases := []struct {
		name, prop, v string
		want          Marks
		f             markFields
	}{
		{"метка 4 КБ", "Label", strings.Repeat("я", maxXMPText/2), Marks{Label: "other", LabelText: strings.Repeat("я", maxLabelLen)}, fieldLabel},
		{"метка длиннее", "Label", strings.Repeat("я", maxXMPText/2) + "x", Marks{}, 0},
		{"оценка 4 КБ", "Rating", strings.Repeat(" ", maxXMPText-1) + "3", Marks{Rating: 3}, fieldRating},
		{"оценка длиннее", "Rating", strings.Repeat(" ", maxXMPText) + "3", Marks{}, 0},
	}
	for _, c := range cases {
		ma, fa, erra := parseXMP(xmpAttrs(`xmp:` + c.prop + `="` + c.v + `"`))
		me, fe, erre := parseXMP(elem(c.prop, c.v))
		if erra != nil || erre != nil || ma != c.want || fa != c.f || me != c.want || fe != c.f {
			t.Errorf("%s: атрибут %+v %b %v, элемент %+v %b %v; ждали %+v %b", c.name, ma, fa, erra, me, fe, erre, c.want, c.f)
		}
	}
}

// Нечитаемая метка — поля нет (сайдкар не снимает метку из JPEG); пустая — явное снятие.
func TestParseXMPLabelField(t *testing.T) {
	desc := func(inner string) []byte {
		return []byte(xmpHead + `<rdf:Description ` + nsDecl + `>` + inner + `</rdf:Description>` + xmpTail)
	}
	cases := []struct {
		name string
		p    []byte
		f    markFields
	}{
		{"пустой атрибут", xmpAttrs(`xmp:Label=""`), fieldLabel},
		{"пустой элемент", desc(`<xmp:Label> </xmp:Label>`), fieldLabel},
		{"только управляющие", xmpAttrs(`xmp:Label="&#x202e;&#x7f;"`), 0},
		{"rdf:Alt", desc(`<xmp:Label><rdf:Alt><rdf:li>Red</rdf:li></rdf:Alt></xmp:Label>`), 0},
		{"rdf:Alt с пробелами", desc("<xmp:Label>\n <rdf:Alt/>\n</xmp:Label>"), 0},
		{"пустой вложенный", desc(`<xmp:Label><x/></xmp:Label>`), 0},
	}
	for _, c := range cases {
		m, f, err := parseXMP(c.p)
		if err != nil || m != (Marks{}) || f != c.f {
			t.Errorf("%s: %+v %b %v, ждали поле %b", c.name, m, f, err, c.f)
		}
	}
}

// xmpScan: текст свойства собирается из нескольких токенов (CDATA), предел — на сумму;
// значением считается только первое вхождение; глубина больше предела — ошибка.
func TestXMPScanTokens(t *testing.T) {
	m, f, err := parseXMP([]byte(xmpHead + `<rdf:Description ` + nsDecl + `><xmp:Label>Re<![CDATA[d]]></xmp:Label>` +
		`<xmp:Label>Blue</xmp:Label></rdf:Description>` + xmpTail))
	if err != nil || m != (Marks{Label: "red"}) || f != fieldLabel {
		t.Errorf("CDATA и второе вхождение: %+v %b %v", m, f, err)
	}

	var x xmpScan
	if err := x.start(xml.StartElement{Name: xml.Name{Space: nsXMP, Local: "Label"}}); err != nil {
		t.Fatal(err)
	}
	x.chars(xml.CharData(strings.Repeat("a", maxXMPText)))
	x.chars(xml.CharData("b")) // сумма больше предела
	x.end()
	if !x.raw.seen[propLabel] || !x.raw.bad[propLabel] || x.raw.val[propLabel] != "" || x.depth != 0 {
		t.Errorf("длинный текст по частям: %+v, глубина %d", x.raw, x.depth)
	}

	var deep xmpScan
	for i := range maxXMPDepth {
		if err := deep.start(xml.StartElement{}); err != nil {
			t.Fatalf("уровень %d: %v", i+1, err)
		}
	}
	if err := deep.start(xml.StartElement{}); !errors.Is(err, errXMPDepth) {
		t.Errorf("глубина %d: %v", maxXMPDepth+1, err)
	}
}

func FuzzParseXMP(f *testing.F) {
	for _, name := range []string{"lr-classic-ru.xmp", "lr-classic-pick.xmp", "bridge-reject.xmp",
		"elements.xmp", "custom-label.xmp", "foreign-prefix.xmp", "broken.xmp", "doctype.xmp"} {
		f.Add(readXMPFile(f, name))
	}
	f.Add(xmpAttrs(`xmp:Rating="4" xmp:Label="Red" xmpDM:pick="-1"`))
	f.Fuzz(func(t *testing.T, p []byte) {
		m, fl, err := parseXMP(p)
		if errors.Is(err, errXMPPanic) { // recover спасает сканер, но паника — ошибка в парсере
			t.Fatalf("паника: %v", err)
		}
		if err != nil && (m != (Marks{}) || fl != 0) {
			t.Fatalf("ошибка, но не нули: %+v %b", m, fl)
		}
		if m.Rating < 0 || m.Rating > 5 || m.Flag < -1 || m.Flag > 1 {
			t.Fatalf("вне диапазона: %+v", m)
		}
		switch m.Label {
		case "", "red", "yellow", "green", "blue", "purple":
			if m.LabelText != "" {
				t.Fatalf("текст у цвета: %+v", m)
			}
		case "other":
			if m.LabelText == "" || utf8.RuneCountInString(m.LabelText) > maxLabelLen || !utf8.ValidString(m.LabelText) {
				t.Fatalf("текст метки: %q", m.LabelText)
			}
		default:
			t.Fatalf("метка: %q", m.Label)
		}
	})
}

// xmp60K — пакет Lightroom с историей обработки, раздутый до ~60 КБ.
func xmp60K(tb testing.TB) []byte { return xmpSized(tb, 60_000) }

// xmpSized — пакет Lightroom с историей обработки, раздутый до ~n байт.
func xmpSized(tb testing.TB, n int) []byte {
	p := string(readXMPFile(tb, "lr-classic-en.xmp"))
	j := strings.Index(p, "</rdf:Seq>")
	i := strings.Index(p, "     <rdf:li")
	li := p[i : i+strings.Index(p[i:], "/>\n")+3] // одна запись истории
	var sb strings.Builder
	sb.WriteString(p[:j])
	for sb.Len()+len(li)+len(p)-j <= n {
		sb.WriteString(li)
	}
	sb.WriteString(p[j:])
	return []byte(sb.String())
}

// Порог из design §1.5: 3 мс на 60 КБ.
func BenchmarkParseXMP(b *testing.B) {
	p := xmp60K(b)
	if m, _, err := parseXMP(p); err != nil || m.Rating != 4 {
		b.Fatalf("%+v %v", m, err)
	}
	b.SetBytes(int64(len(p)))
	b.ReportAllocs()
	for b.Loop() {
		_, _, _ = parseXMP(p)
	}
}
