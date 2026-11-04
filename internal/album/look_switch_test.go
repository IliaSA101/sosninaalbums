package album_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Режим, смена макета по позициям и вопрос о тронутых (131 design §4, G4a).

// lkPrev — поиск, всегда отдающий t (прежний макет альбома).
func lkPrev(t model.AlbumTpl) album.LookFinder {
	return func(model.LookRef) *model.AlbumTpl { return &t }
}

// lkWithBg — редактор с одним пустым разворотом, на фон которого ставится картинка.
func lkWithBg(t *testing.T, lib bool) *album.Editor {
	t.Helper()
	e := lkEditor(t)
	v := edtest.NewSpread(e, 0, nil)
	ph := edtest.Photo("bg1")
	if lib {
		ph = model.PhotoInfo{ID: model.LibPhotoID(lkKeyB), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: lkKeyB}
	}
	if _, err := e.SetBackdrop(v.Spreads[0].ID, model.ScopeSpread, ph); err != nil {
		t.Fatal(err)
	}
	return e
}

// Фото из съёмки — в месте, на «Авто», на фоне без ключа библиотеки, на обложке — собранный
// альбом (Full, серия недоступна); шапка виньетки, надписи, фон из библиотеки и пустые места — нет.
func TestPlanLookMode(t *testing.T) {
	cases := []struct {
		name string
		mk   func(t *testing.T) *album.Editor
		full bool
	}{
		{"пустой альбом", lkEditor, false},
		{"фото в месте", func(t *testing.T) *album.Editor {
			return edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
		}, true},
		{"пустое место", func(t *testing.T) *album.Editor {
			return edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "")})
		}, false},
		{"фото на «Авто»", func(t *testing.T) *album.Editor { e, _ := edtest.AutoEd(t); return e }, true},
		{"фон без ключа", func(t *testing.T) *album.Editor { return lkWithBg(t, false) }, true},
		{"фон из библиотеки", func(t *testing.T) *album.Editor { return lkWithBg(t, true) }, false},
		{"фото обложки", func(t *testing.T) *album.Editor {
			e := lkEditor(t)
			ph := edtest.Photo("cv")
			if _, err := e.CreateCover("", &ph); err != nil {
				t.Fatal(err)
			}
			return e
		}, true},
		{"обложка без фото", func(t *testing.T) *album.Editor {
			e := lkEditor(t)
			if _, err := e.CreateCover("", nil); err != nil {
				t.Fatal(err)
			}
			return e
		}, false},
		{"надпись", func(t *testing.T) *album.Editor {
			e := lkEditor(t)
			v := edtest.NewSpread(e, 0, nil)
			edtest.AddText(t, e, v.Spreads[0].ID, 0)
			return e
		}, false},
		{"виньетка с шапкой", func(t *testing.T) *album.Editor {
			e := lkEditor(t)
			if _, err := e.AddVignette(-1); err != nil {
				t.Fatal(err)
			}
			if len(edtest.P(e).Spreads[0].Texts) == 0 {
				t.Fatal("в виньетке нет шапки — тест ничего не проверяет")
			}
			return e
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { lkCheckMode(t, c.mk(t), c.full) })
	}
}

// lkCheckMode — PlanLook и ApplyLook на альбоме e: Full, режим, чистота сухого прогона, отказ серии.
func lkCheckMode(t *testing.T, e *album.Editor, full bool) {
	t.Helper()
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	plan, err := e.PlanLook(lkRefA, lookA(), nil, model.LookOpts{Spreads: 9}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Full != full {
		t.Fatalf("Full = %v, ждали %v", plan.Full, full)
	}
	if full != (plan.Mode == "") || !full && plan.Spreads != 9 {
		t.Fatalf("режим %q, разворотов %d", plan.Mode, plan.Spreads)
	}
	if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatal("PlanLook изменил проект")
	}
	// Серия собранный альбом не трогает: отказ ErrLookMode без правки.
	_, plan, err = e.ApplyLook(lkRefA, lookA(), nil, model.LookOpts{Mode: model.LookSeries, Spreads: 9}, nil)
	if full != (err != nil) || (err != nil) && (e.View().Rev != rev || lkJSON(e) != data) {
		t.Fatalf("ApplyLook: err = %v, rev %d → %d", err, rev, e.View().Rev)
	}
	if full && plan != (model.LookPlan{}) {
		t.Fatalf("при отказе план должен быть пустым: %+v", plan)
	}
}

