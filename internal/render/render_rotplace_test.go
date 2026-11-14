package render

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

var magenta = color.RGBA{255, 0, 255, 255}

// tiltImg — Img кадра с поворотом фото r (как geom.PlaceImage): фото закрывает рамку.
func tiltImg(cell layout.Rect, aspect, s, fx, fy, r float64) layout.Rect {
	sn, cs := sincosDeg(r)
	as, ac := math.Abs(sn), math.Abs(cs)
	bw, bh := cell.W*ac+cell.H*as, cell.W*as+cell.H*ac
	w, h := bw, bw/aspect
	if h < bh {
		h, w = bh, bh*aspect
	}
	w, h = w*s, h*s
	ux, uy := -(w-bw)*fx/2, -(h-bh)*fy/2
	ox, oy := ux*cs-uy*sn, ux*sn+uy*cs
	cx, cy := center(cell)
	return layout.Rect{X: cx + ox - w/2, Y: cy + oy - h/2, W: w, H: h}
}

// containImg — фото целиком в рамке под наклоном r (design-rotation §2.2), по центру.
func containImg(cell layout.Rect, aspect, r float64) layout.Rect {
	sn, cs := sincosDeg(r)
	s, c := math.Abs(sn), math.Abs(cs)
	w := min(cell.W/(c+s/aspect), cell.H/(s+c/aspect))
	h := w / aspect
	cx, cy := center(cell)
	return layout.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
}

// turn — точка (x, y), повёрнутая на deg по часовой вокруг (cx, cy).
func turn(x, y, cx, cy, deg float64) (float64, float64) {
	s, c := math.Sincos(deg * math.Pi / 180)
	dx, dy := x-cx, y-cy
	return cx + dx*c - dy*s, cy + dx*s + dy*c
}

// onScreen — точка фото (du, dv мм от центра Img в его осях, после зеркала) на развороте, мм.
func onScreen(ph Photo, du, dv float64) (float64, float64) {
	ix, iy := center(ph.Img)
	x, y := turn(ix+du, iy+dv, ix, iy, ph.ImgRot)
	cx, cy := center(ph.Cell)
	return turn(x, y, cx, cy, ph.Rot)
}

// polyMM — прямоугольник r, повёрнутый на deg вокруг (cx, cy), в px при dpi.
func polyMM(r layout.Rect, cx, cy, deg float64, dpi int) []pt {
	k := float64(dpi) / 25.4
	out := make([]pt, 0, 4)
	for _, p := range [4][2]float64{{r.X, r.Y}, {r.X + r.W, r.Y}, {r.X + r.W, r.Y + r.H}, {r.X, r.Y + r.H}} {
		x, y := turn(p[0], p[1], cx, cy, deg)
		out = append(out, pt{x * k, y * k})
	}
	return out
}

// photoPoly — фото на холсте (px): Img вокруг своего центра на ImgRot, затем вокруг центра ячейки на Rot.
func photoPoly(ph Photo, dpi int) []pt {
	k := float64(dpi) / 25.4
	ix, iy := center(ph.Img)
	cx, cy := center(ph.Cell)
	out := polyMM(ph.Img, ix, iy, ph.ImgRot, dpi)
	for i, p := range out {
		x, y := turn(p.x/k, p.y/k, cx, cy, ph.Rot)
		out[i] = pt{x * k, y * k}
	}
	return out
}

