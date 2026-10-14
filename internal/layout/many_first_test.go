package layout

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Быстрые раскладки 17…64: ничьи, текущий блок в ряду страницы, эталон ключей первых.

// manyGeoms — геометрии эталонов 17…64: goldenGeoms (в т. ч. книжная страница с
// промежутком 0), квадрат 20×20 без layflat и широкий 40×30.
var manyGeoms = append(slices.Clone(goldenGeoms),
	Params{PageW: 200, PageH: 200, Margin: 12, Gap: 3, Layflat: false},
	Params{PageW: 400, PageH: 300, Margin: 12, Gap: 3, Layflat: true},
)

// manySetNames — наборы manyAspects (0–2 как у goldenAspects, 3–4 — все фото одной
// ориентации); имя без пробелов — оно в имени группы эталона.
var manySetNames = []string{"свадьба", "кадрирование", "случайные", "горизонтальные",
	"вертикальные", "чередование", "панорама"}

// manyAspects — набор set из n пропорций (имена — manySetNames).
func manyAspects(seed uint64, n, set int) []float64 {
	if set <= 2 {
		return goldenAspects(seed, n, set)
	}
	out := make([]float64, n)
	for i := range out {
		switch set {
		case 3:
			out[i] = L
		case 4:
			out[i] = P
		case 5:
			out[i] = []float64{P, L}[i%2]
		}
	}
	if set == 6 {
		out = weddingAspects(seed, n)
		out[n/3] = 3.5
	}
	return out
}

// TestManyTiesNoise — шум 1e-12 в пропорциях (как FMA на arm64 в оценках) не меняет ряд:
// ничьи решает порядок построения, а не последний бит оценки.
func TestManyTiesNoise(t *testing.T) {
	geoms, ns := manyGeoms, []int{17, 24, 33, 48, 64}
	if testkit.Race {
		geoms, ns = manyGeoms[:2], []int{17, 48} // чистый счёт: остальное — в обычном прогоне
	}
	for _, p := range geoms {
		for _, n := range ns {
			for set := range manySetNames {
				asp := manyAspects(uint64(n), n, set)
				noisy := slices.Clone(asp)
				rng := splitmix64(uint64(n)*100 + uint64(set))
				for i := range noisy {
					noisy[i] *= 1 + (rng.unit()*2-1)*1e-12
				}
				a, b := Layouts(asp, p, ManyBatch), Layouts(noisy, p, ManyBatch)
				if !slices.EqualFunc(a, b, func(x, y Result) bool { return x.Key == y.Key }) {
					t.Errorf("%+v n=%d %s: шум 1e-12 поменял ряд", p, n, manySetNames[set])
				}
			}
		}
	}
}

// TestManyPageRowHasBlock — блок, который поставил manyLayouts, есть в ряду своей страницы
// и на маленькой странице (до maxFullTree фото), где ряд — из перебора деревьев.
func TestManyPageRowHasBlock(t *testing.T) {
	small, step := 0, 1
	if testkit.Race {
		step = 3
	}
	for _, p := range []Params{album30, manyGeoms[3]} {
		for n := ManyFrom; n <= 26; n += step {
			for _, set := range []int{0, 2, 4} {
				asp := manyAspects(uint64(n), n, set)
				for _, r := range Layouts(asp, p, ManyBatch) {
					small += checkBlockInPageRow(t, asp, p, r.Key)
				}
			}
		}
	}
	if small == 0 {
		t.Error("ни одной страницы из 2…7 фото")
	}
}

// checkBlockInPageRow — у постраничного key каждая страница находит его в своём ряду;
// возвращает, сколько страниц из 2…maxFullTree фото проверено.
func checkBlockInPageRow(t *testing.T, asp []float64, p Params, key string) int {
	t.Helper()
	k, ok := pageForm(key)
	if !ok {
		return 0
	}
	small := 0
	for side := range 2 {
		if c := len(k.pages[side].leaves(nil)); c > 1 && c <= maxFullTree {
			small++
		}
		in, _, _, ok := PageChoices(key, asp, p, side, 24)
		if _, has := rowKeys(in)[key]; !ok || !has {
			t.Errorf("n=%d %s сторона %d: текущего блока нет в ряду страницы", len(asp), key, side)
		}
	}
	return small
}

const goldenFirstPath = "testdata/golden/first.sha"

// goldenFirst — ключ первой раскладки (Layouts(…, 1)) на 17…64 фото: геометрии
// manyGeoms × наборы manyAspects. Без оценок и Rects — не зависит от FMA, сверяется на любой
// архитектуре. Хеш ключа — 4 байта: эталон ловит смену, а не ищет её место.
func goldenFirst() []goldenGroup {
	var out []goldenGroup
	for g, p := range manyGeoms {
		for set, name := range manySetNames {
			out = append(out, goldenGroup{name: fmt.Sprintf("p%d/%s", g, name)})
			gr := &out[len(out)-1]
			gr.cases = make([]goldenCase, MaxPhotos-ManyFrom+1)
			var wg sync.WaitGroup
			for n := ManyFrom; n <= MaxPhotos; n++ {
				wg.Go(func() {
					key := Layouts(manyAspects(uint64(g)+1, n, set), p, 1)[0].Key
					h := sha256.Sum256([]byte(key))
					gr.cases[n-ManyFrom] = goldenCase{fmt.Sprintf("n=%d", n), hex.EncodeToString(h[:4])}
				})
			}
			wg.Wait()
		}
	}
	return out
}

// TestGoldenFirst — первая раскладка 17…64 совпадает с эталоном first.sha на любой
// архитектуре: так проверяется, что ничьи не решает шум округления. -update — по правилам many.sha.
func TestGoldenFirst(t *testing.T) {
	checkGolden(t, goldenFirstPath, goldenFirst())
}

// TestBreakTableSame — общая таблица разреза левых страниц даёт те же границы, что и
// отдельный счёт на каждую страницу: ускорение не меняет раскладку.
func TestBreakTableSame(t *testing.T) {
	g := newGeom(album30)
	for _, set := range []int{0, 2, 6} {
		asp := manyAspects(5, MaxPhotos, set)
		e := newEnum(asp, album30.Gap)
		e.breaks = map[breakKey]*breakTable{}
		for _, box := range []Rect{g.pages[0], g.spread} {
			for _, cols := range []bool{false, true} {
				for hi := 1; hi <= len(asp); hi++ {
					for r := 0; r <= 12; r++ {
						got, want := e.lineBreaks(asp, 0, hi, box, r, cols), lineBreaks(asp, 0, hi, box, album30.Gap, r, cols)
						if !slices.Equal(got, want) {
							t.Fatalf("набор %d hi=%d r=%d cols=%t: %v, ждали %v", set, hi, r, cols, got, want)
						}
					}
				}
			}
		}
	}
}
