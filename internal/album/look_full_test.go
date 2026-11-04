package album_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Собранный альбом: «Добавить развороты» и «Фоны по серии», шаг на лист, разворот по серии
// (131 design §4, G4b). Один шаг отмены, отказ без шага, повтор bg — холостой.

// lkBuilt — серия lookA из 9 разворотов, в первые места садятся k фото из съёмки: альбом собранный.
func lkBuilt(t *testing.T, k int) *album.Editor {
	t.Helper()
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	ids := make([]string, k)
	for i := range ids {
		ids[i] = "ph" + string(rune('a'+i))
	}
	if _, left, _ := e.FillPlaces(edtest.Photos(ids...), ""); left != 0 {
		t.Fatalf("фото не встали: %d", left)
	}
	return e
}

// lkSnap — JSON каждого разворота по ID.
func lkSnap(e *album.Editor) map[string]string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	out := map[string]string{}
	for _, s := range edtest.P(e).Spreads {
		b, _ := json.Marshal(s)
		out[s.ID] = string(b)
	}
	return out
}

// lkNoBg — проект без фонов, номеров шагов и ссылки на макет: всё, что режим bg менять не вправе.
func lkNoBg(e *album.Editor) string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	c := edtest.P(e).Clone()
	for _, s := range c.Sheets() {
		s.BgL, s.BgR, s.Pat, s.PatL, s.PatR = "", "", nil, nil, nil
		s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.LookStep = nil, nil, nil, 0
	}
	c.Look = nil
	return string(model.Marshal(c))
}

func lkBgColors(e *album.Editor) []string {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	var out []string
	for _, s := range edtest.P(e).Spreads {
		out = append(out, s.BgL)
	}
	return out
}

// lkPlanned — PlanLook и ApplyLook с одними параметрами дают один план; PlanLook ничего не меняет.
func lkPlanned(t *testing.T, e *album.Editor, ref model.LookRef, tpl model.AlbumTpl, prev album.LookFinder, o model.LookOpts) model.LookPlan {
	t.Helper()
	rev, data := e.View().Rev, lkJSON(e)
	want, err := e.PlanLook(ref, tpl, prev, o, nil)
	if err != nil || e.View().Rev != rev || lkJSON(e) != data {
		t.Fatalf("PlanLook: %v, изменил проект", err)
	}
	_, got, err := e.ApplyLook(ref, tpl, prev, o, nil)
	if err != nil {
		t.Fatalf("ApplyLook: %v", err)
	}
	if got != want {
		t.Fatalf("план не равен итогу:\n plan  %+v\n apply %+v", want, got)
	}
	return got
}

