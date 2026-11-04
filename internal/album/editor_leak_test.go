package album_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Страж границы Editor (аудит 0.8.0): экспортный метод не отдаёт наружу внутренности живого
// проекта — указатель на тип модели, map или срез без копии. Чужой код (App, экспорт, фон)
// держит ответ после снятия замка и иначе правил бы проект в обход change: ни undo, ни
// no-op, ни rev. Две половины: (1) по типам результатов — каждый указатель/map/срез в
// ответе метода обязан быть в таблицах ниже с причиной, новый метод без записи роняет тест;
// (2) по поведению — метод читателя вызывается, ответ портится целиком, вызов повторяется:
// если внутри проекта что-то изменилось, ответ был не копией.

// leakCopies — методы, чей ответ — копия состояния проекта (или отметок); проверяются порчей.
var leakCopies = map[string]string{
	"Snapshot":     "Clone() проекта под замком",
	"SnapshotAt":   "Clone() проекта под замком",
	"SnapshotSide": "Clone() проекта и Side под замком",
	"Refs":         "salbum.Refs — глубокая копия записей фото",
	"Ref":          "PhotoRef.Copy() — Frame, Caption, Adj, Person не общие с проектом",
	"FileRef":      "PhotoRef.Copy() записи семьи",
	"Marks":        "MarkView — значения, построенные заново",
	"LibKeys":      "новая map из ключей библиотеки",
	"Look":         "копия записи макета (c := *p.Look)",
	"Folders":      "slices.Clone",
	"Sources":      "slices.Clone папок и новый срез отдельных фото",
	"BorderIDs":    "новый срез ID, отсортированный и сжатый",
	"UsedFiles":    "новые срез и map занятых файлов",
	"PhotosOf":     "новый срез, собранный append из Photos разворотов",
	"PhotoUses":    "view.Uses — новый срез значений",
	"WhereUsed":    "view.WhereUsed — новый срез значений",
	"PlaceBoxes":   "grid.PlaceBoxes — новые Box, Border копируется",
	"FaceSlots":    "design.FaceSlots — новые значения",
}

// leakFresh — методы, чей срез/указатель строится заново из аргументов или результата правки
// и с проектом не делится; порчей не проверяются (нужны разные аргументы правок).
var leakFresh = map[string]string{
	"AutoSimilar":       "срез отброшенных из переданных photos",
	"AddStickersIf":     "ID новых стикеров, новый срез",
	"FillPlaces":        "names — новые имена из переданных photos",
	"PlacePhotos":       "names — новые имена из переданных photos",
	"CreateCoverLook":   "*LookRef — копия записи макета после правки",
	"CreateCoverLookIf": "*LookRef — копия записи макета после правки",
	"VignetteChoices":   "новые выборы раскладок над копией заголовка листа",
	"Families":          "новый срез семейств по ID",
}

// leakArgs — как вызвать метод читателя для порчи: имя → аргументы по состоянию редактора.
func leakArgs(e *album.Editor, name string) []reflect.Value {
	p := edtest.P(e)
	ids := []string{}
	for id := range p.Photos {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	sp := p.Spreads[0].ID
	in := func(a ...any) []reflect.Value {
		out := make([]reflect.Value, len(a))
		for i, x := range a {
			out[i] = reflect.ValueOf(x)
		}
		return out
	}
	switch name {
	case "PhotosOf", "WhereUsed":
		return in([]string{sp})
	case "PhotoUses", "Ref", "FileRef":
		return in(ids[0])
	case "FaceSlots":
		return in(sp)
	case "PlaceBoxes":
		return in(sp, model.ScopeSpread)
	}
	return nil
}

func TestEditorReturnsNoInternals(t *testing.T) {
	e := leakEditor(t)
	et := reflect.TypeOf(e)
	for i := 0; i < et.NumMethod(); i++ {
		m := et.Method(i)
		for j := 0; j < m.Type.NumOut(); j++ {
			out := m.Type.Out(j)
			if bad := leakKind(out); bad != "" && leakCopies[m.Name] == "" && leakFresh[m.Name] == "" {
				t.Errorf("Editor.%s возвращает %s (%s) без копии или записи в leakCopies/leakFresh: "+
					"внутренность проекта наружу в обход change — верните копию (Clone/slices.Clone) "+
					"или внесите метод в таблицу с причиной", m.Name, out, bad)
			}
			if leakProject(out) && !strings.HasPrefix(m.Name, "Snapshot") {
				t.Errorf("Editor.%s отдаёт *model.Project/*model.Spread: живой проект только у ядра, наружу — Snapshot()", m.Name)
			}
		}
	}
	for _, tab := range []map[string]string{leakCopies, leakFresh} {
		for name := range tab {
			if _, ok := et.MethodByName(name); !ok {
				t.Errorf("в таблице стража нет метода Editor.%s — убрать запись", name)
			}
		}
	}
}

// leakKind — чем тип результата опасен: указатель на тип модели, map, срез; "" — безопасен.
func leakKind(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		if t.Elem().PkgPath() == reflect.TypeFor[model.Project]().PkgPath() {
			return "указатель на тип модели"
		}
	case reflect.Map:
		return "map"
	case reflect.Slice:
		return "срез"
	}
	return ""
}

