package export

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка в плане: имя 00.jpg первым, вопрос о замене, файл 00.jpg задевается только с обложкой.
func TestPlanCoverNames(t *testing.T) {
	pl := NewPlan(nil, 3, false)
	if got := pl.Names(3); !slices.Equal(got, []string{"01.jpg", "02.jpg", "03.jpg"}) {
		t.Fatalf("без обложки: %v", got)
	}
	cp := pl.WithCover(&CoverJob{})
	if got := cp.Names(3); !slices.Equal(got, []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}) {
		t.Fatalf("с обложкой: %v", got)
	}
	f := DirFile{Name: "00.jpg"}
	if len(HitFiles(pl, []DirFile{f})) != 0 || len(HitFiles(cp, []DirFile{f})) != 1 {
		t.Fatal("00.jpg должен задеваться только планом с обложкой")
	}
	// Только обложка: пустой план не Full, хвосты разворотов не задеваются.
	only := Plan{}.WithCover(&CoverJob{})
	if got := HitFiles(only, []DirFile{f, {Name: "05.jpg", Spread: 5}}); len(got) != 1 || got[0].Name != "00.jpg" {
		t.Fatalf("только обложка задела %v", got)
	}
	hit := []DirFile{f, {Name: "02.jpg", Spread: 2}}
	for _, c := range []struct {
		pl   Plan
		hit  []DirFile
		want string
	}{
		{only, hit[:1], "В папке уже есть обложка прежнего экспорта (00.jpg). Заменить?"},
		{cp, hit, "В папке уже есть обложка и развороты прежнего экспорта: 2. Заменить?"},
		{NewPlan([]Unit{{2, 0}}, 3, false).WithCover(&CoverJob{}), hit, "Заменить обложку и файлы разворота 2 (2 шт.)?"},
		{NewPlan([]Unit{{2, 0}}, 3, false), hit[1:], "Заменить файлы разворота 2 (1 шт.)?"},
	} {
		if got := ReplaceQuestion(c.pl, c.hit); got != c.want {
			t.Errorf("вопрос %q, ждали %q", got, c.want)
		}
	}
}

// 00.jpg в папке: наш — в ours, чужой — в foreign, нет файла или обложки в плане — ничего.
func TestAddCoverFile(t *testing.T) {
	dir := testkit.TempDir(t)
	cp := Plan{}.WithCover(&CoverJob{})
	if o, f := AddCoverFile(cp, dir, nil, nil); len(o)+len(f) != 0 {
		t.Fatal("файла нет, а в списках есть")
	}
	if err := os.WriteFile(filepath.Join(dir, "00.jpg"), []byte("чужой"), 0o644); err != nil {
		t.Fatal(err)
	}
	if o, f := AddCoverFile(Plan{}, dir, nil, nil); len(o)+len(f) != 0 {
		t.Fatal("без обложки в плане 00.jpg не трогаем")
	}
	if o, f := AddCoverFile(cp, dir, nil, nil); len(o) != 0 || len(f) != 1 {
		t.Fatalf("чужой: ours %v foreign %v", o, f)
	}
	ours := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	if err := os.WriteFile(filepath.Join(dir, "00.jpg"), ours, 0o644); err != nil {
		t.Fatal(err)
	}
	if o, f := AddCoverFile(cp, dir, nil, nil); len(o) != 1 || len(f) != 0 {
		t.Fatalf("наш: ours %v foreign %v", o, f)
	}
	// Не обычный файл — чужой (отказ), даже если цель ссылки — наш 00.jpg: ссылку не заменяем молча.
	odd := testkit.TempDir(t)
	if err := os.Mkdir(filepath.Join(odd, CoverName), 0o755); err != nil {
		t.Fatal(err)
	}
	if o, f := AddCoverFile(cp, odd, nil, nil); len(o) != 0 || len(f) != 1 {
		t.Fatalf("папка 00.jpg: ours %v foreign %v", o, f)
	}
	ln := testkit.TempDir(t)
	if err := os.Symlink(filepath.Join(dir, CoverName), filepath.Join(ln, CoverName)); err != nil {
		t.Logf("ссылки недоступны: %v", err)
		return
	}
	if o, f := AddCoverFile(cp, ln, nil, nil); len(o) != 0 || len(f) != 1 {
		t.Fatalf("ссылка на наш 00.jpg: ours %v foreign %v", o, f)
	}
}

