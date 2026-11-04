package albums

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Validate — манифест от клиента годен (design §2.2). Одна проверка для программы (до сети) и для
// сервиса (на входе): пределы формата строгие, пределы сервиса — из lim (нули — DefaultLimits).
// Page.File на входе обязан быть пуст: имена файлов даёт сервис, имена от клиента на диск не идут.
// Ошибка не содержит значений полей.
func Validate(m *Manifest, lim Limits) error {
	if m == nil {
		return bad("manifest", "пусто")
	}
	lim = lim.OrDefault()
	if m.Schema != SchemaVersion {
		return bad("schema", "поддерживается только 1")
	}
	if err := checkGenerator(m.Generator); err != nil {
		return err
	}
	if err := checkTitle(m.Title, lim.TitleLen); err != nil {
		return err
	}
	if w, h := m.Book.PageMM.W, m.Book.PageMM.H; !(w >= MinPageMM && w <= MaxPageMM && h >= MinPageMM && h <= MaxPageMM) {
		return badf("book.page_mm", "стороны вне %d…%d мм", MinPageMM, MaxPageMM)
	}
	k := len(m.Pages)
	if k == 0 || k%2 != 0 {
		return bad("pages", "нужно чётное число страниц, не меньше двух")
	}
	if k > lim.Pages {
		return badf("pages", "больше предела %d", lim.Pages)
	}
	var total int64
	for i := range m.Pages {
		p := &m.Pages[i]
		if err := checkPage(p, i, k, m.Book.Cover, lim.PageBytes); err != nil {
			return err
		}
		// total + Bytes без переполнения: Bytes ≤ PageBytes, но PageBytes задаёт сервис.
		if p.Bytes > lim.AlbumBytes-total {
			return bad("pages", "сумма байт больше предела album_bytes")
		}
		total += p.Bytes
	}
	return nil
}

func checkGenerator(g string) error {
	if g == "" || !utf8.ValidString(g) || utf8.RuneCountInString(g) > MaxGenerator {
		return badf("generator", "нужно 1…%d печатных знаков", MaxGenerator)
	}
	for _, r := range g {
		if !unicode.IsPrint(r) {
			return bad("generator", "непечатный знак")
		}
	}
	return nil
}

func checkTitle(t string, limit int) error {
	if !utf8.ValidString(t) || len(t) > MaxTitleBytes || utf8.RuneCountInString(t) > limit {
		return badf("title", "не UTF-8 или длиннее %d знаков / %d байт", limit, MaxTitleBytes)
	}
	if strings.TrimSpace(t) == "" {
		return bad("title", "пусто")
	}
	for _, r := range t {
		if invisible(r) {
			return bad("title", "управляющие и невидимые знаки не допускаются")
		}
	}
	return nil
}

// checkPage — страница i (с нуля) из k: номер, роль, белая или с файлом. Поля пути — pages[i].…
func checkPage(p *Page, i, k int, cover bool, maxBytes int64) error {
	at := func(f string) string { return "pages[" + strconv.Itoa(i) + "]." + f }
	if p.N != i+1 {
		return bad(at("n"), "номера идут подряд с 1")
	}
	if p.File != "" {
		return bad(at("file"), "имя файла даёт сервис, на входе поле пусто")
	}
	switch p.Role {
	case RoleFront:
		if !cover || i != 0 {
			return bad(at("role"), "front — только первая страница книги с обложкой")
		}
	case RoleBack:
		if !cover || i != k-1 {
			return bad(at("role"), "back — только последняя страница книги с обложкой")
		}
	case RolePage:
		if cover && (i == 0 || i == k-1) {
			return bad(at("role"), "у книги с обложкой первая страница front, последняя back")
		}
	default:
		return bad(at("role"), "front, back или page")
	}
	if p.Blank {
		if p.Type != "" || p.SHA256 != "" || p.Bytes != 0 || p.W != 0 || p.H != 0 {
			return bad(at("blank"), "у белой страницы нет файла: type, sha256, bytes, w, h пусты")
		}
		return nil
	}
	if p.Type != TypeJPEG && p.Type != TypeWebP {
		return bad(at("type"), "image/jpeg или image/webp")
	}
	if !isSHA256(p.SHA256) {
		return bad(at("sha256"), "64 шестнадцатеричных знака в нижнем регистре")
	}
	if p.Bytes < 1 || p.Bytes > maxBytes {
		return badf(at("bytes"), "вне 1…%d", maxBytes)
	}
	if p.W < 1 || p.W > MaxSide || p.H < 1 || p.H > MaxSide {
		return badf(at("w"), "размеры вне 1…%d px", MaxSide)
	}
	return nil
}

func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ValidateMissing сверяет список missing из ответа сервера с манифестом: только номера небелых
// страниц, без повторов (разбор №13). Иначе сервис ответил непонятно.
func ValidateMissing(m *Manifest, missing []int) error {
	seen := make(map[int]bool, len(missing))
	for _, n := range missing {
		if n < 1 || n > len(m.Pages) || m.Pages[n-1].Blank || seen[n] {
			return bad("missing", "номер вне небелых страниц манифеста или повтор")
		}
		seen[n] = true
	}
	return nil
}