// Смена макета по позициям: начало и конец серии держат ID, лишние повторы уходят с конца блока,
// недостающие встают перед хвостом, свой разворот остаётся на месте.
func TestSwitchLookByPosition(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	a := lkRows(e) // шаги 1 2 3 4 5 4 6 7 8
	own := &model.Spread{ID: "own000000001", Photos: []string{}, BgL: "#123456"}
	edtest.Change(e, func(p *model.Project) { p.Spreads = slices.Insert(p.Spreads, 1, own) }) // между титулом и классом

	plan := lkApply(t, e, lkRefB, lookB(), lkPrev(lookA()), 6) // B: 0 1 | 2 2 2 | 3
	rows := lkRows(e)
	ids := func() []string {
		var out []string
		for _, r := range rows {
			out = append(out, r.ID)
		}
		return out
	}
	// Начало (2) и конец (1) — прежние; середина — первые три из шести повторов; свой между титулом и классом.
	want := []string{a[0].ID, own.ID, a[1].ID, a[2].ID, a[3].ID, a[4].ID, a[8].ID}
	if !slices.Equal(ids(), want) {
		t.Fatalf("ID после смены: %v\nждали %v", ids(), want)
	}
	if want := []int{1, 0, 2, 3, 3, 3, 4}; !slices.Equal(lkSteps(rows), want) {
		t.Fatalf("шаги: %v, ждали %v", lkSteps(rows), want)
	}
	if rows[2].Places == nil || !slices.Equal(rows[2].Places, a[1].Places) && !rows[2].Vig {
		t.Errorf("виньетка прежняя по ID, настройка новая: %+v", rows[2])
	}
	if !plan.Trial || plan.Touched != 0 || plan.Prev != "Классика" || plan.Spreads != 6 {
		t.Fatalf("план смены: %+v", plan)
	}
	if p := edtest.P(e); p.Look.Key != lkKeyB || p.Spreads[1].LookStep != 0 || p.Spreads[1].BgL != "#123456" {
		t.Fatalf("макет %v; свой разворот: %+v", p.Look, p.Spreads[1])
	}

	// Больше разворотов: недостающие повторы встают перед хвостом, хвост прежний.
	lkApply(t, e, lkRefB, lookB(), lkPrev(lookB()), 10)
	rows = lkRows(e)
	if len(rows) != 11 || rows[len(rows)-1].ID != a[8].ID || rows[0].ID != a[0].ID {
		t.Fatalf("после роста: %d разворотов, концы %v", len(rows), ids())
	}
	for i, id := range []string{a[0].ID, own.ID, a[1].ID, a[2].ID, a[3].ID, a[4].ID} {
		if rows[i].ID != id {
			t.Fatalf("позиция %d: %s, ждали %s", i, rows[i].ID, id)
		}
	}
	if want := []int{1, 0, 2, 3, 3, 3, 3, 3, 3, 3, 4}; !slices.Equal(lkSteps(rows), want) {
		t.Fatalf("шаги после роста: %v", lkSteps(rows))
	}
	// Виньетка на месте обычного шага — разворот заменяется (вид другой), ID новый.
	edtest.Change(e, func(p *model.Project) { p.Look = &model.LookRef{Key: lkKeyB, Name: "Свежий"} })
	swap := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkVig("Класс", 3), lkStep(model.RoleRepeat, "", "#abcdef", 2), lkStep("", "Конец", "#fedcba", 1)}}
	before := lkRows(e)
	lkApply(t, e, lkRefA, swap, lkPrev(lookB()), 5)
	rows = lkRows(e)
	if !rows[0].Vig || rows[0].ID == before[0].ID || rows[len(rows)-1].ID != before[len(before)-1].ID {
		t.Fatalf("замена вида: %+v", rows[0])
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
}

