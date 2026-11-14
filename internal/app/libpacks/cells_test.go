package libpacks

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Клетки листа (спека 144 Г2, security 7–8): проверка вида JPEG до Decode, нарезка, LRU, слот.

// cellFx — стенд с листом в кэше и одним набором в списке.
func cellFx(t *testing.T, data []byte) (*prevFx, *fetch.Preview) {
	t.Helper()
	fx := newPrevFx(t, nil)
	p := previewOf("", data)
	fx.put(p, data)
	fx.list(fetch.Entry{ID: "pack-a", Preview: p})
	return fx, p
}

// near — цвет клетки близок к ожидаемому (JPEG q90 и цветовое подвыборочное).
func near(t *testing.T, cell []byte, w, h int, want color.RGBA) {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(cell))
	if err != nil {
		t.Fatalf("клетка не JPEG: %v", err)
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("размер клетки %v, ждали %dx%d", img.Bounds(), w, h)
	}
	r, g, b, _ := img.At(w/2, h/2).RGBA()
	d := func(a uint32, c uint8) int { return int(a>>8) - int(c) }
	if abs(d(r, want.R)) > 30 || abs(d(g, want.G)) > 30 || abs(d(b, want.B)) > 30 {
		t.Errorf("цвет клетки (%d,%d,%d), ждали %v", r>>8, g>>8, b>>8, want)
	}
}

func abs(x int) int { return max(x, -x) }

// Все клетки обоих рядов — своего размера и цвета; один разбор на лист.
func TestPreviewCells(t *testing.T) {
	fx, _ := cellFx(t, sheetJPEG(t, 1))
	for j := range 3 {
		c, err := fx.s.Cell(context.Background(), "pack-a", "bg", j)
		if err != nil {
			t.Fatal(err)
		}
		near(t, c, 64, 48, palette[j])
	}
	for j := range 4 {
		c, err := fx.s.Cell(context.Background(), "pack-a", "sticker", j)
		if err != nil {
			t.Fatal(err)
		}
		near(t, c, 32, 32, palette[3+j])
	}
	if n := fx.s.decodes.Load(); n != 1 {
		t.Errorf("разборов %d", n)
	}
	// Вне ряда, нет вида, нет набора, плохой id — ErrNoCell.
	for _, c := range []struct {
		pack, kind string
		i          int
	}{{"pack-a", "bg", 3}, {"pack-a", "bg", -1}, {"pack-a", "frame", 0}, {"pack-x", "bg", 0}, {"../x", "bg", 0}, {"", "bg", 0}} {
		if _, err := fx.s.Cell(context.Background(), c.pack, c.kind, c.i); err != ErrNoCell {
			t.Errorf("%+v: %v", c, err)
		}
	}
}

// 24 одновременных запроса одного листа — один разбор.
func TestPreviewCellsOneDecode(t *testing.T) {
	fx, _ := cellFx(t, sheetJPEG(t, 1))
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, n := "bg", 3
			if i%2 == 1 {
				row, n = "sticker", 4
			}
			if _, err := fx.s.Cell(context.Background(), "pack-a", row, i%n); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := fx.s.decodes.Load(); n != 1 {
		t.Fatalf("разборов %d, ждали 1", n)
	}
}

// Общий лист двух наборов: те же байты и раскладка — один разбор и одна запись в LRU.
func TestPreviewCellsSharedSheet(t *testing.T) {
	fx, p := cellFx(t, sheetJPEG(t, 1))
	fx.list(fetch.Entry{ID: "pack-a", Preview: p}, fetch.Entry{ID: "pack-b", Preview: p})
	a, err1 := fx.s.Cell(context.Background(), "pack-a", "bg", 0)
	b, err2 := fx.s.Cell(context.Background(), "pack-b", "bg", 0)
	if err1 != nil || err2 != nil || !bytes.Equal(a, b) || fx.s.decodes.Load() != 1 || len(fx.s.cells.items) != 1 {
		t.Fatalf("общий лист: %v %v, разборов %d, записей %d", err1, err2, fx.s.decodes.Load(), len(fx.s.cells.items))
	}
}

