package packprep

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/sheetcut"
)

// Рамка из уголков (спека 129, решение владельца 27.10): четыре уголка набора кладутся 1:1 по углам
// квадратного прозрачного холста, стороны — пусто или тонкая линия от уголка до уголка, центр — окно.
// Программа режет рамку 9-срезом по окну (gen.FrameWindow): окно, замкнутое линией, дало бы срез в
// толщину линии и затащило бы уголки в тянущиеся части. Поэтому окно обязано быть «открытым»
// (касается края) — тогда срез по третям, а холст берётся втрое больше самого крупного уголка:
// каждый уголок целиком в своей угловой трети. Линия у каждого конца оставляет просвет gap px до
// края картинки уголка — через него окно выходит к краю холста; просвет считан на копии ≤ 1024 px
// (там программа ищет окно): меньше двух блоков копии он бы затёрся средним альфы.

// Значения параметра line рецепта.
const (
	LineNone = "none" // стороны пусты
	LineThin = "thin" // тонкая линия между уголками
)

const (
	thinFrac     = 0.006 // толщина линии без образца в уголке, доля стороны холста
	thinMaxFrac  = 0.010 // и предел толщины образца: тонкая линия, не планка
	insetFrac    = 0.025 // отступ линии от края холста, если в уголках линии нет
	stubMaxFrac  = 0.025 // «образец линии» у края уголка тоньше этой доли длины края
	winAnalyzePx = 1024  // длинная сторона копии, на которой программа ищет окно
	darkLuma     = 90    // образец линии темнее — обводка стиля «контур»
	solidAlpha   = 200   // «плотный» пиксель для цвета и образца линии
	maxStretchOK = 5.0   // «тянется», %: больше — ошибка отчёта (рамка из уголков должна быть ≈ 0)
)

// cornerFrame — собранная рамка и её разбор.
type cornerFrame struct {
	img  *image.NRGBA
	info *sheetcut.FrameInfo
	line string
	ends [4][2]color.NRGBA // цвета концов линии сторон (сверху, снизу, слева, справа); концы разные — градиент
}

// cornerPaths — файлы четырёх уголков по пути tl: «…-tl.png» → «…-tr.png», «…-bl.png», «…-br.png».
func cornerPaths(tl string) ([4]string, error) {
	base, ok := strings.CutSuffix(tl, "-tl.png")
	if !ok {
		return [4]string{}, fmt.Errorf("src %q: уголок сверху слева, имя …-tl.png (остальные — -tr, -bl, -br рядом)", tl)
	}
	return [4]string{tl, base + "-tr.png", base + "-bl.png", base + "-br.png"}, nil
}

// toNRGBA — копия картинки в NRGBA с началом в нуле.
func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// flipImg — зеркало по горизонтали (h) и/или по вертикали (v): любой уголок приводится к «сверху слева».
func flipImg(src *image.NRGBA, h, v bool) *image.NRGBA {
	w, ht := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(src.Bounds())
	for y := range ht {
		for x := range w {
			sx, sy := x, y
			if h {
				sx = w - 1 - x
			}
			if v {
				sy = ht - 1 - y
			}
			copy(dst.Pix[y*dst.Stride+x*4:][:4], src.Pix[sy*src.Stride+sx*4:][:4])
		}
	}
	return dst
}

// stub — «образец линии» у внутреннего края уголка: полоска непрозрачных px, упирающаяся в край.
type stub struct {
	at    float64     // середина от внешнего края холста, px
	thick int         // толщина, px
	col   color.NRGBA // средний цвет
	ok    bool
}

