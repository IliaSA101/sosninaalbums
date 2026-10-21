package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Выгрузка для Фабрики Фотокниги (081) и планы типографий через App; помощники 047 — в exportfs_test.go.

// printEnv — exportEnv на странице w×h (зона 10) с обложкой preset ("" — без) по шаблону
// (лицевая цветная: вопроса о пустой обложке нет); memo — 4-й аргумент export:done.
func printEnv(t *testing.T, n int, w, h float64, preset string) (*exportEnv, *[]string) {
	t.Helper()
	e, memo := fsferaEnv(t, n, "")
	p, _ := e.a.editor().Snapshot()
	p.Settings.PageW, p.Settings.PageH = w, h
	if preset != "" {
		pr, _ := model.CoverPresetFor(preset, p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, BgR: "#203040"}, Spec: pr.Spec}
	}
	e.a.ed.Store(mustEditor(t, p, ""))
	return e, memo
}

// book — проект w×h из n пустых разворотов с обложкой preset ("" — без): для запретов, до записи.
func book(w, h float64, n int, preset string) *model.Project {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = w, h, 10
	for range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	if preset != "" {
		pr, _ := model.CoverPresetFor(preset, p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec}
	}
	return p
}

// quality100 — все таблицы квантования JPEG из единиц: так кодирует только качество 100.
func quality100(t *testing.T, path string) bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 2; i+4 < len(b) && b[i] == 0xFF; {
		marker, size := b[i+1], int(b[i+2])<<8|int(b[i+3])
		if marker == 0xDA { // данные скана: таблицы уже позади
			break
		}
		if marker == 0xDB {
			for seg := b[i+4 : i+2+size]; len(seg) >= 65; seg = seg[65:] {
				if seg[0]>>4 != 0 || slices.ContainsFunc(seg[1:65], func(v byte) bool { return v != 1 }) {
					return false
				}
				found = true
			}
		}
		i += 2 + size
	}
	return found
}

// TestExportFabrika — режим Фабрики: диапазон, страницы, dpi, качество и «только обложка» окна
// игнорируются; в папке книги — 1-00.jpg, 1-01.jpg, 1-02.jpg в 300 dpi и качестве 100; памятка
// в export:done; повтор — только вопрос о замене; файлы обычного экспорта — посторонние.
func TestExportFabrika(t *testing.T) {
	e, memo := printEnv(t, 2, 200, 203, "fabrika-photo")
	o := ExportOptions{Print: "fabrika", Range: "1", Pages: true, CoverOnly: true, Quality: 80}
	if msg := e.export(t, o); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка: %q, вопросы %q", msg, e.asked)
	}
	bk := filepath.Join(e.dir, "Альбом")
	p, v := e.a.editor().Snapshot()
	cw, ch := render.CanvasSize(export.NewCoverJob(p, v, 300, 100).Opt)
	if pxOf(t, filepath.Join(bk, "1-01.jpg")) != [2]int{4724, 2398} || pxOf(t, filepath.Join(bk, "1-00.jpg")) != [2]int{cw, ch} {
		t.Fatalf("px: обложка %v (ждали %d×%d), разворот %v", pxOf(t, filepath.Join(bk, "1-00.jpg")), cw, ch, pxOf(t, filepath.Join(bk, "1-01.jpg")))
	}
	for _, n := range []string{"1-00.jpg", "1-01.jpg"} {
		if !quality100(t, filepath.Join(bk, n)) {
			t.Errorf("%s — не качество 100", n)
		}
	}
	e.dir = bk
	if got := e.names(t); !slices.Equal(got, []string{"1-00.jpg", "1-01.jpg", "1-02.jpg"}) {
		t.Fatalf("файлы книги %v", got)
	}
	if want := []string{"Фотокнига на фотобумаге 20×20", "Разворот: Фотобумага", "Разворотов: 2 (корешок 8 мм)", "Папка: Альбом — 1-00.jpg и 1-01…1-02.jpg"}; !slices.Equal(*memo, want) {
		t.Fatalf("памятка %q", *memo)
	}
	// Повтор из родительской папки: вопрос о замене своих 1-NN.jpg, посторонних нет.
	e.oldify(t)
	e.dir = filepath.Dir(bk)
	if msg := e.export(t, o); msg != "" || !slices.Equal(e.asked, []string{"В папке уже есть обложка и развороты прежнего экспорта: 3. Заменить?"}) {
		t.Fatalf("повтор: %q, вопросы %q", msg, e.asked)
	}
	// Обычный экспорт в ту же папку (01.jpg, 02.jpg с меткой): для Фабрики — посторонние; «нет» ничего не пишет.
	e.dir, e.asked = bk, nil
	if msg := e.export(t, ExportOptions{}); msg != "" {
		t.Fatal(msg)
	}
	e.oldify(t)
	e.asked = nil
	askYes = func(_ *App, msg string) (bool, error) {
		e.asked = append(e.asked, msg)
		return !strings.Contains(msg, "посторонние"), nil
	}
	want := "В папке «Альбом» есть посторонние файлы (2 шт., например 01.jpg): Фабрика Фотокниги загрузит их вместе с альбомом. Продолжить?"
	if ok, err := e.a.Export(o); ok || err != nil || len(e.asked) != 2 || e.asked[1] != want {
		t.Fatalf("посторонние: %v %v, вопросы %q", ok, err, e.asked)
	}
	if e.rewritten(t, "1-01.jpg") || e.rewritten(t, "01.jpg") || e.a.exporting {
		t.Fatal("«нет» — а файлы переписаны или экспорт не снят")
	}
}

