//go:build windows

package osx

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Winqa (150 Т3): сетевой диск — `net use` на административный ресурс этой же машины. Нет прав или
// службы «Сервер» — пропуск с причиной, а не провал: проба тома от этого не зависит.
func TestRemoteMappedDrive(t *testing.T) {
	letter := ""
	for _, l := range []string{"Z:", "Y:", "X:"} {
		if _, err := os.Stat(l + `\`); err != nil {
			letter = l
			break
		}
	}
	if letter == "" {
		t.Skip("нет свободной буквы диска")
	}
	drive := os.Getenv("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	share := `\\localhost\` + strings.TrimSuffix(drive, ":") + "$"
	if out, err := exec.Command("net", "use", letter, share).CombinedOutput(); err != nil {
		t.Skipf("net use %s не удалось: %v: %s", share, err, out)
	}
	t.Cleanup(func() { _ = exec.Command("net", "use", letter, "/delete", "/y").Run() })

	if !Remote(letter + `\`) {
		t.Errorf("сетевой диск %s считается локальным", letter)
	}
	if !Remote(letter + `\Windows`) {
		t.Errorf("папка на сетевом диске %s считается локальной", letter)
	}
	if Remote(drive + `\`) {
		t.Errorf("системный диск %s считается сетевым", drive)
	}
}

// UNC и \\?\ — сетевой путь без обращения к сети; тома без буквы нет.
func TestRemoteUNC(t *testing.T) {
	for _, p := range []string{`\\сервер\ресурс\Фото`, `\\?\C:\Фото`, `\\?\UNC\srv\share`, `//srv/share/x`} {
		if !Remote(p) {
			t.Errorf("%s: ждали «сетевой»", p)
		}
	}
}
