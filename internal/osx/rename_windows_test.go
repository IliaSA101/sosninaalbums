//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Префикс \\?\ — только длинным абсолютным путям: короткие, относительные и уже с префиксом
// не меняются; «..» чистится до префикса; UNC — через \\?\UNC\.
func TestExtPath(t *testing.T) {
	long := strings.Repeat("я", 130) // 260 байт
	cases := map[string]string{
		`C:\a\b`:                         `C:\a\b`,
		`rel\` + long:                    `rel\` + long,
		`\\?\C:\` + long:                 `\\?\C:\` + long,
		`C:\x\..\` + long:                `\\?\C:\` + long,
		`C:/` + long:                     `\\?\C:\` + long,
		`\\server\share\` + long:         `\\?\UNC\server\share\` + long,
		`\\.\pipe\` + long:               `\\.\pipe\` + long,
		`C:\` + strings.Repeat("a", 244): `C:\` + strings.Repeat("a", 244), // 247 — ещё без префикса
	}
	for in, want := range cases {
		if got := extPath(in); got != want {
			t.Errorf("extPath(%.20q…) = %.30q…, ждали %.30q…", in, got, want)
		}
	}
}

// Назначение длиннее MAX_PATH переименовывается и при выключенных длинных путях Windows.
func TestRenameNoReplaceLongPath(t *testing.T) {
	dir := testkit.TempDir(t)
	deep := dir
	for len(deep) < 300 {
		deep = filepath.Join(deep, strings.Repeat("п", 40))
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	from, to := filepath.Join(dir, "a"), filepath.Join(deep, "b.jpg")
	if err := os.WriteFile(from, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RenameNoReplace(from, to); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(to); err != nil || string(got) != "a" {
		t.Fatalf("после переименования: %q %v", got, err)
	}
	back := filepath.Join(dir, "c")
	if err := RenameNoReplace(to, back); err != nil {
		t.Fatalf("из длинного пути: %v", err)
	}
}
