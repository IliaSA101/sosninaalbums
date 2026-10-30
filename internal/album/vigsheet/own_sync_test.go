package vigsheet_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// QA 038 (здесь: vigown_test.go у предела строк): имя своего шаблона — не правка альбома.
// SyncOwnNames ставит его обоим листам (общий Own) и всей истории без шага отмены: Ctrl+Z после
// «Применить» не возвращает старое имя, и удалённый шаблон виден под последним именем.
func TestOwnSyncNames(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl})
	if _, err := e.DuplicateGrid(sid, 1, true); err != nil {
		t.Fatal(err)
	}
	e.AddVignette(-1)
	e.Undo() // снимок с обоими листами — и в возврате
	renamed := tpl
	renamed.Name = "Другой <&>" // знаки, которые JSON экранирует: вес снимка считается по ним
	names := func(what string) {
		t.Helper()
		for _, s := range edtest.P(e).Spreads[:2] {
			if s.Vig.Own.Name != renamed.Name {
				t.Fatalf("%s: имя %q", what, s.Vig.Own.Name)
			}
		}
	}
	// Пустой список и шаблоны сеток — холостая.
	rev, undo, redo := edtest.Rev(e), edtest.UndoLen(e), edtest.RedoLen(e)
	for _, l := range [][]model.Template{nil, {{ID: "t1", Name: renamed.Name, Scope: model.TplSpread}}} {
		if _, ch := e.SyncOwnNames(l); ch || edtest.Rev(e) != rev {
			t.Fatalf("холостая %v: rev %d→%d", l, rev, edtest.Rev(e))
		}
	}
	edtest.SetDirty(e, false)
	if _, ch := e.SyncOwnNames([]model.Template{renamed}); !ch || edtest.Rev(e) != rev+1 || !edtest.Dirty(e) ||
		edtest.UndoLen(e) != undo || edtest.RedoLen(e) != redo {
		t.Fatalf("смена имени: %v, rev %d→%d, шагов %d/%d", ch, rev, edtest.Rev(e), edtest.UndoLen(e), edtest.RedoLen(e))
	}
	names("после смены")
	// Save → Load: у обоих листов в файле новое имя.
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for i, qs := range q.Spreads[:2] {
		if qs.Vig == nil || qs.Vig.Own == nil || qs.Vig.Own.Name != renamed.Name {
			t.Fatalf("Save → Load: лист %d, %+v", i, qs.Vig)
		}
	}
	if _, ch := e.SyncOwnNames([]model.Template{renamed}); ch || edtest.Rev(e) != rev+1 {
		t.Fatal("повтор не холостой")
	}
	sum := 0
	for _, s := range edtest.Undo(e) {
		sum += s.Weight()
	}
	for _, s := range append(edtest.Undo(e), edtest.Redo(e)...) {
		if s.P != nil && s.N != len(model.Marshal(s.P)) || sum != edtest.UndoBytes(e) {
			t.Fatalf("вес снимка %d, JSON %d; undoBytes %d, сумма %d", s.N, len(model.Marshal(s.P)), edtest.UndoBytes(e), sum)
		}
	}
	e.Redo()
	names("возврат")
	e.Undo()
	e.Undo() // до копии листа: имя — у единственного листа
	if n := edtest.P(e).Spreads[0].Vig.Own.Name; n != renamed.Name {
		t.Fatalf("отмена: имя %q", n)
	}
	if cs := e.VignetteChoices(sid, 1, 24, "3:4", nil); !cs[len(cs)-1].Gone || cs[len(cs)-1].Name != renamed.Name {
		t.Fatalf("удалённый шаблон: %+v", cs[len(cs)-1])
	}
}

