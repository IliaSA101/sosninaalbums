package printprof

import (
	"strings"
	"testing"
)

// Кириллические адреса (123 Б3, 0.26-123-52): файл по-прежнему только ASCII, показ хоста человеку
// раскрывает xn--… лишь для безопасной метки.

// Разбор файла: кириллический хост — отказ, xn-- — можно (окно переводит в xn-- само).
func TestIdnHostFileStrict(t *testing.T) {
	for _, u := range []string{"https://фабрика.рф/", "https://фабрика.рф:443/x", "https://Фабрика.рф/", "https://exаmple.com/"} {
		if err := CheckURL(u); err == nil {
			t.Errorf("%s принят разбором", u)
		}
	}
	if err := CheckURL("https://xn--80aac2ak8aza.xn--p1ai/"); err != nil {
		t.Errorf("xn--: %v", err)
	}
}

// Целый профиль с кириллическим хостом: Parse по-прежнему отказ, с xn-- — принимается, SiteHost — Юникод.
func TestIdnHostFileProfile(t *testing.T) {
	p := fabrikaCopy(t)
	p.Export.URL = "https://фабрика.рф/"
	for i := range p.Products {
		p.Products[i].URL = ""
	}
	if _, err := Parse(mustJSON(t, p)); err == nil || !strings.Contains(err.Error(), "export.url") {
		t.Errorf("кириллический хост принят: %v", err)
	}
	p.Export.URL = "https://xn--80aac2ak8aza.xn--p1ai/"
	back, err := Parse(mustJSON(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if h := back.SiteHost(); h != "фабрика.рф" {
		t.Errorf("SiteHost %q", h)
	}
	p.Export.URL = "https://xn--80ak6aa92e.com/" // «аррӏе»
	if h := p.SiteHost(); h != "xn--80ak6aa92e.com" {
		t.Errorf("двойник показан Юникодом: %q", h)
	}
}

func TestDisplayHost(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"xn--80aac2ak8aza.xn--p1ai", "фабрика.рф"},
		{"XN--80AAC2AK8AZA.XN--P1AI", "фабрика.рф"},
		{"fabrika.com", "fabrika.com"},
		{"WWW.Fabrika.com", "www.fabrika.com"},
		{"www.xn--80aac2ak8aza.xn--p1ai", "www.фабрика.рф"},
		{"xn--80aac2ak8aza.com", "фабрика.com"},      // метки независимы: каждая однописьменная
		{"xn--e1afmkfd.xn--p1ai", "пример.рф"},       // «пример»
		{"xn--80ak6aa92e.com", "xn--80ak6aa92e.com"}, // «аррӏе» — вся из двойников латиницы
		{"xn--", "xn--"},
		{"xn--a.com", "xn--a.com"},                       // обрыв: не декодируется
		{"xn--zzzzzzzzzzzz.com", "xn--zzzzzzzzzzzz.com"}, // переполнение
		{"xn--0.com", "xn--0.com"},
		{"xn--tda.com", "xn--tda.com"},                               // «ü»: латиница с диакритикой вне белого списка (SEC-1)
		{"xn--bcher-kva.de", "xn--bcher-kva.de"},                     // «bücher»
		{"xn--hxargifdar.gr", "xn--hxargifdar.gr"},                   // «ελληνικά»: греческий не раскрывается (SEC-5)
		{"xn---80aac2ak8aza.xn--p1ai", "xn---80aac2ak8aza.xn--p1ai"}, // лишний «-»: не канон (ВАЖНО 1, SEC-3)
		{"xn--google-.com", "xn--google-.com"},                       // одна латиница: «google.com» под видом xn--
		{"xn--80aac2ak8aza-.xn--p1ai", "xn--80aac2ak8aza-.xn--p1ai"},
		{"xn--80AAC2AK8AZA.com", "фабрика.com"},
		{"xn--80aac2ak8aza.xn--hxargifdar", "xn--80aac2ak8aza.xn--hxargifdar"}, // хост целиком или никак
		{"xn--80aac2ak8aza.a_b.com", "xn--80aac2ak8aza.a_b.com"},
		{"xn--80aac2ak8aza.ab--c.com", "xn--80aac2ak8aza.ab--c.com"},
		{"xn--80aac2ak8aza.com.", "xn--80aac2ak8aza.com."},
	} {
		got := DisplayHost(c.in)
		if got != c.want {
			t.Errorf("%s → %q, ждали %q", c.in, got, c.want)
		}
	}
}

