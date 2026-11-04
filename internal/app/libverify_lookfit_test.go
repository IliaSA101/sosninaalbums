package app

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Подгонка по страницам в library-verify (165 Т3, design §5): fit шага — минимум 0.37.0; в наборе с fit
// предмет двухстраничного шага целиком в одной странице (центр на сгибе ±0,5 мм — только при fit = "");
// места навылет не считаются в предупреждении о пропорции.

// lvFitStep — шаг-разворот Ratio 2 со стикером, центр которого от центра разворота на dx S
// (S = 1 при Ratio 2): 0 — ровно на сгибе. fit — SpreadTpl.Fit шага.
func lvFitStep(bg, st, fit string, dx float64) model.LookStep {
	s := lvStep(2, 2, bg)
	s.Tpl.Fit = fit
	s.Tpl.Stickers = []model.TplItem{{Anchor: model.Anchor{DX: dx}, W: 0.2, H: 0.2, Lib: st}}
	return s
}

func TestLibraryVerifyLookFitMinApp(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, st := lvGood(t)
	set := func(minApp string, steps ...model.LookStep) string {
		t.Helper()
		return patchManifest(t, packZip(t, "fit-layouts", 1, files[0], files[1], lvLook(t, "v", nil, steps...)), map[string]any{"minApp": minApp})
	}
	// Страницы шагов: слева от сгиба, чтобы правило корешка молчало.
	fit, plain := lvFitStep(bg, st, model.FitPage, -0.5), lvFitStep(bg, st, "", -0.5)

	// Ниже 0.37.0: назван именно шаг с fit (шаг без fit — нет); несколько шагов — все в одном прогоне.
	out, code := g.run(t, "-app", "0.37.0", set("0.36.0", plain, fit, fit))
	if code != 1 || !strings.Contains(out, `шаг 2: у шага fit "page", а minApp 0.36.0 ниже 0.37.0`) || !strings.Contains(out, `шаг 3: у шага fit "page"`) ||
		strings.Contains(out, "шаг 1: у шага fit") {
		t.Errorf("minApp 0.36.0: код %d\n%s", code, out)
	}
	// С 0.37.0 — годно; набор без fit на 0.28.0 — как раньше.
	if out, code = g.run(t, "-app", "0.37.0", set("0.37.0", plain, fit)); code != 0 {
		t.Errorf("minApp 0.37.0: код %d\n%s", code, out)
	}
	if out, code = g.run(t, "-app", "0.37.0", set("0.28.0", plain)); code != 0 {
		t.Errorf("набор без fit, minApp 0.28.0: код %d\n%s", code, out)
	}
}

func TestLibraryVerifyLookSpine(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, st := lvGood(t)
	run := func(steps ...model.LookStep) (string, int) {
		t.Helper()
		pack := patchManifest(t, packZip(t, "spine-layouts", 1, files[0], files[1], lvLook(t, "v", nil, steps...)), map[string]any{"minApp": "0.37.0"})
		return g.run(t, "-app", "0.37.0", pack)
	}
	inPage := lvFitStep(bg, st, model.FitPage, -0.5) // целиком в левой странице
	atFold := func(fit string) model.LookStep { return lvFitStep(bg, st, fit, 0) }
	offFold := func(fit string) model.LookStep { return lvFitStep(bg, st, fit, 0.05) } // центр в 0,05 S от сгиба, ширина 0,2 S: заход 0,05 S

	// fit = "page": предмет через сгиб — провал на 20×30 и 30×30 (кадр страницы на 40×30 уже страницы:
	// предмет в нём уходит от сгиба, и это верно), шаг и размер названы; соседние
	// годные шаги и шаг с fit = "" (центр на сгибе) не затронуты — все блоки за один прогон.
	out, code := run(inPage, atFold(model.FitPage), atFold(""), offFold(model.FitPage), inPage)
	if code != 1 {
		t.Fatalf("код %d\n%s", code, out)
	}
	if n := countRe(t, `(?m)^ - .*шаг 2 · .*стикер 1 .*лежит через корешок.*fit "page"`, out); n != 2 {
		t.Errorf("шаг 2: провалов корешка %d, ждали 2\n%s", n, out)
	}
	if n := countRe(t, `(?m)^ - .*шаг 4 · .*лежит через корешок`, out); n != 2 {
		t.Errorf("шаг 4 (центр в 0,05 S): провалов %d, ждали 2\n%s", n, out)
	}
	for _, no := range []string{"шаг 1 · ", "шаг 3 · ", "шаг 5 · "} {
		if strings.Contains(out, no+"20×30: стикер") || strings.Contains(out, no+"30×30: стикер") || strings.Contains(out, no+"40×30: стикер") {
			t.Errorf("годный %sназван провалом\n%s", no, out)
		}
	}
	// fit = "" — центр на сгибе допустим, но заход не по центру — нет (набор с fit).
	if out, code = run(inPage, offFold("")); code != 1 || countRe(t, `(?m)^ - .*шаг 2 · .*лежит через корешок.*fit ""`, out) != 3 {
		t.Errorf("fit = \"\" вне центра: код %d\n%s", code, out)
	}
	if out, code = run(inPage, atFold("")); code != 0 {
		t.Errorf("fit = \"\" в центре сгиба: код %d\n%s", code, out)
	}
	// Набор без fit: правило не действует (старые наборы 0.28–0.36 не ломаются).
	if out, code = run(atFold(""), offFold("")); code != 0 || strings.Contains(out, "корешок") {
		t.Errorf("набор без fit: код %d\n%s", code, out)
	}
}

