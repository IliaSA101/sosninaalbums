package app

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Свои шаблоны виньетки и сетки через методы App; сквозной путь сетки — TestTemplateGridSeveralMine ниже, помощники — prefs_test.go.

// Сквозной путь своих шаблонов виньетки (помощники — prefs_test.go): сохранить с листа
// или разворота, плитка «Свои» в окне, применить по "my:<id>", перезапуск.

// vigMoves — ручные правки виньетки, мм: учитель, первый и последний ученик. Учитель уходит вверх
// слева под шапку (на его месте пусто), ученики смещаются. Места не перекрываются и лежат в полях,
// хотя для точного режима это не нужно: он определяется размером листа (решение 0.11 №50).
var vigMoves = [][3][4]float64{
	{{8, 52.6, 41.36, 55.15}, {52.36, 52.6, 26.33, 35.1}, {8, 120, 26.33, 35.1}},
	{{8, 200, 41.36, 55.15}, {50.36, 52.6, 26.33, 35.1}, {8, 120, 26.33, 35.1}},
	{{8, 100, 36, 48}, {52.36, 52.6, 26.33, 35.1}, {8, 200, 26.33, 35.1}},
}

// moveVig применяет vigMoves[k] к листу sid: он становится «Ручной».
func moveVig(t *testing.T, a *App, sid string, k int) {
	t.Helper()
	ps := spreadPlaces(a, sid)
	if k < 0 { // разворот 600×300: координаты листа не годятся — учитель уходит вверх слева, под шапку
		for _, p := range ps {
			if p.Role == "teacher" {
				a.SetPlaces(sid, []model.PlaceBox{{ID: p.ID, X: 12, Y: 44, W: p.W, H: p.H}}, "")
			}
		}
		if !spreadView(a, sid).Vig.Manual {
			t.Fatal("разворот не стал ручным")
		}
		return
	}
	idx := [3]int{-1, -1, -1} // учитель, первый, последний ученик
	for i, p := range ps {
		switch {
		case p.Role == "teacher":
			idx[0] = i
		case idx[1] < 0:
			idx[1] = i
		default:
			idx[2] = i
		}
	}
	var bs []model.PlaceBox
	for j, i := range idx {
		c := vigMoves[k][j]
		bs = append(bs, model.PlaceBox{ID: ps[i].ID, X: c[0], Y: c[1], W: c[2], H: c[3]})
	}
	if a.SetPlaces(sid, bs, ""); !spreadView(a, sid).Vig.Manual {
		t.Fatal("лист не стал ручным")
	}
}

// vigWant — что ждём от сохранённого шаблона виньетки: w×h — размер листа, мм.
type vigWant struct {
	id, name, ratio string
	teachers        int
	pupils          int
	w, h            float64
	src             []model.PlaceView // места листа в порядке вида
	head            model.TextView    // шапка листа
}

func headOf(sv model.SpreadView) model.TextView {
	for _, x := range sv.Texts {
		if sv.Vig != nil && x.ID == sv.Vig.Head {
			return x
		}
	}
	return model.TextView{}
}

// saveVig сохраняет лист src как шаблон; он встаёт в конец списка.
func saveVig(t *testing.T, a *App, src, name, wantName string, w, h float64) vigWant {
	t.Helper()
	n := len(a.templateList())
	if err := a.SaveVignetteTemplate(src, name); err != nil {
		t.Fatal(err)
	}
	list := a.templateList()
	if len(list) != n+1 || list[n].Kind != model.TplVignette {
		t.Fatalf("шаблонов %d, ждали %d", len(list), n+1)
	}
	sv := spreadView(a, src)
	return vigWant{id: list[n].ID, name: wantName, ratio: sv.Vig.Ratio, teachers: sv.Vig.Teachers, pupils: sv.Vig.Pupils, w: w, h: h, src: sv.Places, head: headOf(sv)}
}

