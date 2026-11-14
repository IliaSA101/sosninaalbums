//go:build windows

package osx

import (
	"strconv"
	"syscall"
	"unsafe"
)

var procRtlGetVersion = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion")

// osVersionInfo — OSVERSIONINFOW (поле dwOSVersionInfoSize обязательно: по нему ntdll проверяет размер).
type osVersionInfo struct {
	size        uint32
	major       uint32
	minor       uint32
	build       uint32
	platformID  uint32
	servicePack [128]uint16
}

// osVersion — «Major.Minor.Build» из RtlGetVersion: настоящая версия, без подмены манифестом
// совместимости (GetVersionEx врёт старым программам). x/sys/windows в osx запрещён (depguard).
func osVersion() (string, bool) {
	var v osVersionInfo
	v.size = uint32(unsafe.Sizeof(v))
	// NTSTATUS: 0 — успех.
	if st, _, _ := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&v))); st != 0 || v.major == 0 { //nolint:gosec // G103: аргумент WinAPI
		return "", false
	}
	return strconv.Itoa(int(v.major)) + "." + strconv.Itoa(int(v.minor)) + "." + strconv.Itoa(int(v.build)), true
}
