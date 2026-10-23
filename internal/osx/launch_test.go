package osx

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLaunchPlace(t *testing.T) {
	tmp := t.TempDir()
	other := t.TempDir() // отдельная папка рядом: не внутри tmp
	sub := filepath.Join(tmp, "Temp1_album.zip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(sub, "SosninaAlbums.exe")
	mac := "/private/var/folders/x/AppTranslocation/ABC-123/d/sosninaalbums.app/Contents/MacOS/sosninaalbums"

	cases := []struct {
		name, exe, temp string
		want            Place
	}{
		{"подпапка temp", exe, tmp, PlaceTemp},
		{"exe прямо в temp", filepath.Join(tmp, "a.exe"), tmp, PlaceTemp},
		{"temp со слэшем на конце", exe, tmp + string(filepath.Separator), PlaceTemp},
		{"глубокая вложенность", filepath.Join(sub, "a", "b", "x.exe"), tmp, PlaceTemp},
		{"другой регистр exe", strings.ToUpper(exe), tmp, PlaceTemp},
		{"другой регистр temp", exe, strings.ToUpper(tmp), PlaceTemp},
		{"обычная папка", filepath.Join(other, "SosninaAlbums.exe"), tmp, PlaceNormal},
		{"сосед по префиксу", tmp + "2" + string(filepath.Separator) + "a.exe", tmp, PlaceNormal},
		{"AppTranslocation", mac, tmp, PlaceTranslocated},
		{"AppTranslocation, другой регистр", strings.ToLower(mac), tmp, PlaceTranslocated},
		{"AppTranslocation без temp", mac, "", PlaceTranslocated},
		{"слово только в имени папки", filepath.Join(other, "AppTranslocationTool", "a"), tmp, PlaceNormal},
		{"пустой exe", "", tmp, PlaceNormal},
		{"пустой temp", exe, "", PlaceNormal},
		{"обе пустые", "", "", PlaceNormal},
		{"temp — корень, не временная папка", exe, string(filepath.Separator), PlaceNormal},
	}
	for _, c := range cases {
		if got := LaunchPlace(c.exe, c.temp); got != c.want {
			t.Errorf("%s: LaunchPlace(%q, %q) = %q, ждали %q", c.name, c.exe, c.temp, got, c.want)
		}
	}
}

// Пути Windows строкой, без диска: обратную косую черту разбирает только Windows.
func TestLaunchPlaceWindowsStrings(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("н/п: пути с обратной косой чертой разбирает только Windows")
	}
	const temp = `C:\Users\Ilya\AppData\Local\Temp`
	cases := []struct {
		exe, temp string
		want      Place
	}{
		{temp + `\Temp1_album.zip\SosninaAlbums.exe`, temp, PlaceTemp},
		{`c:\users\ilya\appdata\local\temp\Rar$EXa1.2\a.exe`, temp + `\`, PlaceTemp},
		{temp + `\7zO12AB\a.exe`, temp, PlaceTemp},
		{`D:\Albums\SosninaAlbums.exe`, temp, PlaceNormal},
		{`C:\Users\Ilya\Downloads\album\SosninaAlbums.exe`, temp, PlaceNormal},
		{temp + `orary\a.exe`, temp, PlaceNormal},
		{`C:\Temp2\a.exe`, `C:\Temp`, PlaceNormal},
		{`C:\a.exe`, `C:\`, PlaceNormal},
	}
	for _, c := range cases {
		if got := LaunchPlace(c.exe, c.temp); got != c.want {
			t.Errorf("LaunchPlace(%q, %q) = %q, ждали %q", c.exe, c.temp, got, c.want)
		}
	}
}

func TestPlaceNotice(t *testing.T) {
	if PlaceNormal.Notice() != "" {
		t.Error("для обычного места текст должен быть пустым")
	}
	if n := PlaceTemp.noticeFor("windows"); !strings.Contains(n, "из архива") || !strings.Contains(n, "Извлечь все") {
		t.Errorf("текст для temp: %q", n)
	}
	if n := PlaceTranslocated.Notice(); !strings.Contains(n, "«Программы»") {
		t.Errorf("текст для translocation: %q", n)
	}
}

// Тост «из архива» по ОС: на Linux своё слово — без exe и «Извлечь все»; на Windows и Mac прежний;
// остальные места от ОС не зависят.
func TestNoticeTempPerOS(t *testing.T) {
	lin := PlaceTemp.noticeFor("linux")
	if !strings.Contains(lin, "из архива") || !strings.Contains(lin, ".deb") ||
		strings.Contains(lin, ".exe") || strings.Contains(lin, "Извлечь все") {
		t.Errorf("текст Linux: %q", lin)
	}
	for _, goos := range []string{"windows", "darwin"} {
		if n := PlaceTemp.noticeFor(goos); n != noticeTemp {
			t.Errorf("%s: %q", goos, n)
		}
	}
	for _, goos := range []string{"linux", "windows", "darwin"} {
		if PlaceNormal.noticeFor(goos) != "" || PlaceTranslocated.noticeFor(goos) != noticeTranslocated {
			t.Errorf("%s: места normal/translocated изменились", goos)
		}
	}
	if PlaceTemp.Notice() != PlaceTemp.noticeFor(runtime.GOOS) {
		t.Error("Notice() не по текущей ОС")
	}
}
