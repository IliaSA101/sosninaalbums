package keymap

import (
	"reflect"
	"slices"
	"testing"
)

// keys — действующие клавиши команды.
func keys(o Overrides, id string) []string { return Resolve(o)[id] }

// blank — итог без вопроса, замены и перекрытий.
func blank(out Outcome) bool {
	return out.Conflict == nil && out.Replaced == nil && len(out.Shadows) == 0
}

func mustAssign(t *testing.T, o Overrides, id, old, chord string, replace bool) (Overrides, Outcome) {
	t.Helper()
	o2, out, err := Assign(o, id, old, chord, replace)
	if err != nil {
		t.Fatalf("Assign(%s, %q→%q): %v", id, old, chord, err)
	}
	return o2, out
}

func TestAssignAdd(t *testing.T) {
	o, out := mustAssign(t, nil, "toggleBig", "", "KeyB", false)
	if !slices.Equal(keys(o, "toggleBig"), []string{"KeyF", "KeyB"}) || out.Conflict != nil || out.Replaced != nil {
		t.Errorf("keys %v, out %+v", keys(o, "toggleBig"), out)
	}
	// Вторая команда сразу: обе своих в файле, Changed — 2.
	o, _ = mustAssign(t, o, "toggleMarkup", "", "Alt+KeyM", false)
	if v := Build(o); v.Changed != 2 || !slices.Equal(v.Custom["toggleMarkup"], []string{"Alt+KeyM"}) {
		t.Errorf("Changed %d, custom %v", v.Changed, v.Custom)
	}
}

func TestAssignChange(t *testing.T) {
	o, _ := mustAssign(t, nil, "toggleBig", "KeyF", "Shift+Mod+KeyB", false)
	if !slices.Equal(keys(o, "toggleBig"), []string{"Mod+Shift+KeyB"}) {
		t.Errorf("toggleBig: %v", keys(o, "toggleBig"))
	}
	// Старое «F» ушло и не досталось никому.
	for id, ks := range Resolve(o) {
		if slices.Contains(ks, "KeyF") {
			t.Errorf("F у %s", id)
		}
	}
	// Замена второй из двух — на том же месте.
	o, _ = mustAssign(t, nil, "redo", "Mod+Shift+KeyZ", "Mod+KeyU", false)
	if !slices.Equal(keys(o, "redo"), []string{"Mod+KeyY", "Mod+KeyU"}) {
		t.Errorf("redo: %v", keys(o, "redo"))
	}
}

// Холостая правка: сочетание уже у команды — своё не появляется.
func TestAssignIdle(t *testing.T) {
	for _, old := range []string{"", "KeyF"} {
		o, out := mustAssign(t, nil, "toggleBig", old, "KeyF", false)
		if len(o) != 0 || !blank(out) {
			t.Errorf("old %q: o %v, out %+v", old, o, out)
		}
	}
	// Заменить одно на уже имеющееся — это убрать прежнее.
	o, _ := mustAssign(t, nil, "redo", "Mod+KeyY", "Mod+Shift+KeyZ", false)
	if !slices.Equal(keys(o, "redo"), []string{"Mod+Shift+KeyZ"}) {
		t.Errorf("redo: %v", keys(o, "redo"))
	}
}

func TestAssignConflictAsks(t *testing.T) {
	in := Overrides{"toggleMarkup": {"KeyM"}}
	o, out := mustAssign(t, in, "toggleBig", "KeyF", "KeyG", false)
	if out.Conflict == nil || *out.Conflict != (Ref{"toggleSafe", "Безопасная зона", "KeyG"}) || out.Replaced != nil {
		t.Fatalf("out %+v", out)
	}
	if !reflect.DeepEqual(o, in) || !slices.Equal(keys(o, "toggleBig"), []string{"KeyF"}) {
		t.Errorf("при вопросе o изменился: %v", o)
	}
	if !slices.Equal(in["toggleMarkup"], []string{"KeyM"}) || len(in) != 1 {
		t.Errorf("вход изменён: %v", in)
	}
}

func TestAssignReplaceTakesFromHolder(t *testing.T) {
	o, out := mustAssign(t, nil, "toggleBig", "KeyF", "KeyG", true)
	if out.Replaced == nil || out.Replaced.ID != "toggleSafe" || out.Conflict != nil {
		t.Fatalf("out %+v", out)
	}
	if !slices.Equal(keys(o, "toggleBig"), []string{"KeyG"}) || len(keys(o, "toggleSafe")) != 0 {
		t.Errorf("big %v, safe %v", keys(o, "toggleBig"), keys(o, "toggleSafe"))
	}
	if v := Build(o); v.Changed != 2 {
		t.Errorf("Changed = %d", v.Changed)
	}
	// У держателя с двумя клавишами снимается только занятая.
	o, out = mustAssign(t, nil, "toggleBig", "", "Mod+KeyY", true)
	if out.Replaced == nil || out.Replaced.ID != "redo" || !slices.Equal(keys(o, "redo"), []string{"Mod+Shift+KeyZ"}) {
		t.Errorf("redo: %v, out %+v", keys(o, "redo"), out)
	}
}