// «Добавить»: n повторяемых шагов по кругу от курсора — перед концом серии; развороты до и конец
// серии не меняются (ID, места, фото), места новых пустые; один шаг отмены.
func TestApplyLookAddBeforeFinal(t *testing.T) {
	e := lkBuilt(t, 3)
	rows0, snap0, json0 := lkRows(e), lkSnap(e), lkJSON(e)
	undo := edtest.UndoLen(e)
	plan := lkPlanned(t, e, lkRefA, lookA(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 3})
	if plan.Mode != model.LookAdd || !plan.Full || plan.Spreads != 3 || plan.Places != 7 || plan.Left != 0 || plan.Trial {
		t.Fatalf("план: %+v", plan)
	}
	rows := lkRows(e)
	if len(rows) != 12 {
		t.Fatalf("разворотов %d, ждали 12", len(rows))
	}
	for i := range 6 { // начало серии
		if rows[i].ID != rows0[i].ID {
			t.Errorf("разворот %d сдвинут", i)
		}
	}
	for i := range 3 { // конец серии встал после новых
		if rows[9+i].ID != rows0[6+i].ID {
			t.Errorf("конец серии: %d не на месте", i)
		}
	}
	if want := []int{1, 2, 3, 4, 5, 4, 5, 4, 5, 6, 7, 8}; !slices.Equal(lkSteps(rows), want) {
		t.Fatalf("шаги: %v, ждали %v", lkSteps(rows), want)
	}
	for i, n := range []int{3, 1, 3} { // повторяемые по кругу от шага перед ними (4 → 5, 4, 5)
		if len(rows[6+i].Places) != n {
			t.Errorf("новый разворот %d: мест %d, ждали %d", i, len(rows[6+i].Places), n)
		}
	}
	now := lkSnap(e)
	for id, js := range snap0 {
		if now[id] != js {
			t.Errorf("разворот %s изменился", id)
		}
	}
	for _, r := range rows[6:9] {
		for _, pl := range edtest.P(e).Spreads[slices.IndexFunc(rows, func(x lkRow) bool { return x.ID == r.ID })].Grid {
			if pl.Photo != "" {
				t.Fatal("в новых местах не должно быть фото")
			}
		}
	}
	if edtest.P(e).Look == nil || edtest.P(e).Look.Key != lkKeyA {
		t.Fatalf("альбом не помнит макет: %+v", edtest.P(e).Look)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	e.Undo()
	if lkJSON(e) != json0 {
		t.Fatal("Undo не вернул альбом")
	}
}

// «Добавить»: без конца серии — в конец; без повтора — последний неповторяемый шаг; число по
// умолчанию — 1; предел — отказ без шага.
func TestApplyLookAddCases(t *testing.T) {
	t.Run("нет конца серии", func(t *testing.T) {
		e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
		plan := lkPlanned(t, e, lkRefA, lookA(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 2})
		rows := lkRows(e)
		if plan.Spreads != 2 || len(rows) != 3 || !slices.Equal(lkSteps(rows), []int{0, 4, 5}) {
			t.Fatalf("план %+v, шаги %v", plan, lkSteps(rows))
		}
	})
	t.Run("число по умолчанию", func(t *testing.T) {
		e := lkBuilt(t, 1)
		lkPlanned(t, e, lkRefA, lookA(), nil, model.LookOpts{Mode: model.LookAdd})
		if len(lkRows(e)) != 10 {
			t.Fatalf("разворотов %d, ждали 10", len(lkRows(e)))
		}
	})
	t.Run("нет повтора", func(t *testing.T) {
		tpl := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "А", "#111111", 1), lkStep("", "Б", "#222222", 2), lkVig("Класс", 3)}}
		e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
		lkPlanned(t, e, lkRefA, tpl, nil, model.LookOpts{Mode: model.LookAdd, Spreads: 2})
		if got := lkSteps(lkRows(e)); !slices.Equal(got, []int{0, 2, 2}) {
			t.Fatalf("шаги %v: ждали последний неповторяемый (Б) дважды", got)
		}
	})
	t.Run("предел разворотов", func(t *testing.T) {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, model.MaxSpreads)
		e.FillPlaces(edtest.Photos("pa"), "")
		rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
		_, _, err := e.ApplyLook(lkRefA, lookA(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 1}, nil)
		if err == nil || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
			t.Fatalf("ждали отказ без шага: %v", err)
		}
	})
}

// «Добавить» чужим макетом (131-213): альбом остаётся на своём, новые без шага, один шаг отмены.
func TestApplyLookAddForeign(t *testing.T) {
	// Новые встают перед хвостом серии прежнего макета; прежние развороты и ссылка целы.
	t.Run("чужой макет", func(t *testing.T) {
		e := lkBuilt(t, 2)
		snap0, json0, undo := lkSnap(e), lkJSON(e), edtest.UndoLen(e)
		plan := lkPlanned(t, e, lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Mode: model.LookAdd, Spreads: 2})
		rows := lkRows(e)
		if want := []int{1, 2, 3, 4, 5, 4, 0, 0, 6, 7, 8}; !slices.Equal(lkSteps(rows), want) {
			t.Fatalf("шаги %v, ждали %v", lkSteps(rows), want)
		}
		if edtest.P(e).Look.Key != lkKeyA {
			t.Fatal("альбом остаётся на своём макете")
		}
		now := lkSnap(e)
		for id, js := range snap0 {
			if now[id] != js {
				t.Errorf("разворот %s изменился", id)
			}
		}
		for _, r := range rows[6:8] {
			if len(r.Places) != 4 {
				t.Errorf("новый разворот: мест %d, ждали 4", len(r.Places))
			}
		}
		if plan.Spreads != 2 || plan.Places != 8 || plan.Mode != model.LookAdd || !plan.Full {
			t.Fatalf("план: %+v", plan)
		}
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("шагов отмены %d, ждали один", edtest.UndoLen(e)-undo)
		}
		e.Undo()
		if lkJSON(e) != json0 {
			t.Fatal("отмена не вернула альбом")
		}
	})
	t.Run("чужой макет, прежнего нет на ПК", func(t *testing.T) {
		e := lkBuilt(t, 2)
		lkPlanned(t, e, lkRefB, lookB(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 2})
		if want := []int{1, 2, 3, 4, 5, 4, 6, 7, 8, 0, 0}; !slices.Equal(lkSteps(lkRows(e)), want) {
			t.Fatalf("шаги %v, ждали %v", lkSteps(lkRows(e)), want)
		}
		if edtest.P(e).Look.Key != lkKeyA {
			t.Fatal("альбом остаётся на своём макете")
		}
	})
}

