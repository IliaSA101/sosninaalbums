package app

import (
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Макеты альбома в library-verify (спека 131 §7, приёмка C1): места шагов на 20×30, 30×30 и 40×30
// через LookBoxes, пропорция — WARN на эталоне Ratio шага. Набор макетов — минимум 0.28.0 и id на -layouts.

// lvStep — шаг-разворот (pages 2) или страница (pages 1) с фоном bg и местами в долях области;
// ratio — эталон отношения сторон области, при котором нарисованы места.
func lvStep(pages int, ratio float64, bg string, places ...model.Box) model.LookStep {
	page := []model.TplBg{{Lib: bg}, {Lib: bg}}[:pages]
	return model.LookStep{Tpl: model.SpreadTpl{V: model.TplV, Pages: pages, Ratio: ratio, Places: places, Page: page}}
}

// lvLook — файл макета альбома для набора.
func lvLook(t *testing.T, name string, cover *model.CoverTpl, steps ...model.LookStep) pfile {
	t.Helper()
	data, err := json.Marshal(model.AlbumTpl{V: model.LookV, Cover: cover, Steps: steps})
	if err != nil {
		t.Fatal(err)
	}
	return pfile{name: "layouts/" + name + ".json", kind: "album", data: data}
}

// lvLayouts — набор макетов minApp 0.28.0; запуск — с -app 0.28.0 (wails.json дерева может быть старше).
func lvLayouts(t *testing.T, id string, files ...pfile) string {
	t.Helper()
	return patchManifest(t, packZip(t, id, 1, files...), map[string]any{"minApp": "0.28.0"})
}

// Предупреждение о пропорции мест шага (решение 0.27-131-24 для макетов): меньше 80 % мест, где кадр
// 3:2 / 2:3 режется не больше чем на 15 %, на эталоне Ratio шага — WARN с номером шага; код выхода прежний.
func TestVerifyLookAspectWarn(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, _ := lvGood(t)
	bgFile := files[0]
	// Разворот Ratio 2: отношение места = w·2/h.
	portrait := model.Box{X: 0.11, Y: 0.08, W: 0.28, H: 0.84}  // 2:3
	landscape := model.Box{X: 0.60, Y: 0.08, W: 0.30, H: 0.40} // 3:2
	square := model.Box{X: 0.05, Y: 0.1, W: 0.25, H: 0.5}      // 1,0: режет 33 %
	edge := model.Box{X: 0.05, Y: 0.1, W: 0.3825, H: 0.6}      // 1,275: ровно 15 %
	slightly := model.Box{X: 0.05, Y: 0.1, W: 0.35, H: 0.5}    // 1,4: 7 %
	oval := square
	oval.Shape = model.ShapeOval
	good := lvStep(2, 2, bg, portrait, landscape)
	vig := lvStep(2, 2, bg)
	vig.Role, vig.Vig = model.RoleVignette, &model.LookVig{Tpl: model.DefaultVignette().Tpl, Teachers: 1, Pupils: 24}

	cases := []struct {
		name  string
		steps []model.LookStep
		warn  string // "" — предупреждения нет
	}{
		{"all-square", []model.LookStep{lvStep(2, 2, bg, square, square)}, "шаг 1: 2 из 2 мест не 3:2 / 2:3"},
		{"second-step", []model.LookStep{good, lvStep(2, 2, bg, portrait, square, square)}, "шаг 2: 2 из 3 мест не 3:2 / 2:3 (подходят 33%"},
		{"portrait-landscape", []model.LookStep{good}, ""},
		{"edge-15", []model.LookStep{lvStep(2, 2, bg, edge, portrait)}, ""},
		{"four-of-five", []model.LookStep{lvStep(2, 2, bg, portrait, landscape, slightly, edge, square)}, ""}, // ровно 80 %
		{"three-of-four", []model.LookStep{lvStep(2, 2, bg, portrait, landscape, slightly, square)}, "шаг 1: 1 из 4 мест"},
		{"oval-only", []model.LookStep{good, lvStep(2, 2, bg, oval, oval)}, ""}, // фигуры по замыслу автора не считаются
		{"empty-places", []model.LookStep{good, lvStep(2, 2, bg)}, ""},
		// Эталон Ratio: на Ratio 3 место 0,25×0,5 — 3:2, и LookBoxes сохраняет пропорцию на любой странице.
		{"ratio-3", []model.LookStep{lvStep(2, 3, bg, model.Box{X: 0.1, Y: 0.2, W: 0.25, H: 0.5})}, ""},
		{"ratio-3-not-fit", []model.LookStep{lvStep(2, 3, bg, model.Box{X: 0.1, Y: 0.2, W: 0.15, H: 0.5})}, "шаг 1: 1 из 1 мест не 3:2 / 2:3"}, // 0,9
		{"page-step", []model.LookStep{lvStep(1, 1, bg, model.Box{X: 0.1, Y: 0.2, W: 0.75, H: 0.5})}, ""},                                      // 1,5
		{"page-step-square", []model.LookStep{lvStep(1, 1, bg, model.Box{X: 0.1, Y: 0.1, W: 0.5, H: 0.5})}, "шаг 1: 1 из 1 мест не 3:2 / 2:3"},
		// Без ratio зажим ставит 1 (как программа): место 0,6×0,4 — 3:2 на эталоне 1 и 3:1 на 2.
		{"no-ratio", []model.LookStep{lvStep(2, 0, bg, model.Box{X: 0.2, Y: 0.3, W: 0.6, H: 0.4})}, ""},
		{"vignette-not-counted", []model.LookStep{good, vig}, ""}, // оформление виньетки без мест
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, code := g.run(t, "-app", "0.28.0", lvLayouts(t, "aspect-layouts", bgFile, lvLook(t, c.name, nil, c.steps...)))
			if code != 0 || !strings.HasSuffix(strings.TrimSpace(text), "OK") {
				t.Fatalf("предупреждение не должно менять код выхода: код %d\n%s", code, text)
			}
			n := countRe(t, `(?m)^ - WARN `, text)
			if c.warn == "" {
				if n != 0 || strings.Contains(text, "WARN") {
					t.Fatalf("лишнее предупреждение:\n%s", text)
				}
				return
			}
			if n != 1 || !strings.Contains(text, "ПРЕДУПРЕЖДЕНИЙ 1:") || !strings.Contains(text, "layouts/"+c.name+".json: "+c.warn) {
				t.Fatalf("нужно одно предупреждение %q с именем макета:\n%s", c.warn, text)
			}
		})
	}
	// Предупреждение каждого шага отдельно и рядом с провалом: код 1, оба в итоге.
	low := lvBg(t, 1200, 900, 0xc0)
	text, code := g.run(t, "-app", "0.28.0", lvLayouts(t, "aspect-layouts", pfile{name: "bg/low.jpg", kind: "bg", data: low},
		lvLook(t, "x", nil, lvStep(2, 2, lvKey(low), square), lvStep(2, 2, lvKey(low), square))))
	if code != 1 || countRe(t, `(?m)^ - WARN `, text) != 2 || !strings.Contains(text, "ПРОВАЛОВ") {
		t.Fatalf("провал и два предупреждения вместе: код %d\n%s", code, text)
	}
}

