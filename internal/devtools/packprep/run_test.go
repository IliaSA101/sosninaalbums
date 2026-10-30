package packprep

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/devtest"
	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
)

// Пустые -src и -out — ошибка, корень диска и чужая непустая папка — отказ; ничего не записано.
func TestOutRequired(t *testing.T) {
	src := tempDir(t)
	rec := baseRecipe(bgItem("a.jpg", "bg/a.jpg"))
	for name, cfg := range map[string]Config{
		"пустой out":   {Src: src, Out: ""},
		"пробелы out":  {Src: src, Out: "  "},
		"пустой src":   {Src: "", Out: filepath.Join(src, "o")},
		"корень":       {Src: src, Out: string(filepath.Separator)},
		"src не папка": {Src: filepath.Join(src, "нет"), Out: filepath.Join(tempDir(t), "o")},
	} {
		if _, err := Run(cfg, rec); err == nil {
			t.Errorf("%s: ждали ошибку", name)
		}
	}
	foreign := tempDir(t)
	writeFile(t, filepath.Join(foreign, "diploma.docx"), []byte("не трогать"))
	if _, err := Run(Config{Src: src, Out: foreign}, rec); err == nil || !strings.Contains(err.Error(), "не похожа на папку набора") {
		t.Fatalf("чужая папка: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(foreign, "diploma.docx")); string(b) != "не трогать" {
		t.Fatal("чужой файл тронут")
	}
}

// Источник вне -src (ссылка наружу) — ошибка этого предмета; соседние предметы собираются.
func TestSrcInsideOnly(t *testing.T) {
	root := tempDir(t)
	src := filepath.Join(root, "src")
	writePNG(t, filepath.Join(root, "secret.png"), canvas(50, 50, navy))
	writePNG(t, filepath.Join(src, "ok.png"), stickerSheet())
	if err := os.Symlink(filepath.Join(root, "secret.png"), filepath.Join(src, "link.png")); err != nil {
		t.Skip("ссылки недоступны:", err)
	}
	sheet := func(s, out string) Item { return Item{Src: s, Mode: ModeSheet, Out: out, Names: []string{"А", "Б"}} }
	rep, _ := run(t, src, baseRecipe(sheet("link.png", "stickers/l-"), sheet("ok.png", "stickers/ok-"), sheet("нет.png", "stickers/n-")))
	if !hasError(rep, "link.png: исходник") || !hasError(rep, "нет.png") {
		t.Fatalf("ждали ошибки link.png и нет.png: %v", rep.Errors)
	}
	if len(rep.Items) != 2 {
		t.Fatalf("независимые блоки: ждали 2 стикера, получили %d", len(rep.Items))
	}
}

// Лист стикеров → файлы, имена, манифест, лист проверки, report.json.
func TestRunSheetEndToEnd(t *testing.T) {
	src := tempDir(t)
	writePNG(t, filepath.Join(src, "sheets/s.png"), stickerSheet())
	rec := baseRecipe(Item{Src: "sheets/s.png", Mode: ModeSheet, Out: "stickers/s-", Style: "Рисунки", Names: []string{"Диск", "Квадрат"}, Grid: "2x1"})
	rep, out := run(t, src, rec)
	if !rep.OK || len(rep.Items) != 2 {
		t.Fatalf("ждали 2 стикера без ошибок: %v %v", rep.Items, rep.Errors)
	}
	if o := rep.Items[0]; o.File != "stickers/s-01.png" || o.Name != "Диск" || o.N != 1 || o.Style != "Рисунки" || o.Family != "s" || o.Theme != "school" {
		t.Fatalf("первый предмет: %+v", o)
	}
	img := decodePNG(t, readOut(t, out, "stickers/s-02.png"))
	if b := img.Bounds(); b.Dx() < 70 || b.Dx() > 90 {
		t.Fatalf("квадрат 80 px обрезан до %v", b)
	}
	man := string(readOut(t, out, "manifest.src.json"))
	for _, want := range []string{`"id": "test-pack"`, `"file": "stickers/s-01.png"`, `"kind": "sticker"`, `"name": "Квадрат"`, `"theme": "school"`, `"license": "CC0-1.0"`} {
		if !strings.Contains(man, want) {
			t.Errorf("в manifest.src.json нет %s:\n%s", want, man)
		}
	}
	if !strings.Contains(string(readOut(t, out, "LICENSE.txt")), "CC0 1.0") {
		t.Error("LICENSE.txt без CC0")
	}
	for _, f := range []string{"items-dark.png", "items-color.png", "items-big-dark.png", "edges.png"} {
		if b := decodePNG(t, readOut(t, out, "_check/"+f)).Bounds(); b.Dx() < 100 || b.Dy() < 100 {
			t.Errorf("%s: %v", f, b)
		}
	}
	if _, err := os.Stat(filepath.Join(out, TmpDir)); !os.IsNotExist(err) {
		t.Error(".tmp остался после прогона")
	}
	// Имена короче нарезки — ошибка, но предметы и лист проверки есть.
	rec.Items[0].Names = []string{"Диск"}
	rep, out = run(t, src, rec)
	if rep.OK || !hasError(rep, "нет имени") || len(rep.Items) != 2 {
		t.Fatalf("короткие names: %v", rep.Errors)
	}
	readOut(t, out, "_check/items-dark.png")
}

// skip «src#N» убирает один предмет листа.
func TestSkipOneSticker(t *testing.T) {
	src := tempDir(t)
	writePNG(t, filepath.Join(src, "s.png"), stickerSheet())
	rec := baseRecipe(Item{Src: "s.png", Mode: ModeSheet, Out: "st/s-", Names: []string{"Диск", "Квадрат"}, Grid: "2x1"})
	rec.Skip = map[string]string{"s.png#1": "диск кривой"}
	rep, _ := run(t, src, rec)
	if len(rep.Items) != 1 || rep.Items[0].Name != "Квадрат" || len(rep.Skipped) != 1 {
		t.Fatalf("ждали один квадрат и один пропуск: %+v %v", rep.Items, rep.Skipped)
	}
}

// Рамка с неровным центром, фон с апскейлом, мелкий стикер, кайма и дубль попадают в флаги отчёта.
func TestReportFlags(t *testing.T) {
	src := tempDir(t)
	// Рамка: тонкая планка по краю и «узор» в центре, окно найдено, тянется > 20 %.
	fr := canvas(300, 300, white)
	fillRect(fr, image.Rect(10, 10, 290, 290), navy)
	fillRect(fr, image.Rect(40, 40, 260, 260), white)
	fillRect(fr, image.Rect(70, 70, 230, 130), coral) // узор в окне: центр 9-среза тянется
	writePNG(t, filepath.Join(src, "f/fr.png"), fr)
	lite := devtest.Lite() // быстрый уровень: фон 900 → 3602 px (апскейл-флаг) проверяет TestBgUpscale
	if !lite {
		writeJPEG(t, filepath.Join(src, "bg/small.jpg"), texture(900, 900))
	}
	// Стикер с белой каймой и его же повёрнутое отражение — дубль.
	haloed := canvas(400, 200, transp)
	disc(haloed, 100, 100, 70, color.NRGBA{255, 255, 255, 255})
	disc(haloed, 100, 100, 62, navy)
	fillRect(haloed, image.Rect(100, 90, 130, 110), coral)
	disc(haloed, 300, 100, 70, color.NRGBA{255, 255, 255, 255})
	disc(haloed, 300, 100, 62, navy)
	fillRect(haloed, image.Rect(290, 100, 310, 130), coral) // то же, повёрнутое на 90°
	writePNG(t, filepath.Join(src, "s/halo.png"), haloed)
	items := []Item{
		{Src: "f/fr.png", Mode: ModeFrame, Out: "frames/fr.png", Name: "Рамка"},
		bgItem("bg/small.jpg", "bg/small.jpg"),
		{Src: "s/halo.png", Mode: ModeSheet, Out: "st/h-", Names: []string{"Круг", "Круг повёрнутый"}, Grid: "2x1", BG: "none"}}
	minFlags := 8 // кайма, мало пикселей и дубль у двух стикеров, рамка ×2, фон
	if lite {
		items = append(items[:1], items[2:]...)
		minFlags = 7
	}
	rec := baseRecipe(items...)
	rep, _ := run(t, src, rec)
	if !rep.OK {
		t.Fatal(rep.Errors)
	}
	fr1 := byFile(t, rep, "frames/fr.png")
	if fr1.Stretch == nil || *fr1.Stretch <= StretchFlagTest || !hasFlag(fr1, "тянется") {
		t.Errorf("рамка: %v %v", fr1.Stretch, fr1.Flags)
	}
	if !lite && !hasFlag(byFile(t, rep, "bg/small.jpg"), "апскейл ×4.00 от 900 px") {
		t.Errorf("фон: %v", byFile(t, rep, "bg/small.jpg").Flags)
	}
	h1, h2 := byFile(t, rep, "st/h-01.png"), byFile(t, rep, "st/h-02.png")
	if !hasFlag(h1, "кайма?") || !hasFlag(h1, "мало пикселей") {
		t.Errorf("стикер 1: %v", h1.Flags)
	}
	if h2.Similar != h1.N || !hasFlag(h2, "похоже на №") {
		t.Errorf("повёрнутый дубль не найден: %v", h2.Flags)
	}
	if len(rep.Flags) < minFlags {
		t.Errorf("сводка флагов: %v", rep.Flags)
	}
}

// StretchFlagTest — порог из спеки (20 %), чтобы тест не зависел от внутренней константы sheetcut.
const StretchFlagTest = 20.0

// Один и тот же рецепт даёт те же байты во всех файлах набора и листов проверки.
func TestRecipeDeterministic(t *testing.T) {
	src := tempDir(t)
	writePNG(t, filepath.Join(src, "s.png"), stickerSheet())
	writeJPEG(t, filepath.Join(src, "bg.jpg"), texture(300, 300))
	rec := baseRecipe(
		Item{Src: "s.png", Mode: ModeSheet, Out: "st/s-", Names: []string{"Диск", "Квадрат"}, Grid: "2x1"},
		Item{Src: "bg.jpg", Mode: ModeBg, Out: "bg/b.jpg", Name: "Фон"})
	if devtest.Lite() {
		// Быстрый уровень: фон 300 → 3602 px считается секунду на прогон; вместо него второй лист
		// (то же правило — вторая запись, исчезающая при повторном прогоне).
		rec.Items[1] = Item{Src: "s.png", Mode: ModeSheet, Out: "bg/b-", Names: []string{"Диск", "Квадрат"}, Grid: "2x1", Style: "Другой"}
	}
	_, a := run(t, src, rec)
	_, b := run(t, src, rec)
	var n int
	_ = filepath.Walk(a, func(p string, st os.FileInfo, err error) error {
		if err != nil || st.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(a, p)
		n++
		x, _ := os.ReadFile(p)
		y, err := os.ReadFile(filepath.Join(b, rel))
		if err != nil || !bytes.Equal(x, y) {
			t.Errorf("%s различается между прогонами", rel)
		}
		return nil
	})
	if n < 8 {
		t.Fatalf("сравнили только %d файлов", n)
	}
	// Повторный прогон в ту же папку заменяет прежний вывод без остатков.
	if _, err := os.Stat(filepath.Join(a, rec2Out(devtest.Lite()))); err != nil {
		t.Fatalf("второй записи нет после первого прогона: %v", err)
	}
	rec.Items = rec.Items[:1]
	if _, err := Run(Config{Src: src, Out: a}, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a, rec2Out(devtest.Lite()))); !os.IsNotExist(err) {
		t.Error("остался файл прошлого прогона")
	}
}

