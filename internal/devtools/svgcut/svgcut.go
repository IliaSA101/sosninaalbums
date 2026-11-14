package svgcut

import (
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"sync"
)

// Умолчания и пределы нарезки.
const (
	DefaultGap  = 2.0  // пути ближе — один предмет, в координатах листа
	DefaultSide = 1600 // длинная сторона предмета, px
	MaxSide     = 4096 // предел импорта своих стикеров (как decor.MaxStickerSide)
	MinSidePx   = 600  // меньше — предупреждение (для вектора только при маленьком side)
	frameMargin = 2.0  // поле рамки при растре: край сглаживания не обрезается
	clumpRatio  = 1.8  // рамка больше медианной во столько раз — «слиплось?»
	workers     = 4    // одновременных запусков браузера
)

// Options — параметры нарезки одного листа.
type Options struct {
	Gap     float64      // пути ближе — один предмет (0 — только пересекающиеся; < 0 — DefaultGap)
	Cols    int          // сетка: колонки (0 — без сетки)
	Rows    int          // сетка: ряды
	Split   []SplitLine  // резать склеенные группы
	Drop    []int        // снять группы (номера до split/drop)
	Knock   [][2]float64 // точки вырезов в координатах листа
	Side    int          // длинная сторона предмета, px (DefaultSide, ≤ MaxSide)
	Overlay bool         // лист с подсветкой белых путей и рамок (Result.Overlay)
	Render  RenderFunc   // растр SVG; нужен всегда, кроме разбора (Chrome.Render)
}

// DefaultOptions — умолчания CLI.
func DefaultOptions() Options { return Options{Gap: DefaultGap, Side: DefaultSide} }

// White — белый путь (цвета листа) в отчёте.
type White struct {
	N       int        `json:"n"`       // wN: номер белого пути на листе
	Path    int        `json:"path"`    // номер пути в файле, с 1
	Box     [4]float64 `json:"box"`     // рамка в координатах листа
	Knocked bool       `json:"knocked"` // вырезан по knock
}

// Item — один предмет.
type Item struct {
	Index    int          `json:"index"`           // с 1: порядок слева направо, сверху вниз
	Base     int          `json:"base"`            // номер группы до split/drop
	W        int          `json:"w"`               // размер файла, px
	H        int          `json:"h"`               //
	CmW      float64      `json:"cmW"`             // при 300 dpi
	CmH      float64      `json:"cmH"`             //
	Frame    [4]float64   `json:"frame"`           // рамка предмета на листе (без поля)
	Paths    int          `json:"paths"`           // путей в предмете
	Whites   []White      `json:"whites"`          // белые пути предмета
	Bytes    int          `json:"bytes,omitempty"` // размер PNG (после записи)
	Warnings []string     `json:"warnings,omitempty"`
	Img      *image.NRGBA `json:"-"`
}

// Result — итог нарезки.
type Result struct {
	Sheet      [2]float64   `json:"sheet"`
	SheetFill  string       `json:"sheetFill,omitempty"` // цвет листа (пустоты «белые» именно этим цветом)
	Paths      int          `json:"paths"`               // путей без листа
	WhiteTotal int          `json:"whiteTotal"`          // белых путей на листе без самого листа
	Groups     int          `json:"groups"`              // групп до split/drop
	Knocks     []Knock      `json:"knocks,omitempty"`
	EmptyCells [][2]int     `json:"emptyCells,omitempty"`
	Items      []Item       `json:"items"`
	Notes      []string     `json:"notes,omitempty"`
	Overlay    *image.NRGBA `json:"-"`
}

// Cut режет лист. Ошибка — негодный вход (грамматика, пределы), промах knock, сбой растра; ошибки
// предметов собираются все, а не первая.
func Cut(data []byte, o Options) (*Result, error) {
	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}
	return CutDoc(doc, o)
}

