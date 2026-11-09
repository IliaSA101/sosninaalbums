package contactsheet

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
)

// MaxJPEG — предел размера листа (как у картинок службы агентов: ответ MCP ≤ 1 МБ строкой).
const MaxJPEG = 600 << 10

// ErrBudget — лист не уложился в предел размера даже при наименьшем качестве.
var ErrBudget = errors.New("лист не укладывается в предел размера")

// Encode кодирует лист в JPEG не больше budget байт, снижая качество 80 → 70 → 60 → 50. Go-кодировщик
// пишет только изображение: ни EXIF, ни XMP, ни ICC, ни комментария в выходе нет.
func Encode(img image.Image, budget int) ([]byte, error) {
	for _, q := range []int{80, 70, 60, 50} {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
			return nil, err
		}
		if buf.Len() <= budget {
			return buf.Bytes(), nil
		}
	}
	return nil, ErrBudget
}

// Render — Compose и Encode в пределе MaxJPEG: не уложился — сторона листа уменьшается на 15 %, пока
// не дойдёт до нижнего предела (тогда ErrBudget). w и h — размер получившейся картинки.
func Render(cells []Cell, grid, side int) (jpg []byte, w, h int, err error) {
	return render(cells, grid, side, MaxJPEG)
}

// render — Render с заданным пределом (тесты проверяют уменьшение стороны).
func render(cells []Cell, grid, side, budget int) (jpg []byte, w, h int, err error) {
	grid, side = Clamp(grid, side)
	for {
		sheet := Compose(cells, grid, side)
		jpg, err = Encode(sheet, budget)
		if err == nil {
			b := sheet.Bounds()
			return jpg, b.Dx(), b.Dy(), nil
		}
		if !errors.Is(err, ErrBudget) || side <= grid*minCell {
			return nil, 0, 0, err
		}
		side = max(side*85/100, grid*minCell)
	}
}
