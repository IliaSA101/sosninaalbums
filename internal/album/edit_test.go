package album_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ядро правки: холостая правка, удаление пустого разворота, keepLayouts, Restore, журнал
// медленных правок. Спека 013: пустой разворот, созданный щелчком, живёт, пока правка не
// унесёт с него фото.

// emptyOnEnd — [a b] [c] X: X создан пустым в конце.
func emptyOnEnd(t *testing.T) (e *album.Editor, s1, s2, x string) {
	t.Helper()
	e, s1, s2 = edtest.NewTwo(t)
	v := edtest.NewSpread(e, -1, nil)
	if len(v.Spreads) != 3 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	return e, s1, s2, v.Spreads[2].ID
}

func TestNewSpreadEmpty(t *testing.T) {
	for _, tc := range []struct{ at, pos int }{{-1, 2}, {0, 0}} {
		e, s1, s2 := edtest.NewTwo(t)
		before := e.View()
		v := edtest.NewSpread(e, tc.at, nil)
		if len(v.Spreads) != 3 || v.Rev != before.Rev+1 || !v.Dirty || !v.CanUndo {
			t.Fatalf("at=%d: разворотов %d, rev %d→%d, dirty %v", tc.at, len(v.Spreads), before.Rev, v.Rev, v.Dirty)
		}
		x := v.Spreads[tc.pos]
		if x.ID == s1 || x.ID == s2 || len(x.Photos) != 0 || len(x.Rects) != 0 || x.Places != nil {
			t.Fatalf("at=%d: новый разворот %+v", tc.at, x)
		}
		want := edtest.SpreadIDs(v)
		if got := edtest.SpreadIDs(e.Undo()); !slices.Equal(got, []string{s1, s2}) {
			t.Fatalf("at=%d: после Undo %v", tc.at, got)
		}
		if got := edtest.SpreadIDs(e.Redo()); !slices.Equal(got, want) {
			t.Fatalf("at=%d: после Redo %v, ждали %v", tc.at, got, want)
		}
	}
}

func TestEmptySpreadSurvivesEdits(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, e *album.Editor, s1, x string)
	}{
		{"RemovePhotos другого", func(_ *testing.T, e *album.Editor, _, _ string) { e.RemovePhotos([]string{"c"}) }},
		{"AddText и DeleteText", func(t *testing.T, e *album.Editor, _, x string) {
			v, err := e.AddText(x, 0, "")
			if err != nil {
				t.Fatal(err)
			}
			e.DeleteText(v.Spreads[2].Texts[0].ID)
		}},
		{"название и сброс", func(_ *testing.T, e *album.Editor, _, x string) {
			e.SetSpreadName(x, "Вступление")
			e.SetSpreadName(x, "")
		}},
		{"AddPlace и DeletePlaces", func(t *testing.T, e *album.Editor, _, x string) {
			v, err := e.AddPlace(x, 0, "")
			if err != nil {
				t.Fatal(err)
			}
			e.DeletePlaces(x, []string{v.Spreads[2].Places[0].ID})
		}},
		{"AddPlace и SetGrid(false)", func(t *testing.T, e *album.Editor, _, x string) {
			if _, err := e.AddPlace(x, 1, ""); err != nil {
				t.Fatal(err)
			}
			e.SetGrid(x, false)
		}},
		{"MoveSpread", func(_ *testing.T, e *album.Editor, _, x string) { e.MoveSpread(x, 0) }},
		{"SwapPhotos", func(_ *testing.T, e *album.Editor, _, _ string) { e.SwapPhotos("a", "c") }},
		{"SetSettings", func(_ *testing.T, e *album.Editor, _, _ string) {
			s := e.View().Settings
			s.Bg = "#202020"
			e.SetSettings(s)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, s1, _, x := emptyOnEnd(t)
			tc.edit(t, e, s1, x)
			i, s := edtest.P(e).Find(x)
			if i < 0 {
				t.Fatalf("пустой разворот удалён: %v", edtest.SpreadIDs(e.View()))
			}
			if !s.Empty() {
				t.Fatalf("разворот не пустой: %+v", s)
			}
		})
	}
}

