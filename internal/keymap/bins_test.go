package keymap

import (
	"reflect"
	"slices"
	"testing"
)

// Подборки (083 G3): четыре команды ленты фото после отметок, функция «bins»; B, Shift+B, Del/Backspace.
func TestCatalogBins(t *testing.T) {
	want := map[string][]string{
		"binToggle": {"KeyB"}, "binNew": {"Shift+KeyB"}, "binRemove": {"Delete", "Backspace"}, "binAll": {},
	}
	lastMark, firstBin, n := -1, -1, 0
	for i, x := range catalog {
		switch x.Group {
		case "marks":
			lastMark = i
		case "bins":
			n++
			if firstBin < 0 {
				firstBin = i
			}
			keys, ok := want[x.ID]
			if !ok || !slices.Equal(x.Defaults, keys) {
				t.Errorf("%s: клавиши %v, ждали %v", x.ID, x.Defaults, keys)
			}
			if x.Scope != ScopeLibrary || x.Feature != "bins" || x.Fixed {
				t.Errorf("%s: область %q, функция %q, fixed %v", x.ID, x.Scope, x.Feature, x.Fixed)
			}
		}
	}
	if n != len(want) || firstBin < lastMark {
		t.Errorf("команд подборок %d (ждали %d), первая на %d, последняя отметка на %d", n, len(want), firstBin, lastMark)
	}
	i := slices.IndexFunc(groups, func(g Named) bool { return g.ID == "bins" })
	if i < 1 || groups[i-1].ID != "marks" || groups[i].Title != "Подборки фото" {
		t.Errorf("группа bins на %d: %v", i, groups)
	}
}

// Свои сочетания подборок: конфликт в ленте фото, круг через Sanitize (файл настроек), несколько
// команд сразу, сброс, холостая правка, снятие всех клавиш.
func TestBinsOverrides(t *testing.T) {
	// binToggle → KeyP занята markPick той же области: вопрос, без записи.
	o, out := mustAssign(t, nil, "binToggle", "KeyB", "KeyP", false)
	if out.Conflict == nil || out.Conflict.ID != "markPick" || len(o) != 0 {
		t.Fatalf("конфликт: %+v, o %v", out, o)
	}
	// «Заменить»: у markPick клавиша снята, у binToggle — P.
	o, out = mustAssign(t, o, "binToggle", "KeyB", "KeyP", true)
	if out.Replaced == nil || !slices.Equal(keys(o, "binToggle"), []string{"KeyP"}) || len(keys(o, "markPick")) != 0 {
		t.Fatalf("замена: %+v, %v, %v", out, keys(o, "binToggle"), keys(o, "markPick"))
	}
	// Круг Save→Load: Sanitize не меняет итог.
	if back := Sanitize(o); !reflect.DeepEqual(Resolve(back), Resolve(o)) {
		t.Errorf("после Sanitize клавиши другие: %v и %v", Resolve(back), Resolve(o))
	}
	// Сразу несколько подборок: binNew и binAll получили клавиши.
	o, _ = mustAssign(t, o, "binAll", "", "Alt+KeyB", false)
	o, _ = mustAssign(t, o, "binNew", "Shift+KeyB", "Alt+Shift+KeyB", false)
	if v := Build(o); v.Changed != 4 || !slices.Equal(v.Custom["binAll"], []string{"Alt+KeyB"}) {
		t.Errorf("Changed %d, custom %v", Build(o).Changed, Build(o).Custom)
	}
	// Сброс одной команды возвращает стандартные B.
	o, _, err := Reset(o, "binToggle", true)
	if err != nil || !slices.Equal(keys(o, "binToggle"), []string{"KeyB"}) {
		t.Errorf("сброс: %v, %v", keys(o, "binToggle"), err)
	}
	// Холостая правка: то же сочетание — без изменений.
	if o2, out := mustAssign(t, o, "binRemove", "", "Delete", false); !blank(out) || !reflect.DeepEqual(o2, o) {
		t.Errorf("холостая правка: %+v", out)
	}
	// Снять обе клавиши binRemove — команда без клавиши, не ошибка.
	for _, k := range []string{"Delete", "Backspace"} {
		if o, err = Remove(o, "binRemove", k); err != nil {
			t.Fatal(err)
		}
	}
	if len(keys(o, "binRemove")) != 0 {
		t.Errorf("binRemove: %v", keys(o, "binRemove"))
	}
	if got := ResetAll(o); len(got) != 0 {
		t.Errorf("ResetAll оставил %v", got)
	}
}

// «Везде» с клавишей B: перекрытие — в ленте фото сработает подборка. Холст и лента не пересекаются,
// поэтому Delete подборок не отнимает у «удалить выбранное» на холсте.
func TestBinsShadowApp(t *testing.T) {
	_, out := mustAssign(t, nil, "toggleBig", "KeyF", "KeyB", false)
	if out.Conflict != nil || len(out.Shadows) != 1 || out.Shadows[0].ID != "binToggle" {
		t.Errorf("перекрытие: %+v", out)
	}
	if overlaps(ScopeCanvas, ScopeLibrary) {
		t.Error("холст и лента пересекаются")
	}
	if !slices.Equal(Resolve(nil)["removeCell"], []string{"Delete", "Backspace"}) {
		t.Errorf("removeCell: %v", Resolve(nil)["removeCell"])
	}
}
