package printprof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Находки аудита безопасности 123 (№1–№5, №7): хост изделий, двойники названий, хост-число и
// localhost, знаки, которые Wails не откроет, раздувание Encode.

// fabrikaCopy — встроенная Фабрика с отдельными срезами (снимок общий и только для чтения).
func fabrikaCopy(t *testing.T) Profile {
	t.Helper()
	p, err := Parse(builtinData(t, "fabrika"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// №1: адрес изделия ведёт на хост export.url (www. — тот же сайт); без export.url — один хост на
// все изделия. Сводка импорта показывает этот хост.
func TestSiteHostSame(t *testing.T) {
	evil := "https://fabrika-fotoknigi.com.evil.example/login"
	for name, c := range map[string]struct {
		mod func(p *Profile)
		ok  bool
	}{
		"встроенная":              {func(*Profile) {}, true},
		"изделие на чужом хосте":  {func(p *Profile) { p.Products[0].URL = evil }, false},
		"последнее изделие чужое": {func(p *Profile) { p.Products[len(p.Products)-1].URL = evil }, false},
		"www. у изделия":          {func(p *Profile) { p.Products[0].URL = "https://www.fabrika-fotoknigi.com/x" }, true},
		"www. у сайта":            {func(p *Profile) { p.Export.URL = "https://WWW.Fabrika-Fotoknigi.com/" }, true},
		"поддомен — другой сайт":  {func(p *Profile) { p.Products[0].URL = "https://shop.fabrika-fotoknigi.com/x" }, false},
		"без export.url, один хост": {func(p *Profile) {
			p.Export.URL = ""
		}, true},
		"без export.url, два хоста": {func(p *Profile) {
			p.Export.URL = ""
			p.Products[1].URL = evil
		}, false},
		"без export, два хоста": {func(p *Profile) {
			p.Export = nil
			p.Products[2].URL = evil
		}, false},
		"изделия без адресов": {func(p *Profile) {
			for i := range p.Products {
				p.Products[i].URL = ""
			}
		}, true},
	} {
		p := fabrikaCopy(t)
		c.mod(&p)
		err := p.Validate()
		if (err == nil) != c.ok {
			t.Errorf("%s: %v", name, err)
		}
		if err != nil && !strings.Contains(err.Error(), "на другом сайте") {
			t.Errorf("%s: отказ не той причиной: %v", name, err)
		}
		if _, perr := Parse(mustJSON(t, p)); (perr == nil) != c.ok {
			t.Errorf("%s: Parse не согласен с Validate: %v", name, perr)
		}
	}
	p := fabrikaCopy(t)
	if h := p.SiteHost(); h != "fabrika-fotoknigi.com" {
		t.Errorf("хост сайта: %q", h)
	}
	p.Export.URL = ""
	p.Products[0].URL = "https://Www.Fabrika-Fotoknigi.com/a"
	if h := p.SiteHost(); h != "www.fabrika-fotoknigi.com" {
		t.Errorf("хост по изделию: %q", h)
	}
	if h := (Profile{}).SiteHost(); h != "" {
		t.Errorf("без адресов: %q", h)
	}
}

// mustJSON — профиль в JSON без проверки (Encode отказал бы негодному).
func mustJSON(t *testing.T, p Profile) []byte {
	t.Helper()
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// №2: двойники «Фабрика Фотокниги» сводятся к одному скелету.
func TestNameKeyTwins(t *testing.T) {
	const orig = "Фабрика Фотокниги"
	twins := []string{
		"фабрика фотокниги", "ФАБРИКА ФОТОКНИГИ", " Фабрика  Фотокниги ",
		"Фабрика\u00a0Фотокниги", "Фабрика\u3000Фотокниги", "ФабрикаФотокниги",
		"Фaбрикa Фoтoкниги", // латинские a и o
		"Фαбрика Фοтокниги", // греческие альфа и омикрон
		"ФАБPИKA ФOTOKHИГИ", // латинские P, K, O, T, H заглавные
		"Фабрика\u200bФотокниги", "Фабрика\u115fФотокниги", "Фабрика\u1160Фотокниги",
		"Фабрика\u3164Фотокниги", "Фабрика\uffa0Фотокниги", "Фабрика\u2800Фотокниги",
		"Фабрика\u034fФотокниги", "Фабрика\u17b4Фотокниги", "Фабрика\u17b5Фотокниги",
		"Фабрика\ufe0fФотокниги", "Фабрика Фотокниги\u0301", // вариационный селектор, висячий акцент
	}
	for _, s := range twins {
		if !SameName(s, orig) {
			t.Errorf("%q не двойник: %q ≠ %q", s, NameKey(s), NameKey(orig))
		}
	}
	// «Й» из двух знаков (NFD) и во всю ширину — тот же, что обычный.
	if !SameName("Йошкар", "И\u0306ошкар") || !SameName("Studio", "Ｓｔｕｄｉｏ") {
		t.Error("NFD-Й или полноширинные буквы не сведены")
	}
	// Разные названия остаются разными: «Й» не «И», цифры и знаки на месте.
	for _, pair := range [][2]string{{"Йошкар", "Иошкар"}, {"Фото 1", "Фото 2"}, {"Нетпринт", "Фотосфера"}, {"А-1", "А-2"}} {
		if SameName(pair[0], pair[1]) {
			t.Errorf("%q и %q сведены", pair[0], pair[1])
		}
	}
	if NameKey("\u115f\u3164\u2800 \u034f") != "" {
		t.Error("скелет из невидимых не пуст")
	}
}

// №2: слово из смеси алфавитов, невидимые заполнители и название без видимых знаков не
// проходят разбор — ни в названии, ни в изделии, ни в подписи формата, ни в текстах выгрузки.
func TestLabelRejects(t *testing.T) {
	for name, c := range map[string]struct {
		why string
		mod func(p *Profile)
	}{
		"латинская a в названии":   {"разных алфавитов", func(p *Profile) { p.Name = "Фaбрика Фото" }},
		"латиница в gen":           {"разных алфавитов", func(p *Profile) { p.Gen = "Фабрикu" }},
		"греческая о в изделии":    {"разных алфавитов", func(p *Profile) { p.Products[0].Name = "Фοто" }},
		"смесь в подписи формата":  {"разных алфавитов", func(p *Profile) { p.Formats[0].Name = "30x20cм" }},
		"заполнитель Хангыль":      {"невидимый", func(p *Profile) { p.Name = "Фабрика\u3164" }},
		"Брайль в gen":             {"невидимый", func(p *Profile) { p.Gen = "Фабрики\u2800" }},
		"соединитель в изделии":    {"невидимый", func(p *Profile) { p.Products[0].Pages = "Бумага\u034f" }},
		"кхмерская в тексте":       {"невидимый", func(p *Profile) { p.Export.Done = "Готово\u17b4" }},
		"селектор в подсказке":     {"невидимый", func(p *Profile) { p.Page.Hint = "Зона\ufe0f" }},
		"только акценты":           {"нет видимых", func(p *Profile) { p.Name = "\u0301\u0301" }},
		"Хангыль U+115F в формате": {"невидимый", func(p *Profile) { p.Formats[0].Name = "30×20\u115f" }},
	} {
		p := fabrikaCopy(t)
		c.mod(&p)
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: %v, ждали %q", name, err, c.why)
		}
	}
	// Разные алфавиты в разных словах, цифры и знаки — можно.
	p := fabrikaCopy(t)
	p.Name, p.Products[0].Name = "Фото Pro", "Книга-Pro 2"
	if err := p.Validate(); err != nil {
		t.Errorf("слова разных алфавитов: %v", err)
	}
}

// Повторный аудит 123 №2: двойники из других письменностей (армянская, коптская, чероки,
// математические буквы), символ на месте буквы и невидимая нотная головка — отказ; три алфавита,
// цифры, «×», «№», дефис и «ё» — можно; встроенные проходят (TestBuiltinValid и Builtin()).
func TestLabelForeignTwins(t *testing.T) {
	for _, c := range []struct{ name, why string }{
		{"Фабрика Фօтокниги", "только кириллицы"},    // армянская «օ»
		{"Фотօсфера", "только кириллицы"},            // армянская «օ»
		{"Фабрикⲁ Фⲟтокниги", "только кириллицы"},    // коптские «ⲁ», «ⲟ»
		{"ᎠBC", "только кириллицы"},                  // чероки «Ꭰ»
		{"Студия \U0001d41a", "только кириллицы"},    // математическая «𝐚»
		{"㋐ Фото", "только кириллицы"},               // «㋐» после NFKC — катакана
		{"Фабрик⍺ Фотокниги", "символ вплотную"},     // APL-альфа в конце слова
		{"Фо∘то", "символ вплотную"},                 // «∘» внутри слова
		{"Фото❤", "символ вплотную"},                 // эмодзи вплотную
		{"Фабрика\U0001d159Фотокниги", "невидимый"},  // пустая нотная головка
		{"Фабрика Фотокниги\U0001d17a", "невидимый"}, // нотный знак конца фразы
	} {
		p := fabrikaCopy(t)
		p.Name = c.name
		if err := p.Validate(); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%q: %v, ждали %q", c.name, err, c.why)
		}
	}
	// Несколько полей сразу: изделие и gen проверяются так же; подписи форматов «30×30» встроенных
	// проходят (fabrikaCopy).
	p := fabrikaCopy(t)
	p.Products[0].Name, p.Gen = "Кнօга", "Фабрик⍺"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "gen: символ") {
		t.Errorf("gen с APL-альфой: %v", err)
	}
	p.Gen = "Фабрики"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "только кириллицы") {
		t.Errorf("изделие с армянской «օ»: %v", err)
	}
	for _, name := range []string{"Фото-книга 2", "Royal", "Фабрика Photo", "Ёлка", "Альбом №1", "Фото ❤", "Δέλτα", "А4 × А3"} {
		p := fabrikaCopy(t)
		p.Name, p.Products[0].Name = name, name
		if err := p.Validate(); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
}

// №2: двойник названия — тот же спор в реестре: своя, набор и NewSnapshot.
func TestComposeTwinNames(t *testing.T) {
	orig := mustProfile(t, "fabrika").Name
	twins := []string{strings.ToUpper(orig), strings.Replace(orig, " ", "\u00a0", 1), strings.Replace(orig, " ", "", 1)}
	for i, name := range twins {
		o := own(t, "fsfera", fmt.Sprintf("own-0000000%d", i), name)
		if err := o.Validate(); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
		snap, skipped := Compose(nil, []Profile{o})
		if len(skipped) != 1 || !strings.Contains(skipped[0], "название уже есть") || len(snap.Profiles()) != 3 {
			t.Errorf("своя %q: %v %v", name, ids(snap), skipped)
		}
		lib := own(t, "fsfera", fmt.Sprintf("newlab-%d", i), name)
		if snap, skipped = Compose([]Profile{lib}, nil); len(skipped) != 1 || len(snap.Profiles()) != 3 {
			t.Errorf("набор %q: %v %v", name, ids(snap), skipped)
		}
		if _, err := NewSnapshot(append(Builtin().Profiles(), lib)); err == nil {
			t.Errorf("NewSnapshot принял двойника %q", name)
		}
	}
}

// №7: строки пропуска в log.txt называют id, а не название (оно — данные пользователя).
func TestComposeLogNoName(t *testing.T) {
	o := own(t, "fsfera", "own-0000000a", "Секретная студия")
	o2 := own(t, "fsfera", "own-0000000b", "Секретная студия")
	_, skipped := Compose(nil, []Profile{o, o2})
	if len(skipped) != 1 || strings.Contains(skipped[0], "Секретная") || !strings.Contains(skipped[0], `"own-0000000a"`) {
		t.Errorf("строка пропуска: %v", skipped)
	}
}

// TestIdnHostRejected (design §8; аудит 123 №3): хост, который браузер поймёт как IPv4 в
// сокращённой записи, свой компьютер и порт не 443 — отказ; доменное имя и xn--… — можно.
func TestIdnHostRejected(t *testing.T) {
	for _, u := range []string{
		"https://127.1/", "https://0x7f.0.0.1/", "https://0177.0.0.1/", "https://10.1/", "https://1.2.3.4/",
		"https://a.0x7f/", "https://shop.123/", "https://x.localhost/", "https://a.b.LOCALHOST/",
		"https://localhost./", "https://fabrika.com./", "https://-a.com/", "https://a-.com/", "https://a..com/",
		"https://fabrika.com:99999/", "https://fabrika.com:8443/", "https://fabrika.com:80/", "https://fabrika.com:/",
		"https://fаbrika.com/", // кириллическая «а»
		"https://[::1]/", "https://" + strings.Repeat("a", 64) + ".com/",
	} {
		if err := CheckURL(u); err == nil {
			t.Errorf("%s принят", u)
		}
	}
	for _, u := range []string{
		"https://fabrika-fotoknigi.com/photo-books", "https://fsfera.ru/order_pro/calculator/",
		"https://xn--80ak6aa92e.xn--p1ai/", "https://fabrika.com:443/a?b=1&c=2#d", "https://a.co/%D0%BA",
	} {
		if err := CheckURL(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
}

// №4: знаки, с которыми Wails BrowserOpenURL адрес не откроет, отвергаются разбором. «< > | { } `»
// Wails пропускает (url.String() кодирует их как %3C…), но и в адресе сайта им не место — отказ.
func TestCheckURLWailsRules(t *testing.T) {
	for _, ch := range strings.Split("~ ( ) ! $ [ ] * ; < > | { } `", " ") {
		u := "https://fabrika.com/a" + ch + "b"
		if err := CheckURL(u); err == nil {
			t.Errorf("%q принят", u)
		}
		if _, err := wailsValidate(u); err == nil && !strings.Contains("<>|{}`", ch) {
			t.Errorf("Wails принял %q — оракул разошёлся с исходником", u)
		}
	}
	for _, u := range []string{"https://fabrika.com/a\u00a0b", "https://fabrika.com/a\u2028", "https://fabrika.com/\ufeff"} {
		if err := CheckURL(u); err == nil {
			t.Errorf("%q принят", u)
		}
	}
}

// Страж «одна проверка URL = правила Wails»: всё, что пропускает CheckURL, Wails откроет, и на
// тот же хост. wailsValidate — копия ValidateAndSanitizeURL из Wails 2.16; при смене версии
// Wails падает TestWailsVersionPinned — сверить копию и wailsSafe с новым исходником.
func TestCheckURLAgreesWithWails(t *testing.T) {
	urls := []string{
		"https://fabrika-fotoknigi.com/photo-books", "https://a.co/x?y=1&z=2#f", "https://a.co/'\"",
		"https://a.co/%7e", "https://a.co/путь", "https://a.co/a%20b", "https://a.co/a+b,c=d@e:f",
	}
	for _, p := range Builtin().Profiles() {
		for _, pr := range p.Products {
			urls = append(urls, pr.URL)
		}
		if p.Export != nil {
			urls = append(urls, p.Export.URL)
		}
	}
	for _, u := range urls {
		checkAgainstWails(t, u)
	}
}

// checkAgainstWails — если CheckURL пропустил u, Wails его откроет и на тот же хост.
func checkAgainstWails(t *testing.T, u string) {
	t.Helper()
	if u == "" || CheckURL(u) != nil {
		return
	}
	got, err := wailsValidate(u)
	if err != nil {
		t.Fatalf("%q: CheckURL пропустил, Wails откажет: %v", u, err)
	}
	g, err := url.Parse(got)
	if err != nil || !strings.EqualFold(g.Hostname(), siteHostRaw(u)) {
		t.Fatalf("%q: Wails откроет другой хост %q", u, got)
	}
}

func siteHostRaw(s string) string {
	u, _ := url.Parse(s)
	return u.Hostname()
}

// wailsValidate — копия github.com/wailsapp/wails/v2@v2.16.0/internal/frontend/utils/urlValidator.go
// (ValidateAndSanitizeURL, MIT): пакет internal, импортировать нельзя.
func wailsValidate(rawURL string) (string, error) {
	if strings.Contains(rawURL, "\x00") {
		return "", errors.New("null bytes not allowed in URL")
	}
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme == "javascript" || scheme == "data" || scheme == "file" || scheme == "ftp" || scheme == "" {
		return "", errors.New("scheme not allowed")
	}
	if (scheme == "http" || scheme == "https") && parsedURL.Host == "" {
		return "", errors.New("missing host")
	}
	sanitizedURL := parsedURL.String()
	for _, r := range sanitizedURL {
		if r < 32 && r != 9 {
			return "", errors.New("control character")
		}
	}
	shellDangerous := `[;\|` + "`" + `$\\<>*{}\[\]()~! \t\n\r]`
	if matched, _ := regexp.MatchString(shellDangerous, sanitizedURL); matched {
		return "", errors.New("shell metacharacters not allowed")
	}
	unicodeDangerous := "[\u0000-\u001F\u007F\u00a0\u1680\u2000-\u200f\u2028-\u202f\u205f\u2060\u3000\ufeff]"
	if matched, _ := regexp.MatchString(unicodeDangerous, sanitizedURL); matched {
		return "", errors.New("unicode dangerous characters not allowed")
	}
	return sanitizedURL, nil
}

// TestWailsVersionPinned — копия wailsValidate и wailsSafe сверены с Wails v2.16.0: новая версия
// в go.mod — сначала сверить их с urlValidator.go новой версии, потом поднять здесь.
func TestWailsVersionPinned(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`(?m)^\s*github\.com/wailsapp/wails/v2 v2\.16\.0\s*$`).Match(b) {
		t.Fatal("версия Wails в go.mod сменилась: сверить wailsValidate и wailsSafe с internal/frontend/utils/urlValidator.go")
	}
}

// №5: Encode не экранирует «<», «>», «&» — запись годного профиля не раздувается за MaxSize и
// разбирается обратно; профиль, чья запись больше MaxSize, негоден.
func TestEncodeNoHTMLEscape(t *testing.T) {
	p := bigProfile(t, "<&>", 0)
	b, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("<&>")) || bytes.Contains(b, []byte(`\u003c`)) {
		t.Error("знаки HTML экранированы")
	}
	got, err := Parse(b)
	if err != nil {
		t.Fatalf("запись не разбирается: %v (%d байт)", err, len(b))
	}
	if !reflect.DeepEqual(got, p) {
		t.Error("круг Encode→Parse изменил профиль")
	}
	// Тексты из одних «<» почти во весь предел: с экранированием запись была бы вшестеро длиннее.
	lt := bigProfile(t, strings.Repeat("<", 190), 0)
	if b, err := Encode(lt); err != nil || len(b) > MaxSize {
		t.Fatalf("«<» во всех текстах: %v, %d байт", err, len(b))
	}
	// Все таблицы корешка у предела: каждое число на своей строке — запись за 64 КБ.
	huge := bigProfile(t, "x", maxSpine)
	if err := huge.Validate(); err == nil || !strings.Contains(err.Error(), "больше 64 КБ") {
		t.Errorf("запись за пределом принята: %v", err)
	}
	if _, err := Encode(huge); err == nil {
		t.Error("Encode записал профиль за пределом")
	}
}

// bigProfile — профиль у пределов: 32 формата и 32 изделия с текстами из s (до 200 знаков),
// у каждого изделия таблица корешка и spineBy на все форматы по pairs строк.
func bigProfile(t testing.TB, s string, pairs int) Profile {
	t.Helper()
	text := func(i int) string {
		v := fmt.Sprintf("%s%d", s, i)
		return string([]rune(v)[:min(len([]rune(v)), maxText)])
	}
	p := fullProfile()
	p.Name, p.Gen = "Большая", "Большой"
	p.Formats = nil
	for i := range maxFormats {
		p.Formats = append(p.Formats, PageFormat{Name: text(i), Page: Pair{300, 200}})
	}
	var spine []float64
	for i := range pairs {
		spine = append(spine, float64(i+1), 10.25)
	}
	p.Products = nil
	for i := range maxProducts {
		pr := Product{ID: OwnProductID(p.ID, i+1), Name: "Изделие " + itoa(i), Pages: text(i), Item: text(i),
			URL: "https://example.com/" + itoa(i), Spine: slices.Clone(spine)}
		if pairs > 0 {
			pr.SpineBy = map[string][]float64{}
			for _, f := range p.Formats {
				pr.SpineBy[f.Name] = slices.Clone(spine)
			}
		}
		p.Products = append(p.Products, pr)
	}
	p.Default = p.Products[0].ID
	e := *p.Export
	e.Acc, e.Title, e.Done, e.Plain, e.SiteButton = text(1), text(2), text(3), text(4), text(5)
	p.Export = &e
	p.Page.Hint = text(0)
	p.normalize()
	return p
}

// OwnIndex: номер принятой своей в списке own; отвергнутая, чужая и встроенная — ok=false.
func TestOwnIndex(t *testing.T) {
	bad := own(t, "fsfera", "own-0000000a", "Фабрика Фотокниги") // занято встроенной
	good := own(t, "fsfera", "own-0000000b", "Студия Б")
	snap, _ := Compose(nil, []Profile{bad, good})
	if n, ok := snap.OwnIndex("own-0000000b"); !ok || n != 1 {
		t.Errorf("принятая: %d %v", n, ok)
	}
	for _, id := range []string{"own-0000000a", "fabrika", "нет", ""} {
		if n, ok := snap.OwnIndex(id); ok {
			t.Errorf("%q: принята под номером %d", id, n)
		}
	}
}

// siteHost: адрес, который не разбирается, даёт пустой хост (checkURL это отсекает раньше).
func TestSiteHostBadURL(t *testing.T) {
	for in, want := range map[string]string{"%zz": "", "https://WWW.Site.ru/a": "site.ru", "": ""} {
		if got := siteHost(in); got != want {
			t.Errorf("siteHost(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// marshal: значение, которое JSON не умеет (NaN), — ошибка, а не пустой файл.
func TestMarshalError(t *testing.T) {
	if b, err := marshal(math.NaN()); err == nil {
		t.Errorf("NaN записан: %q", b)
	}
	if b, err := marshal("<&>"); err != nil || !strings.Contains(string(b), "<&>") {
		t.Errorf("знаки HTML экранированы: %q %v", b, err)
	}
}

// userError показывает русскую причину как есть (в App она идёт тостом).
func TestUserErrorText(t *testing.T) {
	var err error = userError("причина")
	if got := err.Error(); got != "причина" {
		t.Errorf("userError: %q", got)
	}
}
