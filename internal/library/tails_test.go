package library

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/disintegration/imaging"
)

// tailCache — кэш со своими местами и счётчиком декодов оригинала.
func tailCache(t *testing.T, dir string, total, bgMax int) (*Cache, *atomic.Int32) {
	t.Helper()
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, c.Close)
	c.sched = newSched(total, bgMax)
	var n atomic.Int32
	c.open = func(p string) (image.Image, func(), error) {
		n.Add(1)
		return imgfile.OpenOriented(p)
	}
	return c, &n
}

// holdNow занимает место окна: пока оно занято, горутина хвостов (класс прогрева) стоит.
func holdNow(t *testing.T, c *Cache) func() {
	t.Helper()
	rel, err := c.sched.acquire(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func keyOf(t *testing.T, p Photo) string {
	t.Helper()
	k, err := cacheKey(p.Path, p.Color)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// noLeak ждёт, пока число горутин вернётся к base (проверка утечек без goleak).
func noLeak(t *testing.T, base int) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(3 * time.Second))
	for runtime.NumGoroutine() > base {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			t.Fatalf("горутин %d, было %d:\n%s", runtime.NumGoroutine(), base, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitParked ждёт, пока n запросов встанут в get на ожидание чужой работы (генерации или хвоста):
// состояние читается из стеков горутин — у ожидания своего счётчика нет.
func waitParked(t *testing.T, n int) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(5 * time.Second))
	buf := make([]byte, 1<<20)
	for {
		got := 0
		for g := range strings.SplitSeq(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
			if strings.Contains(g, "library.wait(") && strings.Contains(g, "library.(*Cache).get(") && !strings.Contains(g, "takeTail(") {
				got++
			}
		}
		if got >= n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("ждут чужую работу %d запросов, ждали %d", got, n)
		}
		time.Sleep(time.Millisecond)
	}
}

// oldGenerate — путь до 090 как есть: эталон пикселей миниатюры и превью.
func oldGenerate(src, thumbBase, prevBase string) error {
	img, release, err := imgfile.OpenOriented(src)
	if err != nil {
		return err
	}
	defer release()
	prev := fit(img, PreviewSize, imaging.Box)
	thumb := fit(prev, ThumbSize, imaging.Linear)
	if !isOpaque(img) {
		if err := savePNG(prev, prevBase+".png"); err != nil {
			return err
		}
		return savePNG(thumb, thumbBase+".png")
	}
	if err := saveJPEG(prev, prevBase+".jpg", 88); err != nil {
		return err
	}
	return saveJPEG(thumb, thumbBase+".jpg", 82)
}

// Миниатюра сразу, превью из хвоста — побайтно как у прежнего пути, имена файлов прежние (прогретый кэш
// пользователя подходит), подпись та же; оригинал читается один раз.
func TestThumbFirstSamePixels(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 3, 1)
	photos := []Photo{
		{Path: testkit.JPEG(t, dir, "big.jpg", 2600, 1800), Name: "big.jpg"},
		{Path: testkit.JPEGOriented(t, dir, "rot.jpg", 900, 600, 6), Name: "rot.jpg"},
		{Path: testkit.PNG(t, dir, "logo.png", 60, 40, color.NRGBA{200, 30, 30, 255}), Name: "logo.png"},
	}
	ref := filepath.Join(dir, "ref")
	if err := os.MkdirAll(ref, 0o755); err != nil {
		t.Fatal(err)
	}
	hold := holdNow(t, c)
	defer hold()
	for i, p := range photos {
		key := keyOf(t, p)
		thumb, err := c.FileCtx(context.Background(), p, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := c.find(key, true, isPNGPath(p.Path)); ok || !c.hasTail(key) {
			t.Fatalf("%s: превью не ушло в фон (есть файл %v, хвост %v)", p.Name, ok, c.hasTail(key))
		}
		prev, err := c.File(p, true)
		if err != nil {
			t.Fatal(err)
		}
		if got := n.Load(); got != int32(i+1) {
			t.Fatalf("%s: декодов %d, ждали %d — превью декодировало оригинал второй раз", p.Name, got, i+1)
		}
		rt, rp := filepath.Join(ref, key+"_t"), filepath.Join(ref, key+"_p")
		if err := oldGenerate(p.Path, rt, rp); err != nil {
			t.Fatal(err)
		}
		ext := filepath.Ext(thumb)
		if thumb != c.base(key, false)+ext || prev != c.base(key, true)+ext {
			t.Errorf("%s: имена %s, %s — не прежние", p.Name, thumb, prev)
		}
		if !bytes.Equal(readFile(t, thumb), readFile(t, rt+ext)) || !bytes.Equal(readFile(t, prev), readFile(t, rp+ext)) {
			t.Errorf("%s: копии отличаются от прежнего пути", p.Name)
		}
		img, err := decodeThumb(rt + ext)
		if err != nil {
			t.Fatal(err)
		}
		if s, err := c.Sig(p); err != nil || s != imgsig.Of(img) {
			t.Errorf("%s: подпись не та (%v)", p.Name, err)
		}
	}
}

// /media/p во время фона — один декод: и когда хвост ждёт в очереди, и когда идёт сама генерация.
func TestPreviewTailNoSecondDecode(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 4, 2)
	entered, gate := make(chan struct{}, 1), make(chan struct{})
	c.open = func(p string) (image.Image, func(), error) {
		n.Add(1)
		entered <- struct{}{}
		<-gate
		return imgfile.OpenOriented(p)
	}
	p := Photo{Path: testkit.JPEG(t, dir, "a.jpg", 2200, 1000), Name: "a.jpg"}
	hold := holdNow(t, c) // хвост не уходит горутине: запрос превью забирает его сам
	defer hold()
	var wg sync.WaitGroup
	var thumb, prev string
	var errT, errP error
	wg.Go(func() { thumb, errT = c.FileCtx(context.Background(), p, false) })
	<-entered
	wg.Go(func() { prev, errP = c.FileCtx(context.Background(), p, true) })
	waitParked(t, 1) // запрос превью встал на генерацию миниатюры
	close(gate)
	wg.Wait()
	if errT != nil || errP != nil || !exists(thumb) || !exists(prev) {
		t.Fatalf("миниатюра %q (%v), превью %q (%v)", thumb, errT, prev, errP)
	}
	if got := n.Load(); got != 1 {
		t.Errorf("декодов %d, ждали 1", got)
	}
	if c.hasTail(keyOf(t, p)) {
		t.Error("хвост остался после записи превью")
	}
}