func TestEmptySpreadGoesWithLastPhoto(t *testing.T) {
	t.Run("бросок и RemovePhotos", func(t *testing.T) {
		e, _, _, x := emptyOnEnd(t)
		if _, s := edtest.P(e).Find(x); s == nil || !slices.Equal(edtest.AddPhotos(e, x, edtest.Photos("x"), -1).Spreads[2].Photos, []string{"x"}) {
			t.Fatal("бросок не попал в пустой разворот")
		}
		e.RemovePhotos([]string{"x"})
		if i, _ := edtest.P(e).Find(x); i >= 0 {
			t.Fatal("разворот остался после ухода последнего фото")
		}
	})
	t.Run("бросок и MovePhoto в другой", func(t *testing.T) {
		e, s1, _, x := emptyOnEnd(t)
		edtest.AddPhotos(e, x, edtest.Photos("x"), -1)
		if _, err := e.MovePhoto("x", s1, -1); err != nil {
			t.Fatal(err)
		}
		if i, _ := edtest.P(e).Find(x); i >= 0 {
			t.Fatal("разворот остался после переноса последнего фото")
		}
	})
	t.Run("с текстом живёт", func(t *testing.T) {
		e, _, _, x := emptyOnEnd(t)
		edtest.AddPhotos(e, x, edtest.Photos("x"), -1)
		if _, err := e.AddText(x, 0, ""); err != nil {
			t.Fatal(err)
		}
		e.RemovePhotos([]string{"x"})
		if i, _ := edtest.P(e).Find(x); i < 0 {
			t.Fatal("разворот с текстом удалён")
		}
	})
}

func TestLoadKeepsEmptySpread(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	for _, n := range []string{"a", "b"} {
		path := filepath.Join(dir, n+".jpg")
		id := library.IDFor(path)
		p.Photos[id] = model.PhotoRef{Path: path, W: 3, H: 2}
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + n, Photos: []string{id}})
	}
	// [a] E1 [b] E2: пустой в середине и в конце.
	p.Spreads = slices.Insert(p.Spreads, 1, &model.Spread{ID: "e1"})
	p.Spreads = append(p.Spreads, &model.Spread{ID: "e2"})
	want := []string{"sa", "e1", "sb", "e2"}

	data, err := salbum.Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"null", "[]", "нет ключа"} {
		for _, s := range raw["spreads"].([]any) {
			sp := s.(map[string]any)
			if id := sp["id"]; id != "e1" && id != "e2" {
				continue
			}
			switch form {
			case "null":
				sp["photos"] = nil
			case "[]":
				sp["photos"] = []any{}
			default:
				delete(sp, "photos")
			}
		}
		in, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		q := edtest.MustDecode(t, in, dir)
		if got := edtest.SpreadIDs(edtest.Open(t, q, "").View()); !slices.Equal(got, want) {
			t.Fatalf("%s: развороты после Load %v, ждали %v", form, got, want)
		}
		again, err := salbum.Encode(q, dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := edtest.SpreadIDs(edtest.Open(t, edtest.MustDecode(t, again, dir), "").View()); !slices.Equal(got, want) {
			t.Fatalf("%s: второй круг %v", form, got)
		}
	}
}

// Автосборка в середину (at до пустого хвоста) пустой разворот не заполняет и не удаляет;
// в конец — занимает его вместо нового (035 U1, подробно — spreads/blanktail_test.go).
func TestAutoBuildKeepsEmptySpread(t *testing.T) {
	for _, tc := range []struct {
		at   int
		keep bool
	}{{1, true}, {3, false}} {
		e, _, _, x := emptyOnEnd(t)
		v, err := e.AutoBuild(edtest.Photos("a", "b", "c", "d"), tc.at, model.AutoOpts{Spreads: 2, MaxPer: 64})
		if err != nil {
			t.Fatal(err)
		}
		ids := edtest.SpreadIDs(v)
		if tc.keep && (len(ids) != 3 || ids[2] != x || len(v.Spreads[2].Photos) != 0) ||
			!tc.keep && (len(ids) != 2 || slices.Contains(ids, x)) {
			t.Fatalf("at=%d: развороты после автосборки %v, пустой %s", tc.at, ids, x)
		}
		for _, s := range v.Spreads[:2] {
			if len(s.Photos) == 0 {
				t.Fatalf("at=%d: собранный разворот без фото: %v", tc.at, ids)
			}
		}
	}
}

