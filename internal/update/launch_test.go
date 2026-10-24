package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Командная строка установщика: тихо, с перезапуском и /D= без кавычек даже при пробелах в
// пути; переносная копия — обычный мастер без аргументов.
func TestInstallerCmdLine(t *testing.T) {
	setupPath := `C:\Users\Мария Иванова\AppData\Local\SosninaAlbums\Updates\SosninaAlbums-0.25.1-windows-x64-setup.exe`
	dir := `C:\Users\Мария Иванова\AppData\Local\Programs\Sosnina Albums`
	got, err := CmdLine(setupPath, ModeInstall, dir)
	want := `"` + setupPath + `" /S /relaunch /D=` + dir
	if err != nil || got != want {
		t.Fatalf("install: %q %v", got, err)
	}
	if strings.Contains(got, `/D="`) || !strings.HasSuffix(got, dir) {
		t.Error("/D= должен идти последним и без кавычек")
	}
	for _, m := range []Mode{ModeSetup, ModePage} {
		got, err = CmdLine(setupPath, m, dir)
		if err != nil || got != `"`+setupPath+`"` {
			t.Errorf("%s: %q %v", m, got, err)
		}
	}
	// У переносной копии папка не нужна и не проверяется.
	if _, err := CmdLine(setupPath, ModeSetup, `a"b`); err != nil {
		t.Errorf("setup с плохой папкой: %v", err)
	}
	for name, c := range map[string][2]string{
		"кавычка в установщике": {`C:\a"b.exe`, dir},
		"перевод строки":        {"C:\\a\nb.exe", dir},
		"пустой установщик":     {"", dir},
		"кавычка в папке":       {setupPath, `C:\x" /evil`},
		"пустая папка":          {setupPath, ""},
		"NUL":                   {setupPath, "C:\\x\x00"},
	} {
		if _, err := CmdLine(c[0], ModeInstall, c[1]); !errors.Is(err, ErrPath) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestModeOf(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "uninstall.exe"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	other := t.TempDir() // переносная: uninstall.exe нет
	exe := func(d, n string) string { return filepath.Join(d, n) }
	for name, c := range map[string]struct {
		goos, exe string
		want      Mode
	}{
		"установленная":                {"windows", exe(dir, "SosninaAlbums.exe"), ModeInstall},
		"регистр имени":                {"windows", exe(dir, "sosninaalbums.EXE"), ModeInstall},
		"переносная":                   {"windows", exe(other, "SosninaAlbums-0.25.1.exe"), ModeSetup},
		"переносная с таким же именем": {"windows", exe(other, "SosninaAlbums.exe"), ModeSetup},
		"имя с версией рядом с uninst": {"windows", exe(dir, "SosninaAlbums-0.25.1.exe"), ModeSetup},
		"путь без папки":               {"windows", "SosninaAlbums.exe", ModeSetup},
		"пустой путь":                  {"windows", "", ModeSetup},
		"mac":                          {"darwin", exe(dir, "SosninaAlbums.exe"), ModePage},
		"linux":                        {"linux", "/usr/bin/sosninaalbums", ModePage},
	} {
		if got := ModeOf(c.goos, c.exe); got != c.want {
			t.Errorf("%s: %s, ждали %s", name, got, c.want)
		}
	}
	// Папка с именем uninstall.exe — не файл.
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "uninstall.exe"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := ModeOf("windows", filepath.Join(d, "SosninaAlbums.exe")); got != ModeSetup {
		t.Errorf("папка uninstall.exe: %s", got)
	}
	// Путь Windows с обратными слэшами при проверке на другой ОС: файл ищется рядом по строке.
	seen := ""
	got := modeOf("windows", `C:\Programs\Sosnina Albums\SosninaAlbums.exe`, func(p string) bool { seen = p; return true })
	if got != ModeInstall || seen != `C:\Programs\Sosnina Albums\uninstall.exe` {
		t.Errorf("слэши: %s %q", got, seen)
	}
}

// Запуск установщика есть только в Windows; здесь — отказ, процесс не создан.
func TestStartNotWindows(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Windows")
	}
	if err := Start("/bin/true", `"/bin/true"`); err == nil {
		t.Error("Start на не Windows запустил процесс")
	}
}

func TestStandSource(t *testing.T) {
	pub, _ := testPair()
	key := "BR5F3oQ9KqLpLmGUCHevrz882GSa9KU2cm4noZnW+0Q="
	s, ok := Stand("http://127.0.0.1:8765/", key)
	if !ok || s.CatalogURL != "http://127.0.0.1:8765/"+ManifestName || s.Prefix != "http://127.0.0.1:8765/" || !s.LocalHTTP ||
		len(s.Keys) != 1 || !s.Keys[0].Equal(pub) {
		t.Fatalf("стенд: %v %+v", ok, s)
	}
	if got := New(s).setupURL("0.25.1"); got != "http://127.0.0.1:8765/v0.25.1/"+SetupName("0.25.1") {
		t.Errorf("адрес установщика: %q", got)
	}
	// Не loopback, без «/» в конце, с параметрами, с именем пользователя, чужая схема — не стенд.
	for _, base := range []string{"", "http://10.0.0.1/", "http://127.0.0.1:1/x", "http://127.0.0.1:1/?a=1", "http://u@127.0.0.1:1/",
		"ftp://127.0.0.1/", "http://localhost:8765/", "https://github.com/", "http://127.0.0.1:1/#x"} {
		if _, ok := Stand(base, key); ok {
			t.Errorf("%q принят", base)
		}
	}
	// Ключ не задан или негоден — ключи выпуска (подпись тестовым не сойдётся).
	s, _ = Stand("http://127.0.0.1:8765/", "!!")
	if len(s.Keys) != len(fetch.PublicKeys(Keys[:])) {
		t.Errorf("ключи по умолчанию: %d", len(s.Keys))
	}
}

// Источник по умолчанию: выпуск — GitHub, проверка при запуске разрешена, переменные не читаются;
// dev — стенд из переменных, без них GitHub и без проверки при запуске.
func TestStandUpdateSource(t *testing.T) {
	env := map[string]string{"SA_UPDATE_URL": "http://127.0.0.1:8765/", "SA_UPDATE_KEY": ""}
	get := func(k string) string { return env[k] }
	s, auto := defaultFrom(false, get)
	if !auto || s.CatalogURL != ManifestURL || s.LocalHTTP || s.Roots != nil {
		t.Errorf("выпуск: %v %+v", auto, s)
	}
	s, auto = defaultFrom(true, get)
	if !auto || s.CatalogURL != "http://127.0.0.1:8765/"+ManifestName || !s.LocalHTTP {
		t.Errorf("dev со стендом: %v %+v", auto, s)
	}
	env["SA_UPDATE_URL"] = "http://10.0.0.1/"
	s, auto = defaultFrom(true, get)
	if auto || s.CatalogURL != ManifestURL {
		t.Errorf("dev с чужим адресом: %v %+v", auto, s)
	}
	delete(env, "SA_UPDATE_URL")
	if _, auto = defaultFrom(true, get); auto {
		t.Error("dev без стенда проверяет при запуске")
	}
	if s, auto := Default(); standEnabled == false && (!auto || s.CatalogURL != ManifestURL) {
		t.Errorf("Default в выпуске: %v %+v", auto, s)
	}
}

// Источник выпуска: https, три хоста GitHub, ключи — те, что в keys.go (пока заглушки: допустимы
// пустые, но не негодные).
func TestGitHubSource(t *testing.T) {
	g := GitHub()
	if g.LocalHTTP || g.Roots != nil || g.CatalogURL != ManifestURL || g.Prefix != AssetPrefix || len(g.Hosts) != 3 {
		t.Errorf("GitHub: %+v", g)
	}
	for _, k := range Keys {
		if k != "" && len(fetch.PublicKeys([]string{k})) != 1 {
			t.Errorf("негодный ключ %q", k)
		}
	}
	for _, u := range []string{ManifestURL, AssetPrefix} {
		if !strings.HasPrefix(u, "https://github.com/IliaSA101/sosninaalbums/releases/") {
			t.Errorf("адрес %q", u)
		}
	}
}
