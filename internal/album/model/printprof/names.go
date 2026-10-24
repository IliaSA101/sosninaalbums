package printprof

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Грамматика имён из файла: id попадает в .salbum (cover.spec.preset) и в имена папок, префикс —
// в имена файлов выгрузки. Слэши, точки и пробелы не проходят, поэтому путь из профиля не соберётся.
var (
	idRe     = regexp.MustCompile(`^[a-z][a-z0-9-]{1,63}$`)
	prefixRe = regexp.MustCompile(`^[0-9A-Za-z_-]{0,8}$`)
)

// Пределы строк профиля, символов: тексты интерфейса, подсказка и адрес (design §6: 200 / 300 / 300).
const (
	maxText = 200
	maxHint = 300
	maxURL  = 300
	// MaxProfileID — длина id самого профиля (design §6: {1,31} после первой буквы); id изделия
	// длиннее на «-<изделие>», поэтому у него предел грамматики ValidID.
	MaxProfileID = 32
)

// ValidID — id профиля или изделия по грамматике ^[a-z][a-z0-9-]{1,63}$ (у профиля ещё MaxProfileID).
func ValidID(s string) bool { return idRe.MatchString(s) }

// checkText — строка текста профиля: допустимый UTF-8 без управляющих, форматирующих и невидимых
// знаков (в том числе bidi U+202A…202E, U+2066…2069: «RLO» в названии переворачивает соседний текст
// окна; заполнители Хангыль и пустая клетка Брайля рисуются пустым местом), не длиннее maxText;
// required — непустая. Имена и подсказки идут в интерфейс как есть.
func checkText(what, s string, required bool) error { return checkTextMax(what, s, required, maxText) }

// checkTextMax — checkText с пределом limit символов.
func checkTextMax(what, s string, required bool, limit int) error {
	switch {
	case required && strings.TrimSpace(s) == "":
		return fmt.Errorf("%s: пусто", what)
	case !utf8.ValidString(s):
		return fmt.Errorf("%s: не UTF-8", what)
	case utf8.RuneCountInString(s) > limit:
		return fmt.Errorf("%s: длиннее %d символов", what, limit)
	}
	for _, r := range s {
		// Те же классы, что text.CleanText: Cf — bidi, нулевой ширины и BOM; Zl/Zp — разделители строк.
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) || r == unicode.ReplacementChar || hiddenRune(r) {
			return fmt.Errorf("%s: управляющий или невидимый знак", what)
		}
	}
	return nil
}

// checkLabel — название (типографии, изделия, формата): checkText, видимые знаки, ни одного
// слова из смеси письменностей — иначе «Фaбрика» с латинской «a» прошла бы за настоящую, —
// буквы только трёх алфавитов и ни одного символа внутри слова (двойники из других письменностей).
func checkLabel(what, s string) error {
	if err := checkText(what, s, true); err != nil {
		return err
	}
	switch {
	case NameKey(s) == "":
		return fmt.Errorf("%s: нет видимых знаков", what)
	case mixedWord(s):
		return fmt.Errorf("%s: в одном слове буквы разных алфавитов (кириллица и латиница)", what)
	case foreignLetter(s):
		return fmt.Errorf("%s: буквы только кириллицы, латиницы и греческого", what)
	case symbolInWord(s):
		return fmt.Errorf("%s: символ вплотную к буквам слова", what)
	}
	return nil
}

// wailsBad — знаки, с которыми Wails 2.16 BrowserOpenURL (internal/frontend/utils/urlValidator.go,
// ValidateAndSanitizeURL) адрес молча не откроет: метазнаки оболочки и пробелы.
const wailsBad = ";|`$\\<>*{}[]()~! \t\n\r"

