package grid_test

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Вставка из буфера и «Дублировать» (099, design §4): положение, лесенка, «Авто», обложка,
// виньетка, пределы, слои, записи стикеров и рамочек, вопрос о фото в альбоме.

// mustTake — снимок; нечего копировать — Fatal.
func mustTake(t *testing.T, e *album.Editor, sheet string, sel model.ClipSel) *model.Clip {
	t.Helper()
	c, ok := e.TakeClip(sheet, sel)
	if !ok {
		t.Fatalf("нечего копировать: %s %+v", sheet, sel)
	}
	return &c
}

// mustPaste — вставка без ошибки и без вопроса.
func mustPaste(t *testing.T, e *album.Editor, c *model.Clip, r model.PasteReq) model.PasteOut {
	t.Helper()
	_, out, err := e.PasteClip(c, r)
	if err != nil || len(out.Repeat) > 0 {
		t.Fatalf("вставка %+v: %v, вопрос %v", r, err, out.Repeat)
	}
	return out
}

// textOf — надпись по ID на любом листе.
func textOf(t *testing.T, e *album.Editor, id string) model.TextBlock {
	t.Helper()
	s, i := edtest.P(e).FindText(id)
	if s == nil {
		t.Fatalf("надписи %s нет", id)
	}
	return *s.Texts[i]
}

// stickerOf — стикер по ID на любом листе.
func stickerOf(t *testing.T, e *album.Editor, id string) model.Sticker {
	t.Helper()
	s, i := edtest.P(e).FindSticker(id)
	if s == nil {
		t.Fatalf("стикера %s нет", id)
	}
	return *s.Stickers[i]
}

// sheetByID — лист проекта по ID.
func sheetByID(e *album.Editor, id string) *model.Spread { return edtest.P(e).Sheet(id) }

// at — точка листа.
func at(x, y float64) *model.Pt { return &model.Pt{X: x, Y: y} }

// wantXY — угол элемента (x, y) там, где ждали, до 0,01 мм.
func wantXY(t *testing.T, what string, x, y, wx, wy float64) {
	t.Helper()
	if math.Abs(x-wx) > 0.011 || math.Abs(y-wy) > 0.011 {
		t.Fatalf("%s: (%.2f, %.2f), ждали (%.2f, %.2f)", what, x, y, wx, wy)
	}
}

// TestPasteSamePagePoint — Ctrl+V без курсора над листом: группа одной страницы — в ту же точку той
// же страницы другого листа; через корешок — в ту же точку листа. Один снимок вставляется много
// раз и не меняется.
func TestPasteSamePagePoint(t *testing.T) {
	e, id := clipEd(t)
	src := textOf(t, e, id.text)
	c := mustTake(t, e, "s1", model.ClipSel{Texts: []string{id.text}})
	before := clipJSON(t, *c)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s2"})
	if len(out.Texts) != 1 || out.Froze || len(out.Refs) != 0 {
		t.Fatalf("итог %+v", out)
	}
	got := textOf(t, e, out.Texts[0])
	wantXY(t, "надпись левой страницы", got.X, got.Y, src.X, src.Y)
	if got.Text != src.Text || got.Style != src.Style || got.ID == src.ID {
		t.Fatalf("надпись %+v, источник %+v", got, src)
	}

	// Надпись (левая) и стикер (правая) — через корешок: доли листа.
	st := stickerOf(t, e, id.st)
	c = mustTake(t, e, "s1", model.ClipSel{Texts: []string{id.text}, Stickers: []string{id.st}})
	if c.Anchor.Page != -1 {
		t.Fatalf("якорь %+v", c.Anchor)
	}
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s2", Hover: at(10, 10)})
	got, gs := textOf(t, e, out.Texts[0]), stickerOf(t, e, out.Stickers[0])
	wantXY(t, "надпись группы", got.X, got.Y, src.X, src.Y)
	wantXY(t, "стикер группы", gs.X, gs.Y, st.X, st.Y)
	if s2 := sheetByID(e, "s2"); len(s2.Texts) != 2 || len(s2.Stickers) != 1 || len(s2.Grid) != 0 {
		t.Fatalf("s2: надписей %d, стикеров %d, мест %d", len(s2.Texts), len(s2.Stickers), len(s2.Grid))
	}
	if clipJSON(t, *mustTake(t, e, "s1", model.ClipSel{Texts: []string{id.text}})) != before {
		t.Fatal("вставка изменила источник")
	}
}

// TestPasteHoverPage — курсор над листом выбирает страницу: над правой — та же точка правой
// страницы, над левой — левой; точка с NaN — «курсора нет» (страница источника).
func TestPasteHoverPage(t *testing.T) {
	e, id := clipEd(t)
	src := textOf(t, e, id.text)
	c := mustTake(t, e, "s1", model.ClipSel{Texts: []string{id.text}})
	for _, tc := range []struct {
		hover *model.Pt
		dx    float64
	}{
		{at(400, 10), 281},
		{at(100, 200), 0},
		{at(math.NaN(), 10), 0},
		{at(600, math.Inf(1)), 0},
	} {
		out := mustPaste(t, e, c, model.PasteReq{Sheet: "s2", Hover: tc.hover})
		got := textOf(t, e, out.Texts[0])
		wantXY(t, "надпись", got.X, got.Y, src.X+tc.dx, src.Y)
	}
	if r := (model.PasteReq{At: at(math.NaN(), 0), Hover: at(1, 2), Repeat: "всё"}).Clean(); r.At != nil || r.Hover == nil || r.Repeat != "" {
		t.Fatalf("Clean: %+v", r)
	}
}

// TestPasteAtPoint — «Вставить» из меню: центр вставленного — в точке щелчка, без лесенки и на
// листе-источнике (точка — центр оригинала: копия ложится ровно на него).
func TestPasteAtPoint(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s1", model.ClipSel{Texts: []string{id.text}, Stickers: []string{id.own}})
	cx, cy := c.Box.X+c.Box.W/2, c.Box.Y+c.Box.H/2
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s1", At: at(cx, cy), Hover: at(500, 10)})
	src, got := textOf(t, e, id.text), textOf(t, e, out.Texts[0])
	wantXY(t, "на месте оригинала", got.X, got.Y, src.X, src.Y)
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s2", At: at(cx+200, cy+10)})
	own, gs := stickerOf(t, e, id.own), stickerOf(t, e, out.Stickers[0])
	wantXY(t, "стикер в точке", gs.X, gs.Y, own.X+200, own.Y+10)
}

