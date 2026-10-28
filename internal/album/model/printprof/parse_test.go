package printprof

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
	"unicode/utf8"
)

var update = flag.Bool("update", false, "перезаписать testdata/builtin-hash.json (только вместе с подъёмом version)")

func builtinData(t *testing.T, name string) []byte {
	t.Helper()
	b, err := builtinFS.ReadFile("builtin/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestBuiltinParse — все встроенные файлы разбираются строго, список порядка совпадает с папкой,
// снимок собирается.
func TestBuiltinParse(t *testing.T) {
	ents, err := builtinFS.ReadDir("builtin")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range ents {
		files = append(files, strings.TrimSuffix(e.Name(), ".json"))
	}
	if a, b := slices.Sorted(slices.Values(files)), slices.Sorted(slices.Values(builtinFiles)); !slices.Equal(a, b) {
		t.Fatalf("файлы builtin %v не совпадают со списком порядка %v", a, b)
	}
	ps := Builtin().Profiles()
	if len(ps) != len(builtinFiles) {
		t.Fatalf("профилей %d, ждали %d", len(ps), len(builtinFiles))
	}
	for i, p := range ps {
		if p.ID != builtinFiles[i] || p.Version < 1 {
			t.Errorf("профиль %d: id %q version %d, ждали %q", i, p.ID, p.Version, builtinFiles[i])
		}
		if p2, ok := Builtin().ByID(p.ID); !ok || p2.Name != p.Name {
			t.Errorf("ByID(%q)", p.ID)
		}
		if p2, ok := Builtin().ByName(p.Name); !ok || p2.ID != p.ID {
			t.Errorf("ByName(%q)", p.Name)
		}
	}
	if _, ok := Builtin().ByID(""); ok {
		t.Error("пустой id нашёл профиль")
	}
	if _, ok := Builtin().ByName(""); ok {
		t.Error("пустое название нашло профиль")
	}
	f := PageFormat{Bleed: 4}
	if f.BleedIn(ps[0]) != 4 || (PageFormat{}).BleedIn(ps[0]) != ps[0].Page.Bleed || (PageFormat{Safe: 7}).SafeIn(ps[0]) != 7 {
		t.Error("зона и обрез формата: своё или общее")
	}
	if Current() != Builtin() {
		t.Error("Current без слоёв — не встроенные")
	}
}

// TestRoundTrip — Parse(Encode(p)) == p на встроенных и на профиле, где заняты все поля.
func TestRoundTrip(t *testing.T) {
	full := fullProfile()
	cases := Builtin().Profiles()
	cases = append(cases, full)
	for _, p := range cases {
		b, err := Encode(p)
		if err != nil {
			t.Fatalf("%s: Encode: %v", p.ID, err)
		}
		got, err := Parse(b)
		if err != nil {
			t.Fatalf("%s: Parse: %v\n%s", p.ID, err, b)
		}
		want := p.Clone()
		want.normalize()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: круг Encode→Parse изменил профиль:\n%+v\n%+v", p.ID, got, want)
		}
		again, err := Encode(got)
		if err != nil || !bytes.Equal(again, b) {
			t.Errorf("%s: второй Encode не совпал с первым", p.ID)
		}
	}
	// Числа приводятся к шагу 0,01.
	if got, err := Parse(mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["safe"] = 10.004 })); err != nil || got.Page.Safe != 10 {
		t.Errorf("шаг 0,01: %v, %v", got.Page.Safe, err)
	}
}