// «Фоны по серии»: меняется только фон; свой стикер, своя надпись, название, места и фото — бит в
// бит. Номер шага = индекс SeriesFit + 1, виньетки берут фон шага vignette, обложка — Bg.
func TestApplyLookBgKeepsAll(t *testing.T) {
	e := lkBuilt(t, 4)
	rows := lkRows(e)
	if _, _, err := e.AddSticker(rows[3].ID, 0, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	edtest.AddText(t, e, rows[2].ID, 0)
	e.SetSpreadName(rows[4].ID, "Моя глава")
	e.SetBackground(rows[2].ID, "left", "#aabbcc")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.NewSpread(-1, edtest.Photos("bare1")); err != nil {
		t.Fatal(err)
	}
	b, a := lookB(), lookA()
	b.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#334455"}}
	keep, json0, undo := lkNoBg(e), lkJSON(e), edtest.UndoLen(e)

	o := model.LookOpts{Mode: model.LookBg}
	plan := lkPlanned(t, e, lkRefB, b, lkPrev(a), o)
	// Альбом: 9 макетных + голый разворот; не-виньеток 8 → SeriesFit(B, 8) = 0 2 2 2 2 2 2 3:
	// конец серии по позиции достаётся голому, прежний «Конец» берёт повтор.
	if plan.Mode != model.LookBg || plan.Replaced != 1 || plan.Anchors != 1 || !plan.Cover || plan.Left != 0 || plan.Spreads != 10 {
		t.Fatalf("план: %+v", plan)
	}
	if got := lkNoBg(e); got != keep {
		t.Fatal("режим bg изменил не только фон")
	}
	steps := lkSteps(lkRows(e))
	if want := []int{1, 2, 3, 3, 3, 3, 3, 2, 3, 4}; !slices.Equal(steps, want) {
		t.Fatalf("шаги: %v, ждали %v", steps, want)
	}
	cols := lkBgColors(e)
	for i, w := range []string{"#ffeedd", "#f5f0e8", "#ddeeff", "#ddeeff", "#ddeeff", "#ddeeff", "#ddeeff", "#f5f0e8", "#ddeeff", "#ccbbaa"} {
		if cols[i] != w {
			t.Errorf("разворот %d: фон %q, ждали %q", i, cols[i], w)
		}
	}
	if c := edtest.P(e).Cover.Sheet; c.BgL != "#334455" || c.BgR != "#334455" || c.LookStep != 0 {
		t.Fatalf("обложка: %q %q шаг %d", c.BgL, c.BgR, c.LookStep)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	// Повтор — холостая правка.
	rev, after := e.View().Rev, lkJSON(e)
	if _, p2, err := e.ApplyLook(lkRefB, b, lkPrev(b), o, nil); err != nil || e.View().Rev != rev || lkJSON(e) != after || p2.Replaced != 0 || p2.Anchors != 0 {
		t.Fatalf("повтор: rev %d → %d, план %+v, %v", rev, e.View().Rev, p2, err)
	}
	e.Undo()
	if lkJSON(e) != json0 {
		t.Fatal("Undo не вернул альбом бит в бит")
	}
}

// Свой стикер и своя надпись на том же листе лежат на месте, а фон шага не затирает стикер,
// как затирал бы putDesign; круг Save→Load.
func TestApplyLookBgSaveLoad(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "bg.salbum")
	e := edtest.Open(t, model.New(), path)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	e.FillPlaces(edtest.Photos("pa", "pb"), "")
	sid := lkRows(e)[3].ID
	if _, _, err := e.AddSticker(sid, 0, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	a := lookA()
	if _, _, err := e.ApplyLook(lkRefB, lookB(), lkPrev(a), model.LookOpts{Mode: model.LookBg}, nil); err != nil {
		t.Fatal(err)
	}
	if n := len(edtest.P(e).Spreads[3].Stickers); n != 1 {
		t.Fatalf("стикеров %d, ждали 1 (свой)", n)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e2 := edtest.Open(t, p, path)
	if !slices.Equal(lkSteps(lkRows(e2)), lkSteps(lkRows(e))) || !slices.Equal(lkBgColors(e2), lkBgColors(e)) || p.Look.Key != lkKeyB {
		t.Fatal("фоны и шаги не дошли до файла")
	}
	rev := e2.View().Rev
	b := lookB()
	if _, _, err := e2.ApplyLook(lkRefB, b, lkPrev(b), model.LookOpts{Mode: model.LookBg}, nil); err != nil || e2.View().Rev != rev {
		t.Fatalf("повтор после Load: %v, rev %d → %d", err, rev, e2.View().Rev)
	}
}

// Фоны по серии: собранный альбом без разворотов и без фото на фоне, нет шага vignette — виньетка
// не трогается, обложки в модели нет — пропуск.
func TestApplyLookBgEdges(t *testing.T) {
	noVig := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "А", "#111111", 1), lkStep(model.RoleRepeat, "", "#222222", 1)}}
	noVig.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#334455"}}
	e := lkBuilt(t, 2)
	before := lkBgColors(e)
	plan := lkPlanned(t, e, lkRefB, noVig, nil, model.LookOpts{Mode: model.LookBg})
	cols := lkBgColors(e)
	for i, r := range lkRows(e) {
		if r.Vig && cols[i] != before[i] {
			t.Errorf("виньетка %d: фон изменился без шага vignette", i)
		}
	}
	if plan.Cover || edtest.P(e).Cover != nil || plan.Spreads != 7 {
		t.Fatalf("план: %+v", plan)
	}
	for i, r := range lkRows(e) { // чужой макет: номера шагов старого не остаются, виньетки тоже
		if r.Step != 0 && r.Vig {
			t.Errorf("виньетка %d: номер шага %d чужого макета остался", i, r.Step)
		}
	}
	// Тот же макет (ключ): виньетки без шага vignette остаются со своими номерами.
	e = lkBuilt(t, 2)
	prevSteps := lkSteps(lkRows(e))
	lkPlanned(t, e, lkRefA, noVig, nil, model.LookOpts{Mode: model.LookBg})
	for i, r := range lkRows(e) {
		if r.Vig && r.Step != prevSteps[i] {
			t.Errorf("виньетка %d: номер шага %d, был %d", i, r.Step, prevSteps[i])
		}
	}
	// Голый разворот, которому шаг не даёт фона, якорем не становится.
	plain := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "А", "", 1)}}
	e = lkBuilt(t, 2)
	if _, err := e.NewSpread(-1, edtest.Photos("bare1")); err != nil {
		t.Fatal(err)
	}
	if plan := lkPlanned(t, e, lkRefB, plain, nil, model.LookOpts{Mode: model.LookBg}); plan.Anchors != 0 {
		t.Fatalf("якорей %d, ждали 0", plan.Anchors)
	}
	// Пустой макет и макет без ключа — отказ.
	if _, _, err := e.ApplyLook(lkRefB, model.AlbumTpl{}, nil, model.LookOpts{Mode: model.LookBg}, nil); !errors.Is(err, model.ErrTplEmpty) {
		t.Fatalf("пустой макет: %v", err)
	}
}

