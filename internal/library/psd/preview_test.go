package psd

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func preview(t *testing.T, data []byte, maxSide int) (*image.NRGBA, Info) {
	t.Helper()
	img, in, err := Preview(bytes.NewReader(data), int64(len(data)), maxSide)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	return img, in
}

// blockVal — значение канала c в блоке 2×2 (bx, by): среднее по блоку равно base, пиксели
// отличаются на ±1, чтобы проверить именно усреднение.
func blockVal(c, bx, by int) int { return 20 + c*30 + bx*40 + by*60 }

func blockSpec(mode, ch, depth, comp int) spec {
	return spec{w: 4, h: 4, mode: mode, ch: ch, depth: depth, comp: comp,
		val: func(c, x, y int) byte {
			d := -1
			if (x+y)%2 == 1 {
				d = 1
			}
			return byte(blockVal(c, x/2, y/2) + d)
		}}
}

// wantBlock — ожидаемый цвет блока (bx, by) для режима: Gray → R=G=B; CMYK — C'M'Y' × K' / 255.
func wantBlock(mode, bx, by int) [3]int {
	var want [3]int
	switch mode {
	case modeGray:
		v := blockVal(0, bx, by)
		want = [3]int{v, v, v}
	case modeRGB:
		want = [3]int{blockVal(0, bx, by), blockVal(1, bx, by), blockVal(2, bx, by)}
	case modeCMYK:
		k := blockVal(3, bx, by)
		for j := range 3 {
			want[j] = blockVal(j, bx, by) * k / 255
		}
	}
	return want
}

// TestPreviewModes: RGB/Gray/CMYK × 8/16 бит × raw/RLE; цвет блока 2×2 — среднее, лишние каналы не читаются.
func TestPreviewModes(t *testing.T) {
	for _, mode := range []int{modeGray, modeRGB, modeCMYK} {
		for _, depth := range []int{8, 16} {
			for _, comp := range []int{0, 1} {
				for _, extra := range []int{0, 2} { // альфа и плашки после нужных каналов
					name := fmt.Sprintf("mode%d/%dбит/comp%d/+%dканала", mode, depth, comp, extra)
					t.Run(name, func(t *testing.T) { checkBlocks(t, mode, needChannels(mode)+extra, depth, comp) })
				}
			}
		}
	}
}

func checkBlocks(t *testing.T, mode, ch, depth, comp int) {
	img, in := preview(t, mk(blockSpec(mode, ch, depth, comp)), 2)
	if !in.Image || img == nil || img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("%+v %v", in, img)
	}
	for by := range 2 {
		for bx := range 2 {
			px := img.NRGBAAt(bx, by)
			got, want := [3]int{int(px.R), int(px.G), int(px.B)}, wantBlock(mode, bx, by)
			if got != want || px.A != 255 {
				t.Errorf("(%d,%d): %v A=%d, ждали %v", bx, by, got, px.A, want)
			}
		}
	}
}

// k = 1: картинка берётся как есть (без смешивания), для raw и RLE одинаково.
func TestPreviewIdentity(t *testing.T) {
	val := func(c, x, y int) byte { return byte(c*70 + x*13 + y*5) }
	for _, comp := range []int{0, 1} {
		img, _ := preview(t, mk(spec{w: 7, h: 5, comp: comp, val: val}), 4096)
		if img.Bounds() != image.Rect(0, 0, 7, 5) {
			t.Fatalf("размер %v", img.Bounds())
		}
		for y := range 5 {
			for x := range 7 {
				p := img.NRGBAAt(x, y)
				if p.R != val(0, x, y) || p.G != val(1, x, y) || p.B != val(2, x, y) || p.A != 255 {
					t.Fatalf("comp %d (%d,%d): %+v", comp, x, y, p)
				}
			}
		}
	}
}

// refAvg — прямой подсчёт среднего (с округлением) блока (px, py) канала c при шаге k.
func refAvg(val func(c, x, y int) byte, c, px, py, k, w, h int) int {
	sum, n := 0, 0
	for y := py * k; y < min(py*k+k, h); y++ {
		for x := px * k; x < min(px*k+k, w); x++ {
			sum += int(val(c, x, y))
			n++
		}
	}
	return (sum + n/2) / n
}

