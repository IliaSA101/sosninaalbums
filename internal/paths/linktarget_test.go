package paths

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

// LinkTargetLocal: пустая цель и нулевой байт — отказ; абсолютная — по CheckLocalPath; относительная — без устройства в
// элементах (только Windows) и из локальной папки; на Windows цель с корнем или двоеточием без диска (`\x`, `C:x`) идёт по CheckLocalPath,
// а не считается относительной. Правила — ОС процесса.
func TestLinkTargetLocal(t *testing.T) {
	win := runtime.GOOS == "windows"
	root := string(filepath.Separator) + "photos"
	unc := "//nosuchhost/share/x.jpg"
	if win {
		root = `C:\photos`
		unc = `\\nosuchhost\share\x.jpg`
	}
	abs := filepath.Join(root, "x.jpg")
	cases := []struct {
		name, dir, target string
		want              bool
	}{
		{"пустая цель", root, "", false},
		{"нулевой байт", root, "a\x00.jpg", false},
		{"абсолютная локальная", root, abs, true},
		{"абсолютная сетевая", root, unc, false},
		{"относительная рядом", root, "x.jpg", true},
		{"относительная вверх", root, filepath.Join("..", "x.jpg"), true},
		{"относительная в подпапку", root, filepath.Join("sub", "x.jpg"), true},
		{"устройство в элементе", root, filepath.Join("sub", "CON", "x.jpg"), !win},
		{"устройство без расширения", root, "NUL", !win},
	}
	if win {
		cases = append(cases, []struct {
			name, dir, target string
			want              bool
		}{
			{"корень без диска", root, `\x.jpg`, false},
			{"диск без корня", root, `C:x.jpg`, false},
			{"поток NTFS", root, `x.jpg:поток`, false},
			{"папка ссылки сетевая", `\\nosuchhost\share`, "x.jpg", false},
			{"папка ссылки относительная", "photos", "x.jpg", false},
		}...)
	}
	for _, c := range cases {
		if got := LinkTargetLocal(c.dir, c.target); got != c.want {
			t.Errorf("%s: LinkTargetLocal(%q, %q) = %v, ожидалось %v", c.name, c.dir, c.target, got, c.want)
		}
	}
}

// LinkChainLocal: относительная запись склеивается с папкой ссылки (цепочка идёт дальше по ней), ссылка с пустой записью
// и с устройством — отказ, ошибка чтения посреди цепочки — конец (true). Ровно MaxLinkHops звеньев — отказ.
func TestLinkChainLocalRelative(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "photos")
	if runtime.GOOS == "windows" {
		root = `C:\photos`
	}
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	links := map[string]string{}
	rl := func(p string) (string, error) {
		if v, ok := links[p]; ok {
			return v, nil
		}
		return "", errors.New("не ссылка")
	}
	links[a], links[b] = "b", "c" // относительные записи: a → b → c, c — не ссылка
	if !LinkChainLocal(a, rl) {
		t.Error("цепочка относительных ссылок до файла должна пройти")
	}
	links[b] = "" // пустая запись
	if LinkChainLocal(a, rl) {
		t.Error("пустая запись ссылки пропущена")
	}
	links[b] = "CON"
	if runtime.GOOS == "windows" && LinkChainLocal(a, rl) { // устройства есть только на Windows
		t.Error("устройство в записи ссылки пропущено")
	}
	// Цепочка ровно из MaxLinkHops ссылок: после последней звена-файла не проверяем — отказ как у цикла.
	long := map[string]string{}
	prev := a
	for i := range MaxLinkHops {
		next := filepath.Join(root, "n"+string(rune('a'+i)))
		long[prev] = next
		prev = next
	}
	rlLong := func(p string) (string, error) {
		if v, ok := long[p]; ok {
			return v, nil
		}
		return "", errors.New("не ссылка")
	}
	if LinkChainLocal(a, rlLong) {
		t.Error("цепочка из MaxLinkHops ссылок пропущена")
	}
	delete(long, filepath.Join(root, "n"+string(rune('a'+MaxLinkHops-2)))) // короче на одно звено
	if !LinkChainLocal(a, rlLong) {
		t.Error("цепочка короче MaxLinkHops должна пройти")
	}
}
