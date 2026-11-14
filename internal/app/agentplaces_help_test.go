package app

// Помощники тестов правил и мест (155, design §2.1): правила собираются только через
// agentplaces.New (тесты) — в продукте Default. Копии помощников пакета agentplaces: тестового
// пакета-помощника нет, он потребовал бы ещё одно правило depguard.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// optFn правит Opts перед сборкой правил: поля закрыты, менять правила после New нельзя.
type optFn = func(*agentplaces.Opts)

func build(o agentplaces.Opts, mod []optFn) *agentplaces.Rules {
	for _, m := range mod {
		m(&o)
	}
	return agentplaces.New(o)
}

// diskOK — диск «есть и каталог»: путь настоящий сам себе.
func diskOK(p string) (string, error) { return p, nil }

// unixRules и winRules — правила с запретами ОС (таблицы пакета, home /home/u и C:\Users\u).
func unixRules(goos string, disk func(string) (string, error)) *agentplaces.Rules {
	home := "/home/u"
	if goos == "darwin" {
		home = "/Users/u"
	}
	return agentplaces.New(agentplaces.Opts{
		Fold: func(s string) string { return s }, Homes: []string{home}, Timeout: time.Second, Disk: disk,
		Whole: []string{home + "/Документы", home + "/Рабочий стол", home + "/Загрузки"},
		Ours:  []string{home + "/.config", home + "/.cache"}, Lists: goos,
	})
}

func winRules(disk func(string) (string, error)) *agentplaces.Rules {
	env := map[string]string{
		"SystemRoot": `C:\Windows`, "ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`,
		"ProgramData": `C:\ProgramData`, "AppData": `C:\Users\u\AppData\Roaming`, "LocalAppData": `C:\Users\u\AppData\Local`,
	}
	home := `C:\Users\u`
	return agentplaces.New(agentplaces.Opts{
		Windows: true, Fold: strings.ToLower, Homes: []string{home}, Timeout: time.Second, Disk: disk,
		Whole: []string{home + `\Documents`, home + `\Desktop`, home + `\Downloads`},
		Ours:  []string{env["AppData"], env["LocalAppData"]}, Lists: "windows", Env: func(k string) string { return env[k] },
	})
}

// permissiveOpts — правила без запретов: тестовые папки лежат под TMPDIR (в точечной .gotmp и в домашней),
// настоящие правила их отвергли бы. Диск «есть». Таблицы правил — в пакете agentplaces.
func permissiveOpts() agentplaces.Opts {
	return agentplaces.Opts{Windows: runtime.GOOS == "windows", Fold: paths.Fold, Timeout: time.Second, Disk: diskOK}
}

func permissiveRules() *agentplaces.Rules { return agentplaces.New(permissiveOpts()) }

// permissiveOnDisk — те же правила на настоящем диске (Disk не задан).
func permissiveOnDisk() *agentplaces.Rules {
	o := permissiveOpts()
	o.Disk = nil
	return agentplaces.New(o)
}

// realRules — правила без запретов поверх настоящего диска; mod правит Opts (деревья, домашняя, срок).
func realRules(mod ...optFn) *agentplaces.Rules {
	return build(agentplaces.Opts{Windows: runtime.GOOS == "windows", Fold: func(s string) string { return s }, Timeout: time.Second}, mod)
}

// realEnv — среда с такими правилами; Dev и Mounts App кладёт сам (placeEnv).
func realEnv() *agentplaces.Env { return &agentplaces.Env{Rules: realRules(), System: "C:"} }

func realDir(t *testing.T, elems ...string) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(append([]string{base}, elems...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeDisk подменяет st_dev и таблицу монтирования (переменные placeDev и placeMounts, которые placeEnv()
// кладёт в среду): устройство — по длиннейшей точке. Корень "/" — общий для всех путей (на Windows тома
// нет, как и у DevOf); точки — пути ОС.
func fakeDisk(t *testing.T, points ...string) (devCalls *atomic.Int32) {
	t.Helper()
	oldDev, oldMounts := placeDev, placeMounts
	t.Cleanup(func() { placeDev, placeMounts = oldDev, oldMounts })
	devCalls = new(atomic.Int32)
	// Корень — том временных папок: на Windows «/» не путь тома (правила чистят «X:\…», с «/» паника).
	root := "/"
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.TempDir()) + `\`
	}
	placeMounts = func() ([]string, error) { return append([]string{root}, points...), nil }
	placeDev = func(p string) (uint64, bool) {
		devCalls.Add(1)
		best, id := -1, uint64(0)
		for i, m := range append([]string{root}, points...) {
			sep := string(filepath.Separator)
			if (m == root || p == m || strings.HasPrefix(p, strings.TrimSuffix(m, sep)+sep)) && len(m) > best {
				best, id = len(m), uint64(i)
			}
		}
		return id, true
	}
	return devCalls
}
