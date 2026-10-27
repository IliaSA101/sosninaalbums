package app

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig портретов
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// portraitsDir ищет .testphotos/portraits вверх от папки теста (фото вне git, в основном дереве,
// в том числе из рабочих копий .claude/worktrees); "" — нет.
func portraitsDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, ".testphotos", "portraits")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// TestAppFitFacesPortraits — настоящий детектор на листе из 25 мест (10 детских портретов, копии
// под разными именами): все места подогнаны или помечены, лиц не потеряно, повтор — из кэша.
// Время листа пишется в лог (design §11: 25 мест ≈ 2 с на 4 ядрах).
func TestAppFitFacesPortraits(t *testing.T) {
	testkit.CostTest(t)
	src := portraitsDir()
	if src == "" {
		t.Skip("нет .testphotos/portraits (10 детских портретов вне git, docs/qa/stock-photos.md)")
	}
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, cache.Close)
	a.cache = cache
	files, err := filepath.Glob(filepath.Join(src, "portrait-*.jpg"))
	if err != nil || len(files) == 0 {
		t.Skipf("в %s нет портретов: %v", src, err)
	}
	dir := testkit.TempDir(t)
	infos := make([]model.PhotoInfo, 25)
	for i := range infos {
		data, err := os.ReadFile(files[i%len(files)])
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("ученик%02d.jpg", i))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		infos[i] = model.PhotoInfo{ID: library.IDFor(path), Path: path, W: cfg.Width, H: cfg.Height}
	}
	ed := mustEditor(t, album.NewVignetteSheet(204, 304), "")
	sid := ed.View().Spreads[0].ID
	ed.FillPlaces(infos, sid)
	a.ed.Store(ed)

	start := time.Now()
	res, err := a.FitFaces(sid, "", "")
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	r := res.Report
	t.Logf("лист 25 мест: %v; подогнано %d, не по норме %d (мелких %d, крупных %d, у края %d), несколько лиц %d",
		took, r.Fitted, len(r.Short), r.Small, r.Big, r.Edge, r.Many)
	if r.Missing != 0 || r.NoFace != 0 || r.Fitted+len(r.Short)+r.Many != 25 {
		t.Fatalf("отчёт %+v: на портретах лицо должно находиться везде", r)
	}
	if limit := testkit.Limit(15 * time.Second); took > limit {
		t.Fatalf("лист 25 мест: %v, порог %v", took, limit)
	}
	start = time.Now()
	if _, err := a.FitFaces(sid, "", ""); err != nil {
		t.Fatal(err)
	}
	if again, limit := time.Since(start), testkit.Limit(2*time.Second); again > limit {
		t.Fatalf("повтор из кэша: %v, порог %v", again, limit)
	}
}
