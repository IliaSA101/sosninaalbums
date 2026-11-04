package app

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 113-4: SetTour пишет карту в config.json, проект не трогает (ни rev, ни dirty, ни undo);
// круг Save → Load через State.Prefs.
func TestSetTourRoundTrip(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if _, err := a.NewSpread(-1, nil); err != nil { // проект «грязный» и с историей
		t.Fatal(err)
	}
	before := a.editor().View()
	if !before.Dirty || !before.CanUndo {
		t.Fatalf("исходное состояние проекта: dirty=%v canUndo=%v", before.Dirty, before.CanUndo)
	}
	// Первый запуск: тур должен стартовать сам, запись pending флаг не гасит.
	if p := a.GetState().Prefs; !p.TourDue || p.TourMore {
		t.Fatalf("первый запуск: %+v", p)
	}
	if p := a.SetTour("first", "pending"); !p.TourDue || p.TourMore || !p.First {
		t.Fatalf("pending: %+v", p)
	}
	a.SetLevel(prefs.LevelFull) // выбор уровня в стартовом окне
	if p := a.GetState().Prefs; !p.TourDue {
		t.Fatalf("pending пережил смену уровня, а TourDue нет: %+v", p)
	}
	if p := a.SetTour("first", "done"); p.TourDue || !p.TourMore {
		t.Fatalf("done: %+v", p)
	}
	if s := compact(t, prefs.ConfigPath(a.cfgDir)); !strings.Contains(s, `"tours":{"first":"done"}`) {
		t.Fatalf("tours в файле: %s", s)
	}
	if p := a.SetTour("spread", "done"); p.TourDue || p.TourMore {
		t.Fatalf("spread: %+v", p)
	}
	if got := a.loadConfig().Tours; len(got) != 2 || got["first"] != "done" || got["spread"] != "done" {
		t.Fatalf("tours: %v", got)
	}
	after := a.editor().View()
	if after.Rev != before.Rev || after.Dirty != before.Dirty || after.CanUndo != before.CanUndo || after.CanRedo != before.CanRedo {
		t.Fatalf("SetTour тронул проект: rev %d → %d, dirty %v → %v", before.Rev, after.Rev, before.Dirty, after.Dirty)
	}
}

// Негодное — холостая запись: файл и проект прежние, нового ключа нет.
func TestSetTourInvalidNoop(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.SetLevel(prefs.LevelFull)
	want := compact(t, prefs.ConfigPath(a.cfgDir))
	for _, c := range [][2]string{{"", "done"}, {"first", "later"}, {"bad id", "done"}, {"first", ""}} {
		if p := a.SetTour(c[0], c[1]); p.TourDue || p.TourMore {
			t.Errorf("SetTour(%q, %q): %+v", c[0], c[1], p)
		}
	}
	if got := compact(t, prefs.ConfigPath(a.cfgDir)); got != want {
		t.Errorf("файл изменился:\n%s\n%s", want, got)
	}
}

// 113-3: SetTour не трогает уровень, флажок, «Свой»; их записи не трогают tours.
func TestSetTourKeepsOtherPrefs(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.SetCustom(prefs.CustomLevel{Base: prefs.LevelFull, Off: []string{"zoom"}})
	a.SetLevel(prefs.LevelCustom)
	a.SetShowStart(false)
	p := a.SetTour("first", "done")
	if p.Level != prefs.LevelCustom || p.ShowStart || p.First || p.Custom == nil || p.Custom.Base != prefs.LevelFull {
		t.Fatalf("SetTour затёр чужое поле: %+v", p)
	}
	// Обратное: уровень и флажок не затирают tours.
	a.SetLevel(prefs.LevelStandard)
	a.SetShowStart(true)
	a.SetCustom(prefs.CustomLevel{Base: prefs.LevelSimple})
	if c := a.loadConfig(); c.Tours["first"] != "done" || c.Level != prefs.LevelStandard || c.HideStart {
		t.Fatalf("конфиг %+v", c)
	}
	// Файл от новой версии: чужой id в tours и чужой ключ целы.
	a.updateConfig(func(c *prefs.Config) { c.Tours["future"] = "later" })
	a.SetTour("spread", "done")
	if got := a.loadConfig().Tours; got["future"] != "later" || got["spread"] != "done" || got["first"] != "done" {
		t.Fatalf("tours: %v", got)
	}
}
