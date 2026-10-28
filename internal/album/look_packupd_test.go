package album_test

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Набор обновился (131 design §3 «Поиск макета»): у макета новый ключ, а набор и запись
// манифеста прежние — это тот же макет, шаги разворотов держатся, Project.Look переходит на
// новую запись. Чужой набор или запись без пары — по-прежнему чужой макет.

var (
	lkPackV1    = model.LookRef{Key: lkKeyA, Name: "Классика", Pack: "wedding-layouts", Item: "looks/main.json"}
	lkPackV2    = model.LookRef{Key: lkKeyB, Name: "Классика", Pack: "wedding-layouts", Item: "looks/main.json"}
	lkPackOther = model.LookRef{Key: lkKeyB, Name: "Классика", Pack: "wedding-layouts", Item: "looks/other.json"}
	lkKeyOnly   = model.LookRef{Key: lkKeyB, Name: "Классика"} // без пары набор+запись
)

// lkSetLook ставит ссылку на макет в проект мимо правки (подготовка сценария).
func lkSetLook(e *album.Editor, r model.LookRef) {
	edtest.Change(e, func(p *model.Project) { c := r; p.Look = &c })
}

func TestSameLook(t *testing.T) {
	r := func(key, pack, item string) *model.LookRef { return &model.LookRef{Key: key, Pack: pack, Item: item} }
	for name, c := range map[string]struct {
		a, b *model.LookRef
		want bool
	}{
		"тот же ключ":             {r(lkKeyA, "", ""), r(lkKeyA, "", ""), true},
		"ключ новый, запись та":   {&lkPackV1, &lkPackV2, true},
		"другая запись":           {&lkPackV1, &lkPackOther, false},
		"другой набор":            {r(lkKeyA, "pack-a", "x.json"), r(lkKeyB, "pack-b", "x.json"), false},
		"без пары":                {&lkPackV1, &lkKeyOnly, false},
		"пустые пары не равны":    {r(lkKeyA, "", ""), r(lkKeyB, "", ""), false},
		"пустой путь не равен":    {r(lkKeyA, "pack-a", ""), r(lkKeyB, "pack-a", ""), false},
		"nil слева":               {nil, &lkPackV1, false},
		"nil справа":              {&lkPackV1, nil, false},
		"nil с обеих сторон":      {nil, nil, false},
		"ключ тот же, запись нет": {r(lkKeyA, "pack-a", "x.json"), r(lkKeyA, "", ""), true},
	} {
		if got := model.SameLook(c.a, c.b); got != c.want {
			t.Errorf("%s: %v, ждали %v", name, got, c.want)
		}
	}
}

// lkAddSteps — собранный альбом по ссылке first, «Добавить развороты» (2) по ссылке second:
// шаги разворотов и итоговая ссылка альбома.
func lkAddSteps(t *testing.T, first, second model.LookRef) ([]int, *model.LookRef) {
	t.Helper()
	e := lkBuilt(t, 3)
	lkSetLook(e, first)
	if _, _, err := e.ApplyLook(second, lookA(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 2}, nil); err != nil {
		t.Fatal(err)
	}
	return lkSteps(lkRows(e)), edtest.P(e).Look
}

func TestLookPackUpdateAdd(t *testing.T) {
	want, _ := lkAddSteps(t, lkPackV1, lkPackV1)
	if want[0] == 0 {
		t.Fatalf("опорный прогон без номеров: %v", want)
	}
	got, look := lkAddSteps(t, lkPackV1, lkPackV2)
	if !slices.Equal(got, want) {
		t.Fatalf("набор обновился: шаги %v, ждали %v", got, want)
	}
	if look == nil || *look != lkPackV2 {
		t.Fatalf("ссылка альбома %+v, ждали новую запись", look)
	}
	// Чужие: другая запись и ключ без пары — альбом остаётся на своём макете (его нет на ПК —
	// новые в конец), номера прежних целы, у новых 0.
	for name, ref := range map[string]model.LookRef{"другая запись": lkPackOther, "ключ без пары": lkKeyOnly} {
		steps, look := lkAddSteps(t, lkPackV1, ref)
		if w := []int{1, 2, 3, 4, 5, 4, 6, 7, 8, 0, 0}; !slices.Equal(steps, w) {
			t.Errorf("%s: шаги %v, ждали %v", name, steps, w)
		}
		if look == nil || *look != lkPackV1 {
			t.Errorf("%s: ссылка альбома %+v, ждали прежнюю", name, look)
		}
	}
}

