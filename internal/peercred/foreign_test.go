package peercred

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Чужая учётная запись — настоящая (148, design §5). Помощник — сам тестовый бинарник: его копию
// запускает чужой пользователь (Unix — sudo -n -u nobody, Windows — временный пользователь);
// включается переменной SA_PEER_FOREIGN, без неё тесты пропускаются (шаг CI пропуск считает провалом).

const (
	helperEnv = "SA_PEER_HELPER" // listen | dial
	sockEnv   = "SA_PEER_SOCK"   // путь сокета
	holdTime  = 10 * time.Second // сколько помощник держит соединение
)

// readyWait — сколько ждём «готов» помощника; var, а не const: Windows поднимает срок (foreign_windows_test.go).
var readyWait = 30 * time.Second

// extraHelpers — роли помощника, которые есть только на одной ОС (Windows: elevdial); регистрируются в init.
var extraHelpers = map[string]func(sock string) int{}

func TestMain(m *testing.M) {
	if mode := os.Getenv(helperEnv); mode != "" {
		os.Exit(helper(mode, os.Getenv(sockEnv)))
	}
	os.Exit(m.Run())
}

// helper — роль чужого процесса: слушает (принимает одно подключение) или подключается, пишет файл
// «готов» рядом с сокетом и держит соединение не дольше holdTime.
func helper(mode, sock string) int {
	if mode == "sleep" { // просто живой процесс (TestRepeatedPID)
		time.Sleep(holdTime)
		return 0
	}
	if sock == "" { // путь не строим от пустой переменной
		fmt.Fprintln(os.Stderr, sockEnv+" пуст")
		return 2
	}
	if f, ok := extraHelpers[mode]; ok {
		return f(sock)
	}
	var c net.Conn
	switch mode {
	case "listen":
		l, err := net.Listen("unix", sock)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer l.Close()
		_ = os.Chmod(sock, 0o777) // чужому (нашему тесту) надо уметь подключиться
		ready(sock)
		_ = l.(*net.UnixListener).SetDeadline(time.Now().Add(holdTime))
		if c, err = l.Accept(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	case "dial":
		var err error
		if c, err = net.DialTimeout("unix", sock, holdTime); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		ready(sock)
	default:
		fmt.Fprintln(os.Stderr, "неизвестная роль", mode)
		return 2
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(holdTime))
	_, _ = io.Copy(io.Discard, c) // до закрытия с нашей стороны или конца срока
	return 0
}

// ready пишет рядом с сокетом «готов» с PID помощника: тест сверяет его с PID, который ОС называет
// собеседником (Windows, peerPIDOf) — иначе «не свой» доказывал бы лишь любой чужой процесс.
func ready(sock string) { _ = os.WriteFile(sock+".ready", []byte(strconv.Itoa(os.Getpid())), 0o666) }

// peerPIDOf — PID собеседника по запросу к ОС; задан только на Windows (foreign_windows_test.go).
var peerPIDOf func(c net.Conn) (uint32, error)

// checkPeerPID: PID, который ОС называет собеседником c, равен PID помощника из файла «готов».
// Файл появляется до записи содержимого — читаем с повтором.
func (s *foreignSetup) checkPeerPID(t *testing.T, c net.Conn) {
	t.Helper()
	if peerPIDOf == nil {
		return
	}
	var want int
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		b, _ := os.ReadFile(s.sock + ".ready")
		if n, err := strconv.Atoi(string(b)); err == nil && n > 0 {
			want = n
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("в файле «готов» нет PID помощника\n%s", s.report())
		}
	}
	got, err := peerPIDOf(c)
	if err != nil {
		t.Fatalf("запрос PID собеседника: %v", err)
	}
	if int(got) != want {
		t.Errorf("ОС называет собеседником PID %d, помощник — %d", got, want)
	}
}

// lockedBuf — вывод помощника: пишет горутина exec, читает тест, пока помощник ещё жив.
type lockedBuf struct {
	mu sync.Mutex
	b  []byte
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.b = append(l.b, p...)
	return len(p), nil
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return string(l.b)
}

// foreignSetup — запущенный помощник; sock — сокет в папке dir.
type foreignSetup struct {
	dir, sock string
	cmd       *exec.Cmd
	out       *lockedBuf
	done      chan error
}

