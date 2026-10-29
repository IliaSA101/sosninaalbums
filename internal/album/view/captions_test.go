package view_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// sheetTpl — стиль листа с шаблоном подписи (030).
func sheetTpl(text string, pad float64) model.SheetCaps {
	return model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: pad, Text: text}
}

// rename — путь фото id в альбоме (имя из файла считается по нему).
func rename(e *album.Editor, id, file string) {
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[id]
		r.Path = edtest.Abs(file)
		p.Photos[id] = r
	})
}

func capOfPhoto(sv model.SpreadView, id string) *model.CaptionView {
	for i := range sv.Captions {
		if sv.Captions[i].Photo == id {
			return &sv.Captions[i]
		}
	}
	return nil
}

func TestCaptionPad(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(10, 120, 100, 100, "b")})
	e.SetCaption("a", "Подпись", "")
	st := model.DefaultCaptionStyle()
	cv := capOfPhoto(e.SetCaptionStyle("a", st, false, 1).Spreads[0], "a")
	if cv == nil || cv.Y != 111 || cv.Overflow != 0 {
		t.Fatalf("отступ 1 мм: %+v", cv)
	}
	// 4 мм: подпись на 4 мм ниже фото; ячейка ниже — тоже с отступом: 120 − 4 = 116.
	v := e.SetCaptionStyle("a", st, false, 4)
	cv = capOfPhoto(v.Spreads[0], "a")
	if want := math.Round((114+cv.H-116)*10) / 10; cv.Y != 114 || want <= 0 || cv.Overflow != want {
		t.Fatalf("отступ 4 мм: y %.2f, нехватка %.2f, ждали %.2f", cv.Y, cv.Overflow, want)
	}
	if pad := edtest.P(e).Photos["a"].Caption.PadOf(); pad != 4 {
		t.Fatalf("отступ в проекте %.1f", pad)
	}
	// На фото: от нижнего края pad + 2 мм (при 1 мм — прежние 3 мм).
	cv = capOfPhoto(e.SetCaptionStyle("a", st, true, 4).Spreads[0], "a")
	if math.Abs(cv.Y+cv.H-(110-4-2)) > 1e-6 {
		t.Fatalf("на фото: низ %.2f", cv.Y+cv.H)
	}
	// Стиль листа: его отступ действует на все подписи листа.
	v = e.SetSheetCaps("s1", true, sheetTpl("", 6))
	cv = capOfPhoto(v.Spreads[0], "a")
	if cv == nil || !cv.Sheet || cv.Over || cv.Y != 116 {
		t.Fatalf("отступ листа: %+v", cv)
	}
	// Мусор в отступе зажимается: NaN — 1 мм, 50 — 20.
	e.SetSheetCaps("s1", false, model.SheetCaps{})
	e.SetCaptionStyle("a", st, false, math.NaN())
	if edtest.P(e).Photos["a"].Caption.Pad != nil {
		t.Fatal("NaN — отступ по умолчанию")
	}
	e.SetCaptionStyle("a", st, false, 50)
	if pad := edtest.P(e).Photos["a"].Caption.PadOf(); pad != model.MaxPad {
		t.Fatalf("отступ 50 → %.1f", pad)
	}
}

