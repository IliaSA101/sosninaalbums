package grid_test

import (
	"errors"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Готовые развороты (спека 067, design §2, §9): снять → поставить, якоря, места, план, плитка.

const (
	kBg = "my:00000000000000b1" // своё фото фона из библиотеки
	kSt = "my:00000000000000c1" // свой стикер из библиотеки (400×200)
)

// tplLibs — файлы библиотеки «этого ПК» (как их отдаёт App по ключу).
func tplLibs() map[string]model.PhotoInfo {
	return map[string]model.PhotoInfo{
		kBg: {Path: edtest.Abs("lib-bg.jpg"), W: 3000, H: 2000},
		kSt: {Path: edtest.Abs("lib-st.png"), W: 400, H: 200},
	}
}

// libInfo — сведения о своём из библиотеки для правок 048 (ID — от ключа).
func libInfo(key string) model.PhotoInfo {
	ph := tplLibs()[key]
	ph.ID, ph.Lib = model.LibPhotoID(key), key
	return ph
}

// tplEd — s1: место a слева, b и c справа (с фото); цвета страниц, узор правой, своё фото фона
// левой; стикеры: звезда в правом верхнем углу, свой из библиотеки слева внизу, лента-плашка
// во всю ширину снизу под всеми местами; надпись слева. s2 — «Авто» (d, e), s3 и s4 — пустые.
func tplEd(t *testing.T) *album.Editor {
	t.Helper()
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 200, "a"), edtest.Gp(293, 12, 127, 127, "b"), edtest.Gp(293, 150, 127, 100, "c")},
		[]string{"d", "e"}, []string{}, []string{})
	e.SetBackground("s1", model.ScopeLeft, "#aabbcc")
	e.SetBackground("s1", model.ScopeRight, "#ddeeff")
	mustPattern(t, e, "s1", model.ScopeRight, "dots")
	if _, err := e.SetBackdrop("s1", model.ScopeLeft, libInfo(kBg)); err != nil {
		t.Fatal(err)
	}
	_, star, err := e.AddSticker("s1", 1, model.StickerSrc{Art: &model.Art{Kind: "star"}})
	if err != nil {
		t.Fatal(err)
	}
	e.SetStickerBox("s1", star, 520, 12, 30, 30, 15, "")
	_, own, err := e.AddSticker("s1", 0, model.StickerSrc{Photo: libInfo(kSt)})
	if err != nil {
		t.Fatal(err)
	}
	e.SetStickerBox("s1", own, 40, 225, 40, 20, 0, "")
	band := plate(0, 254, 562, 30, 0)
	putStickers(e, "s1", band)
	edtest.Change(e, func(p *model.Project) { // лента — под всеми местами (фоновый орнамент)
		s := p.Spreads[0]
		st := slices.DeleteFunc(s.Stack(), func(id string) bool { return id == band.ID })
		s.SetStack(append([]string{band.ID}, st...))
	})
	e.SetText(edtest.AddText(t, e, "s1", 0), "Наша свадьба", "")
	edtest.MustGrid(t, e)
	return e
}

// near — мм с допуском круга «снять → поставить» (design §9: ±0,05).
func near(a, b float64) bool { return math.Abs(a-b) <= 0.05 }

// sameRects — места двух листов по порядку совпадают в мм (±0,05).
func sameRects(t *testing.T, got, want []*model.Place) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("мест %d, ждали %d", len(got), len(want))
	}
	for i, g := range got {
		w := want[i]
		if !near(g.X, w.X) || !near(g.Y, w.Y) || !near(g.W, w.W) || !near(g.H, w.H) || g.Rot != w.Rot {
			t.Fatalf("место %d: %+v, ждали %+v", i, *g, *w)
		}
	}
}

// stickerLike — стикер листа с тем же рисунком (своим) и рамкой ±0,05; nil — нет.
func stickerLike(s *model.Spread, w *model.Sticker) *model.Sticker {
	for _, g := range s.Stickers {
		if (g.Art == nil) == (w.Art == nil) && (g.Art == nil || g.Art.Kind == w.Art.Kind) && g.Photo == w.Photo &&
			near(g.X, w.X) && near(g.Y, w.Y) && near(g.W, w.W) && near(g.H, w.H) && g.Rot == w.Rot && g.FlipH == w.FlipH && g.FlipV == w.FlipV {
			return g
		}
	}
	return nil
}

