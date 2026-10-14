package grid_test

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

// Обложка — лист вне разворотов (032 §1): правки мест и текстов находят её через Sheet и
// держат места в её листе; бросок, обмен, автосборка и «Заполнить» соблюдают «одно фото —
// одно место» на весь проект; инварианты — по всем листам.

func TestCoverSheetPlaces(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(20, 20, 100, 80, "a")}, []string{"b", "c"})
	cv := func() *model.Spread { return edtest.P(e).Cover.Sheet }
	// Перенос к правому краю: зажим по листу обложки (два разворота, корешок 6 мм: 615 мм),
	// а не разворота (562).
	if w := edtest.P(e).Geo(cv()).W; w != 615 {
		t.Fatalf("ширина обложки %v", w)
	}
	e.SetPlaces(model.CoverID, []model.PlaceBox{{ID: edtest.CoverPlace, X: 600, Y: 100, W: 120, H: 80}}, "")
	if _, pl := cv().FindPlace(edtest.CoverPlace); pl == nil || pl.X != 615-120 {
		t.Fatalf("место обложки после переноса: %+v", pl)
	}
	// Новое место — на лицевой (страница 1 листа обложки — сторонка с заворотом).
	if _, err := e.AddPlace(model.CoverID, 1); err != nil || len(cv().Grid) != 2 {
		t.Fatalf("AddPlace на обложку: %v, мест %d", err, len(cv().Grid))
	}
	if pl, front := cv().Grid[1], edtest.P(e).BdArea(cv(), model.ScopeRight); pl.X+pl.W/2 < front.X || pl.X+pl.W/2 > front.X+front.W {
		t.Fatalf("новое место не на лицевой: %+v", pl)
	}
	// Текст — по центру задней.
	if _, err := e.AddText(model.CoverID, 0, ""); err != nil || len(cv().Texts) != 2 {
		t.Fatalf("AddText на обложку: %v", err)
	}
	if tb := cv().Texts[1]; tb.X+tb.W/2 < 0 || tb.X+tb.W/2 > 304.5 {
		t.Fatalf("текст не на задней: %+v", tb)
	}
	e.DeleteText(cv().Texts[1].ID)
	if len(cv().Texts) != 1 {
		t.Fatal("DeleteText не нашёл текст обложки")
	}
	edtest.MustGrid(t, e)
}

func TestCoverSheetOnePhotoOnePlace(t *testing.T) {
	e := edtest.CoverEd(t, "k", []*model.Place{edtest.Gp(20, 20, 100, 80, "a")}, []string{"b", "c"})
	cover := func() string { _, pl := edtest.P(e).Cover.Sheet.FindPlace(edtest.CoverPlace); return pl.Photo }
	// Фото разворота на занятое место обложки — обмен (как между разворотами).
	if _, _, err := e.PlacePhotos(model.CoverID, edtest.CoverPlace, edtest.Photos("b")); err != nil || cover() != "b" {
		t.Fatalf("бросок на место обложки: %v, место %q", err, cover())
	}
	if got := edtest.P(e).Spreads[1].Photos; !slices.Equal(got, []string{"k", "c"}) {
		t.Fatalf("обмен с обложкой: разворот %v", got)
	}
	e.SwapPhotos("b", "c")
	if cover() != "c" || !slices.Equal(edtest.P(e).Spreads[1].Photos, []string{"k", "b"}) {
		t.Fatalf("SwapPhotos через обложку: %q %v", cover(), edtest.P(e).Spreads[1].Photos)
	}
	// Автосборка и «Заполнить» не берут фото с места обложки.
	if _, err := e.AutoBuild(edtest.Photos("c", "x", "y"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64}); err != nil {
		t.Fatal(err)
	}
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].Photos
	if cover() != "c" || slices.Contains(last, "c") || !slices.Equal(last, []string{"x", "y"}) {
		t.Fatalf("автосборка: обложка %q, новый разворот %v", cover(), last)
	}
	if _, left := e.FillPlaces(edtest.Photos("c"), ""); left != 0 || cover() != "c" {
		t.Fatalf("«Заполнить» забрал фото обложки: осталось %d, место %q", left, cover())
	}
	// Фото в развороте уходит с места обложки (место пустеет, остаётся).
	if _, err := e.AddPhotos(edtest.P(e).Spreads[1].ID, edtest.Photos("c"), -1); err != nil || cover() != "" {
		t.Fatalf("AddPhotos не снял фото с обложки: %v, место %q", err, cover())
	}
	edtest.MustGrid(t, e)
}

// Операции над порядком альбома и «Авто» обложку не находят (Find): холостая правка.
func TestCoverSheetRejectsSpreadOps(t *testing.T) {
	e := edtest.CoverEd(t, "k", nil, []string{"b", "c"})
	rev, undo, before := edtest.Rev(e), edtest.UndoLen(e), string(model.Marshal(edtest.P(e)))
	_, _ = e.AddPhotos(model.CoverID, edtest.Photos("b"), -1)
	e.SetLayout(model.CoverID, "H(0,1)")
	e.SplitSpread(model.CoverID, 1)
	e.MergeSpreads(model.CoverID)
	e.MoveSpread(model.CoverID, 1)
	e.DeleteSpread(model.CoverID)
	e.SetSpreadName(model.CoverID, "Обложка")
	e.SetGrid(model.CoverID, false)
	_, _, _ = e.ApplyGrid(model.CoverID, model.ScopeSpread, model.TplSpread, []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}})
	edtest.Same(t, e, rev, undo, "операции разворотов на обложке")
	if string(model.Marshal(edtest.P(e))) != before {
		t.Fatal("проект изменился")
	}
}

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
