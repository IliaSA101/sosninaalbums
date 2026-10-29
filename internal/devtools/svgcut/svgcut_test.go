package svgcut

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSvgCutDropsSheetRect(t *testing.T) {
	_, o := newFake(t)
	res := mustCut(t, readTestdata(t, "sheet.svg"), o)
	if res.Paths != 6 || res.WhiteTotal != 2 || res.SheetFill != "rgb(255,255,255)" {
		t.Fatalf("путей %d (ждали 6 без листа), белых %d (ждали 2), лист %q", res.Paths, res.WhiteTotal, res.SheetFill)
	}
	for _, it := range res.Items {
		for _, w := range it.Whites {
			if w.Path == 1 { // путь 1 — лист
				t.Fatalf("лист попал в белые пути предмета %d", it.Index)
			}
		}
	}
	// У садика лист 253,253,252 — определяется по форме, а не по цвету 255.
	kg := svgOf(100, 100, "rgb(253,253,252)", "10 10 40 40 rgb(200,0,0)", "60 10 90 40 rgb(253,253,252)")
	if r := mustCut(t, kg, o); r.SheetFill != "rgb(253,253,252)" || r.WhiteTotal != 1 || r.Paths != 2 {
		t.Fatalf("лист садика: %+v", r)
	}
	// Не лист: белый прямоугольник не во весь лист, тёмный во весь лист.
	for _, d := range [][]byte{
		svgOf(100, 100, "rgb(200,200,200)", "10 10 40 40 rgb(200,0,0)"),
		bytes.Replace(svgOf(100, 100, "rgb(255,255,255)", "10 10 40 40 rgb(200,0,0)"), []byte(`L 100 100 L 0 100 L 0 0 z`), []byte(`L 100 50 L 0 50 L 0 0 z`), 1),
	} {
		doc, err := Parse(d)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Sheet() != -1 {
			t.Fatalf("лист найден там, где его нет: %d", doc.Sheet())
		}
	}
	// Лист без листа — предупреждение, предметы остаются.
	r := mustCut(t, svgOf(100, 100, "rgb(0,200,200)", "10 10 40 40 rgb(200,0,0)"), o)
	if len(r.Items) != 1 || r.Items[0].Paths != 2 || len(r.Notes) == 0 {
		t.Fatalf("без листа: предметов %d, замечаний %d", len(r.Items), len(r.Notes))
	}
}

func TestSvgCutGroups(t *testing.T) {
	_, o := newFake(t)
	res := mustCut(t, readTestdata(t, "sheet.svg"), o)
	if len(res.Items) != 3 || res.Groups != 3 {
		t.Fatalf("предметов %d, групп %d; ждали 3", len(res.Items), res.Groups)
	}
	// Порядок слева направо; число путей и белых по предметам.
	want := []struct{ paths, whites int }{{3, 1}, {2, 1}, {1, 0}}
	for i, it := range res.Items {
		if it.Index != i+1 || it.Paths != want[i].paths || len(it.Whites) != want[i].whites {
			t.Fatalf("предмет %d: путей %d, белых %d; ждали %v", i+1, it.Paths, len(it.Whites), want[i])
		}
	}
	if res.Items[0].Whites[0].N != 1 || res.Items[1].Whites[0].N != 2 {
		t.Fatalf("номера белых: %+v %+v", res.Items[0].Whites, res.Items[1].Whites)
	}
	// Зазор gap склеивает близкие пути, 0 — только касающиеся.
	near := svgOf(100, 50, "rgb(255,255,255)", "10 10 40 40 rgb(200,0,0)", "41.5 10 70 40 rgb(0,0,200)")
	o.Gap = 2
	if r := mustCut(t, near, o); len(r.Items) != 1 {
		t.Fatalf("gap 2: %d предметов, ждали 1", len(r.Items))
	}
	o.Gap = 0
	if r := mustCut(t, near, o); len(r.Items) != 2 {
		t.Fatalf("gap 0: %d предметов, ждали 2", len(r.Items))
	}
	o.Gap = -1 // меньше нуля — умолчание
	if r := mustCut(t, near, o); len(r.Items) != 1 {
		t.Fatalf("gap < 0: %d предметов, ждали 1", len(r.Items))
	}
}

