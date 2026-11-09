package album_test

import (
	"encoding/json"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Разметка разворотов (066): правки через Editor, отмена, слоты прочитанного.

// spreadPSD — шаблон разворота 562 × 284 мм (6638 × 3354 px, 300 dpi) с вертикальной
// направляющей x (px).
func spreadPSD(t *testing.T, dir, name string, x float64) string {
	t.Helper()
	return testkit.PSD(t, dir, name, testkit.PSDSpec{W: 6638 / 10, H: 3354 / 10, DPI: 30,
		Guides: []testkit.PSDGuide{{Pos: x}}})
}

// TestSetSpreadGuideUndo — выбор — один шаг отмены, вид с линиями; тот же путь заново — без
// шага, но файл перечитывается; UNC и PSB — ошибка без правки; «Убрать» — шаг, повтор холостой;
// Ctrl+Z и Ctrl+Y по шагам; разметка и обрез переживают Save → Load вместе с разметкой обложки.
func TestSetSpreadGuideUndo(t *testing.T) {
	dir := testkit.TempDir(t)
	e := edtest.CoverEd(t, "cv", nil)
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Bleed = 281, 284, 2
	e.SetSettings(st)
	path := spreadPSD(t, dir, "spread_30x30.psd", 663.0/2)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, err := e.SetSpreadGuide(path)
	sm := v.SpreadMarkup
	if err != nil || edtest.P(e).SpreadGuide != path || edtest.UndoLen(e) != undo+1 || edtest.Rev(e) != rev+1 {
		t.Fatalf("выбор: %v, шагов %d", err, edtest.UndoLen(e)-undo)
	}
	if sm == nil || !sm.OK || !sm.Image || sm.Name != "spread_30x30.psd" || sm.Warn != "" || len(sm.Lines) != 1 || sm.Lines[0] != (model.GuideLine{V: true, At: 281}) {
		t.Fatalf("вид: %+v", sm)
	}
	// Файл заменили под тем же именем: без повторного выбора вид прежний (читается раз на путь),
	// выбор того же пути — холостая правка, но вид перечитан.
	spreadPSD(t, dir, "spread_30x30.psd", 663.0/4)
	if v = e.View(); v.SpreadMarkup.Lines[0].At != 281 {
		t.Fatalf("перечитан без выбора: %+v", v.SpreadMarkup.Lines)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if v, err = e.SetSpreadGuide(path); err != nil || v.SpreadMarkup.Lines[0].At != 140.5 {
		t.Fatalf("тот же путь: %v %+v", err, v.SpreadMarkup)
	}
	edtest.Same(t, e, rev, undo, "тот же путь разметки")
	for _, bad := range []string{`\\server\share\s.psd`, filepath.Join(dir, "big.psb"), filepath.Join(dir, "s.tif"), "s.psd"} {
		if _, err := e.SetSpreadGuide(bad); err == nil {
			t.Errorf("%q принят", bad)
		}
	}
	if _, err := e.SetSpreadGuide(filepath.Join(dir, "big.psb")); !errors.Is(err, library.ErrPSB) {
		t.Fatalf("PSB: %v", err)
	}
	edtest.Same(t, e, rev, undo, "отказ выбора")
	if v = e.ClearSpreadGuide(); v.SpreadMarkup != nil || edtest.P(e).SpreadGuide != "" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Убрать»: %+v", v.SpreadMarkup)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.ClearSpreadGuide()
	edtest.Same(t, e, rev, undo, "«Убрать» без разметки")
	if v = e.Undo(); edtest.P(e).SpreadGuide != path || v.SpreadMarkup == nil || !v.SpreadMarkup.OK {
		t.Fatalf("отмена «Убрать»: %+v", v.SpreadMarkup)
	}
	if v = e.Undo(); edtest.P(e).SpreadGuide != "" || v.SpreadMarkup != nil {
		t.Fatal("отмена выбора")
	}
	if v = e.Redo(); edtest.P(e).SpreadGuide != path || !v.SpreadMarkup.OK {
		t.Fatal("возврат выбора")
	}
	cov := testkit.PNG(t, dir, "cover.png", 63, 33, color.NRGBA{A: 255})
	if _, err := e.SetCoverGuide(cov); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "a.salbum")
	if err := e.Save(file); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(file, dir)
	if err != nil {
		t.Fatal(err)
	}
	if q.SpreadGuide != path || q.SpreadGuideAlt != "" || q.Settings.Bleed != 2 || q.Cover.Guide != cov {
		t.Fatalf("после Load: %q %q обрез %v, обложка %q", q.SpreadGuide, q.SpreadGuideAlt, q.Settings.Bleed, q.Cover.Guide)
	}
	if v := edtest.Open(t, q, file).View(); !v.SpreadMarkup.OK || !v.Cover.Markup.OK || len(v.Bleed) != 4 {
		t.Fatalf("вид после Load: %+v %+v %+v", v.SpreadMarkup, v.Cover.Markup, v.Bleed)
	}
}

// TestGuideSlotsIndependent — у обложки и разворотов свои слоты прочитанного: выбор, отмена и
// пропажа файла одной разметки не сбивают другую (иначе каждая смена перечитывала бы обе).
func TestGuideSlotsIndependent(t *testing.T) {
	dir := testkit.TempDir(t)
	e := edtest.CoverEd(t, "cv", nil)
	sp := spreadPSD(t, dir, "s.psd", 100)
	if _, err := e.SetSpreadGuide(sp); err != nil {
		t.Fatal(err)
	}
	c1 := testkit.PNG(t, dir, "c1.png", 60, 30, color.NRGBA{A: 255})
	c2 := testkit.PSD(t, dir, "c2.psd", testkit.PSDSpec{W: 60, H: 30, Guides: []testkit.PSDGuide{{Pos: 15, Horz: true}}})
	if _, err := e.SetCoverGuide(c1); err != nil {
		t.Fatal(err)
	}
	// Шаблон разворотов меняется на диске; пока его не выбирают заново, вид держит прочитанное —
	// и смена разметки обложки этого не меняет.
	spreadPSD(t, dir, "s.psd", 200)
	v, err := e.SetCoverGuide(c2)
	if err != nil || !v.Cover.Markup.OK || len(v.Cover.Markup.Lines) != 1 || v.Cover.Markup.Lines[0].V {
		t.Fatalf("обложка: %v %+v", err, v.Cover.Markup)
	}
	if !v.SpreadMarkup.OK || v.SpreadMarkup.Lines[0].At != round2(100.0/663*562) {
		t.Fatalf("развороты сбиты сменой обложки: %+v", v.SpreadMarkup)
	}
	if v = e.Undo(); v.Cover.Markup.Name != "c1.png" || v.SpreadMarkup.Lines[0].At != round2(100.0/663*562) {
		t.Fatalf("отмена обложки: %+v %+v", v.Cover.Markup, v.SpreadMarkup)
	}
	if v = e.ClearCoverGuide(); v.Cover.Markup != nil || v.SpreadMarkup == nil || v.SpreadMarkup.Lines[0].At != round2(100.0/663*562) {
		t.Fatalf("«Убрать» обложки: %+v", v.SpreadMarkup)
	}
	// Обратно: выбор разворотов заново перечитывает только их, обложка — как была.
	if _, err := e.SetCoverGuide(c2); err != nil {
		t.Fatal(err)
	}
	testkit.PSD(t, dir, "c2.psd", testkit.PSDSpec{W: 60, H: 30})
	if v, err = e.SetSpreadGuide(sp); err != nil || v.SpreadMarkup.Lines[0].At != round2(200.0/663*562) || len(v.Cover.Markup.Lines) != 1 {
		t.Fatalf("выбор разворотов: %v %+v %+v", err, v.SpreadMarkup, v.Cover.Markup)
	}
}

// TestSpreadGuideAltSurvivesUndo — запасной путь разметки разворотов (SpreadGuideAlt, json
// spreadGuideAlt,omitempty) едет в снимках отмены: правка → Undo → Redo → Undo его не теряют, и
// Save пишет его в spreadGuideAbs; без запасного (пустой) в файле abs — основной путь.
func TestSpreadGuideAltSurvivesUndo(t *testing.T) {
	dir := testkit.TempDir(t)
	e := edtest.CoverEd(t, "cv", nil)
	guide := spreadPSD(t, dir, "s.psd", 100)
	alt := filepath.Join(dir, "Другой диск", "old.psd")
	edtest.Change(e, func(p *model.Project) { p.SpreadGuide, p.SpreadGuideAlt = guide, alt })
	abs := func(name string) string {
		t.Helper()
		file := filepath.Join(dir, name)
		if err := e.Save(file); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var f struct {
			Guide string `json:"spreadGuide"`
			Abs   string `json:"spreadGuideAbs"`
		}
		if err := json.Unmarshal(raw, &f); err != nil || f.Guide == "" {
			t.Fatalf("файл: %v %q", err, raw)
		}
		return f.Abs
	}
	st := edtest.P(e).Settings
	st.Bleed = 3
	e.SetSettings(st) // посторонняя правка: в снимок попадает весь проект
	for _, step := range []func() model.View{e.Undo, e.Redo, e.Undo} {
		step()
		if got := edtest.P(e).SpreadGuideAlt; got != alt {
			t.Fatalf("после шага отмены запасной путь %q, ждали %q", got, alt)
		}
	}
	if got := abs("a.salbum"); got != alt {
		t.Fatalf("spreadGuideAbs после Undo → Save: %q, ждали %q", got, alt)
	}
	edtest.Change(e, func(p *model.Project) { p.SpreadGuideAlt = "" })
	if got := abs("b.salbum"); got != guide {
		t.Fatalf("без запасного abs = %q, ждали основной %q", got, guide)
	}
}

// round2 — до 0,01 мм, как линии вида.
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
