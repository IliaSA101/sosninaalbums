package album_test

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Вырезать» (099, design §3): снимок и удаление — один шаг отмены; место уходит вместе с фото.

// cutEd — сетка s1: «a» и пустое место на левой, «b» на правой, надпись и стикер; s2 — «Авто» из «c».
func cutEd(t *testing.T) (e *album.Editor, places []string, text, st string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 120, 100, "a"), edtest.Gp(12, 150, 120, 100, ""),
		edtest.Gp(300, 12, 120, 100, "b")}, []string{"c"})
	text = edtest.AddText(t, e, "s1", 0)
	_, st, err := e.AddSticker("s1", 1, model.StickerSrc{Art: &model.Art{Kind: "star"}})
	if err != nil {
		t.Fatal(err)
	}
	return e, edtest.PlaceIDsOf(e), text, st
}

// TestCutClipOneStep — Ctrl+X надписи, стикера и фото с места: один шаг отмены и rev; Ctrl+Z
// возвращает проект байт в байт, Ctrl+Y — снова без них; снимок тот же, что у Ctrl+C. Нечего
// вырезать (пусто, чужое, неизвестный лист) — холостая без шага. Круг Save→Load после вырезания.
func TestCutClipOneStep(t *testing.T) {
	e, places, text, st := cutEd(t)
	sel := model.ClipSel{Refs: []string{"a"}, Texts: []string{text}, Stickers: []string{st}}
	want, _ := e.TakeClip("s1", sel)
	orig := model.Marshal(edtest.P(e))
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, c, ok, err := e.CutClip("s1", sel)
	if err != nil || !ok {
		t.Fatalf("вырезать: %v %v", ok, err)
	}
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d → %d, rev %d → %d", undo, edtest.UndoLen(e), rev, edtest.Rev(e))
	}
	if string(model.Marshal(edtest.P(e))) == string(orig) || clipStr(t, c) != clipStr(t, want) {
		t.Fatal("вырезание: проект не изменился или снимок не тот, что у копирования")
	}
	s := edtest.P(e).Spreads[0]
	if len(s.Texts) != 0 || len(s.Stickers) != 0 || s.PlaceRef(places[0]) != nil || edtest.P(e).Placed("a") {
		t.Fatalf("после вырезания: тексты %d, стикеры %d, места %v", len(s.Texts), len(s.Stickers), edtest.PlaceIDsOf(e))
	}
	if a, b := sheetsStr(t, edtest.SaveLoad(t, e)), sheetsStr(t, edtest.P(e)); a != b {
		t.Fatalf("круг Save→Load после вырезания меняет листы:\n%s\n%s", b, a)
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), orig) {
		t.Fatal("Ctrl+Z не вернул проект байт в байт")
	}
	e.Redo()
	if edtest.P(e).Placed("a") || len(edtest.P(e).Spreads[0].Texts) != 0 {
		t.Fatal("Ctrl+Y не повторил вырезание")
	}

	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	for _, tc := range []struct {
		sheet string
		sel   model.ClipSel
	}{
		{"s1", model.ClipSel{}},
		{"s1", model.ClipSel{Refs: []string{"c"}, Texts: []string{"нет"}}},
		{"нет", model.ClipSel{Refs: []string{"b"}}},
	} {
		if _, _, ok, err := e.CutClip(tc.sheet, tc.sel); ok || err != nil {
			t.Fatalf("%s %+v: вырезано %v %v", tc.sheet, tc.sel, ok, err)
		}
	}
	edtest.Same(t, e, rev, undo, "пустое вырезание")
}

// TestCutRemovesPlaceWithPhoto — у «Ручной» место уходит вместе с фото (Del оставляет его пустым),
// пустое место — тоже; фото — в ленте. У «Авто» фото уходит в ленту, опустевший разворот без
// оформления убирается (Normalize, как Del). Все места листа — лист становится «Авто».
func TestCutRemovesPlaceWithPhoto(t *testing.T) {
	e, places, _, _ := cutEd(t)
	_, _, ok, _ := e.CutClip("s1", model.ClipSel{Refs: []string{"a", places[1]}})
	if !ok || !slices.Equal(edtest.PlaceIDsOf(e), places[2:]) || edtest.P(e).Placed("a") || !edtest.P(e).Placed("b") {
		t.Fatalf("места после вырезания %v (было %v)", edtest.PlaceIDsOf(e), places)
	}
	// Образец Del: место остаётся пустым.
	d, dp, _, _ := cutEd(t)
	d.DeleteSelection("s1", []string{"a"}, nil, nil)
	if !slices.Equal(edtest.PlaceIDsOf(d), dp) {
		t.Fatalf("Del убрал место: %v", edtest.PlaceIDsOf(d))
	}

	// Все места листа: «Ручная» снимается, надпись и стикер остаются.
	_, _, ok, _ = e.CutClip("s1", model.ClipSel{Refs: []string{"b"}})
	if s := edtest.P(e).Spreads[0]; !ok || len(s.Grid) != 0 || len(s.Photos) != 0 || len(s.Texts) != 1 {
		t.Fatalf("последнее место: сетка %d, фото %v", len(s.Grid), s.Photos)
	}

	// «Авто»: единственное фото разворота без оформления — разворот уходит.
	n := len(edtest.P(e).Spreads)
	_, c, ok, _ := e.CutClip("s2", model.ClipSel{Refs: []string{"c"}})
	if !ok || len(c.Auto) != 1 || edtest.P(e).Placed("c") || len(edtest.P(e).Spreads) != n-1 {
		t.Fatalf("«Авто»: вырезано %v, разворотов %d → %d", ok, n, len(edtest.P(e).Spreads))
	}
	if _, keep := c.Refs["c"]; !keep {
		t.Fatal("в снимке нет записи вырезанного фото")
	}
}

// clipStr — снимок строкой для сравнения.
func clipStr(t *testing.T, c model.Clip) string {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sheetsStr — листы проекта строкой без ID фото (Load пересчитывает их из путей — Rekey): места,
// надписи, стикеры и порядок слоёв; у места — есть ли фото.
func sheetsStr(t *testing.T, p *model.Project) string {
	t.Helper()
	type sheet struct {
		ID       string
		Grid     []model.Place
		Photos   int
		Texts    []*model.TextBlock
		Stickers []*model.Sticker
		Order    []string
	}
	var out []sheet
	for _, s := range p.Sheets() {
		sh := sheet{ID: s.ID, Photos: len(s.Photos), Texts: s.Texts, Stickers: s.Stickers, Order: s.Order}
		for _, pl := range s.Grid {
			c := *pl
			if c.Photo != "" {
				c.Photo = "+"
			}
			sh.Grid = append(sh.Grid, c)
		}
		out = append(out, sh)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
