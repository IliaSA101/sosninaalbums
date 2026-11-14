package grid_test

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// Подгонка шага по страницам (165 Т1, design §2): SpreadTpl.Fit = "page".

// fitEd — пустой лист s1 с двумя страницами pw×ph мм.
func fitEd(t *testing.T, pw, ph float64) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = pw, ph
	p.Settings = model.ClampSettings(p.Settings)
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}}
	return edtest.Open(t, p, "")
}

// fitTpl — композиция 2:1 (нарисована как у генератора): места на обеих страницах, на левой
// и на правой по предмету, растяжимая лента на левой, надпись справа.
func fitTpl(fit string) looktpl.SpreadTpl {
	return looktpl.SpreadTpl{
		V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: fit, Page: []looktpl.TplBg{{Color: "#ffffff"}, {Color: "#eeeeee"}},
		Places: []model.Box{{X: 0.04, Y: 0.08, W: 0.4, H: 0.84}, {X: 0.56, Y: 0.1, W: 0.4, H: 0.4}},
		Stickers: []looktpl.TplItem{
			{Anchor: looktpl.Anchor{AX: -1, DX: 0.5, AY: 0, DY: 0}, W: 0.3, H: 0.3, Art: &model.Art{Kind: "star"}},
			{Anchor: looktpl.Anchor{AX: 1, AY: 1, DX: -0.5, DY: -0.2}, W: 0.2, H: 0.2, Rot: 10, Art: &model.Art{Kind: "star"}},
			{Anchor: looktpl.Anchor{AX: 0, AY: 1, DX: -0.25, DY: -0.05}, W: 0.5, H: 0.1, Art: &model.Art{Kind: "plate"}},
		},
		Texts: []looktpl.TplText{{Anchor: looktpl.Anchor{AX: 1, AY: 0, DX: -0.5, DY: 0.3}, W: 0.4, Text: "Имена", Style: model.PlainStyle()}},
	}
}

// fitDump — места, стикеры и надписи листа s1 после шага, мм с точностью 0,01.
func fitDump(e *album.Editor) string {
	s := edtest.P(e).Spreads[0]
	var b strings.Builder
	for _, pl := range s.Grid {
		fmt.Fprintf(&b, "P %.2f %.2f %.2f %.2f\n", pl.X, pl.Y, pl.W, pl.H)
	}
	for _, st := range s.Stickers {
		fmt.Fprintf(&b, "S %.2f %.2f %.2f %.2f %.1f\n", st.X, st.Y, st.W, st.H, st.Rot)
	}
	for _, tx := range s.Texts {
		fmt.Fprintf(&b, "T %.2f %.2f %.2f\n", tx.X, tx.Y, tx.W)
	}
	return b.String()
}

// TestLookFitLegacyUnchanged: шаги без fit — места и предметы как в 0.36 (эталон снят до правки).
func TestLookFitLegacyUnchanged(t *testing.T) {
	want := map[[2]float64]string{
		{200, 300}: "P 16.00 66.00 160.00 168.00\nP 224.00 70.00 160.00 80.00\nS 105.00 105.00 90.00 90.00 0.0\n" +
			"S 220.00 210.00 60.00 60.00 10.0\nS 0.00 270.00 200.00 30.00 0.0\nT 190.00 236.99 120.00\n",
		{400, 300}: "P 124.00 24.00 240.00 252.00\nP 436.00 30.00 240.00 120.00\nS 105.00 105.00 90.00 90.00 0.0\n" +
			"S 620.00 210.00 60.00 60.00 10.0\nS 0.00 270.00 400.00 30.00 0.0\nT 590.00 236.99 120.00\n",
	}
	for sz, w := range want {
		e := fitEd(t, sz[0], sz[1])
		tp := fitTpl("")
		tp.Places = looktpl.LookBoxes(tp, 2*sz[0]/sz[1])
		if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
			t.Fatal(err)
		}
		if got := fitDump(e); got != w {
			t.Errorf("%v:\n%s\nждали:\n%s", sz, got, w)
		}
	}
}

// fitPages — размеры страницы, мм: четыре формата из спеки 165 плюс квадрат 30×30 и 30×20.
var fitPages = [][2]float64{{200, 300}, {250, 250}, {300, 200}, {400, 300}, {300, 300}, {200, 200}}

// stickerAt — стикер шаблона с центром на якоре an.
func stickerAt(an looktpl.Anchor, w float64) looktpl.TplItem {
	return looktpl.TplItem{Anchor: an, W: w, H: w, Art: &model.Art{Kind: "star"}}
}

// applyFit ставит шаблон на лист s1 пустого альбома со страницами pw×ph.
func applyFit(t *testing.T, tp looktpl.SpreadTpl, pw, ph float64) *album.Editor {
	t.Helper()
	e := fitEd(t, pw, ph)
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	return e
}

