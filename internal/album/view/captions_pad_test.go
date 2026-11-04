package view_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Вид отдаёт действующий отступ подписи: свой, по умолчанию, из стиля листа; правка
// стиля с отступом из вида его не меняет (UI шлёт pad в каждой правке).
func TestCaptionViewPad(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(10, 120, 100, 100, "b")})
	e.SetCaption("a", "Подпись", "")
	st := model.DefaultCaptionStyle()
	if cv := capOfPhoto(e.View().Spreads[0], "a"); cv == nil || cv.Pad != model.DefaultPad {
		t.Fatalf("по умолчанию: %+v", cv)
	}
	cv := capOfPhoto(e.SetCaptionStyle("a", st, false, 4).Spreads[0], "a")
	if cv.Pad != 4 {
		t.Fatalf("свой отступ: %.1f", cv.Pad)
	}
	v := e.SetCaptionStyle("a", st, true, cv.Pad)
	if cv = capOfPhoto(v.Spreads[0], "a"); cv.Pad != 4 || edtest.P(e).Photos["a"].Caption.PadOf() != 4 {
		t.Fatalf("после правки стиля: %+v", cv)
	}
	// Стиль листа: отступ листа — и у подписи, и в sv.Caps.
	v = e.SetSheetCaps("s1", true, sheetTpl("", 6))
	cv = capOfPhoto(v.Spreads[0], "a")
	if cv.Pad != 6 || v.Spreads[0].Caps == nil || v.Spreads[0].Caps.Pad != 6 {
		t.Fatalf("отступ листа: %+v, caps %+v", cv, v.Spreads[0].Caps)
	}
	// Свой отступ при стиле листа хранится и возвращается после снятия.
	v = e.SetSheetCaps("s1", false, model.SheetCaps{})
	if cv = capOfPhoto(v.Spreads[0], "a"); cv.Pad != 4 || cv.Sheet {
		t.Fatalf("после снятия стиля листа: %+v", cv)
	}
}

// SetSheetCaps с on = false игнорирует c: стиль листа снят, у подписей действует своё.
func TestSheetCapsOffIgnoresArg(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	e.SetCaption("a", "Подпись", "")
	e.SetSheetCaps("s1", true, sheetTpl("", 6))
	junk := model.SheetCaps{Style: model.TextStyle{Size: 99, Color: "#ff0000"}, Pad: 9, Text: "мусор", Over: true}
	v := e.SetSheetCaps("s1", false, junk)
	if v.Spreads[0].Caps != nil || edtest.P(e).Spreads[0].Caps != nil {
		t.Fatalf("стиль листа не снят: %+v", v.Spreads[0].Caps)
	}
	if cv := capOfPhoto(v.Spreads[0], "a"); cv == nil || cv.Sheet || cv.Pad != model.DefaultPad || cv.Over {
		t.Fatalf("подпись: %+v", cv)
	}
	if c := edtest.P(e).Photos["a"].Caption; c.Text != "Подпись" || c.Style.Size == 99 || c.Over {
		t.Fatalf("аргумент протёк в подпись: %+v", c)
	}
}

// SetCaption("") на фото, подпись которого только по шаблону листа, — холостая правка:
// ни ревизии, ни шага отмены.
func TestSetCaptionEmptyOnTemplateNoop(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	e.SetSheetCaps("s1", true, sheetTpl("Шаблон", 1))
	if cv := capOfPhoto(e.View().Spreads[0], "a"); cv == nil {
		t.Fatal("подписи по шаблону нет")
	}
	rev := edtest.Rev(e)
	v := e.SetCaption("a", "", "")
	if edtest.Rev(e) != rev || v.Rev != rev {
		t.Fatalf("ревизия %d → %d", rev, edtest.Rev(e))
	}
	// Отмена откатывает предыдущую правку (стиль листа), а не пустую подпись.
	if u := e.Undo(); u.Spreads[0].Caps != nil {
		t.Fatalf("отмена: стиль листа остался %+v", u.Spreads[0].Caps)
	}
}
