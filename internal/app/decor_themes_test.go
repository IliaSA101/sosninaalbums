package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"io"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// bytesSrc — архив набора в памяти: предмет i — байты i.
type bytesSrc [][]byte

func (b bytesSrc) Extract(i int, w io.Writer) error { _, err := w.Write(b[i]); return err }

// themedBg — PNG w×20: разная ширина — разные ключи.
func themedBg(t *testing.T, w int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, 20))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = uint8(w), 255
	}
	img.Set(0, 0, color.NRGBA{R: 9, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Темы наборов и «Можно скачать» (спека 170 + 144): в окне любого вида id тем не повторяются, «Можно
// скачать» одна и последняя; набор, объявивший тему с этим служебным id, её не получает.
func TestDecorCatalogThemesUnique(t *testing.T) {
	a, _ := tilesApp(t, true) // в каталоге сети ivory: «Можно скачать» у фонов и разворотов
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	m := decor.PackManifest{ID: "ny-set", Version: 1, Title: "Новый год", License: "CC0-1.0",
		Themes: []decor.PackTheme{
			{ID: "newyear", Name: "Новый год", Base: "holiday", Order: 10},
			{ID: "get", Name: "Скачать", Base: "holiday", Order: 20}, // служебный id: объявление отвергается
		},
		Items: []decor.PackItem{{File: "bg/a.png", Kind: decor.KindBg, Theme: "holiday", Topic: "newyear", Name: "А"},
			{File: "bg/b.png", Kind: decor.KindBg, Theme: "holiday", Topic: "get", Name: "Б"}}}
	src := bytesSrc{themedBg(t, 31), themedBg(t, 32)}
	for i := range m.Items {
		m.Items[i].Size = int64(len(src[i]))
	}
	st, err := s.StagePack(m, src, decor.OriginNet, "")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"bg", "sticker", "frame", "spread", "album"} {
		c, err := a.DecorCatalog(kind)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		var ids []string
		seen := map[string]bool{}
		for _, th := range c.Themes {
			if seen[th.ID] {
				t.Errorf("%s: id %q повторён: %v", kind, th.ID, ids)
			}
			seen[th.ID] = true
			ids = append(ids, th.ID)
		}
		if n := len(c.Get); n > 0 {
			last := c.Themes[len(c.Themes)-1]
			if last.ID != "get" || last.Name != "Можно скачать" || last.Count != n {
				t.Errorf("%s: последняя тема %+v при %d плитках", kind, last, n)
			}
		} else if seen["get"] {
			t.Errorf("%s: «Можно скачать» без плиток: %v", kind, ids)
		}
	}
	// У фонов: тема набора сразу после «Праздника», служебный id не принят, предмет с ним — в базе.
	c, _ := a.DecorCatalog("bg")
	var line []string
	for _, th := range c.Themes {
		line = append(line, th.ID)
	}
	h := slices.Index(line, "holiday")
	if h < 0 || line[h+1] != "newyear" || slices.Index(line, "get") != len(line)-1 {
		t.Errorf("порядок тем фонов: %v", line)
	}
	for _, th := range c.Themes {
		if th.ID == "newyear" && th.Count != 1 {
			t.Errorf("newyear: %+v", th)
		}
	}
}
