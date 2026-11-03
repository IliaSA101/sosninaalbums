package model

import (
	"reflect"
	"testing"
)

// SideTpl и SheetBgTpl: сторона обложки и фон листа — общий источник плитки программы и library-verify.

func coverForTiles() *CoverTpl {
	pat := &Pattern{Kind: "dots"}
	back := SpreadTpl{V: TplV, Pages: 1, Ratio: 0.5, Places: []Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.4}},
		Page: []TplBg{{Color: "#aa0000", Lib: "my:00000000000000b1"}}}
	front := SpreadTpl{V: TplV, Pages: 1, Ratio: 0.5,
		Places:   []Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.4}, {X: 0.1, Y: 0.6, W: 0.8, H: 0.3}},
		Page:     []TplBg{{Pat: pat}},
		Borders:  []TplBorder{{I: 0, Kind: "line"}},
		Drafts:   []TplText{{Text: "[Имена]"}},
		Texts:    []TplText{{Text: "Свадьба"}},
		Stickers: []TplItem{{Art: &Art{Kind: "star"}, W: 0.1, H: 0.1}}}
	return &CoverTpl{Bg: &TplBg{Color: "#00aa00"}, Back: &back, Front: &front}
}

func TestSideTpl(t *testing.T) {
	c := coverForTiles()
	front := c.SideTpl(1, 0.5) // r равен Ratio сторонки: места как есть
	if front.Pages != 1 || len(front.Page) != 1 {
		t.Fatalf("сторона — страница: %+v", front)
	}
	if front.Page[0].Color != "#00aa00" || front.Page[0].Pat == nil {
		t.Errorf("фон лицевой: цвет листа, свой узор (у листа его нет): %+v", front.Page[0])
	}
	if len(front.Borders) != 1 || len(front.Drafts) != 1 || len(front.Texts) != 1 || len(front.Stickers) != 1 {
		t.Errorf("рамочки, заготовки, надписи и стикеры стороны теряться не должны: %+v", front)
	}
	if !reflect.DeepEqual(front.Places, c.Front.Places) {
		t.Errorf("места при r = Ratio: %v", front.Places)
	}
	back := c.SideTpl(0, 0.5)
	if back.Page[0].Color != "#aa0000" || back.Page[0].Lib != "my:00000000000000b1" {
		t.Errorf("фон задней: свой цвет, своё фото: %+v", back.Page[0])
	}
	// Места — по LookBoxes на области: зона в 2 раза шире Ratio — композиция уже, по центру.
	wide := c.SideTpl(1, 1)
	want := LookBoxes(*c.Front, 1)
	if !reflect.DeepEqual(wide.Places, want) || wide.Places[0].X == c.Front.Places[0].X {
		t.Errorf("LookBoxes(r=1): %v, ждали %v", wide.Places, want)
	}
	// r ≤ 0 и NaN — места как есть (lookTpl library-verify ставит их сам по размерам).
	for _, r := range []float64{0, -1} {
		if got := c.SideTpl(1, r).Places; !reflect.DeepEqual(got, c.Front.Places) {
			t.Errorf("r=%v: %v", r, got)
		}
	}
	// Источник не меняется: срез мест и фон макета прежние.
	if c.Front.Page[0].Color != "" || c.Front.Page[0].Pat == nil || c.Back.Page[0].Color != "#aa0000" || c.Front.Places[0].X != 0.1 {
		t.Errorf("SideTpl изменил макет: %+v", c.Front)
	}
}

func TestSideTplEdges(t *testing.T) {
	var nilCover *CoverTpl
	if s := nilCover.SideTpl(1, 1); s.Pages != 1 || len(s.Places) != 0 || len(s.Page) != 0 {
		t.Errorf("обложки нет — пустая страница: %+v", s)
	}
	// Стороны нет: пустая страница с фоном листа (Bg), как рисует альбом.
	c := &CoverTpl{Bg: &TplBg{Color: "#112233"}, Front: coverForTiles().Front}
	if s := c.SideTpl(0, 1); s.Pages != 1 || s.V != TplV || len(s.Places) != 0 || s.Page[0].Color != "#112233" {
		t.Errorf("задней нет: %+v", s)
	}
	// k вне 0…1 — страница с фоном листа, без чужих мест.
	for _, k := range []int{-1, 2, 99} {
		if s := coverForTiles().SideTpl(k, 1); len(s.Places) != 0 || len(s.Stickers) != 0 || s.Page[0].Color != "#00aa00" {
			t.Errorf("k=%d: %+v", k, s)
		}
	}
	// Фон листа — узор: страница стороны его не повторяет своим (CoverPageBg), свой узор стороны уходит.
	d := coverForTiles()
	d.Bg = &TplBg{Pat: &Pattern{Kind: "stripes"}}
	if s := d.SideTpl(1, 1); s.Page[0].Pat == nil || s.Page[0].Pat.Kind != "stripes" {
		t.Errorf("узор листа главнее своего: %+v", s.Page[0])
	}
}

func TestSheetBgTpl(t *testing.T) {
	c := coverForTiles()
	s := c.SheetBgTpl()
	if s.Pages != 2 || len(s.Page) != 2 || s.Whole != nil || s.Page[0].Color != "#aa0000" || s.Page[1].Color != "#00aa00" {
		t.Errorf("цвет листа, свой у задней: %+v", s)
	}
	// Узор листа — на весь лист; на страницах узора и фото нет.
	c.Bg = &TplBg{Color: "#00aa00", Pat: &Pattern{Kind: "dots"}, Fade: 0.3}
	s = c.SheetBgTpl()
	if s.Whole == nil || s.Whole.Pat == nil || s.Whole.Fade != 0.3 {
		t.Fatalf("Whole: %+v", s.Whole)
	}
	for i, p := range s.Page {
		if p.Pat != nil || p.Lib != "" || p.Fade != 0 {
			t.Errorf("страница %d несёт узор листа: %+v", i, p)
		}
	}
	if _, ok := ClampSpreadTpl(s); !ok {
		t.Error("фон листа — годный шаблон")
	}
}
