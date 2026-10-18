package grid_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Табличка места (071) при размножении мест: копии — без табличек (ClonePlace), источник
// свои хранит; правка таблички — обычный шаг отмены, то же имя — холостая.

var (
	clonePetya = model.Person{Last: "Иванов", First: "Петя"}
	cloneOlga  = model.Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}
)

// seatEd — сетка s1: место с фото и табличкой, пустое с табличкой, пустое без; s2 — «Авто».
func seatEd(t *testing.T) *album.Editor {
	t.Helper()
	a, b := edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(12, 142, 127, 130, "")
	a.Person, b.Person = clonePetya, cloneOlga
	return edtest.GridEd(t, []*model.Place{a, b, edtest.Gp(293, 12, 127, 127, "")}, []string{"c", "d"})
}

// seatsOf — таблички мест листа i по порядку.
func seatsOf(p *model.Project, i int) []model.Person {
	var out []model.Person
	for _, pl := range p.Spreads[i].Grid {
		out = append(out, pl.Person)
	}
	return out
}

func wantSeats(t *testing.T, what string, got []model.Person, want ...model.Person) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: мест %d, ждали %d (%+v)", what, len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: место %d — %+v, ждали %+v", what, i, got[i], want[i])
		}
	}
}

func TestClonePlaceDropsPerson(t *testing.T) {
	zero := model.Person{}
	// Деление двух мест с табличками: половинки-копии без табличек, отмена — один шаг.
	e := seatEd(t)
	g := edtest.P(e).Spreads[0].Grid
	undo := edtest.UndoLen(e)
	if _, err := e.ArrangePlaces("s1", "splitH", []string{g[0].ID, g[1].ID}); err != nil {
		t.Fatal(err)
	}
	wantSeats(t, "деление", seatsOf(edtest.P(e), 0), clonePetya, zero, cloneOlga, zero, zero)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("деление: шагов %d → %d", undo, edtest.UndoLen(e))
	}
	e.Undo()
	wantSeats(t, "отмена деления", seatsOf(edtest.P(e), 0), clonePetya, cloneOlga, zero)
	edtest.MustGrid(t, e)

	// Дубль разворота: копия мест без табличек и фото, у источника таблички на месте.
	e = seatEd(t)
	if _, err := e.DuplicateGrid("s1", 2, false); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	wantSeats(t, "дубль: источник", seatsOf(p, 0), clonePetya, cloneOlga, zero)
	for i := 1; i <= 2; i++ {
		wantSeats(t, "дубль", seatsOf(p, i), zero, zero, zero)
	}
	edtest.MustGrid(t, e)

	// Копия сетки на «Авто»: места цели из рамок, таблички источника не едут.
	e = seatEd(t)
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	p = edtest.P(e)
	wantSeats(t, "копия: источник", seatsOf(p, 0), clonePetya, cloneOlga, zero)
	wantSeats(t, "копия: цель", seatsOf(p, 1), zero, zero, zero)
	edtest.MustGrid(t, e)
}

// Табличка — данные проекта: правка через change — шаг отмены и rev, та же — холостая.
func TestSeatNameUndo(t *testing.T) {
	e := seatEd(t)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Grid[1].Person = cloneOlga })
	edtest.Same(t, e, rev, undo, "та же табличка")
	edtest.Change(e, func(p *model.Project) {
		p.Spreads[0].Grid[1].Person = model.Person{}
		p.Spreads[0].Grid[2].Person = clonePetya
	})
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("правка табличек: rev %d → %d, шагов %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	wantSeats(t, "правка", seatsOf(edtest.P(e), 0), clonePetya, model.Person{}, clonePetya)
	e.Undo()
	wantSeats(t, "отмена", seatsOf(edtest.P(e), 0), clonePetya, cloneOlga, model.Person{})
}

