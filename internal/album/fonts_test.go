package album_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Свои шрифты в Editor (082): SyncFonts в конвейере правки, RefitFonts после загрузки шрифта.

// TestRefitFontsStep — шрифта нет: RefitFonts холостая, правка соседнего блока блоки без шрифта не
// двигает; шрифт нашёлся — один шаг отмены на все блоки обоих разворотов и название в таблице,
// повтор — холостой; Undo возвращает прежние ширины, Redo — новые.
func TestRefitFontsStep(t *testing.T) {
	lato := edtest.FontID("Lato")
	block := func(id string, x float64) *model.TextBlock {
		return &model.TextBlock{ID: id, X: x, Y: 50, W: 100, Auto: true, Text: "Анна и Павел",
			Style: model.TextStyle{Font: lato, Size: 30, Color: "#222222", Align: "left"}}
	}
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Texts: []*model.TextBlock{block("0a0b0c0d0e01", 20),
			{ID: "0a0b0c0d0e02", X: 150, Y: 150, W: 40, Auto: true, Text: "x", Style: model.PlainStyle()}}},
		{ID: "s2", Texts: []*model.TextBlock{block("0a0b0c0d0e03", 30)}},
	}
	p.Fonts = map[string]model.FontRef{lato: {}} // название неизвестно (из шаблона)
	p.FitTexts()                                 // как после Load: встроенный блок уже по тексту
	e := edtest.Open(t, p, "")
	widths := func() (a, b float64) {
		q := edtest.P(e)
		return q.Spreads[0].Texts[0].W, q.Spreads[1].Texts[0].W
	}

	rev := edtest.Rev(e)
	e.RefitFonts()
	if edtest.UndoLen(e) != 0 || edtest.Rev(e) != rev || edtest.Dirty(e) {
		t.Fatalf("без шрифта RefitFonts не холостая: шагов %d, rev %d → %d", edtest.UndoLen(e), rev, edtest.Rev(e))
	}
	e.SetText("0a0b0c0d0e02", "Свадьба", "")
	if a, b := widths(); a != 100 || b != 100 || edtest.P(e).Spreads[0].Texts[0].X != 20 {
		t.Fatalf("правка соседа сдвинула блоки без шрифта: %v %v", a, b)
	}
	if edtest.P(e).Fonts[lato] != (model.FontRef{}) {
		t.Fatalf("таблица: %v", edtest.P(e).Fonts)
	}

	edtest.UseFont(t, "Lato", "Oswald-Regular.ttf")
	steps := edtest.UndoLen(e)
	e.RefitFonts()
	a, b := widths()
	if edtest.UndoLen(e) != steps+1 || a == 100 || b == 100 {
		t.Fatalf("шрифт нашёлся: шагов %d (было %d), ширины %v %v", edtest.UndoLen(e), steps, a, b)
	}
	if edtest.P(e).Fonts[lato].Name != "Lato" {
		t.Fatalf("название: %v", edtest.P(e).Fonts)
	}
	rev = edtest.Rev(e)
	e.RefitFonts()
	if edtest.UndoLen(e) != steps+1 || edtest.Rev(e) != rev {
		t.Fatal("повторный RefitFonts — не холостая")
	}

	e.Undo()
	if ua, ub := widths(); ua != 100 || ub != 100 || edtest.P(e).Fonts[lato].Name != "" {
		t.Fatalf("Undo: ширины %v %v, таблица %v", ua, ub, edtest.P(e).Fonts)
	}
	e.Redo()
	if ra, rb := widths(); ra != a || rb != b {
		t.Fatalf("Redo: %v %v, ждали %v %v", ra, rb, a, b)
	}
}
