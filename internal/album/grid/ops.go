package grid

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Правки режима «Сетка» (спека 007, design §3). Всё через try: холостая правка,
// undo и откат при ошибке получаются даром; syncGrid в try — страховка.

var ErrSplitSmall = errors.New("место слишком мало, чтобы делить")

// SetGrid: on — «Закрепить сетку» (места = ячейки раскладки, картинка та же),
// иначе «Вернуть авто» (сохранённый ключ, если строится).
func SetGrid(p *model.Project, spreadID string, on bool) {
	_, s := p.Find(spreadID)
	switch {
	case s == nil:
	case on:
		p.Freeze(s)
	default:
		p.Unfreeze(s)
	}
}

// AddPlace — новое пустое место на странице page (0 — левая, 1 — правая).
// У «Авто» сначала закрепление в той же правке: так сетку начинают с нуля, в
// том числе на развороте только с текстом. Больше MaxPlaces — ошибка.
func AddPlace(p *model.Project, spreadID string, page int) error {
	_, s := p.Find(spreadID)
	if s == nil {
		return nil
	}
	p.Freeze(s)
	_, err := p.AddPlace(s, page, 1, p.PlaceIDs())
	return err
}

// SetPlaces переносит и меняет размер нескольких мест одной правкой (зажим
// внутрь разворота; у повёрнутого — вылет не растёт, clampMoveIn). boxes[].ID — ссылка (placeRef): ID места или ID фото. У
// «Авто» первая правка закрепляет разворот той же правкой (withGrid). Место с
// NaN/Inf не меняется. merge — сессия выделения: серия стрелок — один шаг
// отмены; у «Авто» не действует (закрепление не должно влипнуть в чужую серию).
func SetPlaces(p *model.Project, spreadID string, boxes []model.PlaceBox) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	_ = p.WithGrid(s, func() error {
		for _, b := range boxes {
			pl := s.PlaceRef(b.ID)
			if pl == nil || !model.FinitePlace(&model.Place{X: b.X, Y: b.Y, W: b.W, H: b.H}) {
				continue
			}
			prev := *pl
			pl.X, pl.Y, pl.W, pl.H = b.X, b.Y, b.W, b.H // угол стоит: PlaceBox его не несёт
			model.ClampMoveIn(pl, prev, 2*p.Settings.PageW, p.Settings.PageH)
		}
		return nil
	})
}

// GridIsAuto — разворот есть и он в режиме «Авто» (бывший isAuto).
func GridIsAuto(p *model.Project, spreadID string) bool {
	_, s := p.Find(spreadID)
	return s != nil && len(s.Grid) == 0
}

// DeletePlaces удаляет места по ссылкам (placeRef); их фото уходят в ленту. Не
// осталось мест — разворот становится «Авто». У «Авто» холостая: фото убирает
// RemovePhotos.
func DeletePlaces(p *model.Project, spreadID string, ids []string) { p.DeletePlaces(spreadID, ids) }

// SetPlaceFit: fit "" — с обрезкой (cover и кадр), FitContain — целиком; ids —
// ссылки (placeRef). Иное — холостая правка. У «Авто» FitContain закрепляет
// разворот, "" ничего не меняет.
func SetPlaceFit(p *model.Project, spreadID string, ids []string, fit string) {
	_, s := p.Find(spreadID)
	if s == nil || fit != "" && fit != model.FitContain || fit == "" && len(s.Grid) == 0 {
		return
	}
	_ = p.WithGrid(s, func() error {
		for _, ref := range ids {
			if pl := s.PlaceRef(ref); pl != nil {
				pl.Fit = fit
			}
		}
		return nil
	})
}

