package view_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Завершающий \n в тексте не растит рамку, и блок у низа листа остаётся на месте
// (FitTexts его не поднимает): рамка = строки, видимые в редакторе.
func TestTextTrailingNewlineHeight(t *testing.T) {
	e, id := edtest.AutoEd(t)
	t1 := edtest.AddText(t, e, id, 0)
	t2 := edtest.AddText(t, e, id, 0)
	st := model.TextStyle{Font: "greatvibes", Size: 80, Color: "#222222", Align: "left"}
	find := func(v model.View, tid string) model.TextView {
		for _, s := range v.Spreads {
			for _, tv := range s.Texts {
				if tv.ID == tid {
					return tv
				}
			}
		}
		t.Fatalf("блока %s нет", tid)
		return model.TextView{}
	}
	var ph, h1 float64
	for _, tid := range []string{t1, t2} {
		e.SetTextStyle(tid, st)
		v := e.SetText(tid, "Сава и Вася", "")
		tv := find(v, tid)
		if len(tv.Lines) != 1 {
			t.Fatalf("исходный текст в %d строк", len(tv.Lines))
		}
		ph, h1 = v.Settings.PageH, tv.H
	}
	// оба блока у самого низа листа, строго по высоте одной строки
	y := math.Floor((ph-h1)*100) / 100
	e.SetTextBox(t1, 10, y, 200)
	e.SetTextBox(t2, 250, y, 200)
	e.SetText(t1, "Сава и Вася\n", "")
	v := e.SetText(t2, "Сава и Вася\n", "")
	for _, tid := range []string{t1, t2} {
		tv := find(v, tid)
		if tv.H != h1 || len(tv.Lines) != 1 {
			t.Errorf("%s: H %.3f (нужно %.3f), строк %d", tid, tv.H, h1, len(tv.Lines))
		}
		if tv.Y != y {
			t.Errorf("%s: блок поднят FitTexts: y %.2f → %.2f", tid, y, tv.Y)
		}
	}
	// пустая строка после Enter — видна в редакторе, значит растит рамку
	if tv := find(e.SetText(t1, "Сава и Вася\n\n", ""), t1); len(tv.Lines) != 2 || tv.H <= h1 {
		t.Errorf("два \\n: строк %d, H %.3f", len(tv.Lines), tv.H)
	}
}

// checkBaselines — контракт Go ↔ UI (156): в осях блока (у повёрнутого — до поворота)
// Lines[0].Y − Y = asc, Y + H − Lines[n−1].Y = desc, шаг строк 1,2 кегля. Редактор
// выравнивает свои строки по этим числам, поэтому сдвиг рамки без строк или строк без
// рамки (как у turnCaption) сдвинул бы текст при входе в правку. asc и desc берутся из
// одной строки того же стиля; их связь с hhea держит TestLayoutBaselineModel.
func checkBaselines(t *testing.T, label string, tv model.TextView, n int) {
	t.Helper()
	const tol = 1e-6
	if len(tv.Lines) != n {
		t.Errorf("%s: строк %d, нужно %d", label, len(tv.Lines), n)
		return
	}
	ref := text.Layout("Аб", tv.Style, 1000)
	asc := ref.Lines[0].Y
	desc := ref.H - asc
	if !(asc > 0) || !(desc > 0) {
		t.Fatalf("%s: опорная раскладка asc %.3f, desc %.3f", label, asc, desc)
	}
	if got := tv.Lines[0].Y - tv.Y; math.Abs(got-asc) > tol {
		t.Errorf("%s: Lines[0].Y − Y = %.6f, нужно asc %.6f", label, got, asc)
	}
	if got := tv.Y + tv.H - tv.Lines[n-1].Y; math.Abs(got-desc) > tol {
		t.Errorf("%s: Y + H − Lines[n−1].Y = %.6f, нужно desc %.6f", label, got, desc)
	}
	step := 1.2 * tv.Style.Size * 25.4 / 72
	for i := 1; i < n; i++ {
		if got := tv.Lines[i].Y - tv.Lines[i-1].Y; math.Abs(got-step) > tol {
			t.Errorf("%s: шаг строк %d = %.6f, нужно %.6f", label, i, got, step)
		}
	}
}

// Вид отдаёт asc и desc через рамку и строки: у текста (прямого, повёрнутого, «по
// тексту»), подписи под фото, на фото, у повёрнутого места и с вписыванием; 1 и 3 строки.
func TestTextViewBaselineOffsets(t *testing.T) {
	for _, font := range []string{"badscript", "neucha", "montserrat"} {
		for _, n := range []int{1, 3} {
			src := "Аб"
			for i := 1; i < n; i++ {
				src += "\nВг"
			}
			st := model.TextStyle{Font: font, Size: 80, Color: "#222222", Align: "left"}
			checkFreeTexts(t, font, src, st, n)
			checkPlaceCaptions(t, font, src, st, n)
		}
		checkEmptyBaseline(t, font)
	}
}

