package app

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/keymap"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// hotkeysApp — App со своей папкой настроек.
func hotkeysApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	return a
}

// keysOf — действующие клавиши команды в виде.
func keysOf(v keymap.View, id string) []string {
	for _, e := range v.Commands {
		if e.ID == id {
			return e.Keys
		}
	}
	return nil
}

// fileHotkeys — своё из config.json (то, что увидит следующий запуск).
func fileHotkeys(a *App) map[string][]string { return prefs.LoadConfig(a.cfgDir).Hotkeys }

func mustSet(t *testing.T, a *App, id, old, chord string, replace bool) keymap.SetResult {
	t.Helper()
	r, err := a.SetHotkey(id, old, chord, replace)
	if err != nil {
		t.Fatalf("SetHotkey(%s, %q→%q): %v", id, old, chord, err)
	}
	return r
}

// Без файла — стандартные; State.Keymap с первого GetState и Keymap() — один вид.
func TestHotkeysStateDefaults(t *testing.T) {
	a := hotkeysApp(t)
	st := a.GetState().Keymap
	if st.Changed != 0 || len(st.Commands) == 0 || !slices.Equal(keysOf(st, "undo"), []string{"Mod+KeyZ"}) {
		t.Fatalf("State.Keymap: changed %d, undo %v", st.Changed, keysOf(st, "undo"))
	}
	if st.Custom == nil || len(st.Groups) == 0 || len(st.Scopes) == 0 || len(st.Locked) == 0 {
		t.Errorf("пустые части вида: custom %v, groups %d, scopes %d, locked %d", st.Custom, len(st.Groups), len(st.Scopes), len(st.Locked))
	}
	if !reflect.DeepEqual(a.Keymap(), st) {
		t.Error("Keymap() и State.Keymap разошлись")
	}
	mustSet(t, a, "toggleBig", "KeyF", "KeyB", false)
	if st := a.GetState().Keymap; st.Changed != 1 || !slices.Equal(keysOf(st, "toggleBig"), []string{"KeyB"}) {
		t.Errorf("State.Keymap после правки: %d %v", st.Changed, keysOf(st, "toggleBig"))
	}
}

// Вопрос «Заменить?» ничего не пишет; «Заменить» снимает клавишу у держателя; оба своих —
// в файле и в новом экземпляре (круг записи и чтения). Ошибка и холостая правка файл не меняют.
func TestHotkeysSetConflictReplace(t *testing.T) {
	a := hotkeysApp(t)
	r := mustSet(t, a, "toggleBig", "KeyF", "KeyG", false)
	if r.Conflict == nil || r.Conflict.ID != "toggleSafe" || r.Conflict.Chord != "KeyG" || r.Replaced != nil {
		t.Fatalf("вопрос: %+v", r)
	}
	if r.View.Changed != 0 || fileHotkeys(a) != nil || r.Shadows == nil {
		t.Errorf("при вопросе записано: changed %d, файл %v, shadows %v", r.View.Changed, fileHotkeys(a), r.Shadows)
	}
	r = mustSet(t, a, "toggleBig", "KeyF", "KeyG", true)
	if r.Replaced == nil || r.Replaced.ID != "toggleSafe" || r.Conflict != nil || r.View.Changed != 2 {
		t.Fatalf("замена: %+v", r)
	}
	want := map[string][]string{"toggleBig": {"KeyG"}, "toggleSafe": {}}
	if got := fileHotkeys(a); !reflect.DeepEqual(got, want) {
		t.Errorf("файл: %v", got)
	}
	b := newApp(t)
	b.cfgDir = a.cfgDir
	if v := b.Keymap(); !slices.Equal(keysOf(v, "toggleBig"), []string{"KeyG"}) || len(keysOf(v, "toggleSafe")) != 0 || v.Changed != 2 {
		t.Errorf("после перезапуска: %v / %v", keysOf(v, "toggleBig"), keysOf(v, "toggleSafe"))
	}

	before, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	for _, c := range []struct{ id, chord string }{
		{"guide", "KeyQ"},             // служебная команда
		{"toggleBig", "Escape"},       // служебная клавиша
		{"toggleBig", "Mod+Alt+KeyB"}, // AltGr
		{"toggleBig", "мусор"},        // не сочетание
		{"noSuchCmd", "KeyQ"},         // неизвестная команда
	} {
		if _, err := a.SetHotkey(c.id, "", c.chord, false); err == nil {
			t.Errorf("%s %q принято", c.id, c.chord)
		}
	}
	// Холостая: сочетание уже у команды.
	if r := mustSet(t, a, "toggleBig", "", "KeyG", false); r.View.Changed != 2 || r.Conflict != nil {
		t.Errorf("холостая: %+v", r)
	}
	if after, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir)); !reflect.DeepEqual(fileHotkeysOf(t, before), fileHotkeysOf(t, after)) {
		t.Errorf("ошибка или холостая правка изменили файл:\n%s\n%s", before, after)
	}
}

