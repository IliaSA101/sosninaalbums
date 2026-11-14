package export

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
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Эталон печатного результата: страховка перед переносом App и геометрии. Хеш — по
// декодированным пикселям, а не по байтам файла: так эталон ловит любое изменение картинки,
// но не зависит от служебных сегментов, которые пиксели не трогают.
var updateGolden = flag.Bool("update", false, "перезаписать testdata/golden/export.json")

const goldenPath = "testdata/golden/export.json"

// goldenFile — запись эталона об одном файле экспорта.
type goldenFile struct {
	File   string `json:"file"`   // <вид>-<dpi>/<имя>
	Spread int    `json:"spread"` // номер разворота с 1
	W      int    `json:"w"`
	H      int    `json:"h"`
	DPI    int    `json:"dpi"`  // из JFIF
	RGBA   string `json:"rgba"` // SHA-256 пикселей RGBA
}

// TestGoldenExport собирает альбом правками Editor, выгружает его, как App.Export (View →
// FromView → RunPlan), разворотами и по страницам при 300 и 72 dpi и сверяет с эталоном.
// Два прогона подряд обязаны совпасть: иначе эталон бессмыслен.
func TestGoldenExport(t *testing.T) {
	dir := testkit.TempDir(t)
	p, v := goldenAlbum(t, dir)
	first := goldenRun(t, p, v, filepath.Join(dir, "run1"))
	second := goldenRun(t, p, v, filepath.Join(dir, "run2"))
	if !slices.Equal(first, second) {
		t.Fatalf("два прогона экспорта дали разное:\n%s", goldenDiff(first, second))
	}

	if *updateGolden || os.Getenv("SA_GOLDEN_UPDATE") == "1" {
		data, err := json.MarshalIndent(first, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d файлов)", goldenPath, len(first))
		return
	}
	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): запустите go test ./internal/export -run Golden -update", err)
	}
	var want []goldenFile
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatalf("эталон не читается: %v", err)
	}
	if !slices.Equal(want, first) {
		t.Fatalf("экспорт разошёлся с эталоном %s (если изменение задумано — перезаписать: -update):\n%s",
			goldenPath, goldenDiff(want, first))
	}
}

