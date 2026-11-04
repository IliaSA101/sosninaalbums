package text

import (
	"image"
	"math"
	"testing"
	"unicode/utf8"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// FuzzLineDX: на произвольном тексте, кегле и ширине у строк своего шрифта (Lora с настоящим
// GPOS) DX сходится с рунами — столько же значений, первое 0, конечные и не больше em, —
// отрисовка идёт по DX, а не запасным путём, гарнитура не помечена сломанной и кернинг не отключён.
// Долгий прогон: go test ./internal/text -run ^$ -fuzz FuzzLineDX -fuzztime 60s
// (через очередь: node scripts/heavy.mjs go test …).
func FuzzLineDX(f *testing.F) {
	for _, s := range []string{
		"То 'А .у", "𝐀й̆️ x", "ТV V Т", "abc\n\nдеж  ", "שלום То", "́То", "12 Ёлка, 34", " \n ",
		"", "\n", "\t\x00", "\xff\xfe", "AVAVAVAV Ту Ту", "😀 То 😀",
	} {
		f.Add(s, uint8(30), uint16(500))
	}
	f.Add("Тату", uint8(0), uint16(0))            // предел: кегль и ширина минимальны
	f.Add("Тату Тату", uint8(255), uint16(65535)) // предел: наибольшие значения
	b := builtinBytes(f, "Lora-Regular.ttf")
	fc := CustomFace{Key: keyOf(b), Ext: "ttf"}
	const name = "FuzzLineDX"
	SetCustom([]CustomFamily{{ID: FamilyID(name), Name: name, Faces: []CustomFace{fc}}}, loader(b))
	f.Cleanup(func() { SetCustom(nil, nil) })
	f.Fuzz(func(t *testing.T, s string, size uint8, w uint16) {
		st := Style{Font: FamilyID(name), Size: float64(size%120) + 4, Color: "#000000", Align: "center"}.Clamped()
		box := Layout(s, st, float64(w%3000)/10+0.5)
		em := st.Size * 25.4 / 72
		sf, _, d := fontData(st)
		fb, _ := fallback(st)
		for _, l := range box.Lines {
			n := utf8.RuneCountInString(l.Text)
			if l.Text == "" {
				if l.DX != nil {
					t.Fatalf("пустая строка с DX %v", l.DX)
				}
				continue
			}
			if len(l.DX) != n || l.DX[0] != 0 {
				t.Fatalf("%q: DX %d при %d рунах (%v)", l.Text, len(l.DX), n, l.DX)
			}
			for _, v := range l.DX {
				if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > em+1e-3 {
					t.Fatalf("%q: DX %v", l.Text, l.DX)
				}
			}
			var b1, b2 sfnt.Buffer
			gl, xs := placeCustom(glyphSrc{sf, &b1}, glyphSrc{fb, &b2}, d, l, fixed.Int26_6(sf.UnitsPerEm())<<6, 300/25.4, st.Size/72*300)
			if len(gl) != n || len(xs) != n {
				t.Fatalf("%q: глифов %d, x %d, рун %d — запасной путь", l.Text, len(gl), len(xs), n)
			}
		}
		Draw(image.NewRGBA(image.Rect(0, 0, 64, 64)), box, st, 0, 0, 0, 72)
		if lookupCustom(FamilyID(name)).broken() {
			t.Fatal("гарнитура помечена сломанной")
		}
		if d.kernOff.Load() {
			t.Fatal("кернинг GPOS отключён на настоящем шрифте")
		}
	})
}
