package app

import (
	"errors"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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
	// onStep — вызов на каждом export:progress (номер готового); last — данные export:done
	// и флаг экспорта на момент события (читать после приёма из done).
	onStep func(done int)
	last   []any
	steps  atomic.Int32 // последний готовый номер из export:progress — для отчёта о зависании
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
		switch name {
		case "export:progress":
			e.steps.Store(int32(data[0].(int)))
			if e.onStep != nil {
				e.onStep(data[0].(int))
			}
		case "export:done":
			e.a.mu.Lock()
			busy := e.a.exporting
			e.a.mu.Unlock()
			e.last = append(data, busy)
			e.done <- data[1].(string)
		}
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	oldAsk, oldPick := askYes, pickExportDir
	askYes = func(_ *App, msg string) (bool, error) { e.asked = append(e.asked, msg); return e.answer, nil }
	pickExportDir = func(*App, string) (string, error) { return e.dir, os.MkdirAll(e.dir, 0o755) }
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
	msg, ok := testkit.WaitCh(t, e.done, testkit.Limit(30*time.Second), "экспорт не закончился", e.stuckState)
	if !ok {
		t.Fatal("экспорт не закончился (стеки горутин — в логе выше)")
	}
	return msg
}

// stuckState — что известно об экспорте к таймауту: прогресс, флаги, отмена и бюджет декодов
// (занятый чужой тестовой утечкой бюджет держал бы экспорт вечно).
func (e *exportEnv) stuckState() string {
	e.a.mu.Lock()
	busy, hasCancel := e.a.exporting, e.a.exportCancel != nil
	e.a.mu.Unlock()
	return fmt.Sprintf("готово %d; exporting=%v, есть отмена=%v; stopped=%v, stopCtx.Err=%v; DecodeBudget: занято %d пикс., ждут %d",
		e.steps.Load(), busy, hasCancel, e.a.stopped.Load(), e.a.stopCtx.Err(), imgfile.DecodeBudget.Used(), imgfile.DecodeBudget.Waiting())
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
		if strings.HasSuffix(strings.ToLower(f.Name()), ".tmp") && !testkit.OwnTmp(f.Name()) {
			t.Logf("чужой временный файл (антивирус) %s", f.Name())
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
	pickExportDir = func(*App, string) (string, error) {
		t.Fatal("диалог папки до проверки пропавших фото")
		return "", nil
	}
	_, err := e.a.Export(ExportOptions{DPI: 72})
	if err == nil || !strings.Contains(err.Error(), "p2.jpg") {
		t.Fatalf("ждали отказ со списком, получили %v", err)
	}
	// Пропавшее фото вне выбранного не мешает.
	pickExportDir = func(*App, string) (string, error) { return e.dir, os.MkdirAll(e.dir, 0o755) }
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
	pickExportDir = func(a *App, title string) (string, error) { picked++; return pick(a, title) }

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

// Пустые поля подписей (030): вопрос до выбора папки; «нет» — ничего не записано.
func TestExportFieldsWarning(t *testing.T) {
	e := newExportEnv(t, 2)
	e.a.SetCaption("p0", "{Школа}", "")
	e.a.SetSheetCaps("s1", true, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: "{Фамилия}"})
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App, title string) (string, error) { picked++; return pick(a, title) }

	e.answer = false
	ok, err := e.a.Export(ExportOptions{DPI: 72})
	want := "Не заполнены поля: Заголовок; фото без имени: 1. В выгрузке они будут пустыми. Продолжить?"
	if ok || err != nil || picked != 0 || len(e.asked) != 1 || e.asked[0] != want {
		t.Fatalf("«нет»: Export = %v, %v; выбор папки %d раз; вопросы %q", ok, err, picked, e.asked)
	}
	if ents, _ := os.ReadDir(e.dir); len(ents) != 0 {
		t.Fatalf("после «нет» записано %d файлов", len(ents))
	}
	// Заполнили — вопроса нет. Поля шапки листа правит окно виньетки (031 Т2): до него —
	// проект с полями у листа фото p0.
	p, _ := e.a.editor().Snapshot()
	p.Spreads[0].Vars = &model.Vars{Title: "Школа №5"}
	e.a.ed.Store(mustEditor(t, p, ""))
	e.a.SetPerson("p1", model.Person{Last: "Иванов"}, "")
	e.answer, e.asked = true, nil
	if msg := e.export(t, ExportOptions{}); msg != "" || len(e.asked) != 0 || picked != 1 {
		t.Fatalf("заполнено: %q, вопросы %q", msg, e.asked)
	}
}

