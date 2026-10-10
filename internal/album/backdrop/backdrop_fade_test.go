package backdrop_test

import (
	"bytes"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Прозрачность фото фона (спека 017); фон при склейке и копировании оформления.

// Зажим 0..1, шаг 0.01; NaN/Inf, тот же итог, нет фона — холостые; сессия — один шаг.
func TestSetBackdropFade(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetBackdropFade(s1, "left", 0.5, "")
	edtest.Same(t, e, rev, undo, "нет фона")
	edtest.SetBg(t, e, s1, "left", "bg")
	if v := e.View(); v.Spreads[0].Backdrops[0].Fade != 0 {
		t.Fatalf("новый фон с прозрачностью: %+v", v.Spreads[0].Backdrops[0])
	}
	cases := []struct{ in, want float64 }{
		{0.4, 0.4}, {0.401, 0.4}, {0.005, 0.01}, {-1, 0}, {2, 1}, {1, 1}, {0, 0}, {-0.001, 0}, {0.4, 0.4},
	}
	for _, c := range cases {
		v := e.SetBackdropFade(s1, "left", c.in, "")
		if b := edtest.BdOf(e, s1, "left"); b.Fade != c.want || v.Spreads[0].Backdrops[0].Fade != c.want {
			t.Errorf("%v: в проекте %v, во View %v, ждали %v", c.in, b.Fade, v.Spreads[0].Backdrops[0].Fade, c.want)
		}
	}
	edtest.SetDirty(e, false)
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0.4, 0.4004} {
		e.SetBackdropFade(s1, "left", f, "")
	}
	e.SetBackdropFade(s1, "top", 0.7, "")
	e.SetBackdropFade("нет", "left", 0.7, "")
	e.SetBackdropFade(s1, "right", 0.7, "")
	edtest.Same(t, e, rev, undo, "холостые")
	if edtest.Dirty(e) {
		t.Fatal("холостая правка пометила проект изменённым")
	}
	// Сессия ползунка — один шаг отмены.
	for i := range 5 {
		e.SetBackdropFade(s1, "left", 0.5+float64(i)/100, "fade1")
	}
	if edtest.UndoLen(e) != undo+1 || edtest.BdOf(e, s1, "left").Fade != 0.54 {
		t.Fatalf("сессия: %d шагов, fade %v", edtest.UndoLen(e)-undo, edtest.BdOf(e, s1, "left").Fade)
	}
	// Без сессии — шаг на каждый вызов; Undo возвращает прежнее значение целиком.
	e.SetBackdropFade(s1, "left", 0.3, "")
	e.SetBackdropFade(s1, "left", 0.2, "")
	if edtest.UndoLen(e) != undo+3 {
		t.Fatalf("без сессии: %d шагов", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	e.Undo()
	if u := e.Undo(); u.Spreads[0].Backdrops[0].Fade != 0.4 {
		t.Fatalf("после отмены fade %v", u.Spreads[0].Backdrops[0].Fade)
	}
	// Ноль в файл не пишется; −0 зажим даёт +0 — в View (fade без omitempty) не будет «-0».
	v := e.SetBackdropFade(s1, "left", math.Copysign(0, -1), "")
	if f := v.Spreads[0].Backdrops[0].Fade; f != 0 || math.Signbit(f) {
		t.Fatalf("fade −0: %v", f)
	}
	if data := model.Marshal(edtest.P(e)); bytes.Contains(data, []byte(`"fade"`)) {
		t.Fatalf("fade 0 в JSON: %s", data)
	}
}

// Прозрачность едет с фоном: замена фото, кадр и зеркало, «По области», смена формата,
// копирование оформления; смена формы фона (страницы → разворот) — новый фон с 0.
func TestBackdropFadeTravels(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropFade(s1, "left", 0.4, "")
	fade := func(sp, scope, what string) {
		t.Helper()
		if b := edtest.BdOf(e, sp, scope); b == nil || b.Fade != 0.4 {
			t.Fatalf("%s: %+v", what, b)
		}
	}
	edtest.SetBg(t, e, s1, "left", "bg2")
	fade(s1, "left", "замена фото")
	e.FlipBackdrop(s1, "left", "h")
	e.SetBackdropFrame(s1, "left", 2, 0.1, 0)
	e.SetBackdropBox(s1, "left", 10, 10, 100, 100, 20, "")
	fade(s1, "left", "зеркало, кадр, рамка")
	e.FitBackdrop(s1, "left")
	fade(s1, "left", "по области")
	s := model.DefaultSettings()
	s.PageW, s.PageH = 150, 600
	e.SetSettings(s)
	fade(s1, "left", "смена формата")
	e.Undo()
	if _, _, err := e.CopyGrid(s1, "left", []model.GridTarget{{Spread: s2, Scope: "right"}}, true, false); err != nil {
		t.Fatal(err)
	}
	fade(s2, "right", "копирование оформления")
	edtest.SetBg(t, e, s1, "spread", "bg3")
	if b := edtest.BdOf(e, s1, "spread"); b.Fade != 0 {
		t.Fatalf("новая форма фона унаследовала прозрачность: %+v", b)
	}
}

// Склейка: фон первого остаётся, без него — берётся фон второго; копирование оформления переносит фон.
func TestBackdropMergeAndCopy(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	edtest.SetBg(t, e, s2, "right", "bg")
	e.SetBackdropBox(s2, "right", 320, 10, 100, 100, 10, "")
	v := e.MergeSpreads(s1)
	if len(v.Spreads) != 1 || len(v.Spreads[0].Backdrops) != 1 || v.Spreads[0].Backdrops[0].Box.Rot != 10 {
		t.Fatalf("фон второго при склейке: %+v", v.Spreads)
	}
	e.Undo()
	edtest.SetBg(t, e, s1, "spread", "bg1")
	v = e.MergeSpreads(s1)
	if b := v.Spreads[0].Backdrops; len(b) != 1 || b[0].Photo != "bg1" {
		t.Fatalf("фон первого должен остаться: %+v", b)
	}
	if _, ok := e.Ref("bg"); ok {
		t.Fatal("фон второго остался в Photos")
	}
	e.Undo()
	e.Undo()
	// Делим: фон остаётся первому.
	edtest.SetBg(t, e, s1, "left", "bg1")
	v = e.SplitSpread(s1, 1)
	if len(v.Spreads) != 3 || len(v.Spreads[0].Backdrops) != 1 || len(v.Spreads[1].Backdrops) != 0 {
		t.Fatalf("деление: %+v", v.Spreads)
	}
}

func TestBackdropCopyDesign(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	if _, _, err := e.ApplyGrid(s1, "left", model.TplPage, []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}); err != nil {
		t.Fatal(err)
	}
	edtest.SetBg(t, e, s1, "left", "bg")
	e.SetBackdropBox(s1, "left", 20, 30, 150, 150, 0, "")
	e.SetBackdropFrame(s1, "left", 2, 0, 0)
	if _, _, err := e.CopyGrid(s1, "left", []model.GridTarget{{Spread: s2, Scope: "right"}}, true, false); err != nil {
		t.Fatal(err)
	}
	b := edtest.BdOf(e, s2, "right")
	if b == nil || b.Photo != "bg" || b.X != 320 || b.Y != 30 || b.Frame == nil || b.Frame.S != 2 {
		t.Fatalf("фон не скопирован на правую: %+v", b)
	}
	if b == edtest.BdOf(e, s1, "left") || edtest.BdOf(e, s2, "left") != nil {
		t.Fatal("копия общая с источником или попала не туда")
	}
	// Без design фон не копируется.
	e.ClearBackdrop(s2, "right")
	if _, _, err := e.CopyGrid(s1, "left", []model.GridTarget{{Spread: s2, Scope: "right"}}, false, false); err != nil {
		t.Fatal(err)
	}
	if edtest.BdOf(e, s2, "right") != nil {
		t.Fatal("фон скопирован без design")
	}
	v, err := e.DuplicateGrid(s1, 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads[1].Backdrops) != 1 || v.Spreads[1].Backdrops[0].Box.X != 20 {
		t.Fatalf("размножение с design: %+v", v.Spreads[1].Backdrops)
	}
}

// Фон не берёт зеркало и кадр из PhotoRef: у одного фото — свои у места и у фона.
func TestBackdropIgnoresPhotoFlip(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	e.SetFlip([]string{"a"}, "h")
	e.SetFrame("a", 2, 0.5, 0.5)
	v := edtest.SetBg(t, e, s1, "spread", "a")
	b := v.Spreads[0].Backdrops[0]
	if b.FlipH || b.FlipV || b.Frame != (model.Frame{S: 1}) {
		t.Fatalf("фон взял свойства места: %+v", b)
	}
	if !v.Spreads[0].Frames[0].FlipH {
		t.Fatalf("место потеряло зеркало: %+v", v.Spreads[0].Frames[0])
	}
	e.FlipBackdrop(s1, "spread", "v")
	if r, _ := e.Ref("a"); r.FlipV {
		t.Fatal("зеркало фона попало в PhotoRef")
	}
}
