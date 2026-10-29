package mcpbridge

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// startFx — подставные зависимости start_program: настоящие папки настроек и кэша во временном
// месте, подставные процесс, окружение, часы и признаки ОС.
type startFx struct {
	t      *testing.T
	root   string
	data   string
	exe    string
	clock  time.Time
	env    []string
	spawns []spawnCall
	deps   startDeps
}

type spawnCall struct {
	exe string
	env []string
}

const fxData = "SosninaAlbums-fx"

// newStartFx — доступ агентам включён, окружение с экраном, программа «обычная».
func newStartFx(t *testing.T) *startFx {
	t.Helper()
	root := testkit.TempDir(t)
	fx := &startFx{t: t, root: root, data: fxData, clock: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		env: []string{"PATH=/usr/bin", "HOME=/home/u", "DISPLAY=:0"}}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	fx.exe = filepath.Join(bin, "SosninaAlbums")
	if err := os.WriteFile(fx.exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.setConfig(`{"agents":{"on":true,"level":1}}`)
	fx.deps = startDeps{
		goos: runtime.GOOS, dataName: fxData, instanceKey: "fx-key",
		environ:  func() []string { return fx.env },
		cfgDir:   func() string { return filepath.Join(root, "cfg") },
		cacheDir: func() string { return filepath.Join(root, "cache") },
		tempDir:  func() string { return filepath.Join(root, "othertemp") },
		exe:      func() (string, error) { return fx.exe, nil },
		regular:  isRegular,
		exists:   func(string) bool { return false },
		elevated: func() bool { return false },
		packaged: func() bool { return false },
		instance: func() bool { return false },
		spawn: func(exe string, env []string) error {
			fx.spawns = append(fx.spawns, spawnCall{exe, env})
			return nil
		},
		now: func() time.Time { return fx.clock },
	}
	return fx
}

func (fx *startFx) setConfig(text string) {
	fx.t.Helper()
	dir := filepath.Join(fx.root, "cfg", fx.data)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fx.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(text), 0o600); err != nil {
		fx.t.Fatal(err)
	}
}

func (fx *startFx) markPath() string {
	return filepath.Join(fx.root, "cache", fx.data, "mcp", "start.mark")
}

// bridge — посредник на сокете, которого нет (причина — «программа закрыта»), с этими зависимостями.
func (fx *startFx) bridge() *harness {
	fx.t.Helper()
	cfg := testCfg(newSock(fx.t))
	cfg.start = fx.deps
	return startBridge(fx.t, cfg)
}

