package grid_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// rectsOf — места разворота i.
func rectsOf(e *album.Editor, i int) []layout.Rect {
	var out []layout.Rect
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		out = append(out, rectOfPlace(pl))
	}
	return out
}

func wantRects(t *testing.T, got, want []layout.Rect) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("мест %d, ждали %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !nearMM(got[i], want[i]) {
			t.Fatalf("место %d: %+v, ждали %+v", i, got[i], want[i])
		}
	}
}

func TestMirrorGridPage(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(12, 142, 127, 130, "c"), edtest.Gp(293, 12, 127, 127, "")})
	before := slices.Clone(edtest.P(e).Spreads[0].Grid)
	ids := []string{before[0].ID, before[1].ID, before[2].ID}
	e.MirrorGrid("s1", model.ScopeLeft)
	edtest.MustGrid(t, e)
	wantRects(t, rectsOf(e, 0), []layout.Rect{{X: 142, Y: 12, W: 127, H: 127}, {X: 142, Y: 142, W: 127, H: 130}, {X: 293, Y: 12, W: 127, H: 127}})
	for i, pl := range edtest.P(e).Spreads[0].Grid {
		if pl.ID != ids[i] || pl.Photo != before[i].Photo || pl.Fit != before[i].Fit {
			t.Fatalf("место %d: %+v", i, *pl)
		}
	}
}

func TestMirrorGridSpread(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(200, 150, 150, 100, "b")})
	st := edtest.P(e).Settings
	st.Layflat = false
	e.SetSettings(st)
	v := e.MirrorGrid("s1", model.ScopeSpread)
	wantRects(t, rectsOf(e, 0), []layout.Rect{{X: 423, Y: 12, W: 127, H: 127}, {X: 212, Y: 150, W: 150, H: 100}})
	if !v.Spreads[0].Places[1].Spine || v.Spreads[0].Places[0].Spine {
		t.Fatalf("корешок: %+v", v.Spreads[0].Places)
	}
}

func TestMirrorGridSpineOnPage(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(200, 150, 150, 100, "b")})
	e.MirrorGrid("s1", model.ScopeLeft)
	wantRects(t, rectsOf(e, 0), []layout.Rect{{X: 169, Y: 12, W: 100, H: 100}, {X: 200, Y: 150, W: 150, H: 100}})
}

func TestMirrorGridSymmetricNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(142, 12, 127, 127, "b"), edtest.Gp(200, 150, 162, 100, "")})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.MirrorGrid("s1", model.ScopeLeft)
	e.MirrorGrid("s1", model.ScopeSpread+"x")
	e.MirrorGrid("s1", model.ScopeRight) // на правой странице мест целиком нет
	edtest.Same(t, e, rev, undo, "симметричная пара")
	// Симметричное «Авто»: по фото на страницу, одинаковые пропорции.
	a := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(a, -1, edtest.Photos("x", "y"))
	id := v.Spreads[0].ID
	if v = a.SetLayout(id, "P[0|1]"); v.Spreads[0].Layout != "P[0|1]" {
		t.Fatalf("ключ %q", v.Spreads[0].Layout)
	}
	rev, undo = edtest.Rev(a), edtest.UndoLen(a)
	for _, sc := range []string{model.ScopeSpread, model.ScopeLeft, model.ScopeRight} {
		a.MirrorGrid(id, sc)
	}
	edtest.Same(t, a, rev, undo, "симметричное «Авто»")
	if edtest.P(a).Spreads[0].Grid != nil {
		t.Fatal("«Авто» закрепилось")
	}
}

