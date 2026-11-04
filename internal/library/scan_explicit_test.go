package library

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// scanSkips — Scan без отчётов: лента и пропущенные.
func scanSkips(t *testing.T, s *Scanner, src Sources) (photos []Photo, skipped []Skip) {
	t.Helper()
	photos, skipped, err := s.Scan(context.Background(), src, func([]Photo, ScanProgress, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	return photos, skipped
}

// TestScanLooseCloudExplicit (160-M1 хвост 2): заглушку облака у отдельного фото (097) скачивает только сканирование,
// запущенное добавлением (Sources.Explicit); то же фото при любом другом чтении — «не читается» с причиной, без чтения (readScan отказывает до открытия),
// как файл папки. Блоки независимы; ОС не важна (шов dehydrated).
func TestScanLooseCloudExplicit(t *testing.T) {
	dir := testkit.TempDir(t)
	stubA := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	stubB := testkit.JPEG(t, dir, "b.jpg", 40, 30)
	plain := testkit.JPEG(t, dir, "plain.jpg", 40, 30)
	asStub(t, "a.jpg", "b.jpg")

	t.Run("без признака — заглушка не читается", func(t *testing.T) {
		photos, skipped := scanSkips(t, NewScanner(), Sources{Files: []string{stubA, stubB, plain}})
		if len(photos) != 1 || photos[0].Name != "plain.jpg" {
			t.Errorf("в ленте %+v, ожидалось только plain.jpg", photos)
		}
		if len(skipped) != 2 {
			t.Fatalf("пропущено %+v", skipped)
		}
		for _, sk := range skipped {
			if sk.Why != WhyCloudOnly {
				t.Errorf("%s: причина %q", filepath.Base(sk.Path), sk.Why)
			}
		}
	})
	t.Run("свежее читается, остальные нет", func(t *testing.T) {
		photos, skipped := scanSkips(t, NewScanner(), Sources{Files: []string{stubA, stubB, plain}, Explicit: []string{stubA}})
		names := map[string]bool{}
		for _, p := range photos {
			names[p.Name] = true
		}
		if !names["a.jpg"] || !names["plain.jpg"] || names["b.jpg"] || len(skipped) != 1 || filepath.Base(skipped[0].Path) != "b.jpg" {
			t.Errorf("лента %v, пропущено %+v", names, skipped)
		}
	})
	t.Run("после чтения явным запуском обычный снова отказывает", func(t *testing.T) {
		// Память заголовков не должна пускать заглушку в ленту при следующем, неявном чтении.
		s := NewScanner()
		if photos, _ := scanSkips(t, s, Sources{Files: []string{stubA}, Explicit: []string{stubA}}); len(photos) != 1 {
			t.Fatalf("явное чтение: %+v", photos)
		}
		photos, skipped := scanSkips(t, s, Sources{Files: []string{stubA}})
		if len(photos) != 0 || len(skipped) != 1 || skipped[0].Why != WhyCloudOnly {
			t.Errorf("лента %+v, пропущено %+v", photos, skipped)
		}
	})
	t.Run("признак на файл не из списка и на обычный файл", func(t *testing.T) {
		photos, skipped := scanSkips(t, NewScanner(), Sources{Files: []string{plain}, Explicit: []string{stubB, plain}})
		if len(photos) != 1 || len(skipped) != 0 {
			t.Errorf("лента %+v, пропущено %+v", photos, skipped)
		}
	})
	t.Run("пустые источники", func(t *testing.T) {
		photos, skipped := scanSkips(t, NewScanner(), Sources{Explicit: []string{stubA}})
		if len(photos) != 0 || len(skipped) != 0 {
			t.Errorf("лента %+v, пропущено %+v", photos, skipped)
		}
	})
}