// Пробы аудита (SEC-1, SEC-2, SEC-4): двойники латиницы из других блоков Юникода, комбинирующие
// знаки и селекторы, кириллица из одних двойников — показ остаётся xn--…, каждая метка отдельно.
func TestDisplayHostLookalikes(t *testing.T) {
	for _, u := range []string{
		"fabrıka-fotoknigi", "fabrika-fᴏtᴏknigi", "fabrika-fotokniɡi", "fabrika-fotoknigi\u034f",
		"fabrika-fotoknigi\ufe0f", "fabrika-fotoknigi\U000e0100", "fabrika-fotokni\u0307gi", "fabrikaꞏfotoknigi",
		"fabrika-fotoknigǀ", "рорсогп", "ѵк", "ροκο", "ιοτα", "fsfera\u17b4", "a\u0338b", "фабрика\u0301",
		"ԍооԁ", "ҽхаmрlе", "ѡеb", "фабрикa", "Фабрика", "ФАБРИКА", "фабрика-", "аб--в",
	} {
		host := "xn--" + punyEncode([]rune(u)) + ".com"
		if got := DisplayHost(host); got != strings.ToLower(host) {
			t.Errorf("%q (%s) показан как %q", u, host, got)
		}
	}
	// Годная кириллица по-прежнему Юникодом: русский, украинский, белорусский, с цифрами и «-».
	for _, u := range []string{"фабрика", "їжак", "ґанок", "ўсход", "єдність", "тест-2", "ёж", "й", "печать24"} {
		host := "xn--" + punyEncode([]rune(u)) + ".xn--p1ai"
		if got := DisplayHost(host); got != u+".рф" {
			t.Errorf("%q (%s) показан как %q", u, host, got)
		}
	}
}

// Желательное аудита: двойник встроенного сайта по скелету (кириллица под видом латиницы) — xn--.
func TestDisplayHostLikeBuiltin(t *testing.T) {
	p := fabrikaCopy(t)
	p.Export.URL = "https://kot.com/"
	for i := range p.Products {
		p.Products[i].URL = ""
	}
	s, err := NewSnapshot([]Profile{p})
	if err != nil {
		t.Fatal(err)
	}
	cyr := "xn--" + punyEncode([]rune("кот"))
	before := DisplayHost(cyr + ".com")
	UseBuiltinForTest(t, s)
	for in, want := range map[string]string{
		cyr + ".com":          cyr + ".com", // «кот.com» = kot.com по скелету
		"www." + cyr + ".com": "www." + cyr + ".com",
		cyr + ".xn--p1ai":     "кот.рф", // другой скелет — показ как обычно
		"kot.com":             "kot.com",
	} {
		if got := DisplayHost(in); got != want {
			t.Errorf("%s → %q, ждали %q", in, got, want)
		}
	}
	if before != "кот.com" {
		t.Errorf("без встроенного kot.com: %q", before)
	}
}

// Кодировщик — обратный декодеру на годных метках и совпадает с примерами RFC 3492.
func TestPunyEncode(t *testing.T) {
	for u, a := range map[string]string{"фабрика": "80aac2ak8aza", "bücher": "bcher-kva", "рф": "p1ai", "ü": "tda", "": "", "abc": "abc-"} {
		if got := punyEncode([]rune(u)); got != a {
			t.Errorf("%q → %q, ждали %q", u, got, a)
		}
		if r, ok := punyDecode(a); u != "" && (!ok || string(r) != u) {
			t.Errorf("обратно %q → %q %v", a, string(r), ok)
		}
	}
}

