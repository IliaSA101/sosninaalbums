package view_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Форма места в виде (спека 044, design §6).

// placeOf — вид места по ID.
func placeOf(sv model.SpreadView, id string) model.PlaceView {
	for _, pv := range sv.Places {
		if pv.ID == id {
			return pv
		}
	}
	return model.PlaceView{}
}

// Radius = round × меньшая сторона / 100 (мм считает Go); форма — и у пустого места, и у
// кадра; у «Авто» форм нет.
func TestViewShapeFields(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 60, "a"), edtest.Gp(130, 10, 80, 80, "b"), edtest.Gp(10, 150, 50, 40, ""), edtest.Gp(130, 150, 50, 50, "")})
	g := edtest.P(e).Spreads[0].Grid
	e.SetPlaceRound("s1", []string{g[0].ID, g[2].ID}, 10)
	v := e.SetPlaceShape("s1", []string{g[1].ID, g[3].ID}, model.ShapeOval)
	sv := v.Spreads[0]
	for _, c := range []struct {
		i      int
		shape  string
		round  float64
		radius float64
	}{{0, "", 10, 6}, {1, model.ShapeOval, 0, 0}, {2, "", 10, 4}, {3, model.ShapeOval, 0, 0}} {
		pv := placeOf(sv, g[c.i].ID)
		if pv.Shape != c.shape || pv.Round != c.round || math.Abs(pv.Radius-c.radius) > 1e-12 {
			t.Errorf("место %d: %q %v %v, ждали %q %v %v", c.i, pv.Shape, pv.Round, pv.Radius, c.shape, c.round, c.radius)
		}
	}
	if f := sv.Frames; len(f) != 2 || f[0].Shape != "" || math.Abs(f[0].Radius-6) > 1e-12 || f[1].Shape != model.ShapeOval || f[1].Radius != 0 {
		t.Fatalf("кадры: %+v", f)
	}
	// Место растёт — радиус с ним (проценты, а не мм).
	v = e.SetPlaces("s1", []model.PlaceBox{{ID: g[0].ID, X: 10, Y: 10, W: 200, H: 120}}, "")
	if pv := placeOf(v.Spreads[0], g[0].ID); pv.Round != 10 || math.Abs(pv.Radius-12) > 1e-12 {
		t.Fatalf("после растяжения: %+v", pv)
	}
	a, _ := edtest.AutoEd(t)
	for _, f := range a.View().Spreads[0].Frames {
		if f.Shape != "" || f.Radius != 0 {
			t.Fatalf("«Авто»: %+v", f)
		}
	}
}

// Подпись «на фото» у круга — во вписанном прямоугольнике: по ширине W/√2 и над низом
// вписанного, а не рамки; у скругления — рамка, сжатая на ρ(1 − 1/√2).
func TestCaptionOverOval(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(130, 10, 100, 60, "b")})
	g := edtest.P(e).Spreads[0].Grid
	st := model.DefaultCaptionStyle()
	for _, id := range []string{"a", "b"} {
		e.SetCaption(id, "Анна", "")
		e.SetCaptionStyle(id, st, true, 1)
	}
	rect := capOfPhoto(e.View().Spreads[0], "a")
	e.SetPlaceShape("s1", []string{"a"}, model.ShapeOval)
	sv := e.SetPlaceRound("s1", []string{"b"}, 50).Spreads[0]
	cv := capOfPhoto(sv, "a")
	d := 100 * (1 - math.Sqrt2/2) / 2 // отступ вписанного квадрата круга 100
	if rect == nil || cv == nil || rect.X != 10 || rect.W != 100 {
		t.Fatalf("прямоугольник: %+v", rect)
	}
	if math.Abs(cv.X-(10+d)) > 1e-9 || math.Abs(cv.W-100/math.Sqrt2) > 1e-9 || math.Abs(cv.Y+cv.H-(110-d-1-2)) > 1e-6 {
		t.Fatalf("круг: x %.3f w %.3f низ %.3f", cv.X, cv.W, cv.Y+cv.H)
	}
	// Скругление 50 % у 100×60: ρ = 30, сжатие 30·(1 − 1/√2) с каждой стороны.
	k := 30 * (1 - math.Sqrt2/2)
	if cb := capOfPhoto(sv, "b"); cb == nil || math.Abs(cb.X-(130+k)) > 1e-9 || math.Abs(cb.W-(100-2*k)) > 1e-9 || math.Abs(cb.Y+cb.H-(70-k-3)) > 1e-6 {
		t.Fatalf("скругление: %+v", cb)
	}
	// Заглушки пустых мест — так же.
	e.SetSheetCaps("s1", true, model.SheetCaps{Style: st, Over: true, Pad: 1, Text: "{Фамилия}"})
	e.ArrangePlaces("s1", "splitV", []string{g[0].ID})
	stubs := 0
	for _, s := range e.View().Spreads[0].Stubs {
		if s.Place == "" {
			continue
		}
		if stubs++; s.W > 100/math.Sqrt2+1e-9 {
			t.Fatalf("заглушка шире вписанного: %+v", s)
		}
	}
	if stubs != 1 {
		t.Fatalf("заглушек пустых мест: %d", stubs)
	}
}

// Подпись «под фото» у круга та же, что у прямоугольника: низ круга — низ рамки.
func TestCaptionUnderOvalSame(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(10, 120, 100, 100, "b")})
	e.SetCaption("a", "Анна Иванова", "")
	want, _ := json.Marshal(capOfPhoto(e.View().Spreads[0], "a"))
	e.SetPlaceShape("s1", []string{"a", "b"}, model.ShapeOval)
	got, _ := json.Marshal(capOfPhoto(e.View().Spreads[0], "a"))
	if string(got) != string(want) || len(want) < 10 {
		t.Fatalf("под фото:\n%s\n%s", got, want)
	}
	e.SetPlaceShape("s1", []string{"a", "b"}, "")
	e.SetPlaceRound("s1", []string{"a", "b"}, 30)
	if got, _ = json.Marshal(capOfPhoto(e.View().Spreads[0], "a")); string(got) != string(want) {
		t.Fatalf("под фото со скруглением:\n%s\n%s", got, want)
	}
}

// dpi круга с наклоном 30° равен dpi без наклона: фото в круге не увеличивается; у
// прямоугольника — меньше.
func TestFrameDPIOvalTilt(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")})
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos["a"]
		r.W, r.H = 6000, 4000
		p.Photos["a"] = r
	})
	flat := e.View().Spreads[0].Frames[0]
	tilt := edtest.Tilt(e, "a", 30).Spreads[0].Frames[0]
	oval := e.SetPlaceShape("s1", []string{"a"}, model.ShapeOval).Spreads[0].Frames[0]
	if flat.DPI == 0 || tilt.DPI >= flat.DPI || oval.DPI != flat.DPI || oval.Img.W != flat.Img.W {
		t.Fatalf("dpi: без наклона %d, прямоугольник 30° %d, круг 30° %d", flat.DPI, tilt.DPI, oval.DPI)
	}
	// «Целиком» в круге — фото меньше рамки (вписано в круг), dpi выше.
	if c := e.SetPlaceFit("s1", []string{"a"}, model.FitContain).Spreads[0].Frames[0]; c.DPI <= flat.DPI {
		t.Fatalf("«Целиком» в круге: %+v", c)
	}
}
