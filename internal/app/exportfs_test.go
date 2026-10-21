package app

import (
	"bytes"
	"errors"
	"image/jpeg"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Выгрузка обложки (032) и для Фотосферы Pro (047) через App; общие помощники экспорта — в export_test.go.

// fsferaEnv — exportEnv на 20×15 Фотосферы (201×152, зона 10) с обложкой preset ("" — без)
// по шаблону (лицевая цветная: вопроса о пустой обложке нет); memo — 4-й аргумент export:done.
func fsferaEnv(t *testing.T, n int, preset string) (*exportEnv, *[]string) {
	t.Helper()
	e := newExportEnv(t, n)
	p, _ := e.a.editor().Snapshot()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = 201, 152, 10
	if preset != "" {
		pr, _ := model.CoverPresetFor(preset, p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, BgR: "#203040"}, Spec: pr.Spec}
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	memo := new([]string)
	e.a.emit = func(name string, data ...any) {
		if name == "export:done" {
			*memo, _ = data[3].([]string)
			e.done <- data[1].(string)
		}
	}
	return e, memo
}

// pxOf — размер JPEG в px.
func pxOf(t *testing.T, path string) [2]int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return [2]int{c.Width, c.Height}
}

// TestExportFsfera — режим Фотосферы: диапазон, страницы, dpi и «только обложка» окна
// игнорируются; в выбранной папке — папка книги «Альбом» с 00.jpg, 01.jpg, 02.jpg в px PSD;
// export:done несёт памятку; повтор — вопрос о замене, без вопроса о посторонних; выбрана сама
// папка книги — без вложенной; посторонний файл — вопрос, «нет» ничего не пишет.
func TestExportFsfera(t *testing.T) {
	e, memo := fsferaEnv(t, 2, "fsfera-thin")
	o := ExportOptions{Print: "fsfera", Range: "1", Pages: true, CoverOnly: true, Quality: 80}
	if msg := e.export(t, o); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка: %q, вопросы %q", msg, e.asked)
	}
	book := filepath.Join(e.dir, "Альбом")
	if pxOf(t, filepath.Join(book, "00.jpg")) != [2]int{5409, 2315} || pxOf(t, filepath.Join(book, "01.jpg")) != [2]int{4748, 1795} {
		t.Fatalf("px: обложка %v, разворот %v", pxOf(t, filepath.Join(book, "00.jpg")), pxOf(t, filepath.Join(book, "01.jpg")))
	}
	e.dir = book
	if got := e.names(t); !slices.Equal(got, []string{"00.jpg", "01.jpg", "02.jpg"}) {
		t.Fatalf("файлы книги %v", got)
	}
	if want := []string{"Фотокнига 20×15, обложка «Фото»", "Страницы: Тонкие", "Разворотов: 2 (корешок 6 мм)", "Папка: Альбом — 00.jpg и 01…02.jpg"}; !slices.Equal(*memo, want) {
		t.Fatalf("памятка %q", *memo)
	}
	if e.a.exportDir != book {
		t.Fatalf("«Открыть папку» — %q", e.a.exportDir)
	}
	// Повтор в ту же папку (выбрана родительская): только вопрос о замене своих файлов.
	e.oldify(t)
	e.dir = filepath.Dir(book)
	if msg := e.export(t, o); msg != "" || len(e.asked) != 1 || !strings.Contains(e.asked[0], "Заменить") {
		t.Fatalf("повтор: %q, вопросы %q", msg, e.asked)
	}
	// Выбрана сама папка книги: пишем в неё, вложенной «Альбом/Альбом» нет.
	e.dir, e.asked = book, nil
	if msg := e.export(t, o); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(filepath.Join(book, "Альбом")); err == nil {
		t.Fatal("вложенная папка книги")
	}
	// Посторонний файл: вопрос; «нет» — ничего не переписано.
	if err := os.WriteFile(filepath.Join(book, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.oldify(t)
	e.asked = nil
	askYes = func(_ *App, msg string) (bool, error) { // замене — «да», посторонним — «нет»
		e.asked = append(e.asked, msg)
		return !strings.Contains(msg, "посторонние"), nil
	}
	want := "В папке «Альбом» есть посторонние файлы (1 шт., например notes.txt): Фотосфера загрузит их вместе с альбомом. Продолжить?"
	if ok, err := e.a.Export(o); ok || err != nil || len(e.asked) != 2 || e.asked[1] != want {
		t.Fatalf("посторонние: %v %v, вопросы %q", ok, err, e.asked)
	}
	if e.rewritten(t, "01.jpg") || e.a.exporting {
		t.Fatal("«нет» — а файлы переписаны или экспорт не снят")
	}
}

// TestExportFsferaBlocked — запреты fsfera.Check — ошибка до вопросов и диалога папки:
// Нетпринт, не формат, лист 031; без режима те же альбомы выгружаются как раньше.
func TestExportFsferaBlocked(t *testing.T) {
	e, _ := fsferaEnv(t, 1, "netprint-premium")
	picked := 0
	pickExportDir = func(*App, string) (string, error) { picked++; return e.dir, os.MkdirAll(e.dir, 0o755) }
	sheet := model.NewSheet(204, 304)
	wide := model.New()
	wide.Settings.PageW, wide.Settings.PageH = 300, 300
	for _, c := range []struct {
		p    *model.Project
		want string
	}{
		{nil, "Обложка размечена для «Нетпринт · Премиум» — выберите «Фотосфера» в карточке обложки"},
		{sheet, "Лист виньетки в Фотосферу Pro не выгружается"},
		{wide, "Страница 300 × 300 мм — не формат Фотосферы. В «Страница» выберите пресет Фотосферы"},
	} {
		if c.p != nil {
			e.a.ed.Store(mustEditor(t, c.p, ""))
		}
		if ok, err := e.a.Export(ExportOptions{Print: "fsfera"}); ok || err == nil || err.Error() != c.want {
			t.Errorf("%q: %v %v", c.want, ok, err)
		}
	}
	if picked != 0 || len(e.asked) != 0 || e.a.exporting {
		t.Fatalf("выбор папки %d, вопросы %q, экспорт идёт %v", picked, e.asked, e.a.exporting)
	}
}

// TestExportFsferaAsk — без обложки: вопрос до папки, «нет» — ни папки книги, ни диалога; «да» —
// только развороты, свой 00.jpg прежней выгрузки удаляется (загрузчик взял бы старую обложку).
// Файл на месте папки книги — отказ после диалога, ничего не создано.
func TestExportFsferaAsk(t *testing.T) {
	e, memo := fsferaEnv(t, 1, "")
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App, title string) (string, error) {
		if picked++; title != "Куда выгрузить — папка книги будет внутри" {
			t.Errorf("заголовок диалога %q", title)
		}
		return pick(a, title)
	}
	e.answer = false
	ask := "Обложки нет: в папке будут только развороты, а для фотообложки нужен 00.jpg. Выгрузить без обложки?"
	if ok, err := e.a.Export(ExportOptions{Print: "fsfera"}); ok || err != nil || picked != 0 || !slices.Equal(e.asked, []string{ask}) {
		t.Fatalf("«нет»: %v %v, выбор папки %d, вопросы %q", ok, err, picked, e.asked)
	}
	if _, err := os.Stat(filepath.Join(e.dir, "Альбом")); err == nil {
		t.Fatal("«нет» оставил папку книги")
	}
	e.answer = true
	if msg := e.export(t, ExportOptions{Print: "fsfera"}); msg != "" || picked != 1 {
		t.Fatalf("«да»: %q, выбор папки %d", msg, picked)
	}
	if e.dir = filepath.Join(e.dir, "Альбом"); !slices.Equal(e.names(t), []string{"01.jpg"}) || !slices.Equal(*memo, []string{"Фотокнига 20×15", "Разворотов: 1", "Папка: Альбом — 01.jpg"}) {
		t.Fatalf("без обложки: файлы %v, памятка %q", e.names(t), *memo)
	}
	ours, err := os.ReadFile(filepath.Join(e.dir, "01.jpg")) // с меткой программы
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "00.jpg"), ours, 0o644); err != nil {
		t.Fatal(err)
	}
	if e.dir = filepath.Dir(e.dir); e.export(t, ExportOptions{Print: "fsfera"}) != "" {
		t.Fatal("повтор без обложки")
	}
	if e.dir = filepath.Join(e.dir, "Альбом"); !slices.Equal(e.names(t), []string{"01.jpg"}) {
		t.Fatalf("свой 00.jpg остался: %v", e.names(t))
	}
	// Файл на месте папки книги.
	e.dir = testkit.TempDir(t)
	if err := os.WriteFile(filepath.Join(e.dir, "Альбом"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.a.Export(ExportOptions{Print: "fsfera"}); ok || err == nil || !strings.Contains(err.Error(), "файл или ссылка") {
		t.Fatalf("файл на месте папки: %v %v", ok, err)
	}
}

// TestExportFsferaBookLast — папка книги создаётся только после всех вопросов (design §4.1 п. 4):
// пустое место в развороте, «нет» на вопрос о пустых местах — новой папки книги нет.
func TestExportFsferaBookLast(t *testing.T) {
	e, _ := fsferaEnv(t, 2, "fsfera-thin")
	p, _ := e.a.editor().Snapshot()
	s := p.Spreads[0]
	s.Grid = []*model.Place{{ID: "00000000a001", X: 10, Y: 10, W: 80, H: 60, Photo: s.Photos[0]}, {ID: "00000000a002", X: 110, Y: 10, W: 80, H: 60}}
	e.a.ed.Store(mustEditor(t, p, ""))
	e.answer = false
	if ok, err := e.a.Export(ExportOptions{Print: "fsfera"}); ok || err != nil || len(e.asked) != 1 || !strings.Contains(e.asked[0], "пустые места") {
		t.Fatalf("пустое место: %v %v, вопросы %q", ok, err, e.asked)
	}
	if _, err := os.Lstat(filepath.Join(e.dir, "Альбом")); !errors.Is(err, fs.ErrNotExist) || e.a.exporting {
		t.Fatalf("«нет» оставил папку книги (%v) или экспорт не снят", err)
	}
}

// TestExportCoverOffTemplateAsk — обычный экспорт со старой обложкой «Фотосферы» (заворот 23,5):
// вопрос до папки, «нет» ничего не пишет; без обложки в выгрузке вопроса нет, export:done —
// без памятки (memo nil: обычный тост).
func TestExportCoverOffTemplateAsk(t *testing.T) {
	e, memo := fsferaEnv(t, 1, "fsfera-thin")
	edWith := func(wrap float64) {
		p, _ := e.a.editor().Snapshot()
		p.Cover.Spec.Wrap = wrap
		e.a.ed.Store(mustEditor(t, p, ""))
	}
	edWith(23.5)
	e.answer = false
	want := "Обложка не по шаблону Фотосферы 20×15 (465 × 203 мм, нужно 458 × 196). Выгрузить как есть?"
	if ok, err := e.a.Export(ExportOptions{Cover: true, DPI: 72}); ok || err != nil || !slices.Equal(e.asked, []string{want}) {
		t.Fatalf("старая обложка: %v %v, вопросы %q", ok, err, e.asked)
	}
	if _, err := os.Stat(e.dir); err == nil {
		t.Fatal("«нет» — а папка выбрана")
	}
	e.asked, e.answer, *memo = nil, true, []string{"прежняя"}
	if msg := e.export(t, ExportOptions{Cover: false}); msg != "" || len(e.asked) != 0 || *memo != nil {
		t.Fatalf("без обложки: %q, вопросы %q, памятка %q", msg, e.asked, *memo)
	}
}

// TestFsferaPlan — план через App: имя папки по файлу альбома, rev и отмена не меняются;
// OpenFsfera открывает только адрес-константу; PagePresets — таблица модели.
func TestFsferaPlan(t *testing.T) {
	e, _ := fsferaEnv(t, 2, "fsfera-card")
	p, _ := e.a.editor().Snapshot()
	e.a.ed.Store(mustEditor(t, p, filepath.Join(testkit.TempDir(t), "Иванов.salbum")))
	before := e.a.editor().View()
	pl := e.a.PrintPlans()[0] // 20×15 — формат только Фотосферы: план один
	if pl.Format != "20×15" || pl.Folder != "Иванов" || pl.Pages != "Картон" || len(pl.Block) != 0 {
		t.Fatalf("план %+v", pl)
	}
	if v := e.a.editor().View(); v.Rev != before.Rev || v.CanUndo != before.CanUndo {
		t.Fatal("план изменил проект")
	}
	var opened []string
	old := openURL
	t.Cleanup(func() { openURL = old })
	openURL = func(_ *App, url string) { opened = append(opened, url) }
	e.a.OpenFsfera()
	if !slices.Equal(opened, []string{"https://fsfera.ru/order_pro/calculator/"}) {
		t.Fatalf("открыто %q", opened)
	}
	if !reflect.DeepEqual(e.a.PagePresets(), model.PagePresets()) {
		t.Fatal("PagePresets — не таблица модели")
	}
}

// coverEnv — альбом из n разворотов с обложкой: фото p0 на лицевой, плашка корешка красная.
func coverEnv(t *testing.T, n int, photo string) *exportEnv {
	t.Helper()
	e := newExportEnv(t, n)
	if _, err := e.a.CreateCover("", photo); err != nil {
		t.Fatal(err)
	}
	if v := e.a.SetCoverSpineBg("#ff0000"); v.Cover.SpineBg != "#ff0000" {
		t.Fatalf("плашка %q", v.Cover.SpineBg)
	}
	return e
}

// jpegAt читает 00.jpg и возвращает размер и цвет точки (x, y) мм листа при dpi.
func jpegAt(t *testing.T, path string, dpi int, x, y float64) (w, h int, c [3]uint8) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(int(math.Round(x/25.4*float64(dpi))), int(math.Round(y/25.4*float64(dpi)))).RGBA()
	return img.Bounds().Dx(), img.Bounds().Dy(), [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

// 00.jpg: размер листа обложки при dpi (как pxW×pxH вида), плашка корешка видна, развороты рядом.
func TestExportCoverFile(t *testing.T) {
	e := coverEnv(t, 3, "p0")
	v := e.a.editor().View()
	var spine model.CoverZone
	for _, z := range v.Cover.Zones {
		if z.Kind == "spine" {
			spine = z
		}
	}
	// dpi 0 — из настроек альбома: размер файла совпадает с pxW×pxH из вида.
	ok, err := e.a.Export(ExportOptions{Cover: true})
	if err != nil || !ok {
		t.Fatalf("Export: %v %v", ok, err)
	}
	if msg := <-e.done; msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	dpi := v.Settings.DPI
	w, h, c := jpegAt(t, filepath.Join(e.dir, "00.jpg"), dpi, spine.Rect.X+spine.Rect.W/2, 5)
	if w != v.Cover.PxW || h != v.Cover.PxH {
		t.Errorf("00.jpg %d×%d px, ждали %d×%d", w, h, v.Cover.PxW, v.Cover.PxH)
	}
	if c[0] < 230 || c[1] > 25 || c[2] > 25 {
		t.Errorf("плашка корешка не видна: %v", c)
	}
	// Повтор: наш 00.jpg заменяется после вопроса.
	e.asked = nil
	if msg := e.export(t, ExportOptions{Cover: true, Range: "2"}); msg != "" {
		t.Fatal(msg)
	}
	if len(e.asked) != 1 || !strings.Contains(e.asked[0], "Заменить обложку и файлы разворота 2 (2 шт.)?") {
		t.Fatalf("вопрос: %q", e.asked)
	}
}

// «Только обложка» — один файл; развороты, Range и хвосты прежнего экспорта не трогаются.
func TestExportCoverOnly(t *testing.T) {
	e := coverEnv(t, 3, "p0")
	e.export(t, ExportOptions{})
	e.oldify(t)
	e.asked = nil
	if msg := e.export(t, ExportOptions{CoverOnly: true, Range: "garbage"}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	if !e.rewritten(t, "00.jpg") || e.rewritten(t, "01.jpg") || e.rewritten(t, "03.jpg") || len(e.asked) != 0 {
		t.Fatalf("тронуты развороты или вопрос %q", e.asked)
	}
	// Чистая папка: только 00.jpg; без Cover «только обложка» тоже включает обложку.
	e.dir = filepath.Join(e.dir, "only")
	if msg := e.export(t, ExportOptions{CoverOnly: true}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"00.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("файлы %v", e.names(t))
	}
	// Альбом без разворотов, но с обложкой, выгружается.
	e0 := coverEnv(t, 0, "")
	if msg := e0.export(t, ExportOptions{Cover: true}); msg != "" || !slices.Equal(e0.names(t), []string{"00.jpg"}) {
		t.Fatalf("без разворотов: %q %v", msg, e0.names(t))
	}
}

// Обложки нет — флаги игнорируются и выгружаются развороты; чужой 00.jpg при выгрузке
// обложки — отказ без вопроса; без обложки в плане он не задевается.
func TestExportCoverIgnoredAndForeign(t *testing.T) {
	e := newExportEnv(t, 2)
	if msg := e.export(t, ExportOptions{Cover: true, CoverOnly: true}); msg != "" {
		t.Fatal(msg)
	}
	if want := []string{"01.jpg", "02.jpg"}; !slices.Equal(e.names(t), want) {
		t.Fatalf("без обложки: %v", e.names(t))
	}
	c := coverEnv(t, 2, "p0")
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, c.dir, "00.jpg", 10, 10)
	ok, err := c.a.Export(ExportOptions{DPI: 72, Cover: true})
	if ok || err == nil || !strings.Contains(err.Error(), "00.jpg") || len(c.asked) != 0 {
		t.Fatalf("чужой 00.jpg: %v, %v, вопросы %q", ok, err, c.asked)
	}
	if msg := c.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	if st, err := os.Stat(filepath.Join(c.dir, "00.jpg")); err != nil || st.Size() > 5000 {
		t.Fatalf("чужой 00.jpg тронут: %v", err)
	}
}

// Пустая обложка — то же предупреждение, что о пустых разворотах, до выбора папки.
func TestExportCoverBlankAsks(t *testing.T) {
	e := coverEnv(t, 1, "")
	e.a.SetCoverSpineBg("") // плашка — тоже печать
	e.answer = false
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App, title string) (string, error) { picked++; return pick(a, title) }
	ok, err := e.a.Export(ExportOptions{DPI: 72, Cover: true})
	if ok || err != nil || picked != 0 || len(e.asked) != 1 || !strings.Contains(e.asked[0], "Обложка пустая") {
		t.Fatalf("Export = %v, %v; папка %d; вопросы %q", ok, err, picked, e.asked)
	}
	// Без галочки обложки о ней не спрашиваем.
	e.asked = nil
	if msg := e.export(t, ExportOptions{}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("без обложки: %q, вопросы %q", msg, e.asked)
	}
}

// Подстановки (030) в тексте обложки: на холсте «[Заголовок]» (прежнее {Школа} — синоним), в печати — пусто.
func TestCoverTextExpands(t *testing.T) {
	e := coverEnv(t, 1, "p0")
	ed := e.a.editor()
	v, err := ed.AddText("cover", 0, "")
	if err != nil || len(v.Cover.Sheet.Texts) != 1 {
		t.Fatalf("текст обложки: %v %+v", err, v.Cover)
	}
	v = ed.SetText(v.Cover.Sheet.Texts[0].ID, "{Школа}", "")
	if got := v.Cover.Sheet.Texts[0].Text; got != "[Заголовок]" {
		t.Errorf("на холсте %q", got)
	}
	p, pv := ed.Snapshot()
	if got := pv.Cover.Sheet.Texts[0]; got.Text != "" || printed(got.Lines) != "" {
		t.Errorf("в печати %q, строки %q", got.Text, printed(got.Lines))
	}
	job := export.NewCoverJob(p, pv, 72, 95)
	if job == nil || len(job.Spread.Texts) != 1 || printed(job.Spread.Texts[0].Box.Lines) != "" {
		t.Errorf("задание обложки: %+v", job)
	}
}

// printed — текст всех строк блока подряд.
func printed(lines []text.Line) (out string) {
	for _, l := range lines {
		out += l.Text
	}
	return out
}
