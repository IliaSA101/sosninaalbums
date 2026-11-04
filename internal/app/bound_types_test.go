package app

// Страж пространства имён биндингов (126-2): Wails кладёт тип в models.ts под именем ПАКЕТА
// (binding/reflect.go getPackageName), поэтому тип подпакета internal/app/<служба> в сигнатуре
// метода App переименовал бы его на фронте. Все DTO методов App — типы пакета app.

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/namesfile"
)

// subPkgPrefix — путь, под которым лежат подпакеты app (edit, служба, namesfile…).
func subPkgPrefix() string { return reflect.TypeFor[App]().PkgPath() + "/" }

// boundTypeWalk обходит тип так же, как Wails (указатели, срезы, массивы, словари — и ключи, и
// значения; поля-структуры рекурсивно), но строже: смотрит и встроенные поля. Возвращает
// имена типов подпакетов app, достижимых из t; seen копит обойдённые именованные типы.
func boundTypeWalk(t reflect.Type, seen map[reflect.Type]bool, via string, bad *[]string) {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		boundTypeWalk(t.Elem(), seen, via, bad)
		return
	case reflect.Map:
		boundTypeWalk(t.Key(), seen, via, bad)
		boundTypeWalk(t.Elem(), seen, via, bad)
		return
	}
	if t.Name() != "" && t.PkgPath() != "" {
		if strings.HasPrefix(t.PkgPath(), subPkgPrefix()) { // каждый путь к чужому типу — своя строка
			*bad = append(*bad, via+" → "+t.String())
		}
		if seen[t] {
			return
		}
		seen[t] = true
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for f := range t.Fields() {
		if f.IsExported() {
			boundTypeWalk(f.Type, seen, via+"."+f.Name, bad)
		}
	}
}

// boundTypeViolations — типы подпакетов, видимые из параметров и результатов методов тип typ;
// второе значение — число обойдённых именованных типов (для самопроверки).
func boundTypeViolations(typ reflect.Type) (bad []string, visited int) {
	seen := map[reflect.Type]bool{}
	for i := range typ.NumMethod() {
		m := typ.Method(i)
		mt := m.Type // первый вход — получатель
		for j := 1; j < mt.NumIn(); j++ {
			boundTypeWalk(mt.In(j), seen, m.Name+"(арг)", &bad)
		}
		for j := range mt.NumOut() {
			boundTypeWalk(mt.Out(j), seen, m.Name+"(рез)", &bad)
		}
	}
	sort.Strings(bad)
	return bad, len(seen)
}

func TestBoundTypesStayInApp(t *testing.T) {
	bad, visited := boundTypeViolations(reflect.TypeFor[*App]())
	for _, b := range bad {
		t.Errorf("тип подпакета в сигнатуре метода App: %s (в models.ts он уйдёт в чужое пространство имён; объяви тип в app: type X sub.X)", b)
	}
	t.Logf("обойдено именованных типов: %d", visited)
	if visited <= 100 {
		t.Fatalf("обойдено %d типов: страж смотрит не туда", visited)
	}
}

// subProbe — подставной App-подобный тип для самопроверки: тип подпакета прячется в поле,
// срезе, словаре (значение и ключ), указателе и результате.
type subProbe struct{}

type probeStruct struct{ X namesfile.File }
type probeNested struct {
	In struct{ Deep []*namesfile.File }
}
type probeKey map[namesfile.File]int

func (subProbe) Field(probeStruct)             {}
func (subProbe) Slice([]namesfile.File)        {}
func (subProbe) Map(map[string]namesfile.File) {}
func (subProbe) Key(probeKey)                  {}
func (subProbe) Ptr() *namesfile.File          { return nil }
func (subProbe) Nested(probeNested)            {}
func (subProbe) Array() [2]namesfile.File      { return [2]namesfile.File{} }
func (subProbe) Clean(string) (int, error)     { return 0, nil }

func TestBoundTypesGuardCatches(t *testing.T) {
	bad, _ := boundTypeViolations(reflect.TypeFor[subProbe]())
	for _, m := range []string{"Field(арг)", "Slice(арг)", "Map(арг)", "Key(арг)", "Ptr(рез)", "Nested(арг)", "Array(рез)"} {
		found := false
		for _, b := range bad {
			found = found || strings.HasPrefix(b, m)
		}
		if !found {
			t.Errorf("страж не поймал тип подпакета в %s: %q", m, bad)
		}
	}
	for _, b := range bad {
		if strings.HasPrefix(b, "Clean") {
			t.Errorf("ложное срабатывание: %s", b)
		}
	}
}
