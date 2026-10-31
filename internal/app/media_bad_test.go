package app

// Усечённый оригинал: заголовок цел, фото в ленте, а миниатюры нет. Окно должно узнать причину
// («файл повреждён», тот же текст, что у unread при чтении), а не показать «нет файла».

import (
	"context"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// badApp — App с кэшем миниатюр и записью событий.
func badApp(t *testing.T) (*App, func(string) []evRec) {
	t.Helper()
	a, evs := evApp(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c // закрывает closeBackground из newApp
	return a, evs
}

// truncated пишет JPEG w×h, обрезанный до keep байт: заголовок с размерами цел, данных не хватает.
func truncated(t *testing.T, dir, name string, w, h, keep int) string {
	t.Helper()
	whole := testkit.JPEG(t, dir, "whole-"+name, w, h)
	data, err := os.ReadFile(whole)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(whole); err != nil {
		t.Fatal(err)
	}
	if keep >= len(data) {
		t.Fatalf("JPEG %d байт, обрезать до %d нечем", len(data), keep)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data[:keep], 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Несколько усечённых файлов и целый: у битых /media/t и /media/p — 404 без паники, причина «повреждён»
// в State.Unread и в событии unread; целый отдаётся и в unread не попадает. Повторный запрос причину
// не меняет и события не шлёт. Файл перезаписан целым — снова читается, причина уходит событием.
func TestMediaTruncatedUnread(t *testing.T) {
	a, evs := badApp(t)
	dir := testkit.TempDir(t)
	bad1 := truncated(t, dir, "trunc1.jpg", 400, 300, 700)
	bad2 := truncated(t, dir, "trunc2.jpg", 300, 200, 500)
	good := testkit.JPEG(t, dir, "good.jpg", 40, 30)
	if _, err := a.AddFilePaths([]string{bad1, bad2, good}); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)

	for _, kind := range []string{"t", "p"} {
		for _, p := range []string{bad1, bad2} {
			if rec := getMediaCtx(context.Background(), a, "/media/"+kind+"/"+library.IDFor(p)); rec.Code != http.StatusNotFound {
				t.Fatalf("/media/%s битого %s: %d, ждали 404", kind, filepath.Base(p), rec.Code)
			}
		}
	}
	if rec := getMediaCtx(context.Background(), a, "/media/t/"+library.IDFor(good)); rec.Code != http.StatusOK {
		t.Fatalf("/media/t целого: %d", rec.Code)
	}
	st := a.GetState()
	if len(st.Unread) != 2 || st.Unread[library.IDFor(good)] != "" {
		t.Fatalf("State.Unread: %v", st.Unread)
	}
	for _, p := range []string{bad1, bad2} {
		why := st.Unread[library.IDFor(p)]
		if !strings.HasPrefix(why, "файл повреждён: ") || !strings.Contains(why, "сохраните как JPEG") || strings.Contains(why, dir) {
			t.Errorf("%s: причина %q", filepath.Base(p), why)
		}
	}
	ev, ok := lastUnread(t, evs)
	if !ok || !maps.Equal(ev, st.Unread) {
		t.Fatalf("событие unread: %v (было: %v), State: %v", ev, ok, st.Unread)
	}

	// Повторные запросы к битому: тот же ответ, событий больше нет.
	n := len(evs("unread"))
	if rec := getMediaCtx(context.Background(), a, "/media/t/"+library.IDFor(bad1)); rec.Code != http.StatusNotFound {
		t.Fatalf("повторный /media/t: %d", rec.Code)
	}
	if got := len(evs("unread")); got != n {
		t.Errorf("повторный запрос прислал событие unread: %d → %d", n, got)
	}

	// Файл перезаписан целым (другие размер и время): миниатюра есть, причина снята событием.
	testkit.JPEG(t, dir, "trunc1.jpg", 50, 40)
	if rec := getMediaCtx(context.Background(), a, "/media/t/"+library.IDFor(bad1)); rec.Code != http.StatusOK {
		t.Fatalf("/media/t перезаписанного: %d", rec.Code)
	}
	st = a.GetState()
	if _, still := st.Unread[library.IDFor(bad1)]; still || len(st.Unread) != 1 {
		t.Fatalf("после перезаписи: %v", st.Unread)
	}
	if ev, _ := lastUnread(t, evs); !maps.Equal(ev, st.Unread) {
		t.Fatalf("событие после перезаписи: %v, State: %v", ev, st.Unread)
	}
}

// Пустые и крайние входы: файл обрезали после чтения — до нуля байт и посреди заголовка. Ошибка
// вместо паники, причина не длиннее предела и без пути файла.
func TestMediaTruncatedEdges(t *testing.T) {
	a, _ := badApp(t)
	dir := testkit.TempDir(t)
	empty := testkit.JPEG(t, dir, "empty.jpg", 400, 300)
	head := testkit.JPEG(t, dir, "head.jpg", 400, 300)
	if _, err := a.AddFilePaths([]string{empty, head}); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)
	if err := os.Truncate(empty, 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(head, 300); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{empty: "пустой файл", head: "файл повреждён"}
	for p := range want {
		if rec := getMediaCtx(context.Background(), a, "/media/p/"+library.IDFor(p)); rec.Code != http.StatusNotFound {
			t.Fatalf("/media/p %s: %d", filepath.Base(p), rec.Code)
		}
	}
	st := a.GetState()
	for p, prefix := range want {
		why := st.Unread[library.IDFor(p)]
		if !strings.HasPrefix(why, prefix) || len(why) > 700 || strings.Contains(why, dir) {
			t.Errorf("%s: причина %q, ждали начало %q", filepath.Base(p), why, prefix)
		}
	}
}

// Закрытые пределы карты: сверх badThumbMax карта начинается заново, а не растёт без границ.
func TestBadThumbMapBounded(t *testing.T) {
	a, _ := evApp(t)
	for i := range badThumbMax + 10 {
		a.noteBadThumb(library.IDFor("/x/"+string(rune('a'+i%26))+"/"+string(rune(0x400+i))), "файл повреждён")
	}
	a.mu.Lock()
	n := len(a.badThumb)
	a.mu.Unlock()
	if n == 0 || n > badThumbMax {
		t.Fatalf("записей %d, предел %d", n, badThumbMax)
	}
}
