package model

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Рамочки мест и прозрачность фото (079, design §1–2, §12).

// borderP — проект с картинкой рамочки «pic» 3000×4000 и записью без размеров «nodims».
func borderP() *Project {
	p := New()
	p.Photos["pic"] = PhotoRef{Lib: "my:0123456789abcdef", W: 3000, H: 4000}
	p.Photos["nodims"] = PhotoRef{Path: "C:/x/r.png"}
	return p
}

func TestClampBorder(t *testing.T) {
	p := borderP()
	cases := []struct {
		in, want *Border
	}{
		{nil, nil},
		{&Border{}, nil},
		{&Border{Kind: "line", Photo: "pic"}, nil}, // оба — нельзя
		{&Border{Kind: "Line"}, nil},               // не по грамматике
		{&Border{Kind: "x y"}, nil},
		{&Border{Photo: "нет"}, nil}, // записи нет
		{&Border{Kind: "line"}, &Border{Kind: "line"}},
		{&Border{Kind: "future-kind", Inks: []string{"#AABBCC", "#111111", "#222222"}, Width: 3},
			&Border{Kind: "future-kind", Inks: []string{"#aabbcc", "#111111"}, Width: 3}}, // неизвестный хранится
		{&Border{Kind: "line", Inks: []string{"#ABCDEF", "#000000"}, Width: math.NaN(), Scale: 2, Cut: [4]float64{0.1}, Open: true, Preset: "sa:fr-white-line"},
			&Border{Kind: "line", Inks: []string{"#abcdef"}, Preset: "sa:fr-white-line"}},
		{&Border{Kind: "mat", Width: 0.05, Preset: "my:x"}, &Border{Kind: "mat", Width: 0.2}},
		{&Border{Kind: "mat", Width: 1e308}, &Border{Kind: "mat", Width: 20}},
		{&Border{Kind: "soft", Inks: []string{"#000000"}, Width: 7.04}, &Border{Kind: "soft", Width: 7}},
		{&Border{Photo: "pic", Inks: []string{"#000000"}, Width: 5, Scale: 1, Cut: [4]float64{0.6, 1e308, -1, math.NaN()}, Preset: "sa:fr-mat"},
			&Border{Photo: "pic", Cut: [4]float64{gen.MaxCut, gen.MaxCut, 0, 0}}},
		{&Border{Photo: "pic", Scale: 1e308, Open: true}, &Border{Photo: "pic", Scale: 2, Open: true}},
		{&Border{Photo: "pic", Scale: 0.123}, &Border{Photo: "pic", Scale: 0.5}},
		{&Border{Photo: "pic", Scale: 1.234}, &Border{Photo: "pic", Scale: 1.23}},
		{&Border{Photo: "pic", Scale: math.Inf(1)}, &Border{Photo: "pic"}},
	}
	for i, c := range cases {
		got := ClampBorder(p, c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("случай %d: %+v → %+v, ждали %+v", i, c.in, got, c.want)
		}
		if again := ClampBorder(p, got); !reflect.DeepEqual(again, got) {
			t.Errorf("случай %d: не идемпотентна: %+v → %+v", i, got, again)
		}
		if got != nil && got == c.in {
			t.Errorf("случай %d: вернула тот же указатель", i)
		}
	}
}

func TestClampPhotoFade(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, -1: 0, 0.333: 0.33, 0.5: 0.5, 1: 1, 7: 1, 1e308: 1} {
		if got := ClampPhotoFade(in); got != want || ClampPhotoFade(got) != got {
			t.Errorf("%v → %v, ждали %v", in, got, want)
		}
	}
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if ClampPhotoFade(v) != 0 {
			t.Errorf("%v → %v", v, ClampPhotoFade(v))
		}
	}
	// Ноль не пишется (omitempty): прежние файлы и снимки байт в байт.
	data, _ := json.Marshal(PhotoRef{Path: "a"})
	if bytes.Contains(data, []byte(`"fade"`)) {
		t.Fatalf("нулевая прозрачность в JSON: %s", data)
	}
}