func TestSvgCutGridSplitsGlued(t *testing.T) {
	_, o := newFake(t)
	data := readTestdata(t, "glued.svg")
	if r := mustCut(t, data, o); len(r.Items) != 2 {
		t.Fatalf("без сетки склеенные красный и синий — один предмет: %d предметов", len(r.Items))
	}
	o.Cols, o.Rows = 3, 1
	r := mustCut(t, data, o)
	if len(r.Items) != 3 || r.Items[0].Paths != 1 || r.Items[1].Paths != 1 {
		t.Fatalf("сетка 3x1 разделяет склеенное: %d предметов", len(r.Items))
	}
	o.Cols = 4 // ячейка 75: красный 0, синий 1, зелёный 3, ячейка 2 пуста
	r = mustCut(t, data, o)
	if len(r.Items) != 3 || len(r.EmptyCells) != 1 || r.EmptyCells[0] != [2]int{3, 1} {
		t.Fatalf("пустые ячейки: %+v, предметов %d", r.EmptyCells, len(r.Items))
	}
	o.Cols, o.Rows = 2, 0
	if _, err := Cut(data, o); err == nil {
		t.Fatal("сетка с одной стороной принята")
	}
}

func TestSvgCutSplitDrop(t *testing.T) {
	_, o := newFake(t)
	data := readTestdata(t, "glued.svg")
	x := 90.0
	o.Split = []SplitLine{{Group: 1, X: &x}}
	r := mustCut(t, data, o)
	if len(r.Items) != 3 || r.Items[0].Base != 1 || r.Items[1].Base != 1 || r.Items[2].Base != 2 {
		t.Fatalf("split: %+v", r.Items)
	}
	o.Split, o.Drop = nil, []int{1}
	if r = mustCut(t, data, o); len(r.Items) != 1 || r.Items[0].Base != 2 {
		t.Fatalf("drop 1: предметов %d", len(r.Items))
	}
	// Номера в split и drop — до любых изменений: drop первой группы не сдвигает split второй.
	y := 50.0
	o.Split, o.Drop = []SplitLine{{Group: 2, Y: &y}}, []int{1}
	if r = mustCut(t, data, o); len(r.Items) != 1 {
		t.Fatalf("split по y не делит одиночный путь: %d", len(r.Items))
	}
	for _, bad := range []Options{
		{Drop: []int{9}},
		{Drop: []int{1, 2}},
		{Split: []SplitLine{{Group: 9, X: &x}}},
		{Split: []SplitLine{{Group: 1}}},
		{Split: []SplitLine{{Group: 1, X: &x, Y: &y}}},
	} {
		b := o
		b.Split, b.Drop = bad.Split, bad.Drop
		if _, err := Cut(data, b); err == nil {
			t.Fatalf("плохая правка принята: %+v", bad)
		}
	}
}

// knockoutCase — общая проверка выреза для фальшивого растра и настоящего Chromium.
func knockoutCase(t *testing.T, o Options) {
	t.Helper()
	data := readTestdata(t, "sheet.svg")
	base := mustCut(t, data, o)
	o.Side = 520
	o.Knock = [][2]float64{{70, 70}}
	res := mustCut(t, data, o)
	if len(res.Knocks) != 1 || res.Knocks[0].White != 1 || res.Knocks[0].Path != 3 {
		t.Fatalf("попадание: %+v", res.Knocks)
	}
	it := res.Items[0]
	if len(it.Whites) != 1 || !it.Whites[0].Knocked {
		t.Fatalf("белый путь предмета 1 не вырезан: %+v", it.Whites)
	}
	if got := res.Items[1].Whites[0]; got.Knocked {
		t.Fatalf("белая часть второго предмета вырезана: %+v", got)
	}
	// Масштаб 5 px на ед. листа, начало среза — рамка предмета (20, 20).
	at := func(img *image.NRGBA, x, y float64) color.NRGBA { return img.NRGBAAt(int((x-20)*5), int((y-20)*5)) }
	if c := at(it.Img, 70, 85); c.A != 0 {
		t.Fatalf("пустота не прозрачна, а красный под ней просвечивает: %v", c)
	}
	if c := at(it.Img, 70, 53); c.A != 255 || c.R != 10 {
		t.Fatalf("кант над пустотой прорезан вместе с белым: %v", c)
	}
	if c := at(it.Img, 30, 30); c != (color.NRGBA{200, 30, 30, 255}) {
		t.Fatalf("красное вне пустоты тронуто: %v", c)
	}
	// Без выреза пустота — белая; у белой части второго предмета вырез не нужен.
	if c := base.Items[0].Img.NRGBAAt(base.Items[0].W/2, base.Items[0].H/2); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Fatalf("без knock пустота не белая: %v", c)
	}
	dog := res.Items[1].Img
	if c := dog.NRGBAAt(dog.Bounds().Dx()/2, dog.Bounds().Dy()/2); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Fatalf("белая часть второго предмета не осталась белой: %v", c)
	}
	// Несколько вырезов сразу: обе белые части, предмет 3 — без белых.
	o.Knock = [][2]float64{{70, 70}, {240, 60}}
	two := mustCut(t, data, o)
	if len(two.Knocks) != 2 || two.Knocks[1].White != 2 {
		t.Fatalf("два выреза: %+v", two.Knocks)
	}
	pic := two.Items[1].Img
	if c := pic.NRGBAAt(pic.Bounds().Dx()/2, pic.Bounds().Dy()/2); c.A != 0 {
		t.Fatalf("вторая пустота не вырезана: %v", c)
	}
}

