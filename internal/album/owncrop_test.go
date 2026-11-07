package album_test

// Свой кадр фото (098 §2, design §3.2): «Вернуть авто» с галочкой, число N, «Под фото»,
// поворот, сброс, копии.

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Место Марины (спека 098 §2 п. 1): горизонтальное, фото 3:2 в нём обрезано.
const marinaW, marinaH = 136.5, 81.0

// fracOf — ячейка (окно места) фото id первого листа вида v и это окно в долях фото: сдвиг и
// размер относительно картинки Img. Без наклона видимая часть снимка — ровно эти четыре числа.
func fracOf(t *testing.T, v model.View, id string) (layout.Rect, [4]float64) {
	t.Helper()
	sv := v.Spreads[0]
	i := slices.Index(sv.Photos, id)
	if i < 0 || i >= len(sv.Frames) {
		t.Fatalf("фото %s нет в виде: %v", id, sv.Photos)
	}
	f := sv.Frames[i]
	return f.Win, [4]float64{(f.Win.X - f.Img.X) / f.Img.W, (f.Win.Y - f.Img.Y) / f.Img.H, f.Win.W / f.Img.W, f.Win.H / f.Img.H}
}

func near4(a, b [4]float64, eps float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > eps {
			return false
		}
	}
	return true
}

// aspOf — свой кадр записи id.
func aspOf(e *album.Editor, id string) float64 { return edtest.P(e).Photos[id].Asp }

// setAsp — свой кадр записям одной правкой (как после прошлого «Вернуть авто» с галочкой).
func setAsp(e *album.Editor, asp map[string]float64) {
	edtest.Change(e, func(p *model.Project) {
		for id, a := range asp {
			r := p.Photos[id]
			r.Asp = a
			p.Photos[id] = r
		}
	})
}

// marina — сценарий Марины: «Авто»-разворот (a 3:2, b 2:3, c 1:1), место фото a стало
// 136,5 × 81 мм (разворот закрепился), кадр S 1,3 со сдвигом. Ответ — редактор, лист и окно a
// на ручном месте в долях фото.
func marina(t *testing.T) (*album.Editor, string, [4]float64) {
	t.Helper()
	e, id := edtest.AutoEd(t)
	e.SetPlaces(id, []model.PlaceBox{{ID: "a", X: 20, Y: 20, W: marinaW, H: marinaH}}, "")
	edtest.MustGrid(t, e)
	cell, fr := fracOf(t, e.SetFrame("a", 1.3, 0.2, -0.1), "a")
	if math.Abs(cell.W-marinaW) > 1e-9 || math.Abs(cell.H-marinaH) > 1e-9 {
		t.Fatalf("место a: %+v", cell)
	}
	return e, id, fr
}

// TestBackToAutoKeepsCrop: «Вернуть авто» с галочкой — ячейка a в пропорциях места, видна та же
// часть снимка; ‹ › пять раз — кадр держится в каждом варианте. Один шаг отмены.
func TestBackToAutoKeepsCrop(t *testing.T) {
	e, id, manual := marina(t)
	if n := e.CropsToKeep(id); n != 1 {
		t.Fatalf("N = %d, ждали 1 (b и c в ячейках своих пропорций)", n)
	}
	undo := edtest.UndoLen(e)
	v, err := e.BackToAuto(id, "", true)
	if err != nil {
		t.Fatal(err)
	}
	asp := aspOf(e, "a")
	if want := model.ClampAsp(marinaW / marinaH); asp != want || v.Spreads[0].Grid || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("Asp %v (ждали %v), сетка %v, шагов %d", asp, want, v.Spreads[0].Grid, edtest.UndoLen(e)-undo)
	}
	if aspOf(e, "b") != 0 || aspOf(e, "c") != 0 {
		t.Fatalf("свой кадр у неправленых: b %v, c %v", aspOf(e, "b"), aspOf(e, "c"))
	}
	_, first := fracOf(t, v, "a")
	// Asp округлён до 1e-4 (ClampAsp): с ручным местом окно сходится до этой точности.
	if !near4(first, manual, 1e-4) {
		t.Fatalf("окно в «Авто» %v, на ручном месте %v", first, manual)
	}
	keys := map[string]bool{}
	for range 5 {
		v = e.StepLayout(id, "spread", 1)
		cell, fr := fracOf(t, v, "a")
		key := edtest.P(e).Spreads[0].Layout
		keys[key] = true
		if math.Abs(cell.W/cell.H/asp-1) > 1e-9 {
			t.Fatalf("%s: ячейка a %v × %v — не в пропорциях своего кадра %v", key, cell.W, cell.H, asp)
		}
		if !near4(fr, first, 1e-9) {
			t.Fatalf("%s: окно %v, после «Вернуть авто» %v", key, fr, first)
		}
	}
	if len(keys) != 5 {
		t.Fatalf("листание стояло на месте: %v", keys)
	}
	if f := edtest.P(e).Photos["a"].Frame; f == nil || f.S != 1.3 || aspOf(e, "a") != asp {
		t.Fatalf("листание тронуло кадр: %+v, Asp %v", f, aspOf(e, "a"))
	}
	for range 6 {
		e.Undo()
	}
	edtest.MustGrid(t, e)
	if aspOf(e, "a") != 0 {
		t.Fatalf("отмена «Вернуть авто» не сняла свой кадр: %v", aspOf(e, "a"))
	}
}

