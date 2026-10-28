package osx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestWebviewDataDir(t *testing.T) {
	base := testkit.TempDir(t)
	t.Setenv("LOCALAPPDATA", base)
	if got, want := WebViewDir(DataName("")), filepath.Join(base, "SosninaAlbums", "WebView2"); got != want {
		t.Fatalf("WebViewDir() = %q, ждали %q", got, want)
	}
	// Папка не зависит от имени exe: у каждой версии оно своё.
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("XDG_CACHE_HOME", base)
	t.Setenv("HOME", base)
	if got := WebViewDir(DataName("")); got != "" && filepath.Base(got) != "WebView2" {
		t.Fatalf("запасной путь: %q", got)
	}
}

// Без SA_STAND (нет или пусто) — прежние имена побитно: настоящие настройки и WebView2 на месте.
func TestStandOffKeepsNames(t *testing.T) {
	for _, v := range []string{"", "unset"} {
		t.Setenv(StandEnv, "") // вернёт прежнее значение после теста
		if v == "unset" {
			if err := os.Unsetenv(StandEnv); err != nil {
				t.Fatal(err)
			}
		}
		got, err := Stand()
		if err != nil || got != "" {
			t.Fatalf("%s: Stand() = %q, %v", v, got, err)
		}
		if DataName(got) != "SosninaAlbums" {
			t.Fatalf("%s: DataName = %q", v, DataName(got))
		}
	}
}

// Несколько стендов сразу: у каждого своя папка рядом с основной, регистр не плодит вторую.
func TestStandNames(t *testing.T) {
	base := testkit.TempDir(t)
	t.Setenv("LOCALAPPDATA", base)
	seen := map[string]string{}
	for _, in := range []string{"b", "C", "qa2", "0123456789abcdef"} {
		s, err := ParseStand(in)
		if err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		name := DataName(s)
		if name != "SosninaAlbums-stand-"+strings.ToLower(in) {
			t.Fatalf("%q: DataName = %q", in, name)
		}
		if prev, ok := seen[name]; ok {
			t.Fatalf("%q и %q делят папку %q", in, prev, name)
		}
		seen[name] = in
		if got, want := WebViewDir(name), filepath.Join(base, name, "WebView2"); got != want {
			t.Fatalf("%q: WebViewDir = %q, ждали %q", in, got, want)
		}
	}
	if s, _ := ParseStand("B"); s != "b" {
		t.Fatalf("регистр: %q", s)
	}
}

// Имя стенда входит в путь: всё, что может выйти из папки данных или сменить диск, — отказ.
func TestStandRejectsEscape(t *testing.T) {
	bad := []string{
		"..", `..\x`, "../x", `a\b`, "a/b", `C:\x`, "C:", `\\srv\share`, "/abs",
		".", "a.b", "a b", " b", "b-1", "b_1", "стенд", "b\x00", "0123456789abcdefg",
	}
	for _, in := range bad {
		if s, err := ParseStand(in); err == nil {
			t.Errorf("%q принят как %q", in, s)
		}
	}
	// Принятое имя — ровно один элемент пути внутри базы.
	base := testkit.TempDir(t)
	for _, in := range []string{"b", "z9"} {
		s, err := ParseStand(in)
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(base, DataName(s))
		if filepath.Dir(p) != base {
			t.Fatalf("%q вышел из базы: %q", in, p)
		}
	}
}

// Через переменную: в сборке dev — те же правила и отказ (NUL в окружение не положить, поэтому
// выборочно); в выпуске SA_STAND не читается вовсе — ни верное, ни неверное значение
// (забытая в системе переменная не должна ни менять данные, ни мешать запуску).
func TestStandEnv(t *testing.T) {
	for _, in := range []string{`..\..\x`, `C:\Users`} {
		t.Setenv(StandEnv, in)
		s, err := Stand()
		if standEnabled && err == nil {
			t.Errorf("SA_STAND=%q принят как %q", in, s)
		}
		if !standEnabled && (err != nil || s != "") {
			t.Errorf("выпуск: SA_STAND=%q дал %q, %v", in, s, err)
		}
	}
	for _, in := range []string{"b", "Qa2"} {
		t.Setenv(StandEnv, in)
		s, err := Stand()
		want := strings.ToLower(in)
		if !standEnabled {
			want = ""
		}
		if err != nil || s != want {
			t.Errorf("SA_STAND=%q: Stand() = %q, %v; ждали %q", in, s, err, want)
		}
		if got := DataName(s); standEnabled == (got == "SosninaAlbums") {
			t.Errorf("SA_STAND=%q: DataName = %q", in, got)
		}
	}
}

// standFrom без тега dev: при enabled=false окружение не читается вовсе, при true — имя
// проверяется и сводится к нижнему регистру, пустое и неверное разобраны как в ParseStand.
func TestStandFrom(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k != StandEnv {
				t.Errorf("читается %q, ждали %q", k, StandEnv)
			}
			return v
		}
	}
	failEnv := func(string) string {
		t.Error("getenv вызван при выключенных стендах")
		return ""
	}
	if s, err := standFrom(false, failEnv); s != "" || err != nil {
		t.Errorf("enabled=false: %q, %v", s, err)
	}
	for _, tc := range []struct {
		in, want string
		bad      bool
	}{
		{"", "", false},
		{"b", "b", false},
		{"Qa2", "qa2", false},
		{`..\..\x`, "", true},
		{`C:\Users`, "", true},
		{strings.Repeat("a", standMax+1), "", true},
	} {
		s, err := standFrom(true, env(tc.in))
		if (err != nil) != tc.bad || s != tc.want {
			t.Errorf("enabled=true, %q: %q, %v; ждали %q, ошибка=%v", tc.in, s, err, tc.want, tc.bad)
		}
	}
}

// Относительный LOCALAPPDATA и пустой кэш пользователя — папки данных WebView2 нет (""), а не путь
// от текущей папки процесса (140 Н4).
func TestWebviewDirNeverRelative(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("APPDATA", "")
	for _, v := range []string{"", "rel/dir", "."} {
		t.Setenv("LOCALAPPDATA", v)
		if got := WebViewDir(DataName("")); got != "" {
			t.Errorf("LOCALAPPDATA=%q: WebViewDir() = %q, ждали пусто", v, got)
		}
	}
}
