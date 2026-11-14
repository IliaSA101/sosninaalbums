package layers_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Стикеры (спека 048 Ф2) в потоках листов: пропорция, растяжимые фигуры, копия с оформлением,
// дубль, разрез и склейка, формат.

// TestFitStickers — своя пропорция держится вокруг центра; края (MinSticker, 2·лист), центр на
// листе; свободная (неизвестный ключ, растяжимая) — как есть; идемпотентна; несколько сразу.
func TestFitStickers(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t) // лист 600×300
	a := addSticker(t, e, s1, 0, own(keyA, 400, 200))
	f := addSticker(t, e, s1, 1, art("zz-future"))
	for _, c := range []struct {
		x, y, w, h     float64
		wantW, wantH   float64
		wantCX, wantCY float64
		name           string
	}{
		{100, 100, 80, 10, 80, 40, 140, 105, "высота по ширине"},
		{100, 100, 1, 1, 6, 3, 101.5, 101.5, "меньше MinSticker: угол стоит, затем пропорция"},
		{0, 0, 5000, 10, 1200, 600, 600, 5, "больше двух листов, центр на листе"},
		{-500, 400, 40, 20, 40, 20, 0, 300, "центр за листом"},
	} {
		e.SetStickerBox(s1, a, c.x, c.y, c.w, c.h, 0, "")
		st := stOf(e, a)
		if cx, cy := center(st); st.W != c.wantW || st.H != c.wantH || !near(cx, c.wantCX) || !near(cy, c.wantCY) {
			t.Errorf("%s: %+v, центр (%v, %v)", c.name, *st, cx, cy)
		}
	}
	e.SetStickerBox(s1, f, 300, 100, 120, 20, 0, "")
	if st := stOf(e, f); st.W != 120 || st.H != 20 {
		t.Fatalf("неизвестный ключ — пропорция свободна: %+v", *st)
	}
	p := edtest.P(e).Clone()
	before := string(model.Marshal(p))
	p.FitStickers()
	if string(model.Marshal(p)) != before {
		t.Fatal("FitStickers не идемпотентна")
	}
	// Напрямую: фото без размеров — как есть; неверная высота у нескольких — чинится у каждого.
	p.Photos["nodim"] = model.PhotoRef{Path: edtest.Abs("n.png")}
	_, s := p.Find(s1)
	s.Stickers = append(s.Stickers,
		&model.Sticker{ID: "k1", Photo: "nodim", X: 10, Y: 10, W: 50, H: 7},
		&model.Sticker{ID: "k2", Photo: model.LibPhotoID(keyA), X: 10, Y: 10, W: 50, H: 7},
		&model.Sticker{ID: "k3", Photo: model.LibPhotoID(keyA), X: 200, Y: 10, W: 30, H: 30})
	p.FitStickers()
	if n := len(s.Stickers); s.Stickers[n-3].H != 7 || s.Stickers[n-2].H != 25 || s.Stickers[n-1].H != 15 ||
		!near(s.Stickers[n-1].Y+7.5, 25) {
		t.Fatalf("несколько: %+v %+v %+v", *s.Stickers[n-3], *s.Stickers[n-2], *s.Stickers[n-1])
	}
}

// TestStickerStretch — растяжимая плашка 120×20 цела после правки, FitStickers и Save → Load
// (решение №136); вставка — с пропорцией Meta.Aspect.
func TestStickerStretch(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	k := addSticker(t, e, s1, 0, art("plate"))
	if st := stOf(e, k); st.W/st.H != 3 {
		t.Fatalf("первая вставка: %+v", *st)
	}
	e.SetStickerBox(s1, k, 20, 30, 120, 20, 0, "")
	q := edtest.SaveLoad(t, e)
	q.FitStickers()
	if s, i := q.FindSticker(k); s == nil || s.Stickers[i].W != 120 || s.Stickers[i].H != 20 || s.Stickers[i].X != 20 {
		t.Fatalf("после Save → Load: %s", model.Marshal(q))
	}
	if st := stOf(e, k); st.W != 120 || st.H != 20 {
		t.Fatalf("после правки: %+v", *st)
	}
}

