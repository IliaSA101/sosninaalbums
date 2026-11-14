package paths

import (
	"errors"
	"strings"
	"testing"
)

// Правила Windows — чистые функции: таблицы идут на любой ОС (в CI Linux граница безопасности
// иначе не проверялась бы вовсе); на Windows CheckLocalPath и DeviceElem — те же функции с
// суждением ОС, это сверяет TestWindowsRuleMatchesOS.

var (
	win10 = func(string) bool { return true }  // «CON.jpg» — устройство
	win11 = func(string) bool { return false } // «CON.jpg» — обычный файл
)

type ruleKind int

const (
	ruleOK ruleKind = iota
	ruleNotLocal
	ruleDevice
)

func (k ruleKind) String() string {
	return [...]string{"проходит", "не локальный", "устройство"}[k]
}

func kindOf(err error) ruleKind {
	switch {
	case err == nil:
		return ruleOK
	case errors.Is(err, ErrDevice):
		return ruleDevice
	case errors.Is(err, ErrNotLocal):
		return ruleNotLocal
	}
	return -1
}

// Белый список `X:\…`: что не он — отказ без ErrDevice; пустой хвост и корень проходят.
func TestWindowsLocalRuleShape(t *testing.T) {
	cases := []struct {
		path string
		want ruleKind
	}{
		{`C:\Photos`, ruleOK}, {`D:/a/b`, ruleOK}, {`d:\x`, ruleOK}, {`C:\`, ruleOK}, {`C:/`, ruleOK},
		{`C:\Съёмки\Иванов\1.jpg`, ruleOK}, {`C:\\x\y`, ruleOK}, {`Z:\a\..\b`, ruleOK},
		// границы букв диска: A, a, z проходят, соседние в ASCII (@ ` {) — нет
		{`A:\x`, ruleOK}, {`a:\x`, ruleOK}, {`z:\x`, ruleOK},
		{`@:\x`, ruleNotLocal}, {"`:\\x", ruleNotLocal}, {`{:\x`, ruleNotLocal},
		{`\\host\share\a`, ruleNotLocal}, {`//host/share/a`, ruleNotLocal}, {`\\?\C:\a`, ruleNotLocal},
		{`\\.\PhysicalDrive0`, ruleNotLocal}, {`\??\UNC\h\s`, ruleNotLocal}, {`\x`, ruleNotLocal},
		{`C:x`, ruleNotLocal}, {`C:`, ruleNotLocal}, {`1:\a`, ruleNotLocal}, {`ab\x`, ruleNotLocal},
		{`/x/y`, ruleNotLocal}, {"rel/a.jpg", ruleNotLocal}, {`Я:\x`, ruleNotLocal}, {`[:\x`, ruleNotLocal},
		{`C:\a\b.jpg:s`, ruleNotLocal}, {`C:\a\b.jpg::$DATA`, ruleNotLocal}, {`C:\a:b\c.jpg`, ruleNotLocal},
		{`C:/a/b:`, ruleNotLocal}, {`C:\:`, ruleNotLocal},
	}
	for _, c := range cases {
		err := windowsLocalRule(c.path, win11)
		if got := kindOf(err); got != c.want {
			t.Errorf("%q: %v (%v), ждали %v", c.path, got, err, c.want)
		}
		if c.want == ruleNotLocal && (err == nil || !strings.Contains(err.Error(), c.path)) {
			t.Errorf("%q: тост отказа должен нести путь: %v", c.path, err)
		}
	}
}

// Устройство без расширения — отказ всегда (в любом элементе, регистре, с хвостовыми точками и
// пробелами); похожие имена проходят; текст — имя устройства, но не весь путь; ErrNotLocal и ErrDevice.
func TestWindowsLocalRuleDevices(t *testing.T) {
	for _, bad := range []string{
		`C:\x\COM1`, `C:\x\NUL`, `C:\x\LPT9`, `C:\x\LPT9\a.jpg`, `C:\COM1\a.jpg`,
		`C:\CON\a.jpg`, `C:/x/Aux/y.jpg`, `C:\x\COM¹`, `C:\x\CON.`, `C:\x\CON . .`, `C:\x\LPT1.\a.jpg`,
		`D:\a\b\prn`, `C:\x\CON \y.jpg`, `C:\x\conin$`, `C:\x\CONOUT$`, `C:\x\COM0`,
		`C:\x\LPT²`, `C:\x/COM3/y`, `C:\\x\NUL`, // двойной разделитель не прячет элемент
	} {
		for name, oracle := range map[string]func(string) bool{"win10": win10, "win11": win11} {
			err := windowsLocalRule(bad, oracle)
			if !errors.Is(err, ErrNotLocal) || !errors.Is(err, ErrDevice) {
				t.Errorf("%s: %q — %v, ждали ErrNotLocal и ErrDevice", name, bad, err)
			}
		}
	}
	for _, ok := range []string{
		`C:\x\COM10`, `C:\x\con1`, `C:\x\COMM`, `C:\x\console\a.jpg`, `C:\x\a.con`, `C:\x\`, `C:\`,
		`C:\x\LPT`, `C:\x\COM1a`, `C:\Съёмки\Иванов\1.jpg`, `C:\x\nu`, `C:\x\..\y`,
	} {
		for name, oracle := range map[string]func(string) bool{"win10": win10, "win11": win11} {
			if err := windowsLocalRule(ok, oracle); name == "win11" && err != nil {
				t.Errorf("%s: %q должен проходить: %v", name, ok, err)
			}
		}
	}
	err := windowsLocalRule(`C:\Users\Имя\COM1`, win11)
	if err == nil || !strings.Contains(err.Error(), "COM1") || strings.Contains(err.Error(), "Имя") {
		t.Errorf("текст отказа: %v", err)
	}
}

// Устройство с расширением судит ОС: Windows 10 — отказ с ErrDevice, 11 — путь проходит; без
// расширения оракул не спрашивают вовсе.
func TestWindowsLocalRuleDeviceExt(t *testing.T) {
	for _, p := range []string{
		`C:\x\con.txt`, `C:\x\nul.txt\y`, `C:\x\LPT9.tar.gz`, `C:\x\CONOUT$.log`, `C:\Фото\CON.jpg`,
		`C:\x\aux.2024`, `C:\x\COM¹.jpg`, `C:\x\Con.2024\a.jpg`,
	} {
		if err := windowsLocalRule(p, win10); !errors.Is(err, ErrDevice) {
			t.Errorf("Windows 10: %q — %v, ждали ErrDevice", p, err)
		}
		if err := windowsLocalRule(p, win11); err != nil {
			t.Errorf("Windows 11: %q должен проходить: %v", p, err)
		}
	}
	var asked []string
	spy := func(el string) bool { asked = append(asked, el); return false }
	if err := windowsLocalRule(`C:\x\CON\a.b\c`, spy); !errors.Is(err, ErrDevice) || len(asked) != 0 {
		t.Errorf("CON без расширения: %v, оракул спрошен %q; ждали отказ до оракула", err, asked)
	}
	asked = nil
	if err := windowsLocalRule(`C:\a.b\Иванов.jpg`, spy); err != nil || len(asked) != 2 {
		t.Errorf("элементы с расширением: %v, спрошено %q; ждали проход и 2 вопроса", err, asked)
	}
}

// DeviceElem без ОС: на не-Windows устройств нет; на Windows — основа имени, «.» и «..» не устройства.
func TestDeviceElemRule(t *testing.T) {
	bare := []string{"CON", "con", "Nul", "AUX", "PRN", "COM1", "lpt9", "COM¹", "LPT²", "LPT³", "CONIN$", "CONOUT$", "COM0",
		"CON.", "CON . .", "LPT1.", "CON ", "com2. ."}
	notDev := []string{".", "..", "...", "a", "COM10", "COMM", "LPT", "console", "nu", "", "Иванов", ".con", "a.con"}
	for _, el := range bare {
		if !deviceElemRule(true, el, win11) {
			t.Errorf("Windows: %q должно быть устройством (и при win11)", el)
		}
		if deviceElemRule(false, el, win10) {
			t.Errorf("не Windows: %q — не устройство", el)
		}
	}
	for _, el := range notDev {
		if deviceElemRule(true, el, win11) {
			t.Errorf("Windows: %q — не устройство", el)
		}
	}
	// Без расширения оракул «всё — устройство» не меняет ответа: его спрашивают только про имена с точкой.
	for _, el := range []string{".", "..", "...", "a", "COM10", "COMM", "console", "Иванов"} {
		if deviceElemRule(true, el, win10) {
			t.Errorf("Windows 10: %q — не устройство", el)
		}
	}
	for _, el := range []string{"CON.jpg", "aux.2024", "COM¹.jpg", "nul.tar.gz"} {
		if !deviceElemRule(true, el, win10) || deviceElemRule(true, el, win11) {
			t.Errorf("%q с расширением: Windows 10 — устройство, 11 — файл", el)
		}
		if deviceElemRule(false, el, win10) {
			t.Errorf("не Windows: %q — не устройство", el)
		}
	}
	if deviceElemRule(true, ".", win10) || deviceElemRule(true, "..", win10) {
		t.Error("«.» и «..» не устройства даже по оракулу «всё — устройство»")
	}
}

// На Windows CheckLocalPath и DeviceElem дают то же, что правила с суждением ОС.
func TestWindowsRuleMatchesOS(t *testing.T) {
	skipNotWindows(t)
	for _, p := range []string{`C:\x`, `C:\x\CON`, `C:\x\con.txt`, `\\h\s`, `C:x`, `C:\a:b`, `D:/Съёмки/1.jpg`} {
		if got, want := kindOf(CheckLocalPath(p)), kindOf(windowsLocalRule(p, osDeviceExt)); got != want {
			t.Errorf("%q: CheckLocalPath %v, правило %v", p, got, want)
		}
	}
	for _, el := range []string{"CON", "con.txt", "a", "..", "COM1."} {
		if got, want := DeviceElem(el), deviceElemRule(true, el, osDeviceExt); got != want {
			t.Errorf("DeviceElem(%q) = %v, правило %v", el, got, want)
		}
	}
}