// wailsSafe — строка проходит проверки ValidateAndSanitizeURL после разбора: нет управляющих
// знаков, метазнаков оболочки и «опасного» Юникода (пробелы, нулевая ширина, bidi, BOM).
func wailsSafe(s string) bool {
	for _, r := range s {
		switch {
		case r < 0x20 || r == 0x7F || strings.ContainsRune(wailsBad, r),
			r == 0xA0 || r == 0x1680 || r == 0x205F || r == 0x2060 || r == 0x3000 || r == 0xFEFF,
			r >= 0x2000 && r <= 0x200F || r >= 0x2028 && r <= 0x202F:
			return false
		}
	}
	return true
}

// CheckURL — адрес сайта типографии: только https, хост — доменное имя (asciiHost), порт не
// задан или 443, и строка проходит проверки Wails BrowserOpenURL (wailsSafe по исходной строке
// и по url.String(), которую Wails и откроет). Одна проверка на разбор профиля и на открытие
// (App.OpenPrintSite): что пропущено здесь, кнопка откроет.
func CheckURL(s string) error { return checkURL("url", s) }

func checkURL(what, s string) error {
	if s == "" {
		return nil
	}
	if err := checkTextMax(what, s, false, maxURL); err != nil {
		return err
	}
	if !strings.HasPrefix(s, "https://") || len(s) == len("https://") || strings.ContainsAny(s, " \t\\") {
		return fmt.Errorf("%s: нужен адрес https://…", what)
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return fmt.Errorf("%s: адрес не разбирается или с именем пользователя", what)
	}
	switch {
	case !asciiHost(u.Hostname()):
		return fmt.Errorf("%s: хост — доменное имя с точкой, не IP-адрес и не localhost", what)
	case strings.Contains(u.Host, ":") && u.Port() != "443":
		return fmt.Errorf("%s: порт в адресе — только 443", what)
	case !wailsSafe(s) || !wailsSafe(u.String()):
		return fmt.Errorf("%s: в адресе знак из ; | ` $ < > * { } [ ] ( ) ~ ! — браузер его не откроет", what)
	}
	return nil
}

// asciiHost — доменное имя из латиницы, цифр, «-» и точек с хотя бы одной точкой: кириллическая «а»
// в хосте выглядит как латинская, а IP-литерал прячет, куда ведёт кнопка. Последняя метка — буквы
// или xn--…: числом браузер считает весь хост IPv4 в сокращённой записи (127.1, 0x7f.0.0.1,
// 0177.0.0.1, 10.1). localhost и *.localhost — свой компьютер. Метка xn--… — только каноническая
// (xnLabel): «xn---80aac2ak8aza» и «xn--google-» показ раскрыл бы в чужой хост (SEC-3).
func asciiHost(h string) bool {
	if len(h) < 3 || len(h) > 253 || !strings.Contains(h, ".") || net.ParseIP(h) != nil {
		return false
	}
	for _, r := range h {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '.' {
			return false
		}
	}
	labels := strings.Split(strings.ToLower(h), ".")
	for _, l := range labels {
		if l == "" || len(l) > 63 || strings.HasPrefix(l, "-") || strings.HasSuffix(l, "-") {
			return false
		}
		if rest, ok := strings.CutPrefix(l, "xn--"); ok {
			if _, ok := xnLabel(rest); !ok {
				return false
			}
		}
	}
	tld := labels[len(labels)-1]
	return tld != "localhost" && (strings.HasPrefix(tld, "xn--") || strings.Trim(tld, "abcdefghijklmnopqrstuvwxyz") == "")
}

