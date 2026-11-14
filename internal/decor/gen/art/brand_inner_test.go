package art

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// pngBytes — PNG w×h, залитый непрозрачным красным.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 255, 255
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// mustPanic проверяет, что f паникует с сообщением про вшитый файл.
func mustPanic(t *testing.T, name string, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("%s: паники нет", name)
		}
	}()
	f()
}

// TestBrandDefsBroken — порча вшитых файлов роняет сборку каталога сразу (а не пустым стикером):
// нет файла и не PNG (нулевой размер PNG отвергает уже стандартный разбор).
func TestBrandDefsBroken(t *testing.T) {
	mustPanic(t, "нет файла", func() {
		brandDefsFrom(func(string) ([]byte, error) { return nil, errors.New("нет") })
	})
	mustPanic(t, "не PNG", func() {
		brandDefsFrom(func(string) ([]byte, error) { return []byte("не картинка"), nil })
	})
}

// TestBrandDefsFrom — исправные данные дают все четыре стикера с пропорцией из заголовка;
// PNG с целым заголовком, но битыми данными рисуется пустотой, без паники и записи в холст.
func TestBrandDefsFrom(t *testing.T) {
	good := pngBytes(t, 40, 10)
	defs := brandDefsFrom(func(string) ([]byte, error) { return good, nil })
	if len(defs) != len(brandFiles) {
		t.Fatalf("стикеров %d, ждали %d", len(defs), len(brandFiles))
	}
	for i, d := range defs {
		if d.Kind != brandFiles[i].kind || !d.Image || d.Aspect != 4 || len(d.Inks) != 0 {
			t.Errorf("%d: %+v", i, d)
		}
	}
	draw := func(d Def) int {
		img := image.NewRGBA(image.Rect(0, 0, 40, 10))
		c := &paint.Canvas{Dst: img, Clip: img.Rect, M: f64.Aff3{10, 0, 0, 0, 10, 0}, W: 4, H: 1, Alpha: 1}
		d.Draw(c, Spec{}, nil, d.Aspect)
		n := 0
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] > 0 {
				n++
			}
		}
		return n
	}
	if n := draw(defs[0]); n != 400 {
		t.Errorf("исправный PNG закрасил %d px из 400", n)
	}
	// обрезанный после IHDR: DecodeConfig проходит, Decode — нет
	cut := good[:33]
	bad := brandDefsFrom(func(string) ([]byte, error) { return cut, nil })
	if bad[0].Aspect != 4 {
		t.Fatalf("пропорция %v", bad[0].Aspect)
	}
	if n := draw(bad[0]); n != 0 {
		t.Errorf("битый PNG закрасил %d px", n)
	}
}

// TestSmoothDegenerate — меньше трёх точек кривой не строят: путь пуст, холст чист.
func TestSmoothDegenerate(t *testing.T) {
	for _, ps := range [][]pt{nil, {{1, 1}}, {{1, 1}, {5, 5}}} {
		img := image.NewRGBA(image.Rect(0, 0, 10, 10))
		c := &paint.Canvas{Dst: img, Clip: img.Rect, M: f64.Aff3{1, 0, 0, 0, 1, 0}, W: 10, H: 10, Alpha: 1}
		smooth(c, ps)
		c.Fill(color.NRGBA{255, 0, 0, 255}, 1)
		for _, v := range img.Pix {
			if v != 0 {
				t.Fatalf("%d точек: холст не чист", len(ps))
			}
		}
	}
}
