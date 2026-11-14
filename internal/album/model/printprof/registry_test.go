package printprof

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

// shared — пути, где a и b делят память: указатель, карта или срез с одним началом. Пустые срезы и
// nil не считаются (делить нечего).
func shared(a, b reflect.Value, path string, out *[]string) {
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() {
			return
		}
		if a.Pointer() == b.Pointer() {
			*out = append(*out, path)
			return
		}
		shared(a.Elem(), b.Elem(), path, out)
	case reflect.Map:
		if a.Len() == 0 {
			return
		}
		if a.Pointer() == b.Pointer() {
			*out = append(*out, path)
			return
		}
		for _, k := range a.MapKeys() {
			shared(a.MapIndex(k), b.MapIndex(k), fmt.Sprintf("%s[%v]", path, k), out)
		}
	case reflect.Slice:
		if a.Len() == 0 {
			return
		}
		if a.Pointer() == b.Pointer() {
			*out = append(*out, path)
			return
		}
		for i := range a.Len() {
			shared(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i), out)
		}
	case reflect.Struct:
		for i := range a.NumField() {
			shared(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name, out)
		}
	}
}

// TestCloneNoSharing — Clone глубокий: равен оригиналу и не делит с ним ни среза, ни карты, ни
// указателя. Обход идёт по reflect, поэтому новое поле-ссылка без копирования в Clone ловится само.
func TestCloneNoSharing(t *testing.T) {
	cases := append(Builtin().Profiles(), fullProfile())
	for _, p := range cases {
		c := p.Clone()
		if !reflect.DeepEqual(c, p) {
			t.Errorf("%s: копия не равна оригиналу", p.ID)
		}
		var bad []string
		shared(reflect.ValueOf(p), reflect.ValueOf(c), "", &bad)
		if len(bad) > 0 {
			t.Errorf("%s: общая память: %v", p.ID, bad)
		}
	}
	// Пустой профиль не падает на nil-ссылках.
	if c := (Profile{}).Clone(); !reflect.DeepEqual(c, Profile{}) {
		t.Errorf("пустой профиль: %+v", c)
	}
}

// TestSnapshotReturnsCopies — правка полученного из снимка профиля снимок не меняет: ByID,
// ByName, Profiles, а также входной список NewSnapshot и Compose.
func TestSnapshotReturnsCopies(t *testing.T) {
	want := Builtin().Profiles() // сам по себе копия; сверяем с ним же заново
	same := func(what string) {
		t.Helper()
		if got := Builtin().Profiles(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: встроенный снимок изменился", what)
		}
	}
	scribble := func(p *Profile) {
		p.Formats[0].Name = "испорчено"
		p.Formats[0].Page[0] = 1
		p.Formats[0].Px = &PxPair{1, 1}
		p.Products[0].Spine[1] = 99
		p.Products[0].Limits[0].Min = 99
		p.Products[0].SpineBy = map[string][]float64{"x": {1}}
		p.Cover.Wrap = 99
		p.Export.URL = "https://evil.example/"
	}
	cp, ok := Builtin().ByID("fabrika")
	if !ok {
		t.Fatal("нет fabrika")
	}
	scribble(&cp)
	same("ByID")
	cp, _ = Builtin().ByName(cp.Name)
	// SpineBy есть у fabrika-photo-2: правка таблицы по ключу не должна дойти до снимка.
	cp.Products[2].SpineBy["30×20"][1] = 99
	same("ByName: spineBy")
	for _, p := range Builtin().Profiles() {
		if len(p.Formats) > 0 && len(p.Products) > 0 && p.Cover != nil && p.Export != nil {
			scribble(&p)
		}
	}
	same("Profiles")

	in := []Profile{fullProfile()}
	s, err := NewSnapshot(in)
	if err != nil {
		t.Fatal(err)
	}
	in[0].Formats[0].Name = "испорчено"
	in[0].Products[0].Spine[1] = 99
	if got, _ := s.ByID("own-12345678"); !reflect.DeepEqual(got, fullProfile()) {
		t.Error("NewSnapshot: правка входного списка дошла до снимка")
	}
	lib := []Profile{libOf(t, "fsfera", 2)}
	cs, _ := Compose(lib, nil)
	lib[0].Formats[0].Name = "испорчено"
	if got, _ := cs.ByID("fsfera"); got.Formats[0].Name == "испорчено" {
		t.Error("Compose: правка входного списка дошла до снимка")
	}
}

