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

// 094: «Название разворота…» — F2, группа разворотов, функция «Названия», переназначаемая;
// F2 больше ни у кого (конфликты в области проверяет TestCatalogNoConflictsInScope).
func TestRenameSpreadF2(t *testing.T) {
	i, ok := byID["renameSpread"]
	if !ok {
		t.Fatal("в каталоге нет renameSpread")
	}
	x := catalog[i]
	if x.Group != "spreads" || x.Feature != "names" || x.Scope != ScopeApp || x.Fixed || x.Dialogs {
		t.Errorf("renameSpread: %+v", x)
	}
	if len(x.Defaults) != 1 || x.Defaults[0] != "F2" {
		t.Errorf("клавиши по умолчанию %v, ждали [F2]", x.Defaults)
	}
	for _, o := range catalog {
		if o.ID != x.ID && slices.Contains(o.Defaults, "F2") {
			t.Errorf("F2 занята и командой %s", o.ID)
		}
	}
}

// 0.30-141: пять команд разворота из правого щелчка — группа разворотов, везде, без клавиш по умолчанию;
// функции уровня — как у пунктов правого щелчка (разрез и склейка — «timeline», виньетка — «vignette»).
func TestSpreadCommandsNoKeys(t *testing.T) {
	want := []struct{ id, feature string }{
		{"insertSpread", ""},
		{"splitSpread", "timeline"},
		{"joinSpread", "timeline"},
		{"sortVignette", "vignette"},
		{"clearVignette", "vignette"},
	}
	for _, w := range want {
		i, ok := byID[w.id]
		if !ok {
			t.Errorf("в каталоге нет %s", w.id)
			continue
		}
		x := catalog[i]
		if x.Group != "spreads" || x.Scope != ScopeApp || x.Feature != w.feature || x.Fixed || x.Dialogs {
			t.Errorf("%s: %+v", w.id, x)
		}
		if len(x.Defaults) != 0 {
			t.Errorf("%s: клавиши по умолчанию %v, ждали пусто", w.id, x.Defaults)
		}
	}
}

// 0.26-124-10: «Найти фото по имени» — Ctrl/⌘+F, группа фото, везде, без функции, переназначаемая;
// одинокий KeyF («Во весь экран») — другое сочетание.
func TestFindPhotosModF(t *testing.T) {
	i, ok := byID["findPhotos"]
	if !ok {
		t.Fatal("в каталоге нет findPhotos")
	}
	x := catalog[i]
	if x.Group != "photos" || x.Feature != "" || x.Scope != ScopeApp || x.Fixed || x.Dialogs {
		t.Errorf("findPhotos: %+v", x)
	}
	if x.Label != "Найти фото по имени" || x.Short != "Найти" {
		t.Errorf("подписи findPhotos: %q / %q", x.Label, x.Short)
	}
	if len(x.Defaults) != 1 || x.Defaults[0] != "Mod+KeyF" {
		t.Errorf("клавиши по умолчанию %v, ждали [Mod+KeyF]", x.Defaults)
	}
	for _, o := range catalog {
		if o.ID != x.ID && slices.Contains(o.Defaults, "Mod+KeyF") {
			t.Errorf("Mod+KeyF занята и командой %s", o.ID)
		}
	}
	if big := catalog[byID["toggleBig"]]; !slices.Equal(big.Defaults, []string{"KeyF"}) {
		t.Errorf("toggleBig изменился: %v", big.Defaults)
	}
}

// 0.26-128-2: «Крупный просмотр фото» — Z, группа «Вид», везде, без функции уровня, переназначаемая;
// Z не занята ничем, кроме Mod+Z/Mod+Shift+Z (другие сочетания).
func TestPhotoPreviewKeyZ(t *testing.T) {
	i, ok := byID["photoPreview"]
	if !ok {
		t.Fatal("в каталоге нет photoPreview")
	}
	x := catalog[i]
	if x.Group != "view" || x.Feature != "" || x.Scope != ScopeApp || x.Fixed || x.Dialogs || x.Repeat {
		t.Errorf("photoPreview: %+v", x)
	}
	if x.Label != "Крупный просмотр фото" || x.Short != "Крупно" || utf8.RuneCountInString(x.Short) > 10 {
		t.Errorf("подписи photoPreview: %q / %q", x.Label, x.Short)
	}
	if !slices.Equal(x.Defaults, []string{"KeyZ"}) {
		t.Errorf("клавиши по умолчанию %v, ждали [KeyZ]", x.Defaults)
	}
	for _, o := range catalog {
		if o.ID != x.ID && slices.Contains(o.Defaults, "KeyZ") {
			t.Errorf("KeyZ занята и командой %s", o.ID)
		}
	}
}

// 0.29-138: «Сравнить фото» — C, «Похожие к этому» — Shift+C; группа «Вид», везде, без функции уровня,
// переназначаемые, короткая подпись до 10 знаков; обе клавиши свободны (кроме друг друга по модификатору).
func TestCompareKeysCatalog(t *testing.T) {
	for _, w := range []struct{ id, label, short, key string }{
		{"photoCompare", "Сравнить фото", "Сравнить", "KeyC"},
		{"photoSimilar", "Похожие к этому", "Похожие", "Shift+KeyC"},
	} {
		i, ok := byID[w.id]
		if !ok {
			t.Fatalf("в каталоге нет %s", w.id)
		}
		x := catalog[i]
		if x.Group != "view" || x.Feature != "" || x.Scope != ScopeApp || x.Fixed || x.Dialogs || x.Repeat {
			t.Errorf("%s: %+v", w.id, x)
		}
		if x.Label != w.label || x.Short != w.short || utf8.RuneCountInString(x.Short) > 10 {
			t.Errorf("подписи %s: %q / %q", w.id, x.Label, x.Short)
		}
		if !slices.Equal(x.Defaults, []string{w.key}) {
			t.Errorf("%s: клавиши по умолчанию %v, ждали [%s]", w.id, x.Defaults, w.key)
		}
		for _, o := range catalog {
			if o.ID != x.ID && slices.Contains(o.Defaults, w.key) {
				t.Errorf("%s занята и командой %s", w.key, o.ID)
			}
		}
	}
	// compareLevels («Уровни и функции…») — другая команда, имена не пересекаются.
	if _, ok := byID["compareLevels"]; !ok {
		t.Error("compareLevels пропал из каталога")
	}
}

// 0.26-128-15: режим F называется «Во весь экран»; id, клавиша и функция прежние, Aliases не нужен.
func TestToggleBigFullScreen(t *testing.T) {
	i, ok := byID["toggleBig"]
	if !ok {
		t.Fatal("в каталоге нет toggleBig")
	}
	x := catalog[i]
	if x.Label != "Во весь экран" || x.Short != "Весь экран" {
		t.Errorf("подписи toggleBig: %q / %q", x.Label, x.Short)
	}
	if x.Group != "view" || x.Feature != "timeline" || !slices.Equal(x.Defaults, []string{"KeyF"}) {
		t.Errorf("toggleBig: %+v", x)
	}
	if _, ok := Aliases["toggleBig"]; ok {
		t.Error("toggleBig не должен быть в Aliases")
	}
}
