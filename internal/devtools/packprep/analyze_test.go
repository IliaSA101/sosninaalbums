package packprep

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// asym — асимметричный стикер: буква «Г» из двух прямоугольников разных цветов.
func asym() *image.NRGBA {
	img := canvas(120, 90, transp)
	fillRect(img, image.Rect(10, 10, 100, 30), navy)
	fillRect(img, image.Rect(10, 30, 35, 80), coral)
	return img
}

func rot90(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := range h {
		for x := range w {
			out.SetNRGBA(h-1-y, x, src.NRGBAAt(x, y))
		}
	}
	return out
}

func mirror(src *image.NRGBA) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	out := image.NewNRGBA(src.Bounds())
	for y := range h {
		for x := range w {
			out.SetNRGBA(w-1-x, y, src.NRGBAAt(x, y))
		}
	}
	return out
}

// Поворот на 90° и отражение — тот же предмет (расстояние около нуля); другой предмет — далеко.
func TestDupRotated90(t *testing.T) {
	base := hashOf(asym())
	for name, img := range map[string]*image.NRGBA{
		"поворот 90":          rot90(asym()),
		"поворот 180":         rot90(rot90(asym())),
		"поворот 270":         rot90(rot90(rot90(asym()))),
		"отражение":           mirror(asym()),
		"отражение и поворот": rot90(mirror(asym())),
	} {
		if d := Distance(base, hashOf(img)); d > 6 {
			t.Errorf("%s: расстояние %d, ждали почти 0", name, d)
		}
	}
	other := canvas(120, 90, transp) // полосы через один: ни на одно положение «Г» не похоже
	for x := 0; x < 120; x += 16 {   // расстояние до «Г» — 28 из 64
		fillRect(other, image.Rect(x, 0, x+8, 90), navy)
	}
	if d := Distance(base, hashOf(other)); d <= SimilarMax {
		t.Errorf("другой предмет: расстояние %d, порог %d", d, SimilarMax)
	}
	// В отчёте повёрнутый дубль получает «похоже на №N», разные — нет.
	a := &Output{Kind: KindSticker, N: 1, hash: base, File: "a"}
	b := &Output{Kind: KindSticker, N: 2, hash: hashOf(rot90(asym())), File: "b"}
	var far Hash // противоположные биты во всех положениях: расстояние 64
	for i := range far {
		far[i] = ^base[0]
	}
	c := &Output{Kind: KindSticker, N: 3, hash: far, File: "c"}
	d := &Output{Kind: KindFrame, N: 4, hash: base, File: "d"}
	markSimilar([]*Output{a, b, c, d})
	if b.Similar != 1 || c.Similar != 0 || d.Similar != 0 || !strings.Contains(b.Flags[0], "похоже на №1") {
		t.Fatalf("подсказки: b=%+v c=%+v d=%+v", b, c, d)
	}
}

// Кайма — светлый ободок по контуру тёмного предмета; белый предмет и белая часть внутри — не кайма.
func TestFringe(t *testing.T) {
	rim := canvas(300, 300, transp)
	disc(rim, 150, 150, 120, white)
	disc(rim, 150, 150, 112, navy)
	if f := CheckFringe(rim); !f.Flag || f.RingLight < 0.5 || f.CoreLight > 0.05 {
		t.Errorf("ободок: %+v", f)
	}
	clean := canvas(300, 300, transp)
	disc(clean, 150, 150, 120, navy)
	if f := CheckFringe(clean); f.Flag || f.Ring == 0 || f.Core == 0 {
		t.Errorf("чистый: %+v", f)
	}
	cloud := canvas(300, 300, transp)
	disc(cloud, 150, 150, 120, white)
	if f := CheckFringe(cloud); f.Flag {
		t.Errorf("белый предмет (облако): %+v", f)
	}
	dog := canvas(300, 300, transp)
	disc(dog, 150, 150, 120, color.NRGBA{40, 60, 120, 255})
	disc(dog, 110, 150, 70, white) // светлая часть внутри, не у контура
	if f := CheckFringe(dog); f.Flag {
		t.Errorf("светлая часть внутри: %+v", f)
	}
	if f := CheckFringe(canvas(10, 10, transp)); f.Flag || f.Ring != 0 {
		t.Errorf("пустая картинка: %+v", f)
	}
}

// Шрифт знает кириллицу, регистр не важен, неизвестное — «?»; перенос по словам с многоточием.
func TestFontAndWrap(t *testing.T) {
	if glyphOf('я') != glyphOf('Я') || glyphOf('о') != glyphOf('O') || glyphOf('☃') != glyphOf('?') {
		t.Fatal("шрифт: регистр, двойники или запасной знак")
	}
	for _, r := range "АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ" {
		if g := glyphOf(r); g == glyphOf('?') && r != '?' {
			t.Errorf("нет буквы %c", r)
		}
	}
	got := wrapText("Принадлежности в углах рамки очень длинное название", 14, 2)
	if len(got) != 2 || !strings.HasSuffix(got[1], "...") || len([]rune(got[1])) > 14 {
		t.Errorf("перенос: %q", got)
	}
	if lines := wrapText("СверхдлинноеОднословноеНазвание", 10, 3); len(lines) != 3 {
		t.Errorf("длинное слово: %q", lines)
	}
}
