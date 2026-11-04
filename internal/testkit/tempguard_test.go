package testkit

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Страж: в тестах нет t.TempDir() (и b/f/tb.TempDir()) — только testkit.TempDir. Go кладёт
// t.TempDir в GOTMPDIR, а это .gotmp внутри модуля: имя папки каждый раз новое, go test
// записывает открытый внутри модуля файл в ключ кэша, и пакет не кэшируется никогда
// (ревизия тестов 24.10, С1). testkit.TempDir берёт os.TempDir() вне модуля и ещё удаляет с повтором.
func TestNoStdTempDir(t *testing.T) {
	root := filepath.Join("..") // internal/
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(src, []byte("TempDir()")) {
			return err
		}
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if p := stdTempDirCall(n); p.IsValid() {
				t.Errorf("%s: t.TempDir() — замените на testkit.TempDir(t) (иначе пакет не кэшируется go test)", fset.Position(p))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// stdTempDirCall — позиция вызова X.TempDir() без аргументов, где X — не пакет os.
func stdTempDirCall(n ast.Node) token.Pos {
	call, ok := n.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return token.NoPos
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "TempDir" {
		return token.NoPos
	}
	if x, ok := sel.X.(*ast.Ident); ok && x.Name == "os" {
		return token.NoPos // os.TempDir() — корень временных папок, не папка теста
	}
	return call.Pos()
}

// Сам разбор: t.TempDir/b.TempDir/f.TempDir ловятся, os.TempDir и testkit.TempDir(t) — нет.
func TestStdTempDirCall(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"t.TempDir()", true},
		{"tb.TempDir()", true},
		{"b.TempDir()", true},
		{"f.TempDir()", true},
		{"s.t.TempDir()", true},
		{"os.TempDir()", false},
		{"testkit.TempDir(t)", false},
		{"TempDir(t)", false},
		{"x.Dir()", false},
	}
	for _, c := range cases {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nvar _ = "+c.src, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		got := false
		ast.Inspect(f, func(n ast.Node) bool {
			if stdTempDirCall(n).IsValid() {
				got = true
			}
			return true
		})
		if got != c.want {
			t.Errorf("%s: поймано %v, ждали %v", c.src, got, c.want)
		}
	}
}
