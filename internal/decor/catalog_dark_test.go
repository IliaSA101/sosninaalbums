package decor

import (
	"encoding/json"
	"image"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

// paleShare — доля непрозрачных пикселей стикера, почти белых и без цвета: на светлой шашечке
// плитки они сливаются с фоном.
func paleShare(p preset) float64 {
	sp := gen.Clamp(p.spec)
	rs, _ := gen.Resolve(sp)
	w, h := 180.0, 180.0/rs.Aspect
	if rs.Aspect < 1 {
		w, h = 180*rs.Aspect, 180
	}
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	gen.DrawArt(img, [6]float64{w, 0, 10, 0, h, 10}, sp, 0)
	var pale, n float64
	for i := 0; i < len(img.Pix); i += 4 {
		a := float64(img.Pix[i+3])
		if a < 128 {
			continue
		}
		r, g, b := float64(img.Pix[i])*255/a, float64(img.Pix[i+1])*255/a, float64(img.Pix[i+2])*255/a
		if 0.2126*r+0.7152*g+0.0722*b > 235 && max(r, g, b)-min(r, g, b) < 24 {
			pale++
		}
		n++
	}
	if n == 0 {
		return 0
	}
	return pale / n
}

// TestStickerDarkCatalog — каталог стикеров отдаёт dark у белых фирменных и плашки; у остальных
// и у фонов поля нет; в JSON ключ «dark» есть только у светлых.
func TestStickerDarkCatalog(t *testing.T) {
	s, _ := openTemp(t)
	cat, err := s.Catalog(KindSticker, nil, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	dark := 0
	for _, it := range cat.Items {
		if it.Dark != lightStickers[it.ID] {
			t.Errorf("%s: dark=%v", it.ID, it.Dark)
		}
		raw, _ := json.Marshal(it)
		if has := strings.Contains(string(raw), `"dark":true`); has != it.Dark {
			t.Errorf("%s: JSON %s", it.ID, raw)
		}
		if it.Dark {
			dark++
		}
	}
	for _, id := range []string{"sa:st-brand-albums-white", "sa:st-brand-films-white"} {
		if !slicesHasItem(cat.Items, id, true) {
			t.Errorf("%s без dark", id)
		}
	}
	for _, id := range []string{"sa:st-brand-albums-black", "sa:st-brand-films-black", "sa:st-star-gold"} {
		if !slicesHasItem(cat.Items, id, false) {
			t.Errorf("%s: нет плитки или dark у тёмного", id)
		}
	}
	if dark != len(lightStickers) {
		t.Errorf("dark у %d плиток, ждали %d", dark, len(lightStickers))
	}
	bg, _ := s.Catalog(KindBg, nil, 600, 300)
	for _, it := range bg.Items {
		if it.Dark {
			t.Errorf("фон %s с dark", it.ID)
		}
	}
}

func slicesHasItem(items []Item, id string, dark bool) bool {
	for _, it := range items {
		if it.ID == id {
			return it.Dark == dark
		}
	}
	return false
}

// TestStickerDarkMatchesPixels — список светлых сверен с рисунком: новый белый стикер без отметки
// или отметка у цветного — провал.
func TestStickerDarkMatchesPixels(t *testing.T) {
	for _, p := range stickerPresets {
		share := paleShare(p)
		if light := share > 0.75; light != lightStickers[p.id] {
			t.Errorf("%s: светлых пикселей %.0f %%, dark=%v", p.id, 100*share, lightStickers[p.id])
		}
	}
	for id := range lightStickers {
		if _, ok := findPreset(stickerPresets, id); !ok {
			t.Errorf("%s: нет в каталоге стикеров", id)
		}
	}
}