// leakProject — тип ответа сам *Project или *Spread (или срез/map таких).
func leakProject(t reflect.Type) bool {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	return t == reflect.TypeFor[model.Project]() || t == reflect.TypeFor[model.Spread]()
}

// Порча ответа не должна дойти до проекта: вызов, снимок JSON, порча, повторный вызов.
func TestEditorCopiesAreCopies(t *testing.T) {
	e := leakEditor(t)
	et := reflect.TypeOf(e)
	for name, why := range leakCopies {
		m, ok := et.MethodByName(name)
		if !ok {
			continue // отсутствие ловит TestEditorReturnsNoInternals
		}
		t.Run(name, func(t *testing.T) {
			call := func() []reflect.Value {
				return m.Func.Call(append([]reflect.Value{reflect.ValueOf(e)}, leakArgs(e, name)...))
			}
			before := leakJSON(t, call())
			rs := call()
			n := 0
			for _, r := range rs {
				n += spoilAnswer(r)
			}
			if n == 0 {
				t.Logf("нечего портить в ответе (%s): проверка пуста", why)
			}
			if after := leakJSON(t, call()); after != before {
				t.Fatalf("Editor.%s: порча ответа дошла до проекта — ответ не копия (%s)\nбыло:   %s\nстало: %s", name, why, before, after)
			}
		})
	}
	// Проверка не пуста там, где данные есть заведомо.
	for _, name := range []string{"Snapshot", "Refs", "Ref", "Marks", "Folders", "Look", "PlaceBoxes"} {
		m, _ := et.MethodByName(name)
		n := 0
		for _, r := range m.Func.Call(append([]reflect.Value{reflect.ValueOf(e)}, leakArgs(e, name)...)) {
			n += spoilAnswer(r)
		}
		if n == 0 {
			t.Errorf("Editor.%s: в ответе нечего портить — заполните leakEditor, иначе страж слеп", name)
		}
	}
}

// leakJSON — состояние ответа до порчи (глубокая независимая копия в виде текста).
func leakJSON(t *testing.T, rs []reflect.Value) string {
	t.Helper()
	var sb strings.Builder
	for _, r := range rs {
		b, err := json.Marshal(r.Interface())
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
		sb.WriteByte(';')
	}
	return sb.String()
}

// spoilAnswer — spoil для ответа метода, кроме model.View: View строится заново на каждый
// вызов, но его Rects/Frames автоколонок берутся из общего кэша раскладок (layout.Cached) и
// по договору только читаются (docs/specs/0.37/decisions.md, «Страж Editor»).
func spoilAnswer(v reflect.Value) int {
	if v.Type() == reflect.TypeFor[model.View]() {
		return 0
	}
	return spoil(v)
}

// spoil портит всё, что достижимо из v и доступно записи; возвращает число испорченных листьев.
// Если ответ делит память с проектом, повторный вызов вернёт испорченное.
func spoil(v reflect.Value) int {
	n := 0
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			n += spoil(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				n += spoil(v.Field(i))
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			n += spoil(v.Index(i))
		}
		if v.Len() > 0 && v.Index(0).CanSet() {
			for i := 0; i < v.Len(); i++ {
				v.Index(i).SetZero()
			}
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			c := reflect.New(v.Type().Elem()).Elem()
			c.Set(v.MapIndex(k))
			n += spoil(c)
			v.SetMapIndex(k, c)
		}
		if v.Len() > 0 {
			v.Clear()
			n++
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString("☠")
			n++
		}
	case reflect.Bool:
		if v.CanSet() {
			v.SetBool(!v.Bool())
			n++
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if v.CanSet() {
			v.SetInt(v.Int() + 7)
			n++
		}
	case reflect.Float32, reflect.Float64:
		if v.CanSet() {
			v.SetFloat(v.Float() + 7)
			n++
		}
	}
	return n
}

// leakEditor — альбом, где у каждого читателя есть что портить: рамочки мест, кадр, подпись,
// тон, имя, источники, макет, отметки.
func leakEditor(t *testing.T) *album.Editor {
	t.Helper()
	pl := func(id, photo string, x float64) *model.Place {
		p := edtest.Gp(x, 10, 100, 100, photo)
		p.ID = id
		p.Border = &model.Border{Kind: "frame", Inks: []string{"#112233"}, Width: 2}
		return p
	}
	e := edtest.GridEd(t, []*model.Place{pl("a00000000001", "a", 10), pl("a00000000002", "b", 120)}, []string{"c"})
	edtest.Change(e, func(p *model.Project) {
		p.Folders = []string{edtest.Abs("dir1"), edtest.Abs("dir2")}
		p.Loose = []model.LooseAdd{{ID: "aaaaaaaaaaaa", At: 1, Files: []model.LooseFile{{Path: edtest.Abs("one.jpg")}}}}
		p.Look = &model.LookRef{Key: "my:0123456789abcdef", Name: "n", Pack: "pack-x", Item: "a/b.json"}
		for id, r := range p.Photos {
			r.Frame = &model.Frame{S: 1.5, X: 0.2}
			r.Adj = &model.Tone{Bright: 5}
			r.Person = &model.Person{Last: "Иванов"}
			r.Caption = &model.Caption{Text: "подпись"}
			p.Photos[id] = r
		}
	})
	edtest.SetMarks(t, e, edtest.Photos("a", "b"), model.MarkPatch{Rating: edtest.Ip(4)})
	return e
}
