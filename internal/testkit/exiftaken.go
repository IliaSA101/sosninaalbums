package testkit

import "encoding/binary"

// ExifTaken — APP1 с EXIF, где в Exif IFD лежат DateTimeOriginal (0x9003, orig != "") и
// DateTimeDigitized (0x9004, digi != ""). Строки ложатся как есть + NUL, длина записи —
// len+1: так тест подсовывает и мусор вместо «YYYY:MM:DD HH:MM:SS».
func ExifTaken(bigEndian bool, orig, digi string) []byte {
	var bo order = binary.LittleEndian
	t := []byte("II*\x00")
	if bigEndian {
		bo, t = binary.BigEndian, []byte("MM\x00*")
	}
	t = bo.AppendUint32(t, 8)
	var strs []string
	var tags []uint16
	for i, s := range []string{orig, digi} {
		if s != "" {
			strs = append(strs, s)
			tags = append(tags, uint16(0x9003+i)) //nolint:gosec // 0x9003, 0x9004
		}
	}
	exifOff := uint32(8 + 2 + 12 + 4)
	data := exifOff + uint32(2+12*len(strs)+4) //nolint:gosec // единицы
	// IFD0: одна запись — указатель на Exif IFD.
	t = bo.AppendUint16(t, 1)
	t = bo.AppendUint16(t, 0x8769)
	t = bo.AppendUint16(t, 4)
	t = bo.AppendUint32(t, 1)
	t = bo.AppendUint32(t, exifOff)
	t = bo.AppendUint32(t, 0)
	t = bo.AppendUint16(t, uint16(len(strs))) //nolint:gosec // 0…2
	for i, s := range strs {
		t = bo.AppendUint16(t, tags[i])
		t = bo.AppendUint16(t, 2)
		t = bo.AppendUint32(t, uint32(len(s)+1)) //nolint:gosec // короткие строки
		t = bo.AppendUint32(t, data)
		data += uint32(len(s) + 1) //nolint:gosec // короткие строки
	}
	t = bo.AppendUint32(t, 0)
	for _, s := range strs {
		t = append(append(t, s...), 0)
	}
	return Segment(0xE1, append([]byte("Exif\x00\x00"), t...))
}