// TestSeatSitesGuard — страж design 071 §6: места создаются и заменяются только в известных
// функциях дерева album, у каждой — тест табличек (или причина, почему таблички там не нужны).
// Новое место создания мест без очереди жильцов молча теряет таблички: закрыть тестом §5 и
// дописать сюда.
func TestSeatSitesGuard(t *testing.T) {
	known := map[string]string{
		// Пересоздание мест — очередь жильцов.
		"grid/places.go applyGrid":           "TestApplyGridCarriesSeatNames, TestSpreadTplCarriesSeatNames",
		"vigsheet/vignette.go placeVignette": "TestVignetteRelayCarriesSeatNames, TestResetGrowVignetteSeatNames",
		"model/places.go Unfreeze":           "TestUnfreezeSeatNameToPhoto, TestSetLayoutSeatNameToPhoto",
		"model/places.go DeletePlaces":       "TestDeletePlacesDropsSeatName",
		// Копии мест — без табличек.
		"model/places.go ClonePlace": "TestClonePlaceDropsPerson",
		"grid/copy.go DuplicateGrid": "TestClonePlaceDropsPerson",
		"grid/ops.go splitPlaces":    "TestClonePlaceDropsPerson",
		// Новые пустые места и «Авто»: табличек ещё нет.
		"model/places.go AddPlace": "новое пустое место (TestSeatNamesInvariant: addPlace)",
		"spreads/drop.go tileRest": "плитка броска — новые места (TestSeatNamesInvariant: drop)",
		"model/places.go Freeze":   "места из «Авто»",
		"model/places.go WithGrid": "закрепление без изменений — снова «Авто»",
		"grid/ops.go MirrorGrid":   "«Авто» после Freeze без изменений",
		"grid/ops.go SetPlaces":    "проверка рамки, мест не создаёт",
		"model/order.go SetStack":  "те же места в порядке слоёв (TestSeatNamesInvariant: layers)",
		"model/places.go SyncGrid": "листу без мест — Grid nil",
		// Починка при Load и после ClampGrid: места только отбрасываются.
		"model/places.go syncSpread":   "починка не смогла добавить место — после ClampGrid не бывает",
		"model/clamp.go ClampGrid":     "Load: лишние места, запас знаков таблички уже списан (TestLoadClampsSeatName)",
		"model/cover.go FixCoverSheet": "Load: пустые записи мест",
		"salbum/fix.go dropNilPlaces":  "Load: пустые записи мест",
		"salbum/load.go capRawGrid":    "Load: хвост сырого списка мест сверх 4×MaxPlaces (TestLoadRawGridLimit)",
		"model/limits.go SplitBig":     "Load: сетка с > MaxPhotos фото — битый файл, части становятся «Авто» (TestSplitBigKeepsNameAndBackdrop)",
		// Не места: флаг вида.
		"view/parts.go gridView": "SpreadView.Grid — bool «лист с местами»",
	}
	// `.Grid` слева от `=`, и во множественном присваивании (`a.Photos, a.Grid = …`).
	re := regexp.MustCompile(`\.Grid(,[^=]*)? = |ClonePlace\(|&model\.Place\{|&Place\{`)
	found := map[string]bool{}
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "internal": // inside, edtest — доступ тестов
			return filepath.SkipDir
		case d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go"):
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("..", path)
		fn := ""
		for _, line := range strings.Split(string(data), "\n") {
			if name, ok := strings.CutPrefix(line, "func "); ok {
				if strings.HasPrefix(name, "(") {
					name = name[strings.Index(name, ")")+2:]
				}
				fn = name[:strings.IndexAny(name, "([")]
			}
			if re.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "//") {
				key := filepath.ToSlash(rel) + " " + fn
				found[key] = true
				if _, ok := known[key]; !ok {
					t.Errorf("место создаётся или заменяется в %s: %s", key, strings.TrimSpace(line))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k := range known {
		if !found[k] {
			t.Errorf("в стороже лишнее: %s", k)
		}
	}
}

// slotPhotos — фото мест листа i по порядку ("" — пустое место).
func slotPhotos(p *model.Project, i int) []string {
	var out []string
	for _, pl := range p.Spreads[i].Grid {
		out = append(out, pl.Photo)
	}
	return out
}

func wantSlots(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: фото мест %q, ждали %q", what, got, want)
	}
}

// queueEd — сетка s1: пустое с табличкой «Сидорова», «a» с табличкой «Иванов», «b» без,
// пустое без (справа); s2 — «Авто» c, d. Пустое без таблички сжимается, с табличкой — жилец.
func queueEd(t *testing.T) *album.Editor {
	t.Helper()
	a, b, c := edtest.Gp(12, 12, 127, 127, ""), edtest.Gp(12, 142, 127, 130, "a"), edtest.Gp(150, 12, 120, 127, "b")
	a.Person, b.Person = cloneOlga, clonePetya
	return edtest.GridEd(t, []*model.Place{a, b, c, edtest.Gp(293, 12, 127, 127, "")}, []string{"c", "d"})
}

func TestApplyGridCarriesSeatNames(t *testing.T) {
	zero := model.Person{}
	quarters := []model.Box{{X: 0, Y: 0, W: 0.25, H: 1}, {X: 0.25, Y: 0, W: 0.25, H: 1}, {X: 0.5, Y: 0, W: 0.25, H: 1}, {X: 0.75, Y: 0, W: 0.25, H: 1}}
	// Шаблон на разворот из 4 мест: жильцы по порядку, пустое без таблички — в конце.
	e := queueEd(t)
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	if _, left, err := e.ApplyGrid("s1", model.ScopeSpread, model.TplSpread, quarters); err != nil || left != 0 {
		t.Fatalf("шаблон: %v, ушло %d", err, left)
	}
	p := edtest.P(e)
	wantSeats(t, "4 места", seatsOf(p, 0), cloneOlga, clonePetya, zero, zero)
	wantSlots(t, "4 места", slotPhotos(p, 0), "", "a", "b", "")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шаблон: шагов %d", edtest.UndoLen(e)-undo)
	}
	edtest.MustGrid(t, e)
	q := edtest.SaveLoad(t, e)
	wantSeats(t, "Save → Load", seatsOf(q, 0), cloneOlga, clonePetya, zero, zero)
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена шаблона — не байт в байт")
	}
	// Мест меньше: «b» уходит, таблички двух первых жильцов на месте.
	if _, left, err := e.ApplyGrid("s1", model.ScopeSpread, model.TplSpread, quarters[:2]); err != nil || left != 1 {
		t.Fatalf("2 места: %v, ушло %d", err, left)
	}
	wantSeats(t, "2 места", seatsOf(edtest.P(e), 0), cloneOlga, clonePetya)
	wantSlots(t, "2 места", slotPhotos(edtest.P(e), 0), "", "a")
	e.Undo()
	// Шаблон на левую страницу из одного места: жилец — пустое с «Сидоровой», фото a и b
	// уходят, табличка «Иванова» пропадает; место правой страницы не тронуто.
	if _, left, err := e.ApplyGrid("s1", model.ScopeLeft, model.TplPage, quarters[:1]); err != nil || left != 2 {
		t.Fatalf("левая: %v, ушло %d", err, left)
	}
	wantSeats(t, "левая", seatsOf(edtest.P(e), 0), cloneOlga, zero)
	wantSlots(t, "левая", slotPhotos(edtest.P(e), 0), "", "")
	// Нет такого листа — холостая.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, _, _ = e.ApplyGrid("нет", model.ScopeSpread, model.TplSpread, quarters)
	edtest.Same(t, e, rev, undo, "шаблон на чужой лист")
}