// Те же байты листа, другая раскладка (rows правлены в каталоге): другой ключ, другие клетки, без
// паники; индекс вне нового ряда — ErrNoCell.
func TestPreviewCellsRowsChanged(t *testing.T) {
	fx, p := cellFx(t, sheetJPEG(t, 1))
	if c, err := fx.s.Cell(context.Background(), "pack-a", "bg", 2); err != nil {
		t.Fatal(err)
	} else {
		near(t, c, 64, 48, palette[2])
	}
	q := *p
	q.Rows = []fetch.PreviewRow{{Kind: "bg", W: 32, H: 32, Items: []string{p.Rows[0].Items[0], p.Rows[0].Items[1]}}}
	fx.list(fetch.Entry{ID: "pack-a", Preview: &q})
	c, err := fx.s.Cell(context.Background(), "pack-a", "bg", 1)
	if err != nil {
		t.Fatal(err)
	}
	near(t, c, 32, 32, palette[0]) // клетка 1 ряда 32×32 лежит ещё в первой клетке 64×48
	if _, err := fx.s.Cell(context.Background(), "pack-a", "bg", 2); err != ErrNoCell {
		t.Errorf("вне нового ряда: %v", err)
	}
	if len(fx.s.cells.items) != 2 || fx.s.decodes.Load() != 2 {
		t.Errorf("записей %d, разборов %d", len(fx.s.cells.items), fx.s.decodes.Load())
	}
	// Индекс сверяется с длиной закэшированного ряда: запись старой раскладки не отдаёт чужое.
	if _, ok := fx.s.cells.get(layoutKey(p), 0, 99); ok {
		t.Error("индекс вне ряда отдан из LRU")
	}
}

// LRU не растёт выше бюджета и вытесняет самую давнюю по обращению, а не по записи (не FIFO).
func TestPreviewCellsLRU(t *testing.T) {
	c := newCellCache()
	big := bytes.Repeat([]byte{1}, 2<<20)
	put := func(k string) { c.put(&cellEntry{key: k, rows: [][][]byte{{big}}, size: len(big)}) }
	for _, k := range []string{"a", "b", "c", "d"} {
		put(k)
	}
	if _, ok := c.get("a", 0, 0); !ok { // «a» свежее «b», «c», «d»
		t.Fatal("a нет сразу после записи")
	}
	put("e") // не лезет в бюджет: уходит самая давняя по обращению — «b»
	has := func(k string) bool { _, ok := c.items[k]; return ok }
	if !has("a") || has("b") || !has("c") || !has("d") || !has("e") {
		t.Fatalf("после e: a=%v b=%v c=%v d=%v e=%v (FIFO вытеснил бы a)", has("a"), has("b"), has("c"), has("d"), has("e"))
	}
	put("f") // теперь давнейшая — «c»
	if has("c") || !has("a") || !has("f") || c.bytes > cellBudget || len(c.items) != 4 {
		t.Fatalf("после f: c=%v a=%v f=%v, байт %d, записей %d", has("c"), has("a"), has("f"), c.bytes, len(c.items))
	}
	put("f") // повтор ключа не удваивает счёт
	if c.bytes != 8<<20 {
		t.Errorf("повтор ключа: %d", c.bytes)
	}
}

// Запрос ушёл (ctx отменён), пока слот занят: ждать не должен, разбора нет.
func TestPreviewCellsCtxCancel(t *testing.T) {
	fx, _ := cellFx(t, sheetJPEG(t, 1))
	fx.s.cells.slot <- struct{}{} // слот занят чужим разбором
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := fx.s.Cell(ctx, "pack-a", "bg", 0); done <- err }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("ошибка %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("запрос ждёт слот после отмены")
	}
	<-fx.s.cells.slot
	if fx.s.decodes.Load() != 0 {
		t.Error("разбор шёл")
	}
}

// refused проверяет отказ листа: ErrNoCell, Decode не вызван, лист в плохих (если badMark).
func refused(t *testing.T, name string, data []byte, mut func(p *fetch.Preview)) {
	t.Helper()
	fx := newPrevFx(t, nil)
	p := previewOf("", data)
	if mut != nil {
		mut(p)
	}
	fx.put(p, data)
	fx.list(fetch.Entry{ID: "pack-a", Preview: p})
	if _, err := fx.s.Cell(context.Background(), "pack-a", "bg", 0); err != ErrNoCell {
		t.Errorf("%s: %v", name, err)
	}
	if fx.s.decodes.Load() != 0 {
		t.Errorf("%s: Decode вызван", name)
	}
	if !fx.s.sheetBad(p) {
		t.Errorf("%s: лист не в плохих", name)
	}
	if _, err := fx.s.Cell(context.Background(), "pack-a", "bg", 0); err != ErrNoCell {
		t.Errorf("%s: повтор: %v", name, err)
	}
}