// TestSpreadTplRoundTrip — снимок разворота и страницы, поставленный на тот же формат, даёт те же
// мм (±0,05): места, цвета, узоры, своё фото фона, все стикеры (лента — снова под местами),
// надпись; круг Save → Load сохраняет поставленное (ключи библиотеки находит искатель).
func TestSpreadTplRoundTrip(t *testing.T) {
	e := tplEd(t)
	tp, err := e.SpreadTplOf("s1", model.ScopeSpread, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if tp.Pages != 2 || len(tp.Places) != 3 || len(tp.Stickers) != 3 || !tp.Stickers[0].Under || len(tp.Texts) != 1 ||
		tp.Page[0].Lib != kBg || tp.Whole != nil || tp.W != 562 || tp.H != 284 {
		t.Fatalf("снимок: %s", jsonOf(t, tp))
	}
	_, plan, missing, err := e.ApplySpreadTpl("s3", model.ScopeSpread, tp, tplLibs())
	if err != nil || missing != 0 || plan.Left != 0 {
		t.Fatalf("постановка: %+v, %d, %v", plan, missing, err)
	}
	edtest.MustGrid(t, e)
	src, dst := sheetOf(e, "s1"), sheetOf(e, "s3")
	sameRects(t, dst.Grid, src.Grid)
	if dst.BgL != src.BgL || dst.BgR != src.BgR || kindsOf(dst) != kindsOf(src) || dst.BgPhotoL == nil ||
		jsonOf(t, dst.BgPhotoL) != jsonOf(t, src.BgPhotoL) {
		t.Fatalf("оформление: %s", jsonOf(t, dst))
	}
	if len(dst.Stickers) != len(src.Stickers) {
		t.Fatalf("стикеров %d, ждали %d", len(dst.Stickers), len(src.Stickers))
	}
	for _, w := range src.Stickers {
		if stickerLike(dst, w) == nil {
			t.Fatalf("нет копии стикера %+v среди %s", *w, jsonOf(t, dst.Stickers))
		}
	}
	st := dst.Stack()
	if bandAt := slices.IndexFunc(st, func(id string) bool { return id == dst.Stickers[0].ID }); dst.Stickers[0].Art == nil ||
		dst.Stickers[0].Art.Kind != "plate" || bandAt > slices.Index(st, dst.Grid[0].ID) {
		t.Fatalf("лента не под местами: стек %v, стикеры %s", st, jsonOf(t, dst.Stickers))
	}
	g, w := dst.Texts[0], src.Texts[0]
	if len(dst.Texts) != 1 || !near(g.X, w.X) || !near(g.Y, w.Y) || !near(g.W, w.W) || g.Text != w.Text || g.Style != w.Style {
		t.Fatalf("надпись %+v, ждали %+v", *g, *w)
	}

	// Страница: правая s1 → правая s4 (тот же формат).
	tr, err := e.SpreadTplOf("s1", model.ScopeRight, true, false)
	if err != nil || tr.Pages != 1 || len(tr.Places) != 2 || len(tr.Texts) != 0 || tr.Page[0].Pat == nil {
		t.Fatalf("снимок правой: %s, %v", jsonOf(t, tr), err)
	}
	if _, _, _, err := e.ApplySpreadTpl("s4", model.ScopeRight, tr, tplLibs()); err != nil {
		t.Fatal(err)
	}
	s4 := sheetOf(e, "s4")
	sameRects(t, s4.Grid, src.Grid[1:])
	if s4.BgR != "#ddeeff" || s4.PatR == nil || s4.BgL != "" || stickerLike(s4, src.Stickers[1]) == nil {
		t.Fatalf("правая s4: %s", jsonOf(t, s4))
	}

	edtest.LibFinder(t, map[string]string{kBg: edtest.Abs("lib-bg.jpg"), kSt: edtest.Abs("lib-st.png")})
	q := edtest.SaveLoad(t, e)
	ls := q.Spreads[2]
	if ls.BgPhotoL == nil || q.Photos[ls.BgPhotoL.Photo].Lib != kBg || len(ls.Stickers) != 3 || len(ls.Grid) != 3 || ls.BgR != "#ddeeff" {
		t.Fatalf("после записи и чтения: %s", jsonOf(t, ls))
	}
}

// TestSpreadTplAnchors — страница 300×300 на 200×300, 680×350 и тот же формат (плитка
// PreviewTpl — тот же путь, что правка): угловая звезда у того же угла на 0,1·S, лента-плашка
// во всю ширину у нижнего края, место — те же доли страницы, кегль надписи × S'/S.
func TestSpreadTplAnchors(t *testing.T) {
	p := model.New() // страница 300×300
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}}
	e := edtest.Open(t, p, "")
	e.SetBackground("s1", model.ScopeLeft, "#ffffff")
	_, star, _ := e.AddSticker("s1", 0, model.StickerSrc{Art: &model.Art{Kind: "star"}})
	e.SetStickerBox("s1", star, 255, 15, 30, 30, 0, "") // центр (270, 30): 0,1·S от правого и верхнего краёв
	putStickers(e, "s1", plate(0, 270, 300, 30, 0))
	e.SetText(edtest.AddText(t, e, "s1", 0), "Имя", "")
	e.AddPlace("s1", 0, "")
	tp, err := e.SpreadTplOf("s1", model.ScopeLeft, true, true)
	if err != nil {
		t.Fatal(err)
	}
	size0 := edtest.P(e).Spreads[0].Texts[0].Style.Size
	frac := tp.Places[0]
	for _, f := range [][2]float64{{300, 300}, {200, 300}, {680, 350}} {
		w, h := f[0], f[1]
		q, v, err := album.PreviewTpl(tp, w, h, nil)
		if err != nil {
			t.Fatal(err)
		}
		s := q.Spreads[0]
		k := min(w, h)
		var st, band *model.Sticker
		for _, x := range s.Stickers {
			if x.Art.Kind == "star" {
				st = x
			} else {
				band = x
			}
		}
		if st == nil || band == nil || len(v.Spreads) != 1 {
			t.Fatalf("%v×%v: стикеры %s", w, h, jsonOf(t, s.Stickers))
		}
		if cx, cy := st.X+st.W/2, st.Y+st.H/2; !near(w-cx, 0.1*k) || !near(cy, 0.1*k) || !near(st.W, 0.1*k) {
			t.Errorf("%v×%v: звезда %+v, центр (%v, %v)", w, h, *st, cx, cy)
		}
		if !near(band.X, 0) || !near(band.W, w) || !near(band.Y+band.H, h) || !near(band.H, 0.1*h) {
			t.Errorf("%v×%v: лента %+v", w, h, *band)
		}
		pl := s.Grid[0]
		if !near(pl.X, frac.X*w) || !near(pl.W, frac.W*w) || !near(pl.Y, frac.Y*h) || !near(pl.H, frac.H*h) {
			t.Errorf("%v×%v: место %+v, доли %+v", w, h, *pl, frac)
		}
		if got, want := s.Texts[0].Style.Size, model.Round1(size0*k/300); got != want {
			t.Errorf("%v×%v: кегль %v, ждали %v", w, h, got, want)
		}
	}
}

