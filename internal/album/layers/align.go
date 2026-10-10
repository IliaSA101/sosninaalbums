package layers

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Выравнивание и распределение (спека 010, design §5). Рамка объекта — aabb из
// LayerItems: повёрнутое место и текст выравниваются по описанному прямоугольнику.

// AlignItems — тело Editor.AlignItems над проектом (op и to уже проверены).
func AlignItems(p *model.Project, spreadID string, ids []string, op, to string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	if len(s.Grid) == 0 && hasPhoto(s, ids) {
		_ = p.WithGrid(s, func() error { alignItems(p, s, ids, op, to); return nil })
		return
	}
	alignItems(p, s, ids, op, to)
}

// AlignOverlap — тело Editor.AlignOverlap над проектом (op и to уже проверены); p не меняет.
func AlignOverlap(p *model.Project, spreadID string, ids []string, op, to string) bool {
	_, s := p.Find(spreadID)
	if s == nil {
		return false
	}
	if len(s.Grid) == 0 && hasPhoto(s, ids) {
		p = p.Clone()
		_, s = p.Find(spreadID)
		p.Freeze(s)
	}
	_, _, over := alignPlan(p, s, ids, op, to)
	return over
}

// alignItems — тело AlignItems над разворотом s (у «Авто» с фото — уже закреплённым).
func alignItems(p *model.Project, s *model.Spread, ids []string, op, to string) {
	sel, d, _ := alignPlan(p, s, ids, op, to)
	for i, it := range sel {
		if d[i] != [2]float64{} {
			shiftItem(s, p, it, d[i][0], d[i][1])
		}
	}
}

// alignPlan — выбранные объекты снизу вверх по слоям и их сдвиги; over — распределение
// не поместилось без наложения. Мало объектов — пусто.
func alignPlan(p *model.Project, s *model.Spread, ids []string, op, to string) (sel []model.LayerItem, d [][2]float64, over bool) {
	want := make(map[string]bool, len(ids))
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil {
			want[pl.ID] = true
		} else {
			want[ref] = true
		}
	}
	// Порядок слоёв снизу вверх: он же второй ключ распределения.
	for _, it := range s.LayerItems() {
		if want[it.ID] {
			sel = append(sel, it)
		}
	}
	if len(sel) < alignMin(op, to) {
		return nil, nil, false
	}
	rects := make([]layout.Rect, len(sel))
	for i, it := range sel {
		rects[i] = it.Box.AABB()
	}
	d, over = geom.AlignShifts(model.Page(p.Settings), rects, op, to)
	return sel, d, over
}

// alignMin — сколько объектов нужно: у опоры sel выравнивание 2+, распределение 3+;
// у page/spread опора — поле, выравнивать можно и один, распределять — от двух.
func alignMin(op, to string) int {
	dist := op == "distH" || op == "distV"
	switch {
	case to == geom.AlignSel && dist:
		return 3
	case to == geom.AlignSel || dist:
		return 2
	}
	return 1
}

// shiftItem сдвигает место или текст на (dx, dy) мм. Место зажимается clampPlace
// (aabb внутри разворота ⇒ hold внутри, зажим не сдвигает); тексты потом
// выравнивает fitTexts.
func shiftItem(s *model.Spread, p *model.Project, it model.LayerItem, dx, dy float64) {
	switch it.Kind {
	case model.LayerPlace:
		if _, pl := s.FindPlace(it.ID); pl != nil {
			pl.X += dx
			pl.Y += dy
			p.ClampPlace(pl)
		}
	case model.LayerText:
		for _, t := range s.Texts {
			if t.ID == it.ID {
				t.X, t.Y = geom.Round2(t.X+dx), geom.Round2(t.Y+dy)
			}
		}
	}
}
