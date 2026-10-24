package app

import (
	"math/rand/v2"
	"strings"
	"testing"

	"golang.org/x/net/idna"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// Кириллический адрес в окне «Своя типография» (123 Б3, 0.26-123-52).

func TestIdnURL(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://фабрика.рф/", "https://xn--80aac2ak8aza.xn--p1ai/"},
		{"https://ФАБРИКА.РФ/Каталог?a=1#б", "https://xn--80aac2ak8aza.xn--p1ai/Каталог?a=1#б"}, // регистр хоста сводится, хвост как есть
		{"  https://фабрика.рф:443/x ", "https://xn--80aac2ak8aza.xn--p1ai:443/x"},
		{"https://www.фабрика.рф", "https://www.xn--80aac2ak8aza.xn--p1ai"},
		{"https://фабрика.com/", "https://xn--80aac2ak8aza.com/"},
		{"https://bücher.de/", "https://xn--bcher-kva.de/"},
		{"https://example.com/путь", "https://example.com/путь"}, // хост ASCII — не трогаем
		{"", ""},
		{"http://фабрика.рф/", "http://фабрика.рф/"}, // схема не https — решит CheckURL
		{"https://user@фабрика.рф/", "https://user@фабрика.рф/"},
	} {
		got, why := idnURL(c.in)
		if got != c.want || why != "" {
			t.Errorf("%q → %q (%s), ждали %q", c.in, got, why, c.want)
		}
	}
	for _, in := range []string{
		"https://exаmple.com/", // кириллическая «а» среди латиницы
		"https://фabrika.рф/",
		"https://аррӏе.com/",                          // вся метка из двойников латиницы
		"https://日本.jp/",                              // буквы вне трёх алфавитов
		"https://фабрика!.рф/",                        // знак вне LDH
		"https://фаб рика.рф/",                        // пробел
		"https://фабрика..рф/",                        // пустая метка
		"https://-фабрика.рф/",                        // дефис в начале
		"https://ф\u200bабрика.рф/",                   // невидимый знак
		"https://" + strings.Repeat("ф", 64) + ".рф/", // метка длиннее 63 после перевода
	} {
		if got, why := idnURL(in); why == "" {
			t.Errorf("%q принят: %q", in, got)
		} else if !strings.ContainsAny(why, "абвгде") {
			t.Errorf("%q: текст не по-русски: %q", in, why)
		}
	}
}

