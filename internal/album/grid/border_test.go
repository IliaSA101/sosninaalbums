package grid_test

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочки мест (079 design §8, §10): постановка, правка, отмена, копии мест со своей рамочкой.

const frKey = "my:0123456789abcdef"

// frPic — своя картинка-рамочка из библиотеки (как отдаёт Store.Frame).
func frPic() model.BorderSrc {
	return model.BorderSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(frKey), Path: edtest.Abs("frame.png"), Lib: frKey, W: 3000, H: 4000},
		Cut: [4]float64{0.14, 0.14, 0.14, 0.14}}
}

var line = model.BorderSrc{Kind: "line", Inks: []string{"#FFFFFF"}, Width: 2, Preset: "sa:line"}

// bordersOf — рамочки мест разворота i по порядку.
func bordersOf(e *album.Editor, i int) []*model.Border {
	var out []*model.Border
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		out = append(out, pl.Border)
	}
	return out
}

// threeEd — сетка из трёх мест (два с фото, одно пустое) и «Авто» s2.
func threeEd(t *testing.T) *album.Editor {
	return edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 150, "a"), edtest.Gp(130, 10, 80, 80, "b"), edtest.Gp(130, 120, 80, 80, "")},
		[]string{"c", "d"})
}

// mustBorders — инварианты рамочек и круг Save → Load: рамочки те же.
func mustBorders(t *testing.T, e *album.Editor) {
	t.Helper()
	edtest.MustGrid(t, e)
	q := edtest.SaveLoad(t, e)
	for i, s := range edtest.P(e).Spreads {
		for j, pl := range s.Grid {
			if got := q.Spreads[i].Grid[j].Border; !reflect.DeepEqual(got, pl.Border) {
				t.Fatalf("после Save → Load место %s: %+v, было %+v", pl.ID, got, pl.Border)
			}
		}
	}
}

// Встроенная на группу (ссылки — место и фото): один шаг, у каждого места своя зажатая копия
// (цвет нижним регистром), повтор — холостая; пустое место тоже; отмена снимает все.
func TestApplyBorder(t *testing.T) {
	e := threeEd(t)
	undo := edtest.UndoLen(e)
	ids := []string{"b", "000000000001", "000000000003", "нет такого", "b"}
	if _, err := e.ApplyBorder("s1", ids, line); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("группа — один шаг, шагов +%d", edtest.UndoLen(e)-undo)
	}
	bs := bordersOf(e, 0)
	want := &model.Border{Kind: "line", Inks: []string{"#ffffff"}, Width: 2, Preset: "sa:line"}
	for i, b := range bs {
		if !reflect.DeepEqual(b, want) {
			t.Fatalf("место %d: %+v", i, b)
		}
	}
	if bs[0] == bs[1] || bs[1] == bs[2] {
		t.Fatal("общий указатель рамочки у мест")
	}
	mustBorders(t, e)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyBorder("s1", ids, line); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "та же рамочка")
	if bordersOf(e, 0)[0] != bs[0] {
		t.Fatal("повтор заменил указатель")
	}
	e.Undo()
	for i, b := range bordersOf(e, 0) {
		if b != nil {
			t.Fatalf("отмена: у места %d осталась %+v", i, b)
		}
	}
}

// У «Авто» постановка закрепляет разворот тем же шагом (отмена — снова «Авто»); снятие у «Авто»,
// пустой список и неизвестный лист — холостые.
func TestApplyBorderAuto(t *testing.T) {
	e := threeEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, c := range []struct {
		sp  string
		ids []string
		src model.BorderSrc
	}{{"s2", []string{"c"}, model.BorderSrc{}}, {"s2", nil, line}, {"s2", []string{"нет"}, line}, {"нет", []string{"c"}, line}} {
		if _, err := e.ApplyBorder(c.sp, c.ids, c.src); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "холостая "+c.sp)
	}
	if len(edtest.P(e).Spreads[1].Grid) != 0 {
		t.Fatal("холостая закрепила «Авто»")
	}
	if _, err := e.ApplyBorder("s2", []string{"c", "d"}, line); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 || len(edtest.P(e).Spreads[1].Grid) != 2 {
		t.Fatalf("«Авто»: шагов +%d, мест %d", edtest.UndoLen(e)-undo, len(edtest.P(e).Spreads[1].Grid))
	}
	for _, b := range bordersOf(e, 1) {
		if b == nil || b.Kind != "line" {
			t.Fatalf("«Авто»: рамочка %+v", b)
		}
	}
	mustBorders(t, e)
	e.Undo()
	if len(edtest.P(e).Spreads[1].Grid) != 0 {
		t.Fatal("отмена не вернула «Авто»")
	}
}

