package contactsheet

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// region — копия пикселей прямоугольника листа.
func region(im *image.RGBA, r image.Rectangle) []byte {
	var out []byte
	for y := r.Min.Y; y < r.Max.Y; y++ {
		out = append(out, im.Pix[im.PixOffset(r.Min.X, y):im.PixOffset(r.Max.X, y)]...)
	}
	return out
}

// differs — есть ли в прямоугольнике пиксели не цвета c.
func differs(im *image.RGBA, r image.Rectangle, c color.RGBA) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if im.RGBAAt(x, y) != c {
				return true
			}
		}
	}
	return false
}

// Заглушки: «…» и «×» различимы между собой и от фона, номер остаётся, соседние клетки целы,
// «готово» без картинки, пустая и огромная картинка превращаются в «×», а не в панику.
func TestSheetPlaceholders(t *testing.T) {
	photo := color.RGBA{90, 140, 200, 255}
	const grid, side, cell = 3, 768, 256
	rect := func(i int) image.Rectangle {
		x, y := i%grid*cell, i/grid*cell
		return image.Rect(x, y, x+cell, y+cell)
	}
	cells := []Cell{
		{Img: solid(300, 200, photo), N: 1},
		{State: Pending, N: 2},
		{State: Broken, N: 3},
		{State: Ready, N: 4},                                        // Img == nil
		{State: Ready, Img: image.NewRGBA(image.Rectangle{}), N: 5}, // пустые границы
		{State: Ready, Img: &image.Uniform{C: photo}, N: 6},         // бесконечные границы
		{State: Pending, Img: solid(300, 200, photo), N: 7},         // готовая картинка при Pending не рисуется
		{State: State(99), N: 8},                                    // неизвестное состояние — как битое
	}
	sheet := Compose(cells, grid, side)

	// Знак «…» и «×» — разные картинки: сравниваем середину клетки, мимо плашки с номером.
	if string(region(sheet, rect(1).Inset(pad+24))) == string(region(sheet, rect(2).Inset(pad+24))) {
		t.Error("«…» и «×» неотличимы")
	}
	for i, name := range map[int]string{1: "pending", 2: "broken"} {
		inner := rect(i).Inset(pad + 24) // мимо плашки с номером
		if !differs(sheet, inner, bgColor) {
			t.Errorf("%s: клетка не отличается от фона", name)
		}
	}
	// Знак стоит по центру клетки: центральная строка знака «×» не фоновая и не подложка целиком.
	c := rect(2)
	mid := image.Pt((c.Min.X+c.Max.X)/2, (c.Min.Y+c.Max.Y)/2)
	if sheet.RGBAAt(mid.X, mid.Y) != signColor {
		t.Errorf("центр «×» = %v, ждали цвет знака %v", sheet.RGBAAt(mid.X, mid.Y), signColor)
	}
	if sheet.RGBAAt(c.Min.X+pad+60, c.Min.Y+pad+100) == photo {
		t.Error("в битой клетке остался цвет фото")
	}
	// Остальные «битые» входы дают то же, что явное Broken (с учётом разного номера: сравниваем без плашки).
	body := func(i int) []byte { return region(sheet, rect(i).Inset(pad+24)) }
	for _, i := range []int{3, 4, 5, 7} {
		if string(body(i)) != string(body(2)) {
			t.Errorf("клетка %d (N=%d) не стала «×»", i, cells[i].N)
		}
	}
	// Pending с готовой картинкой — «…», картинка не рисуется.
	if string(body(6)) != string(body(1)) {
		t.Error("Pending с картинкой нарисовал картинку")
	}
	// Клетки вне списка — фон; у первой фото на месте.
	wantColor(t, sheet, rect(0).Min.X+cell/2, rect(0).Min.Y+cell/2, photo, "готовая клетка рядом с заглушками")
	wantColor(t, sheet, rect(8).Min.X+cell/2, rect(8).Min.Y+cell/2, bgColor, "пустая клетка")
	// Номера на заглушках на месте.
	for i := 1; i < 8; i++ {
		wantColor(t, sheet, rect(i).Min.X+pad, rect(i).Min.Y+pad, color.RGBA{0, 0, 0, 255}, "плашка номера заглушки")
	}
}