// Предупреждение о пустом: только обложка, обложка и разворот(ы), без обложки — прежний текст.
func TestBlankWarningCover(t *testing.T) {
	for _, c := range []struct {
		nums         []int
		cover, white bool
		want         string
	}{
		{nil, true, true, "Обложка пустая — выгрузить белой?"},
		{nil, true, false, "Обложка пустая — выгрузить с фоном альбома?"},
		{[]int{3}, true, true, "Обложка и разворот 3 пустые — выгрузить белыми?"},
		{[]int{3, 5}, true, true, "Обложка и развороты 3, 5 пустые — выгрузить белыми?"},
		{[]int{3}, false, true, "Разворот 3 пустой — выгрузить белым?"},
		{nil, false, true, ""},
	} {
		if got := BlankWarningCover(c.nums, c.cover, c.white); got != c.want {
			t.Errorf("%v %v: %q, ждали %q", c.nums, c.cover, got, c.want)
		}
	}
}

// Пустая обложка: ни фото, ни мест, ни текстов, ни плашки; цвет страниц и плашка — печать.
func TestBlankCover(t *testing.T) {
	if BlankCover(model.View{}) {
		t.Fatal("обложки нет — не пустая")
	}
	v := model.View{Cover: &model.CoverView{}}
	if !BlankCover(v) {
		t.Fatal("пустая обложка не распознана")
	}
	v.Cover.SpineBg = "#112233"
	if BlankCover(v) {
		t.Error("плашка корешка — это печать")
	}
	v.Cover.SpineBg = ""
	v.Cover.Sheet.BgSet = [2]string{"#ffeeee", ""}
	if BlankCover(v) {
		t.Error("свой цвет страницы — это печать")
	}
}

// Фото обложки учитываются в выгрузке только при обложке в плане; фон с Fade ≥ 1 не в счёт.
func TestSelectedPhotosCover(t *testing.T) {
	v := model.View{Cover: &model.CoverView{Sheet: model.SpreadView{Photos: []string{"a"},
		Backdrops: []model.BackdropView{{Photo: "bg"}, {Photo: "hidden", Fade: 1}}}}}
	if got := (Plan{}).SelectedPhotos(v, 100); len(got) != 0 {
		t.Fatalf("без обложки: %v", got)
	}
	got := Plan{}.WithCover(&CoverJob{}).SelectedPhotos(v, 100)
	if len(got) != 2 || !got["a"] || !got["bg"] || got["hidden"] {
		t.Fatalf("с обложкой: %v", got)
	}
}

// coverEditor — альбом из трёх пустых разворотов с обложкой «тонкие листы» и плашкой корешка.
func coverEditor(t *testing.T) (*model.Project, model.View) {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 100, 100
	for _, id := range []string{"s1", "s2", "s3"} {
		p.Spreads = append(p.Spreads, &model.Spread{ID: id})
	}
	e := mustEditor(t, p, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	e.SetCoverSpineBg("#102030")
	return e.Snapshot()
}

// Задание обложки: холст — размер листа (PageW = W/2), плашка — во весь корешок; без обложки — nil.
func TestNewCoverJob(t *testing.T) {
	p, v := coverEditor(t)
	j := NewCoverJob(p, v, 20, 90)
	if j == nil || j.Opt.PageW != v.Cover.Sheet.W/2 || j.Opt.PageH != v.Cover.Sheet.H || j.Opt.DPI != 20 || j.Opt.Quality != 90 {
		t.Fatalf("задание обложки: %+v", j)
	}
	if len(j.Spread.Plates) != 1 || j.Spread.Plates[0].Color != render.ParseHex("#102030") {
		t.Fatalf("плашка корешка: %+v", j.Spread.Plates)
	}
	if w, _ := render.CanvasSize(j.Opt); w != int(math.Round(v.Cover.Sheet.W/25.4*20)) {
		t.Fatalf("холст %d px не равен листу %v мм", w, v.Cover.Sheet.W)
	}
	if NewCoverJob(p, model.View{}, 20, 90) != nil {
		t.Fatal("обложки нет, а задание есть")
	}
	v.Cover.SpineBg = ""
	if j = NewCoverJob(p, v, 20, 90); len(j.Spread.Plates) != 0 {
		t.Fatalf("плашка без цвета: %+v", j.Spread.Plates)
	}
}

// RunPlan с обложкой: 00.jpg пишется первым шагом общего прогресса, затем развороты; прежние
// наши файлы, которых новый экспорт не перезапишет, удаляются, чужие остаются.
func TestRunPlanWithCover(t *testing.T) {
	p, v := coverEditor(t)
	job := NewCoverJob(p, v, 20, 90)
	spreads := FromView(p, v)
	dir := filepath.Join(testkit.TempDir(t), "out")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ours := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	for name, data := range map[string][]byte{"05.jpg": ours, "06.jpg": []byte("чужой")} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pl := NewPlan(nil, 3, false).WithCover(job)
	hit := []DirFile{{Name: "01.jpg", Spread: 1}, {Name: "05.JPG", Spread: 5}, {Name: "05.jpg", Spread: 5}, {Name: "06.jpg", Spread: 6}}
	stale := StaleNames(hit, pl.Names(3))
	if !slices.Equal(stale, []string{"05.JPG", "05.jpg", "06.jpg"}) {
		t.Fatalf("устаревшие %v", stale)
	}
	if got := StaleNames(hit, []string{"05.JPG"}); slices.Contains(got, "05.jpg") {
		t.Fatalf("регистр не учтён: %v", got)
	}
	var done, totals []int
	err := RunPlan(context.Background(), spreads, pl, render.Options{PageW: 100, PageH: 100, DPI: 20, Quality: 90}, dir, stale,
		func(d, tot int) { done, totals = append(done, d), append(totals, tot) })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []int{1, 2, 3, 4}) || totals[0] != 4 || totals[3] != 4 {
		t.Fatalf("прогресс %v из %v", done, totals)
	}
	cfg := jpegSize(t, filepath.Join(dir, CoverName))
	if w, h := render.CanvasSize(job.Opt); cfg.Width != w || cfg.Height != h {
		t.Fatalf("00.jpg %dx%d, ждали %dx%d", cfg.Width, cfg.Height, w, h)
	}
	if !IsOurs(filepath.Join(dir, CoverName)) {
		t.Fatal("у обложки нет метки программы")
	}
	if _, err := os.Stat(filepath.Join(dir, "05.jpg")); err == nil {
		t.Error("прежний наш 05.jpg не удалён")
	}
	if _, err := os.Stat(filepath.Join(dir, "06.jpg")); err != nil {
		t.Error("чужой 06.jpg удалён")
	}
	// Только обложка: развороты не пишутся, прогресс — один шаг.
	dir2 := filepath.Join(testkit.TempDir(t), "only")
	done = nil
	if err := runCoverPlan(context.Background(), nil, Plan{}.WithCover(job), render.Options{}, dir2, func(d, _ int) { done = append(done, d) }); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(done, []int{1}) {
		t.Fatalf("прогресс только обложки %v", done)
	}
}

