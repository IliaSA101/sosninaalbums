package psd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

func info(t *testing.T, data []byte) Info {
	t.Helper()
	in, err := ReadInfo(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
	}
	return in
}

func TestReadInfo(t *testing.T) {
	g := []Guide{{Pos: 100}, {Pos: 250.5, Horz: true}, {Pos: 0}, {Pos: 400}, {Pos: 300, Horz: true}}
	in := info(t, mk(spec{w: 400, h: 300, guides: g, comp: 1}))
	if in.W != 400 || in.H != 300 || in.Channels != 3 || in.Depth != 8 || in.Mode != modeRGB {
		t.Fatalf("заголовок: %+v", in)
	}
	if in.DPI != 300 || !in.Merged || !in.Image || in.Note != "" {
		t.Fatalf("dpi/merged/image: %+v", in)
	}
	if len(in.Guides) != len(g) {
		t.Fatalf("направляющие: %+v", in.Guides)
	}
	for i := range g {
		if in.Guides[i] != g[i] {
			t.Fatalf("направляющая %d: %+v, ждали %+v", i, in.Guides[i], g[i])
		}
	}
}

func TestReadInfoDPI(t *testing.T) {
	for _, c := range []struct {
		name  string
		fixed uint32
		unit  uint16
		no    bool
		want  float64
	}{
		{"ppi 300", 300 << 16, 1, false, 300},
		{"ppi дробный", 72<<16 + 1<<15, 1, false, 72.5},
		{"единица ppcm — число всё равно ppi", uint32(math.Round(118.11 * 65536)), 2, false, 118.11},
		{"нет ресурса", 0, 0, true, 0},
		{"0 ppi", 1, 1, false, 0},                   // 1/65536 ppi — вне 1…10000
		{"слишком много", 20000 << 16, 1, false, 0}, // вне предела
		{"ровно 10000", 10000 << 16, 1, false, 10000},
		{"ровно 1", 1 << 16, 1, false, 1},
	} {
		in := info(t, mk(spec{w: 4, h: 4, dpiFixed: c.fixed, dpiUnit: c.unit, noDPI: c.no}))
		if math.Abs(in.DPI-c.want) > 0.01 {
			t.Errorf("%s: dpi = %v, ждали %v", c.name, in.DPI, c.want)
		}
	}
}

func TestReadInfoGuides(t *testing.T) {
	// Обе оси, вне документа отброшены (< 0, > размера по своей оси), на границе — остаются.
	g := []Guide{
		{Pos: -1}, {Pos: 0}, {Pos: 100}, {Pos: 100.5}, // вертикальные: ширина 100
		{Pos: 50, Horz: true}, {Pos: 60, Horz: true}, {Pos: 60.03125, Horz: true}, {Pos: -0.5, Horz: true}, // высота 60
	}
	in := info(t, mk(spec{w: 100, h: 60, guides: g}))
	want := []Guide{{Pos: 0}, {Pos: 100}, {Pos: 50, Horz: true}, {Pos: 60, Horz: true}}
	if len(in.Guides) != len(want) {
		t.Fatalf("%+v", in.Guides)
	}
	for i := range want {
		if in.Guides[i] != want[i] {
			t.Errorf("%d: %+v, ждали %+v", i, in.Guides[i], want[i])
		}
	}
	// Пустой список: ресурс есть, направляющих нет.
	if in := info(t, mk(spec{w: 10, h: 10, guides: []Guide{}})); len(in.Guides) != 0 {
		t.Fatalf("пустой ресурс: %+v", in.Guides)
	}
	// Ровно предел (1000) читается; 1001 — ресурс пропущен целиком, без ошибки.
	many := func(n int) []Guide {
		gs := make([]Guide, n)
		for i := range gs {
			gs[i] = Guide{Pos: float64(i % 50), Horz: i%2 == 1}
		}
		return gs
	}
	if in := info(t, mk(spec{w: 100, h: 100, guides: many(maxGuides)})); len(in.Guides) != maxGuides {
		t.Fatalf("1000 направляющих: прочитано %d", len(in.Guides))
	}
	if in := info(t, mk(spec{w: 100, h: 100, guides: many(maxGuides + 1)})); len(in.Guides) != 0 {
		t.Fatalf("1001 направляющая: прочитано %d, ждали 0", len(in.Guides))
	}
	// Число в блоке больше, чем влезает в размер ресурса: пропуск.
	bad := guideBlock([]Guide{{Pos: 5}})
	binary.BigEndian.PutUint32(bad[12:], 7)
	in = info(t, mk(spec{w: 100, h: 100, extra: []block{{id: 1032, data: bad}}}))
	if len(in.Guides) != 0 {
		t.Fatalf("врущее число: %+v", in.Guides)
	}
	// Неизвестное направление пропускается, остальные читаются.
	two := guideBlock([]Guide{{Pos: 5}, {Pos: 6}})
	two[16+4] = 7
	in = info(t, mk(spec{w: 100, h: 100, extra: []block{{id: 1032, data: two}}}))
	if len(in.Guides) != 1 || in.Guides[0].Pos != 6 {
		t.Fatalf("направление 7: %+v", in.Guides)
	}
	// Второй ресурс 1032 игнорируется.
	in = info(t, mk(spec{w: 100, h: 100, guides: []Guide{{Pos: 5}}, extra: []block{{id: 1032, data: guideBlock([]Guide{{Pos: 9}})}}}))
	if len(in.Guides) != 1 || in.Guides[0].Pos != 5 {
		t.Fatalf("дубль 1032: %+v", in.Guides)
	}
}

