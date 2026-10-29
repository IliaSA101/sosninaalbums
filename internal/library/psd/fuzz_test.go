package psd

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

// bigRLESeeds — затравки с длинными повторами PackBits (k>1, вплоть до 128): обычные затравки
// дают x*3 — повторов нет, и мутации не добирались до ветки повтора. Строки шире 128 байт —
// повторы режутся на куски 128 + остаток, соседние куски литерал/повтор.
func bigRLESeeds() [][]byte {
	half := byte(0x7f)
	return [][]byte{
		mk(spec{w: 600, h: 8, comp: 1, val: func(c, x, y int) byte { return 200 }}),
		mk(spec{w: 700, h: 6, comp: 1, val: func(c, x, y int) byte {
			if x%97 < 3 { // литерал между длинными повторами
				return byte(x + c)
			}
			return byte((x/97 + c) * 40)
		}}),
		mk(spec{w: 300, h: 5, depth: 16, comp: 1, val: func(c, x, y int) byte { return half }}),
		mk(spec{w: 260, h: 4, ch: 5, mode: modeCMYK, comp: 1, val: func(c, x, y int) byte { return 255 }}),
	}
}

// corruptRLE — первая строка первого канала: заголовок повтора заменён на «128 байт» (-127), а
// строка короче; таблица длин и остальные строки целы.
func corruptRLE(w, h, ch int) []byte {
	d, off := build(spec{w: w, h: h, ch: ch, comp: 1, val: func(c, x, y int) byte { return 9 }})
	d = bytes.Clone(d)
	d[off+2*ch*h] = 0x81 // -127 → 128 повторов, а строка в w < 128 байт
	return d
}

// fuzzSeeds — затравки: все режимы, оба сжатия, направляющие, пустые и больные заголовки.
func fuzzSeeds() [][]byte {
	zero := byte(0)
	seeds := [][]byte{
		mk(spec{w: 40, h: 30, comp: 1, guides: []Guide{{Pos: 1}, {Pos: 7, Horz: true}}}),
		mk(spec{w: 17, h: 9, ch: 1, depth: 16, mode: modeGray}),
		mk(spec{w: 8, h: 8, ch: 5, mode: modeCMYK, comp: 1, guides: []Guide{{Pos: 1, Horz: true}}}),
		mk(spec{w: 6, h: 6, depth: 16, comp: 1}),
		mk(spec{w: 6, h: 6, mode: modeLab, comp: 2}),
		mk(spec{w: 6, h: 6, merged: &zero, extra: []block{{id: 1060, data: []byte("<x/>")}}}),
		mk(spec{w: 6, h: 6, version: 2}),
		[]byte("8BPS\x00\x01"),
		nil,
	}
	return append(seeds, append(bigRLESeeds(), corruptRLE(100, 4, 3), corruptRLE(1, 1, 1))...)
}

// checkInfo — инварианты разобранного Info на любом входе.
func checkInfo(t *testing.T, in Info) {
	t.Helper()
	if in.W < 1 || in.H < 1 || in.W > maxSidePx || in.H > maxSidePx || int64(in.W)*int64(in.H) > maxPixels {
		t.Fatalf("размер вне пределов: %+v", in)
	}
	if in.Channels < 1 || in.Channels > maxChannels || len(in.Guides) > maxGuides {
		t.Fatalf("каналы/направляющие вне пределов: %+v", in)
	}
	if in.DPI != 0 && (in.DPI < 1 || in.DPI > 10000) {
		t.Fatalf("dpi вне 1…10000: %v", in.DPI)
	}
	for _, g := range in.Guides {
		lim := in.W
		if g.Horz {
			lim = in.H
		}
		if g.Pos < 0 || g.Pos > float64(lim) {
			t.Fatalf("направляющая вне документа: %+v", g)
		}
	}
	if in.Image == (in.Note != "") {
		t.Fatalf("Image и Note согласованы: %+v", in)
	}
}

// decodeBoth гоняет оба входа на одних байтах; Info из них должен совпасть.
func decodeBoth(t *testing.T, data []byte) {
	t.Helper()
	size := int64(len(data))
	in, err := ReadInfo(bytes.NewReader(data), size)
	if err == nil {
		checkInfo(t, in)
	}
	img, in2, err2 := Preview(bytes.NewReader(data), size, 64)
	if err != nil {
		if err2 == nil {
			t.Fatalf("ReadInfo отказал (%v), а Preview принял", err)
		}
		return
	}
	if err2 != nil {
		if img != nil {
			t.Fatal("ошибка вместе с картинкой")
		}
		return
	}
	if (img != nil) != in2.Image || len(in2.Guides) != len(in.Guides) || in2.W != in.W || in2.H != in.H {
		t.Fatalf("Info расходится: %+v / %+v", in, in2)
	}
	if img != nil {
		b := img.Bounds()
		if b.Dx() < 1 || b.Dy() < 1 || b.Dx() > 64 || b.Dy() > 64 || len(img.Pix) != 4*b.Dx()*b.Dy() {
			t.Fatalf("превью %v", b)
		}
	}
}

// FuzzDecode: что бы ни пришло — без паники и зависаний, пределы и согласованность Info/Preview.
func FuzzDecode(f *testing.F) {
	for _, s := range fuzzSeeds() {
		f.Add(s)
	}
	f.Fuzz(decodeBoth)
}

// Быстрый детерминированный аналог fuzz для обычного прогона: случайные порчи затравок
// (байт, обрыв, вставка) с фиксированным зерном.
func TestMutatedSeeds(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for _, seed := range fuzzSeeds() {
		for range 300 {
			d := bytes.Clone(seed)
			for range 1 + rng.IntN(4) {
				if len(d) == 0 {
					break
				}
				switch rng.IntN(3) {
				case 0:
					d[rng.IntN(len(d))] = byte(rng.IntN(256))
				case 1:
					d = d[:rng.IntN(len(d)+1)]
				default:
					i := rng.IntN(len(d))
					d = append(d[:i], append([]byte{byte(rng.IntN(256))}, d[i:]...)...)
				}
			}
			decodeBoth(t, d)
		}
	}
}

// Длинные повторы PackBits распаковываются целиком; строка короче заявленного повтора и усечение
// данных — без паники и без картинки.
func TestBigRLE(t *testing.T) {
	for i, seed := range bigRLESeeds() {
		img, in := preview(t, seed, 64)
		if img == nil || !in.Image {
			t.Errorf("затравка %d: нет превью", i)
		}
		// Обрезка: ошибка или заметка без картинки, но не картинка из обрывка.
		cut := len(seed) / 2
		if img, in, err := Preview(bytes.NewReader(seed[:cut]), int64(cut), 64); err == nil && (img != nil || in.Image) {
			t.Errorf("затравка %d, обрезка до %d: картинка из обрывка", i, cut)
		}
	}
	solid, _ := preview(t, bigRLESeeds()[0], 64)
	if px := solid.Pix[:3]; px[0] != 200 || px[1] != 200 || px[2] != 200 {
		t.Errorf("сплошной цвет искажён: %v", px)
	}
	for _, bad := range [][]byte{corruptRLE(100, 4, 3), corruptRLE(1, 1, 1)} {
		if img, in, err := Preview(bytes.NewReader(bad), int64(len(bad)), 64); img != nil || in.Image && err == nil {
			t.Errorf("повтор за краем строки принят: err=%v", err)
		}
	}
}