// ArrangePlaces — кнопки правки сетки над выделением ids (ссылки placeRef,
// первое — образец): same/sameW/sameH — размер как у образца, центр на месте;
// fitPhoto — пропорции фото места; splitH/splitV — делит на два с промежутком
// Settings.Gap (фото остаётся в первом); front/back — наверх / вниз по слоям.
// У «Авто» разворот закрепляется той же правкой, кроме front/back (ничего не
// перекрывается — холостая). Неизвестный op — холостая правка; половина меньше
// geom.MinPlace или мест больше MaxPlaces — ошибка.
func ArrangePlaces(p *model.Project, spreadID, op string, ids []string) error {
	_, s := p.Find(spreadID)
	if s == nil || len(s.Grid) == 0 && (op == "front" || op == "back") {
		return nil
	}
	return p.WithGrid(s, func() error { return arrange(p, s, op, ids) })
}

// arrange — тело ArrangePlaces над сеткой s.
func arrange(p *model.Project, s *model.Spread, op string, ids []string) error {
	var sel []*model.Place
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil && !slices.Contains(sel, pl) {
			sel = append(sel, pl)
		}
	}
	if len(sel) == 0 {
		return nil
	}
	switch op {
	case "same", "sameW", "sameH":
		for _, pl := range sel[1:] {
			w, h := pl.W, pl.H
			if op != "sameH" {
				w = sel[0].W
			}
			if op != "sameW" {
				h = sel[0].H
			}
			resize(p, pl, w, h)
		}
	case "fitPhoto":
		for _, pl := range sel {
			if pl.Photo != "" {
				a := p.Aspects([]string{pl.Photo})[0]
				w, h := pl.W, pl.W/a
				if h > p.Settings.PageH {
					h, w = p.Settings.PageH, p.Settings.PageH*a
				}
				resize(p, pl, w, h)
			}
		}
	case "splitH", "splitV":
		return splitPlaces(p, s, sel, op == "splitH")
	case "front", "back":
		// Общий стек со слоями текстов (ArrangeLayers): «наверх» — и над текстами.
		ids := make([]string, len(sel))
		for i, pl := range sel {
			ids[i] = pl.ID
		}
		s.RestackLayers(op, ids)
	}
	return nil
}

// resize — новый размер места с прежним центром, затем зажим.
func resize(p *model.Project, pl *model.Place, w, h float64) {
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	pl.W, pl.H = w, h
	pl.X, pl.Y = cx-w/2, cy-h/2
	p.ClampPlace(pl)
}

// splitPlaces делит места на два равных с промежутком Gap: horiz — рядом
// (⇆), иначе одно над другим. Новое место — сразу за делимым, пустое, с тем же
// Fit. Половина вниз до 0.01 мм: две половины с промежутком не длиннее места.
func splitPlaces(p *model.Project, s *model.Spread, sel []*model.Place, horiz bool) error {
	if len(s.Grid)+len(sel) > model.MaxPlaces {
		return model.ErrTooManyPlaces()
	}
	gap := p.Settings.Gap
	taken := p.PlaceIDs()
	for _, pl := range sel {
		side := pl.H
		if horiz {
			side = pl.W
		}
		half := geom.Floor2((side - gap) / 2)
		if half < geom.MinPlace {
			return ErrSplitSmall
		}
		nw := model.ClonePlace(pl, taken)
		switch {
		case pl.Rot != 0:
			splitRotated(pl, nw, half, gap, horiz)
			p.ClampPlace(pl)
		case horiz:
			pl.W, nw.W, nw.X = half, half, pl.X+half+gap
		default:
			pl.H, nw.H, nw.Y = half, half, pl.Y+half+gap
		}
		p.ClampPlace(nw)
		i, _ := s.FindPlace(pl.ID)
		s.Grid = slices.Insert(s.Grid, i+1, nw)
	}
	return nil
}

