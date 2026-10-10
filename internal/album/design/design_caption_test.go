package design_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// capStyle — стиль подписи фото id из проекта редактора.
func capStyle(e *album.Editor, id string) model.TextStyle {
	return edtest.P(e).Photos[id].Caption.Style
}

func TestCaptionSizeByFormat(t *testing.T) {
	for _, c := range []struct {
		w, h, want float64
	}{{200, 200, 8}, {300, 300, 12}, {400, 300, 12}} {
		e := edtest.Open(t, model.New(), "")
		st := e.View().Settings
		st.PageW, st.PageH = c.w, c.h
		e.SetSettings(st)
		edtest.NewSpread(e, -1, edtest.Photos("a"))
		e.SetCaption("a", "Подпись", "")
		if got := capStyle(e, "a").Size; got != c.want {
			t.Errorf("страница %v×%v: кегль %v, ждали %v", c.w, c.h, got, c.want)
		}
	}
	if got := model.DefaultCaptionStyle().Size; got != 12 {
		t.Errorf("DefaultCaptionStyle: %v", got)
	}
	// Очень маленькая страница — не мельче 8 пт.
	if got := model.CaptionSize(50); got != model.CaptionMinSize {
		t.Errorf("пол кегля: %v", got)
	}
}

func TestCaptionInheritsStyle(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	edtest.NewSpread(e, -1, edtest.Photos("c"))
	edtest.NewSpread(e, -1, edtest.Photos("d", "e", "f"))
	// Подписей нет — стиль по формату.
	e.SetCaption("a", "А", "")
	if capStyle(e, "a") != model.DefaultCaptionStyle() || edtest.P(e).Photos["a"].Caption.Over {
		t.Fatalf("первая подпись: %+v", edtest.P(e).Photos["a"].Caption)
	}
	// Поправленное оформление: 20 пт, слева, на фото.
	mine := model.ClampStyle(model.TextStyle{Font: "ptserif", Size: 20, Color: "#112233", Align: "left", Italic: true})
	e.SetCaptionStyle("a", mine, true)
	// Тот же разворот.
	e.SetCaption("b", "Б", "")
	if c := edtest.P(e).Photos["b"].Caption; c.Style != mine || !c.Over {
		t.Fatalf("с того же разворота: %+v", c)
	}
	// Другой разворот без подписей: ближайшая по ленте (до d ближе b, чем a).
	other := model.ClampStyle(model.TextStyle{Font: "ptserif", Size: 16, Color: "#112233", Align: "right"})
	e.SetCaptionStyle("b", other, false)
	e.SetCaption("d", "Д", "")
	if c := edtest.P(e).Photos["d"].Caption; c.Style != other || c.Over {
		t.Fatalf("ближайшая: %+v", c)
	}
	// На своём развороте подпись есть — берём её, а не ближайшую по ленте.
	e.SetCaptionStyle("d", mine, true)
	e.SetCaption("f", "Ф", "")
	if c := edtest.P(e).Photos["f"].Caption; c.Style != mine || !c.Over {
		t.Fatalf("тот же разворот: %+v", c)
	}
	// Смена текста стиль не трогает.
	e.SetCaption("f", "Ф2", "")
	if c := edtest.P(e).Photos["f"].Caption; c.Style != mine || c.Text != "Ф2" {
		t.Fatalf("смена текста: %+v", c)
	}
	// Снятая подпись образцом не служит: у c ближайшая теперь b (a снята).
	e.SetCaption("a", "", "")
	e.SetCaption("c", "С", "")
	if c := edtest.P(e).Photos["c"].Caption; c.Style != other {
		t.Fatalf("после снятия a: %+v", c)
	}

	// Ничья по расстоянию: подписи слева и справа на равном расстоянии — берётся
	// предыдущая по ленте (строгое d < bestD).
	e = edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("p"))
	edtest.NewSpread(e, -1, edtest.Photos("q"))
	edtest.NewSpread(e, -1, edtest.Photos("r"))
	e.SetCaption("p", "П", "")
	e.SetCaptionStyle("p", other, false)
	e.SetCaption("r", "Р", "")
	e.SetCaptionStyle("r", mine, true)
	e.SetCaption("q", "К", "")
	if c := edtest.P(e).Photos["q"].Caption; c.Style != other || c.Over {
		t.Fatalf("ничья: %+v", c)
	}
	// Фото вне разворотов (at < 0): первая подпись ленты.
	if c := design.CaptionSource(edtest.P(e), "вне ленты"); c == nil || c.Style != other {
		t.Fatalf("вне разворотов: %+v", c)
	}
}

func TestCaptionNoopAndUndo(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	rev := e.View().Rev
	if v := e.SetCaption("a", "  ", ""); v.Rev != rev {
		t.Fatal("пустая подпись без подписи — холостая правка")
	}
	e.SetCaption("a", "Первая", "")
	e.SetCaptionStyle("a", model.ClampStyle(model.TextStyle{Size: 18}), false)
	rev = e.View().Rev
	if v := e.SetCaption("a", "Первая", ""); v.Rev != rev {
		t.Fatal("тот же текст — холостая правка")
	}
	// Новая подпись с унаследованным стилем — один шаг undo.
	e.SetCaption("b", "Вторая", "")
	if capStyle(e, "b").Size != 18 {
		t.Fatalf("наследование: %v", capStyle(e, "b"))
	}
	e.Undo()
	if edtest.P(e).Photos["b"].Caption != nil || edtest.P(e).Photos["a"].Caption == nil {
		t.Fatal("undo должен снять только вторую подпись")
	}
}
