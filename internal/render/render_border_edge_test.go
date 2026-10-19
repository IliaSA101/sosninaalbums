package render

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Края и отказы рамочек в рендере (079 G4): нечисловая и вынесенная за холст геометрия не рисуется
// и не читает файл, битый и пропавший файл — ошибка с именем без каталога, пределы буфера и маски,
// кэши листа (вытеснение, одна сборка на гонку, запомненная ошибка).

// pageBare — страница без рамочек: эталон для «ничего не нарисовано».
func pageBare(t *testing.T) *image.RGBA {
	t.Helper()
	return renderPat(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}})
}

// Рамочки, которым нечего рисовать, не дают ни ошибки, ни пикселя — и не открывают файл
// (пути заведомо нет: прочитай его — вышла бы ошибка).
func TestRenderBorderNothingToDraw(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "нет.png")
	ok := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	nan, inf := math.NaN(), math.Inf(1)
	cases := []struct {
		name string
		b    BorderDraw
	}{
		{"встроенная: NaN в рамке", BorderDraw{Box: layout.Rect{X: nan, Y: 10, W: 40, H: 30}, Kind: "line", Width: 2}},
		{"встроенная: Inf в ширине", BorderDraw{Box: layout.Rect{X: 20, Y: 10, W: inf, H: 30}, Kind: "line", Width: 2}},
		{"встроенная: NaN в повороте", BorderDraw{Box: ok, Rot: nan, Kind: "line", Width: 2}},
		{"встроенная: нулевая ширина", BorderDraw{Box: layout.Rect{X: 20, Y: 10, W: 0, H: 30}, Kind: "line", Width: 2}},
		{"встроенная: отрицательная высота", BorderDraw{Box: layout.Rect{X: 20, Y: 10, W: 40, H: -5}, Kind: "line", Width: 2}},
		{"встроенная: за холстом", BorderDraw{Box: layout.Rect{X: 500, Y: 400, W: 40, H: 30}, Kind: "line", Width: 2}},
		{"встроенная повёрнутая: за холстом", BorderDraw{Box: layout.Rect{X: -300, Y: -300, W: 40, H: 30}, Rot: 30, Kind: "line", Width: 2}},
		{"картинка: NaN в рамке", BorderDraw{Box: layout.Rect{X: nan, Y: 10, W: 40, H: 30}, Pic: true, Path: gone}},
		{"картинка: Inf в повороте", BorderDraw{Box: ok, Rot: inf, Pic: true, Path: gone}},
		{"картинка: нулевая высота", BorderDraw{Box: layout.Rect{X: 20, Y: 10, W: 40, H: 0}, Pic: true, Path: gone}},
		{"картинка: за холстом, прямая", BorderDraw{Box: layout.Rect{X: 500, Y: 400, W: 40, H: 30}, Pic: true, Path: gone}},
	}
	want := pageBare(t)
	for _, c := range cases {
		img, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Borders: []BorderDraw{c.b}}, patOpt)
		if err != nil {
			t.Errorf("%s: ошибка %v", c.name, err)
			continue
		}
		if !samePix(img, want) {
			t.Errorf("%s: что-то нарисовано", c.name)
		}
	}
	// Несколько рамочек разом: годные рисуются, негодные рядом не мешают.
	frame := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 255})
	cut := [4]float64{20.0 / 120, 20.0 / 160, 20.0 / 120, 20.0 / 160}
	s := Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Borders: []BorderDraw{
		{Box: layout.Rect{X: nan, Y: 10, W: 40, H: 30}, Pic: true, Path: gone},
		{Box: layout.Rect{X: 20, Y: 10, W: 40, H: 30}, Pic: true, Path: frame, Cut: cut},
		{Box: layout.Rect{X: -300, Y: -300, W: 40, H: 30}, Rot: 30, Pic: true, Path: frame, Cut: cut}, // повёрнутая за холстом
	}}
	img := renderPat(t, s)
	wantExact(t, "годная рамочка среди негодных", img, 254, 21.5, 25, color.RGBA{20, 140, 40, 255})
}

