package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Показ макета на фото (163 G3): метка ph у /media/look — места заняты свободными фото ленты, ответ
// JPEG. Сцена тестов: три однотонные фото (красное, зелёное, синее), макет с обложкой (задняя 1 место,
// лицевая 2) и двумя шагами по 2 места — по цвету в центре места видно, какое фото где стоит.

var phColors = []string{"#ff0000", "#00ff00", "#0000ff"}

// solidJPEG — однотонный JPEG w×h.
func solidJPEG(t *testing.T, dir, name string, w, h int, hex string) string {
	t.Helper()
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	c := color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// phApp — App с библиотекой макетов, пустым альбомом, кэшем миниатюр и лентой из однотонных фото
// (размер w×h) цветов colors; папок не сканируем. Возвращает App, ленту и каталог кэша.
func phApp(t *testing.T, w, h int, colors []string) (*App, []library.Photo, string) {
	t.Helper()
	a := lookApp(t)
	cacheDir := testkit.TempDir(t)
	c, err := library.NewCache(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir := testkit.TempDir(t)
	var photos []library.Photo
	for i, hex := range colors {
		p, err := library.ReadInfo(solidJPEG(t, dir, fmt.Sprintf("Кадр_%02d.jpg", i), w, h, hex))
		if err != nil {
			t.Fatal(err)
		}
		photos = append(photos, p)
		a.byID[p.ID] = p
	}
	a.photos = slices.Clone(photos)
	return a, photos, cacheDir
}

// setFeed — лента App (под замком).
func setFeed(a *App, list []library.Photo) {
	a.mu.Lock()
	a.photos = slices.Clone(list)
	a.mu.Unlock()
}

// photoLook — макет: обложка (задняя 1 место, лицевая 2), два шага по 2 места на развороте.
func photoLook() model.AlbumTpl {
	l := testLook("#ffffff")
	one := []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.6}}
	two := []model.Box{{X: 0.05, Y: 0.2, W: 0.4, H: 0.6}, {X: 0.55, Y: 0.2, W: 0.4, H: 0.6}}
	back := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: one, Page: []model.TplBg{{Color: "#ffffff"}}}
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: two, Page: []model.TplBg{{Color: "#ffffff"}}}
	l.Cover = &model.CoverTpl{Bg: &model.TplBg{Color: "#ffffff"}, Back: &back, Front: &front}
	l.Steps = l.Steps[:2]
	l.Steps[1].Role = ""
	return l
}

func savePhotoLook(t *testing.T, a *App) string {
	t.Helper()
	return "/media/look/" + saveCoverLook(t, a, "На фото", photoLook())
}

func lookJPEG(t *testing.T, a *App, url string) image.Image {
	t.Helper()
	rec := getTpl(a, http.MethodGet, url)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("%s: %d %q", url, rec.Code, rec.Header().Get("Content-Type"))
	}
	img, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("%s: %v", url, err)
	}
	return img
}

// seatColors — какое фото в центре каждого места: "r", "g", "b" (цвета phColors) или "-" (не фото).
// xs — центры мест по горизонтали в долях ширины, y — по вертикали.
func seatColors(img image.Image, xs []float64, y float64) string {
	b := img.Bounds()
	var s strings.Builder
	for _, x := range xs {
		c := img.At(b.Min.X+int(x*float64(b.Dx())), b.Min.Y+int(y*float64(b.Dy())))
		switch {
		case isHex(c, "#ff0000"):
			s.WriteString("r")
		case isHex(c, "#00ff00"):
			s.WriteString("g")
		case isHex(c, "#0000ff"):
			s.WriteString("b")
		default:
			s.WriteString("-")
		}
	}
	return s.String()
}

var (
	stepXs  = []float64{0.25, 0.75}         // два места на развороте
	coverXs = []float64{0.25, 0.625, 0.875} // обложка: задняя 1, лицевая 2 (середины мест)
)

