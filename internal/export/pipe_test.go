package export

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var pipeOpt = render.Options{PageW: 100, PageH: 60, DPI: 100, Quality: 90}

// pipeSpreads — n разворотов с фото (у каждого своё окно, чтобы пиксели различались).
func pipeSpreads(t *testing.T, n int) []render.Spread {
	t.Helper()
	dir := testkit.TempDir(t)
	out := make([]render.Spread, n)
	for i := range out {
		photo := testkit.JPEG(t, dir, "p"+string(rune('a'+i))+".jpg", 300+20*i, 200)
		r := layout.Rect{X: 8 + float64(i)*3, Y: 8, W: 60, H: 40}
		out[i] = render.Spread{Photos: []render.Photo{{Cell: r, Img: r, Path: photo}}}
	}
	return out
}

func readAll(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = b
	}
	return m
}

func names(m map[string][]byte) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// noLeak: к концу теста горутин не больше, чем было (запись и рендер вышли).
func noLeak(t *testing.T) {
	t.Helper()
	before := runtime.NumGoroutine()
	t.Cleanup(func() {
		deadline := time.Now().Add(testkit.Limit(2 * time.Second))
		for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if n := runtime.NumGoroutine(); n > before {
			t.Errorf("утечка горутин: было %d, стало %d", before, n)
		}
	})
}

func setPipeMax(t *testing.T, v int) {
	t.Helper()
	old := pipeMaxPixels
	pipeMaxPixels = v
	t.Cleanup(func() { pipeMaxPixels = old })
}

// Два холста и один (pipeMaxPixels = 0) дают одни и те же файлы побайтно, а те — то же, что
// рендер и запись по очереди; прогресс — 1…4 по порядку.
func TestRunJobsPipeSameBytes(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 4)
	jobs := []Job{
		{Index: 0, Whole: true, Left: true, Right: true},
		{Index: 1, Left: true, Right: true},
		{Index: 2, Whole: true},
		{Index: 3, Whole: true, Right: true, Prefix: "1-"},
	}
	run := func(name string) (map[string][]byte, []int) {
		out := filepath.Join(testkit.TempDir(t), name)
		var progress []int
		if err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(d, total int) {
			if total != len(jobs) {
				t.Errorf("total = %d", total)
			}
			progress = append(progress, d)
		}); err != nil {
			t.Fatal(err)
		}
		return readAll(t, out), progress
	}
	two, p2 := run("two")
	setPipeMax(t, 0)
	one, p1 := run("one")

	want := []string{"01-1.jpg", "01-2.jpg", "01.jpg", "02-1.jpg", "02-2.jpg", "03.jpg", "1-04-2.jpg", "1-04.jpg"}
	if got := names(two); !slices.Equal(got, want) {
		t.Fatalf("файлы %v, ждали %v", got, want)
	}
	for _, n := range want {
		if !bytes.Equal(two[n], one[n]) {
			t.Errorf("%s: байты при двух холстах и при одном различаются", n)
		}
	}
	for _, p := range [][]int{p1, p2} {
		if !slices.Equal(p, []int{1, 2, 3, 4}) {
			t.Errorf("прогресс %v", p)
		}
	}
	// Эталон: рендер и запись напрямую, без конвейера.
	img, err := render.RenderCtx(context.Background(), spreads[2], pipeOpt)
	if err != nil {
		t.Fatal(err)
	}
	ref := filepath.Join(testkit.TempDir(t), "ref.jpg")
	if err := saveJPEG(img, ref, pipeOpt); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(ref); !bytes.Equal(b, two["03.jpg"]) {
		t.Error("03.jpg отличается от записи без конвейера")
	}
}

