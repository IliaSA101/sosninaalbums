// Пакет colorspace пересчитывает цвета Adobe RGB (1998) в sRGB.
//
// Снимки в Adobe RGB без ICC-профиля в sRGB-программах выглядят тусклыми,
// поэтому пересчёт делается сразу после декода, до любого ресемплинга: тогда
// миниатюры, превью и экспорт получают одинаковые цвета.
package colorspace

import (
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
)

// adobeGamma — гамма из самого профиля Adobe RGB (1998): 563/256 = 2.19921875.
const adobeGamma = 563.0 / 256

// adobeToSRGB — матрица «линейный Adobe → линейный sRGB» (оба D65):
// M(XYZ→sRGB)·M(Adobe→XYZ), строками.
var adobeToSRGB = func() [9]float64 {
	a := [9]float64{0.5767309, 0.1855540, 0.1881852, 0.2973769, 0.6273491, 0.0752741, 0.0270343, 0.0706872, 0.9911085}
	s := [9]float64{3.2404542, -1.5371385, -0.4985314, -0.9692660, 1.8760108, 0.0415560, 0.0556434, -0.2040259, 1.0572252}
	var m [9]float64
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				m[i*3+j] += s[i*3+k] * a[k*3+j]
			}
		}
	}
	return m
}()

var (
	decLUT [256]int32   // 8 бит Adobe → линейное 0…65535
	encLUT [65536]uint8 // линейное 16 бит → 8 бит sRGB
	mFix   [9]int32     // матрица ×2^14: сумма ≤ 1.4·2^14·65535 < 2^31, int32 хватает
)

func init() {
	for i := range decLUT {
		decLUT[i] = int32(math.Round(math.Pow(float64(i)/255, adobeGamma) * 65535))
	}
	for i := range encLUT {
		encLUT[i] = uint8(math.Round(srgbEncode(float64(i)/65535) * 255))
	}
	for i, v := range adobeToSRGB {
		mFix[i] = int32(math.Round(v * (1 << 14)))
	}
}

// srgbEncode — кусочная кривая sRGB: линейный участок у нуля, дальше степень.
func srgbEncode(l float64) float64 {
	if l <= 0.0031308 {
		return 12.92 * l
	}
	return 1.055*math.Pow(l, 1/2.4) - 0.055
}

// clamp16 снимает фиксированную точку и обрезает цвета вне охвата sRGB
// (относительная колориметрия без компенсации чёрной точки, как в LittleCMS).
func clamp16(v int32) int32 {
	v >>= 14
	if v < 0 {
		return 0
	}
	if v > 65535 {
		return 65535
	}
	return v
}

// px пересчитывает один пиксель.
func px(r, g, b uint8) (uint8, uint8, uint8) {
	lr, lg, lb := decLUT[r], decLUT[g], decLUT[b]
	R := clamp16(mFix[0]*lr + mFix[1]*lg + mFix[2]*lb)
	G := clamp16(mFix[3]*lr + mFix[4]*lg + mFix[5]*lb)
	B := clamp16(mFix[6]*lr + mFix[7]*lg + mFix[8]*lb)
	return encLUT[R], encLUT[G], encLUT[B]
}

// refPx — эталон во float без таблиц, для проверки LUT в тестах.
func refPx(r, g, b uint8) (uint8, uint8, uint8) {
	l := [3]float64{
		math.Pow(float64(r)/255, adobeGamma),
		math.Pow(float64(g)/255, adobeGamma),
		math.Pow(float64(b)/255, adobeGamma),
	}
	var o [3]uint8
	for i := range 3 {
		v := adobeToSRGB[i*3]*l[0] + adobeToSRGB[i*3+1]*l[1] + adobeToSRGB[i*3+2]*l[2]
		v = math.Max(0, math.Min(1, v))
		o[i] = uint8(math.Round(srgbEncode(v) * 255))
	}
	return o[0], o[1], o[2]
}

// rows делит h строк между горутинами по числу ядер.
func rows(h int, f func(y0, y1 int)) {
	if h <= 0 {
		return
	}
	n := runtime.NumCPU()
	step := (h + n - 1) / n
	var wg sync.WaitGroup
	for y := 0; y < h; y += step {
		y0, y1 := y, min(y+step, h)
		wg.Go(func() { f(y0, y1) })
	}
	wg.Wait()
}

// AdobeToSRGB возвращает картинку в sRGB. Быстрые пути: *image.NRGBA
// пересчитывается на месте (возвращается тот же указатель), *image.YCbCr
// переводится в новое NRGBA сразу с пересчётом; остальное — через draw.
func AdobeToSRGB(img image.Image) *image.NRGBA {
	switch m := img.(type) {
	case *image.NRGBA:
		convertNRGBA(m)
		return m
	case *image.YCbCr:
		return fromYCbCr(m)
	}
	b := img.Bounds()
	dst := image.NewNRGBA(b)
	draw.Draw(dst, b, img, b.Min, draw.Src)
	convertNRGBA(dst)
	return dst
}

// convertNRGBA пересчитывает цвета на месте, альфа не трогается.
func convertNRGBA(m *image.NRGBA) {
	b := m.Bounds()
	rows(b.Dy(), func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			row := m.Pix[y*m.Stride : y*m.Stride+b.Dx()*4]
			for i := 0; i+3 < len(row); i += 4 {
				row[i], row[i+1], row[i+2] = px(row[i], row[i+1], row[i+2])
			}
		}
	})
}

// fromYCbCr переводит YCbCr в NRGBA и пересчитывает цвет за один проход:
// промежуточное NRGBA не нужно. Формулы YCbCr→RGB те же, что в image/color.
func fromYCbCr(src *image.YCbCr) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(b)
	rows(b.Dy(), func(y0, y1 int) {
		for y := b.Min.Y + y0; y < b.Min.Y+y1; y++ {
			o := (y - b.Min.Y) * dst.Stride
			for x := b.Min.X; x < b.Max.X; x++ {
				yy := int32(src.Y[src.YOffset(x, y)]) * 0x10101
				ci := src.COffset(x, y)
				cb := int32(src.Cb[ci]) - 128
				cr := int32(src.Cr[ci]) - 128
				r8 := clamp8(yy + 91881*cr)
				g8 := clamp8(yy - 22554*cb - 46802*cr)
				b8 := clamp8(yy + 116130*cb)
				dst.Pix[o], dst.Pix[o+1], dst.Pix[o+2] = px(r8, g8, b8)
				dst.Pix[o+3] = 255
				o += 4
			}
		}
	})
	return dst
}

// clamp8 берёт байт 8.16-значения с насыщением (как image/color).
func clamp8(v int32) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 0xffffff:
		return 255
	}
	return uint8(v >> 16) //nolint:gosec // v в [0, 0xffffff] — сдвиг даёт 0..255
}
