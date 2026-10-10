package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// exportEnv — альбом из n разворотов с одним фото на каждом, подмена диалогов и сбор ответов.
type exportEnv struct {
	a      *App
	dir    string // папка экспорта
	asked  []string
	answer bool
	done   chan string // msg из export:done
	photos []string
}

func newExportEnv(t *testing.T, n int) *exportEnv {
	t.Helper()
	e := &exportEnv{dir: filepath.Join(testkit.TempDir(t), "out"), answer: true, done: make(chan string, 4)}
	src := testkit.TempDir(t)
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 60, 40
	for i := range n {
		id := fmt.Sprintf("p%d", i)
		path := testkit.JPEG(t, src, id+".jpg", 60, 40)
		e.photos = append(e.photos, path)
		p.Photos[id] = model.PhotoRef{Path: path, W: 60, H: 40}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{id}})
	}
	e.a = newApp(t)
	e.a.cfgDir = testkit.TempDir(t)
	e.a.emit = func(name string, data ...any) {
		if name == "export:done" {
			e.done <- data[1].(string)
		}
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	oldAsk, oldPick := askYes, pickExportDir
	askYes = func(_ *App, msg string) (bool, error) { e.asked = append(e.asked, msg); return e.answer, nil }
	pickExportDir = func(*App) (string, error) { return e.dir, os.MkdirAll(e.dir, 0o755) }
	t.Cleanup(func() { askYes, pickExportDir = oldAsk, oldPick })
	return e
}

// export запускает Export и ждёт конца записи; возвращает сообщение об ошибке из export:done.
func (e *exportEnv) export(t *testing.T, o ExportOptions) string {
	t.Helper()
	o.DPI = 72
	ok, err := e.a.Export(o)
	if err != nil || !ok {
		t.Fatalf("Export(%+v) = %v, %v", o, ok, err)
	}
	select {
	case msg := <-e.done:
		return msg
	case <-time.After(30 * time.Second):
		t.Fatal("экспорт не закончился")
	}
	return ""
}

