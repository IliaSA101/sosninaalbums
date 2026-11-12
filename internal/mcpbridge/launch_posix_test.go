//go:build !windows

package mcpbridge

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Помощник запуска: тестовый бинарник, запущенный spawnProgram, пишет отчёт о себе в report.txt в
// рабочей папке и выходит. Окружения и аргументов для переключения у него нет — белый список и
// пустой argv как раз проверяются, поэтому режим определяет файл-метка в рабочей папке (это папка
// exe, spawnProgram ставит её сам); в обычном прогоне теста такого файла в папке пакета нет.
const (
	helperName   = "sa-start-helper"
	helperMarker = "sa-start-helper.marker"
)

func init() {
	if _, err := os.Lstat(helperMarker); err == nil {
		os.Exit(startHelper())
	}
}

func startHelper() int {
	var b strings.Builder
	fmt.Fprintf(&b, "args=%d\n", len(os.Args)-1)
	for _, kv := range os.Environ() {
		fmt.Fprintf(&b, "env=%s\n", kv)
	}
	wd, _ := os.Getwd()
	fmt.Fprintf(&b, "cwd=%s\n", wd)
	fmt.Fprintf(&b, "session_leader=%v\n", os.Getpid() == mustSid())
	if runtime.GOOS == "linux" {
		for _, fd := range listFDs() {
			fmt.Fprintf(&b, "fd=%s->%s\n", fd.num, fd.target)
		}
	}
	// Запись целиком в последнюю очередь: тест ждёт появления файла и читает его сразу.
	tmp := "report.tmp"
	if os.WriteFile(tmp, []byte(b.String()), 0o600) != nil || os.Rename(tmp, "report.txt") != nil {
		return 1
	}
	return 0
}

type procFD struct{ num, target string }

// listFDs — открытые дескрипторы процесса (Linux), кроме того, которым открыт сам /proc/self/fd.
func listFDs() []procFD {
	var out []procFD
	ents, _ := os.ReadDir("/proc/self/fd")
	for _, e := range ents {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil || strings.HasPrefix(target, "/proc/") && strings.HasSuffix(target, "/fd") {
			continue
		}
		out = append(out, procFD{e.Name(), target})
	}
	return out
}

// runtimeOwnFD: файл лимита cgroup, который рантайм Go 1.25+ открывает сам при старте (GOMAXPROCS
// по лимиту контейнера) и держит до выхода. Это не наследство от родителя: под лимитом cpu (CI,
// контейнеры) такой дескриптор есть у любого Go-процесса, локально без лимита его нет.
func runtimeOwnFD(target string) bool {
	if !strings.HasPrefix(target, "/sys/fs/cgroup/") {
		return false
	}
	switch filepath.Base(target) {
	case "cpu.max", "cpu.cfs_quota_us", "cpu.cfs_period_us":
		return true
	}
	return false
}

func mustSid() int {
	sid, err := unix.Getsid(0)
	if err != nil {
		return -1
	}
	return sid
}

// runHelper запускает spawnProgram на ссылке-помощнике и возвращает строки отчёта.
func runHelper(t *testing.T, env []string) []string {
	t.Helper()
	dir := testkit.TempDir(t)
	exe := filepath.Join(dir, helperName)
	if err := os.Symlink(testkit.SelfExe(t), exe); err != nil {
		t.Skipf("ссылка недоступна: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, helperMarker), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := spawnProgram(exe, env); err != nil {
		t.Fatalf("spawnProgram: %v", err)
	}
	report := filepath.Join(dir, "report.txt")
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(report); err == nil {
			return strings.Split(strings.TrimSpace(string(b)), "\n")
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("помощник не оставил отчёт: программа не запустилась")
	return nil
}

func hasLine(lines []string, prefix string) []string {
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			out = append(out, l)
		}
	}
	return out
}

