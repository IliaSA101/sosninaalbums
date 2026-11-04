package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// Сухой прогон сохранения макета (131, 6г): PlanSaveLook даёт тот же отказ и то же число шагов,
// что SaveLook, но ничего не пишет и не трогает альбом.

// libSnapshot — файлы библиотеки (путь, размер, время правки): сравнивается до и после прогона.
func libSnapshot(t *testing.T, s *decor.Store) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(s.Root(), func(p string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) { // корень появляется с первой записью
			return nil
		}
		if err != nil || d.IsDir() {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		out[p] = fmt.Sprintf("%d/%d", fi.Size(), fi.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// distinctApp — альбом из n разворотов с разными цветами (n разных шагов) и их id.
func distinctApp(t *testing.T, n int, same bool) (*App, []string) {
	t.Helper()
	a := lookApp(t)
	p := model.New()
	ids := make([]string, n)
	for i := range n {
		ids[i] = fmt.Sprintf("m%02d", i)
		bg := fmt.Sprintf("#0000%02x", i+1)
		if same {
			bg = "#123456"
		}
		p.Spreads = append(p.Spreads, &model.Spread{ID: ids[i], BgL: bg})
	}
	a.setEditor(mustEditor(t, p, ""))
	return a, ids
}

// TestPlanSaveLookMatchesSave: на одном наборе опций прогон отказывает ⇔ отказывает SaveLook, текст
// тот же; шагов столько, сколько в сохранённом макете. Граница: 24 разных — можно, 25 — нельзя.
// bigTextApp — альбом из 12 разных разворотов с 64 длинными надписями: в заготовках макет больше предела файла.
func bigTextApp(t *testing.T) (*App, []string) {
	t.Helper()
	a, ids := distinctApp(t, 12, false)
	p, _ := a.editor().Snapshot()
	for _, sp := range p.Spreads {
		for range model.MaxTexts {
			sp.Texts = append(sp.Texts, &model.TextBlock{ID: model.NewID(), X: 10, Y: 10, W: 50, Auto: true, Text: strings.Repeat("Ы", model.MaxTextRunes),
				Style: model.TextStyle{Font: "", Size: 20, Color: "#222222", Align: "left"}})
		}
	}
	a.setEditor(mustEditor(t, p, ""))
	return a, ids
}

// comparePlanSave — прогон и SaveLook на опциях o: отказ ⇔ отказ, тот же текст, шагов столько же.
func comparePlanSave(t *testing.T, a *App, name string, o model.SaveLookOpts) {
	t.Helper()
	plan, err := a.PlanSaveLook(o)
	if err != nil {
		t.Fatalf("%s: PlanSaveLook: %v", name, err)
	}
	if plan.Max != model.MaxLookSteps {
		t.Errorf("%s: предел %d", name, plan.Max)
	}
	it, serr := a.SaveLook(o)
	if (serr == nil) != plan.Ok {
		t.Errorf("%s: прогон Ok=%v, SaveLook: %v", name, plan.Ok, serr)
		return
	}
	if serr != nil {
		if plan.Reason != serr.Error() || plan.Steps != 0 {
			t.Errorf("%s: прогон %+v, SaveLook: %v", name, plan, serr)
		}
		return
	}
	if plan.Reason != "" || plan.Steps != it.Steps {
		t.Errorf("%s: прогон %+v, сохранено шагов %d", name, plan, it.Steps)
	}
}

func TestPlanSaveLookMatchesSave(t *testing.T) {
	a, all := distinctApp(t, 25, false)
	cases := map[string]model.SaveLookOpts{
		"24 разных":           {Name: "a24", Ids: all[:24]},
		"25 разных":           {Name: "a25", Ids: all},
		"1 разворот":          {Name: "a1", Ids: all[:1]},
		"пустой выбор":        {Name: "e", Ids: nil},
		"чужой id":            {Name: "x", Ids: []string{"нет такого"}},
		"обложка в ids":       {Name: "c", Ids: []string{model.CoverID}},
		"дубль id":            {Name: "d", Ids: []string{all[0], all[0]}},
		"нечего сохранять":    {Name: "n", Ids: all[:3], Places: false, Drafts: false},
		"25 разных, места":    {Name: "p", Ids: all, Places: true},
		"повторы 25→24 нет":   {Name: "r", Ids: all, RepFrom: 1, RepTo: 25}, // все разные: не сливаются
		"повторы 24 из 25":    {Name: "r2", Ids: all, RepFrom: 2, RepTo: 25},
		"место в 25 разных":   {Name: "pl", Ids: all, Places: true, Cover: true},
		"границы за пределом": {Name: "b", Ids: all[:5], RepFrom: -3, RepTo: 99},
	}
	for name, o := range cases {
		comparePlanSave(t, a, name, o)
	}
	// Макет больше 512 КБ (надписи-заготовки): прогон отказывает размером, как SaveLook.
	big, ids := bigTextApp(t)
	comparePlanSave(t, big, "больше 512 КБ", model.SaveLookOpts{Name: "Огромный", Ids: ids, Drafts: true})
	if plan, _ := big.PlanSaveLook(model.SaveLookOpts{Ids: ids, Drafts: true}); plan.Ok || !strings.Contains(plan.Reason, "макет больше 512 КБ") {
		t.Errorf("размер: %+v", plan)
	}
	comparePlanSave(t, big, "малая часть той же надписи", model.SaveLookOpts{Name: "Малый", Ids: ids[:2], Drafts: true})
	// Граница явно: именно 24 → 24 шага, 25 → отказ ErrLookMany.
	p24, _ := a.PlanSaveLook(model.SaveLookOpts{Ids: all[:24]})
	p25, _ := a.PlanSaveLook(model.SaveLookOpts{Ids: all})
	if !p24.Ok || p24.Steps != 24 || p25.Ok || p25.Reason != model.ErrLookMany.Error() {
		t.Errorf("граница: 24 → %+v, 25 → %+v", p24, p25)
	}
}

// TestPlanSaveLookMerge: одинаковые повторяемые сливаются в один шаг и не мешают пределу.
func TestPlanSaveLookMerge(t *testing.T) {
	a, all := distinctApp(t, 25, true)
	plan, err := a.PlanSaveLook(model.SaveLookOpts{Ids: all, RepFrom: 1, RepTo: 25})
	if err != nil || !plan.Ok || plan.Steps != 1 {
		t.Fatalf("25 одинаковых повторяемых: %+v %v", plan, err)
	}
	// Без повторяемых те же 25 одинаковых остаются 25 шагами — отказ.
	plan, _ = a.PlanSaveLook(model.SaveLookOpts{Ids: all})
	if plan.Ok || plan.Reason != model.ErrLookMany.Error() {
		t.Errorf("25 одинаковых без повторяемых: %+v", plan)
	}
	// Первый и последний стоят отдельно, середина (одинаковая) слита: 1 + 1 + 1.
	plan, _ = a.PlanSaveLook(model.SaveLookOpts{Ids: all, RepFrom: 2, RepTo: 24})
	if !plan.Ok || plan.Steps != 3 {
		t.Errorf("начало, повтор, конец: %+v", plan)
	}
}

// TestPlanSaveLookReadOnly: прогон не меняет альбом (rev, проект, отмена) и библиотеку (файлы, плитки);
// лист 031 — отказ тем же errLookSheet.
func TestPlanSaveLookReadOnly(t *testing.T) {
	a, all := distinctApp(t, 25, false)
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	files, n := libSnapshot(t, s), countLooks(t, a)
	before, rev := projectJSON(t, a), a.editor().View().Rev
	canUndo := a.editor().View().CanUndo

	for _, o := range []model.SaveLookOpts{
		{Name: "ok", Ids: all[:5], Places: true, Cover: true},
		{Name: "много", Ids: all},
		{Name: "пусто"},
	} {
		if _, err := a.PlanSaveLook(o); err != nil {
			t.Fatal(err)
		}
	}
	if projectJSON(t, a) != before || a.editor().View().Rev != rev || a.editor().View().CanUndo != canUndo {
		t.Error("PlanSaveLook изменил альбом")
	}
	after := libSnapshot(t, s)
	if len(after) != len(files) || countLooks(t, a) != n {
		t.Errorf("PlanSaveLook изменил библиотеку: файлов %d → %d", len(files), len(after))
	}
	for p, v := range files {
		if after[p] != v {
			t.Errorf("файл %s изменился", p)
		}
	}

	a.setEditor(mustEditor(t, album.NewVignetteSheet(300, 400), ""))
	ids := []string{a.editor().View().Spreads[0].ID}
	plan, err := a.PlanSaveLook(model.SaveLookOpts{Ids: ids, Places: true})
	if err != nil || plan.Ok || plan.Reason != errLookSheet.Error() {
		t.Errorf("лист 031: %+v %v", plan, err)
	}
}

// TestPlanSaveLookNoLibrary: прогону библиотека не нужна (окно спрашивает до записи); SaveLook без неё отказывает.
func TestPlanSaveLookNoLibrary(t *testing.T) {
	a, all := distinctApp(t, 3, false)
	a.decorLib.Store(nil)
	plan, err := a.PlanSaveLook(model.SaveLookOpts{Ids: all, Places: true})
	if err != nil || !plan.Ok || plan.Steps != 3 {
		t.Fatalf("без библиотеки: %+v %v", plan, err)
	}
	if _, err := a.SaveLook(model.SaveLookOpts{Name: "x", Ids: all, Places: true}); err == nil {
		t.Error("SaveLook без библиотеки не отказал")
	}
}