// TestBuiltinURLs — адреса встроенных: страница каждого изделия и общий адрес выгрузки. Таблица
// записана руками: опечатка в JSON или подмена хоста ловится здесь (OpenPrintSite открывает ровно их).
func TestBuiltinURLs(t *testing.T) {
	const fab = "https://fabrika-fotoknigi.com/"
	want := map[string]struct {
		export   string
		products map[string]string
	}{
		"fsfera":   {"https://fsfera.ru/order_pro/calculator/", nil},
		"netprint": {"", nil},
		"fabrika": {fab + "photo-books", map[string]string{
			"fabrika-photo":          fab + "photobook-photopaper",
			"fabrika-photo-1":        fab + "photobook-photopaper",
			"fabrika-photo-2":        fab + "photobook-photopaper",
			"fabrika-layflat-paper":  fab + "layflat",
			"fabrika-layflat-card":   fab + "layflat",
			"fabrika-layflat-card-1": fab + "layflat",
		}},
	}
	for _, p := range Builtin().Profiles() {
		w, ok := want[p.ID]
		if !ok {
			t.Errorf("%s: нет в таблице адресов", p.ID)
			continue
		}
		exportURL := ""
		if p.Export != nil {
			exportURL = p.Export.URL
		}
		if exportURL != w.export {
			t.Errorf("%s: export.url %q, ждали %q", p.ID, exportURL, w.export)
		}
		for _, pr := range p.Products {
			if pr.URL != w.products[pr.ID] {
				t.Errorf("%s: url %q, ждали %q", pr.ID, pr.URL, w.products[pr.ID])
			}
			delete(w.products, pr.ID)
		}
		if len(w.products) > 0 {
			t.Errorf("%s: изделий в таблице больше, чем в профиле: %v", p.ID, w.products)
		}
	}
}

// minimal — наименьший годный профиль для проб границ.
func minimal() Profile {
	return Profile{Format: 1, ID: "ab", Version: 1, Name: "Н", Gen: "Н", Page: Page{DPI: 300}}
}

