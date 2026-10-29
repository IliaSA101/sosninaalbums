package paint

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/math/f64"
)

// TestCanvasAff3 — путь стикера идёт через m: зеркало и поворот переносят фигуру; круги и
// куски штриха одного пути сливаются, а не вычитаются.
func TestCanvasAff3(t *testing.T) {
	ink := color.NRGBA{200, 0, 0, 255}
	draw1 := func(m f64.Aff3) *image.RGBA {
		dst := image.NewRGBA(image.Rect(0, 0, 100, 100))
		c := New(dst, dst.Rect, m, 1, 1, 0)
		c.Poly([]Pt{{0, 0}, {0.5, 0}, {0.5, 1}, {0, 1}}) // левая половина рамки
		c.Fill(ink, 1)
		return dst
	}
	plain := draw1(f64.Aff3{100, 0, 0, 0, 100, 0})
	flip := draw1(f64.Aff3{-100, 0, 100, 0, 100, 0})
	rot := draw1(f64.Aff3{0, -100, 100, 100, 0, 0}) // 90° по часовой: левая → верхняя
	if plain.RGBAAt(10, 50).A != 255 || plain.RGBAAt(90, 50).A != 0 {
		t.Error("без преобразования: не левая половина")
	}
	if flip.RGBAAt(10, 50).A != 0 || flip.RGBAAt(90, 50).A != 255 {
		t.Error("зеркало: не правая половина")
	}
	if rot.RGBAAt(50, 10).A != 255 || rot.RGBAAt(50, 90).A != 0 {
		t.Error("поворот: не верхняя половина")
	}
	dst := image.NewRGBA(image.Rect(0, 0, 100, 100))
	c := New(dst, dst.Rect, f64.Aff3{1, 0, 0, 0, 1, 0}, 100, 100, 0)
	c.Stroke([]Pt{{20, 50}, {80, 50}, {80, 20}}, false, 10)
	c.Fill(ink, 0.5)
	if a, b := dst.RGBAAt(80, 50).A, dst.RGBAAt(50, 50).A; a != b || b < 126 || b > 129 {
		t.Errorf("стык штриха %d, середина %d: куски не слились", a, b)
	}
}

// TestGenStrips — фигуры во всю область (широкая волна, полоса) полосами равны одной маске:
// нет шва на стыке полос. Без узоров gen — только холст.
func TestGenStrips(t *testing.T) {
	old := stripPx
	defer func() { stripPx = old }()
	ink := color.NRGBA{30, 90, 160, 255}
	draw := func() *image.RGBA {
		dst := image.NewRGBA(image.Rect(0, 0, 800, 480))
		c := New(dst, dst.Rect, f64.Aff3{4, 0, 0, 0, 4, 0}, 200, 120, 0)
		// Волна во всю ширину: ломаная по синусоиде, замкнутая низом области.
		wave := []Pt{{-1, 121}}
		for x := -1.0; x <= 201; x += 2 {
			wave = append(wave, Pt{x, 60 + 20*float64(int(x)%40-20)/20})
		}
		wave = append(wave, Pt{201, 121})
		c.Poly(wave)
		c.Fill(ink, 1)
		// Полоса поперёк: штрих по диагонали.
		c.Stroke([]Pt{{10, 10}, {190, 110}}, false, 6)
		c.Fill(ink, 0.7)
		return dst
	}
	stripPx = 1 << 30
	one := draw()
	stripPx = 1000 // десятки полос
	many := draw()
	if bytes.Equal(one.Pix, make([]byte, len(one.Pix))) {
		t.Fatal("холст пуст — тест ничего не проверяет")
	}
	if !bytes.Equal(one.Pix, many.Pix) {
		t.Error("полосы ≠ одна маска")
	}
}

// TestCanvasEdges — края: нет области или fade ≥ 1 — нет холста; NaN fade — как 0; пустой путь
// и вырожденные фигуры ничего не рисуют.
func TestCanvasEdges(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 10, 10))
	m := f64.Aff3{1, 0, 0, 0, 1, 0}
	if New(dst, image.Rect(20, 20, 30, 30), m, 1, 1, 0) != nil {
		t.Error("область вне холста — не nil")
	}
	if New(dst, dst.Rect, m, 1, 1, 1) != nil {
		t.Error("fade 1 — не nil")
	}
	nan := 0.0
	nan /= nan
	if c := New(dst, dst.Rect, m, 1, 1, nan); c == nil || c.Alpha != 1 {
		t.Errorf("NaN fade: %+v", c)
	}
	c := New(dst, dst.Rect, m, 10, 10, 0)
	c.Fill(color.NRGBA{A: 255}, 1) // пустой путь
	c.Poly([]Pt{{0, 0}, {5, 5}})   // меньше трёх точек
	c.Stroke([]Pt{{3, 3}}, false, 2)
	c.Fill(color.NRGBA{A: 255}, 1)
	if !bytes.Equal(dst.Pix, make([]byte, len(dst.Pix))) {
		t.Error("вырожденное что-то нарисовало")
	}
}
