package agents

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func setupBy(t *testing.T, list []Setup, id string) Setup {
	t.Helper()
	for _, s := range list {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("нет клиента %q в %v", id, list)
	return Setup{}
}

// Состав и порядок клиентов, подписи по ОС, кнопка файла только на Windows и Mac, пустой путь —
// пустой не-nil список (в JSON это [], а не null).
func TestSetups(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "linux"} {
		list := Setups("/opt/app", goos)
		var ids []string
		for _, s := range list {
			ids = append(ids, s.ID)
			if s.Title == "" || s.Text == "" || s.Note == "" {
				t.Errorf("%s/%s: пустое поле: %+v", goos, s.ID, s)
			}
			if !strings.Contains(s.Text, "--mcp") || !strings.Contains(s.Text, "/opt/app") {
				t.Errorf("%s/%s: в тексте нет пути или --mcp: %q", goos, s.ID, s.Text)
			}
		}
		if want := []string{SetupClaudeDesktop, SetupClaudeCode, SetupOpenCode, SetupOther}; !slices.Equal(ids, want) {
			t.Errorf("%s: клиенты %v, ждали %v", goos, ids, want)
		}
		wantFile := goos == "windows" || goos == "darwin"
		for _, s := range list {
			if s.File != (wantFile && s.ID == SetupClaudeDesktop) {
				t.Errorf("%s/%s: File = %v", goos, s.ID, s.File)
			}
		}
	}
	notes := map[string]string{"windows": `%APPDATA%\Claude`, "darwin": "Application Support/Claude", "linux": ".config/Claude"}
	for goos, want := range notes {
		if n := setupBy(t, Setups("/opt/app", goos), SetupClaudeDesktop).Note; !strings.Contains(n, want) {
			t.Errorf("%s: Note Claude Desktop %q без %q", goos, n, want)
		}
	}
	if n := setupBy(t, Setups("/opt/app", "windows"), SetupClaudeCode).Note; !strings.Contains(n, "PowerShell") {
		t.Errorf("Windows: Note Claude Code %q без PowerShell", n)
	}
	for _, goos := range []string{"windows", "linux", "darwin"} { // «Другой клиент»: оболочка названа только на Windows
		if n := setupBy(t, Setups("/opt/app", goos), SetupOther).Note; strings.Contains(n, "PowerShell") != (goos == "windows") {
			t.Errorf("%s: Note другого клиента %q", goos, n)
		}
	}
	l := Setups("", "linux")
	if l == nil || len(l) != 0 {
		t.Errorf("Setups(\"\") = %#v, ждали пустой не-nil список", l)
	}
	if b, _ := json.Marshal(l); string(b) != "[]" {
		t.Errorf("JSON пустого списка: %s", b)
	}
}

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

func TestXDGHint(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "u")
	cases := []struct {
		name, goos, xdg, home string
		hint                  bool
	}{
		{"не задана", "linux", "", home, false},
		{"умолчание", "linux", filepath.Join(home, ".config"), home, false},
		{"умолчание с хвостом и точками", "linux", home + "/.config/", home, false},
		{"умолчание через ..", "linux", home + "/x/../.config", home, false},
		{"другая папка", "linux", "/data/cfg", home, true},
		{"относительная папка", "linux", "cfg", home, true},
		{"дом неизвестен", "linux", "/data/cfg", "", false},
		{"Mac", "darwin", "/data/cfg", home, false},
		{"Windows", "windows", "/data/cfg", home, false},
	}
	for _, c := range cases {
		if got := XDGHint(c.goos, c.xdg, c.home); (got != "") != c.hint {
			t.Errorf("%s: XDGHint = %q, ждали предупреждение = %v", c.name, got, c.hint)
		}
	}
}

// awkwardPaths — пути, на которых ломается наивная вставка в кавычки.
var awkwardPaths = []string{
	"/opt/Sosnina Albums/app",
	"/opt/it's/app",
	`/opt/say "hi"/app`,
	"/opt/$HOME/$(id)/app",
	"/opt/`id`/app",
	"/opt/100%/%PATH%/app",
	"/opt/a&b;c|d>e<f/app",
	"/opt/Свадьба Иванов/альбом",
	"/opt/'; touch pwned; '/app",
	`/opt/back\slash/app`,
	"/opt/'''/app",
	"/opt/O’Brien ‘q’ ‚ ‛/app",
	"/opt/x’;Start-Process calc;’/app",
	"/opt/" + strings.Repeat("длинный путь ", 40) + "/app",
}

