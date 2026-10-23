package view_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обрез и разметка типографии в виде (066).

// TestBleedStrips — полосы обреза по внешним краям листа: с layflat и без одинаковы (у сгиба
// полос нет), у листа 031 — по краю листа; 0 — nil; шире половины листа не бывает. Обрез — один
// шаг отмены вместе с «Применить» настроек.
func TestBleedStrips(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	if v := e.View(); v.Bleed != nil {
		t.Fatalf("обрез 0: %+v", v.Bleed)
	}
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Layflat, st.Bleed = 281, 284, true, 2
	undo := edtest.UndoLen(e)
	v := e.SetSettings(st)
	want := []layout.Rect{{W: 562, H: 2}, {Y: 282, W: 562, H: 2}, {Y: 2, W: 2, H: 280}, {X: 560, Y: 2, W: 2, H: 280}}
	if !rectsEq(v.Bleed, want) || v.Settings.Bleed != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("layflat: %+v, обрез %v, шагов %d", v.Bleed, v.Settings.Bleed, edtest.UndoLen(e)-undo)
	}
	st.Layflat = false
	if v = e.SetSettings(st); !rectsEq(v.Bleed, want) || len(v.Safe) != 2 {
		t.Fatalf("без layflat — у сгиба полос нет: %+v", v.Bleed)
	}
	if v = e.Undo(); v.Settings.Layflat != true || !rectsEq(v.Bleed, want) {
		t.Fatal("отмена layflat")
	}
	if v = e.Undo(); v.Bleed != nil || v.Settings.Bleed != 0 {
		t.Fatalf("отмена обреза: %+v %v", v.Bleed, v.Settings.Bleed)
	}
	rev := edtest.Rev(e)
	if v = e.SetSettings(edtest.P(e).Settings); v.Bleed != nil || edtest.Rev(e) != rev {
		t.Fatal("холостое «Применить» без обреза")
	}
	sh := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	s := edtest.P(sh).Settings
	s.Bleed = 3
	want = []layout.Rect{{W: 204, H: 3}, {Y: 301, W: 204, H: 3}, {Y: 3, W: 3, H: 298}, {X: 201, Y: 3, W: 3, H: 298}}
	if v = sh.SetSettings(s); !rectsEq(v.Bleed, want) {
		t.Fatalf("лист 031: %+v", v.Bleed)
	}
}

// rectsEq — списки прямоугольников равны (nil и пустой — разные: UI проверяет null).
func rectsEq(a, b []layout.Rect) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMarkupViewBuild — разметки в виде: у разворотов — по листу Geo(nil) («у разворота»,
// у листа 031 — «у листа»), у обложки — по листу обложки («у обложки»); прочитанное о другом
// пути не годится (OK false); нет пути — nil; линии — мм своего листа.
func TestMarkupViewBuild(t *testing.T) {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	pr, _ := model.CoverPresetFor(model.CoverDefault, p.Settings)
	for range 20 {
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID}, Guide: testkit.AbsPath("Ш/Cover.psd")}
	p.SpreadGuide = testkit.AbsPath("Ш/spread_30x30.psd")
	if v := view.BuildView(p, view.State{}); v.SpreadMarkup == nil || v.SpreadMarkup.OK || v.SpreadMarkup.Name != "spread_30x30.psd" ||
		v.Cover.Markup == nil || v.Cover.Markup.OK || v.Cover.Markup.Name != "Cover.psd" {
		t.Fatalf("не прочитаны: %+v %+v", v.SpreadMarkup, v.Cover.Markup)
	}
	spread := library.MarkupInfo{W: 6638, H: 3354, DPI: 300, Image: true, Guides: []library.MarkupGuide{{Pos: 6638.0 / 2}}}
	cover := library.MarkupInfo{W: 4748, H: 3224, DPI: 300}
	st := view.State{
		SpreadGuide: view.Guide{Path: p.SpreadGuide, Info: spread, OK: true},
		CoverGuide:  view.Guide{Path: p.Cover.Guide, Info: cover, OK: true},
	}
	v := view.BuildView(p, st)
	sm, cm := v.SpreadMarkup, v.Cover.Markup
	if !sm.OK || !sm.Image || sm.Warn != "" || len(sm.Lines) != 1 || sm.Lines[0] != (model.GuideLine{V: true, At: 281}) {
		t.Fatalf("развороты: %+v", sm)
	}
	if !cm.OK || cm.Image || cm.Warn != "Размер разметки 402 × 273 мм — у обложки 626 × 326 мм" {
		t.Fatalf("обложка: %+v", cm)
	}
	st.SpreadGuide.Path = testkit.AbsPath("Ш/другой.psd")
	if v = view.BuildView(p, st); v.SpreadMarkup.OK || !v.Cover.Markup.OK {
		t.Fatalf("сведения о другом пути: %+v", v.SpreadMarkup)
	}
	p.SpreadGuide, p.Cover.Guide = "", ""
	if v = view.BuildView(p, st); v.SpreadMarkup != nil || v.Cover.Markup != nil {
		t.Fatal("разметки нет, а вид есть")
	}
	sh := album.NewVignetteSheet(204, 304)
	sh.SpreadGuide = testkit.AbsPath("Ш/list.psd")
	st = view.State{SpreadGuide: view.Guide{Path: sh.SpreadGuide, Info: cover, OK: true}}
	if v = view.BuildView(sh, st); v.SpreadMarkup.Warn != "Размер разметки 402 × 273 мм — у листа 204 × 304 мм" {
		t.Fatalf("лист 031: %+v", v.SpreadMarkup)
	}
}
