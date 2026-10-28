package view_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Подпись «на фото» в рамочке (079 §2): во вписанном прямоугольнике формы окна, а не места;
// «под фото» — под рамкой места, как без рамочки; заглушка пустого места с рамочкой — так же.
func TestCaptionOverBorder(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(130, 10, 100, 100, "b"), edtest.Gp(10, 150, 100, 60, "c")})
	st := model.DefaultCaptionStyle()
	for _, id := range []string{"a", "b", "c"} {
		e.SetCaption(id, "Анна", "")
	}
	under, _ := json.Marshal(capOfPhoto(e.View().Spreads[0], "c"))
	for _, id := range []string{"a", "b"} {
		e.SetCaptionStyle(id, st, true, 1)
	}
	e.SetPlaceShape("s1", []string{"b"}, model.ShapeOval)
	if _, err := e.ApplyBorder("s1", []string{"a", "c"}, model.BorderSrc{Kind: "line", Width: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ApplyBorder("s1", []string{"b"}, model.BorderSrc{Kind: "polaroid", Width: 2}); err != nil {
		t.Fatal(err)
	}
	sv := e.View().Spreads[0]
	// Линия 2 мм: окно (12, 12, 96, 96); низ подписи — низ окна − pad 1 − 2.
	if a := capOfPhoto(sv, "a"); a == nil || math.Abs(a.X-12) > 1e-9 || math.Abs(a.W-96) > 1e-9 || math.Abs(a.Y+a.H-(108-1-2)) > 1e-6 {
		t.Fatalf("линия: %+v", a)
	}
	// Полароид 2 мм у круга: отступы 7 со всех сторон, окно — круг 86, вписанный квадрат.
	d := 86 * (1 - math.Sqrt2/2) / 2
	if b := capOfPhoto(sv, "b"); b == nil || math.Abs(b.X-(137+d)) > 1e-9 || math.Abs(b.W-86/math.Sqrt2) > 1e-9 || math.Abs(b.Y+b.H-(103-d-1-2)) > 1e-6 {
		t.Fatalf("полароид у круга: %+v", b)
	}
	if got, _ := json.Marshal(capOfPhoto(sv, "c")); string(got) != string(under) || len(under) < 10 {
		t.Fatalf("под фото с рамочкой:\n%s\n%s", got, under)
	}
	// Заглушка пустого места с рамочкой — в окне.
	e.SetSheetCaps("s1", true, model.SheetCaps{Style: st, Over: true, Pad: 1, Text: "{Фамилия}"})
	if _, err := e.AddPlace("s1", 1, ""); err != nil {
		t.Fatal(err)
	}
	g := edtest.P(e).Spreads[0].Grid
	empty := g[len(g)-1]
	if _, err := e.ApplyBorder("s1", []string{empty.ID}, model.BorderSrc{Kind: "line", Width: 3}); err != nil {
		t.Fatal(err)
	}
	pl := edtest.P(e).Spreads[0].Grid[len(g)-1]
	n := 0
	for _, s := range e.View().Spreads[0].Stubs {
		if s.Place != pl.ID {
			continue
		}
		n++
		if math.Abs(s.X-(pl.X+3)) > 1e-9 || math.Abs(s.W-(pl.W-6)) > 1e-9 || s.Y+s.H > pl.Y+pl.H-3+1e-9 {
			t.Fatalf("заглушка не в окне: %+v, место %+v", s.TextView, *pl)
		}
	}
	if n != 1 {
		t.Fatalf("заглушек места с рамочкой: %d", n)
	}
}
