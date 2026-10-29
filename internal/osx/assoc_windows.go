//go:build windows

package osx

import (
	"runtime"
	"syscall"
	"unsafe"
)

// Реестр и оболочка через syscall.NewLazyDLL: x/sys в osx запрещён (depguard), обе DLL в
// KnownDLLs — подмена из папки exe невозможна. Корень всегда HKCU.
var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procRegCreateKeyEx    = advapi32.NewProc("RegCreateKeyExW")
	procRegSetValueEx     = advapi32.NewProc("RegSetValueExW")
	procRegGetValue       = advapi32.NewProc("RegGetValueW")
	procRegDeleteTree     = advapi32.NewProc("RegDeleteTreeW")
	procRegDeleteKeyValue = advapi32.NewProc("RegDeleteKeyValueW")
	procRegCloseKey       = advapi32.NewProc("RegCloseKey")
	procRegOpenKeyEx      = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryInfoKey   = advapi32.NewProc("RegQueryInfoKeyW")
	procRegDeleteKey      = advapi32.NewProc("RegDeleteKeyW")
	procSHChangeNotify    = shell32.NewProc("SHChangeNotify")
)

const (
	hkeyCurrentUser = 0x80000001
	keyWrite        = 0x20006 // KEY_WRITE: создание подключей и запись значений
	keyCount        = 0x9     // KEY_QUERY_VALUE | KEY_ENUMERATE_SUB_KEYS: посчитать значения и подключи
	regNone         = 0
	regSZ           = 1
	rrfRTRegSZ      = 0x00000002
	rrfNoExpand     = 0x10000000
	shcneAssocChg   = 0x08000000
	errMoreData     = 234 // ERROR_MORE_DATA
)

// winReg — настоящий реестр. quiet — не звать SHChangeNotify (тест пишет в свою ветку).
type winReg struct{ quiet bool }

func sysBackend() regBackend { return winReg{} }

// ptr — адрес для Call. Строки живут до возврата вызова: владелец указателя держит его через
// runtime.KeepAlive.
func ptr(p *uint16) uintptr { return uintptr(unsafe.Pointer(p)) } //nolint:gosec // G103: аргументы WinAPI

// regErr — код возврата advapi32 (LONG, не GetLastError); 0 — успех.
func regErr(r uintptr) error {
	if r == 0 {
		return nil
	}
	return syscall.Errno(r)
}

func isNotFound(r uintptr) bool {
	return r == uintptr(syscall.ERROR_FILE_NOT_FOUND) || r == uintptr(syscall.ERROR_PATH_NOT_FOUND)
}

// namePtr — имя значения; пустое имя — NULL, то есть «(по умолчанию)».
func namePtr(name string) (*uint16, error) {
	if name == "" {
		return nil, nil
	}
	return syscall.UTF16PtrFromString(name)
}

func (winReg) get(key, name string) (string, bool) {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return "", false
	}
	n, err := namePtr(name)
	if err != nil {
		return "", false
	}
	defer runtime.KeepAlive(k)
	defer runtime.KeepAlive(n)
	flags := uintptr(rrfRTRegSZ | rrfNoExpand)
	var size uint32
	for range 3 { // значение могло вырасти между вызовами
		r, _, _ := procRegGetValue.Call(hkeyCurrentUser, ptr(k), ptr(n), flags, 0, 0, uintptr(unsafe.Pointer(&size))) //nolint:gosec // G103: аргументы WinAPI
		if r != 0 {
			return "", false
		}
		buf := make([]uint16, size/2+2)
		got := size
		r, _, _ = procRegGetValue.Call(hkeyCurrentUser, ptr(k), ptr(n), flags, 0, ptr(&buf[0]), uintptr(unsafe.Pointer(&got))) //nolint:gosec // G103: аргументы WinAPI
		if r == errMoreData {
			size = got
			continue
		}
		if r != 0 {
			return "", false
		}
		return syscall.UTF16ToString(buf), true
	}
	return "", false
}

// put создаёт ключ (со всеми предками) и пишет в него значение типа typ.
func (winReg) put(key, name string, typ uintptr, data []uint16) error {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return err
	}
	n, err := namePtr(name)
	if err != nil {
		return err
	}
	var h syscall.Handle
	r, _, _ := procRegCreateKeyEx.Call(hkeyCurrentUser, ptr(k), 0, 0, 0, keyWrite, 0, uintptr(unsafe.Pointer(&h)), 0) //nolint:gosec // G103: аргументы WinAPI
	runtime.KeepAlive(k)
	if err := regErr(r); err != nil {
		return err
	}
	defer procRegCloseKey.Call(uintptr(h)) //nolint:errcheck // закрытие ключа: ошибке нечего делать
	var p *uint16
	if len(data) > 0 {
		p = &data[0]
	}
	r, _, _ = procRegSetValueEx.Call(uintptr(h), ptr(n), 0, typ, ptr(p), uintptr(len(data)*2))
	runtime.KeepAlive(n)
	runtime.KeepAlive(data)
	return regErr(r)
}

func (w winReg) set(key, name, val string) error {
	data, err := syscall.UTF16FromString(val) // с завершающим нулём
	if err != nil {
		return err
	}
	return w.put(key, name, regSZ, data)
}

func (w winReg) setNone(key, name string) error { return w.put(key, name, regNone, nil) }

func (winReg) deleteTree(key string) error {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return err
	}
	r, _, _ := procRegDeleteTree.Call(hkeyCurrentUser, ptr(k))
	runtime.KeepAlive(k)
	if isNotFound(r) {
		return nil
	}
	return regErr(r)
}

func (winReg) deleteValue(key, name string) error {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return err
	}
	n, err := namePtr(name)
	if err != nil {
		return err
	}
	r, _, _ := procRegDeleteKeyValue.Call(hkeyCurrentUser, ptr(k), ptr(n))
	runtime.KeepAlive(k)
	runtime.KeepAlive(n)
	if isNotFound(r) {
		return nil
	}
	return regErr(r)
}

// deleteIfEmpty удаляет ключ, если в нём нет ни значений (включая «по умолчанию»), ни подключей.
// Нет ключа — не ошибка.
func (winReg) deleteIfEmpty(key string) error {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return err
	}
	defer runtime.KeepAlive(k)
	var h syscall.Handle
	r, _, _ := procRegOpenKeyEx.Call(hkeyCurrentUser, ptr(k), 0, keyCount, uintptr(unsafe.Pointer(&h))) //nolint:gosec // G103: аргументы WinAPI
	if isNotFound(r) {
		return nil
	}
	if err := regErr(r); err != nil {
		return err
	}
	var subKeys, values uint32
	r, _, _ = procRegQueryInfoKey.Call(uintptr(h), 0, 0, 0, uintptr(unsafe.Pointer(&subKeys)), 0, 0, uintptr(unsafe.Pointer(&values)), 0, 0, 0, 0) //nolint:gosec // G103: аргументы WinAPI
	_, _, _ = procRegCloseKey.Call(uintptr(h))
	if err := regErr(r); err != nil {
		return err
	}
	if subKeys > 0 || values > 0 {
		return nil
	}
	r, _, _ = procRegDeleteKey.Call(hkeyCurrentUser, ptr(k))
	if isNotFound(r) {
		return nil
	}
	return regErr(r)
}

func (w winReg) notify() {
	if w.quiet {
		return
	}
	// SHCNE_ASSOCCHANGED, SHCNF_IDLIST: оболочка перечитывает значки и обработчики.
	_, _, _ = procSHChangeNotify.Call(shcneAssocChg, 0, 0, 0)
}