// copyEd — сетка s1 (место на каждой странице), лист s2 «Авто»; на левой s1 — текст и два
// стикера (своё — над текстом), на правой — стикер; на s2 — стикер (он заменится).
func copyEd(t *testing.T) (e *album.Editor, left []string, right, old string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(300, 10, 100, 100, "b")}, []string{"x"})
	tx := edtest.AddText(t, e, "s1", 0)
	k1 := addSticker(t, e, "s1", 0, art("plate", "#112233"))
	k2 := addSticker(t, e, "s1", 0, own(keyA, 400, 200))
	e.ArrangeLayers("s1", "front", []string{k2})
	right = addSticker(t, e, "s1", 1, art("plate"))
	old = addSticker(t, e, "s2", 0, art("plate"))
	return e, []string{k1, tx, k2}, right, old
}

// TestStickerCopyGrid — «Копировать раскладку» с оформлением: стикеры области — копии с новыми
// ID, сдвиг на страницу, порядок слоёв источника; стикеры цели в области заменены; без
// оформления — не копируются; «Размножить» с оформлением — тоже; пределы — ошибка без шага.
func TestStickerCopyGrid(t *testing.T) {
	e, left, right, old := copyEd(t)
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}, {Spread: "s1", Scope: model.ScopeRight}}, true, false); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	s2 := p.Spreads[1]
	if len(s2.Stickers) != 2 || slices.ContainsFunc(s2.Stickers, func(st *model.Sticker) bool { return st.ID == old || st.ID == left[0] || st.ID == left[2] }) {
		t.Fatalf("цель s2: %s", model.Marshal(p))
	}
	k1, k2 := stOf(e, left[0]), stOf(e, left[2])
	if c := s2.Stickers[0]; c.X != k1.X || c.Art.Inks[0] != "#112233" || s2.Stickers[1].Photo != k2.Photo {
		t.Fatalf("копии на s2: %+v %+v", *c, *s2.Stickers[1])
	}
	// Порядок слоёв источника: плашка, текст, своё (своё — над текстом).
	st, tc := s2.Stack(), s2.Texts[len(s2.Texts)-1].ID
	if i, j, k := slices.Index(st, s2.Stickers[0].ID), slices.Index(st, tc), slices.Index(st, s2.Stickers[1].ID); i >= j || j >= k {
		t.Fatalf("порядок слоёв копии %v", st)
	}
	s1 := p.Spreads[0]
	var onRight []*model.Sticker
	for _, st := range s1.Stickers {
		if st.X+st.W/2 > 281 {
			onRight = append(onRight, st)
		}
	}
	if len(s1.Stickers) != 4 || len(onRight) != 2 || onRight[0].X != k1.X+281 || slices.ContainsFunc(onRight, func(st *model.Sticker) bool { return st.ID == right }) {
		t.Fatalf("правая s1: %+v", s1.Stickers)
	}
	edtest.MustGrid(t, e)

	// Без оформления стикеры цели стоят, источника — не копируются.
	n := len(edtest.P(e).Spreads[1].Stickers)
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s2", Scope: model.ScopeRight}}, false, false); err != nil {
		t.Fatal(err)
	}
	if len(edtest.P(e).Spreads[1].Stickers) != n {
		t.Fatal("копия без оформления тронула стикеры")
	}
	// Размножить с оформлением: все стикеры листа, новые ID.
	if _, err := e.DuplicateGrid("s1", 2, true); err != nil {
		t.Fatal(err)
	}
	for _, d := range edtest.P(e).Spreads[1:3] {
		if len(d.Stickers) != 4 || d.Stickers[0].ID == s1.Stickers[0].ID {
			t.Fatalf("дубль: %+v", d.Stickers)
		}
	}
	edtest.MustGrid(t, e)

	// Пределы: на листе > MaxStickers и в проекте > MaxTotalStickers — ошибка, шага нет.
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		for len(s.Stickers) < model.MaxStickers {
			s.Stickers = append(s.Stickers, &model.Sticker{ID: model.NewID(), Art: &model.Art{Kind: "plate"}, X: 10, Y: 10, W: 30, H: 10})
		}
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, []model.GridTarget{{Spread: "s1", Scope: model.ScopeRight}}, true, false); !errors.Is(err, model.ErrStickers) {
		t.Fatalf("сверх листа: %v", err)
	}
	if _, err := e.DuplicateGrid("s1", model.MaxTotalStickers/model.MaxStickers, true); err == nil {
		t.Fatal("сверх проекта: нет ошибки")
	}
	edtest.Same(t, e, rev, undo, "пределы стикеров")
}

