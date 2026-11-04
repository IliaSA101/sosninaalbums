package packprep

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"path"
	"strings"
)

// Фоны (0.27-129-2): страница — короткая сторона 3602 px (наибольшая страница пресетов 40×30 см даёт
// 229 dpi, порог «Мало разрешения» 200), разворот — 7205×3602.
const (
	BgPage       = 3602
	BgSpreadW    = 7205
	UpscaleFlag  = 1.8 // апскейл больше — флаг в отчёте (разворот Mystic 2880 → 7205 = ×2,5)
	JPEGQuality  = 92
	MaxBgBytes   = 60 << 20 // pack.MaxBg: фон крупнее набор не примет
	spreadAspect = 1.8      // ширина/высота не меньше — разворот
)

// bgOut — готовый фон: байты файла и картинка для проверки и листа.
type bgOut struct {
	Data   []byte
	Img    *image.NRGBA
	Copied bool    // файл байт в байт
	From   int     // исходная сторона для «апскейл ×N от W px»: короткая у страницы, ширина у разворота
	Factor float64 // ×N; 1 — без увеличения
	Spread bool
	Notes  []string
}

// formatGroup — jpeg или png по расширению: байт в байт копируем только в тот же формат.
func formatGroup(ext string) string {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "jpeg"
	case ".png":
		return "png"
	}
	return ""
}

// flatten — NRGBA 8 бит без прозрачности: альфа сводится к белому (фон без альфы — обычный случай).
func flatten(src image.Image) (img *image.NRGBA, hadAlpha bool) {
	b := src.Bounds()
	img = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Over)
	if o, ok := src.(interface{ Opaque() bool }); ok {
		return img, !o.Opaque()
	}
	return img, false
}

// cropRect — копия прямоугольника r картинки с началом координат (0,0).
func cropRect(src *image.NRGBA, r image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := range r.Dy() {
		so := (r.Min.Y+y)*src.Stride + r.Min.X*4
		copy(out.Pix[y*out.Stride:y*out.Stride+r.Dx()*4], src.Pix[so:so+r.Dx()*4])
	}
	return out
}

// makeBg готовит фон: srcData — байты исходного файла, src — он же декодированный, outFile — имя
// выхода (расширение выбирает кодек). trim — срез по краям, % стороны, до увеличения.
func makeBg(srcData []byte, srcExt string, src image.Image, outFile string, trim float64) (*bgOut, error) {
	img, hadAlpha := flatten(src)
	res := &bgOut{Factor: 1}
	if hadAlpha {
		res.Notes = append(res.Notes, "у фона была прозрачность — подложен белый")
	}
	if trim > 0 {
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		dx, dy := int(math.Round(trim/100*float64(w))), int(math.Round(trim/100*float64(h)))
		if w-2*dx < 1 || h-2*dy < 1 {
			return nil, fmt.Errorf("trim %.1f %% съедает всю картинку %d×%d", trim, w, h)
		}
		img = cropRect(img, image.Rect(dx, dy, w-dx, h-dy))
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	res.Spread = float64(w) >= spreadAspect*float64(h)
	res.From = min(w, h)
	need := res.From < BgPage
	if res.Spread {
		res.From = w
		need = h < BgPage
	}
	sameFmt := formatGroup(srcExt) != "" && formatGroup(srcExt) == formatGroup(path.Ext(outFile))
	if !need {
		res.Img = img
		if trim == 0 && sameFmt {
			res.Data, res.Copied = srcData, true // «пиксели решаются до v1»: готовый файл не трогаем
			return res, checkBgSize(res)
		}
		return res, res.encode(outFile)
	}
	if res.Spread { // разворот 2:1: срез по центру, затем 7205×3602
		cw, ch := w, h
		if w > 2*h {
			cw = 2 * h
		} else {
			ch = w / 2
		}
		if cw != w || ch != h {
			x0, y0 := (w-cw)/2, (h-ch)/2
			img = cropRect(img, image.Rect(x0, y0, x0+cw, y0+ch))
			res.Notes = append(res.Notes, fmt.Sprintf("разворот обрезан до 2:1: %d×%d → %d×%d", w, h, cw, ch))
		}
		res.From = cw
		res.Factor = float64(BgSpreadW) / float64(cw)
		res.Img = Lanczos(img, BgSpreadW, BgPage)
	} else {
		dw, dh := BgPage, BgPage
		k := float64(BgPage) / float64(res.From)
		if w < h {
			dh = int(math.Round(float64(h) * k))
		} else {
			dw = int(math.Round(float64(w) * k))
		}
		res.Factor = k
		res.Img = Lanczos(img, dw, dh)
	}
	return res, res.encode(outFile)
}

// encode кодирует картинку в формат расширения outFile: .png — PNG, иначе JPEG q92.
func (b *bgOut) encode(outFile string) error {
	var buf bytes.Buffer
	var err error
	if formatGroup(path.Ext(outFile)) == "png" {
		err = png.Encode(&buf, b.Img)
	} else {
		err = jpeg.Encode(&buf, b.Img, &jpeg.Options{Quality: JPEGQuality})
	}
	if err != nil {
		return err
	}
	b.Data = buf.Bytes()
	return checkBgSize(b)
}

func checkBgSize(b *bgOut) error {
	if len(b.Data) > MaxBgBytes {
		return fmt.Errorf("фон %d МБ больше предела набора %d МБ", len(b.Data)>>20, MaxBgBytes>>20)
	}
	return nil
}

// upscaleNote — «апскейл ×N от W px».
func (b *bgOut) upscaleNote() string {
	return fmt.Sprintf("апскейл ×%.2f от %d px", b.Factor, b.From)
}
