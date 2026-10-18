package edit

import (
	"errors"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// gridKit — gridMethods со счётчиками: разворот-сетка s1 [a, пусто] и «Авто» s2 [b].
func gridKit(t testing.TB) *kit {
	t.Helper()
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Grid: []*model.Place{
			{ID: "00000000000a", X: 12, Y: 12, W: 100, H: 100, Photo: "a"},
			{ID: "00000000000b", X: 312, Y: 12, W: 100, H: 100}}},
		{ID: "s2", Photos: []string{"b"}},
	}
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: `C:\b.jpg`, W: 300, H: 200}
	return newKit(t, p)
}

// person — имя фото в проекте (nil — из имени файла).
func (k *kit) person(id string) *model.Person {
	r, _ := k.ed.Ref(id)
	return r.Person
}

// Имя на фото и общий стиль подписей листа: правка в виде и в проекте, сброс, холостые.
func TestPersonAndSheetCaps(t *testing.T) {
	k := twoSpreads(t)
	r0 := k.rev()
	v := k.SetPerson("a", model.Person{Last: "Иванов", First: "Пётр"}, "")
	if p := k.person("a"); p == nil || p.Last != "Иванов" || p.First != "Пётр" || v.Rev <= r0 || k.calls != 1 {
		t.Errorf("SetPerson: %+v, rev %d → %d, Changed %d", p, r0, v.Rev, k.calls)
	}
	// Все поля пусты — снова из имени файла.
	if k.SetPerson("a", model.Person{}, ""); k.person("a") != nil {
		t.Error("имя не сброшено")
	}
	// Фото другого разворота и неизвестное: чужое имя не появляется.
	rev := k.rev()
	if v = k.SetPerson("нет", model.Person{Last: "Х"}, ""); v.Rev != rev {
		t.Errorf("SetPerson неизвестного фото: rev %d → %d", rev, v.Rev)
	}

	v = k.SetSheetCaps("s1", true, model.SheetCaps{Pad: 3, Text: "{Фамилия}"})
	if c := v.Spreads[0].Caps; c == nil || c.Pad != 3 || c.Text != "{Фамилия}" {
		t.Fatalf("SetSheetCaps: %+v", c)
	}
	if v.Spreads[1].Caps != nil {
		t.Error("стиль листа попал на соседний разворот")
	}
	if v = k.SetSheetCaps("s1", false, model.SheetCaps{}); v.Spreads[0].Caps != nil {
		t.Error("стиль листа не снят")
	}
	rev = k.rev()
	if v = k.SetSheetCaps("s1", false, model.SheetCaps{}); v.Rev != rev {
		t.Errorf("повторное снятие: rev %d → %d", rev, v.Rev)
	}
}

// Табличка места (071): правка в проекте и Changed, сессия — один шаг, пусто — снять,
// чужое место — холостая; SetPerson фото на месте с табличкой правит её.
func TestSetPlacePersonFlow(t *testing.T) {
	k := gridKit(t)
	seat := func(i int) model.Person { p, _ := k.ed.Snapshot(); return p.Spreads[0].Grid[i].Person }
	r0 := k.rev()
	k.SetPlacePerson("s1", "00000000000b", model.Person{Last: "Пет"}, "s")
	v := k.SetPlacePerson("s1", "00000000000b", model.Person{Last: "Петров"}, "s")
	if seat(1) != (model.Person{Last: "Петров"}) || v.Rev <= r0 || k.calls != 2 {
		t.Fatalf("табличка: %+v, rev %d → %d, Changed %d", seat(1), r0, v.Rev, k.calls)
	}
	// Одна сессия — один шаг: отмена снимает весь набор.
	if k.ed.Undo(); seat(1) != (model.Person{}) {
		t.Fatalf("отмена сессии: %+v", seat(1))
	}
	k.ed.Redo()
	rev := k.rev()
	if v = k.SetPlacePerson("s1", "нет", model.Person{Last: "Х"}, ""); v.Rev != rev {
		t.Fatalf("чужое место: rev %d → %d", rev, v.Rev)
	}
	k.SetPlacePerson("s1", "00000000000a", model.Person{Last: "Орлов"}, "")
	k.SetPerson("a", model.Person{Last: "Козлов"}, "")
	if seat(0) != (model.Person{Last: "Козлов"}) || k.person("a") != nil {
		t.Fatalf("SetPerson на месте с табличкой: %+v, у фото %+v", seat(0), k.person("a"))
	}
	if k.SetPlacePerson("s1", "00000000000b", model.Person{}, ""); seat(1) != (model.Person{}) {
		t.Fatalf("не снята: %+v", seat(1))
	}
}

