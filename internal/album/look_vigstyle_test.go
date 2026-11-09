package album_test

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Оформление шага-виньетки макета (164 G1): гарнитура и цвет подписей и шапки, подпись учителя, текст
// шапки. Кегль считает программа по формату; стиль встаёт до раскладки, поэтому лист нетронут.

// vsFormats — форматы страницы, мм (20×20, 20×30, 30×30, 40×30 см).
var vsFormats = []struct {
	name string
	w, h float64
}{{"20×20", 200, 200}, {"20×30", 200, 300}, {"30×30", 300, 300}, {"40×30", 400, 300}}

const vsTText = "{Фамилия}\n{Имя} {Отчество},\nвоспитатель"

// vsStyle — оформление «Наших воспитателей» (пример design §1).
func vsStyle() *model.VigStyle {
	return &model.VigStyle{
		Head:  &model.TextStyle{Font: "marmelad", Color: "#c0563a"},
		Caps:  &model.SheetCaps{Style: model.TextStyle{Font: "nunito", Color: "#5a4a42"}},
		TText: vsTText,
	}
}

// vsLook — макет: обычный шаг и «Наши воспитатели» (split-g, 6 учителей); style и htext — в шаге.
func vsLook(style *model.VigStyle, htext string) looktpl.AlbumTpl {
	vig := lkVig("Воспитатели", 0)
	vig.Vig = &looktpl.LookVig{Tpl: "split-g", Teachers: 6, Pupils: 0, Ratio: "3:4", Style: style, HText: htext}
	return looktpl.AlbumTpl{V: looktpl.LookV, Steps: []looktpl.LookStep{lkStep("", "Титул", "#ffffff", 1), vig}}
}

// vsEditor — редактор с форматом w×h мм.
func vsEditor(t *testing.T, w, h float64) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = w, h
	return edtest.Open(t, p, "")
}

// vsSheet — лист «Воспитатели» (второй) редактора и его шапка.
func vsSheet(e *album.Editor) (*model.Spread, *model.TextBlock) {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	s := edtest.P(e).Spreads[1]
	return s, s.VigHead(s.Vig.Head)
}

// vsManual — лист i помечен «ручным» (места не по плану: «!» у мест).
func vsManual(e *album.Editor, i int) bool {
	v := e.View().Spreads[i].Vig
	return v == nil || v.Manual
}

// Гарнитура и цвет шапки и подписей, подпись учителя — из шага; кегль — как у листа без шага; лист
// нетронут; один ApplyLook — один шаг отмены. Учителей больше имён — пустые подписи.
func TestApplyLookVigStyle(t *testing.T) {
	for _, f := range vsFormats {
		t.Run(f.name, func(t *testing.T) { vsCheckStyle(t, f.w, f.h) })
	}
}

// vsCheckStyle — проверки TestApplyLookVigStyle для формата w×h мм.
func vsCheckStyle(t *testing.T, pageW, pageH float64) {
	t.Helper()
	plain := vsEditor(t, pageW, pageH)
	lkApply(t, plain, lkRefA, vsLook(nil, ""), nil, 2)
	ps, ph := vsSheet(plain)

	e := vsEditor(t, pageW, pageH)
	lkApply(t, e, lkRefA, vsLook(vsStyle(), ""), nil, 2)
	if n := edtest.UndoLen(e); n != 1 {
		t.Fatalf("шагов отмены %d, ждали 1", n)
	}
	s, h := vsSheet(e)
	if s.Caps == nil || h == nil {
		t.Fatalf("нет подписей или шапки: %+v", s.Caps)
	}
	if s.Caps.Style.Font != "nunito" || s.Caps.Style.Color != "#5a4a42" || s.Vig.TText != vsTText {
		t.Errorf("подписи: %+v, подпись учителя %q", s.Caps.Style, s.Vig.TText)
	}
	if h.Style.Font != "marmelad" || h.Style.Color != "#c0563a" {
		t.Errorf("шапка: %+v", h.Style)
	}
	// Кегль, отступ, шаблон подписи, выравнивание — как у листа без шага.
	wc := ps.Caps.Style
	wc.Font, wc.Color = "nunito", "#5a4a42"
	if s.Caps.Style != wc || s.Caps.Pad != ps.Caps.Pad || s.Caps.Text != ps.Caps.Text {
		t.Errorf("подписи: %+v, ждали %+v (кегль формата)", *s.Caps, *ps.Caps)
	}
	wh := ph.Style
	wh.Font, wh.Color = "marmelad", "#c0563a"
	if h.Style != wh {
		t.Errorf("шапка: %+v, ждали %+v (кегль плана)", h.Style, wh)
	}
	if len(s.Grid) != len(ps.Grid) { // места сдвигаются от метрик гарнитуры подписей, их число — нет
		t.Fatalf("мест %d, у листа без шага %d", len(s.Grid), len(ps.Grid))
	}
	if vsManual(e, 1) {
		t.Error("лист тронут: «!» у мест после постановки")
	}
	// Учителей шестеро, имён нет: пустые подписи «[Фамилия]» (FieldStub), без паники и ошибок.
	if got, _ := model.Shown(nil, s.Vig.TText, nil, model.FieldStub); got != "[Фамилия]\n[Имя] [Отчество],\nвоспитатель" {
		t.Errorf("пустая подпись: %q", got)
	}
}

