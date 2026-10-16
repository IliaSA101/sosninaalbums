package library

import (
	"context"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Время съёмки из EXIF доходит до Photo: несколько фото сразу, с мусором и без EXIF; из памяти
// сканера (повторный скан) — то же самое.
func TestScanTaken(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEGSegments(t, dir, "a.jpg", 40, 30, testkit.ExifTaken(false, "2024:06:15 14:30:05", ""))
	testkit.JPEGSegments(t, dir, "b.jpg", 40, 30, testkit.ExifTaken(true, "", "2024:06:15 14:31:00"))
	testkit.JPEGSegments(t, dir, "junk.jpg", 40, 30, testkit.ExifTaken(false, "0000:00:00 00:00:00", ""))
	testkit.JPEGSegments(t, dir, "rot.jpg", 40, 30, testkit.ExifColor(true, 6, 0xFFFF, "R03"))
	testkit.JPEG(t, dir, "plain.jpg", 40, 30)
	at := func(h, m, s int) int64 { return time.Date(2024, 6, 15, h, m, s, 0, time.UTC).Unix() }
	want := map[string]int64{"a.jpg": at(14, 30, 5), "b.jpg": at(14, 31, 0), "junk.jpg": 0, "rot.jpg": 0, "plain.jpg": 0}

	s := NewScanner()
	for pass := 1; pass <= 2; pass++ { // второй проход — из памяти сканера
		photos, skipped, err := s.Scan(context.Background(), []string{dir}, func([]Photo, ScanProgress, bool) {})
		if err != nil || len(skipped) != 0 || len(photos) != len(want) {
			t.Fatalf("проход %d: фото %d, пропущено %v, ошибка %v", pass, len(photos), skipped, err)
		}
		for _, p := range photos {
			if p.Taken != want[p.Name] {
				t.Errorf("проход %d, %s: Taken %d, ждали %d", pass, p.Name, p.Taken, want[p.Name])
			}
		}
	}
	// ReadInfo (смена файла на диске) тоже отдаёт время
	ph, err := ReadInfo(dir + "/a.jpg")
	if err != nil || ph.Taken != want["a.jpg"] {
		t.Errorf("ReadInfo: %d, %v", ph.Taken, err)
	}
}