// Окно «Имена»: предпросмотр — только чтение; плохой список и неизвестный лист — ошибка.
func TestPreviewNamesReadOnly(t *testing.T) {
	k := twoSpreads(t)
	rev := k.rev()
	plan, err := k.PreviewNames("s1", model.NamesOpts{From: "list", Text: "Иванов Пётр\nПетров Иван"})
	if err != nil || plan.Seats != 2 || k.rev() != rev || k.calls != 0 {
		t.Errorf("PreviewNames: err %v, мест %d, rev %d → %d, Changed %d", err, plan.Seats, rev, k.rev(), k.calls)
	}
	if _, err = k.PreviewNames("нет", model.NamesOpts{From: "list", Text: "Иванов"}); err == nil {
		t.Error("неизвестный лист без ошибки")
	}
}

// Фон-фото: ошибки не доходят до Changed, правки рамки/кадра/наклона/прозрачности/зеркала
// видны в виде, обе области живут независимо, серия с merge — один шаг отмены.
func TestBackdropWiring(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("L")

	if _, err := k.SetBackdrop("s1", "сбоку", "L"); !errors.Is(err, ErrBackdropScope) {
		t.Errorf("область: %v", err)
	}
	if _, err := k.SetBackdrop("s1", "left", "нет"); !errors.Is(err, ErrNoPhoto) {
		t.Errorf("фото: %v", err)
	}
	if k.calls != 0 {
		t.Fatalf("ошибки дошли до Changed: %d", k.calls)
	}

	// Фото ленты слева, фото альбома справа.
	v, err := k.SetBackdrop("s1", "left", "L")
	if err != nil || len(v.Spreads[0].Backdrops) != 1 || v.Spreads[0].Backdrops[0].Photo != "L" {
		t.Fatalf("SetBackdrop(left): err %v", err)
	}
	if v, err = k.SetBackdrop("s1", "right", "c"); err != nil || len(v.Spreads[0].Backdrops) != 2 {
		t.Fatalf("SetBackdrop(right): err %v, %d фонов", err, len(v.Spreads[0].Backdrops))
	}
	left := func(v model.View) model.BackdropView {
		t.Helper()
		for _, b := range v.Spreads[0].Backdrops {
			if b.Scope == "left" {
				return b
			}
		}
		t.Fatal("фона left нет")
		return model.BackdropView{}
	}
	grew := func(name string, before int, v model.View) {
		t.Helper()
		if v.Rev <= before {
			t.Errorf("%s: правка не применена (rev %d → %d)", name, before, v.Rev)
		}
	}

	r := k.rev()
	v = k.SetBackdropBox("s1", "left", 10, 20, 150, 100, 0, "")
	grew("SetBackdropBox", r, v)
	if b := left(v).Box; b.X != 10 || b.Y != 20 || b.W != 150 || b.H != 100 {
		t.Errorf("рамка: %+v", b)
	}
	r = k.rev()
	v = k.SetBackdropFrame("s1", "left", 2, 0.5, -0.5)
	grew("SetBackdropFrame", r, v)
	if f := left(v).Frame; f.S != 2 || f.X != 0.5 || f.Y != -0.5 {
		t.Errorf("кадр: %+v", f)
	}
	r = k.rev()
	v = k.SetBackdropAngle("s1", "left", 12, "")
	grew("SetBackdropAngle", r, v)
	r = k.rev()
	v = k.StraightenBackdrop("s1", "left", 100, 3)
	grew("StraightenBackdrop", r, v)
	r = k.rev()
	v = k.FlipBackdrop("s1", "left", "h")
	grew("FlipBackdrop", r, v)
	if !left(v).FlipH {
		t.Error("зеркало не включено")
	}
	r = k.rev()
	v = k.FitBackdrop("s1", "left")
	grew("FitBackdrop", r, v)
	if left(v).Box.W != left(v).Area.W {
		t.Errorf("«По области»: рамка %+v, область %+v", left(v).Box, left(v).Area)
	}

	// Серия ползунка с одной сессией — один шаг отмены.
	r = k.rev()
	k.SetBackdropFade("s1", "left", 0.3, "fade1")
	v = k.SetBackdropFade("s1", "left", 0.6, "fade1")
	grew("SetBackdropFade", r, v)
	if left(v).Fade != 0.6 {
		t.Errorf("прозрачность: %v", left(v).Fade)
	}
	if v = k.ed.Undo(); left(v).Fade != 0 {
		t.Errorf("Undo серии вернул прозрачность %v, а не 0", left(v).Fade)
	}
	k.ed.Redo()

	// Левый фон снят, правый остался.
	if v = k.ClearBackdrop("s1", "left"); len(v.Spreads[0].Backdrops) != 1 || v.Spreads[0].Backdrops[0].Scope != "right" {
		t.Errorf("ClearBackdrop(left): %+v", v.Spreads[0].Backdrops)
	}
	// Повтор — холостая.
	r = k.rev()
	if v = k.ClearBackdrop("s1", "left"); v.Rev != r {
		t.Errorf("повторный ClearBackdrop: rev %d → %d", r, v.Rev)
	}
}

