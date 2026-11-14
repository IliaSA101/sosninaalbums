package faces

import (
	"image"
	"image/color"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// noiseRGBA — детерминированная «фотография» без лиц: градиент с шумом.
func noiseRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(12345)
	for y := range h {
		for x := range w {
			seed = seed*1664525 + 1013904223
			n := uint8(seed >> 28)
			img.SetRGBA(x, y, color.RGBA{uint8(x*255/max(w, 1)) ^ n, uint8(y*255/max(h, 1)) + n, uint8((x + y) & 255), 255})
		}
	}
	return img
}

// rawOuts — выходы голов на картинке при заданном числе потоков.
func rawOuts(t *testing.T, img image.Image, workers int) map[string]headOut {
	t.Helper()
	n, err := loadNet()
	if err != nil {
		t.Fatal(err)
	}
	in, ok := prepareInput(img, inputLong)
	if !ok {
		t.Fatal("картинка не принята")
	}
	outs, err := n.run(in.d, in.H, in.W, workers, &memTrack{})
	if err != nil {
		t.Fatal(err)
	}
	return outs
}

// TestDetectPeakMem: пик живых тензоров одного прогона на кадре 640×448 — не больше 30 МБ.
// Тензор отдаётся после последнего потребителя; прототип держал все и занимал 53 МБ (design §2).
func TestDetectPeakMem(t *testing.T) {
	tr := &memTrack{}
	if _, err := detect(noiseRGBA(640, 448), 1, tr); err != nil {
		t.Fatal(err)
	}
	const limit = 30 << 20
	t.Logf("пик живых тензоров на 640×448: %.1f МБ (предел 30)", float64(tr.peak)/(1<<20))
	if tr.peak > limit {
		t.Errorf("пик %d Б > %d Б: тензоры не освобождаются после последнего потребителя", tr.peak, limit)
	}
	if tr.peak < 1<<20 {
		t.Errorf("пик %d Б: счётчик не считает", tr.peak)
	}
	// к концу живы только выходы голов (≈ 0,5 МБ на 640×640), промежуточных тензоров нет
	if tr.live > 1<<20 {
		t.Errorf("после прогона живо %d Б: утечка тензоров", tr.live)
	}
}

// TestDetectFast: сеть на кадре 640 в один поток укладывается с большим запасом (замер ≈ 0,1–0,3 с;
// порог — на «не стало квадратичным», а не на скорость, testkit.Limit).
func TestDetectFast(t *testing.T) {
	testkit.CostTest(t)
	img := noiseRGBA(2048, 1365)
	d := testkit.Fastest(3, func() { Detect(img) })
	t.Logf("Detect 2048×1365 → 640×448, один поток: %v", d.Round(time.Millisecond))
	if lim := testkit.Limit(2 * time.Second); d > lim {
		t.Errorf("Detect занял %v > %v", d, lim)
	}
}

// TestDetectStable: повтор и число потоков не меняют выходы сети ни на бит (потоки делят выходные
// каналы, суммы не дробятся) — иначе лист по кэшу и без него давал бы разные кадры.
func TestDetectStable(t *testing.T) {
	img := noiseRGBA(320, 213)
	one := rawOuts(t, img, 1)
	again := rawOuts(t, img, 1)
	many := rawOuts(t, img, 5)
	for name, want := range one {
		for label, got := range map[string]headOut{"повтор": again[name], "5 потоков": many[name]} {
			if got.n != want.n || got.c != want.c {
				t.Fatalf("%s %s: форма другая", name, label)
			}
			for i := range want.d {
				if math.Float32bits(want.d[i]) != math.Float32bits(got.d[i]) {
					t.Fatalf("%s %s: элемент %d отличается: %v и %v", name, label, i, want.d[i], got.d[i])
				}
			}
		}
	}
	// и целиком Detect, на фото с лицами, если они есть
	if dir := photosDir("portraits"); dir != "" && !testkit.Race { // под -race четыре прогона по фото — лишнее
		pic := loadJPEG(t, dir+"/portrait-01.jpg")
		a, b, c := Detect(pic), Detect(pic), DetectN(pic, 4)
		if len(a) == 0 || !reflect.DeepEqual(a, b) || !reflect.DeepEqual(a, c) {
			t.Errorf("лица на портрете не повторяются: %+v / %+v / %+v", a, b, c)
		}
	}
}

