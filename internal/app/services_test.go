package app

// Страж служб App (126-4): фон службы запускается только через Host.GoBG (там WaitGroup App,
// закрытие и guard), поэтому в коде updater, assoc, exporter нет ни `go`, ни time.AfterFunc
// (таймер, сработавший после закрытия, жил бы дольше App).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serviceDirs — папки служб разреза 126 (Ш1–Ш3). Нет папки — шаг ещё не сделан, страж её пропускает.
var serviceDirs = []string{"updater", "assoc", "exporter"}

// minServices — сколько служб должно быть найдено: поднимается с каждым шагом (Ш1 → 1, Ш2 → 2,
// Ш3 → 3), чтобы страж не «ослеп» молча после переноса или переименования папки.
const minServices = 3

// bareGoExceptions — файл службы → причина, по которой в нём разрешён голый go/AfterFunc. Пусто.
var bareGoExceptions = map[string]string{}

// bareGoIn — места `go`-операторов и вызовов time.AfterFunc в f.
func bareGoIn(fset *token.FileSet, f *ast.File) (at []token.Position) {
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			at = append(at, fset.Position(x.Pos()))
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "AfterFunc" {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" {
					at = append(at, fset.Position(x.Pos()))
				}
			}
		}
		return true
	})
	return at
}

func TestServicesNoBareGo(t *testing.T) {
	fset := token.NewFileSet()
	services := 0
	for _, dir := range serviceDirs {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		services++
		for _, en := range entries {
			name := en.Name()
			if en.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			if _, ok := bareGoExceptions[path]; ok {
				continue
			}
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, at := range bareGoIn(fset, f) {
				t.Errorf("%s: go или time.AfterFunc в службе; фон — только через Host.GoBG", at)
			}
		}
	}
	if services < minServices {
		t.Fatalf("найдено служб %d, ждали не меньше %d: папка переименована?", services, minServices)
	}
}

// Самопроверка: страж видит go-оператор (в функции, замыкании, методе) и time.AfterFunc, но не
// путает с ними `go` в строке, имя AfterFunc у чужого пакета и обычные вызовы.
func TestBareGoGuardCatches(t *testing.T) {
	src := `package x
import "time"
func a() { go b() }
func (s *S) c() { f := func() { go func() {}() }; f() }
func d() { time.AfterFunc(time.Second, func() {}) }
func e() { s := "go b()"; other.AfterFunc(1); s.h.GoBG(func() {}); _ = s }`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := bareGoIn(fset, f); len(got) != 3 {
		t.Fatalf("найдено %d мест, ждали 3 (go в a, go в замыкании c, AfterFunc в d): %v", len(got), got)
	}
}