// TestPasteCascade — повторная вставка на тот же лист: каждая следующая на 5 мм правее и ниже,
// пока место занято такой же копией; на другом листе лесенки нет.
func TestPasteCascade(t *testing.T) {
	e, id := clipEd(t)
	src := textOf(t, e, id.text)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}})
	pl := *sheetByID(e, "s1").Grid[1]
	for k := 1; k <= 3; k++ {
		out := mustPaste(t, e, c, model.PasteReq{Sheet: "s1"})
		got := textOf(t, e, out.Texts[0])
		wantXY(t, "надпись лесенкой", got.X, got.Y, src.X+5*float64(k), src.Y+5*float64(k))
		np := sheetByID(e, "s1").PlaceRef(out.Refs[0])
		wantXY(t, "место лесенкой", np.X, np.Y, pl.X+5*float64(k), pl.Y+5*float64(k))
	}
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s2"})
	got := textOf(t, e, out.Texts[0])
	wantXY(t, "другой лист", got.X, got.Y, src.X, src.Y)
	// Только надпись: место другое — совпадение надписи всё равно сдвигает.
	c = mustTake(t, e, "s2", model.ClipSel{Texts: []string{out.Texts[0]}})
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s2"})
	got = textOf(t, e, out.Texts[0])
	wantXY(t, "лесенка на «Авто»", got.X, got.Y, src.X+5, src.Y+5)
}

// TestPasteFitInside — группа за краем листа сдвигается внутрь без смены размера; группа шире
// листа — центр внутри листа.
func TestPasteFitInside(t *testing.T) {
	e, id := clipEd(t)
	src := textOf(t, e, id.text)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}})
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s1", At: at(2000, -300)})
	pl, got := sheetByID(e, "s1").PlaceRef(out.Refs[0]), textOf(t, e, out.Texts[0])
	box := c.Box
	if math.Abs(pl.X+pl.W-562) > 0.011 || pl.Y > 0.011 || got.W != src.W || pl.W != 127 || pl.H != 127 {
		t.Fatalf("за краем: место %+v, надпись %+v, рамка группы %+v", *pl, got, box)
	}
	// Стикер шире листа: размер прежний, центр — в точке на краю листа.
	edtest.Change(e, func(p *model.Project) { st := p.Spreads[0].Stickers[0]; st.X, st.W, st.H = -100, 700, 700 })
	c = mustTake(t, e, "s1", model.ClipSel{Stickers: []string{id.st}})
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s2", At: at(-50, 900)})
	gs := stickerOf(t, e, out.Stickers[0])
	if gs.W != 700 || gs.H != 700 || gs.X+gs.W/2 != 0 || gs.Y+gs.H/2 != 284 {
		t.Fatalf("широкий стикер: %+v", gs)
	}
}

// TestPasteCover — обложка — как ручной лист: надпись левой страницы разворота встаёт в ту же
// точку задней сторонки; место обложки — новое место без закрепления (у обложки нет «Авто»).
func TestPasteCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(12, 12, 120, 100, ""), edtest.Gp(300, 12, 120, 100, "b")}, []string{"c"})
	tid := edtest.AddText(t, e, "s1", 0)
	src := textOf(t, e, tid)
	c := mustTake(t, e, "s1", model.ClipSel{Texts: []string{tid}, Refs: []string{edtest.P(e).Spreads[0].Grid[0].ID}})
	out := mustPaste(t, e, c, model.PasteReq{Sheet: model.CoverID})
	g := edtest.P(e).Geo(sheetByID(e, model.CoverID))
	pg := g.Pages[0]
	got := textOf(t, e, out.Texts[0])
	bx, by := c.Box.X+c.Box.W/2, c.Box.Y+c.Box.H/2 // центр группы на левой странице разворота
	wx := pg.X + bx/281*pg.W - bx + src.X
	wy := pg.Y + by/284*pg.H - by + src.Y
	wantXY(t, "надпись на задней сторонке", got.X, got.Y, wx, wy)
	cov := sheetByID(e, model.CoverID)
	if out.Froze || len(cov.Grid) != 2 || cov.PlaceRef(out.Refs[0]) == nil {
		t.Fatalf("места обложки %d, итог %+v", len(cov.Grid), out)
	}
	// Обратно: место обложки — на разворот.
	c = mustTake(t, e, model.CoverID, model.ClipSel{Refs: []string{out.Refs[0]}})
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s1", Hover: at(400, 100)})
	if len(sheetByID(e, "s1").Grid) != 3 || out.Froze {
		t.Fatalf("разворот после вставки с обложки: %+v", out)
	}
	edtest.MustGrid(t, e)
}

// TestPasteVignetteHead — шапка виньетки вставляется обычной надписью: шапка листа прежняя; место
// с виньетки на обычный лист — без роли, на виньетку — ученик.
func TestPasteVignetteHead(t *testing.T) {
	e, _ := clipEd(t)
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	ps := edtest.P(e).Spreads
	v := ps[len(ps)-1]
	head := v.Vig.Head
	if head == "" || len(v.Grid) == 0 {
		t.Fatalf("виньетка без шапки или мест: %+v", v.Vig)
	}
	var teacher string
	for _, pl := range v.Grid {
		if pl.Role == model.RoleTeacher {
			teacher = pl.ID
		}
	}
	if teacher == "" {
		teacher = v.Grid[0].ID
	}
	places := len(v.Grid)
	c := mustTake(t, e, v.ID, model.ClipSel{Refs: []string{teacher}, Texts: []string{head}})
	out := mustPaste(t, e, c, model.PasteReq{Sheet: v.ID})
	v = sheetByID(e, v.ID)
	if v.Vig.Head != head || len(v.Grid) != places+1 || model.RoleOf(v.PlaceRef(out.Refs[0])) != model.RolePupil {
		t.Fatalf("виньетка: шапка %q (была %q), мест %d, роль %q", v.Vig.Head, head, len(v.Grid), v.PlaceRef(out.Refs[0]).Role)
	}
	out = mustPaste(t, e, c, model.PasteReq{Sheet: "s1"})
	s1 := sheetByID(e, "s1")
	if s1.IsVignette() || s1.PlaceRef(out.Refs[0]).Role != "" || textOf(t, e, out.Texts[0]).Text != textOf(t, e, head).Text {
		t.Fatalf("обычный лист: %+v", out)
	}
}

