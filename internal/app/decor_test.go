package app

import (
	"context"
	"errors"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// decorApp — App с двумя разворотами и открытой библиотекой в своей папке настроек.
// Искатель salbum — общий на процесс: снимается в конце теста.
func decorApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2"}}
	a.ed.Store(mustEditor(t, p, ""))
	a.openDecor()
	t.Cleanup(func() { salbum.SetLibFinder(nil); salbum.SetFrameFinder(nil) })
	return a
}

// importBg — свой фон через ImportDecorPaths; ID плитки (my:<hex>).
func importBg(t *testing.T, a *App, files ...string) []string {
	t.Helper()
	res, err := a.ImportDecorPaths(decor.KindBg, "mine", files)
	if err != nil || len(res.Skipped) != 0 {
		t.Fatalf("загрузка: %v %+v", err, res.Skipped)
	}
	ids := make([]string, len(res.Added))
	for i, it := range res.Added {
		ids[i] = it.ID
	}
	return ids
}

// Без библиотеки (startup не дошёл) — ошибка, а не паника и не пустой nil-срез в интерфейс.
func TestDecorNoLibrary(t *testing.T) {
	a := bgApp(t)
	cat, err := a.DecorCatalog(decor.KindBg)
	if !errors.Is(err, errNoLibrary) || cat.Items == nil || cat.Themes == nil || cat.Recent == nil {
		t.Fatalf("каталог без библиотеки: %v %+v", err, cat)
	}
	rev := a.editor().View().Rev
	if v, err := a.ApplyBackground("s1", "spread", "sa:confetti"); !errors.Is(err, errNoLibrary) || v.Rev != rev {
		t.Fatalf("плитка без библиотеки: %v rev %d→%d", err, rev, v.Rev)
	}
	if res, err := a.ImportDecorPaths(decor.KindBg, "mine", nil); !errors.Is(err, errNoLibrary) || res.Added == nil {
		t.Fatalf("загрузка без библиотеки: %v %+v", err, res)
	}
	if a.RenameDecor(decor.KindBg, "my:0123456789abcdef", "x") == nil || a.HideDecor(decor.KindBg, "my:0123456789abcdef") == nil {
		t.Fatal("✎/× без библиотеки — без ошибки")
	}
	rec := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", "/media/gen/pat?k=dots&w=10&h=10&v=1&px=64", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("/media/gen без библиотеки: %d", rec.Code)
	}
}

// Каталог: встроенные фоны с плиткой в пропорции разворота, свои, недавние после щелчка.
func TestDecorCatalog(t *testing.T) {
	a := decorApp(t)
	cat, err := a.DecorCatalog(decor.KindBg)
	if err != nil || cat.Kind != decor.KindBg || len(cat.Recent) != 0 || cat.ReadOnly {
		t.Fatalf("каталог: %v %+v", err, cat)
	}
	i := slices.IndexFunc(cat.Items, func(it DecorItem) bool { return it.ID == "sa:confetti" })
	if i < 0 || cat.Items[i].Own || cat.Items[i].Color == "" {
		t.Fatalf("встроенный фон: %+v", cat.Items)
	}
	// Разворот 300×300 мм на страницу — плитка 600×300.
	if u := cat.Items[i].URL; !strings.Contains(u, "w=600") || !strings.Contains(u, "h=300") {
		t.Errorf("плитка не в пропорции разворота: %s", u)
	}
	if len(cat.Themes) == 0 || cat.Themes[0].ID != decor.ThemeRecent && cat.Themes[0].ID != decor.ThemeAll {
		t.Errorf("темы: %+v", cat.Themes)
	}
	// Проект из листов (031) — плитка в пропорции листа.
	sheet := model.New()
	sheet.Settings.Sheet, sheet.Settings.PageW, sheet.Settings.PageH = true, 204, 304
	b := decorApp(t)
	b.ed.Store(mustEditor(t, sheet, ""))
	if c, _ := b.DecorCatalog(decor.KindBg); !strings.Contains(c.Items[0].URL, "w=204&h=304") {
		t.Errorf("плитка листа: %s", c.Items[0].URL)
	}
	if _, err := a.DecorCatalog("обои"); err == nil {
		t.Error("неизвестный вид принят")
	}
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "Мой фон.jpg", 60, 40))
	for _, id := range []string{"sa:confetti", ids[0], "sa:gold-dots"} {
		if _, err := a.ApplyBackground("s1", "spread", id); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	cat, _ = a.DecorCatalog(decor.KindBg)
	if want := []string{"sa:gold-dots", ids[0], "sa:confetti"}; !slices.Equal(cat.Recent, want) {
		t.Errorf("недавние %v, ждали %v", cat.Recent, want)
	}
	own := cat.Items[slices.IndexFunc(cat.Items, func(it DecorItem) bool { return it.ID == ids[0] })]
	if !own.Own || own.Name != "Мой фон" || own.W != 60 || own.H != 40 || own.Aspect != 1.5 {
		t.Errorf("своя плитка: %+v", own)
	}
}

