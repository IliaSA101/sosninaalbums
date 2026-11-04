//go:build linux

package paths

import "testing"

// Linux: `/x/Весна` и `/x/ВЕСНА` — две папки. Запись `/x/ВЕСНА/sub` не сворачивается в `/x/Весна`
// (раньше ключ был общий — фото пропадали из ленты), каждый файл — под своей папкой; в одной и
// той же папке Cover работает как прежде.
func TestCoverCaseSensitiveLinux(t *testing.T) {
	c := NewCover([]string{"/x/Весна", "/x/ВЕСНА"}, nil)
	for _, tc := range []struct {
		path, root string
		file, ok   bool
	}{
		{"/x/Весна/a.jpg", "/x/Весна", true, true},
		{"/x/ВЕСНА/a.jpg", "/x/ВЕСНА", true, true},
		{"/x/ВЕСНА/sub", "/x/ВЕСНА", false, true},
		{"/x/весна/a.jpg", "", true, false}, // третьей папки в списке нет
	} {
		if r, ok := c.Root(tc.path, tc.file); r != tc.root || ok != tc.ok {
			t.Errorf("Root(%q) = %q %v, ждали %q %v", tc.path, r, ok, tc.root, tc.ok)
		}
	}
	if Reaches("/x/Весна", "/x/ВЕСНА/sub", false, nil) {
		t.Error("Reaches: /x/ВЕСНА/sub внутри /x/Весна")
	}
	if under := UnderAny([]string{"/x/Весна"}); under("/x/ВЕСНА/a.jpg") || !under("/x/Весна/a.jpg") {
		t.Error("UnderAny свёл регистр")
	}
	if top := TopKeys([]string{"/x/Весна", "/x/ВЕСНА", "/x/Весна/sub"}); len(top) != 2 {
		t.Errorf("TopKeys: %q, ждали две папки", top)
	}
}

// Linux: переезд `/x/Весна` не уносит `/x/ВЕСНА`; ReplacePrefix (для эвристик по существованию
// файла) регистр по-прежнему не различает.
func TestReplacePrefixFSLinux(t *testing.T) {
	if got, ok := ReplacePrefixFS("/x/Весна/a/1.jpg", "/x/Весна", "/y/Новая"); !ok || got != "/y/Новая/a/1.jpg" {
		t.Errorf("своя папка: %q %v", got, ok)
	}
	if got, ok := ReplacePrefixFS("/x/ВЕСНА/1.jpg", "/x/Весна", "/y/Новая"); ok {
		t.Errorf("чужая папка другого регистра переехала: %q", got)
	}
	if _, ok := ReplacePrefix("/x/ВЕСНА/1.jpg", "/x/Весна", "/y/Новая"); !ok {
		t.Error("ReplacePrefix перестал игнорировать регистр")
	}
	if _, ok := ReplacePrefixFS("/x/Весна2/1.jpg", "/x/Весна", "/y/Новая"); ok {
		t.Error("Весна2 не внутри Весна")
	}
}