// Рендер следующего листа идёт, пока пишется предыдущий: запись листа 1 не вернётся, пока не
// начался рендер листа 2 (последовательный код тут встал бы — тест упадёт по таймеру). При
// одном холсте — наоборот: рендер 2 не начинается до конца записи 1, и живёт ≤ 1 холста.
func TestRunJobsPipeBigCanvasOneAtATime(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 3)
	jobs := []Job{{Index: 0, Whole: true}, {Index: 1, Whole: true}, {Index: 2, Left: true}}

	var (
		mu         sync.Mutex
		live, peak int
		saves      int
		log        []string
		started    = make(chan struct{}, 8)
	)
	oldR, oldS := renderSheet, saveSheet
	t.Cleanup(func() { renderSheet, saveSheet = oldR, oldS })
	renderSheet = func(ctx context.Context, s render.Spread, opt render.Options) (*image.RGBA, error) {
		mu.Lock()
		live++
		peak = max(peak, live)
		log = append(log, "R")
		mu.Unlock()
		started <- struct{}{}
		return oldR(ctx, s, opt)
	}
	var wait bool
	saveSheet = func(img image.Image, path string, opt render.Options) error {
		mu.Lock()
		log = append(log, "S")
		saves++
		first := saves == 1
		mu.Unlock()
		if wait && first {
			// Дождаться начала рендера следующего листа (R1 и R2 уже в started).
			deadline := time.After(testkit.Limit(5 * time.Second))
			for range 2 {
				select {
				case <-started:
				case <-deadline:
					return errors.New("рендер следующего листа не начался, пока писался предыдущий")
				}
			}
		}
		err := oldS(img, path, opt)
		mu.Lock()
		live--
		log = append(log, "E")
		mu.Unlock()
		return err
	}
	run := func(name string) map[string][]byte {
		mu.Lock()
		live, peak, saves, log = 0, 0, 0, nil
		mu.Unlock()
		for len(started) > 0 {
			<-started
		}
		out := filepath.Join(testkit.TempDir(t), name)
		if err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(int, int) {}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return readAll(t, out)
	}

	wait = true
	two := run("two")
	if peak != 2 {
		t.Errorf("при двух холстах живых одновременно %d, ждали 2", peak)
	}
	wait = false
	setPipeMax(t, 0)
	one := run("one")
	if peak != 1 {
		t.Errorf("при одном холсте живых одновременно %d, ждали 1", peak)
	}
	// Каждый лист: рендер → начало записи → конец записи, строго по очереди.
	if got := strings.Join(log, ""); got != "RSERSERSE" {
		t.Errorf("порядок при одном холсте %q", got)
	}
	if !slices.Equal(names(one), names(two)) {
		t.Fatalf("файлы %v и %v", names(one), names(two))
	}
	for n, b := range two {
		if !bytes.Equal(b, one[n]) {
			t.Errorf("%s: пиксели при одном холсте другие", n)
		}
	}
}

