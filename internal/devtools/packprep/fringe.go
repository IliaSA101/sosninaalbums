package packprep

import (
	"image"
)

// «Кайма?» (решение Т1 0.27-129-21): проверка готового файла на тёмном фоне #1e1e22. Предмет
// кладётся на тёмное, у контура берётся полоса шириной ringW px; если полоса заметно светлее
// внутренности предмета — это остаток белого фона (кайма), а не белая часть самого предмета:
// собака или облако светлы и внутри, кайма светла только по контуру.
const (
	ringW       = 2   // полоса у контура, px внутрь
	coreFrom    = 6   // внутренность: дальше этого от контура
	lightLuma   = 190 // светлее на тёмном фоне — «светлый» пиксель
	fringeMinPx = 200 // полоса меньше — предмет слишком мал для вывода
	fringeRing  = 0.35
	fringeGap   = 0.25 // светлая доля полосы больше светлой доли ядра на столько
	visibleA    = 16   // альфа, с которой пиксель виден на тёмном
)

var darkBG = [3]int{0x1e, 0x1e, 0x22}

// Fringe — итог проверки каймы.
type Fringe struct {
	Ring      int     // px полосы
	Core      int     // px внутренности
	RingLight float64 // доля светлых px в полосе
	CoreLight float64 // доля светлых px во внутренности
	Flag      bool    // «кайма?»
}

// chebyshev — расстояние до ближайшего пикселя, невидимого на тёмном (альфа < visibleA); за краем
// картинки — тоже невидимое. Два прохода по 8 соседям (точное расстояние Чебышёва), не больше 255.
func chebyshev(img *image.NRGBA) []uint8 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	d := make([]uint8, w*h)
	for y := range h {
		for x := range w {
			d[y*w+x] = initDist(img.Pix[y*img.Stride+x*4+3], x, y, w, h)
		}
	}
	// Соседи, уже пройденные при обходе вперёд (слева и сверху) и назад (справа и снизу).
	fwd := [][2]int{{-1, 0}, {0, -1}, {-1, -1}, {1, -1}}
	for y := range h {
		for x := range w {
			relax(d, w, h, x, y, fwd)
		}
	}
	back := [][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 1}}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			relax(d, w, h, x, y, back)
		}
	}
	return d
}

// initDist — начальное расстояние: 0 у невидимого, 1 у видимого на краю картинки, иначе «далеко».
func initDist(alpha uint8, x, y, w, h int) uint8 {
	switch {
	case alpha < visibleA:
		return 0
	case x == 0 || y == 0 || x == w-1 || y == h-1:
		return 1
	}
	return 255
}

// relax сводит расстояние пикселя (x, y) к соседям из списка.
func relax(d []uint8, w, h, x, y int, around [][2]int) {
	for _, o := range around {
		nx, ny := x+o[0], y+o[1]
		if nx < 0 || ny < 0 || nx >= w || ny >= h {
			continue
		}
		if n := d[ny*w+nx]; n != 255 && n+1 < d[y*w+x] {
			d[y*w+x] = n + 1
		}
	}
}

// onDark — яркость пикселя p, положенного на тёмный фон.
func onDark(p []uint8) int {
	a := int(p[3])
	var c [3]int
	for k := range 3 {
		c[k] = (int(p[k])*a + darkBG[k]*(255-a)) / 255
	}
	return (299*c[0] + 587*c[1] + 114*c[2]) / 1000
}

// CheckFringe проверяет готовый стикер или рамку на тёмном фоне.
func CheckFringe(img *image.NRGBA) Fringe {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	dist := chebyshev(img)
	var f Fringe
	var ringLight, coreLight int
	for y := range h {
		for x := range w {
			d := dist[y*w+x]
			if d == 0 {
				continue
			}
			light := onDark(img.Pix[y*img.Stride+x*4:]) >= lightLuma
			switch {
			case d <= ringW:
				f.Ring++
				if light {
					ringLight++
				}
			case d > coreFrom:
				f.Core++
				if light {
					coreLight++
				}
			}
		}
	}
	if f.Ring > 0 {
		f.RingLight = float64(ringLight) / float64(f.Ring)
	}
	if f.Core > 0 {
		f.CoreLight = float64(coreLight) / float64(f.Core)
	}
	f.Flag = f.Ring >= fringeMinPx && f.RingLight >= fringeRing && f.RingLight > f.CoreLight+fringeGap
	return f
}
