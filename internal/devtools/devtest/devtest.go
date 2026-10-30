// Package devtest — общие помощники тестов devtools. testkit сюда нельзя (depguard: devtools не
// тянет internal), поэтому здесь копия двух его приёмов; в программу пакет не входит.
package devtest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TempDir — как t.TempDir(), но в os.TempDir(), а не в GOTMPDIR внутри модуля: иначе имя папки
// каждый раз новое и go test не кэширует пакет. prefix — начало имени папки.
func TempDir(t testing.TB, prefix string) string {
	t.Helper()
	dir, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// SelfExe — символическая ссылка на тестовый бинарь вне модуля (приём TestHelperProcess): запуск
// самого бинаря из .gotmp иначе попадает в ключ кэша go test. Копия testkit.SelfExe; ветка Windows
// та же — ссылка без .exe там не запускается (страж — TestSelfExeWindowsBranch).
func SelfExe(t testing.TB) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return os.Args[0]
	}
	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		return os.Args[0]
	}
	link := filepath.Join(TempDir(t, "selfexe"), "selfexe")
	if os.Symlink(exe, link) != nil {
		return os.Args[0]
	}
	return link
}

// Lite — тяжёлый тест растра берёт урезанный набор: под -race или -short (быстрый уровень: precheck и
// цикл исполнителя). Копия testkit.Lite; полный набор гоняют check.ps1 и CI без -short.
func Lite() bool { return Race || testing.Short() }
