package imgfile

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
)

// Head — то, что фото сообщает о себе в заголовке JPEG до данных изображения.
type Head struct {
	Orient  int    // EXIF Orientation, 1 — нет тега или не JPEG
	Color   Color  // класс профиля
	Profile string // имя профиля для интерфейса
	XMP     []byte // основной пакет XMP без префикса; nil — нет. Разбирает только сканер
	Taken   int64  // EXIF DateTimeOriginal: секунды Unix, местное время камеры как UTC; 0 — нет или мусор

	xf *colorspace.Xform // пересчёт в sRGB для классов adobe, p3 и icc; nil — не нужен
}

// XMPPrefix — начало APP1 с основным пакетом XMP. Расширенный (`…/xmp/extension/`) не читаем:
// оценка и метка лежат в основном.
const XMPPrefix = "http://ns.adobe.com/xap/1.0/\x00"

// Header читает ориентацию, время съёмки, цветовой профиль и XMP за один проход по сегментам
// (ICC важнее EXIF Interop: так камеры Canon/Nikon помечают Adobe RGB без профиля).
// CMYK без ICC здесь не виден — его определяет readInfo по DecodeConfig.
// Ошибка чтения не-JPEG и мусор дают {1, "", "", nil}.
func Header(r io.Reader) Head { return headOf(jpegSegments(r)) }

// headOf собирает Head из уже прочитанных сегментов.
func headOf(s jpegSegs) Head {
	h := Head{Orient: 1, XMP: s.xmp, Taken: s.taken}
	if s.orient > 0 {
		h.Orient = s.orient
	}
	switch {
	case s.icc != nil:
		h.Color, h.Profile, h.xf = classify(s.icc)
	case s.interop == "R03":
		h.Color, h.Profile, h.xf = ColorAdobe, ICCAdobeEXIF, colorspace.Adobe
	}
	return h
}

// jpegSegs — сырое содержимое нужных сегментов заголовка.
type jpegSegs struct {
	icc     []byte // ICC-профиль, куски склеены
	interop string // EXIF InteropIndex
	orient  int    // EXIF Orientation, 0 — тега нет
	taken   int64  // EXIF время съёмки, 0 — нет
	exif    []byte // TIFF-часть первого APP1 «Exif» (срез прочитанного сегмента); nil — нет
	xmp     []byte // первый APP1 XMP без префикса
}

// jpegSegments проходит сегменты от SOI до SOS (не больше iccMaxSegments) и собирает
// ICC, EXIF (ориентация, Interop) и XMP. Остальные сегменты пропускаются.
// APP1 и так читается в память целиком (≤ 65 533 байт), поэтому XMP не стоит лишнего чтения.
func jpegSegments(r io.Reader) jpegSegs {
	var out jpegSegs
	br := bufio.NewReader(r)
	var soi [2]byte
	if _, err := io.ReadFull(br, soi[:]); err != nil || soi != [2]byte{0xFF, 0xD8} {
		return out
	}
	chunks := map[int][]byte{}
	count, total, haveExif := 0, 0, false
	for range iccMaxSegments {
		var m [4]byte
		if _, err := io.ReadFull(br, m[:]); err != nil || m[0] != 0xFF {
			break
		}
		size := int(binary.BigEndian.Uint16(m[2:])) - 2
		if size < 0 || m[1] == 0xDA {
			break
		}
		if m[1] != 0xE1 && m[1] != 0xE2 {
			if _, err := br.Discard(size); err != nil {
				break
			}
			continue
		}
		seg := make([]byte, size) // ≤ 65 533 байта: длина в 16 битах
		if _, err := io.ReadFull(br, seg); err != nil {
			break
		}
		switch {
		case m[1] == 0xE1 && !haveExif && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00":
			haveExif = true
			out.exif = seg[6:]
			out.interop = exifInterop(seg[6:])
			out.orient = exifOrientation(seg)
			out.taken = exifTaken(seg[6:])
		case m[1] == 0xE1 && out.xmp == nil && len(seg) >= len(XMPPrefix) && string(seg[:len(XMPPrefix)]) == XMPPrefix:
			// Некоторые программы добивают APP1 нулями до размера: NUL в XML — ошибка разбора.
			out.xmp = bytes.TrimRight(seg[len(XMPPrefix):], "\x00")
		case m[1] == 0xE2 && len(seg) > 14 && string(seg[:12]) == "ICC_PROFILE\x00":
			seq, n := int(seg[12]), int(seg[13])
			if seq < 1 || seq > n || chunks[seq] != nil || total+len(seg)-14 > iccMaxBytes {
				continue
			}
			if count == 0 {
				count = n
			} else if n != count {
				continue // куски от разных профилей — не склеиваем
			}
			chunks[seq] = seg[14:]
			total += len(seg) - 14
		}
	}
	out.icc = joinICC(chunks, count)
	return out
}