// mutate — встроенный JSON name после правки f над деревом.
func mutate(t *testing.T, name string, f func(map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(builtinData(t, name), &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestParseRejects — битые, усечённые, пустые, огромные и чужие входы дают ошибку, а не панику и
// не молчаливое принятие.
func TestParseRejects(t *testing.T) {
	good := builtinData(t, "fabrika")
	type c struct {
		name string
		why  string // подстрока причины: случай отвергается именно ею, а не соседней проверкой
		data []byte
	}
	prod := func(m map[string]any) map[string]any { return m["products"].([]any)[0].(map[string]any) }
	frm := func(m map[string]any) map[string]any { return m["formats"].([]any)[0].(map[string]any) }
	cases := []c{
		{"пусто", "пустой файл", nil},
		{"пробелы", "файл оборван", []byte("  \n")},
		{"не JSON", "это не JSON", []byte("не json")},
		{"массив", "а нужно объект", []byte("[]")},
		{"null", "format: поддерживается", []byte("null")},
		{"усечён", "файл оборван", good[:len(good)/2]},
		{"усечён на 1 байт", "файл оборван", good[:len(good)-2]},
		{"мусор после", "лишние данные", append(slices.Clone(good), []byte(`{"a":1}`)...)},
		{"два профиля", "лишние данные", append(slices.Clone(good), good...)},
		// Битый байт внутри строки: encoding/json молча заменил бы его на U+FFFD, поэтому
		// единственная защита — проверка UTF-8 до разбора.
		{"не UTF-8", "не UTF-8", bytes.Replace(good, []byte(`"Фабрика Фотокниги"`), []byte("\"Фабрика\xffФотокниги\""), 1)},
		{"огромный", "больше 64 КБ", append(bytes.Repeat([]byte(" "), MaxSize), good...)},
		{"глубокая вложенность", "вложенность", []byte(strings.Repeat("[", 60))},
		{"огромная вложенность", "больше 64 КБ", []byte(strings.Repeat("[", MaxSize+1))},
		{"лишнее поле", "неизвестное поле", mutate(t, "fabrika", func(m map[string]any) { m["extra"] = 1 })},
		{"лишнее поле в формате", "неизвестное поле", mutate(t, "fabrika", func(m map[string]any) { frm(m)["guid"] = 1 })},
		{"лишнее поле в export", "неизвестное поле", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["cmd"] = "x" })},
		{"format 2", "format: поддерживается", mutate(t, "fabrika", func(m map[string]any) { m["format"] = 2 })},
		{"format 0", "format: поддерживается", mutate(t, "fabrika", func(m map[string]any) { delete(m, "format") })},
		{"id с путём", "id: нужны", mutate(t, "fabrika", func(m map[string]any) { m["id"] = "../etc" })},
		{"id с заглавной", "id: нужны", mutate(t, "fabrika", func(m map[string]any) { m["id"] = "Fabrika" })},
		{"id длинный", "id: нужны", mutate(t, "fabrika", func(m map[string]any) { m["id"] = "a" + strings.Repeat("b", 64) })},
		{"id 33 знака", "id: нужны", mutate(t, "fabrika", func(m map[string]any) { m["id"] = "a" + strings.Repeat("b", 32) })},
		{"id изделия 65 знаков", "id негодный или повторён", mutate(t, "fabrika", func(m map[string]any) {
			id := "fabrika-" + strings.Repeat("b", 57)
			prod(m)["id"], m["default"] = id, id
		})},
		{"id на 60 КБ", "id: нужны", mutate(t, "fabrika", func(m map[string]any) { m["id"] = "a" + strings.Repeat("b", 60000) })},
		{"поле на 60 КБ", "неизвестное поле", mutate(t, "fabrika", func(m map[string]any) { m[strings.Repeat("z", 60000)] = 1 })},
		{"формат на 60 КБ", "formats.name: длиннее 200", mutate(t, "fabrika", func(m map[string]any) { frm(m)["name"] = strings.Repeat("z", 60000) })},
		{"предел формата на 60 КБ", "limits: формата", mutate(t, "fabrika", func(m map[string]any) {
			prod(m)["limits"] = []any{map[string]any{"format": strings.Repeat("z", 60000)}}
		})},
		{"version 0", "version: от 1", mutate(t, "fabrika", func(m map[string]any) { m["version"] = 0 })},
		{"version 1000000", "version: от 1", mutate(t, "fabrika", func(m map[string]any) { m["version"] = 1_000_000 })},
		{"version дробная", "целое число", mutate(t, "fabrika", func(m map[string]any) { m["version"] = 1.5 })},
		{"имя пустое", "name: пусто", mutate(t, "fabrika", func(m map[string]any) { m["name"] = " " })},
		{"имя с управляющим", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "Фаб\u0007рика" })},
		{"имя длинное", "name: длиннее 200", mutate(t, "fabrika", func(m map[string]any) { m["name"] = strings.Repeat("я", 201) })},
		{"название 41 знак", "name: длиннее 40", mutate(t, "fabrika", func(m map[string]any) { m["name"] = strings.Repeat("я", 41) })},
		// Bidi и невидимые знаки категории Cf: RLO переворачивает соседний текст окна.
		{"имя с RLO", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "Фаб\u202eрика" })},
		{"имя с ZWSP", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "Фаб\u200bрика" })},
		{"имя с BOM", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "\ufeffФабрика" })},
		{"имя с разделителем строк", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "Фаб\u2028рика" })},
		{"имя с U+FFFD", "name: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["name"] = "Фаб\ufffdрика" })},
		{"item с RLO", "item: управляющий", mutate(t, "fabrika", func(m map[string]any) { prod(m)["item"] = "а\u202eб" })},
		{"export.title с isolate", "export.title: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["title"] = "а\u2066б" })},
		{"item 201 знак", "item: длиннее 200", mutate(t, "fabrika", func(m map[string]any) { prod(m)["item"] = strings.Repeat("я", 201) })},
		{"export.done 201 знак", "export.done: длиннее 200", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["done"] = strings.Repeat("я", 201) })},
		{"hint 301 знак", "page.hint: длиннее 300", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["hint"] = strings.Repeat("я", 301) })},
		{"url 301 знак", "url: длиннее 300", mutate(t, "fabrika", func(m map[string]any) { prod(m)["url"] = "https://fabrika-fotoknigi.com/" + strings.Repeat("a", 271) })},
		{"dpi 0", "page.dpi", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["dpi"] = 0 })},
		{"dpi 5000", "page.dpi", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["dpi"] = 5000 })},
		{"cover заворот 100", "cover: заворот", mutate(t, "fsfera", func(m map[string]any) { m["cover"].(map[string]any)["wrap"] = 100 })},
		{"формат: заворот 99", "зона, обрез и заворот", mutate(t, "fabrika", func(m map[string]any) { frm(m)["wrap"] = 99 })},
		{"формат: сторонка 5 мм", "сторонка от 50", mutate(t, "fabrika", func(m map[string]any) { frm(m)["side"] = []any{5, 5} })},
		{"формат: px 0", "px от 100", mutate(t, "fabrika", func(m map[string]any) { frm(m)["px"] = []any{0, 100} })},
		{"формат: px 99999", "px от 100", mutate(t, "fabrika", func(m map[string]any) { frm(m)["px"] = []any{99999, 100} })},
		{"изделие: расстав 99", "заворот, расстав, A, B", mutate(t, "netprint", func(m map[string]any) { prod(m)["hinge"] = 99 })},
		{"hint управляющий", "page.hint: управляющий", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["hint"] = "а\u0000б" })},
		{"pages управляющий", "pages: управляющий", mutate(t, "fabrika", func(m map[string]any) { prod(m)["pages"] = "а\u0007" })},
		{"spineBy не по возрастанию", "строка 2 вне пределов", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spineBy"] = map[string]any{"30×20": []any{9, 8, 2, 8}} })},
		{"предел 65 строк одного формата", "повторена", mutate(t, "fabrika", func(m map[string]any) {
			var ls []any
			for i := range 65 {
				ls = append(ls, map[string]any{"min": i % 5})
			}
			prod(m)["limits"] = ls
		})},
		{"px одно число", "нужны два целых, а не 1", mutate(t, "fabrika", func(m map[string]any) { frm(m)["px"] = []any{7181} })},
		{"зона отрицательная", "page: зона от 0 до 30", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["safe"] = -1 })},
		{"подстановка чужая", "page.hint: неизвестная подстановка", mutate(t, "fabrika", func(m map[string]any) { m["page"].(map[string]any)["hint"] = "{path}" })},
		{"страница 1 число", "нужны два числа, а не 1", mutate(t, "fabrika", func(m map[string]any) { frm(m)["page"] = []any{200} })},
		{"страница 3 числа", "нужны два числа, а не 3", mutate(t, "fabrika", func(m map[string]any) { frm(m)["page"] = []any{200, 203, 1} })},
		{"страница строкой", "строка, а нужно число", mutate(t, "fabrika", func(m map[string]any) { frm(m)["page"] = []any{"200", "203"} })},
		{"страница 10 мм", "страница от 50 до 1000", mutate(t, "fabrika", func(m map[string]any) { frm(m)["page"] = []any{10, 10} })},
		{"страница 1e9", "страница от 50 до 1000", mutate(t, "fabrika", func(m map[string]any) { frm(m)["page"] = []any{1e9, 203} })},
		{"px дробный", "целое число", mutate(t, "fabrika", func(m map[string]any) { frm(m)["px"] = []any{7180.5, 2397} })},
		{"подпись повторена", "повторена", mutate(t, "fabrika", func(m map[string]any) { m["formats"].([]any)[1].(map[string]any)["name"] = "20×20" })},
		{"форматов 33", "formats: больше 32", mutate(t, "fabrika", func(m map[string]any) {
			// Остальное верно (изделия ссылаются на свои форматы): отказывает только счётчик.
			fs := m["formats"].([]any)
			for i := len(fs); i < 33; i++ {
				fs = append(fs, map[string]any{"name": strings.Repeat("x", i+1), "page": []any{200, 200}})
			}
			m["formats"] = fs
		})},
		{"default чужой", "default: такого изделия нет", mutate(t, "fabrika", func(m map[string]any) { m["default"] = "nope-nope" })},
		{"default пуст при изделиях", "default: нужно", mutate(t, "fabrika", func(m map[string]any) { delete(m, "default") })},
		{"изделие id повторён", "id негодный или повторён", mutate(t, "fabrika", func(m map[string]any) { m["products"].([]any)[1].(map[string]any)["id"] = "fabrika-photo" })},
		{"изделий 33", "products: больше 32", mutate(t, "fabrika", func(m map[string]any) {
			ps := m["products"].([]any)
			for i := len(ps); i < 33; i++ {
				ps = append(ps, map[string]any{"id": "fabrika-x" + strings.Repeat("a", i), "name": "x"})
			}
			m["products"] = ps
		})},
		{"корешок: до N дробное", "spine: строка 1 вне", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spine"] = []any{1.5, 8} })},
		{"корешок нечётный", "spine: пар от 1", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spine"] = []any{8, 8, 12} })},
		{"корешок не по возрастанию", "spine: строка 2 вне", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spine"] = []any{12, 10, 8, 8} })},
		{"корешок 1000 мм", "spine: строка 1 вне", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spine"] = []any{8, 1000} })},
		{"корешок 65 строк", "spine: пар от 1", mutate(t, "fabrika", func(m map[string]any) {
			var s []any
			for i := 1; i <= 65; i++ {
				s = append(s, i, 8)
			}
			prod(m)["spine"] = s
		})},
		{"таблица и формула", "не обе", mutate(t, "fabrika", func(m map[string]any) { prod(m)["a"] = 5 })},
		{"spineBy чужой формат", "spineBy: формата", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spineBy"] = map[string]any{"99×99": []any{1, 8}} })},
		{"spineBy пустой", "пусто", mutate(t, "fabrika", func(m map[string]any) { prod(m)["spineBy"] = map[string]any{"30×20": []any{}} })},
		{"предел чужого формата", "limits: формата", mutate(t, "fabrika", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"format": "99×99"}} })},
		{"предел min больше max", "min больше max", mutate(t, "fabrika", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"min": 9, "max": 3}} })},
		{"предел min выше верха таблицы", "min больше max", mutate(t, "fabrika", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"min": 41}} })},
		{"предел min выше верха spineBy", "min больше max", mutate(t, "fabrika", func(m map[string]any) {
			m["products"].([]any)[2].(map[string]any)["limits"] = []any{map[string]any{"format": "30×20", "min": 11}} // верх spineBy 30×20 — 10
		})},
		{"предел 1000 разворотов", "от 0 до 200", mutate(t, "fabrika", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"max": 1000}} })},
		{"предел повторён", "повторена", mutate(t, "fabrika", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"max": 5}, map[string]any{"max": 6}} })},
		{"max 0 без таблицы", "таблицы нет", mutate(t, "netprint", func(m map[string]any) { prod(m)["limits"] = []any{map[string]any{"min": 3}} })},
		{"url не https", "url: нужен адрес https", mutate(t, "fabrika", func(m map[string]any) { prod(m)["url"] = "javascript:alert(1)" })},
		{"url с пробелом", "url: нужен адрес https", mutate(t, "fabrika", func(m map[string]any) { prod(m)["url"] = "https://a b" })},
		{"префикс с путём", "export.prefix", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["prefix"] = "../" })},
		{"префикс длинный", "export.prefix", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["prefix"] = "123456789" })},
		{"качество 80", "export.quality", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["quality"] = 80 })},
		{"качество 50", "export.quality", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["quality"] = 50 })},
		{"site чужая подстановка", "export.site: неизвестная", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["site"] = "{home}" })},
		{"title пуст", "export.title: пусто", mutate(t, "fabrika", func(m map[string]any) { m["export"].(map[string]any)["title"] = "" })},
		{"число в строке", "число, а нужно строка", mutate(t, "fabrika", func(m map[string]any) { m["name"] = 5 })},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := Parse(c.data)
			if err == nil {
				t.Fatalf("принят: %+v", p)
			}
			if !strings.Contains(err.Error(), c.why) {
				t.Errorf("отказ не по той причине:\n%v\nждали %q", err, c.why)
			}
			// Текст из encoding/json (английский, режется посреди слова) пользователю не показываем.
			for _, en := range []string{"json:", "cannot unmarshal", "unexpected", "invalid character", "Go value"} {
				if strings.Contains(err.Error(), en) {
					t.Errorf("английский текст ошибки (%q): %v", en, err)
				}
			}
			if len(err.Error()) > 400 {
				t.Errorf("текст ошибки из файла не обрезан: %d байт", len(err.Error()))
			}
		})
	}
	// Хорошее остаётся хорошим: набор мутаций не должен ломать сам встроенный файл.
	if _, err := Parse(good); err != nil {
		t.Fatalf("встроенный не разбирается: %v", err)
	}
}

