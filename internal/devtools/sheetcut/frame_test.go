package sheetcut

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

var (
	paper = color.NRGBA{238, 238, 236, 255} // бумага Mystic: «почти белая» ниже 245
	green = color.NRGBA{40, 140, 60, 255}
	red   = color.NRGBA{200, 40, 40, 255}
)

// ringFrame — лист w×h цвета bg с прямоугольным кольцом толщины th, отступ от краёв листа m
// (слева, сверху, справа, снизу); окно внутри кольца — цвета bg.
func ringFrame(w, h int, m [4]int, th int, bg, ink color.NRGBA) *image.NRGBA {
	img := canvas(w, h, bg)
	outer := image.Rect(m[0], m[1], w-m[2], h-m[3])
	fillRect(img, outer, ink)
	fillRect(img, outer.Inset(th), bg)
	return img
}

func frameOpts() Options {
	o := DefaultOptions()
	o.Mode = ModeFrame
	return o
}

func TestFrameWindowSeed(t *testing.T) {
	img := ringFrame(400, 400, [4]int{40, 40, 40, 40}, 50, white, navy)
	res := mustCut(t, img, frameOpts())
	if res.BG != BGSeed {
		t.Fatalf("фон %q, нужно seed: у рамок режим по умолчанию", res.BG)
	}
	if len(res.Stickers) != 1 {
		t.Fatalf("стикеров %d, нужна одна рамка", len(res.Stickers))
	}
	s := res.Stickers[0]
	if s.W != 320 || s.H != 320 {
		t.Fatalf("после обрезки %d×%d, нужно 320×320 (кольцо без полей)", s.W, s.H)
	}
	if fi := s.Frame; fi == nil || !fi.Window || fi.Open {
		t.Fatalf("окно: %+v, нужно закрытое найденное", fi)
	}
	// Окно прозрачно (заливка от центра), кольцо плотное, внешнее поле срезано.
	if a := alphaAt(s.Img, 160, 160); a != 0 {
		t.Fatalf("центр окна: альфа %d, нужно 0", a)
	}
	if a := alphaAt(s.Img, 25, 160); a != 255 {
		t.Fatalf("кольцо: альфа %d, нужно 255", a)
	}
	if res.BGInfo == nil || res.BGInfo.WindowPx != 220*220 {
		t.Fatalf("окно %+v, нужно 220×220 px", res.BGInfo)
	}
	// Срез — по окну: 50/320 с каждой стороны.
	for i, c := range s.Frame.Cut {
		if c < 0.155 || c > 0.157 {
			t.Fatalf("срез[%d] = %v, нужно 0,156", i, c)
		}
	}
}

func TestFrameWindowNotFoundAndOpen(t *testing.T) {
	// Центр закрашен: окна нет — флаг, а не отказ; «тянется» не считается.
	solid := ringFrame(300, 300, [4]int{20, 20, 20, 20}, 140, white, navy)
	res := mustCut(t, solid, frameOpts())
	s := res.Stickers[0]
	if s.Frame.Window || s.Frame.Stretch != -1 || !hasWarn(s, "окно не найдено") {
		t.Fatalf("закрашенный центр: %+v, предупреждения %v", s.Frame, s.Warnings)
	}
	// Уголки: окно — всё место, доходит до края, срез по третям.
	corners := canvas(300, 300, white)
	fillRect(corners, image.Rect(10, 10, 90, 90), navy)
	fillRect(corners, image.Rect(210, 210, 290, 290), navy)
	s = mustCut(t, corners, frameOpts()).Stickers[0]
	if !s.Frame.Open || !hasWarn(s, "открытая") {
		t.Fatalf("уголки: %+v, предупреждения %v", s.Frame, s.Warnings)
	}
	if c := s.Frame.Cut; c[0] < 0.33 || c[0] > 0.34 {
		t.Fatalf("срез открытой рамки %v, нужно трети", c)
	}
	// Окно меньше 5 % площади программа отвергает.
	tiny := ringFrame(300, 300, [4]int{0, 0, 0, 0}, 140, white, navy)
	fillRect(tiny, image.Rect(145, 145, 155, 155), white)
	if s = mustCut(t, tiny, frameOpts()).Stickers[0]; s.Frame.Window {
		t.Fatalf("окно 10×10 принято: %+v", s.Frame)
	}
}

