//go:build windows

package testkit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// skipTB перехватывает Skipf: записывает текст и завершает горутину, как настоящий t.Skipf.
type skipTB struct {
	testing.TB
	msg string
}

func (s *skipTB) Skipf(format string, args ...any) {
	s.msg = fmt.Sprintf(format, args...)
	runtime.Goexit()
}

// runSkipping вызывает fn с skipTB и возвращает текст пропуска ("" — fn дошла до конца).
func runSkipping(t *testing.T, fn func(testing.TB)) string {
	t.Helper()
	s := &skipTB{TB: t}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn(s)
	}()
	<-done
	return s.msg
}

// pattern — детерминированные байты длиной n (разные в разных местах: сдвиг диапазона заметен).
func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

// readFileWithin читает файл целиком; зависшее скачивание — ошибка, а не вечное ожидание.
func readFileWithin(t *testing.T, path string) []byte {
	t.Helper()
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := os.ReadFile(path)
		ch <- res{b, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Errorf("чтение заглушки %s: %v", filepath.Base(path), r.err)
		}
		return r.b
	case <-time.After(Limit(30 * time.Second)):
		t.Fatalf("чтение заглушки %s зависло: колбэк FETCH_DATA не ответил", filepath.Base(path))
		return nil
	}
}

// TestCloudRoot: заглушка не несёт данных до чтения, чтение вызывает колбэк и отдаёт ровно записанные байты (малый файл,
// файл не кратного 4 КБ размера в несколько кусков, чтение с середины); Stub нескольких файлов не мешают друг другу.
// Нет фильтра cldflt — пропуск (NewCloudRoot). Блоки независимы.
func TestCloudRoot(t *testing.T) {
	root := NewCloudRoot(t)
	small, big := pattern(100), pattern(300*1024+123)
	a := root.Stub(t, "a.bin", small)
	b := root.Stub(t, "b.bin", big)
	c := root.Stub(t, "c.bin", big)
	if filepath.Dir(a) != root.Dir {
		t.Fatalf("заглушка %s вне корня %s", a, root.Dir)
	}

	t.Run("заглушка без данных", func(t *testing.T) {
		st, err := os.Stat(b)
		if err != nil || st.Size() != int64(len(big)) {
			t.Fatalf("Stat: %v, %v", st, err)
		}
		if n := root.Fetches(); n != 0 {
			t.Errorf("создание и Stat скачали файл: колбэк вызван %d раз", n)
		}
	})
	t.Run("малый файл", func(t *testing.T) {
		before := root.Fetches()
		if got := readFileWithin(t, a); !bytes.Equal(got, small) {
			t.Errorf("прочитано %d байт, ожидалось %d", len(got), len(small))
		}
		if root.Fetches() == before {
			t.Error("колбэк FETCH_DATA не вызван")
		}
	})
	t.Run("большой файл", func(t *testing.T) {
		if got := readFileWithin(t, b); !bytes.Equal(got, big) {
			t.Errorf("прочитано %d байт, ожидалось %d", len(got), len(big))
		}
	})
	t.Run("чтение с середины", func(t *testing.T) {
		f, err := os.Open(c)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() //nolint:errcheck // тест: только чтение
		buf := make([]byte, 5000)
		if _, err := f.ReadAt(buf, 131073); err != nil {
			t.Fatalf("ReadAt: %v", err)
		}
		if !bytes.Equal(buf, big[131073:131073+5000]) {
			t.Error("середина файла прочитана неверно")
		}
	})
	t.Run("сбой поставщика", func(t *testing.T) {
		// Колбэк с неизвестным файлом и без корня: сбой попадает в Errors, а не в панику; чужой корень игнорируется.
		if got := cloudFetch(&cfCallbackInfo{}, &cfFetchParams{}); got != 0 {
			t.Errorf("колбэк без корня вернул %d", got)
		}
		own := &CloudRoot{files: map[string][]byte{}}
		own.fetch(&cfCallbackInfo{}, &cfFetchParams{RequiredLength: 10})
		own.fetch(&cfCallbackInfo{}, &cfFetchParams{RequiredFileOffset: -1, RequiredLength: 10})
		if errs := own.Errors(); len(errs) < 2 || own.Fetches() != 2 {
			t.Errorf("сбои %v, обращений %d: ожидались записи по каждому вызову", errs, own.Fetches())
		}
	})
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}

// TestLongPath: путь без 8.3 — существующая папка остаётся той же; несуществующий и пустой путь возвращаются как есть;
// wstr отказывает на нулевом байте паникой (имена теста его не содержат).
func TestLongPath(t *testing.T) {
	dir := TempDir(t)
	if got := longPath(dir); !strings.EqualFold(got, dir) && !strings.EqualFold(filepath.Base(got), filepath.Base(dir)) {
		t.Errorf("longPath(%q) = %q", dir, got)
	}
	if got := longPath(filepath.Join(dir, "нет", "такого")); got != filepath.Join(dir, "нет", "такого") {
		t.Errorf("несуществующий путь изменился: %q", got)
	}
	if got := longPath(""); got != "" {
		t.Errorf("пустой путь: %q", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("wstr с нулевым байтом не вызвал панику")
		}
	}()
	wstr("a\x00b")
}

// TestJunctionWindows: точка соединения открывает файлы целевой папки и не символьная ссылка; mklink не вышла (нет папки
// для ссылки) — пропуск с причиной, у Junction и у DirLink.
func TestJunctionWindows(t *testing.T) {
	dir := TempDir(t)
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Run("создаётся", func(t *testing.T) {
		link := Junction(t, target, filepath.Join(dir, "j"))
		if b, err := os.ReadFile(filepath.Join(link, "f.txt")); err != nil || string(b) != "x" {
			t.Errorf("файл через точку соединения: %q, %v", b, err)
		}
		if st, err := os.Lstat(link); err != nil || st.Mode()&os.ModeSymlink != 0 {
			t.Errorf("Lstat: %v, %v: точка соединения не должна быть символьной ссылкой", st, err)
		}
	})
	t.Run("Junction не вышла", func(t *testing.T) {
		bad := filepath.Join(dir, "нет", "j")
		if msg := runSkipping(t, func(tb testing.TB) { Junction(tb, target, bad) }); !strings.Contains(msg, "mklink") {
			t.Errorf("пропуск %q: ожидалась причина про mklink", msg)
		}
	})
	t.Run("DirLink не вышла", func(t *testing.T) {
		bad := filepath.Join(dir, "нет", "d")
		if msg := runSkipping(t, func(tb testing.TB) { DirLink(tb, target, bad) }); !strings.Contains(msg, "junction") {
			t.Errorf("пропуск %q: ожидалась причина про junction", msg)
		}
	})
}
