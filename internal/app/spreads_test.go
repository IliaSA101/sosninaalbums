package app

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// spreadsApp — App с альбомом без папок: развороты s1, s2, … с заданным составом фото;
// фото именуются как в составе, события пишутся (имя → вид по порядку).
func spreadsApp(t *testing.T, spreads ...[]string) (a *App, views func() []model.View) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var got []model.View
	a.emit = func(name string, data ...any) {
		if name != "view" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		got = append(got, data[0].(model.View))
	}
	p := model.New()
	for i, ids := range spreads {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i)), Photos: slices.Clone(ids)})
		for _, id := range ids {
			p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
		}
	}
	a.ed.Store(mustEditor(t, p, ""))
	return a, func() []model.View {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// photosOf — составы разворотов вида по порядку.
func photosOf(v model.View) [][]string {
	out := make([][]string, len(v.Spreads))
	for i, s := range v.Spreads {
		out[i] = s.Photos
	}
	return out
}

func equalComposition(got [][]string, want ...[]string) bool {
	return slices.EqualFunc(got, want, func(x, y []string) bool { return slices.Equal(x, y) })
}

// Перестановка и перенос фото: рабочий случай меняет состав и шлёт вид, отказ (неизвестное фото,
// переполненный разворот) ничего не меняет; Undo и Redo проходят по шагам.
func TestAppPhotoMoves(t *testing.T) {
	a, views := spreadsApp(t, []string{"a", "b", "c"}, []string{"d"}, []string{"e"})
	rev := func() int { return a.editor().View().Rev }

	r := rev()
	v := a.SwapPhotos("a", "d")
	if !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) || v.Rev <= r {
		t.Fatalf("SwapPhotos: %v, rev %d→%d", photosOf(v), r, v.Rev)
	}
	// Неизвестное фото — холостая правка: иначе фото встало бы дважды.
	r = rev()
	if v = a.SwapPhotos("a", "нет"); v.Rev != r || !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) {
		t.Fatalf("SwapPhotos неизвестного: %v, rev %d→%d", photosOf(v), r, v.Rev)
	}

	// Перенос в другой разворот на позицию 1; опустевший прежний разворот исчезает.
	v, err := a.MovePhoto("e", "s1", 1)
	if err != nil || !equalComposition(photosOf(v), []string{"d", "e", "b", "c"}, []string{"a"}) {
		t.Fatalf("MovePhoto: %v, %v", photosOf(v), err)
	}
	// Неизвестное фото — не ошибка и не правка.
	r = rev()
	if v, err = a.MovePhoto("нет", "s1", 0); err != nil || v.Rev != r {
		t.Fatalf("MovePhoto неизвестного: err %v, rev %d→%d", err, r, v.Rev)
	}

	// Разворот уже полон: перенос в него — ошибка, проект как был.
	full := make([]string, model.MaxPhotos)
	for i := range full {
		full[i] = string(rune('A' + i))
	}
	b, _ := spreadsApp(t, full, []string{"z"})
	before := b.editor().View()
	v, err = b.MovePhoto("z", "s1", -1)
	if err == nil || !strings.Contains(err.Error(), "не может быть больше") {
		t.Fatalf("переполнение: err %v", err)
	}
	if v.Rev != before.Rev || !equalComposition(photosOf(v), full, []string{"z"}) {
		t.Fatalf("после отказа: %v, rev %d→%d", photosOf(v), before.Rev, v.Rev)
	}

	// Удаление фото из альбома: уходит из разворота (неизвестный ID пропускается).
	v = a.RemovePhotos([]string{"b", "нет"})
	if !equalComposition(photosOf(v), []string{"d", "e", "c"}, []string{"a"}) {
		t.Fatalf("RemovePhotos: %v", photosOf(v))
	}

	// Каждая успешная правка и каждый ответ — событие view с тем же rev, что вернул метод.
	last := views()
	if len(last) == 0 || last[len(last)-1].Rev != v.Rev {
		t.Fatalf("событие view не совпало с ответом: %d событий", len(last))
	}

	// Откат всех шагов возвращает исходный состав, Redo — вперёд.
	for a.editor().View().CanUndo {
		a.Undo()
	}
	if got := photosOf(a.editor().View()); !equalComposition(got, []string{"a", "b", "c"}, []string{"d"}, []string{"e"}) {
		t.Fatalf("после отката: %v", got)
	}
	if v = a.Redo(); !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) || !v.CanUndo {
		t.Fatalf("Redo: %v", photosOf(v))
	}
}

