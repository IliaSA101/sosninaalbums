package album_test

// Стикеры агента на обложку (154 Т1, design §2): AddStickersIf с листом cover — круг Save → Load,
// отмена, пачка с разворотом, откат при ErrNoSide.

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// inZone — рамка стикера внутри зоны (допуск на округление до сотых).
func inZone(st *model.Sticker, z model.CoverSide) bool {
	const eps = 0.011
	b := z.Box
	return st.X >= b.X-eps && st.Y >= b.Y-eps && st.X+st.W <= b.X+b.W+eps && st.Y+st.H <= b.Y+b.H+eps
}

// TestAddStickersIfCover: пачка из двух стикеров на обложке (лицевая и задняя) и одного на развороте —
// один шаг; рамки внутри Box своей сторонки; Save → Load сохраняет их; отмена по rev снимает всё.
func TestAddStickersIfCover(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "cv.salbum")
	e, ids := ifStickerEd(t, path)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	puts := []album.StickerPut{
		{Spread: model.CoverID, Page: 1, Src: ifStar, Anchor: layers.AnchorTopRight, Size: "s"},
		{Spread: ids[0], Page: 0, Src: ifStar, Anchor: layers.AnchorCenter},
		{Spread: model.CoverID, Page: 0, Src: ifStar, Anchor: layers.AnchorBottomLeft, Size: "l"},
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, got, err := e.AddStickersIf(puts, album.Expect{Rev: rev})
	if err != nil || len(got) != 3 || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("AddStickersIf: ids %d, rev %d → %d, шагов %d, %v", len(got), rev, v.Rev, edtest.UndoLen(e)-undo, err)
	}
	p := edtest.P(e)
	sides := p.CoverSides()
	cs := p.Cover.Sheet.Stickers
	if len(sides) != 2 || len(cs) != 2 || len(p.Spreads[0].Stickers) != 1 {
		t.Fatalf("сторонок %d, на обложке %d, на развороте %d", len(sides), len(cs), len(p.Spreads[0].Stickers))
	}
	// Порядок CoverSides: задняя (Scope left), затем лицевая; стикеры — в порядке пачки.
	if !inZone(cs[0], sides[1]) || !inZone(cs[1], sides[0]) || cs[0].ID != got[0] || cs[1].ID != got[2] {
		t.Fatalf("рамки вне зон: %+v в %+v, %+v в %+v", cs[0], sides[1].Box, cs[1], sides[0].Box)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || q.Cover == nil || len(q.Cover.Sheet.Stickers) != 2 || len(q.Spreads[0].Stickers) != 1 {
		t.Fatalf("после Save → Load: %v", err)
	}
	if _, err := e.UndoIfRev(album.Expect{Rev: v.Rev}); err != nil {
		t.Fatal(err)
	}
	if p := edtest.P(e); len(p.Cover.Sheet.Stickers) != 0 || len(p.Spreads[0].Stickers) != 0 {
		t.Fatal("отмена одного шага не сняла всю пачку")
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, err := e.AddStickersIf(puts, x)
		return err
	})
}

// TestAddStickersIfCoverNoSide: сторонка без безопасной зоны — ErrNoSide, вся пачка (в том числе
// стикер на развороте перед ним) откатана: ни шага, ни rev, проект прежний.
func TestAddStickersIfCoverNoSide(t *testing.T) {
	e, ids := ifStickerEd(t, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe = 400, 400 })
	if n := len(edtest.P(e).CoverSides()); n != 0 {
		t.Fatalf("сторонок %d: зона не пропала, проверять нечего", n)
	}
	s := take(e)
	puts := []album.StickerPut{
		{Spread: ids[0], Src: ifStar, Anchor: layers.AnchorCenter},
		{Spread: model.CoverID, Page: 1, Src: ifStar, Anchor: layers.AnchorCenter},
	}
	if _, got, err := e.AddStickersIf(puts, album.Expect{Rev: s.rev}); !errors.Is(err, album.ErrNoSide) || got != nil {
		t.Fatalf("нет места: ids %v, %v", got, err)
	}
	s.same(t, e, "ErrNoSide")
}
