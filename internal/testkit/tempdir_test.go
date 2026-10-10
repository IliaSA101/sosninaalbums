package testkit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
