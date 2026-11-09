//go:build windows

package mcpbridge

import (
	"errors"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// appmodelNoPackage — APPMODEL_ERROR_NO_PACKAGE: у процесса нет пакетной идентичности (обычный exe).
const appmodelNoPackage = 15700

var procCurrentPackage = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetCurrentPackageFullName")

// platformPackaged — посредник запущен в пакете MSIX (Claude Desktop из Store): AppData там
// виртуализован в Packages\…\LocalCache, и config.json, черновик и сокет программы легли бы в
// контейнер клиента. Пока спайк Т0 не доказал выход, это отказ. Любой ответ, кроме «пакета нет»,
// считается пакетом: ошибиться в сторону отказа безопаснее. Нет самой функции (Windows до 8) — не пакет.
func platformPackaged() bool {
	if procCurrentPackage.Find() != nil {
		return false
	}
	var n uint32
	r, _, _ := procCurrentPackage.Call(uintptr(unsafe.Pointer(&n)), 0) //nolint:gosec // G103: длина по указателю на свою переменную, буфера нет
	return r != appmodelNoPackage
}

// platformInstanceAlive — замок экземпляра Wails: именованный мьютекс "wails-app-<UniqueId>sim"
// (internal/frontend/desktop/windows/single_instance.go Wails). Открывается без создания.
func platformInstanceAlive(key string) bool {
	if key == "" {
		return false
	}
	name, err := windows.UTF16PtrFromString("wails-app-" + key + "sim")
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

// spawnProgram — запуск своей программы без аргументов: без консоли, в своей группе и вне задания
// процессов клиента (иначе закрытие клиента убило бы окно). Если ОС не даёт выйти из задания
// (ERROR_ACCESS_DENIED), это errBreakaway: запуск без отделения не пробуем — программа жила бы в
// задании клиента и погибла бы вместе с ним (design §5, решение 0.33-149-22).
func spawnProgram(exe string, env []string) error {
	return spawnWith(exe, env, spawnFlags|windows.CREATE_BREAKAWAY_FROM_JOB)
}

// spawnFlags — признаки создания процесса программы: без консоли и в своей группе.
const spawnFlags = windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP

// spawnWith — spawnProgram с заданными признаками. errBreakaway — только если среди них был
// CREATE_BREAKAWAY_FROM_JOB: без него отказ в доступе значил бы другое.
func spawnWith(exe string, env []string, flags uint32) error {
	cmd := exec.Command(exe) //nolint:gosec // exe — собственная программа (os.Executable с раскрытыми ссылками), аргументов нет
	cmd.Dir = filepath.Dir(exe)
	cmd.Env = append([]string{}, env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags}
	if err := cmd.Start(); err != nil {
		if flags&windows.CREATE_BREAKAWAY_FROM_JOB != 0 && errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return errBreakaway
		}
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
