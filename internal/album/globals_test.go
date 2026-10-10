package album_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// В рабочем коде дерева альбома нет экспортных переменных уровня пакета, кроме ошибок Err*:
// крючок теста, ставший экспортным, дал бы app/export менять поведение ядра в обход Editor
// (023 Ш4з). Подмена в тестах — строчная переменная и доступ через export_test.go, inside
// или сеттер модели (forbidigo пускает его только в тесты). internal/inside — сам механизм
// доступа (поля-функции ставит ядро в init), internal/edtest — только для тестов.
func TestNoExportedGlobals(t *testing.T) {
	skip := map[string]bool{"testdata": true, "inside": true, "edtest": true}
	fset := token.NewFileSet()
	var bad []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, name := range exportedVars(f) {
			bad = append(bad, filepath.ToSlash(path)+": "+name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("экспортные переменные пакета (нужна строчная + доступ для тестов):\n%s", strings.Join(bad, "\n"))
	}
}

// exportedVars — экспортные переменные уровня пакета в файле, кроме ошибок Err*.
func exportedVars(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				if name.IsExported() && !strings.HasPrefix(name.Name, "Err") {
					out = append(out, name.Name)
				}
			}
		}
	}
	return out
}
