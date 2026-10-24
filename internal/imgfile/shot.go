package imgfile

import (
	"io"
	"strings"
	"unicode"
)

// Пределы разбора данных съёмки недоверенного файла.
const (
	shotMaxText = 64 // знаков (рун) в строке камеры или объектива
	exifRatLen  = 8  // RATIONAL: два uint32
	shotMaxISO  = 1 << 30
)

// Типы тегов TIFF, которые читает разбор съёмки.
const (
	exifTypeShort    = 3
	exifTypeLong     = 4
	exifTypeRational = 5
)

// Shot — данные съёмки из EXIF. Нулевое поле — тега нет или мусор.
type Shot struct {
	Make, Model, Lens string    // 0x010F, 0x0110 (IFD0), 0xA434 LensModel (Exif IFD)
	Exposure          [2]uint32 // 0x829A ExposureTime: числитель, знаменатель ≠ 0
	FNumber           [2]uint32 // 0x829D
	Focal             [2]uint32 // 0x920A FocalLength
	ISO               int       // 0x8827 PhotographicSensitivity > 0
	Taken             int64     // как Head.Taken
}

// Empty — ни одного поля съёмки нет (время тоже).
func (s Shot) Empty() bool { return s == Shot{} }

// ReadShot читает данные съёмки из заголовка JPEG. Не JPEG, нет EXIF, мусор — Shot{}.
func ReadShot(r io.Reader) Shot { return parseShot(jpegSegments(r).exif) }

// HeaderShot — Header и ReadShot за один проход по сегментам (карточке фото нужны и ориентация,
// и съёмка: заголовок не читается дважды).
func HeaderShot(r io.Reader) (Head, Shot) {
	s := jpegSegments(r)
	return headOf(s), parseShot(s.exif)
}

// Rational — значение RATIONAL (тип 5): ровно одно, целиком внутри буфера, знаменатель ≠ 0.
// Смещение — uint32, складывается в uint64 до сравнения с буфером.
func (e exifEntry) Rational() (num, den uint32, ok bool) {
	if e.Type != exifTypeRational || e.Count != 1 {
		return 0, 0, false
	}
	off := uint64(e.Offset())
	if off+exifRatLen > uint64(len(e.t)) {
		return 0, 0, false
	}
	num, den = e.bo.Uint32(e.t[off:]), e.bo.Uint32(e.t[off+4:])
	return num, den, den != 0
}

// Uint — одно число из самой записи: SHORT (тип 3) или LONG (тип 4); иначе ok = false.
func (e exifEntry) Uint() (v uint32, ok bool) {
	if e.Count != 1 {
		return 0, false
	}
	switch e.Type {
	case exifTypeShort:
		return uint32(e.bo.Uint16(e.field)), true
	case exifTypeLong:
		return e.bo.Uint32(e.field), true
	}
	return 0, false
}

// parseShot разбирает TIFF-часть APP1. Тег берётся первый на своём уровне (как ориентация);
// строки чистятся и обрезаются. nil и мусор дают Shot{}.
func parseShot(t []byte) Shot {
	var s Shot
	if len(t) == 0 {
		return s
	}
	seen := map[[2]int]bool{}
	first := func(level int, e exifEntry) bool {
		k := [2]int{level, int(e.Tag)}
		if seen[k] {
			return false
		}
		seen[k] = true
		return true
	}
	rat := func(e exifEntry) [2]uint32 {
		if n, d, ok := e.Rational(); ok {
			return [2]uint32{n, d}
		}
		return [2]uint32{}
	}
	exifWalk(t, func(level int, e exifEntry) {
		switch {
		case level == exifLevelIFD0 && e.Tag == 0x010F && first(level, e):
			s.Make = cleanTag(e.ASCII())
		case level == exifLevelIFD0 && e.Tag == 0x0110 && first(level, e):
			s.Model = cleanTag(e.ASCII())
		case level != exifLevelExif || !first(level, e):
		case e.Tag == 0xA434:
			s.Lens = cleanTag(e.ASCII())
		case e.Tag == 0x829A:
			s.Exposure = rat(e)
		case e.Tag == 0x829D:
			s.FNumber = rat(e)
		case e.Tag == 0x920A:
			s.Focal = rat(e)
		case e.Tag == 0x8827:
			if v, ok := e.Uint(); ok && v > 0 && v <= shotMaxISO {
				s.ISO = int(v)
			}
		}
	})
	s.Taken = exifTaken(t)
	return s
}

// cleanTag — строка тега для показа: срез по первому NUL, не-UTF-8 выброшен, управляющие и
// форматирующие знаки (в том числе смена направления текста) — пробел, пробелы по краям и подряд
// схлопнуты, не больше shotMaxText рун.
func cleanTag(b []byte) string {
	if i := strings.IndexByte(string(b), 0); i >= 0 {
		b = b[:i]
	}
	var out strings.Builder
	space := true // пробел в начале не пишем
	n := 0
	for _, r := range strings.ToValidUTF8(string(b), "") {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.IsSpace(r) {
			r = ' '
		}
		if r == ' ' && space {
			continue
		}
		space = r == ' '
		if n == shotMaxText {
			break
		}
		out.WriteRune(r)
		n++
	}
	return strings.TrimRight(out.String(), " ")
}