// Отказы: пропавший, битый, обрезанный файл, каталог вместо файла, слишком большой буфер — ошибка
// с именем файла и без каталога; ошибка рамочки останавливает лист.
func TestRenderBorderFailures(t *testing.T) {
	dir := testkit.TempDir(t)
	good := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 255})
	data, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	text := filepath.Join(dir, "текст.png")
	cut := filepath.Join(dir, "обрезан.png")
	if err := os.WriteFile(text, []byte("это не картинка"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cut, data[:len(data)/2], 0o600); err != nil { // заголовок цел, пиксели обрезаны
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "каталог.png")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	for _, c := range []struct {
		name string
		b    BorderDraw
		want string // часть текста ошибки
		file string // имя файла, которое должно в нём быть ("" — не проверяем)
	}{
		{"пропавший", BorderDraw{Box: box, Pic: true, Path: filepath.Join(dir, "пропал.png")}, "", "пропал.png"},
		{"не картинка", BorderDraw{Box: box, Pic: true, Path: text}, "", "текст.png"},
		{"обрезан", BorderDraw{Box: box, Pic: true, Path: cut}, "", "обрезан.png"},
		{"каталог", BorderDraw{Box: box, Pic: true, Path: sub}, "не обычный файл", "каталог.png"},
		{"повёрнутая: буфер больше холста печати", BorderDraw{Box: layout.Rect{X: 0, Y: 0, W: 2100, H: 2100}, Rot: 30, Pic: true, Path: good},
			"слишком велика для печати (21000×21000 px): уменьшите DPI", "ring.png"},
	} {
		img, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Borders: []BorderDraw{c.b}}, patOpt)
		if err == nil || img != nil {
			t.Errorf("%s: ошибки нет (%v)", c.name, err)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, c.file) || strings.Contains(msg, dir) || !strings.Contains(msg, c.want) {
			t.Errorf("%s: %q", c.name, msg)
		}
	}
	// Ошибка одной рамочки из нескольких мест останавливает лист, а не теряется среди удачных.
	_, err = Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Borders: []BorderDraw{
		{Box: box, Pic: true, Path: good, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}},
		{Box: box, Pic: true, Path: text},
	}}, patOpt)
	if err == nil || !strings.Contains(err.Error(), "текст.png") {
		t.Errorf("среди удачных: %v", err)
	}
}

// Рамочка-JPEG (не NRGBA: декодер отдаёт YCbCr) копируется в NRGBA и рисуется; поворот — тоже.
func TestRenderBorderJPEGPicture(t *testing.T) {
	frame := testkit.JPEG(t, testkit.TempDir(t), "fr.jpg", 120, 160)
	box := layout.Rect{X: 30, Y: 30, W: 40, H: 30}
	bare := renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}})
	for _, rot := range []float64{0, 30} {
		img := renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Borders: []BorderDraw{{Box: box, Rot: rot, Pic: true, Path: frame, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}}})
		if samePix(img, bare) {
			t.Errorf("поворот %v: JPEG-рамочка ничего не нарисовала", rot)
		}
		x, y := turnPt(box, rot, 20, 15)
		if c := img.RGBAAt(px(x, 254), px(y, 254)); near(c, bdGray, 4) {
			t.Errorf("поворот %v: середина места осталась цвета страницы %v", rot, c)
		}
	}
}

