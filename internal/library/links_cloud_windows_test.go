//go:build windows

package library

import (
	"bytes"
	"errors"
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
	// Обычный JPEG: ExportJPEG несёт метку нашей выгрузки, такое фото сканер уводит из ленты (stOurs).
	photo, err := os.ReadFile(testkit.JPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
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
		// Второй рубеж (decisions 160-M1): тихие пути заголовок выгруженного файла не читают и не скачивают.
		before := root.Fetches()
		if ph, err := ReadInfoLocal(p); !errors.Is(err, ErrCloudOnly) {
			t.Errorf("заголовок заглушки: %+v, %v, ожидалась ErrCloudOnly", ph, err)
		}
		if n := root.Fetches(); n != before {
			t.Errorf("ReadInfoLocal скачал заглушку: колбэк вызван %d раз", n-before)
		}
	})

	t.Run("Scan папки", func(t *testing.T) {
		root.Stub(t, "c.jpg", photo)
		before := root.Fetches()
		photos, skipped := Scan([]string{root.Dir})
		// a.jpg скачан подтестом «OpenLocal» (данные на диске) — читается как обычный; b и c выгружены — в «не прочитано»
		// с причиной про облако, без скачивания (решение 160-M1).
		names := map[string]bool{}
		for _, ph := range photos {
			names[ph.Name] = true
		}
		if len(photos) != 1 || !names["a.jpg"] {
			t.Errorf("в ленте %v, ожидался только скачанный a.jpg", names)
		}
		why := map[string]string{}
		for _, sk := range skipped {
			why[filepath.Base(sk.Path)] = sk.Why
		}
		if len(skipped) != 2 || why["b.jpg"] != whyCloudOnly || why["c.jpg"] != whyCloudOnly {
			t.Errorf("пропущено %v, ожидались b.jpg и c.jpg с причиной про облако", skipped)
		}
		if n := root.Fetches(); n != before {
			t.Errorf("сканирование скачало заглушки: колбэк вызван %d раз", n-before)
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
