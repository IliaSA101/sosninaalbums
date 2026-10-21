package library

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// withLocal подменяет местный пояс на время теста: ключ не должен зависеть от пояса машины.
func withLocal(t *testing.T, loc *time.Location) {
	t.Helper()
	prev := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = prev })
}

// ShotKey: EXIF раньше времени файла; время файла — в местном поясе, теми же полями как UTC
// (одна шкала с EXIF); равные ключи; край — нет ни того, ни другого.
func TestShotKey(t *testing.T) {
	withLocal(t, time.FixedZone("UTC+3", 3*3600))
	wall := func(h, m, s int) int64 { return time.Date(2024, 6, 15, h, m, s, 0, time.UTC).Unix() }

	exif := wall(14, 30, 5)
	if got := ShotKey(exif, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); got != exif {
		t.Errorf("EXIF должен перекрыть время файла: %d, ждали %d", got, exif)
	}
	// Файл изменён в 11:30:05 UTC = 14:30:05 по местному: ключ совпадает со снятым в 14:30:05 по EXIF.
	mod := time.Date(2024, 6, 15, 11, 30, 5, 0, time.UTC)
	if got := ShotKey(0, mod); got != exif {
		t.Errorf("время файла в местном поясе: %d, ждали %d", got, exif)
	}
	if got := ShotKey(0, mod.In(time.FixedZone("UTC-7", -7*3600))); got != exif {
		t.Errorf("пояс самого времени файла не должен влиять: %d, ждали %d", got, exif)
	}
	if got := ShotKey(0, time.Time{}); got != 0 {
		t.Errorf("нет данных: %d, ждали 0", got)
	}
	if got := ShotKey(-5, time.Time{}); got != 0 {
		t.Errorf("отрицательный EXIF — не время: %d", got)
	}
	// Секунды без долей: два файла в одну секунду — равные ключи (порядок решит имя).
	if a, b := ShotKey(0, mod), ShotKey(0, mod.Add(900*time.Millisecond)); a != b {
		t.Errorf("доли секунды: %d и %d", a, b)
	}
}

// Shot доходит до Photo: несколько фото сразу (EXIF, без EXIF, мусор), из памяти сканера
// (повторный проход) и через ReadInfo; в JSON события — поле shot, а у Taken его по-прежнему нет.
func TestScanShot(t *testing.T) {
	withLocal(t, time.FixedZone("UTC+3", 3*3600))
	dir := testkit.TempDir(t)
	testkit.JPEGSegments(t, dir, "a.jpg", 40, 30, testkit.ExifTaken(false, "2024:06:15 14:30:05", ""))
	testkit.JPEGSegments(t, dir, "junk.jpg", 40, 30, testkit.ExifTaken(false, "0000:00:00 00:00:00", ""))
	testkit.JPEG(t, dir, "plain.jpg", 40, 30)
	testkit.JPEG(t, dir, "plain2.jpg", 40, 30)
	mod := time.Date(2024, 6, 15, 8, 0, 0, 0, time.UTC) // 11:00:00 по местному
	for _, n := range []string{"junk.jpg", "plain.jpg", "plain2.jpg"} {
		if err := os.Chtimes(filepath.Join(dir, n), mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	at := func(h, m, s int) int64 { return time.Date(2024, 6, 15, h, m, s, 0, time.UTC).Unix() }
	want := map[string]int64{"a.jpg": at(14, 30, 5), "junk.jpg": at(11, 0, 0), "plain.jpg": at(11, 0, 0), "plain2.jpg": at(11, 0, 0)}

	s := NewScanner()
	for pass := 1; pass <= 2; pass++ { // второй проход — из памяти сканера
		photos, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {})
		if err != nil || len(photos) != len(want) {
			t.Fatalf("проход %d: фото %d, ошибка %v", pass, len(photos), err)
		}
		for _, p := range photos {
			if p.Shot != want[p.Name] {
				t.Errorf("проход %d, %s: Shot %d, ждали %d", pass, p.Name, p.Shot, want[p.Name])
			}
		}
	}
	ph, err := ReadInfo(filepath.Join(dir, "plain.jpg"))
	if err != nil || ph.Shot != want["plain.jpg"] {
		t.Errorf("ReadInfo: %d, %v", ph.Shot, err)
	}

	// Событие library: shot есть, taken нет; нулевой shot не пишется.
	data, err := json.Marshal([]Photo{{ID: "x", Taken: 5, Shot: 7}, {ID: "y"}})
	if err != nil {
		t.Fatal(err)
	}
	if js := string(data); !strings.Contains(js, `"shot":7`) || strings.Contains(js, "taken") || strings.Count(js, "shot") != 1 {
		t.Errorf("JSON: %s", js)
	}
}
