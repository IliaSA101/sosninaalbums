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

// AddPlace — новое пустое место на странице page (0 — левая, 1 — правая), квадратное;
// shape — его форма (044: ShapeOval — круг, чужая — прямоугольник).
// У «Авто» сначала закрепление в той же правке: так сетку начинают с нуля, в
// том числе на развороте только с текстом. Больше MaxPlaces — ошибка.
func AddPlace(p *model.Project, spreadID string, page int, shape string) error {
	s := p.Sheet(spreadID)
	if s == nil {
		return nil
	}
	p.Freeze(s)
	pl, err := p.AddPlace(s, page, 1, p.PlaceIDs())
	if err == nil && shape == model.ShapeRounded {
		pl.Shape, pl.Round = "", model.RoundedPct
	} else if err == nil {
		pl.Shape, pl.Round = model.ClampShape(shape, 0)
	}
	return err
}

// SetPlaces переносит и меняет размер нескольких мест одной правкой (зажим
// внутрь разворота; у повёрнутого — вылет не растёт, clampMoveIn). boxes[].ID — ссылка (placeRef): ID места или ID фото. У
// «Авто» первая правка закрепляет разворот той же правкой (withGrid). Место с
// NaN/Inf не меняется. merge — сессия выделения: серия стрелок — один шаг
// отмены; у «Авто» не действует (закрепление не должно влипнуть в чужую серию).
func SetPlaces(p *model.Project, spreadID string, boxes []model.PlaceBox) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	g := p.Geo(s)
	_ = p.WithGrid(s, func() error {
		for _, b := range boxes {
			pl := s.PlaceRef(b.ID)
			if pl == nil || !model.FinitePlace(&model.Place{X: b.X, Y: b.Y, W: b.W, H: b.H}) {
				continue
			}
			prev := *pl
			pl.X, pl.Y, pl.W, pl.H = b.X, b.Y, b.W, b.H // угол стоит: PlaceBox его не несёт
			model.ClampMoveIn(pl, prev, g.W, g.H)
		}
		return nil
	})
}

