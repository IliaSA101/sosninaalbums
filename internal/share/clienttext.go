package share

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// Link — всё, что клиенту нужно знать об альбоме: название, ссылка, пароль («» — без него) и
// срок (нулевой — не указан).
type Link struct {
	Title     string
	URL       string
	Password  string
	ExpiresAt time.Time
}

// MaxTitle — название альбома, знаков (design §2.2).
const (
	MaxTitle      = albums.MaxTitle
	MaxTitleBytes = albums.MaxTitleBytes // и в байтах (разбор С7)
)

// months — месяцы в родительном падеже («до 29 ноября»).
var months = [...]string{"января", "февраля", "марта", "апреля", "мая", "июня", "июля", "августа", "сентября", "октября", "ноября", "декабря"}

// DateRU — дата по-русски: «29 ноября 2026» (в часовом поясе самого значения).
func DateRU(t time.Time) string {
	return strconv.Itoa(t.Day()) + " " + months[t.Month()-1] + " " + strconv.Itoa(t.Year())
}

// ClientText — текст для мессенджера «Скопировать для клиента» (design §3.6):
//
//	Альбом «Свадьба И. и А.»: https://…/a/k3J/
//	Пароль: лето-4827
//	Ссылка работает до 29 ноября 2026.
//
// Без пароля нет второй строки, без срока — третьей. Название чистится как в манифесте.
func ClientText(l Link) string {
	var b strings.Builder
	b.WriteString("Альбом «" + CleanTitle(l.Title, MaxTitle) + "»: " + l.URL)
	if l.Password != "" {
		b.WriteString("\nПароль: " + l.Password)
	}
	if !l.ExpiresAt.IsZero() {
		b.WriteString("\nСсылка работает до " + DateRU(l.ExpiresAt) + ".")
	}
	return b.String()
}

// CleanTitle — название для манифеста и текста клиенту: без управляющих и невидимых знаков,
// пробельные (в том числе переводы строк) сведены к одному пробелу, края обрезаны, не длиннее
// limit знаков (≤ 0 — MaxTitle); пустое — «Альбом». Название видит клиент, поэтому строка
// пользователя уходит только очищенной.
func CleanTitle(s string, limit int) string {
	if limit <= 0 {
		limit = MaxTitle
	}
	var b strings.Builder
	space, n := false, 0
	for _, r := range strings.ToValidUTF8(s, "") {
		switch {
		case unicode.IsSpace(r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			space = n > 0
			continue
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r):
			continue
		}
		need := 1
		if space {
			need = 2
		}
		if n+need > limit || b.Len()+utf8.RuneLen(r)+need-1 > MaxTitleBytes {
			break
		}
		if space {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		n += need
		space = false
	}
	if b.Len() == 0 {
		return "Альбом"
	}
	return b.String()
}
