package grid_test

import (
	"encoding/json"
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Снимок «Копировать» (099, design §3): копии на момент Ctrl+C, якорь, стек, пустое выделение.

// clipIDs — ID элементов листа s1 фикстуры clipEd.
type clipIDs struct {
	a, empty, b    string // место с фото «a» (рамочка-картинка), пустое (правая), место с фото «b»
	text, st, own  string // надпись (левая), стикер-рисунок (правая), свой стикер (левая)
	ownRef, frameR string // записи своего стикера и картинки рамочки
}

// clipEd — сетка s1 (страница 281×284): «a» и «b» на левой, пустое место на правой; надпись и свой
// стикер на левой, стикер-рисунок на правой; у «a» рамочка-картинка. s2 — «Авто» из «c», «d».
func clipEd(t *testing.T) (*album.Editor, clipIDs) {
	t.Helper()
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(293, 12, 127, 127, ""),
		edtest.Gp(150, 150, 100, 80, "b")}, []string{"c", "d"})
	g := edtest.P(e).Spreads[0].Grid
	ids := clipIDs{a: g[0].ID, empty: g[1].ID, b: g[2].ID}
	ids.text = edtest.AddText(t, e, "s1", 0)
	var err error
	if _, ids.st, err = e.AddSticker("s1", 1, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	ownKey := "my:00000000000000aa"
	src := model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(ownKey), Path: edtest.Abs("logo.png"), Lib: ownKey, W: 400, H: 200}}
	if _, ids.own, err = e.AddSticker("s1", 0, src); err != nil {
		t.Fatal(err)
	}
	ids.ownRef, ids.frameR = model.LibPhotoID(ownKey), model.LibPhotoID(frKey)
	if _, err := e.ApplyBorder("s1", []string{"a"}, frPic()); err != nil {
		t.Fatal(err)
	}
	return e, ids
}

