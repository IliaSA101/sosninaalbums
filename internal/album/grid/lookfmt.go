package grid

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Смена формата альбома с макетом (131 design §2, 0.27-131-23): нетронутые места макетных
// разворотов вписываются заново по LookBoxes нового формата, а не тянутся Rescale по осям.

// lookGeom — места шага st на листе s в мм листа при текущем формате, как их ставит
// PutLookStep (до зажима). false — у листа нечего сверять: лист 031 (места строит виньетка) или
// шаг без мест.
func lookGeom(p *model.Project, s *model.Spread, st model.LookStep) ([]model.Box, bool) {
	if s.IsVignette() || len(st.Tpl.Places) == 0 {
		return nil, false
	}
	t, ok := lookTpl(p, s, st.Tpl)
	if !ok || len(t.Places) == 0 {
		return nil, false
	}
	parts, err := tplParts(p, s, model.ScopeSpread, t)
	if err != nil {
		return nil, false
	}
	return tplBoxes(p, s, parts, t), true
}

// placeOf — место с геометрией и видом коробки b, зажатое листом s.
func placeOf(p *model.Project, s *model.Spread, b model.Box) model.Place {
	n := model.Place{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot, Shape: b.Shape, Round: b.Round}
	p.ClampPlace(s, &n)
	return n
}

// LookPlacesAre — места листа s такие, какие поставил бы шаг st на текущем формате проекта:
// геометрия, поворот, форма, нет «Вписать». Рамочки не сверяются (им нужны файлы библиотеки), фото
// в местах не мешают. Нет мест у шага или лист 031 — false.
func LookPlacesAre(p *model.Project, s *model.Spread, st model.LookStep) bool {
	boxes, ok := lookGeom(p, s, st)
	if !ok || len(boxes) != len(s.Grid) {
		return false
	}
	for i, b := range boxes {
		n, c := placeOf(p, s, b), s.Grid[i]
		if cents(c.X) != cents(n.X) || cents(c.Y) != cents(n.Y) || cents(c.W) != cents(n.W) || cents(c.H) != cents(n.H) ||
			c.Rot != n.Rot || c.Shape != n.Shape || c.Round != n.Round || c.Fit != "" {
			return false
		}
	}
	return true
}

// LookPlacesRefit вписывает места листа s по шагу st на текущем формате: меняется только
// геометрия, ID, фото и рамочки мест остаются. Число мест не то — ничего.
func LookPlacesRefit(p *model.Project, s *model.Spread, st model.LookStep) {
	boxes, ok := lookGeom(p, s, st)
	if !ok || len(boxes) != len(s.Grid) {
		return
	}
	for i, b := range boxes {
		n, c := placeOf(p, s, b), s.Grid[i]
		c.X, c.Y, c.W, c.H = n.X, n.Y, n.W, n.H
	}
}
