package agents

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
)

// Тексты отказа схемы входа — по-русски и по полю. Строка валидатора SDK («validating root: …
// maximum: 9999/1 is greater than 1024») английская и называет не поле, а путь схемы; агенту
// нужны имя поля и предел. Разбор своими руками: первое нарушение по порядку, имена по алфавиту.
const (
	textArgsNotObject = "аргументы должны быть объектом JSON"
	textArgsUnknown   = "аргументы не подходят под схему инструмента — сверьтесь с inputSchema из tools/list"
	textArgsNotJSON   = "аргументы не разобраны как JSON"
)

// explainArgs возвращает причину, по которой v не проходит схему s, русским текстом. Пусто —
// нарушение не нашлось (схема богаче того, что разбирает функция): вызывающий берёт общую фразу.
func explainArgs(s *jsonschema.Schema, v any) string {
	obj, ok := v.(map[string]any)
	if !ok {
		return textArgsNotObject
	}
	names := make([]string, 0, len(obj))
	for k := range obj {
		names = append(names, k)
	}
	slices.Sort(names)
	for _, k := range names {
		if _, known := s.Properties[k]; !known {
			return "неизвестное поле «" + k + "»; " + knownFields(s)
		}
	}
	for _, k := range s.Required {
		if _, have := obj[k]; !have {
			return "нет обязательного поля «" + k + "»"
		}
	}
	for _, k := range names {
		if why := explainValue(s.Properties[k], obj[k]); why != "" {
			return "поле «" + k + "»: " + why
		}
	}
	return ""
}

// knownFields — какие поля есть у инструмента.
func knownFields(s *jsonschema.Schema) string {
	if len(s.Properties) == 0 {
		return "у этого инструмента нет аргументов"
	}
	names := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		names = append(names, "«"+k+"»")
	}
	slices.Sort(names)
	return "допустимые: " + strings.Join(names, ", ")
}

// explainValue — нарушение одного значения: тип, перечисление, предел числа и длины строки.
func explainValue(s *jsonschema.Schema, v any) string {
	if s == nil {
		return ""
	}
	want := s.Types
	if s.Type != "" {
		want = []string{s.Type}
	}
	if len(want) > 0 && !slices.ContainsFunc(want, func(t string) bool { return isType(t, v) }) {
		names := make([]string, len(want))
		for i, t := range want {
			names[i] = typeRu(t)
		}
		return "нужно " + strings.Join(names, " или ") + ", пришло: " + kindRu(v)
	}
	if len(s.Enum) > 0 && !slices.ContainsFunc(s.Enum, func(e any) bool { return e == v }) {
		vals := make([]string, len(s.Enum))
		for i, e := range s.Enum {
			vals[i] = "«" + fmtAny(e) + "»"
		}
		return "допустимые значения: " + strings.Join(vals, ", ")
	}
	switch x := v.(type) {
	case []any:
		return explainEnumItems(s, x)
	case float64:
		return explainNumber(s, x)
	case string:
		n := utf8.RuneCountInString(x)
		if s.MaxLength != nil && n > *s.MaxLength {
			return "длина " + strconv.Itoa(n) + " знаков больше предела " + strconv.Itoa(*s.MaxLength)
		}
		if s.MinLength != nil && n < *s.MinLength {
			return "длина " + strconv.Itoa(n) + " знаков меньше предела " + strconv.Itoa(*s.MinLength)
		}
	}
	return ""
}

// explainEnumItems — список из перечисления (families): неизвестный элемент называется по номеру, а
// допустимые значения перечисляются; иначе агент видел бы общую фразу «не подходят под схему».
// Прочие нарушения элементов (тип, предел) остаются общей фразе: разбор здесь только про перечисление.
func explainEnumItems(s *jsonschema.Schema, items []any) string {
	if s.Items == nil || len(s.Items.Enum) == 0 {
		return ""
	}
	for i, it := range items {
		if why := explainValue(&jsonschema.Schema{Enum: s.Items.Enum}, it); why != "" {
			return "элемент " + strconv.Itoa(i+1) + ": " + why
		}
	}
	return ""
}

// explainNumber — число вне предела; предел называется вместе с допустимым диапазоном.
func explainNumber(s *jsonschema.Schema, x float64) string {
	rng := ""
	switch {
	case s.Minimum != nil && s.Maximum != nil:
		rng = " (допустимо от " + num(*s.Minimum) + " до " + num(*s.Maximum) + ")"
	case s.Minimum != nil:
		rng = " (допустимо не меньше " + num(*s.Minimum) + ")"
	case s.Maximum != nil:
		rng = " (допустимо не больше " + num(*s.Maximum) + ")"
	}
	if s.Maximum != nil && x > *s.Maximum {
		return "значение " + num(x) + " больше предела " + num(*s.Maximum) + rng
	}
	if s.Minimum != nil && x < *s.Minimum {
		return "значение " + num(x) + " меньше предела " + num(*s.Minimum) + rng
	}
	return ""
}

func num(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func fmtAny(e any) string {
	switch x := e.(type) {
	case string:
		return x
	case float64:
		return num(x)
	}
	return "?"
}

// isType — подходит ли значение JSON под тип схемы (целое — число без дробной части).
func isType(t string, v any) bool {
	switch t {
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "number":
		_, ok := v.(float64)
		return ok
	case "integer":
		f, ok := v.(float64)
		return ok && f == math.Trunc(f)
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		_, ok := v.([]any)
		return ok
	case "null":
		return v == nil
	}
	return true
}

func typeRu(t string) string {
	switch t {
	case "string":
		return "строка"
	case "boolean":
		return "логическое значение (true или false)"
	case "number":
		return "число"
	case "integer":
		return "целое число"
	case "object":
		return "объект"
	case "array":
		return "список"
	case "null":
		return "null"
	}
	return t
}

// kindRu — что прислал агент (без самого значения: оно может быть длинным).
func kindRu(v any) string {
	switch x := v.(type) {
	case string:
		return "строка"
	case bool:
		return "логическое значение"
	case float64:
		if x != math.Trunc(x) {
			return "дробное число"
		}
		return "число"
	case map[string]any:
		return "объект"
	case []any:
		return "список"
	}
	return "null"
}
