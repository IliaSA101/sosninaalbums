package looktpl

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// FitPage — SpreadTpl.Fit: двухстраничный шаг подгоняется по страницам (165, design §2).
const FitPage = "page"

// FoldEps — допуск доли композиции при сравнении с краем и сгибом (0, 0,5, 1): один на ByPage
// и прилипание мест в grid.
const FoldEps = 0.001

// ByPage — шаг подгоняется по страницам: Fit = "page" у разворота (Pages = 2), и ни одно место
// не лежит через сгиб (панорама несовместима с «по страницам» — шаг подгоняется по-старому,
// как Fit = ""). Решение принимается один раз по исходным местам шага: LookTpl сбрасывает Fit у
// панорамы, после чего LookBoxes и grid.applySpreadTpl видят уже принятое решение (старая подгонка
// сжимает заход за сгиб, и повторная проверка на подогнанных местах могла бы сказать «не панорама»).
func (t SpreadTpl) ByPage() bool {
	if t.Pages != 2 || t.Fit != FitPage {
		return false
	}
	for _, b := range t.Places {
		if b.X < 0.5-FoldEps && b.X+b.W > 0.5+FoldEps {
			return false
		}
	}
	return true
}

// LookTpl — шаг t для области с отношением сторон r: места по LookBoxes и принятое решение
// ByPage. Панорама (Fit = "page" при месте через сгиб) получает Fit = "" — иначе applySpreadTpl
// повторно проверил бы уже подогнанные места. Так шаг ставят все, кто подгоняет места до
// ApplySpreadTpl (grid.lookTpl, плитка окна, проверка библиотеки).
func LookTpl(t SpreadTpl, r float64) SpreadTpl {
	by := t.ByPage()
	t.Places = LookBoxes(t, r)
	if !by {
		t.Fit = ""
	}
	return t
}

// PageFrame — кадр половины композиции t в странице page (мм): половина (Ratio/2 : 1) одним
// масштабом k вписывается по центру страницы, k — мм на единицу высоты композиции (S композиции
// = min(Ratio, 1) единиц). Ratio не число или ≤ 0 — кадр равен странице, k = её высота.
func PageFrame(t SpreadTpl, page layout.Rect) (frame layout.Rect, k float64) {
	half := t.Ratio / 2
	if math.IsNaN(half) || math.IsInf(half, 0) || half <= 0 || page.W <= 0 || page.H <= 0 {
		return page, page.H
	}
	k = min(page.W/half, page.H)
	w, h := half*k, k
	return layout.Rect{X: page.X + (page.W-w)/2, Y: page.Y + (page.H-h)/2, W: w, H: h}, k
}