// Края при k > 1: делитель — фактическое число пикселей блока. Сверка с прямым подсчётом.
func TestPreviewEdgeBlocks(t *testing.T) {
	val := func(c, x, y int) byte { return byte(3 + c*40 + x*9 + y*11) }
	for _, sz := range [][3]int{{10, 7, 4}, {1, 1, 1}, {1, 9, 3}, {9, 1, 3}, {5, 5, 2}, {13, 13, 4}, {100, 3, 7}} {
		w, h, maxSide := sz[0], sz[1], sz[2]
		k := (max(w, h) + maxSide - 1) / maxSide
		pw, ph := (w+k-1)/k, (h+k-1)/k
		for _, comp := range []int{0, 1} {
			img, _ := preview(t, mk(spec{w: w, h: h, comp: comp, val: val}), maxSide)
			if img.Bounds() != image.Rect(0, 0, pw, ph) || max(pw, ph) > maxSide {
				t.Fatalf("%v: размер %v (k=%d)", sz, img.Bounds(), k)
			}
			for py := range ph {
				for px := range pw {
					for c := range 3 {
						if got, want := int(img.Pix[py*img.Stride+4*px+c]), refAvg(val, c, px, py, k, w, h); got != want {
							t.Fatalf("%v comp %d (%d,%d) c%d: %d, ждали %d", sz, comp, px, py, c, got, want)
						}
					}
				}
			}
		}
	}
}

func TestPreviewBadMaxSide(t *testing.T) {
	data := mk(spec{w: 4, h: 4})
	for _, m := range []int{0, -5} {
		if _, _, err := Preview(bytes.NewReader(data), int64(len(data)), m); err == nil {
			t.Errorf("maxSide %d принят", m)
		}
	}
}

// Битые данные изображения: длина строки RLE больше предела, сумма длин больше файла,
// короткий raw, повреждённый PackBits.
func TestPreviewRejects(t *testing.T) {
	const w, h = 40, 10
	rle, off := build(spec{w: w, h: h, comp: 1})
	rowMax := w + (w+127)/128

	tooLong := bytes.Clone(rle)
	binary.BigEndian.PutUint16(tooLong[off+2*3:], uint16(rowMax+1)) // строка 3 канала 0
	// Длина строки в таблице не совпадает с настоящей: короче (литерал за край) и длиннее (чужой байт в хвосте).
	shorter := bytes.Clone(rle)
	binary.BigEndian.PutUint16(shorter[off:], binary.BigEndian.Uint16(rle[off:])-1)
	flat, offFlat := build(spec{w: w, h: h, comp: 1, val: func(c, x, y int) byte { return 9 }})
	// Ровно минимум PackBits (повтор: 2 байта на строку из 40) — граница проверки снизу, принят.
	if n := binary.BigEndian.Uint16(flat[offFlat:]); n != 2 {
		t.Fatalf("строка flat: %d байт, ждали минимум 2", n)
	}
	if img, _, err := Preview(bytes.NewReader(flat), int64(len(flat)), 100); err != nil || img == nil {
		t.Fatalf("минимум PackBits отвергнут: %v", err)
	}
	longer := bytes.Clone(flat)
	binary.BigEndian.PutUint16(longer[offFlat:], binary.BigEndian.Uint16(flat[offFlat:])+1)

	rle4, off4 := build(spec{w: w, h: h, comp: 1, ch: 4}) // лишний канал: длина сверх предела тоже отказ
	hugeExtra := bytes.Clone(rle4)
	binary.BigEndian.PutUint16(hugeExtra[off4+2*(3*h+1):], uint16(rowMax+5))

	badPack := bytes.Clone(rle)
	badPack[off+2*3*h] = 0x7f // первый заголовок PackBits: литерал на 128 байт в строке из 40

	for name, data := range map[string][]byte{
		"длина строки > предела":   tooLong,
		"длина строки у лишнего":   hugeExtra,
		"длина строки короче":      shorter,
		"длина строки длиннее":     longer,
		"PackBits за край":         badPack,
		"raw короче":               mk(spec{w: w, h: h})[:off+w*h*3-1],
		"RLE без хвоста":           rle[:len(rle)-1],
		"только таблица":           rle[:off+2*3*h],
		"таблица обрезана":         rle[:off+2*3*h-1],
		"сжатие RLE, нет ни байта": rle[:off],
	} {
		img, _, err := Preview(bytes.NewReader(data), int64(len(data)), 100)
		if err == nil || img != nil {
			t.Errorf("%s: принят (err=%v)", name, err)
		}
	}
}