// Клавиша в другой области — без вопроса, с перекрытием; холст и лента фото не пересекаются,
// поэтому Ctrl+A у команды «везде» перекрывает обе.
func TestAssignOtherScopeShadowOnly(t *testing.T) {
	o, out := mustAssign(t, nil, "toggleDetails", "KeyI", "KeyP", false)
	if out.Conflict != nil || len(out.Shadows) != 1 || out.Shadows[0] != (Ref{"markPick", "Флажок «отобрано»", "KeyP"}) {
		t.Errorf("out %+v", out)
	}
	if !slices.Equal(keys(o, "markPick"), []string{"KeyP"}) {
		t.Errorf("markPick: %v", keys(o, "markPick"))
	}
	_, out = mustAssign(t, nil, "addText", "", "Mod+KeyA", false)
	var ids []string
	for _, r := range out.Shadows {
		ids = append(ids, r.ID)
	}
	if out.Conflict != nil || !slices.Equal(ids, []string{"selectSpread", "selectAll"}) {
		t.Errorf("Ctrl+A: %+v", out)
	}
	// Режим фона старше холста: Del у команды холста перекрыт «Убрать фото фона».
	_, out = mustAssign(t, Overrides{"removeCell": {}}, "removeCell", "", "Delete", false)
	if len(out.Shadows) != 1 || out.Shadows[0].ID != "bgRemove" {
		t.Errorf("Del: %+v", out.Shadows)
	}
}

func TestAssignFixedRefused(t *testing.T) {
	for _, id := range []string{"guide", "prevSpread", "nudgePlace", "bgExit", "noSuchCmd", ""} {
		o, out, err := Assign(nil, id, "", "KeyB", false)
		if err == nil || len(o) != 0 || !blank(out) {
			t.Errorf("%q: err %v, o %v", id, err, o)
		}
	}
	if _, err := Remove(nil, "guide", "F1"); err == nil {
		t.Error("Remove у служебной")
	}
	if _, _, err := Reset(nil, "guide", false); err == nil {
		t.Error("Reset у служебной")
	}
}

func TestAssignReservedRefused(t *testing.T) {
	in := Overrides{"toggleMarkup": {"KeyM"}}
	for _, k := range []string{"Escape", "Shift+Tab", "Space", "F1", "ArrowLeft", "Shift+ArrowUp", "Alt+F4", "Mod+KeyW", "Mod+Alt+KeyE"} {
		o, _, err := Assign(in, "toggleBig", "", k, false)
		if err == nil || err.Error() != ReservedReason(k) || !reflect.DeepEqual(o, in) {
			t.Errorf("%s: err %v, o %v", k, err, o)
		}
	}
	for _, k := range []string{"", "Ctrl+KeyZ", "Numpad1", "Mod+"} {
		if _, _, err := Assign(in, "toggleBig", "", k, false); err == nil {
			t.Errorf("%q: принято", k)
		}
	}
}

func TestAssignTooMany(t *testing.T) {
	o, _ := mustAssign(t, nil, "redo", "", "Mod+KeyU", false)
	if len(keys(o, "redo")) != MaxKeys {
		t.Fatalf("redo: %v", keys(o, "redo"))
	}
	o2, _, err := Assign(o, "redo", "", "Mod+KeyJ", false)
	if err == nil || !reflect.DeepEqual(o2, o) {
		t.Errorf("четвёртое: err %v, o %v", err, o2)
	}
	// Замена при трёх — можно.
	o, _ = mustAssign(t, o, "redo", "Mod+KeyU", "Mod+KeyJ", false)
	if !slices.Equal(keys(o, "redo"), []string{"Mod+KeyY", "Mod+Shift+KeyZ", "Mod+KeyJ"}) {
		t.Errorf("redo: %v", keys(o, "redo"))
	}
}

// Вернулся к стандартному набору (в любом порядке) — своего нет: «Стандартное» не видна,
// новые стандартные клавиши новой версии до команды дойдут.
func TestAssignBackToDefaultDropsOverride(t *testing.T) {
	o, _ := mustAssign(t, nil, "toggleBig", "KeyF", "KeyB", false)
	o, _ = mustAssign(t, o, "toggleBig", "KeyB", "KeyF", false)
	if len(o) != 0 || Build(o).Changed != 0 {
		t.Errorf("o %v", o)
	}
	o, err := Remove(nil, "redo", "Mod+KeyY")
	if err != nil {
		t.Fatal(err)
	}
	o, _ = mustAssign(t, o, "redo", "", "Mod+KeyY", false)
	if len(o) != 0 {
		t.Errorf("redo в другом порядке: %v", o)
	}
	// После «Заменить» у держателя, вернувшего своё, запись тоже уходит.
	o, _ = mustAssign(t, nil, "toggleBig", "KeyF", "KeyG", true)
	o, _ = mustAssign(t, o, "toggleBig", "KeyG", "KeyF", false)
	o, _ = mustAssign(t, o, "toggleSafe", "", "KeyG", false)
	if len(o) != 0 {
		t.Errorf("после обмена назад: %v", o)
	}
}

