package faces

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// refBox — эталон усреднения по площади: прямой счёт по непрерывным границам частей.
func refBox(img image.Image, dw, dh int) [][3]float64 {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	out := make([][3]float64, dw*dh)
	for oy := range dh {
		for ox := range dw {
			x0, x1 := float64(ox)*float64(sw)/float64(dw), float64(ox+1)*float64(sw)/float64(dw)
			y0, y1 := float64(oy)*float64(sh)/float64(dh), float64(oy+1)*float64(sh)/float64(dh)
			var acc [3]float64
			for y := int(y0); y < sh && float64(y) < y1; y++ {
				for x := int(x0); x < sw && float64(x) < x1; x++ {
					w := (math.Min(x1, float64(x+1)) - math.Max(x0, float64(x))) * (math.Min(y1, float64(y+1)) - math.Max(y0, float64(y)))
					r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
					acc[0] += w * float64(r>>8)
					acc[1] += w * float64(g>>8)
					acc[2] += w * float64(bl>>8)
				}
			}
			area := (x1 - x0) * (y1 - y0)
			out[oy*dw+ox] = [3]float64{acc[0] / area, acc[1] / area, acc[2] / area}
		}
	}
	return out
}

// TestBoxResample: усреднение по площади совпадает с прямым счётом — при целом и дробном
// коэффициенте, по одной оси и по обеим, на сжатии, увеличении и 1:1, на быстрых путях и через At.
func TestBoxResample(t *testing.T) {
	yc := image.NewYCbCr(image.Rect(0, 0, 37, 23), image.YCbCrSubsampleRatio422)
	for i := range yc.Y {
		yc.Y[i] = uint8(i * 7)
	}
	for i := range yc.Cb {
		yc.Cb[i], yc.Cr[i] = uint8(i*13), uint8(255-i*5)
	}
	gray := image.NewGray(image.Rect(0, 0, 19, 11))
	for i := range gray.Pix {
		gray.Pix[i] = uint8(i * 11)
	}
	imgs := map[string]image.Image{
		"RGBA": noiseRGBA(37, 23), "YCbCr": yc, "Gray": gray,
		"NRGBA":       nrgbaOf(noiseRGBA(29, 31)),
		"подкартинка": noiseRGBA(60, 50).SubImage(image.Rect(11, 7, 48, 30)),
	}
	sizes := [][2]int{{37, 23}, {18, 11}, {12, 23}, {37, 5}, {5, 3}, {1, 1}, {50, 40}, {74, 46}, {7, 29}}
	for name, img := range imgs {
		for _, sz := range sizes {
			dw, dh := sz[0], sz[1]
			want := refBox(img, dw, dh)
			got := make([][3]float64, dw*dh)
			count := 0
			boxResample(img, dw, dh, func(x, y int, r, g, b float32) {
				got[y*dw+x] = [3]float64{float64(r), float64(g), float64(b)}
				count++
			})
			if count != dw*dh {
				t.Fatalf("%s %dx%d: отдано %d пикселей из %d", name, dw, dh, count, dw*dh)
			}
			for i := range want {
				for c := range 3 {
					if math.Abs(got[i][c]-want[i][c]) > 2e-3 {
						t.Fatalf("%s → %dx%d: пиксель %d канал %d: %.4f, эталон %.4f", name, dw, dh, i, c, got[i][c], want[i][c])
					}
				}
			}
		}
	}
}

func nrgbaOf(src *image.RGBA) *image.NRGBA {
	out := image.NewNRGBA(src.Bounds())
	for y := range src.Bounds().Dy() {
		for x := range src.Bounds().Dx() {
			c := src.RGBAAt(x, y)
			out.SetNRGBA(x, y, color.NRGBA{c.R, c.G, c.B, 255})
		}
	}
	return out
}

// TestTurnQuarters: Turn поворачивает по часовой, копии нет, повороты складываются и берутся по
// модулю 4, At всех четвертей совпадает с поворотом руками.
func TestTurnQuarters(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			src.SetRGBA(x, y, color.RGBA{uint8(10 * (y*4 + x + 1)), 0, 0, 255})
		}
	}
	// 4×2 по часовой → 2×4: верхний левый угол уходит в верхний правый
	cw := Turn(src, 1)
	if b := cw.Bounds(); b.Dx() != 2 || b.Dy() != 4 {
		t.Fatalf("после четверти размер %v", b)
	}
	red := func(img image.Image, x, y int) int { r, _, _, _ := img.At(x, y).RGBA(); return int(r >> 8) }
	if got, want := red(cw, 1, 0), 10; got != want { // был (0,0)
		t.Errorf("(0,0) → (1,0): %d, ждали %d", got, want)
	}
	if got, want := red(cw, 0, 0), 50; got != want { // был (0,1) — левый нижний → левый верхний
		t.Errorf("(0,1) → (0,0): %d, ждали %d", got, want)
	}
	if got, want := red(cw, 0, 3), 80; got != want { // был (3,1)
		t.Errorf("(3,1) → (0,3): %d, ждали %d", got, want)
	}
	if Turn(src, 0) != image.Image(src) || Turn(src, 4) != image.Image(src) || Turn(src, -4) != image.Image(src) {
		t.Error("поворот на 0 четвертей вернул не ту же картинку")
	}
	if tt := Turn(Turn(src, 1), 3); tt != image.Image(src) {
		t.Error("1 + 3 четверти не сложились в 0")
	}
	ccw, neg := Turn(src, 3), Turn(src, -1)
	for y := range 4 {
		for x := range 2 {
			if red(ccw, x, y) != red(neg, x, y) {
				t.Fatalf("-1 и 3 четверти различаются в (%d,%d)", x, y)
			}
		}
	}
	if Turn(nil, 1) != nil {
		t.Error("Turn(nil) не nil")
	}
	// At всех четвертей совпадает с поворотом руками
	for q := range 4 {
		lazy, hand := Turn(src, q), rotateRGBA(src, q)
		if lazy.Bounds() != hand.Bounds() {
			t.Fatalf("q=%d: границы %v и %v", q, lazy.Bounds(), hand.Bounds())
		}
		for y := range hand.Bounds().Dy() {
			for x := range hand.Bounds().Dx() {
				if red(lazy, x, y) != red(hand, x, y) {
					t.Fatalf("q=%d: пиксель (%d,%d) не совпал", q, x, y)
				}
			}
		}
	}
}

