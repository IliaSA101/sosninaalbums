package update

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// offerFor — предложение версии ver с установщиком data (по записи file).
func offerFor(ver string, file File) Offer {
	return Offer{Version: ver, Setup: &file}
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, e := range list {
		out = append(out, e.Name())
	}
	return out
}

func freeOf(n uint64) func(string) (uint64, error) {
	return func(string) (uint64, error) { return n, nil }
}

// Скачивание: файл под строгим именем, ход доходит до размера, .part не остаётся.
func TestDownloadOK(t *testing.T) {
	f := newFeed(t)
	data, file := setup("0.25.1", MinSetup)
	f.serveSetup("0.25.1", data)
	dir := filepath.Join(t.TempDir(), "Updates") // папки ещё нет — создаётся
	var last int64
	path, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), dir, freeOf(1<<40), func(d int64) { last = d })
	if err != nil || path != filepath.Join(dir, SetupName("0.25.1")) {
		t.Fatalf("Fetch: %q %v", path, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("содержимое: %v", err)
	}
	if last != file.Size {
		t.Errorf("ход закончился на %d из %d", last, file.Size)
	}
	if n := names(t, dir); len(n) != 1 {
		t.Errorf("в папке %v", n)
	}
	if err := Verify(path, file); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// Подмена байта и другая длина: отказ «не прошло проверку», ни файла, ни .part.
func TestDownloadVerify(t *testing.T) {
	data, file := setup("0.25.1", MinSetup)
	flipped := bytes.Clone(data)
	flipped[len(flipped)/2] ^= 1
	cases := map[string][]byte{
		"подмена байта": flipped,
		"короче":        data[:len(data)-1],
		"длиннее":       append(bytes.Clone(data), 0),
		"пусто":         {},
	}
	for name, body := range cases {
		f := newFeed(t)
		f.serveSetup("0.25.1", body)
		dir := t.TempDir()
		_, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), dir, freeOf(1<<40), nil)
		if !errors.Is(err, ErrVerify) {
			t.Errorf("%s: %v", name, err)
		}
		if n := names(t, dir); len(n) != 0 {
			t.Errorf("%s: в папке %v", name, n)
		}
	}
	// Ассета нет (404) — слова про файл, не про библиотеку.
	f := newFeed(t)
	if _, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), t.TempDir(), freeOf(1<<40), nil); !errors.Is(err, ErrNoFile) {
		t.Errorf("404: %v", err)
	}
}

// Скачанное и сверенное не качается заново; битое — заменяется.
func TestDownloadReuse(t *testing.T) {
	f := newFeed(t)
	data, file := setup("0.25.1", MinSetup)
	f.serveSetup("0.25.1", data)
	dir := t.TempDir()
	c := f.client()
	if _, err := Fetch(context.Background(), c, offerFor("0.25.1", file), dir, freeOf(1<<40), nil); err != nil {
		t.Fatal(err)
	}
	before := f.hits.Load()
	var last int64
	path, err := Fetch(context.Background(), c, offerFor("0.25.1", file), dir, freeOf(0), func(d int64) { last = d })
	if err != nil || f.hits.Load() != before || last != file.Size {
		t.Fatalf("повтор: %v, запросов +%d, ход %d", err, f.hits.Load()-before, last)
	}
	// Файл испорчен на диске — повторное скачивание, нужен запас места.
	if err := os.WriteFile(path, []byte("битый"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Fetch(context.Background(), c, offerFor("0.25.1", file), dir, freeOf(1<<40), nil); err != nil || f.hits.Load() != before+1 {
		t.Fatalf("после порчи: %v, запросов +%d", err, f.hits.Load()-before)
	}
	if err := Verify(path, file); err != nil {
		t.Errorf("после перекачки: %v", err)
	}
}

// Переход на чужой хост — ErrAddress, ничего не записано.
func TestDownloadHosts(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	for _, to := range []string{"https://evil.example/x.exe", "http://127.0.0.1/x.exe", "https://127.0.0.1/x.exe?sig=1"} {
		f.mux = http.NewServeMux()
		f.mux.HandleFunc("/releases/download/v0.25.1/"+SetupName("0.25.1"), func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, to, http.StatusFound)
		})
		dir := t.TempDir()
		_, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), dir, freeOf(1<<40), nil)
		if !errors.Is(err, ErrAddress) {
			t.Errorf("переход на %s: %v", to, err)
		}
		if n := names(t, dir); len(n) != 0 {
			t.Errorf("переход на %s: в папке %v", to, n)
		}
	}
}

// Мало места (меньше двойного размера): понятная ошибка с числом МБ, сеть не тронута.
func TestDownloadNoSpace(t *testing.T) {
	f := newFeed(t)
	data, file := setup("0.25.1", 8<<20+1)
	f.serveSetup("0.25.1", data)
	dir := t.TempDir()
	for _, free := range []uint64{0, 1, uint64(file.Size*2 - 1)} {
		_, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), dir, freeOf(free), nil)
		var ns *NoSpaceError
		if !errors.As(err, &ns) || ns.Need != 17 {
			t.Fatalf("free %d: %v", free, err)
		}
		if err.Error() != "мало места на диске для обновления (нужно 17 МБ)" {
			t.Errorf("текст: %q", err)
		}
	}
	if f.hits.Load() != 0 {
		t.Errorf("запросов %d при нехватке места", f.hits.Load())
	}
	// Ровно двойной размер — хватает.
	if _, err := Fetch(context.Background(), f.client(), offerFor("0.25.1", file), dir, freeOf(uint64(file.Size*2)), nil); err != nil {
		t.Errorf("впритык: %v", err)
	}
	// Место не определилось — не повод отказывать.
	g := newFeed(t)
	g.serveSetup("0.25.1", data)
	bad := func(string) (uint64, error) { return 0, os.ErrNotExist }
	if _, err := Fetch(context.Background(), g.client(), offerFor("0.25.1", file), t.TempDir(), bad, nil); err != nil {
		t.Errorf("ошибка места: %v", err)
	}
}