func TestParseLookQueryPh(t *testing.T) {
	const base = "i=1&px=256&r=1"
	for _, c := range []struct {
		raw string
		ph  uint64
		px  int
	}{
		{base + "&ph=1", 1, 256},
		{base + "&ph=0", 0, 256},
		{base + "&ph=9007199254740991", 1<<53 - 1, 256},
		{base + "&ph=0000000000000007", 7, 256}, // 16 цифр
		{"i=0&px=2048&r=2&ph=5", 5, 2048},
		{"ph=5&r=2&px=1500&i=3", 5, 1500}, // порядок не важен: px проверяется после разбора всех
		{base, 0, 256},
	} {
		q, ok := parseLookQuery(c.raw)
		if !ok || q.ph != c.ph || q.px != c.px {
			t.Errorf("%s: %+v %v", c.raw, q, ok)
		}
	}
	for _, raw := range []string{
		base + "&ph=", base + "&ph=-1", base + "&ph=+1", base + "&ph=1.5", base + "&ph=1e3", base + "&ph=0x10", base + "&ph=abc",
		base + "&ph=1_0", base + "&ph=%20", base + "&ph=1&ph=2", base + "&ph=1&ph=1",
		base + "&ph=9007199254740992",              // 2⁵³
		base + "&ph=00000000000000007",             // 17 цифр
		base + "&ph=12345678901234567890",          // больше 2⁵³
		base + "&ph=18446744073709551616",          // больше uint64
		"i=1&px=2048&r=1",                          // 2048 без ph
		"i=1&px=1025&r=1&ph=0",                     // ph=0 — прежний предел 1024
		"i=1&px=2049&r=1&ph=1",                     // выше 2048 нельзя и с ph
		"i=1&px=63&r=1&ph=1",                       // ниже предела
		base + "&ph=1&bg=2", base + "&ph=1&path=x", // остальные параметры не ослаблены
	} {
		if q, ok := parseLookQuery(raw); ok {
			t.Errorf("%s: принято %+v", raw, q)
		}
	}
}

// Места листа — свободные фото ленты по порядку: лицевая обложки с начала, задняя — после неё, шаги
// дальше по кругу; ответ JPEG; альбом не меняется; повтор из кэша; смена ленты — новая картинка.
func TestServeLookOnPhotos(t *testing.T) {
	a, photos, cacheDir := phApp(t, 600, 400, phColors)
	base := savePhotoLook(t, a)
	ed := a.editor()
	rev, undo := ed.View().Rev, ed.View().CanUndo

	t.Run("шаги и обложка", func(t *testing.T) { phSteps(t, a, base) })
	t.Run("без ph серые места PNG", func(t *testing.T) { phPlain(t, a, base) })
	t.Run("кадр не больше 2048", func(t *testing.T) {
		phFrame(t, a, base)
		// Крупный кадр берёт превью 2048, плитка — миниатюры: у фото два файла копий.
		if n := countFiles(t, cacheDir); n < 2*len(photos) {
			t.Errorf("в кэше %d файлов, ждали миниатюры и превью", n)
		}
	})
	t.Run("альбом не тронут, HEAD", func(t *testing.T) {
		if v := ed.View(); v.Rev != rev || v.CanUndo != undo {
			t.Error("показ на фото изменил альбом")
		}
		if getLook(a, http.MethodHead, base+"?v=1&i=1&px=256&r=2&ph=7") != 200 {
			t.Error("HEAD")
		}
		if getLook(a, http.MethodPost, base+"?v=1&i=1&px=256&r=2&ph=7") != 404 {
			t.Error("POST")
		}
	})
	t.Run("кэш и лента", func(t *testing.T) { phFeed(t, a, photos, base) })
	t.Run("ленты нет", func(t *testing.T) {
		setFeed(a, nil)
		defer setFeed(a, photos)
		if got := seatColors(lookJPEG(t, a, base+"?v=1&i=1&px=256&r=2&ph=7"), stepXs, 0.5); got != "--" {
			t.Errorf("без фото: %q, ждали серые места", got)
		}
	})
	t.Run("запрос снят", func(t *testing.T) { phCanceled(t, a, base) })
}

func phSteps(t *testing.T, a *App, base string) {
	// Шаг 1 начинается после 3 мест обложки (по кругу из 3 фото — с красного), шаг 2 — после мест шага 1.
	if got := seatColors(lookJPEG(t, a, base+"?v=1&i=1&px=256&r=2&ph=7"), stepXs, 0.5); got != "rg" {
		t.Errorf("шаг 1: %q, ждали rg", got)
	}
	if got := seatColors(lookJPEG(t, a, base+"?v=1&i=2&px=256&r=2&ph=7"), stepXs, 0.5); got != "br" {
		t.Errorf("шаг 2: %q, ждали br", got)
	}
	// Обложка из двух сторон: слева задняя (после мест лицевой — синее), справа лицевая с начала.
	if got := seatColors(lookJPEG(t, a, base+"?v=1&i=0&px=256&r=2&ph=7"), coverXs, 0.5); got != "brg" {
		t.Errorf("обложка: %q, ждали brg", got)
	}
	// f=1: одна лицевая с красного.
	if got := seatColors(lookJPEG(t, a, base+"?v=1&i=0&px=256&r=1&f=1&ph=7"), stepXs, 0.5); got != "rg" {
		t.Errorf("лицевая: %q, ждали rg", got)
	}
}

