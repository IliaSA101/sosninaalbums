package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Скрипт проверки набора (спека 129 Т4, scripts/library-verify*.go): собирается и идёт на
// синтетическом наборе. Лежит под ignore, precheck его не собирает — без этого теста он сгнил бы.

// lvTool — собранный скрипт.
type lvTool struct{ exe string }

// buildLibVerify собирает скрипт (четыре файла одного package main); без go или на лёгком уровне — пропуск.
func buildLibVerify(t *testing.T) lvTool {
	t.Helper()
	return buildLibVerifyOf(t, "library-verify.go", "library-verify-sheets.go", "library-verify-looks.go", "library-verify-sprite.go")
}

// buildLibVerifyOf — сборка из файлов scripts/files (сборка для меток 0.26/0.27 — без третьего и четвёртого).
func buildLibVerifyOf(t *testing.T, files ...string) lvTool {
	t.Helper()
	if testkit.Lite() {
		t.Skip("сборка скрипта — полный уровень")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("нет go в PATH")
	}
	dir, err := filepath.Abs(filepath.Join("..", "..", "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	g := lvTool{exe: filepath.Join(testkit.TempDir(t), "library-verify")}
	if runtime.GOOS == "windows" {
		g.exe += ".exe"
	}
	args := []string{"build", "-o", g.exe}
	for _, f := range files {
		args = append(args, filepath.Join(dir, f))
	}
	if b, err := exec.Command(goBin, args...).CombinedOutput(); err != nil {
		t.Fatalf("сборка: %v\n%s", err, b)
	}
	return g
}

// run — запуск с временной библиотекой в своей папке; код выхода и весь вывод.
func (g lvTool) run(t *testing.T, args ...string) (string, int) {
	t.Helper()
	full := append([]string{"-tmp", testkit.TempDir(t)}, args...)
	b, err := exec.Command(g.exe, full...).CombinedOutput()
	if err == nil {
		return string(b), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(b), ee.ExitCode()
	}
	t.Fatalf("запуск: %v", err)
	return "", -1
}

// lvKey — ключ картинки библиотеки: my: + первые 8 байт sha256.
func lvKey(data []byte) string {
	s := sha256.Sum256(data)
	return "my:" + hex.EncodeToString(s[:8])
}

func lvBg(t *testing.T, w, h int, tone uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{tone, 0xc8, 0xd8, 255}), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 60}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func lvPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// lvSticker — круг с альфой, 800×800; tone различает файлы.
func lvSticker(t *testing.T, tone uint8) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 800, 800))
	for y := range 800 {
		for x := range 800 {
			if dx, dy := x-400, y-400; dx*dx+dy*dy < 380*380 {
				img.SetNRGBA(x, y, color.NRGBA{tone, 0x60, 0x90, 255})
			}
		}
	}
	return lvPNG(t, img)
}

// lvFrame — рамка 600×600: кольцо толщиной ring, margin прозрачного поля снаружи (рамка не вровень),
// blob — пятно другого цвета посреди каждой планки (рамка «тянется»), solid — без окна.
type lvFrameOpts struct {
	ring, margin int
	blob, solid  bool
}