// «Разворот макета»: пустой разворот и следующий повторяемый шаг перед концом серии одной
// правкой; нет конца — в конец; нет макета — отказ без шага; предел — отказ.
func TestNewLookSpread(t *testing.T) {
	e := lkBuilt(t, 2)
	rows0, json0, undo, rev := lkRows(e), lkJSON(e), edtest.UndoLen(e), e.View().Rev
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatal(err)
	}
	rows := lkRows(e)
	if len(rows) != 10 || rows[6].Step != 5 || len(rows[6].Places) != 3 || rows[7].ID != rows0[6].ID || e.View().Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("после: %d разворотов, шаг %d, rev %d, отмена %d", len(rows), rows[6].Step, e.View().Rev, edtest.UndoLen(e))
	}
	if _, err := e.NewLookSpread(lookA(), nil); err != nil || lkRows(e)[6].Step != 5 || lkRows(e)[7].Step != 4 {
		t.Fatalf("второй разворот: круг повтора должен идти дальше: %v", lkSteps(lkRows(e)))
	}
	e.Undo()
	e.Undo()
	if lkJSON(e) != json0 {
		t.Fatal("Undo не вернул альбом")
	}
	// Нет макета: отказ без шага.
	e2 := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a")})
	rev, undo, data := e2.View().Rev, edtest.UndoLen(e2), lkJSON(e2)
	if _, err := e2.NewLookSpread(lookA(), nil); !errors.Is(err, album.ErrNoLook) || e2.View().Rev != rev || edtest.UndoLen(e2) != undo || lkJSON(e2) != data {
		t.Fatalf("нет макета: %v", err)
	}
	if _, err := e2.NewLookSpread(model.AlbumTpl{}, nil); !errors.Is(err, model.ErrTplEmpty) {
		t.Fatalf("пустой макет: %v", err)
	}
	// Предел.
	e3 := lkEditor(t)
	lkApply(t, e3, lkRefA, lookA(), nil, model.MaxSpreads)
	rev, data = e3.View().Rev, lkJSON(e3)
	if _, err := e3.NewLookSpread(lookA(), nil); err == nil || e3.View().Rev != rev || lkJSON(e3) != data {
		t.Fatalf("предел: %v", err)
	}
	// Круг Save→Load нового разворота: номер шага и макет доходят.
	path := filepath.Join(testkit.TempDir(t), "nl.salbum")
	e4 := edtest.Open(t, model.New(), path)
	lkApply(t, e4, lkRefA, lookA(), nil, 9)
	if _, err := e4.NewLookSpread(lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if err := e4.Save(""); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil || len(p.Spreads) != 10 || p.Spreads[6].LookStep != 5 {
		t.Fatalf("после Load: %v, %d разворотов", err, len(p.Spreads))
	}
}