func TestLookPackUpdateBg(t *testing.T) {
	// Макет без виньетки: развороты-виньетки шага не получают и держат прежний номер, но только у того
	// же макета (иначе номера снимаются). find нет: прежний макет — тот же, что новый.
	noVig := model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{
		lkStep("", "Титул", "#ffffff", 1), lkStep(model.RoleRepeat, "", "#f0fff0", 1), lkStep("", "Конец", "#dddddd", 1),
	}}
	run := func(second model.LookRef) ([]int, *model.LookRef, model.LookPlan) {
		e := lkBuilt(t, 3)
		lkSetLook(e, lkPackV1)
		_, plan, err := e.ApplyLook(second, noVig, nil, model.LookOpts{Mode: model.LookBg}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return lkSteps(lkRows(e)), edtest.P(e).Look, plan
	}
	want, _, wplan := run(lkPackV1)
	if want[1] == 0 {
		t.Fatalf("опорный прогон: виньетка потеряла номер: %v", want)
	}
	got, look, plan := run(lkPackV2)
	if !slices.Equal(got, want) || plan != wplan || look == nil || *look != lkPackV2 {
		t.Fatalf("bg: шаги %v план %+v ссылка %+v; ждали %v план %+v", got, plan, look, want, wplan)
	}
	if steps, _, _ := run(lkPackOther); steps[1] != 0 {
		t.Fatalf("чужой макет: виньетка держит номер %v", steps)
	}
	// Холостой повтор: ссылка уже новая — шагов не прибавляется.
	e := lkBuilt(t, 3)
	lkSetLook(e, lkPackV1)
	if _, _, err := e.ApplyLook(lkPackV2, noVig, nil, model.LookOpts{Mode: model.LookBg}, nil); err != nil {
		t.Fatal(err)
	}
	undo, rev := edtest.UndoLen(e), e.View().Rev
	if _, _, err := e.ApplyLook(lkPackV2, noVig, nil, model.LookOpts{Mode: model.LookBg}, nil); err != nil || edtest.UndoLen(e) != undo || e.View().Rev != rev {
		t.Fatalf("повтор bg: шаг отмены или rev изменились: %v", err)
	}
}

