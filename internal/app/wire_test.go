package app

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// wireApp — App с альбомом s1…s3 (по фото) и записью событий view и saved.
func wireApp(t *testing.T) (a *App, views func() []model.View, saved func() []any) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var vs []model.View
	var ss []any
	a.emit = func(name string, data ...any) {
		mu.Lock()
		defer mu.Unlock()
		switch name {
		case "view":
			vs = append(vs, data[0].(model.View))
		case "saved":
			ss = append(ss, data[0])
		}
	}
	p := model.New()
	for i, id := range []string{"a", "b", "c"} {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + strconv.Itoa(i+1), Photos: []string{id}})
		p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
	}
	a.ed.Store(mustEditor(t, p, ""))
	views = func() []model.View { mu.Lock(); defer mu.Unlock(); return append([]model.View{}, vs...) }
	saved = func() []any { mu.Lock(); defer mu.Unlock(); return append([]any{}, ss...) }
	return a, views, saved
}

// wireSpreads — ID разворотов, ушедших в JSON (что увидит фронт).
func wireSpreads(t *testing.T, v model.View) (ids []string, m map[string]any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	l, _ := m["spreads"].([]any)
	for _, s := range l {
		ids = append(ids, s.(map[string]any)["id"].(string))
	}
	return ids, m
}

// TestWireChain — цепочка видов App (075-1…075-4): полный, дельта, холостая и обогнанная правка
// без события, отмена — шаг, GetView без сдвига, другой альбом — полный.
func TestWireChain(t *testing.T) {
	a, views, _ := wireApp(t)
	st := a.GetState()
	if st.View.Seq != 1 || st.View.Base != 0 {
		t.Fatalf("GetState: seq %d base %d", st.View.Seq, st.View.Base)
	}
	if ids, _ := wireSpreads(t, st.View); len(ids) != 3 {
		t.Fatalf("полный вид несёт %v", ids)
	}
	if v := a.GetView(); v.Seq != 1 || v.Base != 0 || len(views()) != 0 {
		t.Fatalf("GetView на том же виде: seq %d base %d, событий %d", v.Seq, v.Base, len(views()))
	}
	old := a.editor().View()

	// Правка двух разворотов из трёх подряд: в дельте только изменённый, событие — та же дельта.
	w := a.SetSpreadName("s2", "Свадьба")
	ids, m := wireSpreads(t, w)
	if w.Seq != 2 || w.Base != 1 || len(ids) != 1 || ids[0] != "s2" || len(w.Spreads) != 3 {
		t.Fatalf("дельта: seq %d base %d, в JSON %v, в Go %d разворотов", w.Seq, w.Base, ids, len(w.Spreads))
	}
	if o, _ := m["order"].([]any); len(o) != 3 {
		t.Fatalf("order %v", m["order"])
	}
	if ev := views(); len(ev) != 1 || ev[0].Seq != 2 || ev[0].Base != 1 {
		t.Fatalf("событие view: %d шт.", len(ev))
	}
	w = a.SetSpreadName("s3", "Прогулка")
	if ids, _ = wireSpreads(t, w); w.Seq != 3 || w.Base != 2 || len(ids) != 1 || ids[0] != "s3" {
		t.Fatalf("вторая дельта: seq %d base %d, %v", w.Seq, w.Base, ids)
	}

	// Холостая правка: пустая дельта к нынешнему виду, без события.
	w = a.SetSpreadName("s3", "Прогулка")
	if ids, _ = wireSpreads(t, w); w.Seq != 3 || w.Base != 3 || len(ids) != 0 || len(views()) != 2 {
		t.Fatalf("холостая: seq %d base %d, %v, событий %d", w.Seq, w.Base, ids, len(views()))
	}

	// Обогнанный вид (меньший rev того же альбома): не перекрывает новый, события нет.
	w = a.changed(old)
	if w.Seq != 3 || w.Base != 3 || w.Rev != a.editor().View().Rev || len(views()) != 2 {
		t.Fatalf("обогнанный: seq %d base %d rev %d, событий %d", w.Seq, w.Base, w.Rev, len(views()))
	}
	if v := a.GetView(); v.Seq != 3 || v.Spreads[2].Name != "Прогулка" {
		t.Fatalf("после обогнанного GetView: seq %d, имя %q", v.Seq, v.Spreads[2].Name)
	}

	// Отмена — шаг цепочки.
	w = a.Undo()
	if ids, _ = wireSpreads(t, w); w.Seq != 4 || w.Base != 3 || len(ids) != 1 || ids[0] != "s3" {
		t.Fatalf("отмена: seq %d base %d, %v", w.Seq, w.Base, ids)
	}

	// Вид сменился мимо цепочки — GetView отдаёт полный с новым номером.
	a.editor().SetSpreadName("s1", "Сборы")
	if v := a.GetView(); v.Seq != 5 || v.Base != 0 || v.Spreads[0].Name != "Сборы" {
		t.Fatalf("GetView после правки мимо App: seq %d base %d", v.Seq, v.Base)
	}

	// Другой альбом: первая правка — полный вид, а не дельта к прежнему.
	a.ed.Store(mustEditor(t, model.New(), ""))
	w = a.changed(a.editor().View())
	if w.Seq != 6 || w.Base != 0 {
		t.Fatalf("другой альбом: seq %d base %d", w.Seq, w.Base)
	}
	if ev := views(); len(ev) != 4 || ev[3].Seq != 6 || ev[3].Base != 0 { // GetView событий не шлёт
		t.Fatalf("событий %d", len(ev))
	}
}