// Плитка фона: узор, его отметка плитки (решение №201) и цвет страниц — один шаг отмены, повтор
// — холостой, круг Save→Load; разные развороты и страницы — независимо.
func TestApplyBackgroundPreset(t *testing.T) {
	a := decorApp(t)
	v0 := a.editor().View()
	v, err := a.ApplyBackground("s1", "spread", "sa:cream-gradient")
	if err != nil || len(v.Spreads[0].Patterns) != 1 || v.Spreads[0].Patterns[0].Pat.Kind != "grad-h" ||
		v.Spreads[0].Patterns[0].Preset != "sa:cream-gradient" || v.Spreads[0].Bg[0] != "#fbf6ef" || v.Spreads[0].Bg[1] != "#fbf6ef" || len(v.Spreads[1].Patterns) != 0 {
		t.Fatalf("узор на разворот: %v %+v", err, v.Spreads[0])
	}
	if again, _ := a.ApplyBackground("s1", "spread", "sa:cream-gradient"); again.Rev != v.Rev {
		t.Errorf("повтор той же плитки — не холостой: rev %d→%d", v.Rev, again.Rev)
	}
	v, err = a.ApplyBackground("s2", "left", "sa:gold-dots")
	if err != nil {
		t.Fatal(err)
	}
	v, _ = a.ApplyBackground("s2", "right", "sa:night-stars")
	if ps := v.Spreads[1].Patterns; len(ps) != 2 || ps[0].Scope != "left" || ps[1].Scope != "right" || v.Spreads[1].Bg[1] != "#1f2a4d" {
		t.Fatalf("узоры страниц: %+v", ps)
	}
	if v.Spreads[0].Patterns[0].Pat.Kind != "grad-h" {
		t.Error("правка второго разворота задела первый")
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := p.Spreads[1]; s.PatL == nil || s.PatL.Kind != "dots" || s.PatL.Preset != "sa:gold-dots" || s.PatR == nil ||
		s.PatR.Kind != "stars" || s.PatR.Preset != "sa:night-stars" || p.Spreads[0].Pat == nil {
		t.Fatalf("после Load: %+v %+v", p.Spreads[0], s)
	}
	for range 3 {
		v = a.Undo()
	}
	if len(v.Spreads[0].Patterns) != 0 || v.Spreads[0].Bg != v0.Spreads[0].Bg || v.CanUndo {
		t.Fatalf("три шага отмены не вернули альбом: %+v canUndo=%v", v.Spreads[0], v.CanUndo)
	}
	for _, c := range []struct{ spread, scope, id string }{
		{"s1", "top", "sa:confetti"}, {"s1", "spread", "sa:нет-такого"}, {"s1", "spread", "my:0123456789abcdef"}, {"s1", "spread", ""},
	} {
		if v, err := a.ApplyBackground(c.spread, c.scope, c.id); err == nil || v.Rev != a.editor().View().Rev {
			t.Errorf("%+v: ошибки нет", c)
		}
	}
	if r := prefs.RecentDecor(a.cfgDir, decor.KindBg); slices.Contains(r, "sa:нет-такого") || len(r) != 3 {
		t.Errorf("недавние после отказов: %v", r)
	}
}

// Свой фон: фото фона с ключом, узор области снимается; в файле — ключ, после Load путь
// находит искатель.
func TestApplyBackgroundOwn(t *testing.T) {
	a := decorApp(t)
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "свой.jpg", 50, 50))
	if _, err := a.ApplyBackground("s1", "spread", "sa:confetti"); err != nil {
		t.Fatal(err)
	}
	v, err := a.ApplyBackground("s1", "spread", ids[0])
	if err != nil || len(v.Spreads[0].Patterns) != 0 || len(v.Spreads[0].Backdrops) != 1 || v.Spreads[0].Backdrops[0].Lib != ids[0] {
		t.Fatalf("свой фон: %v %+v", err, v.Spreads[0])
	}
	if !slices.Equal(v.LibPhotos, []string{model.LibPhotoID(ids[0])}) {
		t.Errorf("LibPhotos: %v", v.LibPhotos)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := p.Photos[model.LibPhotoID(ids[0])]
	want, _ := a.decorLib.Load().Find(ids[0])
	if !ok || ref.Lib != ids[0] || ref.Path != want || want == "" {
		t.Fatalf("после Load: %+v, ждали путь %q", ref, want)
	}
	if v = a.Undo(); len(v.Spreads[0].Backdrops) != 0 || len(v.Spreads[0].Patterns) != 1 {
		t.Fatalf("отмена своего не вернула узор: %+v", v.Spreads[0])
	}
}

