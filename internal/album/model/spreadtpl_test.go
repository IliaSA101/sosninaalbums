package model

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

// Шаблон разворота (спека 067, design §1): зажим, разбор, ключ.

const tplKeyA = "my:0123456789abcdef"

// fullTpl — разворот со всем: места, фон всего листа, страницы, стикеры обоих видов, надпись.
func fullTpl() SpreadTpl {
	return SpreadTpl{V: TplV, Pages: 2, Ratio: 2, W: 600, H: 300,
		Places: []Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}},
		Whole:  &TplBg{Color: "#ffffff", Pat: &Pattern{Kind: "dots"}},
		Page:   []TplBg{{Color: "#AABBCC", Lib: tplKeyA, Fade: 0.5}, {Color: "#112233"}},
		Stickers: []TplItem{
			{Anchor: Anchor{AX: 1, AY: -1, DX: -0.1, DY: 0.1}, W: 0.2, H: 0.2, Art: &Art{Kind: "star"}},
			{Anchor: Anchor{AX: 0, AY: 1}, W: 1, H: 0.1, Under: true, Lib: tplKeyA},
		},
		Texts: []TplText{{Anchor: Anchor{AY: -1, DY: 0.1}, W: 0.5, Text: "Наша свадьба", Style: PlainStyle()}},
	}
}

// TestClampSpreadTpl — каждый предел: отказ шапки, зажим чисел, NaN и пары Art/Lib отбрасывают
// предмет, фон листа исключает постраничный узор, «под местами» — первыми, пусто — false;
// зажим идемпотентен и не делит срезы с входом.
func TestClampSpreadTpl(t *testing.T) {
	for name, f := range map[string]func(*SpreadTpl){
		"V 0":         func(t *SpreadTpl) { t.V = 0 },
		"V новее":     func(t *SpreadTpl) { t.V = TplV + 1 },
		"Pages 3":     func(t *SpreadTpl) { t.Pages = 3 },
		"Pages ≠ len": func(t *SpreadTpl) { t.Page = t.Page[:1] },
	} {
		tp := fullTpl()
		f(&tp)
		if _, ok := ClampSpreadTpl(tp); ok {
			t.Errorf("%s: ждали отказ", name)
		}
	}
	if _, ok := ClampSpreadTpl(SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{}}}); ok {
		t.Error("пустой шаблон принят")
	}
	if _, ok := ClampSpreadTpl(SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{Color: "red", Lib: "my:xyz"}}}); ok {
		t.Error("шаблон только из плохих цвета и ключа принят")
	}

	in := fullTpl()
	in.Ratio, in.W, in.H = math.NaN(), 5, 300
	in.Places = append(in.Places, Box{X: math.NaN(), W: 0.1, H: 0.1}, Box{X: 0.95, Y: 0, W: 0.5, H: 0.5})
	in.Stickers = append(in.Stickers,
		TplItem{W: 0.1, H: 0.1}, // ни Art, ни Lib
		TplItem{W: 0.1, H: 0.1, Art: &Art{Kind: "star"}, Lib: tplKeyA}, // оба
		TplItem{W: math.NaN(), H: 0.1, Art: &Art{Kind: "star"}},        // NaN
		TplItem{Anchor: Anchor{DX: math.Inf(1)}, W: 0.1, H: 0.1, Lib: tplKeyA},
		TplItem{W: 0.1, H: 0.1, Lib: "my:XYZ"},               // ключ не по грамматике
		TplItem{W: 0.1, H: 0.1, Art: &Art{Kind: "Bad Kind"}}, // рисунок не по грамматике
		TplItem{Anchor: Anchor{AX: 5, AY: -7, DX: 9, DY: -9}, W: 100, H: 0, Rot: 370, Fade: 2, Art: &Art{Kind: "star"}},
	)
	in.Texts = append(in.Texts, TplText{W: 0.2, Text: " \n "}, TplText{W: 0.2, Rot: math.NaN(), Text: "x"})
	c, ok := ClampSpreadTpl(in)
	if !ok {
		t.Fatal("отказ полного шаблона")
	}
	if c.Ratio != 1 || c.W != 0 || c.H != 0 {
		t.Errorf("Ratio %v, W×H %v×%v: ждали 1 и 0×0", c.Ratio, c.W, c.H)
	}
	if len(c.Places) != 2 || c.Places[1].X+c.Places[1].W > 1+1e-9 {
		t.Errorf("места: %+v", c.Places)
	}
	if c.Page[0].Color != "#aabbcc" || c.Page[0].Lib != tplKeyA || c.Page[0].Fade != 0.5 || c.Page[0].Pat != nil {
		t.Errorf("левая: %+v", c.Page[0])
	}
	if c.Whole == nil || c.Whole.Color != "" || c.Whole.Pat == nil || c.Whole.Pat.Kind != "dots" {
		t.Errorf("весь лист: %+v", c.Whole)
	}
	if len(c.Stickers) != 3 || !c.Stickers[0].Under || c.Stickers[0].Lib != tplKeyA || c.Stickers[1].Art.Kind != "star" {
		t.Fatalf("стикеры: %+v", c.Stickers)
	}
	if e := c.Stickers[2]; e.AX != 1 || e.AY != -1 || e.DX != 2 || e.DY != -2 || e.W != 4 || e.H != 0.01 || e.Rot != 10 || e.Fade != 1 {
		t.Errorf("зажим стикера: %+v", e)
	}
	if len(c.Texts) != 1 || c.Texts[0].Text != "Наша свадьба" {
		t.Errorf("надписи: %+v", c.Texts)
	}
	in.Places[0].X = 0.7 // вход правится — копия нет
	in.Page[0].Pat = &Pattern{Kind: "dots"}
	if c.Places[0].X != 0.1 || c.Page[0].Pat != nil {
		t.Error("зажим делит данные со входом")
	}
	again, ok := ClampSpreadTpl(c)
	if !ok || jsonTpl(t, again) != jsonTpl(t, c) {
		t.Errorf("зажим не идемпотентен:\n%s\n%s", jsonTpl(t, c), jsonTpl(t, again))
	}
}