// CutDoc — Cut над разобранным листом.
func CutDoc(doc *Doc, o Options) (*Result, error) {
	o, err := o.normalized()
	if err != nil {
		return nil, err
	}
	sheet := doc.Sheet()
	isWhite, whites, whiteN := whiteIndex(doc, sheet)
	res := &Result{Sheet: [2]float64{doc.W, doc.H}, WhiteTotal: len(whites)}
	var idx []int
	for i := range doc.Paths {
		if i != sheet && (sheet < 0 || i > sheet) {
			idx = append(idx, i)
		}
	}
	res.Paths = len(idx)
	if sheet < 0 {
		res.Notes = append(res.Notes, "лист (прямоугольник во весь лист с каналами ≥ 245) не найден: ничего не снято")
	} else {
		res.SheetFill = doc.Paths[sheet].Fill()
		if sheet > 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("под листом лежало путей: %d — сняты вместе с ним", sheet))
		}
	}
	groups, err := makeGroups(doc, idx, o, res)
	if err != nil {
		return nil, err
	}
	res.Groups = len(groups)
	if groups, err = applySplitDrop(doc, groups, o.Split, o.Drop); err != nil {
		return nil, err
	}
	groups = orderRows(groups)
	res.Knocks, err = hitKnocks(doc, sheet, isWhite, o.Knock, o.Render)
	if err != nil {
		return nil, err
	}
	knocked, owner := map[int]bool{}, ownerOf(groups)
	for i := range res.Knocks {
		k := &res.Knocks[i]
		pi := k.Path - 1
		if _, ok := owner[pi]; !ok {
			return nil, fmt.Errorf("svgcut: knock (%g, %g): путь %d в снятой группе", k.Point[0], k.Point[1], k.Path)
		}
		k.White = whiteN[pi]
		knocked[pi] = true
	}
	if err := renderItems(doc, groups, knocked, isWhite, whiteN, o, res); err != nil {
		return res, err
	}
	if o.Overlay {
		if res.Overlay, err = renderOverlay(doc, sheet, isWhite, whiteN, knocked, groups, o); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (o Options) normalized() (Options, error) {
	if o.Gap < 0 || math.IsNaN(o.Gap) {
		o.Gap = DefaultGap
	}
	if o.Side == 0 {
		o.Side = DefaultSide
	}
	if o.Side < 64 || o.Side > MaxSide {
		return o, fmt.Errorf("svgcut: side %d вне пределов 64…%d", o.Side, MaxSide)
	}
	if (o.Cols > 0) != (o.Rows > 0) || o.Cols < 0 || o.Rows < 0 {
		return o, errors.New("svgcut: сетка — обе стороны больше нуля")
	}
	if o.Render == nil {
		return o, errors.New("svgcut: не задан растр (браузер PACKPREP_CHROME)")
	}
	return o, nil
}

func makeGroups(doc *Doc, idx []int, o Options, res *Result) ([]group, error) {
	if len(idx) == 0 {
		return nil, errors.New("svgcut: после снятия листа путей не осталось")
	}
	if o.Cols > 0 {
		groups, empty := byGrid(doc, idx, o.Cols, o.Rows)
		res.EmptyCells = empty
		for _, c := range empty {
			res.Notes = append(res.Notes, fmt.Sprintf("пустая ячейка сетки: колонка %d, ряд %d", c[0], c[1]))
		}
		return groups, nil
	}
	return byGap(doc, idx, o.Gap), nil
}

// ownerOf — какие пути остались в группах (номер пути → индекс группы).
func ownerOf(groups []group) map[int]int {
	m := map[int]int{}
	for gi, g := range groups {
		for _, pi := range g.Paths {
			m[pi] = gi
		}
	}
	return m
}

// renderItems растрирует предметы параллельно (по workers запусков браузера); ошибки всех
// предметов собираются, порядок результата — порядок групп.
func renderItems(doc *Doc, groups []group, knocked map[int]bool, isWhite func(int) bool, whiteN map[int]int, o Options, res *Result) error {
	items := make([]Item, len(groups))
	errs := make([]error, len(groups))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for gi := range groups {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			items[gi], errs[gi] = renderItem(doc, groups[gi], knocked, isWhite, whiteN, o)
			items[gi].Index = gi + 1
		}()
	}
	wg.Wait()
	res.Items = items
	median := medianArea(groups)
	for gi := range items {
		if errs[gi] != nil {
			errs[gi] = fmt.Errorf("предмет %d: %w", gi+1, errs[gi])
			continue
		}
		items[gi].Warnings = append(items[gi].Warnings, judge(groups[gi], median)...)
	}
	return errors.Join(errs...)
}

