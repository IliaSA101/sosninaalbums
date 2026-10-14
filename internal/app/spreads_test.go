package app

import (
	"fmt"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// spreadsApp — App с альбомом без папок: развороты s1, s2, … с заданным составом фото;
// фото именуются как в составе, события пишутся (имя → вид по порядку).
func spreadsApp(t *testing.T, spreads ...[]string) (a *App, views func() []model.View) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	views = collected[model.View](a, "view")
	p := model.New()
	for i, ids := range spreads {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i)), Photos: slices.Clone(ids)})
		for _, id := range ids {
			p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
		}
	}
	a.ed.Store(mustEditor(t, p, ""))
	return a, views
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
		full[i] = fmt.Sprintf("F%d", i) // 'A'+i на 64 фото дошло бы до «z»
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

	// Вместе больше MaxPhotos — не склеивается.
	big := make([]string, model.MaxPhotos-1)
	for i := range big {
		big[i] = fmt.Sprintf("k%d", i)
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

// Обложка (032): бросок фото на плитку создаёт обложку, фото остаётся в развороте; пресет,
// свои числа, надпись на корешке, разметка с проверкой файла и пропорций; отказ в проекте-листе.
func TestAppCover(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"}, []string{"b"})
	if n := len(a.CoverPresets()); n != 7 {
		t.Fatalf("пресетов %d", n)
	}
	v, err := a.CreateCover("", "a")
	if err != nil || v.Cover == nil || len(v.Cover.Sheet.Backdrops) != 1 || v.Cover.Sheet.Backdrops[0].Photo != "a" ||
		!equalComposition(photosOf(v), []string{"a"}, []string{"b"}) {
		t.Fatalf("обложка с фото: %v, %+v", err, v.Cover)
	}
	if _, err := a.CreateCover("", ""); err == nil {
		t.Fatal("вторая обложка без ошибки")
	}
	if _, err := a.CreateCover("", "нет такого"); err != errNoPhoto {
		t.Fatalf("неизвестное фото: %v", err)
	}
	if _, err := a.SetCoverLayout("bad"); err == nil {
		t.Fatal("неизвестная раскладка без ошибки")
	}
	if v, err = a.ApplyCoverPreset("netprint-royal"); err != nil || v.Cover.Preset != "netprint-royal" {
		t.Fatalf("пресет: %v %q", err, v.Cover.Preset)
	}
	sp := v.Cover.Spec
	sp.Wrap = 10
	if v, err = a.SetCoverSpec(sp); err != nil || v.Cover.Preset != model.CoverCustom {
		t.Fatalf("свои числа: %v %q", err, v.Cover.Preset)
	}
	if v, err = a.CoverSpineText(); err != nil || len(v.Cover.Sheet.Texts) != 1 || v.Cover.SpineBg == "" {
		t.Fatalf("надпись на корешке: %v %+v", err, v.Cover)
	}
	if v = a.SetCoverSpineBg("#000000"); v.Cover.SpineBg != "#000000" {
		t.Fatalf("плашка %q", v.Cover.SpineBg)
	}
	dir := testkit.TempDir(t)
	guide := testkit.PNG(t, dir, "guide.png", 600, 320, color.NRGBA{A: 255})
	v, err = a.setCoverGuide(guide)
	ratio := 600.0 / 320 / (v.Cover.Sheet.W / v.Cover.Sheet.H)
	if err != nil || v.Cover.Guide != "guide.png" || !v.Cover.GuideOK || v.Cover.GuideWarn != (math.Abs(ratio-1) > 0.01) {
		t.Fatalf("разметка: %v, %q ok=%v warn=%v (пропорции %v)", err, v.Cover.Guide, v.Cover.GuideOK, v.Cover.GuideWarn, ratio)
	}
	if _, err := a.setCoverGuide(filepath.Join(dir, "нет.png")); err == nil {
		t.Fatal("несуществующий файл разметки принят")
	}
	if v = a.ClearCoverGuide(); v.Cover.Guide != "" {
		t.Fatal("разметка не снята")
	}
	if v = a.DeleteCover(); v.Cover != nil || !equalComposition(photosOf(v), []string{"a"}, []string{"b"}) {
		t.Fatalf("обложка не снята: %+v", v.Cover)
	}
	a.NewVignetteSheet(204, 304)
	if _, err := a.CreateCover("", ""); err == nil || a.editor().View().Cover != nil {
		t.Fatalf("обложка в проекте-листе: %v", err)
	}
}

// /media/cover-guide отдаёт только файл разметки из проекта: пути из запроса не принимаются,
// без обложки или разметки, на чужое расширение и с лишним хвостом пути — 404.
func TestCoverGuideMedia(t *testing.T) {
	a, _ := spreadsApp(t, []string{"a"})
	h := a.media(http.NotFoundHandler())
	get := func(url string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		return w
	}
	if w := get("/media/cover-guide?r=1"); w.Code != http.StatusNotFound {
		t.Fatalf("без обложки: %d", w.Code)
	}
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	if w := get("/media/cover-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("без разметки: %d", w.Code)
	}
	dir := testkit.TempDir(t)
	guide := testkit.PNG(t, dir, "guide.png", 60, 30, color.NRGBA{R: 200, A: 255})
	if _, err := a.setCoverGuide(guide); err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(guide)
	if err != nil {
		t.Fatal(err)
	}
	w := get("/media/cover-guide?r=7")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/png" || !slices.Equal(w.Body.Bytes(), want) {
		t.Fatalf("разметка: %d %q, %d байт из %d", w.Code, w.Header().Get("Content-Type"), w.Body.Len(), len(want))
	}
	// Чужие пути: из запроса путь не берётся (отдаётся прежний файл), хвост пути — 404.
	other := testkit.PNG(t, dir, "other.png", 10, 10, color.NRGBA{B: 200, A: 255})
	if w := get("/media/cover-guide?path=" + other + "&r=8"); !slices.Equal(w.Body.Bytes(), want) {
		t.Fatal("путь из запроса принят")
	}
	for _, u := range []string{"/media/cover-guide/other.png", "/media/cover-guide/../x", "/media/cover-guide2"} {
		if w := get(u); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", u, w.Code)
		}
	}
	// Файл пропал или стал не картинкой — 404, а не чтение чужого.
	if err := os.WriteFile(guide, []byte("не png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := get("/media/cover-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("порченый файл: %d", w.Code)
	}
	if err := os.Remove(guide); err != nil {
		t.Fatal(err)
	}
	if w := get("/media/cover-guide"); w.Code != http.StatusNotFound {
		t.Fatalf("нет файла: %d", w.Code)
	}
}