// TestWireSaveAndSaved — Ctrl+S: дельта без разворотов, снимающая dirty (другие клиенты тоже);
// событие saved — {dirty, path}, а не вид (075-5).
func TestWireSaveAndSaved(t *testing.T) {
	a, views, saved := wireApp(t)
	a.GetState()
	a.SetSpreadName("s1", "Сборы")
	path := filepath.Join(a.cfgDir, "Альбом.salbum")
	w, err := a.saveAs(a.editor(), path)
	if err != nil {
		t.Fatal(err)
	}
	ids, m := wireSpreads(t, w)
	if w.Dirty || w.Base != 2 || w.Seq != 3 || len(ids) != 0 || m["dirty"] != false || m["path"] != path {
		t.Fatalf("сохранение: dirty %v base %d seq %d, %v", w.Dirty, w.Base, w.Seq, ids)
	}
	if ev := views(); len(ev) != 2 || ev[1].Dirty {
		t.Fatalf("событий view %d", len(ev))
	}
	// Повторное сохранение без правок: вид тот же — без события.
	if _, err := a.SaveProject(); err != nil {
		t.Fatal(err)
	}
	if len(views()) != 2 {
		t.Fatalf("повторное сохранение разослало вид: %d", len(views()))
	}

	a.SetSpreadName("s2", "Прогулка")
	a.flushSave()
	got := saved()
	if len(got) != 1 {
		t.Fatalf("событий saved %d", len(got))
	}
	s, ok := got[0].(Saved)
	if !ok || s.Dirty || s.Path != path {
		t.Fatalf("saved: %#v", got[0])
	}
	data, err := json.Marshal(got[0])
	if err != nil || string(data) != `{"dirty":false,"path":`+strconv.Quote(path)+`}` {
		t.Fatalf("JSON saved: %s", data)
	}
}

// viewSends — функции файла, где в вызов первым или вторым аргументом передано имя "view"
// (a.emit("view", …), wr.EventsEmit(ctx, "view", …)), и места этих вызовов.
func viewSends(t *testing.T, fset *token.FileSet, file string) (fns []string, at []token.Position) {
	t.Helper()
	f, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, arg := range call.Args[:min(2, len(call.Args))] {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Value == `"view"` {
					fns, at = append(fns, fn.Name.Name), append(at, fset.Position(call.Pos()))
				}
			}
			return true
		})
	}
	return fns, at
}

// TestViewPathsInChain — событие view шлют только функции цепочки (wire.go): вид мимо неё
// пришёл бы фронту без seq, и тот каждый раз просил бы полный (design 075 §12).
func TestViewPathsInChain(t *testing.T) {
	allowed := map[string]bool{"changed": true, "out": true, "emitState": true}
	fset := token.NewFileSet()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	more, err := filepath.Glob(filepath.Join("edit", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, file := range append(names, more...) {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		fns, at := viewSends(t, fset, file)
		found += len(fns)
		for i, fn := range fns {
			if !allowed[fn] {
				t.Errorf("%s: %s шлёт view мимо цепочки видов", at[i], fn)
			}
		}
	}
	if found < 3 {
		t.Fatalf("отправок view найдено %d: разбор не тот", found)
	}
}
