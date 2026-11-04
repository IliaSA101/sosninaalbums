package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// useFont — своя гарнитура name из байтов встроенного файла file (реестр text общий на пакет —
// после теста снимается). Файл берётся непохожий на Montserrat, чтобы ширины отличались.
func useFont(t *testing.T, name, file string) string {
	t.Helper()
	b, ok := text.FontFile(file)
	if !ok {
		t.Fatalf("нет встроенного %s", file)
	}
	h := sha256.Sum256(b)
	id := text.FamilyID(name)
	text.SetCustom([]text.CustomFamily{{ID: id, Name: name, Faces: []text.CustomFace{{Key: hex.EncodeToString(h[:8]), Ext: "ttf"}}}},
		func(string) ([]byte, error) { return b, nil })
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	if !text.Available(id) {
		t.Fatalf("%s не зарегистрирована", name)
	}
	return id
}

// fillStyles — значение типа t, где каждое достижимое поле заполнено (указатель, один элемент
// списка и словаря), а у каждого стиля текста — свой id своей гарнитуры (в ids). Рекурсивный
// тип обрывается пустым значением.
func fillStyles(t reflect.Type, open map[reflect.Type]bool, ids *[]string) reflect.Value {
	v := reflect.New(t).Elem()
	if t == reflect.TypeFor[TextStyle]() {
		id := fmt.Sprintf("my:%016x", len(*ids)+1)
		*ids = append(*ids, id)
		v.Set(reflect.ValueOf(TextStyle{Font: id, Size: 12}))
		return v
	}
	switch t.Kind() {
	case reflect.Pointer:
		v.Set(fillStyles(t.Elem(), open, ids).Addr())
	case reflect.Struct:
		if open[t] {
			return v
		}
		open[t] = true
		defer delete(open, t)
		for i := range t.NumField() {
			if f := t.Field(i); f.IsExported() {
				v.Field(i).Set(fillStyles(f.Type, open, ids))
			}
		}
	case reflect.Slice:
		v.Set(reflect.Append(v, fillStyles(t.Elem(), open, ids)))
	case reflect.Array:
		for i := range t.Len() {
			v.Index(i).Set(fillStyles(t.Elem(), open, ids))
		}
	case reflect.Map:
		v.Set(reflect.MakeMap(t))
		v.SetMapIndex(reflect.ValueOf("k").Convert(t.Key()), fillStyles(t.Elem(), open, ids))
	}
	return v
}

// Обход UsedFonts видит каждый стиль текста, достижимый из Project: новое поле стиля без
// обхода роняет тест (иначе SyncFonts выбросил бы его гарнитуру из таблицы, а полоса «нет
// шрифта» её не показала бы).
func TestUsedFontsCoversStyles(t *testing.T) {
	var ids []string
	p := fillStyles(reflect.TypeFor[Project](), map[reflect.Type]bool{}, &ids).Addr().Interface().(*Project)
	if len(ids) < 3 { // тексты, общий стиль подписей, подпись фото
		t.Fatalf("стилей в Project найдено %d", len(ids))
	}
	slices.Sort(ids)
	if got := p.UsedFonts(); !slices.Equal(got, ids) {
		t.Fatalf("UsedFonts %v, стили Project %v", got, ids)
	}
	// Повтор и встроенные не попадают; без своих — пусто.
	q := New()
	q.Spreads = []*Spread{{ID: "s", Texts: []*TextBlock{
		{ID: "t1", Style: TextStyle{Font: ids[0]}}, {ID: "t2", Style: TextStyle{Font: ids[0]}}, {ID: "t3", Style: PlainStyle()}}}}
	if got := q.UsedFonts(); !slices.Equal(got, ids[:1]) {
		t.Fatalf("повтор и встроенная: %v", got)
	}
	if got := New().UsedFonts(); len(got) != 0 {
		t.Fatalf("пустой проект: %v", got)
	}
}

