package albums

import (
	"fmt"
	"strconv"
	"time"
	"unicode"
	"unicode/utf8"
)

// ValidationError — отказ проверки. Текст короткий и не содержит значения поля: проверяемая строка
// пришла от клиента или с сервера, в журнал и тост её не несём.
type ValidationError struct {
	Field  string // путь к полю: "pages[3].sha256"
	Reason string
}

func (e *ValidationError) Error() string { return "albums: " + e.Field + ": " + e.Reason }

func bad(field, reason string) error { return &ValidationError{Field: field, Reason: reason} }

func badf(field, format string, a ...any) error {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, a...)}
}

// invisible — знак, которого в названии и пароле быть не должно: управляющий (Cc), форматирующий
// (Cf: bidi, нулевая ширина), разделитель строки и абзаца.
func invisible(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r)
}

// CheckID — id альбома годен для подстановки в путь: 22…64 знака [A-Za-z0-9_-], первый не «-»
// (128 бит base64url; первый знак — защита от аргумента внешней команды, разбор №22).
func CheckID(id string) error {
	if len(id) < MinID || len(id) > MaxID {
		return bad("id", "длина вне "+strconv.Itoa(MinID)+"…"+strconv.Itoa(MaxID))
	}
	if id[0] == '-' {
		return bad("id", "первый знак «-»")
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return bad("id", "знак вне A-Z a-z 0-9 _ -")
		}
	}
	return nil
}

// CheckToken — токен годен для заголовка Authorization: 16…512 печатных ASCII без пробелов
// (CR/LF не попадут в заголовок; предел — запас под хранилище ключей Windows, разбор №10).
func CheckToken(tok string) error {
	if len(tok) < MinToken || len(tok) > MaxToken {
		return bad("token", "длина вне "+strconv.Itoa(MinToken)+"…"+strconv.Itoa(MaxToken))
	}
	for i := 0; i < len(tok); i++ {
		if tok[i] <= ' ' || tok[i] >= 0x7f {
			return bad("token", "только печатные знаки ASCII без пробелов")
		}
	}
	return nil
}

// CheckPassword — пароль альбома годен: пустой («без пароля») или 4…64 знака, ≤ 256 байт, корректный
// UTF-8, без управляющих и невидимых (пароль в мегабайты — DoS хэшированием, разбор С7).
func CheckPassword(p string) error {
	if p == "" {
		return nil
	}
	if len(p) > MaxPasswordSize || !utf8.ValidString(p) {
		return bad("password", "слишком длинный или не UTF-8")
	}
	n := utf8.RuneCountInString(p)
	if n < MinPassword || n > MaxPassword {
		return badf("password", "длина вне %d…%d знаков", MinPassword, MaxPassword)
	}
	for _, r := range p {
		if invisible(r) {
			return bad("password", "управляющие и невидимые знаки не допускаются")
		}
	}
	return nil
}

// ParsePageN — разбор {n} из пути: десятичное число 1…m без знака, пробелов и ведущих нулей.
func ParsePageN(s string, m int) (int, error) {
	if s == "" || len(s) > 9 || s[0] == '0' {
		return 0, bad("n", "не номер страницы")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, bad("n", "не номер страницы")
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > m {
		return 0, bad("n", "номер вне книги")
	}
	return n, nil
}

// ValidateCreate проверяет запрос создания: манифест (Validate), срок в будущем и пароль. now —
// текущее время вызывающего. Срок сверх max_days сервис зажимает сам и возвращает итоговый.
func ValidateCreate(r *CreateRequest, lim Limits, now time.Time) error {
	if r == nil {
		return bad("request", "пусто")
	}
	if err := Validate(&r.Manifest, lim); err != nil {
		return err
	}
	if err := checkExpiry("expires_at", r.ExpiresAt, now); err != nil {
		return err
	}
	return CheckPassword(r.Password)
}

// ValidatePatch проверяет правку: срок (если указан) в будущем, пароль (если указан) годен. Пустая
// правка допустима — ответ тот же Album.
func ValidatePatch(p *Patch, now time.Time) error {
	if p == nil {
		return bad("request", "пусто")
	}
	if p.ExpiresAt != nil {
		if err := checkExpiry("expires_at", *p.ExpiresAt, now); err != nil {
			return err
		}
	}
	if p.Password != nil {
		return CheckPassword(*p.Password)
	}
	return nil
}

func checkExpiry(field string, t, now time.Time) error {
	if t.IsZero() {
		return bad(field, "не указан")
	}
	if !t.After(now) {
		return bad(field, "не в будущем")
	}
	return nil
}
