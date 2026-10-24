package sheetcut

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plain — параметры тестов без сетки и без нормализации альфы.
func plain() Options {
	o := DefaultOptions()
	o.BG = BGNone
	o.Dense = 0
	return o
}

func hasWarn(s Sticker, sub string) bool {
	for _, w := range s.Warnings {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}

func TestGridGluesFragmentsOfOneCell(t *testing.T) {
	img := gridSheet(3, 2, 300)
	// Два «пузырька» в первой ячейке, дальше glue (80 px) от главного квадрата 130…170.
	fillRect(img, image.Rect(10, 10, 22, 22), navy)
	fillRect(img, image.Rect(280, 270, 292, 282), navy)
	o := plain()
	o.Cols, o.Rows = 3, 2
	res := mustCut(t, img, o)
	if len(res.Stickers) != 6 {
		t.Fatalf("стикеров %d, нужно 6 (по ячейкам)", len(res.Stickers))
	}
	if res.Stickers[0].Pieces != 3 {
		t.Fatalf("в первой ячейке кусков %d, нужно 3", res.Stickers[0].Pieces)
	}
	for _, s := range res.Stickers[1:] {
		if s.Pieces != 1 {
			t.Fatalf("стикер %d: кусков %d, нужно 1", s.Index, s.Pieces)
		}
	}
	// Рамка первого — от левого пузырька до правого: 10…292 по X, 10…282 по Y.
	if got := res.Stickers[0].Rect; got != [4]int{10, 10, 292, 282} {
		t.Fatalf("рамка первого %v", got)
	}
	if res.Stickers[0].W != 282 || res.Stickers[0].H != 272 {
		t.Fatalf("размер первого %d×%d", res.Stickers[0].W, res.Stickers[0].H)
	}
}

func TestGridOrderCellsAndNames(t *testing.T) {
	o := plain()
	o.Cols, o.Rows = 3, 2
	res := mustCut(t, gridSheet(3, 2, 200), o)
	for i, s := range res.Stickers {
		if s.Index != i+1 || s.Cell == nil || s.Cell[0] != i%3 || s.Cell[1] != i/3 {
			t.Fatalf("стикер %d: index %d, ячейка %v", i, s.Index, s.Cell)
		}
	}
	if StickerName(1) != "01.png" || StickerName(12) != "12.png" || StickerName(100) != "100.png" {
		t.Fatalf("имена: %s %s %s", StickerName(1), StickerName(12), StickerName(100))
	}
}

func TestGridEmptyCellAndDebrisOnlyCell(t *testing.T) {
	img := gridSheet(3, 1, 200)
	fillRect(img, image.Rect(280, 80, 320, 120), transp) // средний квадрат убран
	fillRect(img, image.Rect(300, 10, 303, 13), navy)    // в его ячейке осталась крупинка 3×3
	o := plain()
	o.Cols, o.Rows = 3, 1
	o.MinArea = 50
	res := mustCut(t, img, o)
	if len(res.Stickers) != 2 {
		t.Fatalf("стикеров %d, нужно 2: крупинка меньше minarea — мусор", len(res.Stickers))
	}
	if res.Noise != 1 || len(res.Empty) != 1 || res.Empty[0] != [2]int{1, 0} || len(res.Notes) != 1 {
		t.Fatalf("мусор %d, пустые %v, замечания %v", res.Noise, res.Empty, res.Notes)
	}
	// Нумерация без дыр: второй стикер — из третьей ячейки.
	if res.Stickers[1].Index != 2 || res.Stickers[1].Cell[0] != 2 {
		t.Fatalf("второй: %+v", res.Stickers[1])
	}
}

func TestGlueByDistanceBoundary(t *testing.T) {
	for _, tc := range []struct {
		gap    int
		pieces int
		unglue int
	}{
		{gap: 30, pieces: 2, unglue: 0}, // ровно glue — клеится
		{gap: 31, pieces: 1, unglue: 1}, // на пиксель дальше — отброшен
	} {
		img := canvas(600, 200, transp)
		fillRect(img, image.Rect(50, 50, 150, 150), navy)                // крупный: 100×100
		fillRect(img, image.Rect(150+tc.gap, 90, 160+tc.gap, 100), navy) // мелкий 10×10
		fillRect(img, image.Rect(400, 50, 500, 150), navy)               // второй крупный
		o := plain()
		o.Glue = 30
		res := mustCut(t, img, o)
		if len(res.Stickers) != 2 || res.Stickers[0].Pieces != tc.pieces || res.Unglued != tc.unglue {
			t.Fatalf("gap %d: стикеров %d, кусков %d, без пары %d", tc.gap, len(res.Stickers), res.Stickers[0].Pieces, res.Unglued)
		}
	}
}

func TestGlueSmallGoesToNearestLarge(t *testing.T) {
	img := canvas(600, 200, transp)
	fillRect(img, image.Rect(50, 50, 150, 150), navy)
	fillRect(img, image.Rect(300, 50, 400, 150), navy)
	fillRect(img, image.Rect(270, 90, 280, 100), navy) // до второго 20 px, до первого 120
	o := plain()
	o.Glue = 150
	res := mustCut(t, img, o)
	if len(res.Stickers) != 2 || res.Stickers[0].Pieces != 1 || res.Stickers[1].Pieces != 2 {
		t.Fatalf("куски %d / %d", res.Stickers[0].Pieces, res.Stickers[1].Pieces)
	}
}

func TestRowOrderWithoutGrid(t *testing.T) {
	img := canvas(700, 400, transp)
	// Верхний ряд с разбросом по Y, нижний — тоже; рисуем в «перепутанном» порядке.
	boxes := []image.Rectangle{
		image.Rect(500, 120, 580, 190), image.Rect(40, 60, 120, 130), image.Rect(300, 100, 380, 170), // верх
		image.Rect(520, 300, 600, 370), image.Rect(60, 280, 140, 350), image.Rect(280, 310, 360, 380), // низ
	}
	for _, r := range boxes {
		fillRect(img, r, navy)
	}
	res := mustCut(t, img, plain())
	want := []int{40, 300, 500, 60, 280, 520} // Min.X в порядке «слева направо, сверху вниз»
	if len(res.Stickers) != 6 {
		t.Fatalf("стикеров %d", len(res.Stickers))
	}
	for i, s := range res.Stickers {
		if s.Rect[0] != want[i] || s.Cell != nil {
			t.Fatalf("позиция %d: x0=%d, нужно %d", i, s.Rect[0], want[i])
		}
	}
}

func TestDenseKeepsSoftEdge(t *testing.T) {
	build := func() *image.NRGBA {
		img := canvas(100, 100, transp)
		body := navy
		body.A = 230
		fillRect(img, image.Rect(30, 30, 70, 70), body)
		edge := navy
		edge.A = 100 // мягкий край: выше -alpha, но ниже -dense
		fillRect(img, image.Rect(70, 30, 71, 70), edge)
		return img
	}
	o := plain()
	o.Dense = 224
	got := mustCut(t, build(), o).Stickers[0].Img
	if a := alphaAt(got, 5, 5); a != 255 {
		t.Fatalf("тело: альфа %d, нужно 255 (уплотнено)", a)
	}
	if a := alphaAt(got, got.Bounds().Dx()-1, 5); a != 100 {
		t.Fatalf("мягкий край: альфа %d, нужно 100 (как было)", a)
	}
	o.Dense = 0
	got = mustCut(t, build(), o).Stickers[0].Img
	if a := alphaAt(got, 5, 5); a != 230 {
		t.Fatalf("без уплотнения альфа %d, нужно 230", a)
	}
}

func TestPadAndSoftHaloTrim(t *testing.T) {
	img := canvas(300, 200, transp)
	fillRect(img, image.Rect(100, 80, 140, 120), navy) // 40×40
	halo := navy
	halo.A = 10                                        // ниже порога альфы: ореол
	fillRect(img, image.Rect(140, 80, 142, 120), halo) // рядом (до 3 px) — остаётся
	fillRect(img, image.Rect(160, 80, 170, 120), halo) // далеко — вырезается
	o := plain()
	o.Pad = 7
	s := mustCut(t, img, o).Stickers[0]
	if s.W != 42+14 || s.H != 40+14 {
		t.Fatalf("размер %d×%d, нужно 56×54 (40+2 мягкого края, поле 7)", s.W, s.H)
	}
	if alphaAt(s.Img, 0, 0) != 0 || alphaAt(s.Img, 7, 7) != 255 || alphaAt(s.Img, 7+40, 20) != 10 {
		t.Fatalf("поле/тело/ореол: %d %d %d", alphaAt(s.Img, 0, 0), alphaAt(s.Img, 7, 7), alphaAt(s.Img, 47, 20))
	}
}

func TestClumpWarnings(t *testing.T) {
	img := canvas(800, 200, transp)
	fillRect(img, image.Rect(80, 80, 120, 120), navy)  // ячейка 0
	fillRect(img, image.Rect(680, 80, 720, 120), navy) // ячейка 3
	fillRect(img, image.Rect(290, 90, 510, 130), navy) // широкая полоса: центр в ячейке 2, заходит в пустую 1
	o := plain()
	o.Cols, o.Rows = 4, 1
	res := mustCut(t, img, o)
	if len(res.Stickers) != 3 {
		t.Fatalf("стикеров %d", len(res.Stickers))
	}
	if hasWarn(res.Stickers[0], "слиплось") || hasWarn(res.Stickers[2], "слиплось") {
		t.Fatalf("обычные стикеры помечены: %v %v", res.Stickers[0].Warnings, res.Stickers[2].Warnings)
	}
	if !hasWarn(res.Stickers[1], "слиплось") {
		t.Fatalf("полоса не помечена: %v", res.Stickers[1].Warnings)
	}
}

func TestWideStickerInOccupiedNeighbourIsNotClump(t *testing.T) {
	img := gridSheet(3, 1, 200)
	fillRect(img, image.Rect(280, 80, 320, 120), transp)
	fillRect(img, image.Rect(180, 95, 420, 105), navy) // лента 240×10 шире ячейки, но обе соседние ячейки заняты
	o := plain()
	o.Cols, o.Rows = 3, 1
	res := mustCut(t, img, o)
	if len(res.Stickers) != 3 || hasWarn(res.Stickers[1], "слиплось") {
		t.Fatalf("стикеров %d, предупреждения %v", len(res.Stickers), res.Stickers[1].Warnings)
	}
}

func TestSizeWarnings(t *testing.T) {
	img := canvas(900, 900, transp)
	fillRect(img, image.Rect(100, 100, 140, 140), navy)
	small := mustCut(t, img, plain()).Stickers[0]
	if !hasWarn(small, "мало пикселей") || small.CmW != cm(40) {
		t.Fatalf("мелкий: %v, %.1f см", small.Warnings, small.CmW)
	}
	fillRect(img, image.Rect(100, 300, 800, 400), navy)
	o := plain()
	o.LargeFrac = 0.01
	res := mustCut(t, img, o)
	if len(res.Stickers) != 2 || hasWarn(res.Stickers[1], "мало пикселей") {
		t.Fatalf("700 px — не мало: %v", res.Stickers[1].Warnings)
	}
	if got := cm(300); got != 2.5 { // 300 px при 300 dpi = 1 дюйм = 2,54 см
		t.Fatalf("cm(300)=%.1f", got)
	}
}

func TestMaxSideScalesDown(t *testing.T) {
	img := canvas(5000, 300, transp)
	fillRect(img, image.Rect(0, 0, 5000, 300), navy)
	s := mustCut(t, img, plain()).Stickers[0]
	if !s.Scaled || s.W != 4096 || s.H != 245 || s.Img.Bounds().Dx() != 4096 {
		t.Fatalf("после уменьшения %d×%d, scaled %v", s.W, s.H, s.Scaled)
	}
	if got := s.Img.NRGBAAt(2000, 100); got != navy {
		t.Fatalf("цвет после уменьшения %v", got)
	}
	if !hasWarn(s, "уменьшен") {
		t.Fatal("нет предупреждения об уменьшении")
	}
}

func TestAutoModePicksByAlpha(t *testing.T) {
	if res := mustCut(t, gridSheet(2, 1, 200), DefaultOptions()); res.BG != BGNone || res.BGInfo != nil {
		t.Fatalf("лист с альфой: режим %q", res.BG)
	}
	img := canvas(300, 200, white)
	fillRect(img, image.Rect(100, 50, 200, 150), navy)
	if res := mustCut(t, img, DefaultOptions()); res.BG != BGOwn || res.BGInfo == nil || len(res.Stickers) != 1 {
		t.Fatalf("белый лист: режим %q, стикеров %d", res.BG, len(res.Stickers))
	}
}

func TestBadInput(t *testing.T) {
	o := plain()
	o.Cols = 3
	if _, err := Cut(gridSheet(1, 1, 100), o); err == nil {
		t.Fatal("сетка с одной стороной должна быть ошибкой")
	}
	o = plain()
	o.BG = "magic"
	if _, err := Cut(gridSheet(1, 1, 100), o); err == nil {
		t.Fatal("неизвестный режим фона должен быть ошибкой")
	}
	if _, err := Cut(image.NewNRGBA(image.Rect(0, 0, 0, 0)), plain()); err == nil {
		t.Fatal("пустая картинка должна быть ошибкой")
	}
	res := mustCut(t, canvas(50, 50, transp), plain()) // совсем пустой лист — не падение
	if len(res.Stickers) != 0 || res.Regions != 0 {
		t.Fatalf("пустой лист: %d стикеров", len(res.Stickers))
	}
	if _, err := res.JSON(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveRoundTripAndPreview(t *testing.T) {
	o := plain()
	o.Cols, o.Rows = 3, 2
	res := mustCut(t, gridSheet(3, 2, 200), o)
	dir := filepath.Join(tempDir(t), "out")
	if err := Save(res, dir); err != nil {
		t.Fatal(err)
	}
	if err := SavePreview(res, dir, 3); err != nil {
		t.Fatal(err)
	}
	for _, s := range res.Stickers {
		f, err := os.Open(filepath.Join(dir, StickerName(s.Index)))
		if err != nil {
			t.Fatal(err)
		}
		back, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !toNRGBA(back).Rect.Eq(s.Img.Rect) || string(toNRGBA(back).Pix) != string(s.Img.Pix) {
			t.Fatalf("%s: после записи и чтения картинка другая", StickerName(s.Index))
		}
		if s.Bytes == 0 || max(s.W, s.H) > MaxSidePx {
			t.Fatalf("%s: байт %d, размер %d×%d", StickerName(s.Index), s.Bytes, s.W, s.H)
		}
	}
	for name, want := range map[string][4]uint8{PreviewDark: {0x1e, 0x1e, 0x22, 255}, PreviewColor: {0x4a, 0x7a, 0x6e, 255}} {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		pv, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		c := toNRGBA(pv).NRGBAAt(0, 0)
		if pv.Bounds().Dx() != 3*(previewCell+2*previewGap) || pv.Bounds().Dy() != 2*(previewCell+2*previewGap) || [4]uint8{c.R, c.G, c.B, c.A} != want {
			t.Fatalf("%s: размер %v, угол %v", name, pv.Bounds(), c)
		}
	}
	data, err := res.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var back Result
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Stickers) != 6 || back.Stickers[4].W != res.Stickers[4].W || back.Stickers[4].Bytes != res.Stickers[4].Bytes || back.Sheet != res.Sheet {
		t.Fatalf("JSON после круга: %+v", back.Stickers)
	}
	if !strings.Contains(res.Text(), "01.png") || !strings.Contains(res.Text(), "см@300dpi") {
		t.Fatalf("текст отчёта: %s", res.Text())
	}
}

func TestContactSheetLayout(t *testing.T) {
	o := plain()
	res := mustCut(t, gridSheet(5, 1, 100), o)
	sheet := contactSheet(res.Stickers, 2, darkBG) // 5 стикеров в 2 колонки = 3 ряда
	cell := previewCell + 2*previewGap
	if sheet.Bounds().Dx() != 2*cell || sheet.Bounds().Dy() != 3*cell {
		t.Fatalf("размер %v", sheet.Bounds())
	}
	if c := sheet.NRGBAAt(cell/2, cell/2); c != navy { // стикер 40 px не увеличивается, лежит по центру ячейки
		t.Fatalf("центр первой ячейки %v", c)
	}
}

func TestParseGrid(t *testing.T) {
	cases := []struct {
		in         string
		cols, rows int
		bad        bool
	}{
		{"", 0, 0, false},
		{"3x4", 3, 4, false},
		{"3X4", 3, 4, false},
		{"1x1", 1, 1, false},
		{"12x1", 12, 1, false},
		{"0x4", 0, 0, true},
		{"3x0", 0, 0, true},
		{"-1x2", 0, 0, true},
		{"3", 0, 0, true},
		{"x", 0, 0, true},
		{"3x", 0, 0, true},
		{"ax4", 0, 0, true},
		{"3x4x5", 0, 0, true},
		{" 3x4", 0, 0, true},
	}
	for _, c := range cases {
		cols, rows, err := ParseGrid(c.in)
		if (err != nil) != c.bad || cols != c.cols || rows != c.rows {
			t.Errorf("ParseGrid(%q) = %d, %d, %v", c.in, cols, rows, err)
		}
	}
}

// Зазор между стикерами меньше мягкого края (softPx): соседний стикер не попадает в вырез.
func TestRenderDoesNotTakeNeighbourInsideSoftEdge(t *testing.T) {
	for _, gap := range []int{1, 2, 3, 6} {
		img := canvas(99+gap, 60, transp)
		fillRect(img, image.Rect(1, 5, 50, 54), navy)          // 49×49
		fillRect(img, image.Rect(50+gap, 5, 99+gap, 54), navy) // 49×49
		o := plain()
		o.Cols, o.Rows = 2, 1
		res := mustCut(t, img, o)
		if len(res.Stickers) != 2 {
			t.Fatalf("зазор %d: стикеров %d", gap, len(res.Stickers))
		}
		for _, s := range res.Stickers {
			n := 0
			for y := 0; y < s.H; y++ {
				for x := 0; x < s.W; x++ {
					if alphaAt(s.Img, x, y) != 0 {
						n++
					}
				}
			}
			if s.W != 49 || s.H != 49 || n != 49*49 {
				t.Fatalf("зазор %d, стикер %d: %d×%d, непрозрачных %d (нужно 49×49 и 2401)", gap, s.Index, s.W, s.H, n)
			}
		}
	}
}

// Мусор (область меньше MinArea) рядом со стикером тоже не попадает в вырез.
func TestRenderDropsNoiseNextToSticker(t *testing.T) {
	img := canvas(200, 100, transp)
	fillRect(img, image.Rect(50, 30, 100, 80), navy)
	fillRect(img, image.Rect(102, 30, 104, 32), navy) // 2×2, зазор 2 px
	o := plain()
	o.MinArea = 16
	res := mustCut(t, img, o)
	if len(res.Stickers) != 1 || res.Noise != 1 || res.Stickers[0].W != 50 {
		t.Fatalf("стикеров %d, мусора %d, ширина %d", len(res.Stickers), res.Noise, res.Stickers[0].W)
	}
}

// Glue 0 — не «по умолчанию»: клеятся только соприкасающиеся рамки; меньше нуля — умолчание 80.
func TestGlueZeroMeansTouchingOnly(t *testing.T) {
	img := canvas(200, 120, transp)
	fillRect(img, image.Rect(0, 0, 100, 100), navy)
	fillRect(img, image.Rect(10, 10, 90, 90), transp) // рамка-кольцо: внутри пустота
	fillRect(img, image.Rect(40, 40, 50, 50), navy)   // кусок внутри кольца: зазор рамок 0
	fillRect(img, image.Rect(101, 40, 111, 50), navy) // рядом, зазор рамок 1 px
	cases := []struct {
		glue            int
		pieces, unglued int
	}{
		{0, 2, 1},  // внутренний приклеен, внешний (зазор 1) отброшен
		{1, 3, 0},  // граница: ровно зазор — клеится
		{-1, 3, 0}, // меньше нуля — умолчание 80
	}
	for _, c := range cases {
		o := plain()
		o.Glue = c.glue
		res := mustCut(t, img, o)
		if len(res.Stickers) != 1 || res.Stickers[0].Pieces != c.pieces || res.Unglued != c.unglued {
			t.Fatalf("glue %d: стикеров %d, кусков %d, без пары %d", c.glue, len(res.Stickers), res.Stickers[0].Pieces, res.Unglued)
		}
	}
}

// Save удаляет только остатки прошлого прогона вида NN.png с номером больше числа стикеров.
func TestSaveRemovesOnlyStaleStickerFiles(t *testing.T) {
	dir := tempDir(t)
	keep := []string{"notes.txt", "1.png", "a05.png", "05.png.bak", PreviewDark}
	stale := []string{"03.png", "05.png", "12.png", "100.png"}
	all := append(append(append([]string{}, keep...), stale...), "01.png", "02.png")
	for _, n := range all {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "07.png"), 0o755); err != nil { // папка с таким именем — не трогать
		t.Fatal(err)
	}
	res := mustCut(t, gridSheet(2, 1, 100), plain())
	if err := Save(res, dir); err != nil {
		t.Fatal(err)
	}
	for _, n := range stale {
		if _, err := os.Stat(filepath.Join(dir, n)); !os.IsNotExist(err) {
			t.Errorf("%s должен быть удалён: %v", n, err)
		}
	}
	for _, n := range append(keep, "01.png", "02.png", "07.png") {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Errorf("%s должен остаться: %v", n, err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "01.png")); string(b) == "x" {
		t.Error("01.png не перезаписан")
	}
	if len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "03.png, 05.png, 100.png, 12.png") {
		t.Errorf("отчёт об удалённом: %v", res.Notes)
	}
	// Повторный прогон: остатков нет — замечаний нет.
	res2 := mustCut(t, gridSheet(2, 1, 100), plain())
	if err := Save(res2, dir); err != nil || len(res2.Notes) != 0 {
		t.Errorf("повтор: err %v, замечания %v", err, res2.Notes)
	}
}