// Что считается правкой: сдвинутое место, свой фон, табличка имени, переименованная глава, своя
// надпись, виньетка с другим числом учеников — тронуто; макета нет на ПК — тронуты все.
func TestSwitchLookAsksTouched(t *testing.T) {
	vigID := func(p *model.Project) string {
		for _, s := range p.Spreads {
			if s.IsVignette() {
				return s.ID
			}
		}
		return ""
	}
	muts := map[string]func(t *testing.T, e *album.Editor){
		"сдвинутое место": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) { p.Spreads[2].Grid[0].X += 7 })
		},
		"свой фон": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) { p.Spreads[2].BgL = "#123456" })
		},
		"табличка имени": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) {
				p.Spreads[2].Grid[0].Person = model.Person{Last: "Иванов", First: "Иван"}
			})
		},
		"переименованная глава": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) { p.Spreads[2].Name = "Моя глава" })
		},
		"своя надпись": func(t *testing.T, e *album.Editor) {
			edtest.AddText(t, e, edtest.P(e).Spreads[2].ID, 0)
		},
		"учеников стало меньше": func(t *testing.T, e *album.Editor) {
			p := edtest.P(e)
			v := *p.Spreads[1].Vig
			v.Pupils = 2
			if _, _, err := e.SetVignette(vigID(p), v, nil); err != nil {
				t.Fatal(err)
			}
		},
		"сдвинутое место виньетки": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) { p.Spreads[1].Grid[0].X += 5 })
		},
		"своя заготовка снята": func(t *testing.T, e *album.Editor) {
			edtest.Change(e, func(p *model.Project) { p.Spreads[0].Texts = nil })
		},
	}
	for name, mut := range muts {
		t.Run(name, func(t *testing.T) {
			e := lkEditor(t)
			lkApply(t, e, lkRefA, lookA(), nil, 9)
			if plan, err := e.PlanLook(lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Spreads: 6}, nil); err != nil || plan.Touched != 0 || !plan.Trial {
				t.Fatalf("нетронутый альбом: %+v, %v", plan, err)
			}
			mut(t, e)
			plan, err := e.PlanLook(lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Spreads: 6}, nil)
			if err != nil || plan.Touched != 1 {
				t.Fatalf("после правки «%s»: тронутых %d (%v)", name, plan.Touched, err)
			}
			// Тронутый тоже заменяется: после постановки в альбоме нет следов правки.
			lkApply(t, e, lkRefB, lookB(), lkPrev(lookA()), 6)
			if plan := lkApply(t, e, lkRefB, lookB(), lkPrev(lookB()), 6); plan.Touched != 0 {
				t.Fatalf("после замены тронутых %d", plan.Touched)
			}
		})
	}
	t.Run("макета нет на ПК", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		plan, err := e.PlanLook(lkRefB, lookB(), nil, model.LookOpts{Spreads: 6}, nil)
		if err != nil || plan.Touched != 9 || !plan.Trial {
			t.Fatalf("без прежнего макета: %+v (%v)", plan, err)
		}
	})
	t.Run("шаг прежнего макета короче", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		plan, _ := e.PlanLook(lkRefB, lookB(), lkPrev(lookB()), model.LookOpts{Spreads: 6}, nil)
		if plan.Touched < 5 { // в B четыре шага: развороты 5–8 нечем сверить
			t.Fatalf("тронутых %d", plan.Touched)
		}
	})
	lkTouchedNoPlaces(t)
	t.Run("свои без шага не считаются", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		edtest.Change(e, func(p *model.Project) {
			p.Spreads = append(p.Spreads, &model.Spread{ID: "own000000002", Photos: []string{}, BgL: "#654321"})
		})
		if plan, _ := e.PlanLook(lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Spreads: 6}, nil); plan.Touched != 0 {
			t.Fatalf("свой разворот посчитан: %+v", plan)
		}
	})
}