// TestStickerSplitMerge — склейка «Авто» дописывает стикеры второго (и его слои — выше), сверх
// MaxStickers — не клеит; разрез оставляет стикеры первому.
func TestStickerSplitMerge(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	a := addSticker(t, e, s1, 0, art("plate"))
	t1 := edtest.AddText(t, e, s1, 0)
	t2 := edtest.AddText(t, e, s2, 0)
	b := addSticker(t, e, s2, 0, own(keyA, 400, 200))
	e.ArrangeLayers(s2, "front", []string{b})
	e.MergeSpreads(s1)
	p := edtest.P(e)
	if len(p.Spreads) != 1 || len(p.Spreads[0].Stickers) != 2 {
		t.Fatalf("склейка: %s", model.Marshal(p))
	}
	if got, want := p.Spreads[0].Stack(), []string{a, t1, t2, b}; !slices.Equal(got, want) {
		t.Fatalf("стек склейки %v, ждали %v", got, want)
	}
	edtest.MustGrid(t, e)
	e.SplitSpread(p.Spreads[0].ID, 1)
	p = edtest.P(e)
	if len(p.Spreads) != 2 || len(p.Spreads[0].Stickers) != 2 || p.Spreads[1].Stickers != nil {
		t.Fatalf("разрез: %s", model.Marshal(p))
	}
	edtest.MustGrid(t, e)
	// Сверх MaxStickers — не клеит (холостая).
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[1]
		for len(s.Stickers) < model.MaxStickers-1 {
			s.Stickers = append(s.Stickers, &model.Sticker{ID: model.NewID(), Art: &model.Art{Kind: "plate"}, X: 10, Y: 10, W: 30, H: 10})
		}
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.MergeSpreads(edtest.P(e).Spreads[0].ID)
	edtest.Same(t, e, rev, undo, "склейка сверх MaxStickers")
}

// TestStickerRescale — смена формата: центр стикера — на том же месте страницы, ширина — по
// ширине; своя пропорция держится, свободная — по осям.
func TestStickerRescale(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t) // страница 300×300
	a := addSticker(t, e, s1, 1, own(keyA, 400, 200))
	f := addSticker(t, e, s1, 0, art("plate"))
	e.SetStickerBox(s1, f, 30, 60, 90, 30, 0, "")
	wa, wf := stOf(e, a), stOf(e, f)
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 150, 200
	e.SetSettings(st)
	ax, ay := center(wa)
	if got := stOf(e, a); got.W != wa.W/2 || got.H != got.W/2 || !near(got.X+got.W/2, ax/2) || !near(got.Y+got.H/2, ay*2/3) {
		t.Fatalf("своё: %+v, было %+v", *got, *wa)
	}
	if got := stOf(e, f); got.W != 45 || got.H != 20 || got.X != 15 || got.Y != 40 {
		t.Fatalf("плашка: %+v, было %+v", *got, *wf)
	}
	edtest.MustGrid(t, e)
	stickerRescaleEdges(t)
}

