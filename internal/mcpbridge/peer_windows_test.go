package mcpbridge

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestSockDirHandleWindows: папка читается по дескриптору, а не по пути — точка соединения,
// файл вместо папки и отсутствующая папка отвергаются, годная папка проходит.
func TestSockDirHandleWindows(t *testing.T) {
	good := newSock(t)
	dir := filepath.Dir(good)
	if err := checkDirACL(dir); err != nil {
		t.Fatalf("годная папка: %v", err)
	}
	if err := checkDirACL(filepath.Join(dir, "нет")); err == nil {
		t.Error("отсутствующая папка прошла")
	}
	file := filepath.Join(shortDir(t), "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openDirNoFollow(file); !errors.Is(err, errUnsafe) {
		t.Errorf("файл вместо папки: %v", err)
	}
	link := filepath.Join(shortDir(t), "lnk")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, dir).CombinedOutput(); err != nil {
		t.Skipf("mklink /J не удалась: %v %s", err, out)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	if _, err := openDirNoFollow(link); !errors.Is(err, errUnsafe) {
		t.Errorf("точка соединения: %v, ждали errUnsafe", err)
	}
	if err := checkDirACL(link); err == nil {
		t.Error("точка соединения прошла проверку прав")
	}
}

// renameBlocked: переименование папки dir не удаётся (папка удержана); если вдруг удалось — вернуть назад.
func renameBlocked(t *testing.T, dir string) bool {
	t.Helper()
	moved := dir + "-moved"
	if err := os.Rename(dir, moved); err != nil {
		return true
	}
	if err := os.Rename(moved, dir); err != nil {
		t.Errorf("папку не вернуть: %v", err) // вызов бывает не из горутины теста: без Fatal
	}
	return false
}

// TestSockDirHeldWindows (148-11): пока checkSock не отпустил папку, mcp нельзя переименовать, а значит
// и подменить между проверкой, dial и сверкой собеседника; после release — можно; release повторяем.
func TestSockDirHeldWindows(t *testing.T) {
	sock := newSock(t)
	dir := filepath.Dir(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	present, release, err := checkSock(sock)
	if !present || err != nil {
		_ = ln.Close()
		t.Fatalf("checkSock: present=%v err=%v", present, err)
	}
	if !renameBlocked(t, dir) {
		t.Error("папка переименована, пока checkSock её держит")
	}
	release()
	release() // повтор безопасен
	_ = ln.Close()
	// дескриптор закрыт сразу, но антивирус или индексатор может держать папку мгновения
	for deadline := time.Now().Add(5 * time.Second); renameBlocked(t, dir); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("после release папка всё ещё не переименовывается")
		}
	}
}

// TestBridgeHoldsDirDuringPeerWindows (148-11): во время сверки собеседника, то есть между dial и
// решением «свой», папка сокета удержана — переименовать её нельзя; после подключения отпущена.
func TestBridgeHoldsDirDuringPeerWindows(t *testing.T) {
	sock := newSock(t)
	dir := filepath.Dir(sock)
	p := listenProg(t, sock)
	blocked := make(chan bool, 4)
	cfg := testCfg(sock)
	cfg.peer = func(net.Conn) (bool, error) {
		blocked <- renameBlocked(t, dir)
		return true, nil
	}
	h := startBridge(t, cfg)
	select {
	case ok := <-blocked:
		if !ok {
			t.Error("папку удалось переименовать во время сверки собеседника")
		}
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("сверка собеседника не вызвана")
	}
	c := p.accept()
	h.send(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if m := c.read(); m["method"] != "ping" {
		t.Errorf("в программу ушло %v", m)
	}
	if renameBlocked(t, dir) {
		t.Error("после подключения папка осталась удержана")
	}
}
