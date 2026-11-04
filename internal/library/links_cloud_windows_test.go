//go:build windows

package library

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// readAllWithin читает файл целиком; зависшее скачивание (поставщик не ответил) — ошибка, а не вечное ожидание.
func readAllWithin(t *testing.T, f *os.File) []byte {
	t.Helper()
	type res struct {
		b   []byte
		err error
	}
	ch := make(chan res, 1)
	go func() {
		b, err := io.ReadAll(f)
		ch <- res{b, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Errorf("чтение заглушки: %v", r.err)
		}
		return r.b
	case <-time.After(testkit.Limit(30 * time.Second)):
		t.Fatal("чтение заглушки зависло: колбэк FETCH_DATA не ответил")
		return nil
	}
}

// TestOpenLocalCloudStub (160-M1-1…4): фото OneDrive «только в сети» — облачная заглушка (Cloud Files API, эмуляция
// testkit.CloudRoot). OpenLocal открывает её и скачивает данные при чтении (так идут оригинал, миниатюры, экспорт);
// StatLocal её видит; сканирование папки находит фото и читает заголовок; точка соединения вместо файла по-прежнему
// отклоняется. Нет фильтра cldflt — пропуск.
func TestOpenLocalCloudStub(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	photo, err := os.ReadFile(testkit.ExportJPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("OpenLocal", func(t *testing.T) {
		p := root.Stub(t, "a.jpg", photo)
		st, err := StatLocal(p)
		if err != nil || st.Size() != int64(len(photo)) {
			t.Errorf("StatLocal заглушки: %v, %v", st, err)
		}
		before := root.Fetches()
		f, got, err := OpenLocal(p)
		if err != nil {
			t.Fatalf("заглушка отклонена: %v", err)
		}
		defer f.Close() //nolint:errcheck // тест: только чтение
		if got.Size() != int64(len(photo)) {
			t.Errorf("размер открытого %d, ожидался %d", got.Size(), len(photo))
		}
		if b := readAllWithin(t, f); !bytes.Equal(b, photo) {
			t.Errorf("прочитано %d байт, ожидалось %d", len(b), len(photo))
		}
		if root.Fetches() == before {
			t.Error("колбэк FETCH_DATA не вызван: данные не скачивались")
		}
	})

	t.Run("ReadInfoLocal", func(t *testing.T) {
		p := root.Stub(t, "b.jpg", photo)
		ph, err := ReadInfoLocal(p)
		if err != nil || ph.W != 64 || ph.H != 48 {
			t.Errorf("заголовок заглушки: %+v, %v", ph, err)
		}
	})

	t.Run("Scan папки", func(t *testing.T) {
		root.Stub(t, "c.jpg", photo)
		before := root.Fetches()
		photos, skipped := Scan([]string{root.Dir})
		if len(skipped) != 0 {
			t.Errorf("пропущено: %v", skipped)
		}
		names := map[string]bool{}
		for _, ph := range photos {
			names[ph.Name] = true
		}
		for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
			if !names[n] {
				t.Errorf("в ленте нет %s: %v", n, names)
			}
		}
		if root.Fetches() == before {
			t.Error("сканирование не читало заглушку")
		}
	})

	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}

// TestOpenLocalJunctionNotStub: точка соединения (mklink /J, не symlink) вместо файла отклоняется, как прежде (MaybeRegular
// пропускает ModeIrregular, но цель-папка отсекается по режиму).
func TestOpenLocalJunctionNotStub(t *testing.T) {
	dir := testkit.TempDir(t)
	target := testkit.TempDir(t)
	link := testkit.Junction(t, target, filepath.Join(dir, "j.jpg"))
	if f, _, err := OpenLocal(link); err == nil {
		_ = f.Close()
		t.Error("точка соединения открылась как файл")
	}
}