// Restore возвращает проект как был, без поправок относительно текущего (тексты не
// пересчитываются под нынешний формат страницы); совпало — холостая.
func TestRestoreWhole(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	e.AddText(v.Spreads[0].ID, 0, "s")
	q := edtest.P(e).Clone()
	want := model.Marshal(q)

	s := edtest.P(e).Settings
	s.PageW, s.PageH = s.PageW*1.5, s.PageH*1.2
	e.SetSettings(s) // тексты пересчитаны под новый формат
	e.DeleteSpread(v.Spreads[0].ID)

	rev, undo := e.View().Rev, edtest.UndoLen(e)
	if _, err := e.Restore(q); err != nil {
		t.Fatal(err)
	}
	if got := model.Marshal(edtest.P(e)); !bytes.Equal(got, want) {
		t.Fatalf("восстановлено не как было:\n%s\nждали\n%s", got, want)
	}
	if e.View().Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatal("восстановление — не один шаг отмены")
	}
	if _, err := e.Restore(q); err != nil || e.View().Rev != rev+1 {
		t.Fatal("повтор того же — не холостой")
	}
	e.Undo()
	if len(edtest.P(e).Spreads) != 0 {
		t.Fatal("отмена не вернула альбом до восстановления")
	}
}

// normalize: разворот, из которого убрали все фото, остаётся с названием или фоном.
func TestNormalizeKeepsNamedOrColoredSpread(t *testing.T) {
	for name, set := range map[string]func(e *album.Editor, id string){
		"название": func(e *album.Editor, id string) { e.SetSpreadName(id, "Вступление") },
		"фон":      func(e *album.Editor, id string) { e.SetBackground(id, "left", "#112233") },
	} {
		e, s1, _ := edtest.NewTwo(t)
		set(e, s1)
		v := e.RemovePhotos([]string{"a", "b"})
		if len(v.Spreads) != 2 || v.Spreads[0].ID != s1 || len(v.Spreads[0].Photos) != 0 {
			t.Fatalf("%s: разворот удалён: %+v", name, v.Spreads)
		}
	}
}

