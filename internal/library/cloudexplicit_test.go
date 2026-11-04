package library

import (
	"errors"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// asStub — подмена шва dehydrated: файлы с именем из names «лежат в облаке» (на любой ОС). Сканер берёт признак облака у imgfile напрямую: его проверяет Windows-тест.
func asStub(t *testing.T, names ...string) {
	t.Helper()
	old := dehydrated
	t.Cleanup(func() { dehydrated = old })
	dehydrated = func(st fs.FileInfo) bool {
		for _, n := range names {
			if st != nil && st.Name() == n {
				return true
			}
		}
		return old(st)
	}
}

// TestCloudExplicitBackdropMarkup (160-M1, доправка 4): фон и разметка, выбранные пользователем, читают заглушку
// как явное действие (cloudOK), а тихие пути (ReadInfo, MarkupFile вида и /media) отказывают ErrCloudOnly.
// Блоки независимы.
func TestCloudExplicitBackdropMarkup(t *testing.T) {
	dir := testkit.TempDir(t)
	jpg := testkit.JPEG(t, dir, "stub.jpg", 64, 48)
	png := testkit.PNG(t, dir, "stub.png", 8, 6, color.NRGBA{R: 200, A: 255})
	asStub(t, "stub.jpg", "stub.png")

	t.Run("фон читается", func(t *testing.T) {
		ph, err := BackdropFile(jpg)
		if err != nil || ph.W != 64 || ph.H != 48 {
			t.Fatalf("BackdropFile: %+v, %v", ph, err)
		}
	})
	t.Run("разметка JPEG читается явно", func(t *testing.T) {
		m, err := MarkupFileExplicit(jpg)
		if err != nil || m.W != 64 || !m.Image {
			t.Fatalf("MarkupFileExplicit: %+v, %v", m, err)
		}
	})
	t.Run("разметка PNG читается явно", func(t *testing.T) {
		if m, err := MarkupFileExplicit(png); err != nil || m.W != 8 {
			t.Fatalf("MarkupFileExplicit: %+v, %v", m, err)
		}
	})
	t.Run("тихая разметка отказывает", func(t *testing.T) {
		for _, p := range []string{jpg, png} {
			if _, err := MarkupFile(p); !errors.Is(err, ErrCloudOnly) {
				t.Errorf("MarkupFile(%s): %v, ожидалась ErrCloudOnly", filepath.Base(p), err)
			}
		}
	})
	t.Run("тихая разметка: мусор вместо JPEG — всё равно ErrCloudOnly", func(t *testing.T) {
		// Заглушка с байтами, которые не разберёт ни один формат: отказ облака обязан прийти раньше FileDims
		// (иначе тихий путь открыл бы файл и скачал его, а пользователь увидел бы ошибку разбора).
		for _, name := range []string{"junk.jpg", "junk.png"} {
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte("не картинка, а просто байты"), 0o600); err != nil {
				t.Fatal(err)
			}
			asStub(t, name)
			if _, err := MarkupFile(p); !errors.Is(err, ErrCloudOnly) {
				t.Errorf("MarkupFile(%s): %v, ожидалась ErrCloudOnly, а не ошибка разбора", name, err)
			}
		}
	})
	t.Run("тихая разметка PSD отказывает", func(t *testing.T) {
		psdPath := filepath.Join(dir, "stub.psd")
		if err := os.WriteFile(psdPath, []byte("8BPS"), 0o600); err != nil {
			t.Fatal(err)
		}
		asStub(t, "stub.psd")
		if _, err := MarkupFile(psdPath); !errors.Is(err, ErrCloudOnly) {
			t.Errorf("MarkupFile(psd): %v, ожидалась ErrCloudOnly", err)
		}
	})
	t.Run("ReadInfo отказывает", func(t *testing.T) {
		if _, err := ReadInfo(jpg); !errors.Is(err, ErrCloudOnly) {
			t.Errorf("ReadInfo: %v, ожидалась ErrCloudOnly", err)
		}
	})
}