// Размер листа в файле не равен заявленному в каталоге — отказ до Decode.
func TestPreviewDecodeDims(t *testing.T) {
	refused(t, "ширина", sheetJPEG(t, 1), func(p *fetch.Preview) { p.W++ })
	refused(t, "высота", sheetJPEG(t, 1), func(p *fetch.Preview) { p.H-- })
}

// Не JPEG: PNG, мусор, пустые клетки под верным sha — ErrNoCell, без паники.
func TestPreviewNotJPEG(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, fxW, fxH)))
	refused(t, "png", buf.Bytes(), nil)
	refused(t, "мусор", bytes.Repeat([]byte{0xAB}, 500), nil)
	refused(t, "пустой SOI", []byte{0xFF, 0xD8}, nil)
	refused(t, "один байт", []byte{0xFF}, nil)
}

// patchSOF — копия data, в которой маркер кадра SOF0 заменён на m.
func patchSOF(t *testing.T, data []byte, m byte) []byte {
	t.Helper()
	i := bytes.Index(data, []byte{0xFF, 0xC0})
	if i < 0 {
		t.Fatal("нет SOF0")
	}
	out := bytes.Clone(data)
	out[i+1] = m
	return out
}

// Прогрессивный лист с верным sha: DecodeConfig его видит, но Decode не вызывается.
func TestPreviewProgressiveRefused(t *testing.T) {
	refused(t, "SOF2", patchSOF(t, sheetJPEG(t, 1), 0xC2), nil)
	refused(t, "SOF9 арифметический", patchSOF(t, sheetJPEG(t, 1), 0xC9), nil)
}

// CMYK (4 компоненты) с верным sha — отказ до Decode.
func TestPreviewCMYKRefused(t *testing.T) {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xD8, 0xFF, 0xC0, 0, 20, 8, 0, fxH, 0, fxW, 4})
	for i := range 4 {
		b.Write([]byte{byte(i + 1), 0x11, 0})
	}
	b.Write([]byte{0xFF, 0xDA, 0, 12, 4, 1, 0, 2, 0, 3, 0, 4, 0, 0, 63, 0, 0xFF, 0xD9})
	refused(t, "cmyk", b.Bytes(), nil)
}

// Огромный заявленный размер в заголовке (65535×65535) — отказ без выделения памяти.
func TestPreviewHugeDimsRefused(t *testing.T) {
	data := sheetJPEG(t, 1)
	i := bytes.Index(data, []byte{0xFF, 0xC0})
	huge := bytes.Clone(data)
	binary.BigEndian.PutUint16(huge[i+5:], 65535)
	binary.BigEndian.PutUint16(huge[i+7:], 65535)
	refused(t, "65535", huge, func(p *fetch.Preview) { p.W, p.H = 65535, 65535 })
}

// Усечённый лист (заголовок цел, данные оборваны): Decode вызван и вернул ошибку, не панику.
func TestPreviewTruncated(t *testing.T) {
	data := sheetJPEG(t, 1)
	fx, p := cellFx(t, data[:len(data)/2])
	if _, err := fx.s.Cell(context.Background(), "pack-a", "bg", 0); err != ErrNoCell {
		t.Fatalf("усечённый: %v", err)
	}
	if !fx.s.sheetBad(p) {
		t.Error("усечённый лист не в плохих")
	}
}