// Стоимость: «Фоны по серии», «Добавить» и смена формата с макетом на 120 разворотах × 5 фото.
func TestApplyLookFullFast(t *testing.T) {
	testkit.CostTest(t)
	steps := []model.LookStep{lkStep("", "Начало", "#ffffff", 5), lkStep(model.RoleRepeat, "", "#eeeeee", 5), lkStep("", "Конец", "#dddddd", 5)}
	tpl := model.AlbumTpl{V: model.LookV, Steps: steps}
	other := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Начало", "#111111", 5), lkStep(model.RoleRepeat, "", "#222222", 5)}}
	ids := make([]string, 600)
	for i := range ids {
		ids[i] = "p" + string(rune('a'+i/26/26%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i%26))
	}
	var bg, add, fmtd time.Duration
	for range 3 {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, tpl, nil, 120)
		e.FillPlaces(edtest.Photos(ids...), "")
		bg = timed(bg, func() { e.ApplyLook(lkRefB, other, lkPrev(tpl), model.LookOpts{Mode: model.LookBg}, nil) })
		add = timed(add, func() {
			e.ApplyLook(lkRefB, other, lkPrev(other), model.LookOpts{Mode: model.LookAdd, Spreads: 10}, nil)
		})
		edtest.Mu(e).Lock()
		s := edtest.P(e).Settings
		edtest.Mu(e).Unlock()
		s.PageW = 200
		fmtd = timed(fmtd, func() { e.SetSettingsLook(s, &other) })
	}
	if limit := testkit.Limit(60 * time.Millisecond); bg > limit || add > limit || fmtd > limit {
		t.Fatalf("120 разворотов: фоны %v, добавить %v, формат %v, предел %v", bg, add, fmtd, limit)
	}
}

// timed — меньшее из best и времени fn (первое значение best = 0 не считается).
func timed(best time.Duration, fn func()) time.Duration {
	d := testkit.Fastest(1, fn)
	if best == 0 || d < best {
		return d
	}
	return best
}

// lkOverflow — макет, чей повторяемый шаг-страница с MaxTexts заготовок на развороте даёт вдвое
// больше (зеркало): постановка этого шага отказывает.
func lkOverflow() model.AlbumTpl {
	st := model.LookStep{Role: model.RoleRepeat, Tpl: model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: lkPlaces(1), Page: []model.TplBg{{Color: "#ffffff"}}}}
	for range model.MaxTexts {
		st.Tpl.Drafts = append(st.Tpl.Drafts, model.TplText{W: 0.2, Text: "[Имя]", Style: model.PlainStyle()})
	}
	return model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{lkStep("", "Титул", "#ffffff", 1), st}}
}

// Отказ постановки шага в «Добавить», «Поставить на разворот» и «Разворот макета»: альбом прежний,
// шага отмены нет.
func TestApplyLookFullStepRefused(t *testing.T) {
	tpl := lkOverflow()
	e := lkBuilt(t, 1) // у альбома есть макет: «Разворот макета» дойдёт до постановки шага
	sid := lkRows(e)[0].ID
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	_, _, errAdd := e.ApplyLook(lkRefA, tpl, nil, model.LookOpts{Mode: model.LookAdd}, nil)
	_, _, _, errStep := e.ApplyLookStep(sid, 2, lkRefA, tpl, nil)
	_, errNew := e.NewLookSpread(tpl, nil)
	if errAdd == nil || errStep == nil || errNew == nil {
		t.Fatalf("ждали отказ: add %v, шаг %v, разворот макета %v", errAdd, errStep, errNew)
	}
	if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatal("отказ изменил проект")
	}
}

