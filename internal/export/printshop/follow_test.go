package printshop

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// TestProfilesFollowSnapshot — Profiles и ProfileOf читают действующий снимок реестра, а не
// встроенный: свой профиль с export появляется в «Печати», а профиль без export и снимок без
// профилей — нет.
func TestProfilesFollowSnapshot(t *testing.T) {
	kinds := func() []string {
		var out []string
		for _, pf := range Profiles() {
			out = append(out, pf.Kind)
		}
		return out
	}
	if got := kinds(); len(got) != 2 || got[0] != "fsfera" || got[1] != "fabrika" {
		t.Fatalf("встроенные: %v", got)
	}
	build := func(ids ...string) *printprof.Snapshot {
		var ps []printprof.Profile
		for _, id := range ids {
			p, ok := printprof.Builtin().ByID(id)
			if !ok {
				t.Fatalf("нет %s", id)
			}
			ps = append(ps, p)
		}
		s, err := printprof.NewSnapshot(ps)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	t.Run("только Фабрика", func(t *testing.T) {
		printprof.UseForTest(t, build("fabrika"))
		if got := kinds(); len(got) != 1 || got[0] != "fabrika" {
			t.Errorf("Profiles: %v", got)
		}
		if _, ok := ProfileOf("fsfera"); ok {
			t.Error("ProfileOf нашёл Фотосферу, которой нет в снимке")
		}
	})
	t.Run("только Нетпринт — без выгрузки", func(t *testing.T) {
		printprof.UseForTest(t, build("netprint"))
		if got := kinds(); len(got) != 0 {
			t.Errorf("Profiles: %v", got)
		}
	})
	t.Run("пустой снимок", func(t *testing.T) {
		empty, err := printprof.NewSnapshot(nil)
		if err != nil {
			t.Fatal(err)
		}
		printprof.UseForTest(t, empty)
		if got := kinds(); len(got) != 0 {
			t.Errorf("Profiles: %v", got)
		}
	})
	if got := kinds(); len(got) != 2 {
		t.Errorf("после подмены встроенные не вернулись: %v", got)
	}
}
