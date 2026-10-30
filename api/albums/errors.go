package albums

import (
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Ошибки по статусу: errors.Is(err, albums.ErrToken) — для ветвления вызывающего, текст — для журнала.
var (
	ErrToken    = errors.New("albums: токен не подошёл (401)")
	ErrGone     = errors.New("albums: альбома нет на сервисе (404)")
	ErrLimit    = errors.New("albums: превышен предел сервиса (413)")
	ErrBusy     = errors.New("albums: сервис просит подождать (429)")
	ErrRedirect = errors.New("albums: сервис перенаправляет запрос — проверьте адрес")
	// ErrResponse — ответ не по контракту: не JSON, чужие значения, пропущенные поля.
	ErrResponse = errors.New("albums: сервис ответил непонятно")
	// ErrTooBig — ответ (в том числе тело ошибки) больше MaxBody.
	ErrTooBig = errors.New("albums: ответ больше 1 МБ")
)

// APIError — ошибка сервиса по контракту: {"error": {"code", "message"}} со статусом HTTP.
// Error() — только статус и код (в журнал); Message — отдельным полем, обрезан, очищен, токен
// заменён на «***» (разбор С4: прокси и отладочные страницы повторяют заголовки в теле).
type APIError struct {
	Status  int
	Code    string // [a-z_]{1,40}; иначе пусто
	Message string // для показа разработчику; не для журнала
	// Missing — только у 409 pages_missing: номера 1…limits.pages без повторов (мусор отброшен). Чужой сервер
	// мог назвать белую или несуществующую страницу — сверять с манифестом через ValidateMissing, не брать индексом.
	Missing    []int
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	s := "albums: " + strconv.Itoa(e.Status)
	if e.Code != "" {
		s += " " + e.Code
	}
	return s
}

// cleanMissing — список missing из тела ошибки: не длиннее top, номера 1…top, без повторов.
func cleanMissing(in []int, top int) []int {
	if len(in) > top {
		return nil
	}
	var out []int
	seen := make(map[int]bool, len(in))
	for _, n := range in {
		if n >= 1 && n <= top && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// Is связывает статус с общими ошибками: 401, 404, 413, 429.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrToken:
		return e.Status == 401
	case ErrGone:
		return e.Status == 404
	case ErrLimit:
		return e.Status == 413
	case ErrBusy:
		return e.Status == 429
	}
	return false
}

// NetError — сбой связи (адрес, TLS, тайм-аут, обрыв). Без пути запроса: в нём id (разбор С4).
// Unwrap отдаёт причину для errors.Is(context.Canceled / DeadlineExceeded).
type NetError struct {
	Method string
	Host   string
	msg    string
	err    error
}

func (e *NetError) Error() string { return "albums: " + e.Method + " " + e.Host + ": " + e.msg }
func (e *NetError) Unwrap() error { return e.err }

const maxMessage = 200

// cleanMessage — сообщение сервера для показа: корректный UTF-8, без управляющих, не длиннее
// maxMessage знаков; вхождения secrets заменены на «***».
func cleanMessage(s string, secrets ...string) string {
	s = redact(s, secrets...)
	var b strings.Builder
	n := 0
	for _, r := range strings.ToValidUTF8(s, "") {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if n++; n > maxMessage {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

// redact заменяет вхождения каждой непустой secrets на «***».
func redact(s string, secrets ...string) string {
	for _, x := range secrets {
		if x != "" {
			s = strings.ReplaceAll(s, x, "***")
		}
	}
	return s
}

// cleanCode — код ошибки сервера допустим для Error(): строчные латинские и «_», до 40 знаков.
func cleanCode(c string) string {
	if c == "" || len(c) > 40 {
		return ""
	}
	for i := 0; i < len(c); i++ {
		if (c[i] < 'a' || c[i] > 'z') && c[i] != '_' {
			return ""
		}
	}
	return c
}
