package sheetcut

import (
	"fmt"
	"image"
	"math"
)

// Порог «тянется»: больше — флаг в отчёте (не отсев: отбор — лист рамок на показе, 0.27-129-14).
const (
	StretchFlagPct = 20.0
	opaqueMin      = 128 // непрозрачное: альфа не меньше (как «прозрачное < 128» в gen.FrameWindow)
	maxCut         = 0.45
	unevenTol      = 20 // ≈ 8 %: допуск альфы и цвета до «отличается от медианной линии полосы»
)

// FrameInfo — итог разбора рамки по тем же правилам, что у программы (gen.FrameWindow, 9-срез).
type FrameInfo struct {
	Trim    [4]int     `json:"trim"`    // срезано полей: слева, сверху, справа, снизу, px
	Window  bool       `json:"window"`  // окно найдено
	Open    bool       `json:"open"`    // окно касается края: срез по третям
	Cut     [4]float64 `json:"cut"`     // срез l, t, r, b — доли сторон
	Stretch float64    `json:"stretch"` // тянется, %; −1 — не считали (окна нет)
}

// analyzeFrame разбирает готовую (обрезанную) рамку: окно, срез, мера «тянется».
func analyzeFrame(img *image.NRGBA, trim [4]int) *FrameInfo {
	fi := &FrameInfo{Trim: trim, Stretch: -1}
	cut, open, ok := frameWindow(img)
	if !ok {
		return fi
	}
	fi.Window, fi.Open, fi.Cut = true, open, cut
	fi.Stretch = stretchShare(img, cut)
	return fi
}

// warnings — подозрения по рамке; тянется > 20 % — флаг, не отсев.
func (fi *FrameInfo) warnings() []string {
	var w []string
	switch {
	case !fi.Window:
		w = append(w, "окно не найдено: центр непрозрачен или окно меньше 5 % площади — программа рамку отвергнет")
	case fi.Open:
		w = append(w, "открытая рамка: окно касается края, срез по третям")
	}
	if fi.Stretch > StretchFlagPct {
		w = append(w, fmt.Sprintf("тянется %.0f %%: центр и неровные полосы 9-среза исказятся на месте другой пропорции", fi.Stretch))
	}
	return w
}

// frameWindow — окно картинки-рамочки, как gen.FrameWindow: прозрачное (альфа < 128) от центра,
// 4-связно, не меньше 5 % площади; касается края — «открытая», срез по третям. Срез — доли l, t, r, b,
// не больше 0,45 (gen.MaxCut).
func frameWindow(img *image.NRGBA) (cut [4]float64, open, ok bool) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w < 3 || h < 3 {
		return cut, false, false
	}
	n, box := floodWindow(img)
	if float64(n) < windowMinPct*float64(w*h) {
		return cut, false, false
	}
	if box.Min.X == 0 || box.Min.Y == 0 || box.Max.X == w || box.Max.Y == h {
		return [4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}, true, true
	}
	fw, fh := float64(w), float64(h)
	c := [4]float64{float64(box.Min.X) / fw, float64(box.Min.Y) / fh, 1 - float64(box.Max.X)/fw, 1 - float64(box.Max.Y)/fh}
	for i := range c {
		c[i] = math.Round(min(max(c[i], 0), maxCut)*1e4) / 1e4
	}
	return c, false, true
}

// floodWindow — заливка прозрачного (альфа < 128) от центра отрезками строк, 4-связно: число
// пикселей области и её габарит; центр непрозрачен — 0. Маска — байт на пиксель.
func floodWindow(img *image.NRGBA) (n int, box image.Rectangle) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	m := make([]uint8, w*h) // 1 — прозрачное, не залитое; 2 — залитое
	for i := range m {
		if img.Pix[i*4+3] < opaqueMin {
			m[i] = 1
		}
	}
	start := h/2*w + w/2
	if m[start] != 1 {
		return 0, box
	}
	box = image.Rect(w/2, h/2, w/2+1, h/2+1)
	stack := []int{start}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if m[p] != 1 {
			continue
		}
		y := p / w
		l, r := p%w, p%w
		for l > 0 && m[y*w+l-1] == 1 {
			l--
		}
		for r < w-1 && m[y*w+r+1] == 1 {
			r++
		}
		for x := l; x <= r; x++ {
			m[y*w+x] = 2
		}
		n += r - l + 1
		box = box.Union(image.Rect(l, y, r+1, y+1))
		for _, ny := range [2]int{y - 1, y + 1} {
			if ny >= 0 && ny < h {
				stack = seedRuns(stack, m[ny*w:ny*w+w], ny*w, l, r)
			}
		}
	}
	return n, box
}

