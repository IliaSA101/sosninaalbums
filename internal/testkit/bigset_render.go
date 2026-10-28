package testkit

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"
)

// BigSetQuality — качество JPEG кадров набора.
const BigSetQuality = 92

const bigTile = 256 // сторона плитки шума

// BigSetTile — плитка шума bigTile×bigTile, ±10 уровней, от зерна; кадры берут её со сдвигом
// (NoiseDX, NoiseDY), так что шум у каждого свой, а считается он один раз.
func BigSetTile(seed int64) []int8 {
	t := make([]int8, bigTile*bigTile)
	for i := range t {
		t[i] = int8(bigMix(seed, i, 6)%21) - 10
	}
	return t
}

// bigAxis — для каждой выходной координаты: индекс левого пикселя исходника и вес правого (0..256).
type bigAxis struct {
	i0, i1 []int
	w      []int
}

// bigMapAxis строит отображение n выходных пикселей на окно [from, from+span) исходника длиной size;
// mirror разворачивает порядок.
func bigMapAxis(n, size int, from, span float64, mirror bool) bigAxis {
	a := bigAxis{i0: make([]int, n), i1: make([]int, n), w: make([]int, n)}
	for x := range n {
		u := x
		if mirror {
			u = n - 1 - x
		}
		s := min(float64(size-1), max(0, from+(float64(u)+0.5)*span/float64(n)-0.5))
		i := int(math.Floor(s))
		a.i0[x], a.i1[x], a.w[x] = i, min(i+1, size-1), int((s-float64(i))*256)
	}
	return a
}

// bigWindow — рамка в исходнике sw×sh: максимальная нужной пропорции, уменьшенная до Scale и
// сдвинутая по CropX, CropY.
func bigWindow(sw, sh int, p BigShot) (x0, y0, w, h float64) {
	w, h = float64(sw), float64(sh)
	if w*float64(p.H) > h*float64(p.W) {
		w = h * float64(p.W) / float64(p.H)
	} else {
		h = w * float64(p.H) / float64(p.W)
	}
	w, h = w*p.Scale, h*p.Scale
	return p.CropX * (float64(sw) - w), p.CropY * (float64(sh) - h), w, h
}

// BigSetRender рисует пиксели файла кадра p из исходника src: рамка, масштаб (билинейно), зеркало,
// яркость, шум; при p.Rotate картинка повёрнута на 90° против часовой — вместе с Orientation = 6
// программа видит её прямо. div > 1 уменьшает размеры в div раз (для тестов; 1 — полный кадр).
func BigSetRender(src *image.RGBA, p BigShot, tile []int8, div int) *image.RGBA {
	dw, dh := max(1, p.W/max(1, div)), max(1, p.H/max(1, div))
	sb := src.Bounds()
	x0, y0, ww, wh := bigWindow(sb.Dx(), sb.Dy(), p)
	ax := bigMapAxis(dw, sb.Dx(), x0, ww, p.Mirror)
	ay := bigMapAxis(dh, sb.Dy(), y0, wh, false)
	bright := int(math.Round((1 + p.Bright) * 256))
	out := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range dh {
		row := out.Pix[y*out.Stride : y*out.Stride+dw*4]
		noise := tile[((y+p.NoiseDY)%bigTile)*bigTile:][:bigTile]
		bigRow(row, src, ax, ay, y, bright, noise, p.NoiseDX)
	}
	if p.Rotate {
		return bigRotateCCW(out)
	}
	return out
}

// bigRow считает одну строку кадра.
func bigRow(row []byte, src *image.RGBA, ax, ay bigAxis, y, bright int, noise []int8, dx int) {
	r0 := src.Pix[ay.i0[y]*src.Stride:]
	r1 := src.Pix[ay.i1[y]*src.Stride:]
	wy := ay.w[y]
	for x := range len(row) / 4 {
		a, b, wx := ax.i0[x]*4, ax.i1[x]*4, ax.w[x]
		n := int(noise[(x+dx)%bigTile])
		for c := range 3 {
			top := int(r0[a+c])*(256-wx) + int(r0[b+c])*wx
			bot := int(r1[a+c])*(256-wx) + int(r1[b+c])*wx
			v := ((top*(256-wy)+bot*wy)>>16)*bright>>8 + n
			row[x*4+c] = byte(min(255, max(0, v)))
		}
		row[x*4+3] = 255
	}
}

// bigRotateCCW поворачивает картинку на 90° против часовой стрелки (размеры меняются местами):
// пиксель (x, y) кадра w×h окажется в (y, w-1-x).
func bigRotateCCW(img *image.RGBA) *image.RGBA {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	out := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := range h {
		for x := range w {
			s := img.Pix[y*img.Stride+x*4:][:4]
			copy(out.Pix[(w-1-x)*out.Stride+y*4:], s)
		}
	}
	return out
}

// BigSetEncode кодирует кадр в JPEG (качество BigSetQuality) и вставляет сегменты BigSetSegments
// сразу после SOI.
func BigSetEncode(img *image.RGBA, p BigShot) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: BigSetQuality}); err != nil {
		return nil, err
	}
	data := buf.Bytes()
	out := append([]byte{}, data[:2]...)
	for _, s := range BigSetSegments(p) {
		out = append(out, s...)
	}
	return append(out, data[2:]...), nil
}
