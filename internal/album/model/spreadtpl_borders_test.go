package model

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Рамочки мест и заготовки в шаблоне (131 G1, design §1): зажим списков borders и drafts.

func borderBase(places int) SpreadTpl {
	t := SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{Color: "#ffffff"}}}
	for range places {
		t.Places = append(t.Places, Box{X: 0.1, Y: 0.1, W: 0.3, H: 0.3})
	}
	return t
}

var (
	tplLine = TplBorder{Kind: "line", Inks: []string{"#ABCDEF"}, Width: 2.55}
	tplPic  = TplBorder{Lib: tplKeyA, Scale: 1.5, Cut: [4]float64{0.1, 0.2, 0.3, 9}, Open: true}
)

// TestSpreadTplBordersDrafts — индекс.
func TestSpreadTplBordersDrafts(t *testing.T) {
	tp := borderBase(3)
	tp.Borders = []TplBorder{{I: 2, Kind: "line"}, {I: -1, Kind: "line"}, {I: 3, Kind: "line"}, {I: 0, Kind: "gold"}}
	c, ok := ClampSpreadTpl(tp)
	if !ok || len(c.Borders) != 2 || c.Borders[0].I != 0 || c.Borders[1].I != 2 || c.Borders[0].Kind != "gold" {
		t.Fatalf("%+v, %v", c.Borders, ok)
	}
}

// TestSpreadTplBordersDraftsIndexShift — место выброшено — индекс не съезжает.
func TestSpreadTplBordersDraftsIndexShift(t *testing.T) {
	tp := borderBase(3)
	tp.Places[0].X = math.NaN()
	tp.Borders = []TplBorder{{I: 0, Kind: "line"}, {I: 2, Kind: "gold"}}
	c, _ := ClampSpreadTpl(tp)
	if len(c.Places) != 2 || len(c.Borders) != 1 || c.Borders[0].I != 1 || c.Borders[0].Kind != "gold" {
		t.Fatalf("места %d, рамочки %+v", len(c.Places), c.Borders)
	}
}

// TestSpreadTplBordersDraftsKindXorLib — ровно одно из Kind и Lib.
func TestSpreadTplBordersDraftsKindXorLib(t *testing.T) {
	tp := borderBase(6)
	tp.Borders = []TplBorder{
		{I: 0, Kind: "line", Lib: tplKeyA}, // оба
		{I: 1},                             // ни того ни другого
		{I: 2, Kind: "Bad Kind"},           // вид не по грамматике
		{I: 3, Lib: "my:XYZ"},              // ключ не по грамматике
		{I: 4, Kind: "line", Scale: 2, Open: true, Cut: [4]float64{1, 1, 1, 1}}, // поля картинки у встроенной снимаются
		{I: 5, Lib: tplKeyA, Inks: []string{"#fff"}, Width: 3},                  // поля встроенной у картинки снимаются
	}
	c, _ := ClampSpreadTpl(tp)
	want := []TplBorder{{I: 4, Kind: "line"}, {I: 5, Lib: tplKeyA}}
	if !reflect.DeepEqual(c.Borders, want) {
		t.Fatalf("%+v, ждали %+v", c.Borders, want)
	}
}

// TestSpreadTplBordersDraftsDup — повтор индекса — первая запись.
func TestSpreadTplBordersDraftsDup(t *testing.T) {
	tp := borderBase(2)
	first, second := tplLine, tplPic
	first.I, second.I = 1, 1
	tp.Borders = []TplBorder{first, second}
	if c, _ := ClampSpreadTpl(tp); len(c.Borders) != 1 || c.Borders[0].Kind != "line" {
		t.Errorf("рабочая первая: %+v", c.Borders)
	}
	bad := TplBorder{I: 1, Kind: "line", Lib: tplKeyA}
	tp.Borders = []TplBorder{bad, second}
	if c, _ := ClampSpreadTpl(tp); len(c.Borders) != 0 {
		t.Errorf("испорченная первая решает за место: %+v", c.Borders)
	}
}

