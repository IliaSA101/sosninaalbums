package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обёртки пипетки и «С фото» на настоящем App: фото альбома идёт через mediaPhoto и кэш миниатюр,
// альбом не меняется, ошибки доходят как есть. Подробности — в colorpick.
func TestColorPickWrappers(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	// Кэш закрывает closeBackground из newApp (a.cache): отдельный хук закрыл бы его при идущем фоне.
	a.cache = cache
	im := image.NewRGBA(image.Rect(0, 0, 600, 400))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{R: 220, G: 30, B: 30, A: 255}), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	p := model.New()
	sp := &model.Spread{ID: "s1", Photos: []string{"a"},
		Grid: []*model.Place{{ID: model.NewID(), Photo: "a", X: 5, Y: 5, W: 100, H: 80}}}
	p.Photos["a"] = model.PhotoRef{Path: path, W: 600, H: 400}
	p.Spreads = []*model.Spread{sp}
	a.ed.Store(mustEditor(t, p, ""))
	rev := a.editor().View().Rev

	cols, err := a.SpreadPhotoColors("s1")
	if err != nil || len(cols) == 0 {
		t.Fatalf("SpreadPhotoColors: %v %v", cols, err)
	}
	hex, err := a.PickColorAt("s1", 50, 45)
	if err != nil || len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("PickColorAt: %q %v", hex, err)
	}
	if _, err := a.PickColorAt("нет", 1, 1); err == nil || err.Error() != "лист не найден" {
		t.Errorf("чужой лист: %v", err)
	}
	if cols, err := a.SpreadPhotoColors("нет"); err == nil || cols == nil || len(cols) != 0 {
		t.Errorf("SpreadPhotoColors чужой лист: %#v %v", cols, err)
	}
	if _, err := a.PickColorAt("s1", -1, 1); err == nil || err.Error() != "точка вне листа" {
		t.Errorf("вне листа: %v", err)
	}
	if v := a.editor().View(); v.Rev != rev {
		t.Errorf("методы цвета изменили альбом: rev %d → %d", rev, v.Rev)
	}
}
