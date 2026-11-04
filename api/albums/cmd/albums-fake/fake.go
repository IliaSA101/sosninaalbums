// Команда albums-fake — поддельный сервер листалок для стенда QA: контракт /v1 и страницы клиента
// на loopback, файлы страниц — в <dir>/albums-fake/. Только для разработки: токен ненастоящий,
// попыток входа сервер не считает. Запуск:
//
//	albums-fake -addr 127.0.0.1:34190 -dir /abs/путь/папки -token <строка 16…512 знаков> [-slow 2s]
//
// -slow — медленный режим для QA «Остановить/Продолжить»: ответ на загрузку каждой страницы и на
// publish приходит через указанный срок (страница при этом уже принята), срок прерывает отмена запроса.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

const defaultAddr = "127.0.0.1:34190"

// maxSlow — предел -slow: клиент ждёт заголовки ответа 30 с (api/albums transport.go), дольше он
// оборвал бы запрос сам и пошёл на повтор, а не ждал «Остановить».
const maxSlow = 20 * time.Second

type config struct {
	addr, dir, token string
	slow             time.Duration // задержка ответа на страницу и publish; 0 — без задержки
}

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
	fs.DurationVar(&c.slow, "slow", 0, "медленный режим: задержка ответа на страницу и publish, 0…20s (0 — выкл.)")
	if err := fs.Parse(args); err != nil {
		return c, err
	}
	if fs.NArg() != 0 {
		return c, errors.New("лишние аргументы")
	}
	if c.slow < 0 || c.slow > maxSlow {
		return c, errors.New("-slow: нужно от 0 до " + maxSlow.String())
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
	var h http.Handler = f
	if c.slow > 0 {
		h = slowHandler(ctx, f, c.slow)
	}
	srv := &http.Server{
		Handler:           h,
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

// slowHandler задерживает ответ на загрузку страницы (PUT .../pages/N) и publish на d. Запрос
// обрабатывается сразу (страница принята), держится только ответ — как при медленной сети. Задержку
// прерывает отмена запроса клиентом и stop (остановка сервера): ответа тогда нет, соединение закроется.
func slowHandler(stop context.Context, next http.Handler, d time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slowRoute(r) {
			next.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-t.C:
		case <-r.Context().Done():
			return
		case <-stop.Done():
			return
		}
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	})
}

func slowRoute(r *http.Request) bool {
	p := r.URL.Path
	switch r.Method {
	case http.MethodPut:
		return strings.HasPrefix(p, "/v1/albums/") && strings.Contains(p, "/pages/")
	case http.MethodPost:
		return strings.HasPrefix(p, "/v1/albums/") && strings.HasSuffix(p, "/publish")
	}
	return false
}
