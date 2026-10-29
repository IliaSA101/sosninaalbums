// Команда albums-fake — поддельный сервер листалок для стенда QA: контракт /v1 и страницы клиента
// на loopback, файлы страниц — в <dir>/albums-fake/. Только для разработки: токен ненастоящий,
// попыток входа сервер не считает. Запуск:
//
//	albums-fake -addr 127.0.0.1:34190 -dir /abs/путь/папки -token <строка 16…512 знаков>
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

const defaultAddr = "127.0.0.1:34190"

type config struct{ addr, dir, token string }

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "albums-fake:", err)
		os.Exit(2)
	}
}

// parseArgs разбирает и проверяет аргументы: -dir — абсолютная существующая папка, -token годен,
// -addr — loopback с портом. Пустое, «.» и относительное -dir — отказ (иначе страницы легли бы в текущую папку).
func parseArgs(args []string, stderr io.Writer) (config, error) {
	var c config
	fs := flag.NewFlagSet("albums-fake", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&c.addr, "addr", defaultAddr, "адрес: 127.0.0.1, ::1 или localhost с портом")
	fs.StringVar(&c.dir, "dir", "", "абсолютный путь существующей папки для файлов (обязательно)")
	fs.StringVar(&c.token, "token", "", "токен доступа, 16…512 знаков (обязательно)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, errors.New("лишние аргументы")
	}
	if err := checkDir(c.dir); err != nil {
		return c, err
	}
	if err := albums.CheckToken(c.token); err != nil {
		return c, err
	}
	return c, checkAddr(c.addr)
}

func checkDir(dir string) error {
	if dir == "" || !filepath.IsAbs(dir) {
		return errors.New("-dir: нужен абсолютный путь существующей папки")
	}
	// корень тома (/, C:\): сервер писал бы в <корень>/albums-fake — слишком широкий доступ для стенда
	if clean := filepath.Clean(dir); filepath.Dir(clean) == clean {
		return errors.New("-dir: корень диска не годится, нужна своя папка")
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return errors.New("-dir: папка не существует")
	}
	return nil
}

// checkAddr: хост ровно 127.0.0.1, ::1 или localhost, порт 1…65535. Пустой хост (":34190") и
// 0.0.0.0 открыли бы сервер в сеть — отказ.
func checkAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("-addr: нужен вид хост:порт")
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return errors.New("-addr: порт вне 1…65535")
	}
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return nil
	}
	return errors.New("-addr: хост только 127.0.0.1, ::1 или localhost")
}

// run запускает сервер и работает, пока не отменён ctx. ready (если задан) получает адрес ссылок.
func run(ctx context.Context, args []string, stdout, stderr io.Writer, ready func(url string)) error {
	c, err := parseArgs(args, stderr)
	if err != nil {
		return err
	}
	f, err := albumstest.NewFake(albumstest.Options{Token: c.token, Dir: c.dir})
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", c.addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	// Проверка фактического адреса: localhost мог разрешиться во что угодно.
	ap, err := netip.ParseAddrPort(ln.Addr().String())
	if err != nil || !ap.Addr().IsLoopback() {
		return errors.New("сервер не на loopback: отказ")
	}
	url := "http://" + ln.Addr().String()
	f.SetURL(url)
	srv := &http.Server{
		Handler:           f,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    16 << 10,
	}
	_, _ = fmt.Fprintf(stdout, "albums-fake: %s, файлы — %s\n", url, f.RootDir())
	if ready != nil {
		ready(url)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(sctx)
	}
}
