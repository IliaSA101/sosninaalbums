package album_test

// Правки агента «если всё ещё то же» (151 design §3): отметки, формат, семейство. Общая часть —
// ifRefuse: чужой rev, вышедший срок и закрытый Editor ничего не меняют (ни проект, ни rev, ни шаг).

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ifRefuse — три отказа сверки на редакторе e: call получает Expect и зовёт правку. Закрытый
// Editor проверяется последним (после Close правки уже не вернуть).
func ifRefuse(t *testing.T, e *album.Editor, call func(x album.Expect) error) {
	t.Helper()
	s, rev := take(e), edtest.Rev(e)
	if err := call(album.Expect{Rev: rev + 1}); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("чужой rev: %v", err)
	}
	s.same(t, e, "чужой rev")
	if err := call(album.Expect{Rev: rev, Ctx: canceled()}); !errors.Is(err, album.ErrLate) {
		t.Fatalf("срок: %v", err)
	}
	s.same(t, e, "срок вышел")
	e.Close()
	if err := call(album.Expect{Rev: rev}); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("закрытый Editor: %v", err)
	}
	s.same(t, e, "закрытый Editor")
}

// ifMarkEd — альбом на диске с фото a, b, c, d (ID от пути) в одном развороте.
func ifMarkEd(t *testing.T) (e *album.Editor, ph []model.PhotoInfo, path string) {
	t.Helper()
	dir := testkit.TempDir(t)
	path = filepath.Join(dir, "m.salbum")
	for _, n := range []string{"a", "b", "c", "d"} {
		ph = append(ph, edtest.Lib(filepath.Join(dir, n+".jpg"), model.XmpMarks{}))
	}
	e = edtest.Open(t, model.New(), path)
	edtest.NewSpread(e, -1, ph)
	return e, ph, path
}

// TestSetMarksIf: все четыре фото получают оценку, флажок и цвет одним шагом; changed — длина
// дельты; повтор и пустой список — холостые (ни шага, ни rev); Rating 0 снимает свою звезду;
// круг Save → Load держит отметки; отмена по rev снимает ровно шаг.
func TestSetMarksIf(t *testing.T) {
	e, ph, path := ifMarkEd(t)
	one, red := 1, "red"
	patch := model.MarkPatch{Rating: edtest.Ip(3), Flag: &one, Label: &red}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, n, err := e.SetMarksIf(ph, patch, album.Expect{Rev: rev})
	if err != nil || n != 4 || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("SetMarksIf: changed %d, rev %d → %d, шагов %d → %d, %v", n, rev, v.Rev, undo, edtest.UndoLen(e), err)
	}
	want := map[string][3]any{}
	for _, p := range ph {
		want[p.ID] = [3]any{3, "red", 1}
	}
	edtest.WantMarks(t, e, want)

	// Несколько фото, часть уже с отметкой: changed считает только изменившиеся.
	if _, n, err = e.SetMarksIf(ph, model.MarkPatch{Rating: edtest.Ip(5)}, album.Expect{Rev: v.Rev}); err != nil || n != 4 {
		t.Fatalf("оценка 5: changed %d, %v", n, err)
	}
	s := take(e)
	for name, c := range map[string]struct {
		ph    []model.PhotoInfo
		patch model.MarkPatch
	}{
		"повтор":        {ph, model.MarkPatch{Rating: edtest.Ip(5)}},
		"пустой список": {nil, patch},
		"пустой патч":   {ph, model.MarkPatch{}},
	} {
		if v, n, err := e.SetMarksIf(c.ph, c.patch, album.Expect{Rev: s.rev}); err != nil || n != 0 || v.Rev != s.rev {
			t.Fatalf("%s: changed %d, rev %d (был %d), %v", name, n, v.Rev, s.rev, err)
		}
		s.same(t, e, name)
	}

	// Toggle не задан: тот же цвет повторно не снимается (агент не переключает).
	if _, n, _ = e.SetMarksIf(ph, model.MarkPatch{Label: &red}, album.Expect{Rev: s.rev}); n != 0 {
		t.Fatalf("повтор цвета снял его: changed %d", n)
	}
	s.same(t, e, "повтор цвета")

	// Круг: Save → Load.
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := edtest.Loaded(q).Marks; len(got) != 4 || *got[ph[0].ID].Rating != 5 || *got[ph[3].ID].Label != "red" {
		t.Fatalf("после Save → Load: %v", got)
	}

	// Отмена по rev снимает шаг: оценка 5 → 3.
	if _, err := e.UndoIfRev(album.Expect{Rev: s.rev}); err != nil {
		t.Fatal(err)
	}
	if r := e.Marks()[ph[0].ID].Rating; r == nil || *r != 3 {
		t.Fatalf("после отмены оценка %v, ждали 3", r)
	}
}

