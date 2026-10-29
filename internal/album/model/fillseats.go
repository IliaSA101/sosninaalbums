package model

// Seat — пустое место, куда «Заполнить места» посадит фото, в порядке посадки.
type Seat struct {
	Sheet *Spread
	Place *Place
}

// fillable — берёт ли «Заполнить места» это место листа s. Единственное правило и для числа на
// кнопке, и для посадки: разойтись они не могут. Виньетки (031/071) берутся, как и прежде;
// решение 131-10 (пропуск по виду) меняет только эту функцию.
func fillable(_ *Spread, pl *Place) bool { return pl.Photo == "" }

// CoverSeats — места листа обложки в порядке посадки: лицевая → задняя → остальные (корешок; при
// пустой зоне сторонки — все места: место за пунктиром с центром на странице остаётся у своей
// сторонки), каждая группа — в порядке Grid. Сторонка места — как у grid.InArea
// (центр по X относительно области left/right), но только у сторонок с непустой зоной
// (CoverSides): иначе места ушли бы в сторонку, которой grid не признаёт. Нет обложки — nil.
// Лежит здесь, а не в grid: spreads не может импортировать grid (depguard).
func (p *Project) CoverSeats() []*Place {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return nil
	}
	s := p.Cover.Sheet
	var front, back []*Place
	taken := map[*Place]bool{}
	for _, scope := range []string{ScopeRight, ScopeLeft} {
		if !p.hasCoverSide(scope) {
			continue
		}
		a, ok := p.AreaOf(s, scope)
		if !ok {
			continue
		}
		for _, pl := range s.Grid {
			c := pl.X + pl.W/2
			// Левая — центр левее конца задней страницы, правая — не левее начала лицевой.
			if taken[pl] || (scope == ScopeLeft && c >= a.X+a.W) || (scope == ScopeRight && c < a.X) {
				continue
			}
			taken[pl] = true
			if scope == ScopeRight {
				front = append(front, pl)
			} else {
				back = append(back, pl)
			}
		}
	}
	out := append(front, back...)
	for _, pl := range s.Grid {
		if !taken[pl] {
			out = append(out, pl)
		}
	}
	return out
}

// hasCoverSide — у сторонки scope есть безопасная зона.
func (p *Project) hasCoverSide(scope string) bool {
	for _, sd := range p.CoverSides() {
		if sd.Scope == scope {
			return true
		}
	}
	return false
}

// FillSeats — пустые места, которые займёт «Заполнить места» с листа from: CoverID — сначала
// обложка (CoverSeats), затем все развороты с первого; ID разворота — с него до конца альбома;
// неизвестный — с первого разворота. Порядок развороту — Grid, как прежде.
func (p *Project) FillSeats(from string) []Seat {
	var out []Seat
	start := 0
	if from == CoverID {
		for _, pl := range p.CoverSeats() {
			if fillable(p.Cover.Sheet, pl) {
				out = append(out, Seat{p.Cover.Sheet, pl})
			}
		}
	} else if i, _ := p.Find(from); i > 0 {
		start = i
	}
	for _, s := range p.Spreads[start:] {
		for _, pl := range s.Grid {
			if fillable(s, pl) {
				out = append(out, Seat{s, pl})
			}
		}
	}
	return out
}

// FillCounts — сколько мест займёт «Заполнить» одним проходом: from[i] — с разворота i,
// cover — с обложки (обложка + все развороты); обложки нет — 0 (с неё не начать, UI число не
// читает). Суффиксные суммы по тому же fillable, что и FillSeats: число и посадка не расходятся
// (TestFillCountMatchesFill, TestFillViewCountMatchesFill).
func (p *Project) FillCounts() (from []int, cover int) {
	from = make([]int, len(p.Spreads))
	n := 0
	for i := len(p.Spreads) - 1; i >= 0; i-- {
		s := p.Spreads[i]
		for _, pl := range s.Grid {
			if fillable(s, pl) {
				n++
			}
		}
		from[i] = n
	}
	if p.Cover != nil && p.Cover.Sheet != nil {
		cover = n
		for _, pl := range p.Cover.Sheet.Grid {
			if fillable(p.Cover.Sheet, pl) {
				cover++
			}
		}
	}
	return from, cover
}