func TestCaptionShrink(t *testing.T) {
	look := func(font string) view.Look {
		return view.Look{Style: model.ClampStyle(model.TextStyle{Font: font, Size: 10, Align: "center"}), Pad: 1, Fit: true}
	}
	in := func(l view.Look, w float64) view.CaptionIn {
		return view.CaptionIn{Text: "Константинопольская", Look: l, Rects: []layout.Rect{{X: 0, Y: 0, W: w, H: 30}}, PageH: 300}
	}
	// Узкая гарнитура: ужимается в одну строку, кегль ≥ 6 и кратен 0,5.
	cv := view.CaptionViewOf(in(look("oswald"), 20))
	if sz := cv.Style.Size; len(cv.Lines) != 1 || sz < 6 || sz >= 10 || math.Mod(sz, 0.5) != 0 || cv.Overflow != 0 {
		t.Fatalf("ужимание: кегль %.2f, строк %d, нехватка %.1f", sz, len(cv.Lines), cv.Overflow)
	}
	if cv.Lines[0].W > 20 {
		t.Fatalf("строка %.2f мм шире ячейки", cv.Lines[0].W)
	}
	// Широкая гарнитура не встаёт и при 6 пт: перенос и «!».
	cv = view.CaptionViewOf(in(look("montserrat"), 20))
	if cv.Style.Size != 6 || len(cv.Lines) < 2 || cv.Overflow <= 0 {
		t.Fatalf("предел 6 пт: кегль %.2f, строк %d, нехватка %.1f", cv.Style.Size, len(cv.Lines), cv.Overflow)
	}
	// Без «ужимать» (своя подпись, стиль листа с переносом) кегль прежний.
	l := look("oswald")
	l.Fit = false
	if cv = view.CaptionViewOf(in(l, 20)); cv.Style.Size != 10 || len(cv.Lines) < 2 {
		t.Fatalf("без ужимания: кегль %.2f, строк %d", cv.Style.Size, len(cv.Lines))
	}
	// Короткая строка не растёт и не ужимается.
	if cv = view.CaptionViewOf(in(look("oswald"), 60)); cv.Style.Size != 10 || len(cv.Lines) != 1 {
		t.Fatalf("короткая: %+v", cv.Style)
	}
	// Через Editor: стиль листа ужимает, Wrap — переносит.
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 20, 30, "a")})
	e.SetCaption("a", "Константинопольская", "")
	c := sheetTpl("", 1)
	c.Style = model.ClampStyle(model.TextStyle{Font: "oswald", Size: 10})
	if cv := capOfPhoto(e.SetSheetCaps("s1", true, c).Spreads[0], "a"); cv.Style.Size >= 10 || len(cv.Lines) != 1 {
		t.Fatalf("лист ужимает: %+v", cv.Style)
	}
	c.Wrap = true
	if cv := capOfPhoto(e.SetSheetCaps("s1", true, c).Spreads[0], "a"); cv.Style.Size != 10 || len(cv.Lines) < 2 {
		t.Fatalf("лист переносит: %+v", cv.Style)
	}
}

func TestStubsCanvasOnly(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a"), edtest.Gp(100, 10, 80, 100, "b"), edtest.Gp(190, 10, 80, 100, "")})
	rename(e, "a", "Иванов Петя.jpg")
	rename(e, "b", "DSC_0012.jpg")
	v := e.SetSheetCaps("s1", true, sheetTpl("{Фамилия}\n{Имя}", 1))
	sv := v.Spreads[0]
	if cv := capOfPhoto(sv, "a"); cv == nil || cv.Text != "Иванов\nПетя" || cv.Src != "{Фамилия}\n{Имя}" || !cv.Sheet || len(cv.Lines) != 2 {
		t.Fatalf("подпись по шаблону: %+v", cv)
	}
	if capOfPhoto(sv, "b") != nil || len(sv.Stubs) != 2 {
		t.Fatalf("фото без имени и пустое место — заглушки: подписей %d, заглушек %d", len(sv.Captions), len(sv.Stubs))
	}
	empty := sv.Places[2].ID
	for _, st := range sv.Stubs {
		if st.Text != "[Фамилия]\n[Имя]" || st.Place == "" || (st.Photo == "b") == (st.Place == empty) {
			t.Fatalf("заглушка: %+v", st)
		}
	}
	if v.Unnamed != 1 || len(v.Unfilled) != 0 {
		t.Fatalf("без имени %d, пустые поля %v", v.Unnamed, v.Unfilled)
	}
	// Своё имя — заглушка становится подписью; своя подпись заменяет текст шаблона.
	v = e.SetPerson("b", model.Person{Last: "Петрова", First: "Анна"}, "")
	if cv := capOfPhoto(v.Spreads[0], "b"); cv == nil || cv.Text != "Петрова\nАнна" || len(v.Spreads[0].Stubs) != 1 || v.Unnamed != 0 {
		t.Fatalf("имя вручную: %+v, заглушек %d", cv, len(v.Spreads[0].Stubs))
	}
	v = e.SetCaption("a", "Сам {Имя}", "")
	if cv := capOfPhoto(v.Spreads[0], "a"); cv == nil || cv.Text != "Сам Петя" || !cv.Sheet {
		t.Fatalf("своя подпись под стилем листа: %+v", cv)
	}
	// Вид печати (экспорт): ни заглушек, ни подписи фото без имени.
	e.SetPerson("b", model.Person{}, "")
	_, pv := e.Snapshot()
	ps := pv.Spreads[0]
	if len(ps.Stubs) != 0 || capOfPhoto(ps, "b") != nil || capOfPhoto(ps, "a") == nil || pv.Unnamed != 1 {
		t.Fatalf("печать: заглушек %d, подписи %+v", len(ps.Stubs), ps.Captions)
	}
	// «Авто»: заглушки фото без имени — тоже, место пустое.
	ea, id := edtest.AutoEd(t)
	av := ea.SetSheetCaps(id, true, sheetTpl("{Фамилия}", 1)).Spreads[0]
	if len(av.Stubs) != 3 || av.Stubs[0].Place != "" || av.Stubs[0].Photo == "" || av.Caps == nil || av.Caps.Text != "{Фамилия}" {
		t.Fatalf("«Авто»: %+v", av.Stubs)
	}
}