// Корешок обложки (061) в галерее фонов: своё фото и «Свой файл…» ложатся на корешок, узор
// программы — нет (ошибка области без шага); на развороте корешка нет.
func TestApplyBackgroundSpine(t *testing.T) {
	a := decorApp(t)
	dir := testkit.TempDir(t)
	ids := importBg(t, a, testkit.JPEG(t, dir, "свой.jpg", 50, 50))
	if _, err := a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	if v, err := a.ApplyBackground(model.CoverID, model.ScopeSpine, "sa:confetti"); !errors.Is(err, errBackdropScope) || v.Rev != rev {
		t.Fatalf("узор на корешок: %v rev %d→%d", err, rev, v.Rev)
	}
	if v, err := a.ApplyBackground("s1", model.ScopeSpine, ids[0]); !errors.Is(err, model.ErrScope) || v.Rev != rev {
		t.Fatalf("корешок у разворота: %v rev %d→%d", err, rev, v.Rev)
	}
	v, err := a.ApplyBackground(model.CoverID, model.ScopeSpine, ids[0])
	if bs := v.Cover.Sheet.Backdrops; err != nil || len(bs) != 1 || bs[0].Scope != model.ScopeSpine || bs[0].Lib != ids[0] {
		t.Fatalf("своё на корешок: %v %+v", err, bs)
	}
	v, err = a.setBackdropFile(model.CoverID, model.ScopeSpine, testkit.JPEG(t, dir, "файл.jpg", 40, 60))
	if bs := v.Cover.Sheet.Backdrops; err != nil || len(bs) != 1 || bs[0].Scope != model.ScopeSpine || bs[0].Lib != "" {
		t.Fatalf("«Свой файл…» на корешок: %v %+v", err, bs)
	}
}

