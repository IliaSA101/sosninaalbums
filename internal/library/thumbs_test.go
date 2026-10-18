package library

import (
	"bytes"
	"crypto/sha1" //nolint:gosec // как в cacheKey
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // чтение
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return img
}

// PNG с прозрачностью даёт _t.png/_p.png с сохранённой альфой (JPEG сделал бы её чёрной),
// непрозрачный PNG и JPEG — по-прежнему JPEG.
func TestThumbAlphaPNG(t *testing.T) {
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, c.Close) // фоновая чистка кэша не должна держать папку
	alpha := testkit.PNG(t, dir, "logo.png", 60, 40, color.NRGBA{200, 30, 30, 255})
	for _, preview := range []bool{false, true} {
		f, err := c.File(Photo{Path: alpha, Name: "logo.png"}, preview)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(f, ".png") {
			t.Fatalf("preview=%v: файл %s, ждали .png", preview, f)
		}
		img := decodePNG(t, f)
		b := img.Bounds()
		if _, _, _, a := img.At(b.Dx()-1, b.Dy()/2).RGBA(); a != 0 {
			t.Errorf("preview=%v: прозрачный край стал непрозрачным (a=%d)", preview, a)
		}
		if r, _, _, a := img.At(1, b.Dy()/2).RGBA(); a>>8 != 255 || r>>8 < 190 {
			t.Errorf("preview=%v: закрашенная половина потеряла цвет (r=%d a=%d)", preview, r>>8, a>>8)
		}
		if again, _ := c.File(Photo{Path: alpha, Name: "logo.png"}, preview); again != f {
			t.Errorf("второй вызов вернул %s, ждали тот же файл %s", again, f)
		}
	}

	// PNG без единого прозрачного пикселя и обычный JPEG — JPEG.
	opaque := filepath.Join(dir, "full.png")
	img := image.NewNRGBA(image.Rect(0, 0, 20, 20))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	out, err := os.Create(opaque)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	jpg := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	for _, p := range []string{opaque, jpg} {
		f, err := c.File(Photo{Path: p, Name: filepath.Base(p)}, false)
		if err != nil || !strings.HasSuffix(f, ".jpg") {
			t.Errorf("%s: файл %q (%v), ждали .jpg", p, f, err)
		}
	}
}

// У .png в ключе кэша своя соль: прежние копии (чёрные JPEG) не подхватываются.
// У JPEG ключ прежний (см. TestCacheKeyColor).
func TestCacheKeyPNGSalt(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.PNG(t, dir, "a.png", 20, 10, color.NRGBA{1, 2, 3, 255})
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	old := sha1.Sum(fmt.Appendf(nil, "%s|%d|%d", p, st.Size(), st.ModTime().UnixNano())) //nolint:gosec // как в cacheKey
	got, err := cacheKey(p, imgfile.ColorNone)
	if err != nil || got == hex.EncodeToString(old[:10]) {
		t.Fatalf("ключ PNG совпал с прежним: %s (%v)", got, err)
	}
	if again, _ := cacheKey(p, imgfile.ColorNone); again != got {
		t.Error("ключ нестабилен")
	}
}

// Декод для миниатюр и экспорта не сплющивает альфу.
func TestOpenOrientedKeepsAlpha(t *testing.T) {
	p := testkit.PNG(t, testkit.TempDir(t), "a.png", 20, 10, color.NRGBA{200, 30, 30, 255})
	img, release, err := imgfile.OpenOriented(p)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, _, a := img.At(19, 5).RGBA(); a != 0 {
		t.Errorf("альфа потеряна: a=%d", a)
	}
	if img.(interface{ Opaque() bool }).Opaque() {
		t.Error("картинка стала непрозрачной")
	}
}

// Паника при прогреве одного фото (битый файл в декодере) гасится: воркер и процесс живы.
func TestWarmOnePanicRecovered(t *testing.T) {
	path := testkit.JPEG(t, testkit.TempDir(t), "bad.jpg", 20, 20)
	warmOne(nil, Photo{Path: path, Name: "bad.jpg"}) // nil-кэш → паника внутри File
}

// Запись о панике прогрева — с расширением, без имени файла (073-7).
func TestThumbPanicLogNoName(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	for _, name := range []string{"Иванов_секрет.JPG", "Иванов_без_расширения", "Петров." + strings.Repeat("я", 20)} {
		path := testkit.JPEG(t, testkit.TempDir(t), "x.jpg", 20, 20)
		warmOne(nil, Photo{Path: path, Name: name})
	}
	got := buf.String()
	if strings.Count(got, "ПАНИКА при прогреве") != 3 || !strings.Contains(got, `(файл ".jpg")`) ||
		!strings.Contains(got, `(файл "")`) || !strings.Contains(got, `(файл "прочее")`) {
		t.Errorf("запись: %.300q", got)
	}
	for _, leak := range []string{"Иванов", "Петров", "x.jpg"} {
		if strings.Contains(got, leak) {
			t.Errorf("в логе %q", leak)
		}
	}
}
