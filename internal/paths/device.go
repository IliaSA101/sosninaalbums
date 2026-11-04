package paths

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

// ErrDevice — элемент пути на этой ОС — устройство Windows (CON, NUL, COM1…): обычный файл по
// такому пути не открыть. Отказ CheckLocalPath по устройству узнаётся и как ErrNotLocal, и как
// ErrDevice: Load отличает его от порчи файла — фото недостающее, а не провал альбома (073-10).
var ErrDevice = errors.New("имя устройства Windows")

// deviceOracle — суждение ОС об элементе с расширением; nil — filepath.IsLocal (внутри —
// RtlIsDosDeviceName_U). Подменяется в тестах: Windows 10 и 11 судят «CON.jpg» по-разному.
var deviceOracle atomic.Pointer[func(el string) bool]

// SetDeviceOracle — только для тестов: dev(el) — «элемент с расширением — устройство» вместо
// суждения ОС (Windows 10: «CON.jpg» — устройство, 11 — файл); restore возвращает прежнее.
func SetDeviceOracle(dev func(el string) bool) (restore func()) {
	old := deviceOracle.Swap(&dev)
	return func() { deviceOracle.Store(old) }
}

// DeviceElem — элемент пути (без разделителей) на этой ОС — устройство Windows. Без расширения
// (после срезки точек и пробелов в конце: «CON», «CON . .», «LPT1.», «CONIN$», «COM¹») —
// всегда: такую папку или файл не создать ни на одной версии Windows. С расширением
// («CON.jpg», «aux.2024») — как судит сама ОС: Windows 10 считает устройством, 11 — обычным
// файлом. «.» и «..» — не устройства; на других ОС устройств нет.
func DeviceElem(el string) bool {
	return deviceElemRule(runtime.GOOS == "windows", el, osDeviceExt)
}

// deviceElemRule — DeviceElem без обращения к ОС: windows — правило Windows (на других устройств
// нет), devExt судит элемент с расширением. Проверяется таблицей на любой ОС.
func deviceElemRule(windows bool, el string, devExt func(el string) bool) bool {
	if !windows || el == "." || el == ".." {
		return false
	}
	if bare := strings.TrimRight(el, ". "); !strings.Contains(bare, ".") {
		return reservedName(bare)
	}
	return devExt(el)
}

// osDeviceExt — суждение ОС об элементе с расширением: подмена теста или filepath.IsLocal
// (на Windows внутри RtlIsDosDeviceName_U). Ветку `!filepath.IsLocal` проверяет только раннер
// Windows 10 / Server 2019–2022; на Windows 11 её судит сама ОС.
func osDeviceExt(el string) bool {
	if f := deviceOracle.Load(); f != nil {
		return (*f)(el)
	}
	return !filepath.IsLocal(el)
}
