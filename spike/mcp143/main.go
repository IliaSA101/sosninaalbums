// Прототип спайка 143/Т0 (design §4.3): посредник `--mcp`, слушатель на сокете ОС и проверки С1, С2, С4.
// Не продуктовый код: в dev не идёт, живёт в ветке winqa/143-*. Один исходник собирается дважды:
// консольный spikec (драйверы, слушатель) и spike143 с подсистемой GUI (-ldflags -H=windowsgui) — «программа».
//
// Режимы (первый аргумент): --mcp, --serve, --selftest, --clientcheck, --c4; без аргумента — «окно программы»
// (метка window-opened-<pid> в папке --out: проверка, что ветка --mcp открывается до окна).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// opts — аргументы вида --ключ значение после режима (флаги без значения — «1»).
type opts map[string]string

func parseOpts(args []string) opts {
	o := opts{}
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") {
			continue
		}
		k := args[i][2:]
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			o[k] = args[i+1]
			i++
		} else {
			o[k] = "1"
		}
	}
	return o
}

func (o opts) str(k, def string) string {
	if v := o[k]; v != "" {
		return v
	}
	return def
}

func (o opts) num(k string, def int) int {
	if n, err := strconv.Atoi(o[k]); err == nil {
		return n
	}
	return def
}

// outDir — папка журналов и меток: --out, SPIKE_OUT или временная.
var outDir = ""

func initOut(o opts) {
	outDir = o.str("out", os.Getenv("SPIKE_OUT"))
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "spike143-out")
	}
	_ = os.MkdirAll(outDir, 0o777)
}

// logf пишет в spike.log папки out (GUI-exe без консоли иначе молчал бы). stderr посредника остаётся чистым.
func logf(format string, a ...any) {
	f, err := os.OpenFile(filepath.Join(outDir, "spike.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o666)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s pid=%d %s\n", time.Now().Format("15:04:05.000"), os.Getpid(), fmt.Sprintf(format, a...))
}

func main() {
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	var rest []string
	if len(os.Args) > 2 {
		rest = os.Args[2:]
	}
	o := parseOpts(rest)
	initOut(o)
	switch mode {
	case "--mcp":
		logf("mcp bridge start sock=%s", sockPath(o))
		os.Exit(runBridge(os.Stdin, os.Stdout, sockPath(o), probeEvery()))
	case "--serve":
		os.Exit(runServe(o))
	case "--selftest":
		os.Exit(runSelftest(o))
	case "--clientcheck":
		os.Exit(runClientCheck(o))
	case "--c4":
		os.Exit(runC4(o))
	default:
		// «Окно программы»: в настоящей программе здесь Wails и замок второго экземпляра.
		marker := filepath.Join(outDir, fmt.Sprintf("window-opened-%d", os.Getpid()))
		_ = os.WriteFile(marker, []byte(strings.Join(os.Args, " ")), 0o666)
	}
}

// probeEvery — период пробы сокета у заглушки; в продукте 2 с, в тестах короче (SPIKE_PROBE_MS).
func probeEvery() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("SPIKE_PROBE_MS")); err == nil && n > 0 {
		return time.Duration(n) * time.Millisecond
	}
	return 2 * time.Second
}
