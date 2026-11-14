package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// TestExportFitFromProfile (123 §5): у формата профиля есть px (7180×2397), страница альбома —
// 304 × 203 (файл 7181×2398): выгрузка «Для Фабрики» пишет разворот ровно по цели, без запрета.
func TestExportFitFromProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("рендер файлов 7180×2397 — не быстрый уровень")
	}
	base := printprof.Builtin().Profiles()
	fab := base[2]
	fab.Formats = slices.Clone(fab.Formats)
	fab.Formats[3].Px = &printprof.PxPair{7180, 2397}
	snap, err := printprof.NewSnapshot([]printprof.Profile{base[0], base[1], fab})
	if err != nil {
		t.Fatal(err)
	}
	printprof.UseForTest(t, snap)
	e, _ := printEnv(t, 1, 304, 203, "fabrika-photo")
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка: %q, вопросы %q", msg, e.asked)
	}
	if got := pxOf(t, filepath.Join(e.dir, "Альбом", "1-01.jpg")); got != [2]int{7180, 2397} {
		t.Errorf("разворот %v, ждали 7180×2397", got)
	}
}
