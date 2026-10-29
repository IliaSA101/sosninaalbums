//go:build e2ebig

package app

import (
	"syscall"
	"unsafe"
)

// memStatusEx — MEMORYSTATUSEX из kernel32.
type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// e2eRAMGB — физическая память машины, ГБ (для отчёта e2e); 0 — не узнали.
func e2eRAMGB() int {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	st := memStatusEx{Length: uint32(unsafe.Sizeof(memStatusEx{}))}
	if r, _, _ := proc.Call(uintptr(unsafe.Pointer(&st))); r == 0 {
		return 0
	}
	return int((st.TotalPhys + 1<<29) >> 30)
}
