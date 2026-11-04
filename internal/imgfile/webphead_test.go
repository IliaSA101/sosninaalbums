package imgfile

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"path/filepath"
	"runtime"
	"testing"
	"testing/iotest"
)

// riffWebP собирает RIFF/WEBP из готовых чанков (с выравниванием нечётных до чётных).
func riffWebP(chunks ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.Write(c)
		if len(c)%2 == 1 {
			body.WriteByte(0)
		}
	}
	out := []byte("RIFF\x00\x00\x00\x00")
	binary.LittleEndian.PutUint32(out[4:], uint32(body.Len()))
	return append(out, body.Bytes()...)
}

// chunk — чанк с заданным именем и данными.
func chunk(id string, data []byte) []byte {
	out := append([]byte(id), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
	return append(out, data...)
}

// vp8xChunk — VP8X с холстом w×h и флагами.
func vp8xChunk(flags byte, w, h int) []byte {
	d := make([]byte, 10)
	d[0] = flags
	w, h = w-1, h-1
	d[4], d[5], d[6] = byte(w), byte(w>>8), byte(w>>16)
	d[7], d[8], d[9] = byte(h), byte(h>>8), byte(h>>16)
	return chunk("VP8X", d)
}

// vp8lChunk — VP8L с заголовком w×h без данных (хватает чтению заголовка).
func vp8lChunk(w, h int) []byte {
	d := []byte{0x2f, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(d[1:], uint32(w-1)|uint32(h-1)<<14)
	return chunk("VP8L", d)
}

// vp8Chunk — VP8 с кадровым заголовком w×h без данных.
func vp8Chunk(w, h int) []byte {
	d := []byte{0x10, 0, 0, 0x9d, 0x01, 0x2a, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(d[6:], uint16(w))
	binary.LittleEndian.PutUint16(d[8:], uint16(h))
	return chunk("VP8 ", d)
}

// Кривые и пограничные WebP: анимация не только в первом чанке, VP8X без кадра, расхождение с холстом,
// пределы обхода чанков. Байты собираются в тесте.
func TestDimsOfWebPChunks(t *testing.T) {
	junk := make([]byte, 7) // нечётная длина: проверка выравнивания
	many := make([][]byte, 0, 40)
	for range 40 {
		many = append(many, chunk("JUNK", nil))
	}
	hugeLen := append([]byte("EXIF\xff\xff\xff\xff"), 0)
	bad := []struct {
		name string
		data []byte
	}{
		{"ANIM без флага VP8X", riffWebP(vp8xChunk(0, 4, 4), chunk("ANIM", make([]byte, 6)), vp8lChunk(4, 4))},
		{"ANMF вместо кадра", riffWebP(vp8xChunk(0, 4, 4), chunk("ANMF", make([]byte, 16)))},
		{"ANIM без VP8X", riffWebP(chunk("ANIM", make([]byte, 6)), vp8lChunk(4, 4))},
		{"флаг анимации и кадр", riffWebP(vp8xChunk(vp8xAnimation, 4, 4), vp8lChunk(4, 4))},
		{"VP8X без кадра", riffWebP(vp8xChunk(0, 4, 4))},
		{"VP8X и метаданные без кадра", riffWebP(vp8xChunk(0, 4, 4), chunk("EXIF", junk))},
		{"кадр больше холста", riffWebP(vp8xChunk(0, 4, 4), vp8lChunk(5, 4))},
		{"кадр VP8 не равен холсту", riffWebP(vp8xChunk(0, 8, 8), vp8Chunk(16, 8))},
		{"VP8X не первый", riffWebP(chunk("JUNK", nil), vp8xChunk(0, 4, 4), vp8lChunk(4, 4))},
		{"VP8X не 10 байт", riffWebP(chunk("VP8X", make([]byte, 12)), vp8lChunk(4, 4))},
		{"слишком много чанков", riffWebP(append(many, vp8lChunk(4, 4))...)},
		{"чанк длиннее предела", riffWebP(hugeLen)},
		{"VP8L короче заголовка", riffWebP(chunk("VP8L", []byte{0x2f, 0}))},
		{"VP8L без сигнатуры", riffWebP(chunk("VP8L", []byte{0x2e, 0, 0, 0, 0}))},
		{"VP8 без стартового кода", riffWebP(chunk("VP8 ", make([]byte, 10)))},
		{"VP8 не ключевой кадр", riffWebP(chunk("VP8 ", []byte{0x11, 0, 0, 0x9d, 0x01, 0x2a, 4, 0, 4, 0}))},
		{"обрыв внутри чанка", riffWebP(vp8xChunk(0, 4, 4))[:20]},
		{"только RIFF", []byte("RIFF\x04\x00\x00\x00WEBP")},
	}
	for _, c := range bad {
		if d, err := DimsOf(bytes.NewReader(c.data), 0); err == nil {
			t.Errorf("%s: принят %+v", c.name, d)
		}
	}
	good := []struct {
		name  string
		data  []byte
		w, h  int
		model color.Model
	}{
		{"простой VP8L", riffWebP(vp8lChunk(4, 3)), 4, 3, color.NRGBAModel},
		{"простой VP8", riffWebP(vp8Chunk(6, 5)), 6, 5, color.YCbCrModel},
		{"VP8X и VP8L", riffWebP(vp8xChunk(0, 4, 3), vp8lChunk(4, 3)), 4, 3, color.NRGBAModel},
		{"метаданные до кадра", riffWebP(vp8xChunk(0x20, 6, 5), chunk("ICCP", junk), chunk("EXIF", junk), vp8Chunk(6, 5)), 6, 5, color.YCbCrModel},
		{"ALPH и VP8", riffWebP(vp8xChunk(vp8xAlpha, 6, 5), chunk("ALPH", junk), vp8Chunk(6, 5)), 6, 5, color.NYCbCrAModel},
		{"предел числа чанков", riffWebP(append(many[:webpMaxChunks-1:webpMaxChunks-1], vp8lChunk(2, 2))...), 2, 2, color.NRGBAModel},
	}
	for _, c := range good {
		// Источник с Seek (файл) и поток без него: пропуск чанков идёт разными путями.
		d, err := DimsOf(bytes.NewReader(c.data), 0)
		if err != nil || d.W != c.w || d.H != c.h || d.Format != "webp" || d.Color != c.model {
			t.Errorf("%s (seek): %+v, %v", c.name, d, err)
		}
		d, err = DimsOf(iotest.OneByteReader(bytes.NewReader(c.data)), 0)
		if err != nil || d.W != c.w || d.H != c.h || d.Color != c.model {
			t.Errorf("%s (поток): %+v, %v", c.name, d, err)
		}
	}
}

// Вес WebP в бюджете покрывает аллокации декода: lossy ≈ 0,4 от 4 Б/пкс, lossless — 1,0, палитра — 1,5,
// lossy с альфой — до 2,1 (замер 8000×8000, здесь 1024×1024). Образцы — градиентные блоки из Pillow, до 5 КБ.
func TestWebPDecodeWeight(t *testing.T) {
	const px = 1024 * 1024
	cases := []struct {
		file  string
		model color.Model
		want  int64
	}{
		{"mem_lossy.webp", color.YCbCrModel, px},
		{"mem_truecolor.webp", color.NRGBAModel, 2 * px},
		{"mem_pal16.webp", color.NRGBAModel, 2 * px},
		{"mem_alpha.webp", color.NYCbCrAModel, px * 9 / 4},
	}
	for _, c := range cases {
		p := filepath.Join("testdata", c.file)
		d, err := FileDims(p, 0)
		if err != nil || d.Color != c.model || d.W != 1024 || d.H != 1024 {
			t.Fatalf("%s: %+v, %v", c.file, d, err)
		}
		n, _, err := checkFile(p)
		if err != nil || n != c.want {
			t.Errorf("%s: вес %d, ждали %d (%v)", c.file, n, c.want, err)
		}
		var a, b runtime.MemStats
		runtime.ReadMemStats(&a)
		_, release, err := OpenOriented(p)
		runtime.ReadMemStats(&b)
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		release()
		// 4 байта на единицу веса; запас 512 КБ на фоновые аллокации (TotalAlloc общий на процесс).
		if alloc := int64(b.TotalAlloc - a.TotalAlloc); alloc > 4*n+512<<10 {
			t.Errorf("%s: декод выделил %d Б при весе %d (%d Б)", c.file, alloc, n, 4*n)
		}
	}
}
