package model

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// Табличка места (071): видимое имя, поиск места фото, жильцы, зажим.

func TestNameAt(t *testing.T) {
	file := PhotoRef{Path: "C:/a/Иванов Петя.jpg"}
	own := PhotoRef{Path: "C:/a/IMG_1.jpg", Person: &Person{Last: "Смирнов", First: "Коля", Subject: "Химия"}}
	subj := PhotoRef{Path: "C:/a/Иванов Петя.jpg", Person: &Person{Subject: "Химия"}}
	bare := PhotoRef{Path: "C:/a/IMG_2.jpg"}
	petrov := Person{Last: "Петров", First: "Ваня"}
	teacher := Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}
	for _, c := range []struct {
		name       string
		r          PhotoRef
		pl         *Place
		want       string // fmt %v видимого имени; "<nil>" — нет
		file, seat bool
	}{
		{"Авто: из файла", file, nil, "&{Иванов Петя  }", true, false},
		{"Авто: своё", own, nil, "&{Смирнов Коля  Химия}", false, false},
		{"Авто: нет", bare, nil, "<nil>", false, false},
		{"без таблички — как Name", file, &Place{}, "&{Иванов Петя  }", true, false},
		{"табличка перекрывает файл", file, &Place{Person: petrov}, "&{Петров Ваня  }", false, true},
		{"табличка перекрывает своё, предмет своего", own, &Place{Person: petrov}, "&{Петров Ваня  Химия}", false, true},
		{"предмет таблички главнее", own, &Place{Person: teacher}, "&{Сидорова Ольга  Математика}", false, true},
		{"табличка-предмет и своё имя", own, &Place{Person: Person{Subject: "Физика"}}, "&{Смирнов Коля  Физика}", false, true},
		{"табличка-предмет и имя из файла", subj, &Place{Person: Person{Subject: "Физика"}}, "&{Иванов Петя  Физика}", true, true},
		{"табличка-предмет без имени", bare, &Place{Person: Person{Subject: "Физика"}}, "&{   Физика}", false, true},
	} {
		who, file, seat := NameAt(c.r, c.pl)
		if got := fmt.Sprint(who); got != c.want || file != c.file || seat != c.seat {
			t.Errorf("%s: %s file=%v seat=%v, ждали %s %v %v", c.name, got, file, seat, c.want, c.file, c.seat)
		}
	}
	// Результат — копия: правка не трогает ни место, ни фото.
	pl := &Place{Person: petrov}
	who, _, _ := NameAt(own, pl)
	who.Last = "x"
	if pl.Person != petrov || own.Person.Last != "Смирнов" {
		t.Fatalf("NameAt отдал ссылку: место %+v, фото %+v", pl.Person, own.Person)
	}
}

// SeatNamed: true только у таблички с именем; табличка-предмет (имя фото из файла или своё),
// пустая табличка и «Авто» — false, хотя Place у видимой таблички стоит.
func TestNameViewAtSeatNamed(t *testing.T) {
	file := PhotoRef{Path: "C:/a/Иванов Петя.jpg"}
	own := PhotoRef{Path: "C:/a/IMG_1.jpg", Person: &Person{Last: "Козлов", First: "Олег"}}
	bare := PhotoRef{Path: "C:/a/IMG_2.jpg"}
	for _, c := range []struct {
		name  string
		r     PhotoRef
		pl    *Place
		named bool
		place string
	}{
		{"табличка с именем", file, &Place{ID: "p1", Person: Person{Last: "Петров", First: "Ваня"}}, true, "p1"},
		{"только отчество", file, &Place{ID: "p1", Person: Person{Middle: "Ильич"}}, true, "p1"},
		{"имя и предмет", own, &Place{ID: "p1", Person: Person{Last: "Сидорова", Subject: "Физика"}}, true, "p1"},
		{"предмет, имя из файла", file, &Place{ID: "p1", Person: Person{Subject: "Физика"}}, false, "p1"},
		{"предмет, своё имя", own, &Place{ID: "p1", Person: Person{Subject: "Физика"}}, false, "p1"},
		{"предмет, фото без имени", bare, &Place{ID: "p1", Person: Person{Subject: "Физика"}}, false, "p1"},
		{"место без таблички", file, &Place{ID: "p1"}, false, ""},
		{"Авто (места нет)", own, nil, false, ""},
	} {
		pv := NameViewAt(c.r, c.pl)
		if pv.SeatNamed != c.named || pv.Place != c.place {
			t.Errorf("%s: SeatNamed=%v Place=%q, ждали %v %q", c.name, pv.SeatNamed, pv.Place, c.named, c.place)
		}
	}
	if pv := NameViewAt(own, &Place{ID: "p1", Person: Person{Subject: "Физика"}}); pv.Last != "Козлов" || pv.Subject != "Физика" || pv.FromFile {
		t.Errorf("свое имя под табличкой-предметом: %+v", pv)
	}
}

func TestPlaceOf(t *testing.T) {
	p := New()
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID, Grid: []*Place{{ID: "k1", Photo: "c"}}}}
	p.Spreads = []*Spread{
		{ID: "s1", Photos: []string{"a"}, Grid: []*Place{{ID: "e1"}, {ID: "p1", Photo: "a"}}},
		{ID: "s2", Photos: []string{"b"}},
		{ID: "s3", Grid: []*Place{nil, {ID: "p3", Photo: "d"}}},
	}
	for id, want := range map[string]string{"a": "s1/p1", "c": "cover/k1", "d": "s3/p3", "b": "", "": "", "нет": ""} {
		s, pl := p.PlaceOf(id)
		got := ""
		if s != nil {
			got = s.ID + "/" + pl.ID
		}
		if got != want || (s == nil) != (pl == nil) {
			t.Errorf("PlaceOf(%q) = %s, ждали %q", id, got, want)
		}
	}
}

