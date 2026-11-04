package testkit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// chunkList разбирает PNG на чанки (тип -> данные по порядку), проверяя CRC.
func chunkList(t *testing.T, b []byte) (types []string, data [][]byte) {
	t.Helper()
	b = b[8:]
	for len(b) > 0 {
		n := int(binary.BigEndian.Uint32(b))
		typ, d := string(b[4:8]), b[8:8+n]
		if got := binary.BigEndian.Uint32(b[8+n:]); got != crc32.ChecksumIEEE(b[4:8+n]) {
			t.Fatalf("%s: неверный CRC", typ)
		}
		types, data = append(types, typ), append(data, d)
		b = b[12+n:]
	}
	return types, data
}

func gradient(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x * 4), uint8(y * 4), 90, 255})
		}
	}
	return img
}

// PNGChunk — длина, тип, данные, верный CRC; пустые данные тоже чанк.
func TestPNGChunk(t *testing.T) {
	for _, d := range [][]byte{nil, {}, []byte("abc")} {
		c := PNGChunk("tEXt", d)
		if len(c) != 12+len(d) || binary.BigEndian.Uint32(c) != uint32(len(d)) || string(c[4:8]) != "tEXt" ||
			binary.BigEndian.Uint32(c[8+len(d):]) != crc32.ChecksumIEEE(c[4:8+len(d)]) {
			t.Errorf("данные %q: % x", d, c)
		}
	}
}

// PNGWithICC: файл читается стандартным декодером, iCCP стоит сразу после IHDR и разворачивается в профиль.
func TestPNGWithICC(t *testing.T) {
	icc := SRGBProfile(DescV2("test"))
	img := gradient(20, 12)
	p := PNGWithICC(t, TempDir(t), "a.png", img, icc)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	types, data := chunkList(t, raw)
	if types[0] != "IHDR" || types[1] != "iCCP" {
		t.Fatalf("порядок чанков: %v", types)
	}
	d := data[1]
	if !bytes.HasPrefix(d, []byte("ICC Profile\x00\x00")) {
		t.Fatalf("ключевое слово и метод: %q", d[:14])
	}
	zr, err := zlib.NewReader(bytes.NewReader(d[13:]))
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(zr)
	if err != nil || !bytes.Equal(got, icc) {
		t.Errorf("профиль не вернулся: %v", err)
	}
	dec, err := png.Decode(bytes.NewReader(raw))
	if err != nil || dec.Bounds() != img.Bounds() {
		t.Fatalf("декод: %v", err)
	}
	// Вставка нескольких чанков идёт по порядку; короткий вход не меняется.
	if got := PNGWithChunks([]byte("PNG"), PNGChunk("a", nil)); string(got) != "PNG" {
		t.Errorf("короткий вход: %q", got)
	}
	var plain bytes.Buffer
	_ = png.Encode(&plain, img)
	types, _ = chunkList(t, PNGWithChunks(plain.Bytes(), PNGChunk("tEXt", nil), PNGChunk("zTXt", nil)))
	if types[1] != "tEXt" || types[2] != "zTXt" {
		t.Errorf("порядок вставки: %v", types)
	}
}

// riffChunks разбирает тело RIFF/WEBP на чанки (тип -> данные по порядку), проверяя размеры и выравнивание.
func riffChunks(t *testing.T, b []byte) (types []string, data [][]byte) {
	t.Helper()
	if string(b[:4]) != "RIFF" || string(b[8:12]) != "WEBP" || binary.LittleEndian.Uint32(b[4:]) != uint32(len(b)-8) {
		t.Fatalf("заголовок RIFF: % x", b[:12])
	}
	for b = b[12:]; len(b) > 0; {
		n := int(binary.LittleEndian.Uint32(b[4:]))
		if len(b) < 8+n+n&1 {
			t.Fatalf("чанк %s обрезан", b[:4])
		}
		types, data = append(types, string(b[:4])), append(data, b[8:8+n])
		b = b[8+n+n&1:]
	}
	return types, data
}

// checkWrapped проверяет обёртку src: VP8X и ICCP первыми, флаг ICC по flag, прочие флаги и чанки целы.
func checkWrapped(t *testing.T, f string, src, icc []byte, flag bool, w, h int) {
	t.Helper()
	srcTypes, srcData := riffChunks(t, src)
	out := WebPWithICC(src, icc, flag)
	if out == nil {
		t.Fatalf("%s: nil", f)
	}
	types, data := riffChunks(t, out)
	if types[0] != "VP8X" || types[1] != "ICCP" || len(data[0]) != 10 || !bytes.Equal(data[1], icc) {
		t.Fatalf("%s: чанки %v", f, types)
	}
	// Холст — размер кадра (w×h из заголовка тестового файла), по 24 бита «размер−1» со смещений 4 и 7.
	wantCanvas := []byte{byte(w - 1), byte((w - 1) >> 8), byte((w - 1) >> 16), byte(h - 1), byte((h - 1) >> 8), byte((h - 1) >> 16)}
	if !bytes.Equal(data[0][4:10], wantCanvas) {
		t.Errorf("%s: холст % x, ждали % x (%d×%d)", f, data[0][4:10], wantCanvas, w, h)
	}
	if got := data[0][0]&webpFlagICC != 0; got != flag {
		t.Errorf("%s: флаг ICC %v, ждали %v", f, got, flag)
	}
	// Остальное — те же чанки, что были (исходный VP8X заменён обёрткой).
	rest := srcTypes
	if rest[0] == "VP8X" {
		rest = rest[1:]
		if data[0][0]&^webpFlagICC != srcData[0][0]&^webpFlagICC {
			t.Errorf("%s: прочие флаги VP8X изменились", f)
		}
	}
	if len(types) != 2+len(rest) {
		t.Errorf("%s: чанки %v, исходные %v", f, types, srcTypes)
	}
}

// WebPWithICC: простые lossy и lossless и расширенный с альфой получают VP8X и ICCP сразу за ним, остальные
// чанки целы, холст равен кадру; флаг ICC — по аргументу, прочие флаги целы; нечётная длина выравнивается.
// Что x/image/webp декодирует обёртку, проверяют тесты imgfile (testkit не импортирует декодер).
func TestWebPWithICC(t *testing.T) {
	icc := []byte("odd-length-profile")
	icc = icc[:len(icc)-1] // 17 байт: нужен байт выравнивания
	sizes := map[string][2]int{"lossy16.webp": {16, 16}, "lossless24x16.webp": {24, 16}, "mem_alpha.webp": {1024, 1024}}
	for _, f := range []string{"lossy16.webp", "lossless24x16.webp", "mem_alpha.webp"} {
		w, h := sizes[f][0], sizes[f][1]
		src, err := os.ReadFile(filepath.Join("..", "imgfile", "testdata", f))
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []bool{true, false} {
			checkWrapped(t, f, src, icc, flag, w, h)
		}
		// Профиль пустой и чётной длины тоже собираются.
		for _, p := range [][]byte{nil, icc[:len(icc)-1]} {
			checkWrapped(t, f, src, p, true, w, h)
		}
	}
	for _, bad := range [][]byte{nil, []byte("RIFF"), []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), []byte("not a webp file at all....")} {
		if WebPWithICC(bad, icc, true) != nil {
			t.Errorf("%q: ждали nil", bad)
		}
	}
}
