package view

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Вид обложки (032 §6): лист собирается тем же spreadView, что и развороты (подстановки,
// заглушки, Unfilled/Unnamed — тот же fields), плюс числа, направляющие и зоны броска.

// Guide — что Editor знает о файле-разметке (размер читается один раз на путь, не под каждый вид).
type Guide struct {
	Path string
	W, H int
	OK   bool
}

// coverView — вид обложки p или nil; лист считается в итоги вида v (LowDPI, пустые места, фоны).
func coverView(f *fields, st State, v *model.View) *model.CoverView {
	p := f.p
	c := p.Cover
	if c == nil || c.Sheet == nil {
		return nil
	}
	cg, g := p.CoverGeo(), p.Geo(c.Sheet)
	cv := &model.CoverView{
		Sheet: spreadView(f, c.Sheet, Laid{}), Spec: c.Spec, Preset: c.Spec.Preset,
		Spine: cg.Spine, N: cg.N, Over: cg.Over, Layout: model.CoverLayoutName(c.Layout), SpineBg: c.SpineBg,
		Zones: coverZones(cg, p.Settings.Safe), Drop: dropZones(p, c), TextMaxW: p.TextMaxW(c.Sheet),
		PxW: px(g.W, p.Settings.DPI), PxH: px(g.H, p.Settings.DPI),
	}
	for _, z := range cv.Zones {
		if z.Kind == "safe" {
			cv.Safe = append(cv.Safe, z.Rect)
		}
	}
	if c.Guide != "" {
		cv.Guide = filepath.Base(c.Guide)
		if st.Guide.Path == c.Guide && st.Guide.OK {
			cv.GuideOK = true
			cv.GuideWarn = st.Guide.H > 0 && g.H > 0 && math.Abs(float64(st.Guide.W)/float64(st.Guide.H)/(g.W/g.H)-1) > 0.01
		}
	}
	sv := &cv.Sheet
	sv.LowDPI = lowDPI(sv, v.MinDPI)
	v.LowDPI += sv.LowDPI
	for _, b := range sv.Backdrops {
		v.BackdropUsed = append(v.BackdropUsed, b.Photo)
	}
	for _, pl := range sv.Places {
		if pl.Photo == "" {
			v.EmptyPlaces++
		}
	}
	return cv
}

// px — сторона файла 00.jpg при dpi.
func px(mm float64, dpi int) int { return int(math.Round(mm / 25.4 * float64(dpi))) }

// mm — число для подписи: «14», «13,2».
func mm(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64), ".", ",", 1)
}

// coverZones — направляющие листа: сторонки, корешок, расставы, завороты за линией обреза,
// линия обреза и безопасная зона внутри каждой сторонки (поле safe, спека 028).
func coverZones(cg model.CoverGeo, safe float64) []model.CoverZone {
	x, y := cg.X, cg.Y
	w, h := x[7], y[3]
	band := func(x0, x1, y0, y1 float64) layout.Rect { return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0} }
	z := []model.CoverZone{
		{Kind: "back", Rect: band(x[1], x[2], y[1], y[2]), Label: "Задняя"},
		{Kind: "front", Rect: band(x[5], x[6], y[1], y[2]), Label: "Лицевая"},
		{Kind: "spine", Rect: band(x[3], x[4], 0, h), Label: fmt.Sprintf("Корешок %s мм", mm(cg.Spine))},
	}
	if hinge := x[3] - x[2]; hinge > 0 {
		lbl := fmt.Sprintf("Расстав %s мм", mm(hinge))
		z = append(z, model.CoverZone{Kind: "hinge", Rect: band(x[2], x[3], y[1], y[2]), Label: lbl},
			model.CoverZone{Kind: "hinge", Rect: band(x[4], x[5], y[1], y[2]), Label: lbl})
	}
	if y[1] > 0 {
		for _, r := range []layout.Rect{band(0, w, 0, y[1]), band(0, w, y[2], h), band(0, x[1], y[1], y[2]), band(x[6], w, y[1], y[2])} {
			z = append(z, model.CoverZone{Kind: "wrap", Rect: r, Label: "Заворот"})
		}
	}
	z = append(z, model.CoverZone{Kind: "trim", Rect: band(x[1], x[6], y[1], y[2])})
	for _, side := range [][2]float64{{x[1], x[2]}, {x[5], x[6]}} {
		if r := band(side[0]+safe, side[1]-safe, y[1]+safe, y[2]-safe); r.W > 0 && r.H > 0 {
			z = append(z, model.CoverZone{Kind: "safe", Rect: r})
		}
	}
	return z
}

// dropZones — пустые зоны броска фото фона по раскладке обложки: область фона (с заворотом
// и расставом), у которой фото ещё нет.
func dropZones(p *model.Project, c *model.Cover) []model.CoverZone {
	s := c.Sheet
	type zone struct{ kind, scope, label string }
	var want []zone
	switch c.Layout {
	case model.CoverWrap:
		want = []zone{{"wrap", model.ScopeSpread, "Одно фото через корешок"}}
	case model.CoverBoth:
		want = []zone{{"back", model.ScopeLeft, "Задняя"}, {"front", model.ScopeRight, "Лицевая"}}
	default:
		want = []zone{{"front", model.ScopeRight, "Лицевая"}}
	}
	out := []model.CoverZone{}
	for _, z := range want {
		if *s.Slot(z.scope) == nil {
			out = append(out, model.CoverZone{Kind: z.kind, Rect: p.BdArea(s, z.scope), Label: z.label})
		}
	}
	return out
}