// fileHotkeysOf — hotkeys из содержимого config.json.
func fileHotkeysOf(t *testing.T, data []byte) map[string][]string {
	t.Helper()
	dir := testkit.TempDir(t)
	if err := os.WriteFile(prefs.ConfigPath(dir), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return prefs.LoadConfig(dir).Hotkeys
}

// Убрать, вернуть стандартное (с вопросом, если занято своим), сбросить всё и «Вернуть»;
// незнакомый id другой версии переживает всё.
func TestHotkeysRestore(t *testing.T) {
	a := hotkeysApp(t)
	a.updateConfig(func(c *prefs.Config) { c.Hotkeys = map[string][]string{"futureCmd": {"KeyQ"}} })
	mustSet(t, a, "toggleBig", "KeyF", "KeyB", false)
	mustSet(t, a, "toggleDetails", "", "KeyF", false)
	v, err := a.RemoveHotkey("toggleSafe", "KeyG")
	if err != nil || v.Changed != 3 || len(keysOf(v, "toggleSafe")) != 0 {
		t.Fatalf("RemoveHotkey: %v, changed %d", err, v.Changed)
	}
	if _, err := a.RemoveHotkey("guide", "F1"); err == nil {
		t.Error("служебная убрана")
	}
	prev := v.Custom

	// Стандартное F у «Крупного просмотра» теперь у «Панели деталей» — вопрос, файл прежний.
	r, err := a.ResetHotkey("toggleBig", false)
	if err != nil || r.Conflict == nil || r.Conflict.ID != "toggleDetails" || !reflect.DeepEqual(fileHotkeys(a), map[string][]string(prev)) {
		t.Fatalf("ResetHotkey без замены: %v %+v, файл %v", err, r.Conflict, fileHotkeys(a))
	}
	r, err = a.ResetHotkey("toggleBig", true)
	if err != nil || r.Replaced == nil || r.Replaced.ID != "toggleDetails" || !slices.Equal(keysOf(r.View, "toggleBig"), []string{"KeyF"}) {
		t.Fatalf("ResetHotkey с заменой: %v %+v", err, r)
	}

	v = a.ResetHotkeys()
	if v.Changed != 0 || !reflect.DeepEqual(fileHotkeys(a), map[string][]string{"futureCmd": {"KeyQ"}}) {
		t.Fatalf("ResetHotkeys: changed %d, файл %v", v.Changed, fileHotkeys(a))
	}
	if v := a.RestoreHotkeys(prev); v.Changed != 3 || !reflect.DeepEqual(fileHotkeys(a), map[string][]string(prev)) {
		t.Errorf("RestoreHotkeys: changed %d, файл %v, снимок %v", v.Changed, fileHotkeys(a), prev)
	}
	// Пустой снимок (null из интерфейса) — всё стандартное; ключ hotkeys остаётся из-за чужого id.
	if v := a.RestoreHotkeys(nil); v.Changed != 0 || !reflect.DeepEqual(fileHotkeys(a), map[string][]string{"futureCmd": {"KeyQ"}}) {
		t.Errorf("RestoreHotkeys(nil): %d %v", v.Changed, fileHotkeys(a))
	}
	// Без чужого — сброс убирает ключ из файла целиком.
	a.updateConfig(func(c *prefs.Config) { c.Hotkeys = map[string][]string{"toggleBig": {"KeyB"}} })
	a.ResetHotkeys()
	if data, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir)); strings.Contains(string(data), "hotkeys") {
		t.Errorf("пустое своё оставило ключ: %s", data)
	}
}