// TestOwnCropSaveLoad: свой кадр переживает запись и чтение — «Авто» после открытия показывает ту
// же часть снимка, у фото без своего кадра поля нет.
func TestOwnCropSaveLoad(t *testing.T) {
	e, id, _ := marina(t)
	v, err := e.BackToAuto(id, "", true)
	if err != nil {
		t.Fatal(err)
	}
	_, want := fracOf(t, v, "a")
	path := filepath.Join(testkit.TempDir(t), "own.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Чтение даёт записям ID по пути (Rekey): «a» тестового альбома становится ID файла.
	a, b := library.IDFor(edtest.Abs("a.jpg")), library.IDFor(edtest.Abs("b.jpg"))
	if p.Photos[a].Asp != aspOf(e, "a") || p.Photos[b].Asp != 0 || p.Photos[b].W == 0 {
		t.Fatalf("после чтения Asp a %v (было %v), b %v", p.Photos[a].Asp, aspOf(e, "a"), p.Photos[b].Asp)
	}
	if _, got := fracOf(t, edtest.Open(t, p, path).View(), a); !near4(got, want, 1e-9) {
		t.Fatalf("окно после чтения %v, до записи %v", got, want)
	}
}

// TestBackToAutoKeyRebuilt: ключ из галереи ручного разворота (посчитан без своего кадра)
// строится по новым пропорциям; не проходит сгиб — лучшая (decisions 098-19).
func TestBackToAutoKeyRebuilt(t *testing.T) {
	e, id, _ := marina(t)
	row := e.LayoutChoices(id).Whole.InOrder
	if len(row) < 3 {
		t.Fatalf("галерея: %d вариантов", len(row))
	}
	key := row[2].Key
	if _, err := e.BackToAuto(id, key, true); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	s := p.Spreads[0]
	cur, _, ok := p.Layout(s)
	if !ok || cur.Key != key || len(s.Grid) > 0 {
		t.Fatalf("раскладка %q, ждали %q (сетка %d)", cur.Key, key, len(s.Grid))
	}
	want, _ := layout.Build(key, p.Aspects(s.Photos), p.Params())
	r := cur.Rects[slices.Index(s.Photos, "a")]
	if !slices.Equal(cur.Rects, want.Rects) || math.Abs(r.W/r.H/aspOf(e, "a")-1) > 1e-9 {
		t.Fatalf("ключ построен не по своему кадру: ячейка a %v × %v, Asp %v", r.W, r.H, aspOf(e, "a"))
	}

	// Одно широкое фото через сгиб; место сузилось до 4 : 5 — тот же ключ через сгиб нельзя.
	e2 := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e2, -1, edtest.Photos("w")).Spreads[0].ID
	prm := edtest.P(e2).Params()
	cross := ""
	for _, c := range e2.LayoutChoices(sid).Whole.InOrder {
		if crossesSpine(c.Rects[0], prm) {
			cross = c.Key
			break
		}
	}
	if cross == "" {
		t.Fatal("нет раскладки через сгиб у широкого фото")
	}
	e2.SetPlaces(sid, []model.PlaceBox{{ID: "w", X: 50, Y: 50, W: 100, H: 125}}, "")
	edtest.MustGrid(t, e2)
	narrow := []float64{model.ClampAsp(0.8)}
	if b, ok := layout.Build(cross, narrow, prm); !ok || layout.FoldOK(b.Rects, narrow, prm) {
		t.Fatalf("ключ %q при 4 : 5 должен строиться и не проходить сгиб (ok %v)", cross, ok)
	}
	undo := edtest.UndoLen(e2)
	if _, err := e2.BackToAuto(sid, cross, true); err != nil {
		t.Fatal(err)
	}
	p2 := edtest.P(e2)
	cur, _, _ = p2.Layout(p2.Spreads[0])
	if aspOf(e2, "w") != 0.8 || len(p2.Spreads[0].Grid) > 0 || cur.Key == cross || crossesSpine(cur.Rects[0], prm) || edtest.UndoLen(e2) != undo+1 {
		t.Fatalf("Asp %v, сетка %d, раскладка %q (через сгиб: %v), шагов %d", aspOf(e2, "w"), len(p2.Spreads[0].Grid), cur.Key,
			crossesSpine(cur.Rects[0], prm), edtest.UndoLen(e2)-undo)
	}
}

