package colorpick

import (
	"context"
	"image"
	"image/color"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пипетка окна выбора цвета (109 G3): PickColorAt по пробному кадру листа.

// placeCenter — центр места i сетки colorApp (мм листа).
func placeCenter(i int) (x, y float64) {
	return float64(5+i%8*25) + 11, float64(5+i/8*20) + 8.5
}

// hexRGB разбирает "#rrggbb".
func hexRGB(t testing.TB, hex string) (r, g, b int) {
	t.Helper()
	if len(hex) != 7 || hex[0] != '#' {
		t.Fatalf("не цвет %q", hex)
	}
	v, err := strconv.ParseUint(hex[1:], 16, 32)
	if err != nil {
		t.Fatalf("не цвет %q", hex)
	}
	return int(v >> 16), int(v >> 8 & 255), int(v & 255)
}

// Лист без фото: цвет страницы, обе половины разворота; альбом не меняется.
func TestPickColorAtPageColor(t *testing.T) {
	a := colorApp(t, nil, func(p *model.Project, sp *model.Spread) { p.Settings.Bg = "#336699" })
	rev := a.editor().View().Rev
	sv, _ := viewSheet(a.editor().View(), "s1")
	for _, x := range []float64{1, sv.W / 4, sv.W * 3 / 4, sv.W - 0.01} {
		got, err := a.PickAt("s1", x, sv.H/2)
		if err != nil || got != "#336699" {
			t.Errorf("x=%v: %q, %v", x, got, err)
		}
	}
	if a.editor().View().Rev != rev {
		t.Error("метод изменил альбом")
	}
}

// Два фото разных цветов: в каждом месте — его цвет (JPEG и превью дают малое расхождение).
func TestPickColorAtPhoto(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, nil)
	for i, want := range []color.RGBA{cRed, cBlue} {
		x, y := placeCenter(i)
		got, err := a.PickAt("s1", x, y)
		if err != nil || !hexNear(t, got, want, 3) {
			t.Errorf("фото %d: %q, %v, ждали %v", i, got, err, want)
		}
	}
}

// Ч/б место: серый (R = G = B); соседнее цветное остаётся цветным.
func TestPickColorAtGrayPhoto(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, func(p *model.Project, sp *model.Spread) {
		ref := p.Photos["a"]
		ref.Adj = &model.Tone{Gray: true}
		p.Photos["a"] = ref
	})
	x, y := placeCenter(0)
	got, err := a.PickAt("s1", x, y)
	if r, g, b := hexRGB(t, got); err != nil || r != g || g != b {
		t.Errorf("ч/б место: %q, %v", got, err)
	}
	x, y = placeCenter(1)
	if got, _ = a.PickAt("s1", x, y); !hexNear(t, got, cBlue, 3) {
		t.Errorf("цветное место: %q", got)
	}
}

// В кадр идёт превью из кэша миниатюр, а не оригинал: сцена после подготовки не ссылается на
// файлы альбома, у фото 3000×2000 картинка не больше 2048 px.
func TestPickColorAtUsesPreview(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, func(p *model.Project, sp *model.Spread) {
		for _, id := range []string{"a", "b"} {
			ref := p.Photos[id]
			ref.Path = solidJPEG(t, testkit.TempDir(t), id+"-big.jpg", 3000, 2000, cRed)
			ref.W, ref.H = 3000, 2000
			p.Photos[id] = ref
		}
	})
	p, v := a.editor().Snapshot()
	sv, _ := viewSheet(v, "s1")
	es := export.SheetScene(p, sv)
	a.pickPreviews(context.Background(), p.Photos, &es)
	if len(es.Photos) != 2 {
		t.Fatalf("в сцене %d фото, ждали 2", len(es.Photos))
	}
	for _, ph := range es.Photos {
		if ph.Path == p.Photos["a"].Path || ph.Path == p.Photos["b"].Path {
			t.Errorf("в сцене оригинал %s", ph.Path)
		}
		f, err := os.Open(ph.Path)
		if err != nil {
			t.Fatal(err)
		}
		cfg, _, err := image.DecodeConfig(f)
		_ = f.Close()
		if err != nil || max(cfg.Width, cfg.Height) > 2048 {
			t.Errorf("превью %v, %v", cfg, err)
		}
	}
	x, y := placeCenter(0)
	if got, err := a.PickAt("s1", x, y); err != nil || !hexNear(t, got, cRed, 3) {
		t.Errorf("цвет: %q, %v", got, err)
	}
}