// Шаг на листе с тем же шаблоном, что у нового листа по умолчанию (head-t-left): оформление встаёт,
// потому что лист создан макетом.
func TestApplyLookVigStyleDefaultTpl(t *testing.T) {
	e := lkEditor(t)
	b := lookA()
	b.Steps[1].Vig.Style, b.Steps[1].Vig.HText = vsStyle(), "Наш класс"
	lkApply(t, e, lkRefA, b, nil, 9)
	s := edtest.P(e).Spreads[1]
	h := s.VigHead(s.Vig.Head)
	if s.Caps.Style.Font != "nunito" || s.Vig.TText != vsTText || h == nil || h.Style.Font != "marmelad" || h.Text != "Наш класс" {
		t.Fatalf("стиль не встал на новый лист с шаблоном по умолчанию: %+v, %+v", s.Caps, h)
	}
}

// Шапка — из htext, поля шапки листа не тронуты, в печати нет « · ».
func TestApplyLookVigHeadText(t *testing.T) {
	e := vsEditor(t, 300, 300)
	lkApply(t, e, lkRefA, vsLook(vsStyle(), "Наши воспитатели"), nil, 2)
	s, h := vsSheet(e)
	if h == nil || h.Text != "Наши воспитатели" {
		t.Fatalf("шапка: %+v", h)
	}
	if s.Vars != nil && *s.Vars != (model.Vars{}) {
		t.Errorf("Vars тронуты: %+v", *s.Vars)
	}
	if got, _ := model.Shown(s.Vars, h.Text, nil, model.FieldPrint); got != "Наши воспитатели" || len(got) == 0 {
		t.Errorf("печать шапки: %q", got)
	}
	// Без htext — шапка по умолчанию, как в 0.35.
	e2 := vsEditor(t, 300, 300)
	lkApply(t, e2, lkRefA, vsLook(vsStyle(), ""), nil, 2)
	if _, h2 := vsSheet(e2); h2 == nil || h2.Text != model.VigHeadText {
		t.Errorf("шапка без htext: %+v", h2)
	}
}

// Смена шаблона на существующем листе: шаблонная шапка получает htext; правленая остаётся.
func TestApplyLookVigHeadTextSwitch(t *testing.T) {
	e := vsEditor(t, 300, 300)
	lkApply(t, e, lkRefA, vsLook(nil, ""), nil, 2) // split-g, шапка по умолчанию
	// Другой шаблон (g-top) и htext: шапка листа шаблонная — текст шага вместо неё.
	b := vsLook(vsStyle(), "Наш класс")
	b.Steps[1].Vig.Tpl = "head-g-top"
	lkApply(t, e, lkRefB, b, lkPrev(vsLook(nil, "")), 2)
	if s, h := vsSheet(e); s.Vig.Tpl != "head-g-top" || h == nil || h.Text != "Наш класс" || h.Style.Font != "marmelad" {
		t.Fatalf("после смены шаблона: %q, %+v", s.Vig.Tpl, h)
	}
	// Фотограф правит шапку; повтор того же макета и смена шаблона текст не затирают.
	_, h := vsSheet(e)
	e.SetText(h.ID, "Вписано вручную", "")
	b.Steps[1].Vig.Tpl = "split-g"
	b.Steps[1].Vig.HText = "Другой заголовок"
	lkApply(t, e, lkRefB, b, lkPrev(b), 2)
	if _, h = vsSheet(e); h == nil || h.Text != "Вписано вручную" {
		t.Fatalf("правленая шапка затёрта: %+v", h)
	}
}

