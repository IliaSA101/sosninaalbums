package export

import (
	"context"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
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

// Задание экспорта из View: кадр, фон, подписи перед блоками, развороты только с текстом
// и совсем пустые идут в список все (номер файла = номер разворота).
func TestExportSpreads(t *testing.T) {
	st := text.Style{Font: "montserrat", Size: 14, Color: "#222222", Align: "left"}
	p := model.New()
	p.Settings.Bg = "#f6f1e7"
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200, Frame: &model.Frame{S: 2, X: -1, Y: -1},
		Caption: &model.Caption{Text: "Подпись", Style: model.DefaultCaptionStyle()}}
	p.Photos["b"] = model.PhotoRef{Path: `C:\b.jpg`, W: 200, H: 300, FlipH: true}
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
	if len(s1.Photos) != 1 || s1.Photos[0].Path != `C:\a.jpg` || s1.Photos[0].Img == s1.Photos[0].Cell {
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
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
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
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
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
	if ColorNote(0) != "" || ColorNote(3) != "Пересчитано из Adobe RGB в sRGB: 3 фото." {
		t.Errorf("note: %q %q", ColorNote(0), ColorNote(3))
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
