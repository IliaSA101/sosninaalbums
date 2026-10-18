package pack

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const sumA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// manifestJSON — манифест с правкой mut поверх хорошего (два предмета).
func manifestJSON(t testing.TB, mut func(m map[string]any)) []byte {
	t.Helper()
	m := map[string]any{
		"format": 1, "id": "ivory", "version": 3, "minApp": "0.16.0",
		"title": "Свадьба — айвори", "about": "Светлые фоны", "author": "Sosnina Albums",
		"license": "CC0-1.0", "source": "https://example.org", "future": true,
		"items": []any{
			map[string]any{"file": "bg/ivory-01.jpg", "kind": "bg", "theme": "wedding", "name": "Айвори 1", "sha256": sumA, "size": 734512},
			map[string]any{"file": "spreads/hero.json", "kind": "spread", "theme": "wedding", "name": "Главное", "sha256": sumA, "size": 2210},
		},
	}
	if mut != nil {
		mut(m)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func item(m map[string]any, i int) map[string]any { return m["items"].([]any)[i].(map[string]any) }

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest(manifestJSON(t, nil), "0.16.0")
	if err != nil || m.ID != "ivory" || m.Version != 3 || len(m.Items) != 2 || len(m.Skipped) != 0 ||
		m.Items[0].Size != 734512 || m.Items[1].Kind != KindSpread || m.Source != "https://example.org" {
		t.Fatalf("хороший манифест: %v %+v", err, m)
	}
	// Отказ целиком: шапка и формат предмета.
	bad := map[string]func(m map[string]any){
		"format 0":       func(m map[string]any) { delete(m, "format") },
		"id короткий":    func(m map[string]any) { m["id"] = "iv" },
		"id с /":         func(m map[string]any) { m["id"] = "../ivory" },
		"id заглавные":   func(m map[string]any) { m["id"] = "Ivory" },
		"id с дефиса":    func(m map[string]any) { m["id"] = "-ivory" },
		"version 0":      func(m map[string]any) { m["version"] = 0 },
		"version 10^6":   func(m map[string]any) { m["version"] = 1000000 },
		"version дробь":  func(m map[string]any) { m["version"] = 1.5 },
		"minApp":         func(m map[string]any) { m["minApp"] = "0.16" },
		"minApp буквы":   func(m map[string]any) { m["minApp"] = "0.x.1" },
		"нет названия":   func(m map[string]any) { m["title"] = " " + string(rune(0x200b)) + "\t" },
		"нет предметов":  func(m map[string]any) { m["items"] = []any{} },
		"items null":     func(m map[string]any) { m["items"] = nil },
		"имя ..":         func(m map[string]any) { item(m, 0)["file"] = "bg/../x.jpg" },
		"имя \\":         func(m map[string]any) { item(m, 0)["file"] = "bg\\x.jpg" },
		"имя абсолютное": func(m map[string]any) { item(m, 0)["file"] = "/bg/x.jpg" },
		"имя заглавные":  func(m map[string]any) { item(m, 0)["file"] = "bg/X.jpg" },
		"имя глубоко":    func(m map[string]any) { item(m, 0)["file"] = "a/b/c/d.jpg" },
		"имя пустое":     func(m map[string]any) { item(m, 0)["file"] = "" },
		"повтор имени":   func(m map[string]any) { item(m, 1)["file"] = "bg/ivory-01.jpg"; item(m, 1)["kind"] = "bg" },
		"sha заглавные":  func(m map[string]any) { item(m, 0)["sha256"] = strings.ToUpper(sumA) },
		"sha короткий":   func(m map[string]any) { item(m, 0)["sha256"] = sumA[:63] },
		"size 0":         func(m map[string]any) { item(m, 0)["size"] = 0 },
		"size минус":     func(m map[string]any) { item(m, 0)["size"] = -1 },
		"не JSON":        nil,
	}
	for name, mut := range bad {
		data := []byte("{")
		if mut != nil {
			data = manifestJSON(t, mut)
		}
		if _, err := ParseManifest(data, "0.16.0"); err == nil {
			t.Errorf("%s: принят", name)
		}
	}
	if _, err := ParseManifest(manifestJSON(t, func(m map[string]any) { m["format"] = 2 }), "0.16.0"); !errors.Is(err, ErrNewer) {
		t.Errorf("format 2: %v", err)
	}
	big := make([]byte, MaxManifest+1)
	if _, err := ParseManifest(big, "0.16.0"); !errors.Is(err, ErrManifest) {
		t.Errorf("манифест больше 256 КБ: %v", err)
	}
}

// Пропуск предмета, а не отказ: вид из новой версии, SVG, предел вида; тексты чистятся.
func TestParseManifestSkips(t *testing.T) {
	m, err := ParseManifest(manifestJSON(t, func(m map[string]any) {
		m["items"] = append(m["items"].([]any),
			map[string]any{"file": "x/new.bin", "kind": "frame", "sha256": sumA, "size": 1},
			map[string]any{"file": "stickers/rose.svg", "kind": "sticker", "sha256": sumA, "size": 10},
			map[string]any{"file": "stickers/big.png", "kind": "sticker", "sha256": sumA, "size": MaxSticker + 1},
			map[string]any{"file": "bg/big.jpg", "kind": "bg", "sha256": sumA, "size": MaxBg + 1},
			map[string]any{"file": "spreads/a.txt", "kind": "spread", "sha256": sumA, "size": 5},
			map[string]any{"file": "spreads/b.json", "kind": "spread", "sha256": sumA, "size": MaxSpread + 1},
			map[string]any{"file": "bg/a.gif", "kind": "bg", "sha256": sumA, "size": 5},
			map[string]any{"file": "stickers/ok.png", "kind": "sticker", "theme": "Чужая", "name": "  Роза\n\x00 красная ", "sha256": sumA, "size": MaxSticker},
		)
		m["title"] = strings.Repeat("я", 100)
		m["about"] = "строка" + string(rune(0x202e)) + "обратно"
	}), "0.16.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 3 || len(m.Skipped) != 7 {
		t.Fatalf("предметы %d, пропуски %+v", len(m.Items), m.Skipped)
	}
	if !strings.Contains(m.Skipped[1].Why, "SVG") || !strings.Contains(m.Skipped[2].Why, "30 МБ") ||
		!strings.Contains(m.Skipped[3].Why, "60 МБ") || !strings.Contains(m.Skipped[5].Why, "256 КБ") {
		t.Errorf("причины: %+v", m.Skipped)
	}
	st := m.Items[2]
	if st.Theme != "basic" || st.Name != "Роза красная" {
		t.Errorf("тема и имя: %+v", st)
	}
	if len([]rune(m.Title)) != 60 || m.About != "строкаобратно" {
		t.Errorf("тексты: %q %q", m.Title, m.About)
	}
	// Все предметы — пропуск: отказ целиком.
	_, err = ParseManifest(manifestJSON(t, func(m map[string]any) {
		m["items"] = []any{map[string]any{"file": "a.svg", "kind": "bg", "sha256": sumA, "size": 1}}
	}), "0.16.0")
	if !errors.Is(err, ErrEmpty) {
		t.Errorf("все пропущены: %v", err)
	}
	// 1001 предмет — отказ; ровно 1000 — да.
	many := func(n int) func(m map[string]any) {
		return func(m map[string]any) {
			list := make([]any, n)
			for i := range list {
				list[i] = map[string]any{"file": "bg/" + strings.Repeat("a", 1+i%50) + "-" + itoa(i) + ".png", "kind": "bg", "sha256": sumA, "size": 1}
			}
			m["items"] = list
		}
	}
	if m, err := ParseManifest(manifestJSON(t, many(MaxItems)), "0.16.0"); err != nil || len(m.Items) != MaxItems {
		t.Errorf("1000 предметов: %v", err)
	}
	if _, err := ParseManifest(manifestJSON(t, many(MaxItems+1)), "0.16.0"); err == nil {
		t.Error("1001 предмет принят")
	}
	// Весь набор больше 1 ГБ.
	if _, err := ParseManifest(manifestJSON(t, func(m map[string]any) {
		list := make([]any, 20)
		for i := range list {
			list[i] = map[string]any{"file": "bg/b" + itoa(i) + ".jpg", "kind": "bg", "sha256": sumA, "size": MaxBg}
		}
		m["items"] = list
	}), "0.16.0"); err == nil {
		t.Error("набор 1,2 ГБ принят")
	}
}

func itoa(i int) string {
	return string(rune('a'+i/26%26)) + string(rune('a'+i%26)) + string(rune('0'+i/676))
}

func TestPackMinApp(t *testing.T) {
	cases := []struct {
		need, app string
		ok        bool
	}{
		{"0.16.0", "0.16.0", true},
		{"0.16.0", "0.16.1", true},
		{"0.16.0", "1.0.0", true},
		{"0.16.1", "0.16.0", false},
		{"0.17.0", "0.16.9", false},
		{"1.0.0", "0.99.99", false},
		{"0.10.0", "0.9.0", false}, // числа, а не строки
		{"99.0.0", "dev", true},    // сборка dev — без проверки
	}
	for _, c := range cases {
		_, err := ParseManifest(manifestJSON(t, func(m map[string]any) { m["minApp"] = c.need }), c.app)
		if (err == nil) != c.ok {
			t.Errorf("minApp %s, программа %s: %v", c.need, c.app, err)
		}
		if err != nil && !strings.Contains(err.Error(), c.need) {
			t.Errorf("текст без нужной версии: %v", err)
		}
	}
}

func FuzzParseManifest(f *testing.F) {
	f.Add(manifestJSON(f, nil), "0.16.0")
	f.Add([]byte(`{"format":1,"id":"abc","version":1,"minApp":"0.0.1","title":"x","items":[{"file":"a.png","kind":"bg","sha256":"`+sumA+`","size":1}]}`), "dev")
	f.Add([]byte(`{"format":1e9,"items":[null]}`), "")
	f.Fuzz(func(t *testing.T, data []byte, app string) {
		m, err := ParseManifest(data, app)
		if err != nil {
			return
		}
		if !ValidID(m.ID) || m.Title == "" || len(m.Items) == 0 || len(m.Items) > MaxItems {
			t.Fatalf("принят плохой манифест %+v", m)
		}
		var total int64
		for _, it := range m.Items {
			if !ValidName(it.File) || !validSum(it.SHA256) || it.Size < 1 || it.Size > KindLimit(it.Kind) || !validKind(it.Theme) {
				t.Fatalf("плохой предмет %+v", it)
			}
			total += it.Size
		}
		if total > MaxTotal {
			t.Fatalf("набор %d байт", total)
		}
	})
}