// Маска окна фото: неподходящая — не ошибка и не меняет фото (вид без маски, нечисловая рамка);
// непосильная (больше предела) и с пропавшей картинкой — ошибка с именем фото/файла.
func TestRenderPhotoWinEdges(t *testing.T) {
	dir := testkit.TempDir(t)
	photo := solidPNG(t, 40, 30, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	box := layout.Rect{X: 20, Y: 10, W: 40, H: 30}
	mk := func(w *WinMask, rot float64) Spread {
		return Spread{Bg: [2]color.RGBA{bdGray, bdGray},
			Photos: []Photo{{Cell: box, Img: box, Path: photo, Rot: rot, Pivot: box, Win: w}}}
	}
	for _, rot := range []float64{0, 90, 30} {
		want := renderBig(t, mk(nil, rot))
		for name, w := range map[string]*WinMask{
			"вид без маски":   {Box: box, Rot: rot, Kind: "line", Width: 2},
			"неизвестный вид": {Box: box, Rot: rot, Kind: "zz-future"},
			"NaN в рамке":     {Box: layout.Rect{X: math.NaN(), W: 40, H: 30}, Rot: rot, Kind: "soft", Width: 4},
			"пустая рамка":    {Box: layout.Rect{X: 20, Y: 10}, Rot: rot, Kind: "soft", Width: 4},
		} {
			if got := renderBig(t, mk(w, rot)); !samePix(got, want) {
				t.Errorf("поворот %v, %s: фото изменилось", rot, name)
			}
		}
	}
	// Мягкий край у нескольких фото разом на одном листе: каждое со своей маской.
	s := mk(&WinMask{Box: box, Kind: "soft", Width: 4}, 0)
	other := layout.Rect{X: 60, Y: 10, W: 30, H: 30}
	s.Photos = append(s.Photos, Photo{Cell: other, Img: other, Path: photo, Win: &WinMask{Box: other, Kind: "soft", Width: 4}})
	img := renderBig(t, s)
	for _, b := range []layout.Rect{box, other} {
		wantExact(t, "середина", img, 254, b.X+b.W/2, b.Y+b.H/2, brPhoto)
		if c := img.RGBAAt(px(b.X+0.15, 254), px(b.Y+b.H/2, 254)); !near(c, bdGray, 14) {
			t.Errorf("край %v не растворён: %v", b, c)
		}
	}

	for _, c := range []struct {
		name string
		w    *WinMask
		want string
	}{
		{"маска больше холста печати", &WinMask{Box: layout.Rect{W: 2100, H: 2100}, Kind: "soft", Width: 4}, "слишком велика для печати (21000×21000 px): уменьшите DPI"},
		{"картинка окна пропала", &WinMask{Box: box, Path: filepath.Join(dir, "нет.png")}, "нет.png"},
	} {
		_, err := Render(mk(c.w, 0), patOpt)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), dir) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
	// Картинка без прозрачного окна (сплошная): окно пусто, фото под ней скрыто целиком, но не ошибка.
	solid := solidPNG(t, 120, 160, color.NRGBA{20, 140, 40, 255})
	got := renderBig(t, mk(&WinMask{Box: box, Path: solid, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}, 0))
	if !samePix(got, renderBig(t, Spread{Bg: [2]color.RGBA{bdGray, bdGray}})) {
		t.Error("сплошная картинка: фото должно быть закрыто маской целиком")
	}
}

// Размер картинки после масштаба режется по площади (borderArea): пропорция сохраняется.
func TestPicGeometryAreaCap(t *testing.T) {
	// Место 1000×1000 мм при 10 px/мм — 10000×10000 px, картинка 100×100 заполняет его целиком.
	g := picGeometry(layout.Rect{W: 1000, H: 1000}, 1, [4]float64{0.1, 0.1, 0.1, 0.1}, 100, 100, 10000, 10000)
	if a := g.tw * g.th; a > borderArea+borderArea/100 || a < borderArea-borderArea/100 {
		t.Errorf("площадь %d, ждали около %d", a, borderArea)
	}
	if g.tw != g.th {
		t.Errorf("пропорция потеряна: %d×%d", g.tw, g.th)
	}
	if g.sl != [4]int{g.tw / 10, g.th / 10, g.tw / 10, g.th / 10} {
		t.Errorf("толщины %v при %d×%d", g.sl, g.tw, g.th)
	}
	if g.dsl != [4]int{1000, 1000, 1000, 1000} { // на месте — k·срез, а не от уменьшенной картинки (079-40)
		t.Errorf("толщины на месте %v", g.dsl)
	}
	// Вытянутая: узкая сторона не пропадает.
	g = picGeometry(layout.Rect{W: 4000, H: 0.01}, 1, [4]float64{}, 4000, 1, 40000, 1)
	if g.tw < 1 || g.th < 1 {
		t.Errorf("размер %d×%d", g.tw, g.th)
	}
	// Малая — без изменений: 40×30 мм, картинка 120×160 → k=0,1875 мм/px, при 10 px/мм 22,5×30 px... по формуле.
	g = picGeometry(layout.Rect{W: 40, H: 30}, 1, [4]float64{}, 120, 160, 400, 300)
	if g.tw != 225 || g.th != 300 || g.dsl != g.sl {
		t.Errorf("малая: %d×%d, ждали 225×300; толщины %v / %v", g.tw, g.th, g.sl, g.dsl)
	}
}

