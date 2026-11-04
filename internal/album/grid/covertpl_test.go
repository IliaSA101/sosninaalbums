package grid_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ApplyLookCover (131 §5): пустые и негодные входы — холостая правка, не паника; места сторонки
// в безопасной зоне ± 0,01 мм; шаблон разворота (Pages 2) на сторонку не встаёт.
func TestApplyLookCoverGrid(t *testing.T) {
	c := fsfera(t)
	apply := func(ct *model.CoverTpl) (res grid.CoverRes) {
		t.Helper()
		_, err := edtest.Try(c.e, "", func(p *model.Project) (err error) {
			res, err = grid.ApplyLookCover(p, ct, nil)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	rev := edtest.Rev(c.e)
	two := &model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}}, Page: []model.TplBg{{}, {}}}
	for name, ct := range map[string]*model.CoverTpl{"nil": nil, "пустой": {}, "Pages 2": {Front: two}, "без мест и фона": {Back: &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}}}}} {
		if r := apply(ct); r.Changed || r.Left != 0 {
			t.Fatalf("%s: %+v", name, r)
		}
	}
	if edtest.Rev(c.e) != rev {
		t.Fatal("негодные входы изменили проект")
	}
	side := &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}},
		Places: []model.Box{{X: 0, Y: 0, W: 0.5, H: 1}, {X: 0.5, Y: 0, W: 0.5, H: 1}, {X: 0.25, Y: 0.25, W: 0.5, H: 0.5}}}
	if r := apply(&model.CoverTpl{Front: side}); !r.Changed {
		t.Fatalf("первый раз: %+v", r)
	}
	z := c.box(model.ScopeRight)
	got := c.places(model.ScopeRight)
	boxes := model.LookBoxes(*side, z.W/z.H)
	if len(got) != 3 {
		t.Fatalf("мест %d", len(got))
	}
	for i, b := range boxes {
		pl := got[i]
		if d := pl.X - (z.X + b.X*z.W); d < -0.011 || d > 0.011 || pl.X < z.X-0.011 || pl.X+pl.W > z.X+z.W+0.011 || pl.Y+pl.H > z.Y+z.H+0.011 {
			t.Fatalf("место %d вне зоны: %+v, зона %+v", i, pl, z)
		}
	}
	if r := apply(&model.CoverTpl{Front: side}); r.Changed {
		t.Fatal("повтор должен быть холостым")
	}
}

// Обложки нет — ничего не создаётся.
func TestApplyLookCoverNoCover(t *testing.T) {
	p := model.New()
	res, err := grid.ApplyLookCover(p, &model.CoverTpl{Bg: &model.TplBg{Color: "#112233"}}, nil)
	if err != nil || res.Changed || p.Cover != nil {
		t.Fatalf("%+v %v %v", res, err, p.Cover)
	}
}

