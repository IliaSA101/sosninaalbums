package grid_test

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Правки мест у «Авто» по ссылке ref (спека 008, modeless.md, G14).

func TestSetPlacesFreezesAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	s := edtest.P(e).Spreads[0]
	key, rects := s.Layout, e.View().Spreads[0].Rects
	undo := edtest.UndoLen(e)
	// ref = ID фото: у «Авто» мест ещё нет.
	e.SetPlaces(id, []model.PlaceBox{{ID: "a", X: rects[0].X + 3, Y: rects[0].Y, W: rects[0].W, H: rects[0].H}}, "")
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != 3 || edtest.UndoLen(e) != undo+1 || edtest.P(e).Spreads[0].Layout != key {
		t.Fatalf("мест %d, шагов %d, ключ %q → %q", len(g), edtest.UndoLen(e)-undo, key, edtest.P(e).Spreads[0].Layout)
	}
	if g[0].Photo != "a" || math.Abs(g[0].X-rects[0].X-3) > 0.01 {
		t.Fatalf("первое место: %+v", *g[0])
	}
	for i := 1; i < 3; i++ {
		if !nearMM(rectOfPlace(g[i]), rects[i]) {
			t.Fatalf("место %d: %+v, ячейка %+v", i, *g[i], rects[i])
		}
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Grid != nil || edtest.P(e).Spreads[0].Layout != key {
		t.Fatal("один Undo не вернул «Авто» с прежним ключом")
	}
}

func TestSetPlacesAutoNoop(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rects := e.View().Spreads[0].Rects
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	r := rects[1]
	e.SetPlaces(id, []model.PlaceBox{{ID: "b", X: r.X, Y: r.Y, W: r.W, H: r.H}}, "")
	edtest.Same(t, e, rev, undo, "рамка = ячейка")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая правка закрепила разворот")
	}
	e.SetPlaces(id, []model.PlaceBox{{ID: "нет такого", X: 1, Y: 1, W: 50, H: 50}}, "")
	e.SetPlaces(id, nil, "")
	edtest.Same(t, e, rev, undo, "неизвестный ref")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("неизвестный ref закрепил разворот")
	}
}

func TestSetPlacesMergeBack(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")})
	pl := *edtest.P(e).Spreads[0].Grid[0]
	undo := edtest.UndoLen(e)
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: pl.X + 1, Y: pl.Y, W: pl.W, H: pl.H}}, "sel-1")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d", edtest.UndoLen(e)-undo)
	}
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}}, "sel-1")
	if edtest.UndoLen(e) != undo || edtest.P(e).Spreads[0].Grid[0].X != pl.X {
		t.Fatalf("серия вернулась к началу, а шагов %d", edtest.UndoLen(e)-undo)
	}
}

// У «Авто» сессия не сливается с прежней серией: закрепление — свой шаг.
func TestSetPlacesAutoIgnoresMerge(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a")}, []string{"x", "y"})
	pl := *edtest.P(e).Spreads[0].Grid[0]
	e.SetPlaces("s1", []model.PlaceBox{{ID: pl.ID, X: pl.X + 1, Y: pl.Y, W: pl.W, H: pl.H}}, "sel-1")
	undo := edtest.UndoLen(e)
	e.SetPlaces("s2", []model.PlaceBox{{ID: "x", X: 20, Y: 20, W: 80, H: 80}}, "sel-1")
	if edtest.UndoLen(e) != undo+1 || edtest.P(e).Spreads[1].Grid == nil {
		t.Fatalf("шагов %d, сетка %v", edtest.UndoLen(e)-undo, edtest.P(e).Spreads[1].Grid)
	}
	e.Undo()
	if edtest.P(e).Spreads[1].Grid != nil || edtest.P(e).Spreads[0].Grid[0].X != pl.X+1 {
		t.Fatal("Undo задел чужую серию")
	}
}

func TestArrangeAutoFreeze(t *testing.T) {
	for _, op := range []string{"splitH", "splitV", "same", "sameW"} {
		e, id := edtest.AutoEd(t)
		undo := edtest.UndoLen(e)
		ids := []string{"a", "b"}
		if _, err := e.ArrangePlaces(id, op, ids); err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		edtest.MustGrid(t, e)
		want := 3
		if op == "splitH" || op == "splitV" {
			want = 5
		}
		if got := len(edtest.P(e).Spreads[0].Grid); got != want || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%s: мест %d (ждали %d), шагов %d", op, got, want, edtest.UndoLen(e)-undo)
		}
		e.Undo()
		if edtest.P(e).Spreads[0].Grid != nil {
			t.Fatalf("%s: Undo не вернул «Авто»", op)
		}
	}
	// front/back у «Авто» ничего не перекрывают: холостые.
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ArrangePlaces(id, "front", []string{"a"})
	e.ArrangePlaces(id, "back", []string{"b"})
	e.ArrangePlaces(id, "nope", []string{"b"})
	e.ArrangePlaces(id, "same", []string{"нет такого"})
	edtest.Same(t, e, rev, undo, "front/back/nope")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая операция закрепила разворот")
	}
}

