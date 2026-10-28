package design_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Ч/б и тон фото (068): SetGray «всем или никому», SetTone с зажимом, отмена, вид, перенос.

func adjOf(t *testing.T, e *album.Editor, id string) model.Tone {
	t.Helper()
	r, ok := e.Ref(id)
	if !ok {
		t.Fatalf("фото %s нет", id)
	}
	if r.Adj == nil {
		return model.Tone{}
	}
	return *r.Adj
}

// Не у всех включено → всем, иначе никому; яркость и контраст стоят; группа — один шаг.
func TestSetGrayToggleAll(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetTone("b", 12, -7)
	e.SetGray([]string{"a"})
	undo := edtest.UndoLen(e)
	grays := func() [3]bool {
		return [3]bool{adjOf(t, e, "a").Gray, adjOf(t, e, "b").Gray, adjOf(t, e, "c").Gray}
	}
	v := e.SetGray([]string{"a", "b", "c", "a", "нет", ""})
	if got := grays(); got != [3]bool{true, true, true} {
		t.Fatalf("не у всех → всем: %v", got)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("группа — один шаг, шагов +%d", edtest.UndoLen(e)-undo)
	}
	if b := adjOf(t, e, "b"); b.Bright != 12 || b.Contrast != -7 {
		t.Fatalf("ч/б задел тон: %+v", b)
	}
	if f := frameOf(t, v, "b"); !f.Gray || f.Bright != 12 || f.Contrast != -7 || f.Filter != "grayscale(1) brightness(112%) contrast(93%)" {
		t.Fatalf("вид b: %+v", f)
	}
	if f := frameOf(t, v, "c"); f.Filter != "grayscale(1)" {
		t.Fatalf("вид c: %q", f.Filter)
	}
	v = e.SetGray([]string{"a", "b", "c"})
	if got := grays(); got != [3]bool{} {
		t.Fatalf("у всех → никому: %v", got)
	}
	// Ч/б снят — у a и c тона не осталось (nil), у b остались яркость и контраст.
	if r, _ := e.Ref("a"); r.Adj != nil {
		t.Fatalf("пустой тон не снят: %+v", r.Adj)
	}
	if f := frameOf(t, v, "a"); f.Gray || f.Filter != "" {
		t.Fatalf("вид a без тона: %+v", f)
	}
	if f := frameOf(t, v, "b"); f.Filter != "brightness(112%) contrast(93%)" {
		t.Fatalf("вид b без ч/б: %q", f.Filter)
	}
}

// Ни одного известного ID — холостая правка: без шага и без rev.
func TestSetGrayNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetGray(nil)
	e.SetGray([]string{})
	e.SetGray([]string{"нет", ""})
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостая: rev %d → %d, undo %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
}

// Зажим −50…50 в Go; нули — без тона; ч/б остаётся; чужое фото и те же значения — холостые.
func TestSetToneClamp(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetGray([]string{"a"})
	e.SetTone("a", 70, -1_000_000)
	if a := adjOf(t, e, "a"); a != (model.Tone{Gray: true, Bright: 50, Contrast: -50}) {
		t.Fatalf("зажим: %+v", a)
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetTone("a", 51, -51) // после зажима — то же
	e.SetTone("a", 50, -50)
	e.SetTone("нет", 10, 10)
	e.SetTone("", 10, 10)
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостая: rev %d → %d, undo %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	e.SetTone("a", 0, 0)
	if a := adjOf(t, e, "a"); a != (model.Tone{Gray: true}) {
		t.Fatalf("нули при ч/б: %+v", a)
	}
	e.SetTone("b", 0, 0) // у b тона нет — холостая
	if r, _ := e.Ref("b"); r.Adj != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("нули у фото без тона: %+v, шагов +%d", r.Adj, edtest.UndoLen(e)-undo)
	}
}

