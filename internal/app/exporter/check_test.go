package exporter

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Check без побочных эффектов: ни окон, ни цвета фото, ни запоминания папки, ни флага busy (в том
// числе пока идёт проверка файлов фото), ни хода выгрузки; отказ — текст окна экспорта.
func TestExportCheckNoSideEffects(t *testing.T) {
	missing := filepath.Join(testkit.TempDir(t), "нет-такого.jpg") // локальный путь своей ОС: C:\\… на Windows
	ed := spreadsEditor(t, 2, func(p *model.Project) {
		p.Photos = map[string]model.PhotoRef{"a": {Path: missing}}
		p.Spreads[0].Photos = []string{"a"}
	})
	var s *Service
	h, _, _ := fakeHost(t, func() *album.Editor { t.Error("Host.Editor в Check"); return nil })
	h.Exists = func(string) bool {
		if busy, _ := s.Busy(); busy {
			t.Error("Check поднял busy")
		}
		return true
	}
	h.ColorReport = func(*model.Project) (map[string]int, map[string]int) {
		t.Error("Check читает цвет фото")
		return nil, nil
	}
	h.RememberDir = func(string, string) { t.Error("Check запоминает папку") }
	s = New(h)
	sm, err := s.Check(ed, Options{DPI: 72}, "")
	if err != nil || sm.Files != 2 {
		t.Fatalf("Check: %+v, %v", sm, err)
	}
	if busy, _ := s.Busy(); busy || s.Last().State != StateIdle || s.LastDir() != "" {
		t.Fatalf("след Check: busy %v, ход %+v, папка %q", busy, s.Last(), s.LastDir())
	}
	// Файла фото нет — отказ как в окне, без окон.
	h.Exists = func(string) bool { return false }
	s = New(h)
	if _, err := s.Check(ed, Options{}, ""); err == nil {
		t.Fatal("пропавшее фото должно отказывать в Check")
	}
	if _, err := s.Check(nil, Options{}, ""); err == nil {
		t.Fatal("Check без альбома: ждали ошибку, не панику")
	}
}

// Каждый отказ Export есть и в Check — теми же словами, флаг не остаётся.
func TestExportCheckRefusals(t *testing.T) {
	huge := func(p *model.Project) { p.Settings.PageW, p.Settings.PageH, p.Settings.DPI = 1000, 1000, 1200 }
	cases := []struct {
		name string
		n    int
		mod  func(*model.Project)
		o    Options
		want string
	}{
		{"типография", 2, nil, Options{Print: "нет-такой"}, "неизвестная типография"},
		{"запрет типографии", 2, nil, Options{Print: "fabrika"}, "Фабрик"},
		{"диапазон-мусор", 2, nil, Options{Range: "x"}, "не понимаю «x»"},
		{"диапазон-за-пределом", 1, nil, Options{Range: "99"}, "разворота"},
		{"холст", 2, huge, Options{}, "слишком большой"},
		{"пустой-альбом", 0, nil, Options{}, "нечего экспортировать"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ed := spreadsEditor(t, c.n, c.mod)
			h, _, _ := fakeHost(t, func() *album.Editor { return nil })
			s := New(h)
			_, errC := s.Check(ed, c.o, "")
			_, errE := s.ExportWith(Job{Ed: ed, Rev: -1, Opt: c.o})
			if errC == nil || !strings.Contains(errC.Error(), c.want) || errE == nil || errC.Error() != errE.Error() {
				t.Fatalf("Check %v, ExportWith %v; ждали «%s» в обоих", errC, errE, c.want)
			}
		})
	}
}