// checkVig: плитки «Свои» окна на листе dst идут в порядке want, с id "my:…", именем и долями
// мест; применение каждой (по её id и портрету) ставит места своего шаблона точно; другое число
// учеников — места по числам; Undo возвращает лист.
func checkVig(t *testing.T, a *App, dst string, want []vigWant) {
	t.Helper()
	w0 := want[0]
	var own []model.VigChoice
	for _, c := range a.VignetteChoices(dst, w0.teachers, w0.pupils, w0.ratio) {
		if c.Own {
			own = append(own, c)
		}
	}
	if len(own) != len(want) {
		t.Fatalf("«Свои»: плиток %d, ждали %d", len(own), len(want))
	}
	for i, w := range want {
		c, tag := own[i], fmt.Sprintf("%s #%d", w.name, i)
		if c.ID != model.VigOwnPrefix+w.id || c.Name != w.name || c.Gone || len(c.Roles) != len(w.src) {
			t.Fatalf("%s: плитка %+v, ждали id %s", tag, c, w.id)
		}
		sameBoxes(t, tag+": превью", c.Boxes, fracOf(w.src, 0, w.w, w.h))
		for k, p := range w.src {
			if c.Roles[k] != p.Role {
				t.Fatalf("%s: роль %d: %q, ждали %q", tag, k, c.Roles[k], p.Role)
			}
		}
		r, err := a.SetVignette(dst, model.Vignette{Tpl: c.ID, Teachers: w.teachers, Pupils: w.pupils, Ratio: c.Ratio})
		sv := spreadView(a, dst)
		if err != nil || r.Left != 0 || sv.Vig.OwnName != w.name || sv.Vig.Manual {
			t.Fatalf("%s: %v, left %d, %+v", tag, err, r.Left, sv.Vig)
		}
		samePlaces(t, tag+": применение", sv.Places, w.src, 0)
		if g := headOf(sv); math.Abs(g.X-w.head.X) > 0.02 || math.Abs(g.Y-w.head.Y) > 0.02 || math.Abs(g.W-w.head.W) > 0.02 {
			t.Fatalf("%s: шапка %+v, сохранена %+v", tag, g, w.head)
		}
		a.editor().Undo()
		// Другое число учеников: места по числам, шаблон остаётся своим.
		if _, err = a.SetVignette(dst, model.Vignette{Tpl: c.ID, Teachers: w.teachers, Pupils: w.pupils - 4, Ratio: c.Ratio}); err != nil {
			t.Fatalf("%s: %v", tag, err)
		}
		if sv = spreadView(a, dst); len(sv.Places) != len(w.src)-4 || sv.Vig.OwnName != w.name {
			t.Fatalf("%s: на %d учеников мест %d", tag, w.pupils-4, len(sv.Places))
		}
		if a.editor().Undo(); strings.HasPrefix(spreadView(a, dst).Vig.Tpl, model.VigOwnPrefix) {
			t.Fatalf("%s: отмена не вернула прежний шаблон", tag)
		}
	}
}

// Шаблон виньетки с листа 204×304 и с разворота 600×300: из «Авто» и после ручных правок
// (учитель сверху слева, пустое место под шапкой), перезапуск; виньетка не из умолчаний — тоже «Авто».
func TestTemplateVignetteEndToEnd(t *testing.T) {
	for _, spread := range []bool{false, true} {
		t.Run(fmt.Sprintf("разворот=%v", spread), func(t *testing.T) {
			a, src, dst, w, h, k := vigFixture(t, spread)
			auto := saveVig(t, a, src, "Авто", "Авто", w, h) // «Авто»: места — по плану каталога
			moveVig(t, a, src, k)
			manual := saveVig(t, a, src, " Мой\n класс ", "Мой класс", w, h)
			if slices.EqualFunc(auto.src, manual.src, func(x, y model.PlaceView) bool { return x.X == y.X && x.Y == y.Y }) {
				t.Fatal("правки мест не изменили лист")
			}
			want := []vigWant{auto, manual}
			checkVig(t, a, dst, want)
			checkVig(t, restartApp(t, a), dst, want)
		})
		t.Run(fmt.Sprintf("каталог, разворот=%v", spread), func(t *testing.T) {
			// Виньетка «Авто» не из умолчаний: другой шаблон каталога и числа людей.
			a, src, dst, w, h, _ := vigFixture(t, spread)
			if _, _, err := a.editor().SetVignette(src, model.Vignette{Tpl: "head-t-bottom", Teachers: 2, Pupils: 18, Ratio: "3:4"}, nil); err != nil {
				t.Fatal(err)
			}
			if spreadView(a, src).Vig.Manual {
				t.Fatal("виньетка из каталога «Ручная»")
			}
			want := saveVig(t, a, src, "Низ", "Низ", w, h)
			checkVig(t, restartApp(t, a), dst, []vigWant{want})
		})
	}
}

