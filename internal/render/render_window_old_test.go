package render

import (
	"context"
	"image"
	"math"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Прежние версии функций (до design 091 G1: imaging.Crop вместо window) — эталон побайтового
// совпадения для render_window_test.go. Не менять вслед за боевыми.

// resizeClippedOld — resizeClipped с копией окна.
func resizeClippedOld(src image.Image, w, img layout.Rect, dw, dh int, clip image.Rectangle) *image.NRGBA {
	b := src.Bounds()
	axis := func(c0, cw, i0, iw float64, n, d int) (lo, hi, big, off int, ok bool) {
		f0 := min(max((c0-i0)/iw*float64(n), 0), float64(n))
		f1 := min(max((c0+cw-i0)/iw*float64(n), 0), float64(n))
		if !(f1 > f0) {
			return 0, 0, 0, 0, false
		}
		// Поле в три «ядра» Lanczos с каждой стороны: у среза соседи те же, что у целой ячейки.
		m := math.Ceil(3 * math.Max(1, (f1-f0)/float64(d)))
		lo = int(math.Max(math.Floor(f0-m), 0))
		hi = int(math.Min(math.Ceil(f1+m), float64(n)))
		k := float64(d) / (f1 - f0)
		big = int(math.Round(float64(hi-lo) * k))
		off = int(math.Round((f0 - float64(lo)) * k))
		return lo, hi, max(big, off+d), off, true
	}
	x0, x1, bw, ox, okx := axis(w.X, w.W, img.X, img.W, b.Dx(), dw)
	y0, y1, bh, oy, oky := axis(w.Y, w.H, img.Y, img.H, b.Dy(), dh)
	if !okx || !oky || int64(bw)*int64(bh) > 2*int64(clip.Dx())*int64(clip.Dy()) {
		win := cropWindow(w, img, b.Dx(), b.Dy()).Add(b.Min)
		return resizeBounded(imaging.Crop(src, win), dw, dh)
	}
	win := image.Rect(x0, y0, x1, y1).Add(b.Min)
	big := resizeBounded(imaging.Crop(src, win), bw, bh)
	return imaging.Crop(big, image.Rect(ox, oy, ox+dw, oy+dh))
}

// cropSourceOld — cropSource с копией окна.
func cropSourceOld(ctx context.Context, ph Photo, k float64, covers bool, u0, v0, u1, v1 float64) (*rotSrc, error) {
	W, H := ph.Img.W, ph.Img.H
	src, release, err := imgfile.OpenOrientedCtx(ctx, ph.Path)
	if err != nil {
		return nil, photoErr(ph.Path, err)
	}
	defer release()
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	win := image.Rect(int(math.Floor(u0/W*sw))-1, int(math.Floor(v0/H*sh))-1, int(math.Ceil(u1/W*sw))+1, int(math.Ceil(v1/H*sh))+1).
		Intersect(image.Rect(0, 0, b.Dx(), b.Dy()))
	if win.Empty() {
		return nil, nil
	}
	// Окно крупнее печати больше чем вдвое — Lanczos до размера печати. Только уменьшение:
	// размеры кадра из .salbum могут быть подделаны, растягивание просило бы гигабайты.
	// Копия окна отпускает бюджет декода до своей очереди на холст.
	printW := max(1, int(math.Round(float64(win.Dx())/sw*W*k)))
	printH := max(1, int(math.Round(float64(win.Dy())/sh*H*k)))
	crop := imaging.Crop(src, win.Add(b.Min))
	if win.Dx() > 2*printW || win.Dy() > 2*printH {
		crop = imaging.Resize(crop, min(printW, win.Dx()), min(printH, win.Dy()), imaging.Lanczos)
	}
	pad := 0
	if !covers {
		// Край фото внутри маски: за край продолжаем крайние пиксели, иначе сглаженная
		// маска легла бы на прозрачное и край вышел бы ступенькой.
		pad = 2
		crop = padEdge(crop, pad)
	}
	applyTone(crop, ph.Tone) // пути 2 и 3: до поворота, как путь 1 — после уменьшения
	fadeAlpha(crop, ph.Fade) // пути 2 и 3: draw выберет Over по альфе
	return &rotSrc{im: crop, win: win, sw: sw, sh: sh, pad: pad}, nil
}
