package variety

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// famPool — пул с ключами по порядку и заданными формами.
func famPool(fs ...forms.Form) []Cand {
	out := make([]Cand, len(fs))
	for i, f := range fs {
		out[i] = Cand{Result: layout.Result{Key: string(rune('a' + i))}, Info: Info{Form: f}}
	}
	return out
}

func famKeys(cs []Cand) string {
	var b []byte
	for _, c := range cs {
		b = append(b, c.Key...)
	}
	return string(b)
}

// TestByFamilyKeepsOrder (065): выборка — разрешённые семейства формы в прежнем порядке, несколько
// семейств сразу; nil — «Все», тот же срез без копии; пустой пул — пусто без запасного.
func TestByFamilyKeepsOrder(t *testing.T) {
	pool := famPool(forms.Mosaic, forms.Strip, forms.Hero, forms.Rows, forms.Mosaic, forms.Air, forms.Spine)
	got, fb := ByFamily(pool, nil, layout.Params{})
	if fb || len(got) != len(pool) || &got[0] != &pool[0] {
		t.Fatalf("nil: %q fb=%v — ждали тот же пул", famKeys(got), fb)
	}
	cases := []struct {
		allow map[Family]bool
		want  string
	}{
		{map[Family]bool{Dense: true}, "ae"},
		{map[Family]bool{Strip: true}, "bd"},
		{map[Family]bool{Dense: true, Hero: true}, "aceg"},
		{map[Family]bool{Air: true, Strip: true}, "bdf"},
	}
	for _, c := range cases {
		if got, fb := ByFamily(pool, c.allow, layout.Params{}); fb || famKeys(got) != c.want {
			t.Errorf("%v: %q fb=%v, ждали %q", c.allow, famKeys(got), fb, c.want)
		}
	}
	if got, fb := ByFamily(nil, map[Family]bool{Dense: true}, layout.Params{}); fb || len(got) != 0 {
		t.Errorf("пустой пул: %q fb=%v", famKeys(got), fb)
	}
	if !slices.Equal(Families(), []Family{Dense, Hero, Air, Bleed, Strip, Figure}) {
		t.Errorf("Families: %v", Families())
	}
}

// TestByFamilyFallbackBest (065): разрешённых нет — только лучшая (pool[0]) и флаг запасного,
// а не весь пул (он дал бы ленты и воздух вместо «Плотно»).
func TestByFamilyFallbackBest(t *testing.T) {
	pool := famPool(forms.Strip, forms.Air, forms.Ladder)
	got, fb := ByFamily(pool, map[Family]bool{Dense: true, Bleed: true}, layout.Params{})
	if !fb || famKeys(got) != "a" {
		t.Fatalf("запасной: %q fb=%v, ждали «a» и true", famKeys(got), fb)
	}
	// Пустой набор (не nil) — тоже ничего не разрешено: запасной.
	if got, fb := ByFamily(pool, map[Family]bool{}, layout.Params{}); !fb || famKeys(got) != "a" {
		t.Errorf("пустой набор: %q fb=%v", famKeys(got), fb)
	}
}