// Альбом с чужого компьютера: своего нет в библиотеке — путь пуст; загрузили тот же файл —
// нашлось само, одним шагом отмены. Ничего не нашлось — шага нет.
func TestImportDecorRelinks(t *testing.T) {
	src := testkit.TempDir(t)
	files := []string{testkit.JPEG(t, src, "a.jpg", 40, 30), testkit.JPEG(t, src, "b.jpg", 30, 40)}
	a := decorApp(t)
	ids := importBg(t, a, files...)
	for i, sc := range []struct{ spread, scope string }{{"s1", "spread"}, {"s2", "left"}} {
		if _, err := a.ApplyBackground(sc.spread, sc.scope, ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}

	b := decorApp(t) // другой компьютер: своя пустая библиотека
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b.ed.Store(mustEditor(t, p, path))
	for _, id := range ids {
		if r, _ := b.editor().Ref(model.LibPhotoID(id)); r.Path != "" || r.Lib != id {
			t.Fatalf("своё нашлось без библиотеки: %+v", r)
		}
	}
	rev := b.editor().View().Rev
	importBg(t, b, testkit.JPEG(t, testkit.TempDir(t), "чужое.jpg", 20, 20))
	if v := b.editor().View(); v.Rev != rev || v.CanUndo {
		t.Fatalf("загрузка без своих альбома — правка: rev %d→%d", rev, v.Rev)
	}
	importBg(t, b, files...)
	for _, id := range ids {
		r, _ := b.editor().Ref(model.LibPhotoID(id))
		if want, _ := b.decorLib.Load().Find(id); r.Path == "" || r.Path != want {
			t.Fatalf("не нашлось само: %+v, ждали %q", r, want)
		}
	}
	b.Undo()
	if r, _ := b.editor().Ref(model.LibPhotoID(ids[0])); r.Path != "" {
		t.Errorf("отмена не вернула пустой путь: %+v", r)
	}

	// Те же файлы снова — «уже есть», ничего не добавлено, но своё альбома нашлось и не пропавшее.
	waitMissing(t, b)
	if m := b.GetState().Missing; !slices.Contains(m, model.LibPhotoID(ids[0])) {
		t.Fatalf("после отмены своё не в пропавших: %v", m)
	}
	if n := b.MissingNames(); len(n) != 0 {
		t.Errorf("имена пропавших со своим: %v", n)
	}
	res, err := b.ImportDecorPaths(decor.KindBg, "mine", files)
	if err != nil || len(res.Added) != 0 || len(res.Skipped) != 2 {
		t.Fatalf("повтор: %v %+v", err, res)
	}
	for _, id := range ids {
		if r, _ := b.editor().Ref(model.LibPhotoID(id)); r.Path == "" {
			t.Fatalf("«уже есть» — своё не нашлось: %+v", r)
		}
	}
	waitMissing(t, b)
	if m := b.GetState().Missing; len(m) != 0 {
		t.Errorf("своё осталось пропавшим: %v", m)
	}
}

// Файл своего стёрт при запущенной программе (пропавшее) и загружен снова: путь тот же, правки
// нет, но пропавшим своё больше не числится. Имени для «Показать» у своего нет.
func TestImportDecorRestoresMissing(t *testing.T) {
	src := testkit.TempDir(t)
	file := testkit.JPEG(t, src, "a.jpg", 40, 30)
	a := decorApp(t)
	ids := importBg(t, a, file)
	if _, err := a.ApplyBackground("s1", "spread", ids[0]); err != nil {
		t.Fatal(err)
	}
	pid := model.LibPhotoID(ids[0])
	lib, _ := a.decorLib.Load().Find(ids[0])
	testkit.RemoveAll(t, lib)
	a.refreshMissing()
	waitMissing(t, a)
	if m := a.GetState().Missing; !slices.Equal(m, []string{pid}) {
		t.Fatalf("стёртое своё не пропавшее: %v", m)
	}
	if n := a.MissingNames(); len(n) != 0 {
		t.Errorf("имя своего в списке пропавших: %v", n)
	}
	rev := a.editor().View().Rev
	importBg(t, a, file)
	waitMissing(t, a)
	if m := a.GetState().Missing; len(m) != 0 {
		t.Errorf("загрузили снова — своё всё ещё пропавшее: %v", m)
	}
	if v := a.editor().View(); v.Rev != rev {
		t.Errorf("холостая перепривязка дала правку: rev %d→%d", rev, v.Rev)
	}
}

// «Загрузить своё…»: отказ от диалога — пустой итог; неизвестный вид — ошибка до диалога;
// выбор — загрузка и папка диалога в config.json; у стикеров — свой заголовок, SVG и своя папка.
func TestImportDecorDialog(t *testing.T) {
	a := decorApp(t)
	dir := testkit.TempDir(t)
	file := testkit.JPEG(t, dir, "фон.jpg", 40, 30)
	old := openFilesDialog
	t.Cleanup(func() { openFilesDialog = old })
	var pick []string
	calls := 0
	var opts wr.OpenDialogOptions
	openFilesDialog = func(_ context.Context, o wr.OpenDialogOptions) ([]string, error) { calls++; opts = o; return pick, nil }
	if res, err := a.ImportDecor(decor.KindBg, "mine"); err != nil || len(res.Added) != 0 || res.Added == nil || res.Skipped == nil || res.Svg == nil {
		t.Fatalf("отказ от диалога: %v %+v", err, res)
	}
	if _, err := a.ImportDecor("xx", "mine"); !errors.Is(err, decor.ErrKind) || calls != 1 {
		t.Fatalf("неизвестный вид: %v, диалогов %d", err, calls)
	}
	pick = []string{file, filepath.Join(dir, "нет.jpg")}
	res, err := a.ImportDecor(decor.KindBg, "wedding")
	if err != nil || len(res.Added) != 1 || res.Added[0].Theme != "wedding" || len(res.Skipped) != 1 || res.Skipped[0].Name != "нет" {
		t.Fatalf("выбор: %v %+v", err, res)
	}
	if got := prefs.DialogDir(a.cfgDir, dirDecor); got != dir {
		t.Errorf("папка диалога %q, ждали %q", got, dir)
	}
	sdir := testkit.TempDir(t)
	pick = []string{testkit.PNG(t, sdir, "стикер.png", 20, 20, color.NRGBA{B: 200, A: 255})}
	res, err = a.ImportDecor(decor.KindSticker, "mine")
	if err != nil || len(res.Added) != 1 || opts.Title != "Свои стикеры" || len(opts.Filters) != 1 || !strings.Contains(opts.Filters[0].Pattern, "*.svg") {
		t.Fatalf("стикеры: %v %+v, диалог %+v", err, res, opts)
	}
	if got := prefs.DialogDir(a.cfgDir, dirSticker); got != sdir {
		t.Errorf("папка стикеров %q, ждали %q", got, sdir)
	}
	if got := prefs.DialogDir(a.cfgDir, dirDecor); got != dir {
		t.Errorf("стикеры сдвинули папку фонов: %q", got)
	}
}

// ✎ и × своего: встроенные не меняются; скрытое уходит из каталога, файл остаётся.
func TestRenameHideDecor(t *testing.T) {
	a := decorApp(t)
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 40, 30), testkit.JPEG(t, testkit.TempDir(t), "b.jpg", 30, 40))
	if err := a.RenameDecor(decor.KindBg, ids[0], "  Новое  имя "); err != nil {
		t.Fatal(err)
	}
	if err := a.RenameDecor(decor.KindBg, "sa:confetti", "x"); !errors.Is(err, decor.ErrBuiltin) {
		t.Errorf("✎ встроенного: %v", err)
	}
	if err := a.RenameDecor(decor.KindBg, ids[1], " "); !errors.Is(err, decor.ErrName) {
		t.Errorf("пустое имя: %v", err)
	}
	if err := a.HideDecor(decor.KindBg, ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := a.HideDecor(decor.KindBg, "sa:confetti"); !errors.Is(err, decor.ErrBuiltin) {
		t.Errorf("× встроенного: %v", err)
	}
	cat, _ := a.DecorCatalog(decor.KindBg)
	names := map[string]string{}
	for _, it := range cat.Items {
		names[it.ID] = it.Name
	}
	if names[ids[0]] != "Новое имя" {
		t.Errorf("имя после ✎: %q", names[ids[0]])
	}
	if _, ok := names[ids[1]]; ok {
		t.Error("скрытое осталось в каталоге")
	}
	if _, ok := a.decorLib.Load().Find(ids[1]); !ok {
		t.Error("× удалил файл")
	}
}