func TestMirrorGridFreezesAuto(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, []model.PhotoInfo{edtest.Photo("x"), {ID: "y", Path: "/y.jpg", W: 200, H: 300}})
	id := v.Spreads[0].ID
	v = e.SetLayout(id, "P[0|1]")
	before, key, undo := v.Spreads[0].Rects, edtest.P(e).Spreads[0].Layout, edtest.UndoLen(e)
	v = e.MirrorGrid(id, model.ScopeSpread)
	edtest.MustGrid(t, e)
	sv := v.Spreads[0]
	if !sv.Grid || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("сетка %v, шагов %d", sv.Grid, edtest.UndoLen(e)-undo)
	}
	w := 2 * edtest.P(e).Settings.PageW
	for i, r := range before {
		if m := (layout.Rect{X: w - r.X - r.W, Y: r.Y, W: r.W, H: r.H}); !nearMM(sv.Rects[i], m) {
			t.Fatalf("фото %d: %+v, ждали %+v", i, sv.Rects[i], m)
		}
	}
	v = e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil || edtest.P(e).Spreads[0].Layout != key || v.Spreads[0].Layout != "P[0|1]" {
		t.Fatalf("Undo: сетка %v, ключ %q", edtest.P(e).Spreads[0].Grid, edtest.P(e).Spreads[0].Layout)
	}
}

func TestMirrorGridTwice(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar())
	want := rectsOf(e, 0)
	for _, sc := range []string{model.ScopeLeft, model.ScopeSpread} {
		e.MirrorGrid("s1", sc)
		e.MirrorGrid("s1", sc)
		wantRects(t, rectsOf(e, 0), want)
	}
}

func TestMirrorGridKeepsFrameTexts(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(12, 142, 127, 130, "")})
	e.SetFrame("a", 1.5, -0.4, 0.2)
	if _, err := e.AddText("s1", 0, ""); err != nil {
		t.Fatal(err)
	}
	e.SetBackground("s1", "left", "#112233")
	before := e.View().Spreads[0]
	texts := marshalTexts(t, e)
	v := e.MirrorGrid("s1", model.ScopeLeft)
	after := v.Spreads[0]
	if f := edtest.P(e).Photos["a"].Frame; f == nil || *f != (model.Frame{S: 1.5, X: -0.4, Y: 0.2}) {
		t.Fatalf("кадр: %+v", f)
	}
	rel := func(sv model.SpreadView) layout.Rect {
		r, img := sv.Rects[0], sv.Frames[0].Img
		return layout.Rect{X: img.X - r.X, Y: img.Y - r.Y, W: img.W, H: img.H}
	}
	if after.Rects[0].X != 142 || !nearMM(rel(before), rel(after)) {
		t.Fatalf("картинка в месте: %+v → %+v", rel(before), rel(after))
	}
	if !bytes.Equal(texts, marshalTexts(t, e)) || edtest.P(e).Spreads[0].BgL != "#112233" {
		t.Fatal("тексты или фон изменились")
	}
}

func marshalTexts(t *testing.T, e *album.Editor) []byte {
	t.Helper()
	return []byte(fmt.Sprintf("%+v", *edtest.P(e).Spreads[0].Texts[0]))
}

func TestCopyGridPageToPage(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar()[:2])
	undo := edtest.UndoLen(e)
	_, left, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}, {Spread: "s1", Scope: model.ScopeLeft}}, false, false)
	edtest.MustGrid(t, e)
	if err != nil || left != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("err %v, left %d", err, left)
	}
	wantRects(t, rectsOf(e, 0), []layout.Rect{{X: 12, Y: 12, W: 127, H: 127}, {X: 12, Y: 142, W: 200, H: 130},
		{X: 293, Y: 12, W: 127, H: 127}, {X: 293, Y: 142, W: 200, H: 130}})
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "c", "", ""}) {
		t.Fatalf("фото %v", edtest.PhotosOf(e, 0))
	}
}