// siteHost — хост адреса для сравнения сайтов: нижний регистр, без «www.» (www.X принадлежит
// владельцу X; решение 0.26-123-47). Адрес уже прошёл checkURL.
func siteHost(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// SiteHost — хост сайта типографии для сводки импорта и подписи кнопки сайта своих и библиотечных
// (printshop.siteButtonOf; у встроенных подпись — siteButton из файла; xn--… показан Юникодом, если
// хост из белого списка кириллицы и не двойник — DisplayHost): export.url, без него —
// адрес первого изделия (Validate держит все адреса на одном сайте, www.X = X); "" — адресов нет.
func (p Profile) SiteHost() string {
	urls := []string{}
	if p.Export != nil {
		urls = append(urls, p.Export.URL)
	}
	for _, pr := range p.Products {
		urls = append(urls, pr.URL)
	}
	for _, s := range urls {
		if u, err := url.Parse(s); s != "" && err == nil && u.Hostname() != "" {
			return DisplayHost(u.Hostname()) // xn--… раскрывается только для безопасной метки
		}
	}
	return ""
}

// checkHosts — адреса изделий на том же сайте, что export.url (без него — на одном сайте между
// собой): сводка импорта показывает один хост, а «Открыть сайт» сначала берёт адрес изделия —
// иначе чужой файл с настоящим export.url уводил бы кнопкой на fabrika-fotoknigi.com.evil.example.
func (p Profile) checkHosts() error {
	want := ""
	if p.Export != nil && p.Export.URL != "" {
		want = siteHost(p.Export.URL)
	}
	for _, pr := range p.Products {
		if pr.URL == "" {
			continue
		}
		switch h := siteHost(pr.URL); {
		case want == "":
			want = h
		case h != want:
			return fmt.Errorf("изделие %s: адрес на другом сайте, чем у типографии (%s)", q(pr.ID), want)
		}
	}
	return nil
}

// checkTemplate — в тексте только допустимые подстановки (allowed — например «{safe}»), других
// фигурных скобок нет.
func checkTemplate(what, s string, allowed ...string) error {
	for _, a := range allowed {
		s = strings.ReplaceAll(s, a, "")
	}
	if strings.ContainsAny(s, "{}") {
		return fmt.Errorf("%s: неизвестная подстановка", what)
	}
	return nil
}

// clip — строка из файла для текста ошибки: не длиннее errClip символов, иначе 64 КБ из файла
// попали бы в сообщение.
func clip(s string) string {
	if utf8.RuneCountInString(s) <= errClip {
		return s
	}
	return string([]rune(s)[:errClip]) + "…"
}

// q — значение из файла в кавычках для ошибки.
func q(s string) string { return strconv.Quote(clip(s)) }

const errClip = 60

// userError — причина отказа, уже написанная по-русски: Parse показывает её как есть, а не
// подменяет общей фразой.
type userError string

func (e userError) Error() string { return string(e) }

// reason — понятная причина ошибки разбора JSON. Тексты encoding/json английские и режутся
// clip посреди слова, поэтому пользователю (импорт файла) их не показываем: SyntaxError и
// UnmarshalTypeError переводятся, прочее — общая фраза.
func reason(err error) string {
	var (
		ue userError
		se *json.SyntaxError
		te *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &ue):
		return string(ue)
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "файл оборван"
	case errors.As(err, &se):
		return fmt.Sprintf("это не JSON (ошибка около байта %d)", se.Offset)
	case errors.As(err, &te):
		what := "значение другого вида"
		kind, rest, _ := strings.Cut(te.Value, " ") // «number 7180.5» — число не влезло в целое
		if w, ok := jsonKinds[kind]; ok {
			what = strings.TrimSpace(w + " " + clip(rest))
		}
		if te.Field != "" {
			return fmt.Sprintf("поле %s: %s, а нужно %s", q(te.Field), what, kindWord(te.Type))
		}
		return fmt.Sprintf("%s, а нужно %s", what, kindWord(te.Type))
	}
	return "содержимое не разбирается"
}

// jsonKinds — Value из UnmarshalTypeError по-русски.
var jsonKinds = map[string]string{"string": "строка", "number": "число", "bool": "да/нет", "array": "список", "object": "объект"}

// kindWord — вид значения, который ждёт поле Go-типа t.
func kindWord(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "строка"
	case reflect.Bool:
		return "да/нет"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "целое число"
	case reflect.Float32, reflect.Float64:
		return "число"
	case reflect.Slice, reflect.Array:
		return "список"
	}
	return "объект"
}
