package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"io"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/namesfile"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestFontRoute(t *testing.T) {
	a := newApp(t) // без кэша миниатюр: шрифтам он не нужен
	h := a.media(http.NotFoundHandler())
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		return w
	}
	for _, f := range text.Families() {
		for _, fc := range f.Faces {
			w := get("/media/font/" + fc.File)
			if w.Code != http.StatusOK || w.Body.Len() < 1000 {
				t.Fatalf("%s: %d, %d байт", fc.File, w.Code, w.Body.Len())
			}
			if ct, cc := w.Header().Get("Content-Type"), w.Header().Get("Cache-Control"); ct != "font/ttf" || cc != "max-age=86400" {
				t.Fatalf("%s: заголовки %q %q", fc.File, ct, cc)
			}
		}
	}
	for _, bad := range []string{
		"/media/font/", "/media/font/x.ttf", "/media/font/../app.go", "/media/font/..%2Fapp.go",
		"/media/font/fonts/Montserrat-Regular.ttf", "/media/font/OFL-montserrat.txt",
		"/media/font/montserrat-regular.ttf", "/media/font/Montserrat-Regular.ttf/x",
	} {
		if w := get(bad); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d, ждали 404", bad, w.Code)
		}
	}
	if fonts := a.GetState().Fonts; len(fonts) != len(text.Families()) || len(fonts[0].Faces) == 0 {
		t.Fatalf("State.Fonts: %+v", fonts)
	}
}

// Паника внутри правки не отдаёт интерфейсу пустой вид: текущий альбом и ошибка.
func TestEditPanicKeepsView(t *testing.T) {
	log.SetOutput(io.Discard) // стек паники в выводе тестов не нужен
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"}}}
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	a.ed.Store(mustEditor(t, p, ""))
	// Проект с NaN (снимок для отмены не собирался — паника в try) NewEditor больше не примет
	// (023 Ш7): паника — из рассылки вида внутри метода правки.
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			panic("сбой рассылки вида")
		}
	}
	v, err := a.AddText("s1", 0, "")
	if !errors.Is(err, errInternal) || len(v.Spreads) != 1 {
		t.Fatalf("ошибка %v, развороты %d", err, len(v.Spreads))
	}
	if v := a.SetBackground("s1", "spread", "#000000"); len(v.Spreads) != 1 {
		t.Fatalf("без ошибки в ответе тоже текущий вид: %d разворотов", len(v.Spreads))
	}
}

// bgApp — App с пустым проектом и одним разворотом s1.
func bgApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}}
	a.ed.Store(mustEditor(t, p, ""))
	return a
}

