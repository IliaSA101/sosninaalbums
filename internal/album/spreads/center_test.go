package spreads_test

// 0.23-q6, «центр крупнее»: H(m0,1,m2) доходит до галереи (LayoutChoices) на трёх вертикальных
// фото при «Любых фото». Ряд «Авто» берёт одну форму contrast — Q[0|H(1,m2)] — и центр,
// проигрывавший по Q на штраф сгиба, не показывался; теперь он в «Других вариантах».

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func TestGalleryCenterBig(t *testing.T) {
	const want = "H(m0,1,m2)"
	for _, c := range []struct {
		name string
		wh   [2]int
	}{
		{"1333×2000", [2]int{1333, 2000}}, // 0,6665: не ровно 2:3
		{"2:3 ровно", [2]int{200, 300}},
	} {
		for _, h := range []float64{300, 305} { // умолчание модели и «30×30» из фабрики
			for _, foldAny := range []bool{true, false} {
				e := edtest.Open(t, model.New(), "")
				st := edtest.P(e).Settings
				st.PageH, st.FoldAny = h, foldAny
				e.SetSettings(st)
				id := edtest.SpreadIDs(edtest.NewSpread(e, -1, whPhotos("v", c.wh, c.wh, c.wh)))[0]
				got := false
				ch := e.LayoutChoices(id).Whole
				for _, r := range append(slices.Clone(ch.InOrder), ch.Extra...) {
					got = got || r.Key == want
				}
				// «Только широкие»: вертикальное фото на сгибе не пускает правило 064 — ключа нет.
				if got != foldAny {
					t.Errorf("%s, высота %v, «любые» %v: %s в галерее — %v", c.name, h, foldAny, want, got)
				}
			}
		}
	}
}