func TestCopyGridToList(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 127, 127, "b")}, []string{"d", "e", "f"}, []string{"g"})
	undo := edtest.UndoLen(e)
	_, left, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}, {Spread: "s3", Scope: model.ScopeSpread}, {Spread: "s9", Scope: model.ScopeSpread}}, false, false)
	edtest.MustGrid(t, e)
	if err != nil || left != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("err %v, left %d", err, left)
	}
	for i := 1; i <= 2; i++ {
		wantRects(t, rectsOf(e, i), rectsOf(e, 0))
	}
	if !slices.Equal(edtest.PhotosOf(e, 1), []string{"d", "e"}) || !slices.Equal(edtest.PhotosOf(e, 2), []string{"g", ""}) || edtest.HasRef(edtest.P(e), "f") {
		t.Fatalf("фото %v %v", edtest.PhotosOf(e, 1), edtest.PhotosOf(e, 2))
	}
	e.Undo()
	if edtest.P(e).Spreads[1].Grid != nil || len(edtest.P(e).Spreads[1].Photos) != 3 {
		t.Fatal("один Undo не вернул всё")
	}
	// Источник-разворот на страницу — ошибка; неизвестная область — ошибка.
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, false, false); !errors.Is(err, grid.ErrTplScope) {
		t.Fatalf("разворот → страница: %v", err)
	}
	if _, _, err := e.CopyGrid("s1", "top", []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, false, false); !errors.Is(err, model.ErrScope) {
		t.Fatalf("область: %v", err)
	}
}

func TestCopyGridFromAutoFreezesSource(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	v := edtest.NewSpread(e, -1, edtest.Photos("c"))
	src := v.Spreads[0]
	if _, _, err := e.CopyGrid(src.ID, model.ScopeSpread, []model.GridTarget{{Spread: v.Spreads[1].ID, Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	wantRects(t, rectsOf(e, 0), src.Rects)
	wantRects(t, rectsOf(e, 1), src.Rects)
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "b"}) || !slices.Equal(edtest.PhotosOf(e, 1), []string{"c", ""}) {
		t.Fatalf("фото %v %v", edtest.PhotosOf(e, 0), edtest.PhotosOf(e, 1))
	}
}

func TestCopyGridDesign(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar()[:2], []string{"d"})
	e.AddText("s1", 0, "")
	e.SetBackground("s1", "left", "#aabbcc")
	e.AddText("s2", 0, "")
	e.AddText("s2", 1, "")
	src := *edtest.P(e).Spreads[0].Texts[0]
	keep := edtest.P(e).Spreads[1].Texts[0].ID
	_, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}, true, false)
	if err != nil {
		t.Fatal(err)
	}
	// Копия встаёт в стек куском над новыми местами (putOrder) — под текстом левой страницы.
	ts := slices.Clone(edtest.P(e).Spreads[1].Texts)
	if len(ts) == 2 && ts[0].ID != keep {
		ts[0], ts[1] = ts[1], ts[0]
	}
	if len(ts) != 2 || ts[0].ID != keep || ts[1].ID == src.ID || ts[1].X != src.X+281 || ts[1].Text != src.Text || edtest.P(e).Spreads[1].BgR != "#aabbcc" || edtest.P(e).Spreads[1].BgL != "" {
		t.Fatalf("тексты %+v %+v, фон %q %q", *ts[0], *ts[1], edtest.P(e).Spreads[1].BgL, edtest.P(e).Spreads[1].BgR)
	}
	// Сверх MaxTexts — ошибка всей правки: два блока источника вместо одного у цели.
	e.AddText("s1", 0, "")
	edtest.Change(e, func(p *model.Project) {
		for len(p.Spreads[1].Texts) < model.MaxTexts {
			p.Spreads[1].Texts = append(p.Spreads[1].Texts, &model.TextBlock{ID: model.NewID(), X: 10, Y: 10, W: 50, Text: "x", Style: model.PlainStyle()})
		}
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}, true, false); err == nil {
		t.Fatal("сверх MaxTexts без ошибки")
	}
	edtest.Same(t, e, rev, undo, "MaxTexts")
	// Ровно 64 после копии (спека 026) — проходит: у цели 63, один заменится, два придут.
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Texts = p.Spreads[1].Texts[:model.MaxTexts-1] })
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}, true, false); err != nil || len(edtest.P(e).Spreads[1].Texts) != 64 {
		t.Fatalf("ровно 64: %v, блоков %d", err, len(edtest.P(e).Spreads[1].Texts))
	}
}

