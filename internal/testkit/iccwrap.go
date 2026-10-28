package testkit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"testing"
)

// pngSigIHDR — подпись PNG и IHDR целиком (длина 4 + тип 4 + данные 13 + CRC 4): сюда вставляются чанки.
const pngSigIHDR = 8 + 4 + 4 + 13 + 4

// PNGChunk — чанк PNG: длина, тип, данные, CRC (по типу и данным).
func PNGChunk(typ string, data []byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(data))) //nolint:gosec // тестовые чанки короче 4 ГиБ
	b = append(b, typ...)
	b = append(b, data...)
	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(b[4:]))
}

// PNGICCData — данные чанка iCCP: ключевое слово «ICC Profile», NUL, метод 0, zlib-поток профиля.
func PNGICCData(icc []byte) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	_, _ = w.Write(icc)
	_ = w.Close()
	return append([]byte("ICC Profile\x00\x00"), z.Bytes()...)
}

// PNGWithChunks вставляет чанки сразу после IHDR (iCCP обязан идти до PLTE и IDAT). Короткий вход
// возвращается как есть.
func PNGWithChunks(pngData []byte, chunks ...[]byte) []byte {
	if len(pngData) < pngSigIHDR {
		return pngData
	}
	out := append([]byte{}, pngData[:pngSigIHDR]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, pngData[pngSigIHDR:]...)
}

// PNGWithICC пишет в dir файл name: img, закодированная png.Encode, с чанком iCCP из профиля icc.
// Пиксели не меняются: это «картинка, помеченная профилем».
func PNGWithICC(t testing.TB, dir, name string, img image.Image, icc []byte) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return write(t, dir, name, PNGWithChunks(buf.Bytes(), PNGChunk("iCCP", PNGICCData(icc))))
}

// Флаги первого байта данных VP8X.
const (
	webpFlagICC   = 0x20
	webpFlagAlpha = 0x10
)

// WebPWithICC превращает WebP в расширенный (VP8X) с чанком ICCP сразу после VP8X. Холст — размер кадра;
// флаг ICC в VP8X ставится по flag (false — «флаг не стоит, чанк есть»). У WebP, уже начинающегося с VP8X,
// сохраняются остальные флаги, а флаг ICC выставляется по flag. Нечётный профиль добивается нулём
// выравнивания. Не WebP или обрыв — nil.
func WebPWithICC(src, icc []byte, flag bool) []byte {
	if len(src) < 20 || string(src[:4]) != "RIFF" || string(src[8:12]) != "WEBP" {
		return nil
	}
	body := src[12:]
	var vp8x []byte
	if string(body[:4]) == "VP8X" {
		if len(body) < 18 || binary.LittleEndian.Uint32(body[4:]) != 10 {
			return nil
		}
		vp8x = append([]byte{}, body[8:18]...)
		body = body[18:]
	} else {
		w, h, alpha, ok := webpFrame(body)
		if !ok {
			return nil
		}
		vp8x = make([]byte, 10)
		if alpha {
			vp8x[0] |= webpFlagAlpha
		}
		put24(vp8x[4:], w-1)
		put24(vp8x[7:], h-1)
	}
	vp8x[0] &^= webpFlagICC
	if flag {
		vp8x[0] |= webpFlagICC
	}
	out := []byte("RIFF\x00\x00\x00\x00WEBP")
	out = appendRIFFChunk(out, "VP8X", vp8x)
	out = appendRIFFChunk(out, "ICCP", icc)
	out = append(out, body...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)-8)) //nolint:gosec // тестовые файлы короче 4 ГиБ
	return out
}

func appendRIFFChunk(b []byte, typ string, data []byte) []byte {
	b = append(b, typ...)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(data))) //nolint:gosec // см. WebPWithICC
	b = append(b, data...)
	if len(data)&1 == 1 {
		b = append(b, 0)
	}
	return b
}

func put24(b []byte, v int) {
	b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16)
}

// webpFrame — размер и альфа кадра простого WebP (первый чанк VP8 или VP8L).
func webpFrame(b []byte) (w, h int, alpha, ok bool) {
	if len(b) < 8 {
		return 0, 0, false, false
	}
	d := b[8:]
	switch string(b[:4]) {
	case "VP8 ":
		if len(d) < 10 {
			return 0, 0, false, false
		}
		return int(binary.LittleEndian.Uint16(d[6:]) & 0x3fff), int(binary.LittleEndian.Uint16(d[8:]) & 0x3fff), false, true
	case "VP8L":
		if len(d) < 5 || d[0] != 0x2f {
			return 0, 0, false, false
		}
		bits := binary.LittleEndian.Uint32(d[1:])
		return int(bits&0x3fff) + 1, int(bits>>14&0x3fff) + 1, bits>>28&1 == 1, true
	}
	return 0, 0, false, false
}