// TestStartProgramFDs: ребёнок запущен без аргументов, в своём сеансе, в папке exe; stdin, stdout и
// stderr — /dev/null, дескрипторов родителя выше 2 нет (Linux); окружение — ровно переданное.
// Собственный дескриптор рантайма ребёнка (cpu.max своей cgroup) не в счёт: он не от родителя.
func TestStartProgramFDs(t *testing.T) {
	// Лишний дескриптор у родителя: если бы он наследовался, помощник увидел бы его.
	extra, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = extra.Close() }()
	// Дескриптор без CLOEXEC, как каналы и cgroup, унаследованные от клиента или раннера CI:
	// syscall.Open флаг сам не ставит, os.Open — ставит.
	rawPath := filepath.Join(testkit.TempDir(t), "leak-probe")
	if err := os.WriteFile(rawPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := syscall.Open(rawPath, syscall.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = syscall.Close(raw) }()
	if _, err := os.Stdin.Stat(); err != nil {
		t.Skip("нет stdin")
	}
	t.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--remote-debugging-port=9222")
	t.Setenv("LD_PRELOAD", "")
	t.Setenv("SA_FX_SECRET", "s")
	var parentFDs []procFD
	if runtime.GOOS == "linux" {
		parentFDs = listFDs()
	}
	lines := runHelper(t, childEnv(append(os.Environ(), "DISPLAY=:7"), runtime.GOOS))

	if got := hasLine(lines, "args="); len(got) != 1 || got[0] != "args=0" {
		t.Errorf("аргументы: %v", got)
	}
	if got := hasLine(lines, "session_leader="); len(got) != 1 || got[0] != "session_leader=true" {
		t.Errorf("ребёнок не лидер нового сеанса: %v", got)
	}
	if got := hasLine(lines, "cwd="); len(got) != 1 || filepath.Base(strings.TrimPrefix(got[0], "cwd=")) == "" {
		t.Errorf("рабочая папка: %v", got)
	}
	for _, e := range hasLine(lines, "env=") {
		name, _, _ := strings.Cut(strings.TrimPrefix(e, "env="), "=")
		if !envAllowed(name, runtime.GOOS) {
			t.Errorf("в окружении ребёнка %q", e)
		}
	}
	if !slices.Contains(lines, "env=DISPLAY=:7") {
		t.Errorf("DISPLAY не дошёл: %v", hasLine(lines, "env="))
	}
	if runtime.GOOS != "linux" {
		return
	}
	// Снимок родителя до проверки: дескрипторы выше 2 (в том числе сырой без CLOEXEC и ваш канал
	// раннера) в ребёнке появиться не должны — ни по номеру, ни по цели.
	parent := map[string]bool{}
	for _, fd := range parentFDs {
		if n, err := strconv.Atoi(fd.num); err == nil && n > 2 {
			parent[fd.target] = true
		}
	}
	var low []string
	for _, l := range hasLine(lines, "fd=") {
		num, target, _ := strings.Cut(strings.TrimPrefix(l, "fd="), "->")
		switch {
		case num == "0" || num == "1" || num == "2":
			low = append(low, l)
		case runtimeOwnFD(target):
			// собственный дескриптор рантайма ребёнка, не унаследованный
		case parent[target]:
			t.Errorf("у ребёнка унаследован дескриптор родителя: %s", l)
		default:
			t.Errorf("у ребёнка лишний дескриптор выше 2: %s", l)
		}
	}
	for _, want := range []string{"fd=0->/dev/null", "fd=1->/dev/null", "fd=2->/dev/null"} {
		if !slices.Contains(low, want) {
			t.Errorf("нет %q среди %v", want, low)
		}
	}
}

// TestStartProgramEmptyEnvNotInherited: пустое окружение — это пустое окружение, а не «унаследовать».
func TestStartProgramEmptyEnvNotInherited(t *testing.T) {
	t.Setenv("SA_FX_SECRET", "s")
	lines := runHelper(t, childEnv(nil, runtime.GOOS))
	if got := hasLine(lines, "env="); len(got) != 0 {
		t.Errorf("окружение унаследовано: %v", got)
	}
}

// TestStartProgramNoExe: нет файла — ошибка запуска, не паника и не зависание.
func TestStartProgramNoExe(t *testing.T) {
	if err := spawnProgram(filepath.Join(testkit.TempDir(t), "нет-такого"), []string{}); err == nil {
		t.Error("запуск несуществующего файла без ошибки")
	}
}
