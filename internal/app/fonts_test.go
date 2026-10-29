package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	"github.com/IliaSA101/sosninaalbums/internal/text/fontlib"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Свои шрифты в App (082 G4, design §6): загрузка, откат, ×, /media/font, открытие собранного
// альбома. Реестр своих гарнитур text — общий на процесс: каждый тест снимает его в конце.

// fontEnv — App с библиотекой шрифтов в своей папке данных; события `fonts` — в fonts.
type fontEnv struct {
	a     *App
	src   string // папка исходных файлов шрифтов
	fonts [][]text.Family
}

func newFontEnv(t *testing.T, p *model.Project) *fontEnv {
	t.Helper()
	e := &fontEnv{a: newApp(t), src: testkit.TempDir(t)}
	e.a.cfgDir = testkit.TempDir(t)
	e.a.emit = func(name string, data ...any) {
		if name == "fonts" {
			e.fonts = append(e.fonts, data[0].([]text.Family))
		}
	}
	if p == nil {
		p = model.New()
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	e.a.openFonts()
	return e
}

// file пишет байты встроенного начертания builtin (или data) в папку исходных под именем name.
func (e *fontEnv) file(t *testing.T, name, builtin string, data []byte) string {
	t.Helper()
	if builtin != "" {
		data = fontBytes(t, builtin)
	}
	p := filepath.Join(e.src, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fontBytes(t *testing.T, builtin string) []byte {
	t.Helper()
	b, ok := text.FontFile(builtin)
	if !ok {
		t.Fatalf("нет встроенного %s", builtin)
	}
	return b
}

// familyOf — id и название гарнитуры, которую библиотека даст файлу builtin.
func familyOf(t *testing.T, builtin string) (id, name string) {
	t.Helper()
	info, err := text.Inspect(fontBytes(t, builtin))
	if err != nil {
		t.Fatal(err)
	}
	return text.FamilyID(info.Family), info.Family
}

// restricted — копия файла с OS/2.fsType = 2 (лицензия запрещает перенос): другие байты — другой ключ.
func restricted(t *testing.T, b []byte) []byte {
	t.Helper()
	out := append([]byte(nil), b...)
	for i := range int(binary.BigEndian.Uint16(out[4:])) {
		if rec := out[12+16*i:]; string(rec[:4]) == "OS/2" {
			binary.BigEndian.PutUint16(out[binary.BigEndian.Uint32(rec[8:])+8:], 2)
			return out
		}
	}
	t.Fatal("нет таблицы OS/2")
	return nil
}

// fontBlock — текст «по тексту» своей гарнитурой id с шириной w (как записал другой компьютер).
func fontBlock(id, block string, w float64) *model.TextBlock {
	return &model.TextBlock{ID: block, X: 20, Y: 20, W: w, Auto: true, Text: "Анна и Павел",
		Style: model.TextStyle{Font: id, Size: 20, Color: "#222222", Align: "left"}}
}

// myFamily — своя гарнитура id в списке; nil — нет.
func myFamily(list []text.Family, id string) *text.Family {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// TestImportFontPathsEmitsFonts — загрузка: итог для тоста (гарнитура с двумя начертаниями, ключи
// новых файлов, пропуск с причиной), одно событие `fonts` с полным списком, тексты двух разворотов,
// ждавшие гарнитуру, пересчитаны одним шагом отмены. Повтор и пустой список — без события и шага.
func TestImportFontPathsEmitsFonts(t *testing.T) {
	osw, oswName := familyOf(t, "Oswald-Regular.ttf")
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Texts: []*model.TextBlock{fontBlock(osw, "0a0b0c0d0e01", 100)}},
		{ID: "s2", Texts: []*model.TextBlock{fontBlock(osw, "0a0b0c0d0e02", 100)}}}
	p.Fonts = map[string]model.FontRef{osw: {Name: oswName}}
	e := newFontEnv(t, p)
	files := []string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil), e.file(t, "ob.ttf", "Oswald-Bold.ttf", nil),
		e.file(t, "Lato.woff2", "", []byte("wOF2 не шрифт"))}

	res, err := e.a.ImportFontPaths(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Faces) != 2 || !faceFile.MatchString(res.Faces[0]) || !faceFile.MatchString(res.Faces[1]) {
		t.Fatalf("начертания: %v", res.Faces)
	}
	// Files — те же два файла этой загрузки (порядок — гарнитуры, не загрузки)
	if ad := res.Added; len(ad) != 1 || ad[0].ID != osw || ad[0].Name != oswName || ad[0].Faces != 2 || ad[0].NoCyr ||
		!slices.Equal(slices.Sorted(slices.Values(ad[0].Files)), slices.Sorted(slices.Values(res.Faces))) {
		t.Fatalf("добавлено: %+v", res.Added)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].Name != "Lato.woff2" || res.Skipped[0].Why == "" {
		t.Fatalf("пропуски: %+v", res.Skipped)
	}
	if len(e.fonts) != 1 {
		t.Fatalf("событий fonts: %d", len(e.fonts))
	}
	f := myFamily(e.fonts[0], osw)
	if f == nil || f.Group != text.CustomGroup || len(f.Faces) != 2 || len(e.fonts[0]) != len(text.Families()) {
		t.Fatalf("событие: %+v", f)
	}
	if myFamily(e.a.GetState().Fonts, osw) == nil {
		t.Fatal("State.Fonts без своей гарнитуры")
	}
	v := e.a.editor().View()
	w1, w2 := v.Spreads[0].Texts[0].W, v.Spreads[1].Texts[0].W
	if w1 == 100 || w2 == 100 || !v.CanUndo || !v.Dirty {
		t.Fatalf("ширины не пересчитаны: %v %v, отмена %v", w1, w2, v.CanUndo)
	}
	if u := e.a.Undo(); u.Spreads[0].Texts[0].W != 100 || u.Spreads[1].Texts[0].W != 100 || u.CanUndo { // один шаг на оба
		t.Fatalf("Undo: %v %v, ещё отмена %v", u.Spreads[0].Texts[0].W, u.Spreads[1].Texts[0].W, u.CanUndo)
	}
	e.a.Redo()

	// Повтор тех же файлов — «уже есть», ни события, ни шага; пустой список — пустой итог.
	rev := e.a.editor().View().Rev
	res, err = e.a.ImportFontPaths(files[:2])
	if err != nil || len(res.Added) != 0 || len(res.Faces) != 0 || len(res.Skipped) != 2 {
		t.Fatalf("повтор: %+v %v", res, err)
	}
	res, err = e.a.ImportFontPaths(nil)
	if err != nil || res.Added == nil || res.Faces == nil || res.Skipped == nil || len(res.Added)+len(res.Skipped) != 0 {
		t.Fatalf("пустой список: %#v %v", res, err)
	}
	if len(e.fonts) != 1 || e.a.editor().View().Rev != rev {
		t.Fatalf("повтор дал событие или правку: событий %d", len(e.fonts))
	}
	// Возврат из скрытых — гарнитура в Added, но файлов эта загрузка не писала: Files пуст, не nil.
	if err := e.a.HideFont(osw); err != nil {
		t.Fatal(err)
	}
	res, err = e.a.ImportFontPaths(files[:1])
	if err != nil || len(res.Added) != 1 || res.Added[0].Files == nil || len(res.Added[0].Files) != 0 || len(res.Faces) != 0 {
		t.Fatalf("возврат из скрытых: %+v %v", res, err)
	}
}

