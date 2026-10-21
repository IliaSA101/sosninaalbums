package render

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"os"

	"github.com/disintegration/imaging"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Рамочки мест (079 design §4). Встроенная — вектор gen.DrawFrame в px печати (поворот в
// матрице, без пересэмплирования); картинка — Lanczos в масштаб печати и gen.Nine на место
// (то же, что рисует /media/f). Окно фото (маска мягкого края и прозрачное окно картинки) —
// Photo.Win (borderwin.go).

// BorderDraw — рамочка места. Box — рамка места (мм листа, до поворота), Rot — вокруг её
// центра. Встроенная — Kind, Inks, Width, форма места (поля gen, не gen.FrameSpec: export
// собирает задание без импорта decor/gen, depguard); картинка — Pic, Path, Scale и срез Cut
// (как их зажал ClampBorder). Z — слой своего места: в стеке рамочка ложится над своим фото.
type BorderDraw struct {
	Box    layout.Rect
	Rot    float64
	Z      int
	Kind   string
	Inks   []string
	Width  float64
	Oval   bool
	Radius float64
	Pic    bool
	Path   string
	Scale  float64 // размер картинки; 0 — 1
	Cut    [4]float64
}

// Пределы рамочки-картинки (079-35, 079-40). Сторона больше decor.MaxFrameSide уменьшается при
// декоде: так в печать попадает картинка, загруженная и в «Фоны» (запись с фоном общая, путь — к
// исходнику), и картинка собранного альбома. Размер читается по заголовку до декода: PNG 15000²
// весит 886 КБ, а декодируется в 858 МБ.
const (
	maxBorderSide = 4096
	// maxBorderSrc — px исходника, как у /media/f (framePicMaxSrc): больше — отказ по заголовку.
	maxBorderSrc = 64_000_000
	// borderArea — px картинки после масштаба: больше — уменьшается (память; как на экране).
	borderArea = 20_000_000
	// borderBuf — предел маски окна и буфера повёрнутой рамочки, px: холст печати (maxCanvasPixels).
	// Место больше холста в печать не попадает; память обоих учтена (маска — Alpha, буфер — в
	// DecodeBudget на время укладки).
	borderBuf = maxCanvasPixels
	// borderPad — прозрачное поле буфера повёрнутой рамочки: край места сглаживается при повороте.
	borderPad = 2
)

// errBorderBig — рамочка-картинка больше предела по заголовку.
var errBorderBig = errors.New("картинка рамочки слишком велика (больше 64 Мп)")

// errBorderBuf — место рамочки или окна больше холста печати (такой лист не прошёл бы CheckSize).
func errBorderBuf(what string, r image.Rectangle) error {
	return fmt.Errorf("%s слишком велика для печати (%d×%d px): уменьшите DPI в настройках", what, r.Dx(), r.Dy())
}

// finiteBox — рамка конечная и с площадью.
func finiteBox(b layout.Rect, rot float64) bool {
	for _, v := range [...]float64{b.X, b.Y, b.W, b.H, rot} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return b.W > 0 && b.H > 0
}

// boxAffine — источник → px холста: пиксель (i, j) источника — (i − pad)/sx мм вдоль ширины места
// и (j − pad)/sy вдоль высоты (sx, sy — px источника на мм); место повёрнуто на rot вокруг своего
// центра, k — px холста на мм. Источник «мм» (встроенная) — sx = sy = 1, pad = 0.
func boxAffine(b layout.Rect, rot, k, sx, sy float64, pad int) f64.Aff3 {
	s, c := sincosDeg(rot)
	hw, hh := b.W/2, b.H/2
	cx, cy := b.X+hw, b.Y+hh
	p := float64(pad)
	u0, v0 := -p/sx-hw, -p/sy-hh // начало источника относительно центра места, мм
	return f64.Aff3{
		k * c / sx, -k * s / sy, k * (cx + c*u0 - s*v0),
		k * s / sx, k * c / sy, k * (cy + s*u0 + c*v0),
	}
}

// affBounds — габарит прямоугольника w×h источника под матрицей m, px, с запасом в 1 px.
func affBounds(m f64.Aff3, w, h float64) image.Rectangle {
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, uv := range [...][2]float64{{0, 0}, {w, 0}, {w, h}, {0, h}} {
		x := m[0]*uv[0] + m[1]*uv[1] + m[2]
		y := m[3]*uv[0] + m[4]*uv[1] + m[5]
		x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
	}
	return image.Rect(int(math.Floor(x0))-1, int(math.Floor(y0))-1, int(math.Ceil(x1))+1, int(math.Ceil(y1))+1)
}

// placePx — прямоугольник места в px холста (до поворота), по тем же px(), что ячейки фото; не
// пустой.
func placePx(b layout.Rect, dpi int) image.Rectangle {
	x0, y0 := px(b.X, dpi), px(b.Y, dpi)
	return image.Rect(x0, y0, max(px(b.X+b.W, dpi), x0+1), max(px(b.Y+b.H, dpi), y0+1))
}

// prepareBorderSafe — prepareBorder с паникой в ошибку: горутина без recover уронила бы процесс.
func prepareBorderSafe(ctx context.Context, it item, canvas image.Rectangle, dpi int) (paint func(*image.RGBA) error, err error) {
	b := *it.border
	defer func() {
		if p := recover(); p != nil {
			paint, err = nil, fmt.Errorf("%s: сбой при обработке рамочки: %v", borderName(b), p)
		}
	}()
	return prepareBorder(ctx, b, it.env, canvas, dpi)
}

func borderName(b BorderDraw) string {
	if b.Pic {
		return baseName(b.Path)
	}
	return b.Kind
}

// prepareBorder готовит рамочку: встроенная рисуется при укладке на холст, картинка — здесь
// декодируется и масштабируется (в потоке подготовки), на холст ложится по порядку. nil без
// ошибки — рисовать нечего.
func prepareBorder(ctx context.Context, b BorderDraw, env *borderEnv, canvas image.Rectangle, dpi int) (func(*image.RGBA) error, error) {
	if !finiteBox(b.Box, b.Rot) {
		return nil, nil
	}
	if env == nil {
		env = &borderEnv{}
	}
	k := float64(dpi) / 25.4
	if !b.Pic {
		return builtinPaint(b, k, canvas), nil
	}
	return picPaint(ctx, b, env, k, dpi, canvas)
}

// builtinPaint — встроенная рамочка: gen.DrawFrame в подхолст по габариту места под поворотом
// (при прозрачности gen держит слой размером с холст назначения, а не со всем лист).
func builtinPaint(b BorderDraw, k float64, canvas image.Rectangle) func(*image.RGBA) error {
	m := boxAffine(b.Box, b.Rot, k, 1, 1, 0)
	bb := affBounds(m, b.Box.W, b.Box.H).Intersect(canvas)
	if bb.Empty() {
		return nil
	}
	spec := gen.FrameSpec{Kind: b.Kind, Inks: b.Inks, Width: b.Width, W: b.Box.W, H: b.Box.H, Oval: b.Oval, Radius: b.Radius}
	return func(c *image.RGBA) error {
		if sub, ok := c.SubImage(bb).(*image.RGBA); ok {
			gen.DrawFrame(sub, m, spec)
		}
		return nil
	}
}

// picGeom — размеры картинки-рамочки под место: место и картинка после масштаба, толщины 9-среза.
type picGeom struct {
	pw, ph int     // место, px
	tw, th int     // картинка после масштаба, px
	sl     [4]int  // толщины рамки l, t, r, b в px картинки после масштаба
	dsl    [4]int  // они же на месте, px печати: по масштабу k, и когда картинка уменьшена пределом
	pxmm   float64 // px печати на мм места
}

// picGeometry считает размеры по файлу iw×ih px — те же формулы, что у picOverlay на экране.
// Картинка больше borderArea уменьшается, а толщины на месте остаются k·срез (079-40): рамка доходит
// до окна фото (BorderIns), кольца страницы между ними нет.
func picGeometry(b layout.Rect, scale float64, cut [4]float64, iw, ih, pw, ph int) picGeom {
	g := picGeom{pw: pw, ph: ph, pxmm: float64(pw) / b.W}
	k := min(b.W/float64(iw), b.H/float64(ih)) * gen.ClampPicScale(scale)
	tw, th := float64(iw)*k*g.pxmm, float64(ih)*k*g.pxmm
	c := gen.ClampCut(cut)
	capped := tw*th > borderArea
	if capped {
		g.dsl = gen.SlicePx(c, tw, th)
		f := math.Sqrt(borderArea / (tw * th))
		tw, th = tw*f, th*f
	}
	g.tw, g.th = max(1, int(math.Round(tw))), max(1, int(math.Round(th)))
	g.sl = gen.SlicePx(c, float64(g.tw), float64(g.th))
	if !capped {
		g.dsl = g.sl
	}
	return g
}

// picPaint — рамочка-картинка: масштаб и 9-срез. Прямое место — Nine прямо на холст (draw
// обрезает по холсту, буфера нет); повёрнутое — Nine в буфер с прозрачным полем и поворот. Буфер
// строится при укладке (по одному за раз, а не у каждого из параллельных элементов) и занимает
// место в DecodeBudget: до 1,6 ГБ у места во весь холст.
func picPaint(ctx context.Context, b BorderDraw, env *borderEnv, k float64, dpi int, canvas image.Rectangle) (func(*image.RGBA) error, error) {
	r := placePx(b.Box, dpi)
	rotated := b.Rot != 0
	buf := image.Rect(0, 0, r.Dx()+2*borderPad, r.Dy()+2*borderPad)
	if rotated && int64(buf.Dx())*int64(buf.Dy()) > borderBuf {
		return nil, fmt.Errorf("%s: %w", baseName(b.Path), errBorderBuf("рамочка", r))
	}
	if !rotated && r.Intersect(canvas).Empty() {
		return nil, nil
	}
	pic, err := env.picture(ctx, b.Path)
	if err != nil {
		return nil, photoErr(b.Path, err)
	}
	ob := pic.img.Bounds()
	g := picGeometry(b.Box, b.Scale, b.Cut, ob.Dx(), ob.Dy(), r.Dx(), r.Dy())
	scaled, err := env.scaled(pic, g.tw, g.th)
	if err != nil {
		return nil, photoErr(b.Path, err)
	}
	if !rotated {
		return func(c *image.RGBA) error {
			gen.NineTo(c, r, scaled, g.sl, g.dsl)
			return nil
		}, nil
	}
	m := boxAffine(b.Box, b.Rot, k, float64(g.pw)/b.Box.W, float64(g.ph)/b.Box.H, borderPad)
	bb := affBounds(m, float64(buf.Dx()), float64(buf.Dy())).Intersect(canvas)
	if bb.Empty() {
		return nil, nil
	}
	return func(c *image.RGBA) error {
		n := int64(buf.Dx()) * int64(buf.Dy())
		if err := imgfile.DecodeBudget.AcquireCtx(ctx, n); err != nil {
			return err
		}
		defer imgfile.DecodeBudget.Release(n)
		tmp := image.NewRGBA(buf)
		gen.NineTo(tmp, image.Rect(borderPad, borderPad, borderPad+g.pw, borderPad+g.ph), scaled, g.sl, g.dsl)
		if sub, ok := c.SubImage(bb).(*image.RGBA); ok {
			xdraw.BiLinear.Transform(sub, m, tmp, tmp.Bounds(), draw.Over, nil)
		}
		return nil
	}, nil
}

// checkBorderFile — размеры картинки рамочки по заголовку, до декода (079-35): больше
// maxBorderSrc — отказ. Дальше imgfile.OpenOriented проверит ещё раз на своём дескрипторе.
func checkBorderFile(path string) error {
	_, err := imgfile.FileDims(path, maxBorderSrc)
	if errors.Is(err, imgfile.ErrTooBig) {
		return errBorderBig
	}
	return err
}

// loadBorderPic читает картинку рамочки: заголовок, затем декод под бюджетом памяти; сторона
// больше maxBorderSide уменьшается до неё, пока бюджет ещё занят (079-40). Результат — NRGBA без
// привязки к бюджету (он отпущен сразу, в кэше картинка живёт до конца листа).
func loadBorderPic(ctx context.Context, path string) (*image.NRGBA, error) {
	if err := checkBorderFile(path); err != nil {
		return nil, err
	}
	img, release, err := imgfile.OpenOrientedCtx(ctx, path)
	if err != nil {
		return nil, err
	}
	defer release()
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if int64(w)*int64(h) > maxBorderSrc { // файл подменили между проверками
		return nil, errBorderBig
	}
	if long := max(w, h); long > maxBorderSide {
		return shrinkBorder(img, long), nil
	}
	if n, ok := img.(*image.NRGBA); ok && n.Rect.Min == (image.Point{}) {
		return n, nil
	}
	return imaging.Clone(img), nil
}

// shrinkBorder — картинка с длинной стороной long, уменьшенная до maxBorderSide (Lanczos, как
// уменьшение рамочки при загрузке в библиотеку).
func shrinkBorder(img image.Image, long int) *image.NRGBA {
	b := img.Bounds()
	k := float64(maxBorderSide) / float64(long)
	return imaging.Resize(img, max(1, int(math.Round(float64(b.Dx())*k))), max(1, int(math.Round(float64(b.Dy())*k))), imaging.Lanczos)
}

// stampOf — отпечаток файла для ключа кэша: заменённая картинка под тем же путём не берётся из кэша.
func stampOf(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", errors.New("не обычный файл")
	}
	return fmt.Sprintf("%s|%d|%d", path, fi.Size(), fi.ModTime().UnixNano()), nil
}