// copySelf кладёт копию тестового бинарника в dir: оригинал лежит в папке сборки, чужому недоступной.
func copySelf(t *testing.T, dir string) string {
	t.Helper()
	exe := testkit.SelfExe(t)
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	copyExe := filepath.Join(dir, "peerhelper"+filepath.Ext(exe))
	if err := os.WriteFile(copyExe, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return copyExe
}

// startHelper копирует тестовый бинарник в dir, запускает его от чужого и (waitReady) ждёт «готов».
func startHelper(t *testing.T, dir, mode string, waitReady bool) *foreignSetup {
	t.Helper()
	copyExe := copySelf(t, dir)
	s := &foreignSetup{dir: dir, sock: foreignSock(dir), out: &lockedBuf{}, done: make(chan error, 1)}
	s.cmd = foreignCmd(t, copyExe, mode, s.sock)
	s.cmd.Stdout, s.cmd.Stderr = s.out, s.out
	if err := s.cmd.Start(); err != nil {
		t.Fatalf("запуск помощника: %v", err)
	}
	go func() { s.done <- s.cmd.Wait() }()
	t.Cleanup(func() { s.stop(t) })
	if waitReady {
		s.waitReady(t)
	}
	return s
}

// report — вывод помощника и (Windows) его helper.err/helper.out, для сообщений о провале.
func (s *foreignSetup) report() string {
	out := s.out.String()
	for _, name := range []string{"helper.err", "helper.out"} {
		if b, _ := os.ReadFile(filepath.Join(s.dir, name)); len(b) > 0 {
			out += "\n" + name + ":\n" + string(b)
		}
	}
	return out
}

// foreignSock — сокет в подпапке s: она открыта всем, а сама папка dir с копией бинарника — нет.
func foreignSock(dir string) string { return filepath.Join(dir, "s", "sock") }

// removeDir удаляет папку с повтором (Windows держит файлы только что вышедшего процесса);
// неудача — ошибка теста, а не молчание.
func removeDir(t *testing.T, dir string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := os.RemoveAll(dir)
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("папка помощника не удалена: %v", err)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (s *foreignSetup) waitReady(t *testing.T) {
	t.Helper()
	deadline := time.After(readyWait)
	for {
		if _, err := os.Stat(s.sock + ".ready"); err == nil {
			return
		}
		select {
		case err := <-s.done:
			s.done <- err
			t.Fatalf("помощник вышел до «готов»: %v\n%s", err, s.report())
		case <-deadline:
			t.Fatalf("помощник не готов за %v\n%s", readyWait, s.report())
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// stop ждёт выхода помощника: он держит соединение не дольше holdTime, а наше закрыто раньше.
func (s *foreignSetup) stop(t *testing.T) {
	select {
	case err := <-s.done:
		s.done <- err
	case <-time.After(holdTime + 30*time.Second):
		_ = s.cmd.Process.Kill()
		t.Errorf("помощник не вышел\n%s", s.report())
	}
}

// Чужой подключается к нам: сервер видит клиента другой учётной записи.
func TestForeignClient(t *testing.T) {
	skipUnlessForeign(t)
	dir := foreignDir(t)
	l, err := net.Listen("unix", foreignSock(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_ = os.Chmod(foreignSock(dir), 0o777) // чужой должен уметь подключиться
	s := startHelper(t, dir, "dial", false)
	type accepted struct {
		c   net.Conn
		err error
	}
	got := make(chan accepted, 1)
	go func() {
		c, err := l.Accept()
		got <- accepted{c, err}
	}()
	// Помощник вышел до подключения - это провал сразу, а не через readyWait (так ждали 250 с).
	var c net.Conn
	select {
	case a := <-got:
		if a.err != nil {
			t.Fatalf("чужой не подключился: %v\n%s", a.err, s.report())
		}
		c = a.c
	case err := <-s.done:
		s.done <- err
		t.Fatalf("помощник вышел, не подключившись: %v\n%s", err, s.report())
	case <-time.After(readyWait):
		t.Fatalf("чужой не подключился за %v\n%s", readyWait, s.report())
	}
	defer c.Close()
	s.waitReady(t)
	s.checkPeerPID(t, c)
	if ok, err := Same(c); ok || err != nil {
		t.Errorf("Same(чужой клиент) = %v, %v; ждали false, nil", ok, err)
	}
}

// Мы подключаемся к чужому: клиент видит слушателя другой учётной записи.
func TestForeignServer(t *testing.T) {
	skipUnlessForeign(t)
	s := startHelper(t, foreignDir(t), "listen", true)
	c, err := net.DialTimeout("unix", s.sock, 5*time.Second)
	if err != nil {
		t.Fatalf("подключение к чужому: %v\n%s", err, s.report())
	}
	defer c.Close()
	s.checkPeerPID(t, c)
	if ok, err := Same(c); ok || err != nil {
		t.Errorf("Same(чужой сервер) = %v, %v; ждали false, nil", ok, err)
	}
}
