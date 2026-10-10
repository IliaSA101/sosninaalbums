package imgfile

import (
	"encoding/binary"
	"io"
	"math"
	"os"
	"unicode"
	"unicode/utf16"
)

// Color — класс цветового профиля фото. От него зависит, пересчитывать ли цвета
// при декоде (Adobe RGB → sRGB) или только предупредить.
type Color string

const (
	ColorNone  Color = ""      // профиля нет — считаем sRGB
	ColorSRGB  Color = "srgb"  // sRGB и ч/б: печатается как есть
	ColorAdobe Color = "adobe" // Adobe RGB (1998): при декоде пересчитывается в sRGB
	ColorOther Color = "other" // всё остальное: только предупреждение
)

// Пределы для недоверенного файла: чтение заголовка не должно тратить память и время зря.
const (
	iccMaxSegments  = 64      // сегментов JPEG до SOS
	iccMaxBytes     = 1 << 20 // профиль целиком (сумма кусков)
	iccMaxTags      = 100     // записей в таблице тегов
	iccNameRunes    = 64      // длина имени профиля
	iccUnknownName  = "неизвестный профиль"
	ICCAdobeEXIF    = "Adobe RGB (по EXIF)"
	iccColorantTol  = 0.003 // допуск колорантов, XYZ D50
	iccGammaTol     = 0.02
	iccAdobeGamma   = 2.2
	iccHeaderLen    = 132 // заголовок профиля + число тегов
	iccTagEntryLen  = 12
	iccInteropIndex = 0x0001
)

// Колорант-матрицы в XYZ D50: rX rY rZ gX gY gZ bX bY bZ.
var (
	srgbXYZ  = [9]float64{0.4361, 0.2225, 0.0139, 0.3851, 0.7169, 0.0971, 0.1431, 0.0606, 0.7141}
	adobeXYZ = [9]float64{0.6097, 0.3111, 0.0195, 0.2053, 0.6257, 0.0609, 0.1492, 0.0632, 0.7446}
)