// Каждый SetTone — шаг отмены; Undo и Redo ходят по шагу.
func TestSetToneUndoOneStep(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo := edtest.UndoLen(e)
	e.SetTone("a", 30, 0)
	e.SetTone("a", 30, -20)
	if edtest.UndoLen(e) != undo+2 {
		t.Fatalf("шагов +%d, ждали 2", edtest.UndoLen(e)-undo)
	}
	if f := frameOf(t, e.Undo(), "a"); f.Bright != 30 || f.Contrast != 0 || f.Filter != "brightness(130%)" {
		t.Fatalf("Undo: %+v", f)
	}
	if f := frameOf(t, e.Undo(), "a"); f.Bright != 0 || f.Filter != "" {
		t.Fatalf("второй Undo: %+v", f)
	}
	if r, _ := e.Ref("a"); r.Adj != nil {
		t.Fatalf("Undo оставил тон: %+v", r.Adj)
	}
	if f := frameOf(t, e.Redo(), "a"); f.Bright != 30 {
		t.Fatalf("Redo: %+v", f)
	}
}

// Тон едет с фото: обмен мест, перенос в другой разворот, смена раскладки, удаление и отмена.
func TestAdjustTravelsWithPhoto(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetGray([]string{"a"})
	e.SetTone("a", -10, 25)
	want := "grayscale(1) brightness(90%) contrast(125%)"
	check := func(when string, v model.View) {
		t.Helper()
		if f := frameOf(t, v, "a"); f.Filter != want || !f.Gray || f.Bright != -10 || f.Contrast != 25 {
			t.Fatalf("%s: %+v", when, f)
		}
		if f := frameOf(t, v, "b"); f.Filter != "" || f.Gray {
			t.Fatalf("%s: тон у соседа %+v", when, f)
		}
	}
	check("SetTone", e.View())
	check("раскладка", e.StepLayout(s1, "spread", 1))
	check("обмен", e.SwapPhotos("a", "c"))
	v, err := e.MovePhoto("a", s1, 0)
	if err != nil {
		t.Fatal(err)
	}
	check("перенос", v)
	e.RemovePhotos([]string{"a"})
	check("отмена удаления", e.Undo())
}

// Сетка: тон в виде места, места не меняются; «Авто» тоном не закрепляется.
func TestToneGridAndAuto(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	e.SetGray([]string{"a", "b"})
	e.SetTone("a", 5, 5)
	if edtest.P(e).Spreads[0].Grid != nil {
		t.Fatal("«Авто» закреплена тоном")
	}
	g := edtest.GridEd(t, edtest.Calendar())
	before := marshalGrid(g)
	g.SetGray([]string{"a", "c"})
	v := g.SetTone("c", -5, 0)
	if marshalGrid(g) != before {
		t.Fatal("места изменились")
	}
	if f := frameOf(t, v, "a"); f.Filter != "grayscale(1)" {
		t.Fatalf("сетка a: %+v", f)
	}
	if f := frameOf(t, v, "c"); f.Filter != "grayscale(1) brightness(95%)" {
		t.Fatalf("сетка c: %+v", f)
	}
	edtest.MustGrid(t, g)
}

// Фото, ушедшее из мест на фон, теряет тон (PruneRefs), как кадр и зеркало.
func TestToneDroppedOnBackdrop(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	e.SetGray([]string{"a", "b"})
	edtest.SetBg(t, e, s1, "spread", "a")
	if adjOf(t, e, "a") != (model.Tone{Gray: true}) {
		t.Fatal("фото и в месте, и на фоне потеряло тон")
	}
	e.RemovePhotos([]string{"a"})
	r, ok := e.Ref("a")
	if !ok || r.Adj != nil {
		t.Fatalf("фото только на фоне: есть %v, тон %+v", ok, r.Adj)
	}
	if adjOf(t, e, "b") != (model.Tone{Gray: true}) {
		t.Fatal("сосед потерял тон")
	}
	if v := e.View(); len(v.Spreads[0].Backdrops) != 1 {
		t.Fatalf("фон пропал: %+v", v.Spreads[0].Backdrops)
	}
}