func writeGIF(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	im := image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White})
	if err := gif.Encode(f, im, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// Своё фото фона: отказы до правки проекта, принятый файл — фон с верным ID и размером.
func TestBackdropFileRejects(t *testing.T) {
	dir := testkit.TempDir(t)
	a := bgApp(t)
	ok := testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "dir.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := map[string]string{
		"UNC":              `\\srv\share\a.jpg`,
		"устройство":       `\\?\C:\a.jpg`,
		"поток NTFS":       ok + ":поток",
		"относительный":    "ok.jpg",
		"..":               filepath.Join(dir, "sub") + string(filepath.Separator) + ".." + string(filepath.Separator) + "ok.jpg",
		"gif":              writeGIF(t, dir, "a.gif"),
		"gif под видом":    writeGIF(t, dir, "b.jpg"),
		"папка":            filepath.Join(dir, "dir.jpg"),
		"нет файла":        filepath.Join(dir, "нет.jpg"),
		"больше MaxPixels": testkit.HugeJPEG(t, dir, "huge.jpg", 20000, 20000),
	}
	for name, path := range bad {
		rev := a.editor().View().Rev
		v, err := a.setBackdropFile("s1", "spread", path)
		if err == nil || v.Rev != rev || len(v.Spreads[0].Backdrops) != 0 {
			t.Errorf("%s: принят (err %v, фонов %d)", name, err, len(v.Spreads[0].Backdrops))
		}
	}
	if v, err := a.setBackdropFile("s1", "top", ok); err == nil || len(v.Spreads[0].Backdrops) != 0 {
		t.Errorf("неверная область принята: %v", err)
	}
	v, err := a.setBackdropFile("s1", "left", ok)
	if err != nil || len(v.Spreads[0].Backdrops) != 1 || v.Spreads[0].Backdrops[0].Photo != library.IDFor(ok) {
		t.Fatalf("нормальный файл: err %v, %+v", err, v.Spreads[0].Backdrops)
	}
	if r := a.editor().Refs()[library.IDFor(ok)]; r.W != 40 || r.H != 30 || r.Size == 0 || r.Path != ok {
		t.Errorf("сведения о фото: %+v", r)
	}
	if len(a.editor().Folders()) != 0 {
		t.Error("папка фона попала в библиотеку")
	}

	// Ссылка: на локальный файл — читается по цели, на папку — отказ. Без прав на ссылки — пропуск.
	link := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(ok, link); err != nil {
		t.Logf("ссылки недоступны: %v", err)
		return
	}
	// Путь приводится через EvalSymlinks (на раннере temp — в 8.3), ждём ID цели, не link.jpg.
	resolved, _ := filepath.EvalSymlinks(ok)
	if v, err := a.setBackdropFile("s1", "right", link); err != nil || v.Spreads[0].Backdrops[1].Photo != library.IDFor(resolved) {
		t.Errorf("ссылка на локальный файл: %v", err)
	}
	dlink := filepath.Join(dir, "dlink.jpg")
	if err := os.Symlink(filepath.Join(dir, "sub"), dlink); err == nil {
		if _, err := a.setBackdropFile("s1", "right", dlink); err == nil {
			t.Error("ссылка на папку принята")
		}
	}
}

// Диалог «Свой файл…»: отказ — вид без изменений; выбор — фон.
func TestPickBackdropFile(t *testing.T) {
	dir := testkit.TempDir(t)
	a := bgApp(t)
	ok := testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	pick := ""
	openFileDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return pick, nil }
	rev := a.editor().View().Rev
	if v, err := a.PickBackdropFile("s1", "spread"); err != nil || v.Rev != rev || len(v.Spreads[0].Backdrops) != 0 {
		t.Fatalf("отказ от диалога: %v %+v", err, v.Spreads[0].Backdrops)
	}
	pick = ok
	v, err := a.PickBackdropFile("s1", "spread")
	if err != nil || len(v.Spreads[0].Backdrops) != 1 {
		t.Fatalf("выбор файла: %v %+v", err, v.Spreads[0].Backdrops)
	}
	if _, err := a.PickBackdropFile("s1", "top"); err == nil {
		t.Fatal("неверная область принята")
	}
}

