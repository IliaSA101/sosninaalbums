package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// diskOK — диск «есть и каталог»: путь настоящий сам себе.
func diskOK(p string) (string, error) { return p, nil }

// diskNever — шаг 1 должен отказать до диска: любое обращение к диску — провал теста.
func diskNever(t *testing.T) func(string) (string, error) {
	return func(p string) (string, error) {
		t.Errorf("обращение к диску для %q: строка должна была отказать раньше", p)
		return "", errors.New("диск")
	}
}

func unixRules(goos string, disk func(string) (string, error)) *folderRules {
	home := "/home/u"
	if goos == "darwin" {
		home = "/Users/u"
	}
	r := &folderRules{
		fold: func(s string) string { return s }, homes: []string{home}, timeout: time.Second, disk: disk,
		whole: []string{home + "/Документы", home + "/Рабочий стол", home + "/Загрузки"},
		ours:  []string{home + "/.config", home + "/.cache"},
	}
	r.trees, r.allow, r.exact, r.elems = osLists(goos, home, func(string) string { return "" })
	return r
}

func winRules(disk func(string) (string, error)) *folderRules {
	env := map[string]string{
		"SystemRoot": `C:\Windows`, "ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`,
		"ProgramData": `C:\ProgramData`, "AppData": `C:\Users\u\AppData\Roaming`, "LocalAppData": `C:\Users\u\AppData\Local`,
	}
	home := `C:\Users\u`
	r := &folderRules{
		windows: true, fold: strings.ToLower, homes: []string{home}, timeout: time.Second, disk: disk,
		whole: []string{home + `\Documents`, home + `\Desktop`, home + `\Downloads`},
		ours:  []string{env["AppData"], env["LocalAppData"]},
	}
	r.trees, r.allow, r.exact, r.elems = osLists("windows", home, func(k string) string { return env[k] })
	return r
}