// Фон шага и обложки — картинка библиотеки: файла на ПК нет — слой пропущен и посчитан в Missing,
// файл есть — фото фона встало и осталось записью Photos; Save→Load цел.
func TestApplyLookBgLibPhoto(t *testing.T) {
	const key = "my:00000000000000aa"
	tpl := lookB()
	tpl.Steps[2].Tpl.Whole = &model.TplBg{Lib: key, Fade: 0.2}
	tpl.Cover = &model.CoverTpl{Bg: &model.TplBg{Lib: key}}
	e := lkBuilt(t, 2)
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	a := lookA()
	plan, err := e.PlanLook(lkRefB, tpl, lkPrev(a), model.LookOpts{Mode: model.LookBg}, nil)
	if err != nil || plan.Missing != 1 {
		t.Fatalf("без файла: план %+v, %v", plan, err)
	}
	libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}}
	_, plan, err = e.ApplyLook(lkRefB, tpl, lkPrev(a), model.LookOpts{Mode: model.LookBg}, libs)
	if err != nil || plan.Missing != 0 || !plan.Cover {
		t.Fatalf("с файлом: план %+v, %v", plan, err)
	}
	p := edtest.P(e)
	if p.Spreads[2].BgPhoto == nil || p.Cover.Sheet.BgPhoto == nil || p.Photos[model.LibPhotoID(key)].Lib != key {
		t.Fatal("фото фона шага и обложки не встали")
	}
	rev := e.View().Rev
	if _, _, err := e.ApplyLook(lkRefB, tpl, lkPrev(tpl), model.LookOpts{Mode: model.LookBg}, libs); err != nil || e.View().Rev != rev {
		t.Fatalf("повтор с фото фона: %v, rev %d → %d", err, rev, e.View().Rev)
	}
}

// Поиск прежнего макета вернул негодный макет (пустой): все макетные развороты тронуты.
func TestSwitchLookFinderInvalid(t *testing.T) {
	e := lkEditor(t)
	lkApply(t, e, lkRefA, lookA(), nil, 9)
	empty := func(model.LookRef) *model.AlbumTpl { return &model.AlbumTpl{} }
	plan, err := e.PlanLook(lkRefB, lookB(), empty, model.LookOpts{Spreads: 6}, nil)
	if err != nil || plan.Touched != 9 {
		t.Fatalf("тронутых %d, ждали 9 (%v)", plan.Touched, err)
	}
}

// lkCoverEd — собранный альбом lookA с обложкой.
func lkCoverEd(t *testing.T, ph *model.PhotoInfo) *album.Editor {
	t.Helper()
	e := lkBuilt(t, 2)
	if _, err := e.CreateCover("", ph); err != nil {
		t.Fatal(err)
	}
	return e
}

func lkCoverLook(base model.AlbumTpl, bg model.TplBg) model.AlbumTpl {
	base.Cover = &model.CoverTpl{Bg: &bg}
	return base
}

// «Фоны по серии» и свой фон обложки: цвет, фото из съёмки и фото корешка при фото всего листа
// считаются в Replaced (вопрос сценария 6); фон прежнего макета «своим» не считается.
func TestApplyLookBgCoverOwn(t *testing.T) {
	a, bg := lookA(), model.TplBg{Color: "#334455"}
	b := lkCoverLook(lookB(), bg)
	o := model.LookOpts{Mode: model.LookBg}
	t.Run("свой цвет", func(t *testing.T) {
		e := lkCoverEd(t, nil)
		e.SetBackground(model.CoverID, "spread", "#abcdef")
		if plan := lkPlanned(t, e, lkRefB, b, lkPrev(a), o); plan.Replaced != 1 || !plan.Cover {
			t.Fatalf("план: %+v", plan)
		}
		if c := edtest.P(e).Cover.Sheet; c.BgL != "#334455" {
			t.Fatalf("фон обложки %q", c.BgL)
		}
	})
	t.Run("пустой фон обложки без своего", func(t *testing.T) {
		e := lkCoverEd(t, nil)
		if plan := lkPlanned(t, e, lkRefB, b, lkPrev(a), o); plan.Replaced != 0 || !plan.Cover {
			t.Fatalf("план: %+v", plan)
		}
	})
	t.Run("фото из съёмки", func(t *testing.T) {
		ph := edtest.Photo("cv")
		e := lkCoverEd(t, &ph)
		if plan := lkPlanned(t, e, lkRefB, b, lkPrev(a), o); plan.Replaced != 1 {
			t.Fatalf("план: %+v", plan)
		}
	})
	t.Run("фон прежнего макета", func(t *testing.T) {
		e := lkCoverEd(t, nil)
		lkPlanned(t, e, lkRefB, b, lkPrev(a), o)
		next := lkCoverLook(lookA(), model.TplBg{Color: "#556677"})
		if plan := lkPlanned(t, e, lkRefA, next, lkPrev(b), o); plan.Replaced != 0 || !plan.Cover {
			t.Fatalf("фон прежнего макета посчитан своим: %+v", plan)
		}
	})
	t.Run("фото корешка", func(t *testing.T) {
		const key = "my:00000000000000bb"
		e := lkCoverEd(t, nil)
		if _, err := e.SetCoverLayout(model.CoverBoth); err != nil {
			t.Fatal(err)
		}
		if _, err := e.SetBackdrop(model.CoverID, model.ScopeSpine, edtest.Photo("sp")); err != nil {
			t.Fatal(err)
		}
		if edtest.P(e).Cover.Sheet.BgPhotoS == nil {
			t.Fatal("фото корешка не встало — тест ничего не проверяет")
		}
		libs := map[string]model.PhotoInfo{key: {ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}}
		photo := lkCoverLook(lookB(), model.TplBg{Lib: key})
		_, plan, err := e.ApplyLook(lkRefB, photo, lkPrev(a), o, libs)
		if err != nil || plan.Replaced != 1 {
			t.Fatalf("план: %+v, %v", plan, err)
		}
		if c := edtest.P(e).Cover.Sheet; c.BgPhotoS != nil || c.BgPhoto == nil {
			t.Fatal("фото всего листа должно заменить фото корешка")
		}
		rev := e.View().Rev
		if _, p2, err := e.ApplyLook(lkRefB, photo, lkPrev(photo), o, libs); err != nil || e.View().Rev != rev || p2.Replaced != 0 {
			t.Fatalf("повтор: %+v, %v", p2, err)
		}
	})
}