// Кэши листа: масштаб — по ключу «отпечаток|размер», третий размер вытесняет самый старый;
// ошибка декода запоминается; параллельные запросы одного ключа собирают значение один раз.
func TestBorderEnvEviction(t *testing.T) {
	frame := ringPNG(t, 60, 80, 10, color.NRGBA{20, 140, 40, 255})
	var e borderEnv
	pic, err := e.picture(frame)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := e.scaled(pic, 10, 10)
	b, _ := e.scaled(pic, 20, 20)
	if a2, _ := e.scaled(pic, 10, 10); a2 != a {
		t.Error("запись в пределах ёмкости пересобрана")
	}
	_, _ = e.scaled(pic, 30, 30) // вытесняет самую старую (10×10)
	if a3, _ := e.scaled(pic, 10, 10); a3 == a {
		t.Error("вытесненная запись отдана из кэша")
	}
	if b2, _ := e.scaled(pic, 20, 20); b2 == b {
		t.Error("20×20 вытеснена повторной сборкой 10×10, но отдана из кэша")
	}

	// Битый файл: ошибка одна и та же при втором обращении (память об ошибке), а не новый декод.
	bad := filepath.Join(testkit.TempDir(t), "bad.png")
	if err := os.WriteFile(bad, []byte("не png"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, e1 := e.picture(bad)
	_, e2 := e.picture(bad)
	if e1 == nil || e1 != e2 {
		t.Errorf("ошибка не запомнена: %v / %v", e1, e2)
	}
	// Нет файла: stampOf отказывает до кэша.
	if _, err := e.picture(filepath.Join(filepath.Dir(bad), "нет.png")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("пропавший файл: %v", err)
	}

	// Одна сборка на ключ при гонке потоков (мест с одной рамочкой — десятки).
	var m memo[int]
	var mu sync.Mutex
	calls := 0
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = m.get("k", func() (int, error) { mu.Lock(); calls++; mu.Unlock(); return 7, nil })
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Errorf("сборок %d, ждали 1", calls)
	}
}

// Паника при подготовке рамочки и при укладке — ошибка с именем, а не падение процесса.
func TestBorderPanicsBecomeErrors(t *testing.T) {
	// Укладка: паникующая paint получает имя и вид «рамочки» (встроенная — по Kind, картинка — по файлу).
	for _, c := range []struct {
		b    BorderDraw
		name string
	}{
		{BorderDraw{Kind: "line"}, "line"},
		{BorderDraw{Pic: true, Path: filepath.Join("каталог", "fr.png")}, "fr.png"},
	} {
		it := item{border: &c.b}
		err := paintSafe(func(*image.RGBA) error { panic("сбой рисования") }, nil, it)
		if err == nil || !strings.Contains(err.Error(), c.name) || !strings.Contains(err.Error(), "рамочки") ||
			!strings.Contains(err.Error(), "сбой рисования") {
			t.Errorf("%s: %v", c.name, err)
		}
		err = paintSafe(func(*image.RGBA) error { return fmt.Errorf("отказ") }, nil, it)
		if err == nil || !strings.Contains(err.Error(), "рамочки") || !strings.Contains(err.Error(), "отказ") {
			t.Errorf("%s, ошибка paint: %v", c.name, err)
		}
	}
	if what(item{}) != "фото" || what(item{border: &BorderDraw{}}) != "рамочки" {
		t.Error("what: вид предмета назван неверно")
	}

	// Подделанная рамка ±1e300 мм переполняет проверку размера буфера повёрнутой рамочки, и
	// image.NewRGBA паникует — с 079-40 при укладке (буфер строится там); горутина без recover
	// уронила бы процесс.
	frame := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 255})
	_, err := Render(Spread{Bg: [2]color.RGBA{bdGray, bdGray}, Borders: []BorderDraw{
		{Box: layout.Rect{X: -1e300, W: 1e300, H: 30}, Rot: 30, Pic: true, Path: frame}}}, patOpt)
	if err == nil || !strings.Contains(err.Error(), "ring.png") || !strings.Contains(err.Error(), "сбой при рисовании рамочки") {
		t.Errorf("паника подготовки: %v", err)
	}
}

