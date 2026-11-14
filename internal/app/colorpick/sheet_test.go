package colorpick

// Кадр листа для службы агентов: размер задаёт вызывающий, но большая сторона кадра не больше px.

import (
	"context"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Вытянутый лист: dpi по одной ширине давал кадр 1598×15984 (+100 МБ); теперь длинная сторона
// укладывается в px, а площадь кадра ограничена px².
func TestSheetLongFrame(t *testing.T) {
	cases := []struct {
		name string
		w, h float64 // страница (лист разворота вдвое шире)
	}{
		{"высокий", 25, 1000}, {"широкий", 500, 20}, {"обычный", 210, 297}, {"квадрат", 100, 100},
	}
	const px = 1600
	for _, c := range cases {
		a := colorApp(t, nil, func(p *model.Project, sp *model.Spread) { p.Settings.PageW, p.Settings.PageH = c.w, c.h })
		sv, _ := viewSheet(a.editor().View(), "s1")
		img, err := a.Sheet(context.Background(), "s1", px)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		b := img.Bounds()
		if b.Dx() > px || b.Dy() > px || b.Dx()*b.Dy() > px*px {
			t.Errorf("%s: лист %v×%v мм, кадр %d×%d, ждали стороны не больше %d", c.name, sv.W, sv.H, b.Dx(), b.Dy(), px)
		}
		if b.Empty() {
			t.Errorf("%s: пустой кадр", c.name)
		}
	}
}

// sheetDPI: края — нулевой и отрицательный px, нулевая сторона, сверхдлинный лист; dpi не меньше
// pickMinDPI.
func TestSheetDPI(t *testing.T) {
	cases := []struct {
		px   int
		w, h float64
		want int
	}{
		{1600, 100, 1000, 40}, {1600, 1000, 100, 40}, {1600, 420, 297, 96}, {1600, 100000, 100, pickMinDPI},
		{0, 100, 100, pickMinDPI}, {-5, 100, 100, pickMinDPI}, {1600, 0, 0, 40640}, {1600, 1e9, 1e9, pickMinDPI},
	}
	for _, c := range cases {
		if got := sheetDPI(c.px, c.w, c.h); got != c.want {
			t.Errorf("sheetDPI(%d, %v, %v) = %d, ждали %d", c.px, c.w, c.h, got, c.want)
		}
	}
}
