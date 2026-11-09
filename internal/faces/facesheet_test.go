package faces

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Контактный лист подгона (спека 130 G7): окна мест после подгона с рамкой найденного в окне
// лица и линией глаз. QA смотрит итог без GUI и считает σ по этому же кругу.

const (
	sheetCols  = 6
	sheetCellW = 200 // ширина окна на листе, px
	sheetPad   = 8
	sheetFrame = 3 // рамка флага вокруг окна, px
)

var (
	colBg     = color.NRGBA{40, 40, 40, 255}
	colBox    = color.NRGBA{0, 220, 0, 255}     // рамка лица, найденного в окне
	colEye    = color.NRGBA{255, 220, 0, 255}   // линия глаз, найденная в окне
	colTarget = color.NRGBA{0, 200, 255, 255}   // цель нормы: линия глаз и центр
	colFlag   = color.NRGBA{230, 40, 40, 255}   // не по норме
	colLost   = color.NRGBA{200, 0, 200, 255}   // лицо в окне не найдено
	colSkip   = color.NRGBA{120, 120, 120, 255} // фото без лица или с несколькими
)

// sheetBorder — цвет рамки окна: серый — круг не считался, фиолетовый — лицо потеряно, красный —
// флаг «не по норме», без рамки — по норме.
func sheetBorder(r fitRes) (color.NRGBA, bool) {
	switch {
	case r.Skip != "" || r.Pic == nil:
		return colSkip, true
	case r.Got == nil:
		return colLost, true
	case r.Flag != geom.FaceOK:
		return colFlag, true
	}
	return color.NRGBA{}, false
}

// hline и vline рисуют линию толщиной 1 px на NRGBA, обрезая по границе.
func hline(img *image.NRGBA, x0, x1, y int, c color.NRGBA) {
	for x := max(x0, img.Rect.Min.X); x <= min(x1, img.Rect.Max.X-1); x++ {
		if y >= img.Rect.Min.Y && y < img.Rect.Max.Y {
			img.SetNRGBA(x, y, c)
		}
	}
}

func vline(img *image.NRGBA, x, y0, y1 int, c color.NRGBA) {
	for y := max(y0, img.Rect.Min.Y); y <= min(y1, img.Rect.Max.Y-1); y++ {
		if x >= img.Rect.Min.X && x < img.Rect.Max.X {
			img.SetNRGBA(x, y, c)
		}
	}
}

// sheetCell — окно места в ширину sheetCellW с рамкой найденного лица и линиями.
func sheetCell(r fitRes) *image.NRGBA {
	if r.Pic == nil {
		w := sheetCellW
		h := int(float64(w) * r.Win.Rect.H / r.Win.Rect.W)
		cell := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(cell, cell.Rect, image.NewUniform(colSkip), image.Point{}, draw.Src)
		return cell
	}
	cell := imaging.Resize(r.Pic, sheetCellW, 0, imaging.Linear)
	w, h := cell.Rect.Dx(), cell.Rect.Dy()
	fx, fy := float64(w), float64(h)
	// Цель нормы: линия глаз через всё окно и центр по X.
	ty := int(r.Norm.Eye * fy)
	hline(cell, 0, w-1, ty, colTarget)
	vline(cell, w/2, ty-6, ty+6, colTarget)
	if g := r.Got; g != nil {
		x0, y0 := int(g.Box.X*fx), int(g.Box.Y*fy)
		x1, y1 := int((g.Box.X+g.Box.W)*fx), int((g.Box.Y+g.Box.H)*fy)
		hline(cell, x0, x1, y0, colBox)
		hline(cell, x0, x1, y1, colBox)
		vline(cell, x0, y0, y1, colBox)
		vline(cell, x1, y0, y1, colBox)
		ey := int(g.EyeLine().Y * fy)
		hline(cell, x0, x1, ey, colEye)
	}
	return cell
}

// sheetImage собирает лист: ячейки по sheetCols в ряд, рамка флага вокруг окна.
func sheetImage(res []fitRes) *image.NRGBA {
	cells := make([]*image.NRGBA, len(res))
	rowH := 0
	for i, r := range res {
		cells[i] = sheetCell(r)
		rowH = max(rowH, cells[i].Rect.Dy())
	}
	cols := min(sheetCols, max(1, len(res)))
	rows := (len(res) + cols - 1) / cols
	slot := sheetCellW + 2*sheetFrame
	slotH := rowH + 2*sheetFrame
	out := image.NewNRGBA(image.Rect(0, 0, sheetPad+cols*(slot+sheetPad), sheetPad+max(rows, 1)*(slotH+sheetPad)))
	draw.Draw(out, out.Rect, image.NewUniform(colBg), image.Point{}, draw.Src)
	for i, r := range res {
		x := sheetPad + (i%cols)*(slot+sheetPad)
		y := sheetPad + (i/cols)*(slotH+sheetPad)
		if c, ok := sheetBorder(r); ok {
			draw.Draw(out, image.Rect(x, y, x+slot, y+slotH), image.NewUniform(c), image.Point{}, draw.Src)
		}
		at := image.Pt(x+sheetFrame, y+sheetFrame)
		draw.Draw(out, image.Rectangle{Min: at, Max: at.Add(cells[i].Rect.Size())}, cells[i], image.Point{}, draw.Src)
	}
	return out
}

