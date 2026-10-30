package peercred

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Г-в (148, design §5): процесс «от администратора» (раннер CI) слушает, а помощник с урезанным токеном
// (runas /trustlevel:0x20000 — тот же пользователь, уровень «обычный») подключается и сверяет сервер.
// Помощник пишет по шагам, где (если где-то) отказ: OpenProcess или токен.

func init() { extraHelpers["elevdial"] = elevatedDial }

const elevatedResult = ".result"

// elevatedDial — роль помощника: подключается, по шагам повторяет sameFD и пишет итог в sock+".result".
func elevatedDial(sock string) int {
	c, err := net.DialTimeout("unix", sock, holdTime)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer c.Close()
	var b strings.Builder
	fmt.Fprintf(&b, "helper_elevated=%v\n", windows.GetCurrentProcessToken().IsElevated())
	if rc, err := c.(*net.UnixConn).SyscallConn(); err == nil {
		_ = rc.Control(func(fd uintptr) { peerSteps(&b, fd) })
	}
	ok, err := Same(c)
	fmt.Fprintf(&b, "same=%v\nsame_err=%v\ntoken_denied=%v\n", ok, err, errors.Is(err, ErrTokenDenied))
	tmp := sock + elevatedResult + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o666); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.Rename(tmp, sock+elevatedResult); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	_ = c.SetReadDeadline(time.Now().Add(holdTime))
	_, _ = io.Copy(io.Discard, c)
	return 0
}

// peerSteps — шаги sameFD по одному, каждый со своим итогом.
func peerSteps(b *strings.Builder, fd uintptr) {
	pid, err := ioctlPeerPID(fd)
	if err != nil {
		fmt.Fprintf(b, "getpeerpid=%s\n", errnoText(err))
		return
	}
	fmt.Fprintf(b, "getpeerpid=ok pid=%d\n", pid)
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		fmt.Fprintf(b, "openprocess=%s\n", errnoText(err))
		return
	}
	defer func() { _ = windows.CloseHandle(h) }()
	b.WriteString("openprocess=ok\n")
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok); err != nil {
		fmt.Fprintf(b, "token=%s\n", errnoText(err))
		return
	}
	defer func() { _ = tok.Close() }()
	b.WriteString("token=ok\n")
	if u, err := tok.GetTokenUser(); err == nil {
		fmt.Fprintf(b, "peer_sid=%s\n", u.User.Sid)
	}
}

// parseResult — строки key=value итога помощника.
func parseResult(text string) map[string]string {
	m := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			m[k] = v
		}
	}
	return m
}

func TestElevatedServerWindows(t *testing.T) {
	if os.Getenv("SA_PEER_FOREIGN") != "cred" {
		t.Skip("только CI: SA_PEER_FOREIGN=cred")
	}
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("тест запущен не от администратора: Г-в не проверяется (ручная проверка — docs/qa/agents.md)")
	}
	if _, err := exec.LookPath("runas"); err != nil {
		t.Skipf("runas недоступен: %v (ручная проверка — docs/qa/agents.md)", err)
	}
	dir := foreignDir(t)
	sock := foreignSock(dir)
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	exe := copySelf(t, dir)
	outFile := filepath.Join(dir, "elev.out")
	script := filepath.Join(dir, "elev.cmd") // без вложенных кавычек в строке runas
	body := fmt.Sprintf("@echo off\r\nset %s=elevdial\r\nset %s=%s\r\n\"%s\" > \"%s\" 2>&1\r\n", helperEnv, sockEnv, sock, exe, outFile)
	if err := os.WriteFile(script, []byte(body), 0o666); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("runas", "/trustlevel:0x20000", "cmd.exe /c "+script).CombinedOutput(); err != nil {
		t.Skipf("runas /trustlevel:0x20000 не запустил помощника: %v\n%s\n(ручная проверка — docs/qa/agents.md)", err, out)
	}
	report := func() string {
		out, _ := os.ReadFile(outFile)
		return string(out)
	}
	_ = l.(*net.UnixListener).SetDeadline(time.Now().Add(readyWait))
	c, err := l.Accept()
	if err != nil {
		t.Fatalf("помощник с урезанным токеном не подключился: %v\n%s", err, report())
	}
	defer c.Close()
	var text string
	for deadline := time.Now().Add(readyWait); ; time.Sleep(50 * time.Millisecond) {
		if b, err := os.ReadFile(sock + elevatedResult); err == nil {
			text = string(b)
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("помощник не записал итог за %v\n%s", readyWait, report())
		}
	}
	t.Logf("Г-в, итог помощника (урезанный токен -> сервер от администратора):\n%s", text)
	r := parseResult(text)
	switch {
	case r["same"] == "true":
		t.Log("Г-в подтверждена: процесс и токен «от администратора» открываются обычным процессом, SID совпал")
	case r["openprocess"] == "ok" && r["token"] != "ok" && r["token_denied"] == "true":
		t.Logf("Г-в НЕ подтверждена на шаге OpenProcessToken (%s): Same вернул ErrTokenDenied, посредник ответит textElevated", r["token"])
	default:
		t.Errorf("Г-в: отказ на шаге getpeerpid=%q openprocess=%q token=%q; Same=%s, err=%s", r["getpeerpid"], r["openprocess"], r["token"], r["same"], r["same_err"])
	}
}
