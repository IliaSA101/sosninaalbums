package album

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// Шапка в ряду у шага макета (164): кегль «по полосе» меряется текстом и гарнитурой шага, а не
// умолчанием, пока шапки на листе ещё нет.

func TestVigPlanOfHeadFont(t *testing.T) {
	p := NewVignetteSheet(204, 304)
	s := p.Spreads[0]
	v := *s.Vig
	v.Tpl, v.Pupils, v.Head = "t-left-cell", 40, "" // шапки нет — новая встанет гарнитурой и текстом шага
	base := vigPlan(p, s, v)
	if !base.HeadCell || base.Head == nil {
		t.Fatalf("«t-left-cell»: шапки в ряду нет: %+v", base.Head)
	}
	serif := model.PlainStyle()
	serif.Font = "ptserif"
	const head = "Наши воспитатели"
	wantFont, def := headFit(*base.Head, serif), headFit(*base.Head, model.PlainStyle())
	if got := vigPlanOf("", "ptserif")(p, s, v).HeadFit; got != wantFont || wantFont == def {
		t.Errorf("гарнитура шага: кегль %v, ждали %v (умолчание дало бы %v)", got, wantFont, def)
	}
	if got, want := vigPlanOf(head, "")(p, s, v).HeadFit, headFitText(*base.Head, model.PlainStyle(), head); got != want {
		t.Errorf("текст шага: кегль %v, ждали %v", got, want)
	}
	if got := vigPlanOf("", "")(p, s, v).HeadFit; got != def {
		t.Errorf("без шага: кегль %v, ждали %v", got, def)
	}
}

// Шапка в ряду создаётся заново при смене шаблона: кегль — по замеру гарнитурой и текстом шага.
func TestApplyLookVigCellHeadSize(t *testing.T) {
	pr := model.New()
	pr.Settings.PageW, pr.Settings.PageH = 102, 152
	e, err := NewEditor(pr, "")
	if err != nil {
		t.Fatal(err)
	}
	step := looktpl.LookStep{Role: looktpl.RoleVignette, Tpl: looktpl.SpreadTpl{V: looktpl.TplV, Pages: 1, Ratio: 1, Page: []looktpl.TplBg{{}}},
		Vig: &looktpl.LookVig{Tpl: "t-left-cell", Teachers: 1, Pupils: 40, Ratio: "3:4", HText: "Наши воспитатели",
			Style: &model.VigStyle{Head: &model.TextStyle{Font: "ptserif"}}}}
	reg := looktpl.AlbumTpl{V: looktpl.LookV, Steps: []looktpl.LookStep{{Tpl: looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2,
		Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}}, Page: []looktpl.TplBg{{}, {}}}}, step}}
	// Сначала лист без шапки (g-top): шапку шаг создаёт заново, с гарнитурой и текстом шага.
	first := reg
	first.Steps = []looktpl.LookStep{reg.Steps[0], step}
	first.Steps[1].Vig = &looktpl.LookVig{Tpl: "g-top", Teachers: 1, Pupils: 10, Ratio: "3:4"}
	for i, t0 := range []looktpl.AlbumTpl{first, reg} {
		ref := model.LookRef{Key: []string{"my:0123456789abcdef", "my:fedcba9876543210"}[i], Name: "x"}
		if _, _, err := e.ApplyLook(ref, t0, nil, model.LookOpts{Spreads: 2}, nil); err != nil {
			t.Fatal(err)
		}
	}
	e.read(func(p *model.Project) {
		s := p.Spreads[1]
		h := s.VigHead(s.Vig.Head)
		if h == nil || h.Text != "Наши воспитатели" || h.Style.Font != "ptserif" {
			t.Fatalf("шапка: %+v", h)
		}
		v := *s.Vig
		v.Head = ""
		pl := vigPlan(p, s, v)
		serif := model.PlainStyle()
		serif.Font = "ptserif"
		if want := headFitText(*pl.Head, serif, "Наши воспитатели"); !pl.HeadCell || h.Style.Size != want {
			t.Errorf("кегль шапки %v, ждали %v по гарнитуре и тексту шага", h.Style.Size, want)
		}
	})
}