// Шаг без новых полей — лист как в 0.35: та же постановка, что у SetVignette с теми же числами.
func TestApplyLookVigKeepsOld(t *testing.T) {
	for _, f := range vsFormats {
		t.Run(f.name, func(t *testing.T) {
			e := vsEditor(t, f.w, f.h)
			lkApply(t, e, lkRefA, vsLook(nil, ""), nil, 2)
			s, h := vsSheet(e)

			old := vsEditor(t, f.w, f.h) // путь 0.35: AddVignette и SetVignette шага
			if _, err := old.AddVignette(-1); err != nil {
				t.Fatal(err)
			}
			id := edtest.P(old).Spreads[0].ID
			if _, _, err := old.SetVignette(id, model.Vignette{Tpl: "split-g", Teachers: 6, Pupils: 0, Ratio: "3:4"}, nil); err != nil {
				t.Fatal(err)
			}
			os := edtest.P(old).Spreads[0]
			oh := os.VigHead(os.Vig.Head)
			if *s.Caps != *os.Caps || s.Vig.TText != os.Vig.TText || h == nil || oh == nil || h.Text != oh.Text || h.Style != oh.Style || h.W != oh.W || h.Y != oh.Y {
				t.Errorf("лист не как в 0.35: подписи %+v / %+v, шапка %+v / %+v", s.Caps, os.Caps, h, oh)
			}
			if len(s.Grid) != len(os.Grid) {
				t.Fatalf("мест %d, в 0.35 — %d", len(s.Grid), len(os.Grid))
			}
			for i := range s.Grid {
				a, b := s.Grid[i], os.Grid[i]
				if a.X != b.X || a.Y != b.Y || a.W != b.W || a.H != b.H || a.Role != b.Role {
					t.Fatalf("место %d: %+v, в 0.35 — %+v", i, a, b)
				}
			}
		})
	}
}

// Правки фотографа в подписях и шапке повторное «Применить» не сбрасывает; нетронутый лист — no-op.
func TestLookUntouchedVigCaps(t *testing.T) {
	e := vsEditor(t, 300, 300)
	look := vsLook(vsStyle(), "Наши воспитатели")
	lkApply(t, e, lkRefA, look, nil, 2)
	// Нетронутый лист: повтор холостой (rev, шаги отмены, JSON те же).
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	if plan := lkApply(t, e, lkRefA, look, nil, 2); plan.Touched != 0 || e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatalf("повтор на нетронутом: тронутых %d, rev %d → %d, шагов %d → %d", plan.Touched, rev, e.View().Rev, undo, edtest.UndoLen(e))
	}
	// Фотограф: другая гарнитура и кегль подписей (полоса другая — места не по плану), другая шапка.
	s, h := vsSheet(e)
	c := *s.Caps
	c.Style.Font, c.Style.Color, c.Style.Size = "lora", "#112233", c.Style.Size+3
	e.SetSheetCaps(s.ID, true, c)
	hs := h.Style
	hs.Font, hs.Color = "lora", "#445566"
	e.SetTextStyle(h.ID, hs)
	e.SetText(h.ID, "Своё название", "")
	want, wantH := *edtest.P(e).Spreads[1].Caps, *h
	for range 2 { // и тронутый (ResetVignette), и поправленный лист — оформление остаётся
		lkApply(t, e, lkRefA, look, nil, 2)
		s, h = vsSheet(e)
		if *s.Caps != want || h == nil || h.Style != wantH.Style || h.Text != wantH.Text {
			t.Fatalf("правки фотографа сброшены: подписи %+v (были %+v), шапка %+v (была %+v)", s.Caps, want, h, wantH)
		}
	}
}