func TestReadInfoMerged(t *testing.T) {
	zero, one := byte(0), byte(1)
	in := info(t, mk(spec{w: 4, h: 4, merged: &zero}))
	if in.Merged || in.Image || in.Note != noteNoMerged {
		t.Fatalf("1057 = 0: %+v", in)
	}
	in = info(t, mk(spec{w: 4, h: 4, merged: &one}))
	if !in.Merged || !in.Image {
		t.Fatalf("1057 = 1: %+v", in)
	}
	in = info(t, mk(spec{w: 4, h: 4, noMerged: true}))
	if !in.Merged || !in.Image {
		t.Fatalf("нет 1057 — считаем, что сведённое есть: %+v", in)
	}
	// Короткий (меньше 5 байт) ресурс 1057 игнорируется.
	in = info(t, mk(spec{w: 4, h: 4, noMerged: true, extra: []block{{id: 1057, data: []byte{0, 0, 0}}}}))
	if !in.Merged {
		t.Fatalf("короткий 1057: %+v", in)
	}
}

// Прочие блоки пропускаются Seek (в том числе огромные), неизвестная подпись — конец ресурсов без ошибки,
// все пять известных подписей принимаются.
func TestReadInfoResourceBlocks(t *testing.T) {
	huge := block{id: 1060, data: make([]byte, 3<<20)} // «XMP»
	odd := block{id: 1036, name: "abc", data: []byte{1, 2, 3}}
	gs := []Guide{{Pos: 7}}
	in := info(t, mk(spec{w: 50, h: 50, guides: gs, extra: []block{huge, odd}}))
	if len(in.Guides) != 1 || in.DPI != 300 {
		t.Fatalf("после больших блоков: %+v", in)
	}
	for _, sig := range []string{"MeSa", "AgHg", "PHUT", "DCSR"} {
		in = info(t, mk(spec{w: 50, h: 50, extra: []block{{id: 1032, sig: sig, data: guideBlock(gs)}}}))
		if len(in.Guides) != 1 {
			t.Errorf("подпись %s: %+v", sig, in.Guides)
		}
	}
	// Мусорная подпись после DPI: разбор ресурсов останавливается, ошибки нет, 1057 уже не виден.
	zero := byte(0)
	in = info(t, mk(spec{w: 50, h: 50, merged: &zero, extra: []block{{id: 1, sig: "XXXX", data: []byte{0, 0}}}}))
	if in.DPI != 300 || !in.Merged {
		t.Fatalf("неизвестная подпись: %+v", in)
	}
}

// Больше блоков, чем предел: ресурсы читаются до предела, дальше — без ошибки.
func TestReadInfoBlockLimit(t *testing.T) {
	var ex []block
	for range maxBlocks + 10 {
		ex = append(ex, block{id: 2000, data: []byte{0, 0}})
	}
	ex = append(ex, block{id: 1032, data: guideBlock([]Guide{{Pos: 1}})})
	in := info(t, mk(spec{w: 10, h: 10, noDPI: true, extra: ex}))
	if len(in.Guides) != 0 || !in.Image {
		t.Fatalf("после предела блоков: %+v", in)
	}
}

