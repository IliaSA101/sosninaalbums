package agentplaces

// Списки ОС для правила папки агента (rules.go, design §2.3) и сборка правил: Default для продукта
// (правила этой машины), New для тестов.

import (
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// osLists — системные деревья, исключения, широкие папки и служебные элементы ОС goos. Чистая
// функция: Windows-списки проверяются таблицей на любой ОС. env — GetEnv (подмена в тестах).
func osLists(goos, home string, env func(string) string) (trees, allow, exact, elems []string) {
	if goos == "windows" {
		return windowsLists(home, env)
	}
	// Linux и macOS вместе: чужие имена в списке другой ОС безвредны (нет такой папки — нечего
	// выбирать), а `/Library` и `/System` на Linux никто не держит.
	trees = []string{
		"/net", "/Network", "/afs", "/proc", "/sys", "/dev", "/run", "/etc", "/usr", "/var", "/boot",
		"/snap", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/System", "/Library", "/private",
	}
	if goos == "darwin" && home != "" {
		trees = append(trees, path.Join(home, "Library"))
	}
	allow = []string{"/run/media"}
	exact = []string{"/home", "/Users", "/media", "/mnt", "/Volumes", "/opt", "/srv", "/tmp", "/run/media"}
	return trees, allow, exact, nil
}

func windowsLists(home string, env func(string) string) (trees, allow, exact, elems []string) {
	drive := env("SystemDrive")
	if drive == "" {
		drive = "C:"
	}
	for _, c := range []struct{ key, def string }{
		{"SystemRoot", drive + `\Windows`}, {"ProgramFiles", drive + `\Program Files`},
		{"ProgramFiles(x86)", drive + `\Program Files (x86)`}, {"ProgramW6432", ""},
		{"ProgramData", drive + `\ProgramData`},
		{"AppData", ""}, {"LocalAppData", ""},
	} {
		v := env(c.key)
		if v == "" {
			v = c.def
		}
		if v != "" {
			trees = append(trees, v)
		}
	}
	if home != "" {
		// Если переменных нет, папки данных программ лежат здесь же.
		trees = append(trees, home+`\AppData`)
	}
	exact = []string{drive + `\Users`}
	elems = []string{"system volume information", "$recycle.bin"}
	return trees, nil, exact, elems
}

// Opts — то, из чего New собирает правила для тестов. Нули — «ничего»: пустые списки разрешают всё.
type Opts struct {
	Windows bool                                        // таблицы и разделитель Windows
	Fold    func(string) string                         // ключ сравнения; nil — paths.Fold
	Timeout time.Duration                               // предел одной дисковой пробы
	Disk    func(p string) (resolved string, err error) // проба диска; nil — настоящий диск, как у Default
	Homes   []string
	Whole   []string
	Ours    []string
	Trees   []string // добавочные системные деревья (сверх таблицы Lists)
	System  string   // системный том Windows; "" — Env("SystemDrive"), затем «C:»
	// Lists — ОС таблиц системных папок («windows», «linux», «darwin»); "" — без таблиц. Домашняя папка
	// для таблиц — первая из Homes, переменные — Env (nil — пусто).
	Lists string
	Env   func(string) string
}

// New — правила для тестов из Opts. В продукте запрещён (forbidigo 155): только Default.
func New(o Opts) *Rules { return newRules(o) }

// newRules — общая сборка New и Default: единственное место, где ставится built. New внутри пакета не
// зовётся: forbidigo сработал бы и здесь.
func newRules(o Opts) *Rules {
	r := &Rules{
		built: true, windows: o.Windows, fold: o.Fold, timeout: o.Timeout, disk: o.Disk,
		homes: o.Homes, whole: o.Whole, ours: o.Ours, system: o.System,
	}
	if r.fold == nil {
		r.fold = paths.Fold
	}
	if r.disk == nil {
		r.disk = diskProbe
	}
	env := o.Env
	if env == nil {
		env = func(string) string { return "" }
	}
	if o.Lists != "" {
		home := ""
		if len(o.Homes) > 0 {
			home = o.Homes[0]
		}
		r.trees, r.allow, r.exact, r.elems = osLists(o.Lists, home, env)
	}
	r.trees = append(r.trees, o.Trees...)
	if r.windows && r.system == "" {
		if r.system = env("SystemDrive"); r.system == "" {
			r.system = "C:"
		}
	}
	return r
}

// Default — правила этой машины: домашняя папка, три целые папки пользователя, папки
// данных и кэша (плюс extraOurs: папки данных, которые выбрал пользователь, — App знает их).
// Читает user-dirs.dirs и разрешает ссылки корней, поэтому вызывается под Within; nil — правил нет.
func Default(extraOurs ...string) *Rules {
	rules, ok := appkit.Within(osx.ProbeTimeout, func() *Rules { return buildDefault(extraOurs) })
	if !ok {
		return nil // fail-closed: правил нет — отказ
	}
	return rules
}

func buildDefault(extraOurs []string) *Rules {
	home, _ := os.UserHomeDir()
	if !filepath.IsAbs(home) {
		home = ""
	}
	return newRules(Opts{
		Windows: runtime.GOOS == "windows", Fold: paths.Fold, Timeout: osx.ProbeTimeout,
		Homes: withReal(home), Whole: withReal(osx.UserDirs()...),
		Ours:  withReal(append([]string{osx.ConfigDir(), osx.CacheDir()}, extraOurs...)...),
		Lists: runtime.GOOS, Env: os.Getenv,
	})
}

// withReal — корни как есть и с разрешёнными ссылками: домашняя может лежать за ссылкой
// (/home → /data/home), и настоящий путь выбранной папки тоже её накроет. Пустые отбрасываются.
func withReal(list ...string) []string {
	var out []string
	for _, p := range list {
		if p == "" {
			continue
		}
		out = append(out, p)
		if resolved, err := filepath.EvalSymlinks(p); err == nil && resolved != p {
			out = append(out, resolved)
		}
	}
	return out
}