// captureLog перехватывает log на время теста.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// TestSlowLog — строки G5 и правки (075-6) появляются дольше порога и не появляются ниже него;
// в строке только счётчики, без путей.
func TestSlowLog(t *testing.T) {
	old, oldEdit := edtest.SlowOp(), edtest.SlowEdit()
	defer func() { edtest.SetSlowOp(old); edtest.SetSlowEdit(oldEdit) }()
	photos := make([]model.PhotoInfo, 6)
	for i := range photos {
		photos[i] = model.PhotoInfo{ID: fmt.Sprintf("p%d", i), Path: fmt.Sprintf("/секрет/p%d.jpg", i), W: 300, H: 200}
	}
	run := func(e *album.Editor) {
		if _, err := e.AutoBuild(photos, -1, model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil {
			t.Fatal(err)
		}
		st := e.View().Settings
		st.Gap++
		e.SetSettings(st)
		e.View()
	}

	// Порог 0: любая операция «медленная».
	edtest.SetSlowOp(0)
	edtest.SetSlowEdit(0)
	buf := captureLog(t)
	run(edtest.Open(t, &model.Project{Settings: model.DefaultSettings(), Photos: map[string]model.PhotoRef{}}, ""))
	out := buf.String()
	for _, want := range []string{"автосборка: 6 фото → 2 разворотов за ", "настройки: 2 разворотов за ", "View: 2 разворотов за ",
		"правка: 2 разворотов за "} {
		if !strings.Contains(out, want) {
			t.Errorf("нет строки %q в логе: %q", want, out)
		}
	}
	if strings.Contains(out, "секрет") || strings.Contains(out, ".jpg") {
		t.Errorf("в логе путь: %q", out)
	}

	// Порог правки свой: при часе у правки и нуле у операций строки «правка» нет, прочие есть.
	edtest.SetSlowEdit(time.Hour)
	buf.Reset()
	run(edtest.Open(t, &model.Project{Settings: model.DefaultSettings(), Photos: map[string]model.PhotoRef{}}, ""))
	if out := buf.String(); strings.Contains(out, "правка:") || !strings.Contains(out, "View: ") {
		t.Errorf("порог правки не свой: %q", out)
	}

	// Порог в час: ничего не пишется.
	edtest.SetSlowOp(time.Hour)
	edtest.SetSlowEdit(time.Hour)
	buf.Reset()
	run(edtest.Open(t, &model.Project{Settings: model.DefaultSettings(), Photos: map[string]model.PhotoRef{}}, ""))
	if buf.Len() != 0 {
		t.Errorf("ниже порога в логе: %q", buf.String())
	}
	// Пороги выпуска: правка — от 100 мс (цель 50 мс, 075-6), ниже операций.
	if oldEdit != 100*time.Millisecond || oldEdit >= old {
		t.Errorf("пороги: правка %v, операция %v", oldEdit, old)
	}
}

func TestNoopAddPhotos(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	e.SetLayout(s1, edtest.PickKey(t, e, 0, edtest.AnyKey))
	before := e.View()
	undo := edtest.UndoLen(e)

	// Дроп фото b на его же место (после себя и на себя).
	for _, idx := range []int{1, 2} {
		v := edtest.AddPhotos(e, s1, []model.PhotoInfo{edtest.Photo("b")}, idx)
		if edtest.UndoLen(e) != undo {
			t.Fatalf("холостая правка попала в отмену (index %d)", idx)
		}
		if v.Rev != before.Rev {
			t.Fatalf("rev вырос от холостой правки")
		}
		if v.Spreads[0].Layout != before.Spreads[0].Layout {
			t.Fatalf("раскладка сброшена холостой правкой: %s", v.Spreads[0].Layout)
		}
	}
}

func TestEmptySpreadRemoved(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	rev := e.View().Rev
	v := edtest.AddPhotos(e, s1, []model.PhotoInfo{edtest.Photo("c")}, -1)
	if len(v.Spreads) != 1 || v.Spreads[0].ID != s1 || len(v.Spreads[0].Photos) != 3 {
		t.Fatalf("пустой разворот не удалён: %+v", v.Spreads)
	}
	if v.Rev != rev+1 {
		t.Fatalf("rev = %d, ждали %d", v.Rev, rev+1)
	}
	v = e.Undo()
	if len(v.Spreads) != 2 || v.Spreads[1].ID != s2 || !slices.Equal(v.Spreads[1].Photos, []string{"c"}) {
		t.Fatalf("отмена не вернула разворот: %+v", v.Spreads)
	}
	v = e.Redo()
	if len(v.Spreads) != 1 {
		t.Fatalf("возврат не повторил правку: %+v", v.Spreads)
	}
}

func TestLayoutKeptOnReorder(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	id := v.Spreads[0].ID
	key := edtest.PickKey(t, e, 0, edtest.AnyKey)
	old := e.SetLayout(id, key)
	if old.Spreads[0].Layout != key {
		t.Fatalf("раскладка не поставлена: %s", old.Spreads[0].Layout)
	}
	v = e.SwapPhotos("a", "c")
	if v.Spreads[0].Layout != key {
		t.Fatalf("после обмена ключ %s, ждали %s", v.Spreads[0].Layout, key)
	}
	if edtest.RectOf(v, 0, "c") != edtest.RectOf(old, 0, "a") || edtest.RectOf(v, 0, "a") != edtest.RectOf(old, 0, "c") {
		t.Fatalf("прямоугольники не поменялись местами")
	}
	if edtest.RectOf(v, 0, "b") != edtest.RectOf(old, 0, "b") {
		t.Fatalf("фото b сдвинулось")
	}
}

func TestLayoutKeptOnSwapAcrossSpreads(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	v := edtest.NewSpread(e, -1, edtest.Photos("d", "e"))
	k1, k2 := edtest.PickKey(t, e, 0, edtest.AnyKey), edtest.PickKey(t, e, 1, edtest.AnyKey)
	e.SetLayout(v.Spreads[0].ID, k1)
	e.SetLayout(v.Spreads[1].ID, k2)
	v = e.SwapPhotos("b", "d")
	if v.Spreads[0].Layout != k1 || v.Spreads[1].Layout != k2 {
		t.Fatalf("ключи %s, %s, ждали %s, %s", v.Spreads[0].Layout, v.Spreads[1].Layout, k1, k2)
	}
	if !slices.Equal(v.Spreads[0].Photos, []string{"a", "d", "c"}) {
		t.Fatalf("обмен не прошёл: %v", v.Spreads[0].Photos)
	}
}

func TestLayoutResetWhenInvalid(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	id := v.Spreads[0].ID
	e.SetLayout(id, edtest.PickKey(t, e, 0, edtest.AnyKey))
	v = edtest.AddPhotos(e, id, edtest.Photos("x"), -1)
	if edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("после добавления фото Layout = %q", edtest.P(e).Spreads[0].Layout)
	}
	if best := edtest.RowsOf(e, 0)[0].Key; v.Spreads[0].Layout != best {
		t.Fatalf("раскладка %s, ждали лучшую %s", v.Spreads[0].Layout, best)
	}

	// Блок на весь разворот без layflat не строится.
	e = edtest.Open(t, model.New(), "")
	v = edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	id = v.Spreads[0].ID
	whole := func(k string) bool { return !strings.HasPrefix(k, "P[") && !strings.HasPrefix(k, "B[") }
	key := edtest.PickKey(t, e, 0, whole)
	e.SetLayout(id, key)
	if edtest.P(e).Spreads[0].Layout != key {
		t.Fatalf("раскладка не поставлена")
	}
	st := edtest.P(e).Settings
	st.Layflat = false
	v = e.SetSettings(st)
	if edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("без layflat Layout = %q", edtest.P(e).Spreads[0].Layout)
	}
	if strings.HasPrefix(v.Spreads[0].Layout, "H(") || strings.HasPrefix(v.Spreads[0].Layout, "V(") {
		t.Fatalf("без layflat стоит блок на весь разворот %s", v.Spreads[0].Layout)
	}
}