// vigFixture — App с виньеткой-источником src (лист 204×304 или разворот 600×300, нетронутая «Авто»)
// и пустой целью dst того же вида; k — номер ручной правки для moveVig.
func vigFixture(t *testing.T, spread bool) (a *App, src, dst string, w, h float64, k int) {
	t.Helper()
	w, h, adds := 204.0, 304.0, 1 // лист уже есть: источник, цель — добавленный
	if spread {
		a, _ = rtApp(t)
		a.ed.Store(mustEditor(t, model.New(), ""))
		w, h, k, adds = 600, 300, -1, 2
	} else {
		a, _ = vigTplApp(t)
	}
	for range adds {
		if _, err := a.AddVignette(-1); err != nil {
			t.Fatal(err)
		}
	}
	v := a.editor().View()
	src, dst = v.Spreads[0].ID, v.Spreads[len(v.Spreads)-1].ID
	if len(v.Spreads) != 2 || spreadView(a, src).Vig.Manual {
		t.Fatalf("листов %d или источник «Ручной»", len(v.Spreads))
	}
	return a, src, dst, w, h, k
}

// Несколько своих шаблонов виньетки подряд — одинаковые и похожие имена, «Авто» и три ручные правки
// одного листа: плитки «Свои» и применение не путаются при переименовании, удалении и перезапуске.
func TestTemplateVignetteSeveralMine(t *testing.T) {
	a, src, dst, w, h, _ := vigFixture(t, false)
	all := []vigWant{saveVig(t, a, src, "Класс", "Класс", w, h)} // «Авто»
	for k, name := range []string{"Класс", "класс", " Класс "} {
		moveVig(t, a, src, k)
		all = append(all, saveVig(t, a, src, name, strings.TrimSpace(name), w, h))
	}
	stage := func(x *App, tag string) {
		t.Helper()
		var want, got []string
		for _, w := range all {
			want = append(want, w.id)
		}
		for _, tp := range x.templateList() {
			got = append(got, tp.ID)
		}
		if !slices.Equal(want, got) {
			t.Fatalf("%s: порядок шаблонов в файле: %v, ждали %v", tag, got, want)
		}
		checkVig(t, x, dst, all)
	}
	stage(a, "сохранено")
	b := restartApp(t, a)
	stage(b, "перезапуск")
	for i, name := range map[int]string{1: "Верх", 3: "Класс 2"} {
		if err := b.RenameGridTemplate(model.VigOwnPrefix+all[i].id, name); err != nil { // по id плитки, с «my:»
			t.Fatal(err)
		}
		all[i].name = name
	}
	stage(b, "переименование")
	for _, i := range []int{2, 0} {
		if err := b.DeleteGridTemplate(model.VigOwnPrefix + all[i].id); err != nil {
			t.Fatal(err)
		}
		all = slices.Delete(all, i, i+1)
	}
	stage(b, "удаление")
	stage(restartApp(t, b), "перезапуск после удаления")
}