// /media отдаёт фото фона, которого нет в ленте, по сведениям проекта.
func TestMediaServesBackdrop(t *testing.T) {
	dir := testkit.TempDir(t)
	a := bgApp(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	ok := testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	if _, err := a.setBackdropFile("s1", "spread", ok); err != nil {
		t.Fatal(err)
	}
	id := library.IDFor(ok)
	h := a.media(http.NotFoundHandler())
	for _, kind := range []string{"t", "p"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/media/"+kind+"/"+id, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
			t.Errorf("/media/%s: %d, %q", kind, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/media/t/неизвестный", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("чужой ID: %d", rec.Code)
	}
}

// Методы App ведут правки фона; задание экспорта несёт фон с путём, рамкой и зеркалом.
func TestAppBackdropEditAndExport(t *testing.T) {
	a := gridApp(t)
	if _, err := a.SetBackdrop("s2", "right", "нет"); err == nil {
		t.Fatal("неизвестное фото принято")
	}
	if _, err := a.SetBackdrop("s2", "top", "a"); err == nil {
		t.Fatal("неизвестная область принята")
	}
	v, err := a.SetBackdrop("s2", "right", "a") // фото альбома, чьей папки нет в ленте
	if err != nil || len(v.Spreads[1].Backdrops) != 1 {
		t.Fatalf("SetBackdrop: %v", err)
	}
	a.SetBackdropBox("s2", "right", 320, 10, 200, 150, 30, "")
	a.FlipBackdrop("s2", "right", "h")
	a.SetBackdropFrame("s2", "right", 2, 0.5, 0)
	a.SetBackdropAngle("s2", "right", 4.5, "") // 045: наклон фона доходит до задания
	a.StraightenPhoto("s2", "b", 100, 7.87)    // линия под 4,5° → наклон фото −4,5
	pp, view := a.editor().Snapshot()
	got := export.FromView(pp, view)[1].Backdrops
	if len(got) != 1 {
		t.Fatalf("фонов в задании: %d", len(got))
	}
	b := got[0]
	want := render.Backdrop{Area: view.Spreads[1].Backdrops[0].Area, Rot: 30, FlipH: true, Path: testkit.AbsPath("a.jpg")}
	if b.Path != want.Path || b.Rot != 30 || !b.FlipH || b.Box.X != 320 || b.Box.W != 200 || b.Area != want.Area || b.Img.W <= b.Box.W || b.ImgRot != 4.5 || view.Spreads[1].Frames[0].Tilt != -4.5 {
		t.Fatalf("фон в задании: %+v, наклон фото %v", b, view.Spreads[1].Frames[0].Tilt)
	}
	if v := a.FitBackdrop("s2", "right"); v.Spreads[1].Backdrops[0].Box.Rot != 0 || a.StraightenBackdrop("s2", "right", 100, -7.87).Spreads[1].Backdrops[0].Frame.R != 9 {
		t.Fatal("FitBackdrop не сбросил поворот рамки, сбросил наклон или StraightenBackdrop не выпрямил")
	}
	if v := a.ClearBackdrop("s2", "right"); len(v.Spreads[1].Backdrops) != 0 || len(v.BackdropUsed) != 0 {
		t.Fatalf("ClearBackdrop: %+v", v.Spreads[1].Backdrops)
	}
}

// Прозрачность фона (спека 017): метод App — правка с зажимом, холостая не меняет rev;
// задание экспорта несёт Fade, рендер смешивает белое фото с цветом страницы.
func TestAppBackdropFade(t *testing.T) {
	dir := testkit.TempDir(t)
	a := bgApp(t)
	path := filepath.Join(dir, "white.jpg")
	im := image.NewRGBA(image.Rect(0, 0, 60, 30))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, im, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.setBackdropFile("s1", "spread", path); err != nil {
		t.Fatal(err)
	}
	a.SetBackground("s1", "spread", "#c81e1e")
	v := a.SetBackdropFade("s1", "spread", 0.404, "s")
	if got := v.Spreads[0].Backdrops[0].Fade; got != 0.4 {
		t.Fatalf("прозрачность во View: %v", got)
	}
	rev := v.Rev
	for _, fade := range []float64{0.4, math.NaN()} {
		if v := a.SetBackdropFade("s1", "spread", fade, ""); v.Rev != rev {
			t.Fatalf("холостая правка %v сменила rev", fade)
		}
	}
	if v := a.SetBackdropFade("s1", "top", 0.5, ""); v.Rev != rev {
		t.Fatal("неизвестная область сменила rev")
	}

	p, view := a.editor().Snapshot()
	sp := export.FromView(p, view)[0]
	if len(sp.Backdrops) != 1 || sp.Backdrops[0].Fade != 0.4 {
		t.Fatalf("фон в задании: %+v", sp.Backdrops)
	}
	st := p.Settings
	out, err := render.Render(sp, render.Options{PageW: st.PageW, PageH: st.PageH, DPI: 10})
	if err != nil {
		t.Fatal(err)
	}
	c := out.RGBAAt(out.Bounds().Dx()/2-5, out.Bounds().Dy()/2)
	want := color.RGBA{233, 165, 165, 255}
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	if d(c.R, want.R) > 1 || d(c.G, want.G) > 1 || d(c.B, want.B) > 1 {
		t.Fatalf("пиксель экспорта %v, ждали %v ±1", c, want)
	}
}

// marksApp — App с двумя фото в ленте: у "a" в файле оценка 3 и красная метка, у "b" отметок нет.
func marksApp(t *testing.T) (*App, *int) {
	a := newApp(t)
	views := 0
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			views++
		}
	}
	a.byID["a"] = library.Photo{ID: "a", Path: testkit.AbsPath("lib/a.jpg"), W: 300, H: 200,
		Xmp: &library.Marks{Rating: 3, Label: "red"}}
	a.byID["b"] = library.Photo{ID: "b", Path: testkit.AbsPath("lib/b.jpg"), W: 300, H: 200}
	return a, &views
}

