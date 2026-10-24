package testkit

import (
	"bytes"
	"image"
	"image/color"
	"reflect"
	"testing"
	"time"
)

const bigCount = 600 // размер полного набора

// Те же номер и зерно — тот же план; другое зерно меняет кадрирование, яркость и шум.
// Номер меньше 1 — пустой план, а не паника.
func TestBigSetPlanDeterministic(t *testing.T) {
	for i := 1; i <= bigCount; i++ {
		if a, b := BigSetPlan(i, 7), BigSetPlan(i, 7); !reflect.DeepEqual(a, b) {
			t.Fatalf("кадр %d: %+v != %+v", i, a, b)
		}
	}
	diff := 0
	for i := 1; i <= bigCount; i++ {
		a, b := BigSetPlan(i, 7), BigSetPlan(i, 8)
		if a.Source != b.Source || a.Variant != b.Variant || !a.Taken.Equal(b.Taken) {
			t.Fatalf("кадр %d: зерно не должно менять исходник, вариант и дату", i)
		}
		if a.NoiseDX != b.NoiseDX || a.Bright != b.Bright {
			diff++
		}
	}
	if diff < bigCount*9/10 {
		t.Fatalf("зерно изменило только %d кадров из %d", diff, bigCount)
	}
	for _, i := range []int{0, -1, -600} {
		if p := BigSetPlan(i, 7); !reflect.DeepEqual(p, BigShot{}) {
			t.Fatalf("кадр %d: ждали пустой план, получили %+v", i, p)
		}
	}
}

// Состав набора из 600: серии по 6 из одного исходника, доли ориентаций, Adobe и поворота,
// зеркало у вариантов 3 и 5, даты строго растут и сгруппированы сериями.
func TestBigSetPlanMix(t *testing.T) {
	aspects := StockAspects(t)
	var sq, adobe, rot, portrait, mirror int
	var prev time.Time
	for i := 1; i <= bigCount; i++ {
		p := BigSetPlan(i, 1)
		if p.N != i || p.Variant != (i-1)%6 || p.Source != (i-1)/6%100+1 {
			t.Fatalf("кадр %d: номер/вариант/исходник %+v", i, p)
		}
		if i > 1 && p.Variant != 0 && p.Source != BigSetPlan(i-1, 1).Source {
			t.Fatalf("кадр %d: серия оборвалась", i)
		}
		if p.Mirror != (p.Variant == 3 || p.Variant == 5) {
			t.Fatalf("кадр %d: зеркало %v у варианта %d", i, p.Mirror, p.Variant)
		}
		if !p.Taken.After(prev) {
			t.Fatalf("кадр %d: дата %v не позже предыдущей %v", i, p.Taken, prev)
		}
		prev = p.Taken
		if p.Bright < -bigMaxBright || p.Bright > bigMaxBright || p.CropX < 0 || p.CropX > 1 || p.CropY < 0 || p.CropY > 1 {
			t.Fatalf("кадр %d: яркость/рамка вне пределов %+v", i, p)
		}
		if p.Portrait != (aspects[p.Source-1] < 0.9) {
			t.Fatalf("кадр %d: Portrait=%v при пропорции исходника %.3f", i, p.Portrait, aspects[p.Source-1])
		}
		if px := p.W * p.H; px < 23_900_000 || px > 24_100_000 {
			t.Fatalf("кадр %d: %d×%d — не 24 Мп", i, p.W, p.H)
		}
		sq, adobe, rot = sq+btoi(p.Square), adobe+btoi(p.Adobe), rot+btoi(p.Rotate)
		portrait += btoi(p.Portrait && !p.Square)
		mirror += btoi(p.Mirror)
	}
	if sq != 30 || adobe != 19 || rot != 18 || mirror != 200 {
		t.Fatalf("1:1 %d (ждали 30), Adobe %d (19), поворот %d (18), зеркало %d (200)", sq, adobe, rot, mirror)
	}
	if portrait < bigCount/5 || portrait > bigCount*3/5 {
		t.Fatalf("вертикальных %d из %d", portrait, bigCount)
	}
	if first, last := BigSetPlan(1, 1).Taken, BigSetPlan(bigCount, 1).Taken; last.Sub(first) > 4*time.Hour || first.Hour() != 10 {
		t.Fatalf("даты от %v до %v", first, last)
	}
}

