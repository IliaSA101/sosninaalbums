package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// uiLogRig — App с подменёнными часами и записью журнала интерфейса.
type uiLogRig struct {
	a     *App
	mu    sync.Mutex
	now   time.Time
	lines []string
}

func newUILogRig(t *testing.T) *uiLogRig {
	t.Helper()
	r := &uiLogRig{a: newApp(t), now: time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)}
	r.a.cfgDir = testkit.TempDir(t)
	r.a.emit = func(string, ...any) {}
	r.a.uilog.now = func() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.now }
	r.a.uilog.write = func(s string) { r.lines = append(r.lines, s) }
	return r
}

func (r *uiLogRig) advance(d time.Duration) { r.mu.Lock(); r.now = r.now.Add(d); r.mu.Unlock() }

func TestLogUIClean(t *testing.T) {
	r := newUILogRig(t)
	r.a.LogUI("плохо\r\nвторая\nтретья\rчетвёртая\u202egfj.exe\x00\x1b[31m\u2066x\u2069")
	r.a.LogUI(`Error: не открыть C:\Users\Иван\Фото свадьба\a.jpg`)
	r.a.LogUI(strings.Repeat("я", 1000))
	r.a.LogUI("  \n\t ") // пусто после очистки
	if len(r.lines) != 3 {
		t.Fatalf("строк %d: %q", len(r.lines), r.lines)
	}
	if want := "ui: плохо ⏎ вторая ⏎ третья ⏎ четвёртаяgfj.exe[31mx"; r.lines[0] != want {
		t.Errorf("очистка: %q, ждали %q", r.lines[0], want)
	}
	for _, bad := range []string{"\u202e", "\u2066", "\x00", "\x1b", "\n", "\r"} {
		if strings.Contains(r.lines[0], bad) {
			t.Errorf("остался знак %q", bad)
		}
	}
	if r.lines[1] != "ui: Error: не открыть <путь>" {
		t.Errorf("путь не скрыт: %q", r.lines[1])
	}
	body := strings.TrimPrefix(r.lines[2], "ui: ")
	if n := utf8.RuneCountInString(body); n != 300 || !strings.HasSuffix(body, "…") {
		t.Errorf("длинная строка: %d знаков, ждали 300 с «…»", n)
	}
}

// Имена в «ёлочках» из ошибок Go скрываются целиком, текст вокруг остаётся.
func TestLogUIGuillemets(t *testing.T) {
	r := newUILogRig(t)
	r.a.LogUI("файл уже есть: «Иванова Анна.salbum» и «Петров (2).jpg», дальше")
	r.a.LogUI("нет закрывающей: «Иванова")
	if len(r.lines) != 2 {
		t.Fatalf("строк %d: %q", len(r.lines), r.lines)
	}
	if want := "ui: файл уже есть: «<имя>» и «<имя>», дальше"; r.lines[0] != want {
		t.Errorf("%q, ждали %q", r.lines[0], want)
	}
	if want := "ui: нет закрывающей: «Иванова"; r.lines[1] != want {
		t.Errorf("%q, ждали %q", r.lines[1], want)
	}
}