// TestTurnInput: вход сети от «ленивого» поворота равен входу от картинки, повёрнутой руками.
func TestTurnInput(t *testing.T) {
	pic := noiseRGBA(90, 55)
	for q := range 4 {
		lazy, ok := prepareInput(Turn(pic, q), 64)
		if !ok {
			t.Fatal("не принято")
		}
		hand, ok := prepareInput(rotateRGBA(pic, q), 64)
		if !ok {
			t.Fatal("не принято")
		}
		if lazy.w != hand.w || lazy.h != hand.h || lazy.W != hand.W || lazy.H != hand.H {
			t.Fatalf("q=%d: размеры входа %dx%d/%dx%d и %dx%d/%dx%d", q, lazy.w, lazy.h, lazy.W, lazy.H, hand.w, hand.h, hand.W, hand.H)
		}
		for i := range lazy.d {
			if math.Abs(float64(lazy.d[i]-hand.d[i])) > 1e-3 {
				t.Fatalf("q=%d: элемент %d: %v и %v", q, i, lazy.d[i], hand.d[i])
			}
		}
	}
}

// rotateRGBA — поворот на q четвертей по часовой руками: эталон для Turn.
func rotateRGBA(src *image.RGBA, q int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := src
	for range q {
		nw, nh := h, w
		next := image.NewRGBA(image.Rect(0, 0, nw, nh))
		for y := range nh {
			for x := range nw {
				next.SetRGBA(x, y, out.RGBAAt(y, h-1-x))
			}
		}
		out, w, h = next, nw, nh
	}
	return out
}

// TestPrepareInputShape: длинная сторона 640, короткая кратна 32, картинка — в левом верхнем
// углу, остальное нули; нулевые и огромные картинки отвергаются.
func TestPrepareInputShape(t *testing.T) {
	in, ok := prepareInput(noiseRGBA(2000, 1333), inputLong)
	if !ok || in.w != 640 || in.h != 427 || in.W != 640 || in.H != 448 {
		t.Fatalf("2000×1333: %+v", in)
	}
	plane := in.W * in.H
	if len(in.d) != 3*plane {
		t.Fatalf("длина тензора %d", len(in.d))
	}
	if in.d[in.h*in.W+5] != 0 || in.d[2*plane+(in.H-1)*in.W] != 0 {
		t.Error("поля вокруг картинки не нулевые")
	}
	if in.d[0] == 0 && in.d[1] == 0 && in.d[in.W] == 0 {
		t.Error("картинка в углу не записана")
	}
	if tall, ok := prepareInput(noiseRGBA(300, 900), inputLong); !ok || tall.h != 640 || tall.w != 213 || tall.W != 224 {
		t.Errorf("900 в высоту: %+v", tall)
	}
	if _, ok := prepareInput(image.NewRGBA(image.Rect(0, 0, 0, 10)), inputLong); ok {
		t.Error("нулевая ширина принята")
	}
}

// TestPrepareInputBGR: вход сети — BGR, а не RGB: красная картинка попадает в канал 2, синяя —
// в канал 0 (модель обучена на BGR, перепутанные каналы молча снижают оценки). Поля вокруг
// картинки остаются нулями, на каждом канале.
func TestPrepareInputBGR(t *testing.T) {
	for _, c := range []struct {
		name string
		col  color.RGBA
		want [3]float32 // B, G, R — по плоскостям тензора
	}{
		{"красный", color.RGBA{255, 0, 0, 255}, [3]float32{0, 0, 255}},
		{"зелёный", color.RGBA{0, 255, 0, 255}, [3]float32{0, 255, 0}},
		{"синий", color.RGBA{0, 0, 255, 255}, [3]float32{255, 0, 0}},
		{"серый", color.RGBA{40, 40, 40, 255}, [3]float32{40, 40, 40}},
		{"разные", color.RGBA{10, 100, 200, 255}, [3]float32{200, 100, 10}},
	} {
		img := image.NewRGBA(image.Rect(0, 0, 90, 60))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.col.R, c.col.G, c.col.B, 255
		}
		for q := range 4 { // поворот переставляет пиксели, но не каналы
			in, ok := prepareInput(Turn(img, q), inputLong)
			if !ok {
				t.Fatalf("%s: вход не собран", c.name)
			}
			checkPlanes(t, c.name, q, in, c.want)
		}
	}
}

// checkPlanes: внутри картинки каждая плоскость входа равна want, на поле — нулю.
func checkPlanes(t *testing.T, name string, q int, in netInput, want [3]float32) {
	t.Helper()
	plane := in.W * in.H
	for ch, w := range want {
		for y := range in.H {
			for x := range in.W {
				exp := w
				if x >= in.w || y >= in.h {
					exp = 0 // поле
				}
				if got := in.d[ch*plane+y*in.W+x]; math.Abs(float64(got-exp)) > 0.01 {
					t.Fatalf("%s, поворот %d: канал %d в (%d, %d) = %v, ждали %v", name, q, ch, x, y, got, exp)
				}
			}
		}
	}
}
