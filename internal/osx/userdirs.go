package osx

import (
	"os"
	"path/filepath"
)

// Подмена в тестах: os.UserConfigDir/UserCacheDir зависят от среды машины.
var (
	userConfigDir = os.UserConfigDir
	userCacheDir  = os.UserCacheDir
)

// ConfigDir — папка настроек пользователя (Windows %AppData%); "" — её нет: ошибка os.UserConfigDir
// (нет HOME, XDG_CONFIG_HOME или APPDATA) или путь не абсолютный. Join с пустой или относительной
// базой дал бы путь от текущей папки процесса, поэтому вызывающий выключает функцию, а не пишет туда.
func ConfigDir() string { return absDir(userConfigDir) }

// CacheDir — папка кэша пользователя (Windows %LOCALAPPDATA%); "" — как у ConfigDir.
func CacheDir() string { return absDir(userCacheDir) }

// absDir — результат f, если это абсолютный путь без ошибки; иначе "".
func absDir(f func() (string, error)) string {
	d, err := f()
	if err != nil || !filepath.IsAbs(d) {
		return ""
	}
	return d
}
