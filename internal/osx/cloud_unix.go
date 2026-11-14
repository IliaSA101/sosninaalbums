//go:build !windows

package osx

// Облака на Linux и macOS (160, design §4): Яндекс.Диск — config.cfg (Linux) и папка по умолчанию (macOS). Виртуальных
// облачных дисков здесь нет: macOS отдаёт облака через ~/Library/CloudStorage (корни знает agentplaces), а заглушки
// узнаёт SF_DATALESS в обходе.

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// openCloudFile — открытие файла настроек без ссылки и без ожидания писателя: подмена файла на FIFO между Lstat и
// открытием не повесит чтение.
func openCloudFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

func cloudRootsOS(home string, local func(string) error) []string {
	if !filepath.IsAbs(home) {
		return nil
	}
	switch runtime.GOOS {
	case "linux":
		if data, ok := readCloudFile(filepath.Join(home, ".config", "yandex-disk", "config.cfg"), local); ok {
			return parseYandexConfig(data)
		}
	case "darwin":
		// Яндекс.Диск для macOS кладёт папку сюда; в настройках её пути нет. Берём, только если это папка.
		dir := filepath.Join(home, "Yandex.Disk.localized")
		if st, err := os.Lstat(dir); err == nil && st.IsDir() {
			return []string{dir}
		}
	}
	return nil
}

// VirtualCloud — на этих ОС виртуальных облачных дисков нет (признак — только Windows).
func VirtualCloud(string) bool { return false }
