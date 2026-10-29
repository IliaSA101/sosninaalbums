package spreads

import (
	"errors"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Бросок фото на ручной разворот (спека 042, design §2): фото садятся в пустые места; мест нет —
// новые места плиткой (geom.Tile); мест меньше, чем фото, — сначала вопрос (Ask) без правки, UI
// зовёт ещё раз с ответом. Всё — одна правка Editor.try: один шаг отмены, ошибка — откат.

// Ответ на вопрос «что сделать с остальными» (DropReq.Rest); "" — спросить.
const (
	RestAdd  = "add"  // остальные — плиткой (нетронутая виньетка — ученики)
	RestSkip = "skip" // остальные не трогать
)

// DropReq — бросок: лист Spread, место Place ("" — на лист), фото по порядку, At — доля ширины
// листа под курсором 0…1 (< 0, NaN, Inf — точки нет), Rest — ответ на вопрос.
type DropReq struct {
	Spread, Place string
	Photos        []model.PhotoInfo
	At            float64
	Rest          string
}

// VigHooks — виньетка из области grid (spreads её не импортирует — depguard): ядро передаёт
// функции с планом виньетки.
type VigHooks struct {
	Untouched func(p *model.Project, s *model.Spread) bool
	Grow      func(p *model.Project, s *model.Spread, k int) error
}

// DropOut — итог Drop (поля App.DropResult без вида).
type DropOut struct {
	Ask   bool     // правки не было: UI спрашивает и зовёт Drop с Rest
	Fit   int      // сколько встанет (встало) в имеющиеся места, с целевым
	Total int      // сколько фото пришло на лист (без тех, что уже в его местах)
	Fresh []string // ID новых мест плитки
	Grown int      // на сколько выросло «Учеников» нетронутой виньетки
	Out   bool     // прежнее фото целевого места ушло в ленту (замена)
	Left  int      // не встало: «Не добавлять» или обложка
	Small bool     // место плитки мельче 1/10 меньшей стороны страницы
	Named []string // получили имя из названия файла (060; пишет ядро после Drop)
}

// ErrTileSmall — место плитки вышло бы короче geom.MinPlace: зажим поднял бы места до 5 мм и
// наложил друг на друга.
var ErrTileSmall = errors.New("слишком много фото для свободной формы — бросьте их на новый разворот или лист")

// smallPart — место плитки короче этой доли меньшей стороны страницы — «мелкое» (подсказка).
const smallPart = 10

// Drop — бросок фото на лист (design §2). «Авто» — как AddPhotos в конец; обложка — как
// PlacePhotos (без вопроса и плитки, лишние — Left). Неизвестный лист или место, нечего
// сажать — холостая. План считается до любых изменений: Ask оставляет проект байт в байт.
// Запрос копии (098) — новая запись-копия (model.ResolveCopies).
func Drop(p *model.Project, r DropReq, vig VigHooks) (DropOut, error) {
	photos := model.DedupePhotos(r.Photos)
	s := p.Sheet(r.Spread)
	if s == nil || len(photos) == 0 {
		return DropOut{}, nil
	}
	if len(s.Grid) == 0 {
		if p.IsCover(s) { // обложка без мест: «Авто» у неё не бывает, сажать некуда
			return DropOut{}, nil
		}
		if err := AddPhotos(p, s.ID, photos, -1); err != nil {
			return DropOut{}, err
		}
		return DropOut{Fit: len(photos), Total: len(photos)}, nil
	}
	at, pl := -1, (*model.Place)(nil)
	if r.Place != "" {
		if at, pl = s.FindPlace(r.Place); pl == nil {
			return DropOut{}, nil
		}
	}
	fit, total := planDrop(s, pl, photos)
	out := DropOut{Fit: fit, Total: total}
	rest := restOf(r.Rest, p.IsCover(s))
	switch {
	case total == 0:
		return DropOut{}, nil
	case rest == "" && fit == 0:
		rest = RestAdd // мест нет вовсе — спрашивать не о чем (сценарий 2)
	case rest == "" && fit < total:
		out.Ask = true
		return out, nil
	}
	// Запросы копий (098) — после вопроса: план их не различает (ни копии, ни запроса в местах
	// листа нет), а Ask оставляет проект нетронутым.
	photos, err := model.ResolveCopies(p, photos)
	if err != nil {
		return DropOut{}, err
	}
	grow := rest == RestAdd && fit < total && s.IsVignette() && vig.Untouched != nil && vig.Untouched(p, s)
	left := seat(p, s, pl, at, photos, &out)
	if len(left) == 0 {
		return out, nil
	}
	if err := addRest(p, s, rest, grow, left, tilePage(p, s, r.At, pl), vig, &out); err != nil {
		return DropOut{}, err
	}
	return out, nil
}

// restOf — ответ на вопрос: неизвестный — как "" (спросить); обложка — всегда «не добавлять»
// (как PlacePhotos: без вопроса и плитки, решение 48).
func restOf(rest string, cover bool) string {
	switch {
	case cover:
		return RestSkip
	case rest == RestAdd, rest == RestSkip:
		return rest
	}
	return ""
}

// planDrop — до правки, зеркало seat: сколько встанет и сколько пришло (у броска на место первое
// идёт на цель всегда; фото, что после него останутся в местах листа, не считаются). Пустые места —
// кроме целевого; первое фото из этого же листа на пустую цель освобождает своё место; фото не из
// листа на занятую цель выселяет хозяина (замена, обмен) — брошенный хозяин тоже в очереди.
func planDrop(s *model.Spread, pl *model.Place, photos []model.PhotoInfo) (fit, total int) {
	inGrid := map[string]bool{}
	for _, id := range model.GridPhotos(s) {
		inGrid[id] = true
	}
	lead, rest := 0, photos
	if pl != nil {
		lead, rest = 1, photos[1:]
		if pl.Photo != "" && !inGrid[photos[0].ID] {
			delete(inGrid, pl.Photo)
		}
	}
	queue := 0
	for _, ph := range rest {
		if !inGrid[ph.ID] {
			queue++
		}
	}
	empties := 0
	for _, q := range s.Grid {
		if q.Photo == "" && q != pl {
			empties++
		}
	}
	if pl != nil && pl.Photo == "" && inGrid[photos[0].ID] {
		empties++
	}
	return lead + min(queue, empties), lead + queue
}

// seat — первое фото на цель (DropOn), остальные — в пустые места: у броска на место — после
// целевого, потом с начала, у броска на лист — по порядку мест. «Уже в сетке» проверяется в момент
// посадки (как grid.PlacePhotos): выселенный хозяин цели, если брошен, садится в пустое, а не
// уходит в ленту (Out — только если не брошен). Возвращает, что не встало.
func seat(p *model.Project, s *model.Spread, pl *model.Place, at int, photos []model.PhotoInfo, out *DropOut) (left []string) {
	inAlbum := p.Placed(photos[0].ID)
	ids := model.Remember(p, photos)
	if pl != nil {
		prev := pl.Photo
		ids = ids[1:]
		out.Out = p.DropOn(s, pl, photos[0].ID, inAlbum) && !slices.Contains(ids, prev)
	}
	for _, id := range ids {
		if slices.Contains(model.GridPhotos(s), id) {
			continue // уже в этой сетке: порядок задают места
		}
		if dst := s.NextEmpty(at); dst != nil {
			p.Put(s, dst, id)
		} else {
			left = append(left, id)
		}
	}
	return left
}

// addRest — остальные фото по ответу: «не добавлять» — Left (фото остаются, где были:
// ссылки из ленты снимет PruneRefs); нетронутая виньетка — новые ученики; иначе — плитка.
func addRest(p *model.Project, s *model.Spread, rest string, grow bool, left []string, page int, vig VigHooks, out *DropOut) error {
	switch {
	case rest == RestSkip:
		out.Left = len(left)
	case grow:
		if err := vig.Grow(p, s, len(left)); err != nil {
			return err
		}
		out.Grown = len(left)
		for _, id := range left {
			// Grow дал ровно len(left) пустых мест учеников, так что nil не бывает; защита от
			// паники Put, если план шаблона когда-нибудь вернёт меньше.
			if dst := s.NextEmpty(-1); dst != nil {
				p.Put(s, dst, id)
			}
		}
	default:
		return tileRest(p, s, page, left, out)
	}
	return nil
}

// tileRest — новые места плиткой на странице page (поверх прежних, в конец Grid) и фото в них.
// Больше MaxPlaces мест — ErrTooManyPlaces; место короче geom.MinPlace — ErrTileSmall.
func tileRest(p *model.Project, s *model.Spread, page int, ids []string, out *DropOut) error {
	if len(s.Grid)+len(ids) > model.MaxPlaces {
		return model.ErrTooManyPlaces()
	}
	g := p.Geo(s)
	pg := g.Page(page)
	short := min(pg.W, pg.H)
	area := layout.Rect{X: pg.X + g.Margin, Y: pg.Y + g.Margin, W: pg.W - 2*g.Margin, H: pg.H - 2*g.Margin}
	rs := geom.Tile(area, p.PhotoAspects(ids), g.Gap, short/3)
	if len(rs) != len(ids) {
		return ErrTileSmall
	}
	for _, r := range rs {
		if min(r.W, r.H) < geom.MinPlace {
			return ErrTileSmall
		}
	}
	taken := p.PlaceIDs()
	for i, r := range rs {
		pl := &model.Place{ID: model.NewPlaceID(taken), X: r.X, Y: r.Y, W: r.W, H: r.H}
		p.ClampPlace(s, pl)
		s.Grid = append(s.Grid, pl)
		p.Put(s, pl, ids[i])
		out.Fresh = append(out.Fresh, pl.ID)
		out.Small = out.Small || min(r.W, r.H) < short/smallPart
	}
	return nil
}

// tilePage — страница плитки: под курсором (at); иначе у броска на место — страница его
// центра; иначе — с меньшей суммой площадей мест (центр места на странице), равенство — левая.
func tilePage(p *model.Project, s *model.Spread, at float64, pl *model.Place) int {
	g := p.Geo(s)
	switch {
	case at >= 0 && !math.IsInf(at, 0):
		return g.PageOf(at * g.W)
	case pl != nil:
		a := model.PlaceAABB(pl)
		return g.PageOf(a.X + a.W/2)
	}
	area := make([]float64, len(g.Pages))
	for _, q := range s.Grid {
		a := model.PlaceAABB(q)
		if i := g.PageOf(a.X + a.W/2); i < len(area) {
			area[i] += a.W * a.H
		}
	}
	best := 0
	for i := range area {
		if area[i] < area[best] {
			best = i
		}
	}
	return best
}