// Отказ операции (например, место слишком мало) не оставляет закрепления.
func TestWithGridErrorRollsBack(t *testing.T) {
	e, _ := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, err := edtest.Try(e, "", func(p *model.Project) error {
		return p.WithGrid(p.Spreads[0], func() error { return grid.ErrSplitSmall })
	})
	if err != grid.ErrSplitSmall {
		t.Fatalf("ошибка %v", err)
	}
	edtest.Same(t, e, rev, undo, "отказ")
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("после отказа остался закреплённый разворот")
	}
}

func TestPlaceFitDeleteAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceFit(id, []string{"a"}, "")
	e.DeletePlaces(id, []string{"a"})
	e.SetPlaceFit(id, []string{"a"}, "что-то")
	edtest.Same(t, e, rev, undo, `fit "", DeletePlaces, мусор`)
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("холостая правка закрепила разворот")
	}
	e.SetPlaceFit(id, []string{"a"}, model.FitContain)
	edtest.MustGrid(t, e)
	g := edtest.P(e).Spreads[0].Grid
	if len(g) != 3 || g[0].Fit != model.FitContain || g[1].Fit != "" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("contain: мест %d, шагов %d", len(g), edtest.UndoLen(e)-undo)
	}
}

func TestPlaceRefResolve(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(142, 12, 100, 100, ""), edtest.Gp(12, 150, 100, 100, "b")}, []string{"x"})
	s := edtest.P(e).Spreads[0]
	g := s.Grid
	if s.PlaceRef(g[0].ID) != g[0] || s.PlaceRef("a") != g[0] || s.PlaceRef("b") != g[2] {
		t.Fatal("ссылка по ID места и по фото")
	}
	if s.PlaceRef(g[1].ID) != g[1] {
		t.Fatal("пустое место по своему ID")
	}
	if s.PlaceRef("") != nil || s.PlaceRef("x") != nil || s.PlaceRef("нет") != nil {
		t.Fatal("пустая, чужая и неизвестная ссылки должны пропускаться")
	}
	// ID места важнее фото: фото с ID чужого места не перехватывает ссылку.
	g[2].Photo = g[0].ID
	if s.PlaceRef(g[0].ID) != g[0] {
		t.Fatal("ID места должен быть важнее")
	}
	g[2].Photo = "b"
	// Операции по ссылке на фото действуют на сетке так же.
	e.ArrangePlaces("s1", "sameW", []string{"a", "b"})
	edtest.MustGrid(t, e)
	e.SetPlaceFit("s1", []string{"b"}, model.FitContain)
	if edtest.P(e).Spreads[0].Grid[2].Fit != model.FitContain || edtest.P(e).Spreads[0].Grid[0].Fit != "" {
		t.Fatal("SetPlaceFit по фото")
	}
	// Фото другого разворота пропускается.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetPlaceFit("s1", []string{"x"}, model.FitContain)
	e.DeletePlaces("s1", []string{"x", "нет"})
	edtest.Same(t, e, rev, undo, "чужое фото")
}

// «Удалить» у фото на сетке оставляет место пустым (ответ пользователя, п. 3).
func TestRemovePhotoKeepsPlace(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, "a"), edtest.Gp(142, 12, 100, 100, "b")})
	e.RemovePhotos([]string{"a"})
	edtest.MustGrid(t, e)
	if len(edtest.P(e).Spreads[0].Grid) != 2 || !slices.Equal(edtest.PhotosOf(e, 0), []string{"", "b"}) {
		t.Fatalf("места %v", edtest.PhotosOf(e, 0))
	}
	// DeletePlaces по ссылке на фото: место уходит, фото — в ленту.
	e.DeletePlaces("s1", []string{"b"})
	if len(edtest.P(e).Spreads[0].Grid) != 1 || edtest.HasRef(edtest.P(e), "b") {
		t.Fatalf("места %v", edtest.PhotosOf(e, 0))
	}
}

func nearMM(a, b layout.Rect) bool {
	const eps = 0.01
	return math.Abs(a.X-b.X) <= eps && math.Abs(a.Y-b.Y) <= eps && math.Abs(a.W-b.W) <= eps && math.Abs(a.H-b.H) <= eps
}

func rectOfPlace(pl *model.Place) layout.Rect {
	return layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}
}

