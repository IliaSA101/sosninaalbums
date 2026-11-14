package testkit

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// SelfExe — путь, по которому тест запускает сам тестовый бинарь (приём TestHelperProcess).
// Зачем не os.Args[0]: бинарь лежит в GOTMPDIR внутри модуля, а запуск по этому пути попадает в
// журнал открытых файлов go test и ключ кэша меняется каждый раз («(cached)» не бывает).
// Ссылка во временной папке теста ведёт туда же, но лежит вне модуля. Windows без прав на ссылки
// (и любая неудача) — честный os.Args[0]: кэш не работает, но тест тот же.
func SelfExe(t testing.TB) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return os.Args[0]
	}
	exe, err := filepath.Abs(os.Args[0])
	if err != nil {
		return os.Args[0]
	}
	link := filepath.Join(TempDir(t), "selfexe")
	if err := os.Symlink(exe, link); err != nil {
		return os.Args[0]
	}
	return link
}
