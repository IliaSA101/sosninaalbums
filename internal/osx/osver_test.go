package osx

import (
	"regexp"
	"runtime"
	"testing"
)

// Версия ОС этой машины: darwin — «major.minor[.patch]» с major ≥ 12, windows — «10.0.<сборка>»,
// прочие — false (правила для Linux нет, 171-10).
func TestOSVersionReal(t *testing.T) {
	v, ok := OSVersion()
	switch runtime.GOOS {
	case "darwin":
		if !ok || !regexp.MustCompile(`^(1[2-9]|[2-9]\d)\.\d+(\.\d+)?$`).MatchString(v) {
			t.Fatalf("macOS: %q %v", v, ok)
		}
	case "windows":
		if !ok || !regexp.MustCompile(`^10\.0\.\d+$`).MatchString(v) {
			t.Fatalf("Windows: %q %v", v, ok)
		}
	default:
		if v != "" || ok {
			t.Fatalf("%s: ждали пусто, получили %q %v", runtime.GOOS, v, ok)
		}
		t.Skip("правило minOS только для macOS и Windows (171-10): на " + runtime.GOOS + " версия не отдаётся")
	}
}