// TestClients: тексты настройки возвращают исходный путь. JSON (Claude Desktop, OpenCode) — разбором
// назад; команда — на Unix через `sh -c`, на любой ОС — разбором кавычек по правилам оболочек:
// POSIX и fish (апостроф '\” ), PowerShell (удвоение ' ‘ ’ ‚ ‛). Ничего не раскрывается и не
// выполняется. Путь с обратной косой чертой на Unix — без команд (fish читает её иначе, чем sh).
func TestClients(t *testing.T) {
	winPaths := append([]string{
		`C:\Program Files\Sosnina's Albums\app.exe`, `C:\Users\Иван $x\100% & co\app.exe`,
		`C:\Users\O’Brien\AppData\Local\Programs\SosninaAlbums\SosninaAlbums.exe`,
		`C:\Users\‘a’ ‚b‛\app.exe`,
		`C:\x’;Start-Process calc;’\app.exe`,
		`C:\x‘;Start-Process calc;‘\app.exe`,
		`C:\x‚;Start-Process calc;‚\app.exe`,
		`C:\x‛;Start-Process calc;‛\app.exe`,
		`C:\a\'b\app.exe`,
		`C:\a\'';calc;'\app.exe`,
	}, awkwardPaths...)
	for _, goos := range []string{"linux", "darwin", "windows"} {
		paths := awkwardPaths
		if goos == "windows" {
			paths = winPaths
		}
		for _, exe := range paths {
			checkClientPath(t, goos, exe)
		}
	}
	// Незащищённые символы JSON остаются читаемыми: & < > не превращаются в \u0026.
	if txt := setupBy(t, Setups("/opt/a&b<c>/app", "linux"), SetupClaudeDesktop).Text; !strings.Contains(txt, "a&b<c>") {
		t.Errorf("JSON экранировал символы HTML: %s", txt)
	}
}

// TestClientsControl: путь с управляющими, невидимыми знаками или не-UTF-8 команд не получает на
// любой ОС (Ctrl-U + перевод строки при вставке в терминал выполняют остаток); остаются два JSON,
// и они разбираются обратно в путь. Аудит 143 Г5, Н1.
func TestClientsControl(t *testing.T) {
	paths := []string{
		"/opt/x\x15curl evil|sh\n/app", "/opt/line\nbreak/app", "/opt/tab\there/app", "/opt/del\x7f/app",
		"/opt/nul\x00/app", "/opt/rtl\u202eexe.txt/app", "/opt/zw\u200bx/app", "/opt/bom\ufeffx/app",
		"/opt/bad\xff\xfeutf8/app", `C:\x` + "\x15" + `calc` + "\r\n" + `\app.exe`, `C:\del` + "\x7f" + `\app.exe`,
		`C:\rtl` + "\u202e" + `\app.exe`,
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		for _, exe := range paths {
			list := Setups(exe, goos)
			var ids []string
			for _, s := range list {
				ids = append(ids, s.ID)
			}
			if want := []string{SetupClaudeDesktop, SetupOpenCode}; !slices.Equal(ids, want) {
				t.Errorf("%s %q: клиенты %v, ждали только JSON %v", goos, exe, ids, want)
				continue
			}
			if n := list[0].Note; !strings.Contains(n, "не показаны") || !strings.Contains(n, "невидимые") {
				t.Errorf("%s %q: в пояснении нет причины: %q", goos, exe, n)
			}
			if !utf8.ValidString(exe) { // JSON подменит битые байты на U+FFFD: путь назад не вернуть, но и команды нет
				continue
			}
			var dj struct {
				Servers map[string]struct {
					Command string `json:"command"`
				} `json:"mcpServers"`
			}
			if err := json.Unmarshal([]byte(list[0].Text), &dj); err != nil || dj.Servers[serverName].Command != exe {
				t.Errorf("%s %q: JSON Claude Desktop вернул %q (%v)", goos, exe, dj.Servers[serverName].Command, err)
			}
		}
	}
}

