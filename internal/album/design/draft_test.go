package design_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Заготовки (131 design §6) и номер шага макета в конвейере правки (§3 «Нормализация»).

// Новая строка делает заготовку обычной надписью (одну, вторая остаётся), та же строка —
// холостая (rev стоит, признак стоит), стиль, рамка и поворот признак не снимают; отмена
// возвращает заготовку одним шагом.
func TestSetTextClearsDraft(t *testing.T) {
	ed, s1, s2 := edtest.NewTwo(t)
	var ids []string
	for _, sid := range []string{s1, s2} {
		id := edtest.AddText(t, ed, sid, 0)
		edtest.Change(ed, func(p *model.Project) {
			sp, i := p.FindText(id)
			sp.Texts[i].Text, sp.Texts[i].Draft = "[Имена]", true
		})
		ids = append(ids, id)
	}
	draft := func(id string) bool { return edtest.Block(t, ed, id).Draft }
	rev := ed.View().Rev
	if v := ed.SetText(ids[0], "[Имена]", "s1"); v.Rev != rev || !draft(ids[0]) {
		t.Fatalf("та же строка: rev %d → %d, заготовка %v", rev, v.Rev, draft(ids[0]))
	}
	st := edtest.Block(t, ed, ids[0]).Style
	st.Size += 4
	ed.SetTextStyle(ids[0], st)
	ed.SetTextBox(ids[0], 30, 30, 80)
	ed.SetTextAngle(ids[0], 15)
	if !draft(ids[0]) {
		t.Fatal("стиль, рамка или поворот сняли признак")
	}
	rev = ed.View().Rev
	v := ed.SetText(ids[0], "Анна и Михаил", "s2")
	if v.Rev != rev+1 || draft(ids[0]) || !draft(ids[1]) || edtest.Block(t, ed, ids[0]).Text != "Анна и Михаил" {
		t.Fatalf("новая строка: rev %d → %d, заготовки %v %v", rev, v.Rev, draft(ids[0]), draft(ids[1]))
	}
	ed.Undo()
	if !draft(ids[0]) || edtest.Block(t, ed, ids[0]).Text != "[Имена]" {
		t.Fatal("отмена не вернула заготовку")
	}
	// Пустая строка удаляет надпись — и заготовку.
	ed.SetText(ids[1], "  ", "")
	if sp, _ := edtest.P(ed).FindText(ids[1]); sp != nil {
		t.Fatal("пустая строка не удалила заготовку")
	}
}

// Конвейер правки держит номер шага: лист, у которого правка сняла места, фон и стикеры, теряет
// шаг (и, без фото, удаляется как опустевший); лист с фоном шаг хранит; без макета шагов нет.
func TestEditFixesLookStep(t *testing.T) {
	ed, s1, s2 := edtest.NewTwo(t)
	edtest.Change(ed, func(p *model.Project) {
		p.Look = &model.LookRef{Key: "my:0123456789abcdef"}
		p.Spreads[0].BgL, p.Spreads[0].LookStep = "#ffeeee", 2
		p.Spreads[1].BgR, p.Spreads[1].LookStep = "#eeeeff", 3
	})
	steps := func() (a, b int) {
		ss := edtest.P(ed).Spreads
		return ss[0].LookStep, ss[len(ss)-1].LookStep
	}
	if a, b := steps(); a != 2 || b != 3 {
		t.Fatalf("шаги %d %d", a, b)
	}
	ed.SetBackground(s2, "right", "")
	if a, b := steps(); a != 2 || b != 0 {
		t.Fatalf("фон снят: шаги %d %d", a, b)
	}
	ed.Undo()
	if _, b := steps(); b != 3 {
		t.Fatalf("отмена: шаг %d", b)
	}
	// Холостая правка шагов не трогает.
	rev := ed.View().Rev
	if v := ed.SetBackground(s1, "left", "#ffeeee"); v.Rev != rev {
		t.Fatal("тот же фон — не холостая")
	}
	edtest.Change(ed, func(p *model.Project) { p.Look = nil })
	if a, b := steps(); a != 0 || b != 0 {
		t.Fatalf("без макета: шаги %d %d", a, b)
	}
}