// TestClampSpreadTplLimits — пределы числа предметов (MaxStickers, MaxTexts), фото всего
// разворота вместо постраничных, шаблон из одних мест.
func TestClampSpreadTplLimits(t *testing.T) {
	many := SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{}}}
	for range MaxStickers + 5 {
		many.Stickers = append(many.Stickers, TplItem{W: 0.1, H: 0.1, Art: &Art{Kind: "star"}})
	}
	for range MaxTexts + 5 {
		many.Texts = append(many.Texts, TplText{W: 0.1, Text: "a"})
	}
	if c, _ := ClampSpreadTpl(many); len(c.Stickers) != MaxStickers || len(c.Texts) != MaxTexts {
		t.Errorf("стикеров %d, надписей %d", len(c.Stickers), len(c.Texts))
	}
	// Фото фона всего разворота исключает постраничные (как слоты Spread), узор страниц остаётся.
	wl := fullTpl()
	wl.Whole = &TplBg{Lib: tplKeyA, Fade: 0.3}
	wl.Page[1].Pat = &Pattern{Kind: "stripes"}
	if c, _ := ClampSpreadTpl(wl); c.Whole == nil || c.Whole.Lib != tplKeyA || c.Page[0].Lib != "" || c.Page[0].Fade != 0 || c.Page[1].Pat == nil {
		t.Errorf("фото всего разворота: %+v / %+v", c.Whole, c.Page)
	}
	// Шаблон из одних мест — не пустой; страница без фона.
	if _, ok := ClampSpreadTpl(SpreadTpl{V: TplV, Pages: 1, Page: []TplBg{{}}, Places: []Box{{W: 0.5, H: 0.5}}}); !ok {
		t.Error("шаблон из одних мест отвергнут")
	}
}