// Курсор круга: свой разворот между повторяемыми и концом серии шаг не меняет — новый разворот
// берёт шаг после последнего повторяемого, а не шаг соседа (design §2).
func TestNewLookSpreadCursorSkipsOwn(t *testing.T) {
	e := lkBuilt(t, 1)
	if _, err := e.NewSpread(6, nil); err != nil { // свой пустой разворот между повтором и концом
		t.Fatal(err)
	}
	if got := lkSteps(lkRows(e)); !slices.Equal(got, []int{1, 2, 3, 4, 5, 4, 0, 6, 7, 8}) {
		t.Fatalf("исходные шаги: %v", got)
	}
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if got := lkSteps(lkRows(e)); !slices.Equal(got, []int{1, 2, 3, 4, 5, 4, 0, 5, 6, 7, 8}) {
		t.Fatalf("шаги: %v, ждали после своего разворота шаг 5 (следом за 4)", got)
	}
}

// NewLookSpread: макет, который прочитал App, сверяется под замком с реестром по Project.Look —
// альбом уже на макете B, а App принёс A: отказ без правки (гонка с параллельным ApplyLook).
func TestNewLookSpreadChanged(t *testing.T) {
	e := lkBuilt(t, 2)
	json0, undo, rev := lkJSON(e), edtest.UndoLen(e), e.View().Rev
	b := lookB()
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &b }})
	if _, err := e.NewLookSpread(lookA(), nil); !errors.Is(err, album.ErrLookChanged) || lkJSON(e) != json0 || edtest.UndoLen(e) != undo || e.View().Rev != rev {
		t.Fatalf("макет сменился: %v", err)
	}
	a := lookA()
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &a }})
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatalf("тот же макет: %v", err)
	}
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return nil }}) // макета в реестре нет — как без библиотеки
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatalf("макета в реестре нет: %v", err)
	}
}

// LookHost: SetSettings/SetSettingsToAll сами находят макет альбома и вписывают его места, как
// SetSettingsLook; View.look.found — из Has, без поиска макета нет.
func TestLookHostFormat(t *testing.T) {
	mk := func() *album.Editor {
		e := lkEditor(t)
		lkApply(t, e, lkRefA, lookA(), nil, 9)
		return e
	}
	a := lookA()
	ns := mk().View().Settings
	ns.PageW, ns.PageH = 300, 150
	places := func(e *album.Editor) (out [][][4]float64) { // только геометрия: ID у каждого альбома свои
		for _, s := range e.View().Spreads {
			row := [][4]float64{}
			for _, p := range s.Places {
				row = append(row, [4]float64{p.X, p.Y, p.W, p.H})
			}
			out = append(out, row)
		}
		return out
	}
	want, bare, host, all := mk(), mk(), mk(), mk()
	want.SetSettingsLook(ns, &a)
	bare.SetSettings(ns)
	find := func(model.LookRef) *model.AlbumTpl { return &a }
	host.SetLookHost(album.LookHost{Find: find})
	host.SetSettings(ns)
	all.SetLookHost(album.LookHost{Find: find})
	all.SetSettingsToAll(ns)
	if !reflect.DeepEqual(places(host), places(want)) || !reflect.DeepEqual(places(all), places(want)) || reflect.DeepEqual(places(bare), places(want)) {
		t.Fatal("смена формата с LookHost не вписала макет")
	}
	if l := host.View().Look; l == nil || l.Found {
		t.Fatalf("без Has макета на ПК нет: %+v", l)
	}
	host.SetLookHost(album.LookHost{Has: func(r model.LookRef) bool { return r.Key == lkRefA.Key }})
	if l := host.View().Look; l == nil || !l.Found {
		t.Fatalf("Has: %+v", l)
	}
	if host.Look() == nil || host.Look().Key != lkRefA.Key || host.IsSheet() {
		t.Fatalf("Look/IsSheet: %+v", host.Look())
	}
}