func TestDuplicateGrid(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar(), []string{"d"})
	e.AddText("s1", 0, "")
	e.SetBackground("s1", "spread", "#010203")
	caps, vars := model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 2, Text: "{Имя}"}, model.Vars{Title: "Лицей", Lead: "И. И."}
	edtest.Change(e, func(p *model.Project) { c, v := caps, vars; p.Spreads[0].Caps, p.Spreads[0].Vars = &c, &v })
	undo := edtest.UndoLen(e)
	if _, err := e.DuplicateGrid("s1", 2, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	if len(edtest.P(e).Spreads) != 4 || edtest.P(e).Spreads[3].ID != "s2" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("развороты %d", len(edtest.P(e).Spreads))
	}
	for i := 1; i <= 2; i++ {
		s := edtest.P(e).Spreads[i]
		wantRects(t, rectsOf(e, i), rectsOf(e, 0))
		if len(s.Photos) != 0 || len(s.Texts) != 1 || s.Texts[0].ID == edtest.P(e).Spreads[0].Texts[0].ID || s.BgL != "#010203" {
			t.Fatalf("копия %d: фото %v, тексты %d, фон %q", i, s.Photos, len(s.Texts), s.BgL)
		}
		if s.Caps == nil || *s.Caps != caps || s.Caps == edtest.P(e).Spreads[0].Caps || s.Vars == nil || *s.Vars != vars || s.Vars == edtest.P(e).Spreads[0].Vars {
			t.Fatalf("копия %d: стиль подписей листа %+v, поля шапки %+v", i, s.Caps, s.Vars)
		}
	}
	rev := edtest.Rev(e)
	e.DuplicateGrid("s1", 0, false)
	if edtest.Rev(e) != rev {
		t.Fatal("count 0 поменял проект")
	}
	if v, _ := e.DuplicateGrid("s1", 1000, false); len(v.Spreads) != 4+grid.MaxDuplicate {
		t.Fatalf("предел: %d", len(v.Spreads))
	}
}

func TestApplyGridPour(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 80, 80, "a"), edtest.Gp(293, 12, 127, 127, "d"), edtest.Gp(100, 12, 80, 80, "b"), edtest.Gp(12, 150, 80, 80, "c")})
	right := edtest.P(e).Spreads[0].Grid[1].ID
	_, left, err := e.ApplyGrid("s1", model.ScopeLeft, model.TplPage, []model.Box{{X: 0.1, Y: 0.1, W: 0.4, H: 0.4}, {X: 0.5, Y: 0.5, W: 0.4, H: 0.4}})
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if err != nil || left != 1 || len(g) != 3 || g[2].ID != right || !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "b", "d"}) || edtest.HasRef(edtest.P(e), "c") {
		t.Fatalf("err %v, left %d, фото %v", err, left, edtest.PhotosOf(e, 0))
	}
	wantRects(t, rectsOf(e, 0)[:2], []layout.Rect{{X: 28.1, Y: 28.4, W: 112.4, H: 113.6}, {X: 140.5, Y: 142, W: 112.4, H: 113.6}})
	// NaN — отказ всей правки; шаблон разворота на страницу — отказ.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.ApplyGrid("s1", model.ScopeLeft, model.TplPage, []model.Box{{X: math.NaN(), W: 0.5, H: 0.5}}); err == nil {
		t.Fatal("NaN без ошибки")
	}
	if _, _, err := e.ApplyGrid("s1", model.ScopeLeft, model.TplSpread, []model.Box{{W: 0.5, H: 0.5}}); !errors.Is(err, grid.ErrTplScope) {
		t.Fatalf("область шаблона: %v", err)
	}
	edtest.Same(t, e, rev, undo, "отказ")
	// Шаблон страницы на разворот — на обе страницы; «Авто» закрепляется.
	a := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(a, -1, edtest.Photos("x", "y", "z"))
	_, left, err = a.ApplyGrid(v.Spreads[0].ID, model.ScopeSpread, model.TplPage, []model.Box{{X: 0, Y: 0, W: 1, H: 1}})
	if err != nil || left != 1 || len(edtest.P(a).Spreads[0].Grid) != 2 || edtest.P(a).Spreads[0].Grid[1].X != 300 {
		t.Fatalf("err %v, left %d, мест %d", err, left, len(edtest.P(a).Spreads[0].Grid))
	}
}

