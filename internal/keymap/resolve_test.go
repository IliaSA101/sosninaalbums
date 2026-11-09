package keymap

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestReservedReason(t *testing.T) {
	cases := map[string]string{
		"Escape":           "Esc — отмена и закрытие окон",
		"Mod+Shift+Escape": whyWin, // системное раньше общего «Esc»
		"Shift+Tab":        "Tab — переход между полями",
		"Mod+Space":        whyMac,
		"Shift+Space":      "Пробел — листание раскладок и перемещение разворота",
		"Alt+Space":        whyWin,
		"Mod+F1":           "F1 — справочник функций",
		"ArrowLeft":        whyArrows,
		"Shift+ArrowDown":  whyArrows,
		"Alt+F4":           whyWin,
		"Mod+KeyW":         whyMac,
		"Mod+KeyQ":         whyMac,
		"Mod+Backquote":    whyMac,
		"Shift+Mod+Tab":    whyWin, // не канонический порядок — тот же ответ
		"Mod+ArrowLeft":    "",     // с Ctrl стрелки свободны
		"Alt+ArrowLeft":    "",
		"Mod+KeyZ":         "",
		"KeyQ":             "",
		"F2":               "",
		"bad":              "", // не сочетание — не служебное (Canon откажет раньше)
	}
	for in, want := range cases {
		if got := ReservedReason(in); got != want {
			t.Errorf("ReservedReason(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// Ctrl+Alt+что угодно — AltGr на части раскладок Windows (074-23); системное Ctrl+Alt+Del —
// со своей причиной.
func TestReservedModAlt(t *testing.T) {
	for _, k := range []string{"Mod+Alt+KeyE", "Mod+Alt+Shift+Digit1", "Mod+Alt+F5", "Alt+Mod+BracketLeft"} {
		if got := ReservedReason(k); got != whyModAlt {
			t.Errorf("%s: %q", k, got)
		}
	}
	if got := ReservedReason("Mod+Alt+Delete"); got != whyWin {
		t.Errorf("Mod+Alt+Delete: %q", got)
	}
	if _, _, err := Assign(nil, "toggleBig", "", "Mod+Alt+KeyB", false); err == nil || err.Error() != whyModAlt {
		t.Errorf("назначение Ctrl+Alt: %v", err)
	}
	for _, k := range []string{"Alt+KeyB", "Mod+Shift+KeyB", "Alt+Shift+KeyB"} {
		if ReservedReason(k) != "" {
			t.Errorf("%s запрещено", k)
		}
	}
}

func TestResolveDefaults(t *testing.T) {
	for _, o := range []Overrides{nil, {}} {
		eff := Resolve(o)
		if len(eff) != len(catalog) {
			t.Fatalf("команд %d, ждали %d", len(eff), len(catalog))
		}
		for _, x := range catalog {
			if !slices.Equal(eff[x.ID], x.Defaults) || eff[x.ID] == nil {
				t.Errorf("%s: %v, ждали %v", x.ID, eff[x.ID], x.Defaults)
			}
		}
	}
}

// Своё главнее стандартного (074-5): клавиша, взятая своим, у стандартного держателя той же
// области не действует — так новая стандартная клавиша новой версии не отнимет своё.
func TestResolveOwnBeatsNewDefault(t *testing.T) {
	o := Overrides{"toggleDetails": {"KeyF"}, "save": {"Mod+KeyS", "Digit1"}}
	eff := Resolve(o)
	want := map[string][]string{
		"toggleDetails": {"KeyF"},
		"toggleBig":     {}, // стандартная F занята своим «Панели деталей»
		"save":          {"Mod+KeyS", "Digit1"},
		"markRating1":   {"Digit1"}, // другая область — остаётся (перекрытие)
		"toggleSafe":    {"KeyG"},
	}
	for id, w := range want {
		if !slices.Equal(eff[id], w) {
			t.Errorf("%s: %v, ждали %v", id, eff[id], w)
		}
	}
	// Одно своё у двух команд одной области (файл правили руками): берёт та, что выше по каталогу.
	eff = Resolve(Overrides{"toggleSafe": {"KeyB"}, "toggleBig": {"KeyB"}})
	if !slices.Equal(eff["toggleBig"], []string{"KeyB"}) || len(eff["toggleSafe"]) != 0 {
		t.Errorf("общее своё: big %v, safe %v", eff["toggleBig"], eff["toggleSafe"])
	}
}

func TestResolveDropsBadAndUnknown(t *testing.T) {
	o := Overrides{
		"toggleBig":  {"KeyB", "bad", "Escape"},
		"toggleSafe": {"Shift+Mod+KeyQ", "Shift+Mod+KeyQ"},
		"futureCmd":  {"KeyQ"},
		"guide":      {"KeyH"}, // служебная: своё не действует
		"nudgePlace": {},
	}
	eff := Resolve(o)
	if !slices.Equal(eff["toggleBig"], []string{"KeyB"}) {
		t.Errorf("toggleBig: %v", eff["toggleBig"])
	}
	if !slices.Equal(eff["toggleSafe"], []string{"Mod+Shift+KeyQ"}) {
		t.Errorf("toggleSafe: %v", eff["toggleSafe"])
	}
	if !slices.Equal(eff["guide"], []string{"F1"}) || len(eff["nudgePlace"]) != 8 {
		t.Errorf("служебные: guide %v, nudgePlace %v", eff["guide"], eff["nudgePlace"])
	}
	if _, ok := eff["futureCmd"]; ok {
		t.Error("незнакомый id в действующих")
	}
	v := Build(o)
	if !slices.Equal(v.Custom["futureCmd"], []string{"KeyQ"}) {
		t.Errorf("чужой id потерян: %v", v.Custom)
	}
	if v.Changed != 2 { // toggleBig, toggleSafe; guide и nudgePlace — служебные
		t.Errorf("Changed = %d", v.Changed)
	}
}

func TestSanitizeLimitsKeepsUnknown(t *testing.T) {
	o := Overrides{
		"toggleBig":                    {"KeyA", "KeyB", "KeyC", "KeyD", "KeyE"},
		"toggleSafe":                   {}, // своё «без клавиши»
		"bad-id":                       {"KeyQ"},
		strings.Repeat("a", MaxID+1):   {"KeyQ"},
		"toggleDetails":                {strings.Repeat("Shift+", 6) + "KeyQ", "KeyK"},
		"":                             {"KeyQ"},
		"zz" + strings.Repeat("9", 30): {"KeyQ"},
	}
	for i := range 300 {
		o[fmt.Sprintf("u%03d", i)] = []string{"KeyQ"}
	}
	in := fmt.Sprint(o)
	s := Sanitize(o)
	if fmt.Sprint(o) != in {
		t.Fatal("Sanitize изменил вход")
	}
	if len(s) != MaxIDs {
		t.Fatalf("id %d, ждали %d", len(s), MaxIDs)
	}
	if !slices.Equal(s["toggleBig"], []string{"KeyA", "KeyB", "KeyC"}) {
		t.Errorf("toggleBig: %v", s["toggleBig"])
	}
	if l, ok := s["toggleSafe"]; !ok || l == nil || len(l) != 0 {
		t.Errorf("toggleSafe: %v, %v", l, ok)
	}
	if !slices.Equal(s["toggleDetails"], []string{"KeyK"}) {
		t.Errorf("toggleDetails: %v", s["toggleDetails"])
	}
	for _, bad := range []string{"bad-id", strings.Repeat("a", MaxID+1), ""} {
		if _, ok := s[bad]; ok {
			t.Errorf("%q не отброшен", bad)
		}
	}
	// Предел режет чужие по алфавиту, свои известные целы.
	if _, ok := s["u000"]; !ok {
		t.Error("u000 отброшен")
	}
	if _, ok := s["zz"+strings.Repeat("9", 30)]; ok {
		t.Error("последний по алфавиту остался")
	}
}

// nil в своих (строка вместо списка, null в файле) — записи нет; [] — «без клавиши» (и в JSON []).
func TestSanitizeNilDropped(t *testing.T) {
	o := Overrides{"toggleSafe": nil, "toggleBig": {}, "futureCmd": nil, "toggleDetails": {"KeyY"}}
	s := Sanitize(o)
	for _, id := range []string{"toggleSafe", "futureCmd"} {
		if _, ok := s[id]; ok {
			t.Errorf("%s с nil не отброшен: %v", id, s)
		}
	}
	if v, ok := s["toggleBig"]; !ok || v == nil || len(v) != 0 {
		t.Errorf("пустой срез потерян: %v", s)
	}
	if out, _ := json.Marshal(s); strings.Contains(string(out), "null") {
		t.Errorf("JSON с null: %s", out)
	}
	// Старый id при nil у нового: новое не перебивает, старое применяется.
	defer restoreHistory(Aliases, Retired)
	Aliases = map[string]string{"bigView": "toggleBig"}
	got := Sanitize(Overrides{"bigView": {"KeyB"}, "toggleBig": nil})
	if !reflect.DeepEqual(got, Overrides{"toggleBig": {"KeyB"}}) {
		t.Errorf("алиас при nil: %v", got)
	}
}

// Переименованная команда не теряет своё (074-21), убранная — отбрасывается.
func TestMergeAliasApplied(t *testing.T) {
	defer restoreHistory(Aliases, Retired)
	Aliases = map[string]string{"bigView": "toggleBig", "safeArea": "toggleSafe"}
	Retired = map[string]bool{"goneCmd": true}
	o := Overrides{
		"bigView":    {"KeyB"},
		"safeArea":   {"KeyQ"},
		"toggleSafe": {"KeyW"}, // новое уже есть — старое не перебивает
		"goneCmd":    {"KeyE"},
		"futureCmd":  {"KeyR"},
	}
	want := Overrides{"toggleBig": {"KeyB"}, "toggleSafe": {"KeyW"}, "futureCmd": {"KeyR"}}
	if got := Sanitize(o); !reflect.DeepEqual(got, want) {
		t.Errorf("Sanitize = %v, ждали %v", got, want)
	}
	eff := Resolve(o)
	if !slices.Equal(eff["toggleBig"], []string{"KeyB"}) || !slices.Equal(eff["toggleSafe"], []string{"KeyW"}) {
		t.Errorf("Resolve: big %v, safe %v", eff["toggleBig"], eff["toggleSafe"])
	}
	// Правка пишет уже новый id.
	o2, _, err := Assign(o, "toggleDetails", "", "KeyY", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := o2["bigView"]; ok || !slices.Equal(o2["toggleBig"], []string{"KeyB"}) {
		t.Errorf("после правки: %v", o2)
	}
}

// Уровень интерфейса не участвует: клавиша скрытой на уровне команды («Линейки» — «Полный»)
// занята, вопрос «Заменить?» называет её.
func TestConflictIgnoresLevel(t *testing.T) {
	_, out, err := Assign(nil, "toggleSafe", "KeyG", "Mod+KeyR", false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Conflict == nil || out.Conflict.ID != "toggleRulers" || out.Conflict.Chord != "Mod+KeyR" {
		t.Errorf("Conflict = %+v", out.Conflict)
	}
	x, _ := lookup("toggleRulers")
	if x.Feature != "rulers" {
		t.Errorf("у «Линеек» функция %q — тест потерял смысл", x.Feature)
	}
}

// Круг через config.json: своё пишется JSON и читается обратно с тем же видом.
func TestOverridesRoundTrip(t *testing.T) {
	o, _, err := Assign(nil, "toggleBig", "KeyF", "KeyB", false)
	if err != nil {
		t.Fatal(err)
	}
	o, err = Remove(o, "toggleSafe", "KeyG")
	if err != nil {
		t.Fatal(err)
	}
	o["futureCmd"] = []string{"KeyQ"}
	data, err := json.Marshal(map[string]any{"hotkeys": o, "level": "full"})
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Hotkeys Overrides `json:"hotkeys"`
	}
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(Build(back.Hotkeys), Build(o)) {
		t.Errorf("круг JSON изменил вид: %s", data)
	}
	if !strings.Contains(string(data), `"toggleSafe":[]`) {
		t.Errorf("пустой список потерян: %s", data)
	}
	// Пустое своё — тот же вид, что стандартный.
	var none struct {
		Hotkeys Overrides `json:"hotkeys"`
	}
	if err := json.Unmarshal([]byte(`{}`), &none); err != nil || !reflect.DeepEqual(Build(none.Hotkeys), Build(Overrides{})) {
		t.Errorf("без ключа hotkeys: %v", err)
	}
}

func TestBuildView(t *testing.T) {
	v := Build(nil)
	if len(v.Commands) != len(catalog) || v.Changed != 0 || v.Custom == nil || len(v.Locked) != len(reserved) {
		t.Fatalf("вид: команд %d, changed %d, custom %v, locked %d", len(v.Commands), v.Changed, v.Custom, len(v.Locked))
	}
	for i, e := range v.Commands {
		if e.ID != catalog[i].ID || e.Keys == nil || e.Custom {
			t.Errorf("%d: %+v", i, e)
		}
	}
	v.Commands[0].Defaults[0] = "KeyQ"
	if catalog[0].Defaults[0] == "KeyQ" {
		t.Error("Build делит списки клавиш с каталогом")
	}
	data, _ := json.Marshal(Build(nil))
	if !strings.Contains(string(data), `"custom":{}`) || strings.Contains(string(data), "null") {
		t.Errorf("пустые поля в JSON: null вместо пустого")
	}
	r := Result(nil, Outcome{})
	if r.Shadows == nil || r.Conflict != nil {
		t.Errorf("Result: %+v", r)
	}
}