// Жалоба 0.11 «сохранил шаблон виньетки, а загрузилось другое расположение»: ученик на шапке,
// ученик на 4 мм ниже (полоса подписи на следующем ряду; на 3 мм здесь — встык), «Отразить»
// разворота. Превью «Свои» = сохранённое = применённое, и после перезапуска (checkVig).
func TestTemplateVignetteExact(t *testing.T) {
	pupil := func(a *App, sid string, dy, y float64) {
		p := spreadPlaces(a, sid)[0] // первое место — ученик
		a.SetPlaces(sid, []model.PlaceBox{{ID: p.ID, X: p.X, Y: cmp.Or(y, p.Y+dy), W: p.W, H: p.H}}, "")
	}
	for name, c := range map[string]struct {
		spread bool
		edit   func(a *App, sid string)
	}{
		"на шапке":          {false, func(a *App, sid string) { pupil(a, sid, 0, 20) }},
		"на 4 мм ниже":      {false, func(a *App, sid string) { pupil(a, sid, 4, 0) }},
		"зеркало разворота": {true, func(a *App, sid string) { a.MirrorGrid(sid, "spread") }},
	} {
		t.Run(name, func(t *testing.T) {
			a, src, dst, w, h, _ := vigFixture(t, c.spread)
			c.edit(a, src)
			want := saveVig(t, a, src, name, name, w, h)
			if !spreadView(a, src).Vig.Manual || want.src[0].Role != model.RolePupil {
				t.Fatal("правка не сделала лист ручным")
			}
			for _, p := range want.src { // зеркало отражает и шапку: под ней никого
				if hd := want.head; c.spread && p.X < hd.X+hd.W && hd.X < p.X+p.W && p.Y < hd.Y+hd.H && hd.Y < p.Y+p.H {
					t.Fatalf("место %+v под шапкой %+v", p, hd)
				}
			}
			checkVig(t, a, dst, []vigWant{want})
			checkVig(t, restartApp(t, a), dst, []vigWant{want})
			checkVigNoSize(t, a, dst, want, !c.spread) // зеркало без наложений точно и без размера
			if name == "на шапке" {
				checkVigSaved(t, a, dst, want)
			}
		})
	}
}

// checkVigSaved: точное применение на шапке переживает Save и открытие файла — лист не «Ручная»,
// места те же (допуск 0,006 мм: округление при записи).
func checkVigSaved(t *testing.T, a *App, dst string, want vigWant) {
	t.Helper()
	own := model.Vignette{Tpl: model.VigOwnPrefix + want.id, Teachers: want.teachers, Pupils: want.pupils, Ratio: want.ratio}
	if _, err := a.SetVignette(dst, own); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(testkit.TempDir(t), "vig"+projectExt)
	if err := a.editor().Save(file); err != nil {
		t.Fatal(err)
	}
	if _, err := a.openPath(file); err != nil {
		t.Fatal(err)
	}
	sv := spreadView(a, dst)
	if sv.Vig == nil || sv.Vig.Manual || len(sv.Places) != len(want.src) {
		t.Fatalf("после открытия: vig %+v, мест %d", sv.Vig, len(sv.Places))
	}
	for k, p := range sv.Places {
		w := want.src[k]
		if math.Abs(p.X-w.X) > 0.006 || math.Abs(p.Y-w.Y) > 0.006 || math.Abs(p.W-w.W) > 0.006 || math.Abs(p.H-w.H) > 0.006 {
			t.Fatalf("место %d после открытия %+v, сохранено %+v", k, p, w)
		}
	}
}

