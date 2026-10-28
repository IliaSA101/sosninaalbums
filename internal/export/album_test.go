package export

import (
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// mustEditor — album.NewEditor для тестов: отказ проверки проекта — Fatal.
func mustEditor(t testing.TB, p *model.Project, path string) *album.Editor {
	t.Helper()
	e, err := album.NewEditor(p, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// sameLines — строки совпадают; координаты — до 1e-9 мм (угол блока прибавлен и вычтен).
func sameLines(a, b []text.Line) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Text != b[i].Text || math.Abs(a[i].X-b[i].X) > 1e-9 || math.Abs(a[i].Y-b[i].Y) > 1e-9 || math.Abs(a[i].W-b[i].W) > 1e-9 {
			return false
		}
	}
	return true
}

// Задание экспорта из View: кадр, фон, подписи перед блоками, развороты только с текстом
// и совсем пустые идут в список все (номер файла = номер разворота).
func TestExportSpreads(t *testing.T) {
	st := text.Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}
	p := model.New()
	p.Settings.Bg = "#f6f1e7"
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200, Frame: &model.Frame{S: 2, X: -1, Y: -1},
		Caption: &model.Caption{Text: "Подпись", Style: model.DefaultCaptionStyle()}}
	p.Photos["b"] = model.PhotoRef{Path: testkit.AbsPath("b.jpg"), W: 200, H: 300, FlipH: true}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Texts: []*model.TextBlock{{ID: "t1", X: 20, Y: 30, W: 80, Text: "Привет", Style: st}}},
		{ID: "s2", Texts: []*model.TextBlock{{ID: "t2", X: 30, Y: 40, W: 90, Text: "Только текст", Style: st}}},
		{ID: "s3"},
		{ID: "s4", Photos: []string{"b"}, BgR: "#1c1c1c"},
	}
	pp, v := mustEditor(t, p, "").Snapshot()
	got := FromView(pp, v)
	if len(got) != 4 {
		t.Fatalf("разворотов в задании %d, ждали 4 (пустой не пропускается)", len(got))
	}

	s1 := got[0]
	if len(s1.Photos) != 1 || s1.Photos[0].Path != testkit.AbsPath("a.jpg") || s1.Photos[0].Img == s1.Photos[0].Cell {
		t.Fatalf("фото без кадра: %+v", s1.Photos)
	}
	if s1.Photos[0].Img != v.Spreads[0].Frames[0].Img {
		t.Fatalf("Img %v, в View %v", s1.Photos[0].Img, v.Spreads[0].Frames[0].Img)
	}
	if s1.Photos[0].FlipH || s1.Photos[0].FlipV || !got[3].Photos[0].FlipH || got[3].Photos[0].FlipV {
		t.Fatalf("зеркало не доехало до задания: %+v %+v", s1.Photos[0], got[3].Photos[0])
	}
	if len(s1.Texts) != 2 || s1.Texts[0].Style != model.DefaultCaptionStyle() || s1.Texts[1].Style != st {
		t.Fatalf("порядок слоёв: подпись, затем блок; получено %+v", s1.Texts)
	}
	// Строки блока — от его угла: базовая линия первой строки около кегля, а не около Y блока.
	if l := s1.Texts[1].Box.Lines[0]; l.Y <= 0 || l.Y > 20 || s1.Texts[1].X != 20 || s1.Texts[1].Y != 30 {
		t.Fatalf("строка блока: %+v, угол (%v, %v)", l, s1.Texts[1].X, s1.Texts[1].Y)
	}
	// Text — показанный текст; переразбивка при смене шрифтов (render, 082) из Text по Box.W даёт
	// те же строки, что в View, пока шрифты те же. Gen ставит PinFonts всем текстам.
	for i, want := range []string{"Подпись", "Привет"} {
		td := s1.Texts[i]
		again := text.Cached(td.Text, td.Style, td.Box.W).Lines
		if td.Text != want || td.Gen != 0 || !sameLines(again, td.Box.Lines) {
			t.Fatalf("текст %d: %q, Gen %d, строки %+v, заново %+v", i, td.Text, td.Gen, td.Box.Lines, again)
		}
	}
	PinFonts(got, 7)
	for _, sp := range got {
		for _, td := range sp.Texts {
			if td.Gen != 7 {
				t.Fatalf("PinFonts: %+v", td)
			}
		}
	}
	album := color.RGBA{0xf6, 0xf1, 0xe7, 255}
	if s1.Bg != [2]color.RGBA{album, album} {
		t.Fatalf("фон альбома: %v", s1.Bg)
	}

	if s2 := got[1]; len(s2.Photos) != 0 || len(s2.Texts) != 1 {
		t.Fatalf("разворот только с текстом: %+v", s2)
	}
	if s3 := got[2]; len(s3.Photos) != 0 || len(s3.Texts) != 0 || s3.Bg != [2]color.RGBA{album, album} {
		t.Fatalf("пустой разворот: %+v", s3)
	}
	if s4 := got[3]; s4.Bg != [2]color.RGBA{album, {0x1c, 0x1c, 0x1c, 255}} {
		t.Fatalf("фон правой страницы: %v", s4.Bg)
	}
}