// fontsProject — две свои гарнитуры (текст листа, общий стиль подписей обложки, подпись фото) и
// встроенная.
func fontsProject(a, b string) *Project {
	p := New()
	p.Photos["p1"] = PhotoRef{Path: "a.jpg", W: 3, H: 2, Caption: &Caption{Text: "x", Style: TextStyle{Font: b, Size: 10}}}
	p.Spreads = []*Spread{{ID: "s", Photos: []string{"p1"}, Texts: []*TextBlock{
		{ID: "t1", Text: "Анна", Style: TextStyle{Font: a, Size: 20}}, {ID: "t2", Text: "x", Style: PlainStyle()}}}}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID, Caps: &SheetCaps{Style: TextStyle{Font: a, Size: 9}}}}
	return p
}

// SyncFonts: запись у каждой стоящей своей гарнитуры (имя — из реестра, нет в реестре — прежнее
// или пустое), неиспользуемые убираются, без своих — nil; идемпотентна и без выделения памяти,
// когда своих шрифтов нет.
func TestSyncFontsPrunes(t *testing.T) {
	lato := useFont(t, "  Lato\tScript ", "Oswald-Regular.ttf")
	gone := text.FamilyID("Bodoni Moda")
	p := fontsProject(lato, gone)
	p.Fonts = map[string]FontRef{gone: {Name: "Bodoni Moda"}, text.FamilyID("Старый"): {Name: "Старый"}}
	p.SyncFonts()
	want := map[string]FontRef{lato: {Name: "Lato Script"}, gone: {Name: "Bodoni Moda"}}
	if !maps.Equal(p.Fonts, want) {
		t.Fatalf("таблица %v, ждали %v", p.Fonts, want)
	}
	again := p.Clone()
	again.SyncFonts()
	if !maps.Equal(again.Fonts, want) {
		t.Fatalf("второй SyncFonts: %v", again.Fonts)
	}

	// Своя без записи — запись с пустым именем (из шаблона); после снятия всех — nil.
	p.Spreads[0].Texts[1].Style.Font = text.FamilyID("Из шаблона")
	p.SyncFonts()
	if r, ok := p.Fonts[text.FamilyID("Из шаблона")]; !ok || r.Name != "" || len(p.Fonts) != 3 {
		t.Fatalf("своя без записи: %v", p.Fonts)
	}
	p.Spreads[0].Texts[0].Style.Font, p.Spreads[0].Texts[1].Style.Font = text.DefaultFont, text.DefaultFont
	p.Cover.Sheet.Caps.Style.Font = text.DefaultFont
	p.Photos["p1"].Caption.Style.Font = text.DefaultFont
	p.SyncFonts()
	if p.Fonts != nil {
		t.Fatalf("без своих шрифтов: %v", p.Fonts)
	}
	if n := testing.AllocsPerRun(20, p.SyncFonts); n != 0 {
		t.Fatalf("SyncFonts без своих шрифтов выделяет память: %v", n)
	}

	// Предел: больше MaxFonts стоящих — первые по id, как ClampFonts при чтении.
	q := New()
	s := &Spread{ID: "s"}
	for i := range MaxFonts + 2 {
		s.Texts = append(s.Texts, &TextBlock{ID: fmt.Sprintf("t%d", i), Style: TextStyle{Font: fmt.Sprintf("my:%016x", i)}})
	}
	q.Spreads = []*Spread{s}
	q.SyncFonts()
	if len(q.Fonts) != MaxFonts || q.Fonts[fmt.Sprintf("my:%016x", MaxFonts)] != (FontRef{}) {
		t.Fatalf("предел: %d записей", len(q.Fonts))
	}
	if _, ok := q.Fonts[fmt.Sprintf("my:%016x", MaxFonts+1)]; ok {
		t.Fatal("сверх предела запись осталась")
	}
}