// «Остановить» ровно после записи 03.jpg (fileCtx не доходит до rctx — ловушка конвейера):
// страницы 03 не начинаются и 03.jpg убран, готовые 01 и 02 целы, рендер 04 не зависает.
func TestRunJobsPipeCancelWhileEncoding(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 4)
	jobs := make([]Job, 4)
	for i := range jobs {
		jobs[i] = Job{Index: i, Whole: true, Left: true, Right: true}
	}
	out := filepath.Join(testkit.TempDir(t), "out")
	var progress []int
	ctx := fileCtx{Context: context.Background(), path: filepath.Join(out, "03.jpg")}
	err := RunJobs(ctx, spreads, jobs, pipeOpt, out, func(d, _ int) { progress = append(progress, d) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if !slices.Equal(progress, []int{1, 2}) {
		t.Errorf("прогресс %v", progress)
	}
	if got := names(readAll(t, out)); !slices.Equal(got, []string{"01-1.jpg", "01-2.jpg", "01.jpg", "02-1.jpg", "02-2.jpg", "02.jpg"}) {
		t.Errorf("в папке %v: ждали только 01* и 02* (без 03* и .tmp)", got)
	}
}

// Настоящая отмена контекста, пока пишется лист: ответ Canceled, рендер не висит, недописанного нет.
func TestRunJobsPipeCancelRealContext(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 4)
	jobs := make([]Job, 4)
	for i := range jobs {
		jobs[i] = Job{Index: i, Whole: true, Left: true, Right: true}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	oldS := saveSheet
	t.Cleanup(func() { saveSheet = oldS })
	var once sync.Once
	saveSheet = func(img image.Image, path string, opt render.Options) error {
		err := oldS(img, path, opt)
		if strings.HasSuffix(path, "02.jpg") {
			once.Do(cancel) // файл 02.jpg дописан, остальное 02 не начинать
		}
		return err
	}
	out := filepath.Join(testkit.TempDir(t), "out")
	var progress []int
	err := RunJobs(ctx, spreads, jobs, pipeOpt, out, func(d, _ int) { progress = append(progress, d) })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if !slices.Equal(progress, []int{1}) {
		t.Errorf("прогресс %v", progress)
	}
	if got := names(readAll(t, out)); !slices.Equal(got, []string{"01-1.jpg", "01-2.jpg", "01.jpg"}) {
		t.Errorf("в папке %v: ждали только 01*", got)
	}
}

// Запись разворота 2 падает (на месте 02-1.jpg папка): ошибка называет разворот 2, 01* целы,
// 02.jpg убран, 03 не записывается, рендер не висит.
func TestRunJobsPipeWriteError(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 4)
	jobs := make([]Job, 4)
	for i := range jobs {
		jobs[i] = Job{Index: i, Whole: true, Left: true, Right: true}
	}
	out := filepath.Join(testkit.TempDir(t), "out")
	if err := os.MkdirAll(filepath.Join(out, "02-1.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	var progress []int
	err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(d, _ int) { progress = append(progress, d) })
	if err == nil || !strings.HasPrefix(err.Error(), "разворот 2:") {
		t.Fatalf("ждали ошибку разворота 2: %v", err)
	}
	if errors.Is(err, context.Canceled) {
		t.Errorf("ошибка записи принята за отмену: %v", err)
	}
	if !slices.Equal(progress, []int{1}) {
		t.Errorf("прогресс %v", progress)
	}
	got := readAll2(t, out)
	if want := []string{"01-1.jpg", "01-2.jpg", "01.jpg", "02-1.jpg"}; !slices.Equal(got, want) {
		t.Errorf("в папке %v, ждали %v (02.jpg убран, 03* нет, без .tmp)", got, want)
	}
}

// readAll2 — имена в папке (в ней может быть подпапка, которую readAll не прочтёт как файл).
func readAll2(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// Фото разворота 3 пропало: ошибка называет разворот 3 и хранит причину, 01 и 02 записаны,
// 03 нет (даже целиком, если страницы не удались).
func TestRunJobsPipeRenderError(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 4)
	spreads[2].Photos[0].Path = filepath.Join(testkit.TempDir(t), "gone.jpg")
	jobs := make([]Job, 4)
	for i := range jobs {
		jobs[i] = Job{Index: i, Whole: true, Left: true, Right: true}
	}
	out := filepath.Join(testkit.TempDir(t), "out")
	var progress []int
	err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(d, _ int) { progress = append(progress, d) })
	if err == nil || !strings.HasPrefix(err.Error(), "разворот 3:") || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ждали ошибку разворота 3 с причиной: %v", err)
	}
	if !slices.Equal(progress, []int{1, 2}) {
		t.Errorf("прогресс %v", progress)
	}
	if got := readAll2(t, out); !slices.Equal(got, []string{"01-1.jpg", "01-2.jpg", "01.jpg", "02-1.jpg", "02-2.jpg", "02.jpg"}) {
		t.Errorf("в папке %v", got)
	}
}

