package grid

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ApplyLookBg: негодный шаг (страниц не 1|2) и неизвестная область — холостая правка, не паника;
// повтор того же фона — Changed = false.
func TestApplyLookBgInvalidStep(t *testing.T) {
	p := model.New()
	p.Spreads = append(p.Spreads, &model.Spread{ID: "s1", Photos: []string{}})
	s := p.Spreads[0]
	bad := model.SpreadTpl{V: model.TplV, Pages: 3, Ratio: 1, Page: []model.TplBg{{Color: "#ffffff"}}}
	if r := ApplyLookBg(p, s, bad, nil); r.Changed || s.BgL != "" {
		t.Fatalf("негодный шаг: %+v, фон %q", r, s.BgL)
	}
	good := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#112233"}}}
	if r := ApplyLookBg(p, s, good, nil); !r.Changed || r.Replaced || s.BgL != "#112233" || s.BgR != "#112233" {
		t.Fatalf("первый раз: %+v, фон %q/%q", r, s.BgL, s.BgR)
	}
	if r := ApplyLookBg(p, s, good, nil); r.Changed || !LookBgSame(p, s, good, nil) {
		t.Fatalf("повтор: %+v", r)
	}
	if r := ApplyLookCoverBg(p, &model.CoverTpl{Bg: &model.TplBg{Color: "#445566"}}, nil); r.Changed {
		t.Fatal("обложки нет — холостая")
	}
}

// PutLookTile: неизвестный лист — холостая, шаг без мест и оформления на обычном листе — ErrTplEmpty,
// номер шага пишется как есть.
func TestPutLookTileEdges(t *testing.T) {
	p := model.New()
	p.Spreads = append(p.Spreads, &model.Spread{ID: "s1", Photos: []string{}})
	st := model.LookStep{Tpl: model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#112233"}}}}
	if _, _, err := PutLookTile(p, "нет", st, 1, nil); err != nil {
		t.Fatalf("неизвестный лист: %v", err)
	}
	if _, _, err := PutLookTile(p, "s1", model.LookStep{}, 1, nil); err == nil {
		t.Fatal("пустой шаг на обычном листе должен отказать")
	}
	if _, _, err := PutLookTile(p, "s1", st, 2, nil); err != nil || p.Spreads[0].LookStep != 2 {
		t.Fatalf("шаг: %v, номер %d", err, p.Spreads[0].LookStep)
	}
}

// LookPlacesAre: поворот, форма и «Вписать» делают место тронутым; Refit возвращает геометрию и
// оставляет ID.
func TestLookPlacesAreTouched(t *testing.T) {
	p := model.New()
	p.Spreads = append(p.Spreads, &model.Spread{ID: "s1", Photos: []string{}})
	s := p.Spreads[0]
	st := model.LookStep{Tpl: model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2,
		Places: []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.4, H: 0.8}},
		Page:   []model.TplBg{{Color: "#ffffff"}, {Color: "#ffffff"}}}}
	if _, _, err := PutLookStep(p, "s1", st, 1, nil); err != nil || len(s.Grid) != 2 || !LookPlacesAre(p, s, st) {
		t.Fatalf("после шага места должны совпасть: %v, мест %d", err, len(s.Grid))
	}
	for name, touch := range map[string]func(pl *model.Place){
		"поворот": func(pl *model.Place) { pl.Rot = 5 },
		"форма":   func(pl *model.Place) { pl.Shape = "circle" },
		"вписать": func(pl *model.Place) { pl.Fit = "contain" },
		"сдвиг":   func(pl *model.Place) { pl.X += 3 },
	} {
		_, s2 := p.Find("s1")
		id, x := s2.Grid[1].ID, s2.Grid[1].X
		touch(s2.Grid[1])
		if LookPlacesAre(p, s2, st) {
			t.Errorf("%s: место должно считаться тронутым", name)
		}
		s2.Grid[1].Rot, s2.Grid[1].Shape, s2.Grid[1].Fit, s2.Grid[1].X = 0, "", "", x
		if s2.Grid[1].ID != id || !LookPlacesAre(p, s2, st) {
			t.Errorf("%s: после отката места те же", name)
		}
	}
	s.Grid[0].X += 9
	LookPlacesRefit(p, s, st)
	if !LookPlacesAre(p, s, st) {
		t.Fatal("Refit не вернул места по шагу")
	}
}