// names — файлы папки экспорта, как их видит пользователь. «Призраков» не считаем: на Windows
// антивирус после успешного удаления 02.jpg переименовывает его в 02.JPG.tmp (8.3-имя + .tmp)
// и держит открытым — запись видна в листинге ещё миллисекунды, иногда появляясь уже после
// возврата DeleteFile; файл в ожидании удаления (Lstat его не находит) — туда же. Свои .tmp
// WriteFileAtomic не оставляет (проверяет её тест), а устаревший 02.jpg Lstat находит.
func (e *exportEnv) names(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(e.dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range ents {
		if strings.HasSuffix(strings.ToLower(f.Name()), ".tmp") {
			continue
		}
		if _, err := os.Lstat(filepath.Join(e.dir, f.Name())); err != nil {
			continue
		}
		out = append(out, f.Name())
	}
	slices.Sort(out)
	return out
}

// oldify отодвигает время изменения файлов в прошлое: потом видно, какие из них переписаны.
func (e *exportEnv) oldify(t *testing.T) {
	t.Helper()
	past := time.Now().Add(-time.Hour)
	for _, n := range e.names(t) {
		if err := os.Chtimes(filepath.Join(e.dir, n), past, past); err != nil {
			t.Fatal(err)
		}
	}
}

func (e *exportEnv) rewritten(t *testing.T, name string) bool {
	t.Helper()
	st, err := os.Stat(filepath.Join(e.dir, name))
	if err != nil {
		t.Fatalf("нет %s: %v", name, err)
	}
	return time.Since(st.ModTime()) < 30*time.Minute
}

func TestExportPartialKeepsOthers(t *testing.T) {
	e := newExportEnv(t, 4)
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	e.oldify(t)
	e.asked = nil
	if msg := e.export(t, ExportOptions{Range: "2-3"}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02.jpg", "03.jpg", "04.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	if e.rewritten(t, "01.jpg") || e.rewritten(t, "04.jpg") || !e.rewritten(t, "02.jpg") || !e.rewritten(t, "03.jpg") {
		t.Fatal("частичный экспорт тронул чужие развороты или пропустил свои")
	}
	if len(e.asked) != 1 || !strings.Contains(e.asked[0], "Заменить файлы разворотов 2, 3 (2 шт.)?") {
		t.Fatalf("вопрос: %q", e.asked)
	}
}

func TestExportPartialReplacesOtherMode(t *testing.T) {
	e := newExportEnv(t, 3)
	e.export(t, ExportOptions{})
	e.oldify(t)
	if msg := e.export(t, ExportOptions{Range: "2", Pages: true}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02-1.jpg", "02-2.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	// И обратно: разворот целиком вместо двух страниц.
	if msg := e.export(t, ExportOptions{Range: "2"}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
}

func TestExportPagePartial(t *testing.T) {
	e := newExportEnv(t, 3)
	e.export(t, ExportOptions{Pages: true})
	if want := []string{"01-1.jpg", "01-2.jpg", "02-1.jpg", "02-2.jpg", "03-1.jpg", "03-2.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	e.oldify(t)
	e.asked = nil
	if msg := e.export(t, ExportOptions{Range: "2п", Pages: true}); msg != "" {
		t.Fatal(msg)
	}
	for _, n := range e.names(t) {
		if got := e.rewritten(t, n); got != (n == "02-2.jpg") {
			t.Errorf("%s переписан = %v", n, got)
		}
	}
	// Страница при виде «Разворотами» — тот же файл NN-2.jpg, 02.jpg не появляется.
	e.oldify(t)
	e.export(t, ExportOptions{Range: "2л"})
	if len(e.names(t)) != 6 || !e.rewritten(t, "02-1.jpg") || e.rewritten(t, "02-2.jpg") {
		t.Errorf("2л: файлы %v", e.names(t))
	}
	if len(e.asked) != 2 || !strings.Contains(e.asked[0], "(1 шт.)") {
		t.Errorf("вопросы %q", e.asked)
	}
}

func TestExportFailKeepsOld(t *testing.T) {
	e := newExportEnv(t, 3)
	e.export(t, ExportOptions{})
	e.oldify(t)
	// Второе фото портим уже после проверки наличия: файл есть, но не картинка.
	if err := os.WriteFile(e.photos[1], []byte("не jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if msg := e.export(t, ExportOptions{Pages: true}); !strings.Contains(msg, "разворот 2") {
		t.Fatalf("ждали ошибку разворота 2, получили %q", msg)
	}
	// Хвост нового вида: «01.jpg…03.jpg» — прежние, их удалили бы только после успеха.
	for _, n := range []string{"01.jpg", "02.jpg", "03.jpg"} {
		if _, err := os.Stat(filepath.Join(e.dir, n)); err != nil {
			t.Errorf("прежний файл %s пропал при ошибке: %v", n, err)
		}
	}
}

func TestExportMissingPhotoRefuses(t *testing.T) {
	e := newExportEnv(t, 3)
	if err := os.Remove(e.photos[2]); err != nil {
		t.Fatal(err)
	}
	pickExportDir = func(*App) (string, error) {
		t.Fatal("диалог папки до проверки пропавших фото")
		return "", nil
	}
	_, err := e.a.Export(ExportOptions{DPI: 72})
	if err == nil || !strings.Contains(err.Error(), "p2.jpg") {
		t.Fatalf("ждали отказ со списком, получили %v", err)
	}
	// Пропавшее фото вне выбранного не мешает.
	pickExportDir = func(*App) (string, error) { return e.dir, os.MkdirAll(e.dir, 0o755) }
	if msg := e.export(t, ExportOptions{Range: "1-2"}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	// Флаг «идёт экспорт» сброшен отказом.
	e.a.mu.Lock()
	busy := e.a.exporting
	e.a.mu.Unlock()
	if busy {
		t.Fatal("флаг exporting остался после отказа")
	}
}

func TestExportFullRemovesTails(t *testing.T) {
	e := newExportEnv(t, 3)
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, export.Marker...)
	for name, data := range map[string][]byte{
		"04.jpg": ourData, "05-1.jpg": ourData, "003.jpg": ourData, "00.jpg": []byte("обложка"), "keep.txt": []byte("x"),
	} {
		if err := os.WriteFile(filepath.Join(e.dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg", "keep.txt"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	if len(e.asked) != 1 || !strings.Contains(e.asked[0], "прежнего экспорта: 3") {
		t.Fatalf("вопрос %q", e.asked)
	}
	// Ответ «нет» ничего не удаляет и не пишет.
	e2 := newExportEnv(t, 2)
	if err := os.MkdirAll(e2.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e2.dir, "05.jpg"), ourData, 0o644); err != nil {
		t.Fatal(err)
	}
	e2.answer = false
	if ok, err := e2.a.Export(ExportOptions{DPI: 72}); ok || err != nil {
		t.Fatalf("отказ пользователя: %v, %v", ok, err)
	}
	if want := []string{"05.jpg"}; !slices.Equal(e2.names(t), want) {
		t.Fatalf("файлы %v", e2.names(t))
	}
}

// Вопрос о пустых разворотах — до выбора папки: «нет» не стоит выбора папки и ничего не пишет.
func TestExportAsksBlank(t *testing.T) {
	e := newExportEnv(t, 2)
	if _, err := e.a.editor().NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App) (string, error) { picked++; return pick(a) }

	e.answer = false
	ok, err := e.a.Export(ExportOptions{DPI: 72})
	if ok || err != nil || picked != 0 {
		t.Fatalf("«нет»: Export = %v, %v; выбор папки %d раз", ok, err, picked)
	}
	if len(e.asked) != 1 || e.asked[0] != "Разворот 3 пустой — выгрузить белым?" {
		t.Fatalf("вопросы: %q", e.asked)
	}

	e.answer, e.asked = true, nil
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	if len(e.asked) == 0 || !strings.Contains(e.asked[0], "Разворот 3 пустой") || picked != 1 {
		t.Fatalf("«да»: вопросы %q, выбор папки %d раз", e.asked, picked)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "03.jpg")); err != nil {
		t.Fatalf("пустой разворот не выгружен: %v", err)
	}
}

// Фото из реестра берут цвет оттуда, остальные — из заголовка файла; нечитаемые пропускаются.
func TestColorReport(t *testing.T) {
	dir := testkit.TempDir(t)
	adobe := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)")))
	p3 := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZP3, 2.2, testkit.DescV2("Display P3")))
	fAdobe := testkit.JPEGSegments(t, dir, "adobe.jpg", 20, 10, adobe)
	fP3 := testkit.JPEGSegments(t, dir, "p3.jpg", 20, 10, p3)
	fPlain := testkit.JPEG(t, dir, "plain.jpg", 20, 10)

	a := newApp(t)
	a.byID["reg"] = library.Photo{ID: "reg", Color: imgfile.ColorOther, Profile: "ProPhoto RGB"}
	pr := model.New()
	pr.Photos = map[string]model.PhotoRef{
		"reg":  {Path: filepath.Join(dir, "нет-файла.jpg")}, // цвет берётся из реестра, файл не читается
		"a":    {Path: fAdobe},
		"p3":   {Path: fP3},
		"pl":   {Path: fPlain},
		"gone": {Path: filepath.Join(dir, "gone.jpg")},
	}
	ad, other := a.colorReport(pr)
	if ad != 1 || len(other) != 2 || other["Display P3"] != 1 || other["ProPhoto RGB"] != 1 {
		t.Errorf("adobe=%d other=%v", ad, other)
	}
}