// crossesSpine — ячейка r лежит на корешке разворота.
func crossesSpine(r layout.Rect, prm layout.Params) bool {
	return r.X < prm.PageW-0.01 && r.X+r.W > prm.PageW+0.01
}

// TestBackToAutoPlain: галочка снята — как 0.22 (SetGrid(false)): свой кадр снимается, в том числе
// оставшийся от прошлого «Вернуть авто»; кадр S остаётся. «Авто» и чужой лист — холостая.
func TestBackToAutoPlain(t *testing.T) {
	e, id, _ := marina(t)
	old, oldID, _ := marina(t)
	setAsp(e, map[string]float64{"a": 2, "b": 0.5}) // на ручном месте не действует
	undo := edtest.UndoLen(e)
	v, err := e.BackToAuto(id, "", false)
	if err != nil {
		t.Fatal(err)
	}
	was := old.SetGrid(oldID, false)
	if aspOf(e, "a") != 0 || aspOf(e, "b") != 0 || v.Spreads[0].Grid || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("Asp a %v, b %v, сетка %v, шагов %d", aspOf(e, "a"), aspOf(e, "b"), v.Spreads[0].Grid, edtest.UndoLen(e)-undo)
	}
	if !slices.Equal(v.Spreads[0].Rects, was.Spreads[0].Rects) || v.Spreads[0].Layout != was.Spreads[0].Layout {
		t.Fatalf("не как SetGrid(false): %v / %v", v.Spreads[0].Rects, was.Spreads[0].Rects)
	}
	if f := edtest.P(e).Photos["a"].Frame; f == nil || f.S != 1.3 {
		t.Fatalf("кадр снят: %+v", f)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, keep := range []bool{false, true} {
		e.BackToAuto(id, "", keep)
		e.BackToAuto("нет такого", "", keep)
	}
	edtest.Same(t, e, rev, undo, "«Авто» и чужой лист")
}

// TestCropsToKeep: N — фото, у которых окно отличается от фото больше чем на 1 %; «Целиком»,
// пустое место, «Авто» и чужой лист не в счёт. Счёт — без правки; «Вернуть авто» ставит ровно им.
func TestCropsToKeep(t *testing.T) {
	contain := edtest.Gp(170, 120, 100, 100, "e")
	contain.Fit = model.FitContain
	e := edtest.GridEd(t, []*model.Place{
		edtest.Gp(10, 10, marinaW, marinaH, "a"), // 1,685 — правлено
		edtest.Gp(160, 10, 150, 100, "b"),        // 1,5 — как фото
		edtest.Gp(320, 10, 151, 100, "c"),        // 1,51 — в пределах 1 %
		edtest.Gp(10, 120, 152, 100, "d"),        // 1,52 — за пределом
		contain,                                  // «Целиком»: окно не обрезает
		edtest.Gp(300, 120, 100, 100, ""),        // пустое
	}, []string{"x"})
	setAsp(e, map[string]float64{"b": 2, "e": 0.5}) // прежний свой кадр снимается
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if n := e.CropsToKeep("s1"); n != 2 {
		t.Fatalf("N = %d, ждали 2 (a, d)", n)
	}
	for _, id := range []string{"s2", "нет такого", ""} {
		if n := e.CropsToKeep(id); n != 0 {
			t.Fatalf("%q: N = %d", id, n)
		}
	}
	edtest.Same(t, e, rev, undo, "счёт N")
	if _, err := e.BackToAuto("s1", "", true); err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{"a": model.ClampAsp(marinaW / marinaH), "b": 0, "c": 0, "d": model.ClampAsp(1.52), "e": 0, "x": 0}
	for id, a := range want {
		if got := aspOf(e, id); got != a {
			t.Fatalf("Asp %s = %v, ждали %v", id, got, a)
		}
	}
}