// clipJSON — снимок целиком (сверка «не изменился»).
func clipJSON(t *testing.T, c model.Clip) string {
	t.Helper()
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestTakeClipSnapshot — снимок: места (повтор ссылки схлопнут), надписи, стикеры и записи фото,
// рамочки и своего стикера; стек источника; якорь страницы и листа; копии — правка и удаление
// оригинала снимок не меняют; копирование без шага отмены и rev; «Авто» — ячейки раскладки.
func TestTakeClipSnapshot(t *testing.T) {
	e, id := clipEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	c, ok := e.TakeClip("s1", model.ClipSel{Refs: []string{"a", id.empty, id.a, "нет такого"},
		Texts: []string{id.text, "чужой"}, Stickers: []string{id.st, id.own}})
	if !ok {
		t.Fatal("нечего копировать")
	}
	edtest.Same(t, e, rev, undo, "копирование")
	if in := c.Info(); in != (model.ClipInfo{Photos: 1, Places: 1, Texts: 1, Stickers: 2, Sheet: "s1"}) {
		t.Fatalf("сводка %+v", in)
	}
	if len(c.Places) != 2 || c.Places[0].ID != id.a || c.Places[0].Photo != "a" || c.Places[1].ID != id.empty {
		t.Fatalf("места %+v", c.Places)
	}
	for _, r := range []string{"a", id.ownRef, id.frameR} {
		if _, ok := c.Refs[r]; !ok {
			t.Fatalf("нет записи %s: %v", r, slices.Collect(maps.Keys(c.Refs)))
		}
	}
	if _, ok := c.Refs["b"]; ok {
		t.Fatal("запись невыделенного фото в снимке")
	}
	var want []string
	for _, s := range edtest.StackOf(e) {
		if slices.Contains([]string{id.a, id.empty, id.text, id.st, id.own}, s) {
			want = append(want, s)
		}
	}
	if !slices.Equal(c.Seq, want) {
		t.Fatalf("стек %v, ждали %v", c.Seq, want)
	}
	if c.Anchor.Page != -1 || math.Abs(c.Anchor.Fx-(c.Box.X+c.Box.W/2)/562) > 1e-9 {
		t.Fatalf("группа через корешок: якорь %+v, рамка %+v", c.Anchor, c.Box)
	}
	before := clipJSON(t, c)
	// Оригинал правят и удаляют: снимок прежний.
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		s.Grid[0].X, s.Grid[0].Border.Inks = 20, []string{"#000000"}
		r := p.Photos["a"]
		r.Frame = &model.Frame{S: 2}
		p.Photos["a"] = r
		s.Texts[0].Text, s.Stickers[0].W = "другое", 60
	})
	e.DeleteSelection("s1", []string{"a"}, []string{id.a}, []string{id.text, id.st, id.own})
	if got := clipJSON(t, c); got != before {
		t.Fatalf("снимок изменился вслед за оригиналом:\n%s\n%s", before, got)
	}

	// Одна страница: якорь — левая, центр в её долях.
	e, id = clipEd(t)
	c, _ = e.TakeClip("s1", model.ClipSel{Refs: []string{id.a}, Texts: []string{id.text}})
	cx, cy := c.Box.X+c.Box.W/2, c.Box.Y+c.Box.H/2
	if c.Anchor != (model.ClipAnchor{Page: 0, Fx: cx / 281, Fy: cy / 284}) {
		t.Fatalf("якорь левой страницы %+v, рамка %+v", c.Anchor, c.Box)
	}
	c, _ = e.TakeClip("s1", model.ClipSel{Stickers: []string{id.st}})
	if c.Anchor.Page != 1 || math.Abs(c.Anchor.Fx-(c.Box.X+c.Box.W/2-281)/281) > 1e-9 {
		t.Fatalf("якорь правой страницы %+v, рамка %+v", c.Anchor, c.Box)
	}

	// «Авто»: фото с ячейкой раскладки, в стеке — первыми.
	c, ok = e.TakeClip("s2", model.ClipSel{Refs: []string{"d", "c", "d"}})
	v := e.View()
	if !ok || len(c.Auto) != 2 || c.Auto[0].ID != "d" || c.Auto[0].Box != edtest.RectOf(v, 1, "d") || !slices.Equal(c.Seq, []string{"d", "c"}) {
		t.Fatalf("«Авто»: %+v", c)
	}
	if in := c.Info(); in.Photos != 2 || in.Places != 0 {
		t.Fatalf("сводка «Авто» %+v", in)
	}
}

// TestTakeClipNothing — нечего копировать: неизвестный лист, пустое выделение, чужие и неизвестные
// ID — false и пустой снимок; правки, шага и rev нет. Пустой снимок — пустая сводка, nil тоже.
func TestTakeClipNothing(t *testing.T) {
	e, id := clipEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, tc := range []struct {
		sheet string
		sel   model.ClipSel
	}{
		{"нет", model.ClipSel{Refs: []string{id.a}}},
		{"s1", model.ClipSel{}},
		{"s1", model.ClipSel{Refs: []string{"c", ""}, Texts: []string{id.st}, Stickers: []string{id.text}}},
		{"s2", model.ClipSel{Refs: []string{id.a, "a"}, Texts: []string{id.text}, Stickers: []string{id.st}}},
		{model.CoverID, model.ClipSel{Refs: []string{"a"}}},
	} {
		c, ok := e.TakeClip(tc.sheet, tc.sel)
		if ok || !c.Empty() || c.Info() != (model.ClipInfo{}) {
			t.Fatalf("%s %+v: снимок %+v", tc.sheet, tc.sel, c)
		}
	}
	edtest.Same(t, e, rev, undo, "пустое копирование")
	var nilClip *model.Clip
	if !nilClip.Empty() || nilClip.Info() != (model.ClipInfo{}) || nilClip.PhotoIDs() != nil {
		t.Fatal("nil-снимок не пуст")
	}
}
