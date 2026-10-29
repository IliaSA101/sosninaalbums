package grid

import (
	"errors"
	"fmt"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// twoSpreads — проект из двух разворотов s1, s2; у s1 на левой странице одно место.
func twoSpreads(t *testing.T) *model.Project {
	t.Helper()
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}, {ID: "s2", Photos: []string{}}}
	a, ok := p.AreaOf(p.Spreads[0], model.ScopeLeft)
	if !ok {
		t.Fatal("у разворота нет левой страницы")
	}
	p.Spreads[0].Grid = []*model.Place{{ID: "0a0000000001", X: a.X + 10, Y: a.Y + 10, W: 50, H: 50}}
	return p
}

// tileStep — шаг макета с одним местом на два листа-страницы.
func tileStep() model.LookStep {
	return model.LookStep{Name: "Глава", Tpl: model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Page: []model.TplBg{{}, {}},
		Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.3}}}}
}

// Область, которой нет у листа, не даёт ни InArea, ни applyGrid, ни AreaFrac: отказ без следа в проекте.
func TestUnknownScopeRefused(t *testing.T) {
	p := twoSpreads(t)
	s := p.Spreads[0]
	before := string(model.Marshal(p))
	if InArea(p, s, 10, 50, "нет-такой") {
		t.Error("InArea по неизвестной области")
	}
	if left, err := applyGrid(p, s, "нет-такой", []model.Box{{X: 1, Y: 1, W: 10, H: 10}}); !errors.Is(err, model.ErrScope) || left != 0 {
		t.Errorf("applyGrid: left %d, err %v", left, err)
	}
	if got := AreaFrac(p, s, "нет-такой"); got != nil {
		t.Errorf("AreaFrac по неизвестной области: %v", got)
	}
	if string(model.Marshal(p)) != before {
		t.Fatal("отказ оставил след")
	}
}

// Лист-страница (Sheet) не имеет левой/правой: шаблон на две страницы и вписывание по нему — отказ, места целы.
func TestLookGeomRefusals(t *testing.T) {
	p := model.NewSheet(200, 200)
	s := p.Spreads[0]
	s.Grid = []*model.Place{{ID: "0a0000000001", X: 10, Y: 10, W: 50, H: 50}}
	before := string(model.Marshal(p))
	bad := model.LookStep{Tpl: model.SpreadTpl{Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.3}}}} // V не тот: зажим отказывает
	if _, ok := lookGeom(p, s, bad); ok {
		t.Error("шаблон не прошёл зажим, а места есть")
	}
	if _, ok := lookGeom(p, s, tileStep()); ok {
		t.Error("шаблон на две страницы лёг на лист без страниц")
	}
	if LookPlacesAre(p, s, tileStep()) {
		t.Error("LookPlacesAre без геометрии")
	}
	LookPlacesRefit(p, s, tileStep())
	if string(model.Marshal(p)) != before {
		t.Fatal("отказ оставил след")
	}
}

// Число мест листа не совпало с шагом: вписывание холостое, ни одно место не сдвинулось.
func TestLookPlacesRefitCountMismatch(t *testing.T) {
	p := twoSpreads(t)
	s := p.Spreads[0]
	s.Grid = append(s.Grid, &model.Place{ID: "0a0000000002", X: 200, Y: 20, W: 40, H: 40})
	before := string(model.Marshal(p))
	LookPlacesRefit(p, s, tileStep())
	if LookPlacesAre(p, s, tileStep()) {
		t.Error("два места не могут быть планом из одного")
	}
	if string(model.Marshal(p)) != before {
		t.Fatal("несовпадение числа мест что-то изменило")
	}
}

// PutLookTile на листе 031 без мест у шага — холостая (нет плана, нет ошибки, лист цел).
func TestPutLookTileVignetteEmpty(t *testing.T) {
	p := model.NewSheet(200, 200)
	s := p.Spreads[0]
	s.Kind, s.Vig = model.KindVignette, &model.Vignette{}
	before := string(model.Marshal(p))
	empty := model.LookStep{Name: "Пусто"} // V не TplV: lookTpl отказывает
	plan, missing, err := PutLookTile(p, s.ID, empty, 1, nil)
	if err != nil || missing != nil || plan != (model.GridPlan{}) {
		t.Fatalf("лист 031: %+v %v %v", plan, missing, err)
	}
	if string(model.Marshal(p)) != before {
		t.Fatal("холостая правка изменила лист")
	}
}

