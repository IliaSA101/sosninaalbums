package render

import (
	"fmt"
	"image"
	"math"
	"sync"

	"github.com/disintegration/imaging"
	"golang.org/x/image/math/f64"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Окно фото в рамочке (079 design §4). Маска окна лежит в осях места (до поворота): мягкий край
// встроенной (gen.FrameMask) или прозрачное окно картинки (gen.WindowMask, растянутое
// gen.StretchWindow на окно, которое считает модель). Фото рисуется в окне, маска умножается на
// маску формы во всех трёх путях (прямой, blitMasked, повёрнутый).

// WinMask — маска окна фото места. Box — рамка места (мм листа, до поворота), Rot — поворот
// места вокруг её центра (градусы по часовой; не тот, что сложен в fold: маска знает исходный).
// Мягкий край — Kind (Mask-вид gen), Width и форма места Oval/Radius; прозрачное окно картинки —
// Path, Scale и срез Cut. Что-то одно: Path != "" — картинка.
type WinMask struct {
	Box    layout.Rect
	Rot    float64
	Kind   string
	Width  float64
	Oval   bool
	Radius float64
	Path   string
	Scale  float64
	Cut    [4]float64
}

// FrameHasMask — вид встроенной рамочки меняет маску фото (мягкий край), а не рисует поверх.
// Экспорт решает по нему, нужна ли Photo.Win: render.gen ему недоступен (depguard).
func FrameHasMask(kind string) bool {
	m, ok := gen.FrameLookup(kind)
	return ok && m.Mask
}

// memoCap — записей в каждом кэше листа: рамочка одной картинки на всех местах — один декод и одно
// уменьшение; чужие размеры вытесняют старые.
const memoCap = 2

// memo — кэш одного вида по строковому ключу: вычисление одной записи идёт один раз, остальные
// потоки ждут его; самая старая запись вытесняется. Ошибка запоминается вместе со значением.
type memo[V any] struct {
	mu   sync.Mutex
	m    map[string]*memoEntry[V]
	keys []string
}

type memoEntry[V any] struct {
	once sync.Once
	v    V
	err  error
}

func (c *memo[V]) get(key string, build func() (V, error)) (V, error) {
	c.mu.Lock()
	e, ok := c.m[key]
	if !ok {
		if c.m == nil {
			c.m = map[string]*memoEntry[V]{}
		}
		if len(c.keys) >= memoCap {
			delete(c.m, c.keys[0])
			c.keys = c.keys[1:]
		}
		e = &memoEntry[V]{}
		c.m[key] = e
		c.keys = append(c.keys, key)
	}
	c.mu.Unlock()
	e.once.Do(func() {
		defer func() {
			if p := recover(); p != nil {
				e.err = fmt.Errorf("сбой: %v", p)
			}
		}()
		e.v, e.err = build()
	})
	return e.v, e.err
}

// borderEnv — кэши листа: декод картинки рамочки, её масштаб под место, окно и готовая маска.
// Живёт одну отрисовку листа (Render) и пропадает вместе с ней: без устаревших записей и памяти,
// занятой после выгрузки.
type borderEnv struct {
	pics   memo[*borderPic]
	scale  memo[*image.NRGBA]
	window memo[*image.Alpha]
	alpha  memo[*image.Alpha]
}

// borderPic — картинка рамочки: оригинал (≤ 4096 px по стороне) и отпечаток файла.
type borderPic struct {
	stamp string
	img   *image.NRGBA
}

func (e *borderEnv) picture(path string) (*borderPic, error) {
	st, err := stampOf(path)
	if err != nil {
		return nil, err
	}
	return e.pics.get(st, func() (*borderPic, error) {
		img, err := loadBorderPic(path)
		if err != nil {
			return nil, err
		}
		return &borderPic{stamp: st, img: img}, nil
	})
}

// scaled — картинка в w×h px (Lanczos); того же размера — сам оригинал.
func (e *borderEnv) scaled(p *borderPic, w, h int) (*image.NRGBA, error) {
	if b := p.img.Bounds(); b.Dx() == w && b.Dy() == h {
		return p.img, nil
	}
	return e.scale.get(fmt.Sprintf("%s|%d|%d", p.stamp, w, h), func() (*image.NRGBA, error) {
		return imaging.Resize(p.img, w, h, imaging.Lanczos), nil
	})
}

// winSampler отдаёт покрытие маски окна в пикселях холста.
type winSampler struct {
	a        *image.Alpha // маска в осях места, px: размер места в px печати
	org      image.Point  // пиксель холста, где лежит a(0, 0), когда место не повёрнуто
	box      layout.Rect
	rot      float64
	k        float64 // px холста на мм
	sx, sy   float64 // px маски на мм
	rotating bool
}

// sampler строит выборку маски w при dpi; nil — маски нет (вид без маски, окно не нашлось).
func (e *borderEnv) sampler(w *WinMask, dpi int) (*winSampler, error) {
	if w == nil || !finiteBox(w.Box, w.Rot) {
		return nil, nil
	}
	r := placePx(w.Box, dpi)
	if int64(r.Dx())*int64(r.Dy()) > borderBuf {
		return nil, errBorderBuf("маска окна", r)
	}
	a, err := e.winAlpha(w, r.Dx(), r.Dy())
	if err != nil || a == nil {
		return nil, err
	}
	return &winSampler{a: a, org: r.Min, box: w.Box, rot: w.Rot, k: float64(dpi) / 25.4,
		sx: float64(r.Dx()) / w.Box.W, sy: float64(r.Dy()) / w.Box.H, rotating: w.Rot != 0}, nil
}

// winAlpha — маска окна в осях места размером pw×ph px. Мягкий край — gen.FrameMask; картинка —
// прозрачная область (gen.WindowMask по срезу) растянута на окно (gen.StretchWindowAlpha), которое
// считает модель: отступы k·срез под предел 0,7 стороны (gen.ClampInsets), как у /media/f.
func (e *borderEnv) winAlpha(w *WinMask, pw, ph int) (*image.Alpha, error) {
	if w.Path == "" {
		if !FrameHasMask(w.Kind) {
			return nil, nil
		}
		key := fmt.Sprintf("m|%s|%v|%v|%v|%v|%v|%d|%d", w.Kind, w.Width, w.Oval, w.Radius, w.Box.W, w.Box.H, pw, ph)
		return e.alpha.get(key, func() (*image.Alpha, error) {
			a := image.NewAlpha(image.Rect(0, 0, pw, ph))
			spec := gen.FrameSpec{Kind: w.Kind, Width: w.Width, W: w.Box.W, H: w.Box.H, Oval: w.Oval, Radius: w.Radius}
			gen.FrameMask(a, f64.Aff3{float64(pw) / w.Box.W, 0, 0, 0, float64(ph) / w.Box.H, 0}, spec)
			return a, nil
		})
	}
	pic, err := e.picture(w.Path)
	if err != nil {
		return nil, photoErr(w.Path, err)
	}
	cut := gen.ClampCut(w.Cut)
	key := fmt.Sprintf("p|%s|%v|%v|%v|%v|%d|%d", pic.stamp, cut, gen.ClampPicScale(w.Scale), w.Box.W, w.Box.H, pw, ph)
	return e.alpha.get(key, func() (*image.Alpha, error) {
		wm, err := e.window.get(fmt.Sprintf("%s|%v", pic.stamp, cut), func() (*image.Alpha, error) {
			return gen.WindowMask(pic.img, cut), nil
		})
		if err != nil || wm == nil {
			return nil, err
		}
		ob := pic.img.Bounds()
		iw, ih := float64(ob.Dx()), float64(ob.Dy())
		k := min(w.Box.W/iw, w.Box.H/ih) * gen.ClampPicScale(w.Scale)
		ins := gen.ClampInsets([4]float64{k * cut[0] * iw, k * cut[1] * ih, k * cut[2] * iw, k * cut[3] * ih}, w.Box.W, w.Box.H)
		pxmm := float64(pw) / w.Box.W
		win := image.Rect(int(math.Round(ins[0]*pxmm)), int(math.Round(ins[1]*pxmm)),
			pw-int(math.Round(ins[2]*pxmm)), ph-int(math.Round(ins[3]*pxmm)))
		return gen.StretchWindowAlpha(wm, cut, image.Pt(pw, ph), win), nil // сразу Alpha: без RGBA в 4 раза больше
	})
}

// mulInto умножает маску dst (строки по stride байт, пиксели прямоугольника r холста) на покрытие
// окна. Прямое место — пиксель в пиксель (сетка маски совпадает с px() ячеек); повёрнутое —
// билинейная выборка: точка холста → оси места.
func (s *winSampler) mulInto(dst []uint8, stride int, r image.Rectangle) {
	if !s.rotating {
		s.mulDirect(dst, stride, r)
		return
	}
	sn, cs := sincosDeg(s.rot)
	cx, cy := center(s.box)
	hw, hh := s.box.W/2, s.box.H/2
	w, h := s.a.Rect.Dx(), s.a.Rect.Dy()
	at := func(i, j int) float64 {
		if i < 0 || j < 0 || i >= w || j >= h {
			return 0
		}
		return float64(s.a.Pix[j*s.a.Stride+i])
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := dst[(y-r.Min.Y)*stride:]
		Y := (float64(y)+0.5)/s.k - cy
		for x := r.Min.X; x < r.Max.X; x++ {
			X := (float64(x)+0.5)/s.k - cx
			mx := (X*cs+Y*sn+hw)*s.sx - 0.5
			my := (-X*sn+Y*cs+hh)*s.sy - 0.5
			i, j := int(math.Floor(mx)), int(math.Floor(my))
			fx, fy := mx-float64(i), my-float64(j)
			cov := (at(i, j)*(1-fx)+at(i+1, j)*fx)*(1-fy) + (at(i, j+1)*(1-fx)+at(i+1, j+1)*fx)*fy
			if a := uint32(cov + 0.5); a < 255 {
				row[x-r.Min.X] = mix(row[x-r.Min.X], 0, a)
			}
		}
	}
}

func (s *winSampler) mulDirect(dst []uint8, stride int, r image.Rectangle) {
	w, h := s.a.Rect.Dx(), s.a.Rect.Dy()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := dst[(y-r.Min.Y)*stride:]
		j := y - s.org.Y
		for x := r.Min.X; x < r.Max.X; x++ {
			i := x - s.org.X
			var a uint32
			if i >= 0 && j >= 0 && i < w && j < h {
				a = uint32(s.a.Pix[j*s.a.Stride+i])
			}
			if a < 255 {
				row[x-r.Min.X] = mix(row[x-r.Min.X], 0, a)
			}
		}
	}
}

// winOf — выборка маски окна фото item; nil — маски нет. Кэши берутся у листа (item.env), вне
// Render (тесты) — свои.
func winOf(it item, dpi int) (*winSampler, error) {
	if it.ph.Win == nil {
		return nil, nil
	}
	env := it.env
	if env == nil {
		env = &borderEnv{}
	}
	s, err := env.sampler(it.ph.Win, dpi)
	return s, photoErr(it.ph.Path, err)
}

// fullMask — маска из 255 по прямоугольнику r (с нуля), к ней добавляется окно.
func fullMask(r image.Rectangle) *image.Alpha {
	m := image.NewAlpha(image.Rect(0, 0, r.Dx(), r.Dy()))
	for i := range m.Pix {
		m.Pix[i] = 0xff
	}
	return m
}