// Redo без шагов вперёд — вид без изменений.
func TestAppRedoNothing(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	before := a.editor().View()
	if v := a.Redo(); v.Rev != before.Rev || v.CanRedo {
		t.Fatalf("Redo без шагов: rev %d→%d, canRedo %v", before.Rev, v.Rev, v.CanRedo)
	}
}

// Отмена и возврат шага со сменой списка папок перечитывают библиотеку (rescan): ход сканирования
// становится активным и приходит к концу.
func TestAppUndoRedoFoldersRescan(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "x.jpg", 20, 10)
	if _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	waitScan(t, a)
	if n := len(a.GetState().Photos); n != 1 {
		t.Fatalf("после rescan фото %d", n)
	}
	v := a.Undo() // папка уходит из проекта — и фото из ленты
	if len(v.Folders) != 0 {
		t.Fatalf("папки после Undo: %v", v.Folders)
	}
	waitScan(t, a)
	if n := len(a.GetState().Photos); n != 0 {
		t.Fatalf("после Undo в ленте осталось %d фото", n)
	}
	v = a.Redo()
	if len(v.Folders) != 1 {
		t.Fatalf("папки после Redo: %v", v.Folders)
	}
	waitScan(t, a)
	if n := len(a.GetState().Photos); n != 1 {
		t.Fatalf("после Redo фото %d", n)
	}
}

// Разделить и склеить развороты, переставить и удалить: границы и отказы не меняют проект.
func TestAppSplitMergeMoveDelete(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a", "b", "c"}, []string{"d"})
	rev := func() int { return a.editor().View().Rev }

	// Деление: at вне 1..len−1 и неизвестный разворот — ничего.
	r := rev()
	for _, c := range []struct {
		id string
		at int
	}{{"s1", 0}, {"s1", 3}, {"s1", -1}, {"нет", 1}} {
		if v := a.SplitSpread(c.id, c.at); v.Rev != r || len(v.Spreads) != 2 {
			t.Fatalf("SplitSpread(%q, %d) изменил проект: rev %d→%d", c.id, c.at, r, v.Rev)
		}
	}
	v := a.SplitSpread("s1", 1)
	if len(v.Spreads) != 3 || !equalComposition(photosOf(v), []string{"a"}, []string{"b", "c"}, []string{"d"}) {
		t.Fatalf("SplitSpread: %v", photosOf(v))
	}
	tailID := v.Spreads[1].ID

	// Склейка возвращает состав; у последнего разворота «следующего» нет — отказ.
	r = rev()
	if v = a.MergeSpreads("s2"); v.Rev != r {
		// s2 — исходный «d»: следующего нет
		t.Fatalf("MergeSpreads последнего: rev %d→%d", r, v.Rev)
	}
	if v = a.MergeSpreads("нет"); v.Rev != r {
		t.Fatalf("MergeSpreads неизвестного: rev %d→%d", r, v.Rev)
	}
	v = a.MergeSpreads("s1")
	if !equalComposition(photosOf(v), []string{"a", "b", "c"}, []string{"d"}) {
		t.Fatalf("MergeSpreads: %v", photosOf(v))
	}
	if slices.ContainsFunc(v.Spreads, func(s model.SpreadView) bool { return s.ID == tailID }) {
		t.Error("хвостовой разворот остался после склейки")
	}

	// Вместе больше MaxSpreadPhotos — не склеивается.
	big := make([]string, model.MaxSpreadPhotos-1)
	for i := range big {
		big[i] = string(rune('k' + i))
	}
	b, _ := spreadsApp(t, big, []string{"y", "z"})
	rb := b.editor().View().Rev
	if v = b.MergeSpreads("s1"); v.Rev != rb || len(v.Spreads) != 2 {
		t.Fatalf("склейка сверх лимита прошла: rev %d→%d, разворотов %d", rb, v.Rev, len(v.Spreads))
	}

	// Перемещение: за край зажимается, неизвестный — ничего.
	v = a.MoveSpread("s1", 99)
	if v.Spreads[1].ID != "s1" {
		t.Fatalf("MoveSpread в конец: %v", photosOf(v))
	}
	v = a.MoveSpread("s1", -5)
	if v.Spreads[0].ID != "s1" {
		t.Fatalf("MoveSpread в начало: %v", photosOf(v))
	}
	r = rev()
	if v = a.MoveSpread("нет", 0); v.Rev != r {
		t.Fatalf("MoveSpread неизвестного изменил проект")
	}

	// Удаление: неизвестный — ничего, известный уходит и откатывается.
	if v = a.DeleteSpread("нет"); v.Rev != r || len(v.Spreads) != 2 {
		t.Fatalf("DeleteSpread неизвестного: rev %d→%d", r, v.Rev)
	}
	if v = a.DeleteSpread("s1"); len(v.Spreads) != 1 || v.Spreads[0].ID != "s2" {
		t.Fatalf("DeleteSpread: %v", photosOf(v))
	}
	if v = a.Undo(); len(v.Spreads) != 2 {
		t.Fatalf("Undo удаления: %d разворотов", len(v.Spreads))
	}
}

