package testkit

import (
	"sync"
	"time"
)

// Большой набор для e2e (089): 600 разных кадров 24 Мп сериями по 6 из 100 stock. Здесь — чистый
// план (номер → что рисовать и что писать в EXIF), рисует и пишет его scripts/e2e-photos.go
// через BigSetRender и BigSetEncode. План не зависит от машины: те же номер и зерно — те же байты.

// BigSetVersion — версия плана; идёт в manifest.txt, чтобы старый набор не путали с новым.
const BigSetVersion = 1

const (
	bigSeries     = 6   // кадров в серии (очередь затвора)
	bigSources    = 100 // исходников stock
	bigSquareEach = 20  // каждый 20-й — 1:1
	bigAdobeEach  = 31  // каждый 31-й — Adobe RGB (≈ 3 %)
	bigRotateEach = 33  // каждый 33-й — поворот EXIF (≈ 3 %)
	bigMaxBright  = 0.06
)

// BigShot — план одного кадра набора.
type BigShot struct {
	N        int  // номер кадра, с 1
	Source   int  // исходник stock, 1..100
	Variant  int  // 0..5 внутри серии
	Portrait bool // исходник вертикальный → кадр 2:3
	Square   bool // кадр 1:1
	// W и H — размеры кадра, как их видит программа (после поворота по EXIF).
	W, H    int
	Scale   float64 // доля максимальной рамки нужной пропорции в исходнике, 0,85…1
	CropX   float64 // положение рамки в исходнике, 0..1
	CropY   float64
	Mirror  bool    // зеркало (варианты 3 и 5)
	Bright  float64 // яркость: множитель 1+Bright, ±0,06
	NoiseDX int     // сдвиг плитки шума, 0..255
	NoiseDY int
	Rotate  bool // пиксели повёрнуты на 90°, в EXIF Orientation = 6
	Adobe   bool // ICC Adobe RGB
	Taken   time.Time
}

// Orig — значение DateTimeOriginal («стенное» время камеры).
func (p BigShot) Orig() string { return p.Taken.Format("2006:01:02 15:04:05") }

// StoredSize — размеры пикселей в файле: при повороте EXIF стороны меняются местами.
func (p BigShot) StoredSize() (w, h int) {
	if p.Rotate {
		return p.H, p.W
	}
	return p.W, p.H
}

// Кадрирование вариантов серии: доля рамки и её положение. Варианты 0 и 3 различаются
// только зеркалом, яркостью и шумом — как два кадра подряд с одной точки.
var (
	bigScale = [bigSeries]float64{1, 0.92, 0.85, 1, 0.92, 0.85}
	bigCropX = [bigSeries]float64{0.5, 0.15, 0.85, 0.5, 0.7, 0.3}
	bigCropY = [bigSeries]float64{0.5, 0.3, 0.7, 0.5, 0.2, 0.8}
)

// bigAspects — пропорции 100 stock; таблица зашита в stock.go, ошибка в ней — ошибка кода.
var bigAspects = sync.OnceValue(func() []float64 {
	a, err := parseStockAspects()
	if err != nil {
		panic(err)
	}
	return a
})

// bigMix — splitmix64 от (зерно, номер, соль): без math/rand, чтобы план не менялся с версией Go.
func bigMix(seed int64, i int, salt uint64) uint64 {
	z := uint64(seed) + uint64(i)*0x9E3779B97F4A7C15 + salt*0xD1B54A32D192ED03
	z = (z ^ z>>30) * 0xBF58476D1CE4E5B9
	z = (z ^ z>>27) * 0x94D049BB133111EB
	return z ^ z>>31
}

// bigUnit — число из [0, 1) для (зерно, номер, соль).
func bigUnit(seed int64, i int, salt uint64) float64 {
	return float64(bigMix(seed, i, salt)>>11) / (1 << 53)
}

// bigJitter — положение рамки: база варианта ± 0,05, в пределах 0..1.
func bigJitter(base float64, seed int64, i int, salt uint64) float64 {
	return min(1, max(0, base+(bigUnit(seed, i, salt)-0.5)*0.1))
}

// BigSetPlan — план кадра i (с 1) при зерне seed. i < 1 — пустой план (N == 0).
// Кадр i: исходник (i-1)/6 % 100 + 1, вариант (i-1) % 6; дата — день 2026:06:14, серия +2 мин от
// 10:00, внутри серии +1 с, так что даты строго растут с номером.
func BigSetPlan(i int, seed int64) BigShot {
	if i < 1 {
		return BigShot{}
	}
	series, v := (i-1)/bigSeries, (i-1)%bigSeries
	p := BigShot{
		N: i, Source: series%bigSources + 1, Variant: v,
		Scale: bigScale[v], CropX: bigJitter(bigCropX[v], seed, i, 1), CropY: bigJitter(bigCropY[v], seed, i, 2),
		Mirror:  v == 3 || v == 5,
		Bright:  (bigUnit(seed, i, 3) - 0.5) * 2 * bigMaxBright,
		NoiseDX: int(bigMix(seed, i, 4) % 256), NoiseDY: int(bigMix(seed, i, 5) % 256),
		Rotate: i%bigRotateEach == 0, Adobe: i%bigAdobeEach == 0,
		Taken: time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC).Add(time.Duration(series)*2*time.Minute + time.Duration(v)*time.Second),
	}
	p.Portrait = bigAspects()[p.Source-1] < 0.9
	p.Square = i%bigSquareEach == 0
	switch {
	case p.Square:
		p.W, p.H = 4899, 4899
	case p.Portrait:
		p.W, p.H = 4000, 6000
	default:
		p.W, p.H = 6000, 4000
	}
	return p
}

// BigSetSegments — сегменты JPEG кадра: APP1 EXIF (производитель, модель, дата, при повороте
// Orientation = 6) и для Adobe — APP2 с ICC. Порядок — как их вставляет BigSetEncode.
func BigSetSegments(p BigShot) [][]byte {
	f := ShotFields{Make: "E2E", Model: "Synthetic 24", Orig: p.Orig()}
	if p.Rotate {
		f.Orient = 6
	}
	segs := [][]byte{ExifShot(false, f)}
	if p.Adobe {
		segs = append(segs, ICCSegment(1, 1, MatrixProfile(XYZAdobe, 2.2, DescV2("Adobe RGB (1998)"))))
	}
	return segs
}