// TestEncodeRejectsBad — негодный профиль не кодируется (NaN, Inf, лишнее).
func TestEncodeRejectsBad(t *testing.T) {
	p := Builtin().Profiles()[0]
	p.Page.Safe = nan()
	if _, err := Encode(p); err == nil {
		t.Error("NaN закодирован")
	}
	p = Builtin().Profiles()[0]
	p.ID = "ID"
	if _, err := Encode(p); err == nil {
		t.Error("негодный id закодирован")
	}
}

// TestSnapshotRejectsClash — два профиля с одним id, названием или изделием не складываются в снимок.
func TestSnapshotRejectsClash(t *testing.T) {
	ps := Builtin().Profiles()
	a, b := ps[0], ps[2]
	for name, mod := range map[string]func(*Profile){
		"id":       func(p *Profile) { p.ID = a.ID },
		"название": func(p *Profile) { p.Name = a.Name },
		"изделие":  func(p *Profile) { p.Products = slices.Clone(p.Products); p.Products[0].ID = a.Products[0].ID },
	} {
		bb := b
		mod(&bb)
		if _, err := NewSnapshot([]Profile{a, bb}); err == nil {
			t.Errorf("%s: снимок собран", name)
		}
	}
	bad := a
	bad.Version = 0
	if _, err := NewSnapshot([]Profile{bad}); err == nil {
		t.Error("негодный профиль вошёл в снимок")
	}
	if s, err := NewSnapshot(nil); err != nil || len(s.Profiles()) != 0 {
		t.Errorf("пустой снимок: %v", err)
	}
}