// TestFitPhotoIgnoresAsp: «Под фото» подгоняет место под целое фото, а не под свой кадр (098-18);
// свой кадр остаётся до «Вернуть авто», которое его пересчитает (тогда — 0).
func TestFitPhotoIgnoresAsp(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, marinaW, marinaH, "a"), edtest.Gp(200, 10, 100, 100, "b")})
	setAsp(e, map[string]float64{"a": model.ClampAsp(marinaW / marinaH), "b": 0.7})
	if _, err := e.ArrangePlaces("s1", "fitPhoto", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		// Место округляется до 0,01 мм (100 × 66,67): допуск — доля этой сотой.
		if math.Abs(pl.W/pl.H-1.5) > 1e-3 {
			t.Fatalf("место %s: %v × %v — не под целое фото 3:2", pl.Photo, pl.W, pl.H)
		}
	}
	if aspOf(e, "a") == 0 || aspOf(e, "b") == 0 {
		t.Fatal("«Под фото» тронуло свой кадр")
	}
	if n := e.CropsToKeep("s1"); n != 0 {
		t.Fatalf("после «Под фото» N = %d", n)
	}
}

// TestOwnCropTurn: поворот на 90° поворачивает и свой кадр (1/Asp), на 180° — нет; фото без
// своего кадра остаётся без него. Ячейка «Авто» следует за своим кадром; один шаг отмены.
func TestOwnCropTurn(t *testing.T) {
	e, id := edtest.AutoEd(t)
	a := model.ClampAsp(marinaW / marinaH)
	setAsp(e, map[string]float64{"a": a, "b": 0.5})
	undo := edtest.UndoLen(e)
	v := e.TurnPhotos([]string{"a", "b", "c"}, 1)
	if got := []float64{aspOf(e, "a"), aspOf(e, "b"), aspOf(e, "c")}; !slices.Equal(got, []float64{model.ClampAsp(1 / a), 2, 0}) ||
		edtest.UndoLen(e) != undo+1 {
		t.Fatalf("после 90°: %v, шагов %d", got, edtest.UndoLen(e)-undo)
	}
	if cell, _ := fracOf(t, v, "a"); math.Abs(cell.W/cell.H/aspOf(e, "a")-1) > 1e-9 {
		t.Fatalf("ячейка a %v × %v, свой кадр %v", cell.W, cell.H, aspOf(e, "a"))
	}
	// Карточка «Свой кадр» — из вида: Own у фото со своим кадром, 0 — без него; на ручном месте — 0.
	own := func(v model.View, ph string) float64 {
		return v.Spreads[0].Frames[slices.Index(v.Spreads[0].Photos, ph)].Own
	}
	if own(v, "a") != aspOf(e, "a") || own(v, "c") != 0 {
		t.Fatalf("Own в виде: a %v, c %v", own(v, "a"), own(v, "c"))
	}
	if g := e.SetGrid(id, true); own(g, "a") != 0 {
		t.Fatalf("Own на ручном месте: %v", own(g, "a"))
	}
	e.Undo()
	turned := aspOf(e, "a")
	if e.TurnPhotos([]string{"a"}, 2); aspOf(e, "a") != turned {
		t.Fatalf("180° сменил свой кадр: %v → %v", turned, aspOf(e, "a"))
	}
	if e.TurnPhotos([]string{"a"}, -1); math.Abs(aspOf(e, "a")-a) > 1e-4 {
		t.Fatalf("обратный поворот: %v, было %v", aspOf(e, "a"), a)
	}
	e.Undo()
	e.Undo()
	if aspOf(e, "a") != turned {
		t.Fatalf("отмена: %v, ждали %v", aspOf(e, "a"), turned)
	}
}

