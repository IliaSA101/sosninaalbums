package library

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Аудит 117: разметка WebP под .jpg — Big (файлом не отдаётся), превью перекодирует Go даже для
// малого размера; JPEG/PNG малого размера остаются файлом, формат берётся из содержимого.
func TestMarkupWebPIsBig(t *testing.T) {
	dir := testkit.TempDir(t)
	cases := []struct {
		file, sample, format string
		big                  bool
		ct                   string
	}{
		{"lossy.jpg", "lossy16.webp", "webp", true, "image/jpeg"},
		{"lossless.png", "lossless24x16.webp", "webp", true, "image/jpeg"},
		{"alpha.jpeg", "mem_alpha.webp", "webp", true, "image/png"},
	}
	for _, c := range cases {
		p := writeFile(t, dir, c.file, webpSample(t, c.sample))
		m, err := MarkupFile(p)
		if err != nil || m.Format != c.format || m.Big() != c.big {
			t.Fatalf("%s: %+v, %v", c.file, m, err)
		}
		data, ct, err := MarkupPreview(p)
		if err != nil || ct != c.ct || len(data) == 0 || bytes.HasPrefix(data, []byte("RIFF")) {
			t.Fatalf("%s: превью %q, %d байт, %v", c.file, ct, len(data), err)
		}
		if ct == "image/png" {
			_, err = png.Decode(bytes.NewReader(data))
		} else {
			_, err = jpeg.Decode(bytes.NewReader(data))
		}
		if err != nil {
			t.Fatalf("%s: превью не %s: %v", c.file, ct, err)
		}
	}
	// Обычные малые JPEG и PNG: формат по содержимому, Big нет, превью нет.
	for file, format := range map[string]string{
		testkit.JPEG(t, dir, "s.jpg", 40, 30):                           "jpeg",
		testkit.PNG(t, dir, "s.png", 40, 30, color.NRGBA{R: 1, A: 255}): "png",
	} {
		m, err := MarkupFile(file)
		if err != nil || m.Format != format || m.Big() {
			t.Fatalf("%s: %+v, %v", filepath.Base(file), m, err)
		}
		if data, _, err := MarkupPreview(file); err != nil || data != nil {
			t.Fatalf("%s: превью малого файла: %d байт, %v", filepath.Base(file), len(data), err)
		}
	}
	if u := imgfile.DecodeBudget.Used(); u != 0 {
		t.Fatalf("бюджет не возвращён: %d", u)
	}
}

// Аудит 117 (к сведению): прозрачный WebP под .jpg даёт PNG-миниатюру и превью (JPEG сделал бы
// прозрачные места чёрными), непрозрачный — JPEG. Расширение оригинала на выбор копии не влияет.
func TestThumbAlphaWebPUnderJPG(t *testing.T) {
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, c.Close)
	cases := []struct{ file, sample, ext string }{
		{"alpha.jpg", "mem_alpha.webp", ".png"},
		{"alpha2.png", "mem_alpha.webp", ".png"},
		{"flat.jpg", "mem_lossy.webp", ".jpg"},
	}
	for _, tc := range cases {
		p := writeFile(t, dir, tc.file, webpSample(t, tc.sample))
		ph, err := ReadInfo(p)
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		for _, preview := range []bool{false, true} {
			f, err := c.File(ph, preview)
			if err != nil {
				t.Fatalf("%s: %v", tc.file, err)
			}
			if !strings.HasSuffix(f, tc.ext) {
				t.Errorf("%s preview=%v: копия %s, ждали %s", tc.file, preview, filepath.Base(f), tc.ext)
			}
			if tc.ext == ".png" {
				img := decodePNG(t, f)
				if _, _, _, a := img.At(0, 0).RGBA(); a == 0xFFFF {
					t.Errorf("%s preview=%v: прозрачность потеряна", tc.file, preview)
				}
			}
		}
	}
}
