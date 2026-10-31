package imgfile

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"io"
)

// Пределы обхода чанков PNG до кадра (профиль — iccMaxBytes и iccMaxSegments, как у JPEG).
const (
	pngMaxSkip  = 16 << 20  // суммарно пропускаемых байт до iCCP: XMP и EXIF обычно идут после него
	pngMaxChunk = 1<<31 - 1 // правило стандарта PNG (длина чанка < 2³¹), не защита памяти: ту дают пропуск через Seek и pngMaxSkip
	pngIHDRLen  = 13        // данные IHDR
	pngKeyMax   = 79        // ключевое слово iCCP: 1–79 байт
	pngColorGr  = 0         // типы цвета PNG: ч/б и ч/б с альфой — без RGB-профиля
	pngColorGrA = 4
)

// pngHead читает профиль iCCP из чанков PNG до кадра. r стоит за 12 прочитанными байтами
// (подпись и длина IHDR, она в ihdrLen). Читает только из r без буфера: пропуск идёт через Seek
// файла, и обёртка bufio разошлась бы с ним в позиции. Порядок проверок у чанка: сначала тип,
// потом длина. Профиль заявлен, но не прочитан (длина больше предела, CRC, zlib, обрыв) — ColorOther;
// чанки не по правилам (после PLTE/IDAT, второй iCCP, ч/б с RGB-профилем) и чужой мусор — «без профиля».
func pngHead(r io.Reader, ihdrLen []byte) Head {
	none := Head{Orient: 1}
	var ih [4 + pngIHDRLen + 4]byte // тип, данные, CRC
	if binary.BigEndian.Uint32(ihdrLen) != pngIHDRLen {
		return none
	}
	if _, err := io.ReadFull(r, ih[:]); err != nil || string(ih[:4]) != "IHDR" {
		return none
	}
	colorType := ih[4+9]
	var skipped int64
	for range iccMaxSegments {
		var hdr [8]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return none
		}
		n := int64(binary.BigEndian.Uint32(hdr[:4]))
		switch string(hdr[4:]) {
		case "PLTE", "IDAT", "IEND":
			return none // iCCP после них не действует
		case "iCCP":
			// Первый iCCP решает всё: второй и sRGB после него не читаются.
			return pngICCP(r, n, colorType)
		}
		skipped += n + 4 // данные и CRC
		if n > pngMaxChunk || skipped > pngMaxSkip {
			return none
		}
		if err := skipBytes(r, n+4); err != nil {
			return none
		}
	}
	return none
}

// pngICCP разбирает данные первого iCCP длиной n; ошибка любого шага — ColorOther.
func pngICCP(r io.Reader, n int64, colorType byte) Head {
	bad := Head{Orient: 1, Color: ColorOther, Profile: iccUnknownName}
	if n > iccMaxBytes { // в том числе ≥ 2³¹: данные не читаем
		return bad
	}
	buf := make([]byte, n+4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return bad
	}
	data, sum := buf[:n], binary.BigEndian.Uint32(buf[n:])
	if crc32.Update(crc32.ChecksumIEEE([]byte("iCCP")), crc32.IEEETable, data) != sum {
		return bad
	}
	p, ok := pngProfile(data)
	if !ok {
		return bad
	}
	if (colorType == pngColorGr || colorType == pngColorGrA) && len(p) >= 20 && string(p[16:20]) == "RGB " {
		return Head{Orient: 1} // как libpng: RGB-профиль на ч/б PNG не допускается
	}
	h := Head{Orient: 1}
	h.Color, h.Profile, h.xf = classify(p)
	return h
}

// pngProfile — профиль из данных iCCP: ключевое слово, NUL, метод 0, поток zlib. Распаковка
// не больше iccMaxBytes (бомба 1 МиБ → 1 ГиБ не растёт); FDICT и пустой поток — ошибка.
func pngProfile(data []byte) ([]byte, bool) {
	i := bytes.IndexByte(data[:min(len(data), pngKeyMax+1)], 0)
	if i < 1 || i+2 > len(data) || data[i+1] != 0 {
		return nil, false
	}
	zr, err := zlib.NewReader(bytes.NewReader(data[i+2:]))
	if err != nil {
		return nil, false
	}
	p, err := io.ReadAll(io.LimitReader(zr, iccMaxBytes+1))
	if err != nil || len(p) > iccMaxBytes {
		return nil, false
	}
	return p, true
}