// edgeStub ищет у края картинки первую от внешней стороны полоску: «плотные» px (альфа ≥ solidAlpha)
// в приграничной полосе глубиной ≥ 2 px, тоньше stubMaxFrac длины края. right — правый край (линия
// идёт горизонтально), иначе нижний (вертикально).
func edgeStub(img *image.NRGBA, right bool) stub {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	length, depth := h, min(max(2, w/200), w)
	if !right {
		length, depth = w, min(max(2, h/200), h)
	}
	// px на линии k, слое d от края; нет такого px — nil.
	at := func(k, d int) []uint8 {
		x, y := w-1-d, k
		if !right {
			x, y = k, h-1-d
		}
		if x < 0 || y < 0 || x >= w || y >= h {
			return nil
		}
		return img.Pix[y*img.Stride+x*4 : y*img.Stride+x*4+4]
	}
	solid := func(k int) bool {
		for d := range depth {
			if p := at(k, d); p != nil && p[3] >= solidAlpha {
				return true
			}
		}
		return false
	}
	for i := 0; i < length; i++ {
		if !solid(i) {
			continue
		}
		j := i
		for j < length && solid(j) {
			j++
		}
		if float64(j-i) <= stubMaxFrac*float64(length) {
			c, ok := meanSolid(at, i, j, depth)
			return stub{at: float64(i) + float64(j-i)/2, thick: j - i, col: c, ok: ok}
		}
		i = j
	}
	return stub{}
}

// meanSolid — средний цвет плотных px линий k0…k1 (слои 0…depth).
func meanSolid(at func(k, d int) []uint8, k0, k1, depth int) (color.NRGBA, bool) {
	var sum [3]int
	n := 0
	for k := k0; k < k1; k++ {
		for d := range depth {
			if p := at(k, d); p != nil && p[3] >= solidAlpha {
				sum[0], sum[1], sum[2], n = sum[0]+int(p[0]), sum[1]+int(p[1]), sum[2]+int(p[2]), n+1
			}
		}
	}
	if n == 0 {
		return color.NRGBA{}, false
	}
	return color.NRGBA{avgByte(sum[0], n), avgByte(sum[1], n), avgByte(sum[2], n), 255}, true
}

// avgByte — среднее восьмибитных значений: сумма sum из n слагаемых ≤ 255 каждое.
func avgByte(sum, n int) uint8 {
	return uint8(sum / n) //nolint:gosec // среднее значений ≤ 255 не выходит из байта
}

// dominantColor — доминирующий непрозрачный цвет уголков: мода по ячейкам 16×16×16 среди плотных px,
// без тёмной обводки и почти белого (иначе «доминирует» контур или заливка); не нашлось — мода по всем.
func dominantColor(imgs [4]*image.NRGBA) color.NRGBA {
	for _, strict := range []bool{true, false} {
		type acc struct{ r, g, b, n int }
		cells := map[int]*acc{}
		for _, img := range imgs {
			for i := 0; i+3 < len(img.Pix); i += 4 {
				p := img.Pix[i : i+4]
				if p[3] < solidAlpha {
					continue
				}
				luma := (299*int(p[0]) + 587*int(p[1]) + 114*int(p[2])) / 1000
				if strict && (luma < 70 || min(p[0], p[1], p[2]) > 235) {
					continue
				}
				key := int(p[0]>>4)<<8 | int(p[1]>>4)<<4 | int(p[2]>>4)
				a := cells[key]
				if a == nil {
					a = &acc{}
					cells[key] = a
				}
				a.r, a.g, a.b, a.n = a.r+int(p[0]), a.g+int(p[1]), a.b+int(p[2]), a.n+1
			}
		}
		keys := make([]int, 0, len(cells))
		for k := range cells {
			keys = append(keys, k)
		}
		slices.Sort(keys) // при равенстве — меньший ключ: прогон одинаков
		var best *acc
		for _, k := range keys {
			if a := cells[k]; best == nil || a.n > best.n {
				best = a
			}
		}
		if best != nil {
			return color.NRGBA{avgByte(best.r, best.n), avgByte(best.g, best.n), avgByte(best.b, best.n), 255}
		}
	}
	return color.NRGBA{0x80, 0x80, 0x80, 255}
}

// sideLine — линия одной стороны: расстояние середины от края холста.
func sideLine(stubs ...stub) (at int, found bool) {
	for _, s := range stubs {
		if s.ok {
			at, found = max(at, int(math.Round(s.at))), true
		}
	}
	return at, found
}

