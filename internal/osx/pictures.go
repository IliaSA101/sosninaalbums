package osx

import (
	"os"
	"path/filepath"
)

// PicturesDir — папка «Изображения» пользователя ("" — домашней нет): XDG_PICTURES_DIR на Linux,
// иначе ~/Pictures. Нужна окну ОС как начальная папка; безопасность от неё не зависит (на Windows
// «Изображения» в OneDrive окно просто откроется в ~/Pictures, человек перейдёт сам).
func PicturesDir() string {
	home, _ := os.UserHomeDir()
	if !filepath.IsAbs(home) {
		return ""
	}
	if cfg := ConfigDir(); cfg != "" {
		for _, p := range readXDGDirs(filepath.Join(cfg, "user-dirs.dirs"), home, []string{"XDG_PICTURES_DIR"}) {
			return filepath.Clean(p)
		}
	}
	return filepath.Join(home, "Pictures")
}
