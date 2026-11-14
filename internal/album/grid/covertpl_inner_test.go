package grid

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// tplParts на листе обложки: шаблон страницы встаёт на безопасную зону сторонки, шаблон разворота
// на сторонку — отказ (ErrTplScope), а не зона.
func TestTplPartsCover(t *testing.T) {
	p := model.New()
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID}}
	s := p.Cover.Sheet
	one := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 1, Ratio: 1, Page: []looktpl.TplBg{{}}}
	two := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Page: []looktpl.TplBg{{}, {}}}
	parts, err := tplParts(p, s, model.ScopeLeft, one)
	zone, ok := coverSideOf(p, model.ScopeLeft)
	if err != nil || !ok || len(parts) != 1 || parts[0].a != zone.box {
		t.Fatalf("страница на сторонку: %+v %v, зона %+v", parts, err, zone)
	}
	if _, err = tplParts(p, s, model.ScopeLeft, two); !errors.Is(err, ErrTplScope) {
		t.Fatalf("разворот на сторонку: %v", err)
	}
}