// Правки узора: серия SetPattern с одним merge — один шаг; ClearPattern — шаг, повтор — холостой.
func TestSetClearPattern(t *testing.T) {
	a := decorApp(t)
	v, _ := a.ApplyBackground("s1", "spread", "sa:gold-dots")
	base := v.Spreads[0].Patterns[0].Pat
	pt := base
	pt.Fade, pt.Kind = 0.3, "stripes" // ключ генератора SetPattern не меняет (№172)
	a.SetPattern("s1", "spread", pt, "m1")
	pt.Fade = 0.5
	v = a.SetPattern("s1", "spread", pt, "m1")
	if got := v.Spreads[0].Patterns[0].Pat; got.Fade != 0.5 || got.Kind != base.Kind {
		t.Fatalf("серия узора: %+v", got)
	}
	if v = a.Undo(); v.Spreads[0].Patterns[0].Pat.Fade != base.Fade {
		t.Fatalf("серия — не один шаг: %+v", v.Spreads[0].Patterns[0].Pat)
	}
	v = a.ClearPattern("s1", "spread")
	if len(v.Spreads[0].Patterns) != 0 {
		t.Fatal("узор не снят")
	}
	if again := a.ClearPattern("s1", "spread"); again.Rev != v.Rev {
		t.Errorf("снятие без узора — не холостое: rev %d→%d", v.Rev, again.Rev)
	}
	if again := a.SetPattern("s1", "top", pt, ""); again.Rev != v.Rev {
		t.Error("неверная область принята")
	}
}

