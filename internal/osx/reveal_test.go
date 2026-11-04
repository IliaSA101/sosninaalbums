package osx

import (
	"errors"
	"net/url"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// fakeRun подменяет запуск с ожиданием и без; возвращает записи argv обоих.
func fakeRun(t *testing.T, runErr error) (ran, started *[][]string) {
	t.Helper()
	oldRun, oldStart := runCmd, startCmd
	var r, s [][]string
	runCmd = func(c *exec.Cmd) error { r = append(r, c.Args); return runErr }
	startCmd = func(c *exec.Cmd) error { s = append(s, c.Args); return nil }
	t.Cleanup(func() { runCmd, startCmd = oldRun, oldStart })
	return &r, &s
}

// Трудные пути: пробелы, кириллица, кавычки, запятая, скобки, «;», «$», решётка, проценты.
var hardPaths = []string{
	"/home/u/Фото свадьбы/IMG 1.jpg",
	`/home/u/Иванов,Петрова/"кавычки" 'и' апостроф.jpg`,
	"/home/u/a;b $(rm -rf x) `y` & z|w.jpg",
	"/home/u/100% #1 (копия) [2].jpg",
	"/mnt/диск/Анна  Пётр/файл\tтаб.jpg",
}

// Команда по ОС — чистая функция: Linux и Mac дают argv, Windows — nil (там атрибуты).
func TestRevealArgv(t *testing.T) {
	const p = "/home/u/Фото свадьбы/a.jpg"
	cases := []struct {
		goos string
		file bool
		want []string
	}{
		{"darwin", true, []string{"open", "-R", p}},
		{"darwin", false, []string{"open", p}},
		{"linux", false, []string{"xdg-open", p}},
		{"windows", true, nil},
		{"windows", false, nil},
		{"plan9", true, nil},
	}
	for _, c := range cases {
		if got := revealArgv(c.goos, p, c.file); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s file=%v: %q, ждали %q", c.goos, c.file, got, c.want)
		}
	}
	got := revealArgv("linux", p, true)
	want := []string{
		"dbus-send", "--session", "--print-reply", "--reply-timeout=5000",
		"--dest=org.freedesktop.FileManager1", "/org/freedesktop/FileManager1",
		"org.freedesktop.FileManager1.ShowItems",
		"array:string:file:///home/u/%D0%A4%D0%BE%D1%82%D0%BE%20%D1%81%D0%B2%D0%B0%D0%B4%D1%8C%D0%B1%D1%8B/a.jpg", "string:",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ShowItems: %q, ждали %q", got, want)
	}
}

// Путь — ровно один аргумент без оболочки, что бы в нём ни стояло; URI не содержит запятых.
func TestRevealLinuxShowItems(t *testing.T) {
	for _, p := range hardPaths {
		ran, started := fakeRun(t, nil)
		if err := revealLinux(p, true); err != nil {
			t.Fatal(err)
		}
		if len(*ran) != 1 || len(*started) != 0 {
			t.Fatalf("%q: запусков run %d, start %d; ждали 1 и 0", p, len(*ran), len(*started))
		}
		argv := (*ran)[0]
		if len(argv) != 9 || argv[0] != "dbus-send" || argv[7] != "array:string:"+fileURI(p) {
			t.Fatalf("%q: argv %q", p, argv)
		}
		uri := strings.TrimPrefix(argv[7], "array:string:")
		if strings.ContainsAny(uri, ", \"'`$;&|()[]#\t") {
			t.Errorf("%q: в URI остался опасный знак: %s", p, uri)
		}
		u, err := url.Parse(uri)
		if err != nil || u.Path != p {
			t.Errorf("%q: URI не возвращает путь: %v %v", p, err, u)
		}
	}
}

// Нет службы, нет dbus-send, ошибка ответа — запасной путь: xdg-open папки файла.
func TestRevealLinuxFallback(t *testing.T) {
	for _, p := range hardPaths {
		ran, started := fakeRun(t, errors.New("нет службы"))
		if err := revealLinux(p, true); err != nil {
			t.Fatal(err)
		}
		dir := p[:strings.LastIndex(p, "/")]
		if len(*ran) != 1 || len(*started) != 1 || !reflect.DeepEqual((*started)[0], []string{"xdg-open", dir}) {
			t.Errorf("%q: run %q, start %q; ждали xdg-open %q", p, *ran, *started, dir)
		}
	}
}

// Папка — сразу xdg-open, dbus не зовётся; ошибка запуска xdg-open доходит до вызывающего.
func TestRevealLinuxFolder(t *testing.T) {
	for _, p := range hardPaths {
		ran, started := fakeRun(t, nil)
		if err := revealLinux(p, false); err != nil {
			t.Fatal(err)
		}
		if len(*ran) != 0 || !reflect.DeepEqual(*started, [][]string{{"xdg-open", p}}) {
			t.Errorf("%q: run %q, start %q", p, *ran, *started)
		}
	}
	fakeRun(t, errors.New("нет dbus"))
	startCmd = func(*exec.Cmd) error { return errors.New("нет xdg-open") }
	if err := revealLinux("/a", false); err == nil {
		t.Error("ошибка запуска потеряна")
	}
	if err := revealLinux("/a/b.jpg", true); err == nil {
		t.Error("ошибка запасного пути потеряна")
	}
}

// Кодирование: безопасные знаки и «/» как есть, всё прочее — %XX по байтам UTF-8; пусто и корень.
func TestFileURI(t *testing.T) {
	cases := map[string]string{
		"":                  "file://",
		"/":                 "file:///",
		"/a-b_c.d~e/F1":     "file:///a-b_c.d~e/F1",
		"/a b":              "file:///a%20b",
		"/a,b":              "file:///a%2Cb",
		`/"q"`:              "file:///%22q%22",
		"/Ж":                "file:///%D0%96",
		"/100%":             "file:///100%25",
		"/x\nу":             "file:///x%0A%D1%83",
		"/\x00\xff":         "file:///%00%FF",
		"/a?b#c":            "file:///a%3Fb%23c",
		"/a:b@c+d=e&f;g$":   "file:///a%3Ab%40c%2Bd%3De%26f%3Bg%24",
		"/(x)[y]'z'`w`|<>*": "file:///%28x%29%5By%5D%27z%27%60w%60%7C%3C%3E%2A",
	}
	for in, want := range cases {
		if got := fileURI(in); got != want {
			t.Errorf("fileURI(%q) = %q, ждали %q", in, got, want)
		}
	}
}

// Относительный путь с «-» впереди не станет флагом xdg-open.
func TestRevealArgvDash(t *testing.T) {
	if got := revealArgv("linux", "-h", false); got[1] != "./-h" {
		t.Errorf("%q", got)
	}
	if got := revealArgv("linux", "/-h", false); got[1] != "/-h" {
		t.Errorf("%q", got)
	}
}