// Итог Check: цифры и строки вопроса — разворотов, диапазон, обложка, страницы, типография.
func TestExportCheckSummary(t *testing.T) {
	h, _, _ := fakeHost(t, func() *album.Editor { return nil })
	s := New(h)
	three := spreadsEditor(t, 3, nil)
	dpi := 300
	cases := []struct {
		name string
		ed   *album.Editor
		o    Options
		want string
		n    int
	}{
		{"все", three, Options{DPI: dpi}, "Развороты 1–3 · 300 dpi · качество 95 · 3 файла", 3},
		{"диапазон", three, Options{DPI: dpi, Range: "2-3"}, "Развороты 2-3 · 300 dpi · качество 95 · 2 файла", 2},
		{"один", three, Options{DPI: dpi, Range: "2", Quality: 80}, "Разворот 2 · 300 dpi · качество 80 · 1 файл", 1},
		{"страницы", three, Options{DPI: dpi, Range: "1", Pages: true}, "Разворот 1 по страницам · 300 dpi · качество 95 · 2 файла", 2},
		{"качество вне перечня — 95, dpi зажат", three, Options{DPI: 20, Quality: 90}, "Развороты 1–3 · 72 dpi · качество 95 · 3 файла", 3},
	}
	for _, c := range cases {
		sm, err := s.Check(c.ed, c.o, "")
		if err != nil || len(sm.Lines) < 2 || sm.Lines[0] != c.want || sm.Files != c.n || sm.Lines[1] != "Обычная выгрузка" || sm.Print != "" {
			t.Errorf("%s: %+v, %v; ждали «%s»", c.name, sm, err, c.want)
		}
	}
	// Обложка: с развороты, только она.
	cov, err := album.NewEditor(book(200, 203, 2, "fabrika-photo"), "")
	if err != nil {
		t.Fatal(err)
	}
	sm, err := s.Check(cov, Options{DPI: 100, Cover: true}, "")
	if err != nil || sm.Lines[0] != "Развороты 1–2 и обложка · 100 dpi · качество 95 · 3 файла" || !sm.Cover || sm.CoverOnly {
		t.Fatalf("с обложкой: %+v, %v", sm, err)
	}
	sm, err = s.Check(cov, Options{DPI: 100, CoverOnly: true}, "")
	if err != nil || sm.Lines[0] != "Только обложка · 100 dpi · качество 95 · 1 файл" || !sm.CoverOnly || sm.Spreads != 0 {
		t.Fatalf("только обложка: %+v, %v", sm, err)
	}
	if sm, err = s.Check(three, Options{DPI: 100, Cover: true}, ""); err != nil || sm.Cover {
		t.Fatalf("обложки нет — флаг игнорируется: %+v, %v", sm, err)
	}
}

// Строки вопроса: мало разрешения и предел разворотов — в обычной выгрузке; предупреждения
// типографии — в её режиме, предел там проверяет план.
func TestSummaryLinesWarnings(t *testing.T) {
	v := model.View{LowDPI: 3, PageLimit: &model.PageLimitView{Warn: "Разворотов мало"}}
	pr := &prepared{p: model.New(), v: v, n: 2, o: Options{}, pl: export.NewPlan(nil, 2, false)}
	sm := Summary{DPI: 300, Quality: 95, Files: 2}
	got := summaryLines(pr, sm)
	if !slices.Equal(got[1:], []string{"Обычная выгрузка", "Мало разрешения: 3 фото", "Разворотов мало"}) {
		t.Fatalf("обычная: %q", got)
	}
	prof, ok := printshop.ProfileOf("fabrika")
	if !ok {
		t.Fatal("нет fabrika")
	}
	pr.pj = &printJob{prof: prof, plan: printshop.Plan{Warn: []string{"Совет типографии"}}}
	got = summaryLines(pr, sm)
	if len(got) != 4 || !strings.HasPrefix(got[1], "Для ") || !strings.Contains(got[1], "папка книги будет внутри") ||
		got[2] != "Мало разрешения: 3 фото" || got[3] != "Совет типографии" {
		t.Fatalf("типография: %q", got)
	}
	// Лист 031: «Лист», а не «Развороты».
	sheet := model.New()
	sheet.Settings.Sheet = true
	pr = &prepared{p: sheet, n: 1, pl: export.NewPlan(nil, 1, false)}
	if got := summaryLines(pr, Summary{DPI: 300, Quality: 95, Files: 1}); got[0] != "Лист 1 · 300 dpi · качество 95 · 1 файл" {
		t.Fatalf("лист: %q", got)
	}
	// Длинный диапазон от агента в строку целиком не идёт.
	pr = &prepared{p: model.New(), n: 99, o: Options{Range: strings.Repeat("1, ", 100)}, pl: export.NewPlan([]export.Unit{{Spread: 1}}, 99, false)}
	if got := summaryLines(pr, Summary{}); len([]rune(got[0])) > 120 {
		t.Fatalf("диапазон не обрезан: %d знаков", len([]rune(got[0])))
	}
}