// Вопрос о пустых полях — только по листам плана: диапазон и «Только обложка» не спрашивают
// о листах, которые не выгружаются (итоговое ревью 0.10).
func TestExportFieldsWarningPlan(t *testing.T) {
	e := coverEnv(t, 3, "p0")
	e.a.SetCaption("p1", "{Школа}", "") // лист 2: пустой «Заголовок»
	e.a.SetSheetCaps("s2", true, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: "{Фамилия}"})
	v, err := e.a.AddText(model.CoverID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	e.a.SetText(v.Cover.Sheet.Texts[0].ID, "{Год}", "") // обложка: пустой «Год»
	pick := pickExportDir
	t.Cleanup(func() { pickExportDir = pick })
	pickExportDir = func(*App, string) (string, error) { return "", nil } // отмена выбора папки: ничего не пишется
	for _, c := range []struct {
		o        ExportOptions
		unfilled []string
		unnamed  int
	}{
		{ExportOptions{Range: "1"}, nil, 0},
		{ExportOptions{Range: "2"}, []string{"Заголовок"}, 0},
		{ExportOptions{Range: "3"}, nil, 1},
		{ExportOptions{Range: "2-3"}, []string{"Заголовок"}, 1},
		{ExportOptions{CoverOnly: true}, []string{"Год"}, 0},
		{ExportOptions{CoverOnly: true, Range: "2"}, []string{"Год"}, 0},
		{ExportOptions{Cover: true, Range: "1"}, []string{"Год"}, 0},
		{ExportOptions{Cover: true}, []string{"Год", "Заголовок"}, 1},
		{ExportOptions{}, []string{"Заголовок"}, 1},
	} {
		e.asked = nil
		c.o.DPI = 72
		if ok, err := e.a.Export(c.o); ok || err != nil {
			t.Fatalf("%+v: Export = %v, %v", c.o, ok, err)
		}
		var want []string
		if w := export.FieldsWarning(c.unfilled, c.unnamed, 0); w != "" {
			want = []string{w}
		}
		if !slices.Equal(e.asked, want) {
			t.Errorf("%+v: вопросы %q, ждали %q", c.o, e.asked, want)
		}
	}
}

// Имена без фото (071 §7а): табличка пустого места — в вопросе до выбора папки, только по
// листам плана; «нет» — ничего не записано; место заполнили — вопроса нет.
func TestExportWaitingWarning(t *testing.T) {
	e := newExportEnv(t, 2)
	p, _ := e.a.editor().Snapshot()
	p.Spreads[0].Grid = []*model.Place{{ID: "0000000000a1", X: 5, Y: 5, W: 20, H: 20, Photo: "p0"},
		{ID: "0000000000a2", X: 65, Y: 5, W: 20, H: 20}, {ID: "0000000000a3", X: 90, Y: 5, W: 20, H: 20}}
	e.a.ed.Store(mustEditor(t, p, ""))
	e.a.SetPlacePerson("s0", "0000000000a2", model.Person{Last: "Петров", First: "Ваня"}, "")
	e.a.SetPlacePerson("s0", "0000000000a3", model.Person{Subject: "Химия"}, "") // предмет — не имя
	if pv := e.a.PlacePerson("s0", "0000000000a2"); pv.Last != "Петров" || pv.Place != "0000000000a2" {
		t.Fatalf("PlacePerson: %+v", pv)
	}
	if pv := e.a.PlacePerson("s0", "нет"); pv != (model.PersonView{}) {
		t.Fatalf("нет места: %+v", pv)
	}
	pick := pickExportDir
	t.Cleanup(func() { pickExportDir = pick })
	for _, c := range []struct {
		o    ExportOptions
		want []string
	}{
		{ExportOptions{Range: "2"}, nil},
		{ExportOptions{Range: "1"}, []string{"Имён без фото: 1. В выгрузке они будут пустыми. Продолжить?"}},
		{ExportOptions{}, []string{"Имён без фото: 1. В выгрузке они будут пустыми. Продолжить?"}},
	} {
		e.asked, e.answer = nil, false
		c.o.DPI = 72
		pickExportDir = func(*App, string) (string, error) { return "", nil } // отмена выбора папки
		if ok, err := e.a.Export(c.o); ok || err != nil || !slices.Equal(e.asked, c.want) {
			t.Errorf("%+v: Export = %v, %v; вопросы %q", c.o, ok, err, e.asked)
		}
	}
	if ents, _ := os.ReadDir(e.dir); len(ents) != 0 {
		t.Fatalf("после «нет» записано %d файлов", len(ents))
	}
	// Фото встало на место с табличкой — имени без фото больше нет.
	ph := model.PhotoInfo{ID: "p1", Path: e.photos[1], W: 60, H: 40}
	v, _, err := e.a.editor().DropPhotos("s0", "0000000000a2", []model.PhotoInfo{ph}, -1, "")
	if err != nil || v.Spreads[0].Waiting != 0 {
		t.Fatalf("бросок на место: %v, ждут %d", err, v.Spreads[0].Waiting)
	}
	e.asked, e.answer = nil, true
	pickExportDir = pick
	// Осталось пустое место с одним предметом — вопрос только о пустом месте.
	if msg := e.export(t, ExportOptions{}); msg != "" || len(e.asked) != 1 || strings.Contains(e.asked[0], "без фото") {
		t.Fatalf("заполнено: %q, вопросы %q", msg, e.asked)
	}
}

