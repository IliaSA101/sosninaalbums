package peercred

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// sameAt вызывает sameFD на сокете соединения с заданным началом проверки.
func sameAt(t *testing.T, c net.Conn, start time.Time) (bool, error) {
	t.Helper()
	rc, err := c.(*net.UnixConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var ok bool
	var ierr error
	if err := rc.Control(func(fd uintptr) { ok, ierr = sameFD(fd, start) }); err != nil {
		t.Fatal(err)
	}
	return ok, ierr
}

// Повторный PID (148-7): процесс, созданный позже начала проверки, — «не свой», созданный раньше — свой.
func TestRepeatedPID(t *testing.T) {
	orig := ioctlPeerPID
	defer func() { ioctlPeerPID = orig }()
	before := time.Now()
	time.Sleep(50 * time.Millisecond) // больше такта часов Windows (15,6 мс)
	child := exec.Command(testkit.SelfExe(t))
	child.Env = append(os.Environ(), helperEnv+"=sleep")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	time.Sleep(50 * time.Millisecond)
	after := time.Now()
	pid := uint32(child.Process.Pid) //nolint:gosec // G115: PID Windows — DWORD
	ioctlPeerPID = func(uintptr) (uint32, error) { return pid, nil }
	server, _ := pair(t)
	if ok, err := sameAt(t, server, before); ok || err != nil {
		t.Errorf("процесс создан после начала проверки: sameFD = %v, %v; ждали false, nil", ok, err)
	}
	if ok, err := sameAt(t, server, after); !ok || err != nil {
		t.Errorf("процесс создан до начала проверки: sameFD = %v, %v; ждали true, nil", ok, err)
	}
}

// freshVerdict начинает решение «не поддерживается» заново и возвращает прежнее при выходе из теста.
func freshVerdict(t *testing.T) {
	t.Helper()
	origV, origP, origI := osVerdict, selfProbe, ioctlPeerPID
	osVerdict = new(verdict)
	t.Cleanup(func() { osVerdict, selfProbe, ioctlPeerPID = origV, origP, origI })
}

// Windows без запроса PID у сокета (старые сборки) — ErrUnsupported, а не отказ и не «свой»:
// проба на своей паре тоже получает «не поддерживается» (запрос подменён целиком).
func TestUnsupportedErr(t *testing.T) {
	for _, errno := range []error{windows.WSAEOPNOTSUPP, windows.WSAEINVAL} {
		freshVerdict(t)
		ioctlPeerPID = func(uintptr) (uint32, error) { return 0, errno }
		server, _ := pair(t)
		ok, err := Same(server)
		if ok || !errors.Is(err, ErrUnsupported) {
			t.Errorf("%v: Same = %v, %v; ждали false, ErrUnsupported", errno, ok, err)
		}
		if err := Probe(); !errors.Is(err, ErrUnsupported) {
			t.Errorf("%v: Probe = %v; ждали ErrUnsupported", errno, err)
		}
	}
	// Любая другая ошибка запроса — обычная ошибка, не ErrUnsupported (закрыто по умолчанию).
	freshVerdict(t)
	ioctlPeerPID = func(uintptr) (uint32, error) { return 0, windows.WSAECONNRESET }
	server, _ := pair(t)
	if ok, err := Same(server); ok || err == nil || errors.Is(err, ErrUnsupported) {
		t.Errorf("прочая ошибка: Same = %v, %v", ok, err)
	}
}

// 148-11: проба на своей паре сказала «запрос работает» — тогда WSAEINVAL и WSAEOPNOTSUPP на
// подключении остаются отказом, ErrUnsupported не возвращается; проба одна на запуск.
func TestUnsupportedNeedsProbe(t *testing.T) {
	freshVerdict(t)
	probes := 0
	selfProbe = func() bool { probes++; return false }
	for _, errno := range []error{windows.WSAEINVAL, windows.WSAEOPNOTSUPP, windows.WSAEINVAL} {
		ioctlPeerPID = func(uintptr) (uint32, error) { return 0, errno }
		server, _ := pair(t)
		if ok, err := Same(server); ok || err == nil || errors.Is(err, ErrUnsupported) {
			t.Errorf("%v после удачной пробы: Same = %v, %v; ждали отказ с обычной ошибкой", errno, ok, err)
		}
	}
	if probes != 1 {
		t.Errorf("проб: %d, ждали 1", probes)
	}
	if err := Probe(); err != nil {
		t.Errorf("Probe после удачной пробы = %v", err)
	}
}

// Настоящая проба на своей паре: на этой Windows запрос PID работает (TestPeerPIDQuery), значит «умеет».
func TestSelfProbeReal(t *testing.T) {
	freshVerdict(t)
	if selfProbe() {
		t.Error("проба на своей паре решила «не поддерживается», хотя запрос PID работает")
	}
	if err := Probe(); err != nil {
		t.Errorf("Probe = %v", err)
	}
}

// PID 0 и 4 (Idle, System) и несуществующий процесс — «не свой», без ошибки.
func TestPseudoPeers(t *testing.T) {
	orig := ioctlPeerPID
	defer func() { ioctlPeerPID = orig }()
	for _, pid := range []uint32{0, 4, 0xFFFFFFF0} {
		ioctlPeerPID = func(uintptr) (uint32, error) { return pid, nil }
		server, _ := pair(t)
		if ok, err := Same(server); ok || err != nil {
			t.Errorf("pid %d: Same = %v, %v; ждали false, nil", pid, ok, err)
		}
	}
}

// errnoText — ошибка с числом errno: для гипотезы Г-б важно, WSAEINVAL это или WSAEOPNOTSUPP.
func errnoText(err error) string {
	var en syscall.Errno
	if errors.As(err, &en) {
		return fmt.Sprintf("%v (errno %d, 0x%x)", err, uintptr(en), uintptr(en))
	}
	return err.Error()
}

// Г-б: SIO_AF_UNIX_GETPEERPID отвечает на обоих концах и называет наш же PID; при отказе в сообщении errno.
func TestPeerPIDQuery(t *testing.T) {
	server, client := pair(t)
	for name, c := range map[string]net.Conn{"сервер видит клиента": server, "клиент видит сервер": client} {
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
			t.Errorf("%s: SIO_AF_UNIX_GETPEERPID: %s", name, errnoText(qerr))
			continue
		}
		t.Logf("%s: SIO_AF_UNIX_GETPEERPID pid=%d, свой=%d", name, pid, os.Getpid())
		if int(pid) != os.Getpid() {
			t.Errorf("%s: pid %d, ждали свой %d", name, pid, os.Getpid())
		}
	}
}
