package osx

// Hidden (Windows): у какого-нибудь элемента пути ниже корня тома атрибут HIDDEN или SYSTEM. Корень
// тома проверять нельзя: корень NTFS сам HIDDEN|SYSTEM, и отказал бы любому пути (design 150 §4.1).
// Элемент, атрибуты которого не прочитать, — тоже «скрытый»: тихо брать нельзя то, что не проверить.

import (
	"path/filepath"
	"strings"
	"syscall"
)

const hiddenAttrs = syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM

func Hidden(path string) bool { return hiddenWith(path, fileAttrs) }

func fileAttrs(p string) (uint32, error) {
	u, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return 0, err
	}
	return syscall.GetFileAttributes(u)
}

// hiddenWith — Hidden с подменой чтения атрибутов (тест: временная папка лежит под AppData, а она
// сама скрытая).
func hiddenWith(path string, attrs func(string) (uint32, error)) bool {
	vol := filepath.VolumeName(path)
	if vol == "" {
		return true
	}
	cur := vol + `\`
	for _, el := range strings.FieldsFunc(path[len(vol):], func(r rune) bool { return r == '\\' || r == '/' }) {
		cur = filepath.Join(cur, el)
		a, err := attrs(cur)
		if err != nil || a&hiddenAttrs != 0 {
			return true
		}
	}
	return false
}
