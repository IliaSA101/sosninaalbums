package vigsheet_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Таблички мест виньетки (071) при пересборке: жилец места — фото и табличка вместе, пустое
// место с табличкой идёт в общей очереди роли, пустое без таблички сжимается.

// seatABC — таблички A…E (фамилия = буква), как в эталоне design §6.
func seatABC(n int) []model.Person {
	out := make([]model.Person, n)
	for i := range out {
		out[i] = model.Person{Last: string(rune('A' + i)), First: "Имя"}
	}
	return out
}

// dsc — фото без имени в названии файла: подпись берётся только из таблички.
func dsc(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: edtest.Abs("DSC_" + id + ".jpg"), W: 300, H: 400}
}

// pupilsOf — места учеников листа i по порядку сетки.
func pupilsOf(p *model.Project, i int) []*model.Place {
	var out []*model.Place
	for _, pl := range p.Spreads[i].Grid {
		if model.RoleOf(pl) == model.RolePupil {
			out = append(out, pl)
		}
	}
	return out
}

// pairs — «фото:табличка» мест учеников по порядку ("-" — пусто). Фото — по имени файла:
// Load переводит ID фото в ключ по пути (Rekey).
func pairs(p *model.Project, i int) []string {
	var out []string
	for _, pl := range pupilsOf(p, i) {
		ph := "-"
		if pl.Photo != "" {
			ph = strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p.Photos[pl.Photo].Path), "DSC_"), ".jpg")
		}
		out = append(out, ph+":"+cmpOr(pl.Person.Last, "-"))
	}
	return out
}

func cmpOr(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

func wantPairs(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: места %v, ждали %v", what, got, want)
	}
}

// seatVig — эталон design §6: виньетка из 5 учеников без учителя, таблички A–E, фото на
// местах 1, 3, 5 (p1, p3, p5).
func seatVig(t *testing.T) (*album.Editor, string) {
	t.Helper()
	e, sid := vigSheet(t, 0)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "grid", Pupils: 5}, nil); err != nil {
		t.Fatal(err)
	}
	names := seatABC(5)
	edtest.Change(e, func(p *model.Project) {
		for i, pl := range pupilsOf(p, 0) {
			pl.Person = names[i]
		}
	})
	pls := pupilsOf(edtest.P(e), 0)
	for i, id := range []string{"p1", "", "p3", "", "p5"} {
		if id != "" {
			if _, _, _, err := e.PlacePhotos(sid, pls[i].ID, []model.PhotoInfo{dsc(id)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	wantPairs(t, "начало", pairs(edtest.P(e), 0), "p1:A", "-:B", "p3:C", "-:D", "p5:E")
	return e, sid
}

// boxesOf — рамки мест листа (пересборка должна их сдвинуть, иначе проверка ни о чём).
func boxesOf(e *album.Editor) []string {
	var out []string
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		out = append(out, fmt.Sprintf("%.2f %.2f %.2f %.2f", pl.X, pl.Y, pl.W, pl.H))
	}
	return out
}

func TestVignetteRelayCarriesSeatNames(t *testing.T) {
	want := []string{"p1:A", "-:B", "p3:C", "-:D", "p5:E"}
	// Relay при смене кегля подписей: места пересобраны, пары «фото — имя» те же.
	e, sid := seatVig(t)
	was, undo := boxesOf(e), edtest.UndoLen(e)
	caps := *edtest.P(e).Spreads[0].Caps
	caps.Style.Size += 4
	e.SetSheetCaps(sid, true, caps)
	if slices.Equal(boxesOf(e), was) {
		t.Fatal("пересборки не было: рамки те же")
	}
	wantPairs(t, "Relay (подписи)", pairs(edtest.P(e), 0), want...)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("пересборка: шагов %d", edtest.UndoLen(e)-undo)
	}
	// Relay при смене размеров альбома — тоже.
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 404
	was = boxesOf(e)
	e.SetSettings(st)
	if slices.Equal(boxesOf(e), was) {
		t.Fatal("пересборки по формату не было")
	}
	wantPairs(t, "Relay (формат)", pairs(edtest.P(e), 0), want...)
	// Save → Load: таблички и фото на своих местах.
	wantPairs(t, "Save → Load", pairs(edtest.SaveLoad(t, e), 0), want...)

	// Смена шаблона «Настроить виньетку»: те же пары; холостой повтор — без шага.
	e, sid = seatVig(t)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "t-left", Pupils: 5}, nil); err != nil {
		t.Fatal(err)
	}
	wantPairs(t, "шаблон t-left", pairs(edtest.P(e), 0), want...)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetVignette(sid, vigOf(e, 0), nil)
	edtest.Same(t, e, rev, undo, "повтор «Применить» с табличками")

	// Мест меньше: лишняя табличка D и фото p5 с табличкой E уходят; Ctrl+Z возвращает всё.
	before := model.Marshal(edtest.P(e))
	// Жильцы p1:A, B, p3:C, D, p5:E → три первых; p5 уходит с листа.
	if _, left, err := e.SetVignette(sid, model.Vignette{Tpl: "t-left", Pupils: 3}, nil); err != nil || left != 1 {
		t.Fatalf("меньше мест: %v, ушло %d", err, left)
	}
	wantPairs(t, "3 места", pairs(edtest.P(e), 0), "p1:A", "-:B", "p3:C")
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена уменьшения — не байт в байт")
	}
}