// call зовёт start_program и возвращает текст и признак ошибки.
func callStart(t *testing.T, h *harness, args string) (string, bool) {
	t.Helper()
	h.send(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"start_program","arguments":` + args + `}}`)
	m := h.recv()
	if m["error"] != nil {
		t.Fatalf("JSON-RPC-ошибка вместо результата: %v", m)
	}
	return toolText(t, m), isToolError(m)
}

func toolNames(t *testing.T, h *harness) []string {
	t.Helper()
	h.send(`{"jsonrpc":"2.0","id":8,"method":"tools/list"}`)
	var names []string
	for _, x := range h.recv()["result"].(map[string]any)["tools"].([]any) {
		names = append(names, x.(map[string]any)["name"].(string))
	}
	return names
}

// TestStartProgram: доступ включён, программа закрыта — в списке есть start_program с явными
// аннотациями (М6); вызов запускает ровно свой exe, ответ без путей, не ошибка.
func TestStartProgram(t *testing.T) {
	fx := newStartFx(t)
	h := fx.bridge()
	if got := toolNames(t, h); len(got) != 2 || got[0] != "program_status" || got[1] != "start_program" {
		t.Fatalf("tools/list: %v", got)
	}
	h.send(`{"jsonrpc":"2.0","id":8,"method":"tools/list"}`)
	tools := h.recv()["result"].(map[string]any)["tools"].([]any)
	ann := tools[1].(map[string]any)["annotations"].(map[string]any)
	for _, k := range []string{"readOnlyHint", "destructiveHint", "idempotentHint", "openWorldHint"} {
		if v, ok := ann[k].(bool); !ok || v {
			t.Errorf("аннотация %s должна быть задана явно и ложной: %v", k, ann)
		}
	}
	text, isErr := callStart(t, h, `{}`)
	if isErr || text != textStarting {
		t.Errorf("ответ: %q, ошибка %v", text, isErr)
	}
	if len(fx.spawns) != 1 || fx.spawns[0].exe != fx.exe {
		t.Fatalf("запуски: %v", fx.spawns)
	}
	if strings.Contains(text, fx.root) || strings.Contains(h.errw.String(), fx.root) {
		t.Error("путь в ответе или stderr")
	}
	if _, err := os.Stat(fx.markPath()); err != nil {
		t.Errorf("отметки запуска нет: %v", err)
	}
}

// TestStartProgramArgs: что бы ни прислал агент в arguments, процесс получает только свой exe
// и белый список окружения; сигнатура запуска аргументов не знает.
func TestStartProgramArgs(t *testing.T) {
	fx := newStartFx(t)
	h := fx.bridge()
	if _, isErr := callStart(t, h, `{"args":["--mcp","/etc/passwd"],"exe":"/bin/sh","path":"/tmp","env":{"LD_PRELOAD":"x"}}`); isErr {
		t.Fatal("отказ")
	}
	if len(fx.spawns) != 1 || fx.spawns[0].exe != fx.exe {
		t.Fatalf("запуски: %v", fx.spawns)
	}
	for _, kv := range fx.spawns[0].env {
		if strings.Contains(kv, "LD_PRELOAD") || strings.Contains(kv, "passwd") {
			t.Errorf("данные агента в окружении: %q", kv)
		}
	}
	// Мусор вместо объекта arguments — тоже запуск без последствий для процесса.
	fx.clock = fx.clock.Add(2 * startEvery)
	if _, isErr := callStart(t, h, `[1,2]`); isErr {
		t.Error("отказ на arguments-массив")
	}
	if len(fx.spawns) != 2 || fx.spawns[1].exe != fx.exe {
		t.Errorf("запуски: %v", fx.spawns)
	}
}

// TestStartProgramEnv: ребёнку уходит только белый список (В6), остальное окружение клиента — нет.
func TestStartProgramEnv(t *testing.T) {
	fx := newStartFx(t)
	fx.deps.goos = "linux" // белый список Unix: на Windows те же имена отсеялись бы (список Windows — TestChildEnv)
	fx.env = []string{
		"PATH=/usr/bin", "HOME=/home/u", "USER=u", "LOGNAME=u", "LANG=ru_RU.UTF-8", "LC_ALL=C",
		"DISPLAY=:0", "WAYLAND_DISPLAY=wayland-0", "XDG_RUNTIME_DIR=/run/user/1000",
		"XDG_CONFIG_HOME=/home/u/.config", "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/bus",
		"WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=9222", "WEBKIT_INSPECTOR_SERVER=0.0.0.0:1",
		"GTK_MODULES=evil", "LD_PRELOAD=/x.so", "DYLD_INSERT_LIBRARIES=/x.dylib", "GODEBUG=x=1",
		"ANTHROPIC_API_KEY=secret", "SSH_AUTH_SOCK=/s", "XDG_SESSION_ID=3",
	}
	h := fx.bridge()
	if _, isErr := callStart(t, h, `{}`); isErr {
		t.Fatal("отказ")
	}
	got := strings.Join(fx.spawns[0].env, "\n")
	for _, bad := range []string{"WEBVIEW2", "WEBKIT", "GTK_", "LD_PRELOAD", "DYLD", "GODEBUG", "SECRET", "secret", "SSH_AUTH", "XDG_SESSION"} {
		if strings.Contains(got, bad) {
			t.Errorf("в окружении ребёнка %q:\n%s", bad, got)
		}
	}
	for _, want := range []string{"PATH=", "HOME=", "LC_ALL=", "DISPLAY=", "WAYLAND_DISPLAY=", "XDG_RUNTIME_DIR=", "XDG_CONFIG_HOME=", "DBUS_SESSION_BUS_ADDRESS="} {
		if !strings.Contains(got, want) {
			t.Errorf("нет %q в окружении ребёнка:\n%s", want, got)
		}
	}
}

// TestStartProgramAccessOff: agents.on не true — start_program нет в списке, вызов — как у чужого
// инструмента (isError с прежним текстом), запуска и отметки нет (В5). Битые, усечённые, пустые,
// огромные и не обычные файлы — «выключено», без паники.
func TestStartProgramAccessOff(t *testing.T) {
	pad := strings.Repeat(" ", maxConfigRead)
	cases := map[string]string{
		"нет ключа agents": `{}`,
		"on=false":         `{"agents":{"on":false}}`,
		"on строкой":       `{"agents":{"on":"true"}}`,
		"on числом":        `{"agents":{"on":1}}`,
		"agents null":      `{"agents":null}`,
		"agents массив":    `{"agents":[true]}`,
		"усечённый":        `{"agents":{"on":tr`,
		"пустой":           ``,
		"не JSON":          "\x00\x01\x02",
		"корень — массив":  `[{"agents":{"on":true}}]`,
		"больше предела":   `{"agents":{"on":true}}` + pad,
		"огромный":         `{"agents":{"on":true},"x":"` + strings.Repeat("a", 5*maxConfigRead) + `"}`,
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.setConfig(text)
			h := fx.bridge()
			if got := toolNames(t, h); len(got) != 1 || got[0] != "program_status" {
				t.Errorf("tools/list: %v", got)
			}
			txt, isErr := callStart(t, h, `{}`)
			if !isErr || txt != textClosed {
				t.Errorf("вызов: %q, ошибка %v", txt, isErr)
			}
			if len(fx.spawns) != 0 {
				t.Error("запуск при выключенном доступе")
			}
			if _, err := os.Stat(fx.markPath()); err == nil {
				t.Error("отметка записана при выключенном доступе")
			}
		})
	}
}

// TestStartProgramAccessOffShapes: config.json отсутствует, не файл, по ссылке, папки настроек нет —
// тоже «выключено»; ровно предельный размер читается.
func TestStartProgramAccessOffShapes(t *testing.T) {
	t.Run("нет файла", func(t *testing.T) {
		fx := newStartFx(t)
		if err := os.Remove(filepath.Join(fx.root, "cfg", fx.data, "config.json")); err != nil {
			t.Fatal(err)
		}
		if h := fx.bridge(); len(toolNames(t, h)) != 1 {
			t.Error("start_program без config.json")
		}
	})
	t.Run("каталог вместо файла", func(t *testing.T) {
		fx := newStartFx(t)
		p := filepath.Join(fx.root, "cfg", fx.data, "config.json")
		if err := os.Remove(p); err != nil || os.Mkdir(p, 0o755) != nil {
			t.Fatal("не подготовили каталог")
		}
		if h := fx.bridge(); len(toolNames(t, h)) != 1 {
			t.Error("start_program при каталоге на месте config.json")
		}
	})
	t.Run("нет папки настроек и пустое имя", func(t *testing.T) {
		fx := newStartFx(t)
		fx.deps.cfgDir = func() string { return "" }
		if h := fx.bridge(); len(toolNames(t, h)) != 1 {
			t.Error("start_program без папки настроек")
		}
		fx = newStartFx(t)
		fx.deps.cfgDir = func() string { return "относительный/путь" }
		if h := fx.bridge(); len(toolNames(t, h)) != 1 {
			t.Error("start_program с относительной папкой настроек")
		}
	})
	t.Run("ровно предел — читается", func(t *testing.T) {
		fx := newStartFx(t)
		body := `{"agents":{"on":true}}`
		fx.setConfig(body + strings.Repeat(" ", maxConfigRead-len(body)))
		if h := fx.bridge(); len(toolNames(t, h)) != 2 {
			t.Error("файл ровно в предел не принят")
		}
	})
	t.Run("ссылка вместо файла", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("ссылки на Windows требуют прав")
		}
		fx := newStartFx(t)
		p := filepath.Join(fx.root, "cfg", fx.data, "config.json")
		target := filepath.Join(fx.root, "real.json")
		if err := os.Rename(p, target); err != nil || os.Symlink(target, p) != nil {
			t.Fatal("не подготовили ссылку")
		}
		if h := fx.bridge(); len(toolNames(t, h)) != 1 {
			t.Error("config.json по ссылке принят")
		}
	})
}

// TestStartProgramElevated: посредник с повышенными правами — «запустите сами», запуска и отметки нет.
func TestStartProgramElevated(t *testing.T) {
	fx := newStartFx(t)
	fx.deps.elevated = func() bool { return true }
	fx.refused(t, reasonElevated)
}

// TestStartProgramRoot: настоящая проверка прав на Unix — root по euid; тест сверяет её с ОС, а
// не зовёт подставную.
func TestStartProgramRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("на Windows права — токен процесса, проверка в winqa")
	}
	if got, want := osx.Elevated(), os.Geteuid() == 0; got != want {
		t.Errorf("osx.Elevated = %v, euid == 0: %v", got, want)
	}
}

// TestStartProgramSandbox: Flatpak и Snap (Linux) — отказ; на других ОС переменные ничего не значат.
func TestStartProgramSandbox(t *testing.T) {
	cases := map[string]func(fx *startFx){
		"FLATPAK_ID": func(fx *startFx) { fx.env = append(fx.env, "FLATPAK_ID=org.x.App") },
		"SNAP":       func(fx *startFx) { fx.env = append(fx.env, "SNAP=/snap/x/1") },
		"flatpak-info": func(fx *startFx) {
			fx.deps.exists = func(p string) bool { return p == "/.flatpak-info" }
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.deps.goos = "linux"
			set(fx)
			fx.refused(t, reasonSandbox)
		})
		t.Run(name+" не Linux", func(t *testing.T) {
			fx := newStartFx(t)
			fx.deps.goos = "darwin"
			set(fx)
			h := fx.bridge()
			if _, isErr := callStart(t, h, `{}`); isErr {
				t.Error("песочница Linux остановила запуск на Mac")
			}
		})
	}
	t.Run("пустое значение — не песочница", func(t *testing.T) {
		fx := newStartFx(t)
		fx.deps.goos = "linux"
		fx.env = append(fx.env, "FLATPAK_ID=", "SNAP=")
		h := fx.bridge()
		if _, isErr := callStart(t, h, `{}`); isErr {
			t.Error("пустые переменные приняты за песочницу")
		}
	})
}

// TestStartProgramNoDisplay: Linux без DISPLAY и WAYLAND_DISPLAY — отказ; одной из двух достаточно;
// Windows и Mac экрана в окружении не требуют.
func TestStartProgramNoDisplay(t *testing.T) {
	envOf := func(extra ...string) []string { return append([]string{"PATH=/usr/bin"}, extra...) }
	cases := []struct {
		name string
		goos string
		env  []string
		ok   bool
	}{
		{"Linux без обеих", "linux", envOf(), false},
		{"Linux пустые обе", "linux", envOf("DISPLAY=", "WAYLAND_DISPLAY="), false},
		{"Linux только DISPLAY", "linux", envOf("DISPLAY=:0"), true},
		{"Linux только Wayland", "linux", envOf("WAYLAND_DISPLAY=wayland-0"), true},
		{"Mac без экрана в окружении", "darwin", envOf(), true},
		{"Windows без экрана в окружении", "windows", envOf(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.deps.goos = c.goos
			fx.env = c.env
			fx.deps.regular = func(string) bool { return true } // Windows-ветка ExeHint не должна мешать
			if c.ok {
				if _, isErr := callStart(t, fx.bridge(), `{}`); isErr {
					t.Error("отказ")
				}
				return
			}
			fx.refused(t, reasonNoDisplay)
		})
	}
}

// TestStartProgramPackaged: посредник в пакете MSIX — отказ, пока спайк Т0 не доказал выход.
func TestStartProgramPackaged(t *testing.T) {
	fx := newStartFx(t)
	fx.deps.packaged = func() bool { return true }
	fx.refused(t, reasonPackaged)
}

// TestStartProgramBadExe: путь программы не годится — отказ без запуска и без отметки (минута не
// тратится): ошибка os.Executable, относительный путь, не обычный файл, Mac App Translocation.
func TestStartProgramBadExe(t *testing.T) {
	cases := map[string]func(fx *startFx){
		"ошибка os.Executable": func(fx *startFx) { fx.deps.exe = func() (string, error) { return "", errors.New("ZQsecret") } },
		"пустой путь":          func(fx *startFx) { fx.exe = "" },
		"относительный путь":   func(fx *startFx) { fx.exe = "SosninaAlbums" },
		"exe удалён":           func(fx *startFx) { fx.exe = filepath.Join(fx.root, "bin", "gone") },
		"каталог вместо exe":   func(fx *startFx) { fx.exe = filepath.Join(fx.root, "bin") },
		"Mac AppTranslocation": func(fx *startFx) {
			fx.deps.goos = "darwin"
			fx.exe = "/private/var/folders/x/AppTranslocation/A/d/S.app/Contents/MacOS/app"
			fx.deps.regular = func(string) bool { return true }
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			fx := newStartFx(t)
			set(fx)
			text := fx.refused(t, reasonBadExe)
			if strings.Contains(text, "ZQsecret") {
				t.Errorf("текст системной ошибки в ответе: %q", text)
			}
		})
	}
}

// TestStartProgramOnceMarker: раз в минуту на все посредники — по отметке в кэше, а не по памяти
// процесса; отметка из будущего и битая не запирают запуск; без кэша — отказ.
func TestStartProgramOnceMarker(t *testing.T) {
	fx := newStartFx(t)
	h := fx.bridge()
	if _, isErr := callStart(t, h, `{}`); isErr {
		t.Fatal("первый запуск отказан")
	}
	text, isErr := callStart(t, h, `{}`)
	if !isErr || text != textStartOwn+reasonRecent {
		t.Errorf("второй вызов сразу: %q, ошибка %v", text, isErr)
	}
	// Другой посредник (другой клиент агента) делит только кэш.
	h2 := fx.bridge()
	if text, isErr := callStart(t, h2, `{}`); !isErr || text != textStartOwn+reasonRecent {
		t.Errorf("второй посредник: %q, ошибка %v", text, isErr)
	}
	fx.clock = fx.clock.Add(startEvery - time.Second)
	if _, isErr := callStart(t, h, `{}`); !isErr {
		t.Error("запуск за секунду до конца минуты")
	}
	fx.clock = fx.clock.Add(time.Second)
	if _, isErr := callStart(t, h, `{}`); isErr {
		t.Error("отказ после минуты")
	}
	if len(fx.spawns) != 2 {
		t.Errorf("запусков %d, ждали 2", len(fx.spawns))
	}

	for name, body := range map[string]string{
		"отметка из будущего": "9000000000000000000",
		"мусор в отметке":     "ZZZ",
		"огромная отметка":    strings.Repeat("1", 4096),
		"пустая отметка":      "",
		"отрицательная":       "-5",
	} {
		t.Run(name, func(t *testing.T) {
			fx := newStartFx(t)
			if err := os.MkdirAll(filepath.Dir(fx.markPath()), 0o700); err != nil || os.WriteFile(fx.markPath(), []byte(body), 0o600) != nil {
				t.Fatal("не подготовили отметку")
			}
			if text, isErr := callStart(t, fx.bridge(), `{}`); isErr {
				t.Errorf("отметка %q заперла запуск: %q", name, text)
			}
		})
	}
	t.Run("нет папки кэша", func(t *testing.T) {
		fx := newStartFx(t)
		fx.deps.cacheDir = func() string { return "" }
		fx.refused(t, reasonNoMark)
	})
	t.Run("кэш — не каталог", func(t *testing.T) {
		fx := newStartFx(t)
		if err := os.WriteFile(filepath.Join(fx.root, "cache"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		fx.refused(t, reasonNoMark)
	})
}

// TestStartProgramInstanceAlive: замок экземпляра занят — не запускать и не тратить минуту; жив
// сокет — тоже: клиент получает «уже запущена» и обновлённый список инструментов.
func TestStartProgramInstanceAlive(t *testing.T) {
	fx := newStartFx(t)
	fx.deps.instance = func() bool { return true }
	fx.refused(t, reasonAlive)

	fx = newStartFx(t)
	sock := newSock(t)
	cfg := testCfg(sock)
	cfg.probeEvery = time.Hour // сам не заметит: узнаёт только в момент вызова
	cfg.start = fx.deps
	h := startBridge(t, cfg)
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`) // первая проба при старте уже прошла
	h.recv()
	p := listenProg(t, sock)
	h.send(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"start_program","arguments":{}}}`)
	p.accept()
	var gotList, gotText bool
	for range 2 {
		m := h.recv()
		switch {
		case m["method"] == "notifications/tools/list_changed":
			gotList = true
		case toolText2(m) == textStartAlready && !isToolError(m):
			gotText = true
		default:
			t.Errorf("лишняя строка: %v", m)
		}
	}
	if !gotList || !gotText {
		t.Errorf("list_changed %v, ответ %v", gotList, gotText)
	}
	if len(fx.spawns) != 0 {
		t.Error("запуск при живом сокете")
	}
}

// TestStartProgramSpawnFail: сбой запуска и отказ отделения от задания (Windows) — «запустите
// сами» без текста системной ошибки; программа не запущена, список не меняется.
func TestStartProgramSpawnFail(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		reason string
	}{
		"сбой":       {errors.New("fork/exec /secret/ZQpath: permission denied"), reasonFailed},
		"breakaway":  {errBreakaway, reasonBreakaway},
		"обёрнутый":  {errors.Join(errors.New("ZQpath"), errBreakaway), reasonBreakaway},
		"нет такого": {os.ErrNotExist, reasonFailed},
	} {
		t.Run(name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.deps.spawn = func(string, []string) error { return c.err }
			text := fx.refused(t, c.reason)
			if strings.Contains(text, "ZQpath") || strings.Contains(text, "secret") {
				t.Errorf("текст системной ошибки в ответе: %q", text)
			}
		})
	}
}

// TestStartProgramNotOffered: путей сокета нет — причина не «закрыта»: инструмента нет, вызов
// отдаёт причину, запуска нет.
func TestStartProgramNotOffered(t *testing.T) {
	fx := newStartFx(t)
	cfg := testCfg() // без путей сокета: «нет папки данных»
	cfg.start = fx.deps
	h := startBridge(t, cfg)
	if got := toolNames(t, h); len(got) != 1 {
		t.Errorf("tools/list: %v", got)
	}
	text, isErr := callStart(t, h, `{}`)
	if !isErr || text != textNoDir || len(fx.spawns) != 0 {
		t.Errorf("вызов: %q, ошибка %v, запусков %d", text, isErr, len(fx.spawns))
	}
}

// refused: вызов start_program отказан «запустите сами» с этой причиной; запуска нет.
func (fx *startFx) refused(t *testing.T, reason string) string {
	t.Helper()
	text, isErr := callStart(t, fx.bridge(), `{}`)
	if !isErr || text != textStartOwn+reason {
		t.Errorf("ответ %q, ошибка %v; ждали отказ %q", text, isErr, reason)
	}
	if reason != reasonFailed && reason != reasonBreakaway && len(fx.spawns) != 0 {
		t.Errorf("запуск при отказе: %v", fx.spawns)
	}
	if reason != reasonRecent && reason != reasonNoMark && reason != reasonFailed && reason != reasonBreakaway {
		if _, err := os.Stat(fx.markPath()); err == nil {
			t.Error("отметка записана при отказе: минута потрачена зря")
		}
	}
	return text
}