// Маркеров больше предела до SOS и длина сегмента за телом — false без паники.
func TestBaselineFrameBounds(t *testing.T) {
	long := []byte{0xFF, 0xD8}
	for range maxMarkers + 1 {
		long = append(long, 0xFF, 0xE1, 0, 2)
	}
	cases := map[string][]byte{
		"много маркеров": long,
		"длина за телом": {0xFF, 0xD8, 0xFF, 0xE0, 0xFF, 0xFF, 1},
		"длина 1":        {0xFF, 0xD8, 0xFF, 0xE0, 0, 1},
		"EOI до SOS":     {0xFF, 0xD8, 0xFF, 0xD9},
		"SOS без кадра":  {0xFF, 0xD8, 0xFF, 0xDA, 0, 2},
		"пусто":          nil,
		// Ревью 144 Г2: FF00 — не сегмент с длиной (image/jpeg пропускает его и байты длины), и под
		// «длиной» FF00 прячется настоящий SOF2; видимые SOF0+SOS после неё — приманка.
		"FF00 прячет SOF2": {
			0xFF, 0xD8,
			0xFF, 0x00, 0x00, 0x08, // «сегмент» длиной 8: два байта длины и шесть следующих
			0xFF, 0xC2, 0x00, 0x04, 0, 0, // настоящий прогрессивный кадр
			0xFF, 0xC0, 0x00, 0x04, 0, 0, // приманка SOF0
			0xFF, 0xDA, 0x00, 0x02, // SOS
		},
		"FF01 до SOS":  {0xFF, 0xD8, 0xFF, 0x01, 0xFF, 0xC0, 0x00, 0x04, 0, 0, 0xFF, 0xDA, 0x00, 0x02},
		"RST0 до SOS":  {0xFF, 0xD8, 0xFF, 0xD0, 0xFF, 0xC0, 0x00, 0x04, 0, 0, 0xFF, 0xDA, 0x00, 0x02},
		"второй SOI":   {0xFF, 0xD8, 0xFF, 0xD8, 0xFF, 0xC0, 0x00, 0x04, 0, 0, 0xFF, 0xDA, 0x00, 0x02},
		"FF00 в конце": {0xFF, 0xD8, 0xFF, 0x00},
	}
	for name, d := range cases {
		if baselineFrame(d) {
			t.Errorf("%s принят", name)
		}
	}
	if !baselineFrame(sheetJPEG(t, 1)) {
		t.Error("годный лист отвергнут")
	}
}

// Клетки листа больше 2 МБ — лист отвергается (шум не жмётся), не кэшируется.
func TestPreviewCellsTooBig(t *testing.T) {
	const cell, n = 480, 12
	img := image.NewRGBA(image.Rect(0, 0, cell*n, cell))
	rnd := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = byte(rnd.UintN(256))
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	items := make([]string, n)
	for i := range items {
		items[i] = "d00000000000000" + string(rune('0'+i))
	}
	p := &fetch.Preview{W: cell * n, H: cell, Rows: []fetch.PreviewRow{{Kind: "bg", W: cell, H: cell, Items: items}}}
	s := New(testHost(t, &evs{}, nil))
	if _, err := s.splitSheet(buf.Bytes(), p); err == nil {
		t.Fatal("лист с клетками больше 2 МБ принят")
	}
}

// seg — сегмент маркера m с телом n байт (длина включает свои 2 байта).
func seg(m byte, n int) []byte {
	return append([]byte{0xFF, m, byte((n + 2) >> 8), byte(n + 2)}, make([]byte, n)...)
}

// Граница счёта сегментов: до SOS включительно 64 проходит, 65 — отказ.
func TestBaselineFrameSegmentLimit(t *testing.T) {
	build := func(apps int) []byte {
		d := []byte{0xFF, 0xD8}
		for range apps {
			d = append(d, seg(0xE1, 0)...)
		}
		d = append(d, seg(0xC0, 15)...) // SOF0
		return append(d, seg(0xDA, 4)...)
	}
	if !baselineFrame(build(maxMarkers - 2)) { // 62 + SOF + SOS = 64
		t.Error("64 сегмента отвергнуты")
	}
	if baselineFrame(build(maxMarkers - 1)) { // 65
		t.Error("65 сегментов приняты")
	}
}