// Путь отметки — из ленты по ID; ID, которых в ленте нет, пропускаются. Шаг — с событием view.
func TestAppSetMarksFromLibrary(t *testing.T) {
	a, views := marksApp(t)
	v, err := a.SetMarks([]string{"b", "ghost"}, model.MarkPatch{Rating: ptr(4)})
	if err != nil {
		t.Fatal(err)
	}
	m := a.Marks()
	if len(m) != 1 || m["b"].Rating == nil || *m["b"].Rating != 4 {
		t.Fatalf("отметки %+v, ждали только b с оценкой 4", m)
	}
	if v.MarksRev == 0 || *views != 1 {
		t.Fatalf("marksRev %d, событий view %d", v.MarksRev, *views)
	}
	file := filepath.Join(testkit.TempDir(t), "copy.salbum")
	if err := a.editor().WriteCopy(file); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Marks []struct {
			Path, Abs string
		} `json:"marks"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Marks) != 1 || f.Marks[0].Path != testkit.AbsPath("lib/b.jpg") && f.Marks[0].Abs != testkit.AbsPath("lib/b.jpg") {
		t.Fatalf("в файле отметки %+v, ждали путь из ленты", f.Marks)
	}
}

// Отметки из файла доходят до Editor: цель, равная им, — холостая; «Toggle» видит метку из файла.
func TestAppSetMarksUsesXmp(t *testing.T) {
	a, _ := marksApp(t)
	rev := a.editor().View().Rev
	v, err := a.SetMarks([]string{"a"}, model.MarkPatch{Rating: ptr(3), Label: ptr("red")})
	if err != nil || v.Rev != rev || len(a.Marks()) != 0 {
		t.Fatalf("ждали холостую: rev %d→%d, отметки %+v, ошибка %v", rev, v.Rev, a.Marks(), err)
	}
	if _, err := a.SetMarks([]string{"a"}, model.MarkPatch{Label: ptr("red"), Toggle: true}); err != nil {
		t.Fatal(err)
	}
	if l := a.Marks()["a"].Label; l == nil || *l != "" {
		t.Fatalf("Toggle по красной из файла должен снять метку, своя: %v", l)
	}
	if _, err := a.SetMarks([]string{"a"}, model.MarkPatch{Step: 1}); err != nil {
		t.Fatal(err)
	}
	if r := a.Marks()["a"].Rating; r == nil || *r != 4 {
		t.Fatalf("шаг +1 от оценки из файла 3, своя: %v", r)
	}
}

// ResetMarks убирает свои отметки, отмена возвращает их; сброс без своих — холостой.
func TestAppResetMarks(t *testing.T) {
	a, _ := marksApp(t)
	if _, err := a.SetMarks([]string{"a", "b"}, model.MarkPatch{Flag: ptr(1)}); err != nil {
		t.Fatal(err)
	}
	v := a.ResetMarks([]string{"a", "b", "ghost"})
	if len(a.Marks()) != 0 {
		t.Fatalf("после сброса отметки %+v", a.Marks())
	}
	if again := a.ResetMarks([]string{"a"}); again.Rev != v.Rev || again.MarksRev != v.MarksRev {
		t.Fatalf("повторный сброс не холостой: rev %d→%d", v.Rev, again.Rev)
	}
	u := a.Undo()
	if len(a.Marks()) != 2 || u.MarksRev == v.MarksRev {
		t.Fatalf("отмена сброса: отметки %+v, marksRev %d", a.Marks(), u.MarksRev)
	}
}

func ptr[T any](v T) *T { return &v }

// «Загрузить таблицу…»: диалог и папка — в App, чтение файла — в namesfile (его тесты). Здесь: xlsx
// и csv дают сведения, папка запоминается; отказ — Cancelled без ошибки; сбой диалога, мёртвый диск,
// пропавший файл — ошибка (файл не запоминается). Проект не меняется.
func TestPickNamesFile(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	dir := testkit.TempDir(t)
	put := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldDlg, oldReach := openFileDialog, reachable
	t.Cleanup(func() { openFileDialog, reachable = oldDlg, oldReach })
	var (
		pick    string
		pickErr error
		opts    wr.OpenDialogOptions
	)
	openFileDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) { opts = o; return pick, pickErr }
	snap := func() []byte { p, _ := a.editor().Snapshot(); return model.Marshal(p) }
	before := snap()

	pick = put("Класс.csv", []byte("Фамилия;Имя\r\nИванов;Пётр\r\n"))
	f, err := a.PickNamesFile()
	if want := (NamesFile{Text: "Фамилия\tИмя\nИванов\tПётр", File: "Класс.csv", Rows: 2}); err != nil || f != want {
		t.Fatalf("csv: %+v, %v", f, err)
	}
	if opts.Title != "Таблица с именами" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.xlsx;*.csv;*.txt" ||
		a.dialogDir(dirNames) != dir {
		t.Fatalf("диалог %+v, папка %q", opts, a.dialogDir(dirNames))
	}
	// Отказ — без ошибки; сбой диалога — ошибка, а не отказ.
	pick = ""
	if f, err = a.PickNamesFile(); err != nil || f != (NamesFile{Cancelled: true}) {
		t.Fatalf("отказ: %+v, %v", f, err)
	}
	pickErr = errors.New("диалог не открылся")
	if f, err = a.PickNamesFile(); err == nil || f.Cancelled {
		t.Fatalf("сбой диалога: %+v, %v", f, err)
	}
	pickErr = nil
	// Пропавший файл: слова для пользователя без пути (остальные ошибки — в namesfile).
	other := testkit.TempDir(t)
	pick = filepath.Join(other, "Нет.xlsx")
	if f, err = a.PickNamesFile(); !errors.Is(err, namesfile.ErrGone) || f != (NamesFile{}) || strings.Contains(err.Error(), other) {
		t.Fatalf("нет файла: %+v, %v", f, err)
	}
	reachable = func(string) bool { return false }
	pick = filepath.Join(dir, "Класс.csv")
	if _, err = a.PickNamesFile(); err == nil || !strings.Contains(err.Error(), "недоступен") {
		t.Fatalf("мёртвый диск: %v", err)
	}
	// Отказ и ошибки ничего не запомнили: папка диалога — от последнего принятого файла.
	if a.dialogDir(dirNames) != dir {
		t.Fatalf("папка диалога %q", a.dialogDir(dirNames))
	}
	if !bytes.Equal(snap(), before) {
		t.Fatal("загрузка таблицы изменила проект")
	}
}

// rowsBook — книга .xlsx из одного листа: rows как есть, пустая строка — пустая ячейка (inlineStr).
func rowsBook(t *testing.T, rows [][]string) []byte {
	t.Helper()
	const main = `xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"`
	const rel = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	var sheet strings.Builder
	for i, row := range rows {
		sheet.WriteString(`<row>`)
		for j, s := range row {
			fmt.Fprintf(&sheet, `<c r="%c%d" t="inlineStr"><is><t>%s</t></is></c>`, 'A'+j, i+1, s)
		}
		sheet.WriteString(`</row>`)
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, p := range [][2]string{
		{"xl/workbook.xml", `<workbook ` + main + ` xmlns:r="` + rel + `"><sheets><sheet name="Класс" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + rel + `/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", `<worksheet ` + main + `><sheetData>` + sheet.String() + `</sheetData></worksheet>`},
	} {
		f, err := w.Create(p[0])
		if err == nil {
			_, err = f.Write([]byte(p[1]))
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// Сквозной путь: таблица (CSV «;» и xlsx) → PickNamesFile (table.Read, Sheet.Text) → PreviewNames
// (names.Read). Sheet.Text снимает пустые хвостовые ячейки, и ученик без предмета не должен
// превратиться в учителя или остаться без имени: роли и имена те же из обоих форматов.
func TestPickNamesFileToPreview(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	dir := testkit.TempDir(t)
	oldDlg := openFileDialog
	t.Cleanup(func() { openFileDialog = oldDlg })
	var pick string
	openFileDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return pick, nil }
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a", "b", "c"}}}
	for _, id := range []string{"a", "b", "c"} {
		p.Photos[id] = model.PhotoRef{Path: filepath.Join(dir, "DSC_"+id+".jpg"), W: 300, H: 200}
	}
	a.ed.Store(mustEditor(t, p, ""))
	rows := [][]string{{"ФИО", "Предмет"}, {"Сидорова Ольга Петровна", "Математика"}, {"Иванов Пётр Сергеевич", ""}, {"Козлова Мария", ""}}
	csv := "ФИО;Предмет\r\nСидорова Ольга Петровна;Математика\r\nИванов Пётр Сергеевич;\r\nКозлова Мария;\r\n"
	for name, data := range map[string][]byte{"Класс.csv": []byte(csv), "Класс.xlsx": rowsBook(t, rows)} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		pick = path
		f, err := a.PickNamesFile()
		if err != nil || f.Rows != 4 {
			t.Fatalf("%s: %+v, %v", name, f, err)
		}
		plan, err := a.PreviewNames("s1", model.NamesOpts{Text: f.Text})
		if err != nil || plan.Teachers != 1 || plan.Pupils != 2 || plan.Seats != 3 || plan.Named != 3 || plan.Missing != 0 || plan.Extra != 0 ||
			!slices.Equal(plan.Cols, []string{"ФИО", "Предмет"}) || len(plan.Rows) != 3 {
			t.Fatalf("%s: предпросмотр %+v, %v", name, plan, err)
		}
		for i, want := range []model.Person{{Last: "Сидорова", First: "Ольга", Middle: "Петровна", Subject: "Математика"}, // 049
			{Last: "Иванов", First: "Пётр", Middle: "Сергеевич"}, {Last: "Козлова", First: "Мария"}} {
			if who := plan.Rows[i].Who; who == nil || *who != want {
				t.Errorf("%s: место %d — %+v, ждали %+v", name, i+1, who, want)
			}
		}
	}
}

// PreviewNames и PasteNames через App: предпросмотр без события и rev, вставка — с итогом
// «подписано N из M»; ошибка разбора — без правки.
func TestAppPreviewNames(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a", "b"}}}
	p.Photos["a"] = model.PhotoRef{Path: filepath.Join(testkit.TempDir(t), "01_иванов_петя.jpg"), W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: filepath.Join(testkit.TempDir(t), "DSC_0012.jpg"), W: 300, H: 200}
	a.ed.Store(mustEditor(t, p, ""))
	events := 0
	a.emit = func(string, ...any) { events++ }
	rev := a.editor().View().Rev
	plan, err := a.PreviewNames("s1", model.NamesOpts{From: "files"})
	if err != nil || len(plan.Rows) != 2 || plan.Rows[0].Who == nil || plan.Rows[0].Who.Last != "Иванов" || plan.Rows[1].Who != nil ||
		a.editor().View().Rev != rev || events != 0 {
		t.Fatalf("предпросмотр: %+v, %v, событий %d", plan, err, events)
	}
	r, err := a.PasteNames("s1", model.NamesOpts{From: "files", Caption: true})
	if err != nil || r.Named != 1 || r.Seats != 2 || r.Missing != 1 || r.Extra != 0 || r.View.Rev == rev ||
		r.View.Spreads[0].Caps == nil || r.View.Spreads[0].Caps.Text != "{Фамилия} {Имя}" {
		t.Fatalf("вставка: %+v, %v", r, err)
	}
	if _, err := a.PreviewNames("нет", model.NamesOpts{Text: "А"}); err == nil {
		t.Fatal("чужой разворот принят")
	}
}
