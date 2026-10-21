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

// Совместимость формата с прошлыми выпусками (075-12, 079-26). Программа прошлого выпуска
// (0.17 — формат 12, 0.18 — 13) файл нового формата открыть не обязана, но обязана разобрать до
// проверки версии и сказать «сохранён более новой версией», а не «повреждён»: для этого каждый
// ключ, который она знает, в новом файле обязан остаться того же вида (объект, список, строка,
// число, логическое). Снимки формы — testdata/compat/v<N>-shape.json (рецепт — shape_test.go):
// после повышения формата добавить снимок прошлого выпуска и тест рядом (readableBy).
const (
	v12Shape = "testdata/compat/v12-shape.json" // 0.17
	v13Shape = "testdata/compat/v13-shape.json" // 0.18 (тег v0.18.0)
	v14Shape = "testdata/compat/v14-shape.json" // 0.20 (тег v0.20.0)
)

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
// строка, 1, true, один элемент списка и словаря, все элементы массива. Рекурсивный тип
// обрывается пустым значением.
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
	case reflect.Array:
		// Каждый элемент заполнен: массив с omitzero (срез рамочки, 079) иначе выпал бы из JSON.
		for i := range t.Len() {
			v.Index(i).Set(fillAll(t.Elem(), open))
		}
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
	dir := testkit.TempDir(t)
	readableBy(t, "0.17", v12Shape, seatProject(dir), dir, map[string]string{"$.spreads[].grid[].person": "object"})
}

// Файл формата 14 держит форму ключей, известных 0.18 (079-26): 0.18 разбирает весь файл до
// проверки версии, и ключ знакомого вида, сменивший вид, сделал бы черновик v14 «повреждённым».
// Новые ключи 14 — рамочки мест и прозрачность фото — в файле есть, в снимке 0.18 нет.
func TestV14ReadableBy018(t *testing.T) {
	dir := testkit.TempDir(t)
	readableBy(t, "0.18", v13Shape, borderProject(dir), dir, map[string]string{
		"$.spreads[].grid[].border": "object", "$.spreads[].grid[].border.cut": "array", "$.photos{}.fade": "number"})
}

// Файл формата 15 держит форму ключей, известных 0.20 (083, design §2.5): 0.20 видит номер 15
// и говорит «сохранён более новой версией», а не «повреждён». Новые ключи 15 — подборки, членство
// отметок и свои шрифты (082) — в файле есть, в снимке 0.20 нет; id своей гарнитуры в стиле —
// та же строка style.font.
func TestV15ReadableBy020(t *testing.T) {
	dir := testkit.TempDir(t)
	p := binsProject(dir)
	const lato = "my:0e82892fea1dcf13"
	p.Spreads[0].Texts = []*model.TextBlock{{ID: "0a0b0c0d0e01", W: 50, Text: "Анна", Style: model.TextStyle{Font: lato, Size: 20}}}
	p.Fonts = map[string]model.FontRef{lato: {Name: "Lato"}}
	readableBy(t, "0.20", v14Shape, p, dir, map[string]string{
		"$.bins": "array", "$.bins[]": "object", "$.bins[].id": "string", "$.bins[].name": "string", "$.marks[].bins": "array",
		"$.fonts": "object", "$.fonts{}": "object", "$.fonts{}.name": "string"})
}

// readableBy — общее тело: снимок формы выпуска rel (shapePath) против типов FileProject, полного
// JSON помощника и файла Encode проекта p; fresh — новые ключи формата (путь → вид), которых в
// снимке нет, а в файле и полном JSON — есть (тест не пуст).
func readableBy(t *testing.T, rel, shapePath string, p *model.Project, dir string, fresh map[string]string) {
	t.Helper()
	prev := readShape(t, shapePath)
	cur := shapeOf(reflect.TypeFor[FileProject]())
	if bad := sameShape(prev, cur); len(bad) > 0 {
		t.Errorf("по типам FileProject вид ключей %s сменился:\n%s", rel, strings.Join(bad, "\n"))
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
		t.Errorf("в полном файле вид ключей %s сменился:\n%s", rel, strings.Join(bad, "\n"))
	}

	// Настоящий проект с новыми полями: каждый ключ, что прошлый выпуск знает, того же вида.
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsonShape(data, cur)
	if err != nil {
		t.Fatal(err)
	}
	for path, kind := range got {
		if want, ok := prev[path]; ok && want != kind {
			t.Errorf("файл Encode: %s — %s, %s ждёт %s", path, kind, rel, want)
		}
	}
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil || head.Version != model.FormatVersion {
		t.Errorf("версия в файле %d (%v), ждали %d", head.Version, err, model.FormatVersion)
	}

	// Тест не пуст: новое в формате в файле есть, а прошлый выпуск его не знает.
	for key, kind := range fresh {
		if _, known := prev[key]; known || got[key] != kind || filled[key] != kind {
			t.Errorf("%s: в снимке %s %q, в файле %q — снимок не от %s или поля нет", key, rel, prev[key], got[key], rel)
		}
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