// TestImportFontsReadOnly — библиотека новее программы: ошибка до диалога (выбор не пропадает зря);
// без библиотеки — своя ошибка; закрытый диалог — пустой итог; выбор — загрузка и папка запомнена.
func TestImportFontsReadOnly(t *testing.T) {
	calls := 0
	var opts wr.OpenDialogOptions
	var pick []string
	old := openFilesDialog
	openFilesDialog = func(_ context.Context, o wr.OpenDialogOptions) ([]string, error) { calls++; opts = o; return pick, nil }
	t.Cleanup(func() { openFilesDialog = old })

	bare := newApp(t)
	if res, err := bare.ImportFonts(); !errors.Is(err, errNoFonts) || res.Added == nil || calls != 0 {
		t.Fatalf("без библиотеки: %v %+v, диалогов %d", err, res, calls)
	}
	if _, err := bare.ImportFontPaths([]string{"x.ttf"}); !errors.Is(err, errNoFonts) {
		t.Fatalf("пути без библиотеки: %v", err)
	}

	e := newFontEnv(t, nil)
	dir := filepath.Join(e.a.cfgDir, decor.RootName, fontsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fonts.json"), []byte(`{"version":99,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e.a.openFonts()
	res, err := e.a.ImportFonts()
	if !errors.Is(err, fontlib.ErrReadOnly) || calls != 0 || res.Faces == nil || res.Skipped == nil {
		t.Fatalf("только чтение: %v %+v, диалогов %d", err, res, calls)
	}
	if _, err := e.a.ImportFontPaths([]string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil)}); !errors.Is(err, fontlib.ErrReadOnly) {
		t.Fatalf("пути, только чтение: %v", err)
	}

	if err := os.Remove(filepath.Join(dir, "fonts.json")); err != nil {
		t.Fatal(err)
	}
	e.a.openFonts()
	res, err = e.a.ImportFonts() // закрыли диалог
	if err != nil || calls != 1 || len(res.Added) != 0 || res.Added == nil {
		t.Fatalf("закрытый диалог: %v %+v", err, res)
	}
	if opts.Title != "Свои шрифты" || len(opts.Filters) != 1 || !strings.Contains(opts.Filters[0].Pattern, "*.ttc") ||
		opts.Filters[0].DisplayName != "Шрифты (*.ttf, *.otf)" {
		t.Fatalf("диалог: %+v", opts)
	}
	pick = []string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil)}
	if res, err = e.a.ImportFonts(); err != nil || len(res.Added) != 1 || len(e.fonts) != 1 {
		t.Fatalf("выбор: %v %+v", err, res)
	}
	if got := e.a.dialogDir(dirFont); !strings.EqualFold(filepath.Clean(got), filepath.Clean(e.src)) {
		t.Fatalf("папка диалога: %q", got)
	}
}

