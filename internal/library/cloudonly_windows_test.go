//go:build windows

package library

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestScanCloudStubNotDownloaded (160-M1, ВАЖНО 2): сканирование папки с заглушкой OneDrive не читает заголовок —
// поставщик не вызывается (иначе лента скачала бы всю папку), фото в «не прочитано» с причиной про облако, а обычное
// фото рядом читается. Блоки независимы.
func TestScanCloudStubNotDownloaded(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	// Обычный JPEG: ExportJPEG несёт метку нашей выгрузки, такое фото сканер уводит из ленты (stOurs).
	jpg, err := os.ReadFile(testkit.JPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}
	stub := root.Stub(t, "stub.jpg", jpg)
	if err := os.WriteFile(filepath.Join(filepath.Dir(stub), "plain.jpg"), jpg, 0o600); err != nil {
		t.Fatal(err)
	}
	photos, skipped := Scan([]string{filepath.Dir(stub)})
	t.Run("скачиваний нет", func(t *testing.T) {
		if n := root.Fetches(); n != 0 {
			t.Errorf("сканирование скачало заглушку: колбэк вызван %d раз", n)
		}
	})
	t.Run("заглушка в пропущенных", func(t *testing.T) {
		if len(skipped) != 1 || skipped[0].Why != WhyCloudOnly {
			t.Errorf("пропущено %+v, ожидалась заглушка с причиной про облако", skipped)
		}
	})
	t.Run("обычное фото читается", func(t *testing.T) {
		if len(photos) != 1 || photos[0].W != 64 {
			t.Errorf("фото %+v", photos)
		}
	})
}

// TestRegularEntryCloudSidecar: запись-заглушка .xmp в листинге — файл (сайдкар на месте), но не читается.
func TestRegularEntryCloudSidecar(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	p := root.Stub(t, "a.xmp", []byte("<x/>"))
	list, err := os.ReadDir(filepath.Dir(p))
	if err != nil || len(list) != 1 {
		t.Fatalf("листинг: %v %v", list, err)
	}
	if !regularEntry(p, list[0]) {
		t.Error("сайдкар-заглушка не признан файлом")
	}
	var ss = sideSet{m: map[string]sidecar{}}
	ss.add(p, list[0])
	for _, s := range ss.m {
		if !s.cloud {
			t.Error("сайдкар-заглушка без признака облака: будет скачан при чтении")
		}
	}
	if n := root.Fetches(); n != 0 {
		t.Errorf("листинг скачал сайдкар: %d", n)
	}
}

// TestScanLooseCloudStubDownloads (аудит 160-M1 №5): отдельное фото (097), выбранное или брошенное в окно, — явное
// действие над файлом: заглушка читается (скачивается) и попадает в ленту, а не в «не прочитано». Папка с такой же
// заглушкой по-прежнему её не трогает (TestScanCloudStubNotDownloaded).
func TestScanLooseCloudStubDownloads(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	jpg, err := os.ReadFile(testkit.ExportJPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}
	stub := root.Stub(t, "stub.jpg", jpg)
	photos, skipped, err := NewScanner().Scan(context.Background(), Sources{Files: []string{stub}},
		func([]Photo, ScanProgress, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	t.Run("в ленте", func(t *testing.T) {
		if len(photos) != 1 || photos[0].W != 64 || len(skipped) != 0 {
			t.Errorf("фото %+v, пропущено %+v", photos, skipped)
		}
	})
	t.Run("скачано", func(t *testing.T) {
		if root.Fetches() == 0 {
			t.Error("поставщик не вызывался")
		}
	})
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}

// TestCloudStubExplicitVsQuiet (160-M1, доправка 4): фон и разметка, выбранные пользователем, читают заглушку со
// скачиванием (Fetches > 0), как отдельное фото; тихая разметка (вид, /media) и фоновое сканирование той же папки
// не скачивают ничего (Fetches == 0). Блоки независимы.
func TestCloudStubExplicitVsQuiet(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	jpg, err := os.ReadFile(testkit.JPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}
	bg := root.Stub(t, "bg.jpg", jpg)
	mk := root.Stub(t, "mark.jpg", jpg)
	dir := filepath.Dir(bg)

	t.Run("тихие пути не скачивают", func(t *testing.T) {
		Scan([]string{dir})
		if _, err := MarkupFile(mk); !errors.Is(err, ErrCloudOnly) {
			t.Errorf("MarkupFile: %v, ожидалась ErrCloudOnly", err)
		}
		if _, err := ReadInfoLocal(mk); !errors.Is(err, ErrCloudOnly) {
			t.Errorf("ReadInfoLocal: %v, ожидалась ErrCloudOnly", err)
		}
		if n := root.Fetches(); n != 0 {
			t.Errorf("скачиваний %d, ожидалось 0", n)
		}
	})
	t.Run("фон скачивает", func(t *testing.T) {
		before := root.Fetches()
		ph, err := BackdropFile(bg)
		if err != nil || ph.W != 64 || ph.H != 48 {
			t.Fatalf("BackdropFile: %+v, %v", ph, err)
		}
		if root.Fetches() <= before {
			t.Error("поставщик не вызывался")
		}
	})
	t.Run("разметка скачивает", func(t *testing.T) {
		before := root.Fetches()
		m, err := MarkupFileExplicit(mk)
		if err != nil || m.W != 64 || !m.Image {
			t.Fatalf("MarkupFileExplicit: %+v, %v", m, err)
		}
		if root.Fetches() <= before {
			t.Error("поставщик не вызывался")
		}
	})
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}