// stickerRescaleEdges — свободная фигура после смены формата не выходит из MinSticker..2·лист
// (048 ревью): плашка 60×4 и 4×60 при 300×300 → 200×200 — 3 мм вокруг центра, а не 2,67; высота
// сверх предела листа — предел; Save→Load рамку не меняет.
func stickerRescaleEdges(t *testing.T) {
	t.Helper()
	e, s1, _ := edtest.NewTwo(t) // страница 300×300
	thin := addSticker(t, e, s1, 0, art("plate"))
	tall := addSticker(t, e, s1, 1, art("plate"))
	e.SetStickerBox(s1, thin, 100, 148, 60, 4, 0, "")
	e.SetStickerBox(s1, tall, 400, 120, 4, 60, 0, "")
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 200, 200
	e.SetSettings(st)
	if got := stOf(e, thin); got.W != 40 || got.H != model.MinSticker || !near(got.X+got.W/2, 260./3) || !near(got.Y+got.H/2, 100) {
		t.Fatalf("тонкая по высоте: %+v", *got)
	}
	if got := stOf(e, tall); got.W != model.MinSticker || got.H != 40 || !near(got.X+got.W/2, 268) || !near(got.Y+got.H/2, 100) {
		t.Fatalf("тонкая по ширине: %+v", *got)
	}
	edtest.MustGrid(t, e)

	// Сверх предела: лист 400×200 (предел 800), высота 800 → страница 50×1000 (×5 по высоте, предел 2000).
	e.SetStickerBox(s1, tall, 100, -300, 10, 800, 0, "")
	st.PageW, st.PageH = 50, 1000
	e.SetSettings(st)
	if got := stOf(e, tall); got.H != 2000 || got.W != model.MinSticker {
		t.Fatalf("сверх предела: %+v", *got)
	}
	edtest.MustGrid(t, e)
	q := edtest.SaveLoad(t, e)
	for _, id := range []string{thin, tall} {
		if s, i := q.FindSticker(id); s == nil || !reflect.DeepEqual(s.Stickers[i], stOf(e, id)) {
			t.Fatalf("Save→Load поменял рамку %s: было %+v", id, stOf(e, id))
		}
	}
}

// TestStickerOverBackdrop — один исходник фоном и стикером (одна запись Photos по ключу, 048
// ревью): стикер не переписывает запись фона на копию 4096 px (фон печатается из исходника), в
// том числе замена и холостая на неизвестном листе; фон после стикера — запись на исходник.
func TestStickerOverBackdrop(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	orig := func(key string) model.PhotoInfo {
		return model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs("orig-" + key[3:] + ".jpg"), Lib: key, W: 8000, H: 4000}
	}
	ref := func(key string) model.PhotoRef { return edtest.P(e).Photos[model.LibPhotoID(key)] }
	if _, err := e.ApplyBackdrop(s1, "left", orig(keyA)); err != nil {
		t.Fatal(err)
	}
	want := ref(keyA)
	a1 := addSticker(t, e, s1, 1, own(keyA, 4096, 2048))
	a2 := addSticker(t, e, s2, 0, own(keyA, 4096, 2048))
	if got := ref(keyA); !reflect.DeepEqual(got, want) {
		t.Fatalf("стикер переписал запись фона: %+v, было %+v", got, want)
	}
	for _, id := range []string{a1, a2} {
		if st := stOf(e, id); st.H != st.W/2 {
			t.Fatalf("пропорция по записи фона: %+v", *st)
		}
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, id, err := e.AddSticker("нет", 0, own(keyA, 4096, 2048)); err != nil || id != "" {
		t.Fatalf("неизвестный лист: %q, %v", id, err)
	}
	edtest.Same(t, e, rev, undo, "стикер фона на неизвестный лист")

	// Обратный порядок: стикер, затем фон — запись на исходник; замена стикера её не трогает.
	b := addSticker(t, e, s2, 1, own(keyB, 4096, 2048))
	if _, err := e.ApplyBackdrop(s2, "spread", orig(keyB)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ReplaceSticker(s1, a1, own(keyB, 4096, 2048)); err != nil {
		t.Fatal(err)
	}
	if got := ref(keyB); got.Path != orig(keyB).Path || got.W != 8000 || stOf(e, a1).Photo != model.LibPhotoID(keyB) {
		t.Fatalf("фон после стикера и замена: %+v", got)
	}
	q := edtest.SaveLoad(t, e) // путь своего Load берёт по ключу у библиотеки (здесь её нет)
	for _, key := range []string{keyA, keyB} {
		if got := q.Photos[model.LibPhotoID(key)]; got.Lib != key || got.W != 8000 || got.H != 4000 {
			t.Fatalf("Save→Load %s: %+v", key, got)
		}
	}
	// Фон снят — стикер снова пишет свою копию (запись больше не общая).
	e.ClearBackdrop(s2, "spread")
	e.DeleteSelection(s2, nil, nil, []string{b})
	if _, _, err := e.ReplaceSticker(s2, a2, own(keyB, 4096, 2048)); err != nil {
		t.Fatal(err)
	}
	if got := ref(keyB); got.W != 4096 {
		t.Fatalf("без фона: %+v", got)
	}
	edtest.MustGrid(t, e)
}