// TestApplySpreadTpl — шаблон разворота на лист с пятью фото: три места, фото встают по
// порядку, два — в ленту; стикеры и надписи цели заменены (надписи — потому что они есть в
// шаблоне), места другой страницы не тронуты у постановки на страницу; «Авто» закрепляется.
func TestApplySpreadTpl(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 60, 60, "a"), edtest.Gp(80, 12, 60, 60, "b"), edtest.Gp(150, 12, 60, 60, "c"),
		edtest.Gp(300, 12, 60, 60, "d"), edtest.Gp(380, 12, 60, 60, "f")}, []string{"g", "h", "i"})
	putStickers(e, "s1", plate(20, 200, 40, 20, 0), plate(320, 200, 40, 20, 0))
	edtest.AddText(t, e, "s1", 0)
	tp := model.SpreadTpl{V: model.TplV, Pages: 2, Page: []model.TplBg{{Color: "#101010"}, {Color: "#202020"}},
		Places:   []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.2, H: 0.3}, {X: 0.55, Y: 0.5, W: 0.2, H: 0.3}},
		Stickers: []model.TplItem{{Anchor: model.Anchor{AX: 1, AY: 1, DX: -0.1, DY: -0.1}, W: 0.1, H: 0.1, Art: &model.Art{Kind: "star"}}},
		Texts:    []model.TplText{{Anchor: model.Anchor{AY: -1, DY: 0.05}, W: 0.5, Text: "Заголовок", Style: model.PlainStyle()}},
	}
	_, plan, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil)
	if err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	s := sheetOf(e, "s1")
	if !slices.Equal(edtest.PhotosOf(e, 0), []string{"a", "b", "c"}) || !slices.Equal(s.Photos, []string{"a", "b", "c"}) {
		t.Fatalf("фото мест %v, Photos %v", edtest.PhotosOf(e, 0), s.Photos)
	}
	if plan.Left != 2 || plan.Stickers != 2 || plan.Texts != 1 || plan.Frozen != 0 {
		t.Fatalf("план %+v", plan)
	}
	if len(s.Stickers) != 1 || s.Stickers[0].Art.Kind != "star" || len(s.Texts) != 1 || s.Texts[0].Text != "Заголовок" || s.BgL != "#101010" || s.BgR != "#202020" {
		t.Fatalf("оформление: %s", jsonOf(t, s))
	}
	if _, ok := edtest.P(e).Photos["d"]; ok {
		t.Error("фото без места осталось в альбоме")
	}

	// Страница на левую «Авто»: закрепляется, правая страница — как закрепила раскладка.
	page := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{}}, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}}
	_, plan, _, err = e.ApplySpreadTpl("s2", model.ScopeLeft, page, nil)
	if err != nil || plan.Frozen != 1 {
		t.Fatalf("«Авто»: %+v, %v", plan, err)
	}
	s2 := sheetOf(e, "s2")
	if len(s2.Grid) == 0 || !near(s2.Grid[0].X, 28.1) || s2.Grid[0].Photo != "g" {
		t.Fatalf("s2: %s", jsonOf(t, s2.Grid))
	}
}