// TestSpreadTplBordersDraftsNumbers — числа.
func TestSpreadTplBordersDraftsNumbers(t *testing.T) {
	tp := borderBase(2)
	a, b := tplLine, tplPic
	b.I = 1
	tp.Borders = []TplBorder{a, b}
	c, _ := ClampSpreadTpl(tp)
	if got := c.Borders[0]; got.Inks[0] != "#abcdef" || got.Width != 2.6 {
		t.Errorf("встроенная: %+v", got)
	}
	if got := c.Borders[1]; got.Scale != 1.5 || got.Cut != [4]float64{0.1, 0.2, 0.3, 0.45} || !got.Open || got.Lib != tplKeyA {
		t.Errorf("картинка: %+v", got)
	}
	nan := borderBase(1)
	nan.Borders = []TplBorder{{Lib: tplKeyA, Scale: math.NaN(), Cut: [4]float64{math.NaN(), math.Inf(1), -1, 0}}, {Kind: "line", Width: math.Inf(1)}}
	if c, _ := ClampSpreadTpl(nan); len(c.Borders) != 1 || c.Borders[0].Scale != 0 || c.Borders[0].Cut != [4]float64{0, 0.45, 0, 0} {
		t.Errorf("NaN и Inf: %+v", c.Borders)
	}
}

// TestSpreadTplBordersDraftsNoPlaces — без мест рамочек нет; пусто — не шаблон.
func TestSpreadTplBordersDraftsNoPlaces(t *testing.T) {
	tp := borderBase(0)
	tp.Borders = []TplBorder{tplLine}
	c, ok := ClampSpreadTpl(tp)
	if len(c.Borders) != 0 || !ok { // фон страницы остался — шаблон жив
		t.Errorf("%+v, %v", c.Borders, ok)
	}
	tp.Page = []TplBg{{}}
	if _, ok := ClampSpreadTpl(tp); ok {
		t.Error("шаблон из одних рамочек без мест принят")
	}
}

// TestSpreadTplBordersDraftsCap — записей не больше мест.
func TestSpreadTplBordersDraftsCap(t *testing.T) {
	tp := borderBase(2)
	for i := range 40 {
		tp.Borders = append(tp.Borders, TplBorder{I: i % 2, Kind: "line"})
	}
	if c, _ := ClampSpreadTpl(tp); len(c.Borders) != 2 {
		t.Errorf("%d записей на 2 места", len(c.Borders))
	}
}

// TestSpreadTplBordersDraftsDraftsLimit — заготовки: предел надписей.
func TestSpreadTplBordersDraftsDraftsLimit(t *testing.T) {
	tp := borderBase(0)
	for range 40 {
		tp.Texts = append(tp.Texts, TplText{W: 0.1, Text: "t"})
		tp.Drafts = append(tp.Drafts, TplText{W: 0.1, Text: "[д]"})
	}
	if c, _ := ClampSpreadTpl(tp); len(c.Texts) != 40 || len(c.Drafts) != MaxTexts-40 {
		t.Errorf("надписей %d, заготовок %d", len(c.Texts), len(c.Drafts))
	}
	for range 30 {
		tp.Texts = append(tp.Texts, TplText{W: 0.1, Text: "t"})
	}
	if c, _ := ClampSpreadTpl(tp); len(c.Texts) != MaxTexts || len(c.Drafts) != 0 {
		t.Errorf("надписей %d, заготовок %d", len(c.Texts), len(c.Drafts))
	}
}

// TestSpreadTplBordersDraftsDraftsOnly — заготовки: пустой текст и одни заготовки.
func TestSpreadTplBordersDraftsDraftsOnly(t *testing.T) {
	tp := SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{}}, Drafts: []TplText{{W: 0.4, Text: " \n "}, {W: 0.4, Text: "[Имена]"}}}
	c, ok := ClampSpreadTpl(tp)
	if !ok || len(c.Drafts) != 1 || c.Drafts[0].Text != "[Имена]" {
		t.Fatalf("%+v, %v", c.Drafts, ok)
	}
	tp.Drafts = tp.Drafts[:1]
	if _, ok := ClampSpreadTpl(tp); ok {
		t.Error("шаблон из пустой заготовки принят")
	}
}