// PlanLook возвращает то же, что ApplyLook, и ничего не меняет.
func TestPlanLookMatchesApply(t *testing.T) {
	type stage struct {
		ref  model.LookRef
		tpl  model.AlbumTpl
		prev album.LookFinder
		n    int
	}
	stages := []stage{
		{lkRefA, lookA(), nil, 9},
		{lkRefA, lookA(), nil, 9},
		{lkRefB, lookB(), lkPrev(lookA()), 6},
		{lkRefA, lookA(), nil, 12},
		{lkRefB, lookB(), nil, 4},
	}
	e := lkEditor(t)
	for i, s := range stages {
		o := model.LookOpts{Spreads: s.n}
		rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
		plan, err := e.PlanLook(s.ref, s.tpl, s.prev, o, nil)
		if err != nil {
			t.Fatalf("этап %d: %v", i, err)
		}
		if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
			t.Fatalf("этап %d: PlanLook изменил проект", i)
		}
		_, got, err := e.ApplyLook(s.ref, s.tpl, s.prev, o, nil)
		if err != nil {
			t.Fatalf("этап %d: %v", i, err)
		}
		if plan != got {
			t.Fatalf("этап %d: план %+v, правка %+v", i, plan, got)
		}
		if i == 2 && (got.Touched != 0 || !got.Trial) {
			t.Fatalf("этап 2: %+v", got)
		}
	}
	if edtest.P(e).Look.Key != lkKeyB || len(edtest.P(e).Spreads) != 4 {
		t.Fatalf("итог: макет %v, разворотов %d", edtest.P(e).Look, len(edtest.P(e).Spreads))
	}
}

// Ключи библиотеки без файла на этом ПК считаются один раз на макет; слой пропускается.
func TestApplyLookMissingLibs(t *testing.T) {
	st := lkStep("", "Рамка", "#ffffff", 1)
	st.Tpl.Stickers = []model.TplItem{{Anchor: model.Anchor{AX: 1, AY: -1}, W: 0.1, H: 0.1, Lib: "my:00000000000000aa"}}
	st.Tpl.Page[0].Lib = "my:00000000000000bb"
	st.Tpl.Whole = &model.TplBg{Lib: "my:00000000000000dd"}
	st.Tpl.Borders = []model.TplBorder{{I: 0, Lib: "my:00000000000000ee", Scale: 1}}
	tpl := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{st, lkStep(model.RoleRepeat, "", "#eeeeee", 1)}}
	tpl.Steps[1].Tpl.Stickers = st.Tpl.Stickers
	e := lkEditor(t)
	plan := lkApply(t, e, lkRefA, tpl, nil, 5)
	if plan.Missing != 3 || plan.Spreads != 5 { // фото страницы у шаблона разворота зажим снимает
		t.Fatalf("план: %+v", plan)
	}
	for _, s := range edtest.P(e).Spreads {
		if len(s.Stickers) != 0 || s.BgPhotoL != nil {
			t.Fatal("слой без файла не должен ставиться")
		}
	}
}

// Собранный альбом: без режима и серией — отказ ErrLookMode без правки (PlanLook отвечает Full без
// чисел), add и bg ставятся (G4b); PlanLook без ключа отказывает.
func TestApplyLookFullEntry(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
	rev, data := e.View().Rev, lkJSON(e)
	for _, mode := range []string{"", model.LookSeries} {
		o := model.LookOpts{Mode: mode, Spreads: 3}
		plan, err := e.PlanLook(lkRefA, lookA(), nil, o, nil)
		if err != nil || !plan.Full || plan.Mode != "" || plan.Spreads != 0 {
			t.Fatalf("режим %q: план %+v, %v", mode, plan, err)
		}
		if _, _, err := e.ApplyLook(lkRefA, lookA(), nil, o, nil); !errors.Is(err, album.ErrLookMode) || e.View().Rev != rev || lkJSON(e) != data {
			t.Fatalf("режим %q: ApplyLook %v", mode, err)
		}
	}
	for _, mode := range []string{model.LookAdd, model.LookBg} {
		plan, err := e.PlanLook(lkRefA, lookA(), nil, model.LookOpts{Mode: mode, Spreads: 2}, nil)
		if err != nil || !plan.Full || plan.Mode != mode || plan.Spreads == 0 {
			t.Fatalf("режим %q: план %+v, %v", mode, plan, err)
		}
		if e.View().Rev != rev || lkJSON(e) != data {
			t.Fatalf("режим %q: PlanLook изменил проект", mode)
		}
	}
	if _, err := e.PlanLook(model.LookRef{}, lookA(), nil, model.LookOpts{}, nil); err == nil {
		t.Fatal("PlanLook без ключа должен отказать")
	}
}

// Прежний макет не сходится с тем, что стоит (другой вид шага): все такие развороты тронуты.
func TestSwitchLookPrevMismatch(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	wrong := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkVig("Класс", 4), lkStep(model.RoleRepeat, "", "#f0fff0", 1)}}
	plan, err := e.PlanLook(lkRefB, lookB(), lkPrev(wrong), model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 9 {
		t.Fatalf("тронутых %d, ждали 9: %v", plan.Touched, err)
	}
}

