package testkit

import "encoding/binary"

// ShotFields — что ExifShot кладёт в EXIF; нулевое поле не пишется. Строки ложатся как есть + NUL:
// так тест подсовывает мусор, NUL и управляющие знаки. Рациональные — {числитель, знаменатель},
// знаменатель 0 пишется как есть.
type ShotFields struct {
	Make, Model, Lens string // IFD0: 0x010F, 0x0110; Exif IFD: 0xA434
	Exposure          [2]uint32
	FNumber           [2]uint32
	Focal             [2]uint32
	ISO               int
	ISOLong           bool   // ISO типом LONG, а не SHORT
	RatCount          uint32 // Count у рациональных; 0 — 1
	Orig              string // 0x9003 DateTimeOriginal
	Orient            uint16 // 0x0112 Orientation в IFD0
}

// shotEntry — запись IFD: поле значения (до 4 байт) или данные, которые уедут за таблицу.
type shotEntry struct {
	tag, typ uint16
	count    uint32
	val      []byte // ≤ 4 байт — в самой записи; больше — по смещению
	inline   uint32 // значение-смещение, если val == nil и это указатель на IFD
}

// ExifShot — APP1 с EXIF: Make и Model в IFD0, остальное в Exif IFD (0x8769). Порядок байт —
// bigEndian или little. Построитель рядом с ExifTaken: те же TIFF-заголовок и сегмент.
func ExifShot(bigEndian bool, f ShotFields) []byte {
	var bo order = binary.LittleEndian
	t := []byte("II*\x00")
	if bigEndian {
		bo, t = binary.BigEndian, []byte("MM\x00*")
	}
	t = bo.AppendUint32(t, 8)
	ifd0, exif := shotEntries(bo, f)

	// Раскладка: IFD0 (+ указатель на Exif IFD), Exif IFD, затем данные записей.
	ifd0 = append(ifd0, shotEntry{tag: 0x8769, typ: 4, count: 1})
	ifd0Size := 2 + 12*len(ifd0) + 4
	exifOff := uint32(8 + ifd0Size)            //nolint:gosec // единицы
	data := exifOff + uint32(2+12*len(exif)+4) //nolint:gosec // единицы
	ifd0[len(ifd0)-1].inline = exifOff
	var tail []byte
	table := func(es []shotEntry) []byte {
		b := bo.AppendUint16(nil, uint16(len(es))) //nolint:gosec // единицы
		for _, e := range es {
			b = bo.AppendUint16(b, e.tag)
			b = bo.AppendUint16(b, e.typ)
			b = bo.AppendUint32(b, e.count)
			switch {
			case e.val == nil:
				b = bo.AppendUint32(b, e.inline)
			case len(e.val) <= 4:
				b = append(b, e.val...)
				b = append(b, make([]byte, 4-len(e.val))...)
			default:
				b = bo.AppendUint32(b, data+uint32(len(tail))) //nolint:gosec // единицы
				tail = append(tail, e.val...)
			}
		}
		return bo.AppendUint32(b, 0)
	}
	t = append(t, table(ifd0)...)
	t = append(t, table(exif)...)
	t = append(t, tail...)
	return Segment(0xE1, append([]byte("Exif\x00\x00"), t...))
}

// shotEntries — записи IFD0 и Exif IFD по возрастанию тегов (указатель 0x8769 допишет ExifShot).
func shotEntries(bo order, f ShotFields) (ifd0, exif []shotEntry) {
	str := func(tag uint16, s string) shotEntry {
		b := append([]byte(s), 0)
		return shotEntry{tag: tag, typ: 2, count: uint32(len(b)), val: b} //nolint:gosec // короткие строки
	}
	rat := func(tag uint16, r [2]uint32) shotEntry {
		b := bo.AppendUint32(bo.AppendUint32(nil, r[0]), r[1])
		n := f.RatCount
		if n == 0 {
			n = 1
		}
		return shotEntry{tag: tag, typ: 5, count: n, val: b}
	}
	if f.Make != "" {
		ifd0 = append(ifd0, str(0x010F, f.Make))
	}
	if f.Model != "" {
		ifd0 = append(ifd0, str(0x0110, f.Model))
	}
	if f.Orient != 0 {
		ifd0 = append(ifd0, shotEntry{tag: 0x0112, typ: 3, count: 1, val: bo.AppendUint16(nil, f.Orient)})
	}
	if f.Exposure != [2]uint32{} {
		exif = append(exif, rat(0x829A, f.Exposure))
	}
	if f.FNumber != [2]uint32{} {
		exif = append(exif, rat(0x829D, f.FNumber))
	}
	if f.ISO != 0 {
		e := shotEntry{tag: 0x8827, typ: 3, count: 1}
		if f.ISOLong {
			e.typ, e.val = 4, bo.AppendUint32(nil, uint32(f.ISO)) //nolint:gosec // тестовые значения
		} else {
			e.val = bo.AppendUint16(nil, uint16(f.ISO)) //nolint:gosec // тестовые значения
		}
		exif = append(exif, e)
	}
	if f.Orig != "" {
		exif = append(exif, str(0x9003, f.Orig))
	}
	if f.Focal != [2]uint32{} {
		exif = append(exif, rat(0x920A, f.Focal))
	}
	if f.Lens != "" {
		exif = append(exif, str(0xA434, f.Lens))
	}
	return ifd0, exif
}
