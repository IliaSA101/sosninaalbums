package grid_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Окно «Настроить виньетку» (031 dialog §5): сухой прогон PreviewVignette и «Применить»
// SetupVignette. Предпросмотр по построению равен итогу — тот же settle над копией.

// vigAlbum — альбом: обычный разворот с 5 фото, виньетка с полями, обычный разворот с 3 фото,
// вторая виньетка (25 фото на местах). Несколько листов разного вида: предпросмотр одного не
// должен зависеть от сборки остальных.
func vigAlbum(t *testing.T) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a1", "a2", "a3", "a4", "a5"))
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Vars = &model.Vars{Title: "Гимназия № 1", Year: "2026"} })
	edtest.NewSpread(e, -1, edtest.Photos("b1", "b2", "b3"))
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	e.FillPlaces(classPhotos(25), edtest.P(e).Spreads[3].ID)
	return e
}

// sameSheet — вид листа предпросмотра pr совпал с видом после «Применить» v (JSON). ID новых
// мест и новой шапки случайны (NewPlaceID, NewID) и у двух прогонов разные: они сверяются по
// позиции и подменяются; прежние ID (old) обязаны совпасть как есть.
func sameSheet(t *testing.T, pr, v model.SpreadView, old map[string]bool) {
	t.Helper()
	x, err := json.Marshal(pr)
	if err != nil {
		t.Fatal(err)
	}
	y, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var a, b []string
	for _, pl := range pr.Places {
		a = append(a, pl.ID)
	}
	for _, pl := range v.Places {
		b = append(b, pl.ID)
	}
	for _, tx := range pr.Texts {
		a = append(a, tx.ID)
	}
	for _, tx := range v.Texts {
		b = append(b, tx.ID)
	}
	if len(a) != len(b) {
		t.Fatalf("мест и надписей: предпросмотр %d, «Применить» %d", len(a), len(b))
	}
	for k := range a {
		switch {
		case a[k] == b[k]:
		case old[a[k]] || old[b[k]]:
			t.Fatalf("прежний ID %d: предпросмотр %q, «Применить» %q", k, a[k], b[k])
		default:
			x = bytes.ReplaceAll(x, []byte(a[k]), []byte(b[k]))
		}
	}
	if !bytes.Equal(x, y) {
		i := 0
		for i < min(len(x), len(y)) && x[i] == y[i] {
			i++
		}
		from := max(i-80, 0)
		t.Fatalf("лист: расходится с %d:\n%s\n%s", i, x[from:min(i+120, len(x))], y[from:min(i+120, len(y))])
	}
}

