package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Совместимость формата с прошлым выпуском (075-12). Программа прошлого выпуска (0.17, формат 12)
// файл формата 13 открыть не обязана, но обязана разобрать до проверки версии и сказать
// «сохранён более новой версией», а не «повреждён»: для этого каждый ключ, который она знает,
// в новом файле обязан остаться того же вида (объект, список, строка, число, логическое).
// Снимок формы 0.17 — testdata/compat/v12-shape.json (рецепт — shape_test.go). После повышения
// формата добавить снимок прошлого выпуска и перевести тест на него.
const prevShape = "testdata/compat/v12-shape.json"

func readShape(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil || len(m) == 0 {
		t.Fatalf("снимок формы %s: %v, ключей %d", path, err, len(m))
	}
	return m
}

// sameShape — все ключи снимка prev есть в cur с тем же видом; нарушения — списком.
func sameShape(prev, cur map[string]string) []string {
	var bad []string
	for path, kind := range prev {
		switch got, ok := cur[path]; {
		case !ok:
			bad = append(bad, fmt.Sprintf("%s: ключ пропал (был %s)", path, kind))
		case got != kind:
			bad = append(bad, fmt.Sprintf("%s: был %s, стал %s", path, kind, got))
		}
	}
	slices.Sort(bad)
	return bad
}

// fillAll — значение типа t, где заполнено каждое поле (omitempty и omitzero тоже пишутся):
// строка, 1, true, один элемент списка и словаря. Рекурсивный тип обрывается пустым значением.
func fillAll(t reflect.Type, open map[reflect.Type]bool) reflect.Value {
	v := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.Pointer:
		v.Set(fillAll(t.Elem(), open).Addr())
	case reflect.Struct:
		if open[t] {
			return v
		}
		open[t] = true
		defer delete(open, t)
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() && f.Tag.Get("json") != "-" {
				v.Field(i).Set(fillAll(f.Type, open))
			}
		}
	case reflect.Slice:
		v.Set(reflect.Append(v, fillAll(t.Elem(), open)))
	case reflect.Map:
		v.Set(reflect.MakeMap(t))
		v.SetMapIndex(reflect.ValueOf("k").Convert(t.Key()), fillAll(t.Elem(), open))
	case reflect.String:
		v.SetString("x")
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(1)
	case reflect.Float32, reflect.Float64:
		v.SetFloat(1.5)
	}
	return v
}

// Файл формата 13 держит форму ключей, известных 0.17: по типам (каждое поле заполнено),
// по настоящему файлу Encode (с табличками мест grid[].person) и по самому помощнику shapeOf
// (его вид совпадает с видом готового JSON). Новые ключи формата 13 — в файле есть, в снимке нет.
func TestV13ReadableBy017(t *testing.T) {
	prev := readShape(t, prevShape)
	cur := shapeOf(reflect.TypeFor[FileProject]())
	if bad := sameShape(prev, cur); len(bad) > 0 {
		t.Errorf("по типам FileProject вид ключей 0.17 сменился:\n%s", strings.Join(bad, "\n"))
	}

	// Помощник честен: полностью заполненный FileProject пишется в JSON ровно той формы, что shapeOf.
	full, err := json.Marshal(fillAll(reflect.TypeFor[FileProject](), map[reflect.Type]bool{}).Interface())
	if err != nil {
		t.Fatal(err)
	}
	filled, err := jsonShape(full, cur)
	if err != nil {
		t.Fatal(err)
	}
	for path, kind := range cur {
		if got, ok := filled[path]; !ok || got != kind {
			t.Errorf("shapeOf и JSON расходятся: %s — %s, в JSON %q", path, kind, got)
		}
	}
	if bad := sameShape(prev, filled); len(bad) > 0 {
		t.Errorf("в полном файле вид ключей 0.17 сменился:\n%s", strings.Join(bad, "\n"))
	}

	// Настоящий проект с табличками мест: каждый ключ, что 0.17 знает, того же вида.
	dir := testkit.TempDir(t)
	data, err := Encode(seatProject(dir), dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonShape(data, cur)
	if err != nil {
		t.Fatal(err)
	}
	for path, kind := range got {
		if want, ok := prev[path]; ok && want != kind {
			t.Errorf("файл Encode: %s — %s, 0.17 ждёт %s", path, kind, want)
		}
	}
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil || head.Version != model.FormatVersion {
		t.Errorf("версия в файле %d (%v), ждали %d", head.Version, err, model.FormatVersion)
	}

	// Тест не пуст: новое в формате 13 — таблички мест — в файле есть, а 0.17 их не знает.
	const seat = "$.spreads[].grid[].person"
	if _, known := prev[seat]; known || got[seat] != "object" || filled[seat] != "object" {
		t.Errorf("%s: в снимке 0.17 %q, в файле %q — снимок не от 0.17 или табличек нет", seat, prev[seat], got[seat])
	}
}

// Снимок ловит смену вида и пропажу ключа: подмена в копии снимка даёт нарушения.
func TestSameShapeCatches(t *testing.T) {
	prev := map[string]string{"$.a": "string", "$.b": "object", "$.b.c": "number"}
	if bad := sameShape(prev, map[string]string{"$.a": "string", "$.b": "object", "$.b.c": "number", "$.new": "bool"}); len(bad) != 0 {
		t.Errorf("новый ключ — не нарушение: %v", bad)
	}
	bad := sameShape(prev, map[string]string{"$.a": "number", "$.b": "object"})
	if len(bad) != 2 || !strings.Contains(bad[0], "$.a") || !strings.Contains(bad[1], "$.b.c") {
		t.Errorf("нарушения: %v", bad)
	}
	if bad := sameShape(prev, nil); len(bad) != 3 {
		t.Errorf("пустая форма: %v", bad)
	}
}

// Версия проверяется до полного разбора: файл новее программы с несовместимым полем — ErrNewer,
// а не «повреждён»; своей версии тот же мусор — по-прежнему «повреждён».
func TestNewerVersionBeforeDecode(t *testing.T) {
	dir := testkit.TempDir(t)
	junk := func(v int) string {
		return fmt.Sprintf(`{"version":%d,"settings":7,"spreads":"не список","photos":[1],"folders":{"a":1}}`, v)
	}
	for _, v := range []int{model.FormatVersion + 1, model.FormatVersion + 100, 1 << 30} {
		if _, err := Decode([]byte(junk(v)), dir); !errors.Is(err, ErrNewer) {
			t.Errorf("версия %d: %v, ждали ErrNewer", v, err)
		}
	}
	for _, v := range []int{model.FormatVersion, 1, 0} {
		_, err := Decode([]byte(junk(v)), dir)
		if err == nil || errors.Is(err, ErrNewer) || !strings.Contains(err.Error(), "повреждён") {
			t.Errorf("версия %d: %v, ждали «повреждён»", v, err)
		}
	}
	// Через Load с диска — то же.
	path := filepath.Join(dir, "newer.salbum")
	if err := os.WriteFile(path, []byte(junk(model.FormatVersion+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrNewer) {
		t.Errorf("Load: %v", err)
	}
	// Версия не число — «повреждён»; свой формат без остального открывается.
	if _, err := Decode([]byte(`{"version":"13"}`), dir); err == nil || errors.Is(err, ErrNewer) {
		t.Errorf("версия строкой: %v", err)
	}
	if _, err := Decode([]byte(fmt.Sprintf(`{"version":%d}`, model.FormatVersion)), dir); err != nil {
		t.Errorf("пустой альбом своей версии: %v", err)
	}
}
