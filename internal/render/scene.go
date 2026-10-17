// Package render рисует разворот в картинку: фото, фоны, тексты (мм → px).
// Папки, имена файлов и запись JPEG — в export.
package render

import (
	"fmt"
	"image/color"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Photo — фото в ячейке: Cell — ячейка, Img — фото целиком с учётом кадра
// (View.Frames[i].Img); ячейка обрезает Img. Размеры в мм разворота.
type Photo struct {
	Cell, Img layout.Rect
	Path      string // путь к оригиналу
	// FlipH/FlipV — зеркало картинки внутри Img (видимая часть остаётся той же, но отражённой).
	FlipH, FlipV bool
	// Rot — поворот ячейки вокруг её центра (FrameView.CellRot), ImgRot — фото вокруг
	// центра Img (FrameView.R): сначала зеркало в осях фото, затем ImgRot, затем Rot.
	// Img задан в осях фото относительно неповёрнутой ячейки, как во View. Градусы по часовой.
	Rot, ImgRot float64
	Z           int // слой в общем стеке разворота (с текстами)
	// Fade — прозрачность 0..1 (фон, спека 017; свой стикер, 048): картинка смешивается с
	// тем, что под ней. У фото мест всегда 0.
	Fade float64
	// Outline — форма места (044): эллипс, вписанный в Cell, или скругление, мм; нулевая —
	// прямоугольник (прежний путь дословно). Поворачивается вместе с ячейкой.
	Outline geom.Outline
}

// TextDraw — блок текста: (X, Y) — левый верхний угол в мм разворота,
// строки Box заданы относительно него (как в text.Layout). Rot — градусы по часовой
// вокруг (X, Y); у подписей 0.
type TextDraw struct {
	X, Y  float64
	Rot   float64
	Box   text.Box
	Style text.Style
	Z     int // слой в общем стеке разворота (с фото)
}

// PatternDraw — узор области (048): Area — мм листа, как у фото фона; Kind, Inks, Scale, Seed —
// параметры gen (рисует тот же код, что превью /media/gen; поля, а не gen.Spec: export собирает
// задание без импорта decor/gen, depguard); Fade — прозрачность 0..1, 1 — не рисуется.
type PatternDraw struct {
	Area  layout.Rect
	Kind  string
	Inks  []string
	Scale float64
	Seed  uint32
	Fade  float64
}

// Plate — плашка: прямоугольник сплошного цвета (мм листа), например корешок обложки (032).
type Plate struct {
	Rect  layout.Rect
	Color color.RGBA
}

// Spread — всё, что нужно для рисования разворота. Bg — цвет левой и правой страницы;
// фото, стикеры и тексты рисуются одним стеком по Z (больший сверху), при равных — фото,
// свои стикеры, процедурные, затем тексты по порядку.
type Spread struct {
	Bg        [2]color.RGBA
	Patterns  []PatternDraw // узоры (048): поверх цвета страниц, под фото фона
	Backdrops []Backdrop    // фото на фоне: поверх цвета страниц и узоров, под местами
	Plates    []Plate       // плашки: поверх фонов (кроме OverPlates), под местами и текстами
	Photos    []Photo
	// Stickers — свои стикеры (048): Cell = Img = рамка, Rot — вокруг её центра; Fade ≥ 1 не рисуется.
	Stickers []Photo
	Arts     []ArtDraw // процедурные стикеры (048)
	Texts    []TextDraw
}

type Options struct {
	PageW, PageH float64 // мм
	DPI          int
	Quality      int
	// Split — холст из двух равных страниц (режим «По страницам»), см. CanvasSize.
	Split bool
}

// Пределы холста: JPEG не бывает больше 65535 px по стороне, а RGBA на 400 Мп — это 1.6 ГБ.
const (
	maxCanvasSide   = 65535
	maxCanvasPixels = 400_000_000
)

// CheckSize проверяет размер холста в px до любых действий с файлами.
func CheckSize(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("некорректный размер разворота %d×%d px", w, h)
	}
	if w > maxCanvasSide || h > maxCanvasSide || int64(w)*int64(h) > maxCanvasPixels {
		return fmt.Errorf("разворот %d×%d px слишком большой для печати из программы: уменьшите DPI в настройках", w, h)
	}
	return nil
}

func px(mm float64, dpi int) int { return int(math.Round(mm / 25.4 * float64(dpi))) }

// CanvasSize — размер холста в px. Обычный — px(2·PageW); со Split — две равные страницы
// 2·px(PageW): иначе при 300 мм и 300 dpi (px(600) = 7087, корешок 3543) правая вышла бы
// на пиксель шире левой.
func CanvasSize(opt Options) (w, h int) {
	if opt.Split {
		return 2 * px(opt.PageW, opt.DPI), px(opt.PageH, opt.DPI)
	}
	return px(2*opt.PageW, opt.DPI), px(opt.PageH, opt.DPI)
}
