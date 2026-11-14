package colorspace

import (
	_ "embed"
	"encoding/binary"
)

// Компактный sRGB v2 (CC0, см. NOTICE.md): v2 понимают шире, чем v4.
//
//go:embed sRGB-v2-magic.icc
var srgbICC []byte

// iccMarker — метка сегмента APP2 с ICC-профилем.
const iccMarker = "ICC_PROFILE\x00"

// SRGBProfile возвращает копию встроенного ICC-профиля sRGB.
func SRGBProfile() []byte {
	return append([]byte(nil), srgbICC...)
}

// SRGBSegment возвращает готовый сегмент JPEG APP2 (FF E2, длина,
// «ICC_PROFILE\0», 1/1, профиль) для вставки после APP0/COM.
// Профиль влезает в один сегмент (< 64 КБ); одним куском, без разбиения.
func SRGBSegment() []byte {
	n := 2 + len(iccMarker) + 2 + len(srgbICC)
	seg := make([]byte, 0, 2+n)
	seg = binary.BigEndian.AppendUint16(append(seg, 0xFF, 0xE2), uint16(n&0xFFFF))
	seg = append(seg, iccMarker...)
	seg = append(seg, 1, 1)
	return append(seg, srgbICC...)
}

// iccTag находит тег профиля по сигнатуре и возвращает его данные.
func iccTag(p []byte, sig string) []byte {
	if len(p) < 132 {
		return nil
	}
	n := int(binary.BigEndian.Uint32(p[128:]))
	for i := 0; i < n && 132+12*(i+1) <= len(p); i++ {
		e := p[132+12*i:]
		if string(e[:4]) != sig {
			continue
		}
		off, size := int(binary.BigEndian.Uint32(e[4:])), int(binary.BigEndian.Uint32(e[8:]))
		if off < 0 || size < 0 || off > len(p) || size > len(p)-off {
			return nil
		}
		return p[off : off+size]
	}
	return nil
}
