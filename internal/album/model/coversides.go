package model

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Безопасная зона сторонки обложки (127 G1, 0.26-127-2) — одна точка: вид (CoverView.Zones, Safe,
// Sides) и операции раскладки сторонки считают её здесь, а не двумя формулами. У Фабрики рамка идёт
// по линии загиба шаблона (123 Б5), иначе — по CoverSpec.Safe/SpineSafe, 0 — поле альбома.

// Сторонки обложки: Scope как у области фото фона (ScopeLeft — задняя, ScopeRight — лицевая).
const (
	coverLabelBack  = "Задняя"
	coverLabelFront = "Лицевая"
)

// CoverSide — сторонка обложки с непустой безопасной зоной. Box — зона в мм листа обложки; Places,
// Known и Alone считает область grid (CoverSideState) и кладёт в вид Editor, здесь они нулевые.
type CoverSide struct {
	Scope string      `json:"scope"` // left | right
	Label string      `json:"label"` // «Задняя» | «Лицевая»
	Box   layout.Rect `json:"box"`
	// Places — мест на сторонке; Known — форма мест узнана (стрелки ‹ › листают); Alone — в ряду
	// одна раскладка, и она стоит.
	Places int  `json:"places"`
	Known  bool `json:"known"`
	Alone  bool `json:"alone"`
}

// CoverSideState — что считает grid для вида сторонки и передаёт Editor через view.State;
// в биндинги не попадает.
type CoverSideState struct {
	Places int
	Known  bool
	Alone  bool
}

// orSafe — линия обложки v, 0 — безопасная зона альбома (как до 047).
func orSafe(v, album float64) float64 {
	if v > 0 {
		return v
	}
	return album
}

// coverLines — линии обложки: безопасные от загиба (edge) и от корешка (spine) и направляющие шаблона.
// Обложка по шаблону с линией загиба и без своих Safe/SpineSafe (Project.CoverGuide) берёт числа из
// профиля: загиб по осям, риск шаблона от него, 12 мм от корешка (123 Б5); иначе — CoverSpec.Safe и
// SpineSafe, 0 — безопасная зона альбома, как до Б5, и нулевые направляющие. Загиб, не умещающийся в
// лист, направляющие отключает: лист в 2 раза уже загиба — чужой или правленный профиль.
func coverLines(p *Project, cg CoverGeo) (gd PrintGuide, edge, spine float64) {
	s := p.Cover.Spec
	edge, spine = orSafe(s.Safe, p.Settings.Safe), orSafe(s.SpineSafe, p.Settings.Safe)
	if g, ok := p.CoverGuide(); ok && 2*g.FoldX < cg.X[7] && 2*g.FoldY < cg.Y[3] {
		return g, orSafe(g.FoldSafe, p.Settings.Safe), orSafe(g.SpineSafe, p.Settings.Safe)
	}
	return PrintGuide{}, edge, spine
}

// coverBand — прямоугольник по границам, мм листа.
func coverBand(x0, x1, y0, y1 float64) layout.Rect {
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// coverTrim — границы обреза: край сторонки (x[1], y[1]) или линия загиба шаблона gd.
func coverTrim(cg CoverGeo, gd PrintGuide) (x0, x1, y0, y1 float64) {
	x, y := cg.X, cg.Y
	if gd.HasFold() {
		w, h := x[7], y[3]
		return gd.FoldX, geom.Round2(w - gd.FoldX), gd.FoldY, geom.Round2(h - gd.FoldY)
	}
	return x[1], x[6], y[1], y[2]
}

// sideBoxes — безопасные зоны задней и лицевой сторонок как есть, без отсева пустых: edge — от
// линии загиба, spine — от края корешка или расстава (047, CoverSpec.Safe/SpineSafe). Единственная
// формула зоны: от неё строятся и зоны вида, и CoverSides.
func sideBoxes(cg CoverGeo, edge, spine float64, gd PrintGuide) [2]layout.Rect {
	x := cg.X
	x0, x1, y0, y1 := coverTrim(cg, gd)
	return [2]layout.Rect{
		coverBand(x0+edge, x[2]-spine, y0+edge, y1-edge),
		coverBand(x[5]+spine, x1-edge, y0+edge, y1-edge),
	}
}

// boxOK — зона не пустая; сторонки с W ≤ 0 или H ≤ 0 нет ни в зонах, ни в CoverSides.
func boxOK(r layout.Rect) bool { return r.W > 0 && r.H > 0 }

// CoverZones — направляющие листа обложки: сторонки, корешок, расставы, завороты за линией обреза,
// линия обреза и безопасная зона внутри каждой сторонки (спека 028). gd с линией загиба (Б5) переносит
// обрез, заворот и безопасную зону с границы сторонки на линию загиба шаблона по осям; сторонки, корешок
// и расставы — геометрия листа, они прежние. Без обложки — nil.
func (p *Project) CoverZones() []CoverZone {
	if p.Cover == nil {
		return nil
	}
	cg := p.CoverGeo()
	gd, edge, spine := coverLines(p, cg)
	x, y := cg.X, cg.Y
	w, h := x[7], y[3]
	x0, x1, y0, y1 := coverTrim(cg, gd)
	// Корешок уже MinSpine — зона (черта сгиба) остаётся, надписи нет: вертикальный текст в щели не читается (070 Д).
	spineLabel := ""
	if cg.Spine >= MinSpine {
		spineLabel = fmt.Sprintf("Корешок %s мм", MmText(cg.Spine))
	}
	z := []CoverZone{
		{Kind: "back", Rect: coverBand(x[1], x[2], y[1], y[2]), Label: coverLabelBack},
		{Kind: "front", Rect: coverBand(x[5], x[6], y[1], y[2]), Label: coverLabelFront},
		{Kind: "spine", Rect: coverBand(x[3], x[4], 0, h), Label: spineLabel},
	}
	if hinge := x[3] - x[2]; hinge > 0 {
		lbl := fmt.Sprintf("Расстав %s мм", MmText(hinge))
		z = append(z, CoverZone{Kind: "hinge", Rect: coverBand(x[2], x[3], y[1], y[2]), Label: lbl},
			CoverZone{Kind: "hinge", Rect: coverBand(x[4], x[5], y[1], y[2]), Label: lbl})
	}
	if y0 > 0 {
		for _, r := range []layout.Rect{coverBand(0, w, 0, y0), coverBand(0, w, y1, h), coverBand(0, x0, y0, y1), coverBand(x1, w, y0, y1)} {
			z = append(z, CoverZone{Kind: "wrap", Rect: r, Label: "Заворот"})
		}
	}
	z = append(z, CoverZone{Kind: "trim", Rect: coverBand(x0, x1, y0, y1)})
	for _, r := range sideBoxes(cg, edge, spine, gd) {
		if boxOK(r) {
			z = append(z, CoverZone{Kind: "safe", Rect: r})
		}
	}
	return z
}

// CoverSides — сторонки обложки с безопасной зоной: задняя, затем лицевая; сторонка с пустой зоной
// (W ≤ 0 или H ≤ 0) пропускается. Без обложки и при пустых зонах — пустой список (не nil).
func (p *Project) CoverSides() []CoverSide {
	out := []CoverSide{}
	if p.Cover == nil {
		return out
	}
	cg := p.CoverGeo()
	gd, edge, spine := coverLines(p, cg)
	meta := [2]CoverSide{{Scope: ScopeLeft, Label: coverLabelBack}, {Scope: ScopeRight, Label: coverLabelFront}}
	for i, r := range sideBoxes(cg, edge, spine, gd) {
		if boxOK(r) {
			s := meta[i]
			s.Box = r
			out = append(out, s)
		}
	}
	return out
}

// CoverGridChoices — содержимое галереи сторонки обложки (127): ряды для n мест. Доли в рядах — доли
// Box, порядок = порядок мест; UI переводит их в мм листа: Box.X + b.X·Box.W.
type CoverGridChoices struct {
	Scope   string       `json:"scope"`
	N       int          `json:"n"`      // вкладка 1..7
	Box     layout.Rect  `json:"box"`    // безопасная зона, мм листа; пустая — сторонки нет
	Places  int          `json:"places"` // мест на сторонке
	Photos  int          `json:"photos"` // фото на сторонке (для «−N» и подписей)
	ByPhoto []GridChoice `json:"byPhoto"`
	Builtin []GridChoice `json:"builtin"`
	Mine    []GridChoice `json:"mine"`
}