// Пустой разворот посреди альбома не сдвигает номера: файл N — разворот N,
// у пустого просто фон страниц.
func TestExportNumbering(t *testing.T) {
	dir := testkit.TempDir(t)
	st := text.Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: testkit.JPEG(t, dir, "a.jpg", 60, 40), W: 60, H: 40}
	p.Photos["b"] = model.PhotoRef{Path: testkit.JPEG(t, dir, "b.jpg", 40, 60), W: 40, H: 60}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}},
		{ID: "s2"}, // пустой: нет ни фото, ни текста
		{ID: "s3", Texts: []*model.TextBlock{{ID: "t1", X: 10, Y: 10, W: 40, Text: "Привет", Style: st}}},
		{ID: "s4", Photos: []string{"b"}},
	}
	pp, v := mustEditor(t, p, "").Snapshot()
	spreads := FromView(pp, v)
	if len(spreads) != len(v.Spreads) {
		t.Fatalf("в задании %d разворотов, в альбоме %d", len(spreads), len(v.Spreads))
	}
	out := filepath.Join(dir, "out")
	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 30, Quality: 90}
	if err := Run(context.Background(), spreads, opt, out, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		if _, err := os.Stat(filepath.Join(out, FileName(i, 4))); err != nil {
			t.Fatalf("нет файла разворота %d: %v", i+1, err)
		}
	}
}

// Пустые места: номера разворотов с 1, общее число; в текст вопроса идут не больше 10 номеров.
func TestEmptyPlacesSummary(t *testing.T) {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	full := func(id string) *model.Spread {
		return &model.Spread{ID: id, Grid: []*model.Place{{ID: id + "p", X: 10, Y: 10, W: 50, H: 40, Photo: "a"}}}
	}
	holes := func(id string, n int) *model.Spread {
		s := &model.Spread{ID: id}
		for i := range n {
			s.Grid = append(s.Grid, &model.Place{ID: id + string(rune('a'+i)), X: float64(10 + 20*i), Y: 10, W: 15, H: 15})
		}
		return s
	}
	p.Spreads = []*model.Spread{full("s1"), holes("s2", 2), {ID: "s3"}, holes("s4", 3), full("s5")}
	_, v := mustEditor(t, p, "").Snapshot()
	spreads, n := emptySummary(v)
	if !slices.Equal(spreads, []int{2, 4}) || n != 5 || n != v.EmptyPlaces {
		t.Fatalf("emptySummary = %v, %d (View.EmptyPlaces %d)", spreads, n, v.EmptyPlaces)
	}
	want := "На разворотах 2, 4 пустые места (5). В файлы они не попадут, там будет фон. Выгрузить?"
	if got := EmptyWarning(spreads, n); got != want {
		t.Fatalf("вопрос %q", got)
	}
	if got := EmptyWarning([]int{7}, 1); !strings.HasPrefix(got, "На развороте 7 пустые места (1).") {
		t.Fatalf("один разворот: %q", got)
	}
	if got := EmptyWarning(nil, 0); got != "" {
		t.Fatalf("без пустых мест вопрос не нужен: %q", got)
	}
	many := make([]int, 13)
	for i := range many {
		many[i] = i + 1
	}
	if got := EmptyWarning(many, 20); !strings.Contains(got, "1, 2, 3, 4, 5, 6, 7, 8, 9, 10 …и ещё 3 пустые места (20)") {
		t.Fatalf("больше 10 номеров: %q", got)
	}
	if s, n := emptySummary(model.View{}); len(s) != 0 || n != 0 {
		t.Fatalf("пустой вид: %v, %d", s, n)
	}
}