// Заголовок врёт про огромный размер, а файл крошечный: отказ до выделения превью и строк
// (память по заголовку не выделяется), в том числе когда таблица RLE корректна, а данных нет.
func TestPreviewNoBigAllocation(t *testing.T) {
	const w, h = 30000, 8333 // 249,99 Мп
	var b bytes.Buffer
	b.WriteString("8BPS")
	b.Write(be16(1))
	b.Write(make([]byte, 6))
	b.Write(be16(3))
	b.Write(be32(h))
	b.Write(be32(w))
	b.Write(be16(8))
	b.Write(be16(modeRGB))
	b.Write(be32(0)) // color data
	b.Write(be32(0)) // ресурсы
	b.Write(be32(0)) // слои
	b.Write(be16(1)) // RLE
	for range 3 * h {
		b.Write(be16(w)) // каждая строка по w байт — в пределе, данных нет
	}
	data := b.Bytes()

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	img, _, err := Preview(bytes.NewReader(data), int64(len(data)), 4096)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrCorrupt) || img != nil {
		t.Fatalf("err = %v", err)
	}
	// Таблица RLE (3·h·2 = 50 КБ) и буфер строк; превью (15 МБ) выделяться не должно.
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 2<<20 {
		t.Fatalf("выделено %d байт до проверки размера", grown)
	}

	// Таблица из нулей (и из длин на байт короче минимума PackBits) проходит проверку остатком
	// файла: отказ — тоже до превью (ровно минимум принят — TestPreviewRejects, flat).
	minRow := 2 * ((w + 127) / 128)
	for _, c := range []struct {
		name string
		n    int // длина каждой строки
		last int // поправка длины последней строки
	}{{"нули", 0, 0}, {"минимум − 1", minRow, -1}} {
		table := bytes.Clone(data)
		for i := range 3 * h {
			binary.BigEndian.PutUint16(table[len(table)-2*3*h+2*i:], uint16(c.n))
		}
		end := len(table) - 2
		binary.BigEndian.PutUint16(table[end:], uint16(c.n+c.last))
		table = append(table, make([]byte, 3*h*c.n)...) // хвост, чтобы сумма влезала в файл
		runtime.ReadMemStats(&before)
		img, _, err = Preview(bytes.NewReader(table), int64(len(table)), 4096)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, ErrCorrupt) || img != nil || after.TotalAlloc-before.TotalAlloc > 2<<20 {
			t.Fatalf("%s: err = %v, выделено %d", c.name, err, after.TotalAlloc-before.TotalAlloc)
		}
	}

	raw := bytes.Clone(data[:len(data)-3*h*2])
	binary.BigEndian.PutUint16(raw[len(raw)-2:], 0) // сжатие raw, данных нет
	runtime.ReadMemStats(&before)
	img, _, err = Preview(bytes.NewReader(raw), int64(len(raw)), 4096)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrCorrupt) || img != nil || after.TotalAlloc-before.TotalAlloc > 1<<20 {
		t.Fatalf("raw: err = %v, выделено %d", err, after.TotalAlloc-before.TotalAlloc)
	}
}

// Крупный шаблон Фотосферы по размерам (4748×3224 RGB RLE): укладывается в разумное время и память превью.
func TestPreviewLarge(t *testing.T) {
	if testkit.Lite() {
		t.Skip("крупный файл — в полном прогоне")
	}
	data := mk(spec{w: 4748, h: 3224, comp: 1, val: func(c, x, y int) byte { return byte((x/64 + y/64 + c) * 9) }})
	var img *image.NRGBA
	d := testkit.Fastest(2, func() { img, _ = preview(t, data, 4096) })
	if img.Bounds() != image.Rect(0, 0, 2374, 1612) { // k = 2
		t.Fatalf("размер %v", img.Bounds())
	}
	if lim := testkit.Limit(5e9); d > lim {
		t.Fatalf("разбор %v дольше %v", d, lim)
	}
}

func BenchmarkPreview(b *testing.B) {
	data := mk(spec{w: 4748, h: 3224, comp: 1, val: func(c, x, y int) byte { return byte((x/37 + y/53 + c) * 7) }})
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := Preview(bytes.NewReader(data), int64(len(data)), 4096); err != nil {
			b.Fatal(err)
		}
	}
}
