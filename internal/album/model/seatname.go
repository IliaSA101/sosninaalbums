package model

// Табличка места (спека 071): имя из списка живёт на месте (Place.Person), фото на месте
// показывает её. Действия с фото (обмен, бросок, замена, «Убрать») таблички не трогают —
// она поле места; код нужен там, где места пересоздаются (очередь жильцов) и при «Вернуть авто».

// NameAt — видимое имя фото r на месте pl (nil — «Авто» или фото не на месте): табличка →
// своё → строгое из файла (design 071 §4). Табличка с именем перекрывает имя фото, предмет —
// таблички, а без него — свой предмет фото; табличка только с предметом даёт имя фото (своё
// или из файла) с этим предметом. fromFile — части имени взяты из имени файла; seat — у места
// есть табличка. who — копия: правка результата модель не меняет.
func NameAt(r PhotoRef, pl *Place) (who *Person, fromFile, seat bool) {
	if pl == nil || pl.Person == (Person{}) {
		who, fromFile = r.Name()
		return who, fromFile, false
	}
	t := pl.Person
	if t.Named() {
		if t.Subject == "" && r.Person != nil {
			t.Subject = r.Person.Subject
		}
		return &t, false, true
	}
	c := Person{Subject: t.Subject}
	if own, file := r.Name(); own != nil && own.Named() {
		c.Last, c.First, c.Middle, fromFile = own.Last, own.First, own.Middle, file
	}
	return &c, fromFile, true
}

// PlaceOf — лист (обложка тоже) и место с фото id; nil — фото не на месте. Пустое id — nil:
// у пустых мест Photo тоже "".
func (p *Project) PlaceOf(id string) (*Spread, *Place) {
	if id == "" {
		return nil, nil
	}
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			if pl != nil && pl.Photo == id {
				return s, pl
			}
		}
	}
	return nil, nil
}

// Occupant — жилец места при пересоздании мест: фото и/или табличка. Едут вместе — иначе
// пересборка разнесла бы имя и лицо по разным местам.
type Occupant struct {
	Photo  string
	Person Person
}

// Occupants — жильцы мест pls по порядку: места с фото или табличкой; пустые без таблички
// пропускаются (сжатие к началу, как очередь фото до 071).
func Occupants(pls []*Place) []Occupant {
	var out []Occupant
	for _, pl := range pls {
		if pl != nil && (pl.Photo != "" || pl.Person != (Person{})) {
			out = append(out, Occupant{Photo: pl.Photo, Person: pl.Person})
		}
	}
	return out
}

// Seat — поселить жильца o на место pl листа s: фото — через Put (единственный способ
// посадить), табличка — поле места (ноль снимает прежнюю).
func (p *Project) Seat(s *Spread, pl *Place, o Occupant) {
	if o.Photo != "" {
		p.Put(s, pl, o.Photo)
	}
	pl.Person = o.Person
}

// ClampSeat — табличка из правки: части имени и предмет — как ClampPerson (одна строка,
// ≤ MaxPersonRunes); всё пусто — ноль (таблички нет).
func ClampSeat(t Person) Person {
	if c := ClampPerson(t); c != nil {
		return *c
	}
	return Person{}
}

// clampSeatIn — табличка при Load: ClampSeat и знаки из общего запаса; не влезла — ноль.
func clampSeatIn(t Person, b *textBudget) Person {
	c := ClampSeat(t)
	if !b.takeRunes(c.runes()) {
		return Person{}
	}
	return c
}

// spendSeats — таблички мест листа s в общем запасе текста (spendTexts): fix — зажим и
// отбрасывание не влезших (Load), иначе false — запас превышен (CheckTotals). Порядок один.
func (s *Spread) spendSeats(b *textBudget, fix bool) bool {
	for _, pl := range s.Grid {
		switch {
		case pl == nil:
		case fix:
			pl.Person = clampSeatIn(pl.Person, b)
		case !b.takeRunes(pl.Person.runes()):
			return false
		}
	}
	return true
}
