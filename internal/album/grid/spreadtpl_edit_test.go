package grid_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Готовые развороты (067): отмена, холостой повтор, области, виньетка, пропавшие файлы.

// pageTpl — страница: место слева сверху, звезда в левом верхнем углу под углом, свой стикер
// из библиотеки снизу, надпись у левого края; цвет и своё фото фона.
func pageTpl() model.SpreadTpl {
	return model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 281.0 / 284, W: 281, H: 284,
		Page:   []model.TplBg{{Color: "#faf0e6", Lib: kBg, Fade: 0.2}},
		Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.4}, {X: 0.5, Y: 0.55, W: 0.4, H: 0.35}},
		Stickers: []model.TplItem{
			{Anchor: model.Anchor{AX: -1, AY: -1, DX: 0.08, DY: 0.08}, W: 0.1, H: 0.1, Rot: 20, Art: &model.Art{Kind: "star"}},
			{Anchor: model.Anchor{AX: 0, AY: 1, DY: -0.05}, W: 0.2, H: 0.1, Lib: kSt},
		},
		Texts: []model.TplText{{Anchor: model.Anchor{AX: -1, AY: 0, DX: 0.2}, W: 0.3, Text: "Глава 1", Style: model.PlainStyle()}},
	}
}

// TestApplySpreadTplUndo — места и оформление одной правкой: один шаг; Undo возвращает прежний
// JSON проекта, Redo — поставленный.
func TestApplySpreadTplUndo(t *testing.T) {
	e := tplEd(t)
	tp, err := e.SpreadTplOf("s1", model.ScopeSpread, true, true)
	if err != nil {
		t.Fatal(err)
	}
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	if _, _, _, err := e.ApplySpreadTpl("s2", model.ScopeSpread, tp, tplLibs()); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	after := model.Marshal(edtest.P(e))
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("одна отмена не вернула всё")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("Redo не вернул поставленное")
	}
}

// TestApplySpreadTplNoop — повтор того же шаблона на ту же область — холостая правка (ни шага,
// ни rev, JSON тот же): разворот со всем на пустой лист, страница на весь разворот (зеркало,
// надписи, свои), оформление без мест на «Авто» (оно остаётся «Авто»), и на лист с фото.
func TestApplySpreadTplNoop(t *testing.T) {
	e := tplEd(t)
	full, err := e.SpreadTplOf("s1", model.ScopeSpread, true, true)
	if err != nil {
		t.Fatal(err)
	}
	decor := pageTpl()
	decor.Places = nil
	for _, c := range []struct {
		name, sp, scope string
		tp              model.SpreadTpl
	}{
		{"разворот на пустой", "s3", model.ScopeSpread, full},
		{"страница на разворот", "s4", model.ScopeSpread, pageTpl()},
		{"оформление на «Авто»", "s2", model.ScopeRight, decor},
		{"разворот на свой лист", "s1", model.ScopeSpread, full},
	} {
		if _, _, _, err := e.ApplySpreadTpl(c.sp, c.scope, c.tp, tplLibs()); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		before, undo, rev := model.Marshal(edtest.P(e)), edtest.UndoLen(e), edtest.Rev(e)
		v, plan, _, err := e.ApplySpreadTpl(c.sp, c.scope, c.tp, tplLibs())
		if err != nil || plan != (model.GridPlan{}) {
			t.Fatalf("%s: повтор %+v, %v", c.name, plan, err)
		}
		if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != undo || edtest.Rev(e) != rev || v.Rev != rev {
			t.Fatalf("%s: повтор дал правку:\n%s\n%s", c.name, before, model.Marshal(edtest.P(e)))
		}
	}
	if s2 := sheetOf(e, "s2"); s2.Grid != nil || s2.BgR != "#faf0e6" || len(s2.Stickers) != 2 {
		t.Fatalf("«Авто» без мест: %s", jsonOf(t, s2))
	}
}

