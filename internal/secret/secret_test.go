package secret

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeBackend — хранилище ОС в тесте: словарь, управляемые отказ и зависание.
type fakeBackend struct {
	mu      sync.Mutex
	m       map[string]string
	fail    error         // все вызовы отказывают этой ошибкой
	failDel error         // только удаление отказывает этой ошибкой
	block   chan struct{} // не nil — вызовы ждут закрытия канала (зависание Secret Service)
	calls   int
}

// n — число вызовов ОС (с замком: зависшие вызовы считают из своих горутин).
func (f *fakeBackend) n() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newFake() *fakeBackend { return &fakeBackend{m: map[string]string{}} }

func (f *fakeBackend) enter() error {
	f.mu.Lock()
	f.calls++
	b, fail := f.block, f.fail
	f.mu.Unlock()
	if b != nil {
		<-b
	}
	return fail
}

func (f *fakeBackend) get(a string) (string, error) {
	if err := f.enter(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.m[a]
	if !ok {
		return "", errNotFound
	}
	return t, nil
}

func (f *fakeBackend) set(a, t string) error {
	if err := f.enter(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[a] = t
	return nil
}

func (f *fakeBackend) del(a string) error {
	if err := f.enter(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failDel != nil {
		return f.failDel
	}
	if _, ok := f.m[a]; !ok {
		return errNotFound
	}
	delete(f.m, a)
	return nil
}

const (
	svcA = "https://albums.sosnina-films.ru"
	svcB = "https://evil.example.org"
	svcC = "https://third.example"
)

func TestOrigin(t *testing.T) {
	ok := map[string]string{
		"https://albums.sosnina-films.ru":      svcA,
		"  HTTPS://Albums.Sosnina-Films.RU/ ":  svcA,
		"https://albums.sosnina-films.ru:443":  svcA,
		"http://127.0.0.1:34190":               "http://127.0.0.1:34190",
		"http://localhost:80/":                 "http://localhost",
		"https://host.example:8443":            "https://host.example:8443",
		"http://[::1]:8080":                    "http://[::1]:8080",
		"http://[0:0:0:0:0:0:0:1]":             "http://[::1]",
		"http://[::ffff:127.0.0.1]:34190":      "http://127.0.0.1:34190",
		"https://203.0.113.5:65535":            "https://203.0.113.5:65535",
		"https://albums.sosnina-films.ru:8443": "https://albums.sosnina-films.ru:8443",
	}
	for in, want := range ok {
		got, err := Origin(in)
		if err != nil || got != want {
			t.Errorf("Origin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"https://2130706433", "https://127.1", "https://0x7f.0.0.1", "https://0127.0.0.1", "https://host.example:0",
		"https://host.example:070", "https://host.example:65536", "https://host.example:80a",
		"", "   ", "ftp://host.example", "host.example", "https://", "https://u:p@host.example",
		"https://host.example/path", "https://host.example?x=1", "https://host.example#a",
		"https://host.example\\evil", "https://host%41.example", "https://хост.рф", "https://-.", "https://[fe80::1%25eth0]", "https://host .example",
		"https://host.\nexample", "https://" + strings.Repeat("a", 250) + ".example",
	}
	for _, in := range bad {
		if got, err := Origin(in); !errors.Is(err, ErrBadOrigin) {
			t.Errorf("Origin(%q) = %q, %v; want ErrBadOrigin", in, got, err)
		}
	}
	// Один сервис — один ключ, как ни записан адрес.
	a, _ := Origin("HTTPS://albums.sosnina-films.ru:443/")
	b, _ := Origin(svcA)
	if a != b {
		t.Errorf("ключи разные: %q и %q", a, b)
	}
}

func TestCheckToken(t *testing.T) {
	for _, tok := range []string{"abc123_-.~", strings.Repeat("a", maxToken)} {
		if err := checkToken(tok); err != nil {
			t.Errorf("checkToken(%.10q): %v", tok, err)
		}
	}
	if err := checkToken(""); !errors.Is(err, ErrEmpty) {
		t.Errorf("пустой: %v", err)
	}
	for _, tok := range []string{"a b", "a\nb", "a\x00", "токен", "a\x7f", "a\tb", strings.Repeat("a", maxToken+1)} {
		if err := checkToken(tok); !errors.Is(err, ErrBadToken) {
			t.Errorf("checkToken(%.10q) = %v; want ErrBadToken", tok, err)
		}
	}
}

// Токен одного origin никогда не отдаётся другому: ни из памяти, ни из ОС (С2).
func TestSecretMemoryByOrigin(t *testing.T) {
	stores := map[string]Store{
		"память":  NewMemory(),
		"система": newSystem(newFake(), time.Second, time.Now),
		"ОС отказывает": func() Store {
			f := newFake()
			f.fail = errors.New("нет D-Bus")
			return newSystem(f, time.Second, time.Now)
		}(),
	}
	for name, s := range stores {
		t.Run(name, func(t *testing.T) { byOrigin(t, s) })
	}
}

// byOrigin: два сервиса в одном хранилище не видят токенов друг друга.
func byOrigin(t *testing.T, s Store) {
	if _, err := s.Set(svcA, "tokenA"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Set(svcB, "tokenB"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(svcA); got != "tokenA" {
		t.Errorf("A = %q", got)
	}
	if got, _ := s.Get(svcB); got != "tokenB" {
		t.Errorf("B = %q", got)
	}
	if got, _ := s.Get("https://third.example"); got != "" {
		t.Errorf("третий origin получил %q", got)
	}
	// Другой порт и другая схема — другой origin.
	for _, o := range []string{svcA + ":8443", "http://albums.sosnina-films.ru", "https://albums.sosnina-films.ru.evil.example"} {
		if got, _ := s.Get(o); got != "" {
			t.Errorf("Get(%q) отдал токен %q", o, got)
		}
	}
	// Записанный иначе адрес того же сервиса — тот же токен.
	if got, _ := s.Get("HTTPS://Albums.Sosnina-Films.RU:443/"); got != "tokenA" {
		t.Errorf("тот же сервис по другой записи: %q", got)
	}
	// Удаление одного не трогает другой.
	if err := s.Delete(svcA); err != nil && !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if got, _ := s.Get(svcA); got != "" {
		t.Errorf("после Delete A = %q", got)
	}
	if got, _ := s.Get(svcB); got != "tokenB" {
		t.Errorf("Delete A задел B: %q", got)
	}
	// Перезапись: новое значение, не старое.
	if _, err := s.Set(svcB, "tokenB2"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(svcB); got != "tokenB2" {
		t.Errorf("после перезаписи B = %q", got)
	}
}

func TestStoreBadInput(t *testing.T) {
	f := newFake()
	for name, s := range map[string]Store{"память": NewMemory(), "система": newSystem(f, time.Second, time.Now)} {
		if _, err := s.Set(svcA, ""); !errors.Is(err, ErrEmpty) {
			t.Errorf("%s: пустой токен: %v", name, err)
		}
		if _, err := s.Set(svcA, "a b"); !errors.Is(err, ErrBadToken) {
			t.Errorf("%s: токен с пробелом: %v", name, err)
		}
		if _, err := s.Set("не адрес", "tok"); !errors.Is(err, ErrBadOrigin) {
			t.Errorf("%s: негодный origin: %v", name, err)
		}
		if got, _ := s.Get("не адрес"); got != "" {
			t.Errorf("%s: Get негодного origin: %q", name, got)
		}
		if err := s.Delete("не адрес"); !errors.Is(err, ErrBadOrigin) {
			t.Errorf("%s: Delete негодного origin: %v", name, err)
		}
		if err := s.Delete(svcA); err != nil { // нет токена — не ошибка
			t.Errorf("%s: Delete без токена: %v", name, err)
		}
	}
	if f.n() != 1 { // только последний Delete дошёл до ОС
		t.Errorf("негодный ввод дошёл до ОС: %d вызовов", f.n())
	}
}

func TestSystemPlaces(t *testing.T) {
	f := newFake()
	s := newSystem(f, time.Second, time.Now)
	if tok, pl := s.Get(svcA); tok != "" || pl != InSystem {
		t.Errorf("пусто: %q, %s", tok, pl)
	}
	if pl, err := s.Set(svcA, "tok"); err != nil || pl != InSystem {
		t.Fatalf("Set: %s, %v", pl, err)
	}
	if f.m[svcA] != "tok" {
		t.Errorf("в ОС: %v", f.m)
	}
	if tok, pl := s.Get(svcA); tok != "tok" || pl != InSystem {
		t.Errorf("Get: %q, %s", tok, pl)
	}
	// ОС отказала: токен в памяти, место — сессия; в журнале текст отказа без токена.
	logs := captureLog(t)
	f.fail = errors.New("secret service отказал для tokB")
	if pl, err := s.Set(svcB, "tokB"); err != nil || pl != InSession {
		t.Fatalf("Set при отказе: %s, %v", pl, err)
	}
	if tok, pl := s.Get(svcB); tok != "tokB" || pl != InSession {
		t.Errorf("Get из памяти: %q, %s", tok, pl)
	}
	// ОС вернулась: новый Set уходит в ОС и снимает токен из памяти.
	f.fail = nil
	if pl, err := s.Set(svcB, "tokB2"); err != nil || pl != InSystem {
		t.Fatalf("Set: %s, %v", pl, err)
	}
	if tok, pl := s.Get(svcB); tok != "tokB2" || pl != InSystem {
		t.Errorf("после возврата ОС: %q, %s", tok, pl)
	}
	if err := s.Delete(svcB); err != nil || len(f.m) != 1 {
		t.Errorf("Delete: %v, ОС %v", err, f.m)
	}
	if strings.Contains(logs.String(), "tokB") || !strings.Contains(logs.String(), "***") {
		t.Errorf("журнал отказа: %q", logs.String())
	}
}

// captureLog перенаправляет журнал в буфер на время теста.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return &buf
}

// Delete при отказе ОС: токен на месте — ErrKept; токена нет — nil; чтение тоже отказало или
// тайм-аут — мягкая ErrUnavailable; из памяти токен уходит всегда.
func TestSecretDelete(t *testing.T) {
	deny := errors.New("доступ запрещён")
	t.Run("токен остался", func(t *testing.T) {
		f := newFake()
		s := newSystem(f, time.Second, time.Now)
		if _, err := s.Set(svcA, "tok"); err != nil {
			t.Fatal(err)
		}
		f.failDel = deny
		if err := s.Delete(svcA); !errors.Is(err, ErrKept) {
			t.Errorf("Delete: %v, ждали ErrKept", err)
		}
		if f.m[svcA] != "tok" {
			t.Errorf("фейк потерял токен: %v", f.m)
		}
	})
	t.Run("токена в ОС нет", func(t *testing.T) {
		f := newFake()
		f.failDel = deny
		s := newSystem(f, time.Second, time.Now)
		if err := s.Delete(svcA); err != nil {
			t.Errorf("Delete без токена: %v", err)
		}
	})
	t.Run("ОС не читает", func(t *testing.T) {
		f := newFake()
		f.fail = deny
		s := newSystem(f, time.Second, time.Now)
		_, _ = s.Set(svcA, "tok") // отказ: токен в памяти
		if err := s.Delete(svcA); !errors.Is(err, ErrUnavailable) {
			t.Errorf("Delete: %v, ждали ErrUnavailable", err)
		}
		if tok, _ := s.Get(svcA); tok != "" {
			t.Errorf("токен остался в памяти: %q", tok)
		}
	})
	t.Run("тайм-аут", func(t *testing.T) {
		f := newFake()
		f.block = make(chan struct{})
		defer close(f.block)
		s := newSystem(f, 20*time.Millisecond, time.Now)
		if err := s.Delete(svcA); !errors.Is(err, ErrUnavailable) {
			t.Errorf("Delete: %v, ждали ErrUnavailable", err)
		}
	})
	t.Run("негодный токен в ОС не отдаётся", func(t *testing.T) {
		f := newFake()
		f.m[svcA] = "a b\n" // подложено другой программой
		s := newSystem(f, time.Second, time.Now)
		if tok, pl := s.Get(svcA); tok != "" || pl != InSystem {
			t.Errorf("Get: %q, %s", tok, pl)
		}
	})
}

// Зависшее хранилище: через 15 с (здесь короче) токен уходит в память, пауза не даёт
// повторно ждать при каждом вызове (№9).
func TestSecretTimeout(t *testing.T) {
	f := newFake()
	f.block = make(chan struct{})
	defer close(f.block)
	var mu sync.Mutex
	clock := time.Unix(1_000_000, 0)
	now := func() time.Time { mu.Lock(); defer mu.Unlock(); return clock }
	s := newSystem(f, 30*time.Millisecond, now)

	start := time.Now()
	pl, err := s.Set(svcA, "tok")
	if err != nil || pl != InSession {
		t.Fatalf("Set при зависании: %s, %v", pl, err)
	}
	if d := time.Since(start); d < 25*time.Millisecond || d > 2*time.Second {
		t.Errorf("Set ждал %v", d)
	}
	if tok, pl := s.Get(svcA); tok != "tok" || pl != InSession {
		t.Errorf("Get после тайм-аута: %q, %s", tok, pl)
	}
	// Пауза: ОС не спрашиваем, ответ мгновенный.
	callsBefore := f.n()
	start = time.Now()
	if tok, pl := s.Get(svcB); tok != "" || pl != InSession {
		t.Errorf("Get в паузе: %q, %s", tok, pl)
	}
	if pl, _ := s.Set(svcB, "tokB"); pl != InSession {
		t.Errorf("Set в паузе: %s", pl)
	}
	if d := time.Since(start); d > 20*time.Millisecond {
		t.Errorf("в паузе ждали %v", d)
	}
	if f.n() != callsBefore {
		t.Errorf("в паузе дошло до ОС: %d вызовов", f.n()-callsBefore)
	}
	// Пауза кончилась — ОС снова спрашиваем.
	mu.Lock()
	clock = clock.Add(coolDown + time.Second)
	mu.Unlock()
	if s.isDown() {
		t.Error("пауза не кончилась")
	}
	// Delete в паузе всё равно идёт в ОС (токен надо убрать) и отвечает ошибкой при зависании.
	if err := s.Delete(svcA); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Delete при зависании: %v", err)
	}
	if tok, _ := s.Get(svcA); tok != "" {
		t.Errorf("токен остался в памяти после Delete: %q", tok)
	}
}

func TestSecretTesting(t *testing.T) {
	t.Setenv("SA_KEYRING_TEST", "")
	if _, ok := System(false).(*Memory); !ok {
		t.Errorf("в тесте System(false) = %T, ждали память", System(false))
	}
	if _, ok := System(true).(*Memory); !ok {
		t.Errorf("System(true) = %T, ждали память", System(true))
	}
	// Настоящее хранилище в тесте — только по явному флагу.
	t.Setenv("SA_KEYRING_TEST", "1")
	if _, ok := System(false).(*system); !ok {
		t.Errorf("с SA_KEYRING_TEST=1 System(false) = %T", System(false))
	}
	if _, ok := System(true).(*Memory); !ok {
		t.Errorf("memOnly главнее флага: %T", System(true))
	}
}

// Токен не попадает ни в ошибки, ни в журнал, даже когда ОС отказывает с текстом, где он есть;
// отказ пишется в журнал один раз за процесс.
func TestSecretNotLogged(t *testing.T) {
	logs := captureLog(t)
	const tok = "SECRETTOKEN123"
	f := newFake()
	f.fail = errors.New("dbus: отказ для " + tok)
	s := newSystem(f, 20*time.Millisecond, time.Now)
	_, _ = s.Set(svcA, tok)
	_, _ = s.Get(svcA) // в памяти: до ОС не доходит
	err := s.Delete(svcA)
	_, err1 := s.Set(svcA, tok+" ")
	_, err2 := s.Set("не адрес", tok)
	for _, e := range []error{err, err1, err2} {
		if e != nil && strings.Contains(e.Error(), tok) {
			t.Errorf("ошибка: %v", e)
		}
	}
	// Delete: токен из памяти вырезается из текста отказа ОС и там (Get токена не знает — нечего вырезать).
	f2 := newFake()
	s2 := newSystem(f2, time.Second, time.Now)
	s2.mem.put(svcA, tok)
	f2.failDel = errors.New("del: " + tok)
	f2.m[svcA] = tok
	if err := s2.Delete(svcA); !errors.Is(err, ErrKept) || strings.Contains(err.Error(), tok) {
		t.Errorf("Delete: %v", err)
	}
	// Зависание: тоже без токена в журнале.
	f.fail, f.block = nil, make(chan struct{})
	defer close(f.block)
	s3 := newSystem(f, 20*time.Millisecond, time.Now)
	_, _ = s3.Set(svcB, tok)
	if strings.Contains(logs.String(), tok) {
		t.Errorf("токен в журнале: %s", logs.String())
	}
	if n := strings.Count(logs.String(), "хранилище ключей ОС"); n != 3 { // по разу на Store: s, s2, s3
		t.Errorf("записей об отказе: %d\n%s", n, logs.String())
	}
}

// Отказ ОС попадает в журнал один раз, сколько бы вызовов ни было.
func TestSecretLogOnce(t *testing.T) {
	logs := captureLog(t)
	f := newFake()
	f.fail = errors.New("нет демона ключей")
	s := newSystem(f, time.Second, time.Now)
	for i := range 5 {
		_, _ = s.Set(fmt.Sprintf("https://h%d.example", i), "tok")
		s.Get("https://other.example")
		_ = s.Delete(svcA)
	}
	if n := strings.Count(logs.String(), "хранилище ключей ОС"); n != 1 {
		t.Errorf("записей об отказе: %d\n%s", n, logs.String())
	}
}

// Настоящее хранилище ОС: круг Set/Get/Delete и отображение «нет токена» (NotFound) на каждой
// ОС. Только по SA_KEYRING_TEST=1 (машина владельца, Windows CI): в обычном прогоне не трогает связку.
func TestSystemReal(t *testing.T) {
	if os.Getenv("SA_KEYRING_TEST") != "1" {
		t.Skip("н/п: настоящее хранилище ключей — только по SA_KEYRING_TEST=1")
	}
	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		t.Fatal(err)
	}
	origin := "https://sa-test-" + hex.EncodeToString(rnd[:]) + ".invalid"
	token := "tok" + hex.EncodeToString(rnd[:])
	st := System(false)
	if _, ok := st.(*system); !ok {
		t.Fatalf("System(false) = %T", st)
	}
	t.Cleanup(func() { _ = st.Delete(origin) })

	if tok, pl := st.Get(origin); tok != "" || pl != InSystem {
		t.Fatalf("до записи: %q, %s", tok, pl)
	}
	if pl, err := st.Set(origin, token); err != nil || pl != InSystem {
		t.Fatalf("Set: %s, %v (хранилище ОС недоступно?)", pl, err)
	}
	if tok, pl := st.Get(origin); tok != token || pl != InSystem {
		t.Fatalf("Get: %q, %s", tok, pl)
	}
	if err := st.Delete(origin); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if tok, pl := st.Get(origin); tok != "" || pl != InSystem {
		t.Errorf("после Delete: %q, %s", tok, pl)
	}
	if err := st.Delete(origin); err != nil { // повторно: NotFound — не ошибка
		t.Errorf("Delete без токена: %v", err)
	}
}

// Параллельные вызовы не портят карту (go test -race).
func TestSecretParallel(t *testing.T) {
	for _, s := range []Store{NewMemory(), newSystem(newFake(), time.Second, time.Now)} {
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				o := "https://h" + string(rune('a'+i%5)) + ".example"
				_, _ = s.Set(o, "tok")
				s.Get(o)
				_ = s.Delete(o)
			})
		}
		wg.Wait()
	}
}

// FuzzOrigin: без паники; результат стабилен (Origin от Origin — он же), без лишних знаков.
func FuzzOrigin(f *testing.F) {
	for _, s := range []string{"", svcA, "http://[::1]:8080", "HTTPS://A.b:443/", "https://u@h", "https://h/p?q#f", "\x00", "https://[::1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		o, err := Origin(raw)
		if err != nil {
			return
		}
		again, err := Origin(o)
		if err != nil || again != o {
			t.Fatalf("Origin(%q) = %q, повтор: %q, %v", raw, o, again, err)
		}
		if strings.ContainsAny(o, " \t\r\n?#@\\") || strings.Count(o, "/") != 2 {
			t.Fatalf("Origin(%q) = %q: лишние знаки", raw, o)
		}
	})
}
