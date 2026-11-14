package app

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Аудит 117: WebP под .jpg/.png не уходит в WebView2 байтами файла — только перекодированным Go
// (JPEG, а с прозрачностью PNG); тип малых JPEG/PNG определяется по содержимому, не по имени.
func TestServeGuideWebPReencoded(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	built := countPreview(t)
	h := a.media(http.NotFoundHandler())
	dir := testkit.TempDir(t)
	sample := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join("..", "imgfile", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	pngData, err := os.ReadFile(testkit.PNG(t, dir, "src.png", 30, 20, color.NRGBA{R: 5, A: 255}))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		file    string
		data    []byte
		ct      string
		rebuilt bool // превью строит Go (иначе — файл как есть)
	}{
		{"lossy.jpg", sample("lossy16.webp"), "image/jpeg", true},
		{"lossless.png", sample("lossless24x16.webp"), "image/jpeg", true},
		{"alpha.jpg", sample("mem_alpha.webp"), "image/png", true},
		{"wrongname.jpg", pngData, "image/png", false}, // PNG под именем .jpg: тип по содержимому
	}
	wantBuilt := int32(0)
	for _, c := range cases {
		path := filepath.Join(dir, c.file)
		if err := os.WriteFile(path, c.data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := a.setCoverGuide(path); err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/cover-guide", nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != c.ct {
			t.Fatalf("%s: %d, тип %q", c.file, w.Code, w.Header().Get("Content-Type"))
		}
		body := w.Body.Bytes()
		if bytes.HasPrefix(body, []byte("RIFF")) {
			t.Fatalf("%s: WebP ушёл наружу байтами файла", c.file)
		}
		if c.ct == "image/png" {
			_, err = png.Decode(bytes.NewReader(body))
		} else {
			_, err = jpeg.Decode(bytes.NewReader(body))
		}
		if err != nil {
			t.Fatalf("%s: ответ не %s: %v", c.file, c.ct, err)
		}
		if c.rebuilt {
			wantBuilt++
		} else if !bytes.Equal(body, c.data) {
			t.Fatalf("%s: малый файл должен уйти как есть", c.file)
		}
		if built.Load() != wantBuilt {
			t.Fatalf("%s: построений %d, ждали %d", c.file, built.Load(), wantBuilt)
		}
	}
}