func phPlain(t *testing.T, a *App, base string) {
	for _, q := range []string{"?v=1&i=1&px=256&r=2", "?v=1&i=1&px=256&r=2&ph=0"} {
		rec := getTpl(a, http.MethodGet, base+q)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s: %d %q", q, rec.Code, rec.Header().Get("Content-Type"))
		}
		if got := seatColors(lookPNG(t, a, base+q), stepXs, 0.5); got != "--" {
			t.Errorf("%s: %q, ждали серые места", q, got)
		}
	}
}

func phFrame(t *testing.T, a *App, base string) {
	for _, c := range []struct {
		q    string
		w, h int
	}{
		{"px=2048&r=0.2", 409, 2045}, {"px=2048&r=5", 2048, 410}, {"px=2048&r=1", 2048, 2048}, {"px=1500&r=1.5", 1500, 1000},
	} {
		b := lookJPEG(t, a, base+"?v=1&i=1&ph=3&"+c.q).Bounds()
		if b.Dx() > lookPhotoPx || b.Dy() > lookPhotoPx || b.Dx() < c.w-1 || b.Dy() < c.h-2 {
			t.Errorf("%s: %v, ждали ≈ %d×%d", c.q, b, c.w, c.h)
		}
	}
}

func phFeed(t *testing.T, a *App, photos []library.Photo, base string) {
	url := base + "?v=1&i=1&px=256&r=2&ph=9"
	first := getTpl(a, http.MethodGet, url).Body.Bytes()
	// ph — только метка WebView: другое число, та же картинка.
	if again := getTpl(a, http.MethodGet, strings.Replace(url, "ph=9", "ph=10", 1)).Body.Bytes(); !bytes.Equal(first, again) {
		t.Error("другая метка ph — другая картинка")
	}
	// Лента переставлена: тот же URL — другие фото (ключ кэша учитывает фото).
	setFeed(a, []library.Photo{photos[2], photos[1], photos[0]})
	if got := seatColors(lookJPEG(t, a, url), stepXs, 0.5); got != "bg" {
		t.Errorf("после перестановки: %q, ждали bg", got)
	}
	setFeed(a, photos)
	if got := seatColors(lookJPEG(t, a, url), stepXs, 0.5); got != "rg" {
		t.Errorf("лента возвращена: %q, ждали rg", got)
	}
}

func phCanceled(t *testing.T, a *App, base string) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, base+"?v=1&i=1&px=777&r=2&ph=11", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(rec, req)
	if rec.Body.Len() != 0 || rec.Header().Get("Content-Type") != "" {
		t.Errorf("снятый запрос получил ответ: %d %q", rec.Body.Len(), rec.Header().Get("Content-Type"))
	}
	// Серое в кэш не попало: живой запрос того же вида получает фото.
	if got := seatColors(lookJPEG(t, a, base+"?v=1&i=1&px=777&r=2&ph=11"), stepXs, 0.5); got != "rg" {
		t.Errorf("после снятого запроса: %q, ждали rg", got)
	}
}