func TestLayoutKeptOnSettings(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c"))
	key := edtest.PickKey(t, e, 0, func(k string) bool { return strings.HasPrefix(k, "P[") })
	e.SetLayout(v.Spreads[0].ID, key)
	st := edtest.P(e).Settings
	st.Gap = 5
	v = e.SetSettings(st)
	if v.Spreads[0].Layout != key {
		t.Fatalf("после смены промежутка ключ %s, ждали %s", v.Spreads[0].Layout, key)
	}
}

// Ш7 (решение 8): проект не из Load (NewEditor, Restore) проверяется по правилам Load. UNC и
// относительный путь в папках, фото (и фото фона), запасных путях и своих отметках, NaN —
// отказ; Restore с таким проектом альбом и отметки не трогает.
func TestOpenRejectsForeignPaths(t *testing.T) {
	const unc, rel = `\\srv\share\a.jpg`, `Отбор\a.jpg`
	ok, dir, one := edtest.Abs("a.jpg"), edtest.Abs("Отбор"), 1
	cases := []struct {
		name  string
		spoil func(p *model.Project)
	}{
		{"папка UNC", func(p *model.Project) { p.Folders = []string{`\\srv\share`} }},
		{"папка относительная", func(p *model.Project) { p.Folders = []string{"Отбор"} }},
		{"запасной путь папки", func(p *model.Project) {
			p.Folders, p.FolderAlt = []string{dir}, map[string]string{dir: `\\srv\share`}
		}},
		{"фото UNC", func(p *model.Project) { p.Photos["a"] = model.PhotoRef{Path: unc} }},
		{"фото относительное", func(p *model.Project) { p.Photos["a"] = model.PhotoRef{Path: rel} }},
		{"запасной путь фото", func(p *model.Project) { p.Photos["a"] = model.PhotoRef{Path: ok, Alt: unc} }},
		{"фото фона", func(p *model.Project) {
			p.Photos["bg"] = model.PhotoRef{Path: unc}
			p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}, BgPhoto: &model.Backdrop{Photo: "bg", W: 10, H: 10}}}
		}},
		{"отметка UNC", func(p *model.Project) {
			p.SetLoaded(model.Side{Marks: map[string]model.Mark{"a": {Path: unc, Flag: &one}}})
		}},
		{"запасной путь отметки", func(p *model.Project) {
			p.SetLoaded(model.Side{Marks: map[string]model.Mark{"a": model.Mark{Path: ok, Flag: &one}.WithAlt(rel)}})
		}},
		{"NaN", func(p *model.Project) { p.Photos["a"] = model.PhotoRef{Path: ok, Frame: &model.Frame{S: math.NaN()}} }},
	}
	good := model.New()
	good.Folders, good.FolderAlt = []string{dir}, map[string]string{dir: edtest.Abs("Старое")}
	good.Photos["a"] = model.PhotoRef{Path: ok, Alt: edtest.Abs("b.jpg")}
	good.SetLoaded(model.Side{Marks: map[string]model.Mark{"a": model.Mark{Path: ok, Flag: &one}.WithAlt(edtest.Abs("b.jpg"))}})
	if _, err := album.NewEditor(good, ""); err != nil {
		t.Fatalf("локальные пути не прошли: %v", err)
	}
	for _, c := range cases {
		p := model.New()
		c.spoil(p)
		if e, err := album.NewEditor(p, ""); err == nil || e != nil {
			t.Errorf("%s: NewEditor открыл проект", c.name)
		}
		e := edtest.Open(t, model.New(), "")
		edtest.SetMarks(t, e, edtest.Photos("x"), model.MarkPatch{Rating: &one})
		before, rev, undo := model.Marshal(edtest.P(e)), e.View().Rev, edtest.UndoLen(e)
		if _, err := e.Restore(p); err == nil || e.View().Rev != rev || len(e.Marks()) != 1 ||
			edtest.UndoLen(e) != undo || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
			t.Errorf("%s: Restore принял проект (ошибка %v)", c.name, err)
		}
	}
}