// TestPlanSpreadTpl — сухой прогон: тот же план, что даст правка, проект не тронут (ни шага, ни
// rev); ошибка области — та же, что у правки.
func TestPlanSpreadTpl(t *testing.T) {
	e := tplEd(t)
	tp, err := e.SpreadTplOf("s1", model.ScopeLeft, true, true)
	if err != nil {
		t.Fatal(err)
	}
	before, undo, rev := model.Marshal(edtest.P(e)), edtest.UndoLen(e), edtest.Rev(e)
	plan, err := e.PlanSpreadTpl("s1", model.ScopeRight, tp, tplLibs())
	if err != nil {
		t.Fatal(err)
	}
	if string(model.Marshal(edtest.P(e))) != string(before) || edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatal("сухой прогон тронул проект")
	}
	// Справа два места с фото b, c → одно место: c — в ленту; стикеры справа (звезда и лента —
	// её центр на правой) заменятся.
	if plan.Left != 1 || plan.Stickers != 2 || plan.Texts != 0 {
		t.Fatalf("план %+v", plan)
	}
	_, got, _, err := e.ApplySpreadTpl("s1", model.ScopeRight, tp, tplLibs())
	if err != nil || got != plan {
		t.Fatalf("правка %+v (%v), план %+v", got, err, plan)
	}
	two, _ := e.SpreadTplOf("s1", model.ScopeSpread, true, false)
	if _, err := e.PlanSpreadTpl("s2", model.ScopeLeft, two, nil); !errors.Is(err, grid.ErrTplScope) {
		t.Fatalf("разворот на страницу: %v", err)
	}
}

// TestSpreadTplEmpty — пустые входы: нечего снять — ErrNothing (пустой лист, только места без
// флажка, неизвестный разворот); пустой шаблон — ErrTplEmpty без шага; неизвестный разворот —
// холостая; libs = nil — свои пропущены.
func TestSpreadTplEmpty(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a")}, []string{})
	for _, c := range []struct {
		id     string
		places bool
	}{{"s2", true}, {"s1", false}, {"нет такого", true}} {
		if _, err := e.SpreadTplOf(c.id, model.ScopeSpread, c.places, true); !errors.Is(err, grid.ErrNothing) {
			t.Errorf("%s (места %v): %v", c.id, c.places, err)
		}
	}
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, model.SpreadTpl{}, nil); !errors.Is(err, model.ErrTplEmpty) {
		t.Errorf("пустой шаблон: %v", err)
	}
	ok := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{Color: "#ffffff"}}}
	if _, _, _, err := e.ApplySpreadTpl("нет такого", model.ScopeSpread, ok, nil); err != nil {
		t.Errorf("неизвестный разворот: %v", err)
	}
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatal("отказ или холостая дали шаг")
	}
	if _, _, err := album.PreviewTpl(model.SpreadTpl{}, 300, 300, nil); !errors.Is(err, model.ErrTplEmpty) {
		t.Errorf("плитка пустого шаблона: %v", err)
	}
}

// BenchmarkApplySpreadTpl — плитка шаблона со всем (места, стикеры, надпись): путь превью и правки.
func BenchmarkApplySpreadTpl(b *testing.B) {
	tp := model.SpreadTpl{V: model.TplV, Pages: 2, W: 600, H: 300, Page: []model.TplBg{{Color: "#ffffff", Pat: &model.Pattern{Kind: "dots"}}, {}}}
	for i := range 32 {
		f := float64(i%8) / 8
		tp.Places = append(tp.Places, model.Box{X: f, Y: float64(i/8) / 4, W: 0.1, H: 0.2})
		tp.Stickers = append(tp.Stickers, model.TplItem{Anchor: model.Anchor{AX: int8(i%3 - 1), DX: f / 4}, W: 0.05, H: 0.05, Art: &model.Art{Kind: "star"}})
	}
	tp.Texts = []model.TplText{{W: 0.5, Text: "Наша свадьба", Style: model.PlainStyle()}}
	for b.Loop() {
		if _, _, err := album.PreviewTpl(tp, 600, 300, nil); err != nil {
			b.Fatal(err)
		}
	}
}