// seedRuns кладёт в стек начало каждого незалитого прозрачного отрезка строки под или над отрезком l…r.
func seedRuns(stack []int, row []uint8, base, l, r int) []int {
	for x := l; x <= r; x++ {
		if row[x] == 1 && (x == l || row[x-1] != 1) {
			stack = append(stack, base+x)
		}
	}
	return stack
}

// stretchShare — «тянется, %» после обрезки, по сетке 9-среза (cut): доля непрозрачных px, которые
// растяжение на место другой пропорции исказит. Центр тянется по обеим осям — каждый непрозрачный px
// в счёт. Боковые полосы тянутся только вдоль своей стороны — в счёт лишь неровное вдоль неё
// (пиксель отличается от медианной линии полосы по альфе или цвету): ровная планка не штрафуется,
// растягивание ей не вредит. Углы идут 1:1 — не в счёт.
func stretchShare(img *image.NRGBA, cut [4]float64) float64 {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x0, y0 := int(math.Round(cut[0]*float64(w))), int(math.Round(cut[1]*float64(h)))
	x1, y1 := w-int(math.Round(cut[2]*float64(w))), h-int(math.Round(cut[3]*float64(h)))
	if x0 >= x1 || y0 >= y1 {
		return 100
	}
	total := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] >= opaqueMin {
			total++
		}
	}
	if total == 0 {
		return 0
	}
	n := opaqueIn(img, image.Rect(x0, y0, x1, y1))
	// Верх и низ тянутся по X: линии — строки полосы; лево и право тянутся по Y: линии — столбцы.
	n += unevenAlong(img, image.Rect(x0, 0, x1, y0), true)
	n += unevenAlong(img, image.Rect(x0, y1, x1, h), true)
	n += unevenAlong(img, image.Rect(0, y0, x0, y1), false)
	n += unevenAlong(img, image.Rect(x1, y0, w, y1), false)
	return 100 * float64(n) / float64(total)
}

// opaqueIn — непрозрачных px в прямоугольнике.
func opaqueIn(img *image.NRGBA, r image.Rectangle) int {
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.Pix[y*img.Stride+x*4+3] >= opaqueMin {
				n++
			}
		}
	}
	return n
}

// lineStat — медиана альфы и цвета непрозрачных px одной линии полосы.
type lineStat struct {
	hist  [4][256]int // R, G, B, A
	count [2]int      // всего px, непрозрачных
}

func (s *lineStat) reset() { *s = lineStat{} }

func (s *lineStat) add(p []uint8) {
	s.count[0]++
	s.hist[3][p[3]]++
	if p[3] >= opaqueMin {
		s.count[1]++
		for c := range 3 {
			s.hist[c][p[c]]++
		}
	}
}

// median — медиана канала c среди учтённых px (для цвета — среди непрозрачных).
func (s *lineStat) median(c int) int {
	n := s.count[1]
	if c == 3 {
		n = s.count[0]
	}
	acc := 0
	for v, k := range s.hist[c] {
		acc += k
		if acc*2 >= n {
			return v
		}
	}
	return 0
}

// unevenAlong считает непрозрачные px полосы r, отличающиеся от медианной линии: rows — линии идут
// по строкам (полоса тянется по X), иначе по столбцам. Медиана берётся на каждой линии отдельно:
// пиксель сравнивается с серединой своей линии, поэтому однородная планка даёт 0 при любой её
// толщине и рисунке поперёк.
func unevenAlong(img *image.NRGBA, r image.Rectangle, rows bool) int {
	if r.Empty() {
		return 0
	}
	var st lineStat
	lines, length := r.Dx(), r.Dy()
	if rows {
		lines, length = r.Dy(), r.Dx()
	}
	at := func(line, k int) []uint8 {
		x, y := r.Min.X+k, r.Min.Y+line
		if !rows {
			x, y = r.Min.X+line, r.Min.Y+k
		}
		o := y*img.Stride + x*4
		return img.Pix[o : o+4]
	}
	n := 0
	for line := range lines {
		st.reset()
		for k := range length {
			st.add(at(line, k))
		}
		ma := st.median(3)
		var mc [3]int
		if st.count[1] > 0 {
			mc = [3]int{st.median(0), st.median(1), st.median(2)}
		}
		for k := range length {
			p := at(line, k)
			if p[3] < opaqueMin {
				continue
			}
			if absInt(int(p[3])-ma) > unevenTol || colorFar(p, mc) {
				n++
			}
		}
	}
	return n
}

func colorFar(p []uint8, m [3]int) bool {
	for c := range 3 {
		if absInt(int(p[c])-m[c]) > unevenTol {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// AnalyzeFrame — разбор готовой рамки теми же правилами, что у sheetcut (окно, срез, «тянется»);
// для режимов packprep, которые собирают рамку сами (рамка из уголков).
func AnalyzeFrame(img *image.NRGBA) *FrameInfo {
	return analyzeFrame(img, [4]int{})
}