// TestPasteAutoPhotosJoin — на «Авто» фото встают в раскладку (в конец, как бросок), надпись —
// на своё место; лист остаётся «Авто». Вырезанное фото возвращается без вопроса.
func TestPasteAutoPhotosJoin(t *testing.T) {
	e, id := clipEd(t)
	src := textOf(t, e, id.text)
	_, c, ok, err := e.CutClip("s1", model.ClipSel{Refs: []string{"a", "b"}, Texts: []string{id.text}})
	if !ok || err != nil {
		t.Fatal(err)
	}
	out := mustPaste(t, e, &c, model.PasteReq{Sheet: "s2"})
	s2 := sheetByID(e, "s2")
	if out.Froze || len(s2.Grid) != 0 || !slices.Equal(s2.Photos, []string{"c", "d", "a", "b"}) || !slices.Equal(out.Refs, []string{"a", "b"}) {
		t.Fatalf("«Авто»: фото %v, мест %d, итог %+v", s2.Photos, len(s2.Grid), out)
	}
	got := textOf(t, e, out.Texts[0])
	wantXY(t, "надпись на «Авто»", got.X, got.Y, src.X, src.Y)
	// Фото «Авто» на ручной лист — места с ячейками раскладки.
	c2 := mustTake(t, e, "s2", model.ClipSel{Refs: []string{"c"}})
	if _, _, ok, _ := e.CutClip("s2", model.ClipSel{Refs: []string{"c"}}); !ok {
		t.Fatal("не вырезано")
	}
	n := len(sheetByID(e, "s1").Grid)
	out = mustPaste(t, e, c2, model.PasteReq{Sheet: "s1", At: at(400, 150)})
	pl := sheetByID(e, "s1").PlaceRef(out.Refs[0])
	if len(sheetByID(e, "s1").Grid) != n+1 || pl.Photo != "c" || pl.W != c2.Auto[0].Box.W || pl.H != c2.Auto[0].Box.H {
		t.Fatalf("фото «Авто» на ручной: %+v", pl)
	}
	edtest.MustGrid(t, e)
}

// TestPasteAutoEmptyPlaceFreezes — пустое место в буфере делает «Авто» «Ручной» (как «+ Место»):
// фото раскладки — места, новое — сверху; Ctrl+Z возвращает «Авто» одним шагом.
func TestPasteAutoEmptyPlaceFreezes(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}})
	undo := edtest.UndoLen(e)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s2"})
	s2 := sheetByID(e, "s2")
	if !out.Froze || len(s2.Grid) != 3 || s2.Grid[2].ID != out.Refs[0] || s2.Grid[2].Photo != "" || !slices.Equal(s2.Photos, []string{"c", "d"}) {
		t.Fatalf("закрепление: %+v, мест %d", out, len(s2.Grid))
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatal("закрепление и вставка — не один шаг")
	}
	e.Undo()
	if s2 := sheetByID(e, "s2"); len(s2.Grid) != 0 || len(s2.Texts) != 0 {
		t.Fatalf("Ctrl+Z: мест %d, надписей %d", len(s2.Grid), len(s2.Texts))
	}
	edtest.MustGrid(t, e)
}

// TestPasteLimits — сверх предела листа (надписи, стикеры, места, фото «Авто») не встаёт ничего:
// ошибка, проект байт в байт прежний, шага нет. Ровно до предела — встаёт.
func TestPasteLimits(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}, Stickers: []string{id.st}})
	fill := func(n int, f func(s *model.Spread)) {
		edtest.Change(e, func(p *model.Project) {
			for range n {
				f(p.Spreads[0])
			}
		})
	}
	try := func(what string, fail bool) {
		t.Helper()
		orig, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
		_, out, err := e.PasteClip(c, model.PasteReq{Sheet: "s1"})
		if !fail {
			if err != nil {
				t.Fatalf("%s: до предела — отказ %v", what, err)
			}
			e.Undo()
			return
		}
		if err == nil || len(out.Refs)+len(out.Texts)+len(out.Stickers) != 0 {
			t.Fatalf("%s: сверх предела вставлено %+v", what, out)
		}
		if !bytes.Equal(model.Marshal(edtest.P(e)), orig) {
			t.Fatalf("%s: проект изменён", what)
		}
		edtest.Same(t, e, rev, undo, what)
	}
	fill(model.MaxTexts-1-len(sheetByID(e, "s1").Texts), func(s *model.Spread) {
		s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), W: 20, Text: "т", Style: model.PlainStyle()})
	})
	try("надписи до предела", false)
	fill(1, func(s *model.Spread) {
		s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), W: 20, Text: "т", Style: model.PlainStyle()})
	})
	try("надписи", true)
	fill(1, func(s *model.Spread) { s.Texts = s.Texts[:len(s.Texts)-1] })
	fill(model.MaxStickers-len(sheetByID(e, "s1").Stickers), func(s *model.Spread) {
		s.Stickers = append(s.Stickers, &model.Sticker{ID: model.NewID(), Art: &model.Art{Kind: "star"}, X: 1, Y: 1, W: 10, H: 10})
	})
	try("стикеры", true)
	fill(1, func(s *model.Spread) { s.Stickers = s.Stickers[:len(s.Stickers)-1] })
	fill(model.MaxPlaces-len(sheetByID(e, "s1").Grid), func(s *model.Spread) {
		s.Grid = append(s.Grid, &model.Place{ID: model.NewID(), X: 300, Y: 150, W: 20, H: 20})
	})
	try("места", true)

	// Фото на «Авто»: 64 фото в раскладке и ещё одно.
	e2 := edtest.GridEd(t, nil, []string{"z"})
	edtest.Change(e2, func(p *model.Project) {
		s := p.Spreads[0]
		for i := range model.MaxPhotos - 1 {
			pid := "f" + string(rune('A'+i/26)) + string(rune('a'+i%26))
			p.Photos[pid] = model.PhotoRef{Path: edtest.Abs(pid + ".jpg"), W: 300, H: 200}
			s.Photos = append(s.Photos, pid)
		}
	})
	z := mustTake(t, e2, "s2", model.ClipSel{Refs: []string{"z"}})
	if _, _, ok, _ := e2.CutClip("s2", model.ClipSel{Refs: []string{"z"}}); !ok {
		t.Fatal("не вырезано")
	}
	mustPaste(t, e2, z, model.PasteReq{Sheet: "s1"})
	e2.Undo()
	edtest.Change(e2, func(p *model.Project) {
		p.Photos["zz"] = model.PhotoRef{Path: edtest.Abs("zz.jpg"), W: 300, H: 200}
		p.Spreads[0].Photos = append(p.Spreads[0].Photos, "zz")
	})
	if _, _, err := e2.PasteClip(z, model.PasteReq{Sheet: "s1"}); err == nil || !strings.Contains(err.Error(), "фото") {
		t.Fatalf("65-е фото на «Авто»: %v", err)
	}
}

