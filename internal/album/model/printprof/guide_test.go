package printprof

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseGuide (123 Б5): направляющие формата и безопасные линии обложки разбираются строго,
// числа приводятся к шагу 0,01, круг Encode→Parse равен, пустые и чужие значения отвергаются.
func TestParseGuide(t *testing.T) {
	p, err := Parse(mutate(t, "fabrika", func(m map[string]any) {
		g := m["formats"].([]any)[3].(map[string]any)["guide"].(map[string]any)
		g["fold"] = []any{20.504, 18.496}
	}))
	if err != nil {
		t.Fatal(err)
	}
	g := p.Formats[3].Guide
	if g == nil || *g.Fold != (Pair{20.5, 18.5}) || *g.Bleed != (Pair{3, 3}) {
		t.Fatalf("30×20: %+v", g)
	}
	if c := p.Cover; c.FoldSafe != 3 || c.GuideSpineSafe != 12 {
		t.Errorf("cover: %+v", c)
	}
	b, err := Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if again, err := Parse(b); err != nil || !reflect.DeepEqual(p, again) {
		t.Fatalf("круг Encode→Parse: %v", err)
	}
	// Формат без guide остаётся без него (nil, а не пустая структура).
	if q, err := Parse(mutate(t, "fabrika", func(m map[string]any) { delete(m["formats"].([]any)[0].(map[string]any), "guide") })); err != nil || q.Formats[0].Guide != nil {
		t.Errorf("без guide: %v %+v", err, q.Formats[0].Guide)
	}
}

// TestParseGuideRejects — края guide и безопасных линий: отказ с причиной, а не молчаливое принятие.
func TestParseGuideRejects(t *testing.T) {
	guide := func(m map[string]any, i int) map[string]any {
		return m["formats"].([]any)[i].(map[string]any)["guide"].(map[string]any)
	}
	set := func(i int, k string, v any) func(map[string]any) {
		return func(m map[string]any) { guide(m, i)[k] = v }
	}
	cov := func(k string, v any) func(map[string]any) {
		return func(m map[string]any) { m["cover"].(map[string]any)[k] = v }
	}
	for name, c := range map[string]struct {
		why string
		f   func(map[string]any)
	}{
		"guide пуст":              {"guide пуст", func(m map[string]any) { m["formats"].([]any)[0].(map[string]any)["guide"] = map[string]any{} }},
		"guide не объект":         {"а нужно объект", func(m map[string]any) { m["formats"].([]any)[0].(map[string]any)["guide"] = []any{1} }},
		"лишнее поле guide":       {"неизвестное поле", set(0, "wrap", 3)},
		"fold одно число":         {"нужны два числа", set(0, "fold", []any{20})},
		"fold три числа":          {"нужны два числа", set(0, "fold", []any{20, 19, 18})},
		"fold строка":             {"нужны два числа", set(0, "fold", "20")},
		"fold нулевой":            {"guide.fold от 0,01", set(0, "fold", []any{0, 19})},
		"fold меньше шага":        {"guide.fold от 0,01", set(0, "fold", []any{20, 0.004})},
		"fold отрицательный":      {"guide.fold от 0,01", set(0, "fold", []any{-1, 19})},
		"fold 51":                 {"guide.fold от 0,01", set(0, "fold", []any{51, 19})},
		"bleed 11":                {"guide.bleed от 0", set(0, "bleed", []any{11, 3})},
		"bleed отрицательный":     {"guide.bleed от 0", set(0, "bleed", []any{3, -0.5})},
		"bleed null и fold null":  {"guide пуст", func(m map[string]any) { g := guide(m, 0); g["bleed"], g["fold"] = nil, nil }},
		"повтор ключа fold":       {"повторён", nil},
		"foldSafe 31":             {"cover: заворот", cov("foldSafe", 31)},
		"foldSafe отрицательный":  {"cover: заворот", cov("foldSafe", -1)},
		"guideSpineSafe 31":       {"cover: заворот", cov("guideSpineSafe", 31)},
		"guideSpineSafe строка":   {"а нужно число", cov("guideSpineSafe", "12")},
		"fold у формата без side": {"только у формата с side", func(m map[string]any) { delete(m["formats"].([]any)[0].(map[string]any), "side") }},
	} {
		data := []byte(nil)
		if c.f == nil { // повтор ключа можно записать только текстом
			data = []byte(strings.Replace(string(builtinData(t, "fabrika")), `"fold": [20, 19]`, `"fold": [20, 19], "fold": [1, 1]`, 1))
		} else {
			data = mutate(t, "fabrika", c.f)
		}
		if _, err := Parse(data); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: %v, ждали отказ %q", name, err, c.why)
		}
	}
	// Пределы изнутри допустимы: 0 у обреза и 50 у загиба.
	ok := mutate(t, "fabrika", func(m map[string]any) {
		g := guide(m, 0)
		g["bleed"], g["fold"] = []any{0, 10}, []any{50, 0.01}
	})
	if _, err := Parse(ok); err != nil {
		t.Errorf("граница пределов: %v", err)
	}
	// Только bleed (без fold) у формата без side — допустимо.
	bleedOnly := mutate(t, "fabrika", func(m map[string]any) {
		f := m["formats"].([]any)[0].(map[string]any)
		delete(f, "side")
		f["guide"] = map[string]any{"bleed": []any{3, 3.5}}
	})
	if _, err := Parse(bleedOnly); err != nil {
		t.Errorf("bleed без side: %v", err)
	}
}