// setVars — поля шапки разворота i через конвейер правки (окно виньетки — 031 Т2).
func setVars(e *album.Editor, i int, v model.Vars) model.View {
	return edtest.Change(e, func(p *model.Project) { p.Spreads[i].Vars = model.ClampVars(v) })
}

func TestPrintViewBlanksFields(t *testing.T) {
	e, id := edtest.AutoEd(t)
	tid := edtest.AddText(t, e, id, 0)
	e.SetText(tid, "{Школа} {Год}", "")
	e.SetCaption("a", "Выпуск {школа}", "")
	v := setVars(e, 0, model.Vars{Year: "2026"})
	sv := v.Spreads[0]
	if tv := sv.Texts[0]; tv.Text != "[Заголовок] 2026" || tv.Src != "{Школа} {Год}" {
		t.Fatalf("холст: %q / %q", tv.Text, tv.Src)
	}
	if cv := capOfPhoto(sv, "a"); cv == nil || cv.Text != "Выпуск [Заголовок]" {
		t.Fatalf("подпись на холсте: %+v", cv)
	}
	if !slices.Equal(v.Unfilled, []string{"Заголовок"}) || sv.Vars.Year != "2026" {
		t.Fatalf("пустые поля %v, vars %+v", v.Unfilled, sv.Vars)
	}
	_, pv := e.Snapshot()
	ps := pv.Spreads[0]
	for _, tv := range append([]model.TextView{ps.Texts[0]}, ps.Captions[0].TextView) {
		for _, l := range tv.Lines {
			if strings.ContainsAny(l.Text, "[]{}") {
				t.Fatalf("в печати служебный текст: %q", l.Text)
			}
		}
	}
	if strings.TrimSpace(ps.Texts[0].Text) != "2026" || !slices.Equal(pv.Unfilled, []string{"Заголовок"}) {
		t.Fatalf("печать: %q, пустые %v", ps.Texts[0].Text, pv.Unfilled)
	}
	// Рамка «по тексту» в печати та же, что на холсте.
	if ps.Texts[0].W != sv.Texts[0].W {
		t.Fatalf("ширина рамки: печать %.2f, холст %.2f", ps.Texts[0].W, sv.Texts[0].W)
	}
	// Заполнили — пустых нет; ширина «по тексту» — по подставленному тексту.
	v = setVars(e, 0, model.Vars{Title: "Школа №1234 имени очень длинного названия", Year: "2026"})
	if len(v.Unfilled) != 0 || v.Spreads[0].Texts[0].W <= sv.Texts[0].W || v.Spreads[0].Texts[0].Src == "" {
		t.Fatalf("заполнено: %v, ширина %.2f → %.2f", v.Unfilled, sv.Texts[0].W, v.Spreads[0].Texts[0].W)
	}
}

