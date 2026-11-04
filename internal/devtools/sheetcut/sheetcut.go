// Package sheetcut режет лист стикеров (одна картинка, много предметов) на отдельные PNG RGBA.
// Инструмент разработчика для подготовки наборов библиотеки, не функция программы: чистая логика
// на stdlib (image/png), без зависимостей от project и album. Запуск — scripts/sheetcut.go.
package sheetcut

import (
	"errors"
	"fmt"
	"image"
	"slices"
	"strconv"
	"strings"
)

// Режимы снятия фона.
const (
	BGAuto  = "auto"  // есть содержательная альфа — none; белый лист — rembg, если команда задана, иначе own
	BGNone  = "none"  // альфа как есть
	BGOwn   = "own"   // белый фон снять своим алгоритмом (RemoveBackground)
	BGSeed  = "seed"  // рамки, уголки, разделители: порог по листу, окно от центра, holes (RemoveBackgroundSeed)
	BGRembg = "rembg" // внешняя команда rembg (runRembg)
)

// Предупреждения отчёта.
const (
	MinSidePx    = 600  // длинная сторона стикера меньше — мало для печати
	MaxSidePx    = 4096 // предел импорта своих стикеров (как decor.MaxStickerSide)
	MaxFileBytes = 30 << 20
	clumpRatio   = 1.8 // рамка больше медианной во столько раз — «слиплось?»
	softPx       = 3   // сколько px мягкого края вокруг областей стикера оставлять
	maxSheetPx   = 120_000_000
)

// Options — параметры нарезки. Нулевые значения заменяются умолчаниями (BG, Model, Alpha, LargeFrac,
// MinArea, MaxSide; White, Edge, HoleMin — в RemoveBackground), кроме Dense (0 — выключено), Pad и
// Glue (0 — клеить только соприкасающиеся рамки; меньше нуля — 80).
type Options struct {
	BG        string  // BGAuto (по умолчанию), BGNone, BGOwn, BGRembg
	Rembg     string  // команда rembg (путь к rembg.exe/rembg.cmd); пусто — не задан
	Model     string  // модель rembg (DefaultModel)
	Defringe  bool    // лист, снятый rembg с белого: цвет полупрозрачных пикселей восстановить от белого
	Dense     uint8   // альфа ≥ порога → 255; 0 — выключено
	Alpha     uint8   // порог альфы связной области (32)
	White     uint8   // «почти белый» для BGOwn (250)
	HoleMin   int     // меньше — замкнутая белая область остаётся предметом (авто от размера листа)
	Edge      int     // полоса мягкого края BGOwn, px (3)
	Cols      int     // сетка: колонки (0 — без сетки)
	Rows      int     // сетка: ряды
	Glue      int     // без сетки: макс. расстояние мелкого куска до крупной области, px (0 — только вплотную; < 0 — 80)
	LargeFrac float64 // без сетки: «крупная» — площадь ≥ доли от наибольшей (0,1)
	MinArea   int     // мусор меньше этой площади, px (авто: 1/10000 листа)
	Pad       int     // прозрачное поле вокруг предмета, px
	MaxSide   int     // длинная сторона выхода, больше — уменьшить (4096)

	Mode      string // ModeSheet (по умолчанию), ModeFrame, ModeQuarters, ModeRows
	Thr       uint8  // BGSeed: «почти белый»; 0 — по листу (AdaptiveThr), в рецепте — вручную
	Holes     string // BGSeed: HolesClean (по умолчанию), HolesAll, HolesNone
	SplitRows int    // ModeRows: сколько рядов (N−1 самых широких промежутков); 0 — auto по RowGap
	RowGap    int    // ModeRows, auto: промежутки уже этого числа px склеиваются (DefaultRowGap)
	Stem      string // имя файлов режимов: <Stem>.png, <Stem>-tl.png, <Stem>-1.png; пусто — NN.png
}

// DefaultOptions — умолчания CLI.
func DefaultOptions() Options {
	return Options{BG: BGAuto, Model: DefaultModel, Defringe: true, Dense: 224, Alpha: 32, White: 250, Edge: 3, Glue: 80, LargeFrac: 0.1, MaxSide: MaxSidePx}
}

