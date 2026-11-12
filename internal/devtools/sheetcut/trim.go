package sheetcut

import (
	"image"
	"sort"
)

// eraseNoise стирает (альфа 0) области меньше MinArea: крошки после снятия фона не должны
// раздвигать рамку обрезки. Считает их в Result.Noise.
func eraseNoise(img *image.NRGBA, o Options, res *Result) {
	labels, regs := labelRegions(img, o.Alpha)
	small := make([]bool, len(regs)+1)
	found := false
	for id, r := range regs {
		if r.Area < o.MinArea {
			small[id+1] = true
			res.Noise++
			found = true
		}
	}
	if !found {
		return
	}
	for i, l := range labels {
		if l != 0 && small[l] {
			clear(img.Pix[i*4 : i*4+4])
		}
	}
}

// alphaBox — рамка пикселей с альфой не ниже thr; пустая — нет таких.
func alphaBox(img *image.NRGBA, thr uint8) image.Rectangle {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	x0, y0, x1, y1 := w, h, -1, -1
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := range w {
			if row[x*4+3] >= thr {
				x0, x1 = min(x0, x), max(x1, x)
				y0, y1 = min(y0, y), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1+1, y1+1)
}

// trimSticker — готовый стикер из картинки с уже снятым фоном: шум прочь, обрезка по альфа-bbox
// (изображение вплотную к краям файла), поле Pad, уменьшение до MaxSide. trim — сколько px срезано
// слева, сверху, справа, снизу; Rect — рамка предмета в img. ok = false — видимого нет.
func trimSticker(img *image.NRGBA, o Options, res *Result) (s Sticker, trim [4]int, ok bool) {
	eraseNoise(img, o, res)
	box := alphaBox(img, o.Alpha)
	if box.Empty() {
		return s, trim, false
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	trim = [4]int{box.Min.X, box.Min.Y, w - box.Max.X, h - box.Max.Y}
	out := image.NewNRGBA(image.Rect(0, 0, box.Dx()+2*o.Pad, box.Dy()+2*o.Pad))
	for y := range box.Dy() {
		so := (box.Min.Y+y)*img.Stride + box.Min.X*4
		do := (y+o.Pad)*out.Stride + o.Pad*4
		copy(out.Pix[do:do+box.Dx()*4], img.Pix[so:so+box.Dx()*4])
	}
	s.Pieces = 1
	s.Rect = [4]int{box.Min.X, box.Min.Y, box.Max.X, box.Max.Y}
	for i := 3; i < len(out.Pix); i += 4 {
		if out.Pix[i] >= o.Alpha {
			s.Area++
		}
	}
	if nw, nh := fitSide(out.Bounds().Dx(), out.Bounds().Dy(), o.MaxSide); nw != out.Bounds().Dx() || nh != out.Bounds().Dy() {
		out = scaleDown(out, nw, nh)
		s.Scaled = true
	}
	s.Img = out
	s.W, s.H = out.Bounds().Dx(), out.Bounds().Dy()
	s.CmW, s.CmH = cm(s.W), cm(s.H)
	return s, trim, true
}

// rowSeg — полоса строк [y0, y1), в которых есть видимое.
type rowSeg struct{ y0, y1 int }

// rowSegments — проекция альфы по строкам: непрерывные полосы строк с видимым.
func rowSegments(img *image.NRGBA, thr uint8) []rowSeg {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var segs []rowSeg
	start := -1
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		has := false
		for x := range w {
			if row[x*4+3] >= thr {
				has = true
				break
			}
		}
		switch {
		case has && start < 0:
			start = y
		case !has && start >= 0:
			segs = append(segs, rowSeg{start, y})
			start = -1
		}
	}
	if start >= 0 {
		segs = append(segs, rowSeg{start, h})
	}
	return segs
}

// pickGaps — номера полос i, после которых ряд кончается (разрез между segs[i] и segs[i+1]).
// n ≥ 1: n−1 самых широких промежутков (при равной ширине — верхние); n = 0 (auto): промежутки не
// уже gap (0 — DefaultRowGap). Промежутков меньше, чем нужно, — берутся все. Результат по возрастанию.
func pickGaps(segs []rowSeg, n, gap int) []int {
	if gap <= 0 {
		gap = DefaultRowGap
	}
	idx := make([]int, 0, len(segs))
	for i := 0; i+1 < len(segs); i++ {
		idx = append(idx, i)
	}
	width := func(i int) int { return segs[i+1].y0 - segs[i].y1 }
	var cuts []int
	if n == 0 {
		for _, i := range idx {
			if width(i) >= gap {
				cuts = append(cuts, i)
			}
		}
		return cuts
	}
	sort.SliceStable(idx, func(a, b int) bool { return width(idx[a]) > width(idx[b]) })
	cuts = append(cuts, idx[:min(len(idx), n-1)]...)
	sort.Ints(cuts)
	return cuts
}
