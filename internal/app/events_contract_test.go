package app

// Страж контракта событий Go → UI: число аргументов в a.emit(...) / EventsEmit(...) против
// колонки «Данные» таблицы docs/architecture/events.md. Фронт слушает по позициям
// (done, total), поэтому один объект вместо двух чисел ломает окно (open:progress, QA 0.18).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// undocumentedEvents — события, которых ещё нет в таблице events.md (слушает PacksDialog).
// Убрать запись, когда строка появится в таблице: тест сам напомнит.
var undocumentedEvents = map[string]bool{}

// appSources — файлы кода (без тестов) internal/app и всех подпакетов, кроме testdata: службы
// (updater, assoc, exporter) шлют события и вид через поля Host, и страж смотрит и туда.
func appSources(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "testdata":
			return filepath.SkipDir
		case !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go"):
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// emitsIn добавляет в out отправки событий из f: a.emit(name, …), s.h.Emit(name, …) (поле Host
// службы) и wr.EventsEmit(ctx, name, …). Имя не литералом (обёртка a.emit в app.go) и `data...`
// пропускаются.
func emitsIn(f *ast.File, out map[string][]int) error {
	var bad error
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || call.Ellipsis != token.NoPos {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		first := 0 // индекс аргумента-имени: a.emit(name, …) / wr.EventsEmit(ctx, name, …)
		switch sel.Sel.Name {
		case "emit", "Emit":
		case "EventsEmit":
			first = 1
		default:
			return true
		}
		if len(call.Args) <= first {
			return true
		}
		lit, ok := call.Args[first].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		name, err := strconv.Unquote(lit.Value)
		if err != nil {
			bad = err
			return false
		}
		out[name] = append(out[name], len(call.Args)-first-1)
		return true
	})
	return bad
}

// emittedArity разбирает исходники internal/app/** (без тестов): имя события → число данных в
// каждой отправке.
func emittedArity(t *testing.T) map[string][]int {
	t.Helper()
	fset := token.NewFileSet()
	out := map[string][]int{}
	for _, file := range appSources(t) {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := emitsIn(f, out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

// До разреза (Ш0) в коде было 27 событий и 36 отправок; разрез переносит отправки в службы и
// не должен их терять из виду страж.
const minEventNames, minEventSends = 27, 36

// TestEmitsSeenThroughHost — самопроверка стража: вызов поля Host.Emit службы виден, как и a.emit,
// а сам разбор не стал слепее, чем до разреза.
func TestEmitsSeenThroughHost(t *testing.T) {
	src := `package x
func (s *S) f() {
	s.h.Emit("a:one", 1, 2)
	s.h.Emit("a:two")
	s.h.Emit(name, 1)
	s.h.Emit("a:spread", xs...)
	a.emit("a:one", 3, 4)
	wr.EventsEmit(ctx, "a:three", 5)
}`
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]int{}
	if err := emitsIn(f, got); err != nil {
		t.Fatal(err)
	}
	want := map[string][]int{"a:one": {2, 2}, "a:two": {0}, "a:three": {1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("разобрано %v, ждали %v", got, want)
	}
	fact := emittedArity(t)
	sends := 0
	for _, v := range fact {
		sends += len(v)
	}
	if len(fact) < minEventNames || sends < minEventSends {
		t.Fatalf("событий %d, отправок %d: меньше, чем до разреза (%d и %d)", len(fact), sends, minEventNames, minEventSends)
	}
}

// dataArity — число значений в ячейке «Данные»: верхнеуровневые запятые вне скобок и обратных кавычек.
func dataArity(cell string) int {
	cell = strings.TrimSpace(strings.ReplaceAll(cell, "`", ""))
	if cell == "" {
		return 0
	}
	n, depth := 1, 0
	for _, r := range cell {
		switch r {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case ',':
			if depth == 0 {
				n++
			}
		}
	}
	return n
}

// documentedArity читает строки таблицы events.md: имя события → ожидаемое число данных.
func documentedArity(t *testing.T) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "architecture", "events.md"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for line := range strings.SplitSeq(string(raw), "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(line, "|") // ячейка «Когда» может содержать `|` только в коде — до неё не доходим
		if len(cells) < 4 {
			t.Fatalf("строка таблицы без трёх колонок: %.60s", line)
		}
		out[strings.Trim(strings.TrimSpace(cells[1]), "`")] = dataArity(cells[2])
	}
	return out
}

func TestDataArity(t *testing.T) {
	cases := map[string]int{
		"`text`": 1, "`done, total`": 2, "`[]Photo`": 1, "`{installed: string[]}`": 1,
		"`dir, err, note, memo, stopped`": 5, "": 0,
	}
	for in, want := range cases {
		if got := dataArity(in); got != want {
			t.Errorf("dataArity(%q) = %d, ждали %d", in, got, want)
		}
	}
}

// TestEventsContract: каждое событие из кода — в таблице (или в списке известных пропусков) с тем же
// числом аргументов; каждая строка таблицы где-то шлётся (устаревшая строка — тоже расхождение).
func TestEventsContract(t *testing.T) {
	sent, doc := emittedArity(t), documentedArity(t)
	if len(sent) < 10 || len(doc) < 10 { // разбор не должен молча ничего не находить
		t.Fatalf("разобрано событий: в коде %d, в таблице %d — разбор сломан", len(sent), len(doc))
	}
	names := make([]string, 0, len(sent))
	for name := range sent {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		want, ok := doc[name]
		switch {
		case !ok && undocumentedEvents[name]:
		case !ok:
			t.Errorf("событие %q шлётся, но его нет в events.md", name)
		case undocumentedEvents[name]:
			t.Errorf("событие %q уже в events.md — убрать из undocumentedEvents", name)
		default:
			for _, got := range sent[name] {
				if got != want {
					t.Errorf("событие %q: в коде %d аргументов, в events.md «Данные» — %d", name, got, want)
				}
			}
		}
	}
	for name := range doc {
		if _, ok := sent[name]; !ok {
			t.Errorf("событие %q есть в events.md, но в коде не шлётся", name)
		}
	}
	for name := range undocumentedEvents {
		if _, ok := sent[name]; !ok {
			t.Errorf("undocumentedEvents: %q больше не шлётся — убрать", name)
		}
	}
}