// Миниатюра есть, превью нет (хвост выброшен при выходе) — прогрев доделывает превью; второй прогрев
// ничего не декодирует.
func TestWarmMakesMissingPreview(t *testing.T) {
	dir := testkit.TempDir(t)
	c1, _ := tailCache(t, dir, 2, 1)
	photos := []Photo{
		{Path: testkit.JPEG(t, dir, "a.jpg", 2100, 900), Name: "a.jpg"},
		{Path: testkit.JPEG(t, dir, "b.jpg", 300, 200), Name: "b.jpg"},
	}
	hold := holdNow(t, c1)
	thumbs := make([][]byte, len(photos))
	for i, p := range photos {
		f, err := c1.FileCtx(context.Background(), p, false)
		if err != nil {
			t.Fatal(err)
		}
		thumbs[i] = readFile(t, f)
	}
	c1.Close()
	hold()
	for _, p := range photos {
		if _, ok := c1.find(keyOf(t, p), true, false); ok {
			t.Fatalf("%s: превью записано, хотя хвост выброшен", p.Name)
		}
	}

	c2, n := tailCache(t, dir, 2, 1)
	var last atomic.Value
	for range 2 {
		c2.Warm(context.Background(), photos, func(d, total int) { last.Store(fmt.Sprint(d, "/", total)) })
	}
	if got := last.Load(); got != "2/2" {
		t.Errorf("ход прогрева %v, ждали 2/2", got)
	}
	if got := n.Load(); got != int32(len(photos)) {
		t.Errorf("декодов %d, ждали %d (второй прогрев — без декода)", got, len(photos))
	}
	for i, p := range photos {
		key := keyOf(t, p)
		if _, ok := c2.find(key, true, false); !ok {
			t.Errorf("%s: прогрев не доделал превью", p.Name)
		}
		if f, _ := c2.find(key, false, false); !bytes.Equal(readFile(t, f), thumbs[i]) {
			t.Errorf("%s: миниатюра изменилась", p.Name)
		}
	}
}