// Предел стороны исходной картинки клетки (maxSrc = 8192): 8192×1 рисуется полоской по центру клетки,
// 8193×1 — заглушка «×», как у явного Broken.
func TestSheetSourceLimit(t *testing.T) {
	photo := color.RGBA{90, 140, 200, 255}
	const grid, side, cell = 3, 768, 256
	sheet := Compose([]Cell{
		{Img: solid(8192, 1, photo), N: 1},
		{Img: solid(8193, 1, photo), N: 2},
		{State: Broken, N: 3},
	}, grid, side)
	rect := func(i int) image.Rectangle {
		x, y := i%grid*cell, i/grid*cell
		return image.Rect(x, y, x+cell, y+cell)
	}
	// Полоска высотой 1 px по центру клетки: ищем её цвет по строкам клетки.
	line := 0
	for y := rect(0).Min.Y; y < rect(0).Max.Y; y++ {
		if sheet.RGBAAt(rect(0).Min.X+cell/2, y) == photo {
			line++
		}
	}
	if line != 1 {
		t.Errorf("8192×1: строк цвета фото в клетке %d, ждали 1", line)
	}
	body := func(i int) []byte { return region(sheet, rect(i).Inset(pad+24)) }
	if string(body(1)) != string(body(2)) {
		t.Error("8193×1 не стала «×»")
	}
	if string(body(0)) == string(body(2)) {
		t.Error("8192×1 нарисовала заглушку")
	}
}

// Заглушки в самой малой клетке и в целом листе из одних заглушек — без паники, JPEG собирается.
func TestSheetPlaceholdersAll(t *testing.T) {
	for _, st := range []State{Pending, Broken} {
		cells := make([]Cell, 64)
		for i := range cells {
			cells[i] = Cell{State: st, N: i + 1}
		}
		jpg, w, h, err := Render(cells, 8, 512)
		if err != nil || w != 512 || h != 512 {
			t.Fatalf("%v: %v, %d×%d", st, err, w, h)
		}
		if _, err := jpeg.Decode(bytes.NewReader(jpg)); err != nil {
			t.Fatalf("%v: JPEG не читается: %v", st, err)
		}
	}
	if Pending.String() != "pending" || Broken.String() != "broken" || Ready.String() != "ok" || State(9).String() != "ok" {
		t.Error("имена состояний")
	}
}

// Метаданные: исходное фото с EXIF (здесь — GPS в APP1) на лист не попадает, в JPEG листа нет ни одного
// сегмента APPn кроме отсутствующих, ни комментария; строк Exif и XMP в байтах нет.
func TestSheetNoMeta(t *testing.T) {
	// Исходник с метаданными: JPEG плюс APP1 "Exif" с секретом сразу после SOI; затем он декодируется.
	src := bytes.Buffer{}
	if err := jpeg.Encode(&src, noisy(64, 48, 7), nil); err != nil {
		t.Fatal(err)
	}
	secret := []byte("Exif\x00\x00SECRET-GPS-55.75N-37.61E")
	app1 := append([]byte{0xFF, 0xE1, byte((len(secret) + 2) >> 8), byte(len(secret) + 2)}, secret...)
	withMeta := append(append([]byte{0xFF, 0xD8}, app1...), src.Bytes()[2:]...)
	if !bytes.Contains(withMeta, []byte("SECRET-GPS")) {
		t.Fatal("тест не вшил метаданные")
	}
	img, err := jpeg.Decode(bytes.NewReader(withMeta))
	if err != nil {
		t.Fatal(err)
	}
	jpg, _, _, err := Render([]Cell{{Img: img, N: 1}, {State: Pending, N: 2}, {State: Broken, N: 3}}, 3, 768)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"SECRET", "Exif", "http://ns.adobe.com", "xmp", "ICC_PROFILE", "JFIF"} {
		if bytes.Contains(jpg, []byte(bad)) {
			t.Errorf("в листе нашлось %q", bad)
		}
	}
	// Обход сегментов до SOS: допустимы только SOI, DQT, SOF0, DHT, SOS; APPn и COM — нет.
	if jpg[0] != 0xFF || jpg[1] != 0xD8 {
		t.Fatal("нет SOI")
	}
	for i := 2; i+3 < len(jpg); {
		if jpg[i] != 0xFF {
			t.Fatalf("сегмент на смещении %d: %#x", i, jpg[i])
		}
		m := jpg[i+1]
		if m >= 0xE0 && m <= 0xEF || m == 0xFE {
			t.Fatalf("в листе сегмент метаданных %#x на смещении %d", m, i)
		}
		if m == 0xDA { // SOS: дальше сжатые данные
			break
		}
		i += 2 + int(jpg[i+2])<<8 + int(jpg[i+3])
	}
}

