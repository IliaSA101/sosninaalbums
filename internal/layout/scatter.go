package layout

// Корень Z — «Разброс» (спека 105): 1–5 фото одной высоты в ряд с равным воздухом, у каждого
// свой уровень по вертикали. Разбор — keyparse.go. Числа ниже входят в формат v18: после
// выпуска менять только новой буквой или версией. Доли — двоичные (10/16, 1/16), чтобы
// на arm64 (FMA) геометрия не расходилась; уровни 1/3 и 2/3 двоичными не бывают.

const (
	// kappaN — фото n<i> от высоты его ряда («центр крупнее» с боками покрупнее m).
	kappaN = 0.7
	// scatterH — высота фото Z от высоты области.
	scatterH = 10.0 / 16
	// scatterAir — наименьший воздух между фото и у краёв, доля высоты области.
	scatterAir = 1.0 / 16
)

// zLevels — доли свободной высоты для уровней a…g (keyTree.lvl − 1).
var zLevels = [7]float64{0, 0.25, 1.0 / 3, 0.5, 2.0 / 3, 0.75, 1}

// fitScatter — Z: k фото высоты h в ряд, воздух поровну у краёв и между фото (k+1 промежутков,
// не меньше max(gap, H/16)); y = верх box + свободная высота (H − h) · доля уровня. Блок занимает
// всю область, прижимать его некуда — ax/ay не нужны. Высота для оценки — от верха самого
// высокого до низа самого низкого фото, глубина 1. false — фото меньше 1 мм.
func fitScatter(kt *keyTree, aspects []float64, box Rect, gap float64, out []Rect) (float64, int, bool) {
	k := float64(len(kt.kids))
	sum := 0.0
	for _, l := range kt.kids {
		sum += aspects[l.leaf]
	}
	air0 := max(gap, float64(box.H*scatterAir))
	h := min(float64(box.H*scatterH), (box.W-float64((k+1)*air0))/sum)
	if !(h >= 1) { // и NaN
		return 0, 0, false
	}
	air := (box.W - float64(h*sum)) / (k + 1)
	free := box.H - h
	x := box.X + air
	top, bottom := box.Y+box.H, box.Y
	for _, l := range kt.kids {
		w := float64(h * aspects[l.leaf])
		if w < 1 {
			return 0, 0, false
		}
		y := box.Y + float64(free*zLevels[l.lvl-1])
		out[l.leaf] = Rect{x, y, w, h}
		top, bottom = min(top, y), max(bottom, y+h)
		x += w + air
	}
	return bottom - top, 1, true
}
