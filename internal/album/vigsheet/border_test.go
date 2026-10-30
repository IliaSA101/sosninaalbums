package vigsheet_test

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочки мест виньетки (079-15): переживают пересборку по ID места, новое место роли берёт
// копию рамочки первого места той же роли (079-23).

// roleBorders — рамочки мест роли role листа i по порядку сетки.
func roleBorders(e *album.Editor, i int, role string) []*model.Border {
	var out []*model.Border
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		if model.RoleOf(pl) == role {
			out = append(out, pl.Border)
		}
	}
	return out
}

// roleIDs — ID мест роли role листа i.
func roleIDs(e *album.Editor, i int, role string) []string {
	var out []string
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		if model.RoleOf(pl) == role {
			out = append(out, pl.ID)
		}
	}
	return out
}

// vigBorderEd — виньетка 1 учитель + 6 учеников, у учеников «Линия» 3 мм, у учителя «Полароид».
func vigBorderEd(t *testing.T) (*album.Editor, string, model.VigSetup) {
	t.Helper()
	e, sid := vigSheet(t, 10)
	st := model.VigSetup{Vig: model.Vignette{Tpl: "t-top-g", Teachers: 1, Pupils: 6, Ratio: "1:1"}}
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ApplyBorder(sid, roleIDs(e, 0, model.RolePupil), model.BorderSrc{Kind: "line", Width: 3}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ApplyBorder(sid, roleIDs(e, 0, model.RoleTeacher), model.BorderSrc{Kind: "polaroid"}); err != nil {
		t.Fatal(err)
	}
	return e, sid, st
}

// mustVigBorders — GridInvariants (с рамочками) и круг Save → Load: рамочки листа те же.
func mustVigBorders(t *testing.T, e *album.Editor) {
	t.Helper()
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	q := edtest.SaveLoad(t, e)
	for j, pl := range edtest.P(e).Spreads[0].Grid {
		if got := q.Spreads[0].Grid[j].Border; !reflect.DeepEqual(got, pl.Border) {
			t.Fatalf("после Save → Load место %s: %+v, было %+v", pl.ID, got, pl.Border)
		}
	}
}

// Больше учеников: прежние места — те же рамочки (указатель, тот же ID), новые — копии рамочки
// первого ученика; учитель свою держит; правка нового не задевает прежних; отмена — 6 мест.
func TestVignetteBorderNewRole(t *testing.T) {
	e, sid, st := vigBorderEd(t)
	before := roleBorders(e, 0, model.RolePupil)
	st.Vig.Pupils = 9
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	pb := roleBorders(e, 0, model.RolePupil)
	if len(pb) != 9 {
		t.Fatalf("учеников %d", len(pb))
	}
	for i, b := range pb {
		if b == nil || b.Kind != "line" || b.Width != 3 {
			t.Fatalf("ученик %d: %+v", i, b)
		}
		if i < len(before) && b != before[i] {
			t.Fatalf("ученик %d: рамочка не перенесена по ID", i)
		}
	}
	if tb := roleBorders(e, 0, model.RoleTeacher); len(tb) != 1 || tb[0] == nil || tb[0].Kind != "polaroid" {
		t.Fatalf("учитель: %+v", tb)
	}
	mustVigBorders(t, e)
	ids := roleIDs(e, 0, model.RolePupil)
	e.SetBorder(sid, ids[8:], model.BorderPatch{Width: 5}, "")
	if pb := roleBorders(e, 0, model.RolePupil); pb[0].Width != 3 || pb[7].Width != 3 || pb[8].Width != 5 {
		t.Fatalf("правка нового задела других: %v %v %v", pb[0].Width, pb[7].Width, pb[8].Width)
	}
	e.Undo()
	e.Undo()
	if n := len(roleBorders(e, 0, model.RolePupil)); n != 6 {
		t.Fatalf("отмена: учеников %d", n)
	}
}

// Пересборка со сменой формы (reshape) рамочки не снимает; у первого ученика рамочки нет —
// новые места без рамочки (наследуется рамочка первого, а не любого).
func TestVignetteBorderReshape(t *testing.T) {
	e, sid, st := vigBorderEd(t)
	st.Vig.Shape = model.ShapeOval
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	for i, b := range roleBorders(e, 0, model.RolePupil) {
		if b == nil || b.Kind != "line" {
			t.Fatalf("смена формы: ученик %d — %+v", i, b)
		}
	}
	mustVigBorders(t, e)
	if _, err := e.ApplyBorder(sid, roleIDs(e, 0, model.RolePupil)[:1], model.BorderSrc{}); err != nil {
		t.Fatal(err)
	}
	st.Vig.Pupils = 8
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	pb := roleBorders(e, 0, model.RolePupil)
	if len(pb) != 8 || pb[0] != nil || pb[1] == nil || pb[6] != nil || pb[7] != nil {
		t.Fatalf("наследование от первого без рамочки: %+v", pb)
	}
	mustVigBorders(t, e)
}

// borderAt — рамочка места id листа 0 (nil — нет места или рамочки).
func borderAt(e *album.Editor, id string) *model.Border {
	if _, pl := edtest.P(e).Spreads[0].FindPlace(id); pl != nil {
		return pl.Border
	}
	return nil
}

// Пересборка без новых мест (смена формата страницы, «Вернуть авто»): у двух учеников и учителя
// свои рамочки остаются по ID, сосед без рамочки её не получает; круг Save → Load; отмена
// возвращает прежнюю раскладку с рамочками.
func TestVignetteBorderKeptOnRelay(t *testing.T) {
	e, sid := vigSheet(t, 25)
	pupils, teachers := roleIDs(e, 0, model.RolePupil), roleIDs(e, 0, model.RoleTeacher)
	if len(pupils) < 4 || len(teachers) == 0 {
		t.Fatalf("учеников %d, учителей %d", len(pupils), len(teachers))
	}
	srcs := map[string]model.BorderSrc{
		pupils[1]:   {Kind: "line", Width: 2},
		pupils[3]:   {Kind: "polaroid"},
		teachers[0]: {Kind: "gold"},
	}
	want := map[string]*model.Border{}
	for id, src := range srcs {
		if _, err := e.ApplyBorder(sid, []string{id}, src); err != nil {
			t.Fatal(err)
		}
		if want[id] = borderAt(e, id).Clone(); want[id] == nil {
			t.Fatalf("рамочка места %s не встала", id)
		}
	}
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	e.SetSettings(st)
	if _, _, err := e.ResetVignette(sid); err != nil {
		t.Fatal(err)
	}
	for id, b := range want {
		if got := borderAt(e, id); !reflect.DeepEqual(got, b) {
			t.Fatalf("место %s: рамочка %+v, была %+v", id, got, b)
		}
	}
	if b := borderAt(e, pupils[0]); b != nil {
		t.Fatalf("у соседа без рамочки появилась %+v", b)
	}
	mustVigBorders(t, e)
	e.Undo()
	if got := borderAt(e, pupils[1]); !reflect.DeepEqual(got, want[pupils[1]]) {
		t.Fatalf("после отмены: %+v", got)
	}
}
