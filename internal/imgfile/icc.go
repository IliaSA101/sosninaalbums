package imgfile

import (
	"encoding/binary"
	"io"
	"math"
	"unicode"
	"unicode/utf16"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
)

// Color — класс цветового профиля фото. От него зависит, пересчитывать ли цвета
// при декоде (Adobe RGB, Display P3 и другие матричные профили → sRGB) или только предупредить.
type Color string

const (
	ColorNone  Color = ""      // профиля нет — считаем sRGB
	ColorSRGB  Color = "srgb"  // sRGB и ч/б: печатается как есть
	ColorAdobe Color = "adobe" // Adobe RGB (1998): при декоде пересчитывается в sRGB
	ColorP3    Color = "p3"    // колоранты Display P3 (телефоны, любое имя): при декоде пересчитывается в sRGB
	ColorICC   Color = "icc"   // прочий матричный RGB-профиль (ProPhoto, Rec. 709, линейный sRGB…): так же
	ColorOther Color = "other" // всё остальное (CMYK, LUT-профили, мусор): только предупреждение
)

// Пределы для недоверенного файла: чтение заголовка не должно тратить память и время зря.
const (
	iccMaxSegments  = 64      // сегментов JPEG до SOS
	iccMaxBytes     = 1 << 20 // профиль целиком (сумма кусков)
	iccNameRunes    = 64      // длина имени профиля
	iccUnknownName  = "неизвестный профиль"
	ICCAdobeEXIF    = "Adobe RGB (по EXIF)"
	iccColorantTol  = 0.003 // допуск колорантов, XYZ D50
	iccGammaTol     = 0.02
	iccAdobeGamma   = 2.2
	iccHeaderLen    = 132 // заголовок профиля + число тегов
	iccInteropIndex = 0x0001
)

// Колорант-матрицы в XYZ D50: rX rY rZ gX gY gZ bX bY bZ.
var (
	srgbXYZ  = [9]float64{0.4361, 0.2225, 0.0139, 0.3851, 0.7169, 0.0971, 0.1431, 0.0606, 0.7141}
	adobeXYZ = [9]float64{0.6097, 0.3111, 0.0195, 0.2053, 0.6257, 0.0609, 0.1492, 0.0632, 0.7446}
	p3XYZ    = [9]float64{0.5151, 0.2412, -0.0011, 0.2920, 0.6922, 0.0419, 0.1571, 0.0666, 0.7841}
)

// ReadColor определяет цветовой профиль по заголовку: JPEG — ICC, запасной путь EXIF Interop; PNG — iCCP;
// WebP — ICCP при флаге в VP8X (формат по подписи, не по расширению: 117). Ошибка чтения, мусор и
// файл без профиля дают ("", ""): считаем sRGB.
func ReadColor(path string) (Color, string) {
	f, err := OpenRegular(path)
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

func trimNUL(b []byte) []byte {
	for len(b) > 0 && b[len(b)-1] == 0 {
		b = b[:len(b)-1]
	}
	return b
}

// classifyICC определяет класс и имя профиля (см. classify).
func classifyICC(p []byte) (Color, string) {
	c, name, _ := classify(p)
	return c, name
}

// classify определяет класс и имя профиля и, для классов с пересчётом, таблицы пересчёта в sRGB.
// Чистая функция: p — профиль целиком (или первый кусок), любой мусор даёт ColorOther без паники.
// Профиль распознаётся по содержимому, не по имени: у Xiaomi Display P3 зовётся
// «sRGB EOTF with DCI-P3 Color Gamut» (спека 119).
func classify(p []byte) (Color, string, *colorspace.Xform) {
	if len(p) < iccHeaderLen || string(p[36:40]) != "acsp" {
		return ColorOther, iccUnknownName, nil
	}
	tags := colorspace.ICCTags(p)
	name := iccName(tags["desc"])
	other := func() (Color, string, *colorspace.Xform) {
		if name == "" {
			name = iccUnknownName
		}
		return ColorOther, name, nil
	}
	switch string(p[16:20]) {
	case "GRAY":
		return ColorSRGB, name, nil // ч/б печатается как есть
	case "RGB ":
	default:
		return other() // CMYK, Lab…
	}
	var xyz [9]float64
	for i, s := range []string{"rXYZ", "gXYZ", "bXYZ"} {
		t := tags[s]
		if len(t) < 20 || string(t[:4]) != "XYZ " {
			return other() // чистые LUT-профили
		}
		for k := range 3 {
			xyz[i*3+k] = colorspace.S15(t[8+4*k:])
		}
	}
	if near(xyz, adobeXYZ) {
		g := iccGamma(tags["rTRC"])
		if math.Abs(g-iccAdobeGamma) <= iccGammaTol &&
			iccGamma(tags["gTRC"]) == g && iccGamma(tags["bTRC"]) == g {
			return ColorAdobe, name, colorspace.Adobe
		}
	}
	x, err := colorspace.ParseICC(p)
	switch {
	case err != nil && near(xyz, srgbXYZ):
		return ColorSRGB, name, nil // кривую не проверяем: битая кривая у sRGB не мешала и раньше
	case err != nil:
		return other() // LUT, битые теги, колоранты не дают белый D50
	case near(xyz, srgbXYZ) && x.SRGBCurves():
		// Колоранты sRGB и кривая sRGB: v2/v4 и профили с дрейфом колорантов. Матрицу не сверяем (119-14):
		// решётка по пикселям теряла настоящие sRGB от сдвига колорантов на 0,0002, и они получали «ICC».
		return ColorSRGB, name, nil
	case near(xyz, p3XYZ):
		return ColorP3, name, x
	}
	return ColorICC, name, x
}

func near(a, b [9]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > iccColorantTol {
			return false
		}
	}
	return true
}

// iccGamma возвращает гамму кривой, заданной одним числом (curv count=1 или para типа 0), иначе 0.
func iccGamma(t []byte) float64 {
	if len(t) >= 14 && string(t[:4]) == "curv" && binary.BigEndian.Uint32(t[8:]) == 1 {
		return float64(binary.BigEndian.Uint16(t[12:])) / 256
	}
	if len(t) >= 16 && string(t[:4]) == "para" && binary.BigEndian.Uint16(t[8:]) == 0 {
		return colorspace.S15(t[12:])
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
