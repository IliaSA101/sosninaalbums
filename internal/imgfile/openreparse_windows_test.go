//go:build windows

package imgfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// symlinkOrSkip — ссылка на файл; без прав (машина разработчика без режима разработчика) тест пропускается, на CI
// (раннер — администратор) отсутствие ссылки — провал: эталон не должен терять тест молча.
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("на раннере ссылку создать нельзя: %v", err)
		}
		t.Skip("ссылку создать нельзя: " + err.Error())
	}
}

// TestOpenNoReparse (160 Т2, М1): обычный файл открывается и читается; ссылка в последнем звене не разворачивается —
// открытие отклоняется по дескриптору; папка, пустое имя и несуществующий файл — ошибка; длинный путь открывается.
func TestOpenNoReparse(t *testing.T) {
	dir := testkit.TempDir(t)
	file := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(file, []byte("данные"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenNoReparse(file)
	if err != nil {
		t.Fatalf("обычный файл: %v", err)
	}
	buf := make([]byte, 32)
	n, _ := f.Read(buf)
	_ = f.Close()
	if string(buf[:n]) != "данные" {
		t.Errorf("прочитано %q", buf[:n])
	}
	empty := filepath.Join(dir, "empty.bin")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if f, err := OpenNoReparse(empty); err != nil {
		t.Errorf("пустой файл: %v", err)
	} else {
		_ = f.Close()
	}
	for name, p := range map[string]string{"папка": dir, "нет файла": filepath.Join(dir, "нет.bin"), "пусто": "", "NUL": "NUL", "нулевой байт": file + "\x00"} {
		if f, err := OpenNoReparse(p); err == nil {
			_ = f.Close()
			t.Errorf("%s: открылось без ошибки", name)
		}
	}
	link := filepath.Join(dir, "link.bin")
	symlinkOrSkip(t, file, link)
	if f, err := OpenNoReparse(link); err == nil {
		_ = f.Close()
		t.Errorf("ссылка на файл: открылась (развёрнута)")
	}
	// Ссылка на другой том/шару не разворачивается тоже: цель не существует, ошибка та же и без обращения к ней.
	dangling := filepath.Join(dir, "dangling.bin")
	symlinkOrSkip(t, `\\127.0.0.1\нет\x.jpg`, dangling)
	if f, err := OpenNoReparse(dangling); err == nil {
		_ = f.Close()
		t.Errorf("ссылка на сетевую цель: открылась")
	}
}

// TestOpenNoReparseLongPath: путь длиннее MAX_PATH (CreateFile напрямую, без fixLongPath os) открывается с префиксом.
func TestOpenNoReparseLongPath(t *testing.T) {
	dir := testkit.TempDir(t)
	long := dir
	for len(long) < 300 {
		long = filepath.Join(long, strings.Repeat("д", 40))
	}
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatalf("длинный путь: %v", err)
	}
	file := filepath.Join(long, "x.bin")
	if err := os.WriteFile(file, []byte("1"), 0o644); err != nil {
		t.Fatalf("длинный путь: %v", err)
	}
	f, err := OpenNoReparse(file)
	if err != nil {
		t.Fatalf("длинный путь (%d): %v", len(file), err)
	}
	_ = f.Close()
}