// «Сделать обычной надписью» (131 К1-r2-7): скобки снимаются, только если весь текст — одна пара
// с непустым содержимым; признак снимается у нескольких надписей по одной; повтор, чужой лист,
// неизвестная надпись — холостые; обложка находится по CoverID; отмена — один шаг.
func TestSetTextDraftBrackets(t *testing.T) {
	ed, s1, s2 := edtest.NewTwo(t)
	cases := []struct{ in, want string }{
		{"[Имена]", "Имена"},
		{"[ Анна и Михаил ]", "Анна и Михаил"},
		{"[Имя] и [Имя]", "[Имя] и [Имя]"},
		{"[[Имя]]", "[[Имя]]"},
		{"[ ]", "[ ]"},
		{"[]", "[]"},
		{"Дата: [число]", "Дата: [число]"},
		{"[дата", "[дата"},
		{"Свадьба", "Свадьба"},
	}
	ids := make([]string, len(cases))
	for i, c := range cases {
		ids[i] = edtest.AddText(t, ed, s1, i%2)
		edtest.Change(ed, func(p *model.Project) {
			sp, j := p.FindText(ids[i])
			sp.Texts[j].Text, sp.Texts[j].Draft = c.in, true
		})
	}
	other := edtest.AddText(t, ed, s2, 0)
	rev := ed.View().Rev
	if v := ed.SetTextDraft(s2, ids[0], false); v.Rev != rev || !edtest.Block(t, ed, ids[0]).Draft {
		t.Fatal("надпись чужого листа — не холостая")
	}
	if v := ed.SetTextDraft(s1, "нет", false); v.Rev != rev {
		t.Fatal("неизвестная надпись — не холостая")
	}
	if v := ed.SetTextDraft("нет-листа", ids[0], false); v.Rev != rev || !edtest.Block(t, ed, ids[0]).Draft {
		t.Fatal("неизвестный лист — не холостая")
	}
	if v := ed.SetTextDraft(s2, other, false); v.Rev != rev {
		t.Fatal("обычная надпись → обычная — не холостая")
	}
	for i, c := range cases {
		r := ed.View().Rev
		v := ed.SetTextDraft(s1, ids[i], false)
		b := edtest.Block(t, ed, ids[i])
		if v.Rev != r+1 || b.Draft || b.Text != c.want {
			t.Fatalf("%q: rev %d → %d, заготовка %v, текст %q, ждали %q", c.in, r, v.Rev, b.Draft, b.Text, c.want)
		}
		if i+1 < len(cases) && !edtest.Block(t, ed, ids[i+1]).Draft {
			t.Fatalf("%q: снят признак и у соседней", c.in)
		}
	}
	// Повтор холостой; отмена возвращает скобки и признак одним шагом.
	rev = ed.View().Rev
	if v := ed.SetTextDraft(s1, ids[0], false); v.Rev != rev {
		t.Fatal("повтор — не холостая")
	}
	ed.Undo()
	if b := edtest.Block(t, ed, ids[len(ids)-1]); !b.Draft || b.Text != "Свадьба" {
		t.Fatalf("отмена: %+v", b)
	}
	if b := edtest.Block(t, ed, ids[0]); b.Draft || b.Text != "Имена" {
		t.Fatalf("отмена задела другую надпись: %+v", b)
	}
	// Обратно в заготовку: скобки не добавляются; на обложке — по CoverID.
	ed.SetTextDraft(s1, ids[0], true)
	if b := edtest.Block(t, ed, ids[0]); !b.Draft || b.Text != "Имена" {
		t.Fatalf("в заготовку: %+v", b)
	}
	edtest.Change(ed, func(p *model.Project) {
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Photos: []string{},
			Texts: []*model.TextBlock{{ID: "c0ffee0000c2", X: 40, Y: 40, W: 80, Text: "[Фамилия]", Style: model.PlainStyle(), Draft: true}}}}
	})
	if ed.SetTextDraft(model.CoverID, "c0ffee0000c2", false); edtest.Block(t, ed, "c0ffee0000c2").Text != "Фамилия" {
		t.Fatalf("обложка: %+v", edtest.Block(t, ed, "c0ffee0000c2"))
	}
}
