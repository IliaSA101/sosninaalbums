package colorspace

import (
	"encoding/binary"
	"errors"
	"math"
)

// Пределы разбора недоверенного профиля (design 119 §4).
const (
	iccMaxTags   = 100     // записей в таблице тегов
	iccMaxCurve  = 1 << 16 // точек в curv: с запасом к реальным 1024–4096
	iccWhiteTol  = 0.05    // сумма колорантов против белой D50
	iccMinDet    = 1e-3    // вырожденная матрица колорантов
	iccMaxCoef   = 8.0     // модуль элемента итоговой матрицы
	iccMaxGamma  = 10.0    // гамма кривой
	iccHeaderLen = 132     // заголовок профиля + число тегов
	iccEntryLen  = 12      // запись таблицы тегов
	srgbCurveTol = 2       // допуск SRGBCurves в уровнях (119-14)
)

var (
	errNotMatrix = errors.New("colorspace: не матрично-кривой RGB-профиль")
	errLUT       = errors.New("colorspace: LUT-профиль")
	errWhite     = errors.New("colorspace: колоранты не дают белого D50")
	errMatrix    = errors.New("colorspace: матрица пересчёта вне пределов")
	errCurve     = errors.New("colorspace: непригодная кривая")
)

// d50 — белая точка PCS профилей ICC.
var d50 = [3]float64{0.9642, 1, 0.8249}

// srgbD50 — колоранты sRGB (Rec. 709, D65), адаптированные к D50 по Брэдфорду во float64,
// как у встроенного sRGB в LittleCMS; столбцы — колоранты.
var srgbD50 = func() [9]float64 {
	prim := [3][2]float64{{0.64, 0.33}, {0.30, 0.60}, {0.15, 0.06}}
	w := xyOf(0.3127, 0.3290)
	var p [9]float64
	for i, xy := range prim {
		c := xyOf(xy[0], xy[1])
		for k := range 3 {
			p[k*3+i] = c[k]
		}
	}
	s := mulv(inv(p), w) // яркости колорантов, дающие белый D65
	var m [9]float64
	for k := range 3 {
		for i := range 3 {
			m[k*3+i] = p[k*3+i] * s[i]
		}
	}
	br := [9]float64{0.8951, 0.2664, -0.1614, -0.7502, 1.7135, 0.0367, 0.0389, -0.0685, 1.0296}
	ws, wd := mulv(br, w), mulv(br, d50)
	d := [9]float64{wd[0] / ws[0], 0, 0, 0, wd[1] / ws[1], 0, 0, 0, wd[2] / ws[2]}
	return mul(mul(inv(br), mul(d, br)), m)
}()

func xyOf(x, y float64) [3]float64 { return [3]float64{x / y, 1, (1 - x - y) / y} }

// mul — произведение матриц 3×3 (строками).
func mul(a, b [9]float64) (m [9]float64) {
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				m[i*3+j] += a[i*3+k] * b[k*3+j]
			}
		}
	}
	return m
}

func mulv(a [9]float64, v [3]float64) (o [3]float64) {
	for i := range 3 {
		o[i] = a[i*3]*v[0] + a[i*3+1]*v[1] + a[i*3+2]*v[2]
	}
	return o
}

func det(a [9]float64) float64 {
	return a[0]*(a[4]*a[8]-a[5]*a[7]) - a[1]*(a[3]*a[8]-a[5]*a[6]) + a[2]*(a[3]*a[7]-a[4]*a[6])
}

// inv — обратная матрица; вызывающий проверил, что det не 0.
func inv(a [9]float64) [9]float64 {
	d := det(a)
	return [9]float64{
		(a[4]*a[8] - a[5]*a[7]) / d, (a[2]*a[7] - a[1]*a[8]) / d, (a[1]*a[5] - a[2]*a[4]) / d,
		(a[5]*a[6] - a[3]*a[8]) / d, (a[0]*a[8] - a[2]*a[6]) / d, (a[2]*a[3] - a[0]*a[5]) / d,
		(a[3]*a[7] - a[4]*a[6]) / d, (a[1]*a[6] - a[0]*a[7]) / d, (a[0]*a[4] - a[1]*a[3]) / d,
	}
}