// buildCornerFrame собирает рамку из уголков tl, tr, bl, br (порядок quarterSuffix). Цвет col —
// «#rrggbb» в рецепте или пусто (доминирующий цвет уголков).
func buildCornerFrame(corners [4]*image.NRGBA, line string, col *color.NRGBA) (*cornerFrame, error) {
	if line != LineNone && line != LineThin {
		return nil, fmt.Errorf("line %q: нужно none или thin", line)
	}
	base := 0
	for _, c := range corners {
		base = max(base, c.Bounds().Dx(), c.Bounds().Dy())
	}
	if base < 8 {
		return nil, errors.New("уголки меньше 8 px")
	}
	// Образцы линии: уголок приведён к «сверху слева», расстояние — от внешнего края холста.
	norm := [4]*image.NRGBA{corners[0], flipImg(corners[1], true, false), flipImg(corners[2], false, true), flipImg(corners[3], true, true)}
	hs, vs := [4]stub{}, [4]stub{}
	for i, n := range norm {
		hs[i], vs[i] = edgeStub(n, true), edgeStub(n, false)
	}
	unit := 3 * base
	inset := int(math.Round(insetFrac * float64(unit)))
	top, okT := sideLine(hs[0], hs[1])
	bottom, okB := sideLine(hs[2], hs[3])
	left, okL := sideLine(vs[0], vs[2])
	right, okR := sideLine(vs[1], vs[3])
	dist := [4]int{top, bottom, left, right} // сверху, снизу, слева, справа
	for i, ok := range [4]bool{okT, okB, okL, okR} {
		if !ok {
			dist[i] = inset
		}
	}
	// Сдвиг уголка, чтобы образец лёг на линию стороны: нет образца — в угол без сдвига.
	shift := func(d int, s stub) int {
		if !s.ok {
			return 0
		}
		return max(0, d-int(math.Round(s.at)))
	}
	off := [4]image.Point{ // сдвиг от своих сторон холста: x — от левой/правой, y — от верхней/нижней
		{shift(dist[2], vs[0]), shift(dist[0], hs[0])},
		{shift(dist[3], vs[1]), shift(dist[0], hs[1])},
		{shift(dist[2], vs[2]), shift(dist[1], hs[2])},
		{shift(dist[3], vs[3]), shift(dist[1], hs[3])},
	}
	third := 0
	for i, c := range corners {
		third = max(third, c.Bounds().Dx()+off[i].X, c.Bounds().Dy()+off[i].Y)
	}
	side := 3 * third
	if side > sheetcut.MaxSidePx {
		return nil, fmt.Errorf("рамка %d px больше предела импорта %d: уголки крупнее %d px", side, sheetcut.MaxSidePx, sheetcut.MaxSidePx/3)
	}
	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	pos := [4]image.Point{}
	for i, c := range corners {
		w, h := c.Bounds().Dx(), c.Bounds().Dy()
		x, y := off[i].X, off[i].Y
		if i%2 == 1 {
			x = side - off[i].X - w
		}
		if i >= 2 {
			y = side - off[i].Y - h
		}
		pos[i] = image.Pt(x, y)
		draw.Draw(img, image.Rect(x, y, x+w, y+h), c, image.Point{}, draw.Over)
	}
	cf := &cornerFrame{img: img, line: line}
	if line == LineThin {
		cf.drawLine(corners, pos, dist, hs, vs, col)
	}
	cf.info = sheetcut.AnalyzeFrame(img)
	return cf, cf.check()
}