// /media/gen и /media/lib идут в библиотеку, а не в миниатюры фото.
func TestMediaDecor(t *testing.T) {
	a := decorApp(t)
	cat, _ := a.DecorCatalog(decor.KindBg)
	h := a.media(http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", cat.Items[0].URL+"&px=64", nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("%s: %d %q", cat.Items[0].URL, rec.Code, rec.Header().Get("Content-Type"))
	}
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 40, 30))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", decor.LibURL(ids[0], false), nil))
	if rec.Code != http.StatusNotFound { // кэша миниатюр нет — 404, а не паника
		t.Errorf("/media/lib без кэша: %d", rec.Code)
	}
}

// Поиск пропавших в папке и на другом диске своих из библиотеки не берёт.
func TestMissingOfSkipsLib(t *testing.T) {
	refs := map[string]model.PhotoRef{
		"a": {Path: `C:\нет\a.jpg`},
		"b": {Path: "", Lib: "my:0123456789abcdef"},
		"c": {Path: `C:\нет\c.jpg`, Lib: "my:fedcba9876543210"},
		"d": {Path: `C:\есть\d.jpg`},
	}
	var asked []string
	got := missingOf(refs, func(p string) bool { asked = append(asked, p); return strings.Contains(p, "есть") })
	if !slices.Equal(got, []string{"a"}) {
		t.Errorf("пропавшие %v, ждали [a]", got)
	}
	if slices.Contains(asked, "") {
		t.Error("пустой путь проверялся на диске")
	}
	if got := missingOf(nil, func(string) bool { return false }); len(got) != 0 {
		t.Errorf("пустой альбом: %v", got)
	}
}

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
	if m := v.Cover.Markup; err != nil || m == nil || m.Name != "guide.png" || !m.OK || (m.Warn != "") != (math.Abs(ratio-1) > 0.01) {
		t.Fatalf("разметка: %v, %+v (пропорции %v)", err, m, ratio)
	}
	if _, err := a.setCoverGuide(filepath.Join(dir, "нет.png")); err == nil {
		t.Fatal("несуществующий файл разметки принят")
	}
	if v = a.ClearCoverGuide(); v.Cover.Markup != nil {
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
