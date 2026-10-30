package sheetcut

import (
	"image"
	"image/draw"
)

// toNRGBA — копия картинки в NRGBA 8 бит (цвет без предумножения), начало координат (0,0).
func toNRGBA(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}

// hasContentAlpha — есть ли у картинки «настоящая» альфа: прозрачных (< 128) пикселей не меньше 1 %.
// Одиночные прозрачные точки в белом листе (артефакты) за альфу не считаются.
func hasContentAlpha(img *image.NRGBA) bool {
	n := 0
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] < 128 {
			n++
		}
	}
	return n*100 >= len(img.Pix)/4
}

// densify — «уплотнение»: альфа ≥ порога → 255, мягкий край (ниже порога) остаётся как есть.
// Листы после апскейла просвечивают (альфа видимых 224–254): стикер на тёмном фоне был бы блёклым.
func densify(img *image.NRGBA, thr uint8) {
	if thr == 0 {
		return
	}
	for i := 3; i < len(img.Pix); i += 4 {
		if img.Pix[i] >= thr {
			img.Pix[i] = 255
		}
	}
}

// scaleDown уменьшает картинку до размера nw×nh усреднением по площади (с весом альфы:
// цвет прозрачных пикселей не подмешивается). Увеличивать не умеет: если nw и nh не меньше
// исходных, возвращает src.
func scaleDown(src *image.NRGBA, nw, nh int) *image.NRGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if nw >= sw && nh >= sh {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*sh/nh, max((y+1)*sh/nh, y*sh/nh+1)
		for x := 0; x < nw; x++ {
			x0, x1 := x*sw/nw, max((x+1)*sw/nw, x*sw/nw+1)
			boxAverage(src, dst.Pix[y*dst.Stride+x*4:], x0, y0, x1, y1)
		}
	}
	return dst
}

// boxAverage пишет в out (4 байта) среднее по прямоугольнику исходника, цвет — с весом альфы.
func boxAverage(src *image.NRGBA, out []uint8, x0, y0, x1, y1 int) {
	var r, g, b, a, cnt uint64
	for sy := y0; sy < y1; sy++ {
		row := src.Pix[sy*src.Stride:]
		for sx := x0; sx < x1; sx++ {
			p := row[sx*4 : sx*4+4]
			al := uint64(p[3])
			r += uint64(p[0]) * al
			g += uint64(p[1]) * al
			b += uint64(p[2]) * al
			a += al
			cnt++
		}
	}
	if a == 0 {
		return
	}
	out[0], out[1], out[2] = avg8(r, a), avg8(g, a), avg8(b, a)
	out[3] = avg8(a, cnt)
}

// fitSide — размер (w,h), вписанный в квадрат side по длинной стороне; не увеличивает.
func fitSide(w, h, side int) (int, int) {
	m := max(w, h)
	if m <= side {
		return w, h
	}
	return max(1, w*side/m), max(1, h*side/m)
}

// minC — минимум каналов RGB пикселя по смещению i.
func minC(p []uint8, i int) uint8 {
	return min(p[i], p[i+1], p[i+2])
}

// avg8 — округлённое частное sum/n, зажатое в байт (среднее взвешенных байтов не превышает 255).
func avg8(sum, n uint64) uint8 {
	return uint8(min(255, (sum+n/2)/n))
}
