package variety

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
)

// midOf — «центр крупнее» (contrast) среди «Других вариантов» набора: ключи и кандидаты пула.
func midOf(as []float64, p layout.Params) (keys []string, inPool []Cand) {
	for _, r := range Extra(as, p, 24) {
		if forms.CenterBig(r.Key) {
			keys = append(keys, r.Key)
		}
	}
	for _, c := range Pool(as, p, 24, Normal) {
		if forms.CenterBig(c.Key) {
			inPool = append(inPool, c)
		}
	}
	return keys, inPool
}

// TestExtraCenterBig: «центр крупнее» (0.23-q6) есть в «Других вариантах» — на вертикальных 2:3 и на
// 1333×2000 (0,6665), на 3, 4 и 5 фото; ряд «Авто» прежний (первая раскладка та же, центра в нём
// нет), а сборка центр не берёт: в пуле он не из ряда, не Formed и без семейства featured (Pick
// допускает только их, воздух и звезду — вариант 1 остаётся как в 0.19.0).
func TestExtraCenterBig(t *testing.T) {
	p := layout.Params{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, FoldAny: true}
	for _, a := range []float64{1333.0 / 2000, 2.0 / 3} {
		for _, n := range []int{3, 4, 5} {
			as := sameAspects(n, a)
			keys, inPool := midOf(as, p)
			if n < 5 && len(keys) != midLimit { // 5 вертикальных: Q центра ниже порога centerQSlack
				t.Errorf("a=%v n=%d: центров в «Других вариантах» %d, ждали %d", a, n, len(keys), midLimit)
			}
			for _, c := range inPool {
				if c.InRow || c.Formed || c.Kind != NoKind {
					t.Errorf("a=%v n=%d: %s в пуле как выбор сборки: InRow %v, Formed %v, Kind %v", a, n, c.Key, c.InRow, c.Formed, c.Kind)
				}
			}
			for _, c := range Row(as, p, 24) {
				if forms.CenterBig(c.Key) && c.Form == forms.Contrast {
					t.Errorf("a=%v n=%d: центр %s в ряду «Авто»", a, n, c.Key)
				}
			}
			if first := layout.Cached(as, p, 24)[0].Key; Row(as, p, 24)[0].Key != first {
				t.Errorf("a=%v n=%d: первая раскладка ряда не %s", a, n, first)
			}
		}
	}
	// «Только широкие»: вертикальный центр на сгибе не пускает правило 064 — слота нет.
	p.FoldAny = false
	if keys, _ := midOf(sameAspects(3, 2.0/3), p); len(keys) != 0 {
		t.Errorf("«Только широкие»: центр %v", keys)
	}
	// Горизонтальные проходят сгиб при любом положении.
	if keys, _ := midOf(sameAspects(3, aspL), geomSq); len(keys) == 0 {
		t.Error("3 горизонтальных: центра нет в «Других вариантах»")
	}
}
