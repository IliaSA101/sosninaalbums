package keymap

import (
	"slices"
	"testing"
	"unicode/utf8"
)

func TestCatalogIDsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, x := range catalog {
		if !validID(x.ID) {
			t.Errorf("id %q: не [a-zA-Z0-9]{1,32}", x.ID)
		}
		if seen[x.ID] {
			t.Errorf("id %q повторяется", x.ID)
		}
		seen[x.ID] = true
	}
	if len(byID) != len(catalog) {
		t.Errorf("byID: %d, каталог: %d", len(byID), len(catalog))
	}
}

// Строка каталога заполнена: название, короткое имя на клавишу, известные группа и область.
func TestCatalogShape(t *testing.T) {
	gs := map[string]bool{}
	for _, g := range groups {
		gs[g.ID] = true
	}
	for _, x := range catalog {
		if x.Label == "" || x.Short == "" || utf8.RuneCountInString(x.Short) > 10 {
			t.Errorf("%s: название %q, короткое %q (≤ 10 знаков)", x.ID, x.Label, x.Short)
		}
		if !gs[x.Group] || rank(x.Scope) == 0 {
			t.Errorf("%s: группа %q, область %q", x.ID, x.Group, x.Scope)
		}
		// Под окном действуют только отмена и повтор (074-18).
		if x.Dialogs != (x.ID == "undo" || x.ID == "redo") {
			t.Errorf("%s: dialogs = %v", x.ID, x.Dialogs)
		}
	}
}

func TestCatalogDefaultsCanon(t *testing.T) {
	for _, x := range catalog {
		for _, k := range x.Defaults {
			if cc, ok := Canon(k); !ok || cc != k {
				t.Errorf("%s: %q не каноническое (%q, %v)", x.ID, k, cc, ok)
			}
		}
		if len(slices.Compact(slices.Sorted(slices.Values(x.Defaults)))) != len(x.Defaults) {
			t.Errorf("%s: повтор в стандартных %v", x.ID, x.Defaults)
		}
		if !x.Fixed && len(x.Defaults) > MaxKeys {
			t.Errorf("%s: стандартных больше %d", x.ID, MaxKeys)
		}
	}
}

// У служебных команд — только служебные клавиши, у остальных — ни одной служебной.
func TestCatalogFixedOnlyReserved(t *testing.T) {
	for _, x := range catalog {
		for _, k := range x.Defaults {
			if why := ReservedReason(k); (why != "") != x.Fixed {
				t.Errorf("%s (fixed=%v): %q — причина %q", x.ID, x.Fixed, k, why)
			}
		}
	}
}

func TestCatalogNoConflictsInScope(t *testing.T) {
	owner := map[Scope]map[string]string{}
	for _, x := range catalog {
		if x.Fixed {
			continue
		}
		if owner[x.Scope] == nil {
			owner[x.Scope] = map[string]string{}
		}
		for _, k := range x.Defaults {
			if o := owner[x.Scope][k]; o != "" {
				t.Errorf("%s: %q уже у %s в области %s", x.ID, k, o, x.Scope)
			}
			owner[x.Scope][k] = x.ID
		}
	}
}

// defaultShadows — стандартные клавиши, общие у команд пересекающихся областей разного ранга.
func defaultShadows() []string {
	var out []string
	eff := Resolve(nil)
	for _, x := range catalog {
		for _, r := range shadows(eff, x, x.Defaults) {
			if !slices.Contains(out, r.Chord) {
				out = append(out, r.Chord)
			}
		}
	}
	slices.Sort(out)
	return out
}

// Перекрытия в стандартных — ровно известные (design §2): 0 «вписать» / «звёзды: убрать»,
// Del и Backspace «удалить» / «убрать фото фона». Новое перекрытие — решение, а не случайность.
func TestCatalogShadowsKnown(t *testing.T) {
	want := []string{"Backspace", "Delete", "Digit0"}
	if got := defaultShadows(); !slices.Equal(got, want) {
		t.Errorf("перекрытия %v, ждали %v", got, want)
	}
}

func TestCatalogCopy(t *testing.T) {
	cs := Catalog()
	if len(cs) != len(catalog) {
		t.Fatalf("Catalog: %d", len(cs))
	}
	cs[0].Defaults[0] = "KeyQ"
	if catalog[0].Defaults[0] == "KeyQ" {
		t.Error("Catalog делит списки клавиш с пакетом")
	}
}

func TestOverlaps(t *testing.T) {
	cases := []struct {
		a, b Scope
		want bool
	}{
		{ScopeApp, ScopeLibrary, true}, {ScopeCanvas, ScopeApp, true}, {ScopeBg, ScopeCanvas, true},
		{ScopeLibrary, ScopeBg, true}, {ScopeCanvas, ScopeLibrary, false}, {ScopeLibrary, ScopeCanvas, false},
		{ScopeBg, ScopeBg, true},
	}
	for _, c := range cases {
		if got := overlaps(c.a, c.b); got != c.want {
			t.Errorf("overlaps(%s, %s) = %v", c.a, c.b, got)
		}
	}
}
