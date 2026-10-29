package grid

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Сохранить альбом как макет…» (131 design §8): снимок разворотов и обложки в AlbumTpl. Проект
// не меняется; фото и фото фона из съёмки в макет не идут (решение 067-6) — только места, цвет,
// узоры, фоны и стикеры из библиотеки, надписи заготовками.

var (
	// ErrLookNoSpreads — не выбрано ни одного разворота.
	ErrLookNoSpreads = errors.New("Выберите хотя бы один разворот для макета") //nolint:staticcheck // текст уходит пользователю как есть
	// ErrLookNothing — в выбранных разворотах нечего сохранять: ни мест, ни оформления.
	ErrLookNothing = errors.New("В выбранных разворотах нечего сохранять: включите «Места для фото» или «Надписи — заготовками» либо добавьте им оформление") //nolint:staticcheck // текст уходит пользователю как есть
	// ErrLookIDs — id не из альбома или повторён.
	ErrLookIDs = errors.New("Выбранного разворота нет в альбоме или он назван дважды") //nolint:staticcheck // текст уходит пользователю как есть
)

// AlbumTplOf — макет из разворотов o.Ids проекта p: шаг на разворот, повторяемые (позиции
// RepFrom…RepTo среди Ids) слиты по TplKey, затем обложка (o.Cover). Лист 031 даёт шаг-виньетку
// (параметры листа, оформление — без мест). Разворот, с которого нечего снять, в серию не идёт.
// Имена глав в макет не входят (данные клиента, как текст надписей). Ids пуст, больше MaxSpreads,
// не из альбома или с повтором — ошибка до сборки; после слияния больше MaxLookSteps шагов —
// model.ErrLookMany. Свои стикеры, для которых skip(ключ) верно (код ссылки клиента), в макет не идут.
// Только чтение: «Авто» закрепляется на копии, одной на весь вызов.
func AlbumTplOf(p *model.Project, o model.SaveLookOpts, skip func(lib string) bool) (model.AlbumTpl, error) {
	if len(o.Ids) == 0 {
		return model.AlbumTpl{}, ErrLookNoSpreads
	}
	if len(o.Ids) > model.MaxSpreads {
		return model.AlbumTpl{}, ErrLookIDs
	}
	seen := make(map[string]bool, len(o.Ids))
	for _, id := range o.Ids {
		if _, s := p.Find(id); s == nil || seen[id] {
			return model.AlbumTpl{}, ErrLookIDs
		}
		seen[id] = true
	}
	c := p.Clone() // «Авто» закрепляется здесь один раз, а не копией проекта на каждый разворот
	var steps []model.LookStep
	var rep []bool
	for i, id := range o.Ids {
		_, s := c.Find(id)
		if st, ok := lookStepOf(c, s, o, skip); ok {
			pos := i + 1
			steps, rep = append(steps, st), append(rep, pos >= o.RepFrom && pos <= o.RepTo)
		}
	}
	merged, err := model.MergeLookSteps(steps, rep)
	if err != nil {
		return model.AlbumTpl{}, err
	}
	t := model.AlbumTpl{V: model.LookV, Steps: merged}
	if _, ok := model.ClampLook(t); !ok {
		return model.AlbumTpl{}, ErrLookNothing
	}
	if o.Cover {
		t.Cover = coverTplOf(c, o, skip)
	}
	return t, nil
}

// lookStepOf — шаг макета из разворота s проекта-копии c; false — снять нечего (SpreadTplOfAs
// отказывает только пустому: id проверены, обложки среди них нет). Лист-виньетка: параметры листа и
// оформление страницы, мест нет (их строит лист).
func lookStepOf(c *model.Project, s *model.Spread, o model.SaveLookOpts, skip func(string) bool) (model.LookStep, bool) {
	vig := s.IsVignette()
	places := o.Places && !vig
	if places {
		c.Freeze(s)
	}
	tpl, err := spreadTplOf(c, s.ID, model.ScopeSpread, tplSnap{places: places, texts: o.Drafts, asDrafts: o.Drafts, skip: skip, frozen: true})
	switch {
	case err == nil:
	case vig:
		tpl = model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{}}} // у виньетки оформление может быть пустым
	default:
		return model.LookStep{}, false
	}
	st := model.LookStep{Tpl: tpl}
	if vig {
		st.Role, st.Vig = model.RoleVignette, &model.LookVig{Tpl: s.Vig.Tpl, Teachers: s.Vig.Teachers, Pupils: s.Vig.Pupils, Ratio: s.Vig.Ratio}
	}
	return st, true
}

