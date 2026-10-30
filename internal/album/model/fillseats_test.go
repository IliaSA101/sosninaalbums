package model

import (
	"slices"
	"testing"
)

// seatCover — проект с обложкой: сторонки 0…301 (задняя) и 307…608 (лицевая), корешок между.
func seatCover(t *testing.T, places ...*Place) *Project {
	t.Helper()
	pr, _ := CoverPresetOf(CoverDefault)
	p := New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	p.Cover = &Cover{Spec: pr.Spec, Sheet: &Spread{ID: CoverID, Grid: places}}
	if len(p.CoverSides()) != 2 {
		t.Fatal("нет сторонок")
	}
	return p
}

// seat — место с центром по X в cx; photo "" — пустое.
func seat(id string, cx float64, photo string) *Place {
	return &Place{ID: id, X: cx - 10, Y: 50, W: 20, H: 20, Photo: photo}
}

func ids(pls []*Place) (out []string) {
	for _, pl := range pls {
		out = append(out, pl.ID)
	}
	return out
}

// seatsOf — ID мест из посадки: «лист:место».
func seatsOf(ss []Seat) (out []string) {
	for _, st := range ss {
		out = append(out, st.Sheet.ID+":"+st.Place.ID)
	}
	return out
}

// Порядок обложки: лицевая → задняя → остальные (корешок), каждая группа — в порядке Grid.
func TestCoverSeatsOrder(t *testing.T) {
	p := seatCover(t, seat("b1", 100, ""), seat("sp", 304, ""), seat("f1", 400, ""), seat("b2", 50, ""), seat("f2", 500, ""), seat("far", 900, ""))
	// Центр правее области лицевой — всё равно лицевая (как grid.InArea: «не левее начала»).
	if got := ids(p.CoverSeats()); !slices.Equal(got, []string{"f1", "f2", "far", "b1", "b2", "sp"}) {
		t.Fatalf("порядок %v", got)
	}
	// Граница: центр ровно на краю области — левая берёт «строго левее», правая — «не левее».
	p = seatCover(t, seat("onL", 301, ""), seat("onR", 307, ""))
	if got := ids(p.CoverSeats()); !slices.Equal(got, []string{"onR", "onL"}) {
		t.Fatalf("граница %v", got)
	}
}

// Края: нет обложки, пустая обложка; места с фото остаются в CoverSeats, но не в FillSeats.
func TestCoverSeatsEdges(t *testing.T) {
	if New().CoverSeats() != nil || len(New().FillSeats(CoverID)) != 0 {
		t.Fatal("без обложки места есть")
	}
	p := seatCover(t)
	if len(p.CoverSeats()) != 0 || len(p.FillSeats(CoverID)) != 0 {
		t.Fatal("мест нет, а они есть")
	}
	p = seatCover(t, seat("b", 100, "x"), seat("f", 400, ""), seat("s", 304, ""))
	if got := ids(p.CoverSeats()); len(got) != 3 || got[0] != "f" {
		t.Fatalf("CoverSeats %v", got)
	}
	if got := seatsOf(p.FillSeats(CoverID)); !slices.Equal(got, []string{"cover:f", "cover:s"}) {
		t.Fatalf("FillSeats %v", got)
	}
}

// Пустая зона (поля больше сторонки): сторонок нет — все места «остальные», в порядке Grid.
func TestCoverSeatsNoZone(t *testing.T) {
	p := seatCover(t, seat("b", 100, ""), seat("f", 400, ""), seat("s", 304, ""))
	p.Cover.Spec.Safe = 500
	if len(p.CoverSides()) != 0 {
		t.Fatal("подготовка: зона не пуста")
	}
	if got := ids(p.CoverSeats()); !slices.Equal(got, []string{"b", "f", "s"}) {
		t.Fatalf("порядок %v", got)
	}
}

func spreadWith(id string, photos ...string) *Spread {
	s := &Spread{ID: id}
	for i, ph := range photos {
		s.Grid = append(s.Grid, &Place{ID: id + string(rune('a'+i)), X: 10, Y: 10, W: 20, H: 20, Photo: ph})
	}
	return s
}

// FillSeats: с обложки — обложка, затем все развороты; с разворота — с него до конца;
// неизвестный и пустой ID — с первого разворота (CoverID без обложки — тоже).
func TestFillSeatsFrom(t *testing.T) {
	p := seatCover(t, seat("f", 400, ""), seat("b", 100, ""))
	p.Spreads = []*Spread{spreadWith("s1", "", "x"), spreadWith("s2", "", ""), spreadWith("s3", "")}
	cases := map[string][]string{
		CoverID: {"cover:f", "cover:b", "s1:s1a", "s2:s2a", "s2:s2b", "s3:s3a"},
		"s1":    {"s1:s1a", "s2:s2a", "s2:s2b", "s3:s3a"},
		"s2":    {"s2:s2a", "s2:s2b", "s3:s3a"},
		"s3":    {"s3:s3a"},
		"":      {"s1:s1a", "s2:s2a", "s2:s2b", "s3:s3a"},
		"нет":   {"s1:s1a", "s2:s2a", "s2:s2b", "s3:s3a"},
	}
	for from, want := range cases {
		if got := seatsOf(p.FillSeats(from)); !slices.Equal(got, want) {
			t.Errorf("с %q: %v, ждали %v", from, got, want)
		}
	}
	p.Cover = nil
	if got := seatsOf(p.FillSeats(CoverID)); len(got) != 4 || got[0] != "s1:s1a" {
		t.Errorf("без обложки: %v", got)
	}
	if len((&Project{}).FillSeats("s1")) != 0 {
		t.Error("пустой проект")
	}
}

// Виньетки берутся, как и прежде (fillable без правила вида): место виньетки в списке.
func TestFillSeatsTakesVignette(t *testing.T) {
	p := New()
	s := spreadWith("v", "", "")
	s.Vig = &Vignette{Tpl: "grid"}
	p.Spreads = []*Spread{s}
	if got := len(p.FillSeats("v")); got != 2 {
		t.Fatalf("мест %d", got)
	}
}

// FillCounts — те же числа, что len(FillSeats) с каждого листа.
func TestFillCountsMatchSeats(t *testing.T) {
	p := seatCover(t, seat("f", 400, ""), seat("b", 100, "x"), seat("s", 304, ""))
	p.Spreads = []*Spread{spreadWith("s1", "", "x"), spreadWith("s2"), spreadWith("s3", "", "")}
	from, cover := p.FillCounts()
	if cover != len(p.FillSeats(CoverID)) || len(from) != 3 {
		t.Fatalf("обложка %d, %v", cover, from)
	}
	for i, s := range p.Spreads {
		if from[i] != len(p.FillSeats(s.ID)) {
			t.Errorf("с %s: %d ≠ %d", s.ID, from[i], len(p.FillSeats(s.ID)))
		}
	}
	if f, c := New().FillCounts(); f == nil || len(f) != 0 || c != 0 {
		t.Errorf("пустой: %v %d", f, c)
	}
	// Без обложки CoverFill — 0, хотя на разворотах места есть.
	p.Cover = nil
	if f, c := p.FillCounts(); c != 0 || f[0] != 3 {
		t.Errorf("без обложки: %v %d", f, c)
	}
}
