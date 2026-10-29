package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
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
