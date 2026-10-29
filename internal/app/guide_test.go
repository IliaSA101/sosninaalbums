package app

import (
	"os"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Отметки через App: список в config.json, другой экземпляр (перезапуск) его видит, альбом не
// меняется: rev, dirty и undo целы, «Отменить» не появляется.
func TestMarkGuideRoundTrip(t *testing.T) {
	a := newApp(t)
	// Альбом с историей: правка через Editor, чтобы rev, dirty и undo не были нулевыми.
	if _, err := a.editor().NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	before := a.editor().View()
	if !before.CanUndo || !before.Dirty || before.Rev == 0 {
		t.Fatalf("правка не оставила следа: %+v", before)
	}
	if got := a.GuideLearned(); got == nil || len(got) != 0 {
		t.Fatalf("без файла: %#v", got)
	}
	a.MarkGuide([]string{"auto-build"}, true)
	if got := a.MarkGuide([]string{"layers", "auto-build", "vignette"}, true); !slices.Equal(got, []string{"auto-build", "layers", "vignette"}) {
		t.Fatalf("несколько сразу: %v", got)
	}
	if got := a.MarkGuide([]string{"layers"}, false); !slices.Equal(got, []string{"auto-build", "vignette"}) {
		t.Fatalf("снять: %v", got)
	}
	b := newApp(t)
	b.cfgDir = a.cfgDir
	if got := b.GuideLearned(); !slices.Equal(got, []string{"auto-build", "vignette"}) {
		t.Fatalf("после перезапуска: %v", got)
	}
	if prefs.LoadConfig(a.cfgDir).GuideLearned == nil {
		t.Error("список не в config.json")
	}
	if v := a.editor().View(); v.Rev != before.Rev || v.Dirty != before.Dirty || v.CanUndo != before.CanUndo {
		t.Errorf("альбом изменился: rev %d → %d, dirty %v → %v, undo %v → %v",
			before.Rev, v.Rev, before.Dirty, v.Dirty, before.CanUndo, v.CanUndo)
	}
	// Один шаг отмены снимает именно правку альбома, а не отметки F1.
	if v := a.Undo(); len(v.Spreads) != len(before.Spreads)-1 {
		t.Errorf("отмена не убрала разворот: %d → %d", len(before.Spreads), len(v.Spreads))
	}
}

// Негодные id и повторы: файл не создаётся, ответ — пустой список, не nil.
func TestMarkGuideInvalidNoWrite(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	for _, ids := range [][]string{nil, {}, {"", "A", "x y", "../../etc", "-a"}} {
		if got := a.MarkGuide(ids, true); got == nil || len(got) != 0 {
			t.Errorf("%v: %#v", ids, got)
		}
	}
	if got := a.MarkGuide([]string{"нет"}, false); got == nil || len(got) != 0 {
		t.Errorf("снять отсутствующее: %#v", got)
	}
	if _, err := os.Stat(prefs.ConfigPath(a.cfgDir)); err == nil {
		t.Error("config.json создан без изменений")
	}
}