// TestAppVignette — методы виньетки (031 G5): лист с виньеткой по умолчанию без шага отмены,
// превью листа без Split, «Применить» с тостом, новая виньетка, сортировка, «Обычный разворот».
func TestAppVignette(t *testing.T) {
	a, views := spreadsApp(t, []string{"a", "b", "c"})
	st := a.NewVignetteSheet(204, 304)
	sv := st.View.Spreads[0]
	if st.View.CanUndo || sv.Kind != model.KindVignette || sv.Vig == nil || sv.Vig.Places != 25 || len(sv.Places) != 25 {
		t.Fatalf("лист-виньетка: undo %v, %q %+v", st.View.CanUndo, sv.Kind, sv.Vig)
	}
	if cs := a.VignetteChoices(sv.ID, 1, 24, "3:4"); len(cs) != 17 {
		t.Fatalf("превью листа %d", len(cs))
	}
	r, err := a.SetVignette(sv.ID, model.Vignette{Tpl: "grid", Pupils: 10})
	if err != nil || r.Left != 0 || len(r.View.Spreads[0].Places) != 10 || !r.View.CanUndo {
		t.Fatalf("применить: %v, %+v", err, r.View.Spreads[0].Vig)
	}
	if _, err := a.SetVignette(sv.ID, model.Vignette{Tpl: "grid", Teachers: 18, Pupils: 64}); err == nil {
		t.Fatal("больше 64 мест принято")
	}
	// Окно «Настроить виньетку»: предпросмотр без правки и события, «Применить» — с событием.
	setup := model.VigSetup{Vig: model.Vignette{Tpl: "grid", Pupils: 12}, Vars: model.Vars{Title: "Лицей"}}
	events := len(views())
	pr, err := a.PreviewVignette(sv.ID, setup)
	if err != nil || len(pr.Spread.Places) != 12 || pr.Spread.Vars.Title != "Лицей" || len(views()) != events || a.editor().View().Rev != r.View.Rev {
		t.Fatalf("предпросмотр: %v, мест %d, событий %d", err, len(pr.Spread.Places), len(views())-events)
	}
	if _, err := a.PreviewVignette(sv.ID, model.VigSetup{Vig: model.Vignette{Tpl: "grid", Teachers: 18, Pupils: 64}}); err == nil {
		t.Fatal("предпросмотр больше 64 мест без ошибки")
	}
	r, err = a.SetupVignette(sv.ID, setup)
	if err != nil || len(r.View.Spreads[0].Places) != 12 || r.View.Spreads[0].Vars.Title != "Лицей" || len(views()) != events+1 {
		t.Fatalf("применить из окна: %v, событий %d", err, len(views())-events)
	}
	v, err := a.AddVignette(-1)
	if err != nil || len(v.Spreads) != 2 || v.Spreads[1].Kind != model.KindVignette {
		t.Fatalf("новая виньетка: %v, разворотов %d", err, len(v.Spreads))
	}
	rev := a.SortVignette(sv.ID).Rev // мест с фото нет — холостая
	if v = a.ClearVignette(sv.ID); v.Spreads[0].Kind != "" || v.Spreads[0].Vig != nil || v.Rev != rev+1 {
		t.Fatalf("обычный разворот: %q, rev %d → %d", v.Spreads[0].Kind, rev, v.Rev)
	}
}

