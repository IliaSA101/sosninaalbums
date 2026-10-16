package render

import (
	"fmt"
	"image"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Backdrop — фото на фоне (design-backdrop §8). Area — область (разворот или страница),
// Box — рамка (угол до поворота и размер), Rot — градусы по часовой вокруг угла рамки,
// Img — фото целиком в осях рамки (от её левого верхнего угла). Зеркало — внутри Img;
// из PhotoRef оно не берётся. ImgRot — наклон фото внутри рамки вокруг центра Img (045,
// BackdropView.Frame.R): Img задан в осях фото, как у места.
type Backdrop struct {
	Area, Box    layout.Rect
	Rot          float64
	ImgRot       float64
	Img          layout.Rect
	FlipH, FlipV bool
	Path         string
	Fade         float64 // прозрачность 0..1; 1 — фон не рисуется (спека 017)
}

// photo — фон без поворота и наклона как обычное фото: ячейка — рамка в пределах области.
// С поворотом или наклоном — backdropPhoto (rotated.go).
func (b Backdrop) photo() (Photo, bool) {
	cell := intersect(b.Box, b.Area)
	if cell.W <= 0 || cell.H <= 0 {
		return Photo{}, false
	}
	img := b.Img
	img.X += b.Box.X
	img.Y += b.Box.Y
	return Photo{Cell: cell, Img: img, Path: b.Path, FlipH: b.FlipH, FlipV: b.FlipV, Fade: b.Fade}, true
}

func intersect(a, b layout.Rect) layout.Rect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// fadeAlpha уменьшает альфу на (1 − fade): фон ложится на цвет страницы как CSS opacity
// у .box. fade ≤ 0 — без изменений (прежний путь дословно, байт-в-байт). im — своя копия
// пикселей (Crop/Resize/padEdge), чужое не трогаем.
func fadeAlpha(im *image.NRGBA, fade float64) {
	if im == nil || !(fade > 0) {
		return
	}
	a := uint32(math.Round(255 * (1 - min(fade, 1))))
	w := im.Rect.Dx()
	for y := range im.Rect.Dy() {
		row := im.Pix[y*im.Stride : y*im.Stride+4*w]
		for i := 3; i < len(row); i += 4 {
			row[i] = uint8((uint32(row[i])*a + 127) / 255) //nolint:gosec // a ≤ 255: итог 0..255
		}
	}
}

// drawPatterns кладёт узоры на залитые страницы по порядку (лист, левая, правая — их области не
// пересекаются). Область в px — от тех же px(), что заливка страниц: узор правой страницы
// начинается ровно на шве. Паника генератора — ошибка, как у фото: процесс не падает.
func drawPatterns(canvas *image.RGBA, pats []PatternDraw, dpi int) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("сбой при рисовании узора: %v", p)
		}
	}()
	for _, pd := range pats {
		if pd.Fade >= 1 {
			continue
		}
		a := pd.Area
		r := image.Rect(px(a.X, dpi), px(a.Y, dpi), px(a.X+a.W, dpi), px(a.Y+a.H, dpi))
		spec := gen.Spec{Kind: pd.Kind, Inks: pd.Inks, Scale: pd.Scale, Seed: pd.Seed}
		gen.DrawPattern(canvas, r, spec, float64(dpi)/25.4, pd.Fade)
	}
	return nil
}