// TestVignettePreviewMatchesApply — критерий §8: предпросмотр = вид листа после «Применить» с
// тем же VigSetup (JSON), Left = left, ошибка — та же. Формы — на листе и в альбоме с
// несколькими листами, края: 0 учеников и учителей, длинное и многострочное поле, пустые поля.
func TestVignettePreviewMatchesApply(t *testing.T) {
	long := strings.Repeat("Ж", 200) + "\nвторая строка"
	forms := []struct {
		name string
		fn   func(v model.Vignette, vs model.Vars) model.VigSetup
	}{
		{"25 → 20 учеников", func(v model.Vignette, vs model.Vars) model.VigSetup {
			v.Pupils = 20
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"смена шаблона", func(v model.Vignette, vs model.Vars) model.VigSetup {
			v.Tpl = "g-bottom"
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"2:3", func(v model.Vignette, vs model.Vars) model.VigSetup {
			v.Ratio = "2:3"
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"учителей 0", func(v model.Vignette, vs model.Vars) model.VigSetup {
			v.Teachers = 0
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"шапка: заголовок", func(v model.Vignette, vs model.Vars) model.VigSetup {
			vs.Title, vs.Lead = "Детский сад № 5", "Иванова И. И."
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"поля пусты", func(v model.Vignette, _ model.Vars) model.VigSetup { return model.VigSetup{Vig: v} }},
		{"длинное поле", func(v model.Vignette, vs model.Vars) model.VigSetup {
			vs.Subtitle = long
			return model.VigSetup{Vig: v, Vars: vs}
		}},
		{"0 мест", func(v model.Vignette, vs model.Vars) model.VigSetup {
			return model.VigSetup{Vig: model.Vignette{Tpl: "grid"}, Vars: vs}
		}},
		{"ровно 64", func(_ model.Vignette, vs model.Vars) model.VigSetup {
			return model.VigSetup{Vig: model.Vignette{Tpl: "grid", Pupils: 64}, Vars: vs}
		}},
		{"больше 64", func(_ model.Vignette, vs model.Vars) model.VigSetup {
			return model.VigSetup{Vig: model.Vignette{Tpl: "t-left", Teachers: 1, Pupils: 64}, Vars: vs}
		}},
	}
	check := func(t *testing.T, e *album.Editor, i int, st model.VigSetup) {
		t.Helper()
		sp := edtest.P(e).Spreads[i]
		sid, old := sp.ID, map[string]bool{}
		for _, pl := range sp.Grid {
			old[pl.ID] = true
		}
		for _, tx := range sp.Texts {
			old[tx.ID] = true
		}
		pr, perr := e.PreviewVignette(sid, st, nil)
		v, left, err := e.SetupVignette(sid, st, nil)
		if fmt.Sprint(perr) != fmt.Sprint(err) {
			t.Fatalf("ошибки: предпросмотр %v, «Применить» %v", perr, err)
		}
		if err != nil {
			return
		}
		sameSheet(t, pr.Spread, v.Spreads[i], old)
		if pr.Left != left {
			t.Fatalf("в ленту: предпросмотр %d, «Применить» %d", pr.Left, left)
		}
	}
	for _, f := range forms {
		t.Run("лист/"+f.name, func(t *testing.T) {
			e, _ := vigSheet(t, 25)
			sp := edtest.P(e).Spreads[0]
			check(t, e, 0, f.fn(*sp.Vig, model.Vars{Year: "2026"}))
		})
		t.Run("альбом/"+f.name, func(t *testing.T) {
			e := vigAlbum(t)
			sp := edtest.P(e).Spreads[3]
			check(t, e, 3, f.fn(*sp.Vig, model.Vars{}))
		})
	}
	// Хвост конвейера: надпись «по тексту» с {Заголовок} меняет ширину (FitTexts), свой порядок
	// слоёв принимает новые места (SyncOrder) — без settle предпросмотр разошёлся бы с итогом.
	t.Run("надпись по тексту и свой порядок слоёв", func(t *testing.T) {
		e, sid := vigSheet(t, 25)
		id := edtest.AddText(t, e, sid, 0)
		e.SetText(id, "{Заголовок}", "")
		e.ArrangeLayers(sid, "back", []string{id})
		if edtest.P(e).Spreads[0].Order == nil {
			t.Fatal("порядок слоёв не свой")
		}
		v := *edtest.P(e).Spreads[0].Vig
		v.Pupils = 30
		check(t, e, 0, model.VigSetup{Vig: v, Vars: model.Vars{Title: "Муниципальное бюджетное общеобразовательное учреждение"}})
	})
	// Только поля на тронутой виньетке (сдвинуто место, шапка сдвинута или удалена; оба листа
	// альбома): предпросмотр так же оставляет места и шапку как есть.
	for _, del := range []bool{false, true} {
		t.Run(fmt.Sprintf("только поля, шапка удалена %v", del), func(t *testing.T) {
			e := vigAlbum(t)
			for _, i := range []int{1, 3} {
				touchVig(t, e, i, del)
			}
			for _, i := range []int{1, 3} {
				sp := edtest.P(e).Spreads[i]
				keep := sheetJSON(t, sp)
				check(t, e, i, model.VigSetup{Vig: *sp.Vig, Vars: model.Vars{Title: "Лицей № 7", Year: "2027"}})
				if got := sheetJSON(t, edtest.P(e).Spreads[i]); got != keep {
					t.Fatalf("лист %d: места или шапка изменились\n%s\n%s", i, keep, got)
				}
			}
		})
	}
	// «Сделать виньеткой» из окна над обычным разворотом: фото сверх мест уходят в ленту,
	// пустые поля наследуются у виньетки выше — и в предпросмотре тоже.
	e := vigAlbum(t)
	check(t, e, 2, model.VigSetup{Vig: model.Vignette{Tpl: "grid", Pupils: 2}})
	if sp := edtest.P(e).Spreads[2]; sp.Vars == nil || sp.Vars.Title != "Гимназия № 1" || len(sp.Photos) != 2 {
		t.Fatalf("сделать виньеткой: поля %+v, фото %v", sp.Vars, sp.Photos)
	}
}

// TestVignettePreviewNoEdit — критерий §8: предпросмотр не правит проект, rev, dirty, отмену,
// возврат и отметки; отказ — та же ошибка, что у SetupVignette.
func TestVignettePreviewNoEdit(t *testing.T) {
	e := vigAlbum(t)
	e.Undo() // в возврате шаг: предпросмотр его не сбрасывает
	sid := edtest.P(e).Spreads[1].ID
	before := model.Marshal(edtest.P(e))
	rev, undo, redo, dirty, marks := edtest.Rev(e), edtest.UndoLen(e), edtest.RedoLen(e), edtest.Dirty(e), edtest.MarksLen(e)
	same := func(what string) {
		t.Helper()
		edtest.Same(t, e, rev, undo, what)
		if !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.RedoLen(e) != redo || edtest.Dirty(e) != dirty || edtest.MarksLen(e) != marks {
			t.Fatalf("%s: проект, возврат, dirty или отметки изменились", what)
		}
	}
	pr, err := e.PreviewVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: "head", Pupils: 40}, Vars: model.Vars{Title: "Лицей"}}, nil)
	if err != nil || len(pr.Spread.Places) != 40 || pr.Spread.ID != sid || pr.Spread.Vars.Title != "Лицей" {
		t.Fatalf("предпросмотр: %v, мест %d, ID %q, поля %+v", err, len(pr.Spread.Places), pr.Spread.ID, pr.Spread.Vars)
	}
	if !slices.ContainsFunc(pr.Spread.Texts, func(x model.TextView) bool { return strings.Contains(x.Text, "Лицей") }) {
		t.Fatal("шапка предпросмотра без заголовка")
	}
	same("предпросмотр")
	over := model.VigSetup{Vig: model.Vignette{Tpl: "grid", Teachers: 18, Pupils: 64}}
	_, perr := e.PreviewVignette(sid, over, nil)
	same("предпросмотр больше 64")
	_, _, err = e.SetupVignette(sid, over, nil)
	if perr == nil || perr.Error() != err.Error() || !strings.Contains(perr.Error(), "не больше 64 мест") {
		t.Fatalf("больше 64: предпросмотр %v, «Применить» %v", perr, err)
	}
	if _, err := e.PreviewVignette("нет", model.VigSetup{Vig: model.DefaultVignette()}, nil); err == nil {
		t.Fatal("предпросмотр неизвестного листа без ошибки")
	}
	same("отказы")
}

// TestVignetteSetupOneStep — критерий §8: «Применить» окна — один шаг отмены (места и поля
// вместе), повтор — холостая, Save → Load сохраняет поля листа, пустые поля снимают их;
// только поля на тронутой виньетке — места и шапка как были (fieldsOnly).
func TestVignetteSetupOneStep(t *testing.T) {
	e := vigAlbum(t)
	sid := edtest.P(e).Spreads[3].ID
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	st := model.VigSetup{Vig: model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 20, Ratio: "3:4"},
		Vars: model.Vars{Title: " Гимназия\n№ 2 ", Subtitle: "11 «Б» класс", Year: "2026", Lead: "Петрова А. А."}}
	_, left, err := e.SetupVignette(sid, st, nil)
	sp := edtest.P(e).Spreads[3]
	want := model.ClampVars(st.Vars)
	if err != nil || left != 4 || len(sp.Grid) != 21 || sp.Vars == nil || *sp.Vars != *want || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("применить: %v, left %d, мест %d, поля %+v, шагов %d", err, left, len(sp.Grid), sp.Vars, edtest.UndoLen(e)-undo)
	}
	rev := edtest.Rev(e)
	if _, left, err := e.SetupVignette(sid, st, nil); err != nil || left != 0 {
		t.Fatalf("повтор: %v, left %d", err, left)
	}
	edtest.Same(t, e, rev, undo+1, "повтор «Применить»")

	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if qs := q.Spreads[3]; qs.Vars == nil || *qs.Vars != *want || len(qs.Grid) != 21 || *qs.Vig != *sp.Vig {
		t.Fatalf("Save → Load: поля %+v, мест %d", qs.Vars, len(qs.Grid))
	}

	// Пустые поля снимают свои (nil), а не пишут пустую структуру; тоже один шаг.
	if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: *sp.Vig}, nil); err != nil || edtest.P(e).Spreads[3].Vars != nil || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("пустые поля: %v, %+v", err, edtest.P(e).Spreads[3].Vars)
	}
	e.Undo()
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена не вернула места и поля вместе")
	}
	for _, del := range []bool{false, true} {
		t.Run(fmt.Sprintf("только поля, шапка удалена %v", del), func(t *testing.T) { fieldsOnly(t, del) })
	}
}

