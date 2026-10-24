package colorpick

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// «С фото» окна выбора цвета (109 G2): SpreadPhotoColors по миниатюрам листа.

var (
	cRed  = color.RGBA{R: 220, G: 30, B: 30, A: 255}
	cBlue = color.RGBA{R: 30, G: 50, B: 210, A: 255}
)

// solidJPEG пишет однотонный JPEG w×h цвета c.
func solidJPEG(t testing.TB, dir, name string, w, h int, c color.RGBA) string {
	t.Helper()
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(im, im.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, im, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// kitApp — Picker с редактором и кэшем миниатюр в руках теста (вместо App).
type kitApp struct {
	*Picker
	ed        *album.Editor
	cache     *library.Cache
	reachable func(string) bool
}

func (a *kitApp) editor() *album.Editor { return a.ed }

// photo — как App.mediaPhoto для фото альбома: путь из проекта, файл не проверяется.
func (a *kitApp) photo(id string) (library.Photo, bool, bool) {
	ref, ok := a.ed.Ref(id)
	if !ok {
		return library.Photo{}, false, false
	}
	return library.Photo{ID: model.BaseID(id), Path: ref.Path, Name: filepath.Base(ref.Path)}, true, false
}

// colorApp — kitApp с кэшем миниатюр и листом s1: фото photos (ID → цвет) в сетке мест; tweak
// (может быть nil) правит проект до запуска редактора.
func colorApp(t testing.TB, photos map[string]color.RGBA, tweak func(p *model.Project, sp *model.Spread)) *kitApp {
	t.Helper()
	a := &kitApp{reachable: func(string) bool { return true }}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, cache.Close)
	a.cache = cache
	dir := testkit.TempDir(t)
	p := model.New()
	sp := &model.Spread{ID: "s1"}
	for i, id := range slices.Sorted(maps.Keys(photos)) {
		p.Photos[id] = model.PhotoRef{Path: solidJPEG(t, dir, id+".jpg", 600, 400, photos[id]), W: 600, H: 400}
		sp.Photos = append(sp.Photos, id)
		sp.Grid = append(sp.Grid, &model.Place{ID: model.NewID(), Photo: id,
			X: float64(5 + i%8*25), Y: float64(5 + i/8*20), W: 22, H: 17})
	}
	p.Spreads = []*model.Spread{sp}
	if tweak != nil {
		tweak(p, sp)
	}
	ed, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	a.ed = ed
	a.Picker = New(Host{
		Editor: func() *album.Editor { return a.ed },
		Photo:  a.photo,
		Cache:  func() *library.Cache { return a.cache },
		Ctx:    context.Background,
		// Через поле: тест подменяет проверку тома после создания.
		Reachable: func(path string) bool { return a.reachable(path) },
	})
	return a
}

// hexNear — hex в пределах ±tol по каждому каналу от want.
func hexNear(t testing.TB, hex string, want color.RGBA, tol int) bool {
	t.Helper()
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		t.Fatalf("не цвет %q", hex)
	}
	d := func(a int, b uint8) int { return max(a-int(b), int(b)-a) }
	return d(r, want.R) <= tol && d(g, want.G) <= tol && d(b, want.B) <= tol
}

// hasColors — в got есть цвет рядом с каждым из want.
func hasColors(t testing.TB, got []string, want ...color.RGBA) bool {
	t.Helper()
	for _, w := range want {
		if !slices.ContainsFunc(got, func(h string) bool { return hexNear(t, h, w, 6) }) {
			t.Errorf("нет цвета %v среди %v", w, got)
			return false
		}
	}
	return true
}

// countThumbReads считает чтения миниатюр «С фото» (подмена sampleThumb).
func countThumbReads(t testing.TB) *atomic.Int64 {
	t.Helper()
	var n atomic.Int64
	old := sampleThumb
	t.Cleanup(func() { sampleThumb = old })
	sampleThumb = func(ctx context.Context, path string) ([]colorspace.Lab, error) {
		n.Add(1)
		return old(ctx, path)
	}
	return &n
}

// Две фото разных цветов — оба в ответе; альбом не меняется.
func TestSpreadPhotoColorsTwoPhotos(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, nil)
	rev := a.editor().View().Rev
	got, err := a.PhotoColors("s1")
	if err != nil || len(got) != 2 || !hasColors(t, got, cRed, cBlue) {
		t.Fatalf("цвета: %v, %v", got, err)
	}
	if a.editor().View().Rev != rev {
		t.Error("метод изменил альбом")
	}
	// Одно фото в двух местах весит один раз: красного не вдвое больше, порядок прежний.
	again, err := a.PhotoColors("s1")
	if err != nil || !slices.Equal(again, got) {
		t.Errorf("повторный вызов: %v %v", again, err)
	}
}

