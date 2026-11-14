package library

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// FileBgCtx встаёт в очередь прогрева: запрос окна, пришедший позже, получает место раньше; готовая
// миниатюра отдаётся без места; снятый ctx не начинает генерацию.
func TestFileBgCtxClass(t *testing.T) {
	dir := testkit.TempDir(t)
	c, _ := tailCache(t, dir, 1, 1)
	pa := Photo{Path: testkit.JPEG(t, dir, "a.jpg", 300, 200), Name: "a.jpg"}
	pb := Photo{Path: testkit.JPEG(t, dir, "b.jpg", 200, 300), Name: "b.jpg"}
	pc := Photo{Path: testkit.JPEG(t, dir, "c.jpg", 250, 250), Name: "c.jpg"}

	var mu sync.Mutex
	var order []string
	var runAtOpen [][2]int
	open := c.open
	c.open = func(src string) (image.Image, func(), error) {
		c.sched.mu.Lock()
		run := c.sched.run
		c.sched.mu.Unlock()
		mu.Lock()
		order = append(order, filepath.Base(src))
		runAtOpen = append(runAtOpen, run)
		mu.Unlock()
		return open(src)
	}

	// Снятый ctx: места нет, декода нет.
	dead, kill := context.WithCancel(context.Background())
	kill()
	if _, err := c.FileBgCtx(dead, pa); err != context.Canceled {
		t.Fatalf("снятый ctx: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("снятый ctx начал генерацию: %v", order)
	}

	// Единственное место занято: фоновый запрос (раньше) и запрос окна (позже) ждут.
	hold, err := c.sched.acquire(context.Background(), bg)
	if err != nil {
		t.Fatal(err)
	}
	type res struct {
		name, path string
		err        error
	}
	out := make(chan res, 2)
	go func() { f, err := c.FileBgCtx(context.Background(), pa); out <- res{"bg", f, err} }()
	waitFor(t, c.sched, "фоновый запрос ждёт", func() bool { return len(c.sched.bgWait) == 1 })
	go func() { f, err := c.FileCtx(context.Background(), pb, false); out <- res{"now", f, err} }()
	waitFor(t, c.sched, "запрос окна ждёт", func() bool { return len(c.sched.nowWait) == 1 })
	hold()
	var bgPath string
	for range 2 {
		select {
		case r := <-out:
			if r.err != nil {
				t.Fatalf("%s: %v", r.name, r.err)
			}
			if _, err := os.Stat(r.path); err != nil {
				t.Fatalf("%s: файла нет: %v", r.name, err)
			}
			if r.name == "bg" {
				bgPath = r.path
			}
		case <-time.After(testkit.Limit(5 * time.Second)):
			t.Fatal("запросы не завершились")
		}
	}
	mu.Lock()
	if len(order) != 2 || order[0] != "b.jpg" || order[1] != "a.jpg" {
		t.Fatalf("порядок генерации %v, ждём b.jpg (окно) раньше a.jpg (фон)", order)
	}
	if runAtOpen[1] != [2]int{0, 1} {
		t.Fatalf("фоновый запрос идёт не классом bg: занято %v", runAtOpen[1])
	}
	mu.Unlock()

	// Готовая миниатюра — без места и без декода, даже если все места заняты окном.
	hold2, err := c.sched.acquire(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	defer hold2()
	if f, err := c.FileBgCtx(context.Background(), pa); err != nil || f != bgPath {
		t.Fatalf("готовая миниатюра: %q %v", f, err)
	}

	// Занято окном: холодная клетка не успевает к сроку и уходит с ctx.Err() без обёртки.
	short, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.FileBgCtx(short, pc); err != context.DeadlineExceeded {
		t.Fatalf("срок при занятых местах: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("декоды после готовой и срока: %v", order)
	}
}
