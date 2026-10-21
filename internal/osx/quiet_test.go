package osx

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"testing"
)

// fakeStand подменяет признак стенда, запуск процессов и журнал; возвращает счётчик запусков.
func fakeStand(t *testing.T, on bool) (started *int, logs *bytes.Buffer) {
	t.Helper()
	oldOn, oldStart, oldOut := standOn, startCmd, log.Writer()
	n := 0
	buf := &bytes.Buffer{}
	standOn = func() bool { return on }
	startCmd = func(*exec.Cmd) error { n++; return nil }
	log.SetOutput(buf)
	t.Cleanup(func() {
		standOn, startCmd = oldOn, oldStart
		log.SetOutput(oldOut)
	})
	return &n, buf
}

// Со стендом проводник не запускается, в журнал — действие без пути; без стенда запускается.
func TestRevealStand(t *testing.T) {
	for _, file := range []bool{false, true} { // папка и «выделить файл»
		started, logs := fakeStand(t, true)
		if err := Reveal(`D:\секрет\альбом.jpg`, file); err != nil || *started != 0 {
			t.Fatalf("стенд: err=%v, запусков %d; ждали 0", err, *started)
		}
		if !strings.Contains(logs.String(), "стенд: окно не открыто (проводник)") || strings.Contains(logs.String(), "секрет") {
			t.Errorf("журнал: %q", logs)
		}
	}
	started, logs := fakeStand(t, false)
	if err := Reveal(t.TempDir(), false); err != nil || *started != 1 || logs.Len() != 0 {
		t.Errorf("без стенда: err=%v, запусков %d, журнал %q; ждали 1 запуск и пустой журнал", err, *started, logs)
	}
	if err := OpenDir(t.TempDir()); err != nil || *started != 2 {
		t.Errorf("OpenDir без стенда: err=%v, запусков %d", err, *started)
	}
}

// OpenDir у стенда — тот же путь через Reveal: ничего не запускается.
func TestOpenDirStand(t *testing.T) {
	started, _ := fakeStand(t, true)
	if err := OpenDir(t.TempDir()); err != nil || *started != 0 {
		t.Errorf("стенд: err=%v, запусков %d; ждали 0", err, *started)
	}
}

// OpenURL: со стендом браузер не вызывается, адрес в журнал не попадает; без стенда вызывается.
func TestOpenURLStand(t *testing.T) {
	var got []string
	open := func(u string) { got = append(got, u) }
	_, logs := fakeStand(t, true)
	OpenURL("https://example.org/секрет", open)
	if len(got) != 0 || !strings.Contains(logs.String(), "(браузер)") || strings.Contains(logs.String(), "example") {
		t.Errorf("стенд: вызовов %v, журнал %q", got, logs)
	}
	fakeStand(t, false)
	OpenURL("https://example.org/a", open)
	if len(got) != 1 || got[0] != "https://example.org/a" {
		t.Errorf("без стенда: %v", got)
	}
}

// standActive: пустое имя и выпуск (enabled=false) — не стенд; любое непустое значение — стенд.
func TestStandActive(t *testing.T) {
	env := func(v string) func(string) string { return func(string) string { return v } }
	cases := []struct {
		on   bool
		val  string
		want bool
	}{{true, "", false}, {true, "b", true}, {true, "../x", true}, {false, "b", false}, {false, "", false}}
	for _, c := range cases {
		if got := standActive(c.on, env(c.val)); got != c.want {
			t.Errorf("enabled=%v SA_STAND=%q: %v, ждали %v", c.on, c.val, got, c.want)
		}
	}
}