// Rating 0 — «снять звёзды»: своя оценка 0 (перекрывает файл), а не отсутствие отметки; фото без
// оценки в файле и без своей — холостая.
func TestSetMarksIfRatingZero(t *testing.T) {
	e, ph, _ := ifMarkEd(t)
	edtest.SetMarks(t, e, ph[:2], model.MarkPatch{Rating: edtest.Ip(4)})
	_, n, err := e.SetMarksIf(ph, model.MarkPatch{Rating: edtest.Ip(0)}, album.Expect{Rev: edtest.Rev(e)})
	if err != nil || n != 2 {
		t.Fatalf("снять звёзды: changed %d, %v", n, err)
	}
	edtest.WantMarks(t, e, map[string][3]any{ph[0].ID: {0, "-", -9}, ph[1].ID: {0, "-", -9}})
}

// TestSetMarksIfRefuse: отказ сверки не трогает отметки.
func TestSetMarksIfRefuse(t *testing.T) {
	e, ph, _ := ifMarkEd(t)
	edtest.SetMarks(t, e, ph[:1], model.MarkPatch{Rating: edtest.Ip(2)})
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, err := e.SetMarksIf(ph, model.MarkPatch{Rating: edtest.Ip(5)}, x)
		return err
	})
	if r := e.Marks()[ph[0].ID].Rating; r == nil || *r != 2 {
		t.Fatalf("отказ изменил оценку: %v", r)
	}
}

// Человек правит отметки между чтением агента и его правкой: rev ушёл — агент отказан, отметки
// человека целы.
func TestSetMarksIfHumanFirst(t *testing.T) {
	e, ph, _ := ifMarkEd(t)
	rev := edtest.Rev(e)
	edtest.SetMarks(t, e, ph[:1], model.MarkPatch{Rating: edtest.Ip(1)})
	if _, _, err := e.SetMarksIf(ph, model.MarkPatch{Rating: edtest.Ip(5)}, album.Expect{Rev: rev}); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("после правки человека: %v", err)
	}
	edtest.WantMarks(t, e, map[string][3]any{ph[0].ID: {1, "-", -9}})
}

// TestSetSettingsIf: формат меняется как у SetSettings (тот же итог на двойнике), один шаг;
// тот же формат — холостая; отказ сверки не трогает; отмена возвращает прежний формат.
func TestSetSettingsIf(t *testing.T) {
	mk := func() *album.Editor {
		e := edtest.Open(t, model.New(), "")
		edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
		return e
	}
	e, twin := mk(), mk()
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Safe = 303.95, 202.95, 10
	twin.SetSettings(st)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, err := e.SetSettingsIf(st, album.Expect{Rev: rev})
	if err != nil || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("SetSettingsIf: rev %d → %d, шагов %d → %d, %v", rev, v.Rev, undo, edtest.UndoLen(e), err)
	}
	if got, want := edtest.P(e).Settings, edtest.P(twin).Settings; got != want {
		t.Fatalf("настройки %+v, как у SetSettings %+v", got, want)
	}
	s := take(e)
	if v, err = e.SetSettingsIf(st, album.Expect{Rev: s.rev}); err != nil || v.Rev != s.rev {
		t.Fatalf("тот же формат: rev %d (был %d), %v", v.Rev, s.rev, err)
	}
	s.same(t, e, "тот же формат")
	if _, err := e.UndoIfRev(album.Expect{Rev: s.rev}); err != nil || edtest.P(e).Settings.PageW == 303.95 {
		t.Fatalf("отмена не вернула формат: %v", err)
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, err := e.SetSettingsIf(st, x)
		return err
	})
}