// TestDropFontsOnlyFresh — откат окна снимает только начертания этого сеанса: прежнее начертание
// (до перезапуска) и ключи не по форме (путь, верхний регистр, «..») не трогаются; событие — один раз.
func TestDropFontsOnlyFresh(t *testing.T) {
	e := newFontEnv(t, nil)
	if _, err := e.a.DropFonts([]string{"0123456789abcdef.ttf"}); err != nil {
		t.Fatal(err)
	}
	old, err := e.a.ImportFontPaths([]string{e.file(t, "lora.ttf", "Lora-Regular.ttf", nil)})
	if err != nil || len(old.Faces) != 1 {
		t.Fatalf("загрузка: %v %+v", err, old)
	}
	e.a.openFonts() // перезапуск: прежний ключ уже не свежий
	fresh, err := e.a.ImportFontPaths([]string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil), e.file(t, "ob.ttf", "Oswald-Bold.ttf", nil)})
	if err != nil || len(fresh.Faces) != 2 {
		t.Fatalf("свежая: %v %+v", err, fresh)
	}
	osw, _ := familyOf(t, "Oswald-Regular.ttf")
	lora, _ := familyOf(t, "Lora-Regular.ttf")
	lib := filepath.Join(e.a.cfgDir, decor.RootName, fontsDir)
	events := len(e.fonts)

	upper := strings.ToUpper(fresh.Faces[1][:16]) + fresh.Faces[1][16:]
	in := []string{old.Faces[0], fresh.Faces[0], upper, "../" + fresh.Faces[1], filepath.Join(lib, fresh.Faces[1]), ""}
	got, err := e.a.DropFonts(in)
	if err != nil || !slices.Equal(got, []string{fresh.Faces[0]}) {
		t.Fatalf("удалены %v, %v", got, err)
	}
	if len(e.fonts) != events+1 {
		t.Fatalf("событий fonts: %d", len(e.fonts)-events)
	}
	if _, err := os.Stat(filepath.Join(lib, old.Faces[0])); err != nil {
		t.Fatalf("прежнее начертание удалено: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lib, fresh.Faces[0])); !os.IsNotExist(err) {
		t.Fatalf("свежее не удалено: %v", err)
	}
	if f := myFamily(e.fonts[len(e.fonts)-1], osw); f == nil || len(f.Faces) != 1 || myFamily(e.fonts[len(e.fonts)-1], lora) == nil {
		t.Fatalf("список после отката: %+v", f)
	}
	// Нечего удалять (уже удалено, чужие строки) — пустой срез, не nil, без события.
	got, err = e.a.DropFonts([]string{fresh.Faces[0], "x"})
	if err != nil || got == nil || len(got) != 0 || len(e.fonts) != events+1 {
		t.Fatalf("повтор: %v %v, событий %d", got, err, len(e.fonts)-events)
	}
	if got, _ := e.a.DropFonts(nil); got == nil {
		t.Fatal("nil вместо пустого среза")
	}
}