// Путь, торчащий за 300-м знаком, срезается маской до обрезки: имя не остаётся огрызком.
func TestLogUIMaskBeforeCut(t *testing.T) {
	r := newUILogRig(t)
	r.a.LogUI(strings.Repeat("x", 290) + ` C:\Users\Иван\a.jpg`)
	if strings.Contains(r.lines[0], "Иван") || strings.Contains(r.lines[0], `C:\`) {
		t.Errorf("обрывок пути: %q", r.lines[0])
	}
}

func TestLogUIRate(t *testing.T) {
	r := newUILogRig(t)
	for i := range 25 {
		r.a.LogUI(fmt.Sprintf("ошибка %d", i))
	}
	if len(r.lines) != 20 {
		t.Fatalf("за минуту записано %d строк, ждали 20", len(r.lines))
	}
	r.advance(61 * time.Second)
	r.a.LogUI("новая минута")
	if len(r.lines) != 22 || !strings.Contains(r.lines[20], "пропущено строк: 5") || r.lines[21] != "ui: новая минута" {
		t.Fatalf("итог минуты: %q", r.lines[20:])
	}
	// 20 + итог + новая — по одной строке итога на окно, не больше 21 за минуту.
	r.advance(61 * time.Second)
	r.a.LogUI("ещё")
	if len(r.lines) != 23 {
		t.Errorf("итога без пропусков и повторов быть не должно: %q", r.lines[22:])
	}
}

func TestLogUIRepeat(t *testing.T) {
	r := newUILogRig(t)
	for range 7 {
		r.a.LogUI("одна и та же")
	}
	if len(r.lines) != 1 {
		t.Fatalf("строк %d, ждали 1", len(r.lines))
	}
	r.advance(time.Minute)
	r.a.LogUI("другая")
	if len(r.lines) != 3 || !strings.Contains(r.lines[1], "×7 «одна и та же»") || strings.Contains(r.lines[1], "пропущено") {
		t.Fatalf("итог минуты: %q", r.lines)
	}
	// Несколько повторяющихся текстов — один итог, по убыванию счётчика.
	for range 3 {
		r.a.LogUI("редкая")
	}
	for range 5 {
		r.a.LogUI("частая")
	}
	r.a.uilog.flush()
	last := r.lines[len(r.lines)-1]
	if !strings.Contains(last, "×5 «частая», ×3 «редкая»") {
		t.Errorf("итог: %q", last)
	}
	// Повторный сброс без новых событий молчит.
	n := len(r.lines)
	r.a.uilog.flush()
	if len(r.lines) != n {
		t.Errorf("пустой итог записан: %q", r.lines[n:])
	}
}

func TestLogUIHugeInput(t *testing.T) {
	r := newUILogRig(t)
	huge := `C:\Users\Иван\` + strings.Repeat("я", 1<<20)
	// Лучший из пяти: очистка мегабайта идёт при каждом вызове, повторы в журнал не попадают.
	d := testkit.Fastest(5, func() { r.a.LogUI(huge) })
	if len(r.lines) != 1 || utf8.RuneCountInString(strings.TrimPrefix(r.lines[0], "ui: ")) > 300 || strings.Contains(r.lines[0], "Иван") {
		t.Errorf("строка: %d рун, %q", utf8.RuneCountInString(r.lines[0]), r.lines)
	}
	// Мегабайт невалидного UTF-8 — тоже без падения и без мусора.
	r.a.LogUI(strings.Repeat("\xff\xfe", 1<<19))
	if len(r.lines) != 1 {
		t.Errorf("невалидный UTF-8 дал строки: %q", r.lines[1:])
	}
	testkit.CostTest(t)
	if d > testkit.Limit(50*time.Millisecond) {
		t.Errorf("мегабайт на входе обработан за %v", d)
	}
}

// Параллельные вызовы из горутин Wails: замок держит счёт (запускается и под -race).
func TestLogUIParallel(t *testing.T) {
	r := newUILogRig(t)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 50 {
				r.a.LogUI(fmt.Sprintf("g%d-%d", g, i%5))
			}
		}()
	}
	wg.Wait()
	if len(r.lines) > uiLogMaxLines {
		t.Errorf("строк %d, больше предела %d", len(r.lines), uiLogMaxLines)
	}
}

func TestLogUIShutdownFlush(t *testing.T) {
	r := newUILogRig(t)
	for range 4 {
		r.a.LogUI("повтор")
	}
	if len(r.lines) != 1 {
		t.Fatalf("до выхода строк %d", len(r.lines))
	}
	r.a.shutdown(context.Background())
	if len(r.lines) != 2 || !strings.Contains(r.lines[1], "×4 «повтор»") {
		t.Fatalf("итог при выходе: %q", r.lines)
	}
}