// Фото и копии пропадают: лист всё равно рисуется (серые места), ошибки нет и серое не залипает в кэше.
func TestServeLookOnPhotosGone(t *testing.T) {
	a, photos, _ := phApp(t, 600, 400, phColors)
	base := savePhotoLook(t, a)
	url := base + "?v=1&i=1&px=256&r=2&ph=7"

	t.Run("копия удалена", func(t *testing.T) {
		// Между выдачей копии и рисованием чистка кэша убирает файл первого места: render не находит
		// его — один повтор без фото (163-19): 200, серые места, в кэш не кладётся.
		old, calls := reachable, 0
		t.Cleanup(func() { reachable = old })
		killed := ""
		reachable = func(string) bool {
			if calls++; calls == 2 { // перед вторым фото копия первого уже выдана
				f, err := a.cache.FileCtx(context.Background(), photos[0], false)
				if err != nil {
					t.Error(err)
				} else if killed = f; os.Remove(f) != nil {
					t.Error("копию не удалить")
				}
			}
			return true
		}
		if got := seatColors(lookJPEG(t, a, url), stepXs, 0.5); got != "--" {
			t.Errorf("копия удалена: %q, ждали серые места", got)
		}
		reachable = old
		if killed == "" {
			t.Fatal("копия не удалялась")
		}
		// Повторный запрос: серое в кэш не попало, фото снова на местах.
		if got := seatColors(lookJPEG(t, a, url), stepXs, 0.5); got != "rg" {
			t.Errorf("повтор: %q, ждали rg (серое залипло в кэше?)", got)
		}
	})

	t.Run("том пропал", func(t *testing.T) {
		old := reachable
		t.Cleanup(func() { reachable = old })
		// Доступно одно фото (зелёное): обход ограничен 2 × мест, пропавшие пропускаются — второе место серое.
		reachable = func(p string) bool { return p == photos[1].Path }
		if got := seatColors(lookJPEG(t, a, url+"2"), stepXs, 0.5); got != "g-" {
			t.Errorf("доступно одно фото: %q, ждали g-", got)
		}
		reachable = func(string) bool { return false }
		if got := seatColors(lookJPEG(t, a, url+"3"), stepXs, 0.5); got != "--" {
			t.Errorf("тома нет: %q, ждали серые места", got)
		}
	})

	t.Run("оригинал удалён", func(t *testing.T) {
		for _, p := range photos {
			if err := os.Remove(p.Path); err != nil {
				t.Fatal(err)
			}
		}
		// Крупный кадр: превью 2048 ещё не готовилось, оригинала нет — лист серый, ответ 200.
		if got := seatColors(lookJPEG(t, a, base+"?v=1&i=1&px=1600&r=2&ph=5"), stepXs, 0.5); got != "--" {
			t.Errorf("оригиналов нет: %q, ждали серые места", got)
		}
	})
}

// phHeapLimit — предел роста кучи на кадр с 7 фото: замер G3 × 2 (пик 57–101 МБ на 12 ядрах).
const phHeapLimit = 180 << 20

// Цена кадра с фото (163-24; замер прототипа: 7 фото из превью 2048, кадр 1600 px — 86 мс на 12
// ядрах): просмотр шага — лучший из 3 не дольше 400 мс; куча на кадр — не больше двух замеренных.
func TestServeLookOnPhotosCost(t *testing.T) {
	testkit.CostTest(t)
	if testkit.Lite() {
		t.Skip("готовка семи превью 2048 — секунды, в быстром уровне не гоняем")
	}
	a, _, _ := phApp(t, 3000, 2000, []string{"#ff0000", "#00ff00", "#0000ff", "#ffff00", "#00ffff", "#ff00ff", "#808080"})
	l := photoLook()
	l.Steps[0].Tpl.Places = []model.Box{{X: 0.02, Y: 0.05, W: 0.3, H: 0.4}, {X: 0.35, Y: 0.05, W: 0.3, H: 0.4}, {X: 0.68, Y: 0.05, W: 0.3, H: 0.4},
		{X: 0.02, Y: 0.5, W: 0.3, H: 0.4}, {X: 0.35, Y: 0.5, W: 0.3, H: 0.4}, {X: 0.68, Y: 0.5, W: 0.3, H: 0.4}, {X: 0.4, Y: 0.46, W: 0.2, H: 0.04}}
	url := "/media/look/" + saveCoverLook(t, a, "Семь мест", l) + "?v=1&i=1&r=1.6&ph=1&px="
	lookJPEG(t, a, url+"1600") // первый запрос готовит превью 2048 — это не мерим

	// Время — отдельно от замера кучи: ReadMemStats останавливает мир и замедлил бы кадр.
	n := 0
	frame := func() { n++; lookJPEG(t, a, url+strconv.Itoa(1600+n)) } // новый px — иной ключ ответа, копии из кэша
	best := testkit.Fastest(3, frame)
	if limit := testkit.Limit(400 * time.Millisecond); best > limit {
		t.Errorf("кадр с 7 фото: %v, предел %v", best, limit)
	}

	// Пик кучи сэмплирует горутина, пока рисуется кадр.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	var peak uint64
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		var m runtime.MemStats
		for {
			select {
			case <-stop:
				return
			case <-time.After(time.Millisecond):
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			}
		}
	})
	frame()
	close(stop)
	wg.Wait()
	grow := int64(peak) - int64(before.HeapAlloc)
	t.Logf("кадр с 7 фото: %v, рост кучи ≈ %d МБ", best, grow>>20)
	if grow > phHeapLimit {
		t.Errorf("куча выросла на %d МБ, предел %d МБ", grow>>20, int64(phHeapLimit)>>20)
	}
}
