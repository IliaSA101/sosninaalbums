package salbum

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Форма JSON файла проекта (075-12): путь → вид значения. Снимок формы прошлого выпуска лежит в
// testdata/compat и нужен TestV13ReadableBy017: новая версия не должна менять вид ключа,
// который старая программа знает (иначе она не откроет файл — и не поймёт, что он новее).
//
// Файл самодостаточен (только FileProject, reflect и json), чтобы его можно было скопировать
// в дерево прошлого выпуска. Как снять новый снимок после повышения формата:
//
//	git archive -o .gotmp/prev.tar vX.Y.Z go.mod go.sum internal
//	mkdir .gotmp/prev && tar -xf .gotmp/prev.tar -C .gotmp/prev
//	скопировать shape_test.go в .gotmp/prev/internal/album/salbum/
//	(cd .gotmp/prev && go test ./internal/album/salbum -run TestWriteShape -shapeout=<путь>/vNN-shape.json)
//	удалить .gotmp/prev и .gotmp/prev.tar
//
// Снятые: v12-shape.json (git-тег v0.17.0, FormatVersion 12), v13-shape.json (v0.18.0, 13; 0.19 —
// тоже 13), v14-shape.json (v0.20.0, 14), v15-shape.json (v0.21.0, 15), v16-shape.json (v0.22.0,
// 16). Следующий — v17-shape.json из выпуска 0.23.

var shapeOut = flag.String("shapeout", "", "записать форму JSON FileProject в файл (снимок прошлого выпуска)")

// shapeOf — вид каждого ключа JSON типа t: object, array, string, number, bool, any.
// Путь: «$.spreads[].grid[].x»; элементы словаря — «{}», списка — «[]».
func shapeOf(t reflect.Type) map[string]string {
	out := map[string]string{}
	walkShape(t, "$", out, map[reflect.Type]bool{})
	return out
}

func walkShape(t reflect.Type, path string, out map[string]string, open map[reflect.Type]bool) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct:
		out[path] = "object"
		if open[t] { // рекурсивный тип: вглубь второй раз не идём
			return
		}
		open[t] = true
		defer delete(open, t)
		for i := range t.NumField() {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "-" || (!f.Anonymous && !f.IsExported()) {
				continue
			}
			name, _, _ := strings.Cut(tag, ",")
			if f.Anonymous && name == "" { // встроенная структура — поля поднимаются
				walkShape(f.Type, path, out, open)
				continue
			}
			if name == "" {
				name = f.Name
			}
			walkShape(f.Type, path+"."+name, out, open)
		}
	case reflect.Map:
		out[path] = "object"
		walkShape(t.Elem(), path+"{}", out, open)
	case reflect.Slice, reflect.Array:
		out[path] = "array"
		walkShape(t.Elem(), path+"[]", out, open)
	case reflect.String:
		out[path] = "string"
	case reflect.Bool:
		out[path] = "bool"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		out[path] = "number"
	default:
		out[path] = "any"
	}
}

// jsonShape — вид каждого ключа готового JSON (тот же путь); элементы списка и словаря
// сливаются: вид пути один, пустые списки и null путей не добавляют. Словарь от объекта в JSON
// не отличить — его пути известны по схеме known (ключи вида «путь{}» от shapeOf).
func jsonShape(data []byte, known map[string]string) (map[string]string, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	out := map[string]string{}
	walkJSON(v, "$", out, known)
	return out, nil
}

func walkJSON(v any, path string, out, known map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		out[path] = "object"
		for k, e := range x {
			if _, dict := known[path+"{}"]; dict {
				walkJSON(e, path+"{}", out, known)
			} else {
				walkJSON(e, path+"."+k, out, known)
			}
		}
	case []any:
		out[path] = "array"
		for _, e := range x {
			walkJSON(e, path+"[]", out, known)
		}
	case string:
		out[path] = "string"
	case bool:
		out[path] = "bool"
	case float64:
		out[path] = "number"
	}
}

// TestWriteShape пишет снимок формы текущего FileProject в файл -shapeout; без флага пропускается.
func TestWriteShape(t *testing.T) {
	if *shapeOut == "" {
		t.Skip("нужен -shapeout=<файл>: снимок формы для следующих выпусков")
	}
	data, err := json.MarshalIndent(shapeOf(reflect.TypeFor[FileProject]()), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(*shapeOut, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