// Своя картинка: запись Photos по ключу тем же шагом, срез хранится, вид — картинка; неверный
// ключ, чужой ID, вид не по грамматике и вид вместе с картинкой — ErrBorder без шага; «Убрать»
// снимает, запись уходит; отмена возвращает и рамочки, и запись.
func TestApplyBorderPic(t *testing.T) {
	e := threeEd(t)
	src := frPic()
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	bad := []model.BorderSrc{
		{Photo: model.PhotoInfo{ID: "x", Lib: frKey}},
		{Photo: model.PhotoInfo{ID: model.LibPhotoID("my:x"), Lib: "my:x"}},
		{Photo: model.PhotoInfo{ID: "a"}}, // фото альбома рамочкой не ставится
		{Kind: "Линия"},
		{Kind: "line", Photo: src.Photo, Cut: src.Cut}, // вид и картинка сразу — не «ровно одно»
	}
	for _, b := range bad {
		if _, err := e.ApplyBorder("s1", []string{"a"}, b); !errors.Is(err, model.ErrBorder) {
			t.Fatalf("%+v: ошибка %v", b, err)
		}
		edtest.Same(t, e, rev, undo, "отказ")
	}
	v, err := e.ApplyBorder("s1", []string{"a", "000000000003"}, src)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := e.Ref(src.Photo.ID)
	if !ok || r.Lib != frKey || r.W != 3000 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("запись рамочки %+v (есть %v), шагов +%d", r, ok, edtest.UndoLen(e)-undo)
	}
	b := bordersOf(e, 0)
	if b[0] == nil || b[0].Photo != src.Photo.ID || b[0].Cut != src.Cut || b[1] != nil || b[2] == nil || b[0] == b[2] {
		t.Fatalf("рамочки %+v %+v %+v", b[0], b[1], b[2])
	}
	// Место 100×150, картинка 3000×4000: k = min(100/3000, 150/4000) = 0,0333 мм/px.
	pv := v.Spreads[0].Places[0]
	if pv.Border == nil || !pv.Border.Pic || math.Abs(pv.Border.Ins[0]-14) > 1e-9 || math.Abs(pv.Border.Ins[1]-18.6667) > 1e-3 {
		t.Fatalf("вид: %+v", pv.Border)
	}
	mustBorders(t, e)
	if _, err := e.ApplyBorder("s1", []string{"a", "000000000003"}, model.BorderSrc{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.Ref(src.Photo.ID); ok || bordersOf(e, 0)[0] != nil {
		t.Fatal("«Убрать»: рамочка или запись остались")
	}
	e.Undo()
	if got, ok := e.Ref(src.Photo.ID); !ok || !reflect.DeepEqual(got, r) || bordersOf(e, 0)[0] == nil || bordersOf(e, 0)[2] == nil {
		t.Fatalf("отмена «Убрать»: запись %+v (есть %v), рамочки %+v", got, ok, bordersOf(e, 0))
	}
}

// Своя картинка, уже стоящая фоном (тот же ключ — та же запись): рамочка запись фона не
// переписывает (путь к исходнику, размеры исходника), и после Save → Load тоже — как у стикера.
func TestApplyBorderOverBackdrop(t *testing.T) {
	e := threeEd(t)
	orig := model.PhotoInfo{ID: model.LibPhotoID(frKey), Path: edtest.Abs("orig-frame.png"), Lib: frKey, W: 6000, H: 8000}
	if _, err := e.ApplyBackdrop("s2", model.ScopeLeft, orig); err != nil {
		t.Fatal(err)
	}
	want, ok := e.Ref(orig.ID)
	if !ok || want.W != 6000 {
		t.Fatalf("запись фона: %+v", want)
	}
	if _, err := e.ApplyBorder("s1", []string{"a", "b"}, frPic()); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.Ref(orig.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("рамочка переписала запись фона: %+v, было %+v", got, want)
	}
	if b := bordersOf(e, 0); b[0] == nil || b[0].Photo != orig.ID || b[1] == nil || b[1].Photo != orig.ID {
		t.Fatalf("рамочки %+v", b)
	}
	mustBorders(t, e)
	q := edtest.SaveLoad(t, e) // путь своего Load берёт по ключу у библиотеки (здесь её нет)
	if got := q.Photos[orig.ID]; got.Lib != frKey || got.W != 6000 || got.H != 8000 {
		t.Fatalf("Save → Load: %+v", got)
	}
}

// Шаблон разворота (067) рамочек не несёт: тот же шаблон поверх мест с рамочкой снимает рамочки
// одним шагом — и при том же оформлении, и после смены цвета фона (снятие не зависит от фона).
// Повтор после снятия — холостая; отмена возвращает рамочки.
func TestSpreadTplDropsBorder(t *testing.T) {
	for _, bg := range []string{"", "#aabbcc"} {
		e := threeEd(t)
		tp, err := e.SpreadTplOf("s1", model.ScopeSpread, true, false)
		if err != nil {
			t.Fatal(err)
		}
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "тот же шаблон без рамочек")
		if _, err := e.ApplyBorder("s1", []string{"a", "b", "000000000003"}, line); err != nil {
			t.Fatal(err)
		}
		if bg != "" {
			e.SetBackground("s1", model.ScopeSpread, bg)
		}
		undo = edtest.UndoLen(e)
		if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
			t.Fatal(err)
		}
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("фон %q: шагов +%d", bg, edtest.UndoLen(e)-undo)
		}
		for i, b := range bordersOf(e, 0) {
			if b != nil {
				t.Fatalf("фон %q: у места %d осталась %+v", bg, i, b)
			}
		}
		mustBorders(t, e)
		rev, undo = edtest.Rev(e), edtest.UndoLen(e)
		if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
			t.Fatal(err)
		}
		edtest.Same(t, e, rev, undo, "повтор после снятия")
		e.Undo()
		for i, b := range bordersOf(e, 0) {
			if b == nil || b.Kind != "line" {
				t.Fatalf("фон %q: отмена, место %d — %+v", bg, i, b)
			}
		}
	}
}

