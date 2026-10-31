package export

import (
	"context"
	"crypto/sha256"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Два прогона одних страниц — одни и те же байты: от этого зависит «грузятся только изменённые
// страницы» (146); между машинами это проверяет QA.
func TestWebDeterministic(t *testing.T) {
	photo := testkit.JPEGOriented(t, testkit.TempDir(t), "src.jpg", 640, 480, 1)
	r := layout.Rect{X: 10, Y: 5, W: 80, H: 50}
	spreads := []render.Spread{
		{Bg: [2]color.RGBA{cream, cream}, Photos: []render.Photo{{Cell: r, Img: r, Path: photo}}},
		{Bg: [2]color.RGBA{blue, red}, Plates: []render.Plate{plate(40, 10, 30, 30, green)}},
		{Bg: [2]color.RGBA{white, white}}, // обе страницы белые — файлов нет
	}
	sums := func() map[string][32]byte {
		dir := webDir(t)
		pages, err := RunWeb(context.Background(), spreads, nil, webSpec100(3, 3), dir, nil)
		if err != nil {
			t.Fatal(err)
		}
		files := readAll(t, dir)
		out := map[string][32]byte{}
		for _, p := range pages {
			if !p.Blank {
				out[p.Name] = sha256.Sum256(files[p.Name])
			}
		}
		return out
	}
	a, b := sums(), sums()
	if len(a) != 4 {
		t.Fatalf("страниц с файлом %d, ждали 4", len(a))
	}
	for name, sum := range a {
		if b[name] != sum {
			t.Errorf("страница %s: хэши двух прогонов различаются", name)
		}
	}
	// Другое содержимое — другой хэш: проверка не пустая.
	spreads[1].Plates[0] = plate(40, 10, 31, 30, green)
	if c := sums(); c["003.jpg"] == a["003.jpg"] {
		t.Error("правка разворота не изменила хэш страницы")
	}
}

// Длинная сторона — параметр: файл 800 px и 1600 px по запросу; негодная — отказ до записи.
func TestWebLongSide(t *testing.T) {
	s := model.Settings{PageW: 303, PageH: 300, Bleed: 3}
	wDef, err := NewWebSpec(s, model.View{})
	if err != nil {
		t.Fatal(err)
	}
	wSmall, err := NewWebSpecLong(s, model.View{}, 800)
	if err != nil {
		t.Fatal(err)
	}
	if wDef.LongSide != ShareLongSide || wDef.Quality != 82 || wSmall.LongSide != 800 || wSmall.DPI >= wDef.DPI {
		t.Fatalf("по умолчанию %+v, 800 px: %+v", wDef, wSmall)
	}
	if w0, err := NewWebSpecLong(s, model.View{}, 0); err != nil || w0.LongSide != ShareLongSide {
		t.Errorf("0 — по умолчанию: %+v %v", w0, err)
	}
	if _, err := NewWebSpecLong(s, model.View{}, maxLongSide+1); err == nil {
		t.Error("сторона больше предела принята")
	}
	sp := []render.Spread{{Bg: [2]color.RGBA{cream, cream}}}
	for long, w := range map[int]WebSpec{ShareLongSide: wDef, 800: wSmall} {
		dir := webDir(t)
		if _, err := RunWeb(context.Background(), sp, nil, w, dir, nil); err != nil {
			t.Fatal(err)
		}
		cfg := jpegSize(t, filepath.Join(dir, "001.jpg"))
		if got := max(cfg.Width, cfg.Height); got < long || got > long+10 {
			t.Errorf("%d px: файл %d×%d", long, cfg.Width, cfg.Height)
		}
	}
	for _, bad := range []int{-1, maxLongSide + 1} {
		w := wDef
		w.LongSide = bad
		dir := webDir(t)
		if _, err := RunWeb(context.Background(), sp, nil, w, dir, nil); err == nil {
			t.Errorf("LongSide %d принят", bad)
		}
		if len(readAll(t, dir)) != 0 {
			t.Errorf("LongSide %d: что-то записано", bad)
		}
	}
}