// Рендер страниц падает после записи «целиком»: NN.jpg без NN-1/NN-2 не остаётся (080-11).
func TestRunJobsPipeSplitRenderErrorDropsWhole(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 2)
	oldR := renderSheet
	t.Cleanup(func() { renderSheet = oldR })
	renderSheet = func(ctx context.Context, s render.Spread, opt render.Options) (*image.RGBA, error) {
		if opt.Split {
			return nil, errors.New("рендер страниц упал")
		}
		return oldR(ctx, s, opt)
	}
	jobs := []Job{{Index: 0, Whole: true}, {Index: 1, Whole: true, Left: true}}
	out := filepath.Join(testkit.TempDir(t), "out")
	err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(int, int) {})
	if err == nil || !strings.HasPrefix(err.Error(), "разворот 2:") {
		t.Fatalf("ждали ошибку разворота 2: %v", err)
	}
	if got := readAll2(t, out); !slices.Equal(got, []string{"01.jpg"}) {
		t.Errorf("в папке %v: ждали только 01.jpg", got)
	}
}

// Края: пустой список заданий, разворот вне альбома после готового, задание без файлов.
func TestRunJobsPipeEdges(t *testing.T) {
	noLeak(t)
	spreads := pipeSpreads(t, 2)
	out := filepath.Join(testkit.TempDir(t), "out")
	if err := RunJobs(context.Background(), spreads, nil, pipeOpt, out, func(int, int) { t.Error("progress без заданий") }); err != nil {
		t.Fatal(err)
	}
	var progress []int
	err := RunJobs(context.Background(), spreads, []Job{{Index: 0, Whole: true}, {Index: 1}, {Index: 5, Whole: true}},
		pipeOpt, out, func(d, _ int) { progress = append(progress, d) })
	if err == nil || !strings.Contains(err.Error(), "разворот 6: нет в альбоме") {
		t.Fatalf("ждали «разворот 6: нет в альбоме»: %v", err)
	}
	if !slices.Equal(progress, []int{1, 2}) {
		t.Errorf("прогресс %v: задание без файлов тоже считается", progress)
	}
	if got := readAll2(t, out); !slices.Equal(got, []string{"01.jpg"}) {
		t.Errorf("в папке %v", got)
	}
}

// Паника в рендере листа, в кодере и в прогрессе — ошибка RunJobs, а не падение программы;
// горутин не остаётся, недописанное задание убрано, готовые файлы целы.
func TestRunJobsPipePanic(t *testing.T) {
	spreads := pipeSpreads(t, 3)
	jobs := []Job{{Index: 0, Whole: true}, {Index: 1, Whole: true, Left: true}, {Index: 2, Whole: true}}
	cases := []struct {
		name     string
		render   bool // паника в рендере страниц разворота 2
		save     bool // паника в кодере 02-1.jpg
		progress bool // паника в прогрессе после разворота 1
		want     string
		files    []string
	}{
		{name: "рендер", render: true, want: "разворот 2: сбой рендера: бум", files: []string{"01.jpg"}},
		{name: "запись", save: true, want: "разворот 2: сбой записи: бум", files: []string{"01.jpg"}},
		{name: "прогресс", progress: true, want: "сбой записи: бум", files: []string{"01.jpg"}},
	}
	oldR, oldS := renderSheet, saveSheet
	t.Cleanup(func() { renderSheet, saveSheet = oldR, oldS })
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			noLeak(t)
			renderSheet = func(ctx context.Context, s render.Spread, opt render.Options) (*image.RGBA, error) {
				if c.render && opt.Split {
					panic("бум")
				}
				return oldR(ctx, s, opt)
			}
			saveSheet = func(img image.Image, path string, opt render.Options) error {
				if c.save && filepath.Base(path) == "02-1.jpg" {
					panic("бум")
				}
				return oldS(img, path, opt)
			}
			out := filepath.Join(testkit.TempDir(t), "out")
			err := RunJobs(context.Background(), spreads, jobs, pipeOpt, out, func(d, _ int) {
				if c.progress {
					panic("бум")
				}
			})
			if err == nil || err.Error() != c.want {
				t.Fatalf("ошибка %v, ждали %q", err, c.want)
			}
			if got := readAll2(t, out); !slices.Equal(got, c.files) {
				t.Errorf("в папке %v, ждали %v", got, c.files)
			}
		})
	}
}