// Фото на фоне берётся, как фото места; своё из библиотеки (Lib) — оформление, не берётся.
func TestSpreadPhotoColorsBackdrop(t *testing.T) {
	bg := func(p *model.Project, sp *model.Spread) {
		sp.BgPhoto = &model.Backdrop{Photo: "bg", X: 0, Y: 0, W: 210, H: 148}
	}
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "bg": cBlue}, func(p *model.Project, sp *model.Spread) {
		bg(p, sp)
		sp.Photos, sp.Grid = sp.Photos[:1], sp.Grid[:1] // "bg" — только на фоне
	})
	got, err := a.PhotoColors("s1")
	if err != nil || len(got) != 2 || !hasColors(t, got, cRed, cBlue) {
		t.Fatalf("с фоном: %v %v", got, err)
	}
	// Чистая часть: Lib и повторы.
	ids := colorPhotoIDs(model.SpreadView{Photos: []string{"a", "a-2", "b"}, Backdrops: []model.BackdropView{
		{Photo: "bg"}, {Photo: "lib1", Lib: "my:0123456789abcdef"}, {Photo: "b"}}})
	if !slices.Equal(ids, []string{"a", "b", "bg"}) {
		t.Errorf("colorPhotoIDs: %v", ids)
	}
}

// Пустой лист — [] без ошибки (не nil: на проводе [] вместо null).
func TestSpreadPhotoColorsEmptySpread(t *testing.T) {
	a := colorApp(t, nil, nil)
	got, err := a.PhotoColors("s1")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("пустой лист: %#v %v", got, err)
	}
	a.cache = nil // без кэша миниатюр (до запуска) — тоже []
	if got, err := a.PhotoColors("s1"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("без кэша: %#v %v", got, err)
	}
}

// Пропавшее фото пропускается, остальные считаются; все пропали — [].
func TestSpreadPhotoColorsMissingPhoto(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, nil)
	ref, _ := a.editor().Ref("b")
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
	got, err := a.PhotoColors("s1")
	if err != nil || len(got) != 1 || !hasColors(t, got, cRed) {
		t.Fatalf("одно пропало: %v %v", got, err)
	}
	ref, _ = a.editor().Ref("a")
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
	if got, err := a.PhotoColors("s1"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("все пропали: %#v %v", got, err)
	}
}

// Чужой ID, пустой, обложка без обложки — ошибка по-русски без паники.
func TestSpreadPhotoColorsUnknownSpread(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed}, nil)
	for _, id := range []string{"нет", "", "S1", "../x"} {
		got, err := a.PhotoColors(id)
		if err == nil || err.Error() != "лист не найден" || got == nil || len(got) != 0 {
			t.Errorf("%q: %#v %v", id, got, err)
		}
	}
	// Обложка — тоже лист.
	v := model.View{Cover: &model.CoverView{Sheet: model.SpreadView{ID: model.CoverID}}}
	if _, ok := viewSheet(v, model.CoverID); !ok {
		t.Error("обложка не найдена как лист")
	}
}

// Второй вызов не читает миниатюры; сменился лист — читает только новые фото.
func TestSpreadPhotoColorsCached(t *testing.T) {
	a := colorApp(t, map[string]color.RGBA{"a": cRed, "b": cBlue}, nil)
	reads := countThumbReads(t)
	first, err := a.PhotoColors("s1")
	if err != nil || reads.Load() != 2 {
		t.Fatalf("первый вызов: %d чтений, %v", reads.Load(), err)
	}
	second, err := a.PhotoColors("s1")
	if err != nil || reads.Load() != 2 || !slices.Equal(first, second) {
		t.Fatalf("второй вызов: %d чтений, %v %v", reads.Load(), second, err)
	}
}

// Предел кэша точек: старые вытесняются, свежие остаются.
func TestPointsCacheLimit(t *testing.T) {
	var c pointsCache
	for i := range pointsCacheLen + 10 {
		c.put("k"+strconv.Itoa(i), []colorspace.Lab{{L: float32(i)}})
	}
	if _, ok := c.get("k0"); ok {
		t.Error("самая старая запись не вытеснена")
	}
	if got, ok := c.get("k" + strconv.Itoa(pointsCacheLen+9)); !ok || got[0].L != float32(pointsCacheLen+9) {
		t.Error("свежая запись потеряна")
	}
	if c.ll.Len() != pointsCacheLen {
		t.Errorf("в кэше %d записей", c.ll.Len())
	}
}

// 64 фото при готовых миниатюрах: ответ ≤ порога (Dominant на 65 536 точках + выборка из кэша).
func TestSpreadPhotoColorsCost(t *testing.T) {
	photos := map[string]color.RGBA{}
	for i := range 64 {
		photos["p"+strconv.Itoa(1000+i)] = color.RGBA{R: uint8(i * 4), G: uint8(255 - i*4), B: uint8(i * 2), A: 255}
	}
	a := colorApp(t, photos, nil)
	if got, err := a.PhotoColors("s1"); err != nil || len(got) == 0 { // греем миниатюры и кэш точек
		t.Fatalf("прогрев: %v %v", got, err)
	}
	d := testkit.Fastest(3, func() { _, _ = a.PhotoColors("s1") })
	t.Logf("64 фото, миниатюры готовы: %v", d)
	testkit.CostTest(t)
	if lim := testkit.Limit(300 * time.Millisecond); d > lim {
		t.Errorf("64 фото: %v > %v", d, lim)
	}
}
