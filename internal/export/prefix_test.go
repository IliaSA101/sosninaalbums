package export

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Префикс имён (081 №9, Фабрика Фотокниги: 1-00.jpg, 1-01.jpg…): имена плана, обложка в любом
// порядке WithCover/WithPrefix, вопрос о замене, отбор задетых и устаревших.
func TestPlanPrefix(t *testing.T) {
	job := &CoverJob{}
	want := []string{"1-00.jpg", "1-01.jpg", "1-02.jpg", "1-03.jpg"}
	for name, pl := range map[string]Plan{
		"обложка, затем префикс": NewPlan(nil, 3, false).WithCover(job).WithPrefix("1-"),
		"префикс, затем обложка": NewPlan(nil, 3, false).WithPrefix("1-").WithCover(job),
	} {
		if got := pl.Names(3); !slices.Equal(got, want) || pl.Cover.Name != "1-00.jpg" || pl.CoverName() != "1-00.jpg" {
			t.Errorf("%s: %v, обложка %q", name, got, pl.Cover.Name)
		}
	}
	if job.Name != "" {
		t.Fatalf("WithCover изменил чужое задание: %q", job.Name)
	}
	// Без префикса — прежние имена; префикс снимается тем же WithPrefix.
	pl := NewPlan(nil, 3, false).WithCover(job).WithPrefix("1-")
	if got := pl.WithPrefix("").Names(3); !slices.Equal(got, []string{"00.jpg", "01.jpg", "02.jpg", "03.jpg"}) {
		t.Fatalf("без префикса: %v", got)
	}
	if pl.Jobs[0].Prefix != "1-" || NewPlan(nil, 3, false).Jobs[0].Prefix != "" {
		t.Fatal("префикс не у заданий или задел чужой план")
	}
	if got := ReplaceQuestion(pl, []DirFile{{Name: "1-00.jpg"}}); got != "В папке уже есть обложка прежнего экспорта (1-00.jpg). Заменить?" {
		t.Errorf("вопрос об обложке: %q", got)
	}
	hit := HitFiles(pl, []DirFile{{Name: "1-00.jpg"}, {Name: "1-02.jpg", Spread: 2}, {Name: "1-07.jpg", Spread: 7}})
	if got := StaleNames(hit, pl.Names(3)); !slices.Equal(got, []string{"1-07.jpg"}) || len(hit) != 3 {
		t.Errorf("задетые %v, устаревшие %v", hit, got)
	}
	// Страницы и пустой план: префикс у каждого имени, пустого нет.
	if got := NewPlan([]Unit{{2, 1}}, 3, true).WithPrefix("1-").Names(3); !slices.Equal(got, []string{"1-02-1.jpg"}) {
		t.Errorf("страница: %v", got)
	}
	if got := (Plan{}).WithPrefix("1-").Names(0); len(got) != 0 {
		t.Errorf("пустой план: %v", got)
	}
}

// PlanDirPrefix: при «1-» свои — 1-NN.jpg с меткой, 1-00.jpg не отдаётся (обложка — AddCoverFile),
// NN.jpg и spread_NN.jpg обычного экспорта не трогаются; без префикса — наоборот, как раньше.
func TestPlanDirPrefix(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	for name, data := range map[string][]byte{
		"1-00.jpg": ourData, "1-01.jpg": ourData, "1-02.jpg": ourData, "1-05.jpg": []byte("чужой"),
		"01.jpg": ourData, "spread_03.jpg": []byte("x"), "1-5.jpg": ourData, "2-01.jpg": ourData,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ours, foreign, err := PlanDirPrefix(dir, "1-")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fileNames(ours), []string{"1-01.jpg", "1-02.jpg"}) || !slices.Equal(fileNames(foreign), []string{"1-05.jpg"}) {
		t.Fatalf("1-: свои %v, чужие %v", fileNames(ours), fileNames(foreign))
	}
	if ours[0].Spread == 0 || foreign[0].Spread != 5 {
		t.Fatalf("номера: %+v %+v", ours, foreign)
	}
	pl := NewPlan(nil, 2, false).WithCover(&CoverJob{}).WithPrefix("1-")
	if o, f := AddCoverFile(pl, dir, nil, nil); !slices.Equal(fileNames(o), []string{"1-00.jpg"}) || len(f) != 0 {
		t.Fatalf("обложка 1-00.jpg: %v %v", o, f)
	}
	ours, foreign, err = PlanDir(dir)
	if err != nil || !slices.Equal(fileNames(ours), []string{"01.jpg", "spread_03.jpg"}) || len(foreign) != 0 {
		t.Fatalf("без префикса: %v %v %v", fileNames(ours), fileNames(foreign), err)
	}
}

// RunPlan с префиксом: на диске 1-00.jpg и 1-01.jpg… с меткой, без .tmp; старый остаток записи
// 1-NN.jpg.<n>.tmp убирается, обложечный 1-00.jpg.<n>.tmp — нет.
func TestRunJobsPrefix(t *testing.T) {
	dir := testkit.TempDir(t)
	old := time.Now().Add(-2 * time.Hour)
	for _, n := range []string{"1-05.jpg.12.tmp", "1-00.jpg.3.tmp"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	opt := render.Options{PageW: 10, PageH: 10, DPI: 72, Quality: 80}
	cover := &CoverJob{Opt: opt}
	pl := NewPlan(nil, 2, false).WithCover(cover).WithPrefix("1-")
	if err := RunPlan(context.Background(), []render.Spread{{}, {}}, pl, opt, dir, nil, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"1-00.jpg", "1-00.jpg.3.tmp", "1-01.jpg", "1-02.jpg"}) {
		t.Fatalf("файлы %v", names)
	}
	for _, n := range []string{"1-00.jpg", "1-01.jpg", "1-02.jpg"} {
		if !IsOurs(filepath.Join(dir, n)) {
			t.Errorf("%s без метки", n)
		}
	}
}
