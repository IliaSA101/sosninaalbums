// Package imgsig считает подпись кадра по миниатюре: два перцептивных хеша (dHash и pHash),
// цветность и резкость. Расстояние между подписями (сумма Хэмминга двух хешей) отличает почти
// дубли серии от разных кадров, цветность — исходно чёрно-белые кадры (спека 078).
//
// Пакет — лист: только стандартная библиотека. Подпись всегда считается по декодированной
// миниатюре 400 px, а не по картинке до сжатия: так она не зависит от того, как миниатюра
// появилась (новая или из старого кэша), и пороги ведут себя одинаково (решение 078-2).
package imgsig

import (
	"encoding/binary"
	"image"
	"math"
	"math/bits"
	"sort"
)

// Пороги расстояния для уровней «похожих» (решение 078-3, замер — research §2): на 4950 парах
// разных кадров Dist не опускался ниже 38, на сериях — 0…24 у сдвига, до 36 у смены зума.
const (
	DistDup     = 12 // дубли: экспозиция, пересжатие, сдвиг до 3 %
	DistNear    = 24 // очень похожие: сдвиг, лёгкое приближение
	DistSimilar = 34 // похожие: другой зум, тот же момент
)

// ColorGray — цветность ниже этого значения считается исходно чёрно-белой (у серых кадров
// 0…2 после сжатия, у самого блёклого цветного — заметно выше).
const ColorGray = 4

const (
	gridD     = 9  // dHash: 9×8 по яркости
	gridP     = 32 // pHash: 32×32 → DCT → 8×8 низких частот
	lowP      = 8
	sharpSide = 96 // резкость: длинная сторона сетки
	pEps      = 1e-6
)

// Sig — подпись кадра. Нулевое значение (Ok = false) — «подписи нет».
type Sig struct {
	D, P  uint64  // dHash 9×8 и pHash 32→8×8 по яркости миниатюры
	Color float32 // цветность Hasler–Süsstrunk на 32×32; < ColorGray — исходно ч/б
	Sharp float32 // дисперсия лапласиана яркости на 96 px
	Ok    bool    // false — подписи нет (не посчитана или не прочиталась)
}

// Dist — сумма расстояний Хэмминга dHash и pHash, 0…128. Подписи без Ok не сравнивать:
// у них хеши нулевые, расстояние было бы ложно малым.
func Dist(a, b Sig) int {
	return bits.OnesCount64(a.D^b.D) + bits.OnesCount64(a.P^b.P)
}

// cosT[u][x] = cos((2x+1)·u·π/64): таблица для DCT-II по 32 отсчётам, нужны 8 низких частот.
var cosT = func() (t [lowP][gridP]float64) {
	for u := range lowP {
		for x := range gridP {
			t[u][x] = math.Cos(float64((2*x+1)*u) * math.Pi / (2 * gridP))
		}
	}
	return t
}()

// Of считает подпись картинки. Пустая картинка — подписи нет (Ok = false).
func Of(img image.Image) Sig {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return Sig{}
	}
	s := Sig{Ok: true}

	d := gridOf(img, gridD, 8, false)
	for y := range 8 {
		for x := range 8 {
			if d.y[y*gridD+x] < d.y[y*gridD+x+1] {
				s.D |= 1 << (y*8 + x)
			}
		}
	}

	g := gridOf(img, gridP, gridP, true)
	s.P = pHash(g.y)
	s.Color = float32(colorful(g))

	lw, lh := sharpGrid(b.Dx(), b.Dy())
	s.Sharp = float32(laplacianVar(gridOf(img, lw, lh, false).y, lw, lh))
	return s
}

