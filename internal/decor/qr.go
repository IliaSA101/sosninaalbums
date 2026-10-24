package decor

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	qrcode "github.com/skip2/go-qrcode"
)

// QR-код по ссылке (спека 100, п. 2): Go кодирует и рисует, окно показывает путь SVG. Ссылку
// программа не открывает и в сеть не ходит; в сообщениях об ошибках самой ссылки нет.

// Параметры кода (decisions 100-5).
const (
	QRMaxLen      = 500 // знаков ссылки: длиннее — версия выше 10, модуль на листе мелкий
	QRQuiet       = 4   // белое поле вокруг, модулей (стандарт)
	QRMinCodeMM   = 20  // сторона кода без поля не меньше
	QRMinModuleMM = 0.5 // модуль не мельче
	qrNameMax     = 60  // имя плитки, знаков (как NAME_MAX окна)
)

// Ошибки — текст строки под полем.
var (
	ErrQREmpty = errors.New("введите ссылку")
	ErrQRLong  = fmt.Errorf("ссылка длиннее %d знаков", QRMaxLen)
	ErrQRChars = errors.New("в ссылке не должно быть переносов строк и управляющих знаков")
	errQRMake  = errors.New("не удалось построить код")
)

// QR — код ссылки: текст в коде (после дополнения схемы) и тёмные модули без поля.
type QR struct {
	Text    string
	Modules [][]bool
}

// reScheme — схема только в начале строки: «://» дальше (в параметре) схемой не считается.
var reScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// NormalizeLink — ссылка для кода: пробелы по краям убираются; без схемы дополняется «https://»
// (телефон читает «site.ru» как текст, а не ссылку); «mailto:» и «tel:» остаются как есть.
func NormalizeLink(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrQREmpty
	}
	if utf8.RuneCountInString(s) > QRMaxLen {
		return "", ErrQRLong
	}
	// U+2028/U+2029 — разделители строк вне Cc: в коде они ломают разбор ссылки телефоном.
	if strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) }) {
		return "", ErrQRChars
	}
	low := strings.ToLower(s)
	if !reScheme.MatchString(s) && !strings.HasPrefix(low, "mailto:") && !strings.HasPrefix(low, "tel:") {
		s = "https://" + s
	}
	return s, nil
}

// EncodeQR — код ссылки: коррекция M (15 %), версию библиотека выбирает по длине.
func EncodeQR(link string) (QR, error) {
	text, err := NormalizeLink(link)
	if err != nil {
		return QR{}, err
	}
	c, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return QR{}, errQRMake
	}
	c.DisableBorder = true
	return QR{Text: text, Modules: c.Bitmap()}, nil
}

// Side — сторона кода с полем, модулей.
func (q QR) Side() int { return len(q.Modules) + 2*QRQuiet }

// MinMM — наименьшая сторона печати с полем, мм, вверх до целого: и код без поля не меньше
// QRMinCodeMM, и модуль не мельче QRMinModuleMM.
func (q QR) MinMM() float64 {
	n := len(q.Modules)
	if n == 0 {
		return 0
	}
	side := float64(q.Side())
	return math.Ceil(max(QRMinCodeMM*side/float64(n), QRMinModuleMM*side))
}

// SVGPath — d тёмных модулей рядами («M x y h len v1 h-len z»); единица — модуль, начало — угол
// поля, так что viewBox="0 0 Side Side".
func (q QR) SVGPath() string {
	var b strings.Builder
	for y, row := range q.Modules {
		for x := 0; x < len(row); {
			if !row[x] {
				x++
				continue
			}
			end := x
			for end < len(row) && row[end] {
				end++
			}
			n := strconv.Itoa(end - x)
			b.WriteString("M" + strconv.Itoa(x+QRQuiet) + " " + strconv.Itoa(y+QRQuiet) + "h" + n + "v1h-" + n + "z")
			x = end
		}
	}
	return b.String()
}

// PNG — код с полем, чёрный на белом (два цвета): модуль — целое число px, чтобы края были
// резкими; сторона не больше MaxStickerSide.
func (q QR) PNG() ([]byte, error) {
	if len(q.Modules) == 0 {
		return nil, errQRMake
	}
	side := q.Side()
	mod := MaxStickerSide / side
	w := mod * side
	img := image.NewPaletted(image.Rect(0, 0, w, w), color.Palette{color.White, color.Black})
	for y, row := range q.Modules {
		for x, dark := range row {
			if !dark {
				continue
			}
			for py := (y + QRQuiet) * mod; py < (y+QRQuiet+1)*mod; py++ {
				line := img.Pix[py*img.Stride:]
				for px := (x + QRQuiet) * mod; px < (x+QRQuiet+1)*mod; px++ {
					line[px] = 1
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&buf, img); err != nil {
		return nil, errQRMake
	}
	return buf.Bytes(), nil
}

// qrName — имя плитки: «QR · » и адрес без схемы, всё не длиннее qrNameMax знаков, обрезанное — с «…».
func qrName(text string) string {
	addr := text
	if _, rest, ok := strings.Cut(text, "://"); ok {
		addr = rest
	}
	r := []rune("QR · " + addr)
	if len(r) > qrNameMax {
		r = append(r[:qrNameMax-1], '…')
	}
	return string(r)
}
