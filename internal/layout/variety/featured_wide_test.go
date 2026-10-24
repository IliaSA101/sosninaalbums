package variety

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Генератор featured на 5–7 и 17–40 фото (051 G5, design §1, §5): те же инварианты и запреты,
// что на 8–16, сетка G вместо рядов V(H…) с 17 фото, эталон featured57.sha.

// wideSizes — размеры вне 8–16: 5–7 и 17–40 (шагом, чтобы тесты оставались быстрыми).
var wideSizes = []int{5, 6, 7, 17, 20, 24, 32, 40}

// wideSets — наборы 5–7 и 17–40 фото: горизонтальные, вертикальные, серии, смесь.
func wideSets() map[string][]float64 {
	out := map[string][]float64{}
	for _, n := range wideSizes {
		out[fmt.Sprintf("гориз%d", n)] = sameAspects(n, aspL)
		out[fmt.Sprintf("верт%d", n)] = sameAspects(n, aspP)
		out[fmt.Sprintf("серия%d", n)] = seriesAspects(n, 0.65, uint64(n))
		out[fmt.Sprintf("смесь%d", n)] = mixedAspects(n, uint64(n))
	}
	return out
}

// TestFeaturedWide: ключи генератора на 5–7 и 17–40 фото строятся, проходят инварианты и запреты
// №31; в каждом диапазоне и на каждой геометрии что-то есть; корень G — только с 17 фото.
func TestFeaturedWide(t *testing.T) {
	sets := wideSets()
	for _, g := range featGeoms {
		got := map[bool]int{}   // раскладок: false — 5–7, true — 17–40
		gKeys := map[bool]int{} // из них с корнем G
		for _, name := range sortedNames(sets) {
			as := sets[name]
			many := len(as) >= layout.ManyFrom
			for _, f := range featured(as, g.p) {
				checkFeat(t, g.name+"/"+name, as, g.p, f)
				if msg := foldFault(f.Rects, g.p); msg != "" {
					t.Fatalf("%s/%s %q: %s", g.name, name, f.Key, msg)
				}
				got[many]++
				if strings.Contains(f.Key, "G(") {
					gKeys[many]++
				}
			}
		}
		for _, many := range []bool{false, true} {
			if got[many] == 0 {
				t.Errorf("%s: ни одной раскладки (17–40 фото: %v)", g.name, many)
			}
		}
		if gKeys[false] != 0 || gKeys[true] == 0 {
			t.Errorf("%s: корней G на 5–7 фото %d (ждали 0), на 17–40 — %d (ждали > 0)", g.name, gKeys[false], gKeys[true])
		}
	}
}

// TestFeaturedWideEdges: границы диапазона — 4 и 41 фото пусто, 5 и 40 — работают; бюджет Build
// растёт линейно (не больше 12 на фото) и на 40 фото укладывается; плохая пропорция — пусто.
func TestFeaturedWideEdges(t *testing.T) {
	for _, n := range []int{4, 41} {
		if fs := featured(sameAspects(n, aspL), geomSq); len(fs) != 0 {
			t.Errorf("%d фото: %d раскладок", n, len(fs))
		}
	}
	for _, n := range []int{5, 40} {
		as := mixedAspects(n, 3)
		if b := builds(as, geomSq); b > 12*n {
			t.Errorf("%d фото: Build %d > %d", n, b, 12*n)
		}
	}
	if fs := featured(sameAspects(5, aspL), geomSq); len(fs) == 0 {
		t.Error("5 одинаковых: ни одной раскладки")
	}
	bad := sameAspects(20, aspL)
	bad[3] = -1
	if fs := featured(bad, geomSq); len(fs) != 0 {
		t.Errorf("плохая пропорция: %d раскладок", len(fs))
	}
}

// TestFeaturedWideOldRange: 8–16 фото — те же ключи, что до 051 (корень G на 8–16 не появляется,
// диапазон не влияет на них): ни одного G и ни одного «m».
func TestFeaturedWideOldRange(t *testing.T) {
	for _, name := range sortedNames(featSets()) {
		for _, f := range featured(featSets()[name], geomSq) {
			if strings.Contains(f.Key, "G(") {
				t.Fatalf("%s: %q — G на 8–16 фото", name, f.Key)
			}
		}
	}
}