// TestPasteOrderOnTop — вставленное — одним куском на вершине стека, в порядке слоёв источника
// (надпись под местом остаётся под копией места).
func TestPasteOrderOnTop(t *testing.T) {
	e, id := clipEd(t)
	e.ArrangeLayers("s1", "back", []string{id.text})
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}, Stickers: []string{id.st}})
	if c.Seq[0] != id.text {
		t.Fatalf("стек снимка %v", c.Seq)
	}
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s1"})
	fresh := map[string]string{id.text: out.Texts[0], id.st: out.Stickers[0]}
	for i, pl := range c.Places {
		fresh[pl.ID] = out.Refs[i]
	}
	var want []string
	for _, s := range c.Seq {
		want = append(want, fresh[s])
	}
	st := edtest.StackOf(e)
	if !slices.Equal(st[len(st)-len(want):], want) {
		t.Fatalf("вершина стека %v, ждали %v", st, want)
	}
	if err := edtest.OrderInvariant(edtest.P(e).Spreads[0]); err != nil {
		t.Fatal(err)
	}
}

// TestPasteUndoOneStep — вставка группы — один шаг отмены и rev; Ctrl+Z — байт в байт, Ctrl+Y —
// снова то же; пустой снимок и неизвестный лист — холостая; круг Save→Load вставленное хранит.
func TestPasteUndoOneStep(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}, Stickers: []string{id.st, id.own}})
	orig, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: "s1"})
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 || len(out.Refs)+len(out.Texts)+len(out.Stickers) != 4 {
		t.Fatalf("шагов %d → %d, итог %+v", undo, edtest.UndoLen(e), out)
	}
	after := model.Marshal(edtest.P(e))
	q := edtest.SaveLoad(t, e)
	if s := q.Sheet("s1"); len(s.Grid) != 4 || len(s.Texts) != 2 || len(s.Stickers) != 4 {
		t.Fatalf("Save→Load: мест %d, надписей %d, стикеров %d", len(s.Grid), len(s.Texts), len(s.Stickers))
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), orig) {
		t.Fatal("Ctrl+Z не вернул байт в байт")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("Ctrl+Y не повторил вставку")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	for _, tc := range []struct {
		c *model.Clip
		r model.PasteReq
	}{
		{&model.Clip{}, model.PasteReq{Sheet: "s1"}},
		{nil, model.PasteReq{Sheet: "s1"}},
		{c, model.PasteReq{Sheet: "нет"}},
	} {
		if _, out, err := e.PasteClip(tc.c, tc.r); err != nil || len(out.Texts) != 0 {
			t.Fatalf("холостая: %v %+v", err, out)
		}
	}
	edtest.Same(t, e, rev, undo, "пустая вставка")
}

// TestPasteStickerLibRecord — свой стикер вырезан (запись снята PruneRefs) и вставлен: запись
// возвращается из снимка с тем же ID и ключом библиотеки.
func TestPasteStickerLibRecord(t *testing.T) {
	e, id := clipEd(t)
	want := edtest.P(e).Photos[id.ownRef]
	_, c, ok, _ := e.CutClip("s1", model.ClipSel{Stickers: []string{id.own}})
	if _, has := edtest.P(e).Photos[id.ownRef]; !ok || has {
		t.Fatalf("вырезан %v, запись осталась %v", ok, has)
	}
	out := mustPaste(t, e, &c, model.PasteReq{Sheet: "s2"})
	got, has := edtest.P(e).Photos[id.ownRef]
	if !has || got.Lib != want.Lib || got.Path != want.Path || stickerOf(t, e, out.Stickers[0]).Photo != id.ownRef {
		t.Fatalf("запись стикера %+v (была %+v)", got, want)
	}
}

