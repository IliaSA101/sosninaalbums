package sheetcut

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

// Режимы разбора листа (Options.Mode). Рамки, уголки и разделители — не «лист стикеров»: фон
// снимается от затравок (BGSeed), результат режется по смыслу, а не по областям.
const (
	ModeSheet    = ""         // лист стикеров: сетка или склейка областей (по умолчанию)
	ModeFrame    = "frame"    // одна рамка: фон и окно, обрезка по альфа-bbox, мера «тянется»
	ModeQuarters = "quarters" // уголки 2×2: четверти tl, tr, bl, br, у каждой фон без окна
	ModeRows     = "rows"     // разделители: ряды по проекции альфы (N рядов или auto)
)

// DefaultRowGap — «auto»: промежутки между рядами уже этого числа px склеиваются в один ряд.
const DefaultRowGap = 12

// quarterSuffix — имена четвертей в порядке слева направо, сверху вниз.
var quarterSuffix = [4]string{"-tl", "-tr", "-bl", "-br"}

// checkMode проверяет сочетания параметров режимов; вызывается из Cut до разбора картинки.
func (o Options) checkMode() error {
	switch o.Mode {
	case ModeSheet, ModeFrame, ModeQuarters, ModeRows:
	default:
		return fmt.Errorf("sheetcut: неизвестный режим %q", o.Mode)
	}
	switch o.Holes {
	case "", HolesClean, HolesAll, HolesNone:
	default:
		return fmt.Errorf("sheetcut: holes %q: нужно clean, all или none", o.Holes)
	}
	if o.Mode != ModeSheet && o.Cols > 0 {
		return fmt.Errorf("sheetcut: режим %s и сетка несовместимы", o.Mode)
	}
	if o.SplitRows < 0 || o.RowGap < 0 {
		return errors.New("sheetcut: число рядов и rowgap — не меньше нуля")
	}
	if o.Stem != "" && (strings.ContainsAny(o.Stem, `/\`) || o.Stem == "." || o.Stem == ".." || strings.Contains(o.Stem, "..")) {
		return fmt.Errorf("sheetcut: имя %q: без путей, только имя файла без расширения", o.Stem)
	}
	return nil
}

// applyBG снимает фон выбранным режимом; window — искать окно (только seed одной рамки).
func applyBG(img *image.NRGBA, o Options, res *Result, window bool) (*image.NRGBA, error) {
	switch res.BG {
	case BGOwn:
		st := RemoveBackground(img, BGOptions{White: o.White, HoleMin: o.HoleMin, Edge: o.Edge})
		res.BGInfo = &st // мягкий край уже с восстановлением цвета (unmatte): defringe не нужен
	case BGSeed:
		st := RemoveBackgroundSeed(img, SeedOptions{Thr: o.Thr, Holes: o.Holes, Window: window, HoleMin: o.HoleMin})
		if res.BGInfo == nil {
			res.BGInfo = &st
		} else { // четверти: суммируем, порог — последней
			res.BGInfo.Holes += st.Holes
			res.BGInfo.HolesKept += st.HolesKept
			res.BGInfo.BackgroundPx += st.BackgroundPx
			res.BGInfo.Thr = st.Thr
		}
	case BGRembg:
		var err error
		if img, err = runRembg(img, o.Rembg, o.Model); err != nil {
			return nil, err
		}
		if o.Defringe {
			defringe(img)
		}
	}
	densify(img, o.Dense)
	return img, nil
}

// modeFile — имя файла режима: <Stem><suffix>.png; без Stem — пусто (будет NN.png по номеру).
func modeFile(o Options, suffix string) string {
	if o.Stem == "" {
		return ""
	}
	return o.Stem + suffix + ".png"
}

// cutMode ведёт режимы frame, quarters, rows; ошибка — res не нужен вызывающему.
func cutMode(img *image.NRGBA, o Options, res *Result) error {
	if o.Mode == ModeQuarters {
		return cutQuarters(img, o, res) // фон снимается по четвертям
	}
	img, err := applyBG(img, o, res, o.Mode == ModeFrame)
	if err != nil {
		return err
	}
	if o.Mode == ModeFrame {
		return cutFrame(img, o, res)
	}
	return cutRows(img, o, res)
}

// cutFrame — одна рамка: обрезка по непрозрачному (поля 0,5–17 % стороны стали бы прозрачной
// полосой у края места), затем разбор окна и мера «тянется» по готовой картинке.
func cutFrame(img *image.NRGBA, o Options, res *Result) error {
	s, trim, ok := trimSticker(img, o, res)
	if !ok {
		return errors.New("sheetcut: после снятия фона не осталось видимого")
	}
	s.Index = 1
	s.File = modeFile(o, "")
	s.Frame = analyzeFrame(s.Img, trim)
	s.Warnings = append(s.Warnings, s.Frame.warnings()...)
	s.Warnings = append(s.Warnings, sizeWarnings(&s)...)
	res.Stickers = append(res.Stickers, s)
	return nil
}

// cutQuarters — лист 2×2 с уголками: четверти режутся до снятия фона, у каждой фон от её краёв без
// окна, затем обрезка. Пустая четверть — замечание, не ошибка.
func cutQuarters(img *image.NRGBA, o Options, res *Result) error {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	for i, name := range quarterSuffix {
		col, row := i%2, i/2
		r := image.Rect(col*w/2, row*h/2, (col+1)*w/2, (row+1)*h/2)
		q, err := applyBG(subImage(img, r), o, res, false)
		if err != nil {
			return err
		}
		s, _, ok := trimSticker(q, o, res)
		if !ok {
			res.Notes = append(res.Notes, "пустая четверть: "+strings.TrimPrefix(name, "-"))
			continue
		}
		s.Index = len(res.Stickers) + 1
		s.File = modeFile(o, name)
		s.Cell = &[2]int{col, row}
		s.Rect = [4]int{r.Min.X + s.Rect[0], r.Min.Y + s.Rect[1], r.Min.X + s.Rect[2], r.Min.Y + s.Rect[3]}
		s.Warnings = append(s.Warnings, sizeWarnings(&s)...)
		res.Stickers = append(res.Stickers, s)
	}
	if len(res.Stickers) == 0 {
		return errors.New("sheetcut: во всех четвертях пусто")
	}
	return nil
}

// cutRows — разделители: ряды по проекции альфы на вертикаль. SplitRows = N — N−1 самых широких
// промежутков разделяют ряды (остальные склеиваются: цветок разделителя с дырой не рвётся); 0 —
// auto: делят промежутки не уже RowGap (12 px).
func cutRows(img *image.NRGBA, o Options, res *Result) error {
	eraseNoise(img, o, res)
	segs := rowSegments(img, o.Alpha)
	if len(segs) == 0 {
		return errors.New("sheetcut: после снятия фона не осталось видимого")
	}
	cuts := pickGaps(segs, o.SplitRows, o.RowGap)
	if o.SplitRows > 1 && len(cuts) < o.SplitRows-1 {
		res.Notes = append(res.Notes, fmt.Sprintf("рядов найдено %d, а просили %d: меньше промежутков, чем нужно", len(cuts)+1, o.SplitRows))
	} else if o.SplitRows >= 1 {
		// N ≠ число рядов по явным промежуткам: неверный N молча склеил бы ряды (меньше) или разрезал
		// элемент по узкой щели (больше) — число полос и «авто» видны в отчёте.
		if auto := len(pickGaps(segs, 0, o.RowGap)) + 1; auto != o.SplitRows {
			res.Notes = append(res.Notes, fmt.Sprintf("rows: задано рядов %d, а по промежуткам не уже %d px видно %d (полос альфы %d): проверьте n", o.SplitRows, o.RowGap, auto, len(segs)))
		}
	}
	first := 0
	for k := 0; k <= len(cuts); k++ {
		last := len(segs) - 1
		if k < len(cuts) {
			last = cuts[k]
		}
		r := image.Rect(0, segs[first].y0, img.Bounds().Dx(), segs[last].y1)
		first = last + 1
		s, _, ok := trimSticker(subImage(img, r), o, res)
		if !ok {
			continue
		}
		s.Index = len(res.Stickers) + 1
		s.File = modeFile(o, fmt.Sprintf("-%d", s.Index))
		s.Rect = [4]int{s.Rect[0], r.Min.Y + s.Rect[1], s.Rect[2], r.Min.Y + s.Rect[3]}
		s.Warnings = append(s.Warnings, sizeWarnings(&s)...)
		res.Stickers = append(res.Stickers, s)
	}
	return nil
}

// subImage — копия прямоугольника r листа с началом координат (0,0).
func subImage(img *image.NRGBA, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := range r.Dy() {
		so := (r.Min.Y+y)*img.Stride + r.Min.X*4
		copy(out.Pix[y*out.Stride:y*out.Stride+r.Dx()*4], img.Pix[so:so+r.Dx()*4])
	}
	return out
}
