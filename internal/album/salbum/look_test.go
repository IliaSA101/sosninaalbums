package salbum

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Макет альбома и заготовки в файле (v19, спека 131 design §3, §6).

const lookKey = "my:0123456789abcdef"

// Файл формата 19 держит форму ключей, известных 0.27 (и 0.24–0.26: форма v18 одна): 0.27 видит
// номер 19 и говорит «сохранён более новой версией», а не «повреждён».
func TestV19ReadableBy027(t *testing.T) {
	dir := testkit.TempDir(t)
	p := binsProject(dir)
	p.Look = &model.LookRef{Key: lookKey, Name: "Классика", Pack: "wedding-layouts", Item: "layouts/classic.json"}
	s := p.Spreads[0]
	s.LookStep = 2
	s.Texts = append(s.Texts, &model.TextBlock{ID: "d1a2f3000001", W: 50, Text: "[Имена]", Style: model.PlainStyle(), Draft: true})
	readableBy(t, "0.27", v18Shape, p, dir, map[string]string{
		"$.look": "object", "$.look.key": "string", "$.look.name": "string", "$.look.pack": "string",
		"$.look.item": "string", "$.spreads[].lookStep": "number", "$.spreads[].texts[].draft": "bool"})
}

// lookFile — файл v19: макет ref и листы — с местом (шаг steps[0]), с цветом страницы (steps[1]),
// голый (steps[2]), с местом (steps[3]), с цветом (steps[4]); на первом — заготовка draft и
// заготовка из одних пробелов.
func lookFile(t *testing.T, ref any, steps [5]any, draft any) []byte {
	t.Helper()
	pl := []map[string]any{{"id": "0a0000000001", "x": 10, "y": 10, "w": 100, "h": 80}}
	sp := []map[string]any{
		{"id": "5a0000000001", "photos": []string{}, "grid": pl, "lookStep": steps[0], "texts": []map[string]any{
			{"id": "7e0000000001", "x": 10, "y": 100, "w": 60, "text": "[Имена]", "style": model.PlainStyle(), "draft": draft},
			{"id": "7e0000000002", "x": 10, "y": 120, "w": 60, "text": "   ", "style": model.PlainStyle(), "draft": true}}},
		{"id": "5a0000000002", "photos": []string{}, "bgL": "#ffeeee", "lookStep": steps[1]},
		{"id": "5a0000000003", "photos": []string{}, "lookStep": steps[2]},
		{"id": "5a0000000004", "photos": []string{}, "grid": []map[string]any{{"id": "0a0000000002", "x": 10, "y": 10, "w": 100, "h": 80}}, "lookStep": steps[3]},
		{"id": "5a0000000005", "photos": []string{}, "bgR": "#eeeeff", "lookStep": steps[4]},
	}
	f := map[string]any{"version": model.FormatVersion, "product": "album", "folders": []string{}, "photos": map[string]any{},
		"settings": model.DefaultSettings(), "spreads": sp}
	if ref != nil {
		f["look"] = ref
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// stepsOf — номера шагов листов проекта по порядку.
func stepsOf(p *model.Project) []int {
	out := make([]int, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = s.LookStep
	}
	return out
}

// Load зажимает макет и номера шагов: битый ключ снимает макет и все шаги; набор и запись — парой,
// по грамматике; имя — одна строка ≤ 60; шаг вне 1…24 и у листа без мест и фона — 0; заготовка
// из пробелов уходит вместе с надписью. Чтение идемпотентно: Encode → Decode — тот же проект.
func TestLoadClampsLook(t *testing.T) {
	dir := testkit.TempDir(t)
	ok := [5]any{1, 24, 3, 25, -2}
	good := map[string]any{"key": lookKey, "name": "Свадьба\nлето " + strings.Repeat("я", 100), "pack": "wedding-layouts", "item": "layouts/classic.json"}
	p, err := Decode(lookFile(t, good, ok, true), dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := stepsOf(p); len(got) != 5 || got[0] != 1 || got[1] != 24 || got[2] != 0 || got[3] != 0 || got[4] != 0 {
		t.Fatalf("шаги %v, ждали [1 24 0 0 0]", got)
	}
	if l := p.Look; l == nil || l.Key != lookKey || l.Pack != "wedding-layouts" || l.Item != "layouts/classic.json" ||
		strings.Contains(l.Name, "\n") || len([]rune(l.Name)) != model.MaxNameRunes || !strings.HasPrefix(l.Name, "Свадьба лето") {
		t.Fatalf("макет %+v", p.Look)
	}
	if tx := p.Spreads[0].Texts; len(tx) != 1 || !tx[0].Draft || tx[0].Text != "[Имена]" {
		t.Fatalf("заготовки первого листа: %+v", tx)
	}
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Decode(data, dir)
	if err != nil || string(model.Marshal(p)) != string(model.Marshal(q)) {
		t.Fatalf("Encode → Decode: %v\n%s\n%s", err, model.Marshal(p), model.Marshal(q))
	}

	// Ключ не по грамматике (и любой не-ключ) — макета нет, шагов нет; заготовка остаётся.
	for _, key := range []any{"", "my:0123456789ABCDEF", "my:0123456789abcde", "lib:0123456789abcdef", strings.Repeat("a", 1<<16), nil} {
		ref := map[string]any{"key": key, "name": "x"}
		p, err := Decode(lookFile(t, ref, ok, true), dir)
		if err != nil || p.Look != nil || stepsOf(p)[0] != 0 || stepsOf(p)[1] != 0 || !p.Spreads[0].Texts[0].Draft {
			t.Errorf("ключ %.20v: %v, макет %+v, шаги %v", key, err, p.Look, stepsOf(p))
		}
	}
	// Без макета шаги снимаются.
	if p, err := Decode(lookFile(t, nil, ok, false), dir); err != nil || p.Look != nil || stepsOf(p)[0] != 0 || p.Spreads[0].Texts[0].Draft {
		t.Errorf("без макета: %v, шаги %v", err, stepsOf(p))
	}
	// Набор и запись — парой: любая негодная половина снимает обе, ключ и имя остаются.
	for _, c := range []struct{ pack, item string }{
		{"Wedding", "a.json"}, {"nul", "a.json"}, {"com1", "a.json"}, {"-ab", "a.json"}, {"ab", "a.json"}, {strings.Repeat("a", 41), "a.json"},
		{"wedding", ""}, {"wedding", "../x.json"}, {"wedding", `a\b.json`}, {"wedding", "c:x.json"}, {"wedding", "/abs.json"},
		{"wedding", "a\x00.json"}, {"wedding", "a\nb"}, {"wedding", strings.Repeat("a", 201)},
	} {
		ref := map[string]any{"key": lookKey, "name": "Имя", "pack": c.pack, "item": c.item}
		p, err := Decode(lookFile(t, ref, ok, true), dir)
		if err != nil || p.Look == nil || p.Look.Pack != "" || p.Look.Item != "" || p.Look.Name != "Имя" || stepsOf(p)[0] != 1 {
			t.Errorf("%q/%q: %v, макет %+v", c.pack, c.item, err, p.Look)
		}
	}
	// Пределы пути записи: 200 байт годится.
	ref := map[string]any{"key": lookKey, "pack": "com10", "item": strings.Repeat("я", 100)}
	if p, err := Decode(lookFile(t, ref, ok, true), dir); err != nil || p.Look == nil || p.Look.Item == "" || p.Look.Pack != "com10" {
		t.Errorf("200 байт: %v, %+v", err, p.Look)
	}
}

// Чужой вид полей макета и заготовок — «повреждён», не паника и не ErrNewer; огромный номер шага — 0.
func TestLoadLookBadShapes(t *testing.T) {
	dir := testkit.TempDir(t)
	ok := [5]any{1, 24, 3, 25, -2}
	good := map[string]any{"key": lookKey}
	for _, bad := range []string{`"look":5`, `"look":"my:0123456789abcdef"`, `"look":{"key":7}`, `"look":[]`} {
		raw := strings.Replace(string(lookFile(t, nil, ok, true)), `"photos":{}`, `"photos":{},`+bad, 1)
		if _, err := Decode([]byte(raw), dir); err == nil || !strings.Contains(err.Error(), "повреждён") {
			t.Errorf("%s: %v", bad, err)
		}
	}
	for _, steps := range [][5]any{{"1", 0, 0, 0, 0}, {1.5, 0, 0, 0, 0}, {map[string]int{}, 0, 0, 0, 0}} {
		if _, err := Decode(lookFile(t, good, steps, true), dir); err == nil || !strings.Contains(err.Error(), "повреждён") {
			t.Errorf("шаг %v: %v", steps[0], err)
		}
	}
	if _, err := Decode(lookFile(t, good, ok, "да"), dir); err == nil || !strings.Contains(err.Error(), "повреждён") {
		t.Errorf("draft строкой: %v", err)
	}
	// Огромный номер шага не переполняет ничего: 0.
	if p, err := Decode(lookFile(t, good, [5]any{1 << 62, 0, 0, 0, 0}, true), dir); err != nil || stepsOf(p)[0] != 0 {
		t.Errorf("шаг 1<<62: %v, %v", err, stepsOf(p))
	}
}

// Лист с одним чужим фото и местом null при годном макете удаляется так же, как без номера шага
// (аудит G2): место null уходит до FixLook и не держит шаг. Несколько листов: годный остаётся.
func TestLoadNilPlaceDropsStep(t *testing.T) {
	dir := testkit.TempDir(t)
	file := func(step string) []byte {
		return []byte(`{"version":19,"product":"album","look":{"key":"` + lookKey + `"},"spreads":[` +
			`{"id":"5a0000000001","photos":["нет"],"grid":[null]` + step + `},` +
			`{"id":"5a0000000002","photos":["нет"],"grid":[null,null]` + step + `},` +
			`{"id":"5a0000000003","photos":[],"bgL":"#ffeeee"` + step + `}]}`)
	}
	without, err := Decode(file(""), dir)
	if err != nil {
		t.Fatal(err)
	}
	with, err := Decode(file(`,"lookStep":3`), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(with.Spreads) != len(without.Spreads) || len(with.Spreads) != 1 || with.Spreads[0].ID != "5a0000000003" || with.Spreads[0].LookStep != 3 {
		t.Fatalf("листов с шагом %d (шаги %v), без шага %d", len(with.Spreads), stepsOf(with), len(without.Spreads))
	}
}