func TestRejectPSB(t *testing.T) {
	data := mk(spec{w: 4, h: 4, version: 2})
	if _, err := ReadInfo(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrPSB) {
		t.Fatalf("ReadInfo: %v", err)
	}
	img, _, err := Preview(bytes.NewReader(data), int64(len(data)), 100)
	if !errors.Is(err, ErrPSB) || img != nil {
		t.Fatalf("Preview: %v", err)
	}
	if ErrPSB.Error() == "" || !bytes.Contains([]byte(ErrPSB.Error()), []byte("сохраните его как PSD")) {
		t.Fatalf("текст отказа: %q", ErrPSB)
	}
}

func TestNotPSD(t *testing.T) {
	good := mk(spec{w: 4, h: 4})
	ver3 := bytes.Clone(good)
	ver3[5] = 3
	badSig := bytes.Clone(good)
	badSig[0] = 'X'
	for name, data := range map[string][]byte{
		"пусто": nil, "3 байта": []byte("8BP"), "подпись": badSig, "версия 3": ver3,
		"JPEG": {0xff, 0xd8, 0xff, 0xe0, 0, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0},
	} {
		if _, err := ReadInfo(bytes.NewReader(data), int64(len(data))); !errors.Is(err, ErrNotPSD) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestLimits(t *testing.T) {
	cases := []struct {
		name string
		s    spec
		want error
	}{
		{"ширина 30000", spec{w: 30000, h: 8000}, nil},
		{"ширина 30001", spec{w: 30001, h: 10}, ErrSide},
		{"высота 30001", spec{w: 10, h: 30001}, ErrSide},
		{"250 Мп ровно", spec{w: 25000, h: 10000}, nil},
		{"250 Мп + 1 строка", spec{w: 25000, h: 10001}, ErrPixels},
		{"ширина 0", spec{w: 0, h: 10}, ErrCorrupt},
		{"высота 0", spec{w: 10, h: 0}, ErrCorrupt},
		{"каналов 56", spec{w: 4, h: 4, ch: 56}, nil},
		{"каналов 57", spec{w: 4, h: 4, ch: 57}, ErrCorrupt},
		{"режим 10", spec{w: 4, h: 4, mode: 10}, ErrCorrupt},
		{"глубина 5", spec{w: 4, h: 4, depth: 5}, ErrCorrupt},
		{"color data 1 МБ", spec{w: 4, h: 4, colorData: 1 << 20}, nil},
		{"color data 1 МБ + 1", spec{w: 4, h: 4, colorData: 1<<20 + 1}, ErrCorrupt},
	}
	for _, c := range cases {
		// Данные изображения для ReadInfo не нужны: гигантский размер в заголовке не строим целиком.
		s := c.s
		s.h, s.w = min(s.h, 40), min(s.w, 40)
		data := mk(s)
		patchSize(data, c.s.w, c.s.h)
		_, err := ReadInfo(bytes.NewReader(data), int64(len(data)))
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, ждали %v", c.name, err, c.want)
		}
	}
	// Файл больше 2 ГБ: отказ по size, данные не читаются.
	_, err := ReadInfo(bytes.NewReader(mk(spec{w: 4, h: 4})), 2<<30+1)
	if !errors.Is(err, ErrFileSize) {
		t.Fatalf("2 ГБ + 1: %v", err)
	}
	_, err = ReadInfo(bytes.NewReader(mk(spec{w: 4, h: 4})), 2<<30)
	if errors.Is(err, ErrFileSize) {
		t.Fatalf("ровно 2 ГБ не должен отказывать по размеру: %v", err)
	}
}

// patchSize подставляет в заголовок ширину и высоту (чтобы не строить гигантские данные).
func patchSize(data []byte, w, h int) {
	binary.BigEndian.PutUint32(data[14:], uint32(h))
	binary.BigEndian.PutUint32(data[18:], uint32(w))
}

// Длины из файла больше остатка файла: отказ до выделений.
func TestSectionLengthsBeyondFile(t *testing.T) {
	good, off := build(spec{w: 8, h: 8, guides: []Guide{{Pos: 3}}})
	// Смещения секций: 26 (заголовок), 4 (color data), 4 (длина ресурсов).
	resLen := int(binary.BigEndian.Uint32(good[30:]))
	layerAt := 34 + resLen
	for name, patch := range map[string]func([]byte){
		"color data длиннее файла": func(b []byte) { binary.BigEndian.PutUint32(b[26:], uint32(len(b))) },
		"ресурсы длиннее файла":    func(b []byte) { binary.BigEndian.PutUint32(b[30:], uint32(len(b))) },
		"слои длиннее файла":       func(b []byte) { binary.BigEndian.PutUint32(b[layerAt:], uint32(len(b))) },
		"слои 4 ГБ":                func(b []byte) { binary.BigEndian.PutUint32(b[layerAt:], math.MaxUint32) },
		"ресурсы 4 ГБ":             func(b []byte) { binary.BigEndian.PutUint32(b[30:], math.MaxUint32) },
		"блок длиннее секции": func(b []byte) {
			// Размер первого блока (1005): подпись 4 + id 2 + имя 2 → поле размера на смещении 34+8.
			binary.BigEndian.PutUint32(b[34+8:], math.MaxUint32)
		},
	} {
		b := bytes.Clone(good)
		patch(b)
		if _, err := ReadInfo(bytes.NewReader(b), int64(len(b))); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if off != layerAt+4+2 {
		t.Fatalf("смещения теста разошлись: %d / %d", off, layerAt+6)
	}
}

// Файл, оборванный в любом месте: ReadInfo не паникует и отказывает, пока не дочитан u16 сжатия;
// Preview отказывает при любом обрыве.
func TestTruncated(t *testing.T) {
	for _, comp := range []int{0, 1} {
		data, off := build(spec{w: 12, h: 9, comp: comp, guides: []Guide{{Pos: 2}, {Pos: 5, Horz: true}}, extra: []block{{id: 1036, data: []byte{1, 2, 3}}}})
		for cut := range len(data) {
			part := data[:cut]
			_, err := ReadInfo(bytes.NewReader(part), int64(cut))
			if (err == nil) != (cut >= off) {
				t.Fatalf("comp %d, обрыв %d (данные с %d): ReadInfo err = %v", comp, cut, off, err)
			}
			img, _, err := Preview(bytes.NewReader(part), int64(cut), 100)
			if err == nil || img != nil {
				t.Fatalf("comp %d, обрыв %d: Preview принял обрезанный файл", comp, cut)
			}
		}
	}
}

// size меньше реальных данных: читать за size нельзя.
func TestSizeSmallerThanData(t *testing.T) {
	data, off := build(spec{w: 6, h: 6})
	if _, err := ReadInfo(bytes.NewReader(data), int64(off-3)); err == nil {
		t.Fatal("size обрывает до сжатия, а ошибки нет")
	}
}

// Режимы, которые не показываем: ошибки нет, направляющие и причина есть.
func TestNoImageModes(t *testing.T) {
	gs := []Guide{{Pos: 2}, {Pos: 3, Horz: true}}
	for _, c := range []struct {
		name string
		s    spec
	}{
		{"Lab", spec{mode: modeLab}},
		{"32 бит", spec{depth: 32}},
		{"Bitmap", spec{bitmap: true, depth: 1, ch: 1}},
		{"Indexed", spec{mode: 2, ch: 1, colorData: 768}},
		{"Duotone", spec{mode: 8, ch: 1}},
		{"Multichannel", spec{mode: 7, ch: 4}},
		{"ZIP без предсказания", spec{comp: 2}},
		{"ZIP с предсказанием", spec{comp: 3}},
		{"сжатие 9", spec{comp: 9}},
		{"RGB, но 2 канала", spec{ch: 2}},
		{"CMYK, но 3 канала", spec{mode: modeCMYK, ch: 3}},
	} {
		s := c.s
		s.w, s.h, s.guides = 6, 5, gs
		data := mk(s)
		img, in, err := Preview(bytes.NewReader(data), int64(len(data)), 100)
		if err != nil || img != nil || in.Image || in.Note == "" {
			t.Errorf("%s: img=%v err=%v %+v", c.name, img != nil, err, in)
		}
		if len(in.Guides) != 2 {
			t.Errorf("%s: направляющие не прочитаны: %+v", c.name, in.Guides)
		}
	}
	zero := byte(0)
	data := mk(spec{w: 6, h: 5, merged: &zero, guides: gs})
	img, in, err := Preview(bytes.NewReader(data), int64(len(data)), 100)
	if err != nil || img != nil || in.Note != noteNoMerged || len(in.Guides) != 2 {
		t.Fatalf("без сведённого: %v %+v", err, in)
	}
}