// Отказы записи обложки не оставляют молчаливой пустоты: ошибка с префиксом «обложка», до записи
// — без папки; отмена — до обложки.
func TestRunCoverErrors(t *testing.T) {
	p, v := coverEditor(t)
	job := NewCoverJob(p, v, 20, 90)
	base := testkit.TempDir(t)

	huge := *job
	huge.Opt.PageW = 1e6
	dir := filepath.Join(base, "huge")
	if err := runCover(&huge, dir); err == nil || !strings.HasPrefix(err.Error(), "обложка: ") {
		t.Fatalf("огромный холст: %v", err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Error("папка создана до проверки холста")
	}

	file := filepath.Join(base, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCover(job, filepath.Join(file, "sub")); err == nil {
		t.Fatal("папка внутри файла принята")
	}

	cell := layout.Rect{X: 5, Y: 5, W: 20, H: 20}
	missing := *job
	missing.Spread = render.Spread{Photos: []render.Photo{{Cell: cell, Img: cell, Path: filepath.Join(base, "нет.jpg")}}}
	if err := runCover(&missing, filepath.Join(base, "m")); err == nil || !strings.HasPrefix(err.Error(), "обложка: ") {
		t.Fatalf("нет фото: %v", err)
	}

	busy := filepath.Join(base, "busy")
	if err := os.MkdirAll(filepath.Join(busy, CoverName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runCover(job, busy); err == nil || !strings.HasPrefix(err.Error(), "обложка: ") {
		t.Fatalf("00.jpg — папка: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := filepath.Join(base, "cancel")
	err := runCoverPlan(ctx, nil, Plan{}.WithCover(job), render.Options{}, out, func(int, int) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("отмена: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, CoverName)); err == nil {
		t.Error("после отмены обложка записана")
	}
}

// Задания разворотов: отмена и номер вне альбома — отказ до записи файла.
func TestRunJobsGuards(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "out")
	opt := render.Options{PageW: 50, PageH: 40, DPI: 20, Quality: 90}
	spreads := []render.Spread{{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunJobs(ctx, spreads, []Job{{Index: 0, Whole: true}}, opt, dir, func(int, int) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("отмена: %v", err)
	}
	if err := RunJobs(context.Background(), spreads, []Job{{Index: 3, Whole: true}}, opt, dir, func(int, int) {}); err == nil ||
		!strings.Contains(err.Error(), "разворот 4: нет в альбоме") {
		t.Fatalf("номер вне альбома: %v", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("отказ оставил файлы: %v", entries)
	}
}
