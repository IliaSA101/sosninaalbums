package edit

import (
	"encoding/json"
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
	// 076: "rounded" — 10 % у круга и прямоугольника одним шагом, повтор — тот же rev; во View
	// ключа нет; "" снимает скругление; «Место ▸ Скруглённое» — место с round 10.
	r = v.Rev
	v = a.SetPlaceShape("s1", ids, model.ShapeRounded)
	if p := v.Spreads[0].Places; v.Rev != r+1 || p[0].Shape != "" || p[0].Round != 10 || p[1].Shape != "" || p[1].Round != 10 {
		t.Fatalf("SetPlaceShape(rounded): rev %d → %d, %+v", r, v.Rev, p)
	}
	if v = a.SetPlaceShape("s1", ids, model.ShapeRounded); v.Rev != r+1 {
		t.Errorf("повтор rounded поднял rev %d → %d", r+1, v.Rev)
	}
	if v = a.SetPlaceShape("s1", ids, ""); v.Rev != r+2 || v.Spreads[0].Places[0].Round != 0 || v.Spreads[0].Places[1].Round != 0 {
		t.Fatalf("SetPlaceShape(\"\"): rev %d, %+v", v.Rev, v.Spreads[0].Places)
	}
	v, err := a.AddPlace("s1", 0, model.ShapeRounded)
	if p := v.Spreads[0].Places; err != nil || len(p) != 3 || p[2].Shape != "" || p[2].Round != 10 || p[2].Radius <= 0 {
		t.Fatalf("AddPlace(rounded): %v, %+v", err, p)
	}
}

// BorderPatch из JSON, как его шлёт UI через Wails (frameapi.ts): нет ключа — nil и 0 («не менять»),
// "inks": [] — пустой, но не nil список (цвета вида); правка «[]» возвращает цвета вида.
func TestBorderPatchJSON(t *testing.T) {
	var keep, reset, width model.BorderPatch
	for raw, dst := range map[string]*model.BorderPatch{`{}`: &keep, `{"inks":[]}`: &reset, `{"width":2.5}`: &width} {
		if err := json.Unmarshal([]byte(raw), dst); err != nil {
			t.Fatal(err)
		}
	}
	if keep.Inks != nil || keep.Width != 0 || reset.Inks == nil || len(reset.Inks) != 0 || width.Width != 2.5 || width.Inks != nil {
		t.Fatalf("разбор: {} %+v, inks:[] %+v, width %+v", keep, reset, width)
	}
	a := gridMethods(t)
	ids := []string{"00000000000a"}
	if _, err := a.editor().ApplyBorder("s1", ids, model.BorderSrc{Kind: "line", Inks: []string{"#123456"}}); err != nil {
		t.Fatal(err)
	}
	r := a.editor().View().Rev
	if v := a.SetBorder("s1", ids, keep, ""); v.Rev != r {
		t.Fatalf("{} — правка: rev %d → %d", r, v.Rev)
	}
	v := a.SetBorder("s1", ids, reset, "")
	if b := v.Spreads[0].Places[0].Border; v.Rev != r+1 || !slices.Equal(b.Inks, []string{"#ffffff"}) {
		t.Fatalf("inks:[] — цвета вида: rev %d → %d, %+v", r, v.Rev, b)
	}
}

// Рамочка и прозрачность (079 §8): серия толщины на два места — один шаг, лишний цвет у вида с
// одним цветом отброшен (079-36), NaN, та же толщина и пустой список — холостые; прозрачность —
// по ID фото (ID места — холостая), серия — один шаг с зажимом, отмена возвращает прежнее.
func TestBorderFadeSurface(t *testing.T) {
	a := gridMethods(t)
	ids := []string{"00000000000a", "00000000000b"}
	if _, err := a.editor().ApplyBorder("s1", ids, model.BorderSrc{Kind: "line"}); err != nil {
		t.Fatal(err)
	}
	r := a.editor().View().Rev
	a.SetBorder("s1", ids, model.BorderPatch{Width: 2}, "w1")
	v := a.SetBorder("s1", ids, model.BorderPatch{Width: 3, Inks: []string{"#111111", "#222222"}}, "w1")
	for _, pl := range v.Spreads[0].Places {
		if b := pl.Border; b == nil || b.Width != 3 || !slices.Equal(b.Inks, []string{"#111111"}) || b.DefWidth != 1.5 {
			t.Fatalf("SetBorder %s: %+v", pl.ID, b)
		}
	}
	if v.Rev != r+2 || !v.CanUndo {
		t.Fatalf("серия толщины: rev %d → %d", r, v.Rev)
	}
	r = v.Rev
	for name, v := range map[string]model.View{
		"NaN":         a.SetBorder("s1", ids, model.BorderPatch{Width: math.NaN()}, ""),
		"та же":       a.SetBorder("s1", ids, model.BorderPatch{Width: 3}, ""),
		"пустой":      a.SetBorder("s1", nil, model.BorderPatch{Width: 5}, ""),
		"нет листа":   a.SetBorder("s9", ids, model.BorderPatch{Width: 5}, ""),
		"fade NaN":    a.SetPhotoFade([]string{"a"}, math.NaN(), ""),
		"fade место":  a.SetPhotoFade(ids, 0.5, ""),
		"fade пустой": a.SetPhotoFade(nil, 0.5, ""),
	} {
		if v.Rev != r {
			t.Errorf("%s: холостая правка подняла rev %d → %d", name, r, v.Rev)
		}
	}
	a.SetPhotoFade([]string{"a", "b"}, 0.3, "f1")
	v = a.SetPhotoFade([]string{"a", "b"}, 1.7, "f1") // зажим до 1
	if fv := v.Spreads[0].Frames[0]; v.Rev != r+2 || fv.Fade != 1 || v.Spreads[1].Frames[0].Fade != 1 {
		t.Fatalf("SetPhotoFade: rev %d → %d, %+v", r, v.Rev, fv)
	}
	if v = a.undo(); v.Spreads[0].Frames[0].Fade != 0 || v.Spreads[1].Frames[0].Fade != 0 {
		t.Fatalf("отмена прозрачности: %+v", v.Spreads[0].Frames[0])
	}
	// Один Ctrl+Z на серию: толщина прежняя, следующий шаг — сама постановка рамочки.
	if v = a.undo(); v.Spreads[0].Places[0].Border.Width != 1.5 {
		t.Fatalf("отмена серии толщины: %+v", v.Spreads[0].Places[0].Border)
	}
	if v = a.undo(); v.Spreads[0].Places[0].Border != nil {
		t.Fatalf("вторая отмена — не постановка: %+v", v.Spreads[0].Places[0].Border)
	}
}