func TestOccupants(t *testing.T) {
	petya := Person{Last: "Иванов", First: "Петя"}
	if got := Occupants(nil); got != nil {
		t.Fatalf("nil: %+v", got)
	}
	if got := Occupants([]*Place{nil, {ID: "e"}}); got != nil {
		t.Fatalf("только пустые: %+v", got)
	}
	got := Occupants([]*Place{{Photo: "a", Person: petya}, {}, {Person: Person{Subject: "Физика"}}, nil, {Photo: "b"}})
	want := []Occupant{{"a", petya}, {"", Person{Subject: "Физика"}}, {"b", Person{}}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("жильцы %+v, ждали %+v", got, want)
	}
}

// Seat сажает фото через Put (уходит с прежнего места, хозяин места покидает лист) и ставит
// табличку жильца; жилец без таблички снимает прежнюю, без фото — фото места не трогает.
func TestSeat(t *testing.T) {
	petya, olga := Person{Last: "Иванов", First: "Петя"}, Person{Last: "Сидорова", First: "Ольга"}
	p := New()
	s := &Spread{ID: "s1", Photos: []string{"a", "b"}, Grid: []*Place{{ID: "p1", Photo: "a", Person: olga}, {ID: "p2", Photo: "b"}, {ID: "p3"}}}
	p.Spreads = []*Spread{s}
	p.Seat(s, s.Grid[2], Occupant{Photo: "a", Person: petya})
	p.Seat(s, s.Grid[1], Occupant{Photo: "", Person: olga})
	p.Seat(s, s.Grid[0], Occupant{})
	got := fmt.Sprint(s.Photos, " ", s.Grid[0].Photo, s.Grid[0].Person, s.Grid[1].Photo, s.Grid[1].Person, s.Grid[2].Photo, s.Grid[2].Person)
	if want := fmt.Sprint([]string{"b", "a"}, " ", "", Person{}, "b", olga, "a", petya); got != want {
		t.Fatalf("после Seat: %s, ждали %s", got, want)
	}
}

func TestClampSeatName(t *testing.T) {
	long := strings.Repeat("я", 100)
	c := ClampSeat(Person{Last: long, First: " Пе\nтя\u0000 ", Middle: "\u202e\t", Subject: "Мате\r\nматика"})
	if utf8.RuneCountInString(c.Last) != MaxPersonRunes || c.First != "Пе тя" || c.Middle != "" || c.Subject != "Мате матика" {
		t.Fatalf("зажим: %+v", c)
	}
	if again := ClampSeat(c); again != c {
		t.Fatalf("не идемпотентна: %+v → %+v", c, again)
	}
	for _, empty := range []Person{{}, {Last: "  ", First: "\n", Middle: "\u0000", Subject: "\t"}} {
		if got := ClampSeat(empty); got != (Person{}) {
			t.Errorf("пустая %+v → %+v", empty, got)
		}
	}
	if got := ClampSeat(Person{Subject: "Физика"}); got != (Person{Subject: "Физика"}) {
		t.Errorf("только предмет: %+v", got)
	}
	// Запас при Load: не влезла целиком — ноль, а запас не тронут.
	b := textBudget{blocks: 1, runes: 7}
	if got := clampSeatIn(Person{Last: "Иванов", First: "Петя"}, &b); got != (Person{}) || b.runes != 7 {
		t.Fatalf("сверх запаса: %+v, осталось %d", got, b.runes)
	}
	if got := clampSeatIn(Person{Last: "Иванов"}, &b); got.Last != "Иванов" || b.runes != 1 {
		t.Fatalf("в запасе: %+v, осталось %d", got, b.runes)
	}
	// Лист: nil-место пропускается; счёт (CheckTotals) и зажим (Load) — на одной границе.
	sheet := func() *Spread {
		return &Spread{Grid: []*Place{nil, {Person: Person{Last: "Иванов"}}, {}, {Person: Person{Subject: "Физика"}}}}
	}
	for runes, ok := range map[int]bool{11: false, 12: true} {
		b := textBudget{runes: runes}
		if got := sheet().spendSeats(&b, false); got != ok || ok && b.runes != 0 {
			t.Errorf("счёт при запасе %d: %v, осталось %d", runes, got, b.runes)
		}
		s, b2 := sheet(), textBudget{runes: runes}
		s.spendSeats(&b2, true)
		if kept := s.Grid[3].Person == (Person{Subject: "Физика"}); kept != ok || s.Grid[1].Person.Last != "Иванов" {
			t.Errorf("зажим при запасе %d: %+v", runes, s.Grid)
		}
	}
}

// ClonePlace копирует место без фото и таблички: деление и копия сетки не размножают имя.
func TestClonePlaceDropsPersonUnit(t *testing.T) {
	src := &Place{ID: "p1", X: 1, Y: 2, W: 30, H: 40, Photo: "a", Role: RoleTeacher, Shape: ShapeOval,
		Person: Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}}
	c := ClonePlace(src, map[string]bool{"p1": true})
	if c.ID == "p1" || c.Photo != "" || c.Person != (Person{}) || c.Role != RoleTeacher || c.Shape != ShapeOval || c.W != 30 {
		t.Fatalf("копия: %+v", c)
	}
	if src.Person.Last != "Сидорова" || src.Photo != "a" {
		t.Fatalf("источник изменён: %+v", src)
	}
}
