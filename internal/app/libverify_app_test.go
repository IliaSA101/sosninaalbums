package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Версия программы и названия наборов в library-verify (ревью 129 Т4): minApp набора новее —
// провал, а не молчаливый «dev»; плитки донора и дополнения с одним названием не смешиваются.

// patchManifest переписывает поля манифеста zip-набора и возвращает путь нового архива.
func patchManifest(t *testing.T, src string, patch map[string]any) string {
	t.Helper()
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == "manifest.json" {
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for k, v := range patch {
				m[k] = v
			}
			if data, err = json.Marshal(m); err != nil {
				t.Fatal(err)
			}
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(testkit.TempDir(t), filepath.Base(src))
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestLibraryVerifyAppVersion(t *testing.T) {
	g := buildLibVerify(t)
	files, _, _ := lvGood(t)
	newer := patchManifest(t, packZip(t, "future", 1, files...), map[string]any{"minApp": "0.99.0"})
	same := packZip(t, "now", 1, files...) // minApp 0.26.0

	// Явный -app: набор для более новой программы — провал; для той же или старше — принят.
	if text, code := g.run(t, "-app", "0.26.0", newer); code != 1 || !strings.Contains(text, "нужна версия Sosnina Albums 0.99.0") {
		t.Errorf("minApp новее -app: код %d\n%s", code, text)
	}
	if text, code := g.run(t, "-app", "0.99.0", newer); code != 0 {
		t.Errorf("minApp равен -app: код %d\n%s", code, text)
	}
	if text, code := g.run(t, "-app", "0.25.9", same); code != 1 {
		t.Errorf("0.26.0 при -app 0.25.9: код %d\n%s", code, text)
	}
	// Дополнение новее программы проваливается и как донор.
	if text, code := g.run(t, "-app", "0.26.0", "-with", newer, same); code != 1 || !strings.Contains(text, "(донор)") {
		t.Errorf("донор новее: код %d\n%s", code, text)
	}
	// Без -app версия берётся из wails.json дерева (запуск из internal/app — поиск вверх): не «dev».
	text, code := g.run(t, same)
	if !regexp.MustCompile(`версия программы для minApp: \d+\.\d+\.\d+`).MatchString(text) {
		t.Errorf("версия из wails.json не показана: код %d\n%s", code, text)
	}
	if text, code := g.run(t, newer); code != 1 || !strings.Contains(text, "нужна версия Sosnina Albums 0.99.0") {
		t.Errorf("minApp 0.99.0 прошёл при версии из wails.json: код %d\n%s", code, text)
	}
	// Не x.y.z (в том числе прежнее «dev», отключавшее сверку) — отказ с кодом 2.
	for _, bad := range []string{"dev", "0.26", "v0.26.0", "1.2.3-beta"} {
		if text, code := g.run(t, "-app", bad, same); code != 2 {
			t.Errorf("-app %q: код %d\n%s", bad, code, text)
		}
	}
}

func TestLibraryVerifyTitleClash(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, st := lvGood(t)
	donor := packZip(t, "donor", 1, files[0], files[1])
	addon := patchManifest(t, packZip(t, "addon-layouts", 1, lvTpl(t, 2, bg, st, 0.15)), map[string]any{"title": "Набор donor"})
	text, code := g.run(t, "-with", donor, addon)
	if code != 1 || !strings.Contains(text, "совпадает с набором donor") {
		t.Fatalf("одинаковые названия: код %d\n%s", code, text)
	}
	// Разные названия — как раньше.
	if text, code := g.run(t, "-with", donor, packZip(t, "addon-layouts", 1, lvTpl(t, 2, bg, st, 0.15))); code != 0 {
		t.Fatalf("разные названия: код %d\n%s", code, text)
	}
}

// lvAspectTpl — файл шаблона разворота с заданными местами и без стикеров (фон из набора).
func lvAspectTpl(t *testing.T, name, bg string, places []model.Box) pfile {
	t.Helper()
	tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: places, Page: []model.TplBg{{Lib: bg}, {Lib: bg}}}
	data, err := json.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	return pfile{name: "spreads/" + name + ".json", kind: "spread", data: data}
}

// Предупреждение о пропорции мест (решение 0.27-131-24): меньше 80 % мест, где кадр 3:2 / 2:3 режется
// не больше чем на 15 % (30×30), — строка WARN с именем шаблона и долей; код выхода не меняется.
func TestLibraryVerifyAspectWarn(t *testing.T) {
	g := buildLibVerify(t)
	files, bg, _ := lvGood(t)
	bgFile := files[0]
	// Разворот 60×30: отношение места = w·2/h.
	portrait := model.Box{X: 0.11, Y: 0.08, W: 0.28, H: 0.84}  // 2:3
	landscape := model.Box{X: 0.60, Y: 0.08, W: 0.30, H: 0.40} // 3:2
	square := model.Box{X: 0.05, Y: 0.1, W: 0.25, H: 0.5}      // 1,0: режет 33 %
	edge := model.Box{X: 0.05, Y: 0.1, W: 0.3825, H: 0.6}      // 1,275: ровно 15 %
	slightly := model.Box{X: 0.05, Y: 0.1, W: 0.35, H: 0.5}    // 1,4: 7 %
	oval := square
	oval.Shape = model.ShapeOval
	cases := []struct {
		name   string
		places []model.Box
		warn   string // "" — предупреждения нет
	}{
		{"all-square", []model.Box{square, square}, "2 из 2 мест не 3:2 / 2:3"},
		{"two-of-three", []model.Box{portrait, square, square}, "2 из 3 мест не 3:2 / 2:3 на 30×30 (подходят 33%"},
		{"portrait-landscape", []model.Box{portrait, landscape, landscape}, ""},
		{"edge-15", []model.Box{edge, portrait}, ""},
		{"four-of-five", []model.Box{portrait, landscape, slightly, edge, square}, ""},       // ровно 80 % — без предупреждения
		{"three-of-four", []model.Box{portrait, landscape, slightly, square}, "1 из 4 мест"}, // 75 %
		{"oval-only", []model.Box{oval, oval}, ""},                                           // фигуры по замыслу автора не считаются
		{"oval-and-portrait", []model.Box{oval, portrait}, ""},
		{"empty", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tpl := lvAspectTpl(t, c.name, bg, c.places)
			text, code := g.run(t, packZip(t, "aspect", 1, bgFile, tpl))
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
			if n != 1 || !strings.Contains(text, "ПРЕДУПРЕЖДЕНИЙ 1:") || !strings.Contains(text, "spreads/"+c.name+".json: "+c.warn) {
				t.Fatalf("нужно одно предупреждение %q с именем шаблона:\n%s", c.warn, text)
			}
		})
	}
	// Предупреждение рядом с провалом: код 1, оба в итоге.
	low := lvBg(t, 1200, 900, 0xc0)
	text, code := g.run(t, packZip(t, "aspect-fail", 1, pfile{name: "bg/low.jpg", kind: "bg", data: low},
		lvAspectTpl(t, "sq", lvKey(low), []model.Box{square})))
	if code != 1 || !strings.Contains(text, "ПРЕДУПРЕЖДЕНИЙ 1:") || !strings.Contains(text, "ПРОВАЛОВ") {
		t.Fatalf("провал и предупреждение вместе: код %d\n%s", code, text)
	}
}