// TestFeaturedExtraWide: гейт 8–16 снят (051 G6) — на 5–7 фото featured вне ряда «Авто» идут в
// «Другие варианты», на 17–40 «Других вариантов» с featured нет (подборка их там не показывает), а
// раскладки с главным встают в ряд «Авто» формами — только в первой порции ManyBatch.
func TestFeaturedExtraWide(t *testing.T) {
	for _, n := range []int{5, 6, 7, 17, 24, 32} {
		as := sameAspects(n, aspL)
		if len(featured(as, geomSq)) == 0 {
			t.Fatalf("%d фото: генератор пуст — проверка бессмысленна", n)
		}
		e := extraOf(as, geomSq, rowLen(n))
		re := rowOf(as, geomSq, 2*rowLen(n))
		switch {
		case n < layout.ManyFrom && len(e.kind) == 0:
			t.Errorf("%d фото: featured в «Других вариантах» нет", n)
		case n >= layout.ManyFrom && len(e.kind) != 0:
			t.Errorf("%d фото: featured в «Других вариантах»: %d", n, len(e.kind))
		}
		inRow := 0
		for i, c := range re.row {
			if re.kind[c.Key] == NoKind {
				continue
			}
			inRow++
			if n >= layout.ManyFrom && i >= layout.ManyBatch {
				t.Errorf("%d фото: featured %q на %d — вне первой порции", n, c.Key, i)
			}
			if e.kind[c.Key] != NoKind {
				t.Errorf("%d фото: featured %q и в ряду, и в «Других вариантах»", n, c.Key)
			}
		}
		t.Logf("%d фото: featured в ряду %d, в «Других вариантах» %d", n, inRow, len(e.kind))
	}
}

// (на 30–40 фото полы мельчайшей стороны оставляют мало или ничего — это ожидаемо.)

// TestFeaturedWideSpeed — страж: генератор на 40 фото не взрывается (цель — BenchmarkFeatured40).
func TestFeaturedWideSpeed(t *testing.T) {
	testkit.CostTest(t)
	as := mixedAspects(40, 3)
	d := testkit.Fastest(5, func() { featured(as, geomSq) })
	t.Logf("featured 40: %v", d)
	if lim := testkit.Limit(200 * time.Millisecond); d > lim {
		t.Fatalf("featured 40 фото %v > %v", d, lim)
	}
}

// BenchmarkFeatured40 — генератор на 40 фото (design §12).
func BenchmarkFeatured40(b *testing.B) {
	as := mixedAspects(40, 3)
	for b.Loop() {
		featured(as, geomSq)
	}
}

// BenchmarkFeatured6 — генератор на 6 фото (новый диапазон 5–7).
func BenchmarkFeatured6(b *testing.B) {
	as := mixedAspects(6, 3)
	for b.Loop() {
		featured(as, geomSq)
	}
}

// Эталон новых диапазонов: ключи featured на наборах 5–7 и 17–40 фото × 3 геометрии — sha256
// отсортированных ключей, как featured.sha (там 8–16 — не меняется). amd64.
const wideGoldenPath = "testdata/golden/featured57.sha"

func wideGoldenLines() []string {
	sets := wideSets()
	var out []string
	for _, g := range featGeoms {
		for _, name := range sortedNames(sets) {
			var keys []string
			for _, f := range featured(sets[name], g.p) {
				keys = append(keys, f.Key)
			}
			slices.Sort(keys)
			h := sha256.Sum256([]byte(strings.Join(keys, "\n")))
			out = append(out, fmt.Sprintf("%s/%s %d %s", g.name, name, len(keys), hex.EncodeToString(h[:8])))
		}
	}
	return out
}

func TestGoldenFeaturedWide(t *testing.T) {
	goldenArch(t)
	got := wideGoldenLines()
	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(wideGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(wideGoldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", wideGoldenPath, len(got))
		return
	}
	data, err := os.ReadFile(wideGoldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/layout/variety -run GoldenFeaturedWide -update", err)
	}
	want := strings.Split(strings.TrimSpace(string(data)), "\n")
	if !slices.Equal(want, got) {
		bad := 0
		for i := range min(len(got), len(want)) {
			if got[i] != want[i] {
				if bad++; bad <= 5 {
					t.Errorf("расхождение:\n  есть   %s\n  эталон %s", got[i], want[i])
				}
			}
		}
		t.Errorf("строк %d / %d, разошлось %d", len(got), len(want), bad)
	}
}