// touchVig — ручная правка виньетки i: место 0 сдвинуто на 3 мм, шапка сдвинута или удалена.
func touchVig(t *testing.T, e *album.Editor, i int, del bool) {
	t.Helper()
	sp := edtest.P(e).Spreads[i]
	pl := *sp.Grid[0]
	e.SetPlaces(sp.ID, []model.PlaceBox{{ID: pl.ID, X: pl.X + 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	h := sp.Vig.Head
	if h == "" {
		t.Fatalf("лист %d без шапки", i)
	}
	if del {
		e.DeleteText(h)
	} else {
		var x, y, w float64
		for _, tx := range edtest.P(e).Spreads[i].Texts {
			if tx.ID == h {
				x, y, w = tx.X, tx.Y, tx.W
			}
		}
		e.SetTextBox(h, x+5, y+4, w-10)
	}
	if g := edtest.P(e).Spreads[i].Grid[0]; math.Abs(g.X-pl.X-3) > 0.01 {
		t.Fatalf("лист %d: место не сдвинуто: %v → %v", i, pl.X, g.X)
	}
}

// placeBoxes — рамки мест листа по порядку, без ID (у нового прогона они случайны).
func placeBoxes(sp *model.Spread) []model.PlaceBox {
	var out []model.PlaceBox
	for _, pl := range sp.Grid {
		out = append(out, model.PlaceBox{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H})
	}
	return out
}

// shape — места и надписи листа с ID и рамками и параметры виньетки, без фото: Load даёт
// фото новые ID.
func shape(sp *model.Spread) string {
	var b strings.Builder
	for _, pl := range sp.Grid {
		fmt.Fprintf(&b, "%s %v %v %v %v;", pl.ID, pl.X, pl.Y, pl.W, pl.H)
	}
	for _, tx := range sp.Texts {
		fmt.Fprintf(&b, "T %s %v %v %v;", tx.ID, tx.X, tx.Y, tx.W)
	}
	if sp.Vig != nil {
		fmt.Fprintf(&b, "V %+v", *sp.Vig)
	}
	return b.String()
}

// sheetJSON — места, надписи, шапка и параметры листа без полей шапки (их правит «Применить»).
func sheetJSON(t *testing.T, sp *model.Spread) string {
	t.Helper()
	b, err := json.Marshal(struct {
		G []*model.Place
		T []*model.TextBlock
		V *model.Vignette
	}{sp.Grid, sp.Texts, sp.Vig})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fieldsOnly — «Применить» с прежними параметрами на тронутой виньетке (место сдвинуто,
// шапка сдвинута или удалена — del; оба листа альбома) пишет только поля: места и шапка как
// были, один шаг отмены; повтор — холостая; Save → Load — новые поля и ручные места; отмена
// возвращает прежние поля. Пустая подпись учителя из окна равна умолчанию; смена параметров
// по-прежнему пересобирает.
func fieldsOnly(t *testing.T, del bool) {
	e := vigAlbum(t)
	for _, i := range []int{1, 3} {
		touchVig(t, e, i, del)
	}
	before := model.Marshal(edtest.P(e))
	for k, i := range []int{1, 3} {
		sp := edtest.P(e).Spreads[i]
		keep, undo := sheetJSON(t, sp), edtest.UndoLen(e)
		v := *sp.Vig
		v.TText, v.Head = "", "" // как из окна: шапку окно не знает, подпись — умолчание
		st := model.VigSetup{Vig: v, Vars: model.Vars{Title: "Лицей № 7", Subtitle: "4 «А»"}}
		if _, left, err := e.SetupVignette(sp.ID, st, nil); err != nil || left != 0 {
			t.Fatalf("лист %d: %v, left %d", i, err, left)
		}
		got := edtest.P(e).Spreads[i]
		if sheetJSON(t, got) != keep || got.Vars == nil || *got.Vars != *model.ClampVars(st.Vars) || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("лист %d: места/шапка изменились или поля %+v, шагов %d", i, got.Vars, edtest.UndoLen(e)-undo)
		}
		rev := edtest.Rev(e)
		_, _, _ = e.SetupVignette(sp.ID, st, nil)
		edtest.Same(t, e, rev, undo+1, fmt.Sprintf("повтор, лист %d (%d)", i, k))
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{1, 3} {
		if a, b := shape(q.Spreads[i]), shape(edtest.P(e).Spreads[i]); a != b || q.Spreads[i].Vars == nil || q.Spreads[i].Vars.Title != "Лицей № 7" {
			t.Fatalf("Save → Load, лист %d: поля %+v\n%s\n%s", i, q.Spreads[i].Vars, a, b)
		}
	}
	e.Undo()
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена не вернула прежние поля")
	}
	// Другие параметры — пересборка по плану, как раньше: места как у нетронутого листа.
	v := *edtest.P(e).Spreads[3].Vig
	v.Pupils--
	fresh := vigAlbum(t)
	for _, x := range []*album.Editor{e, fresh} {
		if _, _, err := x.SetupVignette(edtest.P(x).Spreads[3].ID, model.VigSetup{Vig: v}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if a, b := placeBoxes(edtest.P(e).Spreads[3]), placeBoxes(edtest.P(fresh).Spreads[3]); !slices.Equal(a, b) {
		t.Fatalf("смена числа учеников не пересобрала тронутую виньетку:\n%v\n%v", a, b)
	}
}

// TestVignettePreviewFast — критерий §8: 101 разворот + виньетка на 64 места, медиана
// предпросмотра < 25 мс (под замком только Marshal, остальное — над копией).
func TestVignettePreviewFast(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	for i := range 100 {
		ids := make([]string, 2+i%4)
		for k := range ids {
			ids[k] = fmt.Sprintf("p%03d_%d", i, k)
		}
		edtest.NewSpread(e, -1, edtest.Photos(ids...))
	}
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	sid := edtest.P(e).Spreads[100].ID
	st := model.VigSetup{Vig: model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 63}, Vars: model.Vars{Title: "Гимназия"}}
	if _, _, err := e.SetupVignette(sid, st, nil); err != nil {
		t.Fatal(err)
	}
	e.FillPlaces(classPhotos(64), sid)
	ds := make([]time.Duration, 9)
	for i := range ds {
		st.Vig.Pupils = 62 + i%2 // как ввод с клавиатуры: каждый раз другое число
		t0 := time.Now()
		pr, err := e.PreviewVignette(sid, st, nil)
		ds[i] = time.Since(t0)
		if err != nil || len(pr.Spread.Places) != 63+i%2 {
			t.Fatalf("предпросмотр: %v, мест %d", err, len(pr.Spread.Places))
		}
	}
	slices.Sort(ds)
	t.Logf("предпросмотр, 101 разворот, 64 места: медиана %v, мин %v", ds[len(ds)/2], ds[0])
	if lim := testkit.Limit(25 * time.Millisecond); ds[len(ds)/2] > lim {
		t.Fatalf("медиана %v, предел %v", ds[len(ds)/2], lim)
	}
}

// TestVignettePreviewRace — предпросмотр параллельно с правками, отменой и видом: копия
// снимается под замком, дальше живого проекта не касается (go test -race).
func TestVignettePreviewRace(t *testing.T) {
	e := vigAlbum(t)
	sid := edtest.P(e).Spreads[3].ID
	var wg sync.WaitGroup
	for g := range 2 {
		wg.Go(func() {
			for i := range 15 {
				st := model.VigSetup{Vig: model.Vignette{Tpl: "grid", Pupils: 10 + i + g}, Vars: model.Vars{Year: fmt.Sprint(2020 + i)}}
				if pr, err := e.PreviewVignette(sid, st, nil); err != nil || len(pr.Spread.Places) != 10+i+g {
					t.Errorf("предпросмотр: %v, мест %d", err, len(pr.Spread.Places))
					return
				}
			}
		})
	}
	for i := range 10 {
		if _, _, err := e.SetupVignette(sid, model.VigSetup{Vig: model.Vignette{Tpl: "head", Pupils: 20 + i}}, nil); err != nil {
			t.Error(err)
		}
		if i%3 == 2 {
			e.Undo()
		}
		_ = e.View()
	}
	wg.Wait()
}

// Бросок на виньетку (042, решение 47): нетронутая растёт учениками, ручная — плитка.

// placeIDs — ID мест листа i по порядку.
func placeIDs(e *album.Editor, i int) []string {
	var out []string
	for _, pl := range edtest.P(e).Spreads[i].Grid {
		out = append(out, pl.ID)
	}
	return out
}

func TestGrowVignette(t *testing.T) {
	e, sid := vigSheet(t, 25)
	pupils, teachers, ids := rolePhotos(e, 0, model.RolePupil), rolePhotos(e, 0, model.RoleTeacher), placeIDs(e, 0)
	head, before, undo := vigOf(e, 0).Head, model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	_, out, err := e.DropPhotos(sid, "", classPhotos(27)[25:], -1, "")
	if err != nil || out.Ask || out.Grown != 2 || len(out.Fresh) != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("рост: %v, %+v, шагов %d", err, out, edtest.UndoLen(e)-undo)
	}
	if v := vigOf(e, 0); v.Pupils != 26 || v.Head != head || len(edtest.P(e).Spreads[0].Grid) != 27 || manual(e, sid) {
		t.Fatalf("виньетка: %+v, мест %d, ручная %v", v, len(edtest.P(e).Spreads[0].Grid), manual(e, sid))
	}
	if got := rolePhotos(e, 0, model.RolePupil); !slices.Equal(got, append(slices.Clone(pupils), "k25", "k26")) ||
		!slices.Equal(rolePhotos(e, 0, model.RoleTeacher), teachers) {
		t.Fatalf("порядок учеников: %v", got)
	}
	for _, id := range ids {
		if !slices.Contains(placeIDs(e, 0), id) {
			t.Fatalf("место %s потеряло ID", id)
		}
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if r := edtest.Open(t, q, ""); vigOf(r, 0).Pupils != 26 || manual(r, sid) {
		t.Fatalf("Save → Load: %+v, ручная %v", vigOf(r, 0), manual(r, sid))
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена роста — не байт в байт")
	}
	// Пустые места есть, фото больше: сначала вопрос, «Добавить учеников» — места, затем рост.
	e, sid = vigSheet(t, 23)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, out, err := e.DropPhotos(sid, "", classPhotos(27)[23:], -1, ""); err != nil || !out.Ask || out.Fit != 2 || out.Total != 4 {
		t.Fatalf("вопрос: %v, %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "вопрос у виньетки")
	if _, out, err := e.DropPhotos(sid, "", classPhotos(27)[23:], -1, "add"); err != nil || out.Grown != 2 || vigOf(e, 0).Pupils != 26 || manual(e, sid) {
		t.Fatalf("добавить: %v, %+v, %+v", err, out, vigOf(e, 0))
	}
	if n := len(edtest.P(e).Spreads[0].Photos); n != 27 {
		t.Fatalf("фото на листе %d, ждали 27", n)
	}
}

func TestGrowVignetteOwn(t *testing.T) {
	e, sid, tpl := ownEd(t)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "my:t1", Teachers: 1, Pupils: 24, Ratio: "3:4"}, []model.Template{tpl}); err != nil || manual(e, sid) {
		t.Fatalf("свой шаблон: %v", err)
	}
	_, out, err := e.DropPhotos(sid, "", classPhotos(27)[25:], -1, "")
	sp := edtest.P(e).Spreads[0]
	if err != nil || out.Grown != 2 || sp.Vig.Pupils != 26 || sp.Vig.Tpl != "my:t1" || sp.Vig.Own == nil || sp.Vig.Own.Name != "Свой" ||
		sp.Vig.TText != ownTText || sp.Caps.Style.Size != 17 || manual(e, sid) {
		t.Fatalf("рост своего шаблона: %v, %+v, %+v, кегль %v", err, out, sp.Vig, sp.Caps.Style.Size)
	}
}

func TestGrowVignetteLimit(t *testing.T) {
	e, sid := vigSheet(t, 0)
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 63}, nil); err != nil {
		t.Fatal(err)
	}
	e.FillPlaces(classPhotos(64), sid)
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	_, out, err := e.DropPhotos(sid, "", classPhotos(65)[64:], -1, "")
	if err == nil || out.Grown != 0 || !bytes.Equal(model.Marshal(edtest.P(e)), before) || edtest.UndoLen(e) != undo {
		t.Fatalf("предел учеников: %v, %+v", err, out)
	}
}

func TestDropVignetteManualTiles(t *testing.T) {
	e, sid := vigSheet(t, 25)
	pl := *edtest.P(e).Spreads[0].Grid[0]
	e.SetPlaces(sid, []model.PlaceBox{{ID: pl.ID, X: pl.X + 3, Y: pl.Y, W: pl.W, H: pl.H}}, "")
	if !manual(e, sid) {
		t.Fatal("сдвиг места не сделал виньетку ручной")
	}
	_, out, err := e.DropPhotos(sid, "", classPhotos(27)[25:], 0.5, "")
	sp := edtest.P(e).Spreads[0]
	if err != nil || out.Grown != 0 || len(out.Fresh) != 2 || sp.Vig.Pupils != 24 || len(sp.Grid) != 27 || !manual(e, sid) {
		t.Fatalf("ручная: %v, %+v, учеников %d, мест %d", err, out, sp.Vig.Pupils, len(sp.Grid))
	}
	for _, g := range sp.Grid[25:] {
		if model.RoleOf(g) != model.RolePupil || g.Photo == "" {
			t.Fatalf("новое место: %+v", *g)
		}
	}
}
