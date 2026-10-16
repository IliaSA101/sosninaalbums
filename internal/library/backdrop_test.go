package library

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// BackdropFile: принятые JPEG/PNG дают Photo с ID по пути и размерами; прочее — отказ.
func TestBackdropFile(t *testing.T) {
	dir := testkit.TempDir(t)
	jpg := testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	png := testkit.PNG(t, dir, "ok.PNG", 20, 10, color.NRGBA{R: 200, A: 255})

	ph, err := BackdropFile(jpg)
	if err != nil || ph.ID != IDFor(jpg) || ph.Path != jpg || ph.W != 40 || ph.H != 30 || ph.Size == 0 {
		t.Errorf("jpeg: %+v, %v", ph, err)
	}
	ph, err = BackdropFile(png)
	if err != nil || ph.ID != IDFor(png) || ph.W != 20 || ph.H != 10 {
		t.Errorf("png (расширение в верхнем регистре): %+v, %v", ph, err)
	}

	gif := filepath.Join(dir, "a.gif")
	fake := filepath.Join(dir, "fake.jpg") // не картинка под видом JPEG
	if err := os.WriteFile(gif, []byte("GIF89a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fake, []byte("это не картинка"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dir.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	bad := map[string]string{
		"UNC":           `\\srv\share\a.jpg`,
		"относительный": "ok.jpg",
		"..":            dir + sep + ".." + sep + filepath.Base(dir) + sep + "ok.jpg",
		"gif":           gif,
		"не картинка":   fake,
		"папка":         filepath.Join(dir, "dir.jpg"),
		"нет файла":     filepath.Join(dir, "нет.jpg"),
		"огромная":      testkit.HugeJPEG(t, dir, "huge.jpg", 20000, 20000),
	}
	for name, path := range bad {
		if ph, err := BackdropFile(path); err == nil || ph != (Photo{}) {
			t.Errorf("%s: принят (%+v, %v)", name, ph, err)
		}
	}
	if _, err := BackdropFile(filepath.Join(dir, "нет.jpg")); err == nil || strings.Contains(err.Error(), "JPEG и PNG") {
		t.Errorf("нет файла: ждали ошибку ОС, а не про формат: %v", err)
	}
}

// PlainFile: расширение из списка в любом регистре, размер возвращается; чужое расширение — ErrExt,
// остальное — прежние отказы (UNC, «..», папка, нет файла).
func TestPlainFile(t *testing.T) {
	dir := testkit.TempDir(t)
	svg := filepath.Join(dir, "A.SVG")
	if err := os.WriteFile(svg, []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "d.svg"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, size, err := PlainFile(svg, ".png", ".svg")
	if err != nil || p != svg || size != 6 {
		t.Errorf("svg: %q %d %v", p, size, err)
	}
	if _, _, err := PlainFile(svg, ".png"); !errors.Is(err, ErrExt) {
		t.Errorf("чужое расширение: %v", err)
	}
	if _, _, err := PlainFile(svg); !errors.Is(err, ErrExt) {
		t.Errorf("пустой список расширений: %v", err)
	}
	sep := string(filepath.Separator)
	for name, path := range map[string]string{
		"UNC": `\\srv\share\a.svg`, "относительный": "A.SVG", "..": dir + sep + ".." + sep + filepath.Base(dir) + sep + "A.SVG",
		"папка": filepath.Join(dir, "d.svg"), "нет файла": filepath.Join(dir, "нет.svg"),
	} {
		if p, _, err := PlainFile(path, ".svg"); err == nil || errors.Is(err, ErrExt) || p != "" {
			t.Errorf("%s: %q %v", name, p, err)
		}
	}
}

// Ссылка читается по цели; ссылка на папку и висячая — отказ. Без прав на ссылки — пропуск.
func TestBackdropFileSymlink(t *testing.T) {
	dir := testkit.TempDir(t)
	ok := testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	link := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(ok, link); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	target, err := filepath.EvalSymlinks(ok)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := BackdropFile(link)
	if err != nil || ph.Path != target || ph.ID != IDFor(target) || ph.W != 40 {
		t.Errorf("ссылка на файл: %+v, %v", ph, err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	dlink := filepath.Join(dir, "dlink.jpg")
	if err := os.Symlink(sub, dlink); err != nil {
		t.Skipf("ссылка на папку: %v", err)
	}
	if _, err := BackdropFile(dlink); err == nil {
		t.Error("ссылка на папку принята")
	}
	dangling := filepath.Join(dir, "dangling.jpg")
	if err := os.Symlink(filepath.Join(dir, "нет.jpg"), dangling); err != nil {
		t.Skipf("висячая ссылка: %v", err)
	}
	if _, err := BackdropFile(dangling); err == nil {
		t.Error("висячая ссылка принята")
	}
}