// Поля имени подписи по шаблону (037): по порядку шаблона, у своей подписи и шаблона без
// имени — пусто; у заглушки пустого места — те же поля (071: окошко «Имя» места); несколько
// фото сразу; вид печати тот же.
func TestCaptionWhoFields(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a"), edtest.Gp(100, 10, 80, 100, "b"),
		edtest.Gp(190, 10, 80, 100, "c"), edtest.Gp(10, 130, 80, 100, "")})
	rename(e, "a", "Иванов Петя.jpg")
	rename(e, "b", "DSC_0012.jpg")
	rename(e, "c", "Петров Иван.jpg")
	// {Имя} раньше {Фамилия}, повтор и другой регистр — порядок появления, без повторов.
	v := e.SetSheetCaps("s1", true, sheetTpl("{имя} {Фамилия}\n{ИМЯ}", 1))
	sv := v.Spreads[0]
	want := []string{"first", "last"}
	if cv := capOfPhoto(sv, "a"); cv == nil || !slices.Equal(cv.Who, want) {
		t.Fatalf("подпись по шаблону: %+v", cv)
	}
	for _, st := range sv.Stubs {
		switch {
		case st.Photo == "b" && !slices.Equal(st.Who, want):
			t.Fatalf("заглушка фото без имени: %+v", st)
		case st.Photo == "" && !slices.Equal(st.Who, want):
			t.Fatalf("заглушка пустого места: %+v", st)
		}
	}
	if len(sv.Stubs) != 2 {
		t.Fatalf("заглушек %d", len(sv.Stubs))
	}
	// Своя подпись у одного из нескольких фото — только у неё Who пусто.
	sv = e.SetCaption("a", "Сам {Имя}", "").Spreads[0]
	if cv := capOfPhoto(sv, "a"); cv == nil || cv.Who != nil {
		t.Fatalf("своя подпись: %+v", cv)
	}
	if cv := capOfPhoto(sv, "c"); cv == nil || !slices.Equal(cv.Who, want) {
		t.Fatalf("чужая подпись по шаблону: %+v", cv)
	}
	// Шаблон без полей имени, пустой шаблон.
	for _, tpl := range []string{"{Год}", "Просто текст", ""} {
		v = e.SetSheetCaps("s1", true, sheetTpl(tpl, 1))
		for _, cv := range v.Spreads[0].Captions {
			if cv.Who != nil {
				t.Fatalf("шаблон %q: %+v", tpl, cv)
			}
		}
		for _, st := range v.Spreads[0].Stubs {
			if st.Who != nil {
				t.Fatalf("шаблон %q, заглушка: %+v", tpl, st)
			}
		}
	}
	// Печать: те же поля у подписи, текст не меняется.
	e.SetSheetCaps("s1", true, sheetTpl("{Фамилия}\n{Имя}\n{Отчество}", 1))
	_, pv := e.Snapshot()
	if cv := capOfPhoto(pv.Spreads[0], "c"); cv == nil || cv.Text != "Петров\nИван" || !slices.Equal(cv.Who, []string{"last", "first", "middle"}) {
		t.Fatalf("печать: %+v", cv)
	}
}

// Учитель виньетки с отчеством в своём шаблоне — три поля, ученик — два.
func TestCaptionWhoTeacher(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 80, 100, "a"), edtest.Gp(100, 10, 80, 100, "b")})
	rename(e, "a", "Иванова Анна.jpg")
	rename(e, "b", "Петров Иван.jpg")
	e.SetSheetCaps("s1", true, sheetTpl("{Фамилия}\n{Имя}", 1))
	v := edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		s.Kind, s.Vig = model.KindVignette, &model.Vignette{TText: "{Фамилия}\n{Имя} {Отчество}"}
		s.Grid[0].Role = model.RoleTeacher
	})
	sv := v.Spreads[0]
	if t0, st := capOfPhoto(sv, "a"), capOfPhoto(sv, "b"); t0 == nil || st == nil ||
		!slices.Equal(t0.Who, []string{"last", "first", "middle"}) || !slices.Equal(st.Who, []string{"last", "first"}) {
		t.Fatalf("учитель %+v, ученик %+v", t0, st)
	}
}
