package edit

import (
	"math"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Методы над сеткой: места, слои, выравнивание, зеркало, копии — вид меняется как задумано.
func TestGridSurface(t *testing.T) {
	a := gridMethods(t)
	const pa, pb = "00000000000a", "00000000000b"
	rev := func() int { return a.editor().View().Rev }
	places := func(v model.View) []model.PlaceView { return v.Spreads[0].Places }
	grew := func(name string, before int, v model.View) {
		t.Helper()
		if v.Rev <= before {
			t.Errorf("%s: правка не применена (rev %d → %d)", name, before, v.Rev)
		}
	}

	r := rev()
	v, err := a.AddPlace("s1", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	grew("AddPlace", r, v)
	if len(places(v)) != 3 {
		t.Fatalf("мест после AddPlace: %d", len(places(v)))
	}
	newID := places(v)[2].ID

	r = rev()
	v = a.SetPlaces("s1", []model.PlaceBox{{ID: pb, X: 300, Y: 20, W: 90, H: 80}}, "")
	grew("SetPlaces", r, v)
	if p := places(v)[1]; p.X != 300 || p.W != 90 || p.H != 80 {
		t.Errorf("место: %+v", p)
	}
	r = rev()
	v = a.SetPlaceAngle("s1", pb, 20)
	grew("SetPlaceAngle", r, v)
	if places(v)[1].Rot != 20 {
		t.Errorf("угол места: %v", places(v)[1].Rot)
	}
	r = rev()
	v = a.SetPlaceFit("s1", []string{pa}, "contain")
	grew("SetPlaceFit", r, v)
	if places(v)[0].Fit != "contain" {
		t.Errorf("fit: %q", places(v)[0].Fit)
	}
	r = rev()
	v = a.ArrangeLayers("s1", "front", []string{pa})
	grew("ArrangeLayers", r, v)

	// Выравнивание и распределение: два места, op влево.
	r = rev()
	v = a.AlignItems("s1", []string{pa, pb}, "left", "sel")
	grew("AlignItems", r, v)
	// место b повёрнуто: выравнивается его описанная рамка, поэтому сравниваем сдвиг, а не X.
	if places(v)[1].X >= 300 {
		t.Errorf("выравнивание влево не сдвинуло место: X %v", places(v)[1].X)
	}
	// Два места по всему развороту помещаются без наложения, три в рамке выделения — нет.
	if a.AlignOverlap("s1", []string{pa, pb}, "distH", "spread") {
		t.Error("AlignOverlap: два места по развороту не должны перекрываться")
	}
	if !a.AlignOverlap("s1", []string{pa, pb, newID}, "distH", "sel") {
		t.Error("AlignOverlap: три места в рамке выделения должны перекрываться")
	}

	// Группа: перенос, поворот, масштаб по осям.
	r = rev()
	v = a.TransformGroup("s1", []string{pa, pb}, 1, 0, 0, 5, 0, "")
	grew("TransformGroup", r, v)
	r = rev()
	v = a.RotateGroup("s1", []string{pa, pb}, 5, 210, 148)
	grew("RotateGroup", r, v)
	r = rev()
	v = a.ScaleGroupAxes("s1", []string{pa, pb}, 0.9, 0.9, 0, 210, 148)
	grew("ScaleGroupAxes", r, v)

	// Зеркало сетки и закрепление «Авто».
	before := places(a.editor().View())[1] // newID: X 110, W 100, без поворота
	if before.ID != newID {
		t.Fatalf("порядок мест изменился: %+v", before)
	}
	r = rev()
	v = a.MirrorGrid("s1", "spread")
	grew("MirrorGrid", r, v)
	// Отражение по ширине разворота (600 мм): X' = 600 − X − W.
	if p := places(v)[1]; math.Abs(p.X-(600-before.X-before.W)) > 0.01 {
		t.Errorf("MirrorGrid: X %v, ждали %v", p.X, 600-before.X-before.W)
	}
	r = rev()
	v = a.SetGrid("s2", true)
	grew("SetGrid", r, v)
	if !v.Spreads[1].Grid {
		t.Error("разворот s2 не закреплён")
	}

	// Дубликат: новый разворот с теми же местами без фото.
	r = rev()
	v, err = a.DuplicateGrid("s1", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	grew("DuplicateGrid", r, v)
	if len(v.Spreads) != 3 {
		t.Errorf("разворотов после дубликата: %d", len(v.Spreads))
	}

	// Удаление мест и выделения: пустое место уходит, фото остаётся в ленте разворота.
	r = rev()
	v = a.DeletePlaces("s1", []string{newID})
	grew("DeletePlaces", r, v)
	if slices.ContainsFunc(places(v), func(p model.PlaceView) bool { return p.ID == newID }) {
		t.Error("место не удалено")
	}
	r = rev()
	v = a.DeleteSelection("s1", nil, []string{pb}, nil)
	grew("DeleteSelection", r, v)
	if slices.ContainsFunc(places(v), func(p model.PlaceView) bool { return p.ID == pb }) {
		t.Error("выделенное место не удалено")
	}
}

// «Выпрямить по линии» и наклон фона (спека 045): правка с шагом, холостая без rev.
func TestStraightenSurface(t *testing.T) {
	a := gridMethods(t)
	rev := func() int { return a.editor().View().Rev }
	r := rev()
	v := a.StraightenPhoto("s2", "b", 100, 10) // завал 5,7° → довернуть на −5,7
	if f := v.Spreads[1].Frames[0]; f.Tilt != -5.7 || v.Rev != r+1 {
		t.Fatalf("StraightenPhoto: наклон %v, rev %d → %d", f.Tilt, r, v.Rev)
	}
	r = rev()
	for name, v := range map[string]model.View{
		"нулевая":    a.StraightenPhoto("s2", "b", 0, 0),
		"NaN":        a.StraightenPhoto("s2", "b", math.NaN(), 1),
		"чужое фото": a.StraightenPhoto("s2", "a", 100, 10),
		"фон нет":    a.StraightenBackdrop("s2", "right", 100, 10),
		"наклон NaN": a.SetBackdropAngle("s2", "right", math.NaN(), ""),
	} {
		if v.Rev != r {
			t.Errorf("%s: холостая правка подняла rev %d → %d", name, r, v.Rev)
		}
	}
	if _, err := a.SetBackdrop("s2", "right", "a"); err != nil {
		t.Fatal(err)
	}
	v = a.SetBackdropAngle("s2", "right", 4.5, "")
	if b := v.Spreads[1].Backdrops[0]; b.Frame.R != 4.5 || b.DPI == 0 {
		t.Fatalf("SetBackdropAngle: %+v", b.Frame)
	}
	v = a.StraightenBackdrop("s2", "right", 100, 10)
	if b := v.Spreads[1].Backdrops[0]; b.Frame.R != -1.2 {
		t.Fatalf("StraightenBackdrop: R=%v, ждали −1,2", b.Frame.R)
	}
	if v = a.undo(); v.Spreads[1].Backdrops[0].Frame.R != 4.5 {
		t.Fatalf("отмена выпрямления фона: R=%v", v.Spreads[1].Backdrops[0].Frame.R)
	}
}

// Форма места (спека 044): два места
// одним шагом, скругление с зажимом, холостые без rev, отмена.
func TestPlaceShapeSurface(t *testing.T) {
	a := gridMethods(t)
	ids, r := []string{"00000000000a", "00000000000b"}, a.editor().View().Rev
	v := a.SetPlaceShape("s1", ids, model.ShapeOval)
	if p := v.Spreads[0].Places; v.Rev != r+1 || p[0].Shape != model.ShapeOval || p[1].Shape != model.ShapeOval {
		t.Fatalf("SetPlaceShape: rev %d → %d, %+v", r, v.Rev, p)
	}
	a.SetPlaceShape("s1", ids[1:], "")
	v = a.SetPlaceRound("s1", ids, 70) // у круга холостая, прямоугольнику — 50
	if p := v.Spreads[0].Places; v.Rev != r+3 || p[0].Round != 0 || p[1].Shape != "" || p[1].Round != 50 {
		t.Fatalf("SetPlaceRound: rev %d → %d, %+v", r, v.Rev, p)
	}
	r = v.Rev
	for name, v := range map[string]model.View{
		"та же форма": a.SetPlaceShape("s1", ids[:1], model.ShapeOval),
		"неизвестная": a.SetPlaceShape("s1", ids, "star"),
		"NaN":         a.SetPlaceRound("s1", ids, math.NaN()),
		"нет листа":   a.SetPlaceRound("s9", ids, 10),
	} {
		if v.Rev != r {
			t.Errorf("%s: холостая правка подняла rev %d → %d", name, r, v.Rev)
		}
	}
	if v = a.undo(); v.Spreads[0].Places[1].Round != 0 || v.Spreads[0].Places[0].Shape != model.ShapeOval {
		t.Fatalf("отмена: %+v", v.Spreads[0].Places)
	}
}
