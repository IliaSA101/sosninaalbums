package app

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// «Сохранить с применением» (123 Б1): Editor.ApplyPrint через App — формат и изделие обложки
// своей типографии одним шагом отмены.

// applyRig — App с обложкой «Фотосферы» и своей типографией на основе Фабрики; вид до применения.
func applyRig(t *testing.T) (*App, model.View) {
	t.Helper()
	a, _ := printApp(t)
	if v, err := a.editor().CreateCover("fsfera-thin", nil); err != nil || v.Cover == nil {
		t.Fatalf("обложка: %v", err)
	}
	return a, a.editor().View()
}

func TestSavePrintProfileApply(t *testing.T) {
	a, before := applyRig(t)
	e := editOf(t, a, "fabrika", "С применением")
	res, err := a.SavePrintProfile(e, true)
	if err != nil || !printprof.ValidOwnID(res.ID) {
		t.Fatalf("сохранение с apply: %+v %v", res, err)
	}
	pr, _ := printprof.Current().ByID(res.ID)
	f := pr.Formats[0]
	v := res.View
	if v.Settings.PageW != f.Page[0] || v.Settings.PageH != f.Page[1] || v.Settings.Bleed != f.BleedIn(pr) || !v.Settings.Layflat {
		t.Fatalf("формат не поставлен: %+v, ждали %v", v.Settings, f.Page)
	}
	if want := pr.Default; v.Cover == nil || v.Cover.Preset != want || v.Cover.Preset == before.Cover.Preset {
		t.Fatalf("изделие обложки %v, ждали %q", v.Cover, want)
	}
	if v.Rev <= before.Rev || !v.CanUndo {
		t.Fatalf("правка не в истории: rev %d→%d undo %v", before.Rev, v.Rev, v.CanUndo)
	}
	// Один шаг отмены возвращает и формат, и обложку (создание обложки — прежний шаг, остаётся).
	u := a.Undo()
	if u.Settings != before.Settings || u.Cover == nil || u.Cover.Preset != before.Cover.Preset {
		t.Fatalf("Undo: %+v %v", u.Settings, u.Cover)
	}
	if r := a.Redo(); r.Settings != v.Settings || r.Cover.Preset != v.Cover.Preset {
		t.Errorf("Redo: %+v", r.Settings)
	}
}

