package imgfile

import (
	"errors"
	"image"
	"image/color"
	"image/gif"
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