// Лист из трёх компонент, который Go считает RGB (Adobe transform 0 или id R,G,B без JFIF), —
// отказ до Decode: принимаются только YCbCr и Gray.
func TestPreviewRGBRefused(t *testing.T) {
	sof := func(ids [3]byte) []byte {
		b := []byte{0xFF, 0xC0, 0, 17, 8, 0, fxH, 0, fxW, 3}
		for _, id := range ids {
			b = append(b, id, 0x11, 0)
		}
		return b
	}
	adobe := []byte{0xFF, 0xEE, 0, 14, 'A', 'd', 'o', 'b', 'e', 0, 100, 0, 0, 0, 0, 0}
	sos := []byte{0xFF, 0xDA, 0, 12, 3, 1, 0, 2, 0, 3, 0, 0, 63, 0, 0xFF, 0xD9}
	join := func(parts ...[]byte) []byte { return bytes.Join(parts, nil) }
	soi := []byte{0xFF, 0xD8}
	for name, d := range map[string][]byte{
		"adobe transform 0": join(soi, adobe, sof([3]byte{1, 2, 3}), sos),
		"id RGB":            join(soi, sof([3]byte{'R', 'G', 'B'}), sos),
	} {
		if cfg, err := jpeg.DecodeConfig(bytes.NewReader(d)); err != nil || cfg.ColorModel != color.RGBAModel {
			t.Fatalf("%s: Go видит %v, %v — тест не про RGB", name, cfg.ColorModel, err)
		}
		refused(t, name, d, nil)
	}
	// Контроль: те же три компоненты с id 1,2,3 без Adobe — YCbCr, ColorModel принят (дальше отказ Decode).
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(join(soi, sof([3]byte{1, 2, 3}), sos)))
	if err != nil || cfg.ColorModel != color.YCbCrModel {
		t.Errorf("контроль: %v %v", cfg.ColorModel, err)
	}
}

// Фаззинг входа разбора листа: любые байты — ошибка или клетки, не паника и не зависание.
func FuzzSplitSheet(f *testing.F) {
	var seed bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, fxW, fxH))
	_ = jpeg.Encode(&seed, img, nil)
	f.Add(seed.Bytes())
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xC0, 0, 8, 8, 0, 1, 0, 1, 1})
	f.Add([]byte{})
	s := New(testHost(f, &evs{}, nil))
	p := previewOf("", seed.Bytes())
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > fetch.MaxPreview {
			t.Skip()
		}
		_, _ = s.splitSheet(data, p)
	})
}

// Плохой лист живёт до нового списка: тот же seq bad не сбрасывает, новый — сбрасывает, и разбор
// идёт снова один раз (строка журнала с id набора — по одной на разбор).
func TestPreviewBadSheetUntilNewSeq(t *testing.T) {
	buf := logTo(t)
	fx := newPrevFx(t, nil)
	data := sheetJPEG(t, 1)
	p := previewOf("", data)
	p.W++ // раскладка не сходится с листом: разбор отвергнет его
	fx.put(p, data)
	rows := []fetch.Entry{{ID: "pack-a", Preview: p}}
	fx.list(rows...)
	count := func() int { return strings.Count(buf.String(), "лист набора pack-a: ") }
	for range 3 {
		if _, err := fx.s.Cell(context.Background(), "pack-a", "bg", 0); err != ErrNoCell {
			t.Fatalf("плохой лист: %v", err)
		}
	}
	if !fx.s.sheetBad(p) || count() != 1 {
		t.Fatalf("после трёх запросов: bad=%v, строк журнала %d (%q)", fx.s.sheetBad(p), count(), buf.String())
	}
	if !strings.Contains(buf.String(), "библиотека: образцы — лист набора pack-a: ") || !strings.Contains(buf.String(), "размеры не те") {
		t.Errorf("журнал без id или причины: %q", buf.String())
	}
	fx.s.remember(fetch.Catalog{Seq: fx.seq, Packs: rows}) // тот же seq
	if !fx.s.sheetBad(p) {
		t.Fatal("тот же seq сбросил плохие листы")
	}
	fx.s.remember(fetch.Catalog{Seq: fx.seq - 1, Packs: rows}) // меньший отвергнут целиком
	if !fx.s.sheetBad(p) {
		t.Fatal("меньший seq сбросил плохие листы")
	}
	fx.s.remember(fetch.Catalog{Seq: fx.seq + 1, Packs: rows})
	if fx.s.sheetBad(p) {
		t.Fatal("новый seq не сбросил плохие листы")
	}
	for range 2 {
		_, _ = fx.s.Cell(context.Background(), "pack-a", "bg", 0)
	}
	if count() != 2 {
		t.Errorf("после нового seq разборов (строк журнала) %d, нужно 2", count())
	}
}
