package export

import (
	"fmt"
	"math"
	"math/big"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Оценка объёма выгрузки (035 U22): перенос exportopts.ts из интерфейса (0.37, одна правда об объёме).
// Формулы и округления те же, что были в TS, байт в байт по тексту результата.

// PrintDPI — выгрузка для типографии всегда 300 dpi (ExportOptions для типографии ставит то же).
const PrintDPI = 300

// bytesPerPx — байт на пиксель по качеству. ГИПОТЕЗА по типичным JPEG фотокамер: сверить на настоящих фото при QA.
func bytesPerPx(quality int) float64 {
	switch quality {
	case 100:
		return 0.9
	case 80:
		return 0.2
	}
	return 0.45 // 95 и любое другое
}

// sizeQualities — пункты списка «Качество» окна (порядок и числа — как QUALITIES в интерфейсе).
var sizeQualities = [...]int{100, 95, 80}

// pxOf — мм в пиксели при dpi; Math.round в JS округляет половину вверх, math.Round — от нуля: для неотрицательных то же.
func pxOf(mm float64, dpi int) float64 {
	return math.Round(mm / 25.4 * float64(dpi))
}

// EstimateBytes — примерный размер одного файла: разворот (или страница при pages) по числу пикселей.
// dpi <= 0 — из настроек альбома. Лист без шва — один файл; у разворота «по страницам» — половина.
func EstimateBytes(s model.Settings, dpi, quality int, pages bool) float64 {
	d := dpi
	if d <= 0 {
		d = s.DPI
	}
	pg := s.Page()                  // размер листа — через Page, не PageW (страж TestPageWReaders)
	w, h, mid := pg.W*2, pg.H, pg.W // как newSheetSize: разворот 2·pageW, шов pageW
	if s.Sheet {
		w, mid = pg.W, 0
	}
	var wp float64
	if mid > 0 {
		k := 2.0
		if pages {
			k = 1
		}
		wp = k * pxOf(mid, d)
	} else {
		wp = pxOf(w, d)
	}
	return wp * pxOf(h, d) * bytesPerPx(quality)
}

// FormatSize — «≈ 25 МБ»: до 10 МБ один знак с запятой, меньше 0,1 МБ — «< 0,1 МБ».
func FormatSize(bytes float64) string {
	mb := bytes / (1024 * 1024)
	if mb < 0.1 {
		return "< 0,1 МБ"
	}
	if mb < 10 {
		return "≈ " + tenths(mb) + " МБ"
	}
	return "≈ " + strconv.FormatFloat(math.Floor(mb+0.5), 'f', 0, 64) + " МБ"
}

// tenths — как toFixed(1) в JS с запятой: точное значение числа, при равной близости берётся большее
// (strconv округляет такую половину к чётному — 2,25 дало бы «2,2» вместо «2,3»).
func tenths(mb float64) string {
	r := new(big.Rat).SetFloat64(mb)
	r.Mul(r, big.NewRat(10, 1))
	r.Add(r, big.NewRat(1, 2))
	n := new(big.Int).Quo(r.Num(), r.Denom()) // mb > 0: деление к нулю = floor
	i := n.Int64()
	return fmt.Sprintf("%d,%d", i/10, i%10)
}

// SizeIn — что выгружается: выбор окна «Экспорт». Настройки альбома и число разворотов берёт вызывающий из проекта.
type SizeIn struct {
	Scope    string `json:"scope"`    // "all" | "current" | "range"
	DPI      int    `json:"dpi"`      // 0 — из настроек альбома
	Quality  int    `json:"quality"`  // 100 | 95 | 80
	Pages    bool   `json:"pages"`    // по страницам
	Units    []int  `json:"units"`    // для "range": страница каждой цели диапазона (0 — разворот целиком, 1/2 — левая/правая)
	BadRange bool   `json:"badRange"` // диапазон неверен: итога «всего» нет
	Cover    bool   `json:"cover"`    // обложка уйдёт в выгрузку (в том числе «только обложка»); считается как один разворот
	Only     bool   `json:"only"`     // только обложка
	Print    bool   `json:"print"`    // для типографии (047, 081): все развороты, 300 dpi, разворотами; обложка — как в Cover
}

// SizeQ — подпись объёма для одного пункта списка «Качество».
type SizeQ struct {
	Quality int    `json:"quality"`
	Text    string `json:"text"` // «≈ 25 МБ»
}

// SizeOut — готовые тексты окна.
type SizeOut struct {
	Summary string  `json:"summary"` // «≈ X на разворот, всего ≈ Y»; «всего» — если файлов больше одного
	Per     []SizeQ `json:"per"`     // объём файла по каждому качеству при выбранных dpi и виде (до замены для типографии)
}

// SizeSummary — итог по объёму для s и числа разворотов spreads. Слово «на …» — по листу и виду:
// лист, страницу или разворот.
func SizeSummary(s model.Settings, spreads int, in SizeIn) SizeOut {
	out := SizeOut{Per: make([]SizeQ, 0, len(sizeQualities))}
	for _, q := range sizeQualities {
		out.Per = append(out.Per, SizeQ{q, FormatSize(EstimateBytes(s, in.DPI, q, in.Pages))})
	}
	if in.Print {
		in.DPI, in.Pages, in.Scope, in.Only = PrintDPI, false, "all", false
	}
	one := EstimateBytes(s, in.DPI, in.Quality, in.Pages)
	spread := EstimateBytes(s, in.DPI, in.Quality, false)
	page := EstimateBytes(s, in.DPI, in.Quality, true)
	total, ok := 0.0, true
	if in.Cover {
		total = spread
	}
	if !in.Only {
		switch {
		case in.Scope == "all":
			total += float64(spreads) * spread
		case in.Scope == "current":
			total += spread
		case !in.BadRange:
			sum := 0.0
			for _, p := range in.Units {
				if p == 0 {
					sum += spread
				} else {
					sum += page
				}
			}
			total += sum
		default:
			ok = false
		}
	}
	unit := "разворот"
	switch {
	case s.Sheet:
		unit = "лист"
	case in.Pages:
		unit = "страницу"
	}
	out.Summary = FormatSize(one) + " на " + unit
	if ok && total > one {
		out.Summary += ", всего " + FormatSize(total)
	}
	return out
}
