//go:build windows

package library

import (
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
	jpg, err := os.ReadFile(testkit.ExportJPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
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
		if len(skipped) != 1 || skipped[0].Why != whyCloudOnly {
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