// Clone: nil — nil, цвета не общие; ClonePlace даёт свою рамочку.
func TestBorderClone(t *testing.T) {
	if (*Border)(nil).Clone() != nil {
		t.Fatal("nil.Clone")
	}
	b := &Border{Kind: "mat", Inks: []string{"#111111", "#222222"}, Width: 3}
	c := b.Clone()
	c.Inks[0] = "#ffffff"
	if b.Inks[0] != "#111111" || !reflect.DeepEqual(b.Clone(), b) {
		t.Fatalf("Clone поделил цвета: %+v", b)
	}
	pl := &Place{ID: "a", W: 10, H: 10, Border: b}
	cp := ClonePlace(pl, map[string]bool{})
	if cp.Border == b || !reflect.DeepEqual(cp.Border, b) || &cp.Border.Inks[0] == &b.Inks[0] {
		t.Fatalf("ClonePlace: %+v", cp.Border)
	}
	if ClonePlace(&Place{ID: "b"}, map[string]bool{}).Border != nil {
		t.Fatal("рамочка у копии места без рамочки")
	}
}

// BorderSrc.Border — встроенная (цвета — своя копия), картинка (ID записи, срез, Open), пусто — nil.
func TestBorderSrc(t *testing.T) {
	inks := []string{"#111111"}
	b := BorderSrc{Kind: "line", Inks: inks, Width: 2, Preset: "sa:fr-white-line", Cut: [4]float64{0.1}, Open: true}.Border()
	if !reflect.DeepEqual(b, &Border{Kind: "line", Inks: []string{"#111111"}, Width: 2, Preset: "sa:fr-white-line"}) || &b.Inks[0] == &inks[0] {
		t.Fatalf("встроенная: %+v", b)
	}
	cut := [4]float64{0.1, 0.2, 0.3, 0.4}
	if p := (BorderSrc{Photo: PhotoInfo{ID: "pic", Lib: "my:0123456789abcdef"}, Cut: cut, Open: true}).Border(); !reflect.DeepEqual(p, &Border{Photo: "pic", Cut: cut, Open: true}) {
		t.Fatalf("картинка: %+v", p)
	}
	if (BorderSrc{}).Border() != nil {
		t.Fatal("пустой источник — не nil")
	}
}

// TestBorderWindow — числа design §12: линия 2 мм у 100×150 — окно (2, 2, 96, 146); полароид 5 —
// низ 17,5; овал 80×100 с линией 2 — овал 76×96 (у овала полароид ровный); скругление 10 % + линия 3 —
// 7 мм; линия 20 у 40×40 — пара 28 мм; толщина 0 — толщина вида; накладка и неизвестный — место.
func TestBorderWindow(t *testing.T) {
	p := borderP()
	pl := func(w, h float64, b *Border) *Place { return &Place{X: 10, Y: 20, W: w, H: h, Border: b} }
	cases := []struct {
		pl  *Place
		win layout.Rect
		wo  geom.Outline
	}{
		{pl(100, 150, &Border{Kind: "line", Width: 2}), layout.Rect{X: 12, Y: 22, W: 96, H: 146}, geom.Outline{}},
		{pl(100, 150, &Border{Kind: "polaroid", Width: 5}), layout.Rect{X: 15, Y: 25, W: 90, H: 127.5}, geom.Outline{}},
		{pl(100, 150, &Border{Kind: "polaroid"}), layout.Rect{X: 15, Y: 25, W: 90, H: 127.5}, geom.Outline{}}, // 0 — 5 мм вида
		{pl(100, 150, &Border{Kind: "line"}), layout.Rect{X: 11.5, Y: 21.5, W: 97, H: 147}, geom.Outline{}},
		{&Place{X: 10, Y: 20, W: 80, H: 100, Shape: ShapeOval, Border: &Border{Kind: "line", Width: 2}},
			layout.Rect{X: 12, Y: 22, W: 76, H: 96}, geom.Outline{Oval: true}},
		{&Place{X: 10, Y: 20, W: 80, H: 100, Shape: ShapeOval, Border: &Border{Kind: "polaroid", Width: 2}},
			layout.Rect{X: 17, Y: 27, W: 66, H: 86}, geom.Outline{Oval: true}}, // 3,5 × 2 со всех сторон
		{&Place{X: 10, Y: 20, W: 100, H: 100, Round: 10, Border: &Border{Kind: "line", Width: 3}},
			layout.Rect{X: 13, Y: 23, W: 94, H: 94}, geom.Outline{Radius: 7}},
		{pl(40, 40, &Border{Kind: "line", Width: 20}), layout.Rect{X: 24, Y: 34, W: 12, H: 12}, geom.Outline{}},
		{pl(100, 150, &Border{Kind: "stitch", Width: 2}), layout.Rect{X: 10, Y: 20, W: 100, H: 150}, geom.Outline{}},
		{pl(100, 150, &Border{Kind: "soft"}), layout.Rect{X: 10, Y: 20, W: 100, H: 150}, geom.Outline{}},
		{pl(100, 150, &Border{Kind: "future-kind", Width: 5}), layout.Rect{X: 10, Y: 20, W: 100, H: 150}, geom.Outline{}},
		{pl(100, 150, nil), layout.Rect{X: 10, Y: 20, W: 100, H: 150}, geom.Outline{}},
	}
	for i, c := range cases {
		win, wo := p.BorderWindow(c.pl)
		if !nearRect(win, c.win) || math.Abs(wo.Radius-c.wo.Radius) > 1e-9 || wo.Oval != c.wo.Oval {
			t.Errorf("случай %d: окно %+v %+v, ждали %+v %+v", i, win, wo, c.win, c.wo)
		}
	}
}