// pHash — DCT-II 32×32 по яркости (раздельно: строки, затем столбцы), 8×8 низких частот
// без постоянной составляющей; бит = коэффициент больше медианы остальных 63.
func pHash(g []float64) uint64 {
	var tmp [gridP][lowP]float64
	for y := range gridP {
		row := g[y*gridP : (y+1)*gridP]
		for v := range lowP {
			sum := 0.0
			for x, c := range cosT[v] {
				sum += row[x] * c
			}
			tmp[y][v] = sum
		}
	}
	var coef [lowP * lowP]float64
	for u := range lowP {
		for v := range lowP {
			sum := 0.0
			for y, c := range cosT[u] {
				sum += tmp[y][v] * c
			}
			coef[u*lowP+v] = sum
		}
	}
	vals := append([]float64(nil), coef[1:]...)
	sort.Float64s(vals)
	med := vals[len(vals)/2]
	var h uint64
	for i := 1; i < len(coef); i++ {
		// pEps: у однотонной картинки коэффициенты — шум вычислений ~1e-13, хеш должен быть стабилен.
		if coef[i] > med+pEps {
			h |= 1 << i
		}
	}
	return h
}

// colorful — цветность Hasler–Süsstrunk: разброс и сдвиг средних в каналах R−G и (R+G)/2−B.
func colorful(g planes) float64 {
	rg := make([]float64, len(g.y))
	yb := make([]float64, len(g.y))
	for i := range g.y {
		y, cb, cr := g.y[i], g.cb[i]-128, g.cr[i]-128
		r := y + 1.402*cr
		gg := y - 0.344136*cb - 0.714136*cr
		b := y + 1.772*cb
		rg[i] = r - gg
		yb[i] = 0.5*(r+gg) - b
	}
	mr, vr := meanVar(rg)
	my, vy := meanVar(yb)
	return math.Sqrt(vr+vy) + 0.3*math.Sqrt(mr*mr+my*my)
}

// sharpGrid — размер сетки резкости: длинная сторона 96 (но не больше самой картинки).
func sharpGrid(w, h int) (int, int) {
	long := max(w, h)
	if long <= sharpSide {
		return w, h
	}
	return max(1, w*sharpSide/long), max(1, h*sharpSide/long)
}

// laplacianVar — дисперсия 4-соседнего лапласиана яркости; на сетке тоньше 3 px — 0.
func laplacianVar(y []float64, w, h int) float64 {
	if w < 3 || h < 3 {
		return 0
	}
	lap := make([]float64, 0, (w-2)*(h-2))
	for j := 1; j < h-1; j++ {
		for i := 1; i < w-1; i++ {
			k := j*w + i
			lap = append(lap, 4*y[k]-y[k-1]-y[k+1]-y[k-w]-y[k+w])
		}
	}
	_, v := meanVar(lap)
	return v
}

func meanVar(v []float64) (mean, variance float64) {
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	for _, x := range v {
		variance += (x - mean) * (x - mean)
	}
	return mean, variance / float64(len(v))
}

// FileSize — размер записи подписи в файле: версия, D, P, Color, Sharp.
const FileSize = 1 + 8 + 8 + 4 + 4

const fileVersion = 1

// Bytes — запись подписи для файла кэша (little-endian, FileSize байт).
func (s Sig) Bytes() []byte {
	b := make([]byte, FileSize)
	b[0] = fileVersion
	binary.LittleEndian.PutUint64(b[1:], s.D)
	binary.LittleEndian.PutUint64(b[9:], s.P)
	binary.LittleEndian.PutUint32(b[17:], math.Float32bits(s.Color))
	binary.LittleEndian.PutUint32(b[21:], math.Float32bits(s.Sharp))
	return b
}

// Parse читает запись Bytes. Чужой размер, версия, NaN, бесконечность и отрицательные значения
// (цветность и дисперсия не бывают меньше нуля) — подписи нет, вызывающий пересчитает.
func Parse(b []byte) (Sig, bool) {
	if len(b) != FileSize || b[0] != fileVersion {
		return Sig{}, false
	}
	s := Sig{
		D:     binary.LittleEndian.Uint64(b[1:]),
		P:     binary.LittleEndian.Uint64(b[9:]),
		Color: math.Float32frombits(binary.LittleEndian.Uint32(b[17:])),
		Sharp: math.Float32frombits(binary.LittleEndian.Uint32(b[21:])),
		Ok:    true,
	}
	if !okFloat(s.Color) || !okFloat(s.Sharp) {
		return Sig{}, false
	}
	return s, true
}

func okFloat(f float32) bool {
	v := float64(f)
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0
}