// GridIsAuto — разворот есть и он в режиме «Авто» (бывший isAuto).
func GridIsAuto(p *model.Project, spreadID string) bool {
	s := p.Sheet(spreadID)
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
	s := p.Sheet(spreadID)
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

// SetPlaceShape — форма мест ids (ссылки placeRef, 044) по образцу SetPlaceFit: рамка, фото
// и кадр стоят. "" — прямые углы (076: скругление снимается, иначе ▭ у скруглённого холостой);
// ShapeOval — круг без скругления; ShapeRounded — своё скругление, если есть, иначе RoundedPct
// (у круга скругления нет — 10 %). Форма не из белого списка — холостая; у «Авто» ShapeOval и
// ShapeRounded закрепляют разворот, "" ничего не меняет.
func SetPlaceShape(p *model.Project, spreadID string, ids []string, shape string) {
	s := p.Sheet(spreadID)
	if s == nil || shape != "" && shape != model.ShapeOval && shape != model.ShapeRounded || shape == "" && len(s.Grid) == 0 {
		return
	}
	_ = p.WithGrid(s, func() error {
		for _, ref := range ids {
			pl := s.PlaceRef(ref)
			switch {
			case pl == nil:
			case shape != model.ShapeRounded:
				pl.Shape, pl.Round = model.ClampShape(shape, 0)
			case pl.Shape != "" || pl.Round <= 0:
				pl.Shape, pl.Round = "", model.RoundedPct
			}
		}
		return nil
	})
}

// SetPlaceRound — скругление углов мест ids, % меньшей стороны (зажим ClampShape: 0…50,
// целое). Только прямоугольные места: у круга скругления нет. Число — конечное (договор
// обёртки). У «Авто» ненулевое закрепляет разворот, 0 — холостая (WithGrid).
func SetPlaceRound(p *model.Project, spreadID string, ids []string, round float64) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	_, round = model.ClampShape("", round)
	_ = p.WithGrid(s, func() error {
		for _, ref := range ids {
			if pl := s.PlaceRef(ref); pl != nil && pl.Shape == "" {
				pl.Round = round
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
	s := p.Sheet(spreadID)
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
			resize(p, s, pl, w, h)
		}
	case "fitPhoto":
		for _, pl := range sel {
			if pl.Photo != "" {
				w, h := fitPhoto(p, s, pl)
				resize(p, s, pl, w, h)
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

// fitPhoto — размер места pl под пропорцию его фото: в пропорции фото окно рамочки (079 §2), без
// рамочки — само место. Ширина стоит; не встало по высоте страницы — по высоте. Отступы картинки
// зависят от размера места (k), поэтому две итерации; у встроенной они постоянны — точно с первой,
// без размеров картинки отступов нет — как без рамочки.
func fitPhoto(p *model.Project, s *model.Spread, pl *model.Place) (w, h float64) {
	a, ph := p.PhotoAspects([]string{pl.Photo})[0], p.Geo(s).H
	w, h = pl.W, pl.H
	for range 2 {
		q := *pl // только для отступов: Border не меняется
		q.W, q.H = w, h
		ins := p.BorderIns(&q)
		dw, dh := ins[0]+ins[2], ins[1]+ins[3]
		if w, h = pl.W, (pl.W-dw)/a+dh; h > ph {
			h, w = ph, (ph-dh)*a+dw
		}
	}
	return w, h
}

// resize — новый размер места листа s с прежним центром, затем зажим.
func resize(p *model.Project, s *model.Spread, pl *model.Place, w, h float64) {
	cx, cy := pl.X+pl.W/2, pl.Y+pl.H/2
	pl.W, pl.H = w, h
	pl.X, pl.Y = cx-w/2, cy-h/2
	p.ClampPlace(s, pl)
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
			p.ClampPlace(s, pl)
		case horiz:
			pl.W, nw.W, nw.X = half, half, pl.X+half+gap
		default:
			pl.H, nw.H, nw.Y = half, half, pl.Y+half+gap
		}
		p.ClampPlace(s, nw)
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
// закрепляется и отражается одной правкой (Ctrl+Z вернёт «Авто»). У виньетки
// с местами отражается и шапка (mirrorHead).
func MirrorGrid(p *model.Project, spreadID, scope string) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	auto := len(s.Grid) == 0
	p.Freeze(s)
	moved := mirrorGrid(p, s, scope)
	if moved = mirrorHead(p, s, scope) || moved; !moved && auto {
		s.Grid = nil // freeze меняет только Grid: откат — холостая правка, без закрепления
	}
}

// PlacePhotos — бросок фото на место (design §3): первое фото на занятое место —
// обмен, если оно из альбома, иначе замена (прежнее уходит в ленту, считается в
// left); на пустое — перенос откуда угодно. Остальные — в следующие пустые
// места разворота после целевого, потом с начала; не хватило — left, фото
// остаются где были. Одна правка. Неизвестное место — холостая правка.
// На листе обложки «следующие» ищутся только на сторонке целевого места (127 G4,
// 0.26-127-12): лицевая не занимает заднюю; целевое на корешке — только оно само.
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
	photos, err = model.ResolveCopies(p, photos) // запрос копии (098) — новая запись
	if err != nil {
		return 0, err
	}
	inAlbum := p.Placed(photos[0].ID)
	ids := model.Remember(p, photos)
	if p.DropOn(s, pl, ids[0], inAlbum) {
		left++ // замена: прежний хозяин места возвращается в ленту
	}
	side := sideOf(p, s, pl)
	for _, id := range ids[1:] {
		if slices.Contains(side.photos(s), id) {
			continue // уже в этой сетке (на обложке — на этой сторонке): порядок задают места
		}
		dst := side.nextEmpty(s, at)
		if dst == nil {
			left++
			continue
		}
		p.Put(s, dst, id)
	}
	return left, nil
}

// placeSide — где ищутся места броска: весь лист (разворот, страница) или сторонка обложки.
type placeSide struct {
	p     *model.Project
	scope string       // left | right — сторонка обложки
	only  *model.Place // место на корешке: сторонки нет, годится только оно
}

// sideOf — сторонка места pl на листе обложки; центр на корешке — только само место pl
// (корешок не относится ни к одной сторонке); у разворота — весь лист.
func sideOf(p *model.Project, s *model.Spread, pl *model.Place) placeSide {
	if !p.IsCover(s) {
		return placeSide{}
	}
	for _, sc := range []string{model.ScopeLeft, model.ScopeRight} {
		if InArea(p, s, pl.X, pl.W, sc) {
			return placeSide{p: p, scope: sc}
		}
	}
	return placeSide{only: pl}
}

// has — место pl входит в сторонку.
func (d placeSide) has(s *model.Spread, pl *model.Place) bool {
	switch {
	case d.only != nil:
		return pl == d.only
	case d.scope == "":
		return true
	}
	return InArea(d.p, s, pl.X, pl.W, d.scope)
}

// photos — фото мест сторонки.
func (d placeSide) photos(s *model.Spread) (out []string) {
	for _, pl := range s.Grid {
		if pl.Photo != "" && d.has(s, pl) {
			out = append(out, pl.Photo)
		}
	}
	return out
}

// nextEmpty — первое пустое место сторонки после позиции at, потом с начала; нет — nil.
// Для всего листа — прежний Spread.NextEmpty.
func (d placeSide) nextEmpty(s *model.Spread, at int) *model.Place {
	if d.scope == "" && d.only == nil {
		return s.NextEmpty(at)
	}
	n := len(s.Grid)
	for k := 1; k <= n; k++ {
		if pl := s.Grid[((at+k)%n+n)%n]; pl.Photo == "" && d.has(s, pl) {
			return pl
		}
	}
	return nil
}

// FillPlaces — «Заполнить пустые места»: фото не из альбома по порядку садятся в
// пустые места (порядок — model.FillSeats): с fromSpread (неизвестный — с первого разворота)
// и дальше по альбому; с обложки (model.CoverID) — сначала обложка. Фото из альбома (и стоящие
// только копией, 098-17), с фонов и стикеров пропускаются. left — сколько не встало.
func FillPlaces(p *model.Project, photos []model.PhotoInfo, fromSpread string) (left int) {
	photos = model.DedupePhotos(photos)
	onBg, files := p.OnDecor(), p.PlacedFiles()
	fresh := slices.DeleteFunc(slices.Clone(photos), func(ph model.PhotoInfo) bool { return files[model.BaseID(ph.ID)] || onBg[ph.ID] })
	ids := model.Remember(p, fresh)
	k := 0
	for _, st := range p.FillSeats(fromSpread) {
		if k == len(ids) {
			break
		}
		p.Put(st.Sheet, st.Place, ids[k])
		k++
	}
	return len(ids) - k
}