// Встроенная неизвестного, но правильного вида (файл новее) ставится и хранится; накладка
// (Ins = 0) окна не уменьшает.
func TestApplyBorderUnknownKind(t *testing.T) {
	e := threeEd(t)
	v, err := e.ApplyBorder("s1", []string{"a", "b"}, model.BorderSrc{Kind: "future-frame"})
	if err != nil {
		t.Fatal(err)
	}
	pv := v.Spreads[0].Places[0]
	if pv.Border == nil || pv.Border.Known || pv.Border.Ins != [4]float64{} || v.Spreads[0].Frames[0].Win != edtest.RectOf(v, 0, "a") {
		t.Fatalf("неизвестный вид: %+v, окно %+v", pv.Border, v.Spreads[0].Frames[0].Win)
	}
	mustBorders(t, e)
}

// SetBorder: серия одной сессией — один шаг; правка снимает Preset; те же числа — холостая
// (плитка стоит); поля чужого вида не действуют; NaN — холостая до правки; места без рамочки
// не трогаются; новый указатель на каждую правку.
func TestSetBorderPatch(t *testing.T) {
	e := threeEd(t)
	if _, err := e.ApplyBorder("s1", []string{"a"}, line); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ApplyBorder("s1", []string{"b"}, frPic()); err != nil {
		t.Fatal(err)
	}
	all := []string{"a", "b", "000000000003"}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetBorder("s1", all, model.BorderPatch{Width: 2, Inks: []string{"#ffffff"}}, "b1")
	edtest.Same(t, e, rev, undo, "те же числа")
	if bordersOf(e, 0)[0].Preset != "sa:line" {
		t.Fatal("холостая сняла плитку")
	}
	e.SetBorder("s1", all, model.BorderPatch{Width: math.NaN()}, "b1")
	e.SetBorder("s1", all, model.BorderPatch{Scale: math.Inf(1)}, "b1")
	edtest.Same(t, e, rev, undo, "NaN/Inf")
	prev := bordersOf(e, 0)
	e.SetBorder("s1", all, model.BorderPatch{Width: 3.04}, "b1")
	e.SetBorder("s1", all, model.BorderPatch{Width: 4, Inks: []string{"#C8A24A"}, Scale: 1.5}, "b1")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("серия — один шаг, шагов +%d", edtest.UndoLen(e)-undo)
	}
	b := bordersOf(e, 0)
	if want := (&model.Border{Kind: "line", Inks: []string{"#c8a24a"}, Width: 4}); !reflect.DeepEqual(b[0], want) {
		t.Fatalf("встроенная: %+v", b[0])
	}
	if b[1].Scale != 1.5 || b[1].Inks != nil || b[1].Width != 0 || b[2] != nil {
		t.Fatalf("картинка %+v, пустое %+v", b[1], b[2])
	}
	if b[0] == prev[0] || b[1] == prev[1] {
		t.Fatal("правка на месте, а не заменой указателя")
	}
	if prev[0].Width != 2 || prev[0].Preset != "sa:line" {
		t.Fatalf("прежняя рамочка изменилась на месте: %+v", prev[0])
	}
	mustBorders(t, e)
	e.SetBorder("s1", []string{"a"}, model.BorderPatch{Width: 50, Inks: []string{}}, "b2") // предел и цвета вида
	if b := bordersOf(e, 0)[0]; b.Width != 20 || b.Inks != nil {
		t.Fatalf("предел толщины и пустые цвета: %+v", b)
	}
	e.Undo()
	e.Undo()
	if b := bordersOf(e, 0)[0]; b.Width != 2 || b.Preset != "sa:line" {
		t.Fatalf("отмена серии: %+v", b)
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.SetBorder("s2", []string{"c"}, model.BorderPatch{Width: 3}, "")
	edtest.Same(t, e, rev, undo, "«Авто» без рамочек")
}