// TestHideFontEvent — ×: событие со скрытой гарнитурой, файл и регистрация остаются (тексты
// рисуются ею же), правки альбома нет; чужой id — ошибка без события.
func TestHideFontEvent(t *testing.T) {
	osw, _ := familyOf(t, "Oswald-Regular.ttf")
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Texts: []*model.TextBlock{fontBlock(osw, "0a0b0c0d0e01", 100)}}}
	e := newFontEnv(t, p)
	res, err := e.a.ImportFontPaths([]string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil)})
	if err != nil || len(res.Added) != 1 {
		t.Fatal(err, res)
	}
	v := e.a.editor().View()
	if err := e.a.HideFont(osw); err != nil {
		t.Fatal(err)
	}
	if len(e.fonts) != 2 {
		t.Fatalf("событий: %d", len(e.fonts))
	}
	if f := myFamily(e.fonts[1], osw); f == nil || !f.Hidden {
		t.Fatalf("скрытая в событии: %+v", f)
	}
	if !text.Available(osw) || e.a.editor().View().Rev != v.Rev {
		t.Fatal("скрытая снята с регистрации или правка альбома")
	}
	if rec := getMedia(e.a, "/media/font/"+res.Faces[0]); rec.Code != http.StatusOK {
		t.Fatalf("скрытая не отдаётся окну: %d", rec.Code)
	}
	for _, bad := range []string{"", "montserrat", "my:0123456789abcdef"} {
		if err := e.a.HideFont(bad); err == nil {
			t.Errorf("HideFont(%q) без ошибки", bad)
		}
	}
	if len(e.fonts) != 2 {
		t.Fatalf("ошибка дала событие: %d", len(e.fonts))
	}
}

// TestServeFontCustom — /media/font: своё начертание только по ключу зарегистрированного
// (тип по расширению, immutable); чужой ключ, путь, «..», верхний регистр — 404; встроенное — как было.
func TestServeFontCustom(t *testing.T) {
	e := newFontEnv(t, nil)
	cff, err := os.ReadFile(filepath.Join("..", "text", "testdata", "sa-test-cff.otf"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.a.ImportFontPaths([]string{e.file(t, "o.ttf", "Oswald-Regular.ttf", nil), e.file(t, "c.otf", "", cff)})
	if err != nil || len(res.Faces) != 2 {
		t.Fatalf("загрузка: %v %+v", err, res)
	}
	for _, face := range res.Faces {
		rec := getMedia(e.a, "/media/font/"+face)
		want, ctype := fontBytes(t, "Oswald-Regular.ttf"), "font/ttf"
		if strings.HasSuffix(face, ".otf") {
			want, ctype = cff, "font/otf"
		}
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) || rec.Header().Get("Content-Type") != ctype ||
			rec.Header().Get("Cache-Control") != "max-age=31536000, immutable" || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: %d %s %s", face, rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"))
		}
	}
	lib := filepath.Join(e.a.cfgDir, decor.RootName, fontsDir)
	upper := strings.ToUpper(res.Faces[0][:16]) + res.Faces[0][16:]
	key := strings.TrimSuffix(res.Faces[0], ".ttf")
	for _, bad := range []string{"0123456789abcdef.ttf", key + ".otf", key, upper, "..%2f" + res.Faces[0],
		"../" + res.Faces[0], filepath.ToSlash(filepath.Join(lib, res.Faces[0])), "fonts.json", "%2e%2e%2ffonts.json"} {
		if rec := getMedia(e.a, "/media/font/"+bad); rec.Code != http.StatusNotFound {
			t.Errorf("%q: %d", bad, rec.Code)
		}
	}
	rec := getMedia(e.a, "/media/font/Montserrat-Regular.ttf")
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "max-age=86400" || rec.Header().Get("Content-Type") != "font/ttf" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("встроенный: %d %s", rec.Code, rec.Header().Get("Cache-Control"))
	}
	// Файл подменили на диске — не отдаётся.
	if err := os.WriteFile(filepath.Join(lib, res.Faces[0]), fontBytes(t, "Lora-Regular.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := getMedia(e.a, "/media/font/"+res.Faces[0]); rec.Code != http.StatusNotFound {
		t.Fatalf("подменённый файл: %d", rec.Code)
	}
}