// Выход: не начатые хвосты выбрасываются сразу (Close не ждёт места), недописанных файлов нет, горутин
// не остаётся; ждавший хвост делает превью полным путём.
func TestCloseDropsPendingTails(t *testing.T) {
	base := runtime.NumGoroutine()
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 3, 1)
	var photos []Photo
	for i := range 3 {
		name := fmt.Sprintf("p%d.jpg", i)
		photos = append(photos, Photo{Path: testkit.JPEG(t, dir, name, 2100, 1400), Name: name})
	}
	hold := holdNow(t, c)
	for _, p := range photos {
		if _, err := c.FileCtx(context.Background(), p, false); err != nil {
			t.Fatal(err)
		}
	}
	waiter := make(chan error, 1)
	go func() { _, err := c.get(context.Background(), photos[0], true, bg); waiter <- err }()
	waitFor(t, c.sched, "прогрев ждёт место", func() bool { return len(c.sched.bgWait) == 2 }) // + горутина хвостов
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(testkit.Limit(2 * time.Second)):
		t.Fatal("Close ждёт очередь превью")
	}
	c.mu.Lock()
	left := len(c.tails) + len(c.queue)
	c.mu.Unlock()
	if left != 0 {
		t.Errorf("хвостов осталось %d", left)
	}
	hold()
	if err := <-waiter; err != nil {
		t.Fatalf("ждавший превью: %v", err)
	}
	if got := n.Load(); got != 4 {
		t.Errorf("декодов %d, ждали 4 (3 миниатюры + полный путь ждавшего)", got)
	}
	_ = filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".tmp") {
			t.Errorf("недописанный файл %s", p)
		}
		return nil
	})
	for i, p := range photos {
		_, ok := c.find(keyOf(t, p), true, false)
		if ok != (i == 0) {
			t.Errorf("%s: превью есть = %v", p.Name, ok)
		}
	}
	noLeak(t, base)
}

// Очередь хвостов полна — превью кодируется до ответа, как раньше (память ограничена).
func TestTailQueueFullInline(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 3, 1)
	c.tailMax = 1
	hold := holdNow(t, c)
	defer hold()
	for i, name := range []string{"a.jpg", "b.jpg"} {
		p := Photo{Path: testkit.JPEG(t, dir, name, 2100, 900), Name: name}
		if _, err := c.FileCtx(context.Background(), p, false); err != nil {
			t.Fatal(err)
		}
		_, ok := c.find(keyOf(t, p), true, false)
		if queued := i == 0; ok == queued || c.hasTail(keyOf(t, p)) != queued {
			t.Errorf("%s: превью есть %v, хвост %v", name, ok, c.hasTail(keyOf(t, p)))
		}
	}
	if n.Load() != 2 {
		t.Errorf("декодов %d", n.Load())
	}
}

// Снятый запрос окна: ни места, ни декода, ни файла; снятый, пока ждёт чужую генерацию, — выходит
// сразу, а генерация доделывается.
func TestSchedCancelCache(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 1, 1)
	p := Photo{Path: testkit.JPEG(t, dir, "a.jpg", 300, 200), Name: "a.jpg"}
	hold := holdNow(t, c)
	ctx, cancel := context.WithCancel(context.Background())
	res := make(chan error, 1)
	go func() { _, err := c.FileCtx(ctx, p, false); res <- err }()
	waitFor(t, c.sched, "запрос ждёт место", func() bool { return len(c.sched.nowWait) == 1 })
	cancel()
	if err := <-res; err != context.Canceled {
		t.Fatalf("снятый запрос: %v", err)
	}
	hold()
	if n.Load() != 0 || c.sched.run != [2]int{} {
		t.Fatalf("снятый запрос: декодов %d, занято %v", n.Load(), c.sched.run)
	}
	if _, ok := c.find(keyOf(t, p), false, false); ok {
		t.Fatal("снятый запрос сделал миниатюру")
	}

	c.sched = newSched(2, 1)
	entered, gate := make(chan struct{}, 1), make(chan struct{})
	c.open = func(src string) (image.Image, func(), error) {
		n.Add(1)
		entered <- struct{}{}
		<-gate
		return imgfile.OpenOriented(src)
	}
	first := make(chan error, 1)
	go func() { _, err := c.File(p, false); first <- err }()
	<-entered
	ctx2, cancel2 := context.WithCancel(context.Background())
	go func() { _, err := c.FileCtx(ctx2, p, false); res <- err }()
	waitParked(t, 1) // встал на чужую генерацию, а не снят до неё
	cancel2()
	if err := <-res; err != context.Canceled {
		t.Fatalf("снятый на чужой генерации: %v", err)
	}
	close(gate)
	if err := <-first; err != nil || n.Load() != 1 {
		t.Fatalf("начатая генерация: %v, декодов %d", err, n.Load())
	}
}

