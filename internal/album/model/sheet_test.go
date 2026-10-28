package model

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// SpineTextID — первая повёрнутая надпись обложки с центром в корешке; нет обложки, листа,
// надписей, поворота или центр вне корешка — "".
func TestSpineTextID(t *testing.T) {
	p := New()
	if id := p.SpineTextID(); id != "" {
		t.Fatalf("без обложки: %q", id)
	}
	pr, _ := CoverPresetOf(CoverDefault)
	p.Cover = &Cover{Spec: pr.Spec} // листа нет — не паника
	if id := p.SpineTextID(); id != "" {
		t.Fatalf("обложка без листа: %q", id)
	}
	p.Cover.Sheet = &Spread{ID: CoverID}
	if id := p.SpineTextID(); id != "" {
		t.Fatalf("нет надписей: %q", id)
	}

	cg := p.CoverGeo()
	if !(cg.X[4] > cg.X[3]) {
		t.Fatalf("у корешка нет ширины: %+v", cg.X)
	}
	// at — надпись с центром (cx, 100) и поворотом rot.
	at := func(id string, cx, rot float64) *TextBlock {
		tb := &TextBlock{ID: id, Text: "Ивановы", Style: PlainStyle(), W: 40, Rot: rot}
		dx, dy := geom.Rotate(rot, tb.W/2, BoxOf(tb).H/2) // X, Y — угол, вокруг него и поворот
		tb.X, tb.Y = cx-dx, 100-dy
		return tb
	}
	mid := (cg.X[3] + cg.X[4]) / 2
	p.Cover.Sheet.Texts = []*TextBlock{
		at("000000000001", mid, 0),          // не повёрнута
		at("000000000002", cg.X[6]+20, -90), // повёрнута, но на лицевой
		at("000000000003", cg.X[3]-20, -90), // за левой границей корешка
		at("000000000004", cg.X[4]+20, -90), // за правой
		at("000000000005", mid, 90),         // в корешке
		at("000000000006", mid, -90),        // и вторая в корешке — первая выигрывает
	}
	if id := p.SpineTextID(); id != "000000000005" {
		t.Fatalf("ждали первую повёрнутую в корешке (000000000005), получили %q", id)
	}
	p.Cover.Sheet.Texts = p.Cover.Sheet.Texts[:4]
	if id := p.SpineTextID(); id != "" {
		t.Fatalf("в корешке только неповёрнутая и чужие зоны: %q", id)
	}
}

// ClampGrid: сетка из одних негодных мест (NaN в рамке) — «Авто» (Grid nil, не пустой срез);
// годные места рядом на других листах остаются; фото выброшенного места остаётся в Photos.
func TestClampGridAllInvalid(t *testing.T) {
	nan := math.NaN()
	p := New()
	p.Photos["a"] = PhotoRef{Path: "a.jpg", W: 3000, H: 2000}
	p.Photos["b"] = PhotoRef{Path: "b.jpg", W: 3000, H: 2000}
	bad := &Spread{ID: "000000000001", Photos: []string{"a"}, Grid: []*Place{
		{ID: "000000000011", X: nan, Y: 10, W: 50, H: 50, Photo: "a"},
		nil,
		{ID: "000000000012", X: 10, Y: 10, W: math.Inf(1), H: 50},
	}}
	good := &Spread{ID: "000000000002", Photos: []string{"b"}, Grid: []*Place{
		{ID: "000000000013", X: 10, Y: 10, W: 50, H: 50, Photo: "b"},
		{ID: "000000000014", X: nan, Y: 10, W: 50, H: 50},
	}}
	empty := &Spread{ID: "000000000003", Grid: []*Place{}}
	p.Spreads = []*Spread{bad, good, empty}
	p.ClampGrid()
	if bad.Grid != nil || len(bad.Photos) != 1 || bad.Photos[0] != "a" {
		t.Fatalf("негодная сетка: Grid %v, фото %v", bad.Grid, bad.Photos)
	}
	if len(good.Grid) != 1 || good.Grid[0].Photo != "b" {
		t.Fatalf("годное место потеряно: %+v", good.Grid)
	}
	if empty.Grid != nil {
		t.Fatalf("пустая сетка не nil: %#v", empty.Grid)
	}
	p.ClampGrid() // идемпотентность
	if bad.Grid != nil || len(good.Grid) != 1 {
		t.Fatalf("повторный зажим изменил сетки: %v %v", bad.Grid, good.Grid)
	}
}