// TestUseForTest — подмена снимка живёт до конца теста.
func TestUseForTest(t *testing.T) {
	empty, _ := NewSnapshot(nil)
	t.Run("подмена", func(t *testing.T) {
		UseForTest(t, empty)
		if Current() != empty {
			t.Fatal("снимок не подменён")
		}
	})
	if Current() != Builtin() {
		t.Fatal("снимок не вернулся")
	}
	t.Cleanup(func() { Install(nil) }) // упавший тест не оставляет пустой снимок остальным
	Install(empty)
	if Current() != empty {
		t.Fatal("Install не сработал")
	}
	Install(nil)
	if Current() != Builtin() {
		t.Fatal("Install(nil) не вернул встроенные")
	}
}

// hashFile — testdata/builtin-hash.json: sha256 и version каждого встроенного профиля.
type hashEntry struct {
	SHA256  string `json:"sha256"`
	Version int    `json:"version"`
}

// TestBuiltinVersionHash — страж 0.26-123: правка встроенного JSON без подъёма version — провал.
// Иначе исправленная встроенная таблица тихо уступила бы старому набору из библиотеки с той же
// версией (design §2). -update перезаписывает файл, но отказывает, если хеш изменился, а version нет.
func TestBuiltinVersionHash(t *testing.T) {
	path := filepath.Join("testdata", "builtin-hash.json")
	var want map[string]hashEntry
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &want); err != nil {
			t.Fatalf("%s не читается: %v", path, err)
		}
	} else if !*update {
		t.Fatalf("нет %s: go test ./internal/album/model/printprof -run BuiltinVersionHash -update", path)
	}
	got := map[string]hashEntry{}
	for _, name := range builtinFiles {
		got[name] = hashOf(t, name)
	}
	var bad []string
	for _, name := range builtinFiles {
		w, ok := want[name]
		switch {
		case !ok && !*update: // -update добавляет новый профиль в запись
			bad = append(bad, name+": нет в "+path)
		case !ok:
		case w.SHA256 != got[name].SHA256 && got[name].Version <= w.Version:
			bad = append(bad, name+": файл изменён, а version осталась "+itoa(got[name].Version)+" — поднимите version")
		case w.SHA256 != got[name].SHA256 && !*update:
			bad = append(bad, name+": version поднята до "+itoa(got[name].Version)+" — обновите "+path+" (-update)")
		case w.SHA256 == got[name].SHA256 && w.Version != got[name].Version:
			bad = append(bad, name+": version в записи "+itoa(w.Version)+", в файле "+itoa(got[name].Version))
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			bad = append(bad, name+": запись есть, файла нет")
		}
	}
	if *update && len(bad) == 0 {
		b, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(bad) > 0 {
		t.Fatalf("встроенные профили разошлись с %s:\n%s", path, strings.Join(bad, "\n"))
	}
}

