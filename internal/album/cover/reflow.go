package cover

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Reflow — шаг конвейера Editor.edit после Rescale разворотов (design 032 §3, decisions 52):
// геометрия обложки сменилась (число разворотов, Spec, страница при сторонке 0) — содержимое
// едет кусочно-линейно по зонам: что в сторонке — сдвигается и масштабируется с ней, что на
// корешке — остаётся по его центру того же размера. Зоны те же — ничего (холостая правка
// остаётся холостой). Обложку создали или сняли в этой правке — тоже ничего. Restore шаг
// пропускает, как Rescale. Стоимость — O(места + тексты + фоны) и только при смене зон.
func Reflow(p, before *model.Project) {
	if p.Cover == nil || before.Cover == nil {
		return
	}
	og, ng := before.CoverGeo(), p.CoverGeo()
	if og.X == ng.X && og.Y == ng.Y {
		return
	}
	// Сторонки — зоны 1 и 5 по X и 1 по Y: только они масштабируют размер.
	mx := axis{old: og.X[:], new: ng.X[:], panel: map[int]bool{1: true, 5: true}}
	my := axis{old: og.Y[:], new: ng.Y[:], panel: map[int]bool{1: true}}
	s := p.Cover.Sheet
	for _, pl := range s.Grid {
		movePlace(pl, mx, my)
		p.ClampPlace(s, pl)
	}
	for _, scope := range model.BdScopes() {
		if slot := s.Slot(scope); *slot != nil {
			moveBackdrop(*slot, mx, my)
			*slot = p.ClampBackdrop(s, *slot, scope)
		}
	}
	for _, t := range s.Texts {
		moveText(t, mx, my) // в лист текст вернёт FitTexts дальше в конвейере
	}
}

// axis — кусочно-линейная карта одной оси: границы зон до и после; panel — зоны, где
// размер масштабируется вместе с зоной (у корешка, расстава и заворота — 1).
type axis struct {
	old, new []float64
	panel    map[int]bool
}

// zone — зона точки x; за краями листа — крайние зоны. На общей границе нескольких зон
// (расстав или заворот нулевой ширины) first берёт левую, иначе правую: левый и верхний
// край предмета идут к началу выросшей зоны, правый и нижний — к её концу. Так фон по
// области лицевой при расставе 0 после расстава 9 начинается с расстава, как и область.
func (a axis) zone(x float64, first bool) int {
	n := len(a.old) - 1
	if first {
		for z := range n - 1 {
			if x <= a.old[z+1] {
				return z
			}
		}
		return n - 1
	}
	z := 0
	for i := 1; i < n; i++ {
		if a.old[i] <= x {
			z = i
		}
	}
	return z
}

// k — растяжение зоны; у зоны нулевой ширины 1.
func (a axis) k(z int) float64 {
	if ow := a.old[z+1] - a.old[z]; ow > 0 {
		return (a.new[z+1] - a.new[z]) / ow
	}
	return 1
}

// map1 — новое положение точки x в зоне z.
func (a axis) map1(x float64, z int) float64 { return a.new[z] + (x-a.old[z])*a.k(z) }

// at — новое положение точки x (центр, правый или нижний край).
func (a axis) at(x float64) float64 { return a.map1(x, a.zone(x, false)) }

// lo — новое положение левого или верхнего края x.
func (a axis) lo(x float64) float64 { return a.map1(x, a.zone(x, true)) }

// size — множитель размера для предмета с центром в x.
func (a axis) size(x float64) float64 {
	if z := a.zone(x, false); a.panel[z] {
		return a.k(z)
	}
	return 1
}

// movePlace: ровное место — по краям (через корешок растягивается), повёрнутое — по центру
// с размером по зоне центра.
func movePlace(pl *model.Place, mx, my axis) {
	if pl.Rot == 0 {
		x0, x1, y0, y1 := mx.lo(pl.X), mx.at(pl.X+pl.W), my.lo(pl.Y), my.at(pl.Y+pl.H)
		pl.X, pl.W, pl.Y, pl.H = x0, x1-x0, y0, y1-y0
		return
	}
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	pl.W, pl.H = pl.W*mx.size(cx), pl.H*my.size(cy)
	pl.X, pl.Y = mx.at(cx)-pl.W/2, my.at(cy)-pl.H/2
}

// moveBackdrop — как место; поворот у фона вокруг угла (X, Y), поэтому угол — от центра.
func moveBackdrop(b *model.Backdrop, mx, my axis) {
	if b.Rot == 0 {
		x0, x1, y0, y1 := mx.lo(b.X), mx.at(b.X+b.W), my.lo(b.Y), my.at(b.Y+b.H)
		b.X, b.W, b.Y, b.H = x0, x1-x0, y0, y1-y0
		return
	}
	cx, cy := b.TBox().Center()
	b.W, b.H = b.W*mx.size(cx), b.H*my.size(cy)
	dx, dy := geom.Rotate(b.Rot, b.W/2, b.H/2)
	b.X, b.Y = mx.at(cx)-dx, my.at(cy)-dy
}

// moveText: ровный — левый край (у «по тексту» — точка выравнивания: ширину пересчитает
// FitTexts от неё) и середина по высоте; повёрнутый — центр. Закреплённая ширина — по зоне
// центра, кегль не меняется.
func moveText(t *model.TextBlock, mx, my axis) {
	b := model.BoxOf(t)
	cx, cy := b.Center()
	w := t.W
	if !t.Auto {
		w = geom.Round2(t.W * mx.size(cx))
	}
	h := text.Cached(t.Text, t.Style, w).H
	if t.Rot == 0 {
		x := mx.lo(t.X)
		if ax := geom.AlignX(t.Style.Align); t.Auto && ax > 0 {
			x = mx.at(t.X+t.W*ax) - w*ax
		}
		t.X = geom.Round2(x)
		t.Y = geom.Round2(my.at(cy) - h/2)
		t.W = w
		return
	}
	dx, dy := geom.Rotate(t.Rot, w/2, h/2)
	t.X, t.Y, t.W = geom.Round2(mx.at(cx)-dx), geom.Round2(my.at(cy)-dy), w
}