// TestPasteBorderRecord — место с рамочкой-картинкой вырезано и вставлено: рамочка та же, запись
// картинки возвращается из снимка; фото места — с прежней записью.
func TestPasteBorderRecord(t *testing.T) {
	e, id := clipEd(t)
	want := edtest.P(e).Photos[id.frameR]
	_, c, ok, _ := e.CutClip("s1", model.ClipSel{Refs: []string{"a"}})
	if _, has := edtest.P(e).Photos[id.frameR]; !ok || has {
		t.Fatalf("вырезано %v, запись рамочки осталась %v", ok, has)
	}
	out := mustPaste(t, e, &c, model.PasteReq{Sheet: "s1"})
	pl := sheetByID(e, "s1").PlaceRef(out.Refs[0])
	got := edtest.P(e).Photos[id.frameR]
	if pl.Border == nil || pl.Border.Photo != id.frameR || got.Lib != want.Lib || pl.Photo != "a" {
		t.Fatalf("рамочка %+v, запись %+v", pl.Border, got)
	}
	if err := edtest.BorderInvariant(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	// Снимок не делит рамочку с проектом.
	if pl.Border == c.Places[0].Border {
		t.Fatal("рамочка вставленного места — указатель снимка")
	}
}

// TestPasteBorderReopen — Ctrl+X места со своей рамочкой и своего стикера, Ctrl+V (и вторая
// вставка того же снимка с «Перенести сюда»): записи картинок — те же, что у ApplyBorder и
// AddSticker, и переживают Save → Load: путь даёт искатель библиотеки по ключу (048: в файле пути
// своего нет). Без искателя путь пуст у любой своей записи, вставленной и нет, — ключ и размеры
// стоят (пропавшее до RelinkLib). Отмена — шаг, вставка на неизвестный лист — холостая.
func TestPasteBorderReopen(t *testing.T) {
	e, id := clipEd(t)
	p := edtest.P(e)
	wantFr, wantOwn := p.Photos[id.frameR], p.Photos[id.ownRef]
	edtest.LibFinder(t, map[string]string{wantFr.Lib: wantFr.Path, wantOwn.Lib: wantOwn.Path})
	_, c, ok, _ := e.CutClip("s1", model.ClipSel{Refs: []string{id.a}, Stickers: []string{id.own}})
	if _, has := edtest.P(e).Photos[id.frameR]; !ok || has {
		t.Fatalf("вырезано %v, запись рамочки осталась %v", ok, has)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, out, err := e.PasteClip(&c, model.PasteReq{Sheet: "нет такого"}); err != nil || len(out.Refs) > 0 {
		t.Fatalf("вставка на неизвестный лист: %v %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "вставка на неизвестный лист")
	out := mustPaste(t, e, &c, model.PasteReq{Sheet: "s1"})
	if _, out2, err := e.PasteClip(&c, model.PasteReq{Sheet: "s1", Repeat: model.RepeatMove}); err != nil || len(out2.Refs) != 1 {
		t.Fatalf("вторая вставка: %v %+v", err, out2)
	}
	check := func(what string, q *model.Project, fr, own model.PhotoRef) {
		t.Helper()
		if got := q.Photos[id.frameR]; got != fr {
			t.Fatalf("%s: запись рамочки %+v, ждали %+v", what, got, fr)
		}
		if got := q.Photos[id.ownRef]; got != own {
			t.Fatalf("%s: запись стикера %+v, ждали %+v", what, got, own)
		}
		_, s := q.Find("s1")
		pl, st := s.PlaceRef(out.Refs[0]), s.Stickers[slices.IndexFunc(s.Stickers, func(st *model.Sticker) bool { return st.ID == out.Stickers[0] })]
		if pl == nil || pl.Border == nil || pl.Border.Photo != id.frameR || st.Photo != id.ownRef {
			t.Fatalf("%s: место %+v, стикер %+v", what, pl, *st)
		}
	}
	check("после вставки", edtest.P(e), wantFr, wantOwn)
	check("после Save → Load", edtest.SaveLoad(t, e), wantFr, wantOwn)
	edtest.LibFinder(t, map[string]string{}) // другой компьютер: своего нет в библиотеке
	lost := func(r model.PhotoRef) model.PhotoRef { r.Path = ""; return r }
	check("без искателя", edtest.SaveLoad(t, e), lost(wantFr), lost(wantOwn))
	undo = edtest.UndoLen(e)
	e.Undo()
	e.Undo()
	if _, has := edtest.P(e).Photos[id.frameR]; has || edtest.UndoLen(e) != undo-2 {
		t.Fatalf("после отмены вставок запись рамочки %v, шагов %d (было %d)", has, edtest.UndoLen(e), undo)
	}
	e.Redo()
	e.Redo()
	check("после возврата", edtest.P(e), wantFr, wantOwn)
}

// TestPasteMovedPhotoRestoresRef — Ctrl+X, Ctrl+V фото: тот же ID, кадр и зеркало из снимка,
// табличка места едет (оригинала нет). Ctrl+C, Del фото, Ctrl+V: место-оригинал есть — табличка
// обнуляется, фото — с кадром снимка.
func TestPasteMovedPhotoRestoresRef(t *testing.T) {
	e, id := clipEd(t)
	who := model.Person{Last: "Иванов", First: "Петя"}
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos["a"]
		r.Frame, r.FlipH = &model.Frame{S: 1.5, X: 0.1}, true
		p.Photos["a"] = r
		p.Spreads[0].Grid[0].Person = who
		p.Spreads[0].Grid[2].Person = who
	})
	_, c, _, _ := e.CutClip("s1", model.ClipSel{Refs: []string{id.a}})
	if _, has := edtest.P(e).Photos["a"]; has {
		t.Fatal("запись вырезанного фото осталась")
	}
	out := mustPaste(t, e, &c, model.PasteReq{Sheet: "s2"})
	if len(out.Refs) != 1 || out.Refs[0] != "a" {
		t.Fatalf("на «Авто» %+v", out)
	}
	e.Undo()
	out = mustPaste(t, e, &c, model.PasteReq{Sheet: "s1"})
	pl, r := sheetByID(e, "s1").PlaceRef(out.Refs[0]), edtest.P(e).Photos["a"]
	if pl.Photo != "a" || pl.Person != who || r.Frame == nil || *r.Frame != (model.Frame{S: 1.5, X: 0.1}) || !r.FlipH {
		t.Fatalf("перенос: место %+v, запись %+v", *pl, r)
	}
	c2 := mustTake(t, e, "s1", model.ClipSel{Refs: []string{"b"}})
	edtest.Change(e, func(p *model.Project) { r := p.Photos["b"]; r.FlipV = true; p.Photos["b"] = r })
	e.DeleteSelection("s1", []string{"b"}, nil, nil)
	out = mustPaste(t, e, c2, model.PasteReq{Sheet: "s1"})
	pl = sheetByID(e, "s1").PlaceRef(out.Refs[0])
	if pl.Photo != "b" || pl.Person != (model.Person{}) || edtest.P(e).Photos["b"].FlipV {
		t.Fatalf("оригинал есть: место %+v, запись %+v", *pl, edtest.P(e).Photos["b"])
	}
	edtest.MustGrid(t, e)
}

// TestDuplicateKeepsClip — Ctrl+D: копия выделенного на том же листе со сдвигом 5 мм, один шаг;
// буфер (снимок, взятый раньше) не меняется; пустое выделение — холостая.
func TestDuplicateKeepsClip(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s2", model.ClipSel{Refs: []string{"c"}})
	before := clipJSON(t, *c)
	src, st := textOf(t, e, id.text), stickerOf(t, e, id.st)
	undo := edtest.UndoLen(e)
	_, out, err := e.DuplicateClip("s1", model.ClipSel{Refs: []string{id.empty}, Texts: []string{id.text}, Stickers: []string{id.st}})
	if err != nil || len(out.Refs) != 1 || len(out.Texts) != 1 || len(out.Stickers) != 1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("дубль: %v %+v", err, out)
	}
	got, gs := textOf(t, e, out.Texts[0]), stickerOf(t, e, out.Stickers[0])
	wantXY(t, "надпись", got.X, got.Y, src.X+5, src.Y+5)
	wantXY(t, "стикер", gs.X, gs.Y, st.X+5, st.Y+5)
	if clipJSON(t, *c) != before {
		t.Fatal("дубль изменил буфер")
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, out, err := e.DuplicateClip("s1", model.ClipSel{Texts: []string{"нет"}}); err != nil || len(out.Texts) != 0 {
		t.Fatalf("пустой дубль: %v %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "пустой дубль")
}

// TestPasteRepeatAsk — фото снимка ещё стоит в месте альбома: вставка без ответа возвращает его в
// Repeat и ничего не меняет (ни надписи, ни шага, ни rev); чужой ответ — как без ответа.
func TestPasteRepeatAsk(t *testing.T) {
	e, id := clipEd(t)
	c := mustTake(t, e, "s1", model.ClipSel{Refs: []string{"a", id.empty}, Texts: []string{id.text}})
	orig, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
	for _, rep := range []string{"", "что-то"} {
		_, out, err := e.PasteClip(c, model.PasteReq{Sheet: "s2", Repeat: rep})
		if err != nil || !slices.Equal(out.Repeat, []string{"a"}) || len(out.Texts)+len(out.Refs) != 0 || out.Froze {
			t.Fatalf("вопрос %q: %v %+v", rep, err, out)
		}
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), orig) {
		t.Fatal("вопрос изменил проект")
	}
	edtest.Same(t, e, rev, undo, "вопрос")
	// Фото «Авто» тоже спрашивает.
	c = mustTake(t, e, "s2", model.ClipSel{Refs: []string{"d"}})
	if _, out, _ := e.PasteClip(c, model.PasteReq{Sheet: "s1"}); !slices.Equal(out.Repeat, []string{"d"}) {
		t.Fatalf("«Авто»: %+v", out)
	}
}

// TestPasteTwinsHook — точка связки с 098 (Twins): «Поставить копию» зовёт Copy с записью снимка и
// ставит её ID, «Перенести сюда» — Move и тот же ID; ошибка копии — ошибка вставки. Дубль — копии.
func TestPasteTwinsHook(t *testing.T) {
	e, id := clipEd(t)
	var got []model.PhotoRef
	errFull := errors.New("копий слишком много")
	tw := grid.Twins{
		Placed: (*model.Project).Placed,
		Copy: func(p *model.Project, r model.PhotoRef) (string, error) {
			if len(got) == 2 {
				return "", errFull
			}
			got = append(got, r)
			n := "copy" + string(rune('0'+len(got)))
			p.Photos[n] = r
			return n, nil
		},
		Move: func(p *model.Project, id string) { p.Detach([]string{id}) },
	}
	edtest.Change(e, func(p *model.Project) { r := p.Photos["a"]; r.Fade = 0.5; p.Photos["a"] = r })
	p := edtest.P(e).Clone()
	c, _ := grid.Take(p, "s1", model.ClipSel{Refs: []string{id.a}})
	out, err := grid.Paste(p, &c, model.PasteReq{Sheet: "s2", Repeat: model.RepeatCopy}, tw)
	if err != nil || !slices.Equal(out.Refs, []string{"copy1"}) || got[0].Fade != 0.5 || !p.Placed("a") || !slices.Contains(p.Sheet("s2").Photos, "copy1") {
		t.Fatalf("копия: %v %+v", err, out)
	}
	out, err = grid.Paste(p, &c, model.PasteReq{Sheet: "s2", Repeat: model.RepeatMove}, tw)
	if err != nil || !slices.Equal(out.Refs, []string{"a"}) || p.Sheet("s1").Grid[0].Photo != "" || !slices.Contains(p.Sheet("s2").Photos, "a") {
		t.Fatalf("перенос: %v %+v", err, out)
	}
	out, err = grid.Duplicate(p, "s2", model.ClipSel{Refs: []string{"c"}}, tw)
	if err != nil || !slices.Equal(out.Refs, []string{"copy2"}) {
		t.Fatalf("дубль: %v %+v", err, out)
	}
	if _, err := grid.Duplicate(p, "s2", model.ClipSel{Refs: []string{"d"}}, tw); !errors.Is(err, errFull) {
		t.Fatalf("ошибка копии: %v", err)
	}
}

// twinEd — фото с настоящими ID (по пути, как у ленты: у копий общий ID файла): разворот s1 «Авто»
// из a, b, c и s2 из d. У «a» кадр, зеркало и прозрачность. Возвращает ID листов и сведения о фото.
func twinEd(t *testing.T) (e *album.Editor, ph []model.PhotoInfo, s1, s2 string) {
	t.Helper()
	dir := testkit.TempDir(t)
	for _, n := range []string{"a", "b", "c", "d"} {
		path := dir + "/" + n + ".jpg"
		ph = append(ph, model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 300, H: 200})
	}
	e = edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[:3])
	edtest.NewSpread(e, -1, ph[3:])
	s1, s2 = edtest.P(e).Spreads[0].ID, edtest.P(e).Spreads[1].ID
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[ph[0].ID]
		r.Frame, r.Fade, r.FlipH = &model.Frame{S: 2}, 0.3, true
		p.Photos[ph[0].ID] = r
	})
	return e, ph, s1, s2
}