// drawLine рисует тонкую линию четырёх сторон от уголка до уголка с просветом gap у каждого конца.
func (cf *cornerFrame) drawLine(corners [4]*image.NRGBA, pos [4]image.Point, dist [4]int, hs, vs [4]stub, col *color.NRGBA) {
	side := cf.img.Bounds().Dx()
	t := int(math.Round(thinFrac * float64(side)))
	outline, hasOutline := darkStubs(hs[:], vs[:])
	var thick []int
	for _, s := range append(hs[:], vs[:]...) {
		if s.ok && (!hasOutline || lumaOf(s.col) < darkLuma) {
			thick = append(thick, s.thick)
		}
	}
	if len(thick) > 0 {
		slices.Sort(thick)
		t = min(thick[len(thick)/2], int(math.Round(thinMaxFrac*float64(side))))
	}
	t = max(t, 3)
	dom := dominantColor(corners)
	if col != nil {
		dom = *col
	}
	// Конец линии берёт цвет образца в уголке (там линия уже нарисована); нет образца или цвет задан
	// в рецепте — доминирующий цвет уголков. Стиль «контур» (тёмных образцов не меньше половины) —
	// одна тёмная линия везде: светлый «образец» там случайно попавшая заливка, а не линия.
	if hasOutline {
		dom = outline
	}
	endCol := func(s stub) color.NRGBA {
		if col == nil && !hasOutline && s.ok {
			return s.col
		}
		return dom
	}
	cf.ends = [4][2]color.NRGBA{{endCol(hs[0]), endCol(hs[1])}, {endCol(hs[2]), endCol(hs[3])}, {endCol(vs[0]), endCol(vs[2])}, {endCol(vs[1]), endCol(vs[3])}}
	gap := 2*((side+winAnalyzePx-1)/winAnalyzePx) + 2
	var r [4]image.Rectangle // прямоугольники уголков на холсте
	for i, c := range corners {
		r[i] = image.Rectangle{Min: pos[i], Max: pos[i].Add(c.Bounds().Size())}
	}
	half := t / 2
	// fill красит полосу: цвет плавно идёт от конца a к концу b вдоль стороны (horiz — вдоль X).
	fill := func(x0, y0, x1, y1 int, ends [2]color.NRGBA, horiz bool) {
		rc := image.Rect(x0, y0, x1, y1).Intersect(cf.img.Bounds())
		for y := rc.Min.Y; y < rc.Max.Y; y++ {
			for x := rc.Min.X; x < rc.Max.X; x++ {
				k, n := y-y0, y1-y0
				if horiz {
					k, n = x-x0, x1-x0
				}
				cf.img.SetNRGBA(x, y, lerpColor(ends[0], ends[1], float64(k)/float64(max(n-1, 1))))
			}
		}
	}
	top, bottom, left, right := max(dist[0], half), side-max(dist[1], half), max(dist[2], half), side-max(dist[3], half)
	fill(r[0].Max.X+gap, top-half, r[1].Min.X-gap, top-half+t, cf.ends[0], true)
	fill(r[2].Max.X+gap, bottom-half, r[3].Min.X-gap, bottom-half+t, cf.ends[1], true)
	fill(left-half, r[0].Max.Y+gap, left-half+t, r[2].Min.Y-gap, cf.ends[2], false)
	fill(right-half, r[1].Max.Y+gap, right-half+t, r[3].Min.Y-gap, cf.ends[3], false)
}

// check — рамка должна резаться программой как задумано: окно найдено и открыто (срез по третям),
// «тянется» не больше maxStretchOK. Окно проверяется и на копии ≤ 1024 px, как в программе.
func (cf *cornerFrame) check() error {
	fi := cf.info
	if !fi.Window || !fi.Open {
		return errors.New("окно рамки не открылось: срез пошёл бы по линии, а не по третям")
	}
	if cp := alphaCopy(cf.img); cp != nil {
		if pi := sheetcut.AnalyzeFrame(cp); !pi.Window || !pi.Open {
			return errors.New("окно рамки на копии 1024 px не открылось (просвет у линии затёрт): программа срежет по линии")
		}
	}
	if fi.Stretch > maxStretchOK {
		return fmt.Errorf("тянется %.1f %%: уголок вышел за свою треть", fi.Stretch)
	}
	return nil
}