// rec2Out — файл второй записи рецепта TestRecipeDeterministic: фон или (быстрый уровень) стикер листа.
func rec2Out(lite bool) string {
	if lite {
		return "bg/b-01.png"
	}
	return "bg/b.jpg"
}

// Бюджет по видам, на стиль и на набор.
func TestBudget(t *testing.T) {
	if devtest.Lite() {
		// Быстрый уровень: 13 настоящих фонов — это 13 раз Lanczos до 3602 px; бюджет считает отчёт,
		// поэтому здесь отчёт собран напрямую, а прогон целиком — в полном уровне.
		r := &Report{}
		for i := range 13 {
			r.Items = append(r.Items, &Output{Kind: KindBg, N: i + 1})
		}
		if r.budget(); !hasError(r, "фонов 13, бюджет набора — не больше 12") || r.Counts[KindBg] != 13 || r.Names != 13 {
			t.Fatalf("бюджет фонов: %v %v %d", r.Errors, r.Counts, r.Names)
		}
	} else {
		src := tempDir(t)
		writeJPEG(t, filepath.Join(src, "b.jpg"), canvas(64, 64, navy))
		var items []Item
		for i := range 13 {
			items = append(items, Item{Src: "b.jpg", Mode: ModeBg, Out: "bg/b" + string(rune('a'+i)) + ".jpg", Name: "Фон", Family: "bg-" + string(rune('a'+i))})
		}
		rep, _ := run(t, src, baseRecipe(items...))
		if !hasError(rep, "фонов 13, бюджет набора — не больше 12") {
			t.Fatalf("бюджет фонов: %v", rep.Errors)
		}
		if rep.Counts[KindBg] != 13 || rep.Names != 13 {
			t.Fatalf("счётчики: %v %d", rep.Counts, rep.Names)
		}
	}
	r := &Report{Items: nil}
	for i := range 37 {
		r.Items = append(r.Items, &Output{Kind: KindSticker, Style: "Значки", N: i + 1})
	}
	r.budget()
	if len(r.Errors) != 1 || !strings.Contains(r.Errors[0], "в стиле «Значки» 37") {
		t.Fatalf("бюджет стиля: %v", r.Errors)
	}
	r = &Report{}
	for i := range 107 {
		r.Items = append(r.Items, &Output{Kind: KindSticker, Style: "С" + string(rune('A'+i%3)), N: i + 1})
	}
	if r.budget(); !hasError(r, "имён в наборе 107") {
		t.Fatalf("бюджет имён: %v", r.Errors)
	}
}