// TestApplySpreadTplScope — страница на весь разворот: левая как есть, правая зеркально
// (места и стикеры отражены, надпись сдвинута без зеркала); разворот на страницу — ErrTplScope
// без правки; обложка, корешок и чужая область — ErrScope; лист 031 — одна область, без мест.
func TestApplySpreadTplScope(t *testing.T) {
	e := tplEd(t)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, pageTpl(), tplLibs()); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	s := sheetOf(e, "s3")
	if len(s.Grid) != 4 || s.BgL != "#faf0e6" || s.BgR != "#faf0e6" || s.BgPhotoL == nil || s.BgPhotoR == nil || s.BgPhotoR.X != 281 {
		t.Fatalf("оформление обеих: %s", jsonOf(t, s))
	}
	for i := range 2 { // правая — отражение левой в своей странице
		l, r := s.Grid[i], s.Grid[i+2]
		if !near(r.X, 2*281+281-l.X-l.W-281) || !near(r.Y, l.Y) || !near(r.W, l.W) || !near(l.X, 281*pageTpl().Places[i].X) {
			t.Fatalf("место %d: левое %+v, правое %+v", i, *l, *r)
		}
	}
	var ls, rs *model.Sticker
	for _, st := range s.Stickers {
		if st.Art != nil && st.X < 281 {
			ls = st
		} else if st.Art != nil {
			rs = st
		}
	}
	if ls == nil || rs == nil || ls.FlipH || !rs.FlipH || ls.Rot != 20 || rs.Rot != -20 || !near(rs.X+rs.W/2, 562-(ls.X+ls.W/2)) || !near(rs.Y, ls.Y) {
		t.Fatalf("звёзды: %s", jsonOf(t, s.Stickers))
	}
	if len(s.Texts) != 2 || !near(s.Texts[1].X-s.Texts[0].X, 281) || s.Texts[0].Y != s.Texts[1].Y {
		t.Fatalf("надписи: %s", jsonOf(t, s.Texts))
	}

	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	full, _ := e.SpreadTplOf("s1", model.ScopeSpread, true, true)
	for _, c := range []struct {
		sp, scope string
		tp        model.SpreadTpl
		want      error
	}{
		{"s2", model.ScopeLeft, full, grid.ErrTplScope},
		{"s2", model.ScopeSpine, pageTpl(), model.ErrScope},
		{"s2", "куда-то", pageTpl(), model.ErrScope},
	} {
		if _, _, _, err := e.ApplySpreadTpl(c.sp, c.scope, c.tp, tplLibs()); !errors.Is(err, c.want) {
			t.Errorf("%s/%s: %v, ждали %v", c.sp, c.scope, err, c.want)
		}
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != undo {
		t.Fatal("отказ тронул проект")
	}

	ce := edtest.CoverEd(t, "cv", nil)
	if _, _, _, err := ce.ApplySpreadTpl(model.CoverID, model.ScopeRight, pageTpl(), nil); !errors.Is(err, model.ErrScope) {
		t.Errorf("обложка: %v", err)
	}
	if _, err := ce.SpreadTplOf(model.CoverID, model.ScopeRight, true, true); !errors.Is(err, model.ErrScope) {
		t.Errorf("снимок обложки: %v", err)
	}

	// Лист 031: страница — одна область во весь лист, места не ставятся; разворот — отказ.
	se := edtest.Open(t, model.NewSheet(200, 300), "")
	sid := edtest.P(se).Spreads[0].ID
	if _, _, _, err := se.ApplySpreadTpl(sid, model.ScopeSpread, pageTpl(), tplLibs()); err != nil {
		t.Fatal(err)
	}
	sh := edtest.P(se).Spreads[0]
	if sh.Grid != nil || sh.BgL != "#faf0e6" || sh.BgR != "#faf0e6" || sh.BgPhoto == nil || sh.BgPhotoL != nil || len(sh.Stickers) != 2 || len(sh.Texts) != 1 {
		t.Fatalf("лист 031: %s", jsonOf(t, sh))
	}
	if _, _, _, err := se.ApplySpreadTpl(sid, model.ScopeSpread, full, nil); !errors.Is(err, grid.ErrTplScope) {
		t.Errorf("разворот на лист 031: %v", err)
	}
	if _, _, _, err := se.ApplySpreadTpl(sid, model.ScopeLeft, pageTpl(), nil); !errors.Is(err, model.ErrScope) {
		t.Errorf("страница листа 031: %v", err)
	}
	if one, err := se.SpreadTplOf(sid, model.ScopeSpread, true, true); err != nil || one.Pages != 1 || one.Page[0].Lib != kBg {
		t.Errorf("снимок листа 031: %s, %v", jsonOf(t, one), err)
	}
}