func TestLookPackUpdateSeries(t *testing.T) {
	run := func(second model.LookRef) ([]int, *model.LookRef, model.LookPlan) {
		e := lkEditor(t)
		lkApply(t, e, lkPackV1, lookA(), nil, 9)
		_, plan, err := e.ApplyLook(second, lookA(), nil, model.LookOpts{Spreads: 9}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return lkSteps(lkRows(e)), edtest.P(e).Look, plan
	}
	want, _, wplan := run(lkPackV1)
	got, look, plan := run(lkPackV2)
	if !slices.Equal(got, want) || plan != wplan || look == nil || *look != lkPackV2 {
		t.Fatalf("серия: шаги %v план %+v ссылка %+v; ждали %v план %+v", got, plan, look, want, wplan)
	}
	if !plan.Trial || plan.Touched != 0 || plan.Prev != "Классика" {
		t.Fatalf("план: прежний макет не опознан (find нет, он — новый): %+v", plan)
	}
}

func TestLookPackUpdateStep(t *testing.T) {
	e, sid := lkStepEd(t)
	lkSetLook(e, lkPackV1)
	if _, _, _, err := e.ApplyLookStep(sid, 3, lkPackV2, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if s, l := edtest.P(e).Spreads[0], edtest.P(e).Look; s.LookStep != 3 || l == nil || *l != lkPackV2 {
		t.Fatalf("шаг %d, ссылка %+v", s.LookStep, l)
	}
	undo := edtest.UndoLen(e)
	e.Undo()
	if l := edtest.P(e).Look; edtest.P(e).Spreads[0].LookStep != 0 || l == nil || *l != lkPackV1 || edtest.UndoLen(e) != undo-1 {
		t.Fatalf("Undo не вернул прежнюю ссылку: %+v", l)
	}
	// Чужая запись: номер не запоминается, ссылка прежняя.
	if _, _, _, err := e.ApplyLookStep(sid, 3, lkPackOther, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if l := edtest.P(e).Look; edtest.P(e).Spreads[0].LookStep != 0 || *l != lkPackV1 {
		t.Fatalf("чужой макет: шаг %d, ссылка %+v", edtest.P(e).Spreads[0].LookStep, l)
	}
	// Повтор по новой записи — холостая правка (ссылка уже новая).
	if _, _, _, err := e.ApplyLookStep(sid, 3, lkPackV2, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	rev, n := e.View().Rev, edtest.UndoLen(e)
	if _, _, _, err := e.ApplyLookStep(sid, 3, lkPackV2, lookA(), nil); err != nil || e.View().Rev != rev || edtest.UndoLen(e) != n {
		t.Fatalf("повтор: rev %d → %d, %v", rev, e.View().Rev, err)
	}
}

func TestLookPackUpdateCoverStep(t *testing.T) {
	e := lkCoverEditor(t)
	lkSetLook(e, lkPackV1)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkPackV2, lkCoverTpl(), nil); err != nil {
		t.Fatal(err)
	}
	if l := edtest.P(e).Look; l == nil || *l != lkPackV2 {
		t.Fatalf("ссылка %+v, ждали новую запись", l)
	}
	e = lkCoverEditor(t)
	lkSetLook(e, lkPackV1)
	if _, _, _, err := e.ApplyLookStep(model.CoverID, 0, lkPackOther, lkCoverTpl(), nil); err != nil || *edtest.P(e).Look != lkPackV1 {
		t.Fatalf("чужой макет: ссылка %+v, %v", edtest.P(e).Look, err)
	}
}

func TestLookPackUpdateNewSpread(t *testing.T) {
	e := lkBuilt(t, 2)
	lkSetLook(e, lkPackV1)
	lkHost(e, lookA()) // макет по записи проекта находит библиотека (по набору и записи)
	if _, err := e.NewLookSpread(lookA(), nil); err != nil {
		t.Fatal(err)
	}
	rows := lkRows(e)
	if len(rows) != 10 || rows[6].Step != 5 {
		t.Fatalf("«Разворот макета»: %d разворотов, шаги %v", len(rows), lkSteps(rows))
	}
	if st := lkSteps(rows); slices.Contains(st[:6], 0) {
		t.Fatalf("прежние номера стёрты: %v", st)
	}
}

// TestLookFoundByPackItem — сквозной круг: альбом по v1, набор обновился (в реестре макет с
// новым ключом по той же записи), вид знает «макет есть», «Добавить развороты» и «Поставить на
// разворот» по v2 держат номера, Save→Load сохраняют ссылку, Undo возвращает v1.
func TestLookFoundByPackItem(t *testing.T) {
	reg := []model.LookRef{lkPackV2} // в реестре только новая версия
	match := func(r model.LookRef) bool {
		return slices.ContainsFunc(reg, func(x model.LookRef) bool { return model.SameLook(&x, &r) })
	}
	e := lkBuilt(t, 3)
	lkSetLook(e, lkPackV1)
	e.SetLookHost(album.LookHost{
		Find: func(r model.LookRef) *model.AlbumTpl {
			if match(r) {
				t := lookA()
				return &t
			}
			return nil
		},
		Has: match,
	})
	v := e.View()
	if v.Look == nil || !v.Look.Found || v.Look.Key != lkKeyA {
		t.Fatalf("вид: макет по записи набора не найден: %+v", v.Look)
	}
	before := lkSteps(lkRows(e))
	if _, _, err := e.ApplyLook(lkPackV2, lookA(), nil, model.LookOpts{Mode: model.LookAdd, Spreads: 2}, nil); err != nil {
		t.Fatal(err)
	}
	after := lkSteps(lkRows(e))
	if len(after) != len(before)+2 || slices.Contains(after[:6], 0) {
		t.Fatalf("add: шаги %v → %v, номера должны держаться", before, after)
	}
	if v = e.View(); v.Look == nil || v.Look.Key != lkKeyB || !v.Look.Found {
		t.Fatalf("после add ссылка не обновилась: %+v", v.Look)
	}
	// Save → Load: ссылка и номера шагов переживают файл.
	p2 := model.Unmarshal(model.Marshal(edtest.P(e)))
	if p2.Look == nil || *p2.Look != lkPackV2 || !slices.Equal(lkStepsOf(p2), after) {
		t.Fatalf("круг файла: ссылка %+v, шаги %v", p2.Look, lkStepsOf(p2))
	}
	e.Undo()
	if l := edtest.P(e).Look; l == nil || *l != lkPackV1 || !slices.Equal(lkSteps(lkRows(e)), before) {
		t.Fatalf("Undo: ссылка %+v, шаги %v", l, lkSteps(lkRows(e)))
	}
	// Пустой список реестра: макет не найден, вид это показывает.
	reg = nil
	if v = e.View(); v.Look == nil || v.Look.Found {
		t.Fatalf("реестр пуст, а макет найден: %+v", v.Look)
	}
}

func lkStepsOf(p *model.Project) (out []int) {
	for _, s := range p.Spreads {
		out = append(out, s.LookStep)
	}
	return out
}