// «Закрепить» не меняет картинку: место = ячейка, фото целиком (cover по кадру)
// то же, что в «Авто», в том числе у ячейки «в край».
func TestFreezeKeepsPicture(t *testing.T) {
	cases := []struct {
		name string
		ph   []model.PhotoInfo
		key  func(e *album.Editor) string
	}{
		{"лучшая", []model.PhotoInfo{edtest.Photo("a"), {ID: "b", Path: "/b.jpg", W: 200, H: 300}, edtest.Photo("c")}, func(*album.Editor) string { return "" }},
		{"в край", edtest.Photos("a"), func(e *album.Editor) string {
			for _, r := range edtest.RowsOf(e, 0) {
				if strings.HasPrefix(r.Key, "B[") {
					return r.Key
				}
			}
			return ""
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := edtest.Open(t, model.New(), "")
			v := edtest.NewSpread(e, -1, c.ph)
			if k := c.key(e); k != "" {
				v = e.SetLayout(v.Spreads[0].ID, k)
			} else if c.name == "в край" {
				t.Fatal("нет раскладки «в край»")
			}
			v = e.SetFrame("a", 1.4, 0.5, -0.3)
			before := v.Spreads[0]
			if c.name == "в край" && !strings.HasPrefix(before.Layout, "B[") {
				t.Fatalf("раскладка %q не «в край»", before.Layout)
			}
			layoutKey := edtest.P(e).Spreads[0].Layout
			edtest.FreezeAt(t, e, 0)
			s := edtest.P(e).Spreads[0]
			if s.Layout != layoutKey || len(s.Grid) != len(before.Rects) {
				t.Fatalf("ключ %q → %q, мест %d", layoutKey, s.Layout, len(s.Grid))
			}
			for i, pl := range s.Grid {
				if pl.Photo != before.Photos[i] || !nearMM(rectOfPlace(pl), before.Rects[i]) {
					t.Fatalf("место %d: %+v, ячейка %+v (%s)", i, *pl, before.Rects[i], before.Photos[i])
				}
				img := model.PlaceImage(rectOfPlace(pl), edtest.P(e).Aspects([]string{pl.Photo})[0], edtest.P(e).Photos[pl.Photo].FrameOrDefault())
				if !nearMM(img, before.Frames[i].Img) {
					t.Fatalf("фото %s: %+v, было %+v", pl.Photo, img, before.Frames[i].Img)
				}
			}
		})
	}
}

// «Вернуть авто» возвращает сохранённый ключ и ту же картинку.
func TestUnfreezeRestoresKey(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, []model.PhotoInfo{edtest.Photo("a"), {ID: "b", Path: "/b.jpg", W: 200, H: 300}, edtest.Photo("c")})
	key := edtest.PickKey(t, e, 0, edtest.AnyKey)
	before := e.SetLayout(v.Spreads[0].ID, key).Spreads[0]
	edtest.FreezeAt(t, e, 0)
	after := edtest.Change(e, func(p *model.Project) { p.Unfreeze(p.Spreads[0]) }).Spreads[0]
	if edtest.P(e).Spreads[0].Grid != nil || edtest.P(e).Spreads[0].Layout != key || after.Layout != key {
		t.Fatalf("ключ %q, ждали %q", edtest.P(e).Spreads[0].Layout, key)
	}
	for i := range before.Rects {
		if !nearMM(before.Rects[i], after.Rects[i]) || !nearMM(before.Frames[i].Img, after.Frames[i].Img) {
			t.Fatalf("ячейка %d: %+v → %+v", i, before.Rects[i], after.Rects[i])
		}
	}
}

// Лучшая хранится как "": ключ лучшей и ключ, который больше не строится, — "".
func TestUnfreezeBestIsEmptyKey(t *testing.T) {
	for _, key := range []string{"best", "H(0,1,2,3)"} {
		e := edtest.Open(t, model.New(), "")
		edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
		edtest.FreezeAt(t, e, 0)
		edtest.Change(e, func(p *model.Project) {
			s := p.Spreads[0]
			s.Layout = key
			if key == "best" {
				s.Layout = edtest.RowsOf(e, 0)[0].Key
			}
			p.Unfreeze(s)
		})
		if s := edtest.P(e).Spreads[0]; s.Layout != "" || s.Grid != nil {
			t.Fatalf("%s: ключ %q, сетка %v", key, s.Layout, s.Grid)
		}
	}
}