// Сетка с contain уходит в экспорт ячейкой-местом и фото внутри неё: поля остаются фоном.
func TestExportGridContain(t *testing.T) {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Grid: []*model.Place{
		{ID: "p1", X: 10, Y: 10, W: 60, H: 60, Photo: "a", Fit: model.FitContain},
		{ID: "p2", X: 80, Y: 10, W: 30, H: 30},
	}}}
	pp, v := mustEditor(t, p, "").Snapshot()
	got := FromView(pp, v)
	if len(got) != 1 || len(got[0].Photos) != 1 {
		t.Fatalf("задание: %+v (пустое место не передаётся)", got)
	}
	ph := got[0].Photos[0]
	if ph.Cell.W != 60 || ph.Cell.H != 60 || ph.Img.W != 60 || ph.Img.H != 40 || ph.Img.Y != 20 {
		t.Fatalf("ячейка %v, фото %v: ждали 60x40 по центру места 60x60", ph.Cell, ph.Img)
	}
}

func TestColorWarning(t *testing.T) {
	if got := ColorWarning(nil); got != "" {
		t.Errorf("без чужих профилей: %q", got)
	}
	got := ColorWarning(map[string]int{"Display P3": 2, "ProPhoto RGB": 1})
	want := "Фото с цветовым профилем не sRGB: 3 (Display P3 — 2, ProPhoto RGB — 1). Программа их не пересчитывает — " +
		"в печати цвета будут искажены. Лучше пересохранить их в sRGB. Экспортировать всё равно?"
	if got != want {
		t.Errorf("получили %q", got)
	}
	// не больше трёх имён: частые первыми, дальше «…», общий счёт — по всем
	got = ColorWarning(map[string]int{"A": 5, "B": 4, "C": 3, "D": 2, "E": 1})
	if !strings.Contains(got, ": 15 (A — 5, B — 4, C — 3, …)") || strings.Contains(got, "D —") {
		t.Errorf("получили %q", got)
	}
}

// Сводка после экспорта (119): имена как в ColorWarning — до трёх, по убыванию, дальше «…».
func TestColorNote(t *testing.T) {
	cases := []struct {
		conv map[string]int
		want string
	}{
		{nil, ""},
		{map[string]int{}, ""},
		{map[string]int{"Adobe RGB": 0}, ""},
		{map[string]int{"Adobe RGB": 3}, "Пересчитано в sRGB: 3 фото (Adobe RGB — 3)."},
		{map[string]int{"Adobe RGB": 3, "Display P3": 5}, "Пересчитано в sRGB: 8 фото (Display P3 — 5, Adobe RGB — 3)."},
		{map[string]int{"B": 2, "A": 2, "C": 2, "D": 1}, "Пересчитано в sRGB: 7 фото (A — 2, B — 2, C — 2, …)."},
		{map[string]int{"Display P3": 1}, "Пересчитано в sRGB: 1 фото (Display P3 — 1)."},
	}
	for _, c := range cases {
		if got := ColorNote(c.conv); got != c.want {
			t.Errorf("%v: получили %q, ждали %q", c.conv, got, c.want)
		}
	}
}