func TestFrameAdaptiveThreshold(t *testing.T) {
	cases := []struct {
		name string
		bg   color.NRGBA
		want uint8
	}{
		{"белая бумага", white, 247}, // 255 − 8
		{"серая бумага", paper, 228}, // минимум каналов 236 − 8
		{"тёмный лист", color.NRGBA{120, 120, 120, 255}, thrMin},
	}
	for _, c := range cases {
		img := ringFrame(300, 300, [4]int{30, 30, 30, 30}, 40, c.bg, navy)
		if got := AdaptiveThr(img, true); got != c.want {
			t.Errorf("%s: порог %d, нужно %d", c.name, got, c.want)
		}
	}
	// Бумага 238 при постоянном 245 осталась бы пятном; порог по листу снимает её целиком.
	img := ringFrame(400, 400, [4]int{40, 40, 40, 40}, 50, paper, navy)
	o := frameOpts()
	s := mustCut(t, img, o).Stickers[0]
	if a := alphaAt(s.Img, 160, 160); a != 0 {
		t.Fatalf("по листу: окно из бумаги 238 не снято, альфа %d", a)
	}
	o.Thr = 245
	s = mustCut(t, img, o).Stickers[0]
	if s.Frame.Window {
		t.Fatalf("порог 245 вручную: бумага 238 осталась непрозрачной, окна быть не должно")
	}
	if res := mustCut(t, img, o); res.BGInfo.Thr != 245 {
		t.Fatalf("порог в отчёте %d, нужно заданный 245", res.BGInfo.Thr)
	}
}

// pocketSheet — лист с двумя замкнутыми карманами (кольца navy): чистый белый (255) и с фактурой (248).
func pocketSheet() *image.NRGBA {
	img := canvas(400, 200, white)
	for _, cx := range []int{100, 300} {
		ring(img, cx, 100, 70, 55, navy, white)
	}
	ring(img, 300, 100, 55, 0, color.NRGBA{248, 248, 248, 255}, white) // бумага с фактурой: недобелённость 7 > 3
	return img
}

func TestFrameHolesPolicy(t *testing.T) {
	cases := []struct {
		holes        string
		clean, noisy bool // карман снят (прозрачен)
	}{
		{HolesClean, true, false},
		{"", true, false},
		{HolesAll, true, true},
		{HolesNone, false, false},
	}
	for _, c := range cases {
		img := pocketSheet()
		st := RemoveBackgroundSeed(img, SeedOptions{Thr: 240, Holes: c.holes})
		if got := alphaAt(img, 100, 100) == 0; got != c.clean {
			t.Errorf("holes %q: чистый карман снят = %v, нужно %v (%+v)", c.holes, got, c.clean, st)
		}
		if got := alphaAt(img, 300, 100) == 0; got != c.noisy {
			t.Errorf("holes %q: карман с фактурой снят = %v, нужно %v (%+v)", c.holes, got, c.noisy, st)
		}
		// Кольцо и внешний фон от политики не зависят.
		if alphaAt(img, 100, 100-62) != 255 || alphaAt(img, 5, 5) != 0 {
			t.Errorf("holes %q: кольцо или фон тронуты", c.holes)
		}
	}
	o := frameOpts()
	o.Holes = "xx"
	if _, err := Cut(pocketSheet(), o); err == nil || !strings.Contains(err.Error(), "holes") {
		t.Fatalf("holes xx: ошибка %v", err)
	}
}

func TestFrameTrimsMargin(t *testing.T) {
	// Поля разной ширины: слева 80, сверху 10, справа 30, снизу 5 — для рамки места это были бы
	// прозрачные полосы разной толщины.
	img := ringFrame(500, 400, [4]int{80, 10, 30, 5}, 40, white, navy)
	fillRect(img, image.Rect(2, 380, 4, 382), navy) // крошка в поле: рамку обрезки не раздвигает
	res := mustCut(t, img, frameOpts())
	s := res.Stickers[0]
	if s.W != 390 || s.H != 385 {
		t.Fatalf("после обрезки %d×%d, нужно 390×385", s.W, s.H)
	}
	if s.Frame.Trim != [4]int{80, 10, 30, 5} {
		t.Fatalf("срезано %v, нужно поля [80 10 30 5]", s.Frame.Trim)
	}
	// Рамка вплотную к краям файла: на каждой из четырёх сторон есть плотный пиксель.
	hit := func(f func(i int) uint8, n int) bool {
		for i := range n {
			if f(i) == 255 {
				return true
			}
		}
		return false
	}
	if !hit(func(i int) uint8 { return alphaAt(s.Img, i, 0) }, s.W) || !hit(func(i int) uint8 { return alphaAt(s.Img, i, s.H-1) }, s.W) ||
		!hit(func(i int) uint8 { return alphaAt(s.Img, 0, i) }, s.H) || !hit(func(i int) uint8 { return alphaAt(s.Img, s.W-1, i) }, s.H) {
		t.Fatal("у одной из сторон остался прозрачный ряд")
	}
	if res.Noise != 1 {
		t.Fatalf("мусора %d, нужна 1 крошка", res.Noise)
	}
}

// evenFrame — кольцо 300×300 толщины 60: ровные планки, окно 180×180.
func evenFrame() (*image.NRGBA, [4]float64) {
	return ringFrame(300, 300, [4]int{}, 60, transp, navy), [4]float64{0.2, 0.2, 0.2, 0.2}
}

