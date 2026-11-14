package packprep

import (
	"bytes"
	"image"
	"image/color"
	"path/filepath"
	"strings"
	"testing"
)

var (
	artBlue  = color.NRGBA{40, 90, 200, 255}
	stubRed  = color.NRGBA{220, 60, 90, 255}
	stubTeal = color.NRGBA{40, 170, 160, 255}
)

// stubCorner — уголок «сверху слева» w×h: пятно искусства и два образца линии, упирающиеся во
// внутренние края: горизонтальный (строки hy…hy+th, правый край) и вертикальный (столбцы vx…vx+th, нижний).
func stubCorner(w, h, hy, vx, th int, c color.NRGBA) *image.NRGBA {
	img := canvas(w, h, transp)
	fillRect(img, image.Rect(8, 8, 120, 110), artBlue)
	fillRect(img, image.Rect(120, hy, w, hy+th), c)
	fillRect(img, image.Rect(vx, 110, vx+th, h), c)
	return img
}

// fourCorners — набор: все четыре из одного tl зеркалами; tr другого размера и с образцом ниже,
// чтобы уголки пришлось сдвигать на линию стороны.
func fourCorners(c color.NRGBA) [4]*image.NRGBA {
	tl := stubCorner(300, 280, 30, 20, 6, c)
	tr := flipImg(stubCorner(260, 300, 44, 30, 6, c), true, false)
	bl := flipImg(stubCorner(280, 260, 36, 24, 6, c), false, true)
	br := flipImg(stubCorner(290, 270, 30, 20, 6, c), true, true)
	return [4]*image.NRGBA{tl, tr, bl, br}
}