// checkEmptyBaseline — пустой текст: в виде его нет (SetText с пустой строкой удаляет блок,
// design.SetText), но раскладка, из которой берутся Lines, даёт одну строку с тем же asc и
// desc, что и у непустого. На это опирается курсор редактора: пока в нём пусто, он стоит
// на первой строке Go (156).
func checkEmptyBaseline(t *testing.T, font string) {
	t.Helper()
	const tol = 1e-6
	st := model.TextStyle{Font: font, Size: 80, Color: "#222222", Align: "left"}
	ref := text.Layout("Аб", st, 1000)
	empty := text.Layout("", st, 1000)
	if len(empty.Lines) != 1 {
		t.Fatalf("%s пусто: строк %d, нужна 1", font, len(empty.Lines))
	}
	if !(empty.Lines[0].Y > 0) {
		t.Errorf("%s пусто: смещение первой базы %.6f не определено", font, empty.Lines[0].Y)
	}
	if math.Abs(empty.Lines[0].Y-ref.Lines[0].Y) > tol || math.Abs(empty.H-ref.H) > tol {
		t.Errorf("%s пусто: asc %.6f, H %.6f; у непустого asc %.6f, H %.6f", font, empty.Lines[0].Y, empty.H, ref.Lines[0].Y, ref.H)
	}
}

// checkFreeTexts — свободные тексты: прямо, 15° и «по тексту».
func checkFreeTexts(t *testing.T, font, src string, st model.TextStyle, n int) {
	t.Helper()
	e, sid := edtest.AutoEd(t)
	ids := map[string]string{}
	for _, k := range []string{"прямо", "15°", "по тексту"} {
		id := edtest.AddText(t, e, sid, 0)
		e.SetTextStyle(id, st)
		e.SetText(id, src, "")
		ids[k] = id
	}
	e.SetTextBox(ids["прямо"], 10, 10, 200)
	e.SetTextBox(ids["15°"], 10, 80, 200)
	e.SetTextAngle(ids["15°"], 15)
	e.SetTextAuto(ids["по тексту"])
	byID := map[string]model.TextView{}
	for _, tv := range e.View().Spreads[0].Texts {
		byID[tv.ID] = tv
	}
	for k, id := range ids {
		tv, ok := byID[id]
		if !ok {
			t.Errorf("%s текст %s: нет в виде", font, k)
			continue
		}
		checkBaselines(t, font+" текст "+k, tv, n)
	}
	if rot := byID[ids["15°"]].Rot; rot != 15 {
		t.Errorf("%s: поворот текста %.1f, нужно 15", font, rot)
	}
}

// checkPlaceCaptions — подписи: под фото, на фото, у повёрнутого места (под и на фото).
func checkPlaceCaptions(t *testing.T, font, src string, st model.TextStyle, n int) {
	t.Helper()
	turned, turnedOver := edtest.Gp(10, 120, 100, 80, "c"), edtest.Gp(130, 120, 100, 80, "d")
	turned.Rot, turnedOver.Rot = 30, -20
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 80, "a"), edtest.Gp(130, 10, 100, 80, "b"), turned, turnedOver})
	cases := []struct {
		photo, label string
		over         bool
		rot          float64
	}{
		{"a", "под фото", false, 0},
		{"b", "на фото", true, 0},
		{"c", "повёрнутое место", false, 30},
		{"d", "повёрнутое место, на фото", true, -20},
	}
	for _, c := range cases {
		e.SetCaption(c.photo, src, "")
		e.SetCaptionStyle(c.photo, st, c.over, 1)
	}
	sv := e.View().Spreads[0]
	for _, c := range cases {
		cv := capOfPhoto(sv, c.photo)
		if cv == nil {
			t.Errorf("%s подпись %s: нет в виде", font, c.label)
			continue
		}
		if cv.Rot != c.rot {
			t.Errorf("%s подпись %s: Rot %.1f, нужно %.1f", font, c.label, cv.Rot, c.rot)
		}
		checkBaselines(t, font+" подпись "+c.label, cv.TextView, n)
	}
}

// Подпись с вписыванием (стиль листа без переноса): кегль меняет каждый ответ Go,
// а asc и desc по-прежнему считаются от действующего кегля, в том числе у повёрнутого места.
func TestCaptionFitViewBaselineOffsets(t *testing.T) {
	rot := edtest.Gp(130, 10, 60, 80, "b")
	rot.Rot = 25
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 60, 80, "a"), rot})
	for _, id := range []string{"a", "b"} {
		e.SetCaption(id, "Иванова Мария Сергеевна", "")
	}
	sv := e.SetSheetCaps("s1", true, model.SheetCaps{
		Style: model.TextStyle{Font: "badscript", Size: 80, Color: "#222222", Align: "left"}, Pad: 1,
	}).Spreads[0]
	for _, id := range []string{"a", "b"} {
		cv := capOfPhoto(sv, id)
		if cv == nil {
			t.Fatalf("подпись %s: нет в виде", id)
		}
		if cv.Style.Size >= 80 {
			t.Fatalf("подпись %s: кегль %.1f не вписан, проверка вырождена", id, cv.Style.Size)
		}
		checkBaselines(t, "вписанная подпись "+id, cv.TextView, 1)
	}
}