// Годный макет с обложкой, провалы за один прогон (LowDPI шага и стороны обложки на трёх страницах, стикер
// вне листа), правило набора макетов (id, minApp).
func TestLibraryVerifyLooks(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, st := lvGood(t)
	bgFile, stFile := files[0], files[1]
	two := []model.Box{{X: 0.11, Y: 0.08, W: 0.28, H: 0.84}, {X: 0.6, Y: 0.08, W: 0.3, H: 0.4}}                                                        // 2:3 и 3:2 при Ratio 2
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.4}}, Page: []model.TplBg{{Lib: bg}}} // 1,5
	cover := &model.CoverTpl{Bg: &model.TplBg{Lib: bg}, Front: &front}
	sticker := model.TplItem{W: 0.2, H: 0.2, Lib: st}
	withSticker := lvStep(2, 2, bg, two...)
	withSticker.Tpl.Stickers = []model.TplItem{sticker}

	text, code := g.run(t, "-app", "0.28.0", lvLayouts(t, "wedding-layouts", bgFile, stFile, lvLook(t, "ok", cover, lvStep(2, 2, bg, two...), withSticker)))
	if code != 0 || !strings.Contains(text, "шагов 2, сторон обложки 1, фон обложки true") || strings.Contains(text, "WARN") {
		t.Fatalf("годный макет: код %d\n%s", code, text)
	}

	low := lvBg(t, 1200, 900, 0xc0) // 76 dpi
	lowKey := lvKey(low)
	outside := withSticker
	outside.Tpl.Stickers = []model.TplItem{{Anchor: model.Anchor{AX: -1, AY: -1}, W: 0.2, H: 0.2, Lib: st}} // центр в углу листа
	badCover := &model.CoverTpl{Front: &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.4}}, Page: []model.TplBg{{Lib: lowKey}}}}
	out := testkit.TempDir(t)
	bad := lvLayouts(t, "wedding-layouts", bgFile, stFile, pfile{name: "bg/low.jpg", kind: "bg", data: low},
		lvLook(t, "a", badCover, lvStep(2, 2, lowKey, two...)), lvLook(t, "b", nil, outside, lvStep(2, 2, bg, two...)))
	text, code = g.run(t, "-app", "0.28.0", "-out", out, bad)
	if code != 1 {
		t.Fatalf("код %d\n%s", code, text)
	}
	// Каждый провал виден в итоге: LowDPI у шага макета a и его обложки (по 3 размера), стикер и место вне листа макета b.
	if n := countRe(t, `(?m)^ - .*layouts/a\.json: шаг 1 · .*LowDPI`, text); n != 3 {
		t.Errorf("LowDPI шага a: %d, ждали 3\n%s", n, text)
	}
	if n := countRe(t, `(?m)^ - .*layouts/a\.json: обложка, лицо · .*LowDPI`, text); n != 3 {
		t.Errorf("LowDPI обложки a: %d, ждали 3\n%s", n, text)
	}
	if !strings.Contains(text, "layouts/b.json: шаг 1 · 30×30: стикер 1") || !strings.Contains(text, "вне листа") {
		t.Errorf("нет провала «стикер вне листа»\n%s", text)
	}
	if n := countRe(t, `(?m)^ - .*layouts/b\.json: шаг [23]`, text); n != 0 {
		t.Errorf("годные шаги b провалены: %d\n%s", n, text) // независимые блоки не мешают друг другу
	}
	// Снимок на каждый провал: у трёх размеров одного шага разные файлы.
	if m, _ := filepath.Glob(filepath.Join(out, "fail-*.png")); len(m) != 9 {
		t.Errorf("снимков провала %d, ждали 9: %v", len(m), m)
	}
	// Правило набора макетов: id и minApp.
	text, code = g.run(t, "-app", "0.28.0", packZip(t, "wedding-set", 1, bgFile, lvLook(t, "ok", nil, lvStep(2, 2, bg, two...))))
	if code != 1 || !strings.Contains(text, "id должен кончаться на -layouts") || !strings.Contains(text, "minApp 0.26.0 ниже 0.28.0") {
		t.Errorf("id и minApp: код %d\n%s", code, text)
	}
	// Набор без макетов проверяется как раньше.
	if text, code := g.run(t, "-app", "0.28.0", packZip(t, "plain", 1, files...)); code != 0 || strings.Contains(text, "макет") {
		t.Errorf("набор без макетов: код %d\n%s", code, text)
	}
}