// BenchmarkExport64Texts — экспорт разворота 300 dpi с 64 надписями (спека 026) против
// пустого: View → FromView → Render. Разница — цена надписей.
func BenchmarkExport64Texts(b *testing.B) {
	for _, n := range []int{0, model.MaxTexts} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			p := model.New()
			s := &model.Spread{ID: "s1"}
			for i := range n {
				s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), X: float64(10 + i%8*65), Y: float64(10 + i/8*33), W: 60,
					Rot: float64(i % 3 * 15), Text: "Иван и Мария\nглава " + strconv.Itoa(i), Style: model.PlainStyle()})
			}
			p.Spreads = []*model.Spread{s}
			e := mustEditor(b, p, "")
			opt := render.Options{PageW: p.Settings.PageW, PageH: p.Settings.PageH, DPI: 300, Quality: 95}
			for b.Loop() {
				pp, v := e.Snapshot()
				if _, err := render.Render(FromView(pp, v)[0], opt); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestExport64Photos: разворот на 64 фото (предел 027) выгружается, все 64 ячейки нарисованы
// (в центре каждой — фото, а не белый фон). Память держит прежний семафор на 3 оригинала.
func TestExport64Photos(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.Settings.Bg = "#ffffff"
	s := &model.Spread{ID: "s1"}
	for i := range 64 {
		id := "p" + strconv.Itoa(i)
		w, h := 30, 20
		if i%3 == 0 {
			w, h = 20, 30
		}
		p.Photos[id] = model.PhotoRef{Path: testkit.JPEG(t, dir, id+".jpg", w, h), W: w, H: h}
		s.Photos = append(s.Photos, id)
	}
	p.Spreads = []*model.Spread{s}
	pp, v := mustEditor(t, p, "").Snapshot()
	spreads := FromView(pp, v)
	if len(spreads) != 1 || len(spreads[0].Photos) != 64 {
		t.Fatalf("в задании %d фото", len(spreads[0].Photos))
	}
	out := filepath.Join(dir, "out")
	const dpi = 60
	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: dpi, Quality: 95}
	if err := Run(context.Background(), spreads, opt, out, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(out, FileName(0, 1)))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	for i, ph := range spreads[0].Photos {
		x := int((ph.Cell.X + ph.Cell.W/2) / 25.4 * dpi)
		y := int((ph.Cell.Y + ph.Cell.H/2) / 25.4 * dpi)
		r, _, b, _ := img.At(x, y).RGBA()
		if r>>8 > 200 || b>>8 < 90 || b>>8 > 170 { // у тестового JPEG в центре ~(127, 127, 128)
			t.Fatalf("ячейка %d (%d, %d) не нарисована: r=%d b=%d", i, x, y, r>>8, b>>8)
		}
	}
}

