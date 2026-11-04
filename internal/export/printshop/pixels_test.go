package printshop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	albumcore "github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Эталон выгрузки «для типографии» до правок px (0.26-123-33): имена файлов, размеры, плотность,
// пиксели RGBA (как TestGoldenExport) и таблицы квантования JPEG. Побайтно файл не сверяется:
// это хрупко, а качество (100 у Фабрики, 95 у Фотосферы) видно по DQT.
//
// От ОС результат не зависит: альбом без текста (шрифт рисуется по-разному), фото — синтетика без
// цветового профиля, JPEG кодирует и читает сам Go, масштаб и поворот — чистый Go без CGO. Целые
// числа и float64 на amd64 (Linux, Windows) дают те же биты; на arm64 FMA может сдвинуть пиксель.
var updatePrint = flag.Bool("update", false, "перезаписать testdata/golden/print-export.json")

const printGoldenPath = "testdata/golden/print-export.json"

// printFile — запись эталона об одном файле выгрузки.
type printFile struct {
	File  string `json:"file"` // <вид типографии>/<имя>
	W     int    `json:"w"`
	H     int    `json:"h"`
	DPI   int    `json:"dpi"`   // из JFIF
	RGBA  string `json:"rgba"`  // SHA-256 пикселей RGBA
	DQT   string `json:"dqt"`   // SHA-256 всех таблиц квантования (сегменты DQT)
	Quant []int  `json:"quant"` // сумма значений каждой таблицы: у качества 100 — по 64
}

// TestPrintExportPixels выгружает маленький альбом (обложка, два разворота, без текста) так же,
// как App.Export в режиме типографии, — для Фотосферы и Фабрики 20×20 при 300 dpi — и сверяет
// файлы с эталоном. Страховка до правок px (А1, А2): они не должны менять печатный результат
// там, где он уже верен.
func TestPrintExportPixels(t *testing.T) {
	if testing.Short() {
		t.Skip("рендер трёх листов 300 dpi на типографию — не быстрый уровень")
	}
	dir := testkit.TempDir(t)
	var got []printFile
	for _, prof := range Profiles() {
		got = append(got, printRun(t, dir, prof)...)
	}
	// Второй прогон: эталон бессмыслен, если результат случаен.
	var again []printFile
	for _, prof := range Profiles() {
		again = append(again, printRun(t, filepath.Join(dir, "again"), prof)...)
	}
	if !slices.EqualFunc(got, again, printSame) {
		t.Fatalf("два прогона дали разное:\n%s", printDiff(got, again))
	}

	if *updatePrint || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(printGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(printGoldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d файлов)", printGoldenPath, len(got))
		return
	}
	data, err := os.ReadFile(printGoldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/export/printshop -run PrintExportPixels -update", err)
	}
	var want []printFile
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("эталон не читается: %v", err)
	}
	if !slices.EqualFunc(want, got, printSame) {
		t.Fatalf("выгрузка разошлась с эталоном %s (если изменение задумано — перезаписать: -update):\n%s",
			printGoldenPath, printDiff(want, got))
	}
}

// printRun строит альбом типографии prof и выгружает его, как App.Export: Check без запретов,
// 300 dpi, обложка, префикс и качество профиля (окно даёт 95, когда у профиля своего нет).
func printRun(t *testing.T, root string, prof Profile) []printFile {
	t.Helper()
	dir := filepath.Join(root, prof.Kind)
	p, v := printAlbum(t, dir, prof)
	if pl := Check(p, "", prof); len(pl.Block) > 0 || pl.Format != "20×20" {
		t.Fatalf("%s: альбом не готов к выгрузке: формат %q, запреты %v", prof.Kind, pl.Format, pl.Block)
	}
	quality := 95
	if prof.Quality > 0 {
		quality = prof.Quality
	}
	g := p.Geo(nil)
	opt := render.Options{PageW: g.W / 2, PageH: g.H, DPI: 300, Quality: quality}
	n := len(v.Spreads)
	pl := export.NewPlan(nil, n, false).WithCover(export.NewCoverJob(p, v, 300, quality)).WithPrefix(prof.Prefix)
	if err := export.CheckCanvases(opt, false); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := export.RunPlan(context.Background(), export.FromView(p, v), pl, opt, out, nil, func(int, int) {}); err != nil {
		t.Fatalf("%s: %v", prof.Kind, err)
	}
	var files []printFile
	for _, name := range pl.Names(n) {
		files = append(files, printDescribe(t, filepath.Join(out, name), prof.Kind+"/"+name))
	}
	return files
}

