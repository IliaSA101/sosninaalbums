//go:build !windows && !linux

package osx

// На macOS и прочих ОС номер тома не читаем: statfs не отдаёт UUID, getattrlist в x/sys нет
// (153, design §4.2) — место остаётся без привязки к диску.

import (
	"errors"
	"strings"
)

// errVolUnsupported — номер тома на этой ОС недоступен: место остаётся без привязки.
var errVolUnsupported = errors.New("номер тома на этой ОС недоступен")

func isAbsVol(p string) bool { return strings.HasPrefix(p, "/") }

func volumeOS(string) (Vol, error) { return Vol{}, errVolUnsupported }

func volumePathOS(string, string) (string, string, error) { return "", "", errVolUnsupported }