// Картинка рамочки больше 4096 px по стороне (079-40: загружена и в «Фоны», запись общая; собранный
// альбом) печатается, а не роняет лист: уменьшается при декоде так же, как при загрузке в
// «Рамочки». Лист — пиксель в пиксель как с готовой копией 4096 px: накладка прямая и повёрнутая,
// маска окна фото.
func TestRenderBorderShrinksBig(t *testing.T) {
	const w, h, rim = 4200, 300, 30
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < rim || y < rim || x >= w-rim || y >= h-rim {
				im.SetNRGBA(x, y, color.NRGBA{R: uint8(x % 251), G: uint8(y), B: 90, A: 255})
			}
		}
	}
	dir := testkit.TempDir(t)
	big := writePNG(t, filepath.Join(dir, "big.png"), im)
	small := writePNG(t, filepath.Join(dir, "small.png"), imaging.Resize(im, 4096, 293, imaging.Lanczos))
	photo := solidPNG(t, 60, 8, color.NRGBA{brPhoto.R, brPhoto.G, brPhoto.B, 255})
	cut := [4]float64{float64(rim) / w, float64(rim) / h, float64(rim) / w, float64(rim) / h}
	mk := func(path string) Spread {
		var s Spread
		s.Bg = [2]color.RGBA{bdGray, bdGray}
		for i, rot := range []float64{0, 12} {
			box := layout.Rect{X: 8, Y: 10 + 20*float64(i), W: 84, H: 6}
			s.Photos = append(s.Photos, Photo{Cell: box, Img: box, Path: photo, Rot: rot, Pivot: box,
				Win: &WinMask{Box: box, Rot: rot, Path: path, Cut: cut}})
			s.Borders = append(s.Borders, BorderDraw{Box: box, Rot: rot, Z: 1, Pic: true, Path: path, Cut: cut})
		}
		return s
	}
	got, err := Render(mk(big), patOpt)
	if err != nil {
		t.Fatalf("4200 px: %v", err)
	}
	if want := renderPat(t, mk(small)); !samePix(got, want) {
		t.Error("4200 px: лист не как с копией 4096 px")
	}
	if n := imgfile.DecodeBudget.Used(); n != 0 {
		t.Errorf("бюджет декода после листа: %d", n)
	}
}

// Место рамочки больше прежнего предела 100 Мп (079-40, Н-4: 300×300 мм при 900 dpi) печатается:
// предел — холст печати. Буфер повёрнутой рамочки не строится при подготовке (параллельно у трёх
// элементов), а при укладке — и ждёт места в DecodeBudget.
func TestBorderBufBudget(t *testing.T) {
	ring := ringPNG(t, 120, 160, 20, color.NRGBA{G: 140, A: 255})
	cut := [4]float64{1.0 / 6, 1.0 / 8, 1.0 / 6, 1.0 / 8}
	canvas := image.Rect(0, 0, 1000, 500)
	huge := BorderDraw{Box: layout.Rect{W: 1100, H: 1100}, Rot: 30, Pic: true, Path: ring, Cut: cut} // 11000² px при 254 dpi
	paint, err := prepareBorder(huge, nil, canvas, 254)
	if err != nil || paint == nil {
		t.Fatalf("121 Мп: %v, рисунок %v", err, paint != nil)
	}
	if n := imgfile.DecodeBudget.Used(); n != 0 {
		t.Fatalf("подготовка держит бюджет: %d", n)
	}

	paint, err = prepareBorder(BorderDraw{Box: layout.Rect{X: 10, Y: 10, W: 40, H: 30}, Rot: 20, Pic: true, Path: ring, Cut: cut}, nil, canvas, 254)
	if err != nil || paint == nil {
		t.Fatalf("малая: %v", err)
	}
	imgfile.DecodeBudget.Acquire(imgfile.DecodeBudgetPixels)
	done := make(chan error, 1)
	go func() { done <- paint(image.NewRGBA(canvas)) }()
	select {
	case <-done:
		t.Error("буфер поворота строится, когда бюджет занят целиком")
	case <-time.After(50 * time.Millisecond):
	}
	imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := imgfile.DecodeBudget.Used(); n != 0 {
		t.Errorf("бюджет после укладки: %d", n)
	}
}
