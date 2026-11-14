package peercred

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Гипотеза аудита 148 (0.32-148-11): SIO_AF_UNIX_GETPEERPID отдаёт PID процесса, вызвавшего listen,
// а не того, кто сокет принимает. Если так, слушатель A передаёт сокет потомку B и выходит, а его
// PID достаётся чужому процессу — сверка по PID вводит в заблуждение. Защита — не сверка образа, а
// удержание папки mcp посредником (mcpbridge.checkSock); этот тест проверяет только факт.

// selfEnv — путь тестового бинарника для запуска B из помощника A.
const selfEnv = "SA_PEER_SELF"

func init() {
	extraHelpers["handoff"] = handoffA
	extraHelpers["adopt"] = handoffB
}

// handoffA — слушатель: слушает, запускает B, дублирует ему дескриптор слушающего сокета, пишет свой
// PID в «готов» и выходит БЕЗ закрытия слушателя (закрытие Go удалило бы файл сокета).
func handoffA(sock string) int {
	l, err := net.Listen("unix", sock)
	if err != nil {
		return handoffFail("listen", err)
	}
	var h windows.Handle
	rc, err := l.(*net.UnixListener).SyscallConn()
	if err != nil {
		return handoffFail("SyscallConn", err)
	}
	if err := rc.Control(func(fd uintptr) { h = windows.Handle(fd) }); err != nil {
		return handoffFail("Control", err)
	}
	exe := os.Getenv(selfEnv) // путь к бинарю даёт тест (testkit.SelfExe): в помощнике он недоступен
	if exe == "" {
		return handoffFail("Getenv", errors.New(selfEnv+" пуст"))
	}
	b := exec.Command(exe)
	b.Env = append(os.Environ(), helperEnv+"=adopt", sockEnv+"="+sock)
	if err := b.Start(); err != nil {
		return handoffFail("запуск B", err)
	}
	pid := uint32(b.Process.Pid) //nolint:gosec // G115: PID Windows — DWORD
	hb, err := windows.OpenProcess(windows.PROCESS_DUP_HANDLE, false, pid)
	if err != nil {
		return handoffFail("OpenProcess(B)", err)
	}
	var dup windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), h, hb, &dup, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return handoffFail("DuplicateHandle", err)
	}
	_ = os.WriteFile(sock+".b", []byte(strconv.Itoa(b.Process.Pid)), 0o666)
	_ = os.WriteFile(sock+".dup", []byte(strconv.FormatUint(uint64(dup), 10)), 0o666)
	ready(sock) // PID A — после передачи
	return 0    // выход без Close: Go удалил бы файл сокета, слушатель живёт в B
}

// handoffB — получатель: берёт дублированный дескриптор и принимает подключения «вручную» (accept из ws2_32),
// не закрывая их; ошибки не важны — тесту нужен лишь живой слушающий сокет в процессе, не вызывавшем listen.
func handoffB(sock string) int {
	var v uint64
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		b, _ := os.ReadFile(sock + ".dup")
		if n, err := strconv.ParseUint(string(b), 10, 64); err == nil && n != 0 {
			v = n
			break
		}
		if time.Now().After(deadline) {
			return 1
		}
	}
	var data windows.WSAData
	if err := windows.WSAStartup(0x0202, &data); err == nil {
		accept := windows.NewLazySystemDLL("ws2_32.dll").NewProc("accept")
		go func() {
			for {
				r, _, _ := accept.Call(uintptr(v), 0, 0)
				if r == ^uintptr(0) {
					return
				}
				adopted = append(adopted, r) // принятые не закрываем
			}
		}()
	}
	time.Sleep(holdTime)
	return 0
}

// adopted — принятые B подключения: держатся до выхода процесса.
var adopted []uintptr

func handoffFail(what string, err error) int {
	_ = os.WriteFile(os.Getenv(sockEnv)+".fail", []byte(what+": "+err.Error()), 0o666)
	return 1
}

// TestListenerHandoffWindows: A слушает, передаёт сокет B и выходит; клиент подключается к сокету и
// спрашивает PID собеседника. Ожидаем PID A (фиксируется при listen) — тогда Same для вышедшего A
// даёт «не свой» без ошибки. PID B — гипотеза опровергнута (ответ достаётся принимающему): тогда
// проверка по PID надёжнее, чем думали, но решение 0.32-148-11 надо пересмотреть.
func TestListenerHandoffWindows(t *testing.T) {
	dir := testkit.TempDir(t)
	sock := filepath.Join(dir, "s")
	self := testkit.SelfExe(t)
	a := exec.Command(self)
	a.Env = append(os.Environ(), helperEnv+"=handoff", sockEnv+"="+sock, selfEnv+"="+self)
	// Вывод не собираем: B унаследует трубу и продержал бы Run до своего выхода; причину даёт файл .fail.
	err := a.Run() // A выходит сам, сразу после передачи
	fail, _ := os.ReadFile(sock + ".fail")
	if err != nil || len(fail) > 0 {
		t.Fatalf("помощник A: %v %s", err, fail)
	}
	readInt := func(path string) int {
		b, rerr := os.ReadFile(path)
		n, perr := strconv.Atoi(string(b))
		if rerr != nil || perr != nil {
			t.Fatalf("%s: %v %v", filepath.Base(path), rerr, perr)
		}
		return n
	}
	pidA, pidB := readInt(sock+".ready"), readInt(sock+".b")
	if bh, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pidB)); err == nil { //nolint:gosec // G115: PID Windows — DWORD
		t.Cleanup(func() { _ = windows.TerminateProcess(bh, 0); _ = windows.CloseHandle(bh) })
	}
	if pidA == pidB {
		t.Fatalf("A и B — один процесс (%d)", pidA)
	}
	c, err := net.DialTimeout("unix", sock, 5*time.Second)
	if err != nil {
		t.Fatalf("подключение к сокету, переданному B: %v (A=%d, B=%d)", err, pidA, pidB)
	}
	defer c.Close()
	rc, err := c.(*net.UnixConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var pid uint32
	var qerr error
	if err := rc.Control(func(fd uintptr) { pid, qerr = ioctlPeerPID(fd) }); err != nil {
		t.Fatal(err)
	}
	if qerr != nil {
		t.Fatalf("запрос PID: %s", errnoText(qerr))
	}
	same, serr := Same(c)
	t.Logf("A (слушал, вышел) = %d, B (получил сокет) = %d, ОС называет %d; Same = %v, %v", pidA, pidB, pid, same, serr)
	switch int(pid) {
	case pidA:
		t.Log("гипотеза подтверждена: PID фиксируется при listen")
		if same || serr != nil {
			t.Errorf("процесса A уже нет: Same = %v, %v; ждали false, nil", same, serr)
		}
	case pidB:
		t.Errorf("гипотеза опровергнута: ОС называет процесс, получивший сокет (B=%d), а не вызвавший listen (A=%d)", pidB, pidA)
	default:
		t.Errorf("ОС называет PID %d: ни A (%d), ни B (%d)", pid, pidA, pidB)
	}
}
