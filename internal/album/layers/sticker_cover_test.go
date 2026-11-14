package layers_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// coverStickerEd — редактор с обложкой, у которой заданы заворот и расстав (мм).
func coverStickerEd(t *testing.T, wrap, hinge float64) *model.Project {
	t.Helper()
	e := edtest.CoverEd(t, "k", edtest.Calendar(), []string{"x"})
	edtest.Change(e, func(p *model.Project) {
		p.Cover.Spec.Wrap, p.Cover.Spec.Hinge = wrap, hinge
	})
	return edtest.P(e)
}

// TestAddStickerCoverClickSame — эталон щелчка человека по обложке (154 Т1, числа сняты с кода ДО
// рефакторинга): центр и размер стикера — от страницы листа, то есть сторонки целиком вместе с
// заворотом и расставом. Агентские стикеры на обложку (addStickerIn) эти числа менять не вправе.
func TestAddStickerCoverClickSame(t *testing.T) {
	cases := []struct {
		wrap, hinge  float64
		page         int
		cx, cy, w, h float64
	}{
		{0, 0, 0, 140.505, 142.005, 70.25, 70.25},
		{0, 0, 1, 427.505, 142.005, 70.25, 70.25},
		{12, 0, 0, 146.505, 154.005, 73.25, 73.25},
		{12, 0, 1, 445.505, 154.005, 73.25, 73.25},
		{12, 6, 0, 149.505, 154.005, 74.75, 74.75},
		{12, 6, 1, 454.505, 154.005, 74.75, 74.75},
		{0, 6, 0, 143.505, 142.005, 71.75, 71.75},
		{0, 6, 1, 436.505, 142.005, 71.75, 71.75},
	}
	for _, c := range cases {
		p := coverStickerEd(t, c.wrap, c.hinge)
		if err := layers.AddSticker(p, model.CoverID, c.page, "st", model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
			t.Fatal(err)
		}
		st := p.Cover.Sheet.Stickers[0]
		cx, cy := center(st)
		if !near(cx, c.cx) || !near(cy, c.cy) || !near(st.W, c.w) || !near(st.H, c.h) {
			t.Errorf("заворот %v расстав %v сторонка %d: центр (%v, %v) %v×%v, ждали (%v, %v) %v×%v",
				c.wrap, c.hinge, c.page, cx, cy, st.W, st.H, c.cx, c.cy, c.w, c.h)
		}
	}
}

// inBox — рамка стикера внутри зоны (допуск на округление до сотых).
func inBox(st *model.Sticker, z layout.Rect) bool {
	const eps = 0.011
	return st.X >= z.X-eps && st.Y >= z.Y-eps && st.X+st.W <= z.X+z.W+eps && st.Y+st.H <= z.Y+z.H+eps
}

var coverStar = model.StickerSrc{Art: &model.Art{Kind: "star"}}

func coverPut(page int, anchor, size string) layers.StickerPut {
	return layers.StickerPut{Spread: model.CoverID, Page: page, Src: coverStar, Anchor: anchor, Size: size}
}

// coverSpecs — обложка без заворота и с заворотом и расставом (мм).
var coverSpecs = [][2]float64{{0, 0}, {12, 6}}

// TestAddStickerAtCoverCorner: пачка агента на обложку — все пять мест обеих сторонок, всех
// размеров, внутри Box; углы стоят вплотную к углам Box (без отступа: зона уже безопасная).
func TestAddStickerAtCoverCorner(t *testing.T) {
	anchors := []string{layers.AnchorCenter, layers.AnchorTopLeft, layers.AnchorTopRight, layers.AnchorBottomLeft, layers.AnchorBottomRight}
	for _, v := range coverSpecs {
		p := coverStickerEd(t, v[0], v[1])
		sides := p.CoverSides()
		if len(sides) != 2 {
			t.Fatalf("сторонок %d", len(sides))
		}
		var puts []layers.StickerPut
		for page := range 2 {
			for _, a := range anchors {
				for _, size := range []string{"s", "m", "l"} {
					puts = append(puts, coverPut(page, a, size))
				}
			}
		}
		ids, err := layers.AddStickers(p, puts)
		if err != nil || len(ids) != len(puts) {
			t.Fatalf("заворот %v расстав %v: %d из %d, %v", v[0], v[1], len(ids), len(puts), err)
		}
		for i, st := range p.Cover.Sheet.Stickers {
			page, aIdx := i/15, i%15/3 // порядок puts: сторонка, место, размер
			box := sides[page].Box     // CoverSides: задняя (Scope left), затем лицевая
			if !inBox(st, box) {
				t.Errorf("заворот %v расстав %v, стикер %d: %+v вне зоны %+v", v[0], v[1], i, st, box)
			}
			if bad := cornerMiss(st, box, anchors[aIdx]); bad != "" {
				t.Errorf("сторонка %d: %s: %+v, зона %+v", page, bad, st, box)
			}
		}
	}
}