// apply=false: типография записана, альбом, rev и история — прежние.
func TestSavePrintProfileNoApply(t *testing.T) {
	a, before := applyRig(t)
	res, err := a.SavePrintProfile(editOf(t, a, "fabrika", "Без применения"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := printprof.Current().ByID(res.ID); !ok {
		t.Fatal("типография не записана")
	}
	if v := res.View; v.Rev != before.Rev || v.Settings != before.Settings || v.Cover.Preset != before.Cover.Preset {
		t.Fatalf("альбом изменился: rev %d→%d %+v", before.Rev, v.Rev, v.Settings)
	}
}

// Повторное применение той же формы — новая типография (новый id), но формат и изделие те же:
// холостая правка, без шага и без прироста rev.
func TestSavePrintProfileApplyTwice(t *testing.T) {
	a, _ := applyRig(t)
	first, err := a.SavePrintProfile(editOf(t, a, "fabrika", "Раз"), true)
	if err != nil {
		t.Fatal(err)
	}
	pr, _ := printprof.Current().ByID(first.ID)
	e := editOf(t, a, first.ID, "Два")
	second, err := a.SavePrintProfile(e, true)
	if err != nil {
		t.Fatal(err)
	}
	if second.View.Settings != first.View.Settings {
		t.Fatalf("формат разошёлся: %+v и %+v", first.View.Settings, second.View.Settings)
	}
	// Изделия у копий разные (own-<id>-n): обложка меняется, и это один шаг.
	if second.View.Cover.Preset == first.View.Cover.Preset || pr.Default == "" {
		t.Fatalf("изделие второй: %q, первой: %q", second.View.Cover.Preset, first.View.Cover.Preset)
	}
	a.Undo()
	if v := a.editor().View(); v.Cover.Preset != first.View.Cover.Preset || !v.CanUndo {
		t.Fatalf("после отмены второго: %q undo=%v", v.Cover.Preset, v.CanUndo)
	}
	// ownp.Apply той же типографии подряд — холостая: rev и история не растут.
	if _, err := a.ownp.Apply(first.ID); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	if v, err := a.ownp.Apply(first.ID); err != nil || v.Rev != rev {
		t.Fatalf("повтор: rev %d→%d %v", rev, v.Rev, err)
	}
}

// Альбом без обложки и типография без изделий: меняется только формат.
func TestSavePrintProfileApplyBare(t *testing.T) {
	a, _ := printApp(t)
	before := a.editor().View()
	e := PrintEdit{Name: "Голая", DPI: 300, Quality: 95, Safe: 10, Bleed: 3, Formats: []PrintEditFormat{{Name: "A4", PxW: 3508, PxH: 2480}}}
	res, err := a.SavePrintProfile(e, true)
	if err != nil {
		t.Fatal(err)
	}
	pr, _ := printprof.Current().ByID(res.ID)
	if res.View.Cover != nil || res.View.Settings.PageW != pr.Formats[0].Page[0] || !res.View.CanUndo {
		t.Fatalf("голая: %+v", res.View.Settings)
	}
	if u := a.Undo(); u.Settings != before.Settings {
		t.Errorf("Undo: %+v", u.Settings)
	}
}

// QA-1: цепочка смен типографии с обложкой — каждая отдельный шаг; отмена возвращает и формат,
// и изделие обложки прежней типографии (по View.Cover.Preset окно узнаёт её), повтор — свою.
func TestApplyPrintUndoChain(t *testing.T) {
	a, start := applyRig(t)
	presets := []string{start.Cover.Preset}
	views := []model.View{start}
	for _, id := range []string{"netprint", "fabrika"} {
		v, err := a.ownp.Apply(id)
		if err != nil {
			t.Fatal(err)
		}
		presets, views = append(presets, v.Cover.Preset), append(views, v)
	}
	res, err := a.SavePrintProfile(editOf(t, a, "fabrika", "Своя"), true)
	if err != nil {
		t.Fatal(err)
	}
	presets, views = append(presets, res.View.Cover.Preset), append(views, res.View)
	if want := []string{"fsfera-thin", "netprint-premium", "fabrika-photo", res.ID + "-1"}; !slices.Equal(presets, want) {
		t.Fatalf("изделия по шагам %q, ждали %q", presets, want)
	}
	for i := len(views) - 2; i >= 0; i-- {
		if u := a.Undo(); u.Settings != views[i].Settings || u.Cover.Preset != presets[i] {
			t.Fatalf("отмена к шагу %d: %+v %q", i, u.Settings, u.Cover.Preset)
		}
	}
	for i := 1; i < len(views); i++ {
		if r := a.Redo(); r.Settings != views[i].Settings || r.Cover.Preset != presets[i] {
			t.Fatalf("повтор к шагу %d: %+v %q", i, r.Settings, r.Cover.Preset)
		}
	}
}

// QA-1: без обложки типография в проекте — только числа страницы. Своя с теми же числами, что у
// выбранной Фабрики, при «Сохранить» холостая (шага нет): отмена снимает прежний шаг — выбор Фабрики.
func TestSavePrintProfileApplySameNumbers(t *testing.T) {
	a, _ := printApp(t)
	before := a.editor().View()
	fab, err := a.ownp.Apply("fabrika")
	if err != nil || fab.Rev == before.Rev {
		t.Fatalf("Фабрика: %v", err)
	}
	res, err := a.SavePrintProfile(editOf(t, a, "fabrika", "Своя"), true)
	if err != nil {
		t.Fatal(err)
	}
	if v := res.View; v.Rev != fab.Rev || v.Settings != fab.Settings || v.Cover != nil {
		t.Fatalf("холостая правка дала шаг: rev %d→%d %+v", fab.Rev, v.Rev, v.Settings)
	}
	if u := a.Undo(); u.Settings != before.Settings || u.CanUndo {
		t.Fatalf("отмена: %+v undo=%v", u.Settings, u.CanUndo)
	}
	if r := a.Redo(); r.Settings != fab.Settings || r.CanRedo {
		t.Fatalf("повтор: %+v redo=%v", r.Settings, r.CanRedo)
	}
}