// Save→Load: лист с оформлением шага — в файле альбома; повтор макета после Load холостой.
func TestApplyLookVigStyleSaveLoad(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "vs.salbum")
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 300, 300
	e := edtest.Open(t, p, path)
	look := vsLook(vsStyle(), "Наши воспитатели")
	lkApply(t, e, lkRefA, look, nil, 2)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	loaded, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e2 := edtest.Open(t, loaded, path)
	s, h := vsSheet(e2)
	if s.Caps.Style.Font != "nunito" || s.Vig.TText != vsTText || h == nil || h.Text != "Наши воспитатели" || h.Style.Font != "marmelad" {
		t.Fatalf("после Load: %+v, %+v", s.Caps, h)
	}
	rev := e2.View().Rev
	if lkApply(t, e2, lkRefA, look, nil, 2); e2.View().Rev != rev {
		t.Errorf("повтор после Load не холостой: rev %d → %d", rev, e2.View().Rev)
	}
	if vsManual(e2, 1) {
		t.Error("после Load лист тронут")
	}
}

// Отмена: один Undo возвращает пустой альбом, Redo — лист с оформлением.
func TestApplyLookVigStyleUndo(t *testing.T) {
	e := vsEditor(t, 300, 300)
	lkApply(t, e, lkRefA, vsLook(vsStyle(), "Наши воспитатели"), nil, 2)
	done := lkJSON(e)
	e.Undo()
	if n := len(edtest.P(e).Spreads); n != 0 {
		t.Fatalf("после Undo %d разворотов", n)
	}
	e.Redo()
	if lkJSON(e) != done {
		t.Fatal("Redo не вернул лист с оформлением")
	}
}

// Вписанные фотографом поля шапки (Vars) держат шапку «Заголовок · Год»: смена шаблона макетом
// текст шага вместо неё не ставит и поля не трогает; при пустых полях — ставит.
func TestApplyLookVigHeadTextKeepsVars(t *testing.T) {
	e := vsEditor(t, 300, 300)
	lkApply(t, e, lkRefA, vsLook(nil, ""), nil, 2) // split-g, шапка по умолчанию
	vars := model.Vars{Title: "Сад 5", Subtitle: "Ромашки", Year: "2026"}
	edtest.Change(e, func(p *model.Project) { v := vars; p.Spreads[1].Vars = &v })
	b := vsLook(vsStyle(), "Наши воспитатели")
	b.Steps[1].Vig.Tpl = "head-g-top"
	lkApply(t, e, lkRefB, b, lkPrev(vsLook(nil, "")), 2)
	s, h := vsSheet(e)
	if s.Vig.Tpl != "head-g-top" || h == nil || h.Text != model.VigHeadText {
		t.Fatalf("шапка при заполненных Vars: %q, %+v", s.Vig.Tpl, h)
	}
	if s.Vars == nil || *s.Vars != vars {
		t.Errorf("Vars тронуты: %+v", s.Vars)
	}
	if h.Style.Font != "marmelad" { // стиль шага при смене шаблона встаёт, текст — нет
		t.Errorf("стиль шапки: %+v", h.Style)
	}
}

// Два шага-виньетки в одном макете — у каждого свой htext и стиль.
func TestApplyLookVigTwoSheets(t *testing.T) {
	e := vsEditor(t, 300, 300)
	look := vsLook(vsStyle(), "Наши воспитатели")
	second := vsLook(&model.VigStyle{Head: &model.TextStyle{Font: "lora", Color: "#112233"}}, "Наш класс").Steps[1]
	second.Vig.Tpl, second.Vig.Teachers, second.Vig.Pupils = "head-t-left", 1, 20
	look.Steps = append(look.Steps, second)
	lkApply(t, e, lkRefA, look, nil, 3)
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	for i, want := range []struct{ text, font string }{{"Наши воспитатели", "marmelad"}, {"Наш класс", "lora"}} {
		s := edtest.P(e).Spreads[i+1]
		h := s.VigHead(s.Vig.Head)
		if h == nil || h.Text != want.text || h.Style.Font != want.font {
			t.Errorf("лист %d: %+v, ждали %+v", i+1, h, want)
		}
	}
}
