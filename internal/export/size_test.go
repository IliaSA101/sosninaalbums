package export

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// album300 — альбом 2×300 мм, 300 dpi (то же, что const album в exportopts.test.ts до переноса).
var album300 = model.Settings{PageW: 300, PageH: 300, DPI: 300}

const mb = 1024 * 1024

func TestEstimateBytes(t *testing.T) {
	spread := EstimateBytes(album300, 300, 95, false)
	if want := 7086.0 * 3543 * 0.45; math.Abs(spread-want) > 0.5 { // 3543 px страница: 7086 × 3543
		t.Errorf("разворот 300 dpi, 95: %v, ждали %v", spread, want)
	}
	if got := EstimateBytes(album300, 300, 95, true); math.Abs(got-spread/2) > 1000 {
		t.Errorf("страница должна быть вдвое меньше разворота: %v и %v", got, spread)
	}
	if EstimateBytes(album300, 0, 80, false) != EstimateBytes(album300, 300, 80, false) {
		t.Error("dpi 0 берёт из альбома")
	}
	a150 := album300
	a150.DPI = 150
	if EstimateBytes(a150, 0, 80, false) != EstimateBytes(album300, 150, 80, false) {
		t.Error("dpi альбома 150")
	}
	q100, q95, q80 := EstimateBytes(album300, 300, 100, false), spread, EstimateBytes(album300, 300, 80, false)
	if !(q100 > q95 && q95 > q80) {
		t.Errorf("качество монотонно: %v %v %v", q100, q95, q80)
	}
	if EstimateBytes(album300, 300, 90, false) != q95 {
		t.Error("неизвестное качество считается как 95")
	}
	if r := EstimateBytes(album300, 150, 95, false) / spread; math.Abs(r-0.25) > 0.005 {
		t.Errorf("dpi квадратично: %v", r)
	}
	// лист без шва — один файл, «по страницам» его не делит
	sheet := model.Settings{PageW: 200, PageH: 150, DPI: 300, Sheet: true}
	if EstimateBytes(sheet, 300, 95, true) != EstimateBytes(sheet, 300, 95, false) {
		t.Error("лист не делится на страницы")
	}
	// пустой альбом (размер 0): не падаем, объём 0
	if got := EstimateBytes(model.Settings{}, 0, 95, false); got != 0 {
		t.Errorf("нулевые настройки: %v", got)
	}
}

func TestFormatSize(t *testing.T) {
	for _, c := range []struct {
		bytes float64
		want  string
	}{
		{2.5 * mb, "≈ 2,5 МБ"},
		{24.6 * mb, "≈ 25 МБ"},
		{1000, "< 0,1 МБ"},
		{0, "< 0,1 МБ"},
		{2.25 * mb, "≈ 2,3 МБ"}, // ровно половина: как toFixed в JS — вверх
		{9.95 * mb, "≈ 9,9 МБ"}, // 9,95 в двоичном виде чуть меньше
		{9.96 * mb, "≈ 10,0 МБ"},
		{10 * mb, "≈ 10 МБ"},
		{99.5 * mb, "≈ 100 МБ"},
	} {
		if got := FormatSize(c.bytes); got != c.want {
			t.Errorf("FormatSize(%v) = %q, ждали %q", c.bytes, got, c.want)
		}
	}
}

