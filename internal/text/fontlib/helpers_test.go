package fontlib

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// builtin — байты встроенного начертания: тесты берут их как «свой» файл.
func builtin(t testing.TB, file string) []byte {
	t.Helper()
	b, ok := text.FontFile(file)
	if !ok {
		t.Fatalf("нет встроенного %s", file)
	}
	return b
}

// fixture — файл из testdata пакета text (scripts/fonts-testdata.py).
func fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// withFsType — копия файла с другим OS/2.fsType (sfnt контрольные суммы не сверяет): другие
// байты, значит другой ключ, при том же имени и начертании.
func withFsType(t testing.TB, b []byte, v uint16) []byte {
	t.Helper()
	out := append([]byte(nil), b...)
	n := int(binary.BigEndian.Uint16(out[4:]))
	for i := range n {
		rec := out[12+16*i:]
		if string(rec[:4]) == "OS/2" {
			off := int(binary.BigEndian.Uint32(rec[8:]))
			binary.BigEndian.PutUint16(out[off+8:], v)
			return out
		}
	}
	t.Fatal("нет таблицы OS/2")
	return nil
}

// withName — копия файла без записей в таблице name (счётчик записей — 0): имени гарнитуры нет.
func withName(t testing.TB, b []byte) []byte {
	t.Helper()
	out := append([]byte(nil), b...)
	n := int(binary.BigEndian.Uint16(out[4:]))
	for i := range n {
		rec := out[12+16*i:]
		if string(rec[:4]) == "name" {
			binary.BigEndian.PutUint16(out[int(binary.BigEndian.Uint32(rec[8:]))+2:], 0)
			return out
		}
	}
	t.Fatal("нет таблицы name")
	return nil
}

// put пишет файл name в dir и возвращает путь.
func put(t testing.TB, dir, name string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// newStore — пустая библиотека в новой временной папке; src — папка для исходных файлов.
func newStore(t testing.TB) (s *Store, lib, src string) {
	t.Helper()
	root := testkit.TempDir(t)
	lib = filepath.Join(root, "Библиотека", "Шрифты")
	src = filepath.Join(root, "src")
	return Open(lib), lib, src
}

// famOf — гарнитура по имени из Families().
func famOf(t testing.TB, s *Store, name string) text.CustomFamily {
	t.Helper()
	for _, f := range s.Families() {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("нет гарнитуры %q в %+v", name, s.Families())
	return text.CustomFamily{}
}

// captureLog направляет журнал в buf; возвращает функцию возврата.
func captureLog(t testing.TB, buf *bytes.Buffer) (restore func()) {
	t.Helper()
	log.SetOutput(buf)
	return func() { log.SetOutput(io.Discard) }
}

// mustTime — момент sec секунд от сейчас (отрицательное — в прошлом).
func mustTime(t testing.TB, sec int) time.Time {
	t.Helper()
	return time.Now().Add(time.Duration(sec) * time.Second)
}

// TestMain глушит журнал: пропуски файлов пишут по строке, а тесты их не читают
// (кроме TestImportErrorsNoPath, у него свой приёмник).
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}
