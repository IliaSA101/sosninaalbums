package app

// Списки ОС для правила папки агента (agentpath.go, design §2.3) и сборка правил для этой машины.

import (
	"os"
	"path"
	"path/filepath"
	"runtime"

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

// defaultFolderRules — правила этой машины: домашняя папка, три целые папки пользователя, папки
// данных и кэша (плюс extraOurs: папки данных, которые выбрал пользователь, — App знает их).
// Читает user-dirs.dirs и разрешает ссылки корней, поэтому вызывается под within.
func defaultFolderRules(extraOurs ...string) *folderRules {
	rules, ok := within(osx.ProbeTimeout, func() *folderRules { return buildFolderRules(extraOurs) })
	if !ok {
		return nil // fail-closed: правил нет — отказ
	}
	return rules
}

func buildFolderRules(extraOurs []string) *folderRules {
	windows := runtime.GOOS == "windows"
	home, _ := os.UserHomeDir()
	if !filepath.IsAbs(home) {
		home = ""
	}
	r := &folderRules{windows: windows, fold: paths.Fold, timeout: osx.ProbeTimeout, disk: diskProbe}
	r.trees, r.allow, r.exact, r.elems = osLists(runtime.GOOS, home, os.Getenv)
	r.homes = withReal(home)
	r.whole = withReal(osx.UserDirs()...)
	r.ours = withReal(append([]string{osx.ConfigDir(), osx.CacheDir()}, extraOurs...)...)
	return r
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