// Правило папки (К2): запрещённое — отказ нужного вида, разрешённое — проходит. Windows-случаи
// идут через правила Windows на любой ОС, macOS — через свои списки.
func TestAgentFolderRules(t *testing.T) {
	type c struct {
		p    string
		want error // nil — проходит
	}
	wide, ours := errFolderTooWide, errFolderOurs
	groups := []struct {
		name  string
		rules *folderRules
		cases []c
	}{
		{"linux", unixRules("linux", diskOK), []c{
			{"/", wide}, {"/home", wide}, {"/home/u", wide}, {"/home/u/", wide}, {"/etc", wide}, {"/etc/ssh", wide},
			{"/usr/share", wide}, {"/proc/1", wide}, {"/sys/x", wide}, {"/dev/shm", wide}, {"/var/log", wide},
			{"/boot", wide}, {"/snap/x", wide}, {"/net/h", wide}, {"/Network/x", wide}, {"/afs/x", wide},
			{"/run", wide}, {"/run/user/1000", wide}, {"/run/media", wide}, {"/media", wide}, {"/mnt", wide},
			{"/home/u/.ssh", wide}, {"/home/u/Pictures/.hidden/x", wide},
			{"/home/u/Pictures/../.ssh", wide}, {"/home/u/Pictures/../../../etc", wide},
			{"/home/u/Документы", wide}, {"/home/u/Рабочий стол", wide}, {"/home/u/Загрузки/", wide},
			{"/home/u/.config", ours}, {"/home/u/.config/x/y", ours}, {"/home/u/.cache/thumbnails", ours},
			{"/home/u/Pictures", nil}, {"/home/u/Pictures/Свадьба", nil}, {"/home/u/Pictures/Свадьба/", nil},
			{"/media/u/disk/2026", nil}, {"/run/media/u/disk/Фото", nil}, {"/mnt/disk/Photos", nil},
			{"/home/u/Документы/Свадьба", nil}, {"/home/u/Загрузки/2026", nil}, {"/home/u/Рабочий стол/Съёмка", nil},
			{"/data/x", nil}, {"/home/uu", nil}, {"/home/u/Pictures/a.b", nil},
		}},
		{"macos", unixRules("darwin", diskOK), []c{
			{"/System/Library", wide}, {"/Library/x", wide}, {"/private/var", wide}, {"/usr/local", wide},
			{"/Users", wide}, {"/Users/u", wide}, {"/Users/u/Library", wide}, {"/Users/u/Library/Mail", wide},
			{"/Volumes", wide}, {"/Users/u/Pictures/Свадьба", nil}, {"/Volumes/Photos/2026", nil},
		}},
		{"windows", winRules(diskOK), []c{
			{`C:\`, wide}, {`D:\`, wide}, {`D:/`, wide}, {`C:\Users`, wide}, {`C:\Users\u`, wide}, {`c:\users\U`, wide},
			{`C:\Windows`, wide}, {`C:\WINDOWS\System32`, wide}, {`C:\Program Files\x`, wide},
			{`C:\Program Files (x86)`, wide}, {`C:\ProgramData\x`, wide}, {`C:\Users\u\Documents`, wide},
			{`C:\Users\u\desktop`, wide}, {`C:\Users\u\Documents\..`, wide}, {`C:\Users\u\Pictures\..\..`, wide},
			{`D:\$Recycle.Bin\S-1`, wide}, {`D:\System Volume Information`, wide},
			{`C:\Users\u\AppData\Local\Temp`, ours}, {`C:\Users\u\AppData\Roaming`, ours}, {`c:\users\u\appdata\local`, ours},
			{`C:\Users\u\AppData\Other`, wide},
			{`D:\Фото\Петровы`, nil}, {`D:\Фото\Петровы\`, nil}, {`d:/фото/x`, nil}, {`E:\2026`, nil},
			{`C:\Users\u\Pictures\Свадьба`, nil}, {`C:\Users\u\Documents\Свадьба`, nil}, {`C:\Users\uu`, nil},
			{`D:\.git\x`, nil}, // точка в начале — правило Unix; на Windows это обычное имя
		}},
	}
	for _, g := range groups {
		for _, k := range g.cases {
			err := g.rules.folder(k.p)
			if !errors.Is(err, k.want) || (k.want == nil) != (err == nil) {
				t.Errorf("%s %q: %v, ждали %v", g.name, k.p, err, k.want)
			}
			dir, ok := g.rules.hint(k.p)
			if ok != (k.want == nil) || ok && dir != g.rules.clean(k.p) {
				t.Errorf("%s %q: подсказка %q ok=%v, ждали ok=%v", g.name, k.p, dir, ok, k.want == nil)
			}
			if err != nil && strings.Contains(err.Error(), k.p) {
				t.Errorf("%s %q: путь в тексте ошибки: %v", g.name, k.p, err)
			}
		}
	}
}

// Подсказка не доверяется: плохая строка отказывает до диска (диск не трогаем), на обеих ОС.
func TestFolderHintNotTrusted(t *testing.T) {
	bad := []string{
		`\\?\C:\x`, `\\srv\s`, `//srv/s`, `C:x`, `C:\a:b`, `\\.\PhysicalDrive0`, "",
		"/home/u/a\u2028b", "/home/u/a\u3164", "/home/u/\u202Eexe.jpg", "/home/u/a\u200Bb", "/home/u/a\u00A0b",
		"/home/u/e\u0301\u0302\u0303", "/home/u/a\x00b", "/home/u/\xff", "relative/dir", "./x",
		"/home/u/" + strings.Repeat("a", 4096), // 4097 байт и больше
	}
	winBad := append([]string{`C:\a\` + strings.Repeat("b", 4097), "C:\\Фото\\a\u2028b", `\\?\UNC\h\s`, `\??\C:\x`}, bad...)
	for _, c := range []struct {
		name  string
		rules *folderRules
		list  []string
	}{
		{"unix", unixRules("linux", diskNever(t)), bad},
		{"windows", winRules(diskNever(t)), winBad},
	} {
		for _, s := range c.list {
			if _, ok := c.rules.hint(s); ok {
				t.Errorf("%s %.40q: подсказка принята", c.name, s)
			}
			if err := c.rules.folder(s); !errors.Is(err, errFolderTooWide) {
				t.Errorf("%s %.40q: %v, ждали errFolderTooWide", c.name, s, err)
			}
		}
	}
	// Ровно 4096 байт — ещё в пределе, до диска доходит.
	long := "/data/" + strings.Repeat("a", 4096-len("/data/"))
	reached := false
	r := unixRules("linux", func(p string) (string, error) { reached = true; return p, nil })
	if _, ok := r.hint(long); !ok || !reached {
		t.Errorf("4096 байт: ok=%v диск=%v", ok, reached)
	}
}

// Диск висит (мёртвый NFS): отказ за срок, а не вечное ожидание единственного места правки.
func TestFolderHintHang(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	r := unixRules("linux", func(p string) (string, error) { <-release; return p, nil })
	r.timeout = 50 * time.Millisecond
	start := time.Now()
	if err := r.folder("/media/u/disk/2026"); !errors.Is(err, errFolderTooWide) {
		t.Fatalf("висящий диск: %v, ждали отказ", err)
	}
	if _, ok := r.hint("/media/u/disk/2026"); ok {
		t.Fatal("подсказка принята при висящем диске")
	}
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Fatalf("отказ занял %v при сроке 50 мс", d)
	}
	// Паника диска — тоже отказ, а не падение программы.
	r.disk = func(string) (string, error) { panic("диск") }
	if err := r.folder("/media/u/disk/2026"); err == nil {
		t.Fatal("паника диска не дала отказ")
	}
}

// Ссылка, ведущая в корень или домашнюю, отказывает по настоящему пути (шаг 2 + повтор шага 1),
// обычная ссылка и обычная папка проходят; файл вместо папки и несуществующая — отказ.
func TestFolderHintSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("символические ссылки Windows требуют прав; правило путей то же")
	}
	base, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(base, "home")
	shoot := filepath.Join(base, "work", "shoot")
	for _, d := range []string{home, shoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(base, "work", "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := func(name, target string) string {
		p := filepath.Join(base, "work", name)
		if err := os.Symlink(target, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	toRoot, toHome, toShoot, toFile := link("toRoot", "/"), link("toHome", home), link("toShoot", shoot), link("toFile", file)
	r := &folderRules{
		fold: func(s string) string { return s }, homes: []string{home}, timeout: time.Second, disk: diskProbe,
	}
	r.trees, r.allow, r.exact, r.elems = osLists("linux", home, func(string) string { return "" })
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{toRoot, false}, {toHome, false}, {toShoot, true}, {shoot, true},
		{toFile, false}, {file, false}, {filepath.Join(base, "work", "нет"), false},
	} {
		err := r.folder(c.p)
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, ok должно быть %v", filepath.Base(c.p), err, c.ok)
		}
		if _, ok := r.hint(c.p); ok != c.ok {
			t.Errorf("%s: подсказка ok=%v, ждали %v", filepath.Base(c.p), ok, c.ok)
		}
	}
}

// Пакетные функции с правилами этой машины отказывают на домашней, корне и относительном пути.
func TestAgentFolderDefaults(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		t.Skip("домашней папки нет")
	}
	if err := defaultFolderRules().folder(home); err == nil {
		t.Error("домашняя папка принята")
	}
	if err := defaultFolderRules().folder(string(filepath.Separator)); err == nil && runtime.GOOS != "windows" {
		t.Error("корень принят")
	}
	if _, ok := defaultFolderRules().hint("relative"); ok {
		t.Error("относительный путь принят")
	}
}

// Показ подсказки: только последний элемент, не больше 60 рун, лишнее «…»; негодная — пусто.
func TestAskHintDisplay(t *testing.T) {
	long := strings.Repeat("Я", 80)
	for _, c := range []struct {
		dir, want string
		windows   bool
	}{
		{"/media/u/Свадьба Петровых", "Свадьба Петровых", false},
		{"/a/" + strings.Repeat("ё", 60), strings.Repeat("ё", 60), false},
		{"/a/" + strings.Repeat("ё", 61), strings.Repeat("ё", 59) + "…", false},
		{"/a/" + long, strings.Repeat("Я", 59) + "…", false},
		{`D:\Фото\Петровы`, "Петровы", true},
		{`D:/Фото/x`, "x", true},
		{`/a/b\c`, `b\c`, false}, // обратная косая на Unix — часть имени
		{"/", "", false},
	} {
		got := lastElem(c.dir, c.windows)
		if got != c.want {
			t.Errorf("%q: %q, ждали %q", c.dir, got, c.want)
		}
		if n := len([]rune(got)); n > hintShownMax {
			t.Errorf("%q: %d рун", c.dir, n)
		}
	}
	// Ни путь целиком, ни родитель не показываются.
	if got := lastElem("/home/u/secret/Свадьба", false); strings.Contains(got, "secret") {
		t.Errorf("родитель в подсказке: %q", got)
	}
	if dir, label := agentHintShown(defaultFolderRules(), "relative/x"); dir != "" || label != "" {
		t.Errorf("негодная подсказка: %q %q", dir, label)
	}
}