// checkVigNoSize: файл шаблонов без размера листа (до поля) — прежний расчёт: места пересчитаны
// (moved) или нет, но плитка «Свои» показывает то же, что ставит «Применить».
func checkVigNoSize(t *testing.T, a *App, dst string, want vigWant, moved bool) {
	t.Helper()
	size := fmt.Sprintf(`,"w":%v,"h":%v,`, want.w, want.h)
	if raw := string(readTpl(t, a)); strings.Count(raw, size) != 1 {
		t.Fatalf("размер листа %s в шаблоне: %.300s", size, raw)
	} else if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), []byte(strings.Replace(raw, size, ",", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	b := restartApp(t, a)
	cs := b.VignetteChoices(dst, want.teachers, want.pupils, want.ratio)
	own := cs[len(cs)-1]
	if _, err := b.SetVignette(dst, model.Vignette{Tpl: own.ID, Teachers: want.teachers, Pupils: want.pupils, Ratio: own.Ratio}); err != nil || !own.Own {
		t.Fatalf("без размера: %v, %+v", err, own)
	}
	got, diff := spreadView(b, dst).Places, false
	for k, p := range got { // превью — до зажима места (0,01 мм)
		f := own.Boxes[k]
		if math.Abs(f.X*want.w-p.X) > 0.01 || math.Abs(f.Y*want.h-p.Y) > 0.01 || math.Abs(f.W*want.w-p.W) > 0.01 || math.Abs(f.H*want.h-p.H) > 0.01 {
			t.Fatalf("без размера: превью %d %+v, применено %+v", k, f, p)
		}
		diff = diff || math.Abs(p.Y-want.src[k].Y) > 0.1 || math.Abs(p.X-want.src[k].X) > 0.1
	}
	if diff != moved {
		t.Fatalf("без размера: места пересчитаны — %v, ждали %v", diff, moved)
	}
}

// Несколько своих шаблонов подряд — с одинаковыми и похожими именами, с одного разворота после
// правки мест: у каждой плитки «Мои» свои id, доли и применение; переименование, удаление
// и перезапуск ничего не путают (отзыв владельца: чужой рисунок под своим именем).
func TestTemplateGridSeveralMine(t *testing.T) {
	a, ids := rtApp(t, 4, 4, 2)
	src, d4, d2 := ids[0], ids[1], ids[2]
	a.SetGrid(src, true)
	var all []tplWant
	save := func(l int, sc, name, want string) {
		setLayout(t, a, src, l)
		all = append(all, saveTpl(t, a, src, sc, name, want))
	}
	save(0, "spread", "Мой", "Мой")
	save(0, "left", "Мой", "Мой")
	save(0, "right", "Мой", "Мой")
	save(1, "spread", "Мой", "Мой") // то же имя с того же разворота после правки мест
	save(1, "left", "мой", "мой")
	save(2, "left", "Мой", "Мой")
	save(2, "spread", " Мой ", "Мой")
	save(2, "right", "мой", "мой")
	of := func(tpl string) (out []tplWant) {
		for _, w := range all {
			if w.tpl == tpl {
				out = append(out, w)
			}
		}
		return out
	}
	stage := func(x *App, tag string) {
		t.Helper()
		var want, got []string
		for _, w := range all {
			want = append(want, w.id)
		}
		for _, tp := range x.templateList() {
			got = append(got, tp.ID)
		}
		if !slices.Equal(want, got) {
			t.Fatalf("%s: порядок шаблонов в файле: %v, ждали %v", tag, got, want)
		}
		checkMine(t, x, d4, 4, "spread", of(model.TplSpread))
		checkMine(t, x, d2, 2, "left", of(model.TplPage))
		checkMine(t, x, d2, 2, "right", of(model.TplPage))
	}
	stage(a, "сохранено")
	b := restartApp(t, a)
	stage(b, "перезапуск")

	if err := b.RenameGridTemplate(all[1].id, "Левая"); err != nil {
		t.Fatal(err)
	}
	all[1].name = "Левая"
	if err := b.RenameGridTemplate(all[3].id, "Разворот Б"); err != nil {
		t.Fatal(err)
	}
	all[3].name = "Разворот Б"
	stage(b, "переименование")

	del := func(x *App, i int) {
		t.Helper()
		if err := x.DeleteGridTemplate(all[i].id); err != nil {
			t.Fatal(err)
		}
		all = slices.Delete(all, i, i+1)
	}
	del(b, 2)
	del(b, 0)
	stage(b, "удаление")
	stage(restartApp(t, b), "перезапуск после удаления")
}

// TestAppDropPhotos — бросок 042 через App (здесь, а не в grid_test.go/spreads_test.go: те у
// предела 600 строк): фото альбома без папки — по сведениям проекта, вопрос без правки, ответ
// «add» — плитка; поля JSON DropResult — контракт с UI (design §1).
func TestAppDropPhotos(t *testing.T) {
	a := gridApp(t)
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Grid: []*model.Place{
			{ID: "00000000000a", X: 12, Y: 12, W: 100, H: 100, Photo: "a"},
			{ID: "00000000000b", X: 312, Y: 12, W: 100, H: 100}}},
		{ID: "s2", Photos: []string{"b", "c", "d"}},
	}
	for _, id := range []string{"a", "b", "c", "d"} {
		p.Photos[id] = model.PhotoRef{Path: `C:\` + id + `.jpg`, W: 300, H: 200}
	}
	a.ed.Store(mustEditor(t, p, ""))
	views := 0
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			views++
		}
	}
	rev := a.editor().View().Rev
	keys := func(r DropResult) string {
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(slices.Sorted(maps.Keys(m)), " fresh=", m["fresh"])
	}
	r, err := a.DropPhotos("s1", "", []string{"b", "c", "d", "zz"}, math.NaN(), "что-то")
	if err != nil || !r.Ask || r.Fit != 1 || r.Total != 3 || r.View.Rev != rev || len(r.View.Spreads) != 2 {
		t.Fatalf("вопрос: %v, %+v", err, r)
	}
	if views != 0 || a.saveTimer != nil { // вопрос — не правка: ни рассылки вида, ни автосохранения
		t.Fatalf("вопрос разослал вид (%d) или завёл автосохранение", views)
	}
	if got := keys(r); got != "[ask fit fresh grown left out small total view] fresh=[]" {
		t.Fatalf("поля JSON: %s", got)
	}
	r, err = a.DropPhotos("s1", "", []string{"b", "c", "d"}, 0.25, "add")
	if err != nil || r.Ask || r.Fit != 1 || len(r.Fresh) != 2 || r.View.Rev != rev+1 || !slices.Equal(r.View.Spreads[0].Photos, []string{"a", "b", "c", "d"}) {
		t.Fatalf("вставить: %v, %+v", err, r)
	}
	if views != 1 {
		t.Fatalf("правка разослала вид %d раз, ждали 1", views)
	}
	for _, pl := range r.View.Spreads[0].Places[2:] {
		if pl.X > 281 || !slices.Contains(r.Fresh, pl.ID) {
			t.Fatalf("новое место не на левой странице или не в fresh: %+v", pl)
		}
	}
}

// QA 038: имя своего шаблона на листе следует за файлом и не откатывается Ctrl+Z. Переименовать →
// «Применить» с полями шапки (шаг отмены) → отмена → удалить: на листе — последнее имя.
func TestVigOwnNameAfterUndo(t *testing.T) {
	a, sid := vigTplApp(t)
	views := 0
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			views++
		}
	}
	if err := a.SaveVignetteTemplate(sid, "Мой класс"); err != nil {
		t.Fatal(err)
	}
	id := model.VigOwnPrefix + a.templateList()[0].ID
	if _, err := a.SetVignette(sid, model.Vignette{Tpl: id, Teachers: 1, Pupils: 24}); err != nil {
		t.Fatal(err)
	}
	vig := func() *model.VignetteView { return a.editor().View().Spreads[0].Vig }
	views0 := views
	if err := a.RenameGridTemplate(id, "Новый"); err != nil || vig().OwnName != "Новый" || views != views0+1 {
		t.Fatalf("переименование: %v, имя %q, рассылок %d", err, vig().OwnName, views-views0)
	}
	// Шага отмены переименование не дало: Ctrl+Z снимает «Применить», возврат — с новым именем.
	if a.Undo(); vig().Tpl == id {
		t.Fatal("отмена не сняла «Применить»")
	}
	if a.Redo(); vig().OwnName != "Новый" {
		t.Fatalf("возврат: имя %q", vig().OwnName)
	}
	st := model.VigSetup{Vig: model.Vignette{Tpl: id, Teachers: 1, Pupils: 24, Ratio: "3:4"}, Vars: model.Vars{Title: "Школа"}}
	if _, err := a.SetupVignette(sid, st); err != nil {
		t.Fatal(err)
	}
	a.Undo()
	// Как если бы альбом был закрыт, когда шаблон переименовали: на листе старое имя, удаление
	// берёт последнее из файла.
	old := a.templateList()[0]
	old.Name = "Мой класс"
	if a.editor().SyncOwnNames([]model.Template{old}); vig().OwnName != "Мой класс" {
		t.Fatal("старое имя не встало")
	}
	if err := a.DeleteGridTemplate(id); err != nil {
		t.Fatal(err)
	}
	if cs := a.VignetteChoices(sid, 1, 24, "3:4"); vig().OwnName != "Новый" || !cs[len(cs)-1].Gone || cs[len(cs)-1].Name != "Новый" {
		t.Fatalf("после отмены и удаления: имя %q, плитка %+v", vig().OwnName, cs[len(cs)-1])
	}
}