func TestFrameStretchShare(t *testing.T) {
	total := float64(300*300 - 180*180)
	cases := []struct {
		name string
		mod  func(img *image.NRGBA)
		want float64 // тянется, % (± 0,05)
	}{
		{"ровная планка по всем сторонам", func(*image.NRGBA) {}, 0},
		{"однородная середина штрафуется", func(img *image.NRGBA) { fillRect(img, image.Rect(100, 100, 200, 200), navy) }, 100 * 10000 / (total + 10000)},
		{"украшение в окне считается", func(img *image.NRGBA) { fillRect(img, image.Rect(120, 130, 140, 150), green) }, 100 * 400 / (total + 400)},
		{"неровное вдоль верхней стороны", func(img *image.NRGBA) { fillRect(img, image.Rect(100, 10, 130, 40), green) }, 100 * 900 / total},
		{"неровное вдоль левой стороны", func(img *image.NRGBA) { fillRect(img, image.Rect(10, 150, 40, 180), red) }, 100 * 900 / total},
		{"уголок рисунка не считается", func(img *image.NRGBA) { fillRect(img, image.Rect(5, 5, 55, 55), green) }, 0},
		{"дыра в планке не штрафуется", func(img *image.NRGBA) { fillRect(img, image.Rect(150, 240, 160, 300), transp) }, 0},
	}
	for _, c := range cases {
		img, cut := evenFrame()
		c.mod(img)
		got := stretchShare(img, cut)
		if d := got - c.want; d < -0.05 || d > 0.05 {
			t.Errorf("%s: %.2f %%, нужно %.2f %%", c.name, got, c.want)
		}
	}
}

func TestFrameStretchFlagThroughCut(t *testing.T) {
	// Через Cut: ровная рамка без флага, рамка с большим украшением в окне — с флагом, не отсевом.
	even := mustCut(t, ringFrame(400, 400, [4]int{20, 20, 20, 20}, 60, white, navy), frameOpts()).Stickers[0]
	if even.Frame.Stretch != 0 || hasWarn(even, "тянется") {
		t.Fatalf("ровная рамка: %+v, предупреждения %v", even.Frame, even.Warnings)
	}
	img := ringFrame(400, 400, [4]int{20, 20, 20, 20}, 60, white, navy)
	fillRect(img, image.Rect(90, 90, 190, 330), green) // колонна в окне, центр листа свободен
	res := mustCut(t, img, frameOpts())
	s := res.Stickers[0]
	if s.Frame.Stretch <= StretchFlagPct || !hasWarn(s, "тянется") {
		t.Fatalf("рамка с центром: %.1f %%, предупреждения %v", s.Frame.Stretch, s.Warnings)
	}
	if len(res.Stickers) != 1 {
		t.Fatal("флаг не должен отсеивать рамку")
	}
	if b, err := res.JSON(); err != nil || !strings.Contains(string(b), `"stretch"`) {
		t.Fatalf("JSON отчёта без «stretch»: %v", err)
	}
	if !strings.Contains(res.Text(), "тянется") {
		t.Fatal("текст отчёта без «тянется»")
	}
}

func TestFrameAlphaSourceAndTiny(t *testing.T) {
	// Лист уже с альфой: фон не трогаем, режим рамки только обрезает и считает.
	res := mustCut(t, ringFrame(300, 300, [4]int{50, 50, 50, 50}, 50, transp, navy), frameOpts())
	if res.BG != BGNone || res.Stickers[0].W != 200 {
		t.Fatalf("альфа-лист: фон %s, размер %d", res.BG, res.Stickers[0].W)
	}
	// Крошечные и пустые входы — ошибка или результат, не паника.
	for _, sz := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {1, 40}, {40, 1}, {5, 5}} {
		for _, c := range []color.NRGBA{white, navy, transp} {
			for _, m := range []string{ModeFrame, ModeQuarters, ModeRows} {
				o := DefaultOptions()
				o.Mode = m
				_, _ = Cut(canvas(sz[0], sz[1], c), o)
			}
		}
	}
	// Всё белое: видимого нет — ошибка, а не пустой файл.
	if _, err := Cut(canvas(100, 100, white), frameOpts()); err == nil {
		t.Fatal("белый лист: нужна ошибка «не осталось видимого»")
	}
}

func TestFrameModeRefusals(t *testing.T) {
	img := ringFrame(100, 100, [4]int{}, 20, white, navy)
	for name, o := range map[string]Options{
		"режим":     {Mode: "ring"},
		"сетка":     {Mode: ModeFrame, Cols: 2, Rows: 2},
		"имя":       {Mode: ModeFrame, Stem: "../x"},
		"имя слэш":  {Mode: ModeFrame, Stem: `a\b`},
		"ряды":      {Mode: ModeRows, SplitRows: -1},
		"rowgap":    {Mode: ModeRows, RowGap: -1},
		"имя точки": {Mode: ModeFrame, Stem: ".."},
	} {
		if _, err := Cut(img, o); err == nil {
			t.Errorf("%s: нужна ошибка", name)
		}
	}
}

func TestSaveNeedsDir(t *testing.T) {
	res := mustCut(t, ringFrame(100, 100, [4]int{}, 20, white, navy), frameOpts())
	for _, d := range []string{"", "  "} {
		if err := Save(res, d); err == nil {
			t.Fatalf("Save(%q): нужна ошибка: пусто — текущая папка или корень диска", d)
		}
	}
}