func TestSpreadTplCarriesSeatNames(t *testing.T) {
	zero := model.Person{}
	e := queueEd(t)
	tp, err := e.SpreadTplOf("s1", model.ScopeSpread, true, false)
	if err != nil {
		t.Fatal(err)
	}
	// Тот же разворот на себя — холостая (места те же), таблички на месте.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "готовый разворот на себя")
	// Разворот из трёх первых мест: места заново, жильцы по порядку, пустое без таблички
	// сжато — все три жильца сели.
	tp.Places = tp.Places[:3]
	if _, _, _, err := e.ApplySpreadTpl("s1", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	wantSeats(t, "3 места", seatsOf(p, 0), cloneOlga, clonePetya, zero)
	wantSlots(t, "3 места", slotPhotos(p, 0), "", "a", "b")
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("готовый разворот: шагов %d", edtest.UndoLen(e)-undo)
	}
	// На «Авто» s2: таблички источника не едут.
	if _, _, _, err := e.ApplySpreadTpl("s2", model.ScopeSpread, tp, nil); err != nil {
		t.Fatal(err)
	}
	p = edtest.P(e)
	wantSeats(t, "на другой лист", seatsOf(p, 1), zero, zero, zero)
	wantSeats(t, "источник", seatsOf(p, 0), cloneOlga, clonePetya, zero)
	edtest.MustGrid(t, e)
}

// Копия сетки на лист-сетку со своими табличками: таблички источника не едут, свои таблички
// цели садятся по очереди жильцов вместе со своими фото.
func TestCopyGridTargetSeatNames(t *testing.T) {
	zero := model.Person{}
	e := queueEd(t)
	e.SetGrid("s2", true)
	edtest.Change(e, func(p *model.Project) { p.Spreads[1].Grid[1].Person = clonePetya })
	ph := slotPhotos(edtest.P(e), 1)
	if _, _, err := e.CopyGrid("s1", model.ScopeSpread, []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}, false, false); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	wantSeats(t, "цель", seatsOf(p, 1), zero, clonePetya, zero, zero)
	wantSlots(t, "цель", slotPhotos(p, 1), ph[0], ph[1], "", "")
	wantSeats(t, "источник", seatsOf(p, 0), cloneOlga, clonePetya, zero, zero)
	edtest.MustGrid(t, e)
}