// Отмена до начала: в проекте формы больше нет, имя меняется только в возврате — шаг
// «изменилась лишь история»: changed=false, rev и dirty прежние, имя в снимках новое.
func TestOwnSyncNamesHistoryOnly(t *testing.T) {
	e, sid, tpl := ownEd(t)
	e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl})
	if _, err := e.DuplicateGrid(sid, 1, true); err != nil {
		t.Fatal(err)
	}
	e.AddVignette(-1)
	for edtest.UndoLen(e) > 0 {
		e.Undo()
	}
	third := tpl
	third.Name = "Третий <&>"
	edtest.SetDirty(e, false)
	rev := edtest.Rev(e)
	if _, ch := e.SyncOwnNames([]model.Template{third}); ch || edtest.Rev(e) != rev || edtest.Dirty(e) {
		t.Fatalf("только история: changed %v, rev %d→%d, dirty %v", ch, rev, edtest.Rev(e), edtest.Dirty(e))
	}
	for _, s := range edtest.Redo(e) {
		if s.P != nil && s.N != len(model.Marshal(s.P)) {
			t.Fatalf("вес снимка возврата %d, JSON %d", s.N, len(model.Marshal(s.P)))
		}
	}
	for edtest.RedoLen(e) > 0 {
		e.Redo()
	}
	for _, s := range edtest.P(e).Spreads[:2] {
		if s.Vig.Own.Name != third.Name {
			t.Fatalf("имя в снимке возврата %q", s.Vig.Own.Name)
		}
	}
}

// Стыки виньетки со своим шаблоном и броска 042 (итоговое ревью 0.11).

// saveLoad — круг Save → Load проекта редактора.
func saveLoad(t *testing.T, e *album.Editor) *model.Project {
	t.Helper()
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	return q
}

// seats — места листа i: ID, роль, геометрия и файл фото (ID фото Load строит заново).
func seats(p *model.Project, i int) []string {
	var out []string
	for _, pl := range p.Spreads[i].Grid {
		out = append(out, fmt.Sprintf("%s %s %.4f %.4f %.4f %.4f %s", pl.ID, model.RoleOf(pl), pl.X, pl.Y, pl.W, pl.H, p.Photos[pl.Photo].Path))
	}
	return out
}

// Свой шаблон в точном режиме (тот же размер листа и числа) растёт учениками от броска: один шаг
// отмены байт в байт в обе стороны, копия шаблона на листе (имя, W/H) та же и переживает файл.
func TestGrowOwnExactUndoSaveLoad(t *testing.T) {
	e, sid, tpl := ownEd(t)
	if tpl.Vig.W <= 0 || tpl.Vig.H <= 0 {
		t.Fatalf("у шаблона нет размера листа: %+v", tpl.Vig.VigGeo)
	}
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Ratio: "3:4"}, []model.Template{tpl}); err != nil {
		t.Fatal(err)
	}
	wantBoxes(t, e, tpl) // точный режим: места ровно как в шаблоне
	own := edtest.P(e).Spreads[0].Vig.Own
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	_, out, err := e.DropPhotos(sid, "", classPhotos(27)[25:], -1, "")
	if err != nil || out.Grown != 2 || edtest.UndoLen(e) != undo+1 || manual(e, sid) {
		t.Fatalf("рост: %v, %+v, шагов %d", err, out, edtest.UndoLen(e)-undo)
	}
	sp := edtest.P(e).Spreads[0]
	if sp.Vig.Pupils != 26 || len(sp.Grid) != 27 || !sp.Vig.Own.Equal(own) || sp.Vig.Own.W != tpl.Vig.W || sp.Vig.Own.H != tpl.Vig.H {
		t.Fatalf("после роста: %+v, мест %d", sp.Vig, len(sp.Grid))
	}
	after := model.Marshal(edtest.P(e))
	if e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("Undo роста — не байт в байт")
	}
	if e.Redo(); !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("Redo роста — не байт в байт")
	}
	q := saveLoad(t, e)
	qs := q.Spreads[0]
	if qs.Vig == nil || qs.Vig.Tpl != "my:t1" || !qs.Vig.Own.Equal(own) || qs.Vig.Own.Name != "Свой" || qs.Vig.Pupils != 26 {
		t.Fatalf("Save → Load: %+v", qs.Vig)
	}
	if x, y := seats(q, 0), seats(edtest.P(e), 0); !slices.Equal(x, y) {
		t.Fatalf("Save → Load изменил места:\n%v\n%v", x, y)
	}
	if r := edtest.Open(t, q, ""); manual(r, sid) {
		t.Fatal("после Load выросший лист помечен ручным")
	}
}

