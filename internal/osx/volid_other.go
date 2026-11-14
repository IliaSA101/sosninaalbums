//go:build !windows && !linux && !(darwin && cgo)

package osx

// Прочие ОС и macOS без cgo: номер тома не читаем (statfs не отдаёт UUID, getattrlist в x/sys нет,
// 153 §4.2) — место без привязки к диску. macOS с cgo — volid_darwin.go (160 Т4).

import (
	"errors"
	"strings"
)

// errVolUnsupported — номер тома на этой ОС недоступен: место остаётся без привязки.
var errVolUnsupported = errors.New("номер тома на этой ОС недоступен")

// volidCgo — номер тома macOS читает cgo-вариант (volid_darwin.go); тест-страж CI проверяет его на macOS.
const volidCgo = false

func isAbsVol(p string) bool { return strings.HasPrefix(p, "/") }

func volumeOS(string) (Vol, error) { return Vol{}, errVolUnsupported }

func volumePathOS(string, string) (string, string, error) { return "", "", errVolUnsupported }