// alphaCopy — рамка на копии ≤ winAnalyzePx по длинной стороне со средней по блоку альфой
// (как gen.alphaCopy); меньше предела — nil: копия совпала бы с самой рамкой.
func alphaCopy(img *image.NRGBA) *image.NRGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if max(w, h) <= winAnalyzePx {
		return nil
	}
	cw, ch := max((w*winAnalyzePx+max(w, h)/2)/max(w, h), 1), max((h*winAnalyzePx+max(w, h)/2)/max(w, h), 1)
	sum, cnt := make([]uint32, cw*ch), make([]uint32, cw*ch)
	for y := range h {
		for x := range w {
			k := (y*ch/h)*cw + x*cw/w
			sum[k] += uint32(img.Pix[y*img.Stride+x*4+3])
			cnt[k]++
		}
	}
	cp := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	for k := range sum {
		cp.Pix[k*4+3] = uint8((sum[k] + cnt[k]/2) / cnt[k]) //nolint:gosec // среднее восьмибитных значений ≤ 255
	}
	return cp
}

// doCornerFrame собирает рамку из четырёх уголков набора.
func (r *runner) doCornerFrame(idx int, it Item) error {
	paths, err := cornerPaths(it.Src)
	if err != nil {
		return err
	}
	var corners [4]*image.NRGBA
	for i, p := range paths {
		sf, err := readSrc(r.cfg.Src, p)
		if err != nil {
			return err
		}
		img, err := sf.decode()
		if err != nil {
			return fmt.Errorf("%s: %w", path.Base(p), err)
		}
		if i == 0 {
			r.corners(it, img)
		}
		corners[i] = toNRGBA(img)
	}
	var col *color.NRGBA
	if it.Color != "" {
		c, err := parseHexColor(it.Color)
		if err != nil {
			return err
		}
		col = &c
	}
	cf, err := buildCornerFrame(corners, it.Line, col)
	if err != nil {
		return err
	}
	p := piece{file: it.Out, name: it.Name, img: cf.img}
	if cf.info.Window {
		st := cf.info.Stretch
		p.stretch = &st
	}
	return r.emit(idx, it, p)
}

// lumaOf — яркость цвета, 0…255.
func lumaOf(c color.NRGBA) int { return (299*int(c.R) + 587*int(c.G) + 114*int(c.B)) / 1000 }

// darkStubs — средний цвет тёмных образцов линии, если их не меньше половины найденных.
func darkStubs(lists ...[]stub) (color.NRGBA, bool) {
	var sum [3]int
	dark, all := 0, 0
	for _, l := range lists {
		for _, s := range l {
			if !s.ok {
				continue
			}
			all++
			if lumaOf(s.col) < darkLuma {
				dark++
				sum[0], sum[1], sum[2] = sum[0]+int(s.col.R), sum[1]+int(s.col.G), sum[2]+int(s.col.B)
			}
		}
	}
	if dark == 0 || 2*dark < all {
		return color.NRGBA{}, false
	}
	return color.NRGBA{avgByte(sum[0], dark), avgByte(sum[1], dark), avgByte(sum[2], dark), 255}, true
}

// lerpColor — цвет между a и b на доле f (0…1).
func lerpColor(a, b color.NRGBA, f float64) color.NRGBA {
	mix := func(x, y uint8) uint8 { return uint8(math.Round(float64(x) + (float64(y)-float64(x))*f)) } //nolint:gosec // между двумя байтами остаётся байтом
	return color.NRGBA{mix(a.R, b.R), mix(a.G, b.G), mix(a.B, b.B), 255}
}

// parseHexColor разбирает «#rrggbb».
func parseHexColor(s string) (color.NRGBA, error) {
	if len(s) != 7 || s[0] != '#' {
		return color.NRGBA{}, fmt.Errorf("color %q: нужно #rrggbb", s)
	}
	var v [3]uint8
	for i := range v {
		hi, lo := hexDigit(s[1+2*i]), hexDigit(s[2+2*i])
		if hi < 0 || lo < 0 {
			return color.NRGBA{}, fmt.Errorf("color %q: нужно #rrggbb", s)
		}
		v[i] = uint8(hi<<4 | lo) //nolint:gosec // две шестнадцатеричные цифры ≤ 255
	}
	return color.NRGBA{v[0], v[1], v[2], 255}, nil
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
