package testkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeTB перехватывает Errorf: проверить, что паника действия превращается в ошибку с именем и стеком.
type fakeTB struct {
	testing.TB
	msgs []string
}

func (f *fakeTB) Errorf(format string, args ...any) {
	f.msgs = append(f.msgs, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Name() string { return "TestFake/sub" }

// panicInHook — функция, имя которой должно быть в стеке отчёта.
func panicInHook() { panic("boom") }

// Паника в одном действии — ошибка с именем теста и стеком, остальные действия всё равно идут.
func TestHookPanicReported(t *testing.T) {
	f := &fakeTB{TB: t}
	var h hooks
	ran := 0
	h.fns = []func(){func() { ran++ }, panicInHook, func() { ran++ }, panicInHook}
	h.run(f)
	if ran != 2 || len(f.msgs) != 2 {
		t.Fatalf("выполнено %d, ошибок %d", ran, len(f.msgs))
	}
	for _, want := range []string{"TestFake/sub", "boom", "panicInHook"} {
		if !strings.Contains(f.msgs[0], want) {
			t.Errorf("в сообщении нет %q: %s", want, f.msgs[0])
		}
	}
	h.run(f) // повторный запуск ничего не делает
	if ran != 2 || len(f.msgs) != 2 {
		t.Error("повторный run выполнил действия")
	}
}

// BeforeRemove отрабатывает до удаления папки, даже если папка создана после регистрации.
func TestBeforeRemoveRunsBeforeDirRemoval(t *testing.T) {
	var dir string
	var existed bool
	t.Run("sub", func(t *testing.T) {
		BeforeRemove(t, func() {
			_, err := os.Stat(dir)
			existed = err == nil
		})
		dir = TempDir(t)
	})
	if !existed {
		t.Error("действие вызвано после удаления папки")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("папка не удалена: %v", err)
	}
}

// Без папок действия всё равно выполняются; порядок — обратный регистрации.
func TestBeforeRemoveOrderWithoutDirs(t *testing.T) {
	var got []int
	t.Run("sub", func(t *testing.T) {
		BeforeRemove(t, func() { got = append(got, 1) })
		BeforeRemove(t, func() { got = append(got, 2) })
	})
	if len(got) != 2 || got[0] != 2 || got[1] != 1 {
		t.Errorf("порядок: %v", got)
	}
}

// removeAll повторяет попытки, пока папку держат: файл, отпущенный через 50 мс, не валит очистку.
func TestRemoveAllRetriesWhileHeld(t *testing.T) {
	dir := TempDir(t)
	p := filepath.Join(dir, "f")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = f.Close()
	}()
	if err := removeAll(dir, 5*time.Second); err != nil {
		t.Errorf("папка не удалилась: %v", err)
	}
}
