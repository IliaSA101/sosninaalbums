package gen

import (
	"bytes"
	"embed"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io/fs"
	"math"
	"sync"
)

// Текстуры бумаги (research §10): серая плитка 2048 px (JPEG, CC0 ambientCG, список и sha256 —
// textures/sources.json) на 150 мм, рисуется чёрным с альфой 1 − яркость: на холсте —
// «умножение» на цвет страницы, в превью — прозрачная картинка. Генератор есть в сборке,
// только если его файл вшит (до O1 — нет: Known = false).

//go:embed textures
var textureFS embed.FS

// paperMM — сторона плитки текстуры в мм.
const paperMM = 150

// maxMaskPx — предел стороны маски плитки: 8192² — 64 МБ.
const maxMaskPx = 8192

// texture — плитка: маска 1 − яркость и её уменьшенные копии (по 2×), чтобы превью 12 dpi
// не мерцало от прореженных пикселей.
type texture struct {
	file string
	once sync.Once
	mips []*image.Alpha // [0] — исходник; nil — файла нет или не читается

	mu     sync.Mutex
	lastN  int
	lastMs *image.Alpha // последняя маска стороны lastN: плитки области одного размера
}

func (t *texture) ok() bool {
	_, err := fs.Stat(textureFS, "textures/"+t.file)
	return err == nil
}

func (t *texture) load() []*image.Alpha {
	t.once.Do(func() {
		data, err := textureFS.ReadFile("textures/" + t.file)
		if err != nil {
			return
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return
		}
		t.mips = mipChain(inverse(img))
	})
	return t.mips
}

// inverse — маска 255 − яркость (у JPEG в цвете — канал Y).
func inverse(img image.Image) *image.Alpha {
	b := img.Bounds()
	m := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			var g uint8
			switch v := img.(type) {
			case *image.Gray:
				g = v.Pix[v.PixOffset(b.Min.X+x, b.Min.Y+y)]
			case *image.YCbCr:
				g = v.Y[v.YOffset(b.Min.X+x, b.Min.Y+y)]
			default:
				g = color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.Gray).Y
			}
			m.Pix[y*m.Stride+x] = 255 - g
		}
	}
	return m
}

// mipChain — исходник и половинки 2×2 до 16 px.
func mipChain(m *image.Alpha) []*image.Alpha {
	out := []*image.Alpha{m}
	for m.Rect.Dx() >= 32 && m.Rect.Dy() >= 32 {
		w, h := m.Rect.Dx()/2, m.Rect.Dy()/2
		n := image.NewAlpha(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				i := 2*y*m.Stride + 2*x
				s := uint32(m.Pix[i]) + uint32(m.Pix[i+1]) + uint32(m.Pix[i+m.Stride]) + uint32(m.Pix[i+m.Stride+1])
				n.Pix[y*n.Stride+x] = uint8((s + 2) / 4) //nolint:gosec // среднее четырёх байт: 0..255
			}
		}
		out = append(out, n)
		m = n
	}
	return out
}

// mask — маска плитки стороной n px: билинейно из самой малой копии не меньше n.
func (t *texture) mask(n int) *image.Alpha {
	mips := t.load()
	if mips == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastN == n {
		return t.lastMs
	}
	src := mips[0]
	for _, m := range mips[1:] {
		if m.Rect.Dx() < n {
			break
		}
		src = m
	}
	t.lastN, t.lastMs = n, resizeAlpha(src, n)
	return t.lastMs
}

// resizeAlpha — билинейно в квадрат n×n по центрам пикселей, края — повтором (плитка
// бесшовна: research §10).
func resizeAlpha(src *image.Alpha, n int) *image.Alpha {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewAlpha(image.Rect(0, 0, n, n))
	xs := make([]int, 2*n)
	fx := make([]uint32, n)
	for x := range n {
		u := (float64(x)+0.5)*float64(sw)/float64(n) - 0.5
		x0 := int(math.Floor(u))
		fx[x] = uint32(math.Round((u - float64(x0)) * 256))
		xs[2*x], xs[2*x+1] = (x0+sw)%sw, (x0+1+sw)%sw
	}
	for y := range n {
		v := (float64(y)+0.5)*float64(sh)/float64(n) - 0.5
		y0 := int(math.Floor(v))
		fy := uint32(math.Round((v - float64(y0)) * 256))
		r0 := src.Pix[((y0+sh)%sh)*src.Stride:]
		r1 := src.Pix[((y0+1+sh)%sh)*src.Stride:]
		for x := range n {
			a, b := uint32(r0[xs[2*x]]), uint32(r0[xs[2*x+1]])
			c, d := uint32(r1[xs[2*x]]), uint32(r1[xs[2*x+1]])
			top := a*(256-fx[x]) + b*fx[x]
			bot := c*(256-fx[x]) + d*fx[x]
			dst.Pix[y*dst.Stride+x] = uint8((top*(256-fy) + bot*fy + 1<<15) >> 16) //nolint:gosec // смесь байт с весами 256²: 0..255
		}
	}
	return dst
}

// paper — текстура t плитками по paperMM от угла области. Плитка i занимает px
// [floor(xᵢ), floor(xᵢ₊₁)) — без щелей и без двойного перекрытия (оно темнило бы шов).
func paper(t *texture) func(c *canvas, s Spec) {
	return func(c *canvas, _ Spec) {
		if paperMM*c.M[0]+1 > maxMaskPx {
			// Маска плитки вышла бы сотнями МБ (превью с мелким w): рисуем слоем меньшего
			// разрешения и растягиваем — исходник 2048 px всё равно мельче. Без рекурсии: у
			// узкой обрезки слой может остаться того же масштаба, тогда маска просто зажата.
			c.SoftAt((maxMaskPx-2)/paperMM, t.tiles)
			return
		}
		t.tiles(c)
	}
}

// tiles — плитки маски по обрезке c.
func (t *texture) tiles(c *canvas) {
	k := c.M[0]
	side := paperMM * k
	m := t.mask(min(int(math.Ceil(side))+1, maxMaskPx))
	if m == nil {
		return
	}
	src := image.NewUniform(color.NRGBA{A: uint8(math.Round(255 * c.Alpha))})
	edge := func(o float64, i int) int { return int(math.Floor(o + float64(i)*side)) }
	for j := 0; edge(c.M[5], j) < c.Clip.Max.Y; j++ {
		for i := 0; edge(c.M[2], i) < c.Clip.Max.X; i++ {
			x0, y0 := edge(c.M[2], i), edge(c.M[5], j)
			r := image.Rect(x0, y0, edge(c.M[2], i+1), edge(c.M[5], j+1)).Intersect(c.Clip)
			if !r.Empty() {
				draw.DrawMask(c.Dst, r, src, image.Point{}, m, r.Min.Sub(image.Pt(x0, y0)), draw.Over)
			}
		}
	}
}