// FuzzParseProfile — разбор профиля не паникует на любом входе, а принятый профиль кодируется и
// разбирается обратно в тот же. Сиды — встроенные и типичные порчи.
func FuzzParseProfile(f *testing.F) {
	for _, name := range builtinFiles {
		b, err := builtinFS.ReadFile("builtin/" + name + ".json")
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
		f.Add(b[:len(b)/2])
	}
	f.Add([]byte(`{"format":1,"id":"ab","version":1,"name":"a","gen":"a","page":{"dpi":300},"formats":[{"name":"a","page":[100,100]}]}`))
	f.Add([]byte(`{"format":1,"formats":[{"page":[1e999,1]}]}`))
	f.Add([]byte(`{"format":1,"id":"ab","version":1,"name":"a","gen":"a","page":{"dpi":300},"formats":[{"name":"a","page":[100,100],"guide":{"bleed":[3,3.5],"fold":[1e999,0]}}],"cover":{"foldSafe":3}}`))
	f.Add([]byte(`[[[[[[[[[[`))
	f.Add([]byte(`{"format":1,"ID":"ab","id":"ab","id":"cd"}`)) // регистр и повтор ключей
	f.Add([]byte(`{"format":1,"id":"own-0123abcd","version":1,"name":"a","gen":"a","page":{"dpi":300},"formats":[{"name":"a","page":[100,100],"px":[65535,65535]}]}`))
	auditSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Parse(data)
		if err != nil {
			if strings.Contains(err.Error(), "json:") || strings.Contains(err.Error(), "cannot unmarshal") {
				t.Fatalf("английский текст encoding/json в ошибке: %v", err)
			}
			return
		}
		if strings.ContainsFunc(p.Name+p.Gen, func(r rune) bool { return unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp) }) {
			t.Fatalf("принято название с невидимым знаком: %q", p.Name)
		}
		if len(data) > MaxSize || len(p.Formats) > maxFormats || len(p.Products) > maxProducts || utf8.RuneCountInString(p.Name) > MaxName {
			t.Fatalf("принят профиль вне пределов: %d байт, форматов %d, изделий %d", len(data), len(p.Formats), len(p.Products))
		}
		for _, f := range p.Formats {
			if px := f.Px; px != nil && (px[0] < minPx || px[1] < minPx || px[0]*px[1] > maxMegapx) {
				t.Fatalf("принят px вне пределов: %v", *px)
			}
		}
		b, err := Encode(p)
		if err != nil || len(b) > MaxSize {
			t.Fatalf("принятый профиль не кодируется в предел: %v, %d байт", err, len(b))
		}
		fuzzStrings(t, reflect.ValueOf(p))
		again, err := Parse(b)
		if err != nil || !reflect.DeepEqual(p, again) {
			t.Fatalf("круг Encode→Parse изменил профиль: %v", err)
		}
	})
}