// Размеры по ориентации: горизонтальный 6000×4000, вертикальный 4000×6000, квадрат 4899²;
// при повороте EXIF пиксели в файле меняют стороны.
func TestBigSetPlanSizes(t *testing.T) {
	seen := map[[2]int]bool{}
	for i := 1; i <= bigCount; i++ {
		p := BigSetPlan(i, 1)
		seen[[2]int{p.W, p.H}] = true
		w, h := p.StoredSize()
		if p.Rotate && (w != p.H || h != p.W) || !p.Rotate && (w != p.W || h != p.H) {
			t.Fatalf("кадр %d: в файле %d×%d при кадре %d×%d, поворот %v", i, w, h, p.W, p.H, p.Rotate)
		}
	}
	for _, d := range [][2]int{{6000, 4000}, {4000, 6000}, {4899, 4899}} {
		if !seen[d] {
			t.Errorf("в наборе нет размера %v", d)
		}
	}
	if len(seen) != 3 {
		t.Errorf("размеров %d: %v", len(seen), seen)
	}
}

// bigSrc — исходник 120×80 с разными углами, чтобы зеркало и поворот были заметны.
func bigSrc() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for y := range 80 {
		for x := range 120 {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 2), uint8(y * 3), uint8((x + y) % 256), 255})
		}
	}
	return img
}

func zeroTile() []int8 { return make([]int8, bigTile*bigTile) }

// flat — план без случайностей: полный кадр, без зеркала, яркости и поворота.
func flat() BigShot {
	return BigShot{N: 1, Source: 1, W: 6000, H: 4000, Scale: 1, CropX: 0.5, CropY: 0.5}
}

func TestBigSetRenderSize(t *testing.T) {
	for _, c := range []struct {
		name         string
		w, h, wd, hd int
		rot          bool
	}{
		{"горизонтальный", 6000, 4000, 60, 40, false},
		{"вертикальный", 4000, 6000, 40, 60, false},
		{"квадрат", 4899, 4899, 48, 48, false},
		{"поворот", 6000, 4000, 40, 60, true},
		{"div больше кадра", 6000, 4000, 1, 1, false},
	} {
		p := flat()
		p.W, p.H, p.Rotate = c.w, c.h, c.rot
		div := 100
		if c.name == "div больше кадра" {
			div = 100000
		}
		if b := BigSetRender(bigSrc(), p, zeroTile(), div).Bounds(); b.Dx() != c.wd || b.Dy() != c.hd {
			t.Errorf("%s: %d×%d, ждали %d×%d", c.name, b.Dx(), b.Dy(), c.wd, c.hd)
		}
	}
}

// Зеркало — точное отражение по горизонтали.
func TestBigSetRenderMirror(t *testing.T) {
	p := flat()
	a := BigSetRender(bigSrc(), p, zeroTile(), 100)
	p.Mirror = true
	b := BigSetRender(bigSrc(), p, zeroTile(), 100)
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	if a.RGBAAt(0, 0) == a.RGBAAt(w-1, 0) {
		t.Fatal("исходник симметричен — тест ничего не проверяет")
	}
	for y := range h {
		for x := range w {
			if a.RGBAAt(x, y) != b.RGBAAt(w-1-x, y) {
				t.Fatalf("(%d,%d): %v != %v", x, y, a.RGBAAt(x, y), b.RGBAAt(w-1-x, y))
			}
		}
	}
}

// Поворот: пиксель (x, y) кадра лежит в файле в (y, w-1-x) — Orientation 6 вернёт его на место.
func TestBigSetRenderRotate(t *testing.T) {
	p := flat()
	a := BigSetRender(bigSrc(), p, zeroTile(), 100)
	p.Rotate = true
	b := BigSetRender(bigSrc(), p, zeroTile(), 100)
	w, h := a.Bounds().Dx(), a.Bounds().Dy()
	if b.Bounds().Dx() != h || b.Bounds().Dy() != w {
		t.Fatalf("файл %v при кадре %d×%d", b.Bounds(), w, h)
	}
	for y := range h {
		for x := range w {
			if a.RGBAAt(x, y) != b.RGBAAt(y, w-1-x) {
				t.Fatalf("(%d,%d) не на месте после поворота", x, y)
			}
		}
	}
}