// TestValidateBoundaries — на самом пределе профиль годен, на шаг дальше — нет (границы из design
// §6; отказ каждой по своей причине проверяет TestParseRejects).
func TestValidateBoundaries(t *testing.T) {
	long := func(n int) string { return strings.Repeat("я", n) }
	withExport := func(f func(*Export)) Profile {
		p := minimal()
		p.Export = &Export{Acc: "а", Title: "а", CoverWord: "а", Plain: "а", Done: "а", Site: "а", SiteButton: "а"}
		f(p.Export)
		return p
	}
	product := func(f func(*Product, *Profile)) Profile {
		p := minimal()
		p.Formats = []PageFormat{{Name: "20×20", Page: Pair{200, 200}}}
		p.Products = []Product{{ID: "ab-x", Name: "х", Spine: []float64{10, 8, 40, 12}}}
		p.Default = "ab-x"
		f(&p.Products[0], &p)
		return p
	}
	ok := map[string]Profile{
		"id 32 знака":     func() Profile { p := minimal(); p.ID = "a" + strings.Repeat("b", 31); return p }(),
		"version 999999":  func() Profile { p := minimal(); p.Version = 999_999; return p }(),
		"название 40":     func() Profile { p := minimal(); p.Name = long(40); return p }(),
		"hint 300":        func() Profile { p := minimal(); p.Page.Hint = long(300); return p }(),
		"item 200":        product(func(pr *Product, _ *Profile) { pr.Item = long(200) }),
		"done 200":        withExport(func(e *Export) { e.Done = long(200) }),
		"url 300":         withExport(func(e *Export) { e.URL = "https://example.com/" + strings.Repeat("a", 280) }),
		"качество 95":     withExport(func(e *Export) { e.Quality = 95 }),
		"качество 100":    withExport(func(e *Export) { e.Quality = 100 }),
		"форматов 32":     func() Profile { p := minimal(); p.Formats = manyFormats(32); return p }(),
		"изделий 32":      func() Profile { p := minimal(); p.Products, p.Default = manyProducts(p.ID, 32); return p }(),
		"min равен верху": product(func(pr *Product, _ *Profile) { pr.Limits = []Limit{{Min: 40}} }),
		"min равен верху формата": product(func(pr *Product, _ *Profile) {
			pr.SpineBy = map[string][]float64{"20×20": {10, 8, 12, 10}}
			pr.Limits = []Limit{{Format: "20×20", Min: 12}}
		}),
		"строк предела 33": product(func(pr *Product, p *Profile) {
			p.Formats = manyFormats(32)
			pr.Limits = []Limit{{Min: 2}}
			for _, f := range p.Formats {
				pr.Limits = append(pr.Limits, Limit{Format: f.Name, Min: 2})
			}
		}),
	}
	for name, p := range ok {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Шаг за границей.
	over := map[string]Profile{
		"форматов 33": func() Profile { p := minimal(); p.Formats = manyFormats(33); return p }(),
		"изделий 33":  func() Profile { p := minimal(); p.Products, p.Default = manyProducts(p.ID, 33); return p }(),
	}
	for name, p := range over {
		if p.Validate() == nil {
			t.Errorf("%s: принят", name)
		}
	}
}

func manyFormats(n int) []PageFormat {
	out := make([]PageFormat, n)
	for i := range out {
		out[i] = PageFormat{Name: fmt.Sprintf("ф%d", i), Page: Pair{200, 200}}
	}
	return out
}

func manyProducts(profile string, n int) ([]Product, string) {
	out := make([]Product, n)
	for i := range out {
		out[i] = Product{ID: fmt.Sprintf("%s-p%d", profile, i), Name: "х"}
	}
	return out, out[0].ID
}

// TestReasonRussian — причины разбора JSON по-русски для каждого вида ошибки encoding/json и для
// видов значений, которых нет в нашем формате (да/нет, объект, прочее).
func TestReasonRussian(t *testing.T) {
	type s struct {
		B bool           `json:"b"`
		O struct{}       `json:"o"`
		L []int          `json:"l"`
		S string         `json:"s"`
		F float64        `json:"f"`
		M map[string]int `json:"m"`
	}
	for in, want := range map[string]string{
		`{"b":"да"}`:  `поле "b": строка, а нужно да/нет`,
		`{"o":5}`:     `поле "o": число, а нужно объект`,
		`{"l":{}}`:    `поле "l": объект, а нужно список`,
		`{"s":true}`:  `поле "s": да/нет, а нужно строка`,
		`{"f":"x"}`:   `поле "f": строка, а нужно число`,
		`{"m":[]}`:    `поле "m": список, а нужно объект`,
		`{"l":[1.5]}`: `поле "l": число 1.5, а нужно целое число`,
		`{"b":}`:      "это не JSON (ошибка около байта 6)",
		`"строка"`:    "строка, а нужно объект",
	} {
		var v s
		err := json.Unmarshal([]byte(in), &v)
		if err == nil {
			t.Fatalf("%s: разобрано", in)
		}
		if got := reason(err); !strings.Contains(got, want) {
			t.Errorf("%s: %q, ждали %q", in, got, want)
		}
	}
	if got := reason(errors.New("json: unknown field")); got != "содержимое не разбирается" {
		t.Errorf("прочая ошибка: %q", got)
	}
	for _, e := range []error{io.EOF, io.ErrUnexpectedEOF, fmt.Errorf("обёртка: %w", io.ErrUnexpectedEOF)} {
		if got := reason(e); got != "файл оборван" {
			t.Errorf("%v: %q", e, got)
		}
	}
	if got := reason(userError("уже по-русски")); got != "уже по-русски" {
		t.Errorf("userError: %q", got)
	}
	if got := kindWord(reflect.TypeFor[uint8]()); got != "целое число" {
		t.Errorf("uint8: %q", got)
	}
}