// Рамочка едет с местом через правки сетки: перенос, размер, угол, форма, режим кадра,
// отражение, слои, «Пропорции под фото», группа; у овала окно ровное.
func TestBorderSurvivesGridOps(t *testing.T) {
	e := threeEd(t)
	if _, err := e.ApplyBorder("s1", []string{"a", "b", "000000000003"}, model.BorderSrc{Kind: "polaroid", Width: 5}); err != nil {
		t.Fatal(err)
	}
	e.SetPlaces("s1", []model.PlaceBox{{ID: "a", X: 12, Y: 12, W: 90, H: 140}}, "")
	e.SetPlaceAngle("s1", "b", 15)
	e.SetPlaceShape("s1", []string{"b"}, model.ShapeOval)
	e.SetPlaceFit("s1", []string{"a"}, model.FitContain)
	e.MirrorGrid("s1", model.ScopeSpread)
	e.ArrangeLayers("s1", "back", []string{"a"})
	e.TransformGroup("s1", []string{"a", "b"}, 1.1, 100, 100, 3, 4, "")
	for i, b := range bordersOf(e, 0) {
		if b == nil || b.Kind != "polaroid" || b.Width != 5 {
			t.Fatalf("место %d: рамочка %+v", i, b)
		}
	}
	v := e.View()
	if ins := v.Spreads[0].Places[1].Border.Ins; ins != [4]float64{17.5, 17.5, 17.5, 17.5} {
		t.Fatalf("овал: отступы %v", ins)
	}
	mustBorders(t, e)
}

