package library

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func scanSrc(t *testing.T, s *Scanner, src Sources) []Photo {
	t.Helper()
	photos, _, err := s.Scan(context.Background(), src, func([]Photo, ScanProgress, bool) {})
	if err != nil {
		t.Fatal(err)
	}
	return photos
}

// Отдельные файлы читаются как фото папок: несколько сразу, повтор с папкой не дублируется,
// не-фото, папка, пропавший и нелокальный путь молча пропускаются.
func TestScanSourcesFiles(t *testing.T) {
	root := testkit.TempDir(t)
	folder, other := filepath.Join(root, "Папка"), filepath.Join(root, "Другая")
	for _, d := range []string{folder, other, filepath.Join(root, "dir.jpg")} { // «dir.jpg» — папка с именем фото
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	testkit.JPEG(t, folder, "in.jpg", 40, 30)
	testkit.JPEG(t, other, "b.jpg", 40, 30)
	testkit.JPEG(t, other, "a.JPG", 40, 30)
	if err := os.WriteFile(filepath.Join(other, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := Sources{Folders: []string{folder}, Files: []string{
		filepath.Join(other, "b.jpg"), filepath.Join(other, "a.JPG"), // порядок выбора не важен: лента — по пути
		filepath.Join(folder, "in.jpg"), // уже есть через папку
		filepath.Join(other, "note.txt"),
		filepath.Join(root, "dir.jpg"),
		filepath.Join(other, "нет.jpg"),
		`\\srv\share\x.jpg`,
	}}
	photos := scanSrc(t, NewScanner(), src)
	var got []string
	for _, p := range photos {
		got = append(got, p.Name)
	}
	want := []string{"a.JPG", "b.jpg", "in.jpg"} // Другая < Папка
	if len(got) != len(want) {
		t.Fatalf("фото %v, ждали %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("фото %v, ждали %v", got, want)
		}
	}
	if only := scanSrc(t, NewScanner(), Sources{}); len(only) != 0 {
		t.Errorf("пустые источники: %d фото", len(only))
	}
}

// Сайдкар отдельного файла: `имя.расш.xmp` главнее `имя.xmp`; чужой сайдкар в той же папке не берётся.
func TestScanLooseSidecar(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		testkit.JPEG(t, dir, n, 40, 30)
	}
	writeSide(t, dir, "a.xmp", `xmp:Rating="1"`)
	writeSide(t, dir, "b.jpg.xmp", `xmp:Rating="2"`)
	writeSide(t, dir, "b.xmp", `xmp:Rating="4"`)
	writeSide(t, dir, "z.xmp", `xmp:Rating="5"`)
	photos := scanSrc(t, NewScanner(), Sources{Files: []string{
		filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg"), filepath.Join(dir, "c.jpg")}})
	got := marksByName(photos)
	wantMarks(t, got, "a.jpg", &Marks{Rating: 1})
	wantMarks(t, got, "b.jpg", &Marks{Rating: 2})
	wantMarks(t, got, "c.jpg", nil)
}

// Нелокальный путь отклоняется до обращения к диску: на UNC-пути Lstat сходил бы в сеть.
func TestScanLooseUNCNoStat(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UNC — путь Windows")
	}
	photos := scanSrc(t, NewScanner(), Sources{Files: []string{`\\127.0.0.1\nope\x.jpg`, `\\?\C:\x.jpg`}})
	if len(photos) != 0 {
		t.Errorf("нелокальные пути дали %d фото", len(photos))
	}
}

// Отпечаток: список отдельных файлов, их правка и правка сайдкара меняют Changed; без изменений — нет.
func TestFingerprintFiles(t *testing.T) {
	dir := testkit.TempDir(t)
	a, b := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg")
	testkit.JPEG(t, dir, "a.jpg", 40, 30)
	testkit.JPEG(t, dir, "b.jpg", 40, 30)
	src := Sources{Files: []string{a, b}}
	s := NewScanner()
	scanSrc(t, s, src)
	ch := func(src Sources) bool {
		t.Helper()
		c, _, err := s.Changed(context.Background(), src)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if ch(src) {
		t.Fatal("без изменений, а changed")
	}
	if !ch(Sources{Files: []string{a}}) {
		t.Error("список файлов короче — не changed")
	}
	if !ch(Sources{Folders: []string{a}, Files: []string{b}}) {
		t.Error("файл a как папка — не changed")
	}
	touch(t, b, 2*60*60*1e9)
	if !ch(src) {
		t.Error("правка файла не замечена")
	}
	scanSrc(t, s, src)
	writeSide(t, dir, "a.xmp", `xmp:Rating="3"`)
	if !ch(src) {
		t.Error("новый сайдкар не замечен")
	}
	scanSrc(t, s, src)
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	if !ch(src) {
		t.Error("пропажа файла не замечена")
	}
}
