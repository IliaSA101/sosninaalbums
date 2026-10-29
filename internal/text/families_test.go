package text

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// Знаки, которых нет в отдельных гарнитурах (как ₽ у Marck Script): id → знаки.
// Тот же список — в fonts/README.md.
var noGlyph = map[string]string{"alice": "№", "prata": "№"}

const needRunes = "АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯабвгдеёжзийклмнопрстуфхцчшщъыьэюя" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.,:;!?«»—–№"

func TestFontsCyrillic(t *testing.T) {
	m, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	var b sfnt.Buffer
	for _, f := range Families() {
		for _, fc := range f.Faces {
			for _, r := range needRunes {
				if strings.ContainsRune(noGlyph[f.ID], r) {
					// исключение не должно устареть: знак действительно отсутствует
					if g, _ := m[fc.File].GlyphIndex(&b, r); g != 0 {
						t.Errorf("%s: знак %q есть, уберите из noGlyph", fc.File, r)
					}
					continue
				}
				if g, err := m[fc.File].GlyphIndex(&b, r); err != nil || g == 0 {
					t.Errorf("%s: нет знака %q", fc.File, r)
				}
			}
		}
	}
}

// Вариативный шрифт (таблица fvar) sfnt рисует начальным экземпляром, а браузер
// подставил бы ось: жирный на экране и в экспорте разошёлся бы.
func TestFontsStatic(t *testing.T) {
	for _, f := range Families() {
		if _, err := fontFS.ReadFile("fonts/OFL-" + f.ID + ".txt"); err != nil {
			t.Errorf("%s: нет лицензии OFL-%s.txt", f.ID, f.ID)
		}
		for _, fc := range f.Faces {
			b, ok := FontFile(fc.File)
			if !ok {
				t.Fatalf("нет %s", fc.File)
			}
			// каталог таблиц sfnt лежит в начале файла
			if bytes.Contains(b[:min(len(b), 2048)], []byte("fvar")) {
				t.Errorf("%s: вариативный шрифт (fvar)", fc.File)
			}
		}
	}
}

func TestFamiliesGroups(t *testing.T) {
	seen := map[string]bool{}
	files := map[string]bool{}
	for _, f := range Families() {
		switch f.Group {
		case "script", "kids", "serif", "sans", "display":
		default:
			t.Errorf("%s: группа %q", f.ID, f.Group)
		}
		if seen[f.ID] {
			t.Errorf("повтор id %s", f.ID)
		}
		seen[f.ID] = true
		if n := len(f.Faces); n != 1 && n != 2 && n != 4 {
			t.Errorf("%s: %d начертаний, нужно 1, 2 или 4", f.ID, n)
		}
		if !f.has(false, false) {
			t.Errorf("%s: нет обычного начертания", f.ID)
		}
		for _, fc := range f.Faces {
			if files[fc.File] {
				t.Errorf("файл %s у двух гарнитур", fc.File)
			}
			files[fc.File] = true
		}
	}
	if !seen[DefaultFont] {
		t.Error("нет гарнитуры по умолчанию")
	}
	// Вшитый шрифт, не вошедший ни в одну гарнитуру, — лишний вес exe.
	ents, err := fontFS.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".ttf") && !files[e.Name()] {
			t.Errorf("%s вшит, но не входит в families", e.Name())
		}
	}
}

// Экспорт рисует каждое начертание каждой гарнитуры: кириллица видна, а не «тофу».
func TestDrawEveryFace(t *testing.T) {
	const dpi = 150
	for _, f := range Families() {
		for _, fc := range f.Faces {
			st := Style{Font: f.ID, Size: 24, Color: "#000000", Align: "left", Bold: fc.Bold, Italic: fc.Italic}.Clamped()
			if faceFile(st) != fc.File {
				t.Fatalf("%s: faceFile выбрал %s", fc.File, faceFile(st))
			}
			b := Layout("Анна и Павел 2026", st, 120)
			if b.W <= 0 || b.H <= 0 {
				t.Fatalf("%s: пустой блок %v", fc.File, b)
			}
			img := image.NewRGBA(image.Rect(0, 0, 827, 237))
			draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
			Draw(img, b, st, 5, 5, 0, dpi)
			if inkBounds(img).Empty() {
				t.Errorf("%s: ничего не нарисовано", fc.File)
			}
		}
	}
}