// TestApplySpreadTplVignette — на виньетке места не меняются (сетка и фото те же, план без
// ушедших), шапка остаётся; оформление и надписи заменяются.
func TestApplySpreadTplVignette(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")})
	v, err := e.AddVignette(-1)
	if err != nil {
		t.Fatal(err)
	}
	vid := v.Spreads[len(v.Spreads)-1].ID
	vs := sheetOf(e, vid)
	head, grid0 := *vs.VigHead(vs.Vig.Head), jsonOf(t, vs.Grid)
	_, plan, _, err := e.ApplySpreadTpl(vid, model.ScopeSpread, pageTpl(), tplLibs())
	if err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	vs = sheetOf(e, vid)
	if jsonOf(t, vs.Grid) != grid0 || plan.Left != 0 || plan.Frozen != 0 {
		t.Fatalf("места виньетки тронуты: план %+v", plan)
	}
	if h := vs.VigHead(vs.Vig.Head); h == nil || *h != head || vs.BgL != "#faf0e6" || len(vs.Texts) != 3 {
		t.Fatalf("виньетка: %s", jsonOf(t, vs))
	}
}

// TestApplySpreadTplMissing — своего нет в библиотеке этого ПК: слои с ним (фото фона, стикер)
// пропущены, missing — число таких ключей (один ключ на двух страницах — один), рисунки стоят;
// правка — один шаг. Файл есть — ставится всё.
func TestApplySpreadTplMissing(t *testing.T) {
	e := tplEd(t)
	undo := edtest.UndoLen(e)
	_, _, missing, err := e.ApplySpreadTpl("s3", model.ScopeSpread, pageTpl(), map[string]model.PhotoInfo{kSt: tplLibs()[kSt]})
	if err != nil || missing != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("пропавших %d, шагов %d, %v", missing, edtest.UndoLen(e)-undo, err)
	}
	s := sheetOf(e, "s3")
	if s.BgPhotoL != nil || s.BgPhotoR != nil || len(s.Stickers) != 4 || s.BgL != "#faf0e6" {
		t.Fatalf("без фона: %s", jsonOf(t, s))
	}
	_, _, missing, err = e.ApplySpreadTpl("s4", model.ScopeLeft, pageTpl(), nil)
	if s4 := sheetOf(e, "s4"); err != nil || missing != 2 || len(s4.Stickers) != 1 || s4.Stickers[0].Art == nil {
		t.Fatalf("libs = nil: %d, %v, %s", missing, err, jsonOf(t, s4.Stickers))
	}
	_, _, missing, err = e.ApplySpreadTpl("s4", model.ScopeLeft, pageTpl(), tplLibs())
	if s4 := sheetOf(e, "s4"); err != nil || missing != 0 || s4.BgPhotoL == nil || len(s4.Stickers) != 2 {
		t.Fatalf("с файлами: %d, %v, %s", missing, err, jsonOf(t, s4))
	}
	if r := edtest.P(e).Photos[model.LibPhotoID(kBg)]; r.Lib != kBg || r.W != 3000 {
		t.Fatalf("запись своего фона: %+v", r)
	}
}

