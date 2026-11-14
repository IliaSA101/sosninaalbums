package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// SyncGrid — страховка после правки, а не механизм переноса (design 007 §0.3):
// место с фото не из Photos пустеет (и второе место того же фото), фото без
// места садится в первое пустое; Photos = непустые места по порядку. Пустых нет:
// repair (чтение файла, фото терять нельзя) — новое место, иначе errNoPlace.
// Идемпотентна: холостая правка остаётся холостой.
func (p *Project) SyncGrid(repair bool) error {
	var taken map[string]bool // ID мест проекта — только для починки, один раз на проход
	if repair {
		taken = p.PlaceIDs()
	}
	for _, s := range p.Sheets() {
		if len(s.Grid) == 0 {
			s.Grid = nil
			continue
		}
		if err := p.syncSpread(s, repair, taken); err != nil {
			return err
		}
	}
	// Пустые Photos обложки — nil, как их читает FixCoverSheet: иначе "photos":[] после
	// Save → Load стал бы null и правка, круг записи, не совпала бы по JSON.
	if c := p.Cover; c != nil && c.Sheet != nil && len(c.Sheet.Photos) == 0 {
		c.Sheet.Photos = nil
	}
	return nil
}

func (p *Project) syncSpread(s *Spread, repair bool, taken map[string]bool) error {
	in := make(map[string]bool, len(s.Photos))
	for _, id := range s.Photos {
		in[id] = true
	}
	seated := make(map[string]bool, len(s.Grid))
	for _, pl := range s.Grid {
		if pl.Photo == "" {
			continue
		}
		if !in[pl.Photo] || seated[pl.Photo] {
			pl.Photo = ""
			continue
		}
		seated[pl.Photo] = true
	}
	next := 0 // пустые места ищем по порядку, не возвращаясь к занятым
	for _, id := range s.Photos {
		if seated[id] {
			continue
		}
		for next < len(s.Grid) && s.Grid[next].Photo != "" {
			next++
		}
		var pl *Place
		switch {
		case next < len(s.Grid):
			pl = s.Grid[next]
		case !repair:
			return ErrNoPlace
		default:
			var err error
			if pl, err = p.AddPlace(s, 0, p.PhotoAspects([]string{id})[0], taken); err != nil {
				// После clampGrid так не бывает (фото и мест ≤ MaxPlaces); если всё же
				// случилось — разворот уходит в «Авто», но фото не теряются.
				s.Grid = nil
				return nil
			}
		}
		pl.Photo = id
		seated[id] = true
	}
	s.Photos = GridPhotos(s)
	return nil
}

// Freeze («Закрепить») переводит разворот в «Сетку» без изменения картинки:
// место i = ячейка i действующей раскладки, фото то же. У обычной ячейки
// пропорции фото, и cover при S = 1 — фото целиком; у ячейки «в край» cover по
// центру — прежний Fill. Layout не трогается: он нужен «Вернуть авто».
func (p *Project) Freeze(s *Spread) {
	if len(s.Grid) > 0 {
		return
	}
	cur, _, ok := p.Layout(s)
	if !ok || len(cur.Rects) != len(s.Photos) {
		return
	}
	taken := p.PlaceIDs()
	grid := make([]*Place, len(cur.Rects))
	for i, r := range cur.Rects {
		grid[i] = &Place{ID: NewPlaceID(taken), X: r.X, Y: r.Y, W: r.W, H: r.H, Photo: s.Photos[i]}
		p.ClampPlace(s, grid[i])
	}
	s.Grid = grid
}

// Unfreeze («Вернуть авто»): места снимаются, ключ ставится через setLayout —
// сохранённый, если Build строит его для нынешних фото, иначе лучшая. Лучшая
// хранится как "", иначе её выбор в галерее перестал бы быть холостым.
// Кадры фото остаются: в «Авто» они тоже действуют. Мест нет — табличек тоже (071):
// табличка места с фото становится именем фото (подпись та же), пустых — пропадает.
func (p *Project) Unfreeze(s *Spread) {
	if len(s.Grid) == 0 {
		return
	}
	p.seatsToPhotos(s)
	s.Grid = nil
	key := s.Layout
	if key != "" {
		asp, prm := p.LayoutIn(s.Photos)
		if _, ok := layout.Build(key, asp, prm); !ok {
			key = ""
		}
	}
	p.SetLayout(s, key)
}

// seatsToPhotos — видимое имя фото на месте с табличкой (NameAt) переходит в PhotoRef.Person.
// Имя из файла остаётся из файла (у фото только предмет таблички) — карточка по-прежнему
// покажет «из имени файла».
func (p *Project) seatsToPhotos(s *Spread) {
	for _, pl := range s.Grid {
		r, ok := p.Photos[pl.Photo]
		if pl.Photo == "" || !ok || pl.Person == (Person{}) {
			continue
		}
		who, fromFile, _ := NameAt(r, pl)
		c := *who
		if fromFile {
			c = Person{Subject: c.Subject}
		}
		r.Person = &c
		p.Photos[pl.Photo] = r
	}
}

// EmptyGridDupes — у места, чьё фото не в Photos разворота, или второго места
// того же фото, Photo = "": геометрия сетки остаётся, фото не стоит дважды.
func EmptyGridDupes(s *Spread) {
	if len(s.Grid) == 0 {
		return
	}
	in := make(map[string]bool, len(s.Photos))
	for _, id := range s.Photos {
		in[id] = true
	}
	for _, pl := range s.Grid {
		if pl.Photo != "" && !in[pl.Photo] {
			pl.Photo = ""
		}
		delete(in, pl.Photo)
	}
}
