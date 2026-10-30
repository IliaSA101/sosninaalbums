//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Winqa (153 Т2): том системного диска читается, корень и Rel сходятся, обратный поиск по ID
// возвращает тот же путь; сетевой путь и пустой слот — ошибка, а не пустой ID.
func TestVolIDWindows(t *testing.T) {
	win := os.Getenv("SystemRoot")
	if win == "" {
		t.Fatal("SystemRoot не задан")
	}
	v, err := VolumeOf(win)
	if err != nil {
		t.Fatalf("VolumeOf(%s): %v", win, err)
	}
	if !regexp.MustCompile(`^win:[0-9a-f]{8}:[A-Za-z0-9]+$`).MatchString(v.ID) {
		t.Errorf("ID = %q, want win:<hex8>:<ФС>", v.ID)
	}
	if !strings.EqualFold(v.Root, filepath.VolumeName(win)+`\`) {
		t.Errorf("Root = %q, want корень %s", v.Root, filepath.VolumeName(win))
	}
	if want := strings.Trim(filepath.ToSlash(win[len(filepath.VolumeName(win)):]), "/"); !strings.EqualFold(v.Rel, want) {
		t.Errorf("Rel = %q, want %q", v.Rel, want)
	}
	// Другой регистр букв — тот же том.
	if again, err := VolumeOf(strings.ToLower(win)); err != nil || again.ID != v.ID {
		t.Errorf("нижний регистр: %+v, %v", again, err)
	}
	// Папки нет — том известен.
	if got, err := VolumeOf(filepath.Join(win, "нет-такой-папки-vol")); err != nil || got.ID != v.ID {
		t.Errorf("несуществующая папка: %+v, %v", got, err)
	}
	// Обратный поиск.
	p, _, err := VolumePath(v.ID, v.Rel)
	if err != nil || !strings.EqualFold(p, win) {
		t.Errorf("VolumePath(%q, %q) = %q, %v; want %q", v.ID, v.Rel, p, err, win)
	}
	if _, _, err := VolumePath("win:00000000:ntfs", ""); err == nil {
		t.Error("несуществующий том найден")
	}
	// Сетевые пути: привязки нет.
	for _, unc := range []string{`\\localhost\C$\Windows`, `\\?\C:\Windows`, "//localhost/C$/x"} {
		if v, err := VolumeOf(unc); err == nil {
			t.Errorf("VolumeOf(%s) = %+v, want ошибка", unc, v)
		}
	}
}

// Winqa (153 Т2): режим ошибок процесса содержит SEM_FAILCRITICALERRORS — пустой слот картридера
// отвечает ERROR_NOT_READY без системного окна «Вставьте диск». Go ставит режим сам; тест ловит
// случай, когда WebView2 или другая библиотека его сняла.
func TestVolErrorMode(t *testing.T) {
	proc := kernel32.NewProc("GetErrorMode")
	if err := proc.Find(); err != nil {
		t.Skipf("GetErrorMode нет: %v", err)
	}
	mode, _, _ := proc.Call()
	const semFailCriticalErrors = 0x0001
	if mode&semFailCriticalErrors == 0 {
		t.Errorf("GetErrorMode = %#x: SEM_FAILCRITICALERRORS снят, пустой слот покажет окно", mode)
	}
	// Все буквы опрашиваются без окна и зависания: ошибка пустого слота — обычный ответ.
	for i := range 26 {
		_, _, _, _ = volInfo(string(rune('A'+i)) + `:\`)
	}
}