// TestExportFabrikaNoCover — без обложки: вопрос Фабрики до папки; «да» — только 1-01.jpg, свой
// 1-00.jpg прежней выгрузки удаляется, а 00.jpg Фотосферы (тоже свой) — нет: он посторонний.
func TestExportFabrikaNoCover(t *testing.T) {
	e, memo := printEnv(t, 1, 304, 203, "")
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" ||
		!slices.Equal(e.asked, []string{"Обложки нет: в папке будут только развороты, а для обложки нужен 1-00.jpg. Выгрузить без обложки?"}) {
		t.Fatalf("без обложки: %q, вопросы %q", msg, e.asked)
	}
	e.dir = filepath.Join(e.dir, "Альбом")
	if !slices.Equal(e.names(t), []string{"1-01.jpg"}) || !slices.Equal(*memo, []string{"Фотокнига 30×20", "Разворотов: 1", "Папка: Альбом — 1-01.jpg"}) {
		t.Fatalf("файлы %v, памятка %q", e.names(t), *memo)
	}
	ours, err := os.ReadFile(filepath.Join(e.dir, "1-01.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"1-00.jpg", "00.jpg"} {
		if err := os.WriteFile(filepath.Join(e.dir, n), ours, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	e.asked = nil
	if e.dir = filepath.Dir(e.dir); e.export(t, ExportOptions{Print: "fabrika"}) != "" || len(e.asked) != 3 ||
		!strings.Contains(e.asked[2], "(1 шт., например 00.jpg): Фабрика Фотокниги") {
		t.Fatalf("повтор без обложки: вопросы %q", e.asked)
	}
	if e.dir = filepath.Join(e.dir, "Альбом"); !slices.Equal(e.names(t), []string{"00.jpg", "1-01.jpg"}) {
		t.Fatalf("свой 1-00.jpg остался или 00.jpg удалён: %v", e.names(t))
	}
}

// TestExportFabrikaBlocked — запреты printshop.Check для Фабрики — ошибка до вопросов и диалога
// папки, строки через «. » в порядке правил.
func TestExportFabrikaBlocked(t *testing.T) {
	e, _ := printEnv(t, 1, 200, 203, "fabrika-photo")
	picked := 0
	pickExportDir = func(*App, string) (string, error) { picked++; return e.dir, os.MkdirAll(e.dir, 0o755) }
	for _, c := range []struct {
		p    *model.Project
		want string
	}{
		{book(250, 254, 12, "fabrika-layflat-paper"), "«Лайфлат, бумага» не бывает 25×25 — выберите другую обложку"},
		{book(208, 296, 2, "fsfera-card"), "Обложка размечена для «Фотосфера · Картон» — выберите «Фабрика Фотокниги» в карточке обложки"},
		{book(300, 305, 9, "fabrika-photo"), "Разворотов 9 — у Фабрики Фотокниги («Фотобумага») не меньше 10"},
		{book(300, 305, 41, "netprint-premium"), "Обложка размечена для «Нетпринт · Премиум» — выберите «Фабрика Фотокниги» в карточке обложки. " +
			"Разворотов 41 — у Фабрики Фотокниги не больше 40"},
		{book(201, 152, 2, ""), "Страница 201 × 152 мм — не формат Фабрики Фотокниги. В «Страница» выберите пресет Фабрики Фотокниги"},
		{model.NewSheet(204, 304), "Лист виньетки в Фабрику Фотокниги не выгружается"},
	} {
		e.a.ed.Store(mustEditor(t, c.p, ""))
		if ok, err := e.a.Export(ExportOptions{Print: "fabrika"}); ok || err == nil || err.Error() != c.want {
			t.Errorf("%q: %v %v", c.want, ok, err)
		}
	}
	if picked != 0 || len(e.asked) != 0 || e.a.exporting {
		t.Fatalf("выбор папки %d, вопросы %q, экспорт идёт %v", picked, e.asked, e.a.exporting)
	}
}

// TestExportFabrikaReplace — чужой 1-03.jpg в папке книги — отказ с именем обложки Фабрики, ничего
// не записано; без него — запись, свой 1-03.jpg при повторе заменяется после вопроса.
func TestExportFabrikaReplace(t *testing.T) {
	e, _ := printEnv(t, 3, 200, 203, "fabrika-photo")
	bk := filepath.Join(e.dir, "Альбом")
	if err := os.MkdirAll(bk, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, bk, "1-03.jpg", 10, 10)
	want := "В папке есть файлы с номерами разворотов (или 1-00.jpg) не из Sosnina Albums (1 шт., например 1-03.jpg). Выберите другую папку или уберите их."
	if ok, err := e.a.Export(ExportOptions{Print: "fabrika"}); ok || err == nil || err.Error() != want || len(e.asked) != 0 {
		t.Fatalf("чужой 1-03.jpg: %v %v, вопросы %q", ok, err, e.asked)
	}
	if e.dir = bk; !slices.Equal(e.names(t), []string{"1-03.jpg"}) || e.a.exporting {
		t.Fatalf("после отказа: %v", e.names(t))
	}
	if err := os.Remove(filepath.Join(bk, "1-03.jpg")); err != nil {
		t.Fatal(err)
	}
	e.dir = filepath.Dir(bk)
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("запись: %q, вопросы %q", msg, e.asked)
	}
	e.dir = bk
	e.oldify(t)
	e.dir = filepath.Dir(bk)
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" || len(e.asked) != 1 {
		t.Fatalf("повтор: %q, вопросы %q", msg, e.asked)
	}
	if e.dir = bk; !e.rewritten(t, "1-03.jpg") || !slices.Equal(e.names(t), []string{"1-00.jpg", "1-01.jpg", "1-02.jpg", "1-03.jpg"}) {
		t.Fatalf("замена: %v", e.names(t))
	}
}

// TestExportPrintUnknown — неизвестная типография — ошибка до всех вопросов и диалога папки.
func TestExportPrintUnknown(t *testing.T) {
	e, _ := printEnv(t, 1, 200, 203, "fabrika-photo")
	picked := 0
	pickExportDir = func(*App, string) (string, error) { picked++; return e.dir, nil }
	for _, kind := range []string{"netprint", "FSFERA", " "} {
		if ok, err := e.a.Export(ExportOptions{Print: kind}); ok || err == nil || err.Error() != "неизвестная типография" {
			t.Errorf("%q: %v %v", kind, ok, err)
		}
	}
	if picked != 0 || len(e.asked) != 0 || e.a.exporting {
		t.Fatalf("выбор папки %d, вопросы %q, экспорт идёт %v", picked, e.asked, e.a.exporting)
	}
}

// TestPrintPlans — планы типографий, в таблице которых есть страница: 21×30 — Фотосфера и Фабрика
// (по порядку), 30×30 Фабрики — одна, 300×300 и лист 031 — пустой список ([] в JSON). Проект не меняется.
func TestPrintPlans(t *testing.T) {
	e, _ := printEnv(t, 2, 208, 296, "fabrika-layflat-card")
	before := e.a.editor().View()
	kinds := func() (out []string) {
		for _, pl := range e.a.PrintPlans() {
			out = append(out, pl.Kind)
		}
		return out
	}
	if got := kinds(); !slices.Equal(got, []string{"fsfera", "fabrika"}) {
		t.Fatalf("21×30: %v", got)
	}
	plans := e.a.PrintPlans()
	if len(plans[0].Block) != 1 || len(plans[1].Block) != 0 || plans[1].Pages != "Картон" || plans[1].Quality != 100 || plans[0].Quality != 0 {
		t.Fatalf("21×30 с обложкой Фабрики: %+v", plans)
	}
	if v := e.a.editor().View(); v.Rev != before.Rev || v.CanUndo != before.CanUndo {
		t.Fatal("планы изменили проект")
	}
	for _, c := range []struct {
		p    *model.Project
		want []string
	}{
		{book(300, 305, 12, "fabrika-photo"), []string{"fabrika"}},
		{book(201, 152, 2, ""), []string{"fsfera"}},
		{book(300, 300, 2, ""), nil},
		{model.NewSheet(204, 304), nil},
	} {
		e.a.ed.Store(mustEditor(t, c.p, ""))
		if got := kinds(); !slices.Equal(got, c.want) {
			t.Errorf("%v×%v: %v, ждали %v", c.p.Settings.PageW, c.p.Settings.PageH, got, c.want)
		}
	}
	if b, _ := json.Marshal(e.a.PrintPlans()); !bytes.Equal(b, []byte("[]")) {
		t.Fatalf("пустой список: %s", b)
	}
}

// TestOpenFabrikaURL — «Открыть Фабрику Фотокниги» открывает только адрес-константу по обложке альбома.
func TestOpenFabrikaURL(t *testing.T) {
	var opened []string
	old := openURL
	t.Cleanup(func() { openURL = old })
	openURL = func(_ *App, url string) { opened = append(opened, url) }
	e, _ := printEnv(t, 1, 200, 203, "fabrika-layflat-card")
	e.a.OpenFabrika()
	for _, preset := range []string{"fabrika-photo-1", "", "netprint-premium"} {
		e.a.ed.Store(mustEditor(t, book(200, 203, 1, preset), ""))
		e.a.OpenFabrika()
	}
	if want := []string{"https://fabrika-fotoknigi.com/layflat", "https://fabrika-fotoknigi.com/photobook-photopaper",
		"https://fabrika-fotoknigi.com/photo-books", "https://fabrika-fotoknigi.com/photo-books"}; !slices.Equal(opened, want) {
		t.Fatalf("открыто %q", opened)
	}
}
