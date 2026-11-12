package osx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestClaimStart(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "cache", "mcp")
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	win := time.Minute
	claim := func(at time.Time) bool {
		t.Helper()
		ok, err := ClaimStart(dir, at, win)
		if err != nil {
			t.Fatalf("ClaimStart: %v", err)
		}
		return ok
	}
	if !claim(now) {
		t.Fatal("первый раз отказан (папки ещё нет)")
	}
	if claim(now) || claim(now.Add(win-time.Nanosecond)) {
		t.Error("повтор внутри окна принят")
	}
	if !claim(now.Add(win)) {
		t.Error("ровно по окончании окна отказан")
	}
	// Часы переведены назад: отметка «из будущего» не запирает запуск.
	if !claim(now.Add(-time.Hour)) {
		t.Error("отметка из будущего заперла запуск")
	}
	fi, err := os.Stat(filepath.Join(dir, "start.mark"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("права отметки %v", fi.Mode().Perm())
	}
}

func TestClaimStartBad(t *testing.T) {
	root := testkit.TempDir(t)
	now := time.Now()
	for name, dir := range map[string]string{"пусто": "", "относительный": filepath.Join("a", "b")} {
		if ok, err := ClaimStart(dir, now, time.Minute); ok || err == nil {
			t.Errorf("%s: ok=%v err=%v", name, ok, err)
		}
	}
	// Вместо папки — файл: ошибка записи, а не «окно ещё не вышло».
	file := filepath.Join(root, "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := ClaimStart(filepath.Join(file, "sub"), now, time.Minute); ok || err == nil {
		t.Errorf("папка под файлом: ok=%v err=%v", ok, err)
	}
	// Отметка — каталог: запись не удаётся, ошибка.
	dir := filepath.Join(root, "m")
	if err := os.MkdirAll(filepath.Join(dir, "start.mark"), 0o700); err != nil {
		t.Fatal(err)
	}
	if ok, err := ClaimStart(dir, now, time.Minute); ok || err == nil {
		t.Errorf("каталог на месте отметки: ok=%v err=%v", ok, err)
	}
	// Мусор, пустая, огромная, нуль, минус: читается как «давно», запуск разрешён.
	for name, body := range map[string]string{
		"мусор": "abc", "пустая": "", "огромная": strings.Repeat("7", 1<<20), "нуль": "0", "минус": "-1",
	} {
		d := filepath.Join(root, "b-"+name)
		if err := os.MkdirAll(d, 0o700); err != nil || os.WriteFile(filepath.Join(d, "start.mark"), []byte(body), 0o600) != nil {
			t.Fatal("не подготовили отметку")
		}
		if ok, err := ClaimStart(d, now, time.Minute); !ok || err != nil {
			t.Errorf("%s: ok=%v err=%v", name, ok, err)
		}
	}
}