// «Пропорции под фото»: окно встроенной — точно в пропорции фото; у картинки — две итерации;
// картинка без размеров — как без рамочки; без рамочки — прежняя формула.
func TestFitPhotoBorder(t *testing.T) {
	cases := []struct {
		name string
		src  model.BorderSrc
		dims bool
		tol  float64
	}{
		{"без рамочки", model.BorderSrc{}, true, 1e-9},
		{"полароид", model.BorderSrc{Kind: "polaroid", Width: 4}, true, 0.01},
		{"картинка", frPic(), true, 0.05},
		{"картинка без размеров", frPic(), false, 1e-9},
	}
	for _, c := range cases {
		e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 120, 60, "a")}) // фото 3:2
		if c.src.Kind != "" || c.src.Photo.ID != "" {
			if _, err := e.ApplyBorder("s1", []string{"a"}, c.src); err != nil {
				t.Fatal(err)
			}
		}
		if !c.dims {
			edtest.Change(e, func(p *model.Project) {
				r := p.Photos[c.src.Photo.ID]
				r.W, r.H = 0, 0
				p.Photos[c.src.Photo.ID] = r
			})
		}
		if _, err := e.ArrangePlaces("s1", "fitPhoto", []string{"a"}); err != nil {
			t.Fatal(err)
		}
		v := e.View()
		win, pl := v.Spreads[0].Frames[0].Win, edtest.P(e).Spreads[0].Grid[0]
		if math.Abs(win.W/win.H-1.5) > c.tol {
			t.Fatalf("%s: окно %.3f×%.3f, пропорция %.4f", c.name, win.W, win.H, win.W/win.H)
		}
		if !c.dims && (win.W != pl.W || win.H != pl.H || pl.H != 80) {
			t.Fatalf("%s: окно %+v, место %+v", c.name, win, *pl)
		}
		if c.name == "полароид" && (pl.W != 120 || pl.H <= 80) {
			t.Fatalf("%s: место %+v", c.name, *pl)
		}
		mustBorders(t, e)
	}
}

// «Разделить» (079-23): половины со своими рамочками; правка цвета одной не задевает другую,
// и после отмены и возврата тоже.
func TestSplitSetBorderOne(t *testing.T) {
	e := threeEd(t)
	if _, err := e.ApplyBorder("s1", []string{"a"}, line); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ArrangePlaces("s1", "splitV", []string{"a"}); err != nil {
		t.Fatal(err)
	}
	b := bordersOf(e, 0)
	if b[0] == nil || b[1] == nil || b[0] == b[1] || &b[0].Inks[0] == &b[1].Inks[0] {
		t.Fatalf("половины делят рамочку: %p %p", b[0], b[1])
	}
	half := edtest.P(e).Spreads[0].Grid[1].ID
	e.SetBorder("s1", []string{half}, model.BorderPatch{Inks: []string{"#ff0000"}}, "")
	if b := bordersOf(e, 0); b[0].Inks[0] != "#ffffff" || b[1].Inks[0] != "#ff0000" {
		t.Fatalf("цвет половин: %v %v", b[0].Inks, b[1].Inks)
	}
	mustBorders(t, e)
	e.Undo()
	e.Redo()
	if b := bordersOf(e, 0); b[0].Inks[0] != "#ffffff" || b[1].Inks[0] != "#ff0000" {
		t.Fatalf("после отмены и возврата: %v %v", b[0].Inks, b[1].Inks)
	}
}

// «Дублировать» (с оформлением и без, два раза) и копия разворота — у каждой копии свои
// рамочки; правка одной копии не задевает источник и другие копии.
func TestDuplicateBorderOwn(t *testing.T) {
	e := threeEd(t)
	if _, err := e.ApplyBorder("s1", []string{"a", "000000000003"}, line); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DuplicateGrid("s1", 2, false); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DuplicateGrid("s1", 1, true); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	p := edtest.P(e)
	for i := 1; i <= 3; i++ {
		if b := bordersOf(e, i); b[0] == nil || b[1] != nil || b[2] == nil || b[0].Kind != "line" {
			t.Fatalf("копия %d: %+v", i, b)
		}
	}
	e.SetBorder(p.Spreads[2].ID, []string{p.Spreads[2].Grid[0].ID}, model.BorderPatch{Width: 6}, "")
	for i := range 4 {
		want := 2.0
		if i == 2 {
			want = 6
		}
		if w := bordersOf(e, i)[0].Width; w != want {
			t.Fatalf("разворот %d: толщина %v, ждали %v", i, w, want)
		}
	}
	mustBorders(t, e)
}

