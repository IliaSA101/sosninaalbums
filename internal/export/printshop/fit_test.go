package printshop

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	albumcore "github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fitProfile ставит снимок, где у Фабрики 30×20 цель 7180×2397 px (123 §5), и отдаёт её профиль выгрузки.
func fitProfile(t *testing.T) Profile {
	t.Helper()
	base := printprof.Builtin().Profiles()
	fab := base[2]
	fab.Formats = slices.Clone(fab.Formats)
	fab.Formats[3].Px = &printprof.PxPair{7180, 2397}
	snap, err := printprof.NewSnapshot([]printprof.Profile{base[0], base[1], fab})
	if err != nil {
		t.Fatal(err)
	}
	printprof.UseForTest(t, snap)
	prof, ok := ProfileOf("fabrika")
	if !ok {
		t.Fatal("нет профиля fabrika")
	}
	return prof
}

// fitAlbum — разворот страницы w × h мм с одним фото.
func fitAlbum(t *testing.T, dir string, w, h float64) (*model.Project, model.View) {
	t.Helper()
	e, err := albumcore.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	s := model.DefaultSettings()
	s.PageW, s.PageH, s.DPI = w, h, 300
	e.SetSettings(s)
	ph := model.PhotoInfo{ID: "a", Path: printJPEG(t, dir, "a.jpg", 300, 200), W: 300, H: 200}
	if _, err := e.NewSpread(0, []model.PhotoInfo{ph}); err != nil {
		t.Fatal(err)
	}
	return e.Snapshot()
}

func fitCheck(t *testing.T, dir string, w, h float64, prof Profile) Plan {
	t.Helper()
	p, _ := fitAlbum(t, dir, w, h)
	return Check(p, "", prof)
}

// TestCheckFit: страница 304 × 203 даёт файл 7181 × 2398, а цель формата — 7180 × 2397: план
// подгоняет, «Файлы:» называет цель и подгонку; точная страница и формат без px — без подгонки.
func TestCheckFit(t *testing.T) {
	prof := fitProfile(t)
	dir := testkit.TempDir(t)
	pl := fitCheck(t, dir, 304, 203, prof)
	if pl.Format != "30×20" {
		t.Fatalf("формат %q, запреты %v", pl.Format, pl.Block)
	}
	if pl.FitW != 7180 || pl.FitH != 2397 || pl.Fit != 1 || pl.Px != "7180×2397 px" {
		t.Errorf("подгонка %d×%d на %d, Px %q", pl.FitW, pl.FitH, pl.Fit, pl.Px)
	}
	if !slices.Contains(pl.Memo, "Файлы: 7180×2397 px (подогнано на 1 px по обрезу)") {
		t.Errorf("памятка: %q", pl.Memo)
	}
	// Страница из px — файл точен: подгонки и пометки нет.
	pl = fitCheck(t, dir, 303.95, 202.95, prof)
	if pl.Format != "30×20" || pl.FitW != 0 || pl.Fit != 0 || pl.Px != "7180×2397 px" || strings.Contains(strings.Join(pl.Memo, "\n"), "подогнано") {
		t.Errorf("точная страница: формат %q, подгонка %d×%d, Px %q, памятка %q", pl.Format, pl.FitW, pl.FitH, pl.Px, pl.Memo)
	}
	// За допуском (на 8 px по ширине) — «не формат», подгонки нет.
	pl = fitCheck(t, dir, 304.3, 203, prof)
	if pl.Format != "" || pl.FitW != 0 || len(pl.Block) == 0 {
		t.Errorf("страница за допуском: формат %q, подгонка %d, запреты %v", pl.Format, pl.FitW, pl.Block)
	}
	// Встроенная Фабрика без px — прежнее: файл 7181 × 2398, подгонки нет.
	printprof.UseForTest(t, printprof.Builtin())
	pl = fitCheck(t, dir, 304, 203, Fabrika)
	if pl.Px != "7181×2398 px" || pl.FitW != 0 || strings.Contains(strings.Join(pl.Memo, "\n"), "подогнано") {
		t.Errorf("без px: Px %q, подгонка %d", pl.Px, pl.FitW)
	}
}

// TestFitExportSize: выгрузка по плану с подгонкой пишет файл ровно цели с плотностью 300, без
// подгонки (встроенная Фабрика) — 7181 × 2398.
func TestFitExportSize(t *testing.T) {
	if testing.Short() {
		t.Skip("рендер двух листов 300 dpi — не быстрый уровень")
	}
	for _, c := range []struct {
		name string
		fit  bool
		w, h int
	}{{"подгонка", true, 7180, 2397}, {"без px", false, 7181, 2398}} {
		prof := Fabrika
		if c.fit {
			prof = fitProfile(t)
		} else {
			printprof.UseForTest(t, printprof.Builtin())
		}
		dir := testkit.TempDir(t)
		p, v := fitAlbum(t, dir, 304, 203)
		pl := Check(p, "", prof)
		g := p.Geo(nil)
		opt := render.Options{PageW: g.W / 2, PageH: g.H, DPI: 300, Quality: 90, FitW: pl.FitW, FitH: pl.FitH}
		plan := export.NewPlan(nil, len(v.Spreads), false).WithPrefix(prof.Prefix)
		out := filepath.Join(dir, "out")
		if err := export.RunPlan(context.Background(), export.FromView(p, v), plan, opt, out, nil, func(int, int) {}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		f := printDescribe(t, filepath.Join(out, prof.Prefix+"01.jpg"), c.name)
		if f.W != c.w || f.H != c.h || f.DPI != 300 {
			t.Errorf("%s: файл %d×%d, %d dpi; ждали %d×%d, 300", c.name, f.W, f.H, f.DPI, c.w, c.h)
		}
	}
}
