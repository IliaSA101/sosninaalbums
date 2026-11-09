package render

// Подготовка ячейки фото: декодирование в пределах окна, поворот, обрезка по холсту и масштаб.

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"math"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// prepareFn — prepare; подменяется в тестах (паника, счёт запусков).
var prepareFn = prepare

// clipSlack — запас обрезки по холсту, px: холст «По страницам» на пиксель уже листа, и ячейка,
// вылезающая за него на пиксель, считается целиком, как прежде (иначе другая фаза Lanczos
// меняла бы пиксели готовых экспортов без пользы для памяти).
const clipSlack = 2

// prepare вырезает из оригинала окно кадра и масштабирует до размера ячейки в пикселях.
// Ячейка обрезается по clip (холст, px): свой стикер может выходить за лист до двух его
// размеров, а готовая ячейка целиком — это гигабайты (fatal OOM, recover не ловит). Размер
// ячейки никогда не больше clip с запасом clipSlack; то, что целиком внутри, считается как прежде, дословно.
// Фото, вписанное целиком (contain), меньше ячейки: назначение — пересечение ячейки с фото,
// окно — его часть кадра; остальное остаётся фоном страницы. nil — рисовать нечего.
// ImgRot, кратный 90° (путь 1): окно и уменьшение — в осях файла, затем зеркало и поворот
// готовой ячейки; при ImgRot = 0 геометрия прежняя.
func prepare(ctx context.Context, ph Photo, dpi int, clip image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
	if ph.Img.W <= 0 || ph.Img.H <= 0 {
		return nil, image.Rectangle{}, fmt.Errorf("%s: у фото нет кадра", baseName(ph.Path))
	}
	q, _ := quarterOf(ph.ImgRot)
	si := ph.Img // фото на экране: при нечётной четверти стороны меняются вокруг центра
	ix, iy := center(ph.Img)
	if q%2 == 1 {
		si = layout.Rect{X: ix - ph.Img.H/2, Y: iy - ph.Img.W/2, W: ph.Img.H, H: ph.Img.W}
	}
	c := visible(ph.Cell, si)
	r := image.Rect(px(c.X, dpi), px(c.Y, dpi), px(c.X+c.W, dpi), px(c.Y+c.H, dpi))
	if r.Empty() {
		return nil, r, nil
	}
	full := r
	clip = clip.Inset(-clipSlack)
	r = r.Intersect(clip)
	if r.Empty() {
		return nil, r, nil
	}
	if r != full { // видимая часть — доли исходной по пикселям, тем же отображением мм → px
		c = subRect(c, full, r)
	}
	src, release, err := imgfile.OpenOrientedCtx(ctx, ph.Path)
	if err != nil {
		return nil, image.Rectangle{}, photoErr(ph.Path, err)
	}
	defer release()
	// Видимая часть в осях фото: поворот на −ImgRot вокруг центра Img.
	w := c
	dx0, dy0, dx1, dy1 := c.X-ix, c.Y-iy, c.X+c.W-ix, c.Y+c.H-iy
	switch q {
	case 1:
		w = layout.Rect{X: ix + dy0, Y: iy - dx1, W: c.H, H: c.W}
	case 2:
		w = layout.Rect{X: ix - dx1, Y: iy - dy1, W: c.W, H: c.H}
	case 3:
		w = layout.Rect{X: ix - dy1, Y: iy + dx0, W: c.H, H: c.W}
	}
	// Зеркало: окно берётся из отражённой внутри Img видимой части, готовая ячейка
	// переворачивается — на экране та же часть кадра, только отражённая.
	vis := w
	if ph.FlipH {
		w.X = 2*ph.Img.X + ph.Img.W - vis.X - vis.W
	}
	if ph.FlipV {
		w.Y = 2*ph.Img.Y + ph.Img.H - vis.Y - vis.H
	}
	b := src.Bounds()
	dw, dh := r.Dx(), r.Dy()
	if q%2 == 1 {
		dw, dh = dh, dw
	}
	var cell *image.NRGBA
	if r != full {
		cell = resizeClipped(src, w, ph.Img, dw, dh, clip)
	} else {
		win := cropWindow(w, ph.Img, b.Dx(), b.Dy()).Add(b.Min)
		cell = resizeBounded(window(src, win), dw, dh)
	}
	return orientCell(cell, ph, q), r, nil
}

// orientCell — зеркало готовой ячейки и поворот на ImgRot (q четвертей по часовой).
func orientCell(cell *image.NRGBA, ph Photo, q int) *image.NRGBA {
	if ph.FlipH {
		cell = imaging.FlipH(cell)
	}
	if ph.FlipV {
		cell = imaging.FlipV(cell)
	}
	// imaging крутит против часовой, ImgRot — по часовой.
	switch q {
	case 1:
		cell = imaging.Rotate270(cell)
	case 2:
		cell = imaging.Rotate180(cell)
	case 3:
		cell = imaging.Rotate90(cell)
	}
	return cell
}