// Шаг-страница с MaxTexts заготовок на развороте даёт вдвое больше (зеркало): отказ постановки,
// альбом прежний, шага отмены нет.
func TestApplyLookStepErrorRolledBack(t *testing.T) {
	st := model.LookStep{Name: "Глава", Tpl: model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: lkPlaces(1), Page: []model.TplBg{{Color: "#ffffff"}}}}
	for range model.MaxTexts {
		st.Tpl.Drafts = append(st.Tpl.Drafts, model.TplText{W: 0.2, Text: "[Имя]", Style: model.PlainStyle()})
	}
	tpl := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Титул", "#ffffff", 1), st}}
	e := lkEditor(t)
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	if _, _, err := e.ApplyLook(lkRefA, tpl, nil, model.LookOpts{Spreads: 2}, nil); err == nil || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatalf("err = %v, rev %d → %d", err, rev, e.View().Rev)
	}
}

// Шаг-виньетка с шаблоном разворота (Pages 2): на листе 031 он становится страницей — фон
// страницы и узор всего разворота переходят на лист.
func TestApplyLookVignettePages2(t *testing.T) {
	vig := lkVig("Класс", 4)
	vig.Tpl = model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: lkPlaces(1),
		Page:  []model.TplBg{{Color: "#112233"}, {Color: "#445566"}},
		Whole: &model.TplBg{Pat: &model.Pattern{Kind: "dots"}, Lib: "my:00000000000000cc", Fade: 0.3}}
	tpl := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Титул", "#ffffff", 1), vig}}
	e := lkEditor(t)
	plan := lkApply(t, e, lkRefA, tpl, nil, 2)
	if s := edtest.P(e).Spreads[1]; !s.IsVignette() || s.BgL != "#112233" || len(s.Grid) != 5 || plan.Missing != 1 {
		t.Fatalf("виньетка из разворота: фон %q, мест %d, пропущено %d", s.BgL, len(s.Grid), plan.Missing)
	}
}

// Шаги, у которых после постановки нет ни мест, ни цвета: одни надписи, стикер без файла на этом
// ПК (с именем главы). Номер шага остаётся (FixLook держит имя и надписи), повтор — холостой:
// rev, число и ID разворотов прежние (0.28-131-80).
func TestApplyLookBareStepsNoop(t *testing.T) {
	onlyText := model.LookStep{Name: "Титул", Tpl: model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Page: []model.TplBg{{}, {}},
		Drafts: []model.TplText{{W: 0.5, Text: "[Имена]", Style: model.PlainStyle()}}}}
	noName := onlyText
	noName.Name = ""
	lost := model.LookStep{Name: "Рамка", Tpl: model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Page: []model.TplBg{{}, {}},
		Stickers: []model.TplItem{{Anchor: model.Anchor{AX: 1, AY: -1}, W: 0.1, H: 0.1, Lib: "my:00000000000000aa"}}}}
	for name, tpl := range map[string]model.AlbumTpl{
		"одни надписи":     {V: model.LookV, Steps: []model.LookStep{onlyText, noName}},
		"стикер без файла": {V: model.LookV, Steps: []model.LookStep{lost, lkStep(model.RoleRepeat, "", "#eeeeee", 1)}},
	} {
		t.Run(name, func(t *testing.T) {
			e := lkEditor(t)
			lkApply(t, e, lkRefA, tpl, nil, 2)
			rows, rev, undo, data := lkRows(e), e.View().Rev, edtest.UndoLen(e), lkJSON(e)
			if len(rows) != 2 || rows[0].Step != 1 || rows[1].Step != 2 {
				t.Fatalf("шаги после постановки: %+v", rows)
			}
			for range 2 {
				plan := lkApply(t, e, lkRefA, tpl, nil, 2)
				if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data || len(lkRows(e)) != 2 || plan.Touched != 0 {
					t.Fatalf("повтор не холостой: rev %d → %d, разворотов %d, тронутых %d", rev, e.View().Rev, len(lkRows(e)), plan.Touched)
				}
			}
		})
	}
}

