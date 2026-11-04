package album_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestLayoutInOnly — страж 104: пару «пропорции + Params» для раскладки разворота собирает только
// Project.LayoutIn (Lock — свои кадры, без него подрезка тронула бы кадр, выбранный фотографом).
// Провал: в одной строке рабочего кода есть Aspects( и .Params(), либо вызов layout.Build/Cached/…
// получает .Params() прямо. Исключения — с причиной; Params() без раскладки разворота (Pick,
// Describe, сравнение настроек) не трогается.
func TestLayoutInOnly(t *testing.T) {
	root := filepath.Join("..") // internal/
	call := regexp.MustCompile(`layout\.(Build|Cached|CachedRearranged|CachedPages|CachedPagesInOrder)\(`)
	except := map[string]string{} // файл → причина; сейчас исключений нет
	var bad []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "layout/") || except[rel] != "" || strings.Contains(rel, "testdata/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			code, _, _ := strings.Cut(line, "//")
			if strings.Contains(code, ".Aspects(") && strings.Contains(code, ".Params()") ||
				call.MatchString(code) && strings.Contains(code, ".Params()") {
				bad = append(bad, rel+":"+strings.TrimSpace(code)+" (строка "+strconv.Itoa(i+1)+")")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("раскладку разворота собирает Project.LayoutIn, а не пара Aspects + Params():\n%s", strings.Join(bad, "\n"))
	}
}
