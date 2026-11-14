package osx

import (
	"os"
	"runtime"
	"testing"
)

// Страж CI: сборка macOS без cgo берёт volid_other.go, и тесты на образах (volid_darwin_test.go, тег cgo)
// молча исчезают из прогона — зелёный go-mac ничего бы не доказывал.
func TestVolidCgoOnMacCI(t *testing.T) {
	if runtime.GOOS == "darwin" && os.Getenv("CI") != "" && !volidCgo {
		t.Fatal("macOS в CI собран без cgo: тесты тома на образах не запускаются (нужен CGO_ENABLED=1)")
	}
}