// TestExportSheetOneFile — проект-лист 031: файл на лист размером листа (204×304 мм при
// 300 dpi — 2409×3591 px), «отдельными страницами» лист не делит, «1п» — ошибка до диалога.
func TestExportSheetOneFile(t *testing.T) {
	e := newExportEnv(t, 2)
	p := model.NewSheet(204, 304)
	p.Spreads = nil
	for i, path := range e.photos {
		id := fmt.Sprintf("p%d", i)
		p.Photos[id] = model.PhotoRef{Path: path, W: 60, H: 40}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{id}})
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	if ok, err := e.a.Export(ExportOptions{Range: "7", DPI: 300}); ok || err == nil || !strings.Contains(err.Error(), "листа 7 нет") {
		t.Fatalf("лист вне диапазона: %v %v", ok, err)
	}
	if ok, err := e.a.Export(ExportOptions{Range: "1п", DPI: 300}); ok || !errors.Is(err, export.ErrSheetPages) {
		t.Fatalf("«1п» в листе: %v, %v", ok, err)
	}
	if ok, err := e.a.Export(ExportOptions{Range: "1", Pages: true, DPI: 300, Quality: 80}); !ok || err != nil {
		t.Fatalf("Export = %v, %v", ok, err)
	}
	if msg := <-e.done; msg != "" {
		t.Fatal(msg)
	}
	if got := e.names(t); !slices.Equal(got, []string{"01.jpg"}) {
		t.Fatalf("файлы %v, ждали один 01.jpg", got)
	}
	f, err := os.Open(filepath.Join(e.dir, "01.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := jpeg.DecodeConfig(f)
	f.Close() // открытый файл Windows не даст переписать ниже
	if err != nil || c.Width != 2409 || c.Height != 3591 {
		t.Fatalf("размер %dx%d (%v), ждали 2409x3591", c.Width, c.Height, err)
	}
	// Все листы — файл на каждый.
	if msg := e.export(t, ExportOptions{Pages: true}); msg != "" || !slices.Equal(e.names(t), []string{"01.jpg", "02.jpg"}) {
		t.Fatalf("все листы: %q, файлы %v", msg, e.names(t))
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

// Отказы Export до записи: диапазон за альбомом, мусорный диапазон, отмена выбора папки,
// пустой альбом — ничего не пишут и без вопросов; флаг «идёт экспорт» снимается.
func TestExportRefusals(t *testing.T) {
	e := newExportEnv(t, 2)
	ok, err := e.a.Export(ExportOptions{DPI: 72, Range: "9"})
	if ok || err == nil {
		t.Fatalf("диапазон за альбомом: %v, %v", ok, err)
	}
	if ok, err = e.a.Export(ExportOptions{DPI: 72, Range: "x"}); ok || err == nil {
		t.Fatalf("мусорный диапазон: %v, %v", ok, err)
	}
	pickExportDir = func(*App, string) (string, error) { return "", nil }
	if ok, err = e.a.Export(ExportOptions{DPI: 72}); ok || err != nil {
		t.Fatalf("отмена выбора папки: %v, %v", ok, err)
	}
	empty := newExportEnv(t, 0)
	if ok, err = empty.a.Export(ExportOptions{DPI: 72}); ok || err == nil {
		t.Fatalf("пустой альбом: %v, %v", ok, err)
	}
	if len(e.asked) != 0 {
		t.Fatalf("вопросы без причины: %q", e.asked)
	}
}

// Чужие файлы с номерами разворотов в папке экспорта — отказ без перезаписи и без вопроса.
func TestExportForeignFilesRefuse(t *testing.T) {
	e := newExportEnv(t, 1)
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, e.dir, "01.jpg", 10, 10)
	ok, err := e.a.Export(ExportOptions{DPI: 72})
	if ok || err == nil || !strings.Contains(err.Error(), "не из Sosnina Albums") {
		t.Fatalf("Export = %v, %v", ok, err)
	}
	if len(e.asked) != 0 {
		t.Fatalf("вопросы: %q", e.asked)
	}
}

// Фото с чужим профилем: вопрос до выбора папки, «нет» — папка не выбирается.
// Профиль без имени в реестре называется «неизвестный профиль».
func TestExportColorDecline(t *testing.T) {
	e := newExportEnv(t, 1)
	e.a.byID["reg"] = library.Photo{ID: "reg", Color: imgfile.ColorOther}
	if _, other := e.a.colorReport(&model.Project{Photos: map[string]model.PhotoRef{"reg": {Path: "x.jpg"}}}); other["неизвестный профиль"] != 1 {
		t.Fatalf("профиль без имени: %v", other)
	}
	p3 := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZP3, 2.2, testkit.DescV2("Display P3")))
	src := testkit.JPEGSegments(t, testkit.TempDir(t), "p3.jpg", 60, 40, p3)
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.photos[0], data, 0o644); err != nil {
		t.Fatal(err)
	}
	picked := 0
	pickExportDir = func(*App, string) (string, error) { picked++; return e.dir, nil }
	e.answer = false
	ok, err := e.a.Export(ExportOptions{DPI: 72})
	if ok || err != nil || picked != 0 || len(e.asked) != 1 || !strings.Contains(e.asked[0], "Display P3") {
		t.Fatalf("«нет»: %v, %v; выбор папки %d; вопросы %q", ok, err, picked, e.asked)
	}
}

