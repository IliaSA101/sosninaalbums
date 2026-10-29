package printprof

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// maxDepth — вложенность JSON профиля: формат неглубок (профиль → изделие → таблица), больше —
// чужой или враждебный файл.
const maxDepth = 8

// checkKeys сверяет ключи объектов с полями типа t с точностью до регистра и ищет повторы.
// encoding/json принимает "ID" вместо "id" и берёт последнее из двух одинаковых ключей — для
// файла из чужих рук оба случая тихая подмена (формат 1 — только точные имена полей).
func checkKeys(data []byte, t reflect.Type) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	return walkKeys(dec, t, 0)
}

func walkKeys(dec *json.Decoder, t reflect.Type, depth int) error {
	if depth > maxDepth {
		return userError("слишком глубокая вложенность")
	}
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	if d == '[' {
		var elem reflect.Type
		if t != nil && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
			elem = t.Elem()
		}
		for dec.More() {
			if err := walkKeys(dec, elem, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	fields := fieldTypes(t)
	seen := map[string]bool{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := kt.(string)
		if seen[key] {
			return userError(fmt.Sprintf("ключ %s повторён", q(key)))
		}
		seen[key] = true
		var elem reflect.Type
		switch {
		case fields != nil:
			ft, ok := fields[key]
			if !ok {
				return userError(fmt.Sprintf("неизвестное поле %s", q(key)))
			}
			elem = ft
		case t != nil && t.Kind() == reflect.Map:
			elem = t.Elem()
		}
		if err := walkKeys(dec, elem, depth+1); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

// fieldTypes — json-имя → тип поля структуры t; не структура — nil.
func fieldTypes(t reflect.Type) map[string]reflect.Type {
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	out := make(map[string]reflect.Type, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			name = f.Name
		}
		out[name] = f.Type
	}
	return out
}
