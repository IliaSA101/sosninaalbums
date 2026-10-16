package layout

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Эталон раскладки: страховка перед ускорением (спека 020). Хранит не полный дамп, а
// SHA-256 каждого случая (первые 8 байт): ключ, оценка и Rects сверяются побитово.
var updateGolden = flag.Bool("update", false, "перезаписать эталоны testdata/golden/*.sha запущенных тестов")

const goldenPath = "testdata/golden/layouts.sha"

func formatGolden(gs []goldenGroup) string {
	var b strings.Builder
	for _, g := range gs {
		hs := make([]string, len(g.cases))
		for i, c := range g.cases {
			hs[i] = c.hash
		}
		fmt.Fprintf(&b, "%s %s\n", g.name, strings.Join(hs, ","))
	}
	return b.String()
}

// TestGoldenLayouts: Layouts, Rearranged и PageChoices (обе страницы) на
// детерминированном корпусе совпадают с эталоном побитово. Эталон снят с кода до
// ускорения; перезапись — go test ./internal/layout -run Golden -update (или SA_GOLDEN_UPDATE=1).
func TestGoldenLayouts(t *testing.T) {
	// На arm64 компилятор вправе сливать x*y+z в FMA — биты оценок другие.
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	checkGolden(t, goldenPath, goldenCorpus())
}

const goldenLegacyPath = "testdata/golden/legacy.sha"

// TestGoldenLegacy: LegacyLayouts побитово совпадает с Layouts до решения 95 — строки L
// прежнего layouts.sha (миграция альбомов до 0.10 берёт по нему прежнее «Авто»).
// Эталон не перезаписывать: он и есть прежнее поведение.
func TestGoldenLegacy(t *testing.T) {
	if testkit.Race {
		t.Skip("чистый счёт в одной горутине: под -race проверяет обычный прогон")
	}
	if runtime.GOARCH != "amd64" {
		t.Skip("эталон снят на amd64")
	}
	var got []goldenGroup
	for g, p := range goldenGeoms {
		for n := 1; n <= goldenMaxN; n++ {
			if g > 0 && n >= 13 && n < goldenMaxN {
				continue // как goldenCorpus
			}
			gr := goldenGroup{name: fmt.Sprintf("p%d/n%d/L", g, n)}
			for _, set := range goldenSets(n) {
				var d dump
				d.results(LegacyLayouts(goldenAspects(uint64(g)+1, n, set), p, 24))
				gr.cases = append(gr.cases, goldenCase{fmt.Sprintf("набор %d", set), d.sum()})
			}
			got = append(got, gr)
		}
	}
	sort.SliceStable(got, func(i, j int) bool { return got[i].name < got[j].name })
	checkGolden(t, goldenLegacyPath, got)
}

const goldenFirst16Path = "testdata/golden/first16.sha"

// TestGoldenFirst16 — ключ первой раскладки (Layouts(…, 24)[0], «Авто») на 1…16 фото:
// goldenGeoms × три набора goldenAspects. Как first.sha для 17–64 — без оценок и Rects, поэтому
// сверяется и на arm64: ничьи решают tieQ и ключ (решение 95), а не шум FMA. -update — только
// при осознанной смене раскладки.
func TestGoldenFirst16(t *testing.T) {
	var got []goldenGroup
	for g, p := range goldenGeoms {
		for set := range 3 {
			gr := goldenGroup{name: fmt.Sprintf("p%d/набор%d", g, set), cases: make([]goldenCase, goldenMaxN)}
			var wg sync.WaitGroup
			for n := 1; n <= goldenMaxN; n++ {
				wg.Go(func() {
					key := Layouts(goldenAspects(uint64(g)+1, n, set), p, 24)[0].Key
					h := sha256.Sum256([]byte(key))
					gr.cases[n-1] = goldenCase{fmt.Sprintf("n=%d", n), hex.EncodeToString(h[:4])}
				})
			}
			wg.Wait()
			got = append(got, gr)
		}
	}
	checkGolden(t, goldenFirst16Path, got)
}

// checkGolden сверяет группы с файлом эталона path или перезаписывает его (-update).
func checkGolden(t *testing.T, path string, got []goldenGroup) {
	t.Helper()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(formatGolden(got)), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d групп)", path, len(got))
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("нет эталона (%v): запустите go test ./internal/layout -run %s -update", err, t.Name())
	}
	want := map[string][]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		name, hs, _ := strings.Cut(line, " ")
		want[name] = strings.Split(hs, ",")
	}
	var bad []string
	for _, g := range got {
		w, ok := want[g.name]
		delete(want, g.name)
		switch {
		case !ok:
			bad = append(bad, g.name+": группы нет в эталоне")
		case len(w) != len(g.cases):
			bad = append(bad, fmt.Sprintf("%s: случаев %d, в эталоне %d", g.name, len(g.cases), len(w)))
		default:
			for i, c := range g.cases {
				if w[i] != c.hash {
					bad = append(bad, fmt.Sprintf("%s: первый разошедшийся случай %d (%s)", g.name, i, c.desc))
					break
				}
			}
		}
	}
	for name := range want {
		bad = append(bad, name+": группы нет в результате")
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		if len(bad) > 8 {
			bad = append(bad[:8], fmt.Sprintf("… и ещё %d", len(bad)-8))
		}
		t.Fatalf("раскладка разошлась с эталоном %s (если изменение задумано — перезаписать: -update):\n%s",
			path, strings.Join(bad, "\n"))
	}
}

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

// goldenMaxN — до стольких фото эталон layouts.sha (перебор); 17…64 — many.sha (TestGoldenMany).
const goldenMaxN = ManyFrom - 1

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

// goldenCorpus считает весь корпус параллельно; порядок групп фиксирован по имени.
func goldenCorpus() []goldenGroup {
	var jobs []func() []goldenGroup
	for g := range goldenGeoms {
		for n := 1; n <= goldenMaxN; n++ {
			if g > 0 && n >= 13 && n < goldenMaxN {
				continue // 13…15 дороги; на 2-й и 3-й геометрии берём только предел
			}
			jobs = append(jobs, func() []goldenGroup { return goldenN(g, n) })
		}
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
