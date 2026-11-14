package peercred

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// sioAfUnixGetPeerPID = _WSAIOR(IOC_VENDOR, 256) из afunix.h: в x/sys/windows константы нет.
const sioAfUnixGetPeerPID = 0x58000100

// ioctlPeerPID — запрос PID собеседника; тест подменяет его (ответ «не поддерживается»).
var ioctlPeerPID = func(fd uintptr) (uint32, error) {
	var pid, n uint32
	err := windows.WSAIoctl(windows.Handle(fd), sioAfUnixGetPeerPID, nil, 0,
		(*byte)(unsafe.Pointer(&pid)), uint32(unsafe.Sizeof(pid)), &n, nil, 0) //nolint:gosec // G103: WSAIoctl пишет ULONG в буфер размером с него
	return pid, err
}

// osVerdict — решение «запрос PID не поддерживается»: один раз за запуск, пробой на своей паре
// (verdict.go). Указатель, чтобы тест мог начать заново.
var osVerdict = new(verdict)

// selfProbe — true, если запрос PID не поддерживается и на своей паре сокетов. Любая другая
// неудача пробы (нет временной папки, путь длинен, таймаут) — false: закрыто по умолчанию.
// Тест подменяет.
var selfProbe = func() bool {
	dir, err := os.MkdirTemp("", "sap")
	if err != nil {
		return false
	}
	defer func() { _ = os.RemoveAll(dir) }()
	l, err := net.Listen("unix", filepath.Join(dir, "p"))
	if err != nil {
		return false
	}
	defer func() { _ = l.Close() }()
	type accepted struct {
		c   net.Conn
		err error
	}
	got := make(chan accepted, 1)
	go func() {
		c, err := l.Accept()
		got <- accepted{c, err}
	}()
	cl, err := net.DialTimeout("unix", l.Addr().String(), 2*time.Second)
	if err != nil {
		return false
	}
	defer func() { _ = cl.Close() }()
	var srv accepted
	select {
	case srv = <-got:
	case <-time.After(2 * time.Second):
		return false
	}
	if srv.err != nil {
		return false
	}
	defer func() { _ = srv.c.Close() }()
	uc, ok := srv.c.(*net.UnixConn)
	if !ok {
		return false
	}
	rc, err := uc.SyscallConn()
	if err != nil {
		return false
	}
	var qerr error
	if err := rc.Control(func(fd uintptr) { _, qerr = ioctlPeerPID(fd) }); err != nil {
		return false
	}
	return isNotSupported(qerr)
}

// isNotSupported — ответ запроса PID похож на «ОС не умеет» (старые сборки Windows 10).
func isNotSupported(err error) bool {
	return errors.Is(err, windows.WSAEOPNOTSUPP) || errors.Is(err, windows.WSAEINVAL)
}

// osUnsupported — решение для Probe.
func osUnsupported() bool { return osVerdict.decide(selfProbe) }

// sameFD: PID собеседника → токен процесса → SID пользователя. start — начало проверки:
// процесс, созданный позже, — повторный PID после выхода настоящего собеседника (148-7).
// Время создания и start — часы с тактом до 15,6 мс: окно подмены не «микросекунды», а до такта;
// ложный отказ при переводе часов назад принят.
func sameFD(fd uintptr, start time.Time) (bool, error) {
	pid, err := ioctlPeerPID(fd)
	if err != nil {
		// «Не поддерживается» — только если так отвечает и своя пара (проба раз за запуск, 148-11);
		// иначе чужое подключение с той же ошибкой открыло бы путь. Остальное — обычная ошибка.
		return false, osVerdict.classify(err, isNotSupported(err), selfProbe)
	}
	if pid == 0 || pid == 4 { // Idle и System — не пользователь
		return false, nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return false, nil // чужая учётная запись или процесса уже нет
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	var created, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exit, &kernel, &user); err != nil {
		return false, err
	}
	if time.Unix(0, created.Nanoseconds()).After(start) {
		return false, nil
	}
	// PID фиксируется при listen: слушатель мог передать сокет другому процессу и выйти. Объект
	// завершённого процесса жив, пока на него есть дескриптор, и токен у него тот же — верить ему
	// нельзя. Время выхода у работающего процесса нулевое. Права открытия не расширяем
	// (SYNCHRONIZE изменил бы вердикт для процесса «от администратора», ErrTokenDenied).
	if exit.HighDateTime != 0 || exit.LowDateTime != 0 {
		return false, nil
	}
	var tok windows.Token
	if err := windows.OpenProcessToken(h, windows.TOKEN_QUERY, &tok); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return false, ErrTokenDenied
		}
		return false, err
	}
	defer func() { _ = tok.Close() }()
	peer, err := tok.GetTokenUser()
	if err != nil {
		return false, err
	}
	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return false, err
	}
	return peer.User.Sid.Equals(me.User.Sid), nil
}