// NewEditor держит свою копию: правка p после открытия до альбома не доходит, а from и свои
// отметки из файла (JSON копии их не несёт) переносятся явно; p.loaded остаётся у p.
func TestNewEditorOwnsCopy(t *testing.T) {
	ok, one := edtest.Abs("a.jpg"), 1
	p := model.New()
	p.Folders = []string{edtest.Abs("Отбор")}
	p.Photos["a"] = model.PhotoRef{Path: ok, W: 3, H: 2, Frame: &model.Frame{S: 2}}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"}}}
	p.SetFrom(4)
	p.SetLoaded(model.Side{Marks: map[string]model.Mark{"a": {Path: ok, Flag: &one}}})
	e := edtest.Open(t, p, edtest.Abs("альбом.salbum"))
	if !edtest.Backup(e) || len(e.Marks()) != 1 || len(edtest.Loaded(p).Marks) != 1 {
		t.Fatalf("from и отметки: копия старой версии %v, отметок %d, у p %d", edtest.Backup(e), len(e.Marks()), len(edtest.Loaded(p).Marks))
	}
	p.Photos["a"].Frame.S = 3
	p.Spreads[0].Name = "чужое"
	p.Folders[0] = `\\srv\share`
	if r, _ := e.Ref("a"); r.Frame.S != 2 || e.View().Spreads[0].Name != "" || e.Folders()[0] != edtest.Abs("Отбор") {
		t.Fatal("правка p после NewEditor дошла до альбома")
	}
	// Указатели отметки общие с p (maps.Clone без копии полей) — правка в p дошла бы до Editor.
	*edtest.Loaded(p).Marks["a"].Flag = -1
	if f := e.Marks()["a"].Flag; f == nil || *f != 1 {
		t.Fatalf("правка отметки в p после NewEditor дошла до Editor: флаг %v", f)
	}
	// Карта отметок общая с p (без maps.Clone) — новая отметка Editor появилась бы и у p.
	edtest.SetMarks(t, e, edtest.Photos("n"), model.MarkPatch{Rating: &one})
	if len(edtest.Loaded(p).Marks) != 1 {
		t.Fatalf("отметка Editor попала в p.loaded: %d", len(edtest.Loaded(p).Marks))
	}
}

// Ref и Refs отдают глубокую копию: правка кадра и подписи в ней проект не меняет.
func TestRefsDeepCopy(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	e.SetFrame("a", 2, 0.3, 0)
	e.SetCaption("a", "Подпись", "")
	before := model.Marshal(edtest.P(e))
	r, _ := e.Ref("a")
	if r.Frame == nil || r.Caption == nil {
		t.Fatalf("нет кадра или подписи: %+v", r)
	}
	r.Frame.S, r.Caption.Text = 4, "чужое"
	refs := e.Refs()
	refs["a"].Frame.X, refs["a"].Caption.Style.Size = -1, 99
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("правка копии из Ref/Refs дошла до проекта")
	}
}

// TestEditSyncGridRollsBack — правка оставила фото сетки без места (SyncGrid в settle): отказ
// ErrNoPlace, проект целиком как был (и правка соседнего разворота тоже), ни шага, ни rev.
func TestEditSyncGridRollsBack(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(120, 10, 100, 100, "b")}, []string{"c"})
	before := model.Marshal(edtest.P(e))
	rev, undo, dirty := edtest.Rev(e), edtest.UndoLen(e), edtest.Dirty(e)
	_, err := edtest.Try(e, "", func(p *model.Project) error {
		p.Spreads[1].Name = "Соседний"
		p.Photos["z"] = model.PhotoRef{Path: edtest.Abs("z.jpg"), W: 300, H: 200}
		p.Spreads[0].Photos = append(p.Spreads[0].Photos, "z") // мест два, оба заняты
		return nil
	})
	if !errors.Is(err, model.ErrNoPlace) {
		t.Fatalf("ошибка %v, хочу ErrNoPlace", err)
	}
	edtest.Same(t, e, rev, undo, "отказ SyncGrid")
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.Dirty(e) != dirty {
		t.Fatal("отказ SyncGrid изменил проект")
	}
}