// Сетка: операции над несколькими местами; отказ Editor (деление мелкого места) —
// ошибка без Changed; чтения (PlanCopyGrid, GridChoices) проект не трогают и берут шаблоны
// из Host.
func TestGridLeftovers(t *testing.T) {
	k := gridKit(t)
	const pa, pb = "00000000000a", "00000000000b"
	tplCalls := 0
	k.h.Templates = func() []model.Template { tplCalls++; return nil }

	// Оба места одной правкой: равная ширина.
	k.SetPlaces("s1", []model.PlaceBox{{ID: pb, X: 300, Y: 20, W: 60, H: 80}}, "")
	r, calls := k.rev(), k.calls
	v, err := k.ArrangePlaces("s1", "sameW", []string{pa, pb})
	if err != nil || v.Rev <= r || k.calls != calls+1 {
		t.Fatalf("ArrangePlaces: err %v, rev %d → %d, Changed %d → %d", err, r, v.Rev, calls, k.calls)
	}
	if ps := v.Spreads[0].Places; ps[0].W != ps[1].W {
		t.Errorf("ширины после sameW: %v и %v", ps[0].W, ps[1].W)
	}
	// Неизвестное действие — холостая без ошибки.
	r = k.rev()
	if v, err = k.ArrangePlaces("s1", "нечто", []string{pa}); err != nil || v.Rev != r {
		t.Errorf("неизвестный op: err %v, rev %d → %d", err, r, v.Rev)
	}
	// Слишком мелкое для деления место — ошибка, события нет.
	k.SetPlaces("s1", []model.PlaceBox{{ID: pb, X: 300, Y: 20, W: 3, H: 3}}, "")
	calls = k.calls
	if _, err = k.ArrangePlaces("s1", "splitH", []string{pb}); err == nil || k.calls != calls {
		t.Errorf("деление мелкого места: err %v, Changed %d → %d", err, calls, k.calls)
	}

	// Чтения.
	r, calls = k.rev(), k.calls
	plan := k.PlanCopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: "spread"}}, false, false)
	if plan.Frozen != 1 {
		t.Errorf("PlanCopyGrid: %+v (ждали, что «Авто» s2 закрепится)", plan)
	}
	gc := k.GridChoices("s1", 2, "spread")
	if len(gc.Builtin) == 0 || gc.N != 2 || tplCalls != 1 {
		t.Errorf("GridChoices: %d сеток, N %d, шаблоны %d раз", len(gc.Builtin), gc.N, tplCalls)
	}
	if k.rev() != r || k.calls != calls {
		t.Errorf("чтение изменило проект: rev %d → %d, Changed %d → %d", r, k.rev(), calls, k.calls)
	}
	if !reflect.DeepEqual(gc, k.ed.GridChoices("s1", 2, "spread", nil)) {
		t.Error("GridChoices отдал не то, что считает Editor")
	}
}