// Предел размера: худший случай (шум 8 × 8 на 1568) укладывается в 600 КБ уменьшением качества, а если
// нет — стороны; гладкий лист остаётся полной стороны; невозможный бюджет — ErrBudget, не паника.
func TestSheetBudget(t *testing.T) {
	cells := make([]Cell, 64)
	for i := range cells {
		cells[i] = Cell{Img: noisy(400, 300, uint64(i+1)), N: i + 1}
	}
	start := time.Now()
	jpg, w, h, err := Render(cells, 8, 1568)
	if err != nil {
		t.Fatal(err)
	}
	if len(jpg) > MaxJPEG {
		t.Fatalf("лист %d байт, предел %d", len(jpg), MaxJPEG)
	}
	if w > MaxSide || h > MaxSide || w != h || w < 8*minCell {
		t.Fatalf("размер %d×%d", w, h)
	}
	if d := time.Since(start); d > testkit.Limit(10*time.Second) {
		t.Errorf("шумовой лист собирался %v", d)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpg))
	if err != nil || cfg.Width != w || cfg.Height != h {
		t.Fatalf("заявленный размер %d×%d, JPEG %+v %v", w, h, cfg, err)
	}

	// Гладкий лист: сторона не уменьшается.
	smooth := make([]Cell, 36)
	for i := range smooth {
		smooth[i] = Cell{Img: solid(400, 300, color.RGBA{uint8(i * 6), 120, 200, 255}), N: i + 1}
	}
	jpg, w, _, err = Render(smooth, 6, 1056)
	if err != nil || w != 1056 || len(jpg) > MaxJPEG {
		t.Fatalf("гладкий лист: %v, w=%d, %d байт", err, w, len(jpg))
	}

	// Тесный предел: сторона уменьшается, лист укладывается и остаётся читаемым JPEG.
	jpg, w, h, err = render(cells, 8, 1568, 200<<10)
	if err != nil || len(jpg) > 200<<10 || w >= 1568 || w != h || w < 8*minCell {
		t.Fatalf("тесный предел: %v, %d байт, %d×%d", err, len(jpg), w, h)
	}
	if _, err := jpeg.Decode(bytes.NewReader(jpg)); err != nil {
		t.Fatal(err)
	}
	// Предел недостижим даже на наименьшей стороне — ошибка, а не бесконечный цикл.
	if _, _, _, err := render(cells, 8, 1568, 1<<10); !errors.Is(err, ErrBudget) {
		t.Fatalf("недостижимый предел: %v", err)
	}

	// Невозможный бюджет.
	for _, budget := range []int{0, -1, 10} {
		if _, err := Encode(Compose(smooth, 6, 1056), budget); !errors.Is(err, ErrBudget) {
			t.Errorf("бюджет %d: %v", budget, err)
		}
	}
}
