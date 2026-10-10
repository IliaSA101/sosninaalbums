package render

import (
	"image/color"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var flipCols = struct{ red, grn, blu, yel color.RGBA }{
	color.RGBA{220, 20, 20, 255}, color.RGBA{20, 200, 20, 255}, color.RGBA{20, 20, 220, 255}, color.RGBA{220, 220, 20, 255},
}

// quadColors — цвета четырёх четвертей ячейки cell (px при dpi 100) в порядке ЛВ, ПВ, ЛН, ПН.
func quadColors(t *testing.T, ph Photo, cell layout.Rect) [4]color.RGBA {
	t.Helper()
	img, err := Render(Spread{Photos: []Photo{ph}}, Options{PageW: 100, PageH: 100, DPI: 100})
	if err != nil {
		t.Fatal(err)
	}
	x0, y0, x1, y1 := px(cell.X, 100), px(cell.Y, 100), px(cell.X+cell.W, 100), px(cell.Y+cell.H, 100)
	mx, my, d := (x0+x1)/2, (y0+y1)/2, 4
	return [4]color.RGBA{img.RGBAAt(mx-d-3, my-d-3), img.RGBAAt(mx+d+3, my-d-3), img.RGBAAt(mx-d-3, my+d+3), img.RGBAAt(mx+d+3, my+d+3)}
}

func checkQuads(t *testing.T, what string, got, want [4]color.RGBA) {
	t.Helper()
	for i := range got {
		if !near(got[i], want[i], 14) {
			t.Fatalf("%s: четверть %d — %v, ждали %v", what, i, got[i], want[i])
		}
	}
}

// ↔: картинка в ячейке зеркальна по горизонтали, вертикаль не тронута.
func TestRenderFlipH(t *testing.T) {
	c := flipCols
	ph := quadrants(t, testkit.TempDir(t), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	cell := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	p := coverPhoto(cell, 1, ph)
	checkQuads(t, "без зеркала", quadColors(t, p, cell), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	p.FlipH = true
	checkQuads(t, "↔", quadColors(t, p, cell), [4]color.RGBA{c.grn, c.red, c.yel, c.blu})

	// Увеличенный кадр: проект при зеркале меняет X → −X, видимая часть остаётся той же
	// (левый верхний угол снимка), только отражённой: ячейка целиком красная.
	z := framedPhoto(cell, 1, 2, 1, -1, ph)
	z.FlipH = true
	checkQuads(t, "↔ с кадром", quadColors(t, z, cell), [4]color.RGBA{c.red, c.red, c.red, c.red})
}

// ↕: зеркало по вертикали.
func TestRenderFlipV(t *testing.T) {
	c := flipCols
	ph := quadrants(t, testkit.TempDir(t), [4]color.RGBA{c.red, c.grn, c.blu, c.yel})
	cell := layout.Rect{X: 20, Y: 20, W: 40, H: 40}
	p := coverPhoto(cell, 1, ph)
	p.FlipV = true
	checkQuads(t, "↕", quadColors(t, p, cell), [4]color.RGBA{c.blu, c.yel, c.red, c.grn})
	p.FlipH = true
	checkQuads(t, "↔↕", quadColors(t, p, cell), [4]color.RGBA{c.yel, c.blu, c.grn, c.red})

	z := framedPhoto(cell, 1, 2, -1, 1, ph)
	z.FlipV = true
	checkQuads(t, "↕ с кадром", quadColors(t, z, cell), [4]color.RGBA{c.red, c.red, c.red, c.red})
}