func TestRemoveLeavesEmpty(t *testing.T) {
	o, err := Remove(nil, "toggleBig", "KeyF")
	if err != nil {
		t.Fatal(err)
	}
	if l, ok := o["toggleBig"]; !ok || l == nil || len(l) != 0 || len(keys(o, "toggleBig")) != 0 {
		t.Errorf("o %v", o)
	}
	if v := Build(o); v.Changed != 1 || !v.Commands[byID["toggleBig"]].Custom {
		t.Errorf("Changed %d", v.Changed)
	}
	// Нет такой клавиши — без изменений; не сочетание — ошибка.
	o2, err := Remove(o, "toggleSafe", "KeyQ")
	if err != nil || !reflect.DeepEqual(o2, o) {
		t.Errorf("холостое: %v, %v", o2, err)
	}
	if _, err := Remove(o, "toggleSafe", "bad"); err == nil {
		t.Error("не сочетание принято")
	}
}

func TestResetOneConflictAsks(t *testing.T) {
	o, _ := mustAssign(t, nil, "toggleBig", "KeyF", "KeyB", false)
	o, _ = mustAssign(t, o, "toggleDetails", "", "KeyF", false)
	before := Build(o)
	o2, out, err := Reset(o, "toggleBig", false)
	if err != nil || out.Conflict == nil || *out.Conflict != (Ref{"toggleDetails", "Панель деталей", "KeyF"}) {
		t.Fatalf("err %v, out %+v", err, out)
	}
	if !reflect.DeepEqual(Build(o2), before) {
		t.Errorf("при вопросе изменилось: %v", o2)
	}
	o2, out, err = Reset(o, "toggleBig", true)
	if err != nil || out.Replaced == nil || out.Replaced.ID != "toggleDetails" || out.Conflict != nil {
		t.Fatalf("err %v, out %+v", err, out)
	}
	if len(o2) != 0 || !slices.Equal(keys(o2, "toggleBig"), []string{"KeyF"}) {
		t.Errorf("после замены: %v", o2)
	}
	// Без своего — холостой сброс; стандартное без занятых — сразу.
	o3, out, err := Reset(o, "toggleSafe", false)
	if err != nil || !reflect.DeepEqual(o3, o) || out.Conflict != nil {
		t.Errorf("холостой: %v %+v %v", o3, out, err)
	}
	o4, _ := mustAssign(t, nil, "toggleSafe", "KeyG", "KeyH", false)
	o4, out, err = Reset(o4, "toggleSafe", false)
	if err != nil || len(o4) != 0 || out.Conflict != nil {
		t.Errorf("простой: %v %+v %v", o4, out, err)
	}
}

func TestResetAll(t *testing.T) {
	o, _ := mustAssign(t, nil, "toggleBig", "KeyF", "KeyB", false)
	o, _ = mustAssign(t, o, "toggleDetails", "", "KeyF", false)
	o["futureCmd"] = []string{"KeyQ"}
	r := ResetAll(o)
	if !reflect.DeepEqual(r, Overrides{"futureCmd": {"KeyQ"}}) {
		t.Errorf("ResetAll = %v", r)
	}
	if v := Build(r); v.Changed != 0 || !slices.Equal(keys(r, "toggleBig"), []string{"KeyF"}) {
		t.Errorf("Changed %d", v.Changed)
	}
	if len(ResetAll(nil)) != 0 || ResetAll(nil) == nil {
		t.Error("ResetAll(nil)")
	}
	if len(o) != 3 {
		t.Errorf("вход изменён: %v", o)
	}
}

// «Вернуть»: своё каталога — из снимка (вместе с пустым «без клавиши»), чужое — из файла;
// мусор снимка (служебная команда, совпавшее со стандартным) не записывается.
func TestRestoreKeepsForeign(t *testing.T) {
	prev, _ := mustAssign(t, nil, "toggleBig", "KeyF", "KeyB", false)
	prev, _ = mustAssign(t, prev, "toggleDetails", "", "KeyF", false)
	prev["toggleSafe"] = []string{}
	prev["futureCmd"] = []string{"KeyQ"}
	prev["oldOnly"] = []string{"KeyW"}
	cur := Overrides{"futureCmd": {"KeyZ"}, "toggleMarkup": {"KeyM"}}
	got := Restore(cur, prev)
	want := Overrides{
		"toggleBig": {"KeyB"}, "toggleDetails": {"KeyI", "KeyF"}, "toggleSafe": {},
		"futureCmd": {"KeyZ"}, "oldOnly": {"KeyW"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Restore = %v", got)
	}
	// Пустой снимок — всё стандартное, чужое из файла цело.
	if got := Restore(cur, nil); !reflect.DeepEqual(got, Overrides{"futureCmd": {"KeyZ"}}) {
		t.Errorf("пустой снимок: %v", got)
	}
	if got := Restore(nil, Overrides{"guide": {"KeyU"}, "toggleBig": {"KeyF"}}); len(got) != 0 || got == nil {
		t.Errorf("служебная и стандартное: %v", got)
	}
	if len(cur) != 2 || len(prev) != 5 {
		t.Errorf("вход изменён: %v / %v", cur, prev)
	}
}