// Плитка 042 на ручной виньетке, затем «Вернуть авто»: места плитки убраны, их фото — в ленте
// (left), остальные на своих местах; одна отмена возвращает плитку байт в байт.
func TestTileThenResetVignette(t *testing.T) {
	e, sid := vigSheet(t, 25)
	pl := *edtest.P(e).Spreads[0].Grid[0]
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	_, out, err := e.DropPhotos(sid, "", classPhotos(27)[25:], 0.5, "")
	if err != nil || len(out.Fresh) != 2 || out.Grown != 0 {
		t.Fatalf("плитка: %v, %+v", err, out)
	}
	pupils, teachers := rolePhotos(e, 0, model.RolePupil), rolePhotos(e, 0, model.RoleTeacher)
	tiled := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	_, left, err := e.ResetVignette(sid)
	sp := edtest.P(e).Spreads[0]
	if err != nil || left != 2 || len(sp.Grid) != 25 || manual(e, sid) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Вернуть авто»: %v, в ленту %d, мест %d", err, left, len(sp.Grid))
	}
	for _, id := range out.Fresh {
		if _, g := sp.FindPlace(id); g != nil {
			t.Fatalf("место плитки %s осталось", id)
		}
	}
	if got := rolePhotos(e, 0, model.RolePupil); !slices.Equal(got, pupils[:len(pupils)-2]) || !slices.Equal(rolePhotos(e, 0, model.RoleTeacher), teachers) {
		t.Fatalf("ученики после «Вернуть авто»: %v", got)
	}
	for _, id := range []string{"k25", "k26"} {
		if slices.Contains(sp.Photos, id) {
			t.Fatalf("фото плитки %s осталось на листе", id)
		}
	}
	if e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), tiled) || !manual(e, sid) {
		t.Fatal("отмена «Вернуть авто» не вернула плитку байт в байт")
	}
}

// Переименование своего шаблона: SyncOwnNames меняет имя и в истории, поэтому Undo и Redo
// другой правки его не возвращают, а Save → Load пишет в файл новое имя.
func TestOwnRenameUndoRedoSaveLoad(t *testing.T) {
	e, sid, tpl := ownEd(t)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24}, []model.Template{tpl}); err != nil {
		t.Fatal(err)
	}
	pl := lastPupil(e)
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X - 2, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	renamed := tpl
	renamed.Name = "Новое имя"
	undo := edtest.UndoLen(e)
	if _, ch := e.SyncOwnNames([]model.Template{renamed}); !ch || edtest.UndoLen(e) != undo {
		t.Fatalf("смена имени: changed %v, шагов %d", ch, edtest.UndoLen(e)-undo)
	}
	name := func(what string) {
		t.Helper()
		if n := edtest.P(e).Spreads[0].Vig.Own.Name; n != renamed.Name {
			t.Fatalf("%s: имя %q", what, n)
		}
	}
	e.Undo()
	name("отмена сдвига")
	if manual(e, sid) {
		t.Fatal("отмена не вернула места шаблона")
	}
	e.Redo()
	name("возврат сдвига")
	q := saveLoad(t, e)
	if qs := q.Spreads[0]; qs.Vig == nil || qs.Vig.Own == nil || qs.Vig.Own.Name != renamed.Name {
		t.Fatalf("Save → Load: %+v", qs.Vig)
	}
}