// cornerMiss — описание промаха, если угловой стикер не стоит вплотную к углу зоны ("" — стоит).
func cornerMiss(st *model.Sticker, box layout.Rect, anchor string) string {
	switch anchor {
	case layers.AnchorTopLeft:
		if !near(st.X, box.X) || !near(st.Y, box.Y) {
			return "левый верхний не в углу"
		}
	case layers.AnchorBottomRight:
		if !near(st.X+st.W, box.X+box.W) || !near(st.Y+st.H, box.Y+box.H) {
			return "правый нижний не в углу"
		}
	}
	return ""
}

// TestAddStickersCoverCenterClick: center + m агента на обложке равен щелчку человека (размер и
// центр от страницы листа — эталон TestAddStickerCoverClickSame), с заворотом и без.
func TestAddStickersCoverCenterClick(t *testing.T) {
	for _, v := range coverSpecs {
		for page := range 2 {
			click := coverStickerEd(t, v[0], v[1])
			if err := layers.AddSticker(click, model.CoverID, page, "st", coverStar); err != nil {
				t.Fatal(err)
			}
			agent := coverStickerEd(t, v[0], v[1])
			if _, err := layers.AddStickers(agent, []layers.StickerPut{coverPut(page, layers.AnchorCenter, "m")}); err != nil {
				t.Fatal(err)
			}
			a, c := agent.Cover.Sheet.Stickers[0], click.Cover.Sheet.Stickers[0]
			ax, ay := center(a)
			cx, cy := center(c)
			if !near(ax, cx) || !near(ay, cy) || !near(a.W, c.W) || !near(a.H, c.H) {
				t.Errorf("заворот %v расстав %v сторонка %d: агент (%v, %v) %v×%v, щелчок (%v, %v) %v×%v",
					v[0], v[1], page, ax, ay, a.W, a.H, cx, cy, c.W, c.H)
			}
		}
	}
}

// TestAddStickersCoverNarrow: Box меньше стикера — рамка уменьшена до зоны, пропорция та же, центр
// и углы внутри зоны.
func TestAddStickersCoverNarrow(t *testing.T) {
	narrow := func() *model.Project {
		p := coverStickerEd(t, 12, 6)
		p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 110, 40
		return p
	}
	if sd := narrow().CoverSides(); len(sd) != 2 || min(sd[1].Box.W, sd[1].Box.H) >= 70 {
		t.Fatalf("зона не стала узкой: %+v", sd)
	}
	for _, a := range []string{layers.AnchorCenter, layers.AnchorTopRight, layers.AnchorBottomLeft} {
		q := narrow()
		if _, err := layers.AddStickers(q, []layers.StickerPut{coverPut(1, a, "l")}); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
		st := q.Cover.Sheet.Stickers[0]
		if box := q.CoverSides()[1].Box; !inBox(st, box) || !near(st.W, st.H) {
			t.Errorf("%s: рамка %+v не внутри узкой зоны %+v", a, st, box)
		}
	}
}

// TestAddStickersCoverNoSide: у сторонки нет безопасной зоны (CoverSides её пропускает) — ErrNoSide,
// пачка откатывается целиком (стикер на развороте из той же пачки не остаётся — try вызывающего,
// здесь проверяем, что тело вернуло ids nil); зона меньше мельчайшего стикера — тоже ErrNoSide;
// обложки нет — ErrNoSheet.
func TestAddStickersCoverNoSide(t *testing.T) {
	p := coverStickerEd(t, 12, 6)
	p.Cover.Spec.Safe = 400 // зона шире листа: обе сторонки пусты
	if n := len(p.CoverSides()); n != 0 {
		t.Fatalf("сторонок %d, ждали 0", n)
	}
	sp := p.Spreads[0].ID
	puts := []layers.StickerPut{
		{Spread: sp, Src: coverStar, Anchor: layers.AnchorCenter},
		coverPut(1, layers.AnchorCenter, "m"),
	}
	if ids, err := layers.AddStickers(p, puts); !errors.Is(err, layers.ErrNoSide) || ids != nil {
		t.Fatalf("пустая зона: %v, %v", ids, err)
	}
	if len(p.Cover.Sheet.Stickers) != 0 {
		t.Fatal("на обложке остался стикер")
	}
	// Узкая, но не пустая зона: ширина 2 мм (меньше мельчайшего стикера) — нет места.
	q := coverStickerEd(t, 12, 6)
	q.Cover.Spec.Safe, q.Cover.Spec.SpineSafe = 100, 100
	q.Cover.Spec.SpineSafe = 100 + q.CoverSides()[1].Box.W - 2
	sd := q.CoverSides()
	if len(sd) != 2 || sd[1].Box.W < 1.9 || sd[1].Box.W > 2.1 {
		t.Fatalf("зона не стала узкой: %+v", sd)
	}
	if ids, err := layers.AddStickers(q, []layers.StickerPut{coverPut(1, layers.AnchorCenter, "s")}); !errors.Is(err, layers.ErrNoSide) || ids != nil {
		t.Fatalf("узкая зона %v: %v, %v", sd[1].Box, ids, err)
	}
	// Обложки нет.
	if _, err := layers.AddStickers(model.New(), []layers.StickerPut{coverPut(0, layers.AnchorCenter, "m")}); !errors.Is(err, layers.ErrNoSheet) {
		t.Fatalf("нет обложки: %v", err)
	}
}