// printAlbum — альбом 20×20 формата prof из двух разворотов и обложки, без текста:
//  1. сетка: фото с обрезкой, фото целиком (fit contain), пустое место, свой цвет страниц;
//  2. «Авто»: два фото на фото во весь разворот с прозрачностью;
//     обложка: цвет левой стороны, фото на лицевой, плашка корешка.
//
// Фон альбома — цвет, не белый.
func printAlbum(t *testing.T, dir string, prof Profile) (*model.Project, model.View) {
	t.Helper()
	i := slices.IndexFunc(model.GroupFormats(prof.Group), func(f model.PrintFormat) bool { return f.Name == "20×20" })
	if i < 0 {
		t.Fatalf("%s: нет формата 20×20", prof.Kind)
	}
	f := model.GroupFormats(prof.Group)[i]
	photo := func(id string, w, h int) model.PhotoInfo {
		return model.PhotoInfo{ID: id, Path: printJPEG(t, dir, id+".jpg", w, h), W: w, H: h}
	}
	must := func(v model.View, err error) model.View {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	e, err := albumcore.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	s := model.DefaultSettings()
	s.PageW, s.PageH, s.Safe, s.Bleed, s.DPI, s.Layflat, s.Margin, s.Gap, s.Bg = f.PageW, f.PageH, f.Safe, f.Bleed, 300, true, 8, 3, "#f3efe6"
	e.SetSettings(s)

	v := must(e.NewSpread(0, []model.PhotoInfo{photo("a", 300, 200), photo("b", 200, 300)}))
	s1 := v.Spreads[0].ID
	e.SetGrid(s1, true)
	// Ссылка на место — ID фото (placeRef): ID мест случайные. Третье место пустое: AddPlace.
	e.SetPlaces(s1, []model.PlaceBox{
		{ID: "a", X: 10, Y: 12, W: 120, H: 80},
		{ID: "b", X: 150, Y: 20, W: 60, H: 90},
	}, "")
	must(e.AddPlace(s1, 2, model.ShapeOval))
	e.SetPlaceFit(s1, []string{"b"}, model.FitContain)
	e.SetBackground(s1, "left", "#d8e2dc")
	e.SetBackground(s1, "right", "#2b3a4a")

	v = must(e.NewSpread(1, []model.PhotoInfo{photo("c", 300, 200), photo("d", 200, 300)}))
	s2 := v.Spreads[1].ID
	must(e.SetBackdrop(s2, model.ScopeSpread, photo("f", 400, 250)))
	e.SetBackdropFade(s2, model.ScopeSpread, 0.5, "")

	ph := photo("g", 300, 200)
	must(e.CreateCover(map[string]string{"fsfera": "fsfera-thin", "fabrika": "fabrika-photo"}[prof.Kind], &ph))
	e.SetBackground(model.CoverID, "left", "#c81e1e")
	e.SetCoverSpineBg("#1b2a3a")

	p, v := e.Snapshot()
	printScene(t, p, v)
	return p, v
}

// printScene проверяет, что в альбоме есть всё заявленное, а текста нет: иначе эталон записался
// бы с холостой правкой или зависел от шрифта ОС.
func printScene(t *testing.T, p *model.Project, v model.View) {
	t.Helper()
	if len(v.Spreads) != 2 || v.Cover == nil {
		t.Fatalf("разворотов %d, обложка %v; ждали 2 и обложку", len(v.Spreads), v.Cover != nil)
	}
	s1, s2 := v.Spreads[0], v.Spreads[1]
	switch {
	case p.Settings.Bg != "#f3efe6":
		t.Fatalf("фон альбома %q", p.Settings.Bg)
	case !s1.Grid || len(s1.Places) != 3 || s1.BgSet != [2]string{"#d8e2dc", "#2b3a4a"}:
		t.Fatalf("разворот 1: сетка из 3 мест с цветом страниц: %+v", s1)
	case len(s2.Backdrops) != 1 || s2.Backdrops[0].Fade != 0.5 || len(s2.Photos) != 2:
		t.Fatalf("разворот 2: фото и фон с прозрачностью: %+v", s2)
	case v.Cover.SpineBg != "#1b2a3a" || v.Cover.Sheet.BgSet[0] != "#c81e1e" || len(v.Cover.Sheet.Backdrops) == 0:
		t.Fatalf("обложка: плашка корешка, цвет, фото лицевой: %+v", v.Cover)
	}
	for _, s := range append([]model.SpreadView{s1, s2}, v.Cover.Sheet) {
		if len(s.Texts) != 0 || len(s.Captions) != 0 {
			t.Fatalf("в альбоме эталона не должно быть текста: %+v %+v", s.Texts, s.Captions)
		}
	}
}

// printJPEG пишет синтетическое фото w×h: градиент с сеткой линий и диагоналями (резкие края).
func printJPEG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{uint8(x * 255 / w), uint8(y * 255 / h), 128, 255}
			switch {
			case x%25 < 2 || y%25 < 2:
				c = color.RGBA{255, 255, 255, 255}
			case (x+y)%40 < 3:
				c = color.RGBA{20, 20, 20, 255}
			case x < w/5 && y < h/5:
				c = color.RGBA{230, 40, 30, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// printDescribe читает файл выгрузки: размер, dpi из JFIF, хеш пикселей RGBA и таблицы квантования.
func printDescribe(t *testing.T, path, name string) printFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	sum := sha256.Sum256(rgba.Pix)
	dqt, quant, err := jpegQuant(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return printFile{File: name, W: b.Dx(), H: b.Dy(), DPI: printDPI(data), RGBA: hex.EncodeToString(sum[:]), DQT: dqt, Quant: quant}
}

// printDPI — плотность из APP0 JFIF сразу после SOI (так пишет saveJPEG); 0 — нет сегмента или не дюймы.
func printDPI(data []byte) int {
	if len(data) < 18 || !bytes.Equal(data[2:4], []byte{0xFF, 0xE0}) || string(data[6:11]) != "JFIF\x00" || data[13] != 1 {
		return 0
	}
	x, y := binary.BigEndian.Uint16(data[14:]), binary.BigEndian.Uint16(data[16:])
	if x != y {
		return -1
	}
	return int(x)
}

// jpegQuant обходит сегменты JPEG до начала скана и собирает таблицы квантования (DQT): хеш всех
// сегментов и сумму значений каждой таблицы. Свои границы на каждый шаг: файл — наш, но тест
// не должен падать паникой на усечённом.
func jpegQuant(data []byte) (hash string, sums []int, err error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return "", nil, fmt.Errorf("нет SOI")
	}
	h := sha256.New()
	for i := 2; i+4 <= len(data); {
		if data[i] != 0xFF {
			return "", nil, fmt.Errorf("нет маркера на %d", i)
		}
		m, n := data[i+1], int(binary.BigEndian.Uint16(data[i+2:]))
		if n < 2 || i+2+n > len(data) {
			return "", nil, fmt.Errorf("сегмент %#x выходит за файл", m)
		}
		switch m {
		case 0xDA:
			return hex.EncodeToString(h.Sum(nil)), sums, nil
		case 0xDB:
			seg := data[i+4 : i+2+n]
			h.Write(seg)
			if sums, err = quantSums(seg, sums); err != nil {
				return "", nil, err
			}
		}
		i += 2 + n
	}
	return "", nil, fmt.Errorf("нет скана")
}

// quantSums дописывает в sums сумму значений каждой таблицы сегмента DQT.
func quantSums(seg []byte, sums []int) ([]int, error) {
	for len(seg) > 0 {
		wide := seg[0]>>4 != 0 // 16 бит на значение
		size := 64
		if wide {
			size = 128
		}
		if len(seg) < 1+size {
			return nil, fmt.Errorf("таблица DQT усечена")
		}
		sum := 0
		for j := 1; j <= size; j++ {
			if !wide {
				sum += int(seg[j])
			} else if j%2 == 1 {
				sum += int(binary.BigEndian.Uint16(seg[j:]))
			}
		}
		sums = append(sums, sum)
		seg = seg[1+size:]
	}
	return sums, nil
}

func printSame(a, b printFile) bool {
	return a.File == b.File && a.W == b.W && a.H == b.H && a.DPI == b.DPI && a.RGBA == b.RGBA && a.DQT == b.DQT && slices.Equal(a.Quant, b.Quant)
}

// printDiff — какие файлы разошлись, каких не хватает и какие лишние.
func printDiff(want, got []printFile) string {
	byName := func(fs []printFile) map[string]printFile {
		m := make(map[string]printFile, len(fs))
		for _, f := range fs {
			m[f.File] = f
		}
		return m
	}
	wm, gm := byName(want), byName(got)
	var lines []string
	for _, w := range want {
		g, ok := gm[w.File]
		switch {
		case !ok:
			lines = append(lines, fmt.Sprintf("  %s: файла нет", w.File))
		case !printSame(g, w):
			lines = append(lines, fmt.Sprintf("  %s: ждали %d×%d px, %d dpi, %.12s…, DQT %.8s %v; получили %d×%d px, %d dpi, %.12s…, DQT %.8s %v",
				w.File, w.W, w.H, w.DPI, w.RGBA, w.DQT, w.Quant, g.W, g.H, g.DPI, g.RGBA, g.DQT, g.Quant))
		}
	}
	for _, g := range got {
		if _, ok := wm[g.File]; !ok {
			lines = append(lines, fmt.Sprintf("  %s: лишний файл", g.File))
		}
	}
	return strings.Join(lines, "\n")
}

// TestJPEGQuantBroken: разбор таблиц квантования на пустом, усечённом и битом входе — ошибка, а не паника.
func TestJPEGQuantBroken(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	good := buf.Bytes()
	if _, sums, err := jpegQuant(good); err != nil || len(sums) == 0 || sums[0] != 64 {
		t.Fatalf("целый файл: %v %v", sums, err)
	}
	for n := range len(good) {
		if _, _, err := jpegQuant(good[:n]); err == nil && n < 100 {
			t.Fatalf("усечённый до %d байт разобран без ошибки", n)
		}
	}
	for _, in := range [][]byte{nil, {}, {0xFF}, {0xFF, 0xD8}, {0xFF, 0xD8, 0x00, 0x00, 0x00, 0x00}, {0xFF, 0xD8, 0xFF, 0xDB, 0xFF, 0xFF, 0x00}} {
		if _, _, err := jpegQuant(in); err == nil {
			t.Fatalf("вход %x разобран без ошибки", in)
		}
	}
}