// Запись установщика не от этой версии и предложение без неё: скачивать нечего.
func TestDownloadNoInstaller(t *testing.T) {
	f := newFeed(t)
	_, file := setup("0.25.1", MinSetup)
	dir := t.TempDir()
	bad := file
	bad.Name = "evil.exe"
	for name, o := range map[string]Offer{
		"без установщика": {Version: "0.25.1", Page: true},
		"чужое имя":       offerFor("0.25.1", bad),
		"версия не x.y.z": offerFor("../1", file),
	} {
		if _, err := Fetch(context.Background(), f.client(), o, dir, freeOf(1<<40), nil); !errors.Is(err, ErrNoInstaller) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if f.hits.Load() != 0 {
		t.Error("запрос ушёл без установщика")
	}
}

// Отмена обрывает скачивание: ErrCancelled, файлов нет.
func TestDownloadCancel(t *testing.T) {
	f := newFeed(t)
	data, file := setup("0.25.1", MinSetup)
	f.serveSetup("0.25.1", data)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	if _, err := Fetch(ctx, f.client(), offerFor("0.25.1", file), dir, freeOf(1<<40), nil); !errors.Is(err, ErrCancelled) {
		t.Errorf("отмена: %v", err)
	}
	if n := names(t, dir); len(n) != 0 {
		t.Errorf("в папке %v", n)
	}
}

// Verify: размер, байты, не обычный файл — отказ.
func TestVerify(t *testing.T) {
	dir := t.TempDir()
	data, file := setup("0.25.1", 5000)
	p := filepath.Join(dir, "a.exe")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(p, file); err != nil {
		t.Fatalf("верный: %v", err)
	}
	if err := Verify(filepath.Join(dir, "нет.exe"), file); !errors.Is(err, ErrVerify) {
		t.Errorf("нет файла: %v", err)
	}
	if err := Verify(dir, file); !errors.Is(err, ErrVerify) {
		t.Errorf("папка: %v", err)
	}
	link := filepath.Join(dir, "link.exe")
	if err := os.Symlink(p, link); err == nil {
		if err := Verify(link, file); !errors.Is(err, ErrVerify) {
			t.Errorf("ссылка: %v", err)
		}
	}
	bad := file
	bad.Size++
	if err := Verify(p, bad); !errors.Is(err, ErrVerify) {
		t.Errorf("другой размер: %v", err)
	}
	bad = file
	bad.SHA256 = "0" + file.SHA256[1:]
	if err := Verify(p, bad); !errors.Is(err, ErrVerify) {
		t.Errorf("другой хеш: %v", err)
	}
}

// Clean: только свои устаревшие имена; чужое, новее текущей, папки и ссылки не трогает.
func TestCleanOld(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	gone := []string{SetupName("0.24.0"), SetupName("0.25.0"), SetupName("0.25.0") + ".part", SetupName("0.26.0") + ".part"}
	keep := []string{SetupName("0.25.1"), SetupName("1.0.0"), "SosninaAlbums-0.24.0-windows-x64.exe", "notes.txt",
		"SosninaAlbums-dev-windows-x64-setup.exe", "SosninaAlbums-0.1.0-windows-x64-setup.exe.bak", "x.part"}
	for _, n := range append(append([]string{}, gone...), keep...) {
		mk(n)
	}
	if err := os.Mkdir(filepath.Join(dir, SetupName("0.20.0")), 0o700); err != nil { // папка под своим именем
		t.Fatal(err)
	}
	keep = append(keep, SetupName("0.20.0"))
	Clean(dir, "0.25.0")
	for _, n := range gone {
		if _, err := os.Lstat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s не удалён", n)
		}
	}
	for _, n := range keep {
		if _, err := os.Lstat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s удалён: %v", n, err)
		}
	}
	// Своя версия не версия (dev): старые установщики остаются, недокачанное — уходит.
	mk(SetupName("0.1.0"))
	mk(SetupName("0.2.0") + ".part")
	Clean(dir, "dev")
	if _, err := os.Lstat(filepath.Join(dir, SetupName("0.1.0"))); err != nil {
		t.Error("при dev удалён установщик")
	}
	if _, err := os.Lstat(filepath.Join(dir, SetupName("0.2.0")+".part")); !os.IsNotExist(err) {
		t.Error("при dev остался .part")
	}
	Clean(filepath.Join(dir, "нет-такой"), "0.25.0") // нет папки — тихо
	if Dir("X") == "" || filepath.Base(Dir("X")) != "Updates" {
		t.Errorf("Dir: %q", Dir("X"))
	}
}