func TestSvgCutKnockout(t *testing.T) {
	f, o := newFake(t)
	knockoutCase(t, o)
	if f.calls == 0 {
		t.Fatal("растр не вызывался")
	}
	// Служебный растр листа — один на прогон с точками, белые пути — цветами-номерами.
	hits := 0
	for _, s := range f.svgs {
		if strings.Contains(s, `shape-rendering="crispEdges"`) {
			hits++
			if !strings.Contains(s, `fill="rgb(0,0,3)"`) { // путь 3 (с 1) — белая пустота: номер пути в палитре служебного растра
				t.Fatalf("белый путь не закрашен цветом-номером:\n%s", s)
			}
		}
	}
	if hits == 0 {
		t.Fatal("служебного растра не было")
	}
}

func TestSvgCutKnockMiss(t *testing.T) {
	f, o := newFake(t)
	data := readTestdata(t, "sheet.svg")
	for name, pt := range map[string][2]float64{
		"на красном":        {30, 30},
		"на пустом листе":   {150, 150},
		"на градиенте":      {350, 40},
		"вне листа":         {500, 10},
		"отрицательная":     {-1, 10},
		"на канте":          {70, 52}, // кант (тёмный) лежит над белым путём: видимое не белое
		"на синем":          {210, 30},
		"край листа справа": {400.5, 10},
	} {
		o.Knock = [][2]float64{pt}
		_, err := Cut(data, o)
		if err == nil {
			t.Fatalf("%s: промах принят", name)
		}
		if name != "вне листа" && name != "отрицательная" && name != "край листа справа" && !strings.Contains(err.Error(), "нет пути цвета листа") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Все промахи сразу, а не по одному на прогон.
	o.Knock = [][2]float64{{30, 30}, {150, 150}, {70, 70}}
	_, err := Cut(data, o)
	if err == nil || strings.Count(err.Error(), "knock (") != 2 {
		t.Fatalf("ждали два промаха в одной ошибке: %v", err)
	}
	// Предметы при промахе не растрируются: падаем до дорогой работы.
	n := f.calls
	if _, err := Cut(data, o); err == nil || f.calls != n+1 {
		t.Fatalf("растров при промахе: %d, ждали один служебный", f.calls-n)
	}
	// Без листа цвет пустот неизвестен — knock невозможен.
	noSheet := svgOf(100, 100, "rgb(0,200,200)", "10 10 40 40 rgb(200,0,0)")
	o.Knock = [][2]float64{{20, 20}}
	if _, err := Cut(noSheet, o); err == nil || !strings.Contains(err.Error(), "лист не найден") {
		t.Fatalf("knock без листа: %v", err)
	}
	// Точка в снятой группе — ошибка, а не молчаливое «вырезано».
	o.Knock, o.Drop = [][2]float64{{70, 70}}, []int{1}
	if _, err := Cut(data, o); err == nil || !strings.Contains(err.Error(), "снятой группе") {
		t.Fatalf("knock в drop: %v", err)
	}
}

func TestSvgCutNeedsRender(t *testing.T) {
	o := DefaultOptions() // Render не задан
	if _, err := Cut(readTestdata(t, "sheet.svg"), o); err == nil || !strings.Contains(err.Error(), "растр") {
		t.Fatalf("без растра: %v", err)
	}
	for _, side := range []int{10, 5000, -1} {
		_, o := newFake(t)
		o.Side = side
		if _, err := Cut(readTestdata(t, "sheet.svg"), o); err == nil {
			t.Fatalf("side %d принят", side)
		}
	}
}

func TestSvgCutItemErrorsCollected(t *testing.T) {
	_, o := newFake(t)
	o.Render = func(svg []byte, w, h int) (*image.NRGBA, error) {
		return nil, errors.New("сбой браузера")
	}
	res, err := Cut(readTestdata(t, "sheet.svg"), o)
	if err == nil || strings.Count(err.Error(), "сбой браузера") != 3 {
		t.Fatalf("ждали три ошибки предметов сразу: %v", err)
	}
	if res == nil || len(res.Items) != 3 {
		t.Fatal("результат с отчётом при сбое не отдан")
	}
	// Пустой растр (всё прозрачное) — ошибка предмета.
	o.Render = func(svg []byte, w, h int) (*image.NRGBA, error) { return image.NewNRGBA(image.Rect(0, 0, w, h)), nil }
	if _, err := Cut(readTestdata(t, "sheet.svg"), o); err == nil || !strings.Contains(err.Error(), "пуст") {
		t.Fatalf("пустой растр: %v", err)
	}
}

func TestSvgCutSaveLoad(t *testing.T) {
	_, o := newFake(t)
	o.Overlay = true
	o.Knock = [][2]float64{{70, 70}}
	res := mustCut(t, readTestdata(t, "sheet.svg"), o)
	dir := tempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "07.png"), []byte("старый"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("моё"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(res, dir); err != nil {
		t.Fatal(err)
	}
	for i, it := range res.Items {
		f, err := os.Open(filepath.Join(dir, ItemName(i+1)))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != it.W || img.Bounds().Dy() != it.H || it.Bytes == 0 {
			t.Fatalf("предмет %d после записи: %v, отчёт %dx%d", i+1, img.Bounds(), it.W, it.H)
		}
		// Полностью непрозрачный предмет кодек пишет без альфы: сравниваем цвета, не тип буфера.
		for y := 0; y < it.H; y++ {
			for x := 0; x < it.W; x++ {
				if color.NRGBAModel.Convert(img.At(x, y)) != it.Img.NRGBAAt(x, y) {
					t.Fatalf("предмет %d: пиксель (%d, %d) после круга Save→Load изменился", i+1, x, y)
				}
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "07.png")); err == nil {
		t.Fatal("остаток прошлого прогона не удалён")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatal("чужой файл удалён")
	}
	if _, err := os.Stat(filepath.Join(dir, OverlayName)); err != nil {
		t.Fatal("нет листа подсветки")
	}
	if err := Save(res, ""); err == nil {
		t.Fatal("пустая папка вывода принята")
	}
	if b, err := res.JSON(); err != nil || !strings.Contains(string(b), `"knocked": true`) {
		t.Fatalf("JSON: %v\n%s", err, b)
	}
	if txt := res.Text(); !strings.Contains(txt, "белых 1 (вырезано 1)") || !strings.Contains(txt, "w1 путь 3") {
		t.Fatalf("отчёт без числа белых путей:\n%s", txt)
	}
}

func TestSvgCutDeterministic(t *testing.T) {
	_, o := newFake(t)
	o.Knock = [][2]float64{{70, 70}}
	a := mustCut(t, readTestdata(t, "sheet.svg"), o)
	b := mustCut(t, readTestdata(t, "sheet.svg"), o)
	for i := range a.Items {
		if !bytes.Equal(a.Items[i].Img.Pix, b.Items[i].Img.Pix) || a.Items[i].Frame != b.Items[i].Frame {
			t.Fatalf("предмет %d различается между прогонами", i+1)
		}
	}
}

func TestParseHelpers(t *testing.T) {
	for in, want := range map[string]string{"6x6": "6 6", "3X4": "3 4", "": "0 0"} {
		c, r, err := ParseGrid(in)
		if err != nil || strconv.Itoa(c)+" "+strconv.Itoa(r) != want {
			t.Fatalf("ParseGrid(%q) = %d %d %v", in, c, r, err)
		}
	}
	for _, bad := range []string{"6", "0x3", "ax3", "3x", "101x1", "-1x2"} {
		if _, _, err := ParseGrid(bad); err == nil {
			t.Fatalf("сетка %q принята", bad)
		}
	}
	if s, err := ParseSplit("7:x=812.5"); err != nil || s.Group != 7 || s.X == nil || *s.X != 812.5 {
		t.Fatalf("split: %+v %v", s, err)
	}
	if s, err := ParseSplit("2:y=-3"); err != nil || s.Y == nil || *s.Y != -3 {
		t.Fatalf("split y: %+v %v", s, err)
	}
	for _, bad := range []string{"", "7", "7:z=1", "0:x=1", "a:x=1", "7:x=", "7:x=NaN", "7:x=Inf"} {
		if _, err := ParseSplit(bad); err == nil {
			t.Fatalf("split %q принят", bad)
		}
	}
	if p, err := ParseKnock(" 812 , 640.5"); err != nil || p != [2]float64{812, 640.5} {
		t.Fatalf("knock: %v %v", p, err)
	}
	for _, bad := range []string{"", "1", "a,b", "1,NaN", "1,Inf"} {
		if _, err := ParseKnock(bad); err == nil {
			t.Fatalf("knock %q принят", bad)
		}
	}
	if d, err := ParseDrop("3, 5,9"); err != nil || len(d) != 3 || d[2] != 9 {
		t.Fatalf("drop: %v %v", d, err)
	}
	for _, bad := range []string{"", "0", "a", "1,,2"} {
		if _, err := ParseDrop(bad); err == nil {
			t.Fatalf("drop %q принят", bad)
		}
	}
}

// Служебный растр не выходит за пределы Render ни по стороне, ни по площади (лист 7000² = 49 Мп).
func TestHitScaleLimits(t *testing.T) {
	cases := []struct{ w, h float64 }{
		{400, 200}, {7000, 7000}, {6325, 6325}, {8192, 4000}, {20000, 20000}, {20000, 1}, {1, 20000}, {1, 1}, {6324.6, 6324.6},
	}
	for _, c := range cases {
		sx, sy, pw, ph := hitScale(&Doc{W: c.w, H: c.h})
		if pw < 1 || ph < 1 || pw > MaxRenderSide || ph > MaxRenderSide || pw*ph > maxRenderPx {
			t.Errorf("лист %g×%g: растр %d×%d вне пределов Render", c.w, c.h, pw, ph)
		}
		// Пропорции листа сохраняются с точностью до округления в 1 px (не только вписались в пределы).
		if d := float64(pw)*c.h - float64(ph)*c.w; math.Abs(d) > max(c.w, c.h) && min(pw, ph) > 1 {
			t.Errorf("лист %g×%g: растр %d×%d искажает пропорции", c.w, c.h, pw, ph)
		}
		if d := sx*c.w - float64(pw); d > 1e-6 || d < -1e-6 {
			t.Errorf("лист %g×%g: масштаб X %g не даёт ширину %d", c.w, c.h, sx, pw)
		}
		if d := sy*c.h - float64(ph); d > 1e-6 || d < -1e-6 {
			t.Errorf("лист %g×%g: масштаб Y %g не даёт высоту %d", c.w, c.h, sy, ph)
		}
	}
	// Малый лист — 1:1, без уменьшения.
	if _, _, pw, ph := hitScale(&Doc{W: 400, H: 200}); pw != 400 || ph != 200 {
		t.Errorf("малый лист 400×200 стал %d×%d", pw, ph)
	}
	// Лист 7000² режется целиком до растра в пределах (без отказа Render по площади).
	if _, _, pw, ph := hitScale(&Doc{W: 7000, H: 7000}); pw*ph < maxRenderPx*9/10 {
		t.Errorf("лист 7000² уменьшен сильнее нужного: %d×%d", pw, ph)
	}
}

// id маски выреза не совпадает с id градиента файла (reID пропускает «k2»): url(#k2) вёл бы на того,
// кто раньше в defs.
func TestKnockMaskIDAvoidsGradient(t *testing.T) {
	data := bytes.ReplaceAll(readTestdata(t, "sheet.svg"), []byte("Gradient1"), []byte("k2"))
	f, o := newFake(t)
	o.Knock = [][2]float64{{70, 70}}
	res := mustCut(t, data, o)
	if len(res.Knocks) != 1 || res.Knocks[0].Path != 3 {
		t.Fatalf("попадание: %+v", res.Knocks)
	}
	masks := 0
	for _, s := range f.svgs {
		ids := map[string]string{}
		var walk func(n *node)
		walk = func(n *node) {
			if id := n.attrs["id"]; id != "" {
				if prev, dup := ids[id]; dup {
					t.Errorf("id %q повторён: %s и %s\n%s", id, prev, n.name, s)
				}
				ids[id] = n.name
				if n.name == "mask" {
					masks++
				}
			}
			for _, k := range n.kids {
				walk(k)
			}
		}
		walk(parseTree(t, []byte(s)))
	}
	if masks == 0 {
		t.Fatal("маска выреза не попала ни в один документ")
	}
}