// Копирование: повтор цели учитывается один раз; потолок целей; переполнение мест цели — ошибка.
func TestCopyGridTargetEdges(t *testing.T) {
	p := twoSpreads(t)
	tg := model.GridTarget{Spread: "s2", Scope: model.ScopeLeft}
	plan, err := CopyGrid(p, "s1", model.ScopeLeft, []model.GridTarget{tg, tg, tg}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Left != 0 || len(p.Spreads[1].Grid) != 1 {
		t.Errorf("повтор цели: Left %d, мест %d (ждали 1)", plan.Left, len(p.Spreads[1].Grid))
	}

	many := make([]model.GridTarget, 3*model.MaxSpreads+1)
	for i := range many {
		many[i] = tg
	}
	before := string(model.Marshal(p))
	if _, err := CopyGrid(p, "s1", model.ScopeLeft, many, false, false); !errors.Is(err, errTargets) {
		t.Fatalf("слишком много целей: %v", err)
	}
	if string(model.Marshal(p)) != before {
		t.Fatal("отказ по числу целей оставил след")
	}

	// Цель уже занята правой страницей под завязку: новое место слева не влезает.
	q := twoSpreads(t)
	r, _ := q.AreaOf(q.Spreads[1], model.ScopeRight)
	for i := range model.MaxPlaces {
		q.Spreads[1].Grid = append(q.Spreads[1].Grid, &model.Place{ID: fmt.Sprintf("0b%010d", i), X: r.X + 5, Y: r.Y + 5, W: 10, H: 10})
	}
	if _, err := CopyGrid(q, "s1", model.ScopeLeft, []model.GridTarget{tg}, false, false); err == nil || err.Error() != model.ErrTooManyPlaces().Error() {
		t.Fatalf("переполнение мест цели: %v", err)
	}
	if len(q.Spreads[1].Grid) != model.MaxPlaces { // откат целиком — дело Editor.try; тело своих мест не добавило
		t.Fatalf("мест цели %d", len(q.Spreads[1].Grid))
	}
}

// DuplicateGrid не выходит за MaxSpreads: ошибка, число разворотов прежнее.
func TestDuplicateGridSpreadLimit(t *testing.T) {
	p := twoSpreads(t)
	for len(p.Spreads) < model.MaxSpreads {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("x%d", len(p.Spreads)), Photos: []string{}})
	}
	n := len(p.Spreads)
	if err := DuplicateGrid(p, "s1", 1, false); err == nil {
		t.Fatal("двести первый разворот создан")
	}
	if len(p.Spreads) != n {
		t.Fatalf("разворотов %d, было %d", len(p.Spreads), n)
	}
}

// union пустого списка — нулевой прямоугольник; нет мест или нулевая страница — ни ряда «По фото», ни встроенных сеток.
func TestEmptyInputsGrid(t *testing.T) {
	if u := union(nil); u != (layout.Rect{}) {
		t.Errorf("union(nil) = %+v", u)
	}
	prm := layout.Params{PageW: 0, PageH: 100, Gap: 3, Margin: 5}
	if got := byPhotoRow(nil, prm); got != nil {
		t.Errorf("byPhotoRow без мест: %v", got)
	}
	if got := builtinGrids(prm, []float64{1, 1.5, 0.7}, 3); got != nil {
		t.Errorf("builtinGrids без страницы: %v", got)
	}
}

// Рамочки: неизвестный лист — холостая, проект цел.
func TestSetBorderUnknownSheet(t *testing.T) {
	p := twoSpreads(t)
	before := string(model.Marshal(p))
	SetBorder(p, "нет", []string{"0a0000000001"}, model.BorderPatch{Width: 2})
	if string(model.Marshal(p)) != before {
		t.Fatal("неизвестный лист что-то изменил")
	}
}