// Два экземпляра интерфейса (окно и вкладка браузера) на одном config.json: каждый считает от
// файла, а не от своего устаревшего вида, — правки обоих в файле, конфликт виден по файлу.
func TestHotkeysTwoInstances(t *testing.T) {
	a := hotkeysApp(t)
	b := newApp(t)
	b.cfgDir = a.cfgDir
	staleA, staleB := a.Keymap(), b.Keymap()
	mustSet(t, a, "toggleBig", "KeyF", "KeyB", false)
	// b не видел правку a: «B» для «Безопасной зоны» — вопрос о «Крупном просмотре».
	r := mustSet(t, b, "toggleSafe", "KeyG", "KeyB", false)
	if r.Conflict == nil || r.Conflict.ID != "toggleBig" {
		t.Fatalf("конфликт по файлу не найден: %+v", r)
	}
	mustSet(t, b, "toggleDetails", "KeyI", "KeyJ", false)
	want := map[string][]string{"toggleBig": {"KeyB"}, "toggleDetails": {"KeyJ"}}
	if got := fileHotkeys(a); !reflect.DeepEqual(got, want) {
		t.Errorf("файл после двух экземпляров: %v", got)
	}
	// «Вернуть» по снимку b до всех правок возвращает стандартное, чужое из файла не трогает.
	a.updateConfig(func(c *prefs.Config) { c.Hotkeys["futureCmd"] = []string{"KeyQ"} })
	b.RestoreHotkeys(staleB.Custom)
	if got := fileHotkeys(a); !reflect.DeepEqual(got, map[string][]string{"futureCmd": {"KeyQ"}}) {
		t.Errorf("Restore старого снимка: %v", got)
	}
	if staleA.Changed != 0 || staleB.Changed != 0 {
		t.Errorf("старые виды изменились: %d %d", staleA.Changed, staleB.Changed)
	}

	// Одновременные правки разных команд из двух экземпляров — ни одна не потеряна.
	a.ResetHotkeys()
	ids := []string{"addFolder", "relink", "showExport", "settings", "toggleMarkup", "toggleSnap", "allSpreads", "compareLevels"}
	var wg sync.WaitGroup
	for i, id := range ids {
		inst := a
		if i%2 == 1 {
			inst = b
		}
		wg.Go(func() {
			if _, err := inst.SetHotkey(id, "", "Alt+Digit"+string(rune('1'+i)), false); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got := fileHotkeys(a)
	for i, id := range ids {
		if want := []string{"Alt+Digit" + string(rune('1'+i))}; !slices.Equal(got[id], want) {
			t.Errorf("%s: %v, ждали %v", id, got[id], want)
		}
	}
	if v := a.Keymap(); v.Changed != len(ids) {
		t.Errorf("Changed %d", v.Changed)
	}
}

// Испорченный config.json: null и строка вместо списка клавиш — записи нет (стандартная
// клавиша); после правки другой команды в файле нет null, «без клавиши» ([]) цело.
func TestHotkeysNilEntriesDropped(t *testing.T) {
	a := hotkeysApp(t)
	raw := `{"hotkeys":{"undo":null,"redo":"Mod+KeyY","toggleSafe":[]}}`
	if err := os.WriteFile(prefs.ConfigPath(a.cfgDir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	st := a.GetState().Keymap
	if !slices.Equal(keysOf(st, "undo"), []string{"Mod+KeyZ"}) || len(keysOf(st, "redo")) == 0 || len(keysOf(st, "toggleSafe")) != 0 {
		t.Errorf("вид: undo %v, redo %v, toggleSafe %v", keysOf(st, "undo"), keysOf(st, "redo"), keysOf(st, "toggleSafe"))
	}
	mustSet(t, a, "toggleBig", "KeyF", "KeyB", false)
	data, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	if strings.Contains(string(data), "null") || !strings.Contains(string(data), `"toggleSafe": []`) {
		t.Errorf("config.json после правки: %s", data)
	}
	if !slices.Equal(keysOf(a.GetState().Keymap, "undo"), []string{"Mod+KeyZ"}) {
		t.Error("undo потерял стандартную клавишу")
	}
}