func renderOn(t testing.TB, ph Photo, bg color.RGBA, dpi int) *image.RGBA {
	t.Helper()
	img, err := Render(Spread{Bg: [2]color.RGBA{bg, bg}, Photos: []Photo{ph}}, Options{PageW: 100, PageH: 100, DPI: dpi})
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// Поворот места: четверти фото на холсте там, куда их повернул экран, — под ¼ оборота
// (путь 1 через fold) и под произвольным углом (путь 3), в том числе вместе с наклоном фото.
func TestRenderPlaceRot(t *testing.T) {
	c := flipCols
	q := quadsBd(t)
	cell := layout.Rect{X: 30, Y: 30, W: 40, H: 40}
	for _, a := range [][2]float64{{90, 0}, {-90, 0}, {180, 0}, {30, 0}, {-135, 0}, {30, 60}, {90, 10}} {
		ph := Photo{Cell: cell, Img: tiltImg(cell, 1, 1, 0, 0, a[1]), Rot: a[0], ImgRot: a[1], Path: q}
		for _, flip := range []bool{false, true} {
			ph.FlipH = flip
			img := renderOn(t, ph, magenta, 100)
			// Центры четвертей фото (ЛВ, ПВ, ЛН, ПН) — внутри вписанного круга рамки.
			want := [4]color.RGBA{c.red, c.grn, c.blu, c.yel}
			if flip {
				want = [4]color.RGBA{c.grn, c.red, c.yel, c.blu}
			}
			for i, d := range [4][2]float64{{-8, -8}, {8, -8}, {-8, 8}, {8, 8}} {
				x, y := onScreen(ph, d[0], d[1])
				if got := img.RGBAAt(px(x, 100), px(y, 100)); !near(got, want[i], 16) {
					t.Errorf("место %v°, фото %v°, ↔ %v: четверть %d в (%.1f, %.1f) — %v, ждали %v", a[0], a[1], flip, i, x, y, got, want[i])
				}
			}
		}
	}
	// Кадр сдвинут от центра рамки: при ¼ оборота fold поворачивает и сдвиг (знак dy·sin).
	// Точки — сетка внутри рамки (повёрнута с ней); четверть фото — по знаку координат в его осях.
	for _, r := range []float64{90, 180, -90, 30} {
		ph := Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1.4, 1, -0.6, 0), Rot: r, Path: q}
		img := renderOn(t, ph, magenta, 100)
		cols := [4]color.RGBA{c.red, c.grn, c.blu, c.yel}
		n := 0
		for gx := 34.0; gx <= 66; gx += 4 {
			for gy := 34.0; gy <= 66; gy += 4 {
				x, y := turn(gx, gy, 50, 50, r)
				du, dv := offScreen(ph, x, y)
				if math.Abs(du) < 3 || math.Abs(dv) < 3 {
					continue // у границы четвертей цвет смешан
				}
				i := 0
				if du > 0 {
					i++
				}
				if dv > 0 {
					i += 2
				}
				n++
				if got := img.RGBAAt(px(x, 100), px(y, 100)); !near(got, cols[i], 16) {
					t.Errorf("место %v°, сдвинутый кадр: (%.1f, %.1f) — %v, ждали четверть %d %v", r, x, y, got, i, cols[i])
				}
			}
		}
		if n < 40 {
			t.Fatalf("место %v°: проверено всего %d точек", r, n)
		}
	}
}

// offScreen — обратное к onScreen: точка разворота (мм) → du, dv от центра Img в его осях.
func offScreen(ph Photo, x, y float64) (float64, float64) {
	cx, cy := center(ph.Cell)
	x, y = turn(x, y, cx, cy, -ph.Rot)
	ix, iy := center(ph.Img)
	x, y = turn(x, y, ix, iy, -ph.ImgRot)
	return x - ix, y - iy
}

// Наклон фото (fill) закрывает рамку при любом сочетании углов и кадра: внутри нет цвета страницы.
func TestRenderFrameR(t *testing.T) {
	q := quadsBd(t)
	cell := layout.Rect{X: 20, Y: 30, W: 50, H: 30}
	for _, a := range [][6]float64{ // Rot, ImgRot, S, X, Y, ↔ (1 — зеркало по горизонтали)
		{0, 17, 1, 0, 0, 0}, {0, -44.9, 1, 1, -1, 0}, {0, 107, 1.4, -1, 1, 0}, {25, 17, 1, 1, 1, 0}, {90, 10, 1, -1, 0, 0}, {-60, -30.5, 2, 0.3, -0.7, 0},
		// ↔ при сдвиге по X: окно оригинала берётся с другой стороны кадра.
		{0, 17, 1.6, 1, 0, 1}, {30, -12, 1.6, -1, 0.5, 1},
	} {
		ph := Photo{Cell: cell, Img: tiltImg(cell, 1, a[2], a[3], a[4], a[1]), Rot: a[0], ImgRot: a[1], Path: q,
			FlipV: a[2] > 1 && a[5] == 0, FlipH: a[5] == 1}
		img := renderOn(t, ph, magenta, 100)
		frame := polyMM(cell, cell.X+cell.W/2, cell.Y+cell.H/2, a[0], 100)
		n := 0
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				if !inside(frame, pt{float64(x) + 0.5, float64(y) + 0.5}, -1) {
					continue
				}
				n++
				if got := img.RGBAAt(x, y); near(got, magenta, 60) {
					t.Fatalf("углы %v: в рамке (%d, %d) цвет страницы %v", a, x, y, got)
				}
			}
		}
		if n < 4000 {
			t.Fatalf("углы %v: в рамке всего %d px", a, n)
		}
	}
}