// Узор (048) едет в задание из View, как у холста: область, цвета с цветами генератора,
// масштаб 1, прозрачность; неизвестный генератор передаётся и рисуется пусто. В рендере узор
// правой страницы виден только на ней.
func TestExportPatterns(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2"}}
	e := mustEditor(t, p, "")
	for _, c := range []struct {
		sp, scope string
		pt        model.Pattern
	}{
		{"s1", "right", model.Pattern{Kind: "stripes", Inks: []string{"#203040"}, Fade: 0.3}},
		{"s2", "spread", model.Pattern{Kind: "future-kind", Scale: 2}},
	} {
		if _, err := e.ApplyPattern(c.sp, c.scope, c.pt, ""); err != nil {
			t.Fatal(err)
		}
	}
	pp, v := e.Snapshot()
	got := FromView(pp, v)
	if len(got) != 2 || len(got[0].Patterns) != 1 || len(got[1].Patterns) != 1 {
		t.Fatalf("узоры в задании: %+v", got)
	}
	pd, pv := got[0].Patterns[0], v.Spreads[0].Patterns[0]
	if pd.Area != pv.Area || pd.Kind != "stripes" || !slices.Equal(pd.Inks, pv.Pat.Inks) || pd.Scale != 1 || pd.Fade != 0.3 ||
		pd.Area.X != pp.Settings.PageW {
		t.Fatalf("узор правой: %+v, в View %+v", pd, pv)
	}
	if u := got[1].Patterns[0]; u.Kind != "future-kind" || u.Scale != 2 || u.Area.W != 2*pp.Settings.PageW {
		t.Fatalf("неизвестный: %+v", u)
	}

	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 20}
	with, err := render.Render(got[0], opt)
	if err != nil {
		t.Fatal(err)
	}
	bare := got[0]
	bare.Patterns = nil
	without, err := render.Render(bare, opt)
	if err != nil {
		t.Fatal(err)
	}
	mid, differ := with.Rect.Dx()/2, 0
	for y := range with.Rect.Dy() {
		for x := range with.Rect.Dx() {
			if with.RGBAAt(x, y) != without.RGBAAt(x, y) {
				if x < mid {
					t.Fatalf("узор правой страницы на левой: (%d, %d)", x, y)
				}
				differ++
			}
		}
	}
	if differ == 0 {
		t.Fatal("узор не нарисован")
	}
	if img, err := render.Render(got[1], opt); err != nil || string(img.Pix) != string(without.Pix) {
		t.Fatalf("неизвестный генератор нарисован: %v", err)
	}
}

// Стикеры (048) едут в задание из View: свой — фото целиком в рамке (путь из Photos), поворот,
// зеркало, прозрачность и Z; процедурный — цвета с цветами генератора; неизвестный передаётся и
// рисуется пусто; прозрачный на 100 % файл не читает. В рендере стикеры видны.
func TestExportStickers(t *testing.T) {
	file := filepath.Join(testkit.TempDir(t), "my.png")
	im := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	draw.Draw(im, image.Rect(0, 0, 20, 20), image.NewUniform(color.NRGBA{200, 30, 40, 255}), image.Point{}, draw.Src)
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, im); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2"}}
	e := mustEditor(t, p, "")
	add := func(sp string, src model.StickerSrc) string {
		_, id, err := e.AddSticker(sp, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	own := model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID("my:0123456789abcdef"), Path: file, Lib: "my:0123456789abcdef", W: 40, H: 20}}
	k1 := add("s1", own)
	e.SetStickerBox("s1", k1, 10, 10, 40, 20, 30, "")
	e.FlipSticker("s1", k1, "h")
	e.SetStickerFade("s1", k1, 0.2, "")
	add("s1", model.StickerSrc{Art: &model.Art{Kind: "heart", Inks: []string{"#203040"}}})
	add("s1", model.StickerSrc{Art: &model.Art{Kind: "zz-future"}})
	k4 := add("s2", own)
	e.SetStickerFade("s2", k4, 1, "")
	pp, v := e.Snapshot()
	got := FromView(pp, v)
	if len(got) != 2 || len(got[0].Stickers) != 1 || len(got[0].Arts) != 2 || len(got[1].Stickers) != 1 || len(got[1].Arts) != 0 {
		t.Fatalf("стикеры в задании: %+v", got)
	}
	sv := v.Spreads[0].Stickers
	box := layout.Rect{X: sv[0].X, Y: sv[0].Y, W: sv[0].W, H: sv[0].H}
	if ph := got[0].Stickers[0]; ph.Path != file || ph.Cell != box || ph.Img != box || ph.Rot != 30 || !ph.FlipH || ph.FlipV ||
		ph.Fade != 0.2 || ph.Z != sv[0].Z || ph.Z == 0 {
		t.Fatalf("свой: %+v, в View %+v", ph, sv[0])
	}
	if a := got[0].Arts[0]; a.Kind != "heart" || !slices.Equal(a.Inks, sv[1].Art.Inks) || len(a.Inks) != 2 || a.Z != sv[1].Z ||
		a.Box != (layout.Rect{X: sv[1].X, Y: sv[1].Y, W: sv[1].W, H: sv[1].H}) || got[0].Arts[1].Kind != "zz-future" {
		t.Fatalf("процедурные: %+v, в View %+v", got[0].Arts, sv[1:])
	}

	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 20}
	bare := render.Spread{Bg: got[0].Bg}
	without, err := render.Render(bare, opt)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []bool{true, false} {
		img, err := render.Render(got[i], opt)
		if err != nil || (string(img.Pix) != string(without.Pix)) != want {
			t.Errorf("лист %d: нарисовано %v, ждали %v (%v)", i+1, string(img.Pix) != string(without.Pix), want, err)
		}
	}
}