// Пропавший файл фото (109-g3): кадр строится без него, соседнее фото на месте, ошибки нет.
func TestPickColorAtMissingPhoto(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue, "c": cRed}, nil)
	p, _ := a.editor().Snapshot()
	if err := os.Remove(p.Photos["b"].Path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.Photos["c"].Path); err != nil { // два пропавших сразу
		t.Fatal(err)
	}
	x, y := placeCenter(0)
	if got, err := a.PickAt("s1", x, y); err != nil || !hexNear(t, got, cRed, 3) {
		t.Errorf("целое фото: %q, %v", got, err)
	}
	page, err := a.PickAt("s1", 150, 100) // пустое место листа
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range []int{1, 2} {
		x, y = placeCenter(i)
		got, err := a.PickAt("s1", x, y)
		if err != nil || got != page {
			t.Errorf("пропавшее фото %d: %q, %v (ждали цвет страницы)", i, got, err)
		}
	}
}

// Пропавший фон-фото и фото-стикер убираются из сцены так же, как фото мест.
func TestPickColorAtMissingBackdrop(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "bg": cBlue}, func(p *model.Project, sp *model.Spread) {
		sp.BgPhoto = &model.Backdrop{Photo: "bg", X: 0, Y: 0, W: 210, H: 148}
		sp.Photos, sp.Grid = sp.Photos[:1], sp.Grid[:1]
	})
	p, _ := a.editor().Snapshot()
	x, y := 150.0, 100.0 // вне места: виден только фон-фото
	if got, err := a.PickAt("s1", x, y); err != nil || !hexNear(t, got, cBlue, 3) {
		t.Fatalf("фон есть: %q, %v", got, err)
	}
	if err := os.Remove(p.Photos["bg"].Path); err != nil {
		t.Fatal(err)
	}
	a.editor().SetTone("a", 1, 0) // новый rev: прежний кадр с фоном не годится
	got, err := a.PickAt("s1", x, y)
	if err != nil || hexNear(t, got, cBlue, 30) {
		t.Errorf("фон пропал: %q, %v (ждали цвет страницы)", got, err)
	}
}

// Кадр кэшируется по rev: второй вызов не рисует, правка даёт новый кадр; параллельные запросы
// одного ключа ждут один рендер.
func TestPickColorAtCacheByRev(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed}, nil)
	x, y := placeCenter(0)
	first, err := a.PickAt("s1", x, y)
	if err != nil || a.pick.renders.Load() != 1 {
		t.Fatalf("первый вызов: %q, %v, кадров %d", first, err, a.pick.renders.Load())
	}
	if _, err := a.PickAt("s1", 150, 100); err != nil || a.pick.renders.Load() != 1 {
		t.Errorf("второй вызов (другая точка) нарисовал кадр: %d", a.pick.renders.Load())
	}
	// Холостая правка: rev тот же — кадр тот же.
	a.editor().SetTone("a", 0, 0)
	if _, err := a.PickAt("s1", x, y); err != nil || a.pick.renders.Load() != 1 {
		t.Errorf("холостая правка нарисовала кадр: %d", a.pick.renders.Load())
	}
	a.editor().SetTone("a", 40, 0) // реальная правка: rev вырос
	changed, err := a.PickAt("s1", x, y)
	if err != nil || a.pick.renders.Load() != 2 || changed == first {
		t.Errorf("после правки: %q → %q, кадров %d", first, changed, a.pick.renders.Load())
	}
	// Другой rev (после отмены — возврат) снова ключ нового кадра; ключ первого уже мог уйти.
	a.editor().SetTone("a", 41, 0)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := a.PickAt("s1", x, y); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := a.pick.renders.Load(); n != 3 {
		t.Errorf("8 параллельных вызовов нарисовали %d кадров, ждали 1", n-2)
	}
}