// bboxAlpha — габарит непрозрачных px картинки в прямоугольнике r.
func bboxAlpha(img *image.NRGBA, r image.Rectangle) image.Rectangle {
	box := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if img.NRGBAAt(x, y).A > 0 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

// wantCornerAt — уголок лёг в свою угловую треть 1:1 (рамка без линии: в трети только он): все его
// px на холсте совпали байт в байт. Возвращает сдвиг уголка.
func wantCornerAt(t *testing.T, fr, c *image.NRGBA, quad int) image.Point {
	t.Helper()
	side := fr.Bounds().Dx()
	third := side / 3
	cell := image.Rect((quad%2)*(side-third), (quad/2)*(side-third), (quad%2)*(side-third)+third, (quad/2)*(side-third)+third)
	got := bboxAlpha(fr, cell)
	own := bboxAlpha(c, c.Bounds())
	if got.Dx() != own.Dx() || got.Dy() != own.Dy() {
		t.Fatalf("уголок %d: габарит на холсте %v, у самого уголка %v — масштаб или обрезка", quad, got, own)
	}
	checkCornerAt(t, fr, c, quad, got.Min.Sub(own.Min), cell)
	return got.Min.Sub(own.Min)
}

// checkCornerAt — уголок c лёг в cell со сдвигом off байт в байт.
func checkCornerAt(t *testing.T, fr, c *image.NRGBA, quad int, off image.Point, cell image.Rectangle) {
	t.Helper()
	for y := range c.Bounds().Dy() {
		for x := range c.Bounds().Dx() {
			if fr.NRGBAAt(off.X+x, off.Y+y) != c.NRGBAAt(x, y) {
				t.Fatalf("уголок %d: px (%d,%d) отличается от исходного", quad, x, y)
			}
		}
	}
	if !image.Rect(off.X, off.Y, off.X+c.Bounds().Dx(), off.Y+c.Bounds().Dy()).In(cell) {
		t.Fatalf("уголок %d вышел из своей трети %v", quad, cell)
	}
}

// Без линии: квадрат втрое больше крупнейшего уголка, уголки 1:1 в своих третях, всё остальное
// прозрачно, окно открыто (срез по третям), «тянется» ≈ 0.
func TestCornerFrameNone(t *testing.T) {
	cs := fourCorners(stubRed)
	cf, err := buildCornerFrame(cs, LineNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	b := cf.img.Bounds()
	if b.Dx() != b.Dy() || b.Dx()%3 != 0 || b.Dx() < 3*300 || b.Dx() > 3*340 {
		t.Fatalf("холст %v: ждали квадрат, кратный 3, от 900 до 1020", b)
	}
	if !cf.info.Window || !cf.info.Open || cf.info.Cut != [4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3} {
		t.Fatalf("срез: %+v", cf.info)
	}
	if cf.info.Stretch > 0.5 {
		t.Fatalf("тянется %.2f %%", cf.info.Stretch)
	}
	for q, c := range cs {
		wantCornerAt(t, cf.img, c, q)
	}
	// Центральная крестовина (стороны и окно) прозрачна: уголки её не занимают.
	third := b.Dx() / 3
	if box := bboxAlpha(cf.img, image.Rect(third, 0, 2*third, b.Dy())); !box.Empty() {
		t.Errorf("средняя колонка не пуста: %v", box)
	}
	if box := bboxAlpha(cf.img, image.Rect(0, third, b.Dx(), 2*third)); !box.Empty() {
		t.Errorf("средний ряд не пуст: %v", box)
	}
}

// Тонкая линия: цвет и толщина образца, просвет у концов, линия на одной оси у обеих сторон,
// центр прозрачен, окно открыто и на копии 1024 px.
func TestCornerFrameThin(t *testing.T) {
	cs := fourCorners(stubRed)
	cf, err := buildCornerFrame(cs, LineThin, nil)
	if err != nil {
		t.Fatal(err)
	}
	side := cf.img.Bounds().Dx()
	if !cf.info.Open || cf.info.Stretch > 5 {
		t.Fatalf("окно: %+v", cf.info)
	}
	gap := 2*((side+winAnalyzePx-1)/winAnalyzePx) + 2
	// Уголки стоят там же, где без линии (линия начинается за просветом, в прямоугольник уголка не заходит).
	bare, err := buildCornerFrame(cs, LineNone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bare.img.Bounds() != cf.img.Bounds() {
		t.Fatalf("холст с линией %v, без — %v", cf.img.Bounds(), bare.img.Bounds())
	}
	offs := [4]image.Point{}
	third := side / 3
	for q, c := range cs {
		offs[q] = wantCornerAt(t, bare.img, c, q)
		checkCornerAt(t, cf.img, c, q, offs[q], image.Rect((q%2)*(side-third), (q/2)*(side-third), (q%2)*(side-third)+third, (q/2)*(side-third)+third))
	}
	// Верхняя линия — на оси образца самого низкого из двух уголков: 30 у tl и 44 у tr → 44+3.
	topY := 44 + 3
	tlRight := offs[0].X + cs[0].Bounds().Dx()
	mid := side / 2
	if got := cf.img.NRGBAAt(mid, topY); got != stubRed {
		t.Errorf("верхняя линия посреди: %v, ждали цвет образца %v", got, stubRed)
	}
	if got := cf.img.NRGBAAt(mid, topY-4).A; got != 0 {
		t.Errorf("над линией (толщина 6, ось %d): альфа %d", topY, got)
	}
	if got := cf.img.NRGBAAt(tlRight+gap/2, topY).A; got != 0 {
		t.Errorf("просвет у уголка: альфа %d, ждали 0", got)
	}
	if got := cf.img.NRGBAAt(tlRight+gap+2, topY); got != stubRed {
		t.Errorf("линия сразу за просветом: %v", got)
	}
	// Внутри линий — только окно.
	in := image.Rect(side/3, side/3, 2*side/3, 2*side/3)
	if box := bboxAlpha(cf.img, in); !box.Empty() {
		t.Errorf("центр не прозрачен: %v", box)
	}
	// Остальные стороны — на осях своих образцов: право 33 от края (30 у tr и 20 у br, толщина 6),
	// низ 39 (36 у bl, 30 у br), лево 27 (20 у tl, 24 у bl).
	for name, p := range map[string]image.Point{"право": {side - 33, mid}, "низ": {mid, side - 39}, "лево": {27, mid}} {
		if got := cf.img.NRGBAAt(p.X, p.Y); got != stubRed {
			t.Errorf("%s: %v в %v, ждали %v", name, got, p, stubRed)
		}
	}
	if full := bboxAlpha(cf.img, cf.img.Bounds()); full.Empty() {
		t.Fatal("рамка пуста")
	}
}

// Холст крупнее 1024 px: окно ищется на копии со средней альфой — просвет у линии не должен затереться.
func TestCornerFrameBigCopy(t *testing.T) {
	big := func(w, h int) *image.NRGBA { return stubCorner(w, h, 30, 20, 8, stubTeal) }
	cs := [4]*image.NRGBA{big(800, 760), flipImg(big(760, 800), true, false), flipImg(big(780, 780), false, true), flipImg(big(800, 800), true, true)}
	cf, err := buildCornerFrame(cs, LineThin, nil)
	if err != nil {
		t.Fatal(err)
	}
	if side := cf.img.Bounds().Dx(); side < 3*800 || side > 4096 {
		t.Fatalf("холст %d", side)
	}
	cp := alphaCopy(cf.img)
	if cp == nil || cp.Bounds().Dx() != winAnalyzePx {
		t.Fatalf("копия для разбора: %v", cp)
	}
}

// Образцов линии в уголках нет — цвет доминирующий: самый частый непрозрачный, не тёмная обводка.
func TestCornerFrameDominantColor(t *testing.T) {
	mk := func() *image.NRGBA {
		img := canvas(240, 240, transp)
		fillRect(img, image.Rect(10, 10, 130, 130), stubTeal)                 // доминирует
		fillRect(img, image.Rect(140, 10, 170, 40), coral)                    // реже
		fillRect(img, image.Rect(5, 5, 9, 200), color.NRGBA{10, 10, 10, 255}) // тёмная обводка — не цвет линии
		return img
	}
	tl := mk()
	cs := [4]*image.NRGBA{tl, flipImg(mk(), true, false), flipImg(mk(), false, true), flipImg(mk(), true, true)}
	if got := dominantColor(cs); got != stubTeal {
		t.Fatalf("доминирующий цвет %v, ждали %v", got, stubTeal)
	}
	cf, err := buildCornerFrame(cs, LineThin, nil)
	if err != nil {
		t.Fatal(err)
	}
	side := cf.img.Bounds().Dx()
	found := false
	for y := 0; y < side/3 && !found; y++ {
		found = cf.img.NRGBAAt(side/2, y) == stubTeal
	}
	if !found {
		t.Error("верхняя линия не цвета доминирующего")
	}
	// Цвет из рецепта главнее.
	red := color.NRGBA{255, 0, 0, 255}
	cf, err = buildCornerFrame(cs, LineThin, &red)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for y := 0; y < side/3 && !found; y++ {
		found = cf.img.NRGBAAt(side/2, y) == red
	}
	if !found {
		t.Error("цвет из рецепта не применён")
	}
}

// Края и отказы: пустое, крошечное, огромное, неверный line и цвет — ошибка, а не паника.
func TestCornerFrameEdges(t *testing.T) {
	cs := fourCorners(stubRed)
	if _, err := buildCornerFrame(cs, "wavy", nil); err == nil {
		t.Error("line wavy: ждали отказ")
	}
	tiny := canvas(4, 4, artBlue)
	if _, err := buildCornerFrame([4]*image.NRGBA{tiny, tiny, tiny, tiny}, LineNone, nil); err == nil {
		t.Error("уголки 4×4: ждали отказ")
	}
	empty := image.NewNRGBA(image.Rect(0, 0, 0, 0))
	if _, err := buildCornerFrame([4]*image.NRGBA{empty, empty, empty, empty}, LineThin, nil); err == nil {
		t.Error("пустые уголки: ждали отказ")
	}
	huge := canvas(1400, 1400, transp)
	fillRect(huge, image.Rect(0, 0, 100, 100), artBlue)
	if _, err := buildCornerFrame([4]*image.NRGBA{huge, huge, huge, huge}, LineNone, nil); err == nil || !strings.Contains(err.Error(), "предела импорта") {
		t.Errorf("уголки 1400 px: %v", err)
	}
	// Полностью прозрачные уголки: рамка без изображения, но без паники; доминирующий цвет — запасной серый.
	blank := canvas(120, 120, transp)
	if _, err := buildCornerFrame([4]*image.NRGBA{blank, blank, blank, blank}, LineThin, nil); err != nil {
		t.Errorf("прозрачные уголки: %v", err)
	}
	for _, bad := range []string{"", "#12", "123456", "#12345g", "#1234567"} {
		if _, err := parseHexColor(bad); err == nil {
			t.Errorf("цвет %q принят", bad)
		}
	}
	if c, err := parseHexColor("#0A1b2C"); err != nil || c != (color.NRGBA{10, 27, 44, 255}) {
		t.Errorf("цвет: %v %v", c, err)
	}
	for _, bad := range []string{"a.png", "a-tr.png", "a-tl.jpg", "-tl", ""} {
		if _, err := cornerPaths(bad); err == nil {
			t.Errorf("путь %q принят", bad)
		}
	}
	if p, err := cornerPaths("c/x-tl.png"); err != nil || p != [4]string{"c/x-tl.png", "c/x-tr.png", "c/x-bl.png", "c/x-br.png"} {
		t.Errorf("пути: %v %v", p, err)
	}
}

// Рецепт: два варианта на один набор уголков, файлы, манифест, «тянется» в отчёте, одинаковый прогон;
// битый набор (нет уголка) — ошибка этой рамки, соседняя собирается.
func TestRunCornerFrame(t *testing.T) {
	src := tempDir(t)
	cs := fourCorners(stubRed)
	for i, suf := range quarterSuffix {
		writePNG(t, filepath.Join(src, "corners", "x-"+suf+".png"), cs[i])
	}
	for _, suf := range quarterSuffix[1:] { // второй набор без уголков tr/bl/br
		_ = suf
	}
	writePNG(t, filepath.Join(src, "corners", "lone-tl.png"), cs[0])
	item := func(line, out string) Item {
		return Item{Src: "corners/x-tl.png", Mode: ModeCornerFrame, Line: line, Out: out, Name: "Рамка " + line, Family: "x-frame"}
	}
	rec := baseRecipe(item("none", "frames/x-none.png"), item("thin", "frames/x-thin.png"),
		Item{Src: "corners/lone-tl.png", Mode: ModeCornerFrame, Out: "frames/lone.png", Name: "Одинокий", Family: "lone"})
	rep, out := run(t, src, rec)
	if len(rep.Items) != 2 || !hasError(rep, "lone-tl.png") {
		t.Fatalf("ждали 2 рамки и ошибку lone: %v %v", len(rep.Items), rep.Errors)
	}
	for _, f := range []string{"frames/x-none.png", "frames/x-thin.png"} {
		o := byFile(t, rep, f)
		if o.Kind != KindFrame || o.Stretch == nil || *o.Stretch > 5 || o.W != o.H {
			t.Errorf("%s: %+v", f, o)
		}
		if b := decodePNG(t, readOut(t, out, f)).Bounds(); b.Dx() != o.W {
			t.Errorf("%s: файл %v, отчёт %d", f, b, o.W)
		}
	}
	man := string(readOut(t, out, "manifest.src.json"))
	if !strings.Contains(man, `"kind": "frame"`) || !strings.Contains(man, `"file": "frames/x-thin.png"`) {
		t.Errorf("манифест:\n%s", man)
	}
	rep2, out2 := run(t, src, rec)
	_ = rep2
	for _, f := range []string{"frames/x-none.png", "frames/x-thin.png"} {
		if !bytes.Equal(readOut(t, out, f), readOut(t, out2, f)) {
			t.Errorf("%s: два прогона дали разные байты", f)
		}
	}
}

// Рецепт: режим принимает только свои поля, line и color проверяются, src — «…-tl.png».
func TestCornerFrameRecipeValidation(t *testing.T) {
	ok := Item{Src: "c/x-tl.png", Mode: ModeCornerFrame, Out: "frames/x.png", Name: "Рамка", Line: "thin", Color: "#aabbcc"}
	if err := ok.validate(); err != nil {
		t.Fatalf("годный предмет: %v", err)
	}
	for name, mod := range map[string]func(*Item){
		"line":  func(i *Item) { i.Line = "dashed" },
		"color": func(i *Item) { i.Color = "синий" },
		"src":   func(i *Item) { i.Src = "c/x.png" },
		"поле":  func(i *Item) { i.Thr = 230 },
		"out":   func(i *Item) { i.Out = "frames/x.jpg" },
		"имя":   func(i *Item) { i.Name = "" },
		"names": func(i *Item) { i.Names = []string{"А"} },
	} {
		it := ok
		mod(&it)
		if err := it.validate(); err == nil {
			t.Errorf("%s: ждали отказ", name)
		}
	}
	// Поле line чужому режиму не годится.
	fr := Item{Src: "c/f.png", Mode: ModeFrame, Out: "frames/f.png", Name: "Р", Line: "thin"}
	if err := fr.validate(); err == nil {
		t.Error("line у режима frame принят")
	}
}

// Фаззинг размеров и содержимого уголков: ошибка или рамка, но не паника и не выход за холст.
func FuzzCornerFrame(f *testing.F) {
	f.Add(uint16(300), uint16(280), uint8(30), uint8(6), uint8(255), true)
	f.Add(uint16(8), uint16(9), uint8(0), uint8(1), uint8(0), false)
	f.Add(uint16(1000), uint16(40), uint8(255), uint8(200), uint8(128), true)
	f.Fuzz(func(t *testing.T, w, h uint16, at, th, alpha uint8, thin bool) {
		w, h = max(w%1400, 1), max(h%1400, 1)
		c := canvas(int(w), int(h), transp)
		fillRect(c, image.Rect(int(at)%int(w), int(at)%int(h), min(int(w), int(at)%int(w)+int(th)+1), min(int(h), int(at)%int(h)+int(th)+1)), color.NRGBA{9, 99, 199, alpha})
		line := LineNone
		if thin {
			line = LineThin
		}
		cf, err := buildCornerFrame([4]*image.NRGBA{c, flipImg(c, true, false), flipImg(c, false, true), flipImg(c, true, true)}, line, nil)
		if err == nil && (cf.img.Bounds().Dx() != cf.img.Bounds().Dy() || cf.img.Bounds().Dx() > 4096) {
			t.Fatalf("холст %v", cf.img.Bounds())
		}
	})
}