// checkClip — вне многоугольника poly дальше 1 px холст цел, внутри дальше 1,5 px — нарисовано.
func checkClip(t *testing.T, what string, img *image.RGBA, poly []pt, bg color.RGBA) {
	t.Helper()
	in := 0
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			p, got := pt{float64(x) + 0.5, float64(y) + 0.5}, img.RGBAAt(x, y)
			switch {
			case !inside(poly, p, 1) && got != bg:
				t.Fatalf("%s: вне маски (%d, %d) — %v", what, x, y, got)
			case inside(poly, p, -1.5):
				in++
				if near(got, bg, 60) {
					t.Fatalf("%s: внутри маски (%d, %d) цвет страницы", what, x, y)
				}
			}
		}
	}
	if in < 1000 {
		t.Fatalf("%s: внутри маски всего %d px", what, in)
	}
}

// Путь 3: вне повёрнутой рамки холст не тронут; у contain — и вне фото; рамка за краем разворота
// обрезается холстом.
func TestRenderRotMaskClips(t *testing.T) {
	q := quadsBd(t)
	for _, a := range [][2]float64{{30, 0}, {30, 12}, {-75, -40}} {
		cell := layout.Rect{X: 40, Y: 20, W: 60, H: 35}
		ph := Photo{Cell: cell, Img: tiltImg(cell, 1, 1.2, 0.5, -0.5, a[1]), Rot: a[0], ImgRot: a[1], Path: q}
		frame := polyMM(cell, cell.X+cell.W/2, cell.Y+cell.H/2, a[0], 100)
		checkClip(t, "fill", renderOn(t, ph, magenta, 100), frame, magenta)

		ph.Img = containImg(cell, 1, a[1])
		checkClip(t, "contain", renderOn(t, ph, magenta, 100), clipPoly(frame, photoPoly(ph, 100)), magenta)
	}
	// Место наполовину за левым краем разворота.
	cell := layout.Rect{X: -20, Y: 30, W: 50, H: 40}
	ph := Photo{Cell: cell, Img: tiltImg(cell, 1, 1, 0, 0, 0), Rot: 20, Path: q}
	img := renderOn(t, ph, magenta, 100)
	if got := img.RGBAAt(px(5, 100), px(50, 100)); near(got, magenta, 60) {
		t.Fatalf("видимая часть места не нарисована: %v", got)
	}
}

// contain с наклоном при прямом месте — тоже путь 3: край фото внутри рамки сглажен маской,
// а не ступенькой; вне фото холст цел.
func TestRenderContainTiltMask(t *testing.T) {
	cream := color.RGBA{250, 235, 200, 255}
	q := quadrants(t, testkit.TempDir(t), [4]color.RGBA{cream, cream, cream, cream})
	cell := layout.Rect{X: 20, Y: 20, W: 60, H: 40}
	ph := Photo{Cell: cell, Img: containImg(cell, 1, 20), ImgRot: 20, Path: q}
	img := renderOn(t, ph, magenta, 100)
	poly := clipPoly(polyMM(cell, 50, 40, 0, 100), photoPoly(ph, 100))
	checkClip(t, "contain 20°", img, poly, magenta)
	mixed := 0
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if got := img.RGBAAt(x, y); !near(got, magenta, 20) && !near(got, cream, 20) {
				mixed++
			}
		}
	}
	if mixed < 40 {
		t.Fatalf("на краю фото %d смешанных пикселей: край не сглажен", mixed)
	}
	// Цвет краевого пикселя — доля фото в нём (суперсэмпл 8×8): без поля из крайних
	// пикселей (padEdge) пиксели с покрытием < ½ остались бы цветом страницы.
	edge, bad := 0, 0
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			if c := (pt{float64(x) + 0.5, float64(y) + 0.5}); !inside(poly, c, 1) || inside(poly, c, -1) {
				continue
			}
			in := 0
			for sy := range 8 {
				for sx := range 8 {
					if inside(poly, pt{float64(x) + (float64(sx)+0.5)/8, float64(y) + (float64(sy)+0.5)/8}, 0) {
						in++
					}
				}
			}
			if in == 0 || in == 64 {
				continue
			}
			edge++
			cov, got := float64(in)/64, float64(img.RGBAAt(x, y).G)/float64(cream.G)
			if math.Abs(got-cov) > 0.25 {
				bad++
				t.Logf("(%d, %d): покрытие %.2f, доля цвета фото %.2f", x, y, cov, got)
			}
		}
	}
	if edge < 100 || bad > 0 {
		t.Fatalf("край фото: %d из %d пикселей не по покрытию", bad, edge)
	}
}

