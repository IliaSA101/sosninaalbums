//go:build !windows

package osx

import (
	"path/filepath"
	"slices"
	"testing"
)

// XDG user-dirs.dirs — формат Unix (абсолютные пути вида /home/u, «$HOME»); на Windows файла нет,
// адреса отдаёт SHGetKnownFolderPath (userdirs_known_windows.go), поэтому тест только для Unix.
// parseXDGDirs: $HOME, абсолютный путь, комментарии, чужие ключи, подстановки, мусор.
func TestUserDirsXDG(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "u")
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"русская система", "# комментарий\nXDG_DESKTOP_DIR=\"$HOME/Рабочий стол\"\nXDG_DOWNLOAD_DIR=\"$HOME/Загрузки\"\nXDG_DOCUMENTS_DIR=\"$HOME/Документы\"\nXDG_MUSIC_DIR=\"$HOME/Музыка\"\n",
			[]string{home + "/Рабочий стол", home + "/Загрузки", home + "/Документы"}},
		{"абсолютный путь", "XDG_DOCUMENTS_DIR=\"/data/docs\"\n", []string{"/data/docs"}},
		{"сам $HOME", "XDG_DESKTOP_DIR=\"$HOME\"\n", []string{home}},
		{"пусто", "", nil},
		{"без кавычек", "XDG_DESKTOP_DIR=$HOME/x\n", nil},
		{"одна кавычка", "XDG_DESKTOP_DIR=\"\n", nil},
		{"относительный", "XDG_DESKTOP_DIR=\"docs\"\n", nil},
		{"чужая подстановка", "XDG_DESKTOP_DIR=\"$XDG_X/y\"\nXDG_DOWNLOAD_DIR=\"`id`\"\nXDG_DOCUMENTS_DIR=\"/a\\\\b\"\n", nil},
		{"пробелы и CRLF", "  XDG_DESKTOP_DIR = \"$HOME/D\"\r\n", []string{home + "/D"}},
		{"мусор", "\x00\xff=\n====\n\"\n", nil},
	}
	for _, c := range cases {
		var want []string
		for _, w := range c.want {
			want = append(want, filepath.FromSlash(w))
		}
		if got := parseXDGDirs([]byte(c.in), home); !slices.Equal(got, want) {
			t.Errorf("%s: %q, ждали %q", c.name, got, want)
		}
	}
}
