package model

import (
	"fmt"
	"testing"
)

// keepProject — лист-сетка с местами places; фото 3:2 (кроме "v" — 2:3) по местам.
func keepProject(places ...*Place) (*Project, *Spread) {
	p := New()
	s := &Spread{ID: "s1"}
	for i, pl := range places {
		pl.ID = fmt.Sprintf("%012x", i+1)
		if pl.Photo == "" {
			continue
		}
		w, h := 300, 200
		if pl.Photo == "v" {
			w, h = 200, 300
		}
		p.Photos[pl.Photo] = PhotoRef{Path: "/" + pl.Photo + ".jpg", W: w, H: h}
		s.Photos = append(s.Photos, pl.Photo)
	}
	s.Grid = places
	p.Spreads = []*Spread{s}
	return p, s
}

// TestKeepCropsRule — правило KeepCrops (098 design §3.2): окно рамочки, а не место; порог 1 %;
// четверть поворота входит в пропорции фото; «Целиком» и пустое — без своего кадра; apply = false
// проект не меняет, apply снимает прежний свой кадр у неправленых.
func TestKeepCropsRule(t *testing.T) {
	turned := &Place{X: 0, Y: 0, W: 100, H: 150, Photo: "t"} // 2:3 у фото 3:2, повёрнутого на 90°
	framed := &Place{X: 0, Y: 0, W: 150, H: 100, Photo: "f", Border: &Border{Kind: "mat", Width: 20}}
	contain := &Place{X: 0, Y: 0, W: 100, H: 100, Photo: "c", Fit: FitContain}
	p, s := keepProject(
		&Place{X: 0, Y: 0, W: 136.5, H: 81, Photo: "a"}, // 1,685 — правлено
		&Place{X: 0, Y: 0, W: 151, H: 100, Photo: "b"},  // 1,51 — в пределах 1 %
		&Place{X: 0, Y: 0, W: 152, H: 100, Photo: "d"},  // 1,52 — за пределом
		&Place{X: 0, Y: 0, W: 100, H: 150, Photo: "v"},  // 2:3 как фото
		turned, framed, contain,
		&Place{X: 0, Y: 0, W: 50, H: 100}, // пустое
	)
	r := p.Photos["t"]
	r.Frame = &Frame{S: 1, R: 90}
	p.Photos["t"] = r
	for _, id := range []string{"b", "c"} { // прежний свой кадр
		r := p.Photos[id]
		r.Asp = 2
		p.Photos[id] = r
	}
	win, _ := p.BorderWindow(framed)
	if win.W/win.H < 1.5*1.01 {
		t.Fatalf("окно рамочки %v × %v — тест не про окно", win.W, win.H)
	}
	before := string(Marshal(p))
	if n := KeepCrops(p, s, false); n != 3 {
		t.Fatalf("счёт %d, ждали 3 (a, d, f)", n)
	}
	if string(Marshal(p)) != before {
		t.Fatal("счёт изменил проект")
	}
	if n := KeepCrops(p, s, true); n != 3 {
		t.Fatalf("apply: %d", n)
	}
	want := map[string]float64{"a": ClampAsp(136.5 / 81), "b": 0, "d": ClampAsp(1.52), "v": 0, "t": 0, "f": ClampAsp(win.W / win.H), "c": 0}
	for id, a := range want {
		if got := p.Photos[id].Asp; got != a {
			t.Fatalf("Asp %s = %v, ждали %v", id, got, a)
		}
	}
	after := string(Marshal(p))
	if KeepCrops(p, s, true); string(Marshal(p)) != after {
		t.Fatal("повтор изменил проект")
	}
	if n := KeepCrops(p, &Spread{ID: "s2"}, true); n != 0 {
		t.Fatalf("лист без мест: %d", n)
	}
}