// SVG: растр подменён, предметы нарезаны svgcut, имена и файлы по правилам; без браузера — ошибка.
func TestSvgModeFakeRender(t *testing.T) {
	src := tempDir(t)
	data, err := os.ReadFile(filepath.Join("testdata", "sheet.svg"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(src, "v/sheet.svg"), data)
	it := Item{Src: "v/sheet.svg", Mode: ModeSvg, Out: "stickers/z-", Style: "Значки", Names: []string{"Раз", "Два", "Три"}}
	fake := func(svg []byte, w, h int) (*image.NRGBA, error) {
		img := canvas(w, h, transp)
		fillRect(img, image.Rect(3, 3, w-3, h-3), coral)
		return img, nil
	}
	out := filepath.Join(tempDir(t), "pack")
	rep, err := Run(Config{Src: src, Out: out, Render: svgcut.RenderFunc(fake)}, baseRecipe(it))
	if err != nil || !rep.OK || len(rep.Items) != 3 {
		t.Fatalf("svg: %v %v %v", err, rep.Errors, len(rep.Items))
	}
	if rep.Items[2].File != "stickers/z-03.png" || rep.Items[2].Name != "Три" {
		t.Fatalf("третий: %+v", rep.Items[2])
	}
	rep, _ = run(t, src, baseRecipe(it))
	if !hasError(rep, "браузер не задан") {
		t.Fatalf("без браузера: %v", rep.Errors)
	}
}

// rows с неверным n не молчит: в report.json — сколько рядов видно по промежуткам и полос альфы.
func TestRowsWrongNHint(t *testing.T) {
	src := tempDir(t)
	img := canvas(300, 200, transp)
	for _, y := range []int{10, 36, 86, 114, 154} {
		fillRect(img, image.Rect(20, y, 280, y+20), navy)
	}
	writePNG(t, filepath.Join(src, "d.png"), img)
	for n, want := range map[int]bool{2: true, 3: false} {
		rep, _ := run(t, src, baseRecipe(Item{Src: "d.png", Mode: ModeRows, Out: "div/d-", N: n, Names: []string{"А", "Б", "В"}[:min(n, 3)], BG: "none"}))
		got := false
		for _, note := range rep.Notes {
			got = got || strings.Contains(note, "видно 3 (полос альфы 5)")
		}
		if got != want {
			t.Errorf("n=%d: подсказка=%v, ждали %v: %v %v", n, got, want, rep.Notes, rep.Errors)
		}
	}
}