func nearRect(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

// TestBorderWindowPic — картинка 3000×4000, срез 0,14 со всех сторон, место 30×40 → k = 0,01 мм/px,
// отступы 4,2 / 5,6; размер 2 — вдвое; Open — окно = место; другая пропорция места — k по меньшей.
func TestBorderWindowPic(t *testing.T) {
	p := borderP()
	cut := [4]float64{0.14, 0.14, 0.14, 0.14}
	cases := []struct {
		pl   *Place
		want [4]float64
	}{
		{&Place{W: 30, H: 40, Border: &Border{Photo: "pic", Cut: cut}}, [4]float64{4.2, 5.6, 4.2, 5.6}},
		{&Place{W: 30, H: 40, Border: &Border{Photo: "pic", Cut: cut, Scale: 0.5}}, [4]float64{2.1, 2.8, 2.1, 2.8}},
		{&Place{W: 60, H: 40, Border: &Border{Photo: "pic", Cut: cut}}, [4]float64{4.2, 5.6, 4.2, 5.6}}, // k = min(0,02; 0,01)
		{&Place{W: 30, H: 40, Border: &Border{Photo: "pic", Cut: cut, Open: true}}, [4]float64{}},
		{&Place{W: 30, H: 40, Border: &Border{Photo: "pic", Cut: [4]float64{0, 0, 0, 0.45}, Scale: 2}}, [4]float64{0, 0, 0, 28}}, // 36 > 0,7·40
	}
	for i, c := range cases {
		got := p.BorderIns(c.pl)
		for k := range got {
			if math.Abs(got[k]-c.want[k]) > 1e-9 {
				t.Errorf("случай %d: отступы %v, ждали %v", i, got, c.want)
				break
			}
		}
	}
}

// TestBorderWindowNoDims — 079-24: у записи без размеров (нет файла, чужой файл) окно = место,
// без NaN: View сериализуется.
func TestBorderWindowNoDims(t *testing.T) {
	p := borderP()
	pl := &Place{X: 5, Y: 5, W: 30, H: 40, Border: &Border{Photo: "nodims", Cut: [4]float64{0.2, 0.2, 0.2, 0.2}}}
	if ins := p.BorderIns(pl); ins != ([4]float64{}) {
		t.Fatalf("отступы без размеров: %v", ins)
	}
	win, _ := p.BorderWindow(pl)
	if win != (layout.Rect{X: 5, Y: 5, W: 30, H: 40}) {
		t.Fatalf("окно %+v", win)
	}
	if _, err := json.Marshal(BorderView{Ins: p.BorderIns(pl)}); err != nil {
		t.Fatal(err)
	}
}

// TestBorderPhotoRefs — запись рамочки (079 §1 «Обходы»): в PhotoIDs и OnDecor, не «в альбоме»;
// PruneRefs держит её и снимает свойства места (кадр, тон, прозрачность), а запись без дела
// уходит; RenameRefs переводит Border.Photo заменой указателя. Несколько мест и обложка.
func TestBorderPhotoRefs(t *testing.T) {
	p := borderP()
	p.Photos["a"] = PhotoRef{Path: "C:/a.jpg", W: 3, H: 2, Fade: 0.5}
	p.Photos["pic"] = PhotoRef{Lib: "my:0123456789abcdef", W: 3000, H: 4000, Frame: &Frame{S: 2}, Adj: &Tone{Gray: true}, Fade: 0.3}
	p.Photos["gone"] = PhotoRef{Path: "C:/gone.png"}
	shared := &Border{Photo: "pic"}
	s := &Spread{ID: "s", Photos: []string{"a"}, Grid: []*Place{
		{ID: "000000000001", W: 10, H: 10, Photo: "a", Border: &Border{Kind: "line"}},
		{ID: "000000000002", W: 10, H: 10, Border: shared},
		{ID: "000000000003", W: 10, H: 10, Border: &Border{Photo: "nodims"}},
	}}
	p.Spreads = []*Spread{s}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID, Grid: []*Place{{ID: "000000000004", W: 10, H: 10, Border: &Border{Photo: "pic"}}}}}
	if ids := s.PhotoIDs(); !slices.Equal(ids, []string{"a", "pic", "nodims"}) {
		t.Fatalf("PhotoIDs %v", ids)
	}
	if on := p.OnDecor(); !on["pic"] || !on["nodims"] || on["a"] || len(on) != 2 {
		t.Fatalf("OnDecor %v", on)
	}
	if p.Placed("pic") {
		t.Fatal("рамочка «в альбоме»")
	}
	p.PruneRefs()
	if _, ok := p.Photos["gone"]; ok {
		t.Fatal("запись без дела осталась")
	}
	r := p.Photos["pic"]
	if r.Frame != nil || r.Adj != nil || r.Fade != 0 || r.Lib == "" || p.Photos["a"].Fade != 0.5 {
		t.Fatalf("PruneRefs: рамочка %+v, фото места %+v", r, p.Photos["a"])
	}
	p.RenameRefs(map[string]string{"pic": "pic2"})
	if s.Grid[1].Border == shared || s.Grid[1].Border.Photo != "pic2" || shared.Photo != "pic" || p.Cover.Sheet.Grid[0].Border.Photo != "pic2" {
		t.Fatalf("RenameRefs: %+v %+v", s.Grid[1].Border, p.Cover.Sheet.Grid[0].Border)
	}
	if s.Grid[2].Border.Photo != "nodims" || s.Grid[0].Border.Kind != "line" {
		t.Fatal("RenameRefs тронул чужое")
	}
}

