package paths

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// logCalls — функции записи в log.txt: log.Print*, logRare (app), xl.printf (library).
var logCalls = map[string]bool{"Printf": true, "Print": true, "Println": true, "printf": true, "logRare": true}

// logNameOK — места, где имя файла в логе — своё, не пользователя: templates.bad-<время>.json.
var logNameOK = map[string]bool{"prefs/templates.go": true}

// Страж 073-7: в аргументах записи в лог нет filepath.Base(…) и поля или метода Name — имена
// фото, альбомов и папок в log.txt не пишем; вид файла — через LogExt.
func TestLogNoNames(t *testing.T) {
	root := filepath.Join("..")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isLogCall(call) {
				return true
			}
			for _, arg := range call.Args {
				if bad := nameIn(arg); bad != "" && !logNameOK[rel] {
					t.Errorf("%s: имя файла в логе (%s) — пишите paths.LogExt", fset.Position(call.Pos()), bad)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func isLogCall(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == "logRare"
	case *ast.SelectorExpr:
		x, ok := fn.X.(*ast.Ident)
		return ok && logCalls[fn.Sel.Name] && (x.Name == "log" || x.Name == "xl")
	}
	return false
}

// nameIn — "filepath.Base" или ".Name" внутри выражения, кроме обёрнутых в LogExt.
func nameIn(e ast.Expr) (bad string) {
	ast.Inspect(e, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if sel, ok := v.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "LogExt" {
				return false
			}
		case *ast.SelectorExpr:
			if x, ok := v.X.(*ast.Ident); ok && x.Name == "filepath" && v.Sel.Name == "Base" {
				bad = "filepath.Base"
			}
			if v.Sel.Name == "Name" {
				bad = ".Name"
			}
		}
		return bad == ""
	})
	return bad
}