// lkForeignHost — хост с макетами A и B по ключу: смена формата вписывает макет альбома.
func lkForeignHost(e *album.Editor) {
	a, b := lookA(), lookB()
	e.SetLookHost(album.LookHost{
		Find: func(r model.LookRef) *model.AlbumTpl {
			if r.Key == lkKeyA {
				return &a
			}
			return &b
		},
		Has: func(model.LookRef) bool { return true },
	})
}

// «Добавить» чужим макетом, затем смена формата: развороты A вписываются по шагам A, как в
// альбоме без чужих разворотов (131-213: шаги не снимаются).
func TestApplyLookAddForeignFormat(t *testing.T) {
	// geo — места разворотов по порядку; id у редакторов свои, сверка по порядку без новых (шаг 0).
	geo := func(e *album.Editor, skip bool) (out [][][4]float64) {
		steps := map[string]int{}
		for _, r := range lkRows(e) {
			steps[r.ID] = r.Step
		}
		for _, s := range e.View().Spreads {
			if skip && steps[s.ID] == 0 {
				continue
			}
			row := [][4]float64{}
			for _, p := range s.Places {
				row = append(row, [4]float64{p.X, p.Y, p.W, p.H})
			}
			out = append(out, row)
		}
		return out
	}
	ctl, mix := lkBuilt(t, 3), lkBuilt(t, 3)
	for _, e := range []*album.Editor{ctl, mix} {
		lkForeignHost(e)
	}
	var first []string // ID разворота первой пачки B
	for i := range 2 {
		if _, _, err := mix.ApplyLook(lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Mode: model.LookAdd, Spreads: 1}, nil); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = append(first, lkRows(mix)[6].ID)
		}
	}
	rows := lkRows(mix) // новые встали перед хвостом A, одна пачка за другой
	if want := []int{1, 2, 3, 4, 5, 4, 0, 0, 6, 7, 8}; !slices.Equal(lkSteps(rows), want) {
		t.Fatalf("шаги %v, ждали %v", lkSteps(rows), want)
	}
	if len(first) != 1 || rows[6].ID != first[0] || rows[7].ID == first[0] {
		t.Fatalf("вторая пачка B должна стоять после первой: %v", rows)
	}
	st := ctl.View().Settings
	st.PageW, st.PageH = 300, 150
	for _, e := range []*album.Editor{ctl, mix} {
		e.SetSettings(st)
	}
	gc, gm := geo(ctl, false), geo(mix, true)
	if len(gc) != 9 || len(gm) != 9 {
		t.Fatalf("разворотов A: контроль %d, смешанный %d, ждали 9", len(gc), len(gm))
	}
	for i := range gc {
		if !reflect.DeepEqual(gc[i], gm[i]) {
			t.Errorf("разворот %d: места после смены формата не как в контроле\n%v\n%v", i, gc[i], gm[i])
		}
	}
}

// После «Добавить» чужим макетом «+ ▸ Разворот макета» работает по макету альбома: A — встаёт
// после последнего повторяемого перед новым местом, B (не макет альбома) — ErrLookChanged без следа.
func TestApplyLookAddForeignNewSpread(t *testing.T) {
	e := lkBuilt(t, 3)
	lkForeignHost(e)
	if _, _, err := e.ApplyLook(lkRefB, lookB(), lkPrev(lookA()), model.LookOpts{Mode: model.LookAdd, Spreads: 2}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 3, 4, 5, 4, 0, 0, 5, 6, 7, 8}; !slices.Equal(lkSteps(lkRows(e)), want) {
		t.Fatalf("шаги %v, ждали %v", lkSteps(lkRows(e)), want)
	}
	json0, undo, rev := lkJSON(e), edtest.UndoLen(e), e.View().Rev
	if _, err := e.NewLookSpread(lookB(), nil); !errors.Is(err, album.ErrLookChanged) {
		t.Fatalf("макет B: %v", err)
	}
	if lkJSON(e) != json0 || edtest.UndoLen(e) != undo || e.View().Rev != rev {
		t.Fatal("отказ оставил след")
	}
}
