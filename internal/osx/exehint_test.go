package osx

import (
	"path/filepath"
	"runtime"
	"testing"
)

// Мёртвый путь: перенос из папки временных файлов, App Translocation, не-файл, относительный путь.
// Правила путей (Windows — с `\` и буквой диска, Unix — с `/`) разбирает filepath ОС, поэтому
// проверяются только там, где он их понимает: Unix-случаи с абсолютным путём не идут на Windows.
func TestExeHint(t *testing.T) {
	yes := func(string) bool { return true }
	no := func(string) bool { return false }
	abs := filepath.Join(string(filepath.Separator), "opt", "Sosnina", "app")
	type tc struct {
		name, exe, goos, tmp string
		regular              func(string) bool
		dead                 bool
	}
	cases := []tc{
		{"пустой путь", "", "linux", "/tmp", yes, true},
		{"относительный путь", "app", "linux", "/tmp", yes, true},
		{"путь с ./", "./app", "linux", "/tmp", yes, true},
	}
	if runtime.GOOS != "windows" {
		cases = append(cases,
			tc{"обычный файл", abs, "linux", "/tmp", yes, false},
			tc{"файл удалён или заменён каталогом", abs, "linux", "/tmp", no, true},
			tc{"Mac: App Translocation", "/private/var/folders/x/AppTranslocation/ABC/d/Sosnina.app/Contents/MacOS/app", "darwin", "/tmp", yes, true},
			tc{"Mac: из «Программ»", "/Applications/Sosnina.app/Contents/MacOS/app", "darwin", "/tmp", yes, false},
			tc{"Linux: AppTranslocation — не Mac", "/x/AppTranslocation/app", "linux", "/tmp", yes, false},
			tc{"Linux: exe в /tmp годится", "/tmp/app", "linux", "/tmp", yes, false},
		)
	}
	if runtime.GOOS == "windows" {
		cases = append(cases,
			tc{"Windows: exe из zip под Temp", `C:\Users\u\AppData\Local\Temp\x\app.exe`, "windows", `C:\Users\u\AppData\Local\Temp`, yes, true},
			tc{"Windows: регистр букв", `c:\users\U\appdata\local\TEMP\app.exe`, "windows", `C:\Users\u\AppData\Local\Temp\`, yes, true},
			tc{"Windows: соседняя папка Temp2", `C:\Users\u\AppData\Local\Temp2\app.exe`, "windows", `C:\Users\u\AppData\Local\Temp`, yes, false},
			tc{"Windows: Program Files", `C:\Program Files\Sosnina\app.exe`, "windows", `C:\Users\u\AppData\Local\Temp`, yes, false},
			tc{"Windows: Temp неизвестна", `C:\Temp\app.exe`, "windows", "", yes, false},
			tc{"Windows: распакованный в Проводнике zip", `C:\Users\u\AppData\Local\Temp1_Sosnina.zip\app.exe`, "windows", `D:\Other`, yes, true},
		)
	}
	for _, c := range cases {
		got := ExeHint(c.exe, c.goos, c.tmp, c.regular)
		if (got != "") != c.dead {
			t.Errorf("%s: ExeHint = %q, мёртвый путь = %v", c.name, got, c.dead)
		}
	}
	// regular не зовётся для явно мёртвого пути: пустая строка не должна дойти до диска.
	if got := ExeHint("", "linux", "/tmp", func(string) bool { t.Error("regular вызван для пустого пути"); return true }); got == "" {
		t.Error("пустой путь без предупреждения")
	}
}

// TestWindowsTemp: правило временной папки Windows — по тексту, на любой ОС. Распаковка zip в
// Проводнике — Temp<N>_<имя>.zip в любой Temp (в том числе при 8.3 в %TMP%); соседние папки
// и обычные имена не задеваются.
func TestWindowsTemp(t *testing.T) {
	cases := []struct {
		exe, tmp string
		dead     bool
	}{
		{`C:\Users\Ivan Ivan\AppData\Local\Temp\Temp1_SosninaAlbums.zip\SosninaAlbums.exe`, `C:\Users\IVANOV~1\AppData\Local\Temp`, true},
		{`C:\Users\Ivan\AppData\Local\Temp\Temp12_x.zip\a.exe`, "", true},
		{`D:\Распаковка\temp1_Альбом.ZIP\a.exe`, `C:\Temp`, true},
		{`C:\Temp\x\a.exe`, `C:\Temp`, true},
		{`C:\TEMP\x\a.exe`, `c:\temp`, true},
		{`C:\Temp2\a.exe`, `C:\Temp`, false},
		{`C:\Temp1_x.zipper\a.exe`, `C:\Other`, false},
		{`C:\Temp1_.zip\a.exe`, `C:\Other`, false},
		{`C:\Temp_x.zip\a.exe`, `C:\Other`, true},
		{`C:\Tempx_x.zip\a.exe`, `C:\Other`, false},
		{`C:\Program Files\Sosnina\a.exe`, `C:\Users\u\AppData\Local\Temp`, false},
		{`C:\Temp\a.exe`, "", false},
	}
	for _, c := range cases {
		if got := inWindowsTemp(c.exe, c.tmp); got != c.dead {
			t.Errorf("inWindowsTemp(%q, %q) = %v, ждали %v", c.exe, c.tmp, got, c.dead)
		}
	}
}
