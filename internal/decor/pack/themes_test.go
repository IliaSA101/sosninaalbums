package pack

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// themedManifest — манифест с объявлениями и topic у предметов (поверх хорошего из manifestJSON).
func themedManifest(t testing.TB, themes any, topics ...any) []byte {
	t.Helper()
	return manifestJSON(t, func(m map[string]any) {
		if themes != nil {
			m["themes"] = themes
		}
		for i, tp := range topics {
			item(m, i)["topic"] = tp
		}
	})
}

func decl(id, name, base string, order int) map[string]any {
	return map[string]any{"id": id, "name": name, "base": base, "order": order}
}

func TestManifestThemes(t *testing.T) {
	data := themedManifest(t, []any{decl("newyear", "Новый год", "holiday", 10), map[string]any{"id": "love", "name": "Любовь"}},
		"newyear", "holiday")
	m, err := ParseManifest(data, "0.40.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []Theme{{"newyear", "Новый год", "holiday", 10}, {"love", "Любовь", "basic", ThemeOrderDef}}
	if len(m.Themes) != 2 || m.Themes[0] != want[0] || m.Themes[1] != want[1] {
		t.Fatalf("темы %+v, ждали %+v", m.Themes, want)
	}
	if m.Items[0].Topic != "newyear" || m.Items[1].Topic != "holiday" {
		t.Errorf("topic: %q, %q", m.Items[0].Topic, m.Items[1].Topic)
	}
	if m.ThemeSkips.Note("ivory") != "" || m.ThemesRaw != nil || m.Items[0].TopicRaw != nil {
		t.Errorf("пропусков нет, сырые поля обнулены: %+v", m.ThemeSkips)
	}
	// theme предмета остаётся базой: её видят 0.29–0.39
	if m.Items[0].Theme != "wedding" {
		t.Errorf("theme изменена: %q", m.Items[0].Theme)
	}
}

// TestManifestThemesBad: плохие записи пропускаются, набор ставится; журнал — одна строка на набор,
// причины словарём, значения манифеста в них не попадают.
func TestManifestThemesBad(t *testing.T) {
	evil := "evil\nнабор \"x\": темы"
	data := themedManifest(t, []any{
		map[string]any{"id": evil, "name": "Лето"},
		map[string]any{"id": "ta", "name": evil},
		decl("tb", "Зима", "party\nbase", 5),
		decl("tc", "Весна", "kids", 500),
		decl("td", "Осень", "kids", 7),
	}, "tb")
	m, err := ParseManifest(data, "0.40.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Themes) != 1 || m.Themes[0].ID != "td" || m.Items[0].Topic != "" {
		t.Fatalf("темы %+v, topic %q", m.Themes, m.Items[0].Topic)
	}
	note := m.ThemeSkips.Note("iv\nory")
	if want := `набор "iv\nory": темы пропущены: 4 (id, имя, база, order)`; note != want {
		t.Errorf("строка журнала %q, ждали %q", note, want)
	}
	if strings.Contains(note, "evil") || strings.Contains(note, "\n") {
		t.Errorf("значение манифеста или перевод строки в журнале: %q", note)
	}
}

func TestManifestThemesLimit(t *testing.T) {
	var recs []any
	for i := range 17 {
		recs = append(recs, decl("t"+string(rune('a'+i)), "Тема "+itoa(i+1), "kids", i+1))
	}
	m, err := ParseManifest(themedManifest(t, recs, "tq", "tp"), "0.40.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Themes) != MaxThemes || m.Themes[15].ID != "tp" {
		t.Fatalf("принято %d тем", len(m.Themes))
	}
	if m.ThemeSkips.N != 1 || len(m.ThemeSkips.Why) != 1 || m.ThemeSkips.Why[0] != WhyLimit {
		t.Errorf("пропуски %+v", m.ThemeSkips)
	}
	if m.Items[0].Topic != "" || m.Items[1].Topic != "tp" {
		t.Errorf("topic 17-й темы должен уйти в базу: %q %q", m.Items[0].Topic, m.Items[1].Topic)
	}
}

// TestManifestForwardCompat: будущие поля любого типа в шапке, объявлении и предмете не роняют разбор
// (в 0.29–0.39 те же манифесты ставятся — 0.40 не строже); themes/topic чужого типа — набор цел.
func TestManifestForwardCompat(t *testing.T) {
	future := []any{1, "s", true, nil, []any{1, []any{}}, map[string]any{"a": map[string]any{"b": []any{nil}}}}
	for i, f := range future {
		data := manifestJSON(t, func(m map[string]any) {
			m["tv"], m["f1"] = f, f
			d := decl("newyear", "Новый год", "holiday", 10)
			d["future"], d["own"] = f, f
			m["themes"] = []any{d}
			it := item(m, 0)
			it["topic"], it["form"], it["style"] = "newyear", f, f
		})
		m, err := ParseManifest(data, "0.40.0")
		if err != nil || len(m.Themes) != 1 || m.Items[0].Topic != "newyear" || len(m.Items) != 2 {
			t.Errorf("будущее поле %d (%v): err %v, темы %v", i, f, err, m.Themes)
		}
	}
	for i, v := range future {
		data := manifestJSON(t, func(m map[string]any) {
			m["themes"] = v
			item(m, 0)["topic"] = v
		})
		m, err := ParseManifest(data, "0.40.0")
		if err != nil || len(m.Items) != 2 || len(m.Themes) != 0 || m.Items[0].Topic != "" {
			t.Errorf("themes/topic типа %d (%v): err %v", i, v, err)
		}
	}
}

func TestTopicBuiltin(t *testing.T) {
	var topics []any
	for _, b := range builtinThemes {
		topics = append(topics, b)
	}
	topics = append(topics, "recent", "constructor", "Holiday")
	data := manifestJSON(t, func(m map[string]any) {
		var its []any
		for i, tp := range topics {
			its = append(its, map[string]any{"file": "bg/" + itoa(i) + ".png", "kind": "bg", "theme": "wedding", "name": "n", "sha256": sumA, "size": 10, "topic": tp})
		}
		m["items"] = its
	})
	m, err := ParseManifest(data, "0.40.0")
	if err != nil {
		t.Fatal(err)
	}
	for i, it := range m.Items {
		want := ""
		if i < len(builtinThemes) {
			want = builtinThemes[i]
		}
		if it.Topic != want {
			t.Errorf("topic %v → %q, ждали %q", topics[i], it.Topic, want)
		}
	}
}

// TestThemeNameBytes: края имени, которые файл случаев выразить не может (битый UTF-8), и регистр.
func TestThemeNameBytes(t *testing.T) {
	for _, bad := range []string{"Ле\xffто", "\xef\xbf\xbd", "Лето\xd0", "\xd0", "Лето\x00"} {
		if ValidThemeName(bad) {
			t.Errorf("принято %q", bad)
		}
	}
	for _, c := range [][2]string{{"ЁЛКА", "ёлка"}, {"Summer Лето", "summer лето"}, {"İ", "İ"}, {"Σ", "Σ"}} {
		if got := themeFold(c[0]); got != c[1] {
			t.Errorf("themeFold(%q) = %q", c[0], got)
		}
	}
	if ValidThemeID("constructor") || ValidThemeID("holiday") || !ValidThemeID("newyear") {
		t.Error("id: constructor и встроенные отвергаются")
	}
}

// TestParseThemesHostile: пустые, усечённые, огромные и битые входы — результат, а не паника.
func TestParseThemesHostile(t *testing.T) {
	inputs := map[string]string{
		"nil": "", "null": "null", "пустой массив": "[]", "усечённое": `[{"id":"ta","name":"Ле`, "строка": `"x"`,
		"объект": `{}`, "число": `1e999`, "мусор": "\x00\xff\xfe", "вложенность": strings.Repeat("[", 5000),
		"нулевые записи": strings.Repeat("null,", 100)[:499], "огромное": strings.Repeat("{}", 5000),
	}
	for name, in := range inputs {
		themes, sk := ParseThemes([]byte(in))
		if len(themes) > MaxThemes || len(sk.Why) > maxReason {
			t.Errorf("%s: тем %d, причин %d", name, len(themes), len(sk.Why))
		}
	}
	if th, sk := ParseThemes(nil); th != nil || sk.Note("x") != "" {
		t.Error("пустой вход — не пропуск")
	}
	if _, sk := ParseThemes([]byte("null")); sk.Note("x") != "" {
		t.Error("null — как нет поля")
	}
	if _, sk := ParseThemes(make([]byte, MaxThemesRaw+1)); sk.Why[0] != WhySize || sk.N != 1 {
		t.Errorf("4 КиБ + 1: %+v", sk)
	}
	// битый UTF-8: байты считаются своими (как в JS rawField), а не после замены на U+FFFD
	if _, sk := ParseThemes([]byte(`["` + strings.Repeat("\xff", 4090) + `"]`)); sk.Why[0] != WhyType || sk.N != 1 {
		t.Errorf("4094 Б битого UTF-8 — в пределе, запись чужого типа: %+v", sk)
	}
	if _, sk := ParseThemes([]byte(`["` + strings.Repeat("\xff", 4093) + `"]`)); sk.Why[0] != WhySize {
		t.Errorf("4097 Б битого UTF-8: %+v", sk)
	}
}

func FuzzParseThemes(f *testing.F) {
	rec, _ := json.Marshal([]any{decl("newyear", "Новый год", "holiday", 10), decl("constructor", "Х", "basic", 1)})
	for _, s := range []string{string(rec), `[{}]`, `null`, `[{"id":"a","name":"Ё","order":99,"x":[1]}]`, `[1,"a",[],null]`, `[{"id":"ta","name":"АБ"}]`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		themes, sk := ParseThemes(raw)
		if len(themes) > MaxThemes || len(sk.Why) > maxReason {
			t.Fatalf("тем %d, причин %d", len(themes), len(sk.Why))
		}
		ids, names := map[string]bool{}, map[string]bool{}
		for _, th := range themes {
			if !ValidThemeID(th.ID) || !ValidThemeName(th.Name) || !utf8.ValidString(th.Name) || !IsBuiltinTheme(th.Base) ||
				th.Order < ThemeOrderMin || th.Order > ThemeOrderMax || ids[th.ID] || names[themeFold(th.Name)] {
				t.Fatalf("принята плохая тема %+v", th)
			}
			ids[th.ID], names[themeFold(th.Name)] = true, true
		}
		if len(raw) > MaxThemesRaw && len(themes) != 0 {
			t.Fatalf("поле %d Б принято", len(raw))
		}
	})
}
