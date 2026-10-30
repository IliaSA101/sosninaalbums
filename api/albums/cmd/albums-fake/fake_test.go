package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

const tok = "albums-fake-token-0123456789"

// TestFakeArgs (разбор С6): негодные аргументы — выход с ошибкой, а не запуск.
func TestFakeArgs(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	good := []string{"-dir", dir, "-token", tok}
	root := filepath.Dir(dir)
	for filepath.Dir(root) != root {
		root = filepath.Dir(root)
	}
	for name, args := range map[string][]string{
		"-dir пуст":                      {"-dir", "", "-token", tok},
		"-dir не задан":                  {"-token", tok},
		"-dir точка":                     {"-dir", ".", "-token", tok},
		"-dir относительный":             {"-dir", "rel/dir", "-token", tok},
		"-dir две точки":                 {"-dir", "..", "-token", tok},
		"-dir нет":                       {"-dir", filepath.Join(dir, "нет"), "-token", tok},
		"-dir корень тома":               {"-dir", root, "-token", tok},
		"-dir корень со слэшами":         {"-dir", root + string(filepath.Separator) + ".", "-token", tok},
		"-dir файл":                      {"-dir", file, "-token", tok},
		"-token не задан":                {"-dir", dir},
		"-token короткий":                {"-dir", dir, "-token", "short"},
		"-token с пробелом":              {"-dir", dir, "-token", "abc def ghi jkl mno pqr"},
		"-addr без хоста":                append([]string{"-addr", ":34190"}, good...),
		"-addr 0.0.0.0":                  append([]string{"-addr", "0.0.0.0:34190"}, good...),
		"-addr [::]":                     append([]string{"-addr", "[::]:34190"}, good...),
		"-addr чужой IP":                 append([]string{"-addr", "192.168.1.5:34190"}, good...),
		"-addr имя":                      append([]string{"-addr", "example.org:34190"}, good...),
		"-addr без порта":                append([]string{"-addr", "127.0.0.1"}, good...),
		"-addr порт 0":                   append([]string{"-addr", "127.0.0.1:0"}, good...),
		"-addr порт 70000":               append([]string{"-addr", "127.0.0.1:70000"}, good...),
		"-addr localhost.evil":           append([]string{"-addr", "localhost.evil.org:34190"}, good...),
		"лишний аргумент":                append([]string{"extra"}, good...),
		"неизвестный флаг":               append([]string{"-nope"}, good...),
		"-addr 127.0.0.2 (не из списка)": append([]string{"-addr", "127.0.0.2:34190"}, good...),
	} {
		if _, err := parseArgs(args, io.Discard); err == nil {
			t.Errorf("%s: принято", name)
		}
	}
	for _, addr := range []string{"127.0.0.1:34190", "[::1]:34190", "localhost:34190"} {
		c, err := parseArgs(append([]string{"-addr", addr}, good...), io.Discard)
		if err != nil || c.addr != addr || c.dir != dir || c.token != tok {
			t.Errorf("%s: %+v %v", addr, c, err)
		}
	}
	for _, v := range []string{"-1s", "21s", "1h", "abc", "2", "1e3s"} {
		if _, err := parseArgs(append([]string{"-slow", v}, good...), io.Discard); err == nil {
			t.Errorf("-slow %s: принято", v)
		}
	}
	for v, want := range map[string]time.Duration{"0": 0, "0s": 0, "1ms": time.Millisecond, "2s": 2 * time.Second, "20s": maxSlow} {
		c, err := parseArgs(append([]string{"-slow", v}, good...), io.Discard)
		if err != nil || c.slow != want {
			t.Errorf("-slow %s: %v %v", v, c.slow, err)
		}
	}
	// run с негодными аргументами не открывает порт и не пишет в папку
	if err := run(context.Background(), []string{"-dir", ".", "-token", tok}, io.Discard, io.Discard, nil); err == nil {
		t.Error("run с -dir . запустился")
	}
	if left, _ := os.ReadDir(dir); len(left) != 1 {
		t.Errorf("после отказа в папке появилось лишнее: %v", left)
	}
}