// TestSetSettingsIfCoverFollows: обложка идёт за форматом так же, как у SetSettings (изделие и
// числа те же, что на двойнике), и круг Save → Load держит итог.
func TestSetSettingsIfCoverFollows(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	mk := func(p string) *album.Editor {
		e := edtest.Open(t, model.New(), p)
		edtest.NewSpread(e, -1, edtest.Photos("a"))
		if _, err := e.CreateCover("", nil); err != nil {
			t.Fatal(err)
		}
		return e
	}
	e, twin := mk(path), mk("")
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 250, 250
	twin.SetSettings(st)
	if _, err := e.SetSettingsIf(st, album.Expect{Rev: edtest.Rev(e)}); err != nil {
		t.Fatal(err)
	}
	if got, want := edtest.P(e).Cover.Spec, edtest.P(twin).Cover.Spec; !reflect.DeepEqual(got, want) {
		t.Fatalf("обложка %+v, как у SetSettings %+v", got, want)
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if q.Settings.PageW != 250 || q.Cover == nil || !reflect.DeepEqual(q.Cover.Spec, edtest.P(e).Cover.Spec) {
		t.Fatalf("после Save → Load: страница %v, обложка %+v", q.Settings.PageW, q.Cover)
	}
}

// TestApplyPrintIf: формат и изделие обложки одним шагом; неизвестное изделие — отказ без следа;
// отказ сверки не трогает; итог равен ApplyPrint на двойнике.
func TestApplyPrintIf(t *testing.T) {
	mk := func() *album.Editor {
		e := edtest.Open(t, model.New(), "")
		edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
		if _, err := e.CreateCover("", nil); err != nil {
			t.Fatal(err)
		}
		return e
	}
	e, twin := mk(), mk()
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 300, 200
	preset := "fabrika-30x20"
	if _, ok := model.CoverPresetOf(preset); !ok {
		preset = model.CoverPresets()[0].ID
	}
	if _, err := twin.ApplyPrint(st, preset); err != nil {
		t.Fatal(err)
	}
	undo := edtest.UndoLen(e)
	if _, err := e.ApplyPrintIf(st, preset, album.Expect{Rev: edtest.Rev(e)}); err != nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("ApplyPrintIf: шагов %d → %d, %v", undo, edtest.UndoLen(e), err)
	}
	if !reflect.DeepEqual(edtest.P(e).Cover.Spec, edtest.P(twin).Cover.Spec) || edtest.P(e).Settings != edtest.P(twin).Settings {
		t.Fatal("итог не равен ApplyPrint")
	}
	s := take(e)
	if v, err := e.ApplyPrintIf(st, preset, album.Expect{Rev: s.rev}); err != nil || v.Rev != s.rev {
		t.Fatalf("повтор: rev %d (был %d), %v", v.Rev, s.rev, err)
	}
	s.same(t, e, "повтор")
	if _, err := e.ApplyPrintIf(st, "нет-такого", album.Expect{Rev: s.rev}); err == nil {
		t.Fatal("неизвестное изделие принято")
	}
	s.same(t, e, "неизвестное изделие")
	ifRefuse(t, e, func(x album.Expect) error {
		st2 := st
		st2.PageW = 280
		_, err := e.ApplyPrintIf(st2, preset, x)
		return err
	})
}

