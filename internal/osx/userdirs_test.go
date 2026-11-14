package osx

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ConfigDir и CacheDir: абсолютный путь проходит, пустой, относительный и ошибка дают "".
func TestUserDirs(t *testing.T) {
	abs := testkit.TempDir(t)
	boom := errors.New("нет HOME")
	cases := []struct {
		name string
		dir  string
		err  error
		want string
	}{
		{"абсолютный", abs, nil, abs},
		{"пусто", "", nil, ""},
		{"относительный", filepath.Join("a", "b"), nil, ""},
		{"точка", ".", nil, ""},
		{"ошибка", abs, boom, ""}, // путь при ошибке не доверяем
		{"ошибка и пусто", "", boom, ""},
	}
	oldC, oldK := userConfigDir, userCacheDir
	t.Cleanup(func() { userConfigDir, userCacheDir = oldC, oldK })
	for _, c := range cases {
		userConfigDir = func() (string, error) { return c.dir, c.err }
		userCacheDir = func() (string, error) { return c.dir, c.err }
		if got := ConfigDir(); got != c.want {
			t.Errorf("%s: ConfigDir() = %q, ждали %q", c.name, got, c.want)
		}
		if got := CacheDir(); got != c.want {
			t.Errorf("%s: CacheDir() = %q, ждали %q", c.name, got, c.want)
		}
	}
}

// Конфиг и кэш независимы: ошибка одного не гасит другой.
func TestUserDirsIndependent(t *testing.T) {
	abs := testkit.TempDir(t)
	oldC, oldK := userConfigDir, userCacheDir
	t.Cleanup(func() { userConfigDir, userCacheDir = oldC, oldK })
	userConfigDir = func() (string, error) { return "", errors.New("нет") }
	userCacheDir = func() (string, error) { return abs, nil }
	if ConfigDir() != "" || CacheDir() != abs {
		t.Fatalf("ConfigDir=%q CacheDir=%q", ConfigDir(), CacheDir())
	}
}