func (o Options) normalized(w, h int) Options {
	if o.BG == "" {
		o.BG = BGAuto
	}
	if o.Model == "" {
		o.Model = DefaultModel
	}
	if o.Alpha == 0 {
		o.Alpha = 32
	}
	if o.Glue < 0 {
		o.Glue = 80
	}
	if o.LargeFrac <= 0 {
		o.LargeFrac = 0.1
	}
	if o.MinArea <= 0 {
		o.MinArea = max(16, w*h/10000)
	}
	if o.MaxSide <= 0 {
		o.MaxSide = MaxSidePx
	}
	o.Pad = max(0, o.Pad)
	if o.RowGap <= 0 {
		o.RowGap = DefaultRowGap
	}
	return o
}

// ParseGrid разбирает «3x4» (колонки×ряды); пусто — без сетки (0, 0).
func ParseGrid(s string) (cols, rows int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	a, b, ok := strings.Cut(strings.ToLower(s), "x")
	cols, e1 := strconv.Atoi(a)
	rows, e2 := strconv.Atoi(b)
	if !ok || e1 != nil || e2 != nil || cols < 1 || rows < 1 {
		return 0, 0, fmt.Errorf("сетка %q: нужно КОЛОНКИxРЯДЫ, например 3x4", s)
	}
	return cols, rows, nil
}

// Sticker — один вырезанный стикер.
type Sticker struct {
	Index    int          `json:"index"`           // с 1: порядок слева направо, сверху вниз
	W        int          `json:"w"`               // размер файла, px
	H        int          `json:"h"`               //
	CmW      float64      `json:"cmW"`             // при 300 dpi
	CmH      float64      `json:"cmH"`             //
	Rect     [4]int       `json:"rect"`            // рамка предмета в листе: x0, y0, x1, y1 (без поля)
	Cell     *[2]int      `json:"cell,omitempty"`  // колонка, ряд (при сетке)
	Pieces   int          `json:"pieces"`          // сколько связных кусков склеено
	Area     int          `json:"area"`            // площадь видимых пикселей предмета в листе
	Scaled   bool         `json:"scaled"`          // уменьшен до MaxSide
	Bytes    int          `json:"bytes,omitempty"` // размер PNG (после записи)
	File     string       `json:"file,omitempty"`  // имя файла режимов frame, quarters, rows; пусто — NN.png
	Frame    *FrameInfo   `json:"frame,omitempty"` // только ModeFrame: окно, срез, «тянется»
	Warnings []string     `json:"warnings,omitempty"`
	Img      *image.NRGBA `json:"-"`
}

// Result — итог нарезки.
type Result struct {
	Sheet    [2]int    `json:"sheet"`
	BG       string    `json:"bg"` // фактический режим: none, own или rembg
	BGInfo   *BGStats  `json:"bgInfo,omitempty"`
	Regions  int       `json:"regions"`           // связных областей в листе
	Noise    int       `json:"noise"`             // выброшено областей меньше MinArea
	Unglued  int       `json:"unglued,omitempty"` // без сетки: мелких кусков без пары
	Empty    [][2]int  `json:"emptyCells,omitempty"`
	Stickers []Sticker `json:"stickers"`
	Notes    []string  `json:"notes,omitempty"`
}

// Cut режет лист. Ошибка — только для негодного входа (пустая картинка, слишком большая, сетка).
func Cut(src image.Image, o Options) (*Result, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 || w*h > maxSheetPx {
		return nil, fmt.Errorf("sheetcut: лист %d×%d вне пределов", w, h)
	}
	if (o.Cols > 0) != (o.Rows > 0) || o.Cols < 0 || o.Rows < 0 {
		return nil, errors.New("sheetcut: сетка — обе стороны больше нуля")
	}
	if err := o.checkMode(); err != nil {
		return nil, err
	}
	o = o.normalized(w, h)
	img := toNRGBA(src)
	res := &Result{Sheet: [2]int{w, h}}
	switch o.BG {
	case BGNone, BGOwn, BGSeed, BGRembg:
		res.BG = o.BG
	case BGAuto:
		switch {
		case hasContentAlpha(img):
			res.BG = BGNone
		case o.Mode != ModeSheet:
			res.BG = BGSeed // rembg для рамок и разделителей — только явным bg: тонкие линии он съедает
		case o.Rembg != "":
			res.BG = BGRembg
		default:
			res.BG = BGOwn
			res.Notes = append(res.Notes, "rembg не задан — свой алгоритм, хуже на замкнутых белых областях")
		}
	default:
		return nil, fmt.Errorf("sheetcut: неизвестный режим фона %q", o.BG)
	}
	if o.Mode != ModeSheet {
		if err := cutMode(img, o, res); err != nil {
			return nil, err
		}
		return res, nil
	}
	var err error
	if img, err = applyBG(img, o, res, false); err != nil {
		return nil, err
	}
	cutRegions(img, o, res)
	return res, nil
}

