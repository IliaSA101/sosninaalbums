package testkit

import (
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// SIMDPath согласован с настоящей сборкой: только amd64 без тега noasm.
func TestSIMDPath(t *testing.T) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("нет сборочной информации")
	}
	noasm := false
	for _, s := range bi.Settings {
		if s.Key == "-tags" {
			for _, tag := range strings.Split(s.Value, ",") {
				noasm = noasm || tag == "noasm"
			}
		}
	}
	if want := runtime.GOARCH == "amd64" && !noasm; SIMDPath != want {
		t.Errorf("SIMDPath = %v, ждали %v (GOARCH %s, noasm %v)", SIMDPath, want, runtime.GOARCH, noasm)
	}
}