// S15 читает s15Fixed16 из первых 4 байт b.
func S15(b []byte) float64 {
	return float64(int32(binary.BigEndian.Uint32(b))) / 65536 //nolint:gosec // битовое переинтерпретирование знака
}

// ICCTags собирает таблицу тегов; теги за длиной p пропускаются, повтор — первый. Длина p ≥ 132.
func ICCTags(p []byte) map[string][]byte {
	t := map[string][]byte{}
	n := min(binary.BigEndian.Uint32(p[128:]), iccMaxTags)
	for i := range int(n) {
		e := iccHeaderLen + iccEntryLen*i
		if e+iccEntryLen > len(p) {
			break
		}
		off, sz := uint64(binary.BigEndian.Uint32(p[e+4:])), uint64(binary.BigEndian.Uint32(p[e+8:]))
		if off+sz > uint64(len(p)) {
			continue
		}
		if sig := string(p[e : e+4]); t[sig] == nil {
			t[sig] = p[off : off+sz]
		}
	}
	return t
}

// ParseICC строит пересчёт в sRGB по матрично-кривому RGB-профилю (колоранты + три кривые).
// Любой мусор и профили с таблицами цвета (A2B*) — ошибка, паники нет: p недоверенный.
func ParseICC(p []byte) (*Xform, error) {
	if len(p) < iccHeaderLen || string(p[36:40]) != "acsp" || string(p[16:20]) != "RGB " || string(p[20:24]) != "XYZ " {
		return nil, errNotMatrix
	}
	switch string(p[12:16]) {
	case "link", "abst", "nmcl":
		return nil, errNotMatrix
	}
	tags := ICCTags(p)
	for _, s := range []string{"A2B0", "A2B1", "A2B2", "D2B0"} {
		if tags[s] != nil {
			return nil, errLUT
		}
	}
	m, err := iccMatrix(tags)
	if err != nil {
		return nil, err
	}
	x := &Xform{}
	x.setMatrix(m)
	for c, s := range []string{"rTRC", "gTRC", "bTRC"} {
		f, err := parseCurve(tags[s])
		if err != nil {
			return nil, err
		}
		x.cur[c] = f
		for i := range 256 {
			x.dec[c][i] = int32(math.Round(unit(f(float64(i)/255)) * 65535)) //nolint:gosec // 0…65535
		}
	}
	return x, nil
}

// iccMatrix: колоранты профиля (уже D50) → матрица «линейный профиль → линейный sRGB».
func iccMatrix(tags map[string][]byte) ([9]float64, error) {
	var c [9]float64 // столбцы — колоранты
	for i, s := range []string{"rXYZ", "gXYZ", "bXYZ"} {
		v := tags[s]
		if len(v) < 20 || string(v[:4]) != "XYZ " {
			return c, errNotMatrix
		}
		for k := range 3 {
			c[k*3+i] = S15(v[8+4*k:])
		}
	}
	w := mulv(c, [3]float64{1, 1, 1})
	for k, d := range d50 {
		if math.Abs(w[k]-d) > iccWhiteTol {
			return c, errWhite
		}
	}
	if math.Abs(det(c)) < iccMinDet {
		return c, errWhite
	}
	m := mul(inv(srgbD50), c)
	for _, v := range m {
		if math.IsNaN(v) || math.Abs(v) > iccMaxCoef {
			return m, errMatrix
		}
	}
	return m, nil
}

// unit зажимает значение кривой в 0…1; NaN и бесконечность дают 0.
func unit(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Max(0, math.Min(1, v))
}

// pow — степень без NaN: неположительное основание даёт 0.
func pow(v, g float64) float64 {
	if v <= 0 {
		return 0
	}
	return math.Pow(v, g)
}