// blit кладёт готовую ячейку на холст. Прозрачный PNG (логотип, дата) ложится на уже залитый
// фон страницы, а не затирает его чёрным; для обычных фото Src быстрее и даёт тот же результат.
func blit(canvas *image.RGBA, cell *image.NRGBA, r image.Rectangle) {
	op := draw.Src
	if !cell.Opaque() {
		op = draw.Over
	}
	draw.Draw(canvas, r, cell, image.Point{}, op)
}

// visible — часть ячейки, где есть фото: у cover (кадр закрывает ячейку) это вся ячейка,
// у contain — само фото внутри. Разница в доли микрона от округления — всё ещё ячейка:
// иначе px() мог бы съесть пиксель у края обычного фото.
func visible(cell, img layout.Rect) layout.Rect {
	const eps = 1e-6
	x0, y0 := max(cell.X, img.X), max(cell.Y, img.Y)
	x1, y1 := min(cell.X+cell.W, img.X+img.W), min(cell.Y+cell.H, img.Y+img.H)
	if x0-cell.X < eps {
		x0 = cell.X
	}
	if y0-cell.Y < eps {
		y0 = cell.Y
	}
	if cell.X+cell.W-x1 < eps {
		x1 = cell.X + cell.W
	}
	if cell.Y+cell.H-y1 < eps {
		y1 = cell.Y + cell.H
	}
	return layout.Rect{X: x0, Y: y0, W: max(x1-x0, 0), H: max(y1-y0, 0)}
}

// cropWindow — окно оригинала sw×sh px, которое видно в ячейке: доли берутся
// из положения ячейки в кадре, границы округляются, зажимаются в фото и не бывают пустыми.
func cropWindow(cell, img layout.Rect, sw, sh int) image.Rectangle {
	axis := func(c0, cw, i0, iw float64, n int) (int, int) {
		a := math.Round((c0 - i0) / iw * float64(n))
		b := math.Round((c0 + cw - i0) / iw * float64(n))
		lo := int(min(max(a, 0), float64(n-1)))
		hi := int(min(max(b, float64(lo+1)), float64(n)))
		return lo, hi
	}
	x0, x1 := axis(cell.X, cell.W, img.X, img.W, sw)
	y0, y1 := axis(cell.Y, cell.H, img.Y, img.H, sh)
	return image.Rect(x0, y0, x1, y1)
}

// subRect — часть c (мм), которой на экране соответствует подпрямоугольник part
// прямоугольника full (px): доли по пикселям, как у обрезки по холсту.
func subRect(c layout.Rect, full, part image.Rectangle) layout.Rect {
	fw, fh := float64(full.Dx()), float64(full.Dy())
	x0 := float64(part.Min.X-full.Min.X) / fw
	x1 := float64(part.Max.X-full.Min.X) / fw
	y0 := float64(part.Min.Y-full.Min.Y) / fh
	y1 := float64(part.Max.Y-full.Min.Y) / fh
	return layout.Rect{X: c.X + x0*c.W, Y: c.Y + y0*c.H, W: (x1 - x0) * c.W, H: (y1 - y0) * c.H}
}

// resizeClipped — окно w (мм, оси файла) до dw×dh px, когда видна лишь часть кадра. Окно
// не округляется внутрь (иначе картинка съехала бы на полпикселя оригинала, а у мелкого
// стикера, раздутого на лист, это десятки px печати): берётся целое окно снаружи, тянется
// с тем же масштабом и обрезается до dw×dh. Слишком большое (окно в доли пикселя файла) —
// прежнее округление: размер выделения важнее точности в полпикселя оригинала.
func resizeClipped(src image.Image, w, img layout.Rect, dw, dh int, clip image.Rectangle) *image.NRGBA {
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
		return resizeBounded(window(src, win), dw, dh)
	}
	win := image.Rect(x0, y0, x1, y1).Add(b.Min)
	big := resizeBounded(window(src, win), bw, bh)
	return imaging.Crop(big, image.Rect(ox, oy, ox+dw, oy+dh))
}

// resizeBounded — imaging.Resize Lanczos, у которого промежуточный буфер не больше окна
// оригинала и результата. imaging тянет сначала по ширине (буфер dw × высота окна): у узкого
// высокого оригинала в широкой низкой рамке (пропорции PhotoRef в .salbum подделываемы) это
// сотни МБ и больше, fatal OOM. Если по ширине растём, а по высоте уменьшаемся — идём
// по высоте первой (буфер ширина окна × dh). Остальные случаи — прежним порядком, пиксели те же.
func resizeBounded(src image.Image, dw, dh int) *image.NRGBA {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 || dw <= 0 || dh <= 0 || dw <= sw || dh >= sh {
		return imaging.Resize(src, dw, dh, imaging.Lanczos)
	}
	return imaging.Resize(imaging.Resize(src, sw, dh, imaging.Lanczos), dw, dh, imaging.Lanczos)
}
