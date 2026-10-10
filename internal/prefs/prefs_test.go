package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestPrefsNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"": LevelSimple, "simple": LevelSimple, "standard": LevelStandard, "full": LevelFull,
		"Full": LevelSimple, "expert": LevelSimple, " standard": LevelSimple,
	} {
		if got := NormLevel(in); got != want {
			t.Errorf("NormLevel(%q) = %q, ждали %q", in, got, want)
		}
	}
	// Пустой уровень в конфиге — первый запуск; стартовое окно по умолчанию показывается.
	if p := Of(Config{}); p != (Prefs{Level: LevelSimple, ShowStart: true, First: true}) {
		t.Errorf("первый запуск: %+v", p)
	}
	if p := Of(Config{Level: "мусор", HideStart: true}); p != (Prefs{Level: LevelSimple, ShowStart: false}) {
		t.Errorf("чужой уровень: %+v", p)
	}
	// «Свой» — уровень, но не основа: основа custom или чужая — simple.
	if NormLevel(LevelCustom) != LevelCustom || NormLevel("Custom") != LevelSimple {
		t.Error("NormLevel не принял custom")
	}
	for in, want := range map[string]string{
		"simple": LevelSimple, "standard": LevelStandard, "full": LevelFull, "custom": LevelSimple, "": LevelSimple, "x": LevelSimple,
	} {
		if got := normBase(in); got != want {
			t.Errorf("normBase(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// Набор «Своего» без уровня custom — только набор; custom без набора — пустой «Свой» от simple;
// ни того ни другого — null.
func TestPrefsCustomDefault(t *testing.T) {
	if p := Of(Config{Level: LevelFull}); p.Custom != nil {
		t.Fatalf("набора нет, а Custom %+v", p.Custom)
	}
	p := Of(Config{Level: LevelCustom})
	if p.Level != LevelCustom || p.First || p.Custom == nil || p.Custom.Base != LevelSimple ||
		p.Custom.On == nil || p.Custom.Off == nil || len(p.Custom.On)+len(p.Custom.Off) != 0 {
		t.Fatalf("custom без набора: %+v %+v", p, p.Custom)
	}
	data, _ := json.Marshal(p)
	if !strings.Contains(string(data), `"custom":{"base":"simple","on":[],"off":[]}`) {
		t.Fatalf("JSON: %s", data)
	}
	if data, _ := json.Marshal(Of(Config{})); !strings.Contains(string(data), `"custom":null`) {
		t.Fatalf("JSON без набора: %s", data)
	}
	// Набор есть, уровень другой — набор отдаётся (UI покажет его при выборе «Своего»).
	c := Config{Level: LevelStandard, Custom: &CustomLevel{Base: LevelFull, On: []string{"a"}}}
	if p := Of(c); p.Custom == nil || p.Custom.Base != LevelFull || !slices.Equal(p.Custom.On, []string{"a"}) {
		t.Fatalf("набор при другом уровне: %+v", p.Custom)
	}
}

func TestPrefsCustomSanitize(t *testing.T) {
	long := strings.Repeat("a", maxCustomIDLen)
	got := SanitizeCustom(CustomLevel{
		Base: "custom",
		On:   []string{"manual", "", "rot ate", "rotate", "manual", "flip", "кадр", "a1", long, long + "b", "<b>", "x"},
		Off:  []string{"flip", "flip", "zoom-in", "zoom"},
	})
	if got.Base != LevelSimple {
		t.Errorf("основа %q", got.Base)
	}
	// В on и off сразу — только в off; повторы, пустые, не латиница, длинные — прочь.
	if want := []string{"manual", "rotate", long, "x"}; !slices.Equal(got.On, want) {
		t.Errorf("on = %v, ждали %v", got.On, want)
	}
	if want := []string{"flip", "zoom"}; !slices.Equal(got.Off, want) {
		t.Errorf("off = %v, ждали %v", got.Off, want)
	}
	if got := SanitizeCustom(CustomLevel{Base: LevelFull}); got.Base != LevelFull || got.On == nil || got.Off == nil {
		t.Errorf("пустой набор: %+v", got)
	}
	// Не больше maxCustomIDs в каждом списке.
	many := make([]string, 0, 200)
	for i := range 200 {
		many = append(many, string(rune('a'+i%26))+strings.Repeat("z", i/26))
	}
	if got := SanitizeCustom(CustomLevel{On: many, Off: many[100:]}); len(got.On) != maxCustomIDs || len(got.Off) != maxCustomIDs {
		t.Errorf("предел: on %d, off %d", len(got.On), len(got.Off))
	}
	// Из файла: чужие типы не роняют чтение, остальные поля читаются.
	dir := testkit.TempDir(t)
	src := `{"level":"custom","hideStart":true,"custom":{"base":"full","on":["ok",5,null,"x y"],"off":"flip"}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p := Of(LoadConfig(dir))
	if p.Level != LevelCustom || p.ShowStart || p.Custom == nil || p.Custom.Base != LevelFull ||
		!slices.Equal(p.Custom.On, []string{"ok"}) || len(p.Custom.Off) != 0 {
		t.Fatalf("из файла: %+v %+v", p, p.Custom)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"level":"custom","custom":"мусор"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if p := Of(LoadConfig(dir)); p.Custom == nil || p.Custom.Base != LevelSimple {
		t.Fatalf("набор-строка: %+v", p.Custom)
	}
}

func TestRecentList(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(n string) string { return filepath.Join(dir, n+".salbum") }
	var list []string
	for _, n := range []string{"a", "b", "c"} {
		list = PushRecent(list, at(n))
	}
	if want := []string{at("c"), at("b"), at("a")}; !slices.Equal(list, want) {
		t.Fatalf("порядок: %v", list)
	}
	// Повтор (в том числе другим регистром) поднимается наверх без дубля.
	list = PushRecent(list, strings.ToUpper(at("a")))
	if len(list) != 3 || list[0] != strings.ToUpper(at("a")) {
		t.Fatalf("повтор: %v", list)
	}
	// Нелокальные пути не пишутся.
	for _, bad := range []string{`\\srv\share\a.salbum`, "a.salbum", ""} {
		if got := PushRecent(list, bad); !slices.Equal(got, list) {
			t.Errorf("%q попал в список: %v", bad, got)
		}
	}
	// Не больше maxRecent, самые старые отпадают.
	for i := range 12 {
		list = PushRecent(list, at(string(rune('k'+i))))
	}
	if len(list) != maxRecent || list[0] != at("v") {
		t.Fatalf("предел: %d %v", len(list), list)
	}
	// Нелокальная запись из файла при следующем добавлении отсеивается.
	if got := PushRecent([]string{`\\srv\x.salbum`, at("a")}, at("b")); !slices.Equal(got, []string{at("b"), at("a")}) {
		t.Fatalf("чужая запись: %v", got)
	}
	if got := DropRecent(list, strings.ToUpper(list[1])); len(got) != maxRecent-1 {
		t.Fatalf("DropRecent: %v", got)
	}
	// Строки для UI: имя без расширения, наличие файла.
	saved := at("real")
	if err := salbum.Save(model.New(), saved); err != nil {
		t.Fatal(err)
	}
	rows := RecentView([]string{saved, at("нет")})
	if rows[0].Name != "real" || !rows[0].Exists || rows[0].Modified == 0 || rows[1].Exists || rows[1].Modified != 0 {
		t.Fatalf("RecentView: %+v", rows)
	}
	// Руками правленный config.json: нелокальные строки пропускаются, лишние отрезаются.
	long := []string{`\\srv\share\x.salbum`, "rel.salbum", ""}
	for i := range maxRecent + 3 {
		long = append(long, at(string(rune('k'+i))))
	}
	rows = RecentView(long)
	if len(rows) != maxRecent || rows[0].Path != at("k") || rows[maxRecent-1].Path != at(string(rune('k'+maxRecent-1))) {
		t.Fatalf("RecentView из чужого конфига: %+v", rows)
	}
}

func TestConfigKeepsUnknownKeys(t *testing.T) {
	dir := testkit.TempDir(t)
	src := `{"lastProject":"x","future":{"a":[1,2]},"flag":true,"level":"full","custom":{"base":"full","on":["zoom"],"off":[]}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	UpdateConfig(dir, func(c *Config) { c.LastProject = ""; c.Level = "" })
	data, _ := os.ReadFile(ConfigPath(dir))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	var future struct{ A []int }
	if err := json.Unmarshal(raw["future"], &future); err != nil || !slices.Equal(future.A, []int{1, 2}) || string(raw["flag"]) != "true" {
		t.Fatalf("чужие ключи потеряны: %s", data)
	}
	// Свой omitempty-ключ, ставший пустым, исчезает, а не залипает от прежнего файла.
	if _, ok := raw["level"]; ok {
		t.Fatalf("level залип: %s", data)
	}
	if string(raw["lastProject"]) != `""` {
		t.Fatalf("lastProject: %s", data)
	}
	// Набор «Своего» переживает чужую запись.
	if c := LoadConfig(dir); c.Custom == nil || c.Custom.Base != LevelFull || !slices.Equal(c.Custom.On, []string{"zoom"}) {
		t.Fatalf("custom потерян: %s", data)
	}
	// Битый файл — как пустой.
	os.WriteFile(ConfigPath(dir), []byte("{не json"), 0o644)
	UpdateConfig(dir, func(c *Config) { c.Level = LevelFull })
	if LoadConfig(dir).Level != LevelFull {
		t.Fatal("после битого файла конфиг не записан")
	}
}