// Окно: ввод кириллицей → в файле xn--, при повторном открытии — Юникод, и снова круг без изменений.
func TestSavePrintProfileCyrillicSite(t *testing.T) {
	a, ev := printApp(t)
	e := editOf(t, a, "fabrika", "Кириллическая")
	e.URL = "https://Фабрика.рф/заказ"
	if chk := a.CheckPrintProfile(e); !chk.OK || chk.Errs["url"] != "" {
		t.Fatalf("расчёт: %+v", chk.Errs)
	}
	res := saved(t, a, e)
	if ev.count("print:changed") != 1 {
		t.Errorf("событий %d", ev.count("print:changed"))
	}
	p, _ := printprof.Current().ByID(res.ID)
	if want := "https://xn--80aac2ak8aza.xn--p1ai/заказ"; p.Export.URL != want {
		t.Errorf("в профиле %q, ждали %q", p.Export.URL, want)
	}
	if p.Export.SiteButton != "Открыть фабрика.рф" || p.SiteHost() != "фабрика.рф" {
		t.Errorf("кнопка %q, хост %q", p.Export.SiteButton, p.SiteHost())
	}
	if err := printprof.CheckURL(p.Export.URL); err != nil {
		t.Errorf("сохранённый адрес не прошёл разбор: %v", err)
	}
	// Файл на диске разбирается строго и даёт тот же профиль (Save → Load).
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	back, err := printprof.Parse(data)
	if err != nil || back.Export.URL != p.Export.URL {
		t.Fatalf("Parse(Encode): %v %q", err, back.Export.URL)
	}
	// Реестр после перезагрузки с диска.
	a.reloadPrints()
	if q, ok := printprof.Current().ByID(res.ID); !ok || q.Export.URL != p.Export.URL {
		t.Fatalf("после перезагрузки реестра: %v %+v", ok, q.Export)
	}
	// Повторное открытие на основе своей: хост показан Юникодом; сохранение не меняет адрес.
	again, err := a.PrintProfileEdit(res.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.URL != "https://фабрика.рф/заказ" {
		t.Errorf("в окне %q", again.URL)
	}
	again.Name = "Вторая"
	res2 := saved(t, a, again)
	if p2, _ := printprof.Current().ByID(res2.ID); p2.Export.URL != p.Export.URL {
		t.Errorf("круг изменил адрес: %q", p2.Export.URL)
	}
}

// Несколько типографий с разными адресами сразу; пустой адрес — без кнопки.
func TestSavePrintProfileSitesMany(t *testing.T) {
	a, _ := printApp(t)
	want := map[string]string{
		"https://фабрика.рф/":   "https://xn--80aac2ak8aza.xn--p1ai/",
		"https://пример.рф/":    "https://xn--e1afmkfd.xn--p1ai/",
		"https://example.com/a": "https://example.com/a",
		"":                      "",
		"https://bücher.de/":    "https://xn--bcher-kva.de/",
	}
	i := 0
	for in, out := range want {
		i++
		e := editOf(t, a, "fabrika", "Сайт "+string(rune('А'+i)))
		e.URL = in
		res := saved(t, a, e)
		if p, _ := printprof.Current().ByID(res.ID); p.Export.URL != out {
			t.Errorf("%q → %q, ждали %q", in, p.Export.URL, out)
		}
	}
}

// Смесь алфавитов и битый ввод: отказ с текстом под полем адреса, ничего не записано.
func TestCheckPrintProfileCyrillicSiteRejected(t *testing.T) {
	a, ev := printApp(t)
	for _, in := range []string{
		"https://exаmple.com/", "https://фabrika.рф/", "https://аррӏе.com/", "https://日本.jp/",
		"https://фабрика!.рф/", "https://фабрика..рф/", "https://" + strings.Repeat("ф", 200) + ".рф/",
		"https://фабрика.рф:8443/", "https://фабрика/", "https://\u0000.рф/",
		"https://xn--80ak6aa92e.com/x\x00", "https://фабрика.рф/" + strings.Repeat("я", 400),
	} {
		e := editOf(t, a, "fabrika", "Отказ")
		e.URL = in
		chk := a.CheckPrintProfile(e)
		if chk.OK || chk.Errs["url"] == "" {
			t.Errorf("%q: расчёт %+v", in, chk.Errs)
		}
		if _, err := a.SavePrintProfile(e, false); err == nil {
			t.Errorf("%q сохранён", in)
		}
	}
	if ownCount(a) != 0 || ev.count("print:changed") != 0 {
		t.Errorf("свои: %d, событий %d", ownCount(a), ev.count("print:changed"))
	}
}

// Опасная метка в файле (xn-- смесь/двойник) в окне остаётся xn--, а не Юникодом.
func TestIdnShowKeepsDangerous(t *testing.T) {
	for in, want := range map[string]string{
		"https://xn--80aac2ak8aza.xn--p1ai/x": "https://фабрика.рф/x",
		"https://xn--80ak6aa92e.com/":         "https://xn--80ak6aa92e.com/", // «аррӏе»
		"https://example.com/":                "https://example.com/",
		"":                                    "",
		"https://xn--/":                       "https://xn--/",
	} {
		if got := idnShow(in); got != want {
			t.Errorf("%q → %q, ждали %q", in, got, want)
		}
	}
}

// DisplayHost (свой декодер) совпадает с x/net/idna на безопасных метках; латиница с диакритикой
// и греческий — вне белого списка (0.26-123-56): остаются xn--….
func TestDisplayHostAgreesWithIdna(t *testing.T) {
	for _, u := range []string{"фабрика.рф", "пример.рф", "печать.москва", "bücher.de", "ελλάδα.gr", "тест-2.рф", "ёж.рф", "й.рф", "їжак.укр"} {
		ascii, err := idna.Lookup.ToASCII(u)
		if err != nil {
			t.Fatal(u, err)
		}
		want := u
		if strings.ContainsAny(u, "üελ") {
			want = ascii
		}
		if got := printprof.DisplayHost(ascii); got != want {
			t.Errorf("%s → %q, ждали %q", ascii, got, want)
		}
	}
}

// diffIdna — раскрытый DisplayHost хост x/net раскрывает так же и кодирует обратно в исходный (SEC-6).
func diffIdna(t *testing.T, h string) {
	t.Helper()
	low := strings.ToLower(h)
	shown := printprof.DisplayHost(h)
	if shown == low {
		return
	}
	uni, err := idnProfile.ToUnicode(low)
	back, err2 := idnProfile.ToASCII(shown)
	if err != nil || uni != shown || err2 != nil || back != low {
		t.Errorf("%s показан %q; x/net: %q %v, обратно %q %v", h, shown, uni, err, back, err2)
	}
}

// Разностный тест показа против x/net в обе стороны: пробы аудита (№1–5, неканонические xn--),
// случайные LDH-метки и случайная кириллица белого списка.
func TestDisplayHostDiffIdna(t *testing.T) {
	hosts := []string{"xn---80aac2ak8aza.xn--p1ai", "xn--google-.com", "xn--80aac2ak8aza-.xn--p1ai", "xn--abc-.com",
		"xn--80aac2ak8aza.xn--p1ai", "www.xn--e1afmkfd.xn--p1ai", "xn--80ak6aa92e.com", "XN--80AAC2AK8AZA.com"}
	for _, u := range []string{ // двойники аудита: кодируем x/net без проверок
		"fabrıka-fotoknigi", "fabrika-fᴏtᴏknigi", "fabrika-fotokniɡi", "fabrika-fotoknigi\u034f", "fabrika-fotoknigi\ufe0f",
		"fabrika-fotoknigi\U000e0100", "fabrika-fotokni\u0307gi", "fabrikaꞏfotoknigi", "fabrika-fotoknigǀ", "рорсогп", "ѵк",
		"ροκο", "ιοτα", "fsfera\u17b4", "a\u0338b", "ü", "bücher",
	} {
		a, err := idna.Punycode.ToASCII(u)
		if err != nil {
			t.Fatal(u, err)
		}
		if got := printprof.DisplayHost(a + ".com"); got != a+".com" {
			t.Errorf("двойник %q показан как %q", u, got)
		}
		hosts = append(hosts, a+".com")
	}
	for _, h := range hosts {
		diffIdna(t, h)
	}
	rng := rand.New(rand.NewPCG(123, 56))
	const ldh = "abcdefghijklmnopqrstuvwxyz0123456789-"
	shown := 0
	for range 200000 {
		b := make([]byte, 1+rng.IntN(20))
		for i := range b {
			b[i] = ldh[rng.IntN(len(ldh))]
		}
		h := "xn--" + string(b) + ".xn--p1ai"
		diffIdna(t, h)
		if printprof.DisplayHost(h) != h {
			shown++
		}
	}
	// Обратно: годная метка кириллицей, закодированная x/net, раскрывается в себя же.
	cyr := []rune("абвгдежзийклмнопрстуфхцчшщъыьэюяёєіїўґ0123456789-")
	for range 20000 {
		r := make([]rune, 1+rng.IntN(15))
		for i := range r {
			r[i] = cyr[rng.IntN(len(cyr))]
		}
		u := string(r) + ".рф"
		a, err := idnProfile.ToASCII(u)
		if err != nil {
			continue
		}
		diffIdna(t, a)
		hyph := len(r) >= 4 && r[2] == '-' && r[3] == '-' // x/net смотрит «--» в байтах, а не в знаках
		if got := printprof.DisplayHost(a); got != u && printprof.LabelProblem(string(r)) == "" && !hyph {
			t.Errorf("%q (%s) показан как %q", u, a, got)
		}
	}
	if shown == 0 {
		t.Error("ни одна случайная метка не раскрылась: проверка пустая")
	}
}

// Любая строка: без паники. Принятая окном (why "") с не-ASCII хостом отвергается разбором файла;
// годный для файла адрес — с ASCII-хостом и переживает круг «показ в окне → снова перевод».
func FuzzIdnURL(f *testing.F) {
	for _, s := range []string{"https://фабрика.рф/", "https://exаmple.com", "https://xn--80aac2ak8aza.xn--p1ai:443/x", "", "https://",
		"https://[::1]/", "https://日本.jp/\x00", "https://xn---80aac2ak8aza.xn--p1ai/", "https://ФАБРИКА.рф/a?b#c"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out, why := idnURL(s)
		_ = idnShow(s)
		if why != "" {
			return
		}
		_, host, _, ok := splitHost(out)
		cerr := printprof.CheckURL(out)
		switch {
		case ok && !isASCII(host) && cerr == nil:
			t.Fatalf("%q → %q: хост не ASCII, а разбор принял", s, out)
		case cerr != nil:
			return
		}
		back, why2 := idnURL(idnShow(out))
		if why2 != "" || !strings.EqualFold(back, out) {
			t.Fatalf("%q → %q → показ %q → %q (%s)", s, out, idnShow(out), back, why2)
		}
	})
}