// Шаблон страницы на обычный разворот при существующей обложке встаёт по странице, а не по
// безопасной зоне сторонки: места те же, что в альбоме без обложки.
func TestApplySpreadTplPageIgnoresCover(t *testing.T) {
	tpl := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}},
		Places:   []model.Box{{X: 0.1, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.35, H: 0.8}},
		Stickers: []model.TplItem{{Anchor: model.Anchor{AX: 1, AY: 1}, W: 0.1, H: 0.1, Art: &model.Art{Kind: "star"}}}}
	placed := func(c *cv) [][4]float64 {
		t.Helper()
		id := edtest.P(c.e).Spreads[0].ID
		if _, err := edtest.Try(c.e, "", func(p *model.Project) (err error) {
			_, _, err = grid.ApplySpreadTpl(p, id, model.ScopeLeft, tpl, nil)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		var out [][4]float64
		for _, pl := range edtest.P(c.e).Spreads[0].Grid {
			if pl.X < edtest.P(c.e).Settings.PageW {
				out = append(out, [4]float64{pl.X, pl.Y, pl.W, pl.H})
			}
		}
		st := edtest.P(c.e).Spreads[0].Stickers // угловой стикер: его место зависит от области шаблона
		if len(st) != 1 {
			t.Fatalf("стикеров %d", len(st))
		}
		return append(out, [4]float64{st[0].X, st[0].Y, st[0].W, st[0].H})
	}
	with, without := fsfera(t), fsfera(t)
	without.e.DeleteCover()
	a, b := placed(with), placed(without)
	if len(a) != 3 || len(a) != len(b) || a[0] != b[0] || a[1] != b[1] || a[2] != b[2] {
		t.Fatalf("с обложкой %v, без неё %v", a, b)
	}
}

// Фото фона всего листа под сторонкой не делает повтор правкой: Changed = false (сверка видит
// фото листа у сторонки как своё).
func TestApplyLookCoverWholePhotoRepeat(t *testing.T) {
	const key = "my:00000000000000ee"
	libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("w.jpg"), W: 300, H: 200, Lib: key}}
	side := &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}}, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}}
	ct := &model.CoverTpl{Bg: &model.TplBg{Lib: key}, Front: side}
	c := fsfera(t)
	apply := func() (res grid.CoverRes) {
		t.Helper()
		if _, err := edtest.Try(c.e, "", func(p *model.Project) (err error) {
			res, err = grid.ApplyLookCover(p, ct, libs)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return res
	}
	if r := apply(); !r.Changed || edtest.P(c.e).Cover.Sheet.BgPhoto == nil {
		t.Fatalf("первый раз: %+v", r)
	}
	if r := apply(); r.Changed || r.Own != 0 || edtest.P(c.e).Cover.Sheet.BgPhoto == nil {
		t.Fatalf("повтор: %+v", r)
	}
}

// CoverRes.Own считает заменённое по видам: места, только надписи, только стикеры.
func TestApplyLookCoverOwnCount(t *testing.T) {
	own := func(c *cv, text, sticker bool) {
		edtest.Change(c.e, func(p *model.Project) {
			z := c.box(model.ScopeRight)
			s := p.Cover.Sheet
			s.Texts = nil // свои надписи CoverEd — вне подсчёта
			if text {
				s.Texts = append(s.Texts, &model.TextBlock{ID: "c0ffee0000b3", X: z.X + 5, Y: z.Y + z.H/2, W: 40, Text: "своя", Style: model.PlainStyle()})
			}
			if sticker {
				s.Stickers = append(s.Stickers, &model.Sticker{ID: "c0ffee0000b4", X: z.X + z.W/2, Y: z.Y + 5, W: 10, H: 10, Art: &model.Art{Kind: "star"}})
			}
			s.Grid = nil
		})
	}
	draft := []model.TplText{{Anchor: model.Anchor{AY: 1, DY: -0.1}, W: 0.5, Text: "[Имена]", Style: model.PlainStyle()}}
	star := []model.TplItem{{W: 0.1, H: 0.1, Art: &model.Art{Kind: "star"}}}
	run := func(c *cv, side model.SpreadTpl) grid.CoverRes {
		var res grid.CoverRes
		if _, err := edtest.Try(c.e, "", func(p *model.Project) (err error) {
			res, err = grid.ApplyLookCover(p, &model.CoverTpl{Front: &side}, nil)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return res
	}
	base := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}}}
	c := fsfera(t)
	own(c, true, true)
	withText := base
	withText.Drafts = draft
	if r := run(c, withText); r.Texts != 1 || r.Stickers != 1 || r.Own != 2 {
		t.Fatalf("надпись и стикер: %+v", r)
	}
	c = fsfera(t)
	own(c, true, false)
	if r := run(c, withText); r.Texts != 1 || r.Stickers != 0 || r.Own != 1 {
		t.Fatalf("только надпись: %+v", r)
	}
	c = fsfera(t)
	own(c, false, true)
	withStar := base
	withStar.Stickers = star
	if r := run(c, withStar); r.Texts != 0 || r.Stickers != 1 || r.Own != 1 {
		t.Fatalf("только стикер: %+v", r)
	}
	// Места: своё место заменяется — 1; те же места повтор не пересоздаёт.
	c = fsfera(t)
	own(c, false, false)
	edtest.Change(c.e, func(p *model.Project) {
		z := c.box(model.ScopeRight)
		p.Cover.Sheet.Grid = []*model.Place{{ID: "c0ffee0000b5", X: z.X, Y: z.Y, W: 20, H: 20}}
	})
	withPlace := base
	withPlace.Places = []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}
	if r := run(c, withPlace); r.Own != 1 || !r.Changed {
		t.Fatalf("своё место: %+v", r)
	}
	if r := run(c, withPlace); r.Own != 0 || r.Changed {
		t.Fatalf("повтор: %+v", r)
	}
}

// Макет без фона обложки её фон не трогает: ApplyLookCoverBg холостая, LookCoverBgSame — true.
func TestLookCoverBgWithoutBg(t *testing.T) {
	c := fsfera(t)
	edtest.Change(c.e, func(p *model.Project) { p.Cover.Sheet.BgL, p.Cover.Sheet.BgR = "#abcdef", "#fedcba" })
	ct := &model.CoverTpl{Front: &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}},
		Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}}}
	p := edtest.P(c.e)
	if r := grid.ApplyLookCoverBg(p, ct, nil); r.Changed || p.Cover.Sheet.BgL != "#abcdef" || p.Cover.Sheet.BgR != "#fedcba" {
		t.Fatalf("фон изменён: %+v %q %q", r, p.Cover.Sheet.BgL, p.Cover.Sheet.BgR)
	}
	if !grid.LookCoverBgSame(p, ct, nil) {
		t.Fatal("макет без фона: фон «тот же»")
	}
	if grid.LookCoverBgSame(p, &model.CoverTpl{Bg: &model.TplBg{Color: "#000000"}}, nil) {
		t.Fatal("другой фон: не тот же")
	}
}

// CoverExtras: нет макета или обложки — нет и «своего»; сторона без шаблона не сверяется.
func TestCoverExtrasEdges(t *testing.T) {
	p := edtest.P(fsfera(t).e)
	if grid.CoverExtras(p, nil) || grid.CoverExtras(model.New(), &model.CoverTpl{}) {
		t.Fatal("нет макета или обложки — «своего» нет")
	}
	if grid.CoverExtras(p, &model.CoverTpl{Bg: &model.TplBg{Color: "#112233"}}) {
		t.Fatal("сторон в макете нет — сверять нечего")
	}
}
