package backdrop_test

import (
	"errors"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Узор и своё из библиотеки (048 G4): ApplyPattern, ApplyBackdrop, SetPattern, ClearPattern.

func sheetOf(e *album.Editor, id string) *model.Spread {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	return edtest.P(e).Sheet(id)
}

func apply(t *testing.T, e *album.Editor, sp, scope string, pt model.Pattern, c string) {
	t.Helper()
	if _, err := e.ApplyPattern(sp, scope, pt, c); err != nil {
		t.Fatal(err)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Инвариант узоров ловит лист с узором всего листа и страницы сразу и незажатый узор.
func TestPatInvariant(t *testing.T) {
	dots := &model.Pattern{Kind: "dots"}
	for _, c := range []struct {
		name string
		s    model.Spread
		bad  bool
	}{
		{"пусто", model.Spread{}, false},
		{"лист", model.Spread{Pat: dots}, false},
		{"страницы", model.Spread{PatL: dots, PatR: dots}, false},
		{"лист и левая", model.Spread{Pat: dots, PatL: dots}, true},
		{"лист и правая", model.Spread{Pat: dots, PatR: dots}, true},
		{"не зажат", model.Spread{PatL: &model.Pattern{Kind: "dots", Scale: 9}}, true},
	} {
		if err := edtest.PatInvariant(&c.s); (err != nil) != c.bad {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}

// Плитка программы на весь лист: узор (зажатый), цвет обеих страниц, фото фона всех областей
// сняты, второй разворот не тронут; один шаг, отмена возвращает всё; файл помнит узор.
func TestApplyPatternSpread(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	edtest.SetBg(t, e, s2, "spread", "bg3")
	apply(t, e, s2, "spread", model.Pattern{Kind: "stripes"}, "")
	undo := edtest.UndoLen(e)

	apply(t, e, s1, "spread", model.Pattern{Kind: "dots", Inks: []string{"#AABBCC", "#000000"}, Scale: 2.004, Fade: 0.333}, "#F0E0D0")
	s := sheetOf(e, s1)
	want := model.Pattern{Kind: "dots", Inks: []string{"#aabbcc"}, Scale: 2, Fade: 0.33}
	if s.Pat == nil || s.Pat.Kind != want.Kind || !slices.Equal(s.Pat.Inks, want.Inks) || s.Pat.Scale != 2 || s.Pat.Fade != 0.33 {
		t.Fatalf("узор %+v, ждали %+v", s.Pat, want)
	}
	if s.PatL != nil || s.PatR != nil || len(s.Backdrops()) != 0 || s.BgL != "#f0e0d0" || s.BgR != "#f0e0d0" {
		t.Fatalf("фон листа: L=%v R=%v фото=%v цвет=%q/%q", s.PatL, s.PatR, s.Backdrops(), s.BgL, s.BgR)
	}
	if _, ok := e.Ref("bg"); ok {
		t.Fatal("снятое фото фона осталось в Photos")
	}
	if o := sheetOf(e, s2); o.Pat == nil || o.Pat.Kind != "stripes" || o.BgPhoto != nil {
		t.Fatalf("второй разворот: %+v %+v", o.Pat, o.BgPhoto)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}

	q := edtest.SaveLoad(t, e)
	if p := q.Spreads[0].Pat; p == nil || p.Kind != "dots" || !slices.Equal(p.Inks, want.Inks) || p.Scale != 2 || p.Fade != 0.33 {
		t.Fatalf("после Save → Load: %+v", p)
	}
	if q.Spreads[1].Pat == nil || q.Spreads[1].Pat.Kind != "stripes" {
		t.Fatalf("второй после Save → Load: %+v", q.Spreads[1].Pat)
	}

	e.Undo()
	s = sheetOf(e, s1)
	if s.Pat != nil || s.BgPhotoL == nil || s.BgPhotoR == nil || s.BgL != "" || s.BgR != "" {
		t.Fatalf("после отмены: pat=%v L=%v R=%v цвет=%q/%q", s.Pat, s.BgPhotoL, s.BgPhotoR, s.BgL, s.BgR)
	}
}

// Плитка на страницу: узор всего листа остаётся другой странице, фото всего листа снимается
// (его не разрезать), фото другой страницы стоит; "" — цвет страниц не трогается.
func TestApplyPatternPage(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	e.SetBackground(s1, "spread", "#112233")
	apply(t, e, s1, "spread", model.Pattern{Kind: "stripes"}, "")
	edtest.SetBg(t, e, s1, "spread", "bg")

	apply(t, e, s1, "left", model.Pattern{Kind: "dots"}, "")
	s := sheetOf(e, s1)
	if s.Pat != nil || s.PatL == nil || s.PatL.Kind != "dots" || s.PatR == nil || s.PatR.Kind != "stripes" {
		t.Fatalf("узоры: S=%v L=%v R=%v", s.Pat, s.PatL, s.PatR)
	}
	if s.BgPhoto != nil || s.BgL != "#112233" || s.BgR != "#112233" {
		t.Fatalf("фото листа %v, цвет %q/%q", s.BgPhoto, s.BgL, s.BgR)
	}

	edtest.SetBg(t, e, s1, "left", "bg2")
	edtest.SetBg(t, e, s1, "right", "bg3")
	apply(t, e, s1, "right", model.Pattern{Kind: "bands"}, "#445566")
	s = sheetOf(e, s1)
	if s.PatL == nil || s.PatL.Kind != "dots" || s.PatR == nil || s.PatR.Kind != "bands" || s.BgPhotoL == nil || s.BgPhotoR != nil {
		t.Fatalf("правая: L=%v R=%v фото L=%v R=%v", s.PatL, s.PatR, s.BgPhotoL, s.BgPhotoR)
	}
	if s.BgL != "#112233" || s.BgR != "#445566" {
		t.Fatalf("цвет %q/%q", s.BgL, s.BgR)
	}

	// Узор всего листа снимает узоры страниц.
	apply(t, e, s1, "spread", model.Pattern{Kind: "stripes"}, "")
	if s = sheetOf(e, s1); s.Pat == nil || s.PatL != nil || s.PatR != nil || len(s.Backdrops()) != 0 {
		t.Fatalf("весь лист: S=%v L=%v R=%v фото=%v", s.Pat, s.PatL, s.PatR, s.Backdrops())
	}
}

// Отказы и холостые: неверный ключ или цвет — ErrPattern, область — ErrScope (и у страницы
// листа 031), неизвестный разворот и тот же узор повторно — без шага и rev.
func TestApplyPatternNoop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	apply(t, e, s1, "left", model.Pattern{Kind: "dots", Inks: []string{"#aabbcc"}}, "#ffffff")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)

	cases := []struct {
		name, sp, scope, color string
		pt                     model.Pattern
		want                   error
	}{
		{"ключ не по грамматике", s1, "left", "", model.Pattern{Kind: "Dots!"}, model.ErrPattern},
		{"пустой ключ", s1, "left", "", model.Pattern{}, model.ErrPattern},
		{"цвет", s1, "left", "red", model.Pattern{Kind: "dots"}, model.ErrPattern},
		{"область", s1, "top", "", model.Pattern{Kind: "dots"}, model.ErrScope},
		{"пустая область", s1, "", "", model.Pattern{Kind: "dots"}, model.ErrScope},
		{"разворот", "нет", "left", "", model.Pattern{Kind: "dots"}, nil},
		{"тот же", s1, "left", "#FFFFFF", model.Pattern{Kind: "dots", Inks: []string{"#AABBCC"}, Scale: 1}, nil},
	}
	for _, c := range cases {
		if _, err := e.ApplyPattern(c.sp, c.scope, c.pt, c.color); !errors.Is(err, c.want) {
			t.Fatalf("%s: ошибка %v, ждали %v", c.name, err, c.want)
		}
		edtest.Same(t, e, rev, undo, c.name)
	}

	// Лист 031: страниц нет — только весь лист.
	sh := edtest.Open(t, model.NewSheet(200, 300), "")
	sid := edtest.NewSpread(sh, -1, edtest.Photos("a")).Spreads[0].ID
	if _, err := sh.ApplyPattern(sid, "left", model.Pattern{Kind: "dots"}, ""); !errors.Is(err, model.ErrScope) {
		t.Fatalf("страница листа 031: %v", err)
	}
	apply(t, sh, sid, "spread", model.Pattern{Kind: "dots"}, "#010203")
	if s := sheetOf(sh, sid); s.Pat == nil || s.BgL != "#010203" || s.BgR != "#010203" {
		t.Fatalf("лист 031: %+v %q/%q", s.Pat, s.BgL, s.BgR)
	}
}

// Неизвестный, но правильный ключ (узор из новой версии) хранится и переживает файл (решение №91).
func TestApplyPatternUnknownKind(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	apply(t, e, s1, "spread", model.Pattern{Kind: "future-kind", Inks: []string{"#010203", "#040506"}, Seed: 7}, "")
	q := edtest.SaveLoad(t, e)
	if p := q.Spreads[0].Pat; p == nil || p.Kind != "future-kind" || len(p.Inks) != 2 || p.Seed != 7 {
		t.Fatalf("после Save → Load: %+v", p)
	}
}

// libPhoto — своё из библиотеки: ID от ключа.
func libPhoto(key, path string) model.PhotoInfo {
	return model.PhotoInfo{ID: model.LibPhotoID(key), Path: path, Lib: key, W: 400, H: 300, Size: 10}
}

// Своё из галереи: фото фона с ключом, узоры области сняты (узор листа — другой странице),
// один шаг; «Свой файл…» (SetBackdrop) узор не снимает; в файле — ключ, путь от искателя.
func TestApplyBackdropOwn(t *testing.T) {
	const key, key2 = "my:0123456789abcdef", "my:fedcba9876543210"
	lib := filepath.Join(testkit.TempDir(t), "Фоны", "0123456789abcdef.jpg")
	edtest.PutFile(t, lib, 10)
	edtest.LibFinder(t, map[string]string{key: lib})
	e, s1, s2 := edtest.BdEd(t)
	apply(t, e, s1, "spread", model.Pattern{Kind: "stripes"}, "")
	apply(t, e, s2, "spread", model.Pattern{Kind: "dots"}, "")
	undo := edtest.UndoLen(e)

	ph := libPhoto(key, lib)
	if _, err := e.ApplyBackdrop(s1, "left", ph); err != nil {
		t.Fatal(err)
	}
	s := sheetOf(e, s1)
	if s.BgPhotoL == nil || s.BgPhotoL.Photo != ph.ID || s.PatL != nil || s.Pat != nil || s.PatR == nil || s.PatR.Kind != "stripes" {
		t.Fatalf("своё на левую: фото=%v узоры S=%v L=%v R=%v", s.BgPhotoL, s.Pat, s.PatL, s.PatR)
	}
	if r, ok := e.Ref(ph.ID); !ok || r.Lib != key || r.Path != lib {
		t.Fatalf("запись своего: %+v %v", r, ok)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	rev := edtest.Rev(e)
	if _, err := e.ApplyBackdrop(s1, "left", ph); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo+1, "то же своё")

	// «Свой файл…» узор не снимает: фото с прозрачностью поверх узора.
	edtest.SetBg(t, e, s2, "spread", "bg")
	if o := sheetOf(e, s2); o.Pat == nil || o.BgPhoto == nil {
		t.Fatalf("SetBackdrop снял узор: %+v %+v", o.Pat, o.BgPhoto)
	}

	q := edtest.SaveLoad(t, e)
	if r, ok := q.Photos[ph.ID]; !ok || r.Lib != key || r.Path != lib || q.Spreads[0].BgPhotoL.Photo != ph.ID {
		t.Fatalf("после Save → Load: %+v %v", r, ok)
	}

	// Ключ не по грамматике или ID не от ключа — ErrLibKey, проект не тронут.
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	bad := []model.PhotoInfo{libPhoto("my:XYZ", lib), {ID: "чужой", Path: lib, Lib: key2, W: 4, H: 3}}
	for _, b := range bad {
		if _, err := e.ApplyBackdrop(s1, "right", b); !errors.Is(err, model.ErrLibKey) {
			t.Fatalf("%+v: ошибка %v", b, err)
		}
		if _, err := e.SetBackdrop(s1, "right", b); !errors.Is(err, model.ErrLibKey) {
			t.Fatalf("SetBackdrop %+v: ошибка %v", b, err)
		}
		edtest.Same(t, e, rev, undo, "плохой ключ")
	}
	if _, err := e.ApplyBackdrop(s1, "top", ph); !errors.Is(err, model.ErrScope) {
		t.Fatalf("область: %v", err)
	}
	edtest.Same(t, e, rev, undo, "плохая область")

	e.Undo()
	e.Undo()
	if s = sheetOf(e, s1); s.BgPhotoL != nil || s.Pat == nil || s.Pat.Kind != "stripes" {
		t.Fatalf("после отмены: фото=%v узор=%v", s.BgPhotoL, s.Pat)
	}
	if _, ok := e.Ref(ph.ID); ok {
		t.Fatal("после отмены запись своего осталась")
	}
}

// Ползунок узора: серия по сессии — один шаг, ключ генератора стоит, зажим; без узора, NaN и
// неизвестная область — холостые.
func TestSetPattern(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	apply(t, e, s1, "left", model.Pattern{Kind: "dots-scatter", Seed: 3}, "")
	apply(t, e, s2, "spread", model.Pattern{Kind: "dots"}, "")
	undo := edtest.UndoLen(e)

	for _, f := range []float64{0.1, 0.2, 0.5} {
		e.SetPattern(s1, "left", model.Pattern{Kind: "dots", Fade: f, Scale: 1.5, Seed: 9}, "fade-1")
	}
	s := sheetOf(e, s1)
	if s.PatL.Kind != "dots-scatter" || s.PatL.Fade != 0.5 || s.PatL.Scale != 1.5 || s.PatL.Seed != 9 {
		t.Fatalf("узор после серии: %+v", s.PatL)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("серия: шагов %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	if o := sheetOf(e, s2); o.Pat.Kind != "dots" || o.Pat.Fade != 0 {
		t.Fatalf("второй разворот: %+v", o.Pat)
	}

	e.SetPattern(s1, "left", model.Pattern{Inks: []string{"#ABCDEF"}, Scale: 99, Fade: -3, Seed: 9}, "")
	if s = sheetOf(e, s1); s.PatL.Scale != 4 || s.PatL.Fade != 0 || !slices.Equal(s.PatL.Inks, []string{"#abcdef"}) {
		t.Fatalf("зажим: %+v", s.PatL)
	}

	// Вид отдаёт цвета дополненными цветами генератора: прислать их обратно — не правка.
	var full []string
	for _, pv := range e.View().Spreads[0].Patterns {
		if pv.Scope == "left" {
			full = pv.Pat.Inks
		}
	}
	if len(full) < 2 || full[0] != "#abcdef" {
		t.Fatalf("цвета узора в виде: %v", full)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for name, f := range map[string]func(){
		"тот же":        func() { e.SetPattern(s1, "left", model.Pattern{Inks: []string{"#abcdef"}, Scale: 4, Seed: 9}, "") },
		"цвета из вида": func() { e.SetPattern(s1, "left", model.Pattern{Inks: full, Scale: 4, Seed: 9}, "") },
		"без узора":     func() { e.SetPattern(s1, "right", model.Pattern{Kind: "dots", Fade: 0.5}, "") },
		"NaN":           func() { e.SetPattern(s1, "left", model.Pattern{Fade: math.NaN()}, "") },
		"область":       func() { e.SetPattern(s1, "top", model.Pattern{Fade: 0.5}, "") },
		"разворот":      func() { e.SetPattern("нет", "left", model.Pattern{Fade: 0.5}, "") },
		"узор листа":    func() { e.SetPattern(s1, "spread", model.Pattern{Fade: 0.5}, "") },
		"снять ничто":   func() { e.ClearPattern(s1, "right") },
		"снять чужое":   func() { e.ClearPattern(s1, "top") },
	} {
		f()
		edtest.Same(t, e, rev, undo, name)
	}

	e.Undo()
	e.Undo()
	if s = sheetOf(e, s1); s.PatL.Fade != 0 || s.PatL.Scale != 0 || s.PatL.Seed != 3 {
		t.Fatalf("после отмены серии: %+v", s.PatL)
	}
}

// «Убрать» узор: шаг, цвет страниц и фото фона остаются; отмена возвращает.
func TestClearPattern(t *testing.T) {
	e, s1, s2 := edtest.BdEd(t)
	apply(t, e, s1, "spread", model.Pattern{Kind: "dots"}, "#123456")
	apply(t, e, s2, "spread", model.Pattern{Kind: "dots"}, "")
	edtest.SetBg(t, e, s1, "spread", "bg")
	undo := edtest.UndoLen(e)

	e.ClearPattern(s1, "spread")
	s := sheetOf(e, s1)
	if s.Pat != nil || s.BgPhoto == nil || s.BgL != "#123456" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("убрать: pat=%v фото=%v цвет=%q шагов=%d", s.Pat, s.BgPhoto, s.BgL, edtest.UndoLen(e))
	}
	if sheetOf(e, s2).Pat == nil {
		t.Fatal("снят узор другого разворота")
	}
	q := edtest.SaveLoad(t, e)
	if q.Spreads[0].Pat != nil || q.Spreads[1].Pat == nil {
		t.Fatalf("после Save → Load: %+v %+v", q.Spreads[0].Pat, q.Spreads[1].Pat)
	}
	e.Undo()
	if sheetOf(e, s1).Pat == nil {
		t.Fatal("отмена не вернула узор")
	}
}

// Обложка: сторонки — страницы, узор ставится и на весь лист.
func TestApplyPatternCover(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	apply(t, e, model.CoverID, "left", model.Pattern{Kind: "dots"}, "#203040")
	s := sheetOf(e, model.CoverID)
	if s.PatL == nil || s.BgL != "#203040" || s.BgR != "" {
		t.Fatalf("задняя сторонка: %+v %q/%q", s.PatL, s.BgL, s.BgR)
	}
	q := edtest.SaveLoad(t, e)
	if q.Cover == nil || q.Cover.Sheet.PatL == nil || q.Cover.Sheet.PatL.Kind != "dots" {
		t.Fatal("узор обложки потерян при Save → Load")
	}
}