// «Вернуть авто»: табличка места с фото — имя этого фото (подпись та же), пустых — пропадает.
func TestUnfreezeSeatNameToPhoto(t *testing.T) {
	a, f, o, b := edtest.Gp(12, 12, 127, 127, "a"), edtest.Gp(12, 142, 127, 130, "Петров Ваня"),
		edtest.Gp(150, 12, 120, 127, ""), edtest.Gp(293, 12, 127, 127, "b")
	a.Person, f.Person, o.Person = clonePetya, model.Person{Subject: "Физика"}, cloneOlga
	e := edtest.GridEd(t, []*model.Place{a, f, o, b})
	edtest.Change(e, func(p *model.Project) { // своё имя a с предметом: имя — таблички, предмет — свой
		r := p.Photos["a"]
		r.Person = &model.Person{Last: "Свой", Subject: "Хор"}
		p.Photos["a"] = r
	})
	shown := map[string]model.Person{}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		if who, _, _ := model.NameAt(edtest.P(e).Photos[pl.Photo], pl); pl.Photo != "" && who != nil {
			shown[pl.Photo] = *who
		}
	}
	if want := (model.Person{Last: "Иванов", First: "Петя", Subject: "Хор"}); shown["a"] != want {
		t.Fatalf("a на месте: %+v", shown["a"])
	}
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	e.SetGrid("s1", false)
	p := edtest.P(e)
	if len(p.Spreads[0].Grid) != 0 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Вернуть авто»: мест %d, шагов %d", len(p.Spreads[0].Grid), edtest.UndoLen(e)-undo)
	}
	for id, want := range shown {
		got, fromFile := p.Photos[id].Name()
		if got == nil || *got != want {
			t.Fatalf("фото %s: имя %+v, на месте было %+v", id, got, want)
		}
		if fromFile != (id == "Петров Ваня") {
			t.Fatalf("фото %s: из файла %v", id, fromFile)
		}
	}
	if r := p.Photos["b"]; r.Person != nil {
		t.Fatalf("b без таблички получил имя %+v", r.Person)
	}
	q := edtest.SaveLoad(t, e)
	if len(q.Photos) != 3 {
		t.Fatalf("Save → Load: фото %d", len(q.Photos))
	}
	for _, r := range q.Photos {
		if r.Person != nil && *r.Person == cloneOlga {
			t.Fatal("табличка пустого места досталась фото")
		}
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("отмена «Вернуть авто» — не байт в байт")
	}
	// Уже «Авто» — холостая.
	e.SetGrid("s1", false)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.SetGrid("s1", false)
	edtest.Same(t, e, rev, undo, "«Вернуть авто» у «Авто»")
}

// Раскладка из галереи на листе-сетке — тот же Unfreeze: табличка переходит фото.
func TestSetLayoutSeatNameToPhoto(t *testing.T) {
	e, id := edtest.AutoEd(t)
	e.SetGrid(id, true)
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Grid[1].Person = cloneOlga })
	ph := edtest.P(e).Spreads[0].Grid[1].Photo
	e.SetLayout(id, edtest.PickKey(t, e, 0, edtest.AnyKey))
	p := edtest.P(e)
	if len(p.Spreads[0].Grid) != 0 {
		t.Fatal("раскладка не сняла места")
	}
	if got, _ := p.Photos[ph].Name(); got == nil || *got != cloneOlga {
		t.Fatalf("фото %s: имя %+v", ph, got)
	}
}

func TestDeletePlacesDropsSeatName(t *testing.T) {
	zero := model.Person{}
	e := seatEd(t)
	g := edtest.P(e).Spreads[0].Grid
	ids := []string{g[0].ID, g[1].ID, g[2].ID}
	undo := edtest.UndoLen(e)
	e.DeletePlaces("s1", ids[1:2]) // пустое с табличкой
	wantSeats(t, "удалено пустое", seatsOf(edtest.P(e), 0), clonePetya, zero)
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("удаление: шагов %d", edtest.UndoLen(e)-undo)
	}
	e.Undo()
	wantSeats(t, "отмена", seatsOf(edtest.P(e), 0), clonePetya, cloneOlga, zero)
	// Все места: фото уходят до Unfreeze — табличка «Иванова» не становится именем «a».
	e.DeletePlaces("s1", ids)
	p := edtest.P(e)
	if len(p.Spreads[0].Grid) != 0 {
		t.Fatalf("мест %d", len(p.Spreads[0].Grid))
	}
	for id, r := range p.Photos {
		if r.Person != nil {
			t.Fatalf("фото %s получило имя %+v", id, r.Person)
		}
	}
}