// goldenAlbum собирает альбом 100×70 мм из трёх разворотов:
//  1. сетка: повёрнутое место, фото с поворотом на четверть и наклоном, зеркало, места
//     внахлёст с порядком слоёв не по умолчанию, свой цвет левой и правой страницы, текст с
//     поворотом, опущенный под фото, подпись под фото;
//  2. «Авто»: два фото (одно отражено по вертикали) на фото во весь разворот (отражено);
//  3. пустой — белый (фон альбома белый);
//  4. фото на фоне страниц с прозрачностью (017): левая 40 % под 30° над своим цветом,
//     правая 70 % без поворота. Добавлен последним: файлы разворотов 1–3 не меняются.
func goldenAlbum(t *testing.T, dir string) (*model.Project, model.View) {
	t.Helper()
	photo := func(id string, w, h int) model.PhotoInfo {
		return model.PhotoInfo{ID: id, Path: goldenJPEG(t, dir, id+".jpg", w, h), W: w, H: h}
	}
	e := mustEditor(t, model.New(), "")
	s := model.DefaultSettings()
	s.PageW, s.PageH, s.Margin, s.Gap = 100, 70, 5, 2
	e.SetSettings(s)

	must := func(v model.View, err error) model.View {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	v := must(e.NewSpread(0, []model.PhotoInfo{photo("a", 300, 200), photo("b", 200, 300), photo("c", 300, 200)}))
	s1 := v.Spreads[0].ID
	e.SetGrid(s1, true)
	// Ссылка на место — ID фото (placeRef): ID мест случайные.
	e.SetPlaces(s1, []model.PlaceBox{
		{ID: "a", X: 8, Y: 8, W: 70, H: 50},
		{ID: "b", X: 60, Y: 15, W: 45, H: 45},
		{ID: "c", X: 115, Y: 10, W: 70, H: 45},
	}, "")
	e.SetPlaceAngle(s1, "c", 8)
	e.TurnPhotos([]string{"b"}, 1)
	e.SetFrameAngle(s1, "b", 6.5, "")
	e.SetFlip([]string{"a"}, "h")
	e.SetFrame("c", 1.3, 0.4, -0.3)
	e.ArrangeLayers(s1, "back", []string{"b"}) // b был над a — теперь под ним
	e.SetBackground(s1, "left", "#efe6d8")
	e.SetBackground(s1, "right", "#2b3a4a")
	v = must(e.AddText(s1, 1, "t1"))
	tid := v.Spreads[0].Texts[len(v.Spreads[0].Texts)-1].ID
	e.SetText(tid, "Сосновый бор", "t1")
	e.SetTextStyle(tid, model.TextStyle{Font: "cormorant", Size: 18, Color: "#f5f0e6", Align: "center", Italic: true})
	e.SetTextBox(tid, 110, 52, 80)
	e.SetTextAngle(tid, -10)
	e.ArrangeLayers(s1, "down", []string{tid}) // текст уходит под фото c, хвост виден на фоне
	e.SetCaption("a", "Анна и Пётр", "c1")

	v = must(e.NewSpread(1, []model.PhotoInfo{photo("d", 300, 200), photo("e", 200, 300)}))
	s2 := v.Spreads[1].ID
	e.SetFlip([]string{"e"}, "v")
	must(e.SetBackdrop(s2, model.ScopeSpread, photo("f", 400, 250)))
	e.FlipBackdrop(s2, model.ScopeSpread, "h")

	must(e.NewSpread(2, nil))

	v = must(e.NewSpread(3, nil))
	s4 := v.Spreads[3].ID
	e.SetBackground(s4, "left", "#c81e1e")
	must(e.SetBackdrop(s4, model.ScopeLeft, photo("g", 300, 200)))
	e.SetBackdropBox(s4, model.ScopeLeft, 20, 5, 70, 50, 30, "")
	e.SetBackdropFade(s4, model.ScopeLeft, 0.4, "")
	must(e.SetBackdrop(s4, model.ScopeRight, photo("h", 200, 300)))
	e.SetBackdropFade(s4, model.ScopeRight, 0.7, "")

	p, v := e.Snapshot()
	goldenScene(t, v)
	return p, v
}

// goldenJPEG пишет синтетическое фото w×h: градиент с сеткой линий и диагоналями. Резкие края
// нужны, чтобы сдвиг геометрии на долю пикселя менял пиксели, а не тонул в плавном градиенте.
func goldenJPEG(t *testing.T, dir, name string, w, h int) string {
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
			case x < w/5 && y < h/5: // угол-метка: видно поворот и зеркало
				c = color.RGBA{230, 40, 30, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// goldenScene проверяет, что в альбоме действительно есть всё, что заявлено: иначе эталон
// записался бы с холостой правкой и ничего бы не сторожил.
func goldenScene(t *testing.T, v model.View) {
	t.Helper()
	if len(v.Spreads) != 4 {
		t.Fatalf("разворотов %d, ждали 4", len(v.Spreads))
	}
	s1, s2, s3, s4 := v.Spreads[0], v.Spreads[1], v.Spreads[2], v.Spreads[3]
	frame := func(s model.SpreadView, id string) model.FrameView {
		i := slices.Index(s.Photos, id)
		if i < 0 || i >= len(s.Frames) {
			t.Fatalf("нет фото %s в развороте: %v", id, s.Photos)
		}
		return s.Frames[i]
	}
	zOf := func(id string) int {
		for _, pc := range s1.Places {
			if pc.Photo == id {
				return pc.Z
			}
		}
		t.Fatalf("нет места с фото %s", id)
		return 0
	}
	var placeRot bool
	for _, pc := range s1.Places {
		placeRot = placeRot || pc.Rot != 0
	}
	switch {
	case !s1.Grid || len(s1.Places) != 3 || !placeRot:
		t.Fatalf("разворот 1: сетка из 3 мест с поворотом: %+v", s1.Places)
	case frame(s1, "b").Turn != 1 || frame(s1, "b").Tilt == 0 || !frame(s1, "a").FlipH:
		t.Fatalf("разворот 1: поворот, наклон, зеркало: %+v", s1.Frames)
	case zOf("b") >= zOf("a"):
		t.Fatalf("разворот 1: b должен лечь под a: z(a)=%d z(b)=%d", zOf("a"), zOf("b"))
	case s1.BgSet != [2]string{"#efe6d8", "#2b3a4a"}:
		t.Fatalf("разворот 1: цвет страниц %v", s1.BgSet)
	case len(s1.Texts) != 1 || s1.Texts[0].Rot == 0 || len(s1.Captions) != 1:
		t.Fatalf("разворот 1: текст с поворотом и подпись: %+v %+v", s1.Texts, s1.Captions)
	case s1.Texts[0].Z >= zOf("c"):
		t.Fatalf("разворот 1: текст должен лечь под c: z(текст)=%d z(c)=%d", s1.Texts[0].Z, zOf("c"))
	case s2.Grid || len(s2.Photos) != 2 || !frame(s2, "e").FlipV:
		t.Fatalf("разворот 2: «Авто» с отражённым фото: %+v", s2)
	case len(s2.Backdrops) != 1 || !s2.Backdrops[0].FlipH:
		t.Fatalf("разворот 2: фото на фоне с зеркалом: %+v", s2.Backdrops)
	case len(s3.Photos) != 0 || len(s3.Texts) != 0 || s3.Bg != [2]string{"#ffffff", "#ffffff"}:
		t.Fatalf("разворот 3: пустой белый: %+v", s3)
	case len(s4.Backdrops) != 2 || s4.Backdrops[0].Fade != 0.4 || s4.Backdrops[0].Box.Rot != 30 || s4.Backdrops[1].Fade != 0.7:
		t.Fatalf("разворот 4: фоны с прозрачностью: %+v", s4.Backdrops)
	}
}

// goldenRun выгружает альбом в четыре папки (вид × dpi) и описывает каждый файл.
func goldenRun(t *testing.T, p *model.Project, v model.View, root string) []goldenFile {
	t.Helper()
	var out []goldenFile
	for _, pages := range []bool{false, true} {
		for _, dpi := range []int{300, 72} {
			mode := "spreads"
			if pages {
				mode = "pages"
			}
			sub := mode + "-" + strconv.Itoa(dpi)
			dir := filepath.Join(root, sub)
			n := len(v.Spreads)
			pl := NewPlan(nil, n, pages)
			opt := render.Options{PageW: p.Settings.PageW, PageH: p.Settings.PageH, DPI: dpi, Quality: 95}
			if err := CheckCanvases(opt, pages); err != nil {
				t.Fatal(err)
			}
			if err := RunPlan(context.Background(), FromView(p, v), pl, opt, dir, nil, func(int, int) {}); err != nil {
				t.Fatalf("%s: %v", sub, err)
			}
			for _, j := range pl.Jobs {
				for _, name := range j.Names(n) {
					out = append(out, goldenDescribe(t, filepath.Join(dir, name), sub+"/"+name, j.Index+1))
				}
			}
		}
	}
	slices.SortFunc(out, func(a, b goldenFile) int { return strings.Compare(a.File, b.File) })
	return out
}

// goldenDescribe читает файл экспорта: размер, dpi из JFIF и хеш пикселей RGBA.
func goldenDescribe(t *testing.T, path, name string, spread int) goldenFile {
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
	return goldenFile{File: name, Spread: spread, W: b.Dx(), H: b.Dy(), DPI: jfifDPI(data), RGBA: hex.EncodeToString(sum[:])}
}

// jfifDPI — плотность из APP0 JFIF сразу после SOI (так пишет saveJPEG); 0 — сегмента нет
// или единицы не дюймы, -1 — плотность по осям разная.
func jfifDPI(data []byte) int {
	if len(data) < 18 || !bytes.Equal(data[2:4], []byte{0xFF, 0xE0}) || string(data[6:11]) != "JFIF\x00" || data[13] != 1 {
		return 0
	}
	x, y := binary.BigEndian.Uint16(data[14:]), binary.BigEndian.Uint16(data[16:])
	if x != y {
		return -1
	}
	return int(x)
}

// goldenDiff — какие файлы разошлись (с номером разворота), каких не хватает и какие лишние.
func goldenDiff(want, got []goldenFile) string {
	byName := func(fs []goldenFile) map[string]goldenFile {
		m := make(map[string]goldenFile, len(fs))
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
			lines = append(lines, fmt.Sprintf("  %s (разворот %d): файла нет", w.File, w.Spread))
		case g != w:
			lines = append(lines, fmt.Sprintf("  %s (разворот %d): ждали %d×%d px, %d dpi, %.12s…; получили %d×%d px, %d dpi, %.12s…",
				w.File, w.Spread, w.W, w.H, w.DPI, w.RGBA, g.W, g.H, g.DPI, g.RGBA))
		}
	}
	for _, g := range got {
		if _, ok := wm[g.File]; !ok {
			lines = append(lines, fmt.Sprintf("  %s (разворот %d): лишний файл", g.File, g.Spread))
		}
	}
	return strings.Join(lines, "\n")
}
