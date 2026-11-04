//go:build linux

package library

import (
	"crypto/sha1" //nolint:gosec // ключ старого вида в тесте, не криптография
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Linux: одно имя файла в `Весна` и `ВЕСНА` — разные ID; в одной папке `a.jpg` и `A.JPG` — тоже.
// ID не зависит от того, как записан путь (Clean). Under различает регистр папки.
func TestIDForCaseLinux(t *testing.T) {
	a, b := "/x/Весна/DSC_1.JPG", "/x/ВЕСНА/DSC_1.JPG"
	if IDFor(a) == IDFor(b) {
		t.Fatal("одно имя в двух папках разного регистра — один ID")
	}
	if IDFor("/x/Весна/a.jpg") == IDFor("/x/Весна/A.JPG") {
		t.Fatal("a.jpg и A.JPG в одной папке — один ID")
	}
	if IDFor("/x/Весна/./DSC_1.JPG") != IDFor(a) {
		t.Fatal("Clean пути меняет ID")
	}
	sum := sha1.Sum([]byte(strings.ToLower(b))) //nolint:gosec // как IDFor 0.22
	if old := hex.EncodeToString(sum[:8]); IDFor(b) == old {
		t.Fatal("ID на Linux по-прежнему от нижнего регистра")
	}
	if Under(b, "/x/Весна") || !Under(b, "/x/ВЕСНА") {
		t.Fatal("Under свёл регистр папки")
	}
}

// Сценарий 101 п. 4: две папки, различающиеся регистром, с одинаковыми именами файлов — в ленте
// фото обеих (раньше второе выпадало как повтор), ID разные; сайдкар каждой папки — только своему
// фото; порядок — стабильный (сначала точный путь меньше).
func TestScanCaseLinux(t *testing.T) {
	root := testkit.TempDir(t)
	lo, up := filepath.Join(root, "Весна"), filepath.Join(root, "ВЕСНА")
	for _, d := range []string{lo, up} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "весна")); err == nil {
		t.Skip("ФС без учёта регистра")
	}
	pa := testkit.JPEG(t, lo, "IMG_1.JPG", 40, 30)
	pb := testkit.JPEG(t, up, "IMG_1.JPG", 40, 30)
	writeSide(t, up, "IMG_1.xmp", `xmp:Rating="4"`)
	for range 2 { // второй раз — из памяти заголовков
		photos := scanSrc(t, NewScanner(), Sources{Folders: []string{lo, up}})
		if len(photos) != 2 {
			t.Fatalf("фото: %d, ждали 2", len(photos))
		}
		got := []string{photos[0].Path, photos[1].Path}
		want := []string{pa, pb}
		slices.Sort(want)
		if !slices.Equal(got, want) || photos[0].ID == photos[1].ID {
			t.Fatalf("лента: %v, ждали %v; ID %s %s", got, want, photos[0].ID, photos[1].ID)
		}
		for _, ph := range photos {
			if ph.ID != IDFor(ph.Path) {
				t.Errorf("ID %s не IDFor(%s)", ph.ID, ph.Path)
			}
			rated := ph.Xmp != nil && ph.Xmp.Rating == 4
			if rated != (ph.Path == pb) {
				t.Errorf("сайдкар из ВЕСНА у %s: %v", ph.Path, rated)
			}
		}
	}
}
