package sheetcut

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// ringSheet — белый лист 200×200 с кольцом (центр 100,100; внешний радиус 60, внутренний ri).
func ringSheet(ri float64) *image.NRGBA {
	img := canvas(200, 200, white)
	ring(img, 100, 100, 60, ri, navy, white)
	return img
}

// fillInside заливает середину кольца (радиус < r) цветом f(x, y).
func fillInside(img *image.NRGBA, r float64, f func(x, y int) color.NRGBA) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if math.Hypot(float64(x-100), float64(y-100)) < r {
				img.SetNRGBA(x, y, f(x, y))
			}
		}
	}
}

func ownOpts() BGOptions { return BGOptions{White: 250, HoleMin: 500, Edge: 3} }

func TestOwnRingWithWhiteInsideBecomesTransparent(t *testing.T) {
	img := ringSheet(35)
	st := RemoveBackground(img, ownOpts())
	if st.Holes != 1 || st.HolesKept != 0 {
		t.Fatalf("дыр снято %d, оставлено %d", st.Holes, st.HolesKept)
	}
	if alphaAt(img, 100, 100) != 0 || alphaAt(img, 0, 0) != 0 || alphaAt(img, 199, 199) != 0 {
		t.Fatalf("середина/углы должны быть прозрачными: %d %d %d", alphaAt(img, 100, 100), alphaAt(img, 0, 0), alphaAt(img, 199, 199))
	}
	if alphaAt(img, 100+48, 100) != 255 || alphaAt(img, 100, 100-48) != 255 {
		t.Fatal("тело кольца должно остаться непрозрачным")
	}
}

func TestOwnSoftEdgeRestoresColour(t *testing.T) {
	img := ringSheet(35)
	RemoveBackground(img, ownOpts())
	// Пиксели на радиусе 60 и 35 покрыты наполовину: альфа ≈ 128, цвет — тёмный предмет, не серый
	// (серый = смесь с белым: без восстановления на тёмной подложке был бы светлый ореол).
	for _, p := range []image.Point{{160, 100}, {135, 100}, {100, 160}, {100, 40}} {
		c := img.NRGBAAt(p.X, p.Y)
		if c.A < 100 || c.A > 160 {
			t.Fatalf("%v: альфа %d, ожидалась ≈128", p, c.A)
		}
		if d := colorDist(c, navy); d > 12 {
			t.Fatalf("%v: цвет %v далёк от предмета (%.0f)", p, c, d)
		}
	}
}

func colorDist(a, b color.NRGBA) float64 {
	return math.Sqrt(math.Pow(float64(a.R)-float64(b.R), 2) + math.Pow(float64(a.G)-float64(b.G), 2) + math.Pow(float64(a.B)-float64(b.B), 2))
}

func TestOwnTexturedWhiteInsideStays(t *testing.T) {
	img := ringSheet(35)
	// «Бумага с фактурой»: почти белый 250–252 с мелким рисунком — недобелённость ≈ 4 > порога.
	fillInside(img, 30, func(x, y int) color.NRGBA {
		v := uint8(250 + (x*3+y*5)%3)
		return color.NRGBA{v, v, v, 255}
	})
	st := RemoveBackground(img, ownOpts())
	if st.Holes != 0 || st.HolesKept != 1 {
		t.Fatalf("дыр снято %d, оставлено %d", st.Holes, st.HolesKept)
	}
	if alphaAt(img, 100, 100) != 255 || alphaAt(img, 90, 105) != 255 {
		t.Fatal("белое с фактурой должно остаться непрозрачным")
	}
}

func TestOwnLightPaperStays(t *testing.T) {
	// Светло-серая «бумага» (246) не «почти белая»: к фону не относится вообще.
	img := ringSheet(35)
	fillInside(img, 30, func(int, int) color.NRGBA { return color.NRGBA{246, 246, 246, 255} })
	RemoveBackground(img, ownOpts())
	if a := alphaAt(img, 100, 100); a != 255 {
		t.Fatalf("альфа бумаги %d", a)
	}
}

func TestOwnNoisyCleanWhiteInsideIsHole(t *testing.T) {
	// Шум апскейла: 253–255, недобелённость ≈ 1 — всё ещё «чистая» область.
	img := ringSheet(35)
	fillInside(img, 31, func(x, y int) color.NRGBA {
		v := uint8(253 + (x*7+y*3)%3)
		return color.NRGBA{v, v, v, 255}
	})
	st := RemoveBackground(img, ownOpts())
	if st.Holes != 1 || alphaAt(img, 100, 100) != 0 {
		t.Fatalf("дыр %d, альфа середины %d", st.Holes, alphaAt(img, 100, 100))
	}
}

func TestOwnSmallClosedWhiteStaysAndOptionsEdge(t *testing.T) {
	img := ringSheet(8) // середина ≈ 200 px — меньше HoleMin
	st := RemoveBackground(img, ownOpts())
	if st.Holes != 0 || alphaAt(img, 100, 100) != 255 {
		t.Fatalf("малая замкнутая область: снято %d, альфа %d", st.Holes, alphaAt(img, 100, 100))
	}
	// Лист целиком из белого: всё фон, предметов нет.
	blank := canvas(40, 40, white)
	if st := RemoveBackground(blank, BGOptions{}); st.BackgroundPx != 1600 || alphaAt(blank, 20, 20) != 0 {
		t.Fatalf("пустой белый лист: фон %d px", st.BackgroundPx)
	}
}

func TestOwnWhiteOnWhiteNotDissolved(t *testing.T) {
	// Белая «голубка» с тенью акварели: светло-серый (232) предмет на белом — край не растворяется,
	// внутри альфа 255.
	img := canvas(120, 120, white)
	fillRect(img, image.Rect(30, 30, 90, 90), color.NRGBA{232, 232, 232, 255})
	RemoveBackground(img, ownOpts())
	for _, p := range []image.Point{{30, 60}, {31, 60}, {60, 60}, {89, 60}} {
		if a := alphaAt(img, p.X, p.Y); a < 250 {
			t.Fatalf("%v: альфа %d, светлый предмет должен остаться", p, a)
		}
	}
	if alphaAt(img, 10, 10) != 0 {
		t.Fatal("фон должен быть прозрачным")
	}
}

func TestOwnCutSeveralObjects(t *testing.T) {
	// Лист 3×1: диск, кольцо с чистой белой серединой, кольцо с фактурной серединой.
	img := canvas(600, 200, white)
	ring(img, 100, 100, 60, 0, navy, white)
	ring(img, 300, 100, 60, 35, navy, white)
	ring(img, 500, 100, 60, 35, navy, white)
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			if math.Hypot(float64(x-500), float64(y-100)) < 30 {
				v := uint8(250 + (x*3+y*5)%3)
				img.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
			}
		}
	}
	o := DefaultOptions()
	o.BG = BGOwn
	o.HoleMin = 500
	o.Cols, o.Rows = 3, 1
	res := mustCut(t, img, o)
	if len(res.Stickers) != 3 || res.BGInfo.Holes != 1 || res.BGInfo.HolesKept != 1 {
		t.Fatalf("стикеров %d, дыры %+v", len(res.Stickers), res.BGInfo)
	}
	mid := func(i int) uint8 { s := res.Stickers[i].Img; return alphaAt(s, s.Bounds().Dx()/2, s.Bounds().Dy()/2) }
	if mid(0) != 255 || mid(1) != 0 || mid(2) != 255 {
		t.Fatalf("середины: диск %d, кольцо %d, кольцо с фактурой %d", mid(0), mid(1), mid(2))
	}
}
