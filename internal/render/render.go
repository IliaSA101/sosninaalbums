package render

import (
	"cmp"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Render рисует разворот в полном разрешении: фон страниц, узоры, фото на фоне, затем фото мест,
// стикеры и тексты одним стеком по Z. Для ячейки «в край» окно — центр кадра, как у прежнего Fill.
func Render(s Spread, opt Options) (*image.RGBA, error) {
	W, H := CanvasSize(opt)
	if err := CheckSize(W, H); err != nil {
		return nil, err
	}
	canvas := image.NewRGBA(image.Rect(0, 0, W, H))
	// Страницы красим двумя половинами; корешок — граница между ними.
	mid := min(max(px(opt.PageW, opt.DPI), 0), W)
	fill(canvas, image.Rect(0, 0, mid, H), s.Bg[0])
	fill(canvas, image.Rect(mid, 0, W, H), s.Bg[1])
	if err := drawPatterns(canvas, s.Patterns, opt.DPI); err != nil {
		return nil, err
	}

	items := make([]item, 0, len(s.Backdrops)+len(s.Plates)+len(s.Photos)+len(s.Stickers)+len(s.Arts)+len(s.Texts))
	for _, b := range s.Backdrops {
		if b.Fade >= 1 { // не виден: файл не открываем
			continue
		}
		if ph, clip, ok := backdropPhoto(b, opt.DPI); ok {
			items = append(items, item{ph: ph, clip: clip, backdrop: true})
		}
	}
	for i := range s.Plates { // плашки лежат поверх фонов и под всем остальным
		items = append(items, item{plate: &s.Plates[i]})
	}
	nb := len(items)
	for _, ph := range s.Photos {
		items = append(items, item{ph: ph, z: ph.Z})
	}
	for _, ph := range s.Stickers {
		if ph.Fade < 1 { // не виден: файл не открываем
			items = append(items, item{ph: ph, z: ph.Z, sticker: true})
		}
	}
	for i := range s.Arts {
		items = append(items, item{art: &s.Arts[i], z: s.Arts[i].Z})
	}
	for i := range s.Texts {
		items = append(items, item{text: &s.Texts[i], z: s.Texts[i].Z})
	}
	// Фоны — под всем; остальное по слоям, при равных Z — прежний порядок (фото, затем тексты).
	slices.SortStableFunc(items[nb:], func(a, b item) int { return cmp.Compare(a.z, b.z) })
	if err := drawItems(canvas, items, opt.DPI); err != nil {
		return nil, err
	}
	return canvas, nil
}

// item — элемент стека разворота: фото (место, фон или свой стикер), процедурный стикер,
// плашка либо текст.
type item struct {
	ph       Photo
	clip     image.Rectangle // путь 3 у фона: его область в px; пусто — весь холст
	backdrop bool
	sticker  bool
	text     *TextDraw
	plate    *Plate
	art      *ArtDraw
	z        int
}

// ParseHex разбирает «#rrggbb»; не цвет — белый (страница не должна пропасть из-за порченого файла).
func ParseHex(s string) color.RGBA {
	white := color.RGBA{255, 255, 255, 255}
	if len(s) != 7 || s[0] != '#' {
		return white
	}
	var c [3]uint8
	for i := range c {
		v, err := strconv.ParseUint(s[1+2*i:3+2*i], 16, 8)
		if err != nil {
			return white
		}
		c[i] = uint8(v)
	}
	return color.RGBA{c[0], c[1], c[2], 255}
}

// fill заливает прямоугольник цветом. Нулевой цвет (не задан) — белый.
func fill(dst *image.RGBA, r image.Rectangle, c color.RGBA) {
	if c == (color.RGBA{}) {
		c = color.RGBA{255, 255, 255, 255}
	}
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// drawItems готовит фото параллельно, но кладёт на холст строго по порядку списка: последнее
// сверху, как на экране; тексты рисуются в той же очереди. Слот семафора (3 готовые ячейки
// в памяти: оригинал на 24 Мп — это ~100 МБ) берётся здесь, по порядку и до go, поэтому
// младший индекс всегда получает слот раньше старших и тех, кто его ждёт, не блокирует.
// Тексту слот не нужен: памяти он не держит.
func drawItems(canvas *image.RGBA, items []item, dpi int) error {
	sem := make(chan struct{}, 3)
	errs := make([]error, len(items))
	done := make([]chan struct{}, len(items))
	for i := range done {
		done[i] = make(chan struct{})
	}
	var wg sync.WaitGroup
	var failed atomic.Bool // после первой ошибки новые фото не запускаем: результат всё равно отброшен
	for i, it := range items {
		photo := it.text == nil && it.plate == nil && it.art == nil
		if photo {
			sem <- struct{}{}
		}
		if failed.Load() {
			if photo {
				<-sem
			}
			break
		}
		wg.Go(func() {
			if photo {
				defer func() { <-sem }()
			}
			defer close(done[i]) // и при ошибке: иначе следующий ждал бы вечно
			var paint func(*image.RGBA) error
			var err error
			switch {
			case photo:
				paint, err = prepareSafe(it, canvas.Bounds(), dpi)
			case it.art != nil:
				a := *it.art
				paint = func(c *image.RGBA) error {
					drawArt(c, a, dpi)
					return nil
				}
			case it.plate != nil:
				pl := *it.plate
				paint = func(c *image.RGBA) error {
					r := image.Rect(px(pl.Rect.X, dpi), px(pl.Rect.Y, dpi), px(pl.Rect.X+pl.Rect.W, dpi), px(pl.Rect.Y+pl.Rect.H, dpi))
					fill(c, r.Intersect(c.Bounds()), pl.Color)
					return nil
				}
			default:
				t := *it.text
				paint = func(c *image.RGBA) error {
					text.Draw(c, t.Box, t.Style, t.X, t.Y, t.Rot, dpi)
					return nil
				}
			}
			if i > 0 {
				<-done[i-1]
			}
			if err == nil && paint != nil {
				err = paintSafe(paint, canvas, it)
			}
			if err != nil {
				errs[i] = err
				failed.Store(true)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// prepareSafe готовит фото по одному из трёх путей (design-rotation §3); паника декодера
// становится ошибкой: горутина без recover уронила бы весь процесс. nil без ошибки — рисовать нечего.
func prepareSafe(it item, canvas image.Rectangle, dpi int) (paint func(*image.RGBA) error, err error) {
	defer func() {
		if p := recover(); p != nil {
			paint, err = nil, fmt.Errorf("%s: сбой при обработке %s: %v", baseName(it.ph.Path), what(it), p)
		}
	}()
	ph := fold(it.ph)
	clip := canvas
	if !it.clip.Empty() {
		clip = it.clip.Intersect(canvas)
	}
	if _, quarter := quarterOf(ph.ImgRot); ph.Rot == 0 && quarter {
		cell, r, err := prepareFn(ph, dpi, clip)
		if err != nil || cell == nil {
			return nil, err
		}
		fadeAlpha(cell, ph.Fade) // снаружи prepareFn: подмена в тестах его не обходит
		if hasShape(ph.Outline) {
			// Маска — от рамки, а не от r: у «Целиком» r меньше рамки.
			m := shapeMask(r, ph.Cell, 0, ph.Outline, float64(dpi)/25.4)
			return func(c *image.RGBA) error {
				blitMasked(c, cell, r, m)
				return nil
			}, nil
		}
		return func(c *image.RGBA) error {
			blit(c, cell, r)
			return nil
		}, nil
	}
	job, err := prepareRot(ph, dpi, clip)
	if err != nil || job == nil {
		return nil, err
	}
	return job.draw, nil
}

// paintSafe кладёт готовое на холст; паника — ошибка, как в prepareSafe. Паника в потоках
// пути 3 приходит ошибкой из inParallel.
func paintSafe(paint func(*image.RGBA) error, canvas *image.RGBA, it item) (err error) {
	name := baseName(it.ph.Path)
	if it.art != nil { // файла нет: в тексте — ключ рисунка
		name = it.art.Kind
	}
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%s: сбой при рисовании %s: %v", name, what(it), p)
		}
	}()
	if err := paint(canvas); err != nil {
		return fmt.Errorf("%s: сбой при рисовании %s: %w", name, what(it), err)
	}
	return nil
}

func what(it item) string {
	switch {
	case it.text != nil:
		return "текста"
	case it.backdrop:
		return "фона"
	case it.sticker, it.art != nil:
		return "стикера"
	}
	return "фото"
}

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
func prepare(ph Photo, dpi int, clip image.Rectangle) (*image.NRGBA, image.Rectangle, error) {
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
	src, release, err := imgfile.OpenOriented(ph.Path)
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
		cell = resizeBounded(imaging.Crop(src, win), dw, dh)
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
		return resizeBounded(imaging.Crop(src, win), dw, dh)
	}
	win := image.Rect(x0, y0, x1, y1).Add(b.Min)
	big := resizeBounded(imaging.Crop(src, win), bw, bh)
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