// TestPasteRepeatCopy — «Поставить копию»: фото, стоящие в альбоме, встают новыми записями 098 с
// кадром, зеркалом и прозрачностью из буфера (не с нынешними у оригинала); оригинал остаётся; один
// шаг отмены; повтор — следующий номер; копия копии — копия файла; дубль фото — копия без вопроса;
// все номера заняты — ошибка и проект прежний; круг Save→Load.
func TestPasteRepeatCopy(t *testing.T) {
	e, ph, s1, s2 := twinEd(t)
	a, b := ph[0].ID, ph[1].ID
	c := mustTake(t, e, s1, model.ClipSel{Refs: []string{a, b}})
	edtest.Change(e, func(p *model.Project) { // после снимка оригинал меняется
		r := p.Photos[a]
		r.Frame = &model.Frame{S: 3}
		p.Photos[a] = r
	})
	undo := edtest.UndoLen(e)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy})
	a2, b2 := model.CopyID(a, 2), model.CopyID(b, 2)
	if !slices.Equal(out.Refs, []string{a2, b2}) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("копии: %+v, шагов %d → %d", out, undo, edtest.UndoLen(e))
	}
	p := edtest.P(e)
	if !slices.Contains(p.Sheet(s2).Photos, a2) || !slices.Contains(p.Sheet(s2).Photos, b2) || !slices.Equal(p.Sheet(s1).Photos[:2], []string{a, b}) {
		t.Fatalf("листы: s1 %v, s2 %v", p.Sheet(s1).Photos, p.Sheet(s2).Photos)
	}
	if r := p.Photos[a2]; r.Frame == nil || r.Frame.S != 2 || r.Fade != 0.3 || !r.FlipH || p.Photos[a].Frame.S != 3 {
		t.Fatalf("запись копии %+v, оригинала %+v", r, p.Photos[a])
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}
	// Повтор — следующие номера; круг записи сохраняет обе копии.
	out = mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy})
	if !slices.Equal(out.Refs, []string{model.CopyID(a, 3), model.CopyID(b, 3)}) {
		t.Fatalf("повтор: %+v", out)
	}
	q := edtest.SaveLoad(t, e)
	if !q.Placed(a2) || !q.Placed(model.CopyID(b, 3)) || q.Photos[a2].Frame == nil || q.Photos[a2].Frame.S != 2 {
		t.Fatalf("после Save→Load копий нет: %v", q.Sheet(s2).Photos)
	}
	e.Undo() // повтор — один шаг: первая пара цела
	if p := edtest.P(e); p.Placed(model.CopyID(a, 3)) || !p.Placed(a2) {
		t.Fatalf("отмена повтора: %v", p.Sheet(s2).Photos)
	}
	// Копия копии — запись того же файла, следующий номер; дубль фото — копия сразу.
	cc := mustTake(t, e, s2, model.ClipSel{Refs: []string{a2}})
	out = mustPaste(t, e, cc, model.PasteReq{Sheet: s1, Repeat: model.RepeatCopy})
	if !slices.Equal(out.Refs, []string{model.CopyID(a, 3)}) {
		t.Fatalf("копия копии: %+v", out)
	}
	if _, out, err := e.DuplicateClip(s1, model.ClipSel{Refs: []string{ph[2].ID}}); err != nil || !slices.Equal(out.Refs, []string{model.CopyID(ph[2].ID, 2)}) {
		t.Fatalf("дубль фото: %v %+v", err, out)
	}
	// Копия b не помещается: номера заняты — всё или ничего, у a копии тоже не остаётся.
	edtest.Change(e, func(p *model.Project) { // записи без мест снял бы PruneRefs: ставим их по листам
		var ids []string
		for n := 3; n <= model.MaxCopy; n++ {
			id := model.CopyID(b, n)
			p.Photos[id] = p.Photos[b]
			ids = append(ids, id)
		}
		for len(ids) > 0 {
			k := min(len(ids), 30)
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: ids[:k]})
			ids = ids[k:]
		}
	})
	orig, rev, undo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)
	if _, out, err := e.PasteClip(c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy}); !errors.Is(err, model.ErrCopies) || len(out.Refs) != 0 {
		t.Fatalf("номера заняты: %v %+v", err, out)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), orig) {
		t.Fatal("отказ изменил проект")
	}
	edtest.Same(t, e, rev, undo, "отказ копии")
}