func jsonTpl(t *testing.T, v SpreadTpl) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestParseSpreadTpl — разбор: неизвестные поля пропускаются, новее — ErrTplNewer, пусто —
// ErrTplEmpty, больше MaxTplFile — отказ, не JSON — ошибка.
func TestParseSpreadTpl(t *testing.T) {
	tp, err := ParseSpreadTpl([]byte(`{"v":1,"pages":1,"page":[{"color":"#FFEEDD","glow":3}],"future":{"x":1}}`))
	if err != nil || tp.Page[0].Color != "#ffeedd" || tp.Ratio != 1 {
		t.Fatalf("%+v, %v", tp, err)
	}
	if _, err := ParseSpreadTpl([]byte(`{"v":2,"pages":1,"page":[{"color":"#ffffff"}]}`)); !errors.Is(err, ErrTplNewer) {
		t.Errorf("v2: %v", err)
	}
	if _, err := ParseSpreadTpl([]byte(`{"v":1,"pages":1,"page":[{}]}`)); !errors.Is(err, ErrTplEmpty) {
		t.Errorf("пустой: %v", err)
	}
	if _, err := ParseSpreadTpl([]byte(`[`)); err == nil {
		t.Error("не JSON принят")
	}
	big := `{"v":1,"pages":1,"page":[{"color":"#ffffff"}],"pad":"` + strings.Repeat("x", MaxTplFile) + `"}`
	if _, err := ParseSpreadTpl([]byte(big)); err == nil {
		t.Error("файл больше MaxTplFile принят")
	}
}

// TestSpreadTplKey — ключ от канонического JSON: порядок полей и пробелы файла не важны, другое
// содержимое — другой ключ; грамматика — ключ библиотеки.
func TestSpreadTplKey(t *testing.T) {
	a, err := ParseSpreadTpl([]byte(`{"v":1,"pages":1,"ratio":1,"page":[{"color":"#ffffff","pat":{"kind":"dots"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseSpreadTpl([]byte("{ \"page\" : [ { \"pat\":{\"kind\":\"dots\"}, \"color\":\"#FFFFFF\" } ],\n \"pages\":1, \"v\":1 }"))
	if err != nil {
		t.Fatal(err)
	}
	ka, kb := TplKey(a), TplKey(b)
	if ka != kb || !ValidLibKey(ka) {
		t.Fatalf("ключи %q и %q", ka, kb)
	}
	b.Page[0].Color = "#fffffe"
	if TplKey(b) == ka {
		t.Error("другой шаблон — тот же ключ")
	}
	if TplKey(SpreadTpl{Ratio: math.NaN()}) != "" {
		t.Error("NaN — ждали пустой ключ")
	}
}

// FuzzParseSpreadTpl — любые байты: без паники; принятый шаблон — неподвижная точка зажима и
// разбора (запись → чтение даёт тот же ключ), без NaN.
func FuzzParseSpreadTpl(f *testing.F) {
	seed, _ := json.Marshal(fullTpl())
	f.Add(seed)
	f.Add([]byte(`{"v":1,"pages":1,"page":[{"color":"#ffffff"}],"places":[{"x":0.9,"y":0.9,"w":0.5,"h":0.5,"rot":30}]}`))
	f.Add([]byte(`{"v":1,"pages":2,"page":[{},{}],"stickers":[{"ax":1,"ay":1,"dx":0.3333333,"w":0.1,"h":0.1,"lib":"my:0123456789abcdef"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		tp, err := ParseSpreadTpl(data)
		if err != nil {
			return
		}
		b, err := json.Marshal(tp)
		if err != nil {
			t.Fatalf("принятый шаблон не маршалится: %v", err)
		}
		again, err := ParseSpreadTpl(b)
		if err != nil || TplKey(again) != TplKey(tp) {
			t.Fatalf("круг записи изменил шаблон: %v\n%s", err, b)
		}
	})
}
