package album_test

// Страж разбора 0.25-auto-after-manual: «Вернуть авто» снимает только места (Grid, Layout, свой
// кадр) — тексты, стикеры и фон остаются.

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// decorEd — ручной лист s1: прямоугольник, круг, место с поворотом 12° и пустое; текст
// «Наша свадьба» со своим положением, стикер и узор на весь лист — всё через Editor.
func decorEd(t *testing.T) (e *album.Editor, textID string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{
		edtest.Gp(10, 10, 80, 60, "a"),
		{X: 100, Y: 10, W: 60, H: 60, Photo: "b", Shape: model.ShapeOval},
		{X: 10, Y: 100, W: 80, H: 60, Photo: "c", Rot: 12},
		edtest.Gp(200, 100, 60, 40, ""),
	})
	if _, err := e.AddText("s1", 0, ""); err != nil {
		t.Fatal(err)
	}
	texts := edtest.P(e).Spreads[0].Texts
	if len(texts) != 1 {
		t.Fatalf("текстов %d, ждали 1", len(texts))
	}
	textID = texts[0].ID
	e.SetText(textID, "Наша свадьба", "")
	e.SetTextBox(textID, 50, 200, 120)
	if _, _, err := e.AddSticker("s1", 1, model.StickerSrc{Art: &model.Art{Kind: "plate"}}); err != nil {
		t.Fatal(err)
	}
	bg := []model.GridTarget{{Spread: "s1", Scope: model.ScopeSpread}}
	src := model.BgSource{Pattern: &model.Pattern{Kind: "dots", Preset: "sa:dots"}, Color: "#f5efe6"}
	if _, n, err := e.ApplyBackgroundTo(bg, src); err != nil || n != 1 {
		t.Fatalf("фон: n=%d err=%v", n, err)
	}
	return e, textID
}

// TestBackToAutoKeepsDecor: «Вернуть авто» сохраняет тексты (ID, текст, положение, стиль),
// стикеры и фон; мест нет; один шаг отмены возвращает места.
func TestBackToAutoKeepsDecor(t *testing.T) {
	e, textID := decorEd(t)
	before := edtest.P(e).Spreads[0]
	if before.Pat == nil || len(before.Grid) != 4 || len(before.Stickers) != 1 {
		t.Fatalf("подготовка: узор %v, мест %d, стикеров %d", before.Pat, len(before.Grid), len(before.Stickers))
	}
	txt, stk, pat := *before.Texts[0], *before.Stickers[0], *before.Pat
	undo := edtest.UndoLen(e)

	v, err := e.BackToAuto("s1", "", true)
	if err != nil {
		t.Fatal(err)
	}
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 0 || v.Spreads[0].Grid {
		t.Fatalf("мест %d, сетка в виде %v", len(s.Grid), v.Spreads[0].Grid)
	}
	if len(s.Texts) != 1 || !reflect.DeepEqual(*s.Texts[0], txt) || s.Texts[0].ID != textID {
		t.Fatalf("текст изменился: было %+v, стало %+v", txt, s.Texts)
	}
	tv := v.Spreads[0].Texts
	if len(tv) != 1 || tv[0].ID != textID || tv[0].Text != "Наша свадьба" || tv[0].X != txt.X || tv[0].Y != txt.Y || tv[0].W != txt.W {
		t.Fatalf("текст в виде: %+v", tv)
	}
	if len(s.Stickers) != 1 || !reflect.DeepEqual(*s.Stickers[0], stk) || len(v.Spreads[0].Stickers) != 1 {
		t.Fatalf("стикер изменился: было %+v, стало %+v", stk, s.Stickers)
	}
	if s.Pat == nil || !reflect.DeepEqual(*s.Pat, pat) || s.BgL != before.BgL || s.BgR != before.BgR {
		t.Fatalf("фон изменился: узор %+v, bgL %q bgR %q", s.Pat, s.BgL, s.BgR)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	if got := edtest.P(e).Spreads[0]; len(got.Grid) != 4 || got.Grid[1].Shape != model.ShapeOval || got.Grid[2].Rot != 12 {
		t.Fatalf("отмена не вернула места: %+v", got.Grid)
	}
	if edtest.UndoLen(e) != undo {
		t.Fatalf("после отмены шагов %d, ждали %d", edtest.UndoLen(e), undo)
	}
}

// TestBackToAutoKeepsTextUnder: текст, опущенный под фото, после «Авто» остаётся в виде
// (порядок слоёв сбрасывается — у «Авто» тексты сверху, но не пропадают).
func TestBackToAutoKeepsTextUnder(t *testing.T) {
	e, textID := decorEd(t)
	e.ArrangeLayers("s1", "back", []string{textID})
	if order := edtest.P(e).Spreads[0].Order; len(order) == 0 || order[0] != textID {
		t.Fatalf("подготовка: текст не опущен, порядок %v", order)
	}
	v, err := e.BackToAuto("s1", "", true)
	if err != nil {
		t.Fatal(err)
	}
	tv := v.Spreads[0].Texts
	if len(tv) != 1 || tv[0].ID != textID || tv[0].Text != "Наша свадьба" {
		t.Fatalf("текст после «Авто»: %+v", tv)
	}
}