// Р3: сборка без library-verify-looks.go (как для меток 0.26/0.27) набор с макетами не пропускает молча.
func TestLibraryVerifyLooksFileMissing(t *testing.T) {
	g := buildLibVerifyOf(t, "library-verify.go", "library-verify-sheets.go")
	files, bg, _ := lvGood(t)
	text, code := g.run(t, "-app", "0.28.0", lvLayouts(t, "wedding-layouts", files[0], lvLook(t, "ok", nil, lvStep(2, 2, bg, model.Box{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}))))
	if code != 1 || !strings.Contains(text, "макеты альбома не проверены: соберите с library-verify-looks.go") {
		t.Fatalf("код %d\n%s", code, text)
	}
	// Набор без макетов такой сборкой проверяется как раньше.
	if text, code := g.run(t, "-app", "0.28.0", packZip(t, "plain", 1, files...)); code != 0 {
		t.Errorf("набор без макетов: код %d\n%s", code, text)
	}
}

// previewImage — общий лист -preview набора макетов (PNG читается, иначе тест падает).
func previewImage(t *testing.T, g lvTool, set string) image.Image {
	t.Helper()
	prev := filepath.Join(testkit.TempDir(t), "preview.png")
	text, code := g.run(t, "-app", "0.28.0", "-preview", prev, set)
	if code != 0 {
		t.Fatalf("код %d\n%s", code, text)
	}
	fh, err := os.Open(prev)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	img, err := png.Decode(fh)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// previewWidth — ширина общего листа -preview набора макетов.
func previewWidth(t *testing.T, g lvTool, set string) int {
	t.Helper()
	return previewImage(t, g, set).Bounds().Dx()
}

// wantTone — красный канал пикселя (x, y) листа близок к tone (jpeg и сжатие плитки — допуск 14).
func wantTone(t *testing.T, img image.Image, x, y int, tone uint8, what string) {
	t.Helper()
	r, _, _, _ := img.At(x, y).RGBA()
	if d := int(r>>8) - int(tone); d < -14 || d > 14 {
		t.Errorf("%s: пиксель (%d,%d) красный %d, ждали около %d", what, x, y, r>>8, tone)
	}
}

// Лист превью макета «Обложка, 1…N»: плитки обложки (оборот и лицо) и каждого шага в одной строке;
// без обложки строка короче ровно на две квадратные плитки.
func TestLibraryVerifyLookSheet(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, _ := lvGood(t)
	two := []model.Box{{X: 0.11, Y: 0.08, W: 0.28, H: 0.84}, {X: 0.6, Y: 0.08, W: 0.3, H: 0.4}}
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.4}}, Page: []model.TplBg{{Lib: bg}}}
	cover := &model.CoverTpl{Bg: &model.TplBg{Lib: bg}, Front: &front}
	steps := []model.LookStep{lvStep(2, 2, bg, two...), lvStep(2, 2, bg, two...)}
	with := previewWidth(t, g, lvLayouts(t, "wedding-layouts", files[0], lvLook(t, "a", cover, steps...)))
	without := previewWidth(t, g, lvLayouts(t, "wedding-layouts", files[0], lvLook(t, "a", nil, steps...)))
	if d := with - without; d < 2*200 || d > 2*(200+8) {
		t.Errorf("обложка добавила %d px, ждали две плитки 200 px с зазором", d)
	}
}

