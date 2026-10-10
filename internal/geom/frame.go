package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Crop — кадр фото в рамке: значения model.Frame (S, X, Y, R), теги JSON те же
// (векторы testdata/parity/frames.json).
type Crop struct {
	S float64 `json:"s"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	R float64 `json:"r,omitempty"`
}

// ContainImage — фото целиком внутри места по центру (Fit = contain). С r≠0 фото
// повёрнуто на r вокруг центра и целиком в ячейке (Img — в осях фото).
func ContainImage(cell layout.Rect, aspect, r float64) layout.Rect {
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	var w, h float64
	if r == 0 {
		w, h = cell.W, cell.W/aspect
		if h > cell.H {
			h, w = cell.H, cell.H*aspect
		}
	} else {
		sn, cs := sinCosDeg(r)
		s, c := math.Abs(sn), math.Abs(cs)
		w = math.Min(cell.W/(c+s/aspect), cell.H/(s+c/aspect))
		h = w / aspect
	}
	return layout.Rect{X: cell.X + (cell.W-w)/2, Y: cell.Y + (cell.H-h)/2, W: w, H: h}
}

// PlaceImage — фото целиком (мм) для ячейки cell: «закрыть ячейку» × S, сдвиг
// X/Y — доля свободного хода. С R≠0 Img — прямоугольник фото в его осях, на экране
// он повёрнут на R вокруг своего центра и закрывает ячейку (design-rotation §2.2).
// Зеркало — frontend geom/frame.ts, паритет держит testdata/parity/frames.json.
func PlaceImage(cell layout.Rect, aspect float64, f Crop) layout.Rect {
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	if f.R != 0 {
		return placeImageRot(cell, aspect, f)
	}
	// Без поворота — прежняя формула дословно: иной порядок сложений меняет
	// последний бит, и px() старых альбомов сдвинулся бы на пиксель.
	w, h := cell.W, cell.W/aspect
	if h < cell.H {
		h, w = cell.H, cell.H*aspect
	}
	w, h = w*f.S, h*f.S
	fx, fy := w-cell.W, h-cell.H // ≥ 0
	return layout.Rect{X: cell.X - fx*(1+f.X)/2, Y: cell.Y - fy*(1+f.Y)/2, W: w, H: h}
}

// sinCosDeg — синус и косинус угла в градусах; кратные 90° точные (без хвоста 1e-16).
func sinCosDeg(deg float64) (s, c float64) {
	if math.Mod(deg, 90) == 0 {
		switch ((int(deg/90) % 4) + 4) % 4 {
		case 0:
			return 0, 1
		case 1:
			return 1, 0
		case 2:
			return 0, -1
		default:
			return -1, 0
		}
	}
	return math.Sincos(deg * math.Pi / 180)
}

// placeImageRot — PlaceImage при R≠0. Фото закрывает повёрнутую рамку ⇔ описанный
// прямоугольник рамки в осях фото (W', H') лежит внутри фото; ход сдвига задан в
// осях фото, поэтому остаётся прямоугольником.
func placeImageRot(cell layout.Rect, aspect float64, f Crop) layout.Rect {
	sn, cs := sinCosDeg(f.R)
	s, c := math.Abs(sn), math.Abs(cs)
	bw, bh := cell.W*c+cell.H*s, cell.W*s+cell.H*c
	w, h := bw, bw/aspect
	if h < bh {
		h, w = bh, bh*aspect
	}
	w, h = w*f.S, h*f.S
	fx, fy := w-bw, h-bh // ≥ 0
	ux, uy := -fx*f.X/2, -fy*f.Y/2
	ox, oy := ux*cs-uy*sn, ux*sn+uy*cs
	cx, cy := cell.X+cell.W/2, cell.Y+cell.H/2
	return layout.Rect{X: cx + ox - w/2, Y: cy + oy - h/2, W: w, H: h}
}

// FrameDPI — эффективное разрешение фото шириной px пикселей, растянутого на
// img.W мм; 0 — размер фото неизвестен.
func FrameDPI(px int, img layout.Rect) int {
	if px <= 0 || !(img.W > 0) {
		return 0
	}
	return int(math.Round(float64(px) / (img.W / 25.4)))
}