// Box.Border только в памяти: templates.json прежний байт в байт и той же версии (079-10).
func TestTemplatesNoBorder(t *testing.T) {
	boxes := func(b *Border, shape string) []Box {
		return []Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.4, Shape: shape, Border: b}, {X: 0.5, Y: 0.1, W: 0.3, H: 0.4, Border: b.Clone()}}
	}
	for _, shape := range []string{"", ShapeOval} {
		plain := []Template{{ID: "t1", Name: "Шаблон", Scope: TplSpread, Boxes: boxes(nil, shape)}}
		framed := []Template{{ID: "t1", Name: "Шаблон", Scope: TplSpread, Boxes: boxes(&Border{Kind: "mat", Inks: []string{"#111111"}, Width: 8}, shape)}}
		a, err := MarshalTemplates(plain)
		if err != nil {
			t.Fatal(err)
		}
		b, err := MarshalTemplates(framed)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) || bytes.Contains(b, []byte("border")) || bytes.Contains(b, []byte("mat")) {
			t.Fatalf("рамочка попала в templates.json:\n%s\n%s", a, b)
		}
		list, ver, err := ParseTemplates(b)
		if err != nil || len(list) != 1 || list[0].Boxes[0].Border != nil || ver > 5 {
			t.Fatalf("чтение: %v, версия %d, %+v", err, ver, list)
		}
	}
}
