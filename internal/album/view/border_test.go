package view_test

import (
	"encoding/json"
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Рамочки мест и прозрачность фото в виде (079, design §7).

func nearR(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

// TestBorderImgInWindow — фото садится в окно рамочки: Win = место − отступы, Img — кадр в окне
// (cover и «Целиком»), dpi по Img, форма окна у овала и скругления; без рамочки Win = место и Img
// прежний. Вид рамочки: цвета вида дополнены, толщина 0 — вида, картинка — Pic и Lib, неизвестный
// вид — Known false и окно = место; пустое место с рамочкой — в Places, кадра нет. Прозрачность фото
// — в кадре и у «Авто».
func TestBorderImgInWindow(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(10, 10, 100, 150, "a"), edtest.Gp(130, 10, 80, 100, "b"), edtest.Gp(10, 170, 100, 100, "c"),
		edtest.Gp(130, 170, 60, 80, ""), edtest.Gp(220, 10, 50, 50, "d"), edtest.Gp(220, 70, 50, 50, "e"),
	}, []string{"f"})
	before := e.View().Spreads[0]
	fr := model.LibPhotoID("my:0123456789abcdef")
	v := edtest.Change(e, func(p *model.Project) {
		p.Photos[fr] = model.PhotoRef{Lib: "my:0123456789abcdef", W: 3000, H: 4000}
		g := p.Spreads[0].Grid
		g[0].Border = &model.Border{Kind: "line", Width: 2}
		g[1].Shape, g[1].Border = model.ShapeOval, &model.Border{Kind: "polaroid", Width: 2}
		g[2].Round, g[2].Fit, g[2].Border = 10, model.FitContain, &model.Border{Kind: "mat", Inks: []string{"#111111"}, Width: 3}
		g[3].Border = &model.Border{Photo: fr, Cut: [4]float64{0.14, 0.14, 0.14, 0.14}}
		g[4].Border = &model.Border{Kind: "future-kind", Inks: []string{"#222222"}, Width: 4}
		for _, id := range []string{"a", "f"} {
			r := p.Photos[id]
			r.Fade = 0.4
			p.Photos[id] = r
		}
	})
	sv := v.Spreads[0]
	if _, err := json.Marshal(v); err != nil {
		t.Fatal(err)
	}
	ids := edtest.P(e).Spreads[0].Grid
	frame := func(photo string) model.FrameView {
		i := slices.Index(sv.Photos, photo)
		return sv.Frames[i]
	}
	a, b, c, d, f := frame("a"), frame("b"), frame("c"), frame("d"), frame("e")
	if !nearR(a.Win, layout.Rect{X: 12, Y: 12, W: 96, H: 146}) || a.WinShape != "" ||
		!nearR(a.Img, model.PlaceImage(a.Win, 1.5, model.Frame{S: 1})) || a.DPI != geom.FrameDPI(300, a.Img) || a.Fade != 0.4 {
		t.Fatalf("линия: %+v", a)
	}
	if !nearR(b.Win, layout.Rect{X: 137, Y: 17, W: 66, H: 86}) || b.WinShape != model.ShapeOval || b.Shape != model.ShapeOval {
		t.Fatalf("овал: %+v", b)
	}
	if !nearR(c.Win, layout.Rect{X: 13, Y: 173, W: 94, H: 94}) || math.Abs(c.WinRadius-7) > 1e-9 || c.Radius != 10 ||
		!nearR(c.Img, geom.ContainImageIn(c.Win, 1.5, 0, geom.Outline{Radius: 7})) {
		t.Fatalf("скругление и «Целиком»: %+v", c)
	}
	if !nearR(d.Win, layout.Rect{X: 220, Y: 10, W: 50, H: 50}) || !nearR(d.Img, before.Frames[3].Img) {
		t.Fatalf("неизвестный вид: окно %+v, Img %+v (было %+v)", d.Win, d.Img, before.Frames[3].Img)
	}
	if f != before.Frames[4] || f.Fade != 0 || !nearR(f.Win, layout.Rect{X: 220, Y: 70, W: 50, H: 50}) {
		t.Fatalf("без рамочки кадр изменился: %+v, было %+v", f, before.Frames[4])
	}
	if auto := v.Spreads[1].Frames[0]; auto.Fade != 0.4 || !nearR(auto.Win, v.Spreads[1].Rects[0]) {
		t.Fatalf("«Авто»: %+v", auto)
	}

	pv := func(i int) *model.BorderView { return placeOf(sv, ids[i].ID).Border }
	if m := pv(2); m == nil || !m.Known || m.Name != "Паспарту" || !slices.Equal(m.Inks, []string{"#111111", "#c8a24a"}) || m.Width != 3 || m.Pic ||
		m.Ins != [4]float64{3, 3, 3, 3} {
		t.Fatalf("паспарту: %+v", m)
	}
	if l := pv(0); l.Width != 2 || !slices.Equal(l.Inks, []string{"#ffffff"}) || l.Name != "Линия" {
		t.Fatalf("линия: %+v", l)
	}
	if o := pv(3); o == nil || !o.Pic || !o.Known || o.Lib != "my:0123456789abcdef" || o.Scale != 1 || o.Photo != fr ||
		math.Abs(o.Ins[0]-8.4) > 1e-9 || math.Abs(o.Ins[1]-11.2) > 1e-9 {
		t.Fatalf("своя у пустого места: %+v", o)
	}
	if u := pv(4); u.Known || u.Name != "" || !slices.Equal(u.Inks, []string{"#222222"}) || u.Width != 4 || u.Ins != [4]float64{} {
		t.Fatalf("неизвестный вид: %+v", u)
	}
	if pv(5) != nil || len(sv.Frames) != 5 {
		t.Fatalf("без рамочки: %+v, кадров %d", pv(5), len(sv.Frames))
	}
	if !slices.Contains(v.BackdropUsed, fr) || !slices.Contains(v.LibPhotos, fr) {
		t.Fatalf("лента и свои: %v %v", v.BackdropUsed, v.LibPhotos)
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	// Толщина 0 — толщина вида (линия 1,5 мм): и в виде рамочки, и в окне.
	v = edtest.Change(e, func(p *model.Project) { p.Spreads[0].Grid[0].Border = &model.Border{Kind: "line"} })
	if l, a := placeOf(v.Spreads[0], ids[0].ID).Border, v.Spreads[0].Frames[0]; l.Width != 1.5 || !nearR(a.Win, layout.Rect{X: 11.5, Y: 11.5, W: 97, H: 147}) {
		t.Fatalf("толщина вида: %+v, окно %+v", l, a.Win)
	}
}

// TestBorderViewURL — адреса рамочки в виде (G6, design §5, 079-34, 079-36): встроенная — накладка
// /media/gen/frame под размер и форму места, без маски, толщина вида defWidth; мягкий край — только
// маска; картинка — /media/f/<запись> со срезом альбома, маска — тот же адрес с m=1; без окна (open)
// — маски нет; неизвестный вид — без адресов, defWidth и имени. px не пишется нигде (его дописывает UI).
func TestBorderViewURL(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(10, 10, 100, 150, "a"), edtest.Gp(130, 10, 80, 100, ""), edtest.Gp(10, 170, 60, 40, "b"),
		edtest.Gp(130, 170, 60, 80, "c"), edtest.Gp(220, 10, 50, 50, "d"),
	}, nil)
	fr := model.LibPhotoID("my:0123456789abcdef")
	v := edtest.Change(e, func(p *model.Project) {
		p.Photos[fr] = model.PhotoRef{Lib: "my:0123456789abcdef", W: 3000, H: 4000}
		g := p.Spreads[0].Grid
		g[0].Round, g[0].Border = 10, &model.Border{Kind: "line", Width: 2}
		g[1].Shape, g[1].Border = model.ShapeOval, &model.Border{Kind: "soft"}
		g[2].Border = &model.Border{Photo: fr, Scale: 1.5, Cut: [4]float64{0.14, 0.1, 0.14, 0.2}}
		g[3].Border = &model.Border{Photo: fr, Open: true}
		g[4].Border = &model.Border{Kind: "future-kind"}
	})
	ids := edtest.P(e).Spreads[0].Grid
	pv := func(i int) *model.BorderView { return placeOf(v.Spreads[0], ids[i].ID).Border }
	want := []struct{ url, mask string }{
		{"/media/gen/frame?k=line&i=ffffff&bw=2&w=100&h=150&rd=10&v=1", ""},
		{"", "/media/gen/frame?k=soft&bw=8&w=80&h=100&o=1&m=1&v=1"},
		{"/media/f/" + fr + "?w=60&h=40&s=1.5&c=0.14,0.1,0.14,0.2&v=1", "/media/f/" + fr + "?w=60&h=40&s=1.5&c=0.14,0.1,0.14,0.2&m=1&v=1"},
		{"/media/f/" + fr + "?w=60&h=80&c=0,0,0,0&o=1&v=1", ""},
		{"", ""},
	}
	defs := []float64{1.5, 8, 0, 0, 0}
	// Имя — только у встроенной известной: своей UI берёт имя из каталога, у вида новее — пусто (079-38).
	names := []string{"Линия", "Мягкий край", "", "", ""}
	for i, w := range want {
		b := pv(i)
		if b.URL != w.url || b.MaskURL != w.mask || b.DefWidth != defs[i] || b.Name != names[i] {
			t.Errorf("место %d: url %q mask %q defWidth %v имя %q\nждали %q %q %v %q", i, b.URL, b.MaskURL, b.DefWidth, b.Name, w.url, w.mask, defs[i], names[i])
		}
	}
}
