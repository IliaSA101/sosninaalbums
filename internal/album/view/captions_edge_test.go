package view_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// LookOf: ни своего стиля, ни стиля листа — прежний вид (отступ по умолчанию, без ужимания).
func TestLookOfDefaults(t *testing.T) {
	l := view.LookOf(nil, nil)
	if l.Pad != view.CaptionPad || l.Fit || l.Sheet || l.Over {
		t.Fatalf("без стилей: %+v", l)
	}
	st := model.DefaultCaptionStyle()
	st.Size = 9
	if l = view.LookOf(&model.Caption{Style: st}, nil); l.Style.Size != 9 || l.Pad != model.DefaultPad || l.Fit || l.Sheet {
		t.Fatalf("своя подпись: %+v", l)
	}
	if l = view.LookOf(&model.Caption{Style: st}, &model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 5}); l.Style.Size == 9 || l.Pad != 5 || !l.Fit || !l.Sheet {
		t.Fatalf("лист сильнее своей: %+v", l)
	}
}

// Ужимание при любой ширине ячейки: строка встаёт в ширину либо кегль упёрся в 6 пт и
// подпись помечена «!». Шаг ширины мелкий, чтобы пройти и округление кегля в кэше.
func TestCaptionShrinkAnyWidth(t *testing.T) {
	look := view.Look{Style: model.ClampStyle(model.TextStyle{Font: "oswald", Size: 10, Align: "center"}), Pad: 1, Fit: true}
	// Грубая сетка по всему диапазону и частая у 18,63 мм: там кегль по пропорции
	// после шага 0,5 пт ещё не встаёт в ширину и добирается циклом.
	var widths []float64
	for w := 12.0; w <= 40; w += 0.5 {
		widths = append(widths, w)
	}
	for w := 18.55; w <= 18.7; w += 0.01 {
		widths = append(widths, w)
	}
	for _, w := range widths {
		cv := view.CaptionViewOf(view.CaptionIn{Text: "Константинопольская", Look: look,
			Rects: []layout.Rect{{X: 0, Y: 0, W: w, H: 30}}, PageH: 300})
		fits := len(cv.Lines) == 1 && cv.Lines[0].W <= w
		if sz := cv.Style.Size; sz < 6 || sz > 10 || !fits && (sz != 6 || cv.Overflow <= 0) {
			t.Fatalf("ширина %.2f: кегль %.2f, строк %d, нехватка %.1f", w, sz, len(cv.Lines), cv.Overflow)
		}
	}
}

// Шаблон листа с полем имени у фото, имя которого заполнено частично: подпись по шаблону
// остаётся, пока заполнено хоть одно поле имени; заглушка — когда не заполнено ни одно.
func TestStubPartlyNamed(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a")})
	rename(e, "a", "Иванов Петя.jpg")
	sv := e.SetSheetCaps("s1", true, sheetTpl("{Фамилия} {Отчество}", 1)).Spreads[0]
	if cv := capOfPhoto(sv, "a"); cv == nil || cv.Text != "Иванов [Отчество]" || len(sv.Stubs) != 0 {
		t.Fatalf("часть имени: %+v, заглушек %d", cv, len(sv.Stubs))
	}
	v := e.SetSheetCaps("s1", true, sheetTpl("{Отчество}", 1))
	sv = v.Spreads[0]
	if capOfPhoto(sv, "a") != nil || len(sv.Stubs) != 1 || sv.Stubs[0].Text != "[Отчество]" || v.Unnamed != 1 {
		t.Fatalf("нет отчества: подписей %d, заглушек %+v, без имени %d", len(sv.Captions), sv.Stubs, v.Unnamed)
	}
}