// TestSpreadTplWhole — узор и своё фото на весь лист снимаются в Whole (постраничных нет) и
// ставятся на весь лист цели; стикер-фото не из библиотеки (кадр съёмки) в шаблон не идёт;
// снимок чужой области — ErrScope.
func TestSpreadTplWhole(t *testing.T) {
	e := tplEd(t)
	mustPattern(t, e, "s1", model.ScopeSpread, "stripes")
	if _, err := e.SetBackdrop("s1", model.ScopeSpread, libInfo(kBg)); err != nil {
		t.Fatal(err)
	}
	putStickers(e, "s1", &model.Sticker{ID: model.NewID(), Photo: "a", X: 300, Y: 260, W: 30, H: 20})
	tp, err := e.SpreadTplOf("s1", model.ScopeSpread, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if tp.Whole == nil || tp.Whole.Pat == nil || tp.Whole.Pat.Kind != "stripes" || tp.Whole.Lib != kBg || tp.Page[0].Lib != "" || tp.Page[1].Pat != nil || len(tp.Stickers) != 3 {
		t.Fatalf("снимок листа: %s", jsonOf(t, tp))
	}
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, tp, tplLibs()); err != nil {
		t.Fatal(err)
	}
	s := sheetOf(e, "s3")
	if s.Pat == nil || s.Pat.Kind != "stripes" || s.PatL != nil || s.PatR != nil || s.BgPhoto == nil || s.BgPhotoL != nil || s.BgPhoto.W != 562 || s.Grid != nil {
		t.Fatalf("лист s3: %s", jsonOf(t, s))
	}
	if _, err := e.SpreadTplOf("s1", model.ScopeSpine, true, true); !errors.Is(err, model.ErrScope) {
		t.Errorf("корешок: %v", err)
	}
}

// TestApplySpreadTplLimits — пределы листа: страница с 40 местами на весь разворот (80 > 64
// мест) и с 40 стикерами (80 > 64) — отказ всей правки без шага; на одну страницу — встаёт.
func TestApplySpreadTplLimits(t *testing.T) {
	e := tplEd(t)
	many := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{}}}
	for i := range 40 {
		many.Places = append(many.Places, model.Box{X: float64(i%8) / 8, Y: float64(i/8) / 5, W: 0.1, H: 0.1})
	}
	stickers := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{}}}
	for i := range 40 {
		stickers.Stickers = append(stickers.Stickers, model.TplItem{Anchor: model.Anchor{DX: float64(i) / 100}, W: 0.05, H: 0.05, Art: &model.Art{Kind: "star"}})
	}
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, many, nil); err == nil || err.Error() != model.ErrTooManyPlaces().Error() {
		t.Errorf("80 мест: %v", err)
	}
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeSpread, stickers, nil); !errors.Is(err, model.ErrStickers) {
		t.Errorf("80 стикеров: %v", err)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != undo {
		t.Fatal("отказ тронул проект")
	}
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeLeft, many, nil); err != nil || len(sheetOf(e, "s3").Grid) != 40 {
		t.Fatalf("40 мест на страницу: %v", err)
	}
}

// TestPreviewTpl — плитка: разворот — лист шаблона в мм плитки, страница — левая страница
// листа той же ширины; места пустые (плашки рисует экспорт), вид строится.
func TestPreviewTpl(t *testing.T) {
	two := pageTpl()
	two.Pages, two.Page = 2, []model.TplBg{{Color: "#111111"}, {Color: "#222222"}}
	for _, c := range []struct {
		tp         model.SpreadTpl
		pageW      float64
		places     int
		bgL, bgR   string
		withPhotos bool
	}{
		{pageTpl(), 300, 2, "#faf0e6", "", true},
		{two, 150, 2, "#111111", "#222222", false},
	} {
		p, v, err := album.PreviewTpl(c.tp, 300, 200, tplLibs())
		if err != nil {
			t.Fatal(err)
		}
		s := p.Spreads[0]
		if p.Settings.PageW != c.pageW || p.Settings.PageH != 200 || len(s.Grid) != c.places || len(v.Spreads[0].Places) != c.places ||
			s.BgL != c.bgL || s.BgR != c.bgR || (s.BgPhotoL != nil) != c.withPhotos {
			t.Fatalf("плитка %d стр.: %s", c.tp.Pages, jsonOf(t, s))
		}
		for _, pl := range s.Grid {
			if pl.Photo != "" {
				t.Fatalf("в плитке фото: %+v", *pl)
			}
		}
	}
}
