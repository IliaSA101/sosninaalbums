package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// res печатает строку результата: RESULT<TAB>имя<TAB>OK|FAIL|INFO<TAB>подробности (одной строкой, UTF-8).
func res(name, status, format string, a ...any) {
	d := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(fmt.Sprintf(format, a...))
	fmt.Printf("RESULT\t%s\t%s\t%s\n", name, status, d)
}

func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

const rawInit = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"spike-raw","version":"1"}}}`

// rawRoundtrip: подключиться к сокету, отправить initialize, прочитать строку ответа.
func rawRoundtrip(path string) (string, error) {
	c, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		return "", err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := c.Write([]byte(rawInit + "\n")); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	return strings.TrimSpace(line), err
}

// runSelftest — С2 (а) и (д) в одном процессе от имени того, кто запустил: путь по профилю, Listen, подключение
// клиентом SDK, Close и исчезновение файла. Результаты — строки RESULT на stdout.
func runSelftest(o opts) int {
	bad := 0
	fail := func(name, f string, a ...any) { bad++; res(name, "FAIL", f, a...) }
	path := sockPath(o)
	cur, _ := user.Current()
	name := "?"
	if cur != nil {
		name = cur.Username
	}
	res("env", "INFO", "user=%s APPDATA=%q knownFolder=%q GOOS-path=%q", name, os.Getenv("APPDATA"), appDataDir(), path)
	res("path", "INFO", "bytes=%d (sun_path 108) utf8=%v nonASCII=%v", len(path), utf8.ValidString(path), hasNonASCII(path))
	if len(path) >= 108 {
		fail("path-fits", "путь %d байт не помещается в sun_path", len(path))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, _, err := startProgram(ctx, path, true)
	if err != nil {
		fail("listen", "%v", err)
		return 1
	}
	res("listen", "OK", "слушает %s", path)
	if st, err := os.Lstat(path); err == nil {
		res("sock-file", "INFO", "mode=%v size=%d", st.Mode(), st.Size())
	} else {
		fail("sock-file", "после Listen файла нет: %v", err)
	}

	if line, err := rawRoundtrip(path); err != nil || !strings.Contains(line, "sosnina-albums") {
		fail("connect-raw", "err=%v line=%.200q", err, line)
	} else {
		res("connect-raw", "OK", "initialize через сокет ответил")
	}
	c, err := net.DialTimeout("unix", path, 3*time.Second)
	if err != nil {
		fail("connect-sdk", "dial: %v", err)
	} else {
		cl := mcp.NewClient(&mcp.Implementation{Name: "selftest", Version: "1"}, nil)
		cs, err := cl.Connect(ctx, &mcp.IOTransport{Reader: c, Writer: c}, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
		if err != nil {
			fail("connect-sdk", "Connect: %v", err)
		} else {
			tl, err := cs.ListTools(ctx, nil)
			if err != nil || len(tl.Tools) != 2 {
				fail("connect-sdk", "tools/list err=%v", err)
			} else if cr, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "program_status"}); err != nil || len(cr.Content) == 0 {
				fail("connect-sdk", "tools/call err=%v", err)
			} else {
				res("connect-sdk", "OK", "клиент SDK: 2 инструмента, program_status вызван")
			}
			_ = cs.Close()
		}
	}

	_ = p.Close()
	if _, err := os.Lstat(path); err == nil {
		fail("close-removes-file", "после Close файл сокета остался")
		_ = os.Remove(path)
	} else {
		res("close-removes-file", "OK", "файл удалён (%v)", err)
	}

	// Слишком длинный путь: ошибка, а не паника (design §8, пределы sun_path)
	long := filepath.Join(filepath.Dir(path), strings.Repeat("x", 130), "s")
	if ln, err := net.Listen("unix", long); err != nil {
		res("path-too-long", "INFO", "ошибка Listen: %v", err)
	} else {
		_ = ln.Close()
		res("path-too-long", "INFO", "длинный путь принят (%d байт)", len(long))
	}
	if bad > 0 {
		return 1
	}
	return 0
}

// runClientCheck — С2 (б): подключиться к чужому сокету (--sock) и пройти initialize. Код 0 — удалось, 3 — отказ.
func runClientCheck(o opts) int {
	path := sockPath(o)
	cur, _ := user.Current()
	name := "?"
	if cur != nil {
		name = cur.Username
	}
	if st, err := os.Lstat(path); err != nil {
		res("clientcheck", "INFO", "user=%s Lstat: %v", name, err)
	} else {
		res("clientcheck", "INFO", "user=%s Lstat ok mode=%v", name, st.Mode())
	}
	line, err := rawRoundtrip(path)
	if err != nil {
		res("clientcheck", "DENIED", "user=%s %v", name, err)
		return 3
	}
	res("clientcheck", "CONNECTED", "user=%s %.120s", name, line)
	return 0
}