// ReadColor определяет цветовой профиль JPEG по заголовку (ICC, запасной путь — EXIF Interop).
// Ошибка чтения, не-JPEG и PNG дают ("", ""): считаем sRGB.
func ReadColor(path string) (Color, string) {
	f, err := os.Open(path)
	if err != nil {
		return ColorNone, ""
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	return readColor(f)
}

// readColor — ReadColor по уже открытому потоку (см. Header).
func readColor(r io.Reader) (Color, string) {
	h := Header(r)
	return h.Color, h.Profile
}

// joinICC склеивает куски 1..count; если какого-то нет (или лимит съел) — берёт один первый:
// нужные теги за его пределами classifyICC отнесёт к other.
func joinICC(chunks map[int][]byte, count int) []byte {
	if len(chunks) == 0 {
		return nil
	}
	if count > 1 && len(chunks) == count {
		var out []byte
		for i := 1; i <= count; i++ {
			out = append(out, chunks[i]...)
		}
		return out
	}
	return chunks[1]
}

// exifInterop идёт IFD0 → Exif IFD (0x8769) → Interop IFD (0xA005) → InteropIndex.
// Ровно три уровня, без рекурсии; каждое смещение проверяется на границы.
func exifInterop(t []byte) string {
	if len(t) < 8 {
		return ""
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return ""
	}
	// ifd вызывает f для каждой записи; смещение — uint32, сравнивается до перевода в int.
	ifd := func(off uint32, f func(tag uint16, e []byte)) {
		if off < 8 || uint64(off)+2 > uint64(len(t)) {
			return
		}
		n := int(bo.Uint16(t[off:]))
		for i := range n {
			e := int(off) + 2 + i*12
			if e+12 > len(t) {
				return
			}
			f(bo.Uint16(t[e:]), t[e:e+12])
		}
	}
	var exifOff, interOff uint32
	ifd(bo.Uint32(t[4:]), func(tag uint16, e []byte) {
		if tag == 0x8769 {
			exifOff = bo.Uint32(e[8:])
		}
	})
	ifd(exifOff, func(tag uint16, e []byte) {
		if tag == 0xA005 {
			interOff = bo.Uint32(e[8:])
		}
	})
	var res string
	ifd(interOff, func(tag uint16, e []byte) {
		// ASCII (тип 2), до 4 байт — значение лежит прямо в записи
		if tag == iccInteropIndex && bo.Uint16(e[2:]) == 2 && bo.Uint32(e[4:]) <= 4 {
			res = string(trimNUL(e[8:12]))
		}
	})
	return res
}

func trimNUL(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

// classifyICC определяет класс и имя профиля. Чистая функция: p — профиль целиком
// (или первый кусок), любой мусор даёт ColorOther без паники.
func classifyICC(p []byte) (Color, string) {
	if len(p) < iccHeaderLen || string(p[36:40]) != "acsp" {
		return ColorOther, iccUnknownName
	}
	tags := iccTags(p)
	name := iccName(tags["desc"])
	other := func() (Color, string) {
		if name == "" {
			name = iccUnknownName
		}
		return ColorOther, name
	}
	switch string(p[16:20]) {
	case "GRAY":
		return ColorSRGB, name // ч/б печатается как есть
	case "RGB ":
	default:
		return other() // CMYK, Lab…
	}
	var xyz [9]float64
	for i, s := range []string{"rXYZ", "gXYZ", "bXYZ"} {
		t := tags[s]
		if len(t) < 20 || string(t[:4]) != "XYZ " {
			return other()
		}
		for k := range 3 {
			xyz[i*3+k] = s15(t[8+4*k:])
		}
	}
	if near(xyz, srgbXYZ) { // кривую не проверяем: у sRGB она разная в v2/v4
		return ColorSRGB, name
	}
	if near(xyz, adobeXYZ) {
		g := iccGamma(tags["rTRC"])
		if math.Abs(g-iccAdobeGamma) <= iccGammaTol &&
			iccGamma(tags["gTRC"]) == g && iccGamma(tags["bTRC"]) == g {
			return ColorAdobe, name
		}
	}
	return other()
}

func near(a, b [9]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > iccColorantTol {
			return false
		}
	}
	return true
}

// iccTags собирает таблицу тегов. Теги, вышедшие за длину p, пропускаются.
func iccTags(p []byte) map[string][]byte {
	tags := map[string][]byte{}
	n := min(binary.BigEndian.Uint32(p[128:]), iccMaxTags)
	for i := range int(n) {
		e := iccHeaderLen + iccTagEntryLen*i
		if e+iccTagEntryLen > len(p) {
			break
		}
		off, sz := uint64(binary.BigEndian.Uint32(p[e+4:])), uint64(binary.BigEndian.Uint32(p[e+8:]))
		if off+sz > uint64(len(p)) {
			continue
		}
		sig := string(p[e : e+4])
		if _, dup := tags[sig]; !dup {
			tags[sig] = p[off : off+sz]
		}
	}
	return tags
}

// iccGamma возвращает гамму кривой, заданной одним числом (curv count=1 или para типа 0), иначе 0.
func iccGamma(t []byte) float64 {
	if len(t) >= 14 && string(t[:4]) == "curv" && binary.BigEndian.Uint32(t[8:]) == 1 {
		return float64(binary.BigEndian.Uint16(t[12:])) / 256
	}
	if len(t) >= 16 && string(t[:4]) == "para" && binary.BigEndian.Uint16(t[8:]) == 0 {
		return s15(t[12:])
	}
	return 0
}

// iccName читает имя из тега desc: v2 — ASCII, v4 (mluc) — первая запись UTF-16BE.
// Только печатные символы, не больше iccNameRunes.
func iccName(t []byte) string {
	var runes []rune
	switch {
	case len(t) >= 12 && string(t[:4]) == "desc":
		c := uint64(binary.BigEndian.Uint32(t[8:]))
		if c == 0 || 12+c > uint64(len(t)) {
			return ""
		}
		for _, b := range t[12 : 12+c] {
			runes = append(runes, rune(b))
			if len(runes) >= iccNameRunes*2 {
				break
			}
		}
	case len(t) >= 28 && string(t[:4]) == "mluc" && binary.BigEndian.Uint32(t[8:]) >= 1:
		l, o := uint64(binary.BigEndian.Uint32(t[20:])), uint64(binary.BigEndian.Uint32(t[24:]))
		if o+l > uint64(len(t)) {
			return ""
		}
		l = min(l, iccNameRunes*4)
		u := make([]uint16, 0, l/2)
		for i := o; i+1 < o+l; i += 2 {
			u = append(u, binary.BigEndian.Uint16(t[i:]))
		}
		runes = utf16.Decode(u)
	}
	out := make([]rune, 0, iccNameRunes)
	for _, r := range runes {
		if r == 0 {
			break // конец C-строки
		}
		if unicode.IsPrint(r) {
			out = append(out, r)
		}
		if len(out) >= iccNameRunes {
			break
		}
	}
	return string(out)
}

// s15 читает s15Fixed16 (знаковое с 16 дробными битами) из первых 4 байт b.
func s15(b []byte) float64 {
	return float64(int32(binary.BigEndian.Uint32(b))) / 65536 //nolint:gosec // битовое переинтерпретирование знака
}
