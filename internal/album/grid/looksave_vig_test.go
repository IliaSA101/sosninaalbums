package grid_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// «Сохранить как макет» и лист-виньетка (164 G1): оформление листа — в Style и HText шага, надписи
// листа — нет (шаг ставится на обе страницы), Vars и имена людей — нет.

// vigSaved — альбом tplEd с виньеткой вторым разворотом; настроен — гарнитуры, цвет, подпись
// учителя, шапка, поля шапки, имя на месте и лишняя надпись листа.
func vigSaved(t *testing.T, styled bool) (looktpl.AlbumTpl, model.SaveLookOpts) {
	t.Helper()
	e := tplEd(t)
	if _, err := e.AddVignette(1); err != nil {
		t.Fatal(err)
	}
	s := edtest.P(e).Spreads[1]
	if styled {
		c := *s.Caps
		c.Style.Font, c.Style.Color = "nunito", "#5a4a42"
		e.SetSheetCaps(s.ID, true, c)
		h := s.VigHead(s.Vig.Head)
		hs := h.Style
		hs.Font, hs.Color = "marmelad", "#c0563a"
		e.SetTextStyle(h.ID, hs)
		e.SetText(h.ID, "Наш класс", "")
		edtest.Change(e, func(p *model.Project) {
			sp := p.Spreads[1]
			sp.Vars = &model.Vars{Title: "Школа 7", Year: "2026"}
			sp.Vig.TText = "{Фамилия}\n{Имя},\nучитель"
			sp.Grid[0].Person = model.Person{Last: "Иванова", First: "Анна"}
		})
	}
	edtest.Change(e, func(p *model.Project) { // своя надпись на листе-виньетке: на обе страницы шага она не пойдёт
		p.Spreads[1].Texts = append(p.Spreads[1].Texts, &model.TextBlock{ID: model.NewID(), Text: "Лишняя", Style: model.PlainStyle(), W: 50, Draft: true})
	})
	o := saveOpts("s1", edtest.P(e).Spreads[1].ID)
	tpl, err := e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	return tpl, o
}

// Гарнитура, цвет и подпись учителя уходят в шаг; htext — только при «с надписями»; Vars, имён людей
// и кегля в файле нет; круг записи и разбора даёт тот же шаг.
func TestSaveLookKeepsVigCaps(t *testing.T) {
	tpl, _ := vigSaved(t, true)
	v := tpl.Steps[1].Vig
	if v == nil || v.Style == nil || v.Style.Caps == nil || v.Style.Head == nil {
		t.Fatalf("оформление не снято: %+v", v)
	}
	if c := v.Style.Caps.Style; c.Font != "nunito" || c.Color != "#5a4a42" || c.Size != 0 {
		t.Errorf("подписи: %+v", c)
	}
	if h := v.Style.Head; h.Font != "marmelad" || h.Color != "#c0563a" || h.Size != 0 {
		t.Errorf("шапка: %+v", h)
	}
	if v.Style.TText != "{Фамилия}\n{Имя},\nучитель" || v.HText != "Наш класс" {
		t.Errorf("подпись учителя %q, htext %q", v.Style.TText, v.HText)
	}
	data, _ := json.Marshal(tpl)
	for _, bad := range []string{"Школа 7", "2026", "Иванова", "Анна", "vars", "Лишняя"} {
		if strings.Contains(string(data), bad) {
			t.Errorf("в файле макета %q: %s", bad, data)
		}
	}
	back, err := looktpl.ParseLook(data)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := json.Marshal(back)
	if string(again) != string(data) {
		t.Errorf("Save→Parse изменил макет:\n%s\n%s", data, again)
	}
	// Без «Надписи — заготовками» текст шапки в шаг не идёт, а гарнитура и цвет — идут.
	if tpl2, _ := vigSavedNoDrafts(t); tpl2.Steps[1].Vig.HText != "" || tpl2.Steps[1].Vig.Style == nil {
		t.Errorf("без заготовок: %+v", tpl2.Steps[1].Vig)
	}
}

// vigSavedNoDrafts — как vigSaved(styled), но без заготовок.
func vigSavedNoDrafts(t *testing.T) (looktpl.AlbumTpl, model.SaveLookOpts) {
	t.Helper()
	e := tplEd(t)
	if _, err := e.AddVignette(1); err != nil {
		t.Fatal(err)
	}
	s := edtest.P(e).Spreads[1]
	h := s.VigHead(s.Vig.Head)
	hs := h.Style
	hs.Font = "marmelad"
	e.SetTextStyle(h.ID, hs)
	e.SetText(h.ID, "Наш класс", "")
	o := saveOpts("s1", s.ID)
	o.Drafts = false
	tpl, err := e.AlbumTplOf(o)
	if err != nil {
		t.Fatal(err)
	}
	return tpl, o
}

// Лист без правок оформления даёт шаг как в 0.35: ни style, ни htext (умолчания в файл не пишутся).
func TestSaveLookVigPlainNoStyle(t *testing.T) {
	tpl, _ := vigSaved(t, false)
	if v := tpl.Steps[1].Vig; v == nil || v.Style != nil || v.HText != "" {
		t.Errorf("простой лист: %+v", v)
	}
}

// Надписи листа-виньетки в шаг не идут: ни texts, ни drafts (в том числе помеченные заготовкой).
func TestSaveLookVigNoTexts(t *testing.T) {
	tpl, _ := vigSaved(t, true)
	if st := tpl.Steps[1]; len(st.Tpl.Texts) != 0 || len(st.Tpl.Drafts) != 0 {
		t.Errorf("надписи листа-виньетки в шаге: texts %d, drafts %d", len(st.Tpl.Texts), len(st.Tpl.Drafts))
	}
}

// Простой лист на шаблонах «Руководитель…» (подпись предметника — умолчание шаблона) сохраняется без
// style и ttext, а своя подпись — уходит в шаг.
func TestSaveLookVigStaffDefault(t *testing.T) {
	for _, tpl := range []string{"lead-left", "head-lead-left", "split-lead"} {
		e := tplEd(t)
		if _, err := e.AddVignette(1); err != nil {
			t.Fatal(err)
		}
		s := edtest.P(e).Spreads[1]
		if _, _, err := e.SetVignette(s.ID, model.Vignette{Tpl: tpl, Teachers: 2, Pupils: 10, Ratio: "3:4"}, nil); err != nil {
			t.Fatal(err)
		}
		if got := edtest.P(e).Spreads[1].Vig.TText; got != model.StaffCaption {
			t.Fatalf("%s: подпись листа %q, ждали подпись предметника", tpl, got)
		}
		tplOf, err := e.AlbumTplOf(saveOpts("s1", s.ID))
		if err != nil {
			t.Fatal(err)
		}
		if v := tplOf.Steps[1].Vig; v == nil || v.Style != nil {
			t.Errorf("%s: простой лист: %+v", tpl, v)
		}
		edtest.Change(e, func(p *model.Project) { p.Spreads[1].Vig.TText = "{Фамилия}" })
		tplOf, err = e.AlbumTplOf(saveOpts("s1", s.ID))
		if err != nil || tplOf.Steps[1].Vig.Style == nil || tplOf.Steps[1].Vig.Style.TText != "{Фамилия}" {
			t.Errorf("%s: своя подпись не ушла в шаг: %v %+v", tpl, err, tplOf.Steps[1].Vig)
		}
	}
}
