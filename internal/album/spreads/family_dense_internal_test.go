package spreads

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// TestChoiceFamilyDense (0.23-q7): плитки подборки (ряд «Авто» и «Другие варианты») несут семью по
// заполнению; сценарий владельца — 6 вертикальных 2:3 на 600×305, поля/промежуток 12/3 и 3/3:
// «Плотно» не пусто, и каждая такая плитка — «Две крупных» либо мозаика.
func TestChoiceFamilyDense(t *testing.T) {
	asp := []float64{2.0 / 3, 2.0 / 3, 2.0 / 3, 2.0 / 3, 2.0 / 3, 2.0 / 3}
	for _, mg := range [][2]float64{{12, 3}, {3, 3}} {
		prm := layout.Params{PageW: 300, PageH: 305, Margin: mg[0], Gap: mg[1], Layflat: true, FoldAny: true}
		rows := choiceRows(variety.RowResults(asp, prm, 24), nil, "", prm).InOrder
		rows = append(rows, extraChoices(asp, prm)...)
		dense := 0
		for _, c := range rows {
			f := forms.Of(c.Key, c.Rects, prm)
			if want := string(variety.TileFamily(f, c.Rects, prm)); c.Family != want {
				t.Errorf("%g/%g %s: семья %q, ждали %q", mg[0], mg[1], c.Key, c.Family, want)
			}
			if c.Family == string(variety.Dense) {
				dense++
				if f != forms.Pair && variety.FamilyOf(f) != variety.Dense {
					t.Errorf("%g/%g %s: «Плотно» у формы %s", mg[0], mg[1], c.Key, f)
				}
			}
		}
		if dense == 0 {
			t.Errorf("%g/%g: «Плотно» пусто среди %d плиток", mg[0], mg[1], len(rows))
		}
		// «Что соберётся» (104) — семья плитки, как в галерее и в фильтре окна.
		for _, r := range variety.RowResults(asp, prm, 24) {
			f := forms.Of(r.Key, r.Rects, prm)
			if one := describeOne(r, prm); one.Family != string(variety.TileFamily(f, r.Rects, prm)) {
				t.Errorf("%g/%g %s: describeOne %q, ждали %q", mg[0], mg[1], r.Key, one.Family, variety.TileFamily(f, r.Rects, prm))
			}
		}
	}
}
