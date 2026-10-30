package export

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// TestPxParity (123 §1.1): model.Px — та же формула, что render.px (model рендер не импортирует,
// depguard), а страница из PageFromPx даёт холст CanvasSize ровно в введённые px при 72…1200 dpi.
func TestPxParity(t *testing.T) {
	for _, dpi := range []int{72, 150, 200, 300, 400, 600, 1200} {
		for mm := 0.0; mm <= 1000; mm += 0.37 {
			w, h := render.CanvasSize(render.Options{PageW: mm, PageH: mm, DPI: dpi})
			if w != model.Px(2*mm, dpi) || h != model.Px(mm, dpi) {
				t.Fatalf("%v мм @%d: render %d×%d, model %d×%d", mm, dpi, w, h, model.Px(2*mm, dpi), model.Px(mm, dpi))
			}
		}
		for W := 800; W <= 40000; W += 997 {
			H := W/2 + 13
			pw, ph, err := model.PageFromPx(W, H, dpi)
			if err != nil {
				continue // страница вне 50…1000 мм при этом dpi
			}
			if gw, gh := render.CanvasSize(render.Options{PageW: pw, PageH: ph, DPI: dpi}); gw != W || gh != H {
				t.Fatalf("%d×%d px @%d dpi → %v×%v мм → холст %d×%d", W, H, dpi, pw, ph, gw, gh)
			}
		}
	}
}