// TestPasteRepeatMove — «Перенести сюда»: все записи файла уходят с прежних мест, встаёт запись с
// ID снимка (прежняя, если есть: фото переезжает таким, каким его видно); место на «Ручной»
// остаётся пустым; если стоит не сам снимок, а копия файла, — уходит она, а запись снимка
// возвращается из буфера; один шаг отмены.
func TestPasteRepeatMove(t *testing.T) {
	e, ph, s1, s2 := twinEd(t)
	a, b, c3 := ph[0].ID, ph[1].ID, ph[2].ID
	c := mustTake(t, e, s1, model.ClipSel{Refs: []string{a, b}})
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[a]
		r.Frame = &model.Frame{S: 3}
		p.Photos[a] = r
	})
	undo := edtest.UndoLen(e)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatMove})
	p := edtest.P(e)
	if !slices.Equal(out.Refs, []string{a, b}) || edtest.UndoLen(e) != undo+1 || !slices.Equal(p.Sheet(s1).Photos, []string{c3}) ||
		!slices.Contains(p.Sheet(s2).Photos, a) || !slices.Contains(p.Sheet(s2).Photos, b) || p.Photos[a].Frame.S != 3 {
		t.Fatalf("перенос: %+v, s1 %v, s2 %v, шагов %d → %d", out, p.Sheet(s1).Photos, p.Sheet(s2).Photos, undo, edtest.UndoLen(e))
	}
	e.Undo()
	if p := edtest.P(e); !slices.Equal(p.Sheet(s1).Photos, []string{a, b, c3}) || slices.Contains(p.Sheet(s2).Photos, a) {
		t.Fatalf("отмена переноса: s1 %v, s2 %v", p.Sheet(s1).Photos, p.Sheet(s2).Photos)
	}

	// Копия файла стоит на s2, исходную вырезали: вопрос, «Перенести» — копия уходит, запись a из буфера.
	mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy})
	cut, ok := e.TakeClip(s1, model.ClipSel{Refs: []string{a}})
	if !ok {
		t.Fatal("нечего копировать")
	}
	e.RemovePhotos([]string{a})
	_, out, err := e.PasteClip(&cut, model.PasteReq{Sheet: s1})
	if err != nil || !slices.Equal(out.Repeat, []string{a}) {
		t.Fatalf("вопрос о копии: %v %+v", err, out)
	}
	out = mustPaste(t, e, &cut, model.PasteReq{Sheet: s1, Repeat: model.RepeatMove})
	p = edtest.P(e)
	if !slices.Equal(out.Refs, []string{a}) || p.Placed(model.CopyID(a, 2)) || !slices.Contains(p.Sheet(s1).Photos, a) || p.Photos[a].Frame == nil || p.Photos[a].Frame.S != 3 {
		t.Fatalf("перенос копии: %+v, s1 %v, s2 %v, запись a %+v", out, p.Sheet(s1).Photos, p.Sheet(s2).Photos, p.Photos[a])
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}

	// «Ручной»: место источника остаётся пустым.
	e.SetGrid(s1, true)
	place := edtest.P(e).Sheet(s1).Grid
	gc := mustTake(t, e, s1, model.ClipSel{Refs: []string{c3}})
	mustPaste(t, e, gc, model.PasteReq{Sheet: s2, Repeat: model.RepeatMove})
	p = edtest.P(e)
	if g := p.Sheet(s1).Grid; len(g) != len(place) || slices.ContainsFunc(g, func(pl *model.Place) bool { return pl.Photo == c3 }) || !p.Placed(c3) || p.Placed(c3) && slices.Contains(p.Sheet(s1).Photos, c3) {
		t.Fatalf("«Ручной»: места %d → %d, фото %v", len(place), len(g), p.Sheet(s1).Photos)
	}
}