// cutRegions — разметка, мусор, склейка, сборка стикеров.
func cutRegions(img *image.NRGBA, o Options, res *Result) {
	labels, all := labelRegions(img, o.Alpha)
	res.Regions = len(all)
	var regs []region
	var ids []int // номер метки для regs[j]
	for id, r := range all {
		if r.Area < o.MinArea {
			res.Noise++
			continue
		}
		regs = append(regs, r)
		ids = append(ids, id+1)
	}
	sheet := image.Pt(res.Sheet[0], res.Sheet[1])
	var groups []group
	if o.Cols > 0 {
		var empty []image.Point
		groups, empty = byGrid(regs, sheet, o.Cols, o.Rows)
		for _, c := range empty {
			res.Empty = append(res.Empty, [2]int{c.X, c.Y})
			res.Notes = append(res.Notes, fmt.Sprintf("пустая ячейка сетки: колонка %d, ряд %d", c.X+1, c.Y+1))
		}
	} else {
		groups, res.Unglued = byGlue(regs, o.Glue, o.LargeFrac)
		if res.Unglued > 0 {
			res.Notes = append(res.Notes, fmt.Sprintf("мелких кусков без пары отброшено: %d", res.Unglued))
		}
	}
	stickerOf := make([]int, len(all)+1)
	for i := range stickerOf {
		stickerOf[i] = -1
	}
	for gi, g := range groups {
		for _, j := range g.Regs {
			stickerOf[ids[j]] = gi
		}
	}
	median := medianRectArea(groups)
	emptySet := map[image.Point]bool{}
	for _, c := range res.Empty {
		emptySet[image.Pt(c[0], c[1])] = true
	}
	for gi, g := range groups {
		s := render(img, labels, stickerOf, gi, g, o)
		s.Index = gi + 1
		s.Warnings = append(s.Warnings, judge(g, sheet, o, median, emptySet)...)
		s.Warnings = append(s.Warnings, sizeWarnings(&s)...)
		res.Stickers = append(res.Stickers, s)
	}
}

// medianRectArea — медиана площадей рамок групп.
func medianRectArea(groups []group) float64 {
	if len(groups) == 0 {
		return 0
	}
	a := make([]int, len(groups))
	for i, g := range groups {
		a[i] = g.Rect.Dx() * g.Rect.Dy()
	}
	slices.Sort(a)
	if len(a)%2 == 1 {
		return float64(a[len(a)/2])
	}
	return float64(a[len(a)/2-1]+a[len(a)/2]) / 2
}

// judge — подозрения на «слиплось?»: рамка сильно больше медианной или кусок задел пустую ячейку сетки.
func judge(g group, sheet image.Point, o Options, median float64, empty map[image.Point]bool) []string {
	var w []string
	if median > 0 && float64(g.Rect.Dx()*g.Rect.Dy()) > clumpRatio*median {
		w = append(w, fmt.Sprintf("слиплось? рамка в %.1f раза больше медианной", float64(g.Rect.Dx()*g.Rect.Dy())/median))
	}
	if o.Cols > 0 && spillsIntoEmpty(g.Rect, sheet, o.Cols, o.Rows, empty) {
		w = append(w, "слиплось? рамка заходит в пустую ячейку сетки")
	}
	return w
}

// sizeWarnings — предупреждения о размере выхода.
func sizeWarnings(s *Sticker) []string {
	var w []string
	if m := max(s.W, s.H); m < MinSidePx {
		w = append(w, fmt.Sprintf("мало пикселей: длинная сторона %d px < %d", m, MinSidePx))
	}
	if s.Scaled {
		w = append(w, fmt.Sprintf("уменьшен до %d px по длинной стороне (предел импорта стикеров)", MaxSidePx))
	}
	return w
}