// Паника в потоке полосы — ошибка наружу, остальные полосы не падают вместе с процессом.
func TestInParallelPanicBecomesError(t *testing.T) {
	parts := strips(image.Rect(0, 0, 10, 100))
	err := inParallel(parts, 4, func(_ int, r image.Rectangle) {
		if r.Min.Y == parts[1].Min.Y {
			panic("полоса")
		}
	})
	if err == nil || !strings.Contains(err.Error(), "полоса") {
		t.Fatalf("ждали ошибку с причиной паники, получили %v", err)
	}
	if err := inParallel(parts, 4, func(int, image.Rectangle) {}); err != nil {
		t.Fatal(err)
	}
	// И через drawItems: путь 3 с испорченным заданием — ошибка рендера, а не падение.
	job := &rotJob{bb: image.Rect(0, 0, 10, 10), poly: rectPoly(image.Rect(0, 0, 10, 10))}
	if err := paintSafe(job.draw, image.NewRGBA(image.Rect(0, 0, 10, 10)), item{ph: Photo{Path: "x.jpg"}}); err == nil ||
		!strings.Contains(err.Error(), "x.jpg") {
		t.Fatalf("паника в потоке пути 3: %v", err)
	}
}

// Фото меньше тысячной пикселя — рисовать нечего, файл даже не открывается.
func TestPrepareRotTinyPhoto(t *testing.T) {
	cell := layout.Rect{X: 10, Y: 10, W: 20, H: 20}
	for _, img := range []layout.Rect{{X: 20, Y: 20, W: 1e-6, H: 5}, {X: 20, Y: 20, W: 5, H: 1e-6}} {
		job, err := prepareRot(context.Background(), Photo{Cell: cell, Img: img, ImgRot: 10, Path: "нет.jpg"}, 300, image.Rect(0, 0, 1000, 1000))
		if job != nil || err != nil {
			t.Fatalf("%v: %v, %v", img, job, err)
		}
	}
}

// writePNG пишет изображение без потерь: эталон не должен отличаться пережатием.
func writePNG(t *testing.T, path string, im image.Image) string {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// ¼ оборота (путь 1) = prepare на заранее повёрнутом исходнике (и отражённом, если есть зеркало).
func TestRenderTurnMatchesPrepare(t *testing.T) {
	dir := testkit.TempDir(t)
	path := testkit.JPEG(t, dir, "g.jpg", 300, 200)
	src, release, err := imgfile.OpenOriented(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cell := layout.Rect{X: 20, Y: 15, W: 40, H: 60}
	for _, c := range []struct {
		r    float64
		flip bool
	}{{90, false}, {180, false}, {-90, false}, {90, true}, {180, true}} {
		ph := Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1.3, 0.5, -0.4, c.r), ImgRot: c.r, FlipH: c.flip, Path: path}
		ref := image.Image(src)
		if c.flip {
			ref = imaging.FlipH(ref)
		}
		ix, iy := center(ph.Img)
		refImg := ph.Img
		switch c.r {
		case 90:
			ref = imaging.Rotate270(ref)
		case 180:
			ref = imaging.Rotate180(ref)
		case -90:
			ref = imaging.Rotate90(ref)
		}
		if c.r != 180 {
			refImg = layout.Rect{X: ix - ph.Img.H/2, Y: iy - ph.Img.W/2, W: ph.Img.H, H: ph.Img.W}
		}
		refPath := writePNG(t, filepath.Join(dir, "ref.png"), ref)
		got := renderOn(t, ph, magenta, 100)
		want := renderOn(t, Photo{Cell: cell, Img: refImg, Path: refPath}, magenta, 100)
		var sum, n int
		for y := range got.Bounds().Dy() {
			for x := range got.Bounds().Dx() {
				a, b := got.RGBAAt(x, y), want.RGBAAt(x, y)
				for _, d := range []int{int(a.R) - int(b.R), int(a.G) - int(b.G), int(a.B) - int(b.B)} {
					sum += max(d, -d)
					n++
				}
			}
		}
		if mean := float64(sum) / float64(n); mean > 0.2 {
			t.Fatalf("%v° ↔ %v: средняя разница с эталоном %.3f/255", c.r, c.flip, mean)
		}
	}
}

