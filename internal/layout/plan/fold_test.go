package plan

import (
	"fmt"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestPlanFoldRule — окна автосборки при «только широкие», «владелец 2026-10-17» (правило сгиба
// одно для всех путей подбора, план — тоже): матрица побитово как у прежнего пути (RefQuickScore
// через manyGen.spread, где фильтр стоит с G1), то есть ряд на разворот вне правила окно не
// оценивает. Чтобы сверка не была пустой, на тех же наборах при «любые фото» есть окна, где
// оценка выше, — их и отсёк фильтр.
func TestPlanFoldRule(t *testing.T) {
	if testkit.Lite() {
		t.Skip("чистый счёт в одной горутине: под -race и -short проверяет обычный прогон")
	}
	differ := 0
	for gi, pAny := range planGeoms {
		if !pAny.Layflat {
			continue
		}
		wide := pAny
		wide.FoldAny = false
		asp := planSets(uint64(gi)+11, 40)[gi%4]
		where := fmt.Sprintf("geom %d «широкие»", gi)
		sameCols(t, where, windowCols(asp, wide, 1, layout.MaxPhotos), refCols(asp, wide, layout.MaxPhotos)[1:])
		ca, cw := windowCols(asp, pAny, 1, layout.MaxPhotos), windowCols(asp, wide, 1, layout.MaxPhotos)
		for k := layout.MaxFullTree; k < len(ca); k++ {
			for i := range ca[k] {
				if ca[k][i] < cw[k][i] {
					t.Fatalf("geom %d: окно [%d, +%d) при «широких» %v выше, чем при «любые» %v", gi, i, k+1, cw[k][i], ca[k][i])
				}
				if ca[k][i] > cw[k][i] {
					differ++
				}
			}
		}
	}
	if differ == 0 {
		t.Error("ни одно окно не отличается при «любые» и «широких»: сверка ничего не проверяет")
	}
}