// Раскладки: галерея — только чтение (без правки и события), SetLayout/StepLayout меняют ключ,
// битый ключ, тот же ключ, чужой scope и неизвестный разворот — холостые.
func TestAppLayoutMethods(t *testing.T) {
	a, views := spreadsApp(t, []string{"a", "b", "c"})
	spread := func(v model.View) model.SpreadView { return v.Spreads[0] }

	events := len(views())
	ch := a.LayoutChoices("s1")
	if ch.Spread != "s1" || ch.Rev != a.editor().View().Rev || len(ch.Whole.InOrder) < 2 {
		t.Fatalf("LayoutChoices: разворот %q, rev %d, раскладок %d", ch.Spread, ch.Rev, len(ch.Whole.InOrder))
	}
	if len(views()) != events {
		t.Error("LayoutChoices прислал событие view")
	}
	if none := a.LayoutChoices("нет"); none.Spread != "нет" || len(none.Whole.InOrder) != 0 || none.Left != nil || none.Right != nil {
		t.Errorf("LayoutChoices неизвестного разворота: %+v", none)
	}

	cur := spread(a.editor().View()).Layout
	var key string
	for _, c := range ch.Whole.InOrder {
		if c.Key != cur {
			key = c.Key
			break
		}
	}
	if key == "" {
		t.Fatal("в галерее нет другой раскладки")
	}
	r := a.editor().View().Rev
	v := a.SetLayout("s1", key)
	if spread(v).Layout != key || v.Rev <= r {
		t.Fatalf("SetLayout: ключ %q (ждали %q), rev %d→%d", spread(v).Layout, key, r, v.Rev)
	}
	// Тот же ключ, битый ключ, неизвестный разворот.
	r = v.Rev
	for _, c := range [][2]string{{"s1", key}, {"s1", "битый"}, {"нет", key}} {
		if v = a.SetLayout(c[0], c[1]); v.Rev != r {
			t.Errorf("SetLayout(%q, %q) изменил проект", c[0], c[1])
		}
	}

	// Листание вперёд меняет ключ, назад — возвращает; delta 0, scope «не тот» и неизвестный разворот — холостые.
	v = a.StepLayout("s1", "spread", 1)
	if spread(v).Layout == key {
		t.Fatal("StepLayout вперёд не сменил раскладку")
	}
	if v = a.StepLayout("s1", "spread", -1); spread(v).Layout != key {
		t.Fatalf("StepLayout назад: %q, ждали %q", spread(v).Layout, key)
	}
	r = v.Rev
	for _, c := range []struct {
		id, scope string
		delta     int
	}{{"s1", "spread", 0}, {"s1", "всё", 1}, {"нет", "spread", 1}} {
		if v = a.StepLayout(c.id, c.scope, c.delta); v.Rev != r {
			t.Errorf("StepLayout(%q, %q, %d) изменил проект", c.id, c.scope, c.delta)
		}
	}
}

// Настройки: значения зажимаются в допустимые пределы, цвет нормализуется, Undo возвращает прежние.
func TestAppSetSettings(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a", "b"})
	before := a.editor().View()

	s := before.Settings
	s.PageW, s.PageH, s.Margin, s.Gap, s.DPI, s.Bg = 1e9, 250, 1e9, -4, 0, "#ABCDEF"
	r := before.Rev
	v := a.SetSettings(s)
	want := model.ClampSettings(s)
	if v.Settings != want || v.Rev <= r {
		t.Fatalf("SetSettings: %+v (ждали %+v), rev %d→%d", v.Settings, want, r, v.Rev)
	}
	if v.Settings.PageW >= 1e9 || v.Settings.Gap != 0 || v.Settings.DPI == 0 {
		t.Errorf("пределы не применены: %+v", v.Settings)
	}
	// Те же настройки — холостая правка.
	if v2 := a.SetSettings(v.Settings); v2.Rev != v.Rev {
		t.Errorf("повторные настройки — не холостая правка: rev %d→%d", v.Rev, v2.Rev)
	}
	if v = a.Undo(); v.Settings != before.Settings {
		t.Errorf("Undo настроек: %+v, было %+v", v.Settings, before.Settings)
	}
}
