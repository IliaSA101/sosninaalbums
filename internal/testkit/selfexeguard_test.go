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

// Страж: тест не берёт путь к своему бинарю сам (os.Args[0], os.Executable) — только
// testkit.SelfExe. Ссылка os.Symlink(exe, ".../selfexe") без .exe на Linux работает, а на Windows
// нет: так CI уронил sheetcut (24.10). На Linux это ловится только разбором исходников.
func TestNoOwnBinaryLink(t *testing.T) {
	fset := token.NewFileSet()
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil || !bytes.Contains(src, []byte("os.")) {
			return err
		}
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		var first token.Pos
		ast.Inspect(f, func(n ast.Node) bool {
			if !first.IsValid() && ownBinaryRef(n) {
				first = n.Pos()
			}
			return true
		})
		if !first.IsValid() {
			return nil
		}
		t.Errorf("%s: путь к своему бинарю — только testkit.SelfExe(t) (на Windows ссылка без .exe не запускается; в devtools — devtest.SelfExe)", fset.Position(first))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ownBinaryRef — os.Args[...] с индексом 0 или вызов os.Executable().
func ownBinaryRef(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.IndexExpr:
		lit, ok := v.Index.(*ast.BasicLit)
		return ok && lit.Value == "0" && isOs(v.X, "Args")
	case *ast.SelectorExpr:
		return v.Sel.Name == "Executable" && isOs(v, "Executable")
	}
	return false
}

func isOs(e ast.Expr, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	return ok && x.Name == "os"
}

// Сам разбор: os.Args[0] и os.Executable ловятся, чужие Args и os.Args без индекса — нет.
func TestOwnBinaryRef(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"os.Args[0]", true},
		{"os.Executable", true},
		{"len(os.Args)", false},
		{"call.Args[0]", false},
		{"filepath.Executable", false},
	}
	for _, c := range cases {
		f, err := parser.ParseFile(token.NewFileSet(), "x.go", "package x\nvar _ = "+c.src, 0)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		got := false
		ast.Inspect(f, func(n ast.Node) bool {
			if ownBinaryRef(n) {
				got = true
			}
			return true
		})
		if got != c.want {
			t.Errorf("%s: поймано %v, ждали %v", c.src, got, c.want)
		}
	}
}