// TestOwnCropReset: «Сбросить кадр» — кадр по умолчанию и без своего кадра, ячейка снова в
// пропорциях фото; повтор и фото без правок — холостая. Фото ушло из мест — свой кадр забыт.
func TestOwnCropReset(t *testing.T) {
	e, _ := edtest.AutoEd(t)
	e.SetFrame("a", 1.3, 0.2, 0)
	setAsp(e, map[string]float64{"a": 1.8, "b": 0.5, "c": 1.2})
	undo := edtest.UndoLen(e)
	v := e.ResetFrame("a")
	if f := edtest.P(e).Photos["a"].FrameOrDefault(); f.S != 1 || f.X != 0 || aspOf(e, "a") != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("кадр %+v, Asp %v, шагов %d", f, aspOf(e, "a"), edtest.UndoLen(e)-undo)
	}
	if cell, _ := fracOf(t, v, "a"); math.Abs(cell.W/cell.H-1.5) > 1e-9 {
		t.Fatalf("ячейка a %v × %v — не 3:2", cell.W, cell.H)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.ResetFrame("a")
	e.ResetFrame("нет такого")
	edtest.Same(t, e, rev, undo, "повторный сброс")
	if e.ResetFrame("b"); aspOf(e, "b") != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("свой кадр без S не сброшен: %v", aspOf(e, "b"))
	}
	e.RemovePhotos([]string{"c"})
	if _, ok := edtest.P(e).Photos["c"]; ok {
		t.Fatal("фото без мест осталось в альбоме со своим кадром")
	}
}

// TestCopyOwnCrop: у копий одного файла свой кадр свой — по своим местам; сброс одной другую
// не трогает.
func TestCopyOwnCrop(t *testing.T) {
	p := model.New()
	path := edtest.Abs("k.jpg")
	base := library.IDFor(path)
	cp := model.CopyID(base, 2)
	for _, id := range []string{base, cp} {
		p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200}
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{base, cp}, Grid: []*model.Place{
		{ID: "000000000001", X: 10, Y: 10, W: marinaW, H: marinaH, Photo: base},
		{ID: "000000000002", X: 320, Y: 10, W: 100, H: 125, Photo: cp},
	}}}
	e := edtest.Open(t, p, "")
	if n := e.CropsToKeep("s1"); n != 2 {
		t.Fatalf("N = %d, ждали 2", n)
	}
	if _, err := e.BackToAuto("s1", "", true); err != nil {
		t.Fatal(err)
	}
	if a, c := aspOf(e, base), aspOf(e, cp); a != model.ClampAsp(marinaW/marinaH) || c != 0.8 {
		t.Fatalf("Asp исходной %v, копии %v", a, c)
	}
	if err := edtest.FamilyInvariant(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	if e.ResetFrame(cp); aspOf(e, cp) != 0 || aspOf(e, base) == 0 {
		t.Fatalf("сброс копии: исходная %v, копия %v", aspOf(e, base), aspOf(e, cp))
	}
}

// TestBackToAutoOnAuto — «Вернуть авто» на развороте «Авто» только ставит ключ из галереи (свой
// кадр не трогает): ключ, который строится и проходит сгиб, — один шаг отмены; чужой — холостая.
func TestBackToAutoOnAuto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.BackToAuto(id, "нет-такого", true); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "чужой ключ")
	for _, r := range edtest.RowsOf(e, 0)[1:] {
		v, err := e.BackToAuto(id, r.Key, true)
		if err != nil {
			t.Fatal(err)
		}
		if v.Spreads[0].Layout != r.Key {
			continue // не прошёл сгиб — холостая
		}
		if edtest.UndoLen(e) != undo+1 || v.Spreads[0].Grid {
			t.Fatalf("ключ %s: шагов %d, сетка %v", r.Key, edtest.UndoLen(e)-undo, v.Spreads[0].Grid)
		}
		return
	}
	t.Fatal("ни один ключ галереи не встал")
}