func TestResetGrowVignetteSeatNames(t *testing.T) {
	// «Вернуть авто» виньетки: места по плану, пары те же.
	e, sid := seatVig(t)
	pl := *pupilsOf(edtest.P(e), 0)[1]
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	moved := boxesOf(e)
	if _, _, err := e.ResetVignette(sid); err != nil {
		t.Fatal(err)
	}
	if slices.Equal(boxesOf(e), moved) {
		t.Fatal("«Вернуть авто» не вернул место")
	}
	wantPairs(t, "«Вернуть авто»", pairs(edtest.P(e), 0), "p1:A", "-:B", "p3:C", "-:D", "p5:E")

	// «+ ученики»: 4 фото на лист — два в пустые места с B и D, два — новыми учениками в конце.
	e, sid = seatVig(t)
	undo := edtest.UndoLen(e)
	_, out, err := e.DropPhotos(sid, "", []model.PhotoInfo{dsc("q1"), dsc("q2"), dsc("q3"), dsc("q4")}, -1, "add")
	if err != nil || out.Grown != 2 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("рост: %v, %+v, шагов %d", err, out, edtest.UndoLen(e)-undo)
	}
	got := pairs(edtest.P(e), 0)
	wantPairs(t, "рост", got, "p1:A", "q1:B", "p3:C", "q2:D", "p5:E", "q3:-", "q4:-")
	wantPairs(t, "рост: Save → Load", pairs(edtest.SaveLoad(t, e), 0), got...)
}

func TestSortVignetteSeatNames(t *testing.T) {
	e, sid := vigSheet(t, 0)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "t-left", Teachers: 1, Pupils: 5}, nil); err != nil {
		t.Fatal(err)
	}
	// Ученики: s0 + табличка «Яковлев», пустое + «Абрамов», s2 со своим «Ежов», пустое без
	// таблички, s4 без имени; учитель t со своим «Ааа».
	pls := pupilsOf(edtest.P(e), 0)
	for i, id := range map[int]string{0: "s0", 2: "s2", 4: "s4"} {
		e.PlacePhotos(sid, pls[i].ID, []model.PhotoInfo{dsc(id)})
	}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if model.RoleOf(pl) == model.RoleTeacher {
			e.PlacePhotos(sid, pl.ID, []model.PhotoInfo{dsc("t")})
		}
	}
	e.SetPerson("s2", model.Person{Last: "Ежов", First: "Анна"}, "")
	e.SetPerson("t", model.Person{Last: "Ааа"}, "")
	edtest.Change(e, func(p *model.Project) {
		g := pupilsOf(p, 0)
		g[0].Person = model.Person{Last: "Яковлев", First: "Иван"}
		g[1].Person = model.Person{Last: "Абрамов", First: "Борис"}
	})
	wantPairs(t, "начало", pairs(edtest.P(e), 0), "s0:Яковлев", "-:Абрамов", "s2:-", "-:-", "s4:-")
	undo := edtest.UndoLen(e)
	e.SortVignette(sid)
	// Абрамов (пустое с табличкой), Ежов, Яковлев с фото s0; без имени — s4 в конце; пустое без
	// таблички (4-е) стоит.
	wantPairs(t, "сортировка", pairs(edtest.P(e), 0), "-:Абрамов", "s2:-", "s0:Яковлев", "-:-", "s4:-")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("сортировка: шагов %d", edtest.UndoLen(e)-undo)
	}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if model.RoleOf(pl) == model.RoleTeacher && pl.Photo != "t" {
			t.Fatalf("учитель сдвинулся: %q", pl.Photo)
		}
	}
	rev := edtest.Rev(e)
	e.SortVignette(sid)
	edtest.Same(t, e, rev, undo+1, "повторная сортировка")
	wantPairs(t, "Save → Load", pairs(edtest.SaveLoad(t, e), 0), "-:Абрамов", "s2:-", "s0:Яковлев", "-:-", "s4:-")
	e.Undo()
	wantPairs(t, "отмена", pairs(edtest.P(e), 0), "s0:Яковлев", "-:Абрамов", "s2:-", "-:-", "s4:-")
}

// Шаблоны — без табличек: свой шаблон виньетки, сетка «Сохранить как шаблон» (PlaceBoxes)
// и готовый разворот (SpreadTplOf) не несут имён людей (design §3).
func TestTemplatesNoSeatNames(t *testing.T) {
	e, sid := seatVig(t)
	check := func(what string, v any) {
		t.Helper()
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if s := string(data); strings.Contains(s, `"Имя"`) || strings.Contains(s, "person") {
			t.Fatalf("%s несёт табличку: %s", what, s)
		}
	}
	tpl, err := e.VigTemplate(sid)
	if err != nil {
		t.Fatal(err)
	}
	check("свой шаблон виньетки", tpl)
	boxes, _, err := e.PlaceBoxes(sid, model.ScopeSpread)
	if err != nil || len(boxes) != len(edtest.P(e).Spreads[0].Grid) {
		t.Fatalf("сетка в шаблон: %v, %d", err, len(boxes))
	}
	check("сетка в шаблон", boxes)
	for _, texts := range []bool{false, true} {
		st, err := e.SpreadTplOf(sid, model.ScopeSpread, true, texts)
		if err != nil || len(st.Places) == 0 {
			t.Fatalf("готовый разворот: %v, мест %d", err, len(st.Places))
		}
		check("готовый разворот", st)
	}
}
