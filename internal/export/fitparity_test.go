package export

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// TestFitTolParity (123 §5): допуск подгонки рендера — тот же, что допуск формата модели
// (render model не импортирует).
func TestFitTolParity(t *testing.T) {
	if render.FitTolMm != model.PageTol {
		t.Fatalf("render.FitTolMm = %v, model.PageTol = %v", render.FitTolMm, model.PageTol)
	}
	for _, dpi := range []int{72, 150, 300, 600, 1200} {
		if a, b := render.FitTolPx(dpi), model.FitTolPx(dpi); a != b {
			t.Errorf("%d dpi: render %d px, model %d px", dpi, a, b)
		}
	}
}