// Паника декодера под местом: место и запись inflight освобождены, следующий запрос идёт.
func TestSchedPanicCache(t *testing.T) {
	dir := testkit.TempDir(t)
	c, _ := tailCache(t, dir, 1, 1)
	p := Photo{Path: testkit.JPEG(t, dir, "a.jpg", 300, 200), Name: "a.jpg"}
	open := c.open
	c.open = func(string) (image.Image, func(), error) { panic("декодер") }
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("паники не было")
			}
		}()
		_, _ = c.File(p, false)
	}()
	c.open = open
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(2*time.Second))
	defer cancel()
	if _, err := c.FileCtx(ctx, p, false); err != nil {
		t.Fatalf("после паники: %v", err)
	}
	if len(c.inflight) != 0 {
		t.Errorf("inflight: %v", c.inflight)
	}
}

// Всплеск: окно, холст и прогрев на одних фото разом — каждый оригинал читается один раз, все копии
// на месте, хвостов и горутин не остаётся.
func TestTailBurstOneDecodeEach(t *testing.T) {
	base := runtime.NumGoroutine()
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 4, 2)
	c.tailMax = 3
	var photos []Photo
	for i := range testkit.Steps(16, 8) {
		name := fmt.Sprintf("p%02d.jpg", i)
		photos = append(photos, Photo{Path: testkit.JPEG(t, dir, name, 320, 200+i), Name: name})
	}
	var wg sync.WaitGroup
	wg.Go(func() { c.Warm(context.Background(), photos, func(int, int) {}) })
	for i, p := range photos {
		wg.Go(func() {
			if _, err := c.FileCtx(context.Background(), p, false); err != nil {
				t.Error(err)
			}
		})
		if i%3 == 0 {
			wg.Go(func() {
				if _, err := c.FileCtx(context.Background(), p, true); err != nil {
					t.Error(err)
				}
			})
		}
	}
	wg.Wait()
	if got := n.Load(); got != int32(len(photos)) {
		t.Errorf("декодов %d на %d фото", got, len(photos))
	}
	for _, p := range photos {
		key := keyOf(t, p)
		_, okT := c.find(key, false, false)
		_, okP := c.find(key, true, false)
		if !okT || !okP {
			t.Errorf("%s: миниатюра %v, превью %v", p.Name, okT, okP)
		}
	}
	c.Close()
	if len(c.tails)+len(c.queue)+len(c.inflight) != 0 || c.sched.run != [2]int{} {
		t.Errorf("остаток: хвосты %d/%d, inflight %d, места %v", len(c.tails), len(c.queue), len(c.inflight), c.sched.run)
	}
	noLeak(t, base)
}

// Миниатюру убрал Trim, а хвост превью этого фото ещё в очереди: запрос миниатюры ждёт хвост, не
// занимая места (раньше — холостой круг acquire → retry), потом делает миниатюру заново.
func TestThumbWaitsQueuedTail(t *testing.T) {
	base := runtime.NumGoroutine()
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 3, 1)
	c.tailMax = 4
	hold := holdNow(t, c) // горутина хвостов стоит: хвосты остаются в очереди
	var photos []Photo
	for _, name := range []string{"a.jpg", "b.jpg"} {
		p := Photo{Path: testkit.JPEG(t, dir, name, 2100, 900), Name: name}
		photos = append(photos, p)
		f, err := c.FileCtx(context.Background(), p, false)
		if err != nil {
			t.Fatal(err)
		}
		if !c.hasTail(keyOf(t, p)) {
			t.Fatalf("%s: хвоста нет", name)
		}
		if err := os.Remove(f); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(10*time.Second))
	defer cancel()
	res := make(chan error, len(photos))
	for _, p := range photos {
		go func() {
			f, err := c.FileCtx(ctx, p, false)
			if err == nil && !exists(f) {
				err = fmt.Errorf("%s: файла %s нет", p.Name, f)
			}
			res <- err
		}()
	}
	waitParked(t, len(photos))
	c.sched.mu.Lock()
	run := c.sched.run
	c.sched.mu.Unlock()
	if run != [2]int{1, 0} {
		t.Errorf("пока ждут хвост, заняты места %v — ждали только удержанное", run)
	}
	hold()
	for range photos {
		if err := <-res; err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range photos {
		if _, ok := c.find(keyOf(t, p), true, false); !ok {
			t.Errorf("%s: превью нет", p.Name)
		}
	}
	if got := n.Load(); got != 4 {
		t.Errorf("декодов %d, ждали 4 (2 первых + 2 миниатюры заново)", got)
	}
	c.Close()
	noLeak(t, base)
}