func lvFrame(t *testing.T, o lvFrameOpts) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 600, 600))
	grey, red := color.NRGBA{0x90, 0x90, 0x96, 255}, color.NRGBA{0xd0, 0x30, 0x30, 255}
	for y := o.margin; y < 600-o.margin; y++ {
		for x := o.margin; x < 600-o.margin; x++ {
			inner := x >= o.margin+o.ring && x < 600-o.margin-o.ring && y >= o.margin+o.ring && y < 600-o.margin-o.ring
			if inner && !o.solid {
				continue
			}
			c := grey
			// Пятно в средних 40 % каждой планки, вдоль стороны (между углами).
			mid := func(v int) bool { return v >= 240 && v < 360 }
			if o.blob && (mid(x) && (y < o.margin+o.ring || y >= 600-o.margin-o.ring) || mid(y) && (x < o.margin+o.ring || x >= 600-o.margin-o.ring)) {
				c = red
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return lvPNG(t, img)
}

// lvTpl — шаблон: разворот (pages 2) или страница с двумя местами и угловым стикером. anchor — DX/DY
// якоря стикера (0 — центр в углу листа: часть стикера вне).
func lvTpl(t *testing.T, pages int, bg, st string, anchor float64) pfile {
	places := []model.Box{{X: 0.1, Y: 0.1, W: 0.35, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.35, H: 0.8}}
	page := []model.TplBg{{Lib: bg}, {Lib: bg}}
	ratio := 2.0
	if pages == 1 {
		places, page, ratio = places[:1], page[:1], 0.67
	}
	tpl := model.SpreadTpl{V: model.TplV, Pages: pages, Ratio: ratio, Places: places, Page: page}
	if st != "" {
		tpl.Stickers = []model.TplItem{{Anchor: model.Anchor{AX: -1, AY: -1, DX: anchor, DY: anchor}, W: 0.2, H: 0.2, Lib: st}}
	}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	return pfile{name: "spreads/t" + itoa(pages) + "-" + hex.EncodeToString([]byte(bg + st))[:8] + ".json", kind: "spread", data: data}
}

// lvGood — годные предметы: фон 209 dpi на 40×30, стикер, рамка, два шаблона (разворот и страница).
func lvGood(t *testing.T) (files []pfile, bg, st string) {
	b, s := lvBg(t, 3300, 2475, 0xe0), lvSticker(t, 0xd0)
	bg, st = lvKey(b), lvKey(s)
	files = []pfile{
		{name: "bg/sky.jpg", kind: "bg", data: b},
		{name: "stickers/dot.png", kind: "sticker", data: s},
		{name: "frames/plain.png", kind: "frame", data: lvFrame(t, lvFrameOpts{ring: 120})},
		lvTpl(t, 2, bg, st, 0.15), lvTpl(t, 1, bg, st, 0.15),
	}
	return files, bg, st
}

func countRe(t *testing.T, re, text string) int {
	t.Helper()
	return len(regexp.MustCompile(re).FindAllString(text, -1))
}

// lvEnv — собранный скрипт и годные предметы, общие для проверок.
type lvEnv struct {
	g      lvTool
	files  []pfile
	bg, st string
}

func TestLibraryVerifyScript(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, st := lvGood(t)
	e := lvEnv{g: g, files: files, bg: bg, st: st}
	t.Run("годный набор и листы", e.good)
	t.Run("несколько наборов: донор и дополнение", e.donor)
	t.Run("все провалы за один прогон", e.allFails)
	t.Run("рамка тянется — флаг, не провал", e.stretchy)
	t.Run("мера «тянется» на границе допуска", e.stretchTolerance)
	t.Run("плохие входы — ошибка, не паника", e.badInputs)
	t.Run("pull: шаблоны из набора", e.pull)
}

// Годный набор и листы
func (e lvEnv) good(t *testing.T) {
	out, prev := testkit.TempDir(t), filepath.Join(testkit.TempDir(t), "preview.png")
	text, code := e.g.run(t, "-out", out, "-preview", prev, packZip(t, "synth", 1, e.files...))
	if code != 0 || !strings.HasSuffix(strings.TrimSpace(text), "OK") {
		t.Fatalf("код %d\n%s", code, text)
	}
	// Ровная рамка не тянется; лист рамок и общий лист читаются как PNG.
	if !strings.Contains(text, "тянется 0%") || strings.Contains(text, "FLAG") || !strings.Contains(text, "в библиотеке: synth v1") {
		t.Errorf("вывод:\n%s", text)
	}
	for _, f := range []string{filepath.Join(out, "frames.png"), prev} {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := png.DecodeConfig(fh)
		_ = fh.Close()
		if err != nil || cfg.Width < 300 || cfg.Height < 200 {
			t.Errorf("%s: %v %+v", filepath.Base(f), err, cfg)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(out, "fail-*")); len(m) != 0 {
		t.Errorf("снимки провала у годного набора: %v", m)
	}
}

// Несколько наборов: донор и дополнение
func (e lvEnv) donor(t *testing.T) {
	donor := packZip(t, "donor", 1, e.files[0], e.files[1])
	addon := packZip(t, "addon-layouts", 1, lvTpl(t, 2, e.bg, e.st, 0.15))
	text, code := e.g.run(t, addon)
	if code != 1 || !strings.Contains(text, "не из набора") {
		t.Fatalf("дополнение без донора прошло: код %d\n%s", code, text)
	}
	text, code = e.g.run(t, "-with", donor, addon)
	if code != 0 || !strings.Contains(text, "донор: donor-1.zip") || !strings.Contains(text, "в библиотеке: addon-layouts v1") {
		t.Fatalf("с донором: код %d\n%s", code, text)
	}
	// Два набора по порядку в одной библиотеке.
	text, code = e.g.run(t, donor, packZip(t, "second", 1, lvTpl(t, 1, e.bg, e.st, 0.15)))
	if code != 0 || !strings.Contains(text, "в библиотеке: donor") || !strings.Contains(text, "в библиотеке: second") {
		t.Fatalf("два набора: код %d\n%s", code, text)
	}
}

// Все провалы за один прогон
func (e lvEnv) allFails(t *testing.T) {
	low := lvBg(t, 1200, 900, 0xc0) // 76 dpi
	out := testkit.TempDir(t)
	bad := []pfile{
		{name: "bg/sky.jpg", kind: "bg", data: e.files[0].data},
		{name: "bg/low.jpg", kind: "bg", data: low},
		{name: "stickers/dot.png", kind: "sticker", data: e.files[1].data},
		{name: "frames/margin.png", kind: "frame", data: lvFrame(t, lvFrameOpts{ring: 120, margin: 60})},
		{name: "frames/solid.png", kind: "frame", data: lvFrame(t, lvFrameOpts{ring: 120, solid: true})},
		lvTpl(t, 2, lvKey(low), e.st, 0.15), // LowDPI на трёх страницах
		lvTpl(t, 1, e.bg, e.st, 0),          // стикер центром в углу листа
		lvTpl(t, 2, e.bg, e.st, 0.15),       // годный между плохими
	}
	text, code := e.g.run(t, "-out", out, packZip(t, "badset", 1, bad...))
	if code != 1 {
		t.Fatalf("код %d\n%s", code, text)
	}
	// Каждый блок виден в итоге: рамка без окна (StagePack), поле у рамки, LowDPI ×3, стикер вне листа.
	if n := countRe(t, `(?m)^ - .*LowDPI`, text); n != 3 {
		t.Errorf("LowDPI в итоге %d, ждали 3\n%s", n, text)
	}
	for _, want := range []string{"StagePack пропустил", "не вровень с краем", "вне листа"} {
		if !strings.Contains(text, want) {
			t.Errorf("нет провала %q\n%s", want, text)
		}
	}
	if m, _ := filepath.Glob(filepath.Join(out, "fail-*.png")); len(m) < 4 { // 3 LowDPI + стикер вне листа (минимум на одном размере)
		t.Errorf("снимков провала %d: %v", len(m), m)
	}
}

// Рамка тянется — флаг, не провал
func (e lvEnv) stretchy(t *testing.T) {
	fr := pfile{name: "frames/blob.png", kind: "frame", data: lvFrame(t, lvFrameOpts{ring: 150, blob: true})}
	text, code := e.g.run(t, packZip(t, "stretchy", 1, fr))
	if code != 0 || !strings.Contains(text, "FLAG") || !regexp.MustCompile(`тянется (2\d|3\d)%`).MatchString(text) {
		t.Fatalf("код %d\n%s", code, text)
	}
}

// Мера «тянется» — копия devtools/sheetcut (frame.go, unevenTol = 20): те же файлы на границе допуска
// держат обе копии. До 20 включительно пятно не считается, с 21 — четыре пятна из 270 000 px кольца
// (26,67 % → «27»). Мутация допуска или канала в скрипте ломает этот тест, в sheetcut — TestStretchToleranceBoundary.
func (e lvEnv) stretchTolerance(t *testing.T) {
	dir := filepath.Join("..", "devtools", "sheetcut", "testdata")
	want := map[int]string{15: "0", 20: "0", 21: "27", 25: "27"}
	var files []pfile
	for _, d := range []int{15, 20, 21, 25} {
		data, err := os.ReadFile(filepath.Join(dir, "stretch-"+itoa(d)+".png"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, pfile{name: "frames/stretch-" + itoa(d) + ".png", kind: "frame", data: data})
	}
	text, code := e.g.run(t, packZip(t, "tolerance", 1, files...))
	if code != 0 {
		t.Fatalf("код %d\n%s", code, text)
	}
	for i, d := range []int{15, 20, 21, 25} {
		row := regexp.MustCompile(`(?m)^ +рамка ` + itoa(i+1) + ` .*: тянется (\d+)%(.*)$`).FindStringSubmatch(text)
		if row == nil {
			t.Errorf("stretch-%d: нет строки рамки %d\n%s", d, i+1, text)
			continue
		}
		if row[1] != want[d] {
			t.Errorf("stretch-%d: тянется %s%%, нужно %s%%", d, row[1], want[d])
		}
		if flagged := strings.Contains(row[2], "FLAG"); flagged != (want[d] != "0") {
			t.Errorf("stretch-%d: FLAG=%v при %s%%", d, flagged, want[d])
		}
	}
}

// Плохие входы — ошибка, не паника
func (e lvEnv) badInputs(t *testing.T) {
	dir := testkit.TempDir(t)
	good := packZip(t, "okset", 1, e.files...)
	raw, err := os.ReadFile(good)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	huge := write("huge.zip", nil)
	if err := os.Truncate(huge, 301<<20); err != nil { // больше pack.MaxZip: разреженный файл
		t.Fatal(err)
	}
	cases := map[string]string{
		"пустой":       write("empty.zip", nil),
		"усечённый":    write("cut.zip", raw[:len(raw)/2]),
		"не zip":       write("junk.zip", bytes.Repeat([]byte("не zip "), 400)),
		"огромный":     huge,
		"папка":        dir,
		"нет файла":    filepath.Join(dir, "nope.zip"),
		"усечён на 22": write("tail.zip", raw[:22]),
	}
	for name, p := range cases {
		text, code := e.g.run(t, p)
		if code != 1 || strings.Contains(text, "panic") || strings.Contains(text, "goroutine") || !strings.Contains(text, "ПРОВАЛ") {
			t.Errorf("%s: код %d\n%s", name, code, text)
		}
	}
	// Один плохой не мешает следующему: годный после битого ставится.
	if text, code := e.g.run(t, cases["пустой"], good); code != 1 || !strings.Contains(text, "в библиотеке: okset v1") {
		t.Errorf("годный после битого: код %d\n%s", code, text)
	}
	for name, args := range map[string][]string{
		"без аргументов":     nil,
		"относительный путь": {"rel.zip"},
		"относительный out":  {"-out", "rel", good},
		"-pull без -pack":    {"-pull", dir, "-out", filepath.Join(dir, "o")},
		"неизвестный ключ":   {"-nosuch", "0.26.0", good},
	} {
		if text, code := e.g.run(t, args...); code != 2 {
			t.Errorf("%s: код %d\n%s", name, code, text)
		}
	}
}

// Pull: шаблоны из набора
func (e lvEnv) pull(t *testing.T) {
	a := decorApp(t)
	if res, err := a.AddPackPath(packZip(t, "pullset", 1, e.files[:3]...)); err != nil || res.Ask != "" || len(res.Skipped) != 0 {
		t.Fatalf("набор: %v %+v", err, res)
	}
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	mine := func(name string, tpl pfile) {
		var tp model.SpreadTpl
		if err := json.Unmarshal(tpl.data, &tp); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveSpreadTpl("mine", name, tp); err != nil {
			t.Fatal(err)
		}
	}
	mine("Маринин из набора", lvTpl(t, 2, e.bg, e.st, 0.15))
	mine("Чужой фон", lvTpl(t, 1, "my:0123456789abcdef", e.st, 0.15))
	mine("Только цвет", pfile{data: []byte(`{"v":1,"pages":1,"ratio":0.67,"page":[{"color":"#ffffff"}],"places":[{"x":0.1,"y":0.1,"w":0.8,"h":0.8}]}`)})
	// Рамочки мест (131): своя рамка набора — забирается, чужая — нет (ключ borders[].lib учтён).
	framed := func(lib string) pfile {
		return pfile{data: []byte(`{"v":1,"pages":1,"ratio":0.67,"page":[{"lib":"` + e.bg + `"}],"places":[{"x":0.1,"y":0.1,"w":0.8,"h":0.8}],"borders":[{"i":0,"lib":"` + lib + `"}]}`)}
	}
	mine("Рамочка из набора", framed(lvKey(e.files[2].data)))
	mine("Чужая рамочка", framed("my:0123456789abcdef"))
	out := filepath.Join(testkit.TempDir(t), "pulled")
	lib := filepath.Join(a.cfgDir, "Библиотека")
	text, code := e.g.run(t, "-pull", lib, "-pack", "pullset", "-out", out)
	if code != 0 || !strings.Contains(text, "забрано шаблонов: 2") || strings.Contains(text, "Чужая рамочка") {
		t.Fatalf("код %d\n%s", code, text)
	}
	m, _ := filepath.Glob(filepath.Join(out, "*.json"))
	if len(m) != 2 {
		t.Fatalf("файлов %v", m)
	}
	for _, f := range m {
		data, _ := os.ReadFile(f)
		if _, err := model.ParseSpreadTpl(data); err != nil {
			t.Errorf("забранный шаблон %s не читается: %v", filepath.Base(f), err)
		}
	}
	if text, code := e.g.run(t, "-pull", lib, "-pack", "net-takogo", "-out", out); code != 1 {
		t.Errorf("чужой id: код %d\n%s", code, text)
	}
}
