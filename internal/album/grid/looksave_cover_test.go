package grid_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Круг «сохранить альбом как макет» → «поставить на обложку другого альбома» (131 G6b + G5): AlbumTplOf
// снимает обложку, ApplyLookStep(CoverID, 0) ставит её на чистую обложку того же формата.

var lkSaveRef = model.LookRef{Key: "my:00000000000000f1", Name: "Круг"}

// cleanCover — чистая обложка того же формата, что у fsfera: ни мест, ни надписей, ни фона.
func cleanCover(t *testing.T) *cv {
	t.Helper()
	c := fsfera(t)
	c.clear(model.ScopeLeft)
	c.clear(model.ScopeRight)
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Sheet.Texts = nil })
	return c
}

// coverMacro — макет из обложки src (развороту для шага задаётся цвет: серия не пуста).
func coverMacro(t *testing.T, src *cv) model.AlbumTpl {
	t.Helper()
	first := ids(src.e)[0]
	src.e.SetBackground(first, model.ScopeLeft, "#abcdef")
	o := saveOpts(first)
	o.Cover = true
	tpl, err := src.e.AlbumTplOf(o)
	if err != nil || tpl.Cover == nil {
		t.Fatalf("макет с обложкой: %v %+v", err, tpl.Cover)
	}
	return tpl
}

// roundCover ставит обложку макета tpl на чистую обложку и сверяет с src: места ±0,01 мм, цвета,
// узоры листа и сторон, число стикеров и заготовок; повтор — холостая правка (rev, проект).
func roundCover(t *testing.T, src *cv, tpl model.AlbumTpl) *cv {
	t.Helper()
	dst := cleanCover(t)
	if _, _, _, err := dst.e.ApplyLookStep(model.CoverID, 0, lkSaveRef, tpl, nil); err != nil {
		t.Fatal(err)
	}
	sp, dp := src.p().Cover.Sheet, dst.p().Cover.Sheet
	for _, scope := range []string{model.ScopeLeft, model.ScopeRight} {
		want, got := rectsOfPlaces(src.p(), scope), rectsOfPlaces(dst.p(), scope)
		if len(want) != len(got) {
			t.Fatalf("%s: мест %d, было %d", scope, len(got), len(want))
		}
		for i := range want {
			for k := range want[i] {
				if math.Abs(want[i][k]-got[i][k]) > 0.011 {
					t.Errorf("%s место %d: %v, было %v", scope, i+1, got[i], want[i])
					break
				}
			}
		}
	}
	if dp.BgL != sp.BgL || dp.BgR != sp.BgR || !reflect.DeepEqual(dp.Pat, sp.Pat) || !reflect.DeepEqual(dp.PatL, sp.PatL) || !reflect.DeepEqual(dp.PatR, sp.PatR) {
		t.Errorf("фон: цвета %q/%q (было %q/%q), узоры лист %v лев %v прав %v (было %v %v %v)",
			dp.BgL, dp.BgR, sp.BgL, sp.BgR, dp.Pat, dp.PatL, dp.PatR, sp.Pat, sp.PatL, sp.PatR)
	}
	if len(dp.Stickers) != len(sp.Stickers) || len(dp.Texts) != len(sp.Texts) {
		t.Errorf("стикеров %d / надписей %d, было %d / %d", len(dp.Stickers), len(dp.Texts), len(sp.Stickers), len(sp.Texts))
	}
	for _, tx := range dp.Texts {
		if !tx.Draft {
			t.Errorf("надпись %q не заготовка", tx.Text)
		}
	}
	for _, pl := range dp.Grid {
		if pl.Photo != "" {
			t.Errorf("фото в месте %s попало в макет", pl.Photo)
		}
	}
	rev, data := edtest.Rev(dst.e), snapshot(dst.e)
	if _, _, _, err := dst.e.ApplyLookStep(model.CoverID, 0, lkSaveRef, tpl, nil); err != nil || edtest.Rev(dst.e) != rev || string(snapshot(dst.e)) != string(data) {
		t.Errorf("повтор не холостой: %v", err)
	}
	return dst
}

