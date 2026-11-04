package agents

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// TestBadArgsRussian: отказ схемы — русский текст с именем поля и пределом, без кусков SDK
// («validating», «maximum», «root»): тип, предел, неизвестное поле, перечисление, длина,
// обязательное поле — на инструментах с аргументами и без.
func TestBadArgsRussian(t *testing.T) {
	_, cs, _ := previewRig(t, 3, Options{})
	long := strings.Repeat("я", 70)
	for _, c := range []struct {
		name string
		args map[string]any
		want []string // все подстроки обязательны
	}{
		{"get_photo_preview", map[string]any{"id": idA, "size": 9999}, []string{"«size»", "9999", "1024", "512"}},
		{"get_photo_preview", map[string]any{"id": idA, "size": 100}, []string{"«size»", "меньше предела 512"}},
		{"get_photo_preview", map[string]any{"id": idA, "size": "800"}, []string{"«size»", "целое число", "строка"}},
		{"get_photo_preview", map[string]any{"id": idA, "size": 700.5}, []string{"«size»", "целое число", "дробное"}},
		{"get_photo_preview", map[string]any{"id": idA, "bogus": 1}, []string{"неизвестное поле «bogus»", "«id»", "«size»"}},
		{"get_photo_preview", map[string]any{}, []string{"нет обязательного поля «id»"}},
		{"get_photo_preview", map[string]any{"id": long}, []string{"«id»", "70", "предела 64"}},
		{"get_spread_preview", map[string]any{"spread": 1, "size": 1 << 40}, []string{"«size»", "1099511627776", "1600"}},
		{"get_spread_preview", map[string]any{"spread": true}, []string{"«spread»", "строка или целое число", "логическое"}},
		{"get_album", map[string]any{"x": 1}, []string{"неизвестное поле «x»", "нет аргументов"}},
		{"list_spreads", map[string]any{"limit": 101}, []string{"«limit»", "101", "100"}},
		{"list_spreads", map[string]any{"offset": -1}, []string{"«offset»", "-1", "меньше предела 0"}},
		{"list_photos", map[string]any{"filter": "все"}, []string{"«filter»", "«used»", "«marked»"}},
		{"list_decor", map[string]any{"kind": "foo"}, []string{"«kind»", "«sticker»"}},
		{"list_decor", map[string]any{}, []string{"нет обязательного поля «kind»"}},
		{"export_album", map[string]any{"album": map[string]any{"key": "k", "rev": 1}, "quality": 50}, []string{"«quality»", "«100»", "«95»", "«80»"}},
	} {
		r := tcall(t, cs, c.name, c.args)
		if !r.IsError || !strings.HasPrefix(r.Text, "неверные аргументы вызова: ") {
			t.Errorf("%s %v: %+v", c.name, c.args, r)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(r.Text, w) {
				t.Errorf("%s %v: в %q нет %q", c.name, c.args, r.Text, w)
			}
		}
		for _, bad := range []string{"validating", "maximum", "minimum", "root", "greater", "less than", "unexpected", "type:"} {
			if strings.Contains(r.Text, bad) {
				t.Errorf("%s %v: кусок SDK %q в %q", c.name, c.args, bad, r.Text)
			}
		}
	}
}

// TestExplainArgsEdges: края разбора — не объект, пустое, допустимое (пустая причина), nil-схема
// значения, предел; без паники.
func TestExplainArgsEdges(t *testing.T) {
	lim := 10.0
	s := &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{
		"n": {Type: "integer", Maximum: &lim},
	}}
	for _, v := range []any{nil, 5.0, "x", []any{}} {
		if got := explainArgs(s, v); got != textArgsNotObject {
			t.Errorf("%v: %q", v, got)
		}
	}
	if got := explainArgs(s, map[string]any{}); got != "" {
		t.Errorf("пустой объект без обязательных: %q", got)
	}
	if got := explainArgs(s, map[string]any{"n": 3.0}); got != "" {
		t.Errorf("допустимое: %q", got)
	}
	if got := explainArgs(s, map[string]any{"n": 11.0}); !strings.Contains(got, "«n»") || !strings.Contains(got, "10") {
		t.Errorf("выше предела: %q", got)
	}
	if explainValue(nil, 1.0) != "" {
		t.Error("nil-схема значения")
	}
	if got := explainArgs(&jsonschema.Schema{}, map[string]any{"a": 1.0}); !strings.Contains(got, "нет аргументов") {
		t.Errorf("схема без полей: %q", got)
	}
}

// TestExplainEnumNumbers: перечисление чисел в схеме (int: quality 100/95/80) называется числами,
// а не «?», допустимое число (float64 из JSON) нарушением не считается; строки не сломаны.
func TestExplainEnumNumbers(t *testing.T) {
	num := &jsonschema.Schema{Type: "integer", Enum: []any{100, 95, 80}}
	if got := explainValue(num, 50.0); got != "допустимые значения: «100», «95», «80»" {
		t.Errorf("числа: %q", got)
	}
	for _, ok := range []any{100.0, 95.0, 80.0} {
		if got := explainValue(num, ok); got != "" {
			t.Errorf("%v допустимо, а сказано %q", ok, got)
		}
	}
	str := &jsonschema.Schema{Type: "string", Enum: []any{"a", "b"}}
	if got := explainValue(str, "c"); got != "допустимые значения: «a», «b»" {
		t.Errorf("строки: %q", got)
	}
	if explainValue(str, "a") != "" {
		t.Error("допустимая строка")
	}
	if fmtAny(struct{}{}) != "?" || fmtAny(int64(7)) != "7" || fmtAny(json.Number("2.5")) != "2.5" {
		t.Error("fmtAny")
	}
}