// splitRotated — деление повёрнутого места в его осях: центры половин на оси места
// (u1 = −side/2 + half/2, u2 = u1 + half + gap), повёрнутые вокруг центра; угол тот же.
// Зажим — у вызывающего: у края половины зажимаются по отдельности.
func splitRotated(pl, nw *model.Place, half, gap float64, horiz bool) {
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	side := pl.H
	if horiz {
		side = pl.W
	}
	u1 := -side/2 + half/2
	u2 := u1 + half + gap
	at := func(q *model.Place, u float64) {
		if horiz {
			q.W = half
		} else {
			q.H = half
		}
		du, dv := u, 0.0
		if !horiz {
			du, dv = 0, u
		}
		dx, dy := geom.Rotate(pl.Rot, du, dv)
		q.X, q.Y = cx+dx-q.W/2, cy+dy-q.H/2
	}
	at(pl, u1)
	at(nw, u2)
}

// MirrorGrid отражает места области scope (left | right | spread) — design §3
// «Отражение». Симметричная сетка, нет мест, неизвестная область — холостая
// правка. «Авто» проверяется на симметрию по своей раскладке, иначе
// закрепляется и отражается одной правкой (Ctrl+Z вернёт «Авто»).
func MirrorGrid(p *model.Project, spreadID, scope string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	auto := len(s.Grid) == 0
	p.Freeze(s)
	if !mirrorGrid(p, s, scope) && auto {
		s.Grid = nil // freeze меняет только Grid: откат — холостая правка, без закрепления
	}
}

// PlacePhotos — бросок фото на место (design §3): первое фото на занятое место —
// обмен, если оно из альбома, иначе замена (прежнее уходит в ленту, считается в
// left); на пустое — перенос откуда угодно. Остальные — в следующие пустые
// места разворота после целевого, потом с начала; не хватило — left, фото
// остаются где были. Одна правка. Неизвестное место — холостая правка.
func PlacePhotos(p *model.Project, spreadID, placeID string, photos []model.PhotoInfo) (left int, err error) {
	photos = model.DedupePhotos(photos)
	s := p.GridSpread(spreadID)
	if s == nil || len(photos) == 0 {
		return 0, nil
	}
	at, pl := s.FindPlace(placeID)
	if pl == nil {
		return 0, nil
	}
	inAlbum := p.Placed(photos[0].ID)
	ids := model.Remember(p, photos)
	switch f := ids[0]; {
	case pl.Photo == f:
	case pl.Photo != "" && inAlbum:
		p.Swap(f, pl.Photo)
	default:
		if pl.Photo != "" {
			left++ // замена: прежний хозяин места возвращается в ленту
		}
		p.Put(s, pl, f)
	}
	for _, id := range ids[1:] {
		if slices.Contains(model.GridPhotos(s), id) {
			continue // уже в этой сетке: порядок задают места
		}
		dst := nextEmpty(s, at)
		if dst == nil {
			left++
			continue
		}
		p.Put(s, dst, id)
	}
	return left, nil
}

// nextEmpty — первое пустое место после позиции at, потом с начала.
func nextEmpty(s *model.Spread, at int) *model.Place {
	n := len(s.Grid)
	for k := 1; k <= n; k++ {
		if pl := s.Grid[(at+k)%n]; pl.Photo == "" {
			return pl
		}
	}
	return nil
}

// FillPlaces — «Заполнить пустые места»: фото не из альбома по порядку садятся в
// пустые места разворотов-сеток, начиная с fromSpread (неизвестный — с начала)
// и дальше по альбому. Фото из альбома и с фонов пропускаются. left — сколько не встало.
func FillPlaces(p *model.Project, photos []model.PhotoInfo, fromSpread string) (left int) {
	photos = model.DedupePhotos(photos)
	onBg := p.OnBackdrops()
	fresh := slices.DeleteFunc(slices.Clone(photos), func(ph model.PhotoInfo) bool { return p.Placed(ph.ID) || onBg[ph.ID] })
	ids := model.Remember(p, fresh)
	start, _ := p.Find(fromSpread)
	k := 0
	for _, s := range p.Spreads[max(start, 0):] {
		for _, pl := range s.Grid {
			if k < len(ids) && pl.Photo == "" {
				p.Put(s, pl, ids[k])
				k++
			}
		}
	}
	return len(ids) - k
}