// Кэш держит не больше трёх кадров; самый старый уходит первым.
func TestPickCacheKeepsThree(t *testing.T) {
	ed := newEditor(t)
	var c pickCache
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	build := func() (*image.RGBA, error) { return img, nil }
	for rev := range 5 {
		k := pickKey{ed: weak.Make(ed), id: "s1", rev: rev}
		if _, err := c.get(context.Background(), k, build); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.keys) != pickKeep || len(c.imgs) != pickKeep {
		t.Fatalf("в кэше %d ключей и %d кадров", len(c.keys), len(c.imgs))
	}
	if c.keys[0].rev != 2 || c.keys[pickKeep-1].rev != 4 {
		t.Errorf("остались ревизии %v", c.keys)
	}
}

// Ошибки: не число, бесконечность, вне листа, чужой лист; точка у самого края — не ошибка.
func TestPickColorAtBadArgs(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed}, nil)
	sv, _ := viewSheet(a.editor().View(), "s1")
	inf, nan := math.Inf(1), math.NaN()
	for _, c := range [][2]float64{{nan, 5}, {5, nan}, {inf, 5}, {5, -inf}, {-0.01, 5}, {5, -0.01}, {sv.W, 5}, {5, sv.H}, {sv.W + 50, sv.H + 50}} {
		if got, err := a.PickAt("s1", c[0], c[1]); err == nil || err.Error() != "точка вне листа" || got != "" {
			t.Errorf("%v: %q, %v", c, got, err)
		}
	}
	for _, id := range []string{"нет", ""} {
		if _, err := a.PickAt(id, 5, 5); err == nil || err.Error() != "лист не найден" {
			t.Errorf("лист %q: %v", id, err)
		}
	}
	if _, err := a.PickAt("s1", 0, 0); err != nil {
		t.Errorf("левый верхний угол: %v", err)
	}
	if _, err := a.PickAt("s1", sv.W-1e-9, sv.H-1e-9); err != nil {
		t.Errorf("правый нижний угол: %v", err)
	}
}

// Обложка: точка на корешке даёт цвет его плашки (сцена обложки, а не листа).
func TestPickColorAtCoverSpine(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed}, nil)
	if _, err := a.editor().CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	a.editor().SetCoverSpineBg("#102030")
	cv := a.editor().View().Cover
	if cv == nil {
		t.Fatal("обложки нет")
	}
	got, err := a.PickAt(cv.Sheet.ID, cv.Sheet.W/2, cv.Sheet.H/2)
	if err != nil || got != "#102030" {
		t.Errorf("корешок: %q, %v", got, err)
	}
}

// Цена кадра: 16 фото на листе, превью готовы, — не дольше 400 мс (замер 251 мс, спека 109 §4).
func TestPickColorAtCost(t *testing.T) {
	photos := map[string]color.RGBA{}
	for i := range 16 {
		photos["p"+strconv.Itoa(100+i)] = color.RGBA{R: uint8(20 + i*12), G: 90, B: uint8(250 - i*12), A: 255}
	}
	a := colorApp(t, photos, nil)
	x, y := placeCenter(0)
	if _, err := a.PickAt("s1", x, y); err != nil { // превью создаются здесь
		t.Fatal(err)
	}
	a.editor().SetTone("p100", 10, 0) // новый rev: рисуется заново, превью уже в кэше
	t0 := time.Now()
	if _, err := a.PickAt("s1", x, y); err != nil {
		t.Fatal(err)
	}
	d := time.Since(t0)
	t.Logf("кадр 16 фото: %v", d)
	if lim := testkit.Limit(400 * time.Millisecond); d > lim {
		t.Errorf("кадр 16 фото: %v, предел %v", d, lim)
	}
	if n := a.pick.renders.Load(); n != 2 {
		t.Errorf("кадров %d, ждали 2", n)
	}
}

// Длинный лист: кадр укладывается в pickSidePx по большей стороне (раньше dpi считался по
// ширине, и лист 100×1000 мм давал кадр 1600×16000 px).
func TestPickColorAtLongSheetFrame(t *testing.T) {
	a := colorApp(t, nil, func(p *model.Project, sp *model.Spread) {
		p.Settings.PageW, p.Settings.PageH = 50, 1000 // лист разворота 100×1000 мм
	})
	sv, _ := viewSheet(a.editor().View(), "s1")
	if sv.W != 100 || sv.H != 1000 {
		t.Fatalf("лист %v×%v, ждали 100×1000", sv.W, sv.H)
	}
	if _, err := a.PickAt("s1", 50, 500); err != nil {
		t.Fatal(err)
	}
	if len(a.pick.imgs) != 1 {
		t.Fatalf("кадров в кэше %d", len(a.pick.imgs))
	}
	for _, img := range a.pick.imgs {
		b := img.Bounds()
		if b.Dy() > pickSidePx || b.Dx() > pickSidePx || b.Dy() < pickSidePx/2 {
			t.Errorf("кадр %d×%d, ждали большую сторону ≈ %d", b.Dx(), b.Dy(), pickSidePx)
		}
	}
}