// Перенос между местами одного разворота: одно вхождение, старое место пустое,
// кадр и подпись целы.
func TestPlaceMoveWithinGrid(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	e.SetFrame("a", 2, 0.5, 0)
	e.SetCaption("a", "Первый танец", "")
	edtest.FreezeAt(t, e, 0)
	var dst *model.Place
	edtest.Change(e, func(p *model.Project) {
		var err error
		if dst, err = p.AddPlace(p.Spreads[0], 1, 1.5, p.PlaceIDs()); err != nil {
			t.Fatal(err)
		}
	})
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		p.Put(s, s.Grid[len(s.Grid)-1], "a")
	})
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 3 || s.Grid[0].Photo != "" || s.Grid[2].Photo != "a" || s.Grid[2].ID != dst.ID || !slices.Equal(s.Photos, []string{"b", "a"}) {
		t.Fatalf("места %+v %+v %+v, фото %v", *s.Grid[0], *s.Grid[1], *s.Grid[2], s.Photos)
	}
	if r := edtest.P(e).Photos["a"]; r.Frame == nil || r.Frame.S != 2 || r.Caption == nil {
		t.Fatalf("кадр или подпись потеряны: %+v", r)
	}
	// Замена: в занятое место — фото из ленты, прежний хозяин уходит из альбома.
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		p.Photos["c"] = model.PhotoRef{Path: "/c.jpg", W: 3, H: 2}
		p.Put(s, s.Grid[1], "c")
	})
	if s := edtest.P(e).Spreads[0]; !slices.Equal(s.Photos, []string{"c", "a"}) || edtest.HasRef(edtest.P(e), "b") {
		t.Fatalf("после замены: %v, b в записях: %v", s.Photos, edtest.HasRef(edtest.P(e), "b"))
	}
}

// Мест не больше MaxPlaces; без пустого места фото не садится (errNoPlace, правка откатывается).
func TestPlacesLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.FreezeAt(t, e, 0)
	rev := e.View().Rev
	v, err := edtest.Try(e, "", func(p *model.Project) error {
		s := p.Spreads[0]
		for len(s.Grid) < model.MaxPlaces {
			if _, err := p.AddPlace(s, len(s.Grid)%2, 1, p.PlaceIDs()); err != nil {
				return err
			}
		}
		_, err := p.AddPlace(s, 0, 1, p.PlaceIDs())
		return err
	})
	if err == nil || len(edtest.P(e).Spreads[0].Grid) != 1 || v.Rev != rev {
		t.Fatalf("17-е место: ошибка %v, мест %d, rev %d", err, len(edtest.P(e).Spreads[0].Grid), v.Rev)
	}
	// Лесенка: новые места не ложатся друг на друга и стоят на своей странице.
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		for range 3 {
			if _, err := p.AddPlace(s, 1, 0.5, p.PlaceIDs()); err != nil {
				t.Fatal(err)
			}
		}
	})
	g := edtest.P(e).Spreads[0].Grid
	for i := 1; i < len(g); i++ {
		if g[i].X < edtest.P(e).Settings.PageW || i > 1 && g[i].X == g[i-1].X {
			t.Fatalf("место %d: %+v", i, *g[i])
		}
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	// Все места заняты — фото некуда сесть.
	_, err = edtest.Try(e, "", func(p *model.Project) error {
		s := p.Spreads[0]
		for i := range len(s.Grid) {
			id := string(rune('k' + i))
			p.Photos[id] = model.PhotoRef{Path: "/" + id, W: 1, H: 1}
			s.Photos = append(s.Photos, id)
		}
		return p.SyncGrid(false)
	})
	if !errors.Is(err, model.ErrNoPlace) || len(edtest.P(e).Spreads[0].Photos) != 1 {
		t.Fatalf("ошибка %v, фото %v", err, edtest.P(e).Spreads[0].Photos)
	}
}

// Сетка, из которой ушло последнее фото, остаётся: пустые места — заготовка.
func TestGridSurvivesEmpty(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.FreezeAt(t, e, 0)
	v := e.RemovePhotos([]string{"a"})
	if len(v.Spreads) != 1 || len(edtest.P(e).Spreads[0].Grid) != 1 || edtest.P(e).Spreads[0].Grid[0].Photo != "" || len(edtest.P(e).Spreads[0].Photos) != 0 {
		t.Fatalf("развороты %d, сетка %+v", len(v.Spreads), edtest.P(e).Spreads[0].Grid)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Перепривязка переводит ID и в местах: P1=A P2="" P3=B остаются на своих местах.
func TestRelinkKeepsPlaces(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	edtest.FreezeAt(t, s.E, 0)
	edtest.Change(s.E, func(p *model.Project) {
		sp := p.Spreads[0]
		pl, _ := p.AddPlace(sp, 1, 1, p.PlaceIDs())
		sp.Grid = []*model.Place{sp.Grid[0], pl, sp.Grid[1]}
	})
	s.E.Relink(map[string]string{s.IdA: s.NewA, s.IdB: s.NewB}, "")
	g := edtest.P(s.E).Spreads[0].Grid
	nA, nB := library.IDFor(s.NewA), library.IDFor(s.NewB)
	if len(g) != 3 || g[0].Photo != nA || g[1].Photo != "" || g[2].Photo != nB {
		t.Fatalf("места: %+v %+v %+v", *g[0], *g[1], *g[2])
	}
	if err := edtest.GridInvariants(edtest.P(s.E)); err != nil {
		t.Fatal(err)
	}
}