// TestLookFitPageCentersDecor: предмет в центре левой (правой) половины композиции при
// Fit = "page" стоит в центре левой (правой) страницы и целиком в ней — на любом формате.
func TestLookFitPageCentersDecor(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Stickers: []looktpl.TplItem{
			stickerAt(looktpl.Anchor{AX: -1, AY: 0, DX: 0.5}, 0.5),        // центр левой половины
			stickerAt(looktpl.Anchor{AX: 1, AY: 0, DX: -0.5}, 0.4),        // центр правой половины
			stickerAt(looktpl.Anchor{AX: 0, AY: 0, DX: -0.5, DY: 0}, 0.6), // центр левой, крупный
		},
		Texts: []looktpl.TplText{{Anchor: looktpl.Anchor{AX: 1, AY: 0, DX: -0.5}, W: 0.4, Text: "Имена", Style: model.PlainStyle()}},
	}
	for _, sz := range fitPages {
		pw, ph := sz[0], sz[1]
		e := applyFit(t, tp, pw, ph)
		s := edtest.P(e).Spreads[0]
		if len(s.Stickers) != 3 || len(s.Texts) != 1 {
			t.Fatalf("%v: стикеров %d, надписей %d", sz, len(s.Stickers), len(s.Texts))
		}
		k := math.Min(pw, ph) // S кадра = высота композиции = min(страница.W, страница.H)
		for i, want := range []float64{pw / 2, pw + pw/2, pw / 2} {
			w := []float64{0.5, 0.4, 0.6}[i] * k // стикеры в слоях идут не в порядке шаблона: ищем по размеру
			j := slices.IndexFunc(s.Stickers, func(st *model.Sticker) bool { return math.Abs(st.W-w) < 0.1 })
			if j < 0 {
				t.Fatalf("%v: стикера шириной %.2f нет", sz, w)
			}
			st := s.Stickers[j]
			cx, cy := st.X+st.W/2, st.Y+st.H/2
			if math.Abs(cx-want) > 0.1 || math.Abs(cy-ph/2) > 0.1 {
				t.Errorf("%v: стикер %d в (%.2f; %.2f), центр страницы (%.2f; %.2f)", sz, i, cx, cy, want, ph/2)
			}
			left := float64(int(cx/pw)) * pw
			if st.X < left-0.01 || st.X+st.W > left+pw+0.01 {
				t.Errorf("%v: стикер %d [%.2f; %.2f] вышел из страницы [%.0f; %.0f]", sz, i, st.X, st.X+st.W, left, left+pw)
			}
		}
		if tx := s.Texts[0]; math.Abs(tx.X+tx.W/2-(pw+pw/2)) > 0.1 {
			t.Errorf("%v: надпись по центру %.2f, ждали %.2f", sz, tx.X+tx.W/2, pw+pw/2)
		}
	}
}

// TestLookFitPageBleed: место от края композиции до сгиба — навылет на всю страницу; место с
// полем остаётся в кадре; наклонённое не прилипает.
func TestLookFitPageBleed(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Places: []model.Box{{X: 0, Y: 0, W: 0.5, H: 1}, {X: 0.56, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.5, Y: 0, W: 0.5, H: 1, Rot: 5}}}
	for _, sz := range [][2]float64{{200, 300}, {300, 200}} {
		pw, ph := sz[0], sz[1]
		s := edtest.P(applyFit(t, tp, pw, ph)).Spreads[0]
		if len(s.Grid) != 3 {
			t.Fatalf("%v: мест %d", sz, len(s.Grid))
		}
		a := s.Grid[0]
		if a.X != 0 || a.Y != 0 || math.Abs(a.W-pw) > 0.01 || math.Abs(a.H-ph) > 0.01 {
			t.Errorf("%v: навылет %.2f %.2f %.2f %.2f, ждали 0 0 %.0f %.0f", sz, a.X, a.Y, a.W, a.H, pw, ph)
		}
		b := s.Grid[1]
		if b.X <= pw || b.X+b.W >= 2*pw || b.Y <= 0 || b.Y+b.H >= ph {
			t.Errorf("%v: место с полем %.2f %.2f %.2f %.2f упёрлось в край", sz, b.X, b.Y, b.W, b.H)
		}
		// Наклонённое: размер кадра без прилипания (кадр уже страницы по одной из осей).
		if c := s.Grid[2]; c.Rot != 5 || (math.Abs(c.W-pw) < 0.01 && math.Abs(c.H-ph) < 0.01) {
			t.Errorf("%v: наклонённое прилипло: %.2f %.2f %.2f %.2f", sz, c.X, c.Y, c.W, c.H)
		}
	}
}