// TestLoadBuiltinErrors — негодный встроенный набор называет файл: нет файла, битый файл, два
// профиля с одним id.
func TestLoadBuiltinErrors(t *testing.T) {
	good := builtinData(t, "fabrika")
	for name, c := range map[string]struct {
		fsys  fstest.MapFS
		names []string
	}{
		"нет файла": {fstest.MapFS{}, []string{"fabrika"}},
		"битый":     {fstest.MapFS{"builtin/fabrika.json": {Data: good[:10]}}, []string{"fabrika"}},
		"дубль":     {fstest.MapFS{"builtin/fabrika.json": {Data: good}}, []string{"fabrika", "fabrika"}},
	} {
		if _, err := loadBuiltin(c.fsys, c.names); err == nil {
			t.Errorf("%s: набор принят", name)
		}
	}
	if s, err := loadBuiltin(fstest.MapFS{"builtin/fabrika.json": {Data: good}}, []string{"fabrika"}); err != nil || len(s.Profiles()) != 1 {
		t.Errorf("хороший набор: %v", err)
	}
}

// TestValidateText — профиль, собранный в коде, проходит те же проверки текста, что разобранный:
// не UTF-8, управляющий символ в адресе.
func TestValidateText(t *testing.T) {
	p := Builtin().Profiles()[0]
	p.Name = "\xff"
	if p.Validate() == nil {
		t.Error("не UTF-8 принят")
	}
	p = Builtin().Profiles()[2]
	p.Products = slices.Clone(p.Products)
	p.Products[0].URL = "https://a\x07b"
	if p.Validate() == nil {
		t.Error("управляющий символ в адресе принят")
	}
}