// coverTplOf — оформление обложки: фон всего листа (узор и фото из библиотеки), задняя и лицевая
// сторонки страницами на безопасную зону (127; доли зоны, как их ставит ApplyLookCover). Фон — как
// его ставит model.CoverPageBg: есть узор или фото листа — они в Bg, а страница стороны несёт только
// цвет; иначе узор и фото сторон (только из библиотеки) — в страницах. Корешок не входит. Обложки или оформления нет — nil.
func coverTplOf(c *model.Project, o model.SaveLookOpts, skip func(string) bool) *model.CoverTpl {
	if c.Cover == nil || c.Cover.Sheet == nil {
		return nil
	}
	s := c.Cover.Sheet
	out := &model.CoverTpl{}
	own := [2]model.TplBg{{Color: s.BgL}, {Color: s.BgR}} // сторона при узоре или фото листа даёт только цвет
	if bg := bgOf(c, "", s.Pat, s.BgPhoto); bg.Pat != nil || bg.Lib != "" {
		out.Bg = &bg
	} else { // узора и фото листа нет — узор и фото сторон ставятся на их страницы (CoverPageBg)
		own = [2]model.TplBg{bgOf(c, s.BgL, s.PatL, s.BgPhotoL), bgOf(c, s.BgR, s.PatR, s.BgPhotoR)}
	}
	out.Back = coverSideTplOf(c, model.ScopeLeft, own[0], o, skip)
	out.Front = coverSideTplOf(c, model.ScopeRight, own[1], o, skip)
	if out.Bg == nil && out.Back == nil && out.Front == nil {
		return nil
	}
	return out
}

// coverSideTplOf — сторонка scope обложки страницей-шаблоном на её зону; nil — сторонки нет или с
// неё нечего снять.
func coverSideTplOf(c *model.Project, scope string, own model.TplBg, o model.SaveLookOpts, skip func(string) bool) *model.SpreadTpl {
	side, ok := coverSideOf(c, scope)
	if !ok {
		return nil
	}
	z := side.box
	t := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: z.W / z.H, W: z.W, H: z.H, Page: []model.TplBg{own}}
	if o.Places {
		t.Places, t.Borders = zonePlaces(c, side)
	}
	tplLayers(c, side.s, scope, z, &t, tplSnap{texts: o.Drafts, asDrafts: o.Drafts, skip: skip})
	cl, ok := model.ClampSpreadTpl(t)
	if !ok {
		return nil
	}
	return &cl
}

// zonePlaces — места сторонки в долях её зоны (целиком в зоне, как держит holdInZone) и их рамочки.
func zonePlaces(c *model.Project, side *coverSide) ([]model.Box, []model.TplBorder) {
	z := side.box
	boxes := make([]model.Box, 0, len(side.places))
	for _, pl := range side.places {
		b := holdInZone(model.Box{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Rot: pl.Rot, Shape: pl.Shape, Round: pl.Round}, z)
		boxes = append(boxes, model.Box{X: (b.X - z.X) / z.W, Y: (b.Y - z.Y) / z.H, W: b.W / z.W, H: b.H / z.H,
			Rot: b.Rot, Shape: b.Shape, Round: b.Round, Border: pl.Border.Clone()})
	}
	boxes = model.ClampBoxes(boxes)
	borders := bordersOf(c, boxes)
	for i := range boxes {
		boxes[i].Border = nil // в памяти шаблона рамочек у коробок нет: они в Borders
	}
	return boxes, borders
}
