package export

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cancelWhenWaiting отменяет контекст, когда кто-то уже встал в ожидание общего бюджета
// декодов (детерминированно, без таймера); не дождавшись за Limit — тоже отменяет, тест
// тогда упадёт на других проверках.
func cancelWhenWaiting(t *testing.T, cancel context.CancelFunc) {
	t.Helper()
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		deadline := time.Now().Add(testkit.Limit(10 * time.Second))
		for imgfile.DecodeBudget.Waiting() < 1 && time.Now().Before(deadline) {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
			}
		}
		cancel()
	}()
}

// fileCtx — контекст, который «отменяется» в момент появления файла path: так отмена приходит
// ровно после записи этого файла, без таймеров и гонок.
type fileCtx struct {
	context.Context
	path string
}

func (c fileCtx) Err() error {
	if _, err := os.Stat(c.path); err == nil {
		return context.Canceled
	}
	return nil
}

// Отмена между частями задания «целиком + левая/правая»: NN.jpg уже записан, а NN-1/NN-2 нет —
// недописанного листа в папке не остаётся (080-11); готовые листы целы.
func TestRunJobsCancelBetweenParts(t *testing.T) {
	dir := testkit.TempDir(t)
	photo := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	sp := render.Spread{Photos: []render.Photo{{
		Cell: layout.Rect{X: 10, Y: 10, W: 45, H: 30},
		Img:  layout.Rect{X: 10, Y: 10, W: 45, H: 30},
		Path: photo,
	}}}
	spreads := []render.Spread{sp, sp}
	jobs := []Job{{Index: 0, Whole: true, Left: true, Right: true}, {Index: 1, Whole: true, Left: true, Right: true}}
	opt := render.Options{PageW: 100, PageH: 60, DPI: 100, Quality: 90}
	out := filepath.Join(dir, "out")

	var done []int
	ctx := fileCtx{Context: context.Background(), path: filepath.Join(out, "02.jpg")}
	err := RunJobs(ctx, spreads, jobs, opt, out, func(d, _ int) { done = append(done, d) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if len(done) != 1 {
		t.Errorf("прогресс %v: готов только первый лист", done)
	}
	got := map[string]bool{}
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		got[e.Name()] = true
	}
	want := map[string]bool{"01.jpg": true, "01-1.jpg": true, "01-2.jpg": true}
	if len(got) != len(want) {
		t.Errorf("в папке %v, ждали ровно %v", got, want)
	}
	for n := range want {
		if !got[n] {
			t.Errorf("нет готового %s", n)
		}
	}
}

// «Остановить» во время кодирования последнего листа: RenderCtx уже вернулся, но прежние файлы
// (stale) не удаляются, ответ — отмена, а не «готово». Без отмены те же stale удаляются.
func TestRunPlanCancelAfterLastKeepsStale(t *testing.T) {
	dir := testkit.TempDir(t)
	opt := render.Options{PageW: 100, PageH: 60, DPI: 100, Quality: 90}
	spreads := []render.Spread{{}}
	run := func(name string, ctx context.Context) (string, error) {
		out := filepath.Join(dir, name)
		if err := os.MkdirAll(out, 0o755); err != nil {
			t.Fatal(err)
		}
		old := filepath.Join(out, "spread_05.jpg")
		if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := RunPlan(ctx, spreads, NewPlan(nil, 1, false), opt, out, []string{"spread_05.jpg"}, func(int, int) {})
		return old, err
	}
	old, err := run("cancel", fileCtx{Context: context.Background(), path: filepath.Join(dir, "cancel", "01.jpg")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Errorf("прежний файл удалён при отмене: %v", err)
	}
	old, err = run("ok", context.Background())
	if err != nil {
		t.Fatalf("без отмены: %v", err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("без отмены прежний файл должен быть удалён")
	}
}

// «Остановить», пока лист ждёт памяти под большое фото (бюджет занят прогревом миниатюр):
// RunJobs возвращает отмену сразу, а не после листа; готовый лист остаётся, недописанного
// файла нет (080-8). То же у обложки (RunPlan).
func TestExportStopWhileWaitingBudget(t *testing.T) {
	dir := testkit.TempDir(t)
	photo := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	withPhoto := render.Spread{Photos: []render.Photo{{
		Cell: layout.Rect{X: 10, Y: 10, W: 45, H: 30},
		Img:  layout.Rect{X: 10, Y: 10, W: 45, H: 30},
		Path: photo,
	}}}
	spreads := []render.Spread{{}, withPhoto, withPhoto}
	jobs := []Job{{Index: 0, Whole: true}, {Index: 1, Whole: true}, {Index: 2, Left: true, Right: true}}
	opt := render.Options{PageW: 100, PageH: 60, DPI: 100, Quality: 90}
	out := filepath.Join(dir, "out")

	imgfile.DecodeBudget.Acquire(imgfile.DecodeBudgetPixels)
	held := true
	defer func() {
		if held {
			imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var done []int
	cancelWhenWaiting(t, cancel)
	start := time.Now()
	err := RunJobs(ctx, spreads, jobs, opt, out, func(d, _ int) { done = append(done, d) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if d := time.Since(start); d > testkit.Limit(500*time.Millisecond) {
		t.Errorf("остановка заметна через %v", d)
	}
	if len(done) != 1 {
		t.Errorf("прогресс %v: готов только первый лист", done)
	}
	if _, err := os.Stat(filepath.Join(out, "01.jpg")); err != nil {
		t.Errorf("готовый лист пропал: %v", err)
	}
	for _, n := range []string{"02.jpg", "03-1.jpg", "03-2.jpg"} {
		if _, err := os.Stat(filepath.Join(out, n)); err == nil {
			t.Errorf("недописанный %s остался", n)
		}
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 1 {
		t.Errorf("в папке %d файлов, ждали один (без .tmp)", len(entries))
	}

	// Обложка с фото: тоже останавливается в ожидании; 00.jpg нет.
	cov := &CoverJob{Spread: withPhoto, Opt: opt}
	out2 := filepath.Join(dir, "out2")
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	cancelWhenWaiting(t, cancel2)
	err = RunPlan(ctx2, spreads, NewPlan(nil, 3, false).WithCover(cov), opt, out2, nil, func(int, int) {})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("обложка: ждали Canceled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out2, CoverName)); err == nil {
		t.Error("недописанная обложка осталась")
	}

	// После освобождения бюджета тот же план проходит целиком.
	imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
	held = false
	out3 := filepath.Join(dir, "out3")
	if err := RunJobs(context.Background(), spreads, jobs, opt, out3, func(int, int) {}); err != nil {
		t.Fatalf("после освобождения: %v", err)
	}
	for _, n := range []string{"01.jpg", "02.jpg", "03-1.jpg", "03-2.jpg"} {
		if _, err := os.Stat(filepath.Join(out3, n)); err != nil {
			t.Errorf("нет %s: %v", n, err)
		}
	}
	if imgfile.DecodeBudget.Used() != 0 {
		t.Errorf("бюджет не пуст: %d", imgfile.DecodeBudget.Used())
	}
}