// Разбор файла: метка xn--, которая не декодируется, неканонична или без не-ASCII, — отказ (SEC-3).
func TestIdnHostNonCanonical(t *testing.T) {
	for _, h := range []string{"xn---80aac2ak8aza.xn--p1ai", "xn--google-.com", "www.xn--abc-.com", "xn--.com", "a.xn--",
		"xn--zzzzzzzzzzzz.com", "xn--a.com", "xn--80aac2ak8aza-.com", "xn--80aac2ak8aza.xn---p1ai"} {
		if err := CheckURL("https://" + h + "/"); err == nil {
			t.Errorf("%s принят", h)
		}
	}
	// Ревью 3: окно принимает кириллицу — текст не отсылает к xn--.
	if err := CheckURL("https://127.0.0.1/"); err == nil || !strings.Contains(err.Error(), "доменное имя с точкой, не IP-адрес") || strings.Contains(err.Error(), "xn--") {
		t.Errorf("текст: %v", err)
	}
	for _, h := range []string{"xn--80aac2ak8aza.xn--p1ai", "XN--80AAC2AK8AZA.XN--P1AI", "xn--bcher-kva.de", "xn--80ak6aa92e.com"} {
		if err := CheckURL("https://" + h + "/"); err != nil {
			t.Errorf("%s: %v", h, err)
		}
	}
}

func TestLabelProblem(t *testing.T) {
	for label, bad := range map[string]bool{
		"фабрика": false, "fabrika": false, "фабрика-2": false, "ελλάδα": false, "рф": false,
		"fаbrika": true, // кириллическая «а» среди латиницы
		"фabrika": true, "аррӏе": true, "сор": true, "хуу": true,
		"фабрика日本": true, "фабрика!": true, "фабрика٣": true, "": false,
		"рорсогп": true, "пор": true, "ъ": true, "гор": true, "город": false, // г п ъ — двойники r n b (SEC-4)
		"a\u0338b": true, "фабрика\u0301": true, "и\u0306": true, // комбинирующие знаки (SEC-2)
	} {
		if got := LabelProblem(label) != ""; got != bad {
			t.Errorf("%q: проблема=%v, ждали %v (%s)", label, got, bad, LabelProblem(label))
		}
	}
}

// Битые и огромные метки — ошибка вместо паники, без раздувания.
func TestPunyDecodeBad(t *testing.T) {
	for _, in := range []string{"", "-", "--", "ё", "a-ё", "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", strings.Repeat("9", 64), "80aac2ak", "80aac2ak8aza-"} {
		_, _ = punyDecode(in) // не паникует
	}
	if _, ok := punyDecode(strings.Repeat("a", 64)); ok {
		t.Error("метка длиннее 63 знаков принята")
	}
	for _, in := range []string{"99999999", "zzzzzzzzzzzz", "ё-abc", "a-b!", "80aac2ak8az", "\x00", "a-9999999999"} {
		if r, ok := punyDecode(in); ok {
			t.Errorf("%q принят: %q", in, string(r))
		}
	}
	if r, ok := punyDecode("80AAC2AK8AZA"); !ok || string(r) != "фабрика" {
		t.Errorf("верхний регистр: %q %v", string(r), ok)
	}
	if r, ok := punyDecode("80aac2ak8aza"); !ok || string(r) != "фабрика" {
		t.Errorf("фабрика: %q %v", string(r), ok)
	}
	for _, in := range []string{"-80aac2ak8aza", "-", "-abc"} { // разделитель без основных знаков
		if r, ok := punyDecode(in); ok {
			t.Errorf("%q принят: %q", in, string(r))
		}
	}
	if r, ok := punyDecode("bcher-kva"); !ok || string(r) != "bücher" {
		t.Errorf("bücher: %q %v", string(r), ok)
	}
}

func FuzzPunyDecode(f *testing.F) {
	for _, s := range []string{"80aac2ak8aza", "bcher-kva", "", "-", "zzzz", "a-b-c", "80ak6aa92e", strings.Repeat("9", 63)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r, ok := punyDecode(s)
		if ok && len(r) > 63 {
			t.Fatalf("длинный выход: %d", len(r))
		}
		// Кодировщик обратен декодеру: код декодированного раскрывается в те же знаки.
		if enc := punyEncode(r); ok && len(enc) <= 63 {
			if back, ok2 := punyDecode(enc); !ok2 || string(back) != string(r) {
				t.Fatalf("%q → %q → %q → %q %v", s, string(r), enc, string(back), ok2)
			}
		}
		if u, ok := xnLabel(strings.ToLower(s)); ok && "xn--"+punyEncode([]rune(u)) != "xn--"+strings.ToLower(s) {
			t.Fatalf("канон %q не круг", s)
		}
		_ = DisplayHost("xn--" + s + ".com")
		_ = LabelProblem(s)
	})
}