// parseCurve разбирает curv и para (ICC.1 10.6, 10.18): значение канала 0…1 → линейное.
func parseCurve(t []byte) (func(float64) float64, error) {
	if len(t) < 12 {
		return nil, errCurve
	}
	switch string(t[:4]) {
	case "curv":
		return parseCurv(t)
	case "para":
		return parsePara(t)
	}
	return nil, errCurve
}

func parseCurv(t []byte) (func(float64) float64, error) {
	n := uint64(binary.BigEndian.Uint32(t[8:]))
	switch {
	case n == 0:
		return func(x float64) float64 { return x }, nil
	case n == 1:
		if len(t) < 14 {
			return nil, errCurve
		}
		g := float64(binary.BigEndian.Uint16(t[12:])) / 256
		if g <= 0 || g > iccMaxGamma {
			return nil, errCurve
		}
		return func(x float64) float64 { return pow(x, g) }, nil
	case n > iccMaxCurve || uint64(len(t)) < 12+2*n:
		return nil, errCurve
	}
	// Таблица читается прямо из байт профиля, линейная интерполяция — как у LittleCMS.
	at := func(i uint64) float64 { return float64(binary.BigEndian.Uint16(t[12+2*i:])) / 65535 }
	last := float64(n - 1)
	return func(x float64) float64 {
		f := x * last
		switch {
		case f <= 0 || math.IsNaN(f):
			return at(0)
		case f >= last:
			return at(n - 1)
		}
		i := uint64(f)
		return at(i) + (at(i+1)-at(i))*(f-float64(i))
	}, nil
}

func parsePara(t []byte) (func(float64) float64, error) {
	cnt := [...]int{1, 3, 4, 5, 7}
	typ := binary.BigEndian.Uint16(t[8:])
	if int(typ) >= len(cnt) || len(t) < 12+4*cnt[typ] {
		return nil, errCurve
	}
	var a [7]float64
	for i := range cnt[typ] {
		a[i] = S15(t[12+4*i:])
	}
	g, A, B, C, D, E, F := a[0], a[1], a[2], a[3], a[4], a[5], a[6]
	if g <= 0 || g > iccMaxGamma || (typ == 1 || typ == 2) && A == 0 {
		return nil, errCurve
	}
	switch typ {
	case 0:
		return func(x float64) float64 { return pow(x, g) }, nil
	case 1:
		return func(x float64) float64 {
			if x >= -B/A {
				return pow(A*x+B, g)
			}
			return 0
		}, nil
	case 2:
		return func(x float64) float64 {
			if x >= -B/A {
				return pow(A*x+B, g) + C
			}
			return C
		}, nil
	case 3:
		return func(x float64) float64 {
			if x >= D {
				return pow(A*x+B, g)
			}
			return C * x
		}, nil
	}
	return func(x float64) float64 {
		if x >= D {
			return pow(A*x+B, g) + E
		}
		return C*x + F
	}, nil
}

// SRGBCurves: кривая каждого канала совпадает с кривой sRGB до srgbCurveTol уровней на всей шкале 0…255
// (профиль sRGB v2 с таблицей, v4 с параметрической кривой, встроенный в LittleCMS). Матрицу не смотрим:
// колоранты проверяет вызывающий допуском 0,003 (решение 119-14). Решётка по пикселям не годилась:
// сдвиг колорантов на ±0,0002 у насыщенного красного давал 2 уровня и ронял настоящие sRGB в «ICC».
// Зазор по замеру: настоящие sRGB 0–1 уровень, гамма 2.2 — 9, Rec. 709 — 16, линейный — 73, e-sRGB — 95.
func (x *Xform) SRGBCurves() bool {
	for c := range 3 {
		for i, d := range x.dec[c] {
			if absInt(int(encLUT[d])-i) > srgbCurveTol {
				return false
			}
		}
	}
	return true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
