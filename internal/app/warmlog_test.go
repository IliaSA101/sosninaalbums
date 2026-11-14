package app

// Строки журнала 089 G3: «миниатюры: N за X» и «память: <этап> — Go N МБ, куча M МБ».

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// warmLogApp — App с кэшем миниатюр и тихими событиями.
func warmLogApp(t *testing.T) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	a.emit = func(string, ...any) {}
	return a
}

// TestWarmLogsDone: чтение папки и прогрев без отмены пишут обе строки (фото 3 — число в строке,
// пути нет); отменённый и пустой прогрев строк «миниатюры»/«память: прогрев» не пишут.
func TestWarmLogsDone(t *testing.T) {
	thumbs := regexp.MustCompile(`миниатюры: 3 за \S+\n`)
	mem := func(stage string) *regexp.Regexp {
		return regexp.MustCompile(`память: ` + stage + ` — Go \d+ МБ, куча \d+ МБ\n`)
	}

	a := warmLogApp(t)
	buf := captureLog(t)
	dir := testkit.TempDir(t)
	for i := range 3 {
		testkit.JPEG(t, dir, fmt.Sprintf("секрет%d.jpg", i), 60, 40)
	}
	if _, _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	// Прогрев идёт в фоне и пишет строку «память: прогрев миниатюр» уже после «миниатюры: 3 за …»:
	// ждём последнюю, а не первую, иначе чтение журнала попадает между ними.
	memWarm := mem("прогрев миниатюр")
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for log := buf.String(); !thumbs.MatchString(log) || !memWarm.MatchString(log); log = buf.String() {
		if time.Now().After(deadline) {
			t.Fatalf("нет строк «миниатюры: 3 за …» и «память: прогрев миниатюр» в журнале: %q", buf.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	out := buf.String()
	for _, stage := range []string{"чтение папок", "прогрев миниатюр"} {
		if !mem(stage).MatchString(out) {
			t.Errorf("нет строки «память: %s — Go … МБ, куча … МБ»: %q", stage, out)
		}
	}
	if strings.Contains(out, "секрет") || strings.Contains(out, dir) {
		t.Errorf("в журнале путь или имя файла: %q", out)
	}

	// Отмена: прогрев встал до старта — ни «миниатюры», ни «память: прогрев».
	b := warmLogApp(t)
	var photos []library.Photo
	pdir := testkit.TempDir(t)
	for i := range 3 {
		name := fmt.Sprintf("p%d.jpg", i)
		photos = append(photos, library.Photo{ID: name, Path: testkit.JPEG(t, pdir, name, 60, 40), Name: name})
	}
	buf = captureLog(t)
	b.layouts.enter() // прогрев ждёт раскладки
	done := make(chan struct{})
	gen := b.scanGen
	go func() { defer close(done); b.warmThumbs(gen, photos) }()
	for { // ждём, когда warmThumbs поставил отмену
		b.mu.Lock()
		cancel := b.warm
		b.mu.Unlock()
		if cancel != nil {
			cancel()
			break
		}
		time.Sleep(time.Millisecond)
	}
	b.layouts.leave()
	select {
	case <-done:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("отменённый прогрев не кончился")
	}
	if out := buf.String(); strings.Contains(out, "миниатюры:") || strings.Contains(out, "прогрев миниатюр") {
		t.Errorf("отменённый прогрев написал в журнал: %q", out)
	}

	// Пустой список: греть нечего — строки нет.
	buf = captureLog(t)
	b.warmThumbs(b.scanGen, nil)
	if out := buf.String(); strings.Contains(out, "миниатюры:") {
		t.Errorf("пустой прогрев написал в журнал: %q", out)
	}
}