func TestApplyKeepsTextsAndBg(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar())
	e.AddText("s1", 0, "")
	e.SetBackground("s1", "left", "#123456")
	texts := marshalTexts(t, e)
	e.ApplyGrid("s1", model.ScopeLeft, model.TplPage, []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.6}})
	if !bytes.Equal(texts, marshalTexts(t, e)) || edtest.P(e).Spreads[0].BgL != "#123456" {
		t.Fatal("тексты или фон изменились")
	}
}

func TestFillPlaces(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, ""), edtest.Gp(150, 12, 100, 100, "a")}, []string{"x"})
	e.MoveSpread("s1", 1)
	edtest.Change(e, func(p *model.Project) {
		pl := edtest.Gp(12, 12, 50, 50, "")
		pl.ID = "00000000000a"
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s3", Photos: []string{}, Grid: []*model.Place{pl}})
	})
	_, left := e.FillPlaces(edtest.Photos("x", "c", "d", "f"), "s1")
	edtest.MustGrid(t, e)
	if left != 1 || !slices.Equal(edtest.PhotosOf(e, 1), []string{"c", "a"}) || !slices.Equal(edtest.PhotosOf(e, 2), []string{"d"}) ||
		!slices.Equal(edtest.P(e).Spreads[0].Photos, []string{"x"}) || edtest.HasRef(edtest.P(e), "f") {
		t.Fatalf("left %d, %v %v %v", left, edtest.P(e).Spreads[0].Photos, edtest.PhotosOf(e, 1), edtest.PhotosOf(e, 2))
	}
}

func TestPlanCopyGrid(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 127, 127, "b")}, []string{"d", "f", "g"}, []string{"h"})
	e.AddText("s2", 0, "")
	data, rev := model.Marshal(edtest.P(e)), edtest.Rev(e)
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}, {Spread: "s3", Scope: model.ScopeSpread}}
	plan := e.PlanCopyGrid("s1", model.ScopeSpread, to, true, false)
	if !bytes.Equal(data, model.Marshal(edtest.P(e))) || edtest.Rev(e) != rev {
		t.Fatal("план поменял проект")
	}
	// s2: три фото на два места — одно уходит, блок текста заменяется; s3 — h остаётся.
	if plan != (model.GridPlan{Frozen: 2, Left: 1, Texts: 1}) {
		t.Fatalf("план %+v", plan)
	}
	_, left, err := e.CopyGrid("s1", model.ScopeSpread, to, true, false)
	if err != nil || left != plan.Left {
		t.Fatalf("копия: left %d, err %v", left, err)
	}
	if p := e.PlanCopyGrid("s1", "nope", to, false, false); p != (model.GridPlan{}) {
		t.Fatalf("ошибка — пустой план: %+v", p)
	}
}

func TestCopyGridMirror(t *testing.T) {
	e := edtest.GridEd(t, edtest.Calendar()[:2], []string{}, []string{}) // цели без фото: места только из копий
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}, {Spread: "s3", Scope: model.ScopeLeft}, {Spread: "s3", Scope: model.ScopeRight}}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, to, false, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	mirrored := []layout.Rect{{X: 281 + 142, Y: 12, W: 127, H: 127}, {X: 281 + 69, Y: 142, W: 200, H: 130}}
	asIs := rectsOf(e, 0)
	wantRects(t, rectsOf(e, 1), mirrored)
	wantRects(t, rectsOf(e, 2), append(slices.Clone(asIs), mirrored...))
	// Источник-разворот: флажок ничего не делает.
	e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, true)
	wantRects(t, rectsOf(e, 1), rectsOf(e, 0))
}