// TestFakeRun: сервер запускается на loopback, проходит набор проверок и останавливается по отмене.
func TestFakeRun(t *testing.T) {
	if testing.Short() {
		t.Skip("полный набор проверок: не в быстром уровне")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urlc := make(chan string, 1)
	errc := make(chan error, 1)
	var out bytes.Buffer
	go func() {
		errc <- run(ctx, []string{"-addr", "127.0.0.1:0", "-dir", dir, "-token", tok}, &out, io.Discard, func(u string) { urlc <- u })
	}()
	// порт 0 запрещён проверкой аргументов: берём свободный порт заранее
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("порт 0 принят")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run завис")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	go func() {
		errc <- run(ctx, []string{"-addr", addr, "-dir", dir, "-token", tok}, &out, io.Discard, func(u string) { urlc <- u })
	}()
	var url string
	select {
	case url = <-urlc:
	case err := <-errc:
		t.Fatalf("run: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("сервер не запустился")
	}
	if url != "http://"+addr {
		t.Errorf("адрес ссылок %q", url)
	}
	albumstest.Conformance(t, url, tok)
	cancel()
	select {
	case err := <-errc:
		if err != nil && !strings.Contains(err.Error(), "closed") {
			t.Errorf("остановка: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("сервер не остановился")
	}
	if strings.Contains(out.String(), tok) {
		t.Error("токен попал в вывод")
	}
	if _, err := os.Stat(filepath.Join(dir, "albums-fake")); err != nil {
		t.Errorf("нет своей подпапки: %v", err)
	}
}

// slowScale — множитель порога времени SA_TEST_SLOW (testkit модулю недоступен, как в client_review_test.go).
func slowScale(d time.Duration) time.Duration {
	if v, err := strconv.ParseFloat(os.Getenv("SA_TEST_SLOW"), 64); err == nil && v >= 1 {
		return time.Duration(float64(d) * v)
	}
	return d
}

// TestSlowHandler: медленный режим держит ответ на страницу и publish, прочие запросы не трогает;
// отмена запроса и остановка сервера прерывают задержку быстро, а не через весь срок.
func TestSlowHandler(t *testing.T) {
	var calls atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("X-Test", "1")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, "ok")
	})
	serve := func(ctx context.Context, h http.Handler, method, path string) (*httptest.ResponseRecorder, time.Duration) {
		r := httptest.NewRequest(method, path, nil).WithContext(ctx)
		w := httptest.NewRecorder()
		t0 := time.Now()
		h.ServeHTTP(w, r)
		return w, time.Since(t0)
	}
	const d = 400 * time.Millisecond
	h := slowHandler(context.Background(), next, d)
	for _, c := range []struct{ method, path string }{
		{"PUT", "/v1/albums/abc/pages/3"},
		{"POST", "/v1/albums/abc/publish"},
	} {
		w, el := serve(context.Background(), h, c.method, c.path)
		if el < d*9/10 || w.Code != http.StatusCreated || w.Body.String() != "ok" || w.Header().Get("X-Test") != "1" {
			t.Errorf("%s %s: %v, %d, %q", c.method, c.path, el, w.Code, w.Body)
		}
	}
	// остальное — без задержки: проверка, список, создание, правка, страницы клиента
	for _, c := range []struct{ method, path string }{
		{"GET", "/v1/health"}, {"GET", "/v1/albums"}, {"POST", "/v1/albums"}, {"PUT", "/v1/albums/abc"},
		{"PATCH", "/v1/albums/abc"}, {"DELETE", "/v1/albums/abc"}, {"GET", "/a/abc/"}, {"POST", "/a/abc/publish"},
	} {
		if _, el := serve(context.Background(), h, c.method, c.path); el > d/2 {
			t.Errorf("%s %s задержан: %v", c.method, c.path, el)
		}
	}
	// отмена запроса клиентом прерывает задержку: 20 с, а возврат — быстро и без ответа
	long := slowHandler(context.Background(), next, maxSlow)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if w, el := serve(ctx, long, "PUT", "/v1/albums/abc/pages/1"); el > slowScale(2*time.Second) || w.Body.Len() != 0 {
		t.Errorf("отмена запроса: %v, тело %q", el, w.Body)
	}
	// остановка сервера — то же
	stop, halt := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, halt)
	if _, el := serve(context.Background(), slowHandler(stop, next, maxSlow), "POST", "/v1/albums/abc/publish"); el > slowScale(2*time.Second) {
		t.Errorf("остановка сервера: %v", el)
	}
	if calls.Load() == 0 {
		t.Error("следующий обработчик не вызван")
	}
}

// TestFakeSlowRun: с -slow настоящий сервер отвечает на страницу не раньше срока, отмена клиента быстрая.
func TestFakeSlowRun(t *testing.T) {
	dir := t.TempDir()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	urlc := make(chan string, 1)
	errc := make(chan error, 1)
	go func() {
		errc <- run(ctx, []string{"-addr", addr, "-dir", dir, "-token", tok, "-slow", "20s"}, io.Discard, io.Discard, func(u string) { urlc <- u })
	}()
	var url string
	select {
	case url = <-urlc:
	case err := <-errc:
		t.Fatalf("run: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("сервер не запустился")
	}
	// health не медленный
	t0 := time.Now()
	res, err := http.Get(url + "/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if el := time.Since(t0); el > slowScale(2*time.Second) {
		t.Errorf("health задержан: %v", el)
	}
	// страница: клиент отменяет через 200 мс — запрос возвращается сразу, сервер цел
	rctx, rcancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer rcancel()
	req, _ := http.NewRequestWithContext(rctx, http.MethodPut, url+"/v1/albums/abc/pages/1", strings.NewReader("x"))
	req.Header.Set("Authorization", "Bearer "+tok)
	t0 = time.Now()
	if res, err := http.DefaultClient.Do(req); err == nil {
		_ = res.Body.Close()
		t.Error("ответ пришёл без задержки")
	}
	if el := time.Since(t0); el > slowScale(2*time.Second) {
		t.Errorf("отмена клиента: %v", el)
	}
	// остановка с запросом в задержке: Shutdown не ждёт весь срок
	go func() {
		req, _ := http.NewRequest(http.MethodPut, url+"/v1/albums/abc/pages/2", strings.NewReader("x"))
		req.Header.Set("Authorization", "Bearer "+tok)
		if res, err := http.DefaultClient.Do(req); err == nil {
			_ = res.Body.Close()
		}
	}()
	time.Sleep(200 * time.Millisecond)
	t0 = time.Now()
	cancel()
	select {
	case <-errc:
	case <-time.After(slowScale(8 * time.Second)):
		t.Fatal("сервер не остановился")
	}
	if el := time.Since(t0); el > slowScale(3*time.Second) {
		t.Errorf("остановка: %v", el)
	}
}