// «Копировать раскладку» на несколько целей (страница → разворот: одни доли на обе страницы;
// и на страницу другого листа) — рамочки едут, у каждой цели свои; правка одной цели не
// задевает другие и источник. Готовый разворот (067) несёт рамочки мест — свои указатели (131 G1).
func TestCopyLayoutBorderOwn(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a")}, []string{"c", "d"}, []string{"x"})
	if _, err := e.ApplyBorder("s1", []string{"a"}, line); err != nil {
		t.Fatal(err)
	}
	targets := []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}, {Spread: "s3", Scope: model.ScopeRight}}
	if _, _, err := e.CopyGrid("s1", model.ScopeLeft, targets, false, false); err != nil {
		t.Fatal(err)
	}
	edtest.MustGrid(t, e)
	// Рамочки мест, скопированных на правую страницу s3 (левая — места «Авто» без рамочек).
	right := func() []*model.Border {
		var out []*model.Border
		for _, pl := range edtest.P(e).Spreads[2].Grid {
			if pl.X >= 281 {
				out = append(out, pl.Border)
			}
		}
		return out
	}
	if n2, r3 := len(bordersOf(e, 1)), right(); n2 != 2 || len(r3) != 1 || r3[0] == nil || r3[0].Width != 2 {
		t.Fatalf("цели: мест s2 %d, правая s3 %+v", n2, r3)
	}
	for _, b := range bordersOf(e, 1) {
		if b == nil || b.Kind != "line" || b.Width != 2 {
			t.Fatalf("цель s2: %+v", b)
		}
	}
	p := edtest.P(e)
	e.SetBorder("s2", []string{p.Spreads[1].Grid[1].ID}, model.BorderPatch{Width: 7}, "")
	got := []float64{bordersOf(e, 0)[0].Width, bordersOf(e, 1)[0].Width, bordersOf(e, 1)[1].Width, right()[0].Width}
	if !reflect.DeepEqual(got, []float64{2, 2, 7, 2}) {
		t.Fatalf("толщины после правки одной цели: %v", got)
	}
	mustBorders(t, e)
	tp, err := e.SpreadTplOf("s1", model.ScopeLeft, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.ApplySpreadTpl("s3", model.ScopeLeft, tp, nil); err != nil {
		t.Fatal(err)
	}
	// 131 G1: готовый разворот несёт рамочки мест (borders; 0.27 их не снимала, 079-10) — на левой
	// странице встаёт рамочка источника, и она своя у места, а не общий указатель с s1.
	if len(tp.Borders) != 1 || tp.Borders[0].Kind != "line" {
		t.Fatalf("снимок без рамочки: %+v", tp.Borders)
	}
	src := edtest.P(e).Spreads[0].Grid[0]
	for _, pl := range edtest.P(e).Spreads[2].Grid {
		if pl.X < 281 && (pl.Border == nil || pl.Border.Kind != "line" || pl.Border.Width != 2 || pl.Border == src.Border) {
			t.Fatalf("готовый разворот не принёс свою рамочку: %+v", pl.Border)
		}
	}
	edtest.MustGrid(t, e)
}

// «Выбрать раскладку» (079-29) снимает рамочки вместе с сеткой; Ctrl+Z возвращает и сетку, и
// рамочки; выбор той же раскладки — холостая.
func TestSetLayoutDropsBorderUndo(t *testing.T) {
	e := threeEd(t)
	if _, err := e.ApplyBorder("s2", []string{"c", "d"}, line); err != nil {
		t.Fatal(err)
	}
	key := edtest.PickKey(t, e, 1, edtest.AnyKey)
	undo := edtest.UndoLen(e)
	e.SetLayout("s2", key)
	if len(edtest.P(e).Spreads[1].Grid) != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("выбор раскладки: мест %d, шагов +%d", len(edtest.P(e).Spreads[1].Grid), edtest.UndoLen(e)-undo)
	}
	e.Undo()
	b := bordersOf(e, 1)
	if len(b) != 2 || b[0] == nil || b[1] == nil || b[0].Kind != "line" {
		t.Fatalf("отмена: рамочки %+v", b)
	}
	mustBorders(t, e)
}
