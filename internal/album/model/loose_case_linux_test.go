//go:build linux

package model

import "testing"

// Linux: отдельные фото `/x/Весна/a.jpg` и `/x/ВЕСНА/a.jpg` — два файла, два ключа; Clean — один.
// Свёртка источников не сливает запись `/x/ВЕСНА/sub` в папку `/x/Весна`.
func TestLooseKeyCaseLinux(t *testing.T) {
	if LooseKey("/x/Весна/a.jpg") == LooseKey("/x/ВЕСНА/a.jpg") {
		t.Fatal("регистр папки свёлся")
	}
	if LooseKey("/x/Весна/a.jpg") == LooseKey("/x/Весна/A.JPG") {
		t.Fatal("регистр имени свёлся")
	}
	if LooseKey("/x/Весна/./a.jpg") != LooseKey("/x/Весна/a.jpg") {
		t.Fatal("Clean не сводит путь к одному ключу")
	}
	if LooseKey("") != LooseKey(".") {
		t.Fatal("пустой путь — не ключ Clean")
	}
}
