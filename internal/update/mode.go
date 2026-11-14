package update

import (
	"os"
	"strings"
)

// Mode — как программа обновляется на этой машине (design §3).
type Mode string

const (
	// ModeInstall — установленная установщиком Windows-копия: обновляется тихо, на месте.
	ModeInstall Mode = "install"
	// ModeSetup — переносная Windows-копия (zip): обновление — обычный мастер установки.
	ModeSetup Mode = "setup"
	// ModePage — Mac, Linux, а также выпуск, который автообновление не переживёт: страница выпуска.
	ModePage Mode = "page"
)

// installedExe — имя exe установленной копии (в zip оно с версией, 0.25-q111-11).
const installedExe = "SosninaAlbums.exe"

// ModeOf — режим по ОС и пути запущенного exe: не Windows — page; рядом uninstall.exe и имя
// SosninaAlbums.exe (регистр не важен) — install; иначе setup. Реестр не читается: файл рядом
// надёжнее при переносе папки (design §4).
func ModeOf(goos, exe string) Mode {
	return modeOf(goos, exe, func(p string) bool {
		st, err := os.Lstat(p)
		return err == nil && st.Mode().IsRegular()
	})
}

// modeOf — ModeOf с подставляемой проверкой файла (тесты).
func modeOf(goos, exe string, isFile func(path string) bool) Mode {
	if goos != "windows" {
		return ModePage
	}
	i := strings.LastIndexAny(exe, `\/`) + 1 // exe может быть в виде Windows и в тесте на другой ОС
	if strings.EqualFold(exe[i:], installedExe) && isFile(exe[:i]+"uninstall.exe") {
		return ModeInstall
	}
	return ModeSetup
}