// Итог по объёму: случаи exportopts.test.ts («итог по объёму», «для типографии») одной таблицей.
func TestSizeSummaryCases(t *testing.T) {
	one := EstimateBytes(album300, 300, 95, false)
	base := SizeIn{Scope: "all", DPI: 300, Quality: 95}
	with := func(f func(*SizeIn)) SizeIn { in := base; f(&in); return in }
	for _, c := range []struct {
		name string
		in   SizeIn
		want string
	}{
		{"«Все» — на разворот и всего", base, FormatSize(one) + " на разворот, всего " + FormatSize(4*one)},
		{"обложка — как один разворот", with(func(i *SizeIn) { i.Cover = true }), FormatSize(one) + " на разворот, всего " + FormatSize(5*one)},
		{"только обложка", with(func(i *SizeIn) { i.Cover, i.Only = true, true }), FormatSize(one) + " на разворот"},
		{"«Текущий» без обложки — без «всего»", with(func(i *SizeIn) { i.Scope = "current" }), FormatSize(one) + " на разворот"},
		{"диапазон: разворот целиком и страница", with(func(i *SizeIn) { i.Scope, i.Units = "range", []int{0, 2} }), FormatSize(one) + " на разворот, всего " + FormatSize(1.5*one)},
		{"неверный диапазон — без итога", with(func(i *SizeIn) { i.Scope, i.BadRange = "range", true }), FormatSize(one) + " на разворот"},
		{"по страницам — размер страницы", with(func(i *SizeIn) { i.Pages = true }), FormatSize(one/2) + " на страницу, всего " + FormatSize(4*one)},
		{"типография: все развороты и обложка при 300 dpi, как ни выбрано в окне",
			SizeIn{Scope: "current", DPI: 150, Quality: 95, Pages: true, Cover: true, Only: true, Print: true},
			FormatSize(one) + " на разворот, всего " + FormatSize(5*one)},
		{"нет разворотов — без «всего»", with(func(i *SizeIn) { i.Scope = "all" }), ""}, // заполняется ниже
	} {
		if c.name == "нет разворотов — без «всего»" {
			c.want = FormatSize(one) + " на разворот"
			if got := SizeSummary(album300, 0, c.in).Summary; got != c.want {
				t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
			}
			continue
		}
		if got := SizeSummary(album300, 4, c.in).Summary; got != c.want {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}

func TestSizeSummaryPerAndSheet(t *testing.T) {
	out := SizeSummary(album300, 4, SizeIn{Scope: "all", DPI: 300, Quality: 80})
	want := []SizeQ{{100, "≈ 22 МБ"}, {95, "≈ 11 МБ"}, {80, "≈ 4,8 МБ"}}
	if len(out.Per) != 3 || out.Per[0] != want[0] || out.Per[1] != want[1] || out.Per[2] != want[2] {
		t.Errorf("Per: %+v, ждали %+v", out.Per, want)
	}
	// типография подмену dpi и вида в Per не вносит (как ExportQuality в окне: dpiNow уже 300 сам)
	pr := SizeSummary(album300, 4, SizeIn{Scope: "all", DPI: 150, Quality: 95, Pages: true, Print: true})
	if pr.Per[1].Text != "≈ 1,3 МБ" || !strings.HasPrefix(pr.Summary, "≈ 11 МБ на разворот") {
		t.Errorf("типография: %+v", pr)
	}
	sheet := model.Settings{PageW: 200, PageH: 150, DPI: 300, Sheet: true}
	got := SizeSummary(sheet, 10, SizeIn{Scope: "all", DPI: 0, Quality: 80}).Summary
	if got != "≈ 0,8 МБ на лист, всего ≈ 8,0 МБ" {
		t.Errorf("лист: %q", got)
	}
}

// Паритет с прежней формулой интерфейса: testdata/exportsize.json снят с exportopts.ts (estimateBytes,
// formatSize, sizeSummary) до переноса — сетка настроек, dpi, качеств и выборов; сверка байт в байт.
func TestSizeParityWithOldUI(t *testing.T) {
	type set struct {
		PageW, PageH float64
		DPI          int
		Sheet        bool
	}
	toModel := func(s set) model.Settings {
		return model.Settings{PageW: s.PageW, PageH: s.PageH, DPI: s.DPI, Sheet: s.Sheet}
	}
	var g struct {
		Estimate []struct {
			S     set
			DPI   int
			Q     int
			Pages bool
			Bytes float64
			Text  string
		}
		Format []struct {
			Bytes float64
			Text  string
		}
		Summary []struct {
			S     set
			DPI   int
			Q     int
			Pages bool
			W     struct {
				Scope   string
				Spreads int
				Units   []int
				Cover   bool
				Only    bool
				Print   bool
			}
			Text string
		}
	}
	data, err := os.ReadFile(filepath.Join("testdata", "exportsize.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Estimate) < 100 || len(g.Format) < 10 || len(g.Summary) < 100 {
		t.Fatalf("мало примеров: %d, %d, %d", len(g.Estimate), len(g.Format), len(g.Summary))
	}
	for i, c := range g.Estimate {
		got := EstimateBytes(toModel(c.S), c.DPI, c.Q, c.Pages)
		if got != c.Bytes || FormatSize(got) != c.Text {
			t.Errorf("estimate[%d] %+v: %v %q, ждали %v %q", i, c, got, FormatSize(got), c.Bytes, c.Text)
		}
	}
	for _, c := range g.Format {
		if got := FormatSize(c.Bytes); got != c.Text {
			t.Errorf("FormatSize(%v) = %q, ждали %q", c.Bytes, got, c.Text)
		}
	}
	for i, c := range g.Summary {
		in := SizeIn{Scope: c.W.Scope, DPI: c.DPI, Quality: c.Q, Pages: c.Pages, Units: c.W.Units,
			BadRange: c.W.Scope == "range" && c.W.Units == nil, Cover: c.W.Cover, Only: c.W.Only, Print: c.W.Print}
		if got := SizeSummary(toModel(c.S), c.W.Spreads, in).Summary; got != c.Text {
			t.Errorf("summary[%d] %+v: %q, ждали %q", i, c, got, c.Text)
		}
	}
}
