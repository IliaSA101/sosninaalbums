package agents

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// psProbe — сценарий PowerShell: команду из SA_CMD (текст, как его копирует пользователь) разбирает
// настоящий токенизатор, затем исполняется SA_RUN с функциями-заглушками вместо Start-Process,
// calc, claude и a: функции берутся раньше командлетов и программ, поэтому даже сломанное
// экранирование ничего не откроет, а только бросит EXEC. Заглушки печатают свои аргументы в
// base64 (UTF-8). SA_RUN — тот же текст; для «Другого клиента» путь вызывает оператор &, а файла
// по такому пути нет, поэтому & заменён на a (остаток строки тот же).
const psProbe = `$ErrorActionPreference = 'Stop'
function b64($s) { [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes([string]$s)) }
$e = $null
$t = [Management.Automation.PSParser]::Tokenize($env:SA_CMD, [ref]$e)
'ERRORS ' + $(if ($e) { $e.Count } else { 0 })
foreach ($x in $t) { 'TOK ' + $x.Type + ' ' + (b64 $x.Content) }
function Start-Process { throw 'EXEC' }
function calc { throw 'EXEC' }
function a { foreach ($x in $args) { 'ARG ' + (b64 $x) } }
function claude { foreach ($x in $args) { 'ARG ' + (b64 $x) } }
Invoke-Expression $env:SA_RUN
`

// TestClientsPowerShell: команды подключения для путей-ловушек (типографские кавычки U+2018…U+201B,
// ASCII ') разбирает и исполняет настоящий PowerShell ровно в том виде, в каком их копирует
// пользователь: ни ошибок разбора, ни разделителей команд, путь — одна строка, аргументы —
// путь и --mcp. Ничего не запускается (аудит 143 Г5, Н1). Без PowerShell в PATH — пропуск; на
// CI windows-latest он есть.
func TestClientsPowerShell(t *testing.T) {
	ps := ""
	for _, name := range []string{"pwsh", "powershell"} {
		if p, err := exec.LookPath(name); err == nil {
			ps = p
			break
		}
	}
	if ps == "" {
		t.Skip("нет pwsh и powershell")
	}
	dir := testkit.TempDir(t)
	paths := []string{
		`C:\x’;Start-Process calc;’\app.exe`, `C:\x‘;Start-Process calc;‘\app.exe`,
		`C:\x‚;Start-Process calc;‚\app.exe`, `C:\x‛;Start-Process calc;‛\app.exe`,
		`C:\a\'';Start-Process calc;'\app.exe`, `C:\Users\O’Brien $x ` + "`n" + `\app.exe`,
		`C:\Users\‘a’ ‚b‛ & (calc)\app.exe`, `C:\Program Files\Sosnina's Albums\app.exe`,
	}
	for _, exe := range paths {
		list := Setups(exe, "windows")
		other := setupBy(t, list, SetupOther).Text
		code := setupBy(t, list, SetupClaudeCode).Text
		if !strings.HasPrefix(other, "& '") {
			t.Fatalf("%q: «Другой клиент» без оператора вызова &: %q", exe, other)
		}
		cases := []struct {
			text, run string
			want      [][]string // допустимые списки аргументов
		}{
			// Путь в кавычках в начале оператора — выражение; вызывает его только &.
			{other, "a " + strings.TrimPrefix(other, "& "), [][]string{{exe, "--mcp"}}},
			// PowerShell 5.1 и 7 по-разному передают «--» в функцию: допускаем оба вида.
			{code, code, [][]string{{"mcp", "add", serverName, "--", exe, "--mcp"}, {"mcp", "add", serverName, exe, "--mcp"}}},
		}
		for _, c := range cases {
			var strs, args []string
			for _, ln := range strings.Split(runPS(t, ps, dir, c.text, c.run), "\n") {
				f := strings.Fields(ln)
				switch {
				case len(f) == 2 && f[0] == "ERRORS" && f[1] != "0":
					t.Errorf("%q: ошибки разбора PowerShell: %s", c.text, ln)
				case len(f) == 3 && f[0] == "TOK" && slices.Contains([]string{"StatementSeparator", "NewLine", "Variable", "GroupStart"}, f[1]):
					t.Errorf("%q: токен %s вне пути: %s", c.text, f[1], ln)
				case len(f) == 3 && f[0] == "TOK" && f[1] == "String":
					strs = append(strs, unb64(t, f[2]))
				case len(f) == 2 && f[0] == "ARG":
					args = append(args, unb64(t, f[1]))
				}
			}
			if !slices.Equal(strs, []string{exe}) {
				t.Errorf("%q: строки команды %q, ждали один путь %q", c.text, strs, exe)
			}
			// Исполнение: заглушка получила аргументы, EXEC не брошен (runPS упал бы на нём).
			if !slices.ContainsFunc(c.want, func(w []string) bool { return slices.Equal(args, w) }) {
				t.Errorf("%q: аргументы %q, ждали один из %q", c.text, args, c.want)
			}
		}
	}
}

// runPS запускает сценарий psProbe: токенизирует cmd, исполняет run, в папке dir; отдаёт вывод.
func runPS(t *testing.T, ps, dir, cmd, run string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	c := exec.CommandContext(ctx, ps, "-NoProfile", "-NonInteractive", "-Command", psProbe)
	c.Dir = dir
	c.Env = append(os.Environ(), "SA_CMD="+cmd, "SA_RUN="+run)
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("PowerShell для %q: %v\n%s", cmd, err, out)
	}
	return strings.ReplaceAll(string(out), "\r", "")
}

func unb64(t *testing.T, s string) string {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64 %q: %v", s, err)
	}
	return string(b)
}
