package imgfile

import (
	"encoding/binary"
	"time"
)

// exifOrientation читает тег Orientation (0x0112) из сегмента APP1 (вместе с «Exif\0\0»).
// 0 — тега нет или значение вне 1–8. Значения 5–8 означают, что фото повёрнуто на 90°:
// ширину и высоту надо поменять местами.
func exifOrientation(seg []byte) int {
	if len(seg) < 14 || string(seg[:6]) != "Exif\x00\x00" {
		return 0
	}
	res, found := 0, false
	exifWalk(seg[6:], func(level int, e exifEntry) {
		if found || level != exifLevelIFD0 || e.Tag != 0x0112 {
			return
		}
		found = true // берётся первая запись; значение вне 1–8 — «нет тега»
		if v := int(e.bo.Uint16(e.field)); v >= 1 && v <= 8 {
			res = v
		}
	})
	return res
}

// Пределы обхода EXIF недоверенного файла: записей в одном IFD, длина записи и строки времени.
const (
	exifMaxEntries = 512
	exifEntryLen   = 12
	exifTakenLen   = 20 // «YYYY:MM:DD HH:MM:SS» + NUL
)

// Уровни IFD в обходе: IFD0 → Exif IFD (0x8769) → Interop IFD (0xA005).
const (
	exifLevelIFD0 = iota
	exifLevelExif
	exifLevelInterop
)

// exifEntry — одна 12-байтовая запись IFD вместе с буфером TIFF, на который она ссылается.
type exifEntry struct {
	Tag, Type uint16
	Count     uint32
	field     []byte // 4 байта значения или смещения
	t         []byte
	bo        binary.ByteOrder
}

// Offset — поле значения как смещение от начала TIFF.
func (e exifEntry) Offset() uint32 { return e.bo.Uint32(e.field) }

// ASCII — значение строкового тега (тип 2): до 4 байт лежат в самой записи, больше — по
// смещению. nil — тип не тот, длина 0 или значение выходит за буфер. Смещение и длина — uint32,
// складываются в uint64 до сравнения с буфером.
func (e exifEntry) ASCII() []byte {
	if e.Type != 2 || e.Count == 0 {
		return nil
	}
	if e.Count <= 4 {
		return e.field[:e.Count]
	}
	off := uint64(e.Offset())
	if off+uint64(e.Count) > uint64(len(e.t)) {
		return nil
	}
	return e.t[off : off+uint64(e.Count)]
}

// exifWalk обходит IFD0 → Exif IFD → Interop IFD (ровно три уровня, без рекурсии) и зовёт f
// на каждую запись. t — TIFF-часть APP1 (после «Exif\0\0»). Смещения проверяются против длины
// буфера, записей в IFD не больше exifMaxEntries: мусор даёт пустой обход, не выход за буфер.
func exifWalk(t []byte, f func(level int, e exifEntry)) {
	if len(t) < 8 {
		return
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return
	}
	// ifd зовёт f и g на каждую запись; смещение — uint32, сравнивается до перевода в int.
	ifd := func(level int, off uint32, g func(e exifEntry)) {
		if off < 8 || uint64(off)+2 > uint64(len(t)) {
			return
		}
		n := min(int(bo.Uint16(t[off:])), exifMaxEntries)
		for i := range n {
			p := int(off) + 2 + i*exifEntryLen
			if p+exifEntryLen > len(t) {
				return
			}
			r := t[p : p+exifEntryLen]
			e := exifEntry{Tag: bo.Uint16(r), Type: bo.Uint16(r[2:]), Count: bo.Uint32(r[4:]), field: r[8:], t: t, bo: bo}
			f(level, e)
			g(e)
		}
	}
	ifd0 := bo.Uint32(t[4:])
	var exifOff, interOff uint32
	ifd(exifLevelIFD0, ifd0, func(e exifEntry) {
		if e.Tag == 0x8769 && e.Offset() != ifd0 { // указатель на IFD0: дата оттуда — не DateTimeOriginal
			exifOff = e.Offset()
		}
	})
	ifd(exifLevelExif, exifOff, func(e exifEntry) {
		if e.Tag == 0xA005 && e.Offset() != ifd0 && e.Offset() != exifOff { // тот же запрет на петлю
			interOff = e.Offset()
		}
	})
	ifd(exifLevelInterop, interOff, func(exifEntry) {})
}

// exifInterop — InteropIndex (0x0001 в Interop IFD): «R03» у Adobe RGB. Только ASCII до 4 байт,
// разбор строго по Count: Count 0 — пусто (даже при «R03\0» в поле), Count 1–3 — первые Count
// байт без NUL, Count 4 — четыре байта, хвостовые NUL срезаются.
func exifInterop(t []byte) string {
	var res string
	exifWalk(t, func(level int, e exifEntry) {
		if level == exifLevelInterop && e.Tag == iccInteropIndex && e.Type == 2 && e.Count <= 4 {
			res = string(trimNUL(e.field[:e.Count]))
		}
	})
	return res
}

// exifTaken — время съёмки из Exif IFD: DateTimeOriginal (0x9003), нет — DateTimeDigitized
// (0x9004). Секунды Unix, местное время камеры читается как UTC; 0 — нет или мусор.
func exifTaken(t []byte) int64 {
	var orig, digi int64
	exifWalk(t, func(level int, e exifEntry) {
		if level != exifLevelExif || (e.Tag != 0x9003 && e.Tag != 0x9004) || e.Count != exifTakenLen {
			return
		}
		v := parseExifTime(e.ASCII())
		switch {
		case e.Tag == 0x9003 && orig == 0:
			orig = v
		case e.Tag == 0x9004 && digi == 0:
			digi = v
		}
	})
	if orig != 0 {
		return orig
	}
	return digi
}

// parseExifTime разбирает ровно «YYYY:MM:DD HH:MM:SS\0»: цифры на местах, год 1900…2200, дата
// существует (30 февраля — мусор). Даты до 1970 дают 0: у камеры это не время съёмки.
func parseExifTime(b []byte) int64 {
	if len(b) != exifTakenLen || b[19] != 0 || b[4] != ':' || b[7] != ':' || b[10] != ' ' || b[13] != ':' || b[16] != ':' {
		return 0
	}
	var f [6]int
	for k, pos := range [6][2]int{{0, 4}, {5, 2}, {8, 2}, {11, 2}, {14, 2}, {17, 2}} {
		for _, c := range b[pos[0] : pos[0]+pos[1]] {
			if c < '0' || c > '9' {
				return 0
			}
			f[k] = f[k]*10 + int(c-'0')
		}
	}
	if f[0] < 1900 || f[0] > 2200 || f[1] < 1 || f[1] > 12 || f[2] < 1 || f[3] > 23 || f[4] > 59 || f[5] > 59 {
		return 0
	}
	d := time.Date(f[0], time.Month(f[1]), f[2], f[3], f[4], f[5], 0, time.UTC)
	if d.Day() != f[2] { // 31 апреля, 30 февраля
		return 0
	}
	return max(d.Unix(), 0)
}