// TestTplScenePlaceholders — плитка шаблона (067): пустые места — серые плашки в своём слое
// (стикер «под местами» ниже, остальные выше), повёрнутое — описанным прямоугольником; лист без
// мест — без плашек; пустой вид — false. Экспорт альбома плашек не получает (эталоны — golden_test).
func TestTplScenePlaceholders(t *testing.T) {
	tp := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Page: []model.TplBg{{Color: "#ffeedd"}, {}},
		Places: []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.4, H: 0.4, Rot: 30}},
		Stickers: []model.TplItem{
			{W: 0.9, H: 0.5, Under: true, Art: &model.Art{Kind: "plate"}},
			{Anchor: model.Anchor{AX: 1, AY: 1, DX: -0.1, DY: -0.1}, W: 0.1, H: 0.1, Art: &model.Art{Kind: "star"}},
		}}
	p, v, err := album.PreviewTpl(tp, 400, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := TplScene(p, v)
	if !ok || len(sc.Slots) != 2 || len(sc.Arts) != 2 || len(sc.Photos) != 0 {
		t.Fatalf("сцена: %v, мест %d, стикеров %d, фото %d", ok, len(sc.Slots), len(sc.Arts), len(sc.Photos))
	}
	if sc.Bg[0] != render.ParseHex("#ffeedd") {
		t.Errorf("цвет левой %v", sc.Bg[0])
	}
	under, above := sc.Arts[0], sc.Arts[1]
	if under.Kind != "plate" {
		under, above = above, under
	}
	for _, sl := range sc.Slots {
		if sl.Color != slotColor || sl.Z <= under.Z || sl.Z >= above.Z {
			t.Errorf("место %+v: под %d, над %d", sl, under.Z, above.Z)
		}
	}
	straight, rot := sc.Slots[0], sc.Slots[1]
	if pl := v.Spreads[0].Places[1]; rot.Rect.W <= pl.W || rot.Rect.H <= pl.H {
		t.Errorf("повёрнутое место %+v, рамка %+v", rot.Rect, pl)
	}
	if want := (layout.Rect{X: 20, Y: 20, W: 160, H: 160}); !nearRect(straight.Rect, want) {
		t.Errorf("место %+v, ждали %+v", straight.Rect, want)
	}
	if _, ok := TplScene(p, model.View{}); ok {
		t.Fatal("пустой вид дал сцену")
	}
	tp.Places = nil
	p, v, err = album.PreviewTpl(tp, 400, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sc, _ := TplScene(p, v); len(sc.Slots) != 0 {
		t.Fatalf("без мест: %+v", sc.Slots)
	}
	if es := FromView(p, v); len(es) != 1 || len(es[0].Slots) != 0 {
		t.Fatal("экспорт получил плашки мест")
	}
}

func nearRect(a, b layout.Rect) bool {
	d := func(x, y float64) bool { return x-y < 0.01 && y-x < 0.01 }
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}
