//go:build ignore

// e2e-photos делает набор для e2e (089): 600 разных JPEG по 24 Мп сериями по 6 из 100 фото stock,
// с EXIF-датой, 3 % Adobe RGB и 3 % поворота EXIF. Папка a/ — кадры 1–120 (быстрый прогон), b/ — остальные.
//
//	go run scripts/e2e-photos.go -src .testphotos/stock -out .testphotos-e2e
//
// План кадров — testkit.BigSetPlan (чистая функция номера и зерна), рисует и кодирует testkit.BigSetRender
// и BigSetEncode: повторный запуск с тем же зерном даёт те же байты. Существующие файлы не трогаются
// (без -force), так что после сбоя запуск дописывает недостающее. Набор ≈ 3–5 ГБ, ≈ 3–5 минут;
// папка .testphotos-e2e/ вне git.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // декодер исходников
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const quickCount = 120 // кадры 1..120 лежат в a/

func main() {
	src := flag.String("src", ".testphotos/stock", "папка с исходниками (stock, по порядку имён)")
	out := flag.String("out", ".testphotos-e2e", "куда писать набор (подпапки a/ и b/)")
	count := flag.Int("count", 600, "сколько кадров")
	seed := flag.Int64("seed", 1, "зерно плана")
	force := flag.Bool("force", false, "перезаписывать существующие файлы")
	flag.Parse()
	if err := run(*src, *out, *count, *seed, *force); err != nil {
		fmt.Fprintln(os.Stderr, "e2e-photos:", err)
		os.Exit(1)
	}
}

// job — один кадр: номер, путь файла, исходник.
type job struct {
	n    int
	path string
}

func run(src, out string, count int, seed int64, force bool) error {
	if strings.TrimSpace(src) == "" || strings.TrimSpace(out) == "" {
		return fmt.Errorf("-src и -out не могут быть пустыми")
	}
	if count < 1 || count > 9999 {
		return fmt.Errorf("-count 1..9999, получено %d", count)
	}
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	outAbs, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if filepath.Dir(outAbs) == outAbs {
		return fmt.Errorf("-out не может быть корнем диска: %s", outAbs)
	}
	srcs, err := listSources(srcAbs)
	if err != nil {
		return err
	}
	jobs, err := planJobs(outAbs, count, force)
	if err != nil {
		return err
	}
	t0 := time.Now()
	fmt.Printf("исходников %d, кадров %d, к созданию %d, зерно %d\n", len(srcs), count, len(jobs), seed)
	if err := render(jobs, srcs, seed); err != nil {
		return err
	}
	if err := writeManifest(outAbs, seed); err != nil {
		return err
	}
	fmt.Printf("готово за %v\n", time.Since(t0).Round(time.Second))
	return nil
}

// listSources — файлы .jpg/.jpeg папки src по имени.
func listSources(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !e.IsDir() && (ext == ".jpg" || ext == ".jpeg") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("в %s нет JPEG (ждём .testphotos/stock, docs/QA.md «Подготовка»)", dir)
	}
	sort.Strings(out)
	return out, nil
}

// planJobs создаёт папки a/ и b/ и выбирает кадры, которых ещё нет (все — при -force).
func planJobs(out string, count int, force bool) ([]job, error) {
	var jobs []job
	for n := 1; n <= count; n++ {
		sub := "a"
		if n > quickCount {
			sub = "b"
		}
		dir := filepath.Join(out, sub)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, fmt.Sprintf("e2e-%03d.jpg", n))
		if _, err := os.Stat(path); err == nil && !force {
			continue
		}
		jobs = append(jobs, job{n: n, path: path})
	}
	return jobs, nil
}

// render делает кадры в NumCPU/2 потоков (кадр 24 Мп — 100 МБ в памяти, больше потоков не нужно).
func render(jobs []job, srcs []string, seed int64) error {
	workers := max(1, runtime.NumCPU()/2)
	tile := testkit.BigSetTile(seed)
	ch := make(chan job)
	var wg sync.WaitGroup
	var done atomic.Int64
	var failed atomic.Value
	for range workers {
		wg.Go(func() {
			for j := range ch {
				if failed.Load() != nil {
					continue
				}
				if err := one(j, srcs, seed, tile); err != nil {
					failed.CompareAndSwap(nil, err)
					continue
				}
				if d := done.Add(1); d%20 == 0 || int(d) == len(jobs) {
					fmt.Printf("  %d / %d\n", d, len(jobs))
				}
			}
		})
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
	if err, _ := failed.Load().(error); err != nil {
		return err
	}
	return nil
}

// one рисует кадр и пишет его через временный файл: оборванный запуск не оставляет битых e2e-NNN.jpg.
func one(j job, srcs []string, seed int64, tile []int8) error {
	p := testkit.BigSetPlan(j.n, seed)
	img, err := loadRGBA(srcs[(p.Source-1)%len(srcs)])
	if err != nil {
		return err
	}
	data, err := testkit.BigSetEncode(testkit.BigSetRender(img, p, tile, 1), p)
	if err != nil {
		return fmt.Errorf("кадр %d: %w", j.n, err)
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, j.path)
}

// loadRGBA читает исходник в RGBA.
func loadRGBA(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgba, nil
}

// writeManifest пишет manifest.txt по тому, что лежит на диске: число, суммарный размер, зерно, версия плана.
func writeManifest(out string, seed int64) error {
	var n int
	var size int64
	for _, sub := range []string{"a", "b"} {
		ents, err := os.ReadDir(filepath.Join(out, sub))
		if err != nil {
			continue
		}
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), "e2e-") && strings.HasSuffix(e.Name(), ".jpg") {
				if fi, err := e.Info(); err == nil {
					n, size = n+1, size+fi.Size()
				}
			}
		}
	}
	text := fmt.Sprintf("files %d\nbytes %d\nseed %d\nplan %d\n", n, size, seed, testkit.BigSetVersion)
	return os.WriteFile(filepath.Join(out, "manifest.txt"), []byte(text), 0o644)
}
