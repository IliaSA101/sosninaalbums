package app

import (
	"fmt"
	"log"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы журнала интерфейса (092 §3): вход до разбора, строка после очистки, строк в окне.
const (
	uiLogMaxIn    = 4096
	uiLogMaxLine  = 300
	uiLogMaxLines = 20
	uiLogWindow   = time.Minute
	uiLogHead     = 60 // начало текста в строке итога
)

// uiLog — состояние журнала ошибок интерфейса: окно в минуту, 20 строк, повторы счётчиком.
// Нулевое значение рабочее (часы — time.Now, запись — log.Print); в тесте оба подменяются.
type uiLog struct {
	mu      sync.Mutex
	now     func() time.Time
	w0      time.Time      // начало окна
	lines   int            // записано строк в окне
	dropped int            // не записано сверх лимита
	rep     map[string]int // текст → повторов сверх первого (только записанные в окне)
	write   func(string)
}

// LogUI — строка журнала от интерфейса (ошибки скриптов). Текст чистится, пути скрываются;
// поток ограничен: 20 строк в минуту, повторы — счётчиком (092 §3). Никогда не отказывает:
// лишний текст просто не пишется.
func (a *App) LogUI(text string) { a.uilog.add(text) }

func (u *uiLog) add(text string) {
	text = cleanUIText(text)
	if text == "" {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.clock()
	if now.Sub(u.w0) >= uiLogWindow {
		u.flushLocked()
		u.w0 = now
	}
	if _, seen := u.rep[text]; seen {
		u.rep[text]++
		return
	}
	if u.lines >= uiLogMaxLines {
		u.dropped++
		return
	}
	if u.rep == nil {
		u.rep = map[string]int{}
	}
	u.out("ui: " + text)
	u.rep[text] = 0
	u.lines++
}

// flush пишет итог окна (повторы и пропуски), если они есть; зовётся и при выходе.
func (u *uiLog) flush() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.flushLocked()
}

func (u *uiLog) flushLocked() {
	type rep struct {
		text string
		n    int
	}
	var reps []rep
	for t, n := range u.rep {
		if n > 0 {
			reps = append(reps, rep{t, n + 1}) // ×N — сколько раз текст встретился всего
		}
	}
	dropped := u.dropped
	u.rep, u.lines, u.dropped = nil, 0, 0
	if len(reps) == 0 && dropped == 0 {
		return
	}
	slices.SortFunc(reps, func(x, y rep) int {
		if x.n != y.n {
			return y.n - x.n
		}
		return strings.Compare(x.text, y.text)
	})
	var parts []string
	for _, r := range reps {
		parts = append(parts, fmt.Sprintf("×%d «%s»", r.n, cutRunes(r.text, uiLogHead)))
	}
	line := "ui: за минуту:"
	if len(parts) > 0 {
		line += " повторы " + strings.Join(parts, ", ")
	}
	if dropped > 0 {
		if len(parts) > 0 {
			line += ";"
		}
		line += fmt.Sprintf(" пропущено строк: %d", dropped)
	}
	u.out(cutRunes(line, uiLogMaxLine))
}

func (u *uiLog) clock() time.Time {
	if u.now != nil {
		return u.now()
	}
	return time.Now()
}

func (u *uiLog) out(s string) {
	if u.write != nil {
		u.write(s)
		return
	}
	log.Print(s)
}

// uiNewlines — переводы строк (и разделители строк Unicode) сводятся к видимому знаку.
var uiNewlines = strings.NewReplacer("\r\n", " ⏎ ", "\n", " ⏎ ", "\r", " ⏎ ", " ", " ⏎ ", " ", " ⏎ ")

// reUIQuoted — имя файла в «ёлочках» («файл уже есть: «Иванова.salbum»»): скрывается целиком.
var reUIQuoted = regexp.MustCompile("«[^»]*»")

// cleanUIText — безопасная для журнала строка: вход режется до 4 КБ (мегабайтный аргумент не
// гоняет регулярные выражения), пути скрываются ДО обрезки по длине (срезанный путь не узнать),
// управляющие знаки и знаки смены направления убираются, строка — не длиннее 300 знаков.
func cleanUIText(s string) string {
	if len(s) > uiLogMaxIn {
		s = s[:uiLogMaxIn]
	}
	s = strings.ToValidUTF8(s, "")
	s = reUIQuoted.ReplaceAllString(s, "«<имя>»") // до путей: имя в «ёлочках» из ошибок Go
	s = paths.MaskText(s)
	s = uiNewlines.Replace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r), // C0, DEL, C1
			r == 0x200E, r == 0x200F,
			r >= 0x202A && r <= 0x202E,
			r >= 0x2066 && r <= 0x2069:
			return -1
		}
		return r
	}, s)
	return cutRunes(strings.Trim(s, " ⏎"), uiLogMaxLine) // края: пробелы и пустые переводы строк
}

// cutRunes режет s до n знаков; обрезанное заканчивается «…» (в пределах n).
func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