// TestLookFitPagePanoramaFallback: место через сгиб — шаг подгоняется по-старому, как Fit = "".
func TestLookFitPagePanoramaFallback(t *testing.T) {
	mk := func(fit string) looktpl.SpreadTpl {
		tp := fitTpl(fit)
		tp.Places = []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.6}, {X: 0.1, Y: 0.75, W: 0.3, H: 0.2}}
		return tp
	}
	for _, sz := range fitPages {
		var dumps [2]string
		for i, fit := range []string{"", looktpl.FitPage} {
			tp := looktpl.LookTpl(mk(fit), 2*sz[0]/sz[1])
			dumps[i] = fitDump(applyFit(t, tp, sz[0], sz[1]))
		}
		if dumps[0] != dumps[1] {
			t.Errorf("%v: с fit\n%s\nбез fit\n%s", sz, dumps[1], dumps[0])
		}
	}
}

// TestLookFitPageNoDoubleFit: ApplySpreadTpl без LookBoxes (генератор) и через LookBoxes
// (альбом, плитка) дают один лист — места при fit подгоняются один раз.
func TestLookFitPageNoDoubleFit(t *testing.T) {
	for _, sz := range fitPages {
		a, b := fitTpl(looktpl.FitPage), fitTpl(looktpl.FitPage)
		b = looktpl.LookTpl(b, 2*sz[0]/sz[1])
		if da, db := fitDump(applyFit(t, a, sz[0], sz[1])), fitDump(applyFit(t, b, sz[0], sz[1])); da != db {
			t.Errorf("%v: без LookBoxes\n%s\nс LookBoxes\n%s", sz, da, db)
		}
	}
}

// TestLookFitPageMirrorsAndOrder: места двух страниц зеркальны у зеркальной композиции, порядок
// мест — порядок шаблона, несколько предметов на странице стоят каждый на своей.
func TestLookFitPageMirrorsAndOrder(t *testing.T) {
	tp := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{}, {}},
		Places: []model.Box{{X: 0.56, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.04, Y: 0.1, W: 0.4, H: 0.8}}}
	for _, sz := range fitPages {
		pw := sz[0]
		s := edtest.P(applyFit(t, tp, pw, sz[1])).Spreads[0]
		r, l := s.Grid[0], s.Grid[1] // порядок шаблона: сначала правое
		if r.X <= pw || l.X+l.W >= pw {
			t.Fatalf("%v: порядок мест нарушен: %+v %+v", sz, r, l)
		}
		if math.Abs((l.X+l.W/2)-(2*pw-(r.X+r.W/2))) > 0.02 || math.Abs(l.W-r.W) > 0.02 {
			t.Errorf("%v: страницы не зеркальны: л %.2f+%.2f п %.2f+%.2f", sz, l.X, l.W, r.X, r.W)
		}
	}
}

// TestLookFitPageUndoAndNoop: постановка — один шаг отмены, повтор — холостая правка (без шага),
// Undo возвращает пустой лист, Marshal/Unmarshal листа с шагом круг не меняет.
func TestLookFitPageUndoAndNoop(t *testing.T) {
	tp := fitTpl(looktpl.FitPage)
	e := fitEd(t, 200, 300)
	before := string(model.Marshal(edtest.P(e)))
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	if n := edtest.UndoLen(e); n != 1 {
		t.Fatalf("шагов отмены %d, ждали 1", n)
	}
	rev := edtest.Rev(e)
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != 1 || edtest.Rev(e) != rev {
		t.Errorf("повтор шага не холостая: отмен %d, rev %d → %d", edtest.UndoLen(e), rev, edtest.Rev(e))
	}
	saved := model.Marshal(edtest.P(e))
	back := model.Unmarshal(saved)
	if string(model.Marshal(back)) != string(saved) {
		t.Error("круг Marshal → Unmarshal изменил лист с шагом")
	}
	e.Undo()
	if got := string(model.Marshal(edtest.P(e))); got != before {
		t.Errorf("после отмены лист не пустой:\n%s", got)
	}
}

// TestLookFitPageEmpty: шаг без мест и без предметов, один предмет, шаг только с надписью —
// подгонка не падает на краях.
func TestLookFitPageEmpty(t *testing.T) {
	base := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Fit: looktpl.FitPage, Page: []looktpl.TplBg{{Color: "#ffffff"}, {}}}
	only := base
	only.Texts = []looktpl.TplText{{Anchor: looktpl.Anchor{AX: -1, DX: 0.5}, W: 0.3, Text: "Имена", Style: model.PlainStyle()}}
	one := base
	one.Places = []model.Box{{X: 0.6, Y: 0.1, W: 0.3, H: 0.8}}
	for name, tp := range map[string]looktpl.SpreadTpl{"фон": base, "надпись": only, "одно место": one} {
		for _, sz := range fitPages {
			e := fitEd(t, sz[0], sz[1])
			if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
				t.Errorf("%s %v: %v", name, sz, err)
			}
		}
	}
}