// TestClientsBackslash: на Unix путь с `\` (в fish внутри кавычек это экранирование) команд не
// получает — только JSON и пояснение; на Windows `\` — разделитель, команды остаются.
func TestClientsBackslash(t *testing.T) {
	for _, exe := range []string{`/opt/back\slash/app`, `/tmp/x\';id;#/app`, `/opt/x\`} {
		list := Setups(exe, "linux")
		var ids []string
		for _, s := range list {
			ids = append(ids, s.ID)
			if strings.Contains(s.Text, "claude mcp add") || strings.Contains(s.Text, "'") && s.ID == SetupOther {
				t.Errorf("%q: команда при пути с обратной чертой: %q", exe, s.Text)
			}
		}
		if want := []string{SetupClaudeDesktop, SetupOpenCode}; !slices.Equal(ids, want) {
			t.Errorf("%q: клиенты %v, ждали %v", exe, ids, want)
		}
		if n := list[0].Note; !strings.Contains(n, "не показаны") {
			t.Errorf("%q: в пояснении нет причины: %q", exe, n)
		}
	}
	if len(Setups(`C:\a\b.exe`, "windows")) != 4 {
		t.Error("Windows: команды пропали")
	}
}

// checkClientPath проверяет тексты настройки для одного пути программы на ОС goos.
func checkClientPath(t *testing.T, goos, exe string) {
	t.Helper()
	list := Setups(exe, goos)

	var dj struct {
		Servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(setupBy(t, list, SetupClaudeDesktop).Text), &dj); err != nil {
		t.Fatalf("%s %q: Claude Desktop не JSON: %v", goos, exe, err)
	}
	if s := dj.Servers[serverName]; s.Command != exe || !slices.Equal(s.Args, []string{"--mcp"}) {
		t.Errorf("%s %q: Claude Desktop вернул %+v", goos, exe, s)
	}

	var oj struct {
		Mcp map[string]struct {
			Type    string   `json:"type"`
			Command []string `json:"command"`
			Enabled bool     `json:"enabled"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal([]byte(setupBy(t, list, SetupOpenCode).Text), &oj); err != nil {
		t.Fatalf("%s %q: OpenCode не JSON: %v", goos, exe, err)
	}
	if s := oj.Mcp[serverName]; s.Type != "local" || !s.Enabled || !slices.Equal(s.Command, []string{exe, "--mcp"}) {
		t.Errorf("%s %q: OpenCode вернул %+v", goos, exe, s)
	}

	if goos != "windows" && strings.Contains(exe, `\`) { // команд нет по решению 0.31-143-81
		if len(list) != 2 {
			t.Errorf("%s %q: ждали только JSON, получили %d клиентов", goos, exe, len(list))
		}
		return
	}
	// Команды: свой разбор кавычек даёт путь и --mcp.
	other := setupBy(t, list, SetupOther).Text
	code := setupBy(t, list, SetupClaudeCode).Text
	if !strings.HasPrefix(code, "claude mcp add "+serverName+" -- ") {
		t.Errorf("%s %q: Claude Code: %q", goos, exe, code)
	}
	if goos == "windows" { // путь в кавычках в начале оператора PowerShell вызывает только &
		if !strings.HasPrefix(other, "& '") {
			t.Errorf("%s %q: «Другой клиент» без оператора вызова: %q", goos, exe, other)
		}
		other = strings.TrimPrefix(other, "& ")
	} else if !strings.HasPrefix(other, "'") { // sh и fish: оператор & запустил бы фоновую задачу
		t.Errorf("%s %q: «Другой клиент» начинается не с пути: %q", goos, exe, other)
	}
	for _, cmd := range []string{other, strings.TrimPrefix(code, "claude mcp add "+serverName+" -- ")} {
		dialects := []string{"sh", "fish"}
		if goos == "windows" {
			dialects = []string{"powershell"}
		}
		for _, d := range dialects {
			if got := unquoteSingle(d, cmd); !slices.Equal(got, []string{exe, "--mcp"}) {
				t.Errorf("%s/%s: команда %q разобралась в %q, ждали путь и --mcp", goos, d, cmd, got)
			}
		}
	}
	if goos == "windows" { // доказать оболочкой нельзя: разбор выше
		return
	}
	runSh(t, exe, other)
	runSh(t, exe, strings.TrimPrefix(code, "claude mcp add "+serverName+" -- "))
}

// runSh отдаёт команду настоящему sh: слова на выходе — путь и --mcp, ни одной подстановки.
func runSh(t *testing.T, exe, cmd string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("нет sh")
	}
	// printf вместо запуска программы: слова разделены NUL, переводы строк в пути целы.
	c := exec.Command(sh, "-c", `printf '%s\0' `+cmd)
	c.Dir = testkit.TempDir(t) // сломанное экранирование не должно писать в дерево проекта
	out, err := c.Output()
	if err != nil {
		t.Errorf("sh -c для %q: %v", cmd, err)
		return
	}
	got := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	if !slices.Equal(got, []string{exe, "--mcp"}) {
		t.Errorf("sh разобрал %q в %q, ждали путь и --mcp", cmd, got)
	}
}

// unquoteSingle — слова команды по правилам оболочки dialect ("sh", "fish", "powershell") для
// одинарных кавычек; подстановок нет, как и у настоящих оболочек. Вне кавычек разделитель —
// пробел. sh: внутри кавычек всё как есть, `\x` вне кавычек — x. fish: внутри кавычек `\'` и
// `\\` — знак, прочее с `\` остаётся как есть. powershell: кавычкой считаются ' и U+2018…U+201B,
// двойной знак внутри строки — один такой знак.
func unquoteSingle(dialect, cmd string) []string {
	isQ := func(r rune) bool { return r == '\'' }
	if dialect == "powershell" {
		isQ = func(r rune) bool { return strings.ContainsRune(psQuotes, r) }
	}
	var words []string
	var cur []rune
	started, in := false, false
	rs := []rune(cmd)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case in && dialect == "powershell" && isQ(r) && i+1 < len(rs) && rs[i+1] == r:
			cur = append(cur, r)
			i++
		case in && isQ(r):
			in = false
		case in && dialect == "fish" && r == '\\' && i+1 < len(rs) && (rs[i+1] == '\'' || rs[i+1] == '\\'):
			cur = append(cur, rs[i+1])
			i++
		case in:
			cur = append(cur, r)
		case isQ(r):
			in, started = true, true
		case r == '\\' && dialect != "powershell" && i+1 < len(rs):
			cur = append(cur, rs[i+1])
			started = true
			i++
		case r == ' ':
			if started {
				words, cur, started = append(words, string(cur)), nil, false
			}
		default:
			cur = append(cur, r)
			started = true
		}
	}
	if started {
		words = append(words, string(cur))
	}
	return words
}

// TestUnquoteSingle: разборщики сами верны по документации оболочек — иначе TestClients ничего не
// доказывает. Ловушки прошлого: PowerShell закрывает строку на U+2019, fish читает `\'` внутри.
func TestUnquoteSingle(t *testing.T) {
	cases := []struct {
		dialect, cmd string
		want         []string
	}{
		{"powershell", `'a''b' --mcp`, []string{"a'b", "--mcp"}},
		{"powershell", `'a’’b' --mcp`, []string{"a’b", "--mcp"}},
		{"powershell", `'a’b' --mcp`, []string{"ab --mcp"}}, // одиночная ’ закрыла строку: слова склеились
		{"fish", `'a\'b' c`, []string{"a'b", "c"}},
		{"fish", `'a\\b' c`, []string{`a\b`, "c"}},
		{"fish", `'a\nb' c`, []string{`a\nb`, "c"}},
		{"sh", `'a\nb' c`, []string{`a\nb`, "c"}},
		{"sh", `'a'\''b' c`, []string{"a'b", "c"}},
		{"fish", `'a'\''b' c`, []string{"a'b", "c"}},
	}
	for _, c := range cases {
		got := unquoteSingle(c.dialect, c.cmd)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s %q → %q, ждали %q", c.dialect, c.cmd, got, c.want)
		}
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