// tileFamily — Family плитки действующей раскладки листа sp среди плиток LayoutChoices (ряд «Авто»,
// «Другие варианты»); плитки нет или у неё пусто — ошибка: проверять нечего.
func tileFamily(t *testing.T, e *album.Editor, sp model.SpreadView) string {
	t.Helper()
	c := e.LayoutChoices(sp.ID)
	for _, row := range [][]model.Choice{c.Whole.InOrder, c.Whole.Rearranged, c.Whole.Extra} {
		for _, tl := range row {
			if tl.Key == sp.Layout {
				if tl.Family == "" {
					t.Fatalf("лист %s: у плитки %q нет семейства", sp.ID, tl.Key)
				}
				return tl.Family
			}
		}
	}
	t.Fatalf("лист %s: плитки действующей раскладки %q нет в LayoutChoices", sp.ID, sp.Layout)
	return ""
}

// TestEditorFamilies: семейство — по действующей раскладке каждого листа, в порядке запроса;
// неизвестный ID, пустой лист и сетка — "", чтение не меняет проект, rev и шаги.
func TestEditorFamilies(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	edtest.NewSpread(e, -1, edtest.Photos("d"))
	empty := edtest.NewSpread(e, -1, nil).Spreads[2].ID
	v := e.View()
	ids := []string{v.Spreads[0].ID, v.Spreads[1].ID, empty, "нет-такого", ""}
	s := take(e)
	got, _ := e.Families(ids)
	if len(got) != len(ids) || got[0] == "" || got[1] == "" || got[2] != "" || got[3] != "" || got[4] != "" {
		t.Fatalf("семейства %q", got)
	}
	// Семейство — Family плитки действующей раскладки того же листа в галерее (LayoutChoices).
	for i := range 2 {
		if want := tileFamily(t, e, v.Spreads[i]); got[i] != want {
			t.Fatalf("лист %d: семейство %q, плитка галереи %q", i, got[i], want)
		}
	}
	if again, _ := e.Families(ids); !reflect.DeepEqual(again, got) {
		t.Fatalf("повтор дал другое: %q и %q", again, got)
	}
	s.same(t, e, "чтение Families")
	if f, _ := e.Families(nil); len(f) != 0 {
		t.Fatalf("пустой запрос: %d", len(f))
	}
	// Ручные места: сетка не имеет семейства.
	edtest.FreezeAt(t, e, 0)
	if g, _ := e.Families(ids[:1]); g[0] != "" {
		t.Fatalf("сетка: %q", g)
	}
}

// TestEditorFamiliesRev: ревизия берётся под тем же замком, что и чтение, — совпадает с rev вида;
// чтение её не двигает, правка и отмена двигают (по ней agentVary узнаёт, что человек успел
// поправить альбом между «до» и «после»). Несколько листов сразу; пустой запрос тоже отдаёт rev.
func TestEditorFamiliesRev(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	edtest.NewSpread(e, -1, edtest.Photos("d", "f"))
	v := e.View()
	ids := []string{v.Spreads[0].ID, v.Spreads[1].ID}
	fams, r0 := e.Families(ids)
	if r0 != v.Rev || r0 != edtest.Rev(e) || len(fams) != 2 {
		t.Fatalf("rev %d, вид %d, семейства %q", r0, v.Rev, fams)
	}
	if _, r := e.Families(nil); r != r0 {
		t.Fatalf("пустой запрос: rev %d, ждали %d", r, r0)
	}
	if _, r := e.Families(ids); r != r0 {
		t.Fatalf("чтение сдвинуло rev: %d → %d", r0, r)
	}
	e.SetNoAlbumBg(ids[0], true)
	if _, r1 := e.Families(ids); r1 != r0+1 {
		t.Fatalf("после правки rev %d, ждали %d", r1, r0+1)
	}
	e.SetNoAlbumBg(ids[0], true) // холостая: ни шага, ни rev
	if _, r := e.Families(ids); r != r0+1 {
		t.Fatalf("холостая сдвинула rev: %d", r)
	}
	e.Undo()
	if _, r2 := e.Families(ids); r2 != r0+2 {
		t.Fatalf("после отмены rev %d, ждали %d (отмена растёт, не откатывает)", r2, r0+2)
	}
}