// TestPasteRepeatMoveFamily — «Перенести сюда», когда файл стоит и исходной записью, и копиями на
// разных листах: с мест снимаются все записи семьи (не только первая), вставленная остаётся одна;
// второй файл снимка при этом переезжает как обычно; один шаг отмены, отмена возвращает всё.
func TestPasteRepeatMoveFamily(t *testing.T) {
	e, ph, s1, s2 := twinEd(t)
	a, b := ph[0].ID, ph[1].ID
	c := mustTake(t, e, s1, model.ClipSel{Refs: []string{a, b}})
	mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy}) // a-2, b-2 на s2
	mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy}) // a-3, b-3 на s2
	edtest.NewSpread(e, -1, nil)
	s3 := edtest.P(e).Spreads[2].ID
	// Копии стоят на s2 и s3, исходная — на s1: три записи файла a на трёх местах.
	mustPaste(t, e, c, model.PasteReq{Sheet: s3, Repeat: model.RepeatCopy})
	p := edtest.P(e)
	if n := len(p.Family(a)); n != 4 {
		t.Fatalf("семья a: %v", p.Family(a))
	}
	edtest.NewSpread(e, -1, nil)
	s5 := edtest.P(e).Spreads[3].ID
	undo := edtest.UndoLen(e)
	out := mustPaste(t, e, c, model.PasteReq{Sheet: s5, Repeat: model.RepeatMove})
	p = edtest.P(e)
	var onSheets []string
	for _, s := range p.Sheets() {
		for _, x := range s.Photos {
			if model.BaseID(x) == a {
				onSheets = append(onSheets, x)
			}
		}
	}
	if edtest.UndoLen(e) != undo+1 || !slices.Equal(onSheets, []string{a}) || !slices.Equal(p.Sheet(s5).Photos, []string{a, b}) || len(out.Refs) != 2 {
		t.Fatalf("перенос семьи: на листах %v, s5 %v, шагов %d → %d, %+v", onSheets, p.Sheet(s5).Photos, undo, edtest.UndoLen(e), out)
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}
	e.Undo()
	if p := edtest.P(e); len(p.Sheet(s5).Photos) != 0 || !slices.Contains(p.Sheet(s1).Photos, a) || !slices.Contains(p.Sheet(s3).Photos, model.CopyID(a, 4)) {
		t.Fatalf("отмена: s1 %v, s3 %v, s5 %v", p.Sheet(s1).Photos, p.Sheet(s3).Photos, p.Sheet(s5).Photos)
	}
}

// TestPasteFitsFamilyFile — запись из снимка встаёт со свойствами файла семьи (FamilyInvariant): между
// копированием и вставкой другая запись семьи получила новые W/H (Remember из другого места).
// Две ветви вставки: файл нигде не стоит, а запись семьи есть (фон) — «нет вопроса»; файл стоит
// копией — «Перенести сюда».
func TestPasteFitsFamilyFile(t *testing.T) {
	// Запись a-2 на фоне s2 (в местах не стоит): новые размеры файла.
	e, ph, s1, s2 := twinEd(t)
	a, a2 := ph[0].ID, model.CopyID(ph[0].ID, 2)
	c := mustTake(t, e, s1, model.ClipSel{Refs: []string{a}})
	e.RemovePhotos([]string{a})
	edtest.Change(e, func(p *model.Project) {
		r := c.Refs[a].Copy()
		r.W, r.H, r.Size = 600, 400, 1234
		p.Photos[a2] = r
		p.Sheet(s2).BgPhoto = &model.Backdrop{Photo: a2}
	})
	mustPaste(t, e, c, model.PasteReq{Sheet: s1})
	p := edtest.P(e)
	if r := p.Photos[a]; r.W != 600 || r.H != 400 || r.Size != 1234 || !slices.Contains(p.Sheet(s1).Photos, a) {
		t.Fatalf("фон: запись a %+v, s1 %v", r, p.Sheet(s1).Photos)
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}

	// Копия a-2 стоит на s2 с новыми размерами; исходную вырезали; «Перенести сюда».
	e, ph, s1, s2 = twinEd(t)
	a, a2 = ph[0].ID, model.CopyID(ph[0].ID, 2)
	c = mustTake(t, e, s1, model.ClipSel{Refs: []string{a}})
	mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy})
	e.RemovePhotos([]string{a})
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[a2]
		r.W, r.H = 900, 600
		p.Photos[a2] = r
	})
	mustPaste(t, e, c, model.PasteReq{Sheet: s1, Repeat: model.RepeatMove})
	p = edtest.P(e)
	if r := p.Photos[a]; r.W != 900 || r.H != 600 || !slices.Contains(p.Sheet(s1).Photos, a) {
		t.Fatalf("перенос: запись a %+v, s1 %v", r, p.Sheet(s1).Photos)
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}
}

// TestPasteRepeatAskFamily — предикат 098 «по файлу»: фото вырезано, но его копия стоит — вставка
// спрашивает (Repeat — ID снимка) и ничего не меняет; копию убрали — встаёт без вопроса, записью
// из буфера.
func TestPasteRepeatAskFamily(t *testing.T) {
	e, ph, s1, s2 := twinEd(t)
	a := ph[0].ID
	c := mustTake(t, e, s1, model.ClipSel{Refs: []string{a}})
	mustPaste(t, e, c, model.PasteReq{Sheet: s2, Repeat: model.RepeatCopy})
	e.RemovePhotos([]string{a})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, out, err := e.PasteClip(c, model.PasteReq{Sheet: s1}); err != nil || !slices.Equal(out.Repeat, []string{a}) {
		t.Fatalf("копия стоит: %v %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "вопрос о копии")
	e.RemovePhotos([]string{model.CopyID(a, 2)})
	out := mustPaste(t, e, c, model.PasteReq{Sheet: s1})
	if p := edtest.P(e); !slices.Equal(out.Refs, []string{a}) || p.Photos[a].Frame == nil || p.Photos[a].Frame.S != 2 {
		t.Fatalf("без копий: %+v, запись %+v", out, p.Photos[a])
	}
}
