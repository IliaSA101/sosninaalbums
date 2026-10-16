package plan

import "github.com/IliaSA101/sosninaalbums/internal/layout"

// Тест-хелперы: копии из layout (тесты двух пакетов не делятся). Генераторы определяют
// наборы эталонов, поэтому менять их нельзя.

var album30 = layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true}

const (
	L = 1.5     // горизонтальное 3:2
	P = 2.0 / 3 // вертикальное 2:3
	S = 1.0     // квадрат
)

var invariantParams = []layout.Params{album30, {PageW: 300, PageH: 200, Margin: 10, Gap: 4, Layflat: false}}

// splitmix64 — свой генератор: последовательность не должна зависеть от версии Go.
type splitmix64 uint64

func (s *splitmix64) next() uint64 {
	*s += 0x9e3779b97f4a7c15
	z := uint64(*s)
	z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
	z = (z ^ z>>27) * 0x94d049bb133111eb
	return z ^ z>>31
}

// unit — число в [0, 1), точное (53 бита).
func (s *splitmix64) unit() float64 { return float64(s.next()>>11) / (1 << 53) }

var (
	goldenWedding = []float64{L, P, L, P, L, P, S, 0.8, 1.25, 16.0 / 9, 9.0 / 16, L, P}
	goldenCrop    = []float64{1, 1.1, 0.9, 1.25, 0.8, 1.05, 0.95, 4.0 / 3, 0.75}
)

// goldenAspects — детерминированный набор из n пропорций: 0 «свадьба», 1 «кадрирование»,
// 2 «случайные».
func goldenAspects(seed uint64, n, set int) []float64 {
	rng := splitmix64(seed*1000 + uint64(n)*10 + uint64(set))
	out := make([]float64, n)
	for i := range out {
		switch set {
		case 0:
			out[i] = goldenWedding[rng.next()%uint64(len(goldenWedding))]
		case 1:
			out[i] = goldenCrop[rng.next()%uint64(len(goldenCrop))]
		default:
			out[i] = 0.5 + 1.5*rng.unit()
		}
	}
	return out
}

var goldenGeoms = []layout.Params{
	album30,
	{PageW: 300, PageH: 200, Margin: 10, Gap: 4, Layflat: false},
	{PageW: 200, PageH: 300, Margin: 5, Gap: 0, Layflat: true},
}