// TestSpreadTplBordersDraftsRound — круг: зажим идемпотентен, запись → чтение — тот же ключ.
func TestSpreadTplBordersDraftsRound(t *testing.T) {
	tp := borderBase(2)
	a, b := tplLine, tplPic
	b.I = 1
	tp.Borders = []TplBorder{b, a}
	tp.Drafts = []TplText{{W: 0.4, Text: "[Дата]", Style: PlainStyle()}}
	c, _ := ClampSpreadTpl(tp)
	again, ok := ClampSpreadTpl(c)
	if !ok || !reflect.DeepEqual(c, again) {
		t.Fatalf("зажим не идемпотентен:\n%+v\n%+v", c, again)
	}
	back, err := ParseSpreadTpl([]byte(jsonTpl(t, c)))
	if err != nil || TplKey(back) != TplKey(c) || len(back.Borders) != 2 || len(back.Drafts) != 1 {
		t.Fatalf("круг записи: %v %+v", err, back)
	}
	if TplKey(c) == TplKey(SpreadTpl{V: TplV, Pages: 1, Ratio: 1, Page: c.Page, Places: c.Places}) {
		t.Error("рамочки и заготовки не входят в ключ")
	}
	if !strings.Contains(jsonTpl(t, c), `"borders":[`) || !strings.Contains(jsonTpl(t, c), `"drafts":[`) {
		t.Errorf("ключи JSON: %s", jsonTpl(t, c))
	}
	// Шаблон без списков пишется без ключей borders/drafts: ключ шаблонов 0.27 прежний.
	plain := jsonTpl(t, borderBase(1))
	if strings.Contains(plain, "borders") || strings.Contains(plain, "drafts") {
		t.Errorf("пустые списки в JSON: %s", plain)
	}
}

// TestSpreadTplBordersDraftsParse — разбор из файла: записей чужих полей нет.
func TestSpreadTplBordersDraftsParse(t *testing.T) {
	raw := `{"v":1,"pages":1,"page":[{}],"places":[{"x":0.1,"y":0.1,"w":0.3,"h":0.3}],` +
		`"borders":[{"i":0,"kind":"line","future":1},{"i":"x"}],"drafts":[{"w":0.4,"text":"[Имена]","ax":0,"ay":0}]}`
	if _, err := ParseSpreadTpl([]byte(raw)); err == nil {
		t.Error(`"i":"x" — ждали ошибку разбора JSON`)
	}
	ok := strings.Replace(raw, `,{"i":"x"}`, "", 1)
	c, err := ParseSpreadTpl([]byte(ok))
	if err != nil || len(c.Borders) != 1 || len(c.Drafts) != 1 {
		t.Fatalf("%+v, %v", c, err)
	}
}

// TestSpreadTplBordersPreset — отметка плитки Preset: у встроенной по грамматике хранится (двойной
// зажим тот же), негодная снимается, у картинки её нет; в шаблоне Preset едет в JSON и круг
// Marshal → Parse его сохраняет, без Preset ключ в JSON не появляется.
func TestSpreadTplBordersPreset(t *testing.T) {
	tp := borderBase(5)
	tp.Borders = []TplBorder{
		{I: 0, Kind: "line", Preset: "sa:line"},
		{I: 1, Kind: "line", Preset: "bad"},
		{I: 2, Lib: tplKeyA, Preset: "sa:line"},
		{I: 3, Kind: "gold", Preset: "sa:gold"},
		{I: 4, Kind: "line"},
	}
	c, ok := ClampSpreadTpl(tp)
	if !ok {
		t.Fatal("шаблон отброшен")
	}
	want := []TplBorder{{I: 0, Kind: "line", Preset: "sa:line"}, {I: 1, Kind: "line"}, {I: 2, Lib: tplKeyA}, {I: 3, Kind: "gold", Preset: "sa:gold"}, {I: 4, Kind: "line"}}
	if !reflect.DeepEqual(c.Borders, want) {
		t.Fatalf("%+v, ждали %+v", c.Borders, want)
	}
	if c2, _ := ClampSpreadTpl(c); !reflect.DeepEqual(c2.Borders, want) {
		t.Fatalf("зажим не идемпотентен: %+v", c2.Borders)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseSpreadTpl(data)
	if err != nil || !reflect.DeepEqual(back.Borders, want) {
		t.Fatalf("круг через JSON: %v, %+v", err, back.Borders)
	}
	if n := strings.Count(string(data), `"preset"`); n != 2 {
		t.Fatalf("preset в JSON %d раз, ждали 2: %s", n, data)
	}
}

// TestClampTplBordersForeignMap — карта at от чужих данных, несогласованная с n (индекс за
// пределами seen), даёт пропуск записи, а не панику.
func TestClampTplBordersForeignMap(t *testing.T) {
	in := []TplBorder{{I: 0, Kind: "line"}, {I: 1, Kind: "gold"}}
	got := clampTplBorders(in, []int{0, 5}, 2)
	if len(got) != 1 || got[0].I != 0 {
		t.Fatalf("%+v", got)
	}
}