// TestDetectRace: три Detect одновременно (так ищет App) дают одинаковый ответ; под -race
// ловит общее состояние сети.
func TestDetectRace(t *testing.T) {
	var pic image.Image = noiseRGBA(200, 150)
	if dir := photosDir("portraits"); dir != "" { // на настоящем лице ответ содержательный
		pic = loadJPEG(t, dir+"/portrait-02.jpg")
	}
	want := Detect(pic)
	var wg sync.WaitGroup
	got := make([][]Face, 3)
	for i := range got {
		wg.Go(func() { got[i] = Detect(pic) })
	}
	wg.Wait()
	for i, g := range got {
		if !reflect.DeepEqual(want, g) {
			t.Errorf("горутина %d: %+v, ждали %+v", i, g, want)
		}
	}
}

// TestDetectOddInputs: странные картинки дают пустой ответ, а не панику: 1×1, узкие полосы,
// серое, CMYK, прозрачное, палитра, 16 бит, подкартинка со смещением, пустая и nil.
func TestDetectOddInputs(t *testing.T) {
	pal := image.NewPaletted(image.Rect(0, 0, 40, 30), color.Palette{color.Black, color.White, color.RGBA{200, 10, 10, 255}})
	for i := range pal.Pix {
		pal.Pix[i] = uint8(i % 3)
	}
	nr := image.NewNRGBA(image.Rect(0, 0, 50, 50)) // полностью прозрачный
	yc := func(r image.YCbCrSubsampleRatio) image.Image {
		img := image.NewYCbCr(image.Rect(0, 0, 33, 17), r)
		for i := range img.Y {
			img.Y[i] = uint8(i)
		}
		return img
	}
	sub := noiseRGBA(200, 100).SubImage(image.Rect(50, 20, 130, 90))
	cases := map[string]image.Image{
		"1×1":              noiseRGBA(1, 1),
		"3×2000":           noiseRGBA(3, 2000),
		"2000×3":           noiseRGBA(2000, 3),
		"Gray":             image.NewGray(image.Rect(0, 0, 77, 51)),
		"Gray16":           image.NewGray16(image.Rect(0, 0, 21, 13)),
		"CMYK":             image.NewCMYK(image.Rect(0, 0, 64, 48)),
		"NRGBA прозрачный": nr,
		"RGBA64":           image.NewRGBA64(image.Rect(0, 0, 31, 29)),
		"Paletted":         pal,
		"YCbCr 4:2:0":      yc(image.YCbCrSubsampleRatio420),
		"YCbCr 4:4:0":      yc(image.YCbCrSubsampleRatio440),
		"YCbCr 4:1:0":      yc(image.YCbCrSubsampleRatio410),
		"подкартинка":      sub,
		"пустая":           image.NewRGBA(image.Rect(0, 0, 0, 0)),
		"узкая нулевая":    image.NewRGBA(image.Rect(5, 5, 5, 40)),
		"nil":              nil,
	}
	for name, img := range cases {
		t.Run(name, func(t *testing.T) {
			// все четверти — на малой выборке: каждая — полный прогон сети
			quarters := 1
			if !testkit.Lite() && (name == "1×1" || name == "3×2000" || name == "YCbCr 4:2:0" || name == "подкартинка") {
				quarters = 4
			}
			for q := range quarters {
				if fs := Detect(Turn(img, q)); len(fs) != 0 {
					t.Errorf("поворот %d: на картинке без лиц нашлось %+v", q, fs)
				}
			}
		})
	}
	// огромная по счёту пикселей картинка отвергается до чтения
	huge := &hugeImage{}
	if fs := Detect(huge); fs != nil || huge.reads != 0 {
		t.Errorf("картинка в 2^40 пикселей: ответ %v, прочитано %d пикселей", fs, huge.reads)
	}
}

// hugeImage — картинка 1048576×1048576 без пикселей: проверяет, что огромный размер отсекается.
type hugeImage struct{ reads int }

func (*hugeImage) ColorModel() color.Model { return color.RGBAModel }
func (*hugeImage) Bounds() image.Rectangle { return image.Rect(0, 0, 1<<20, 1<<20) }
func (h *hugeImage) At(int, int) color.Color {
	h.reads++
	return color.RGBA{}
}
