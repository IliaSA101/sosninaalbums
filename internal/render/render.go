package render

import (
	"cmp"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Render рисует разворот в полном разрешении: фон страниц, узоры, фото на фоне, затем фото мест,
// стикеры и тексты одним стеком по Z. Для ячейки «в край» окно — центр кадра, как у прежнего Fill.
func Render(s Spread, opt Options) (*image.RGBA, error) {
	return RenderCtx(context.Background(), s, opt)
}

// RenderCtx — Render с отменой (080-8): ctx доходит до ожидания бюджета памяти и декодов, и
// перед каждым элементом листа проверяется. Отмена — ctx.Err() без обёртки, холст отбрасывается.
func RenderCtx(ctx context.Context, s Spread, opt Options) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	W, H := sceneSize(opt)
	if err := CheckSize(W, H); err != nil {
		return nil, err
	}
	canvas, err := newCanvas(opt, W, H)
	if err != nil {
		return nil, err
	}
	// Страницы красим двумя половинами; корешок — граница между ними.
	mid := min(max(px(opt.PageW, opt.DPI), 0), W)
	fill(canvas, image.Rect(0, 0, mid, H), s.Bg[0])
	fill(canvas, image.Rect(mid, 0, W, H), s.Bg[1])
	if err := drawPatterns(canvas, s.Patterns, opt.DPI); err != nil {
		return nil, err
	}

	items := make([]item, 0, len(s.Backdrops)+len(s.Plates)+len(s.Slots)+len(s.Photos)+len(s.Borders)+len(s.Stickers)+len(s.Arts)+len(s.Texts))
	env := &borderEnv{} // кэши рамочек этого листа
	// Фоны без OverPlates, плашки, фоны с OverPlates (фото корешка над плашкой, 061).
	addBackdrops := func(over bool) {
		for _, b := range s.Backdrops {
			if b.OverPlates != over || b.Fade >= 1 { // не виден: файл не открываем
				continue
			}
			if ph, clip, ok := backdropPhoto(b, opt.DPI); ok {
				items = append(items, item{ph: ph, clip: clip, backdrop: true})
			}
		}
	}
	addBackdrops(false)
	for i := range s.Plates { // плашки лежат поверх фонов и под всем остальным
		items = append(items, item{plate: &s.Plates[i]})
	}
	addBackdrops(true)
	nb := len(items)
	for _, ph := range s.Photos {
		if ph.Fade < 1 { // не виден (079): файл не открываем; рамочка у него своя
			items = append(items, item{ph: ph, z: ph.Z, env: env})
		}
	}
	for i := range s.Borders { // сразу за фото: при равных Z рамочка над своим фото
		items = append(items, item{border: &s.Borders[i], z: s.Borders[i].Z, env: env})
	}
	for i := range s.Slots {
		items = append(items, item{plate: &s.Slots[i], z: s.Slots[i].Z})
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
	// Подгонка: элементы считаются по сцене, а не по холсту — пиксели те же, что без подгонки,
	// а полосы за сценой остаются для fitEdges.
	if err := drawItemsIn(ctx, canvas, image.Rect(0, 0, W, H), items, opt.DPI); err != nil {
		if cerr := ctx.Err(); cerr != nil { // отмена важнее обёртки «фото: …» от того, кто упал первым
			return nil, cerr
		}
		return nil, err
	}
	fitEdges(canvas, W, H)
	return canvas, nil
}

// newCanvas — холст разворота: сцена W×H на Rect(0,0,W,H); с подгонкой (Fit) — холст FitW×FitH
// на том же буфере со сдвигом Rect.Min, чтобы сцена осталась в прежних координатах (123 §5).
// Лишнее срезается поровну по краям, недостающие полосы (≤ FitTolPx/2 на край) закрывает fitEdges.
func newCanvas(opt Options, W, H int) (*image.RGBA, error) {
	if !opt.fitted() {
		return image.NewRGBA(image.Rect(0, 0, W, H)), nil
	}
	if err := CheckSize(opt.FitW, opt.FitH); err != nil {
		return nil, err
	}
	tol := FitTolPx(opt.DPI)
	if dw, dh := opt.FitW-W, opt.FitH-H; dw > tol || dw < -tol || dh > tol || dh < -tol {
		return nil, fmt.Errorf("подгонка %d×%d px к разворотам %d×%d px больше допуска %d px", opt.FitW, opt.FitH, W, H, tol)
	}
	x0, y0 := -(opt.FitW-W)/2, -(opt.FitH-H)/2 // усечение к нулю: полоса с краю на пиксель шире справа/снизу
	return image.NewRGBA(image.Rect(x0, y0, x0+opt.FitW, y0+opt.FitH)), nil
}

// fitEdges закрывает полосы подгонки (холст шире сцены W×H): строки и столбцы за сценой
// повторяют крайний ряд. Холст без сдвига — ничего не делает (прежний путь).
func fitEdges(c *image.RGBA, W, H int) {
	b := c.Rect
	if b.Min.X >= 0 && b.Min.Y >= 0 && b.Max.X <= W && b.Max.Y <= H {
		return
	}
	scene := b.Intersect(image.Rect(0, 0, W, H))
	if scene.Empty() {
		return
	}
	row := func(y int) []uint8 { o := c.PixOffset(b.Min.X, y); return c.Pix[o : o+4*b.Dx()] }
	for y := scene.Min.Y; y < scene.Max.Y; y++ { // по краям строки сцены — крайние пиксели
		r := row(y)
		l, rt := 4*(scene.Min.X-b.Min.X), 4*(scene.Max.X-1-b.Min.X)
		for x := 0; x < l; x += 4 {
			copy(r[x:x+4], r[l:l+4])
		}
		for x := rt + 4; x < len(r); x += 4 {
			copy(r[x:x+4], r[rt:rt+4])
		}
	}
	for y := b.Min.Y; y < scene.Min.Y; y++ { // строки выше и ниже сцены — крайняя строка целиком
		copy(row(y), row(scene.Min.Y))
	}
	for y := scene.Max.Y; y < b.Max.Y; y++ {
		copy(row(y), row(scene.Max.Y-1))
	}
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
	border   *BorderDraw // рамочка места (079)
	env      *borderEnv  // кэши рамочек листа; nil вне Render
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

// photoSlots — сколько фото листа готовятся сразу: половина ядер, не меньше 3 (как было) и не
// больше 6 — память (design 091 §5). Пиксели от числа слотов не зависят: на холст фото
// ложатся по порядку. Переменная — тест подменяет.
var photoSlots = func() int { return min(max(runtime.NumCPU()/2, 3), 6) }

// window — окно r оригинала для ресайза: SubImage без копии (сканер imaging читает те же байты,
// что у imaging.Crop, design 091 F3); тип без SubImage — копия, как раньше. Результат только
// читается: окно не уходит дальше ресайза — всё, что меняет пиксели на месте, получает новый
// буфер Resize/Clone/Crop. Оригинал живёт до конца prepare/cropSource (defer release()).
func window(src image.Image, r image.Rectangle) image.Image {
	if s, ok := src.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok {
		return s.SubImage(r)
	}
	return imaging.Crop(src, r)
}

// drawItems готовит фото параллельно, но кладёт на холст строго по порядку списка: последнее
// сверху, как на экране; тексты рисуются в той же очереди. Слот семафора (photoSlots() готовых
// ячеек в памяти: оригинал на 24 Мп — это ~100 МБ) берётся здесь, по порядку и до go, поэтому
// младший индекс всегда получает слот раньше старших и тех, кто его ждёт, не блокирует.
// Тексту слот не нужен: памяти он не держит.
func drawItems(ctx context.Context, canvas *image.RGBA, items []item, dpi int) error {
	return drawItemsIn(ctx, canvas, canvas.Bounds(), items, dpi)
}

// drawItemsIn — drawItems с областью сцены scene: по ней обрезаются фото и рамочки (холст
// подгонки отличается от сцены сдвигом и размером).
func drawItemsIn(ctx context.Context, canvas *image.RGBA, scene image.Rectangle, items []item, dpi int) error {
	sem := make(chan struct{}, photoSlots())
	errs := make([]error, len(items))
	done := make([]chan struct{}, len(items))
	for i := range done {
		done[i] = make(chan struct{})
	}
	var wg sync.WaitGroup
	var failed atomic.Bool // после первой ошибки новые фото не запускаем: результат всё равно отброшен
	for i, it := range items {
		photo := it.text == nil && it.plate == nil && it.art == nil && (it.border == nil || it.border.Pic)
		if photo {
			sem <- struct{}{}
		}
		if failed.Load() || ctx.Err() != nil {
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
			paint, err := preparePaint(ctx, it, photo, scene, dpi)
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
	return ctx.Err() // запуск прерван отменой до первой ошибки: холст недорисован
}

// preparePaint готовит элемент листа: фото и рамочки-картинки декодируются здесь (в потоке
// подготовки), остальное — замыкание рисования. Отменённый ctx — ошибка до любой работы.
func preparePaint(ctx context.Context, it item, photo bool, canvas image.Rectangle, dpi int) (paint func(*image.RGBA) error, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch {
	case it.border != nil:
		return prepareBorderSafe(ctx, it, canvas, dpi)
	case photo:
		return prepareSafe(ctx, it, canvas, dpi)
	case it.art != nil:
		a := *it.art
		return func(c *image.RGBA) error {
			drawArt(c, a, dpi)
			return nil
		}, nil
	case it.plate != nil:
		pl := *it.plate
		return func(c *image.RGBA) error {
			r := image.Rect(px(pl.Rect.X, dpi), px(pl.Rect.Y, dpi), px(pl.Rect.X+pl.Rect.W, dpi), px(pl.Rect.Y+pl.Rect.H, dpi))
			fill(c, r.Intersect(c.Bounds()), pl.Color)
			return nil
		}, nil
	}
	t := *it.text
	return func(c *image.RGBA) error {
		if t.Gen != 0 && t.Gen != text.Gen() {
			t.Box = text.Cached(t.Text, t.Style, t.Box.W)
		}
		// text.Draw отсчитывает от Rect.Min холста; у холста подгонки Min сдвинут, а текст задан в
		// координатах сцены.
		pxmm, o := float64(dpi)/25.4, c.Rect.Min
		text.Draw(c, t.Box, t.Style, t.X-float64(o.X)/pxmm, t.Y-float64(o.Y)/pxmm, t.Rot, dpi)
		return nil
	}, nil
}

// prepareSafe готовит фото по одному из трёх путей (design-rotation §3); паника декодера
// становится ошибкой: горутина без recover уронила бы весь процесс. nil без ошибки — рисовать нечего.
func prepareSafe(ctx context.Context, it item, canvas image.Rectangle, dpi int) (paint func(*image.RGBA) error, err error) {
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
	// Маска окна — после фото: фото за холстом (nil) её не строит (до 400 Мп у большого места).
	if _, quarter := quarterOf(ph.ImgRot); ph.Rot == 0 && quarter {
		cell, r, err := prepareFn(ctx, ph, dpi, clip)
		if err != nil || cell == nil {
			return nil, err
		}
		win, err := winOf(ctx, it, dpi)
		if err != nil {
			return nil, err
		}
		applyTone(cell, ph.Tone) // до маски формы: тон касается только картинки (068)
		fadeAlpha(cell, ph.Fade) // снаружи prepareFn: подмена в тестах его не обходит
		if hasShape(ph.Outline) || win != nil {
			// Маска — от рамки, а не от r: у «Целиком» r меньше рамки.
			var m *image.Alpha
			if hasShape(ph.Outline) {
				m = shapeMask(r, ph.Cell, 0, ph.Outline, float64(dpi)/25.4)
			} else {
				m = fullMask(r)
			}
			if win != nil {
				win.mulInto(m.Pix, m.Stride, r)
			}
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
	job, err := prepareRot(ctx, ph, dpi, clip)
	if err != nil || job == nil {
		return nil, err
	}
	if job.win, err = winOf(ctx, it, dpi); err != nil {
		return nil, err
	}
	return job.draw, nil
}

// paintSafe кладёт готовое на холст; паника — ошибка, как в prepareSafe. Паника в потоках
// пути 3 приходит ошибкой из inParallel.
func paintSafe(paint func(*image.RGBA) error, canvas *image.RGBA, it item) (err error) {
	name := baseName(it.ph.Path)
	switch {
	case it.art != nil: // файла нет: в тексте — ключ рисунка
		name = it.art.Kind
	case it.border != nil:
		name = borderName(*it.border)
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
	case it.border != nil:
		return "рамочки"
	}
	return "фото"
}