// missInfos — фото names в папке cfgDir (файлов нет, наличие решает a.exists) и их ID.
func missInfos(a *App, names ...string) (infos []model.PhotoInfo, ids []string) {
	for _, n := range names {
		p := filepath.Join(a.cfgDir, n)
		infos = append(infos, model.PhotoInfo{ID: library.IDFor(p), Path: p, W: 30, H: 20})
		ids = append(ids, library.IDFor(p))
	}
	return infos, ids
}

// missAdd — правка, как у методов App (через changed): новый разворот из фото infos.
func missAdd(a *App, infos []model.PhotoInfo) {
	v, _ := a.editor().NewSpread(-1, infos) // отказ — прежний вид, тест увидит по списку
	a.changed(v)
}

func sorted(ids ...string) []string {
	slices.Sort(ids)
	return ids
}

// Две правки из разных горутин: первая прочитала набор фото и стоит, вторая уже в проекте.
// Снимок первой не ложится поверх снимка второй — ни фантома в списке, ни спрятанного пропавшего.
func TestMissingSnapshotOrder(t *testing.T) {
	for _, tc := range []struct{ name, add string }{{"убрали второе", ""}, {"добавили новое", "w.jpg"}} {
		t.Run(tc.name, func(t *testing.T) {
			a, x, y, _ := missSetApp(t)
			gate, read := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			a.snapRefs = func(ed *album.Editor) map[string]string {
				m := editorPaths(ed)
				if calls.Add(1) == 1 {
					close(read)
					<-gate
				}
				return m
			}
			want := []string{}
			second := func() { a.RemovePhotos([]string{y}) }
			if tc.add != "" {
				infos, ids := missInfos(a, tc.add)
				want = sorted(ids[0], y)
				second = func() { missAdd(a, infos) }
			}
			rev := a.editor().View().Rev
			var wg sync.WaitGroup
			wg.Go(func() { a.RemovePhotos([]string{x}) })
			<-read
			wg.Go(second)
			// Вторая правка в проекте: её syncMissing ждёт первую (без замка — обгонял бы).
			for deadline := time.Now().Add(10 * time.Second); a.editor().View().Rev < rev+2; time.Sleep(time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatal("вторая правка не дошла до проекта")
				}
			}
			close(gate)
			wg.Wait()
			waitMissing(t, a)
			if m := a.GetState().Missing; !slices.Equal(m, want) {
				t.Fatalf("недостающие %v, ждали %v", m, want)
			}
		})
	}
}

// После правки фон проверяет только новые фото и фото с новым путём, а не весь альбом, и
// сливает итог с уже известными; правка без новых фото файлы не проверяет и событий не шлёт.
func TestMissingChecksOnlyNew(t *testing.T) {
	a, x, y, events := missSetApp(t)
	var mu sync.Mutex
	var stat []string
	a.exists = func(p string) bool {
		mu.Lock()
		stat = append(stat, filepath.Base(p))
		mu.Unlock()
		return strings.HasPrefix(filepath.Base(p), "ok")
	}
	check := func(step string, wantStat []string, want []string) {
		t.Helper()
		waitMissing(t, a)
		mu.Lock()
		got := slices.Sorted(slices.Values(stat))
		stat = nil
		mu.Unlock()
		if !slices.Equal(got, wantStat) {
			t.Fatalf("%s: проверены %v, ждали %v", step, got, wantStat)
		}
		if m := a.GetState().Missing; !slices.Equal(m, want) {
			t.Fatalf("%s: недостающие %v, ждали %v", step, m, want)
		}
	}
	infos, ids := missInfos(a, "w.jpg", "ok2.jpg")
	w := ids[0]
	missAdd(a, infos)
	check("добавили w и ok2", []string{"ok2.jpg", "w.jpg"}, sorted(w, x, y))
	a.changed(a.editor().Relink(map[string]string{w: filepath.Join(a.cfgDir, "ok3.jpg")}, ""))
	check("w нашлось", []string{"ok3.jpg"}, sorted(x, y))
	a.Undo()
	check("отмена перепривязки", []string{"w.jpg"}, sorted(w, x, y))
	n := len(events())
	if _, err := a.NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	check("пустой разворот", nil, sorted(w, x, y))
	if len(events()) != n {
		t.Fatalf("правка без новых фото: событий missing %d", len(events())-n)
	}
}
