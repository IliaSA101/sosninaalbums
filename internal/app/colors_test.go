package app

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Remember → Forget → Import через App: список в config.json, второй экземпляр его видит, альбом
// не меняется (rev прежний, шага отмены нет).
func TestMyColorsMethods(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	rev := a.editor().View().Rev
	if got := a.MyColors(); got == nil || len(got) != 0 {
		t.Fatalf("без файла: %#v", got)
	}
	a.RememberColor("#112233")
	if got := a.RememberColor("#AABBCC"); !slices.Equal(got, []string{"#aabbcc", "#112233"}) {
		t.Fatalf("Remember: %v", got)
	}
	if got := a.RememberColor("мусор"); len(got) != 2 {
		t.Fatalf("мусор принят: %v", got)
	}
	if got := a.ForgetColor("#112233"); !slices.Equal(got, []string{"#aabbcc"}) {
		t.Fatalf("Forget: %v", got)
	}
	if got := a.ForgetColor("#000000"); !slices.Equal(got, []string{"#aabbcc"}) {
		t.Fatalf("Forget нет такого: %v", got)
	}
	// Перенос при непустом списке ничего не меняет.
	if got := a.ImportColors([]string{"#ffffff"}); !slices.Equal(got, []string{"#aabbcc"}) {
		t.Fatalf("Import в непустой: %v", got)
	}
	// Круг: другой экземпляр читает тот же config.json.
	b := newApp(t)
	b.cfgDir = a.cfgDir
	if got := b.MyColors(); !slices.Equal(got, []string{"#aabbcc"}) {
		t.Fatalf("после перезапуска: %v", got)
	}
	// Перенос в пустой.
	b.ForgetColor("#aabbcc")
	if got := b.ImportColors([]string{"#ffffff", "x", "#000000"}); !slices.Equal(got, []string{"#ffffff", "#000000"}) {
		t.Fatalf("Import в пустой: %v", got)
	}
	if v := a.editor().View(); v.Rev != rev {
		t.Errorf("методы цвета изменили альбом: rev %d → %d", rev, v.Rev)
	}
	if prefs.LoadConfig(a.cfgDir).MyColors == nil {
		t.Error("список не в config.json")
	}
}