func TestLibraryVerifyLookBleedNoWarn(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, _ := lvGood(t)
	// Ratio 2: место w·2/h. Квадрат режется 33 % — при обычном месте это WARN.
	square := model.Box{X: 0.05, Y: 0.1, W: 0.25, H: 0.5}
	left := model.Box{X: 0, Y: 0, W: 0.5, H: 1}          // страница навылет: 1,0
	right := model.Box{X: 0.5, Y: 0.1, W: 0.5, H: 0.8}   // до правого края: 1,25
	bottom := model.Box{X: 0.1, Y: 0.5, W: 0.25, H: 0.5} // до нижнего: 1,0
	top := model.Box{X: 0.6, Y: 0, W: 0.25, H: 0.5}      // от верхнего: 1,0
	tilted := model.Box{X: 0, Y: 0.1, W: 0.25, H: 0.5, Rot: 5}
	type bleedCase struct {
		name   string
		places []model.Box
		warn   string
		fails  bool // повёрнутое место у края уходит за лист: провал, предупреждение проверяем рядом
	}
	cases := []bleedCase{
		{name: "left-page", places: []model.Box{left}, warn: ""},
		{name: "right-edge", places: []model.Box{right}, warn: ""},
		{name: "bottom", places: []model.Box{bottom}, warn: ""},
		{name: "top", places: []model.Box{top}, warn: ""},
		{name: "all-edges", places: []model.Box{left, right, bottom, top}, warn: ""},
		// Навылет не в счёт: считаются два квадрата, а не три места.
		{name: "edge-and-squares", places: []model.Box{left, square, square}, warn: "2 из 2 мест не 3:2 / 2:3"},
		// Навылет не разбавляет долю: 3 плохих из 4 внутренних, а не из 6 мест.
		{name: "edge-not-diluting", places: []model.Box{left, right, square, square, square, {X: 0.11, Y: 0.08, W: 0.28, H: 0.84}}, warn: "3 из 4 мест не 3:2 / 2:3"},
		// Наклон — не навылет: сторона на краю композиции не край фото.
		{name: "tilted-counts", places: []model.Box{tilted}, warn: "1 из 1 мест не 3:2 / 2:3", fails: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text, code := g.run(t, "-app", "0.28.0", lvLayouts(t, "bleed-layouts", files[0], lvLook(t, c.name, nil, lvStep(2, 2, bg, c.places...))))
			if (code != 0) != c.fails {
				t.Fatalf("предупреждение не меняет код: код %d\n%s", code, text)
			}
			if c.warn == "" {
				if strings.Contains(text, "WARN") {
					t.Fatalf("лишнее предупреждение:\n%s", text)
				}
				return
			}
			if !strings.Contains(text, "шаг 1: "+c.warn) {
				t.Fatalf("нужно предупреждение %q:\n%s", c.warn, text)
			}
		})
	}
}