// Сухой прогон проходит тот же хвост конвейера (settle), что и правка: отказ, который даёт только
// он (общий запас знаков проекта), виден до «Применить» — вопрос не обещает невозможного.
func TestPlanLookRefusalFromSettle(t *testing.T) {
	step := lkStep(model.RoleRepeat, "", "#ffffff", 1)
	for range model.MaxTexts { // 64 заготовки на каждом из 200 разворотов — больше общего запаса
		step.Tpl.Drafts = append(step.Tpl.Drafts, model.TplText{W: 0.2, Text: "[Имя]", Style: model.PlainStyle()})
	}
	tpl := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Титул", "#ffffff", 1), step}}
	e := lkEditor(t)
	o := model.LookOpts{Spreads: model.MaxSpreads}
	rev, data := e.View().Rev, lkJSON(e)
	_, _, errApply := e.ApplyLook(lkRefA, tpl, nil, o, nil)
	_, errPlan := e.PlanLook(lkRefA, tpl, nil, o, nil)
	if !errors.Is(errApply, model.ErrTextBudget) || !errors.Is(errPlan, model.ErrTextBudget) {
		t.Fatalf("ApplyLook: %v; PlanLook: %v — ждали отказ запаса знаков у обоих", errApply, errPlan)
	}
	if e.View().Rev != rev || lkJSON(e) != data {
		t.Fatal("отказ оставил след")
	}
}

// lkTouchedNoPlaces — подпункты «на шаге без мест»: свои места тронуты, шаг без мест снимает места.
func lkTouchedNoPlaces(t *testing.T) {
	t.Helper()
	// Шаг прежнего макета без мест (одни цвета): места, расставленные фотографом, повтор шага не
	// трогает — но замена макета их унесёт, значит, разворот тронут (место на шаге без мест).
	noPlaces := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Фон", "#fff0f0", 0), lkStep(model.RoleRepeat, "", "#f0f0ff", 0)}}
	t.Run("свои места на шаге без мест", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, noPlaces, nil, 4)
		if plan, _ := e.PlanLook(lkRefB, lookB(), lkPrev(noPlaces), model.LookOpts{Spreads: 6}, nil); plan.Touched != 0 {
			t.Fatalf("без правок тронутых %d", plan.Touched)
		}
		edtest.Change(e, func(p *model.Project) {
			for i := range 3 {
				p.Spreads[1].Grid = append(p.Spreads[1].Grid, &model.Place{ID: fmt.Sprintf("0a00000000%02d", i+1), X: 10 + float64(i)*50, Y: 10, W: 40, H: 40})
			}
		})
		plan, err := e.PlanLook(lkRefB, lookB(), lkPrev(noPlaces), model.LookOpts{Spreads: 6}, nil)
		if err != nil || plan.Touched != 1 {
			t.Fatalf("три своих места: тронутых %d (%v)", plan.Touched, err)
		}
		// Тот же макет повторно: места тоже тронуты и снимаются (разворот — как шаг).
		lkApply(t, e, lkRefA, noPlaces, nil, 4)
		if g := edtest.P(e).Spreads[1].Grid; len(g) != 0 {
			t.Fatalf("после повтора мест %d, ждали 0", len(g))
		}
	})
	t.Run("шаг без мест снимает места", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		if len(edtest.P(e).Spreads[0].Grid) == 0 {
			t.Fatal("у титула A есть место")
		}
		bare := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Фон", "#fff0f0", 0), lkStep("", "Конец", "#fff0f0", 0)}}
		// Табличка имени (071) на месте титула: разворот тронут до замены и уходит вместе с местом.
		edtest.Change(e, func(p *model.Project) {
			p.Spreads[0].Grid[0].Person = model.Person{Last: "Иванов", First: "Аня"}
		})
		if plan, err := e.PlanLook(lkRefB, bare, lkPrev(lookA()), model.LookOpts{Spreads: 2}, nil); err != nil || plan.Touched == 0 {
			t.Fatalf("с табличкой имени тронутых %d (%v)", plan.Touched, err)
		}
		lkApply(t, e, lkRefB, bare, lkPrev(lookA()), 2)
		for _, s := range edtest.P(e).Spreads {
			if len(s.Grid) != 0 || s.Order != nil {
				t.Fatalf("разворот %s: мест %d", s.ID, len(s.Grid))
			}
		}
	})
}