// TestMustLoadBuiltinPanics — негодный встроенный набор роняет программу с причиной, а не тихо
// отдаёт пустые таблицы.
func TestMustLoadBuiltinPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "fabrika") {
			t.Fatalf("паника не названа: %v", r)
		}
	}()
	mustLoadBuiltin(fstest.MapFS{}, []string{"fabrika"})
}

// fuzzStrings — инварианты аудита 123 по всем строкам принятого профиля: нет невидимых и
// форматирующих знаков, адреса проходят Wails и ведут на один сайт, хост — не число и не localhost.
func fuzzStrings(t *testing.T, v reflect.Value) {
	t.Helper()
	var urls []string
	var walk func(v reflect.Value, name string)
	walk = func(v reflect.Value, name string) {
		switch v.Kind() {
		case reflect.Pointer:
			if !v.IsNil() {
				walk(v.Elem(), name)
			}
		case reflect.Struct:
			for i := range v.NumField() {
				walk(v.Field(i), v.Type().Field(i).Name)
			}
		case reflect.Slice:
			for i := range v.Len() {
				walk(v.Index(i), name)
			}
		case reflect.Map:
			for _, k := range v.MapKeys() {
				walk(k, name)
				walk(v.MapIndex(k), name)
			}
		case reflect.String:
			s := v.String()
			if strings.ContainsFunc(s, func(r rune) bool {
				return hiddenRune(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp, unicode.Cc)
			}) {
				t.Fatalf("%s: принят невидимый знак: %q", name, s)
			}
			if name == "URL" && s != "" {
				urls = append(urls, s)
			}
		}
	}
	walk(v, "")
	host := ""
	for _, u := range urls {
		checkAgainstWails(t, u)
		h := siteHost(u)
		if host != "" && h != host {
			t.Fatalf("адреса на разных сайтах: %v", urls)
		}
		host = h
		labels := strings.Split(h, ".")
		if tld := labels[len(labels)-1]; tld == "localhost" || strings.Trim(tld, "0123456789") == "" || strings.HasPrefix(tld, "0x") {
			t.Fatalf("принят хост %q", h)
		}
	}
}

