package keymap

import (
	"slices"
	"testing"
)

// TestClipCommands: четыре команды «Копировать и вставить» (099) — на холсте, со своими
// Ctrl+X/C/V/D, функция clipboard; клавиши не пересекаются с другими командами области, а
// общий разбор (Resolve) отдаёт их без потерь и перекрытий.
func TestClipCommands(t *testing.T) {
	want := map[string]string{
		"cutItems": "Mod+KeyX", "copyItems": "Mod+KeyC", "pasteItems": "Mod+KeyV", "duplicateItems": "Mod+KeyD",
	}
	eff := Resolve(nil)
	for id, key := range want {
		x, ok := lookup(id)
		if !ok {
			t.Fatalf("%s: нет в каталоге", id)
		}
		if x.Group != "photos" || x.Scope != ScopeCanvas || x.Feature != "clipboard" || x.Fixed {
			t.Errorf("%s: группа %q, область %q, функция %q, служебная %v", id, x.Group, x.Scope, x.Feature, x.Fixed)
		}
		if !slices.Equal(x.Defaults, []string{key}) || !slices.Equal(eff[id], []string{key}) {
			t.Errorf("%s: стандартные %v, действующие %v, нужно [%s]", id, x.Defaults, eff[id], key)
		}
	}
	// Ни одна другая команда не держит эти же сочетания по умолчанию (в любой области).
	for _, x := range catalog {
		if _, own := want[x.ID]; own {
			continue
		}
		for _, k := range x.Defaults {
			for id, key := range want {
				if k == key {
					t.Errorf("%s и %s делят %s", x.ID, id, k)
				}
			}
		}
	}
	if x, _ := lookup("copyItems"); x.Hint == "" {
		t.Error("у copyItems нет подсказки")
	}
	// Назначение занятого сочетания другой команде — конфликт, а не тихая подмена.
	if _, out, err := Assign(nil, "removeCell", "", "Mod+KeyC", false); err != nil || out.Conflict == nil || out.Conflict.ID != "copyItems" {
		t.Errorf("Mod+KeyC на removeCell: конфликт %+v, ошибка %v", out.Conflict, err)
	}
}