// renderItem растрирует один предмет и обрезает по альфе.
func renderItem(doc *Doc, g group, knocked map[int]bool, isWhite func(int) bool, whiteN map[int]int, o Options) (Item, error) {
	it := Item{Base: g.Base, Frame: g.Box.Arr(), Paths: len(g.Paths), Whites: []White{}}
	for _, pi := range g.Paths {
		if isWhite(pi) {
			it.Whites = append(it.Whites, White{N: whiteN[pi], Path: pi + 1, Box: doc.Paths[pi].Box.Arr(), Knocked: knocked[pi]})
		}
	}
	frame := Box{g.Box.X0 - frameMargin, g.Box.Y0 - frameMargin, g.Box.X1 + frameMargin, g.Box.Y1 + frameMargin}
	k := float64(o.Side) / math.Max(frame.W(), frame.H())
	pw, ph := max(1, int(math.Ceil(frame.W()*k))), max(1, int(math.Ceil(frame.H()*k)))
	img, err := o.Render(itemSVG(doc, g, knocked, frame, pw, ph), pw, ph)
	if err != nil {
		return it, err
	}
	crop, ok := cropAlpha(img)
	if !ok {
		return it, errors.New("после растра предмет пуст (всё вырезано или пути вне листа)")
	}
	it.Img = crop
	it.W, it.H = crop.Bounds().Dx(), crop.Bounds().Dy()
	it.CmW, it.CmH = float64(it.W)/300*2.54, float64(it.H)/300*2.54
	if m := max(it.W, it.H); m < MinSidePx {
		it.Warnings = append(it.Warnings, fmt.Sprintf("мало пикселей: длинная сторона %d px < %d", m, MinSidePx))
	}
	return it, nil
}

func renderOverlay(doc *Doc, sheet int, isWhite func(int) bool, whiteN map[int]int, knocked map[int]bool, groups []group, o Options) (*image.NRGBA, error) {
	_, _, pw, ph := hitScale(doc)
	k := math.Min(1, 2048/math.Max(float64(pw), float64(ph)))
	pw, ph = max(1, int(float64(pw)*k)), max(1, int(float64(ph)*k))
	return o.Render(overlaySVG(doc, sheet, isWhite, whiteN, knocked, groups, pw, ph), pw, ph)
}

// medianArea — медиана площадей рамок групп.
func medianArea(groups []group) float64 {
	a := make([]float64, len(groups))
	for i, g := range groups {
		a[i] = g.Box.W() * g.Box.H()
	}
	slices.Sort(a)
	if len(a) == 0 {
		return 0
	}
	if len(a)%2 == 1 {
		return a[len(a)/2]
	}
	return (a[len(a)/2-1] + a[len(a)/2]) / 2
}

// judge — подозрение на «слиплось?»: рамка сильно больше медианной.
func judge(g group, median float64) []string {
	if area := g.Box.W() * g.Box.H(); median > 0 && area > clumpRatio*median {
		return []string{fmt.Sprintf("слиплось? рамка в %.1f раза больше медианной", area/median)}
	}
	return nil
}

// cropAlpha обрезает по непрозрачным пикселям (альфа > 0); ok=false — картинка пуста.
func cropAlpha(img *image.NRGBA) (*image.NRGBA, bool) {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[(y-b.Min.Y)*img.Stride : (y-b.Min.Y)*img.Stride+b.Dx()*4]
		for x := 0; x < b.Dx(); x++ {
			if row[x*4+3] != 0 {
				minX, maxX = min(minX, b.Min.X+x), max(maxX, b.Min.X+x)
				minY, maxY = min(minY, y), max(maxY, y)
			}
		}
	}
	if maxX < minX {
		return nil, false
	}
	r := image.Rect(minX, minY, maxX+1, maxY+1)
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		copy(out.Pix[y*out.Stride:y*out.Stride+r.Dx()*4], img.Pix[(r.Min.Y+y-b.Min.Y)*img.Stride+(r.Min.X-b.Min.X)*4:])
	}
	return out, true
}