func TestPluralAndSameNames(t *testing.T) {
	for n, w := range map[int]string{0: "файлов", 1: "файл", 2: "файла", 4: "файла", 5: "файлов", 11: "файлов", 14: "файлов", 21: "файл", 25: "файлов", 101: "файл", 112: "файлов"} {
		if got := plural(n, "файл", "файла", "файлов"); got != w {
			t.Errorf("plural(%d)=%q, ждали %q", n, got, w)
		}
	}
	if !sameNames(nil, []string{}) || !sameNames([]string{"b", "a"}, []string{"a", "b", "a"}) || sameNames([]string{"a"}, []string{"b"}) || sameNames([]string{"a"}, nil) {
		t.Fatal("sameNames")
	}
	if short("абвгд", 3) != "абв…" || short("аб", 3) != "аб" {
		t.Fatal("short")
	}
}

// Ход последней выгрузки: idle → running (ключ альбома) → done; остановка и ошибка — без пути;
// фон закрыт — stopped. Выгрузка другого альбома не мешает сверке по AlbumID.
func TestExportLast(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	dir := testkit.TempDir(t)
	r := newRig(t, nil)
	if st := r.s.Last(); st.State != StateIdle || st.AlbumID != 0 {
		t.Fatalf("до выгрузок: %+v", st)
	}
	job := Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, false, nil)}
	if ok, err := r.s.ExportWith(job); !ok || err != nil || len(r.bg) != 1 {
		t.Fatalf("старт: %v, %v, фон %d", ok, err, len(r.bg))
	}
	if st := r.s.Last(); st.State != StateRunning || st.AlbumID != ed.ID() || st.Total != 2 || st.Done != 0 {
		t.Fatalf("идёт: %+v", st)
	}
	r.bg[0]()
	if st := r.s.Last(); st.State != StateDone || st.Done != 2 || st.Total != 2 || st.Error != "" {
		t.Fatalf("готово: %+v", st)
	}
	// Остановка: контекст выхода отменён до записи.
	ctx, cancel := context.WithCancel(context.Background())
	r.s.h.StopCtx = func() context.Context { return ctx }
	r.bg = nil
	if ok, err := r.s.ExportWith(job); !ok || err != nil {
		t.Fatal(ok, err)
	}
	cancel()
	r.bg[0]()
	if st := r.s.Last(); st.State != StateStopped || st.Error != "" {
		t.Fatalf("остановлена: %+v", st)
	}
	// Ошибка записи: папку убрали после старта; в ход идёт текст без пути.
	r.s.h.StopCtx = context.Background
	r.bg = nil
	if ok, err := r.s.ExportWith(job); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir, []byte("файл на месте папки"), 0o644); err != nil {
		t.Fatal(err)
	}
	var done []any
	r.s.h.Emit = func(name string, data ...any) {
		if name == "export:done" {
			done = data
		}
	}
	r.bg[0]()
	st := r.s.Last()
	if st.State != StateFailed || st.Error == "" || strings.Contains(st.Error, dir) || strings.ContainsAny(st.Error, `/\`) {
		t.Fatalf("ошибка записи: %+v", st)
	}
	if msg, _ := done[1].(string); !strings.Contains(msg, dir) {
		t.Fatalf("в export:done путь должен быть (человек видит его тостом), а в ходе агента — нет: %q", msg)
	}
	// Фон закрыт: выгрузки не будет — не «идёт».
	r.s.h.GoBG = func(func()) bool { return false }
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.s.ExportWith(job); ok || err != nil {
		t.Fatal(ok, err)
	}
	if st := r.s.Last(); st.State != StateStopped {
		t.Fatalf("фон закрыт: %+v", st)
	}
	if busy, _ := r.s.Busy(); busy {
		t.Fatal("флаг остался")
	}
}