// Слои: текст с меньшим Z закрыт фото, с большим — поверх; фон всегда под ними.
func TestRenderZOrderTextUnderPhoto(t *testing.T) {
	cream := color.RGBA{250, 235, 200, 255}
	q := quadrants(t, testkit.TempDir(t), [4]color.RGBA{cream, cream, cream, cream})
	st := text.Style{Font: "montserrat", Size: 24, Color: "#000000", Align: "left"}
	cell := layout.Rect{X: 10, Y: 10, W: 80, H: 40}
	ink := func(tz, pz int) bool {
		blk := textBlock("Тест", 15, 20, 60, st)
		blk.Z = tz
		img, err := Render(Spread{Photos: []Photo{{Cell: cell, Img: cell, Path: q, Z: pz}}, Texts: []TextDraw{blk}},
			Options{PageW: 100, PageH: 100, DPI: 100})
		if err != nil {
			t.Fatal(err)
		}
		// Чернила — тёмные пиксели внутри ячейки (JPEG даёт кремовому ±несколько единиц).
		for y := px(11, 100); y < px(49, 100); y++ {
			for x := px(11, 100); x < px(89, 100); x++ {
				if img.RGBAAt(x, y).R < 120 {
					return true
				}
			}
		}
		return false
	}
	if ink(1, 2) {
		t.Fatal("текст ниже фото виден поверх него")
	}
	if !ink(3, 2) {
		t.Fatal("текст выше фото не нарисован")
	}
	if !ink(0, 0) {
		t.Fatal("при равных Z текст должен лечь поверх фото, как раньше")
	}
}

// Бенчмарки — рамка во всю страницу 280×280 мм при 300 dpi, фото 6000×4000 (design-rotation §3).
func benchRender(b *testing.B, ph func(cell layout.Rect, path string) Photo) {
	path := testkit.JPEG(b, testkit.TempDir(b), "big.jpg", 6000, 4000)
	cell := layout.Rect{W: 280, H: 280}
	s := Spread{Photos: []Photo{ph(cell, path)}}
	opt := Options{PageW: 280, PageH: 280, DPI: 300}
	b.ResetTimer()
	for range b.N {
		if _, err := Render(s, opt); err != nil {
			b.Fatal(err)
		}
	}
}

// Путь 3: место под 5°.
func BenchmarkRenderPlaceRot(b *testing.B) {
	benchRender(b, func(cell layout.Rect, path string) Photo {
		return Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1, 0, 0, 0), Rot: 5, Path: path}
	})
}

// Путь 2: наклон фото 5° при прямом месте.
func BenchmarkRenderFrameTilt(b *testing.B) {
	benchRender(b, func(cell layout.Rect, path string) Photo {
		return Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1, 0, 0, 5), ImgRot: 5, Path: path}
	})
}

// Путь 1: фото на ¼ оборота.
func BenchmarkRenderTurn90(b *testing.B) {
	benchRender(b, func(cell layout.Rect, path string) Photo {
		return Photo{Cell: cell, Img: tiltImg(cell, 1.5, 1, 0, 0, 90), ImgRot: 90, Path: path}
	})
}