// ClampFonts: остаются только записи стоящих в стилях (ключ не id своей — выброшен), название —
// одна строка ≤ MaxFontNameRunes, не больше первых MaxFonts стоящих; лишние записи их не вытесняют.
func TestClampFonts(t *testing.T) {
	id := text.FamilyID("Lato")
	if ClampFonts(nil, []string{id}) != nil || ClampFonts(map[string]FontRef{id: {}}, nil) != nil ||
		ClampFonts(map[string]FontRef{"montserrat": {Name: "M"}, "my:XYZ": {}}, []string{"montserrat", "my:XYZ"}) != nil {
		t.Fatal("без годных стоящих записей — nil")
	}
	got := ClampFonts(map[string]FontRef{id: {Name: " Lato\n" + strings.Repeat("я", 100)}, "../x": {Name: "x"}}, []string{id})
	if len(got) != 1 || got[id].Name != "Lato "+strings.Repeat("я", MaxFontNameRunes-5) {
		t.Fatalf("зажим: %q", got)
	}
	// Лишние записи (128 нестоящих) не вытесняют стоящие.
	many := map[string]FontRef{id: {Name: "Lato"}}
	for i := range MaxFonts * 2 {
		many[fmt.Sprintf("my:%016x", i)] = FontRef{Name: "f"}
	}
	used := []string{fmt.Sprintf("my:%016x", 3), fmt.Sprintf("my:%016x", 100), id}
	slices.Sort(used)
	if got = ClampFonts(many, used); len(got) != 3 || got[id].Name != "Lato" {
		t.Fatalf("лишние записи: %d, Lato %q", len(got), got[id].Name)
	}
	// Стоящих больше MaxFonts — первые MaxFonts по возрастанию id.
	used = used[:0]
	for i := range MaxFonts + 6 {
		used = append(used, fmt.Sprintf("my:%016x", i))
	}
	if got = ClampFonts(many, used); len(got) != MaxFonts {
		t.Fatalf("предел: %d", len(got))
	}
	if _, ok := got[used[MaxFonts]]; ok {
		t.Fatal("сверх первых MaxFonts стоящих запись осталась")
	}
}

// FitTexts не трогает блок своим шрифтом, которого нет на этом компьютере (082-5): ни ширину
// «по тексту», ни положение; соседний блок встроенным шрифтом зажимается как обычно. Шрифт
// нашёлся — блок считается его шириной.
func TestFitTextsSkipsMissingFont(t *testing.T) {
	lato := text.FamilyID("Lato")
	p := New()
	missing := &TextBlock{ID: "t1", X: -40, Y: 900, W: 123.45, Auto: true, Text: "Анна и Павел", Style: TextStyle{Font: lato, Size: 30}}
	fixed := &TextBlock{ID: "t2", X: 10, Y: 10, W: 5000, Text: "x", Style: TextStyle{Font: lato, Size: 30}}
	plain := &TextBlock{ID: "t3", X: -40, Y: 10, W: 123.45, Auto: true, Text: "Анна и Павел", Style: TextStyle{Font: "montserrat", Size: 30}}
	p.Spreads = []*Spread{{ID: "s", Texts: []*TextBlock{missing, fixed, plain}}}
	p.FitTexts()
	if missing.X != -40 || missing.Y != 900 || missing.W != 123.45 {
		t.Fatalf("блок без шрифта сдвинут: %+v", *missing)
	}
	if fixed.W != 5000 || fixed.X != 10 {
		t.Fatalf("закреплённый блок без шрифта зажат: %+v", *fixed)
	}
	if plain.W == 123.45 || plain.X < 0 {
		t.Fatalf("встроенный не зажат: %+v", *plain)
	}

	useFont(t, "Lato", "Oswald-Regular.ttf")
	p.FitTexts()
	want := AutoW(nil, missing, p.Geo(p.Spreads[0]).W)
	if missing.W != want || missing.W == plain.W || missing.Y >= 900 {
		t.Fatalf("шрифт нашёлся: %+v, ширина ждали %v (Montserrat %v)", *missing, want, plain.W)
	}
	if fixed.W >= 5000 {
		t.Fatalf("закреплённый не зажат: %v", fixed.W)
	}
}