// Содержимое плиток листа: фон обложки ложится на лицо без своего фона, место шага с Ratio ≠ числу
// страниц вписано по центру (LookBoxes), шаги идут в порядке Series. Строка макета — последняя на листе
// (отступ 16 px, плитки rowH = 200 px, зазор 8): оборот x 16, лицо 224, шаг 1 (разворот 400 px) 432, шаг 2 840.
func TestLibraryVerifyLookSheetTiles(t *testing.T) {
	g := buildLibVerify(t)
	tone := func(name string, v uint8) (pfile, string) {
		data := lvBg(t, 3300, 2475, v)
		return pfile{name: "bg/" + name + ".jpg", kind: "bg", data: data}, lvKey(data)
	}
	fa, a := tone("a", 0x40) // фон обложки
	fb, b := tone("b", 0x90) // шаг 1
	fc, c := tone("c", 0xe0) // шаг 2
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.4, Y: 0.4, W: 0.2, H: 0.2}}, Page: []model.TplBg{{}}}
	cover := &model.CoverTpl{Bg: &model.TplBg{Lib: a}, Front: &front}
	narrow := lvStep(2, 1, b, model.Box{X: 0.05, Y: 0.05, W: 0.9, H: 0.9}) // Ratio 1 на развороте: композиция в середине, по бокам фон
	corner := lvStep(2, 2, c, model.Box{X: 0.4, Y: 0.4, W: 0.2, H: 0.2})
	img := previewImage(t, g, lvLayouts(t, "wedding-layouts", fa, fb, fc, lvLook(t, "a", cover, narrow, corner)))
	y0 := img.Bounds().Dy() - 16 - 200
	wantTone(t, img, 16+10, y0+10, 0x40, "оборот: фон обложки")
	wantTone(t, img, 224+10, y0+10, 0x40, "лицо без своего фона: фон обложки")
	wantTone(t, img, 224+100, y0+100, 0xcf, "лицо: место")
	wantTone(t, img, 432+40, y0+100, 0x90, "шаг 1: полоса фона слева от вписанной композиции")
	wantTone(t, img, 432+200, y0+100, 0xcf, "шаг 1: место по центру")
	wantTone(t, img, 840+10, y0+10, 0xe0, "шаг 2 после шага 1")
}
