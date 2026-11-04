package appkit

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
)

// logTo перенаправляет log в буфер на время теста.
func logTo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &b
}

func TestGuardSwallowsPanic(t *testing.T) {
	b := logTo(t)
	func() {
		defer Guard("метод")
		panic("бум")
	}()
	if got := b.String(); !strings.Contains(got, "ПАНИКА в метод: бум") || !strings.Contains(got, "goroutine") {
		t.Fatalf("запись о панике без имени, причины или стека: %q", got)
	}
	b.Reset()
	func() { defer Guard("тихо") }() // без паники — тишина
	if b.Len() != 0 {
		t.Fatalf("Guard без паники пишет в журнал: %q", b.String())
	}
}

func TestGuardErrGivesErrInternal(t *testing.T) {
	b := logTo(t)
	run := func(panics bool) (err error) {
		defer GuardErr("метод", &err)
		if panics {
			panic(errors.New("бум"))
		}
		return nil
	}
	if err := run(true); !errors.Is(err, ErrInternal) {
		t.Fatalf("после паники ждали ErrInternal, получили %v", err)
	}
	if !strings.Contains(b.String(), "ПАНИКА в метод") {
		t.Fatalf("паника не записана: %q", b.String())
	}
	if err := run(false); err != nil {
		t.Fatalf("без паники ошибка %v", err)
	}
}

func TestRepoURL(t *testing.T) {
	if !strings.HasPrefix(RepoURL, "https://github.com/") || strings.HasSuffix(RepoURL, "/") {
		t.Fatalf("RepoURL %q: нужен https-адрес без хвостового слэша (к нему дописывают /releases/…)", RepoURL)
	}
}
