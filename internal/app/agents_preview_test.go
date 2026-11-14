package app

// Превью службы агентов на настоящем App (0.31-143-61, -71): источник тот же, что у /media/p
// (mediaPhoto), недоступный том и не локальный путь — ErrUnavailable без обращения к диску, а
// Host.Status считает такие фото недоступными.

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/jpeg" // декодер для DecodeConfig в проверках
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// probes подменяет reachable: том «closed» не отвечает; список проб — для проверок «диска не касались».
type probes struct {
	mu   sync.Mutex
	list []string
}

func stubReachable(t *testing.T) *probes {
	t.Helper()
	pr := &probes{}
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(p string) bool {
		pr.mu.Lock()
		pr.list = append(pr.list, p)
		pr.mu.Unlock()
		return !strings.Contains(p, "closed")
	}
	return pr
}

func (p *probes) saw(sub string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, x := range p.list {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}

// cacheFiles — файлы кэша миниатюр и превью.
func agCacheFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// uncApp — App с проектом из двух фото (хорошее и на закрытом томе) и лентой: фото закрытого тома
// и два UNC (обе записи). В проект UNC не пускает сам Editor, в ленту — AddFolder/AddFilePaths, но
// Host не полагается на это: превью проверяет путь сам. Файлы закрытого тома лежат на диске: прочти
// код файл — превью бы удалось.
func uncApp(t *testing.T) (a *App, cacheDir string, ids map[string]string) {
	t.Helper()
	a = newApp(t)
	cacheDir = testkit.TempDir(t)
	c, err := library.NewCache(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir := testkit.TempDir(t)
	paths := map[string]string{
		"good":   testkit.JPEG(t, dir, "good.jpg", 600, 400),
		"closed": testkit.JPEG(t, dir, "closed_ref.jpg", 600, 400),
	}
	strip := testkit.JPEG(t, dir, "closed_strip.jpg", 600, 400)
	p := model.New()
	ids = map[string]string{}
	sp := &model.Spread{ID: "s1", Photos: []string{}, BgL: "#ffffff"}
	for k, path := range paths {
		id := library.IDFor(path)
		ids[k] = id
		p.Photos[id] = model.PhotoRef{Path: path, W: 600, H: 400}
		sp.Photos = append(sp.Photos, id)
	}
	p.Spreads = []*model.Spread{sp}
	a.ed.Store(mustEditor(t, p, ""))
	for k, path := range map[string]string{"strip": strip, "unc1": "//srv/share/unc1.jpg", "unc2": `\\srv\share\unc2.jpg`} {
		ids[k] = library.IDFor(path)
		a.byID[ids[k]] = library.Photo{ID: ids[k], Path: path, Name: filepath.Base(path), W: 600, H: 400}
	}
	return a, cacheDir, ids
}

func TestPreviewNoUNC(t *testing.T) {
	pr := stubReachable(t)
	a, cacheDir, ids := uncApp(t)
	h := a.agentsHost()
	ctx := context.Background()

	// Хорошее фото: превью получено из кэша превью (тот же источник, что у /media/p), JPEG нужной ширины.
	b, err := h.Preview(ctx, ids["good"], 512)
	if err != nil {
		t.Fatalf("хорошее фото: %v", err)
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(b)); err != nil || cfg.Width != 512 || cfg.Height != 341 {
		t.Fatalf("превью хорошего фото: %v %+v", err, cfg)
	}
	before := agCacheFiles(t, cacheDir)
	if len(before) == 0 {
		t.Fatal("превью не прошло через кэш миниатюр: источник не mediaPhoto/cache")
	}

	// Закрытый том (проект и лента), UNC в обеих записях, чужой ID: отказ без файла и кэша.
	for _, k := range []string{"closed", "unc1", "unc2", "strip"} {
		if _, err := h.Preview(ctx, ids[k], 512); !errors.Is(err, agents.ErrUnavailable) {
			t.Errorf("%s: ошибка %v, ждали ErrUnavailable", k, err)
		}
	}
	if _, err := h.Preview(ctx, "нет-такого", 512); !errors.Is(err, agents.ErrUnavailable) {
		t.Errorf("чужой ID: %v", err)
	}
	for _, unc := range []string{"srv/share", `srv\share`} {
		if pr.saw(unc) {
			t.Errorf("UNC %q дошёл до проверки тома: CheckLocalPath должен отказать раньше", unc)
		}
	}
	if !pr.saw("closed_ref") || !pr.saw("closed_strip") {
		t.Errorf("тома закрытых фото не проверены: %v", pr.list)
	}
	if after := agCacheFiles(t, cacheDir); len(after) != len(before) {
		t.Errorf("кэш пополнился отказанными фото: %v → %v", before, after)
	}
}

// Недоступный том попадает в Host.Status как пропавший (0.31-143-71): get_spread_preview
// считает их в unavailable, хотя в a.missing их нет. Хорошее фото — нет.
func TestStatusUnavailable(t *testing.T) {
	stubReachable(t)
	a, _, ids := uncApp(t)
	missing, unread := a.agentsHost().Status()
	if !contains(missing, ids["closed"]) {
		t.Errorf("Status.missing без фото закрытого тома: %v", missing)
	}
	if contains(missing, ids["good"]) || len(unread) != 0 {
		t.Errorf("Status: missing %v unread %v, хорошее фото не должно быть недоступным", missing, unread)
	}
	for i := 1; i < len(missing); i++ {
		if missing[i-1] >= missing[i] {
			t.Fatalf("missing не отсортирован без повторов: %v", missing)
		}
	}
	// Фото из ленты с закрытого тома — в a.missing ленты нет, но mediaPhoto называет его gone; в Status
	// только фото альбома (ленту Status не расширяет), повтор из a.missing не дублируется.
	a.mu.Lock()
	a.missing = []string{ids["closed"]}
	a.mu.Unlock()
	if m, _ := a.agentsHost().Status(); countOf(m, ids["closed"]) != 1 {
		t.Errorf("пропавшее из a.missing задвоено: %v", m)
	}
}

func contains(list []string, s string) bool { return countOf(list, s) > 0 }

func countOf(list []string, s string) (n int) {
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// Через сокет: unavailable в get_spread_preview растёт, когда том закрылся, и совпадает с числом
// недоступных фото разворота.
func TestSpreadPreviewUnavailable(t *testing.T) {
	r := newAgRig(t)
	r.start(t)
	if info := r.a.SetAgentsLevel(3); info.Level != 3 {
		t.Fatalf("уровень: %+v", info)
	}
	total := func() int {
		n := 0
		for i, sp := range r.a.editor().View().Spreads {
			res := r.call(t, "get_spread_preview", map[string]any{"spread": i + 1})
			if res.IsError || agImage(res) == nil {
				t.Fatalf("разворот %s: %q", sp.ID, agText(res))
			}
			u, _ := structured(t, res)["unavailable"].(float64) // 0 в ответе опущен
			n += int(u)
		}
		return n
	}
	base := total() // пропавшее фото стоит на развороте: оно в a.missing
	if base != 1 {
		t.Fatalf("до закрытия тома unavailable = %d, ждали 1 (пропавший файл)", base)
	}
	old := reachable
	t.Cleanup(func() { reachable = old })
	reachable = func(p string) bool { return !strings.Contains(p, "Петрова") }
	if got := total(); got != 2 {
		t.Errorf("том фото закрыт: unavailable = %d, ждали 2", got)
	}
}
