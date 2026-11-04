package vigsheet

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Шапка и подписи виньетки (031, 038, 049, 164): стиль и кегль новой шапки, её снятие и подгонка
// при смене шаблона, стиль подписей от шага. Тела правок — vignette.go.

// dropHead — смена шаблона (fresh) на шаблон без шапки (у плана pl её нет) снимает шапку листа,
// если её текст шаблонный (решение №44 0.10: «убрать насовсем — шаблон без шапки»), и v.Head;
// правленая пользователем остаётся; шаблонный и текст шага макета (headText, 164), и VigHeadText.
// Поля шапки (Vars) не трогаются.
func dropHead(s *model.Spread, v *model.Vignette, pl VigPlan, fresh bool, headText string) {
	if !fresh || pl.Head != nil {
		return
	}
	if h := s.VigHead(v.Head); h != nil && (h.Text == model.VigHeadText || headText != "" && h.Text == headText) {
		s.Texts = slices.DeleteFunc(s.Texts, func(t *model.TextBlock) bool { return t == h })
		v.Head = ""
	}
}

// refitHead — кегль шапки h при смене шаблона (049 §3.2): кегль «по полосе» прежнего плана
// was встаёт по полосе нового cur; свой только уменьшается — до кегля новой полосы, если она
// мельче прежней (на такой же полосе свой кегль влезал, решение №177). Прежней полосы нет —
// свой не больше кегля новой. Гарнитура, цвет, выравнивание и текст — прежние.
func refitHead(h *model.TextBlock, was, cur VigPlan) {
	if cur.Head == nil {
		return
	}
	size, old := headStyle(*cur.Head, cur.HeadCell, cur.HeadFit).Size, math.Inf(1)
	if was.Head != nil {
		old = headStyle(*was.Head, was.HeadCell, was.HeadFit).Size
	}
	switch {
	case h.Style.Size == old:
		h.Style.Size = size
	case size < old:
		h.Style.Size = min(h.Style.Size, size)
	}
}

// headStyleOr — стиль новой шапки: шаблона, иначе по высоте полосы; стиль шага макета (кегль 0) —
// по высоте полосы с его гарнитурой и цветом.
func (pl VigPlan) headStyleOr() model.TextStyle {
	base := headStyle(*pl.Head, pl.HeadCell, pl.HeadFit)
	switch st := pl.HeadStyle; {
	case st == nil:
		return base
	case st.Size == 0:
		return withFontColor(base, *st)
	}
	return *pl.HeadStyle
}

// withFontColor — base с гарнитурой и цветом из over (пустое поле — как у base); зажим сбрасывает
// жирный и курсив, которых у новой гарнитуры нет.
func withFontColor(base, over model.TextStyle) model.TextStyle {
	if over.Font != "" {
		base.Font = over.Font
	}
	if over.Color != "" {
		base.Color = over.Color
	}
	return model.ClampStyle(base)
}

// ownCaps — стиль подписей листа s от шага c: кегль 0 (шаг макета, 164) — стиль листа (нет —
// VigCaps по формату) с гарнитурой и цветом шага; иначе — стиль своего шаблона как есть (копия).
func ownCaps(p *model.Project, s *model.Spread, c model.SheetCaps) *model.SheetCaps {
	if c.Style.Size != 0 {
		return &c
	}
	base := model.VigCaps(p.Geo(s).H)
	if s.Caps != nil {
		base = *s.Caps
	}
	base.Style = withFontColor(base.Style, c.Style)
	return &base
}

// headStyle — стиль новой шапки: шрифт по умолчанию, по центру, кегль от высоты полосы band
// (две строки в полосе), 12…48 пт. Шапка в ряду (cell, 049 §3.2) высокая и узкая: кегль — fit,
// замер ядра (VigPlan.HeadFit); без замера (0) — ещё и не больше ширины/9.
func headStyle(band layout.Rect, cell bool, fit float64) model.TextStyle {
	mm := band.H / 2.6
	if cell {
		mm = min(mm, band.W/9)
	}
	st := model.PlainStyle()
	st.Size = math.Round(geom.ClampF(mm/0.3528, 12, 48))
	if cell && fit > 0 {
		st.Size = fit
	}
	return model.ClampStyle(st)
}

// putHead ставит рамку шапки в полосу band (угол и ширина полосы); текст и стиль не
// трогаются (031 §5, шаг 6). Кегль новой шапки — от высоты полосы (две строки), поэтому
// текст полосу и заполняет; правленый текст «Применить» не сдвигает.
func putHead(t *model.TextBlock, band layout.Rect) {
	t.X, t.Y, t.W, t.Auto = geom.Round2(band.X), geom.Round2(band.Y), geom.Round2(band.W), false
}

// applyOwnStyle — оформление st своего шаблона или шага макета для нового листа или смены шаблона:
// подписи листа s, подпись учителя и форма мест v; возвращает стиль шапки (nil — не задан).
func applyOwnStyle(p *model.Project, s *model.Spread, v *model.Vignette, st *model.VigStyle) *model.TextStyle {
	if c := st.Caps; c != nil {
		s.Caps = ownCaps(p, s, *c)
	}
	if st.TText != "" {
		v.TText = st.TText
	}
	if st.Shape != "" || st.Round != 0 {
		v.Shape, v.Round = st.Shape, st.Round
	}
	return st.Head
}

// restyleHead — стиль уже стоящей шапки при оформлении st: кегль 0 (шаг макета) — прежний стиль с
// гарнитурой и цветом из st, иначе стиль st целиком (свой шаблон 038).
func restyleHead(cur, st model.TextStyle) model.TextStyle {
	if st.Size == 0 {
		return withFontColor(cur, st)
	}
	return st
}