// TestAlbumTplOfCoverRoundFull: обложка фотографа с фото в местах, свои цвета сторон, узор листа,
// стикер и надпись → макет → чистая обложка другого альбома; фото в макет не идут.
func TestAlbumTplOfCoverRoundFull(t *testing.T) {
	src := fsfera(t)
	src.fill(model.ScopeLeft, "h1", "v2")
	src.fill(model.ScopeRight, "v3", "h4", "s5")
	src.e.SetBackground(model.CoverID, model.ScopeLeft, "#112233")
	src.e.SetBackground(model.CoverID, model.ScopeRight, "#445566")
	mustPattern(t, src.e, model.CoverID, model.ScopeSpread, "dots")
	if _, _, err := src.e.AddSticker(model.CoverID, 1, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	tpl := coverMacro(t, src)
	if tpl.Cover.Bg == nil || tpl.Cover.Bg.Pat == nil || tpl.Cover.Back.Page[0].Pat != nil || tpl.Cover.Front.Page[0].Pat != nil {
		t.Fatalf("узор листа должен быть в Bg один раз: %+v / %+v / %+v", tpl.Cover.Bg, tpl.Cover.Back.Page, tpl.Cover.Front.Page)
	}
	dst := roundCover(t, src, tpl)
	if dst.p().Cover.Sheet.Pat == nil || len(dst.p().Cover.Sheet.Grid) != 5 {
		t.Errorf("узор %v, мест %d", dst.p().Cover.Sheet.Pat, len(dst.p().Cover.Sheet.Grid))
	}
}

// TestAlbumTplOfCoverRoundSidePattern: узор только на лицевой (без узора листа) → Front.Page[0].Pat;
// на чистой обложке он встаёт на правую страницу, левая без узора; повтор холостой.
func TestAlbumTplOfCoverRoundSidePattern(t *testing.T) {
	src := fsfera(t)
	src.fill(model.ScopeRight, "v1", "h2")
	mustPattern(t, src.e, model.CoverID, model.ScopeRight, "dots")
	tpl := coverMacro(t, src)
	if tpl.Cover.Bg != nil || tpl.Cover.Front == nil || tpl.Cover.Front.Page[0].Pat == nil || tpl.Cover.Front.Page[0].Pat.Kind != "dots" {
		t.Fatalf("узор лицевой не в Front.Page[0]: Bg %+v, Front %+v", tpl.Cover.Bg, tpl.Cover.Front)
	}
	if tpl.Cover.Back != nil && tpl.Cover.Back.Page[0].Pat != nil {
		t.Errorf("узор на задней: %+v", tpl.Cover.Back.Page[0])
	}
	dst := roundCover(t, src, tpl)
	if s := dst.p().Cover.Sheet; s.PatR == nil || s.PatL != nil || s.Pat != nil {
		t.Errorf("узоры на чистой обложке: лист %v, лев %v, прав %v", s.Pat, s.PatL, s.PatR)
	}
}

// TestAlbumTplOfCoverRoundBackColor: свой цвет только у задней, фона листа нет — Bg пуст, цвет в Back.
func TestAlbumTplOfCoverRoundBackColor(t *testing.T) {
	src := fsfera(t)
	src.fill(model.ScopeLeft, "h1")
	src.fill(model.ScopeRight, "v2")
	src.e.SetBackground(model.CoverID, model.ScopeLeft, "#a1b2c3")
	tpl := coverMacro(t, src)
	if tpl.Cover.Bg != nil || tpl.Cover.Back.Page[0].Color != "#a1b2c3" || tpl.Cover.Front.Page[0].Color != "" {
		t.Fatalf("обложка макета: Bg %+v, Back %+v, Front %+v", tpl.Cover.Bg, tpl.Cover.Back.Page, tpl.Cover.Front.Page)
	}
	if dst := roundCover(t, src, tpl); dst.p().Cover.Sheet.BgR != "" {
		t.Errorf("лицевая получила цвет %q", dst.p().Cover.Sheet.BgR)
	}
}

// TestAlbumTplOfCoverRoundNoBg: без фона вовсе (только места) — ничего лишнего не появляется.
func TestAlbumTplOfCoverRoundNoBg(t *testing.T) {
	src := fsfera(t)
	src.fill(model.ScopeLeft, "h1", "v2")
	src.fill(model.ScopeRight, "v3")
	edtest.Change(src.e, func(p *model.Project) { p.Cover.Sheet.Texts = nil })
	tpl := coverMacro(t, src)
	if tpl.Cover.Bg != nil || tpl.Cover.Back.Page[0] != (model.TplBg{}) || tpl.Cover.Front.Page[0] != (model.TplBg{}) {
		t.Fatalf("фон появился: %+v", tpl.Cover)
	}
	roundCover(t, src, tpl)
}

// TestAlbumTplOfCoverSidePhoto: фото фона стороны из библиотеки идёт в страницу стороны (Lib, Fade), а
// кадр съёмки — нет; при фото листа из библиотеки сторона несёт только цвет.
func TestAlbumTplOfCoverSidePhoto(t *testing.T) {
	src := fsfera(t)
	src.fill(model.ScopeRight, "v1")
	if _, err := src.e.SetBackdrop(model.CoverID, model.ScopeRight, libInfo(kBg)); err != nil {
		t.Fatal(err)
	}
	if _, err := src.e.SetBackdrop(model.CoverID, model.ScopeLeft, model.PhotoInfo{ID: "shot", Path: edtest.Abs("shot.jpg"), W: 300, H: 200}); err != nil {
		t.Fatal(err)
	}
	tpl := coverMacro(t, src)
	if tpl.Cover.Bg != nil || tpl.Cover.Front.Page[0].Lib != kBg {
		t.Fatalf("фото фона лицевой: Bg %+v, Front %+v", tpl.Cover.Bg, tpl.Cover.Front.Page)
	}
	if tpl.Cover.Back != nil && tpl.Cover.Back.Page[0] != (model.TplBg{}) {
		t.Errorf("кадр съёмки задней попал в макет: %+v", tpl.Cover.Back.Page)
	}
	whole := fsfera(t)
	whole.fill(model.ScopeRight, "v1")
	if _, err := whole.e.SetBackdrop(model.CoverID, model.ScopeSpread, libInfo(kBg)); err != nil {
		t.Fatal(err)
	}
	wt := coverMacro(t, whole)
	if wt.Cover.Bg == nil || wt.Cover.Bg.Lib != kBg || wt.Cover.Front.Page[0].Lib != "" || wt.Cover.Back != nil && wt.Cover.Back.Page[0].Lib != "" {
		t.Errorf("фото листа продублировано в сторонах: Bg %+v, Front %+v", wt.Cover.Bg, wt.Cover.Front.Page)
	}
}