// sheetSource — портреты, затем сток: окна места Марины без поворота, до 30 мест.
func sheetSource(t *testing.T) []fitRes {
	t.Helper()
	var all []string
	for _, sub := range []string{"portraits", "stock"} {
		if dir := photosDir(sub); dir != "" {
			all = append(all, photoList(t, dir)...)
		}
	}
	if len(all) == 0 {
		t.Skip("нет .testphotos/portraits и stock (docs/QA.md «Подготовка»)")
	}
	res := fitAll(all, testkit.TempDir(t), winMarina, func(int) fitVariant { return fitVariant{} }, geom.DefaultFaceNorm)
	// Серые клетки без лица листу ни к чему: сначала считавшиеся места, потом по одному с хвоста.
	var out []fitRes
	for _, r := range res {
		if r.Skip == "" && r.Err == nil {
			out = append(out, r)
		}
	}
	return out[:min(len(out), 30)]
}

// TestFaceSheetPNG: контактный лист PNG. SA_FACES_SHEET не задана — пропуск, пустая — ошибка,
// путь — файл листа туда (папка должна существовать).
func TestFaceSheetPNG(t *testing.T) {
	dst, set := os.LookupEnv("SA_FACES_SHEET")
	if !set {
		t.Skip("SA_FACES_SHEET не задана: лист для QA пишется по запросу")
	}
	if dst == "" {
		t.Fatal("SA_FACES_SHEET задана пустой: нужен путь к файлу листа")
	}
	if err := paths.CheckLocalPath(dst); err != nil {
		t.Fatalf("SA_FACES_SHEET: %v", err)
	}
	res := sheetSource(t)
	if len(res) == 0 {
		t.Fatal("нет мест для листа: на фото не нашлось лиц")
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, sheetImage(res)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Clean(dst), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	s := collect(res)
	t.Logf("лист %s: %d мест, по норме %d, с флагом %d, потеряно %d", dst, len(res), s.N, s.Flagged, s.Lost)
}

// TestFaceSheetLayout: размеры листа и рамки флага на выдуманных окнах, без фото.
func TestFaceSheetLayout(t *testing.T) {
	win := fitWin{"тест", winMarina.Rect}
	pic := func() *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 100, 133))
		draw.Draw(img, img.Rect, image.NewUniform(color.RGBA{200, 200, 200, 255}), image.Point{}, draw.Src)
		return img
	}
	face := &Face{Box: Rect{X: 0.3, Y: 0.2, W: 0.4, H: 0.35}, Eyes: [2]Pt{{0.4, 0.3}, {0.6, 0.3}}}
	res := []fitRes{
		{Win: win, Norm: geom.DefaultFaceNorm, Pic: pic(), Got: face},                     // по норме: рамки нет
		{Win: win, Norm: geom.DefaultFaceNorm, Pic: pic(), Got: face, Flag: geom.FaceBig}, // красная
		{Win: win, Norm: geom.DefaultFaceNorm, Pic: pic()},                                // потеряно: фиолетовая
		{Win: win, Skip: "без лица"},                                                      // серая, без картинки
	}
	img := sheetImage(res)
	slot := sheetCellW + 2*sheetFrame
	if want := sheetPad + 4*(slot+sheetPad); img.Rect.Dx() != want {
		t.Errorf("ширина листа %d, ждали %d", img.Rect.Dx(), want)
	}
	px := func(i int) color.NRGBA { return img.NRGBAAt(sheetPad+i*(slot+sheetPad), sheetPad) }
	for i, want := range []color.NRGBA{colBg, colFlag, colLost, colSkip} {
		if got := px(i); got != want {
			t.Errorf("угол ячейки %d: %v, ждали %v", i, got, want)
		}
	}
	// Рамка найденного лица легла в окно: зелёный пиксель на её верхней границе.
	fx := sheetPad + sheetFrame + int(0.5*float64(sheetCellW))
	cellH := float64(sheetCellW) * 133 / 100
	fy := sheetPad + sheetFrame + int(0.2*cellH)
	if got := img.NRGBAAt(fx, fy); got != colBox {
		t.Errorf("верх рамки лица: %v, ждали %v", got, colBox)
	}
	// Пустой набор не падает.
	if e := sheetImage(nil); e.Rect.Dx() <= 0 || e.Rect.Dy() <= 0 {
		t.Errorf("лист пустого набора %v", e.Rect)
	}
}