// Кнопки вопроса: на Mac — «Да / Нет» строками, иначе системные "Yes"/"No"; по умолчанию и Esc — «нет».
func TestAskYesButtons(t *testing.T) {
	cases := []struct{ goos, yes, no string }{
		{"darwin", "Да", "Нет"},
		{"windows", "Yes", "No"},
		{"linux", "Yes", "No"},
		{"", "Yes", "No"},
	}
	for _, c := range cases {
		o := askOptions(c.goos, "вопрос?")
		if len(o.Buttons) != 2 || o.Buttons[0] != c.yes || o.Buttons[1] != c.no {
			t.Errorf("%q: кнопки %v, ждали %s/%s", c.goos, o.Buttons, c.yes, c.no)
		}
		if o.DefaultButton != c.no || o.CancelButton != c.no {
			t.Errorf("%q: по умолчанию %q, Esc %q, ждали %q", c.goos, o.DefaultButton, o.CancelButton, c.no)
		}
		if o.Message != "вопрос?" || o.Title == "" {
			t.Errorf("%q: текст/заголовок: %+v", c.goos, o)
		}
	}
}

// DeleteSpreads: одно событие view на весь набор, один шаг отмены.
func TestAppDeleteSpreads(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	views := 0
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			mu.Lock()
			views++
			mu.Unlock()
		}
	}
	ed := a.editor()
	var v model.View
	for range 4 {
		var err error
		if v, err = ed.NewSpread(-1, nil); err != nil {
			t.Fatal(err)
		}
	}
	ids := []string{v.Spreads[0].ID, v.Spreads[2].ID, "нет"}
	v = a.DeleteSpreads(ids)
	if len(v.Spreads) != 2 {
		t.Fatalf("осталось разворотов %d, ждали 2", len(v.Spreads))
	}
	mu.Lock()
	n := views
	mu.Unlock()
	if n != 1 {
		t.Errorf("событий view %d, ждали одно", n)
	}
	if v = a.Undo(); len(v.Spreads) != 4 {
		t.Errorf("отмена вернула %d разворотов, ждали 4", len(v.Spreads))
	}
}

// App.NewSpread: фото пропали из ленты при пересканировании во время переноса — ErrNoPhoto
// (092; до 0.22 была тихая холостая), правки нет, пустой разворот вместо них не создаётся (013).
func TestNewSpreadUnknownIDsError(t *testing.T) {
	a := newApp(t)
	a.emit = func(string, ...any) {}
	p := model.New()
	p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	a.ed.Store(mustEditor(t, p, ""))
	before := a.editor().View()
	v, err := a.NewSpread(-1, []string{"нет-такого"})
	if !errors.Is(err, errNoPhoto) || v.Rev != before.Rev || len(v.Spreads) != 1 {
		t.Fatalf("NewSpread с пропавшими фото: %v, rev %d→%d, разворотов %d", err, before.Rev, v.Rev, len(v.Spreads))
	}
	if got := a.editor().View(); got.Rev != before.Rev || got.Dirty {
		t.Fatalf("проект изменился: rev %d, dirty %v", got.Rev, got.Dirty)
	}
}