// Яркость и шум: однотонный исходник даёт ровно ожидаемое значение; пределы зажимаются.
func TestBigSetRenderToneAndNoise(t *testing.T) {
	gray := func(v uint8) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 30, 20))
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 255
		}
		return img
	}
	noise := zeroTile()
	for i := range noise {
		noise[i] = 7
	}
	for _, c := range []struct {
		name   string
		v      uint8
		bright float64
		tile   []int8
		want   uint8
	}{
		{"без правок", 100, 0, zeroTile(), 100},
		{"светлее на 5 %", 100, 0.05, zeroTile(), 105},
		{"темнее на 6 %", 100, -0.06, zeroTile(), 94},
		{"шум +7", 100, 0, noise, 107},
		{"белое не выше 255", 255, 0.06, noise, 255},
		{"чёрное не ниже 0", 0, -0.06, func() []int8 { n := zeroTile(); n[0] = -10; return n }(), 0},
	} {
		p := flat()
		p.W, p.H, p.Bright = 300, 200, c.bright
		img := BigSetRender(gray(c.v), p, c.tile, 10)
		if got := img.RGBAAt(0, 0); got.R != c.want || got.G != c.want || got.B != c.want || got.A != 255 {
			t.Errorf("%s: %v, ждали %d", c.name, got, c.want)
		}
	}
}

// Рамка Scale < 1 берёт меньший кусок исходника: углы кадра — не углы исходника.
func TestBigSetRenderCropWindow(t *testing.T) {
	p := flat()
	full := BigSetRender(bigSrc(), p, zeroTile(), 100)
	p.Scale, p.CropX, p.CropY = 0.5, 0, 0
	part := BigSetRender(bigSrc(), p, zeroTile(), 100)
	if a, b := full.RGBAAt(0, 0), part.RGBAAt(0, 0); int(b.R)-int(a.R) > 2 || int(a.R)-int(b.R) > 2 {
		t.Fatalf("рамка у левого верхнего угла: (0,0) должен почти совпасть: %v и %v", a, b)
	}
	if full.RGBAAt(59, 39) == part.RGBAAt(59, 39) {
		t.Fatal("правый нижний угол уменьшенной рамки не изменился")
	}
}

// Плитка шума: размер, пределы ±10, зависит от зерна и не бывает однотонной.
func TestBigSetTile(t *testing.T) {
	a, b := BigSetTile(1), BigSetTile(2)
	if len(a) != bigTile*bigTile {
		t.Fatalf("длина %d", len(a))
	}
	same, distinct := 0, map[int8]bool{}
	for i, v := range a {
		if v < -10 || v > 10 {
			t.Fatalf("[%d] = %d", i, v)
		}
		distinct[v] = true
		same += btoi(v == b[i])
	}
	if len(distinct) < 15 || same > len(a)/4 {
		t.Fatalf("разных значений %d, совпало с другим зерном %d", len(distinct), same)
	}
}

// Сегменты — это APP1 с EXIF и (только у Adobe) APP2 с ICC, ровно в этом порядке.
func TestBigSetSegmentsShape(t *testing.T) {
	plain := BigSetSegments(BigSetPlan(1, 1))
	if len(plain) != 1 || plain[0][1] != 0xE1 {
		t.Fatalf("обычный кадр: %d сегментов", len(plain))
	}
	adobe := BigSetSegments(BigSetPlan(31, 1))
	if len(adobe) != 2 || adobe[0][1] != 0xE1 || adobe[1][1] != 0xE2 {
		t.Fatalf("Adobe: %d сегментов", len(adobe))
	}
	if got := BigSetSegments(BigShot{}); len(got) != 1 {
		t.Fatalf("пустой план: %d сегментов", len(got))
	}
}

// BigSetEncode: настоящий JPEG, а сегменты плана лежат сразу после SOI в порядке BigSetSegments;
// кадр читается обратно нужного размера.
func TestBigSetEncode(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for _, n := range []int{1, 31} { // обычный и Adobe (два сегмента)
		p := BigSetPlan(n, 1)
		data, err := BigSetEncode(img, p)
		if err != nil {
			t.Fatal(err)
		}
		var head []byte
		head = append(head, 0xFF, 0xD8)
		for _, s := range BigSetSegments(p) {
			head = append(head, s...)
		}
		if !bytes.HasPrefix(data, head) {
			t.Errorf("кадр %d: сегменты не сразу после SOI", n)
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != 24 || cfg.Height != 16 {
			t.Errorf("кадр %d: cfg=%v err=%v", n, cfg, err)
		}
	}
	// кадр шире предела JPEG кодировщик отвергает ошибкой, а не паникой
	if _, err := BigSetEncode(image.NewRGBA(image.Rect(0, 0, 1<<16, 1)), BigShot{}); err == nil {
		t.Error("слишком широкий кадр без ошибки")
	}
}