// pickDPI: большая сторона побеждает; нулевого dpi нет, а сверхдлинный лист не раздувает кадр
// сверх предела (при pickMinDPI = 10 лист 100 м давал кадр в 39 000 px).
func TestPickDPI(t *testing.T) {
	cases := []struct{ w, h float64 }{
		{420, 297}, {100, 1000}, {1000, 100}, {297, 297}, {100, 100000}, {100000, 100}, {1e9, 1},
	}
	for _, c := range cases {
		dpi := pickDPI(c.w, c.h)
		if dpi < pickMinDPI {
			t.Errorf("%v×%v: dpi %d", c.w, c.h, dpi)
		}
		if long := math.Max(c.w, c.h) / 25.4 * float64(dpi); long > pickSidePx && dpi > pickMinDPI { // на полу решает предел пикселей render
			t.Errorf("%v×%v: большая сторона %.0f px при dpi %d", c.w, c.h, long, dpi)
		}
	}
}

// Рендеров кадров одновременно не больше pickParallel (разные листы); отмена ctx снимает ожидание
// слота без рендера.
func TestPickCacheLimitsParallel(t *testing.T) {
	ed := newEditor(t)
	var c pickCache
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var cur, peak atomic.Int64
	var peakMu sync.Mutex
	release := make(chan struct{})
	build := func() (*image.RGBA, error) {
		n := cur.Add(1)
		peakMu.Lock()
		peak.Store(max(peak.Load(), n))
		peakMu.Unlock()
		<-release
		cur.Add(-1)
		return img, nil
	}
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Go(func() {
			k := pickKey{ed: weak.Make(ed), id: "s" + strconv.Itoa(i), rev: 1}
			if _, err := c.get(context.Background(), k, build); err != nil {
				t.Error(err)
			}
		})
	}
	deadline := time.Now().Add(5 * time.Second)
	for cur.Load() < pickParallel && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // лишний рендер успел бы стартовать
	if n := cur.Load(); n != pickParallel {
		t.Errorf("идёт %d рендеров, ждали %d", n, pickParallel)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	k := pickKey{ed: weak.Make(ed), id: "late", rev: 1}
	if _, err := c.get(ctx, k, func() (*image.RGBA, error) { t.Error("рендер после отмены"); return img, nil }); err == nil {
		t.Error("отменённый запрос без ошибки")
	}
	close(release)
	wg.Wait()
	if p := peak.Load(); p != pickParallel {
		t.Errorf("пик %d рендеров, ждали %d", p, pickParallel)
	}
	if len(c.imgs) != pickKeep {
		t.Errorf("в кэше %d кадров", len(c.imgs))
	}
}

// Рамочка-картинка с недоступного тома убирается до Stat: том не отвечает, пипетка не виснет.
func TestPickSceneBorderUnreachable(t *testing.T) {
	a := colorApp(t, nil, nil)
	dir := testkit.TempDir(t)
	files := []string{solidJPEG(t, dir, "b1.jpg", 10, 10, cRed), solidJPEG(t, dir, "b2.jpg", 10, 10, cRed)}
	probed := 0
	a.reachable = func(p string) bool { probed++; return p == files[0] }
	es := render.Spread{Borders: []render.BorderDraw{
		{Pic: true, Path: files[0]}, {Pic: true, Path: files[1]}, {Pic: true, Path: files[1]}, {Pic: true, Path: ""}}}
	a.pickPreviews(context.Background(), nil, &es)
	if len(es.Borders) != 1 || es.Borders[0].Path != files[0] {
		t.Errorf("рамочки: %+v", es.Borders)
	}
	if probed != 2 { // по разу на путь, пустой путь не пробуется
		t.Errorf("проб тома %d, ждали 2", probed)
	}
}

func newEditor(t testing.TB) *album.Editor {
	t.Helper()
	ed, err := album.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	return ed
}
