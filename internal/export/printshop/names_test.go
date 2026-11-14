package printshop

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// TestPrintNamesNoPath (design §6.4, §8): имена файлов выгрузки из префикса профиля — только имя в
// папке книги: Base(n) == n, без «..», не устройство Windows. Префиксы — у всех встроенных и
// любые, которые пропускает разбор профиля (кандидаты с путём и устройствами разбор отвергает).
func TestPrintNamesNoPath(t *testing.T) {
	base, _ := printprof.Builtin().ByID("fabrika")
	prefixes := []string{"", "1-", "_", "-", "CON", "NUL", "COM1", "LPT1-", "aux_", "12345678", "Zz-_09"}
	for _, bad := range []string{"..", "../", "a/b", `a\b`, "C:", "1-.", ".", "con.", "123456789", "й", " "} {
		p := base
		e := *p.Export
		e.Prefix = bad
		p.Export = &e
		if p.Validate() == nil {
			prefixes = append(prefixes, bad) // пропущенный разбором — тоже под проверкой имён
		}
	}
	for _, p := range printprof.Builtin().Profiles() {
		if p.Export != nil {
			prefixes = append(prefixes, p.Export.Prefix)
		}
	}
	for _, pre := range prefixes {
		pf := Profile{Prefix: pre}
		for _, total := range []int{1, 9, 99, 100, 200} {
			names := []string{pf.CoverName()}
			for i := range total {
				names = append(names, export.Job{Index: i, Whole: true, Left: true, Right: true, Prefix: pre}.Names(total)...)
			}
			for _, n := range names {
				if filepath.Base(n) != n || strings.Contains(n, "..") || paths.Device(n) || !filepath.IsLocal(n) {
					t.Fatalf("префикс %q: имя %q не годится в папку книги", pre, n)
				}
			}
		}
	}
}