// collectedAlbum — «собранный» альбом в своей папке: тексты гарнитурами ids (ширина 100, как на
// другом компьютере) и папка «Шрифты» с файлами files (имя → встроенное начертание).
func collectedAlbum(t *testing.T, ids []string, files map[string]string) string {
	t.Helper()
	dir := testkit.TempDir(t)
	p := model.New()
	for i, id := range ids {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i)),
			Texts: []*model.TextBlock{fontBlock(id, "0a0b0c0d0e0"+string(rune('1'+i)), 100)}})
	}
	p.SyncFonts()
	path := filepath.Join(dir, "Иванов"+projectExt)
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	fonts := filepath.Join(dir, "Шрифты")
	if err := os.MkdirAll(fonts, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, builtin := range files {
		if err := os.WriteFile(filepath.Join(fonts, name), fontBytes(t, builtin), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TestOpenImportsAlbumFonts — собранный альбом на чистой библиотеке: из его «Шрифтов» добавляются
// только пропавшие гарнитуры (не стоящая в альбоме и уже загруженная — нет), не больше 64 файлов
// папки; ширины сразу нужным шрифтом, без шага отмены и без «изменён», тост — в State.Info, новые файлы — в State.FreshFonts.
// Черновик рядом с такой же папкой ничего не добавляет.
func TestOpenImportsAlbumFonts(t *testing.T) {
	osw, oswName := familyOf(t, "Oswald-Regular.ttf")
	lora, _ := familyOf(t, "Lora-Regular.ttf")
	neucha, _ := familyOf(t, "Neucha-Regular.ttf")
	path := collectedAlbum(t, []string{osw, lora}, map[string]string{"z-Oswald-Regular.ttf": "Oswald-Regular.ttf",
		"z-Oswald-Bold.ttf": "Oswald-Bold.ttf", "z-Lora.ttf": "Lora-Regular.ttf", "z-Neucha.ttf": "Neucha-Regular.ttf"})
	e := newFontEnv(t, nil)
	if _, err := e.a.ImportFontPaths([]string{e.file(t, "l.ttf", "Lora-Regular.ttf", nil)}); err != nil {
		t.Fatal(err)
	}
	events := len(e.fonts)
	lib := filepath.Join(e.a.cfgDir, decor.RootName, fontsDir)
	libFiles := func() int {
		list, _ := os.ReadDir(lib)
		return len(list)
	}
	before := libFiles()

	checkOver64(t, e, path, osw, events, libFiles, before)

	st, err := e.a.openPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Info != "Добавлены шрифты из папки альбома: „"+oswName+"“" || st.Notice != "" {
		t.Fatalf("тост: %q, ошибка %q", st.Info, st.Notice)
	}
	if len(st.FreshFonts) != 2 || !faceFile.MatchString(st.FreshFonts[0]) || !faceFile.MatchString(st.FreshFonts[1]) {
		t.Fatalf("новые начертания: %v", st.FreshFonts)
	}
	if !text.Available(osw) || text.Available(neucha) || len(e.fonts) != events+1 || libFiles() != before+2 {
		t.Fatalf("добавлено: Oswald %v, Neucha %v, событий %d, файлов %d (было %d)", text.Available(osw),
			text.Available(neucha), len(e.fonts)-events, libFiles(), before)
	}
	if f := myFamily(e.fonts[len(e.fonts)-1], osw); f == nil || len(f.Faces) != 2 {
		t.Fatalf("событие: %+v", f)
	}
	v := st.View
	if v.Dirty || v.CanUndo || v.Spreads[0].Texts[0].W == 100 || v.Spreads[1].Texts[0].W == 100 {
		t.Fatalf("вид: изменён %v, отмена %v, ширины %v %v", v.Dirty, v.CanUndo, v.Spreads[0].Texts[0].W, v.Spreads[1].Texts[0].W)
	}
	if again := e.a.GetState(); again.Info != "" || again.FreshFonts != nil || again.Notice != "" {
		t.Fatalf("тост второй раз: %q %v", again.Info, again.FreshFonts)
	}
	// Повторное открытие — пропавших нет: ни события, ни тоста.
	if st, err = e.a.openPath(path); err != nil || st.Info != "" || st.FreshFonts != nil || len(e.fonts) != events+1 {
		t.Fatalf("повтор: %v %q", err, st.Info)
	}

	checkStartupAndDraft(t, path, osw)
}

// checkOver64 — папка из 64 файлов раньше по имени: дальше она не читается, Oswald не добавлен;
// лишние файлы потом убираются.
func checkOver64(t *testing.T, e *fontEnv, path, osw string, events int, libFiles func() int, before int) {
	t.Helper()
	// 64 файла раньше по имени — дальше папка не читается: Oswald не добавлен.
	fonts := filepath.Join(filepath.Dir(path), "Шрифты")
	var junk []string
	for i := range fontlib.MaxAlbumFiles {
		j := filepath.Join(fonts, "a"+string(rune('A'+i/26))+string(rune('a'+i%26))+".ttf")
		if err := os.WriteFile(j, []byte("не шрифт"), 0o644); err != nil {
			t.Fatal(err)
		}
		junk = append(junk, j)
	}
	st, err := e.a.openPath(path)
	if err != nil || st.Info != "" || text.Available(osw) || len(e.fonts) != events || libFiles() != before {
		t.Fatalf("сверх 64: %v %q, Oswald %v, событий %d", err, st.Info, text.Available(osw), len(e.fonts)-events)
	}
	for _, j := range junk {
		if err := os.Remove(j); err != nil {
			t.Fatal(err)
		}
	}
}

// checkStartupAndDraft — запуск с альбомом: тост в Info и начертания; черновик папку рядом не читает.
func checkStartupAndDraft(t *testing.T, path, osw string) {
	t.Helper()
	// При старте тост — в Info (не в ошибку запуска) с новыми начертаниями.
	e2 := newFontEnv(t, nil) // чистая библиотека: и Lora, и два начертания Oswald
	if _, note := loadNote(e2.a, path); note != "" || !strings.HasPrefix(e2.a.startInfo, "Добавлены шрифты из папки альбома") || len(e2.a.startFresh) != 3 {
		t.Fatalf("при запуске: ошибка %q, тост %q, начертаний %d", note, e2.a.startInfo, len(e2.a.startFresh))
	}
	e3 := newFontEnv(t, nil)
	p, _ := salbum.Load(path)
	if err := salbum.Save(p, e3.a.draftPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(e3.a.cfgDir, "Шрифты"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e3.a.cfgDir, "Шрифты", "o.ttf"), fontBytes(t, "Oswald-Regular.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if loaded, note := loadNote(e3.a, ""); loaded == nil || note != "" || e3.a.startInfo != "" || text.Available(osw) || len(e3.fonts) != 0 {
		t.Fatalf("черновик: %q %q, Oswald %v", note, e3.a.startInfo, text.Available(osw))
	}
}

// TestPublishFontsParallel — параллельные загрузки, скрытие и откат: набор, зарегистрированный в
// text, и последнее событие `fonts` совпадают с библиотекой (fontsMu: снимок библиотеки →
// регистрация → событие одним шагом). Гонки — под -race.
func TestPublishFontsParallel(t *testing.T) {
	e := newFontEnv(t, nil)
	var mu sync.Mutex
	var last []text.Family
	e.a.emit = func(name string, data ...any) {
		if name == "fonts" {
			mu.Lock()
			last = data[0].([]text.Family)
			mu.Unlock()
		}
	}
	builtins := []string{"Oswald-Regular.ttf", "Lora-Regular.ttf", "Neucha-Regular.ttf", "Caveat-Regular.ttf",
		"Rubik-Regular.ttf", "Forum-Regular.ttf", "Prata-Regular.ttf", "Kurale-Regular.ttf"}
	paths := make([]string, len(builtins))
	for i, b := range builtins {
		paths[i] = e.file(t, b, b, nil)
	}
	oswID, _ := familyOf(t, "Oswald-Regular.ttf")
	var wg sync.WaitGroup
	for i := range paths {
		wg.Go(func() {
			if _, err := e.a.ImportFontPaths(paths[i : i+1]); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Go(func() { _ = e.a.HideFont(oswID) }) // может прийти раньше загрузки — тогда ErrNotFound
	wg.Go(func() { _, _ = e.a.DropFonts([]string{"0123456789abcdef.ttf"}) })
	wg.Wait()
	s, _ := e.a.fontStore()
	want := len(s.Families())
	custom := func(list []text.Family) int {
		n := 0
		for _, f := range list {
			if f.Group == text.CustomGroup {
				n++
			}
		}
		return n
	}
	mu.Lock()
	defer mu.Unlock()
	if want != len(builtins) || custom(text.Families()) != want || custom(last) != want {
		t.Fatalf("в библиотеке %d, зарегистрировано %d, в последнем событии %d", want, custom(text.Families()), custom(last))
	}
}

// loadNote — loadStartup с последним альбомом last: проект и уведомление запуска.
func loadNote(a *App, last string) (*model.Project, string) {
	p, _, note := a.loadStartup(last)
	return p, note
}
