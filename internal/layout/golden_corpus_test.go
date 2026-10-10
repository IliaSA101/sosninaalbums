package layout

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
)

// Корпус эталона TestGoldenLayouts: детерминированные входы и их свёртка в хеши.

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

var goldenGeoms = []Params{
	album30,
	{PageW: 300, PageH: 200, Margin: 10, Gap: 4, Layflat: false},
	{PageW: 200, PageH: 300, Margin: 5, Gap: 0, Layflat: true},
}

// goldenSets — какие наборы пропорций берём на n фото: счёт дорожает быстро, поэтому
// на больших n по одному набору, а они чередуются, чтобы все три были и на малых, и на больших.
func goldenSets(n int) []int {
	if n <= 4 {
		return []int{0, 1, 2}
	}
	return []int{n % 3}
}

// goldenAutoN — фото в проверке Group (корпус укладывается в секунды).
const goldenAutoN = 24

// dump накапливает побитовое представление результатов.
type dump struct{ buf []byte }

func (d *dump) u64(v uint64) { d.buf = binary.LittleEndian.AppendUint64(d.buf, v) }
func (d *dump) str(s string) { d.u64(uint64(len(s))); d.buf = append(d.buf, s...) }
func (d *dump) results(rs []Result) {
	d.u64(uint64(len(rs)))
	for _, r := range rs {
		d.str(r.Key)
		d.u64(math.Float64bits(r.Score))
		d.u64(uint64(len(r.Rects)))
		for _, c := range r.Rects {
			d.u64(math.Float64bits(c.X))
			d.u64(math.Float64bits(c.Y))
			d.u64(math.Float64bits(c.W))
			d.u64(math.Float64bits(c.H))
		}
	}
}
func (d *dump) sum() string { h := sha256.Sum256(d.buf); return hex.EncodeToString(h[:8]) }

// goldenCase — случай группы: подпись для сообщения и хеш. goldenGroup — группы p0/n7/L.
type goldenCase struct{ desc, hash string }
type goldenGroup struct {
	name  string
	cases []goldenCase
}

// pagedKeys — первые три постраничных ключа из Layouts(limit 24).
func pagedKeys(asp []float64, p Params) []string {
	var keys []string
	for _, r := range Layouts(asp, p, 24) {
		if IsPaged(r.Key) {
			keys = append(keys, r.Key)
			if len(keys) == 3 {
				break
			}
		}
	}
	return keys
}

// goldenN считает группы L, R, P0, P1 для геометрии g и числа фото n.
func goldenN(g, n int) []goldenGroup {
	p := goldenGeoms[g]
	kinds := []string{"L", "R", "P0", "P1"}
	groups := make([]goldenGroup, len(kinds))
	for i, k := range kinds {
		groups[i].name = fmt.Sprintf("p%d/n%d/%s", g, n, k)
	}
	for _, set := range goldenSets(n) {
		asp := goldenAspects(uint64(g)+1, n, set)
		desc := fmt.Sprintf("набор %d, пропорции %.4g", set, asp)
		var d dump
		d.results(Layouts(asp, p, 24))
		groups[0].cases = append(groups[0].cases, goldenCase{desc, d.sum()})

		d = dump{}
		r, b := Rearranged(asp, p, 24)
		d.results(r)
		d.str(string(b))
		groups[1].cases = append(groups[1].cases, goldenCase{desc, d.sum()})

		for side := range 2 {
			d = dump{}
			for _, key := range pagedKeys(asp, p) {
				in, re, blk, ok := PageChoices(key, asp, p, side, 24)
				d.str(key)
				d.results(in)
				d.results(re)
				d.str(string(blk))
				if ok {
					d.u64(1)
				} else {
					d.u64(0)
				}
			}
			groups[2+side].cases = append(groups[2+side].cases, goldenCase{desc, d.sum()})
		}
	}
	return groups
}

// goldenGroupAuto считает автосборку Group на 24 фото при target 3, 5, 7.
func goldenGroupAuto(g int) []goldenGroup {
	p := goldenGeoms[g]
	var out []goldenGroup
	for _, target := range []int{3, 5, 7} {
		gr := goldenGroup{name: fmt.Sprintf("p%d/G/t%d", g, target)}
		for _, set := range []int{0, 2} {
			asp := goldenAspects(uint64(g)+1, goldenAutoN, set)
			var d dump
			for _, pr := range Group(asp, p, target) {
				d.u64(uint64(pr[0]))
				d.u64(uint64(pr[1]))
			}
			gr.cases = append(gr.cases, goldenCase{fmt.Sprintf("набор %d, %d фото", set, goldenAutoN), d.sum()})
		}
		out = append(out, gr)
	}
	return out
}

// goldenCorpus считает весь корпус параллельно; порядок групп фиксирован по имени.
func goldenCorpus() []goldenGroup {
	var jobs []func() []goldenGroup
	for g := range goldenGeoms {
		for n := 1; n <= MaxPhotos; n++ {
			if g > 0 && n >= 13 && n < MaxPhotos {
				continue // 13…15 дороги; на 2-й и 3-й геометрии берём только предел
			}
			jobs = append(jobs, func() []goldenGroup { return goldenN(g, n) })
		}
		jobs = append(jobs, func() []goldenGroup { return goldenGroupAuto(g) })
	}
	res := make([][]goldenGroup, len(jobs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, job := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res[i] = job()
		}()
	}
	wg.Wait()
	var all []goldenGroup
	for _, r := range res {
		all = append(all, r...)
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].name < all[j].name })
	return all
}