// auditSeeds — затравки аудита 123: у предела (32 изделия, таблицы корешка, spineBy), тексты из
// «<&>», двойники и невидимые знаки в названии, чужой хост изделия, хост-число и порт.
func auditSeeds(f *testing.F) {
	for _, p := range []Profile{bigProfile(f, "<&>", 0), bigProfile(f, "<", 2), bigProfile(f, "x", maxSpine)} {
		if b, err := json.Marshal(p); err == nil {
			f.Add(b)
		}
	}
	for _, m := range []func(map[string]any){
		func(m map[string]any) { m["name"] = "Фaбрика\u3164" },
		func(m map[string]any) { m["name"] = "Фабрика\u00a0Фотокниги" },
		func(m map[string]any) {
			m["products"].([]any)[0].(map[string]any)["url"] = "https://fabrika-fotoknigi.com.evil.example/login"
		},
		func(m map[string]any) { m["export"].(map[string]any)["url"] = "https://0x7f.0.0.1:99999/~(x)" },
	} {
		var tree map[string]any
		if raw, err := builtinFS.ReadFile("builtin/fabrika.json"); err == nil && json.Unmarshal(raw, &tree) == nil {
			m(tree)
			b, _ := json.Marshal(tree)
			f.Add(b)
		}
	}
}
