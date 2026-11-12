package spreads

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ReplacePhoto — фото ph на место фото target листа «Авто» spreadID (168): раскладка и ячейки те
// же. left = 1 — прежнее фото ушло в ленту. Ручной лист, обложка, неизвестные лист или фото,
// ph == target — холостая. Фото из альбома (inAlbum) меняется с target местами, как DropOn ручного.
// Геометрию держит свой кадр (098), его ставит keepCell; дальше KeepLayouts конвейера edit видит
// те же пропорции и замки и строит те же прямоугольники.
func ReplacePhoto(p *model.Project, spreadID, target string, ph model.PhotoInfo) (left int, err error) {
	_, s := p.Find(spreadID)
	if s == nil || len(s.Grid) > 0 || p.IsCover(s) {
		return 0, nil
	}
	i := slices.Index(s.Photos, target)
	if i < 0 {
		return 0, nil
	}
	phs, err := model.ResolveCopies(p, []model.PhotoInfo{ph}) // запрос копии (098) — новая запись
	if err != nil {
		return 0, err
	}
	id := phs[0].ID
	if id == target {
		return 0, nil
	}
	// Ячейки меряем до правки: после подстановки раскладка уже пересчитана бы по новым пропорциям.
	w, own := cellOf(p, s, i)
	s2, j := whereAuto(p, id)
	w2, own2 := cellOf(p, s2, j)
	inAlbum := p.Placed(id)
	model.Remember(p, phs)
	if !inAlbum {
		s.Photos[i], left = id, 1 // запись прежнего фото снимет PruneRefs конвейера
		keepCell(p, id, w, own)
		return left, nil
	}
	p.Swap(id, target)
	keepCell(p, id, w, own)
	if s2 != nil {
		keepCell(p, target, w2, own2)
	} else {
		r := p.Photos[target] // на ручном месте свой кадр не действует: «Вернуть авто» посчитает его заново
		r.Asp = 0
		p.Photos[target] = r
	}
	return 0, nil
}

// whereAuto — лист «Авто» (не сетка и не обложка), где стоит фото id, и его индекс; иначе nil.
func whereAuto(p *model.Project, id string) (*model.Spread, int) {
	for _, s := range p.Sheets() {
		if i := slices.Index(s.Photos, id); i >= 0 {
			if len(s.Grid) > 0 || p.IsCover(s) {
				return nil, -1
			}
			return s, i
		}
	}
	return nil, -1
}

// cellOf — пропорции ячейки i листа «Авто» s (порядок Rects = порядок Photos) и есть ли у её
// фото свой кадр (замок подрезки, 104). Нет листа или раскладка не строится — 0.
func cellOf(p *model.Project, s *model.Spread, i int) (w float64, own bool) {
	if s == nil {
		return 0, false
	}
	r, _, ok := p.Layout(s)
	if !ok || i >= len(r.Rects) || !(r.Rects[i].H > 0) {
		return 0, false
	}
	return r.Rects[i].W / r.Rects[i].H, p.Photos[s.Photos[i]].Asp > 0
}

// keepCell — свой кадр фото id = пропорции ячейки w (замер 168 design §2, правило 5): кроме
// случая, когда они равны пропорциям самого фото и замок подрезки не нужен (у прежнего фото
// своего кадра не было или подрезки нет): иначе у необрезанного фото появилась бы строка «свой кадр».
func keepCell(p *model.Project, id string, w float64, own bool) {
	r := p.Photos[id]
	a, e := model.ClampAsp(w), model.ClampAsp(p.PhotoAspects([]string{id})[0])
	if a == e && (!own || p.Settings.Crop == 0) {
		a = 0
	}
	r.Asp = a
	p.Photos[id] = r
}
