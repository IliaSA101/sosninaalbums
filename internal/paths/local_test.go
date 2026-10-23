package paths

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Имя устройства без расширения в любом элементе пути — отказ всегда (073-8, 073-10): в середине,
// в конце, в нижнем регистре, с надстрочной цифрой, с хвостовыми точками и пробелами.
func TestCheckLocalPathDevices(t *testing.T) {
	skipNotWindows(t)
	for _, bad := range []string{
		`C:\x\COM1`, `C:\x\NUL`, `C:\x\LPT9`, `C:\x\LPT9\a.jpg`, `C:\COM1\a.jpg`,
		`C:\CON\a.jpg`, `C:/x/Aux/y.jpg`, `C:\x\COM¹`, `C:\x\CON.`, `C:\x\CON . .`, `C:\x\LPT1.\a.jpg`,
		`D:\a\b\prn`, `C:\x\CON \y.jpg`, `C:\x\conin$`, `C:\x\CONOUT$`, `C:\x\COM0`,
		`C:\x/COM3/y`, `C:\\x\NUL`, // двойной разделитель не прячет элемент
	} {
		err := CheckLocalPath(bad)
		if err == nil {
			t.Errorf("путь %q должен отклоняться", bad)
			continue
		}
		if !errors.Is(err, ErrNotLocal) || !errors.Is(err, ErrDevice) {
			t.Errorf("%q: ошибка %v не ErrNotLocal и ErrDevice", bad, err)
		}
	}
	for _, ok := range []string{
		`C:\x\COM10`, `C:\x\con1`, `C:\x\COMM`, `C:\x\console\a.jpg`, `C:\x\a.con`, `C:\x\`, `C:\`,
		`C:\x\LPT`, `C:\x\COM1a`, `C:\Съёмки\Иванов\1.jpg`, `C:\x\nu`, `C:\x\..\y`,
	} {
		if err := CheckLocalPath(ok); err != nil {
			t.Errorf("путь %q должен проходить: %v", ok, err)
		}
	}
	// В тексте — имя устройства, но не весь путь.
	err := CheckLocalPath(`C:\Users\Имя\COM1`)
	if err == nil || !strings.Contains(err.Error(), "COM1") || strings.Contains(err.Error(), "Имя") {
		t.Errorf("текст отказа: %v", err)
	}
	// Не устройство — не ErrDevice: Load отличает его от порчи.
	if err := CheckLocalPath(`\\сервер\a`); err == nil || errors.Is(err, ErrDevice) {
		t.Errorf("UNC: %v, ждали отказ без ErrDevice", err)
	}
}

// devExt — пути с устройством с расширением: Windows 10 считает его устройством, 11 — файлом.
var devExt = []string{
	`C:\x\con.txt`, `C:\x\nul.txt\y`, `C:\x\LPT9.tar.gz`, `C:\x\CONOUT$.log`, `C:\Фото\CON.jpg`,
	`C:\x\aux.2024`, `C:\x\COM¹.jpg`, `C:\x\Con.2024\a.jpg`,
}

// Устройство с расширением судит ОС (073-10): «старая ОС» (подмена оракула: Windows 10) — отказ
// с ErrDevice, «новая» (Windows 11) — путь проходит; без подмены — как filepath.IsLocal элемента.
func TestCheckLocalPathDeviceExt(t *testing.T) {
	skipNotWindows(t)
	restore := SetDeviceOracle(Device)
	for _, p := range devExt {
		if err := CheckLocalPath(p); !errors.Is(err, ErrDevice) {
			t.Errorf("Windows 10: %q — %v, ждали ErrDevice", p, err)
		}
	}
	restore()
	restore = SetDeviceOracle(func(string) bool { return false })
	for _, p := range devExt {
		if err := CheckLocalPath(p); err != nil {
			t.Errorf("Windows 11: %q должен проходить: %v", p, err)
		}
	}
	if err := CheckLocalPath(`C:\x\CON`); !errors.Is(err, ErrDevice) {
		t.Errorf("без расширения — отказ и на Windows 11: %v", err)
	}
	restore()
	for _, p := range devExt {
		osDev := false
		for _, el := range strings.FieldsFunc(p[3:], IsSepRune) {
			osDev = osDev || !filepath.IsLocal(el)
		}
		if got := CheckLocalPath(p) != nil; got != osDev {
			t.Errorf("%q: отказ %v, а ОС судит %v", p, got, osDev)
		}
	}
}

// DeviceElem: «.» и «..» — не устройства; на других ОС устройств нет.
func TestDeviceElemDots(t *testing.T) {
	for _, el := range []string{".", "..", "...", "a", "Иванов.jpg"} {
		if DeviceElem(el) {
			t.Errorf("DeviceElem(%q) = true", el)
		}
	}
	if got := DeviceElem("CON"); got != (runtime.GOOS == "windows") {
		t.Errorf("DeviceElem(CON) = %v на %s", got, runtime.GOOS)
	}
}

// NoPath для «не локального» пути заменяет текст отказа (с путём) на ErrNotLocal, приставки
// обёрток оставляет — в log.txt; сам отказ для тоста сохраняет путь и узнаётся через errors.Is.
func TestNoPathNotLocal(t *testing.T) {
	for _, bad := range []string{`\\сервер\Иванов\a`, "Иванов/rel.jpg", `C:\Иванов\..\x:s`, `\\?\C:\Иванов`} {
		err := CheckLocalPath(bad)
		if err == nil {
			t.Fatalf("путь %q должен отклоняться", bad)
		}
		if !errors.Is(err, ErrNotLocal) {
			t.Errorf("%q: %v не ErrNotLocal", bad, err)
		}
		if bad != `C:\Иванов\..\x:s` && !strings.Contains(err.Error(), "Иванов") {
			t.Errorf("тост должен нести путь: %v", err) // тексты тостов прежние
		}
		for _, c := range []struct {
			err  error
			want string
		}{
			{err, ErrNotLocal.Error()},
			{fmt.Errorf("добавление: %w", err), "добавление: " + ErrNotLocal.Error()},
			{fmt.Errorf("разметка разворотов в проекте: %w", fmt.Errorf("x: %w", err)), "разметка разворотов в проекте: x: " + ErrNotLocal.Error()},
		} {
			got := NoPath(c.err)
			if got.Error() != c.want || !errors.Is(got, ErrNotLocal) || errors.Is(got, ErrDevice) {
				t.Errorf("NoPath(%v) = %v, ждали %q (ErrNotLocal, приставка обёртки, без пути)", c.err, got, c.want)
			}
		}
	}
	if NoPath(nil) != nil {
		t.Error("nil должен остаться nil")
	}
	// Хвост после отказа — не приставка, в нём может быть что угодно: остаётся только ErrNotLocal.
	tail := fmt.Errorf("%w (Иванов)", CheckLocalPath("Иванов"))
	if got := NoPath(tail); got.Error() != ErrNotLocal.Error() || !errors.Is(got, ErrNotLocal) {
		t.Errorf("NoPath(%v) = %v", tail, got)
	}
	if runtime.GOOS == "windows" {
		dev := fmt.Errorf("фото в проекте: %w", CheckLocalPath(`C:\Иванов\NUL`))
		got := NoPath(dev)
		if !errors.Is(got, ErrDevice) || strings.Contains(got.Error(), "Иванов") || got.Error() != "фото в проекте: "+ErrNotLocal.Error() {
			t.Errorf("NoPath(%v) = %v: ждали ErrDevice с приставкой без пути", dev, got)
		}
	}
}

// LogExt — вид расширения для лога: нижний регистр, без расширения — "", необычное — «прочее»
// (не латиница с цифрами, длиннее 8, в «расширении» — имя).
func TestLogExt(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Иванов\IMG_1.HEIC`: ".heic", "a.cr3": ".cr3", "a": "", "a.": "", "": "",
		"IMG.Иванов": "прочее", "a.jpeg2000x": "прочее", "a.12345678": ".12345678", "a.j-g": "прочее",
		"a.jpg ": "прочее", `C:\x.y\name`: "", "/x.y/name": "", `/дом/x.y\IMG.JPG`: ".jpg",
	} {
		if got := LogExt(in); got != want {
			t.Errorf("LogExt(%q) = %q, ждали %q", in, got, want)
		}
	}
}
