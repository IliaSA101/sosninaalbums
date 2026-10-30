package imgfile

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Паника декодера после взятия бюджета: место возвращается, паника идёт дальше.
func TestOpenOrientedPanicReleasesBudget(t *testing.T) {
	p := testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 20, 20)
	orig := decodeImage
	decodeImage = func(io.Reader, ...imaging.DecodeOption) (image.Image, error) { panic("сбой декодера") }
	t.Cleanup(func() { decodeImage = orig })
	before := DecodeBudget.Used()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("паника должна дойти до вызывающего")
			}
		}()
		_, _, _ = OpenOriented(p)
	}()
	if got := DecodeBudget.Used(); got != before {
		t.Fatalf("бюджет не возвращён после паники: %d вместо %d", got, before)
	}
}

// Ошибка декода тоже возвращает бюджет; не-картинка отказывает до декода.
func TestOpenOrientedErrorReleasesBudget(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEG(t, dir, "a.jpg", 20, 20)
	orig := decodeImage
	decodeImage = func(io.Reader, ...imaging.DecodeOption) (image.Image, error) { return nil, errors.New("битый") }
	t.Cleanup(func() { decodeImage = orig })
	before := DecodeBudget.Used()
	if _, _, err := OpenOriented(p); err == nil {
		t.Fatal("ждали ошибку декода")
	}
	bad := filepath.Join(dir, "x.jpg")
	if err := os.WriteFile(bad, []byte("не картинка"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenOriented(bad); err == nil {
		t.Fatal("ждали отказ на не-картинке")
	}
	if got := DecodeBudget.Used(); got != before {
		t.Fatalf("бюджет не возвращён: %d вместо %d", got, before)
	}
}

// Декод идёт из того же дескриптора, что и проверка: обычное открытие работает.
func TestOpenOrientedSameDescriptor(t *testing.T) {
	p := testkit.PNG(t, testkit.TempDir(t), "a.png", 20, 10, color.NRGBA{R: 9, A: 255})
	img, release, err := OpenOriented(p)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if b := img.Bounds(); b.Dx() != 20 || b.Dy() != 10 {
		t.Fatalf("размер %v", b)
	}
}

// Кроме JPEG и PNG ничего не декодируем: у GIF/BMP/TIFF в x/image были уязвимости, а в альбомах их нет.
func TestCheckSizeOnlyJPEGAndPNG(t *testing.T) {
	dir := testkit.TempDir(t)
	if err := CheckSize(testkit.JPEG(t, dir, "a.jpg", 20, 20)); err != nil {
		t.Fatalf("jpeg: %v", err)
	}
	if err := CheckSize(testkit.PNG(t, dir, "a.png", 20, 20, color.NRGBA{A: 255})); err != nil {
		t.Fatalf("png: %v", err)
	}
	g := filepath.Join(dir, "a.jpg.gif") // расширение не важно: формат определяется по содержимому
	f, err := os.Create(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.Encode(f, image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckSize(g); err == nil {
		t.Fatal("gif принят")
	}
}

// Не обычный файл (каталог, устройство) отклоняется по дескриптору открытого файла.
func TestOpenRegularRejectsDir(t *testing.T) {
	dir := testkit.TempDir(t)
	if f, err := OpenRegular(dir); err == nil {
		_ = f.Close()
		t.Error("каталог должен отклоняться")
	}
	if err := CheckSize(dir); err == nil {
		t.Error("CheckSize: каталог должен отклоняться")
	}
	f, err := OpenRegular(testkit.JPEG(t, dir, "a.jpg", 20, 20))
	if err != nil {
		t.Fatalf("обычный файл: %v", err)
	}
	_ = f.Close()
}

// writePNG пишет img в PNG (16-битные модели кодируются 16 битами: непрозрачная — RGBA64,
// с прозрачностью — NRGBA64).
func writePNG(t *testing.T, dir, name string, img image.Image) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

// Вес декода в бюджете: 16-битный PNG (8 байт на пиксель) и JPEG с EXIF-поворотом (оригинал и
// повёрнутая копия) — вдвое; Adobe RGB с поворотом — тоже вдвое (пересчёт по копии на месте);
// Gray16 (2 байта) и обычные — W×H. OpenOriented держит в бюджете ровно этот вес.
func TestCheckFileDecodeWeight(t *testing.T) {
	dir := testkit.TempDir(t)
	const w, h = 10, 6
	r := image.Rect(0, 0, w, h)
	opaque := image.NewRGBA64(r)
	for y := range h {
		for x := range w {
			opaque.SetRGBA64(x, y, color.RGBA64{R: 0x1234, A: 0xFFFF})
		}
	}
	cases := []struct {
		name  string
		path  string
		model color.Model // nil — не сверяем
		want  int64
	}{
		{"png 8 бит", testkit.PNG(t, dir, "8.png", w, h, color.NRGBA{R: 9, A: 255}), color.NRGBAModel, w * h},
		{"png RGBA64", writePNG(t, dir, "rgba64.png", opaque), color.RGBA64Model, 2 * w * h},
		{"png NRGBA64", writePNG(t, dir, "nrgba64.png", image.NewNRGBA64(r)), color.NRGBA64Model, 2 * w * h},
		{"png Gray16", writePNG(t, dir, "g16.png", image.NewGray16(r)), color.Gray16Model, w * h},
		{"jpeg без поворота", testkit.JPEG(t, dir, "1.jpg", w, h), nil, w * h},
		{"jpeg поворот 2", testkit.JPEGOriented(t, dir, "2.jpg", w, h, 2), nil, 2 * w * h},
		{"jpeg поворот 6", testkit.JPEGOriented(t, dir, "6.jpg", w, h, 6), nil, 2 * w * h},
		{"jpeg Adobe", testkit.JPEGSegments(t, dir, "a.jpg", w, h, testkit.ExifColor(true, 0, 0xFFFF, "R03")), nil, 2 * w * h},
		{"jpeg Adobe и поворот 6", testkit.JPEGSegments(t, dir, "a6.jpg", w, h, testkit.ExifColor(true, 6, 0xFFFF, "R03")), nil, 2 * w * h},
	}
	for _, c := range cases {
		if c.model != nil {
			if d, err := FileDims(c.path, 0); err != nil || d.Color != c.model {
				t.Fatalf("%s: файл не той модели: %v %v", c.name, d.Color, err)
			}
		}
		n, _, err := checkFile(c.path)
		if err != nil || n != c.want {
			t.Errorf("%s: вес %d, ждали %d (%v)", c.name, n, c.want, err)
			continue
		}
		before := DecodeBudget.Used()
		img, release, err := OpenOriented(c.path)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := DecodeBudget.Used() - before; got != c.want {
			t.Errorf("%s: в бюджете %d, ждали %d", c.name, got, c.want)
		}
		release()
		if b := img.Bounds(); b.Dx()*b.Dy() != w*h {
			t.Errorf("%s: размер %v", c.name, b)
		}
		if got := DecodeBudget.Used(); got != before {
			t.Errorf("%s: бюджет не возвращён: %d вместо %d", c.name, got, before)
		}
	}
}
