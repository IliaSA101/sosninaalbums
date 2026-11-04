package grid

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestLookFitPageVignette: на листе 031 страниц нет — шаг становится страницей и fit не
// действует (одна область); на обычном листе остаётся.
func TestLookFitPageVignette(t *testing.T) {
	tp := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Fit: model.FitPage, Page: []model.TplBg{{Color: "#ffffff"}, {}},
		Stickers: []model.TplItem{{Anchor: model.Anchor{AX: -1, DX: 0.5}, W: 0.3, H: 0.3, Art: &model.Art{Kind: "star"}}}}
	p := model.NewSheet(400, 300)
	s := p.Spreads[0]
	if got, ok := lookTpl(p, s, tp); !ok || got.Fit != model.FitPage || got.Pages != 2 {
		t.Errorf("обычный лист: %+v %v", got, ok)
	}
	s.Kind, s.Vig = model.KindVignette, &model.Vignette{}
	got, ok := lookTpl(p, s, tp)
	if !ok || got.Fit != "" || got.Pages != 1 {
		t.Errorf("лист 031: fit %q, страниц %d, %v", got.Fit, got.Pages, ok)
	}
}
