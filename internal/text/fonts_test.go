package text

import (
	"math"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestFontsEmbedded(t *testing.T) {
	m, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	var b sfnt.Buffer
	n := 0
	for _, f := range Families() {
		for _, fc := range f.Faces {
			sf := m[fc.File]
			if sf == nil {
				t.Fatalf("нет %s", fc.File)
			}
			n++
			for _, r := range "Aaя" {
				if g, err := sf.GlyphIndex(&b, r); err != nil || g == 0 {
					t.Errorf("%s: нет знака %q", fc.File, r)
				}
			}
			if _, ok := FontFile(fc.File); !ok {
				t.Errorf("FontFile(%s) не нашёл", fc.File)
			}
		}
	}
	if n != 13 {
		t.Errorf("начертаний %d, ждали 13", n)
	}
	if _, ok := FontFile("../fonts.go"); ok {
		t.Error("FontFile пропустил путь вне белого списка")
	}
}

func TestStyleClamped(t *testing.T) {
	s := Style{Font: "xx", Size: math.NaN(), Color: "red", Align: "?", Bold: true}.Clamped()
	if s.Font != DefaultFont || s.Size != DefaultSize || s.Color != DefaultColor || s.Align != "center" {
		t.Errorf("%+v", s)
	}
	m := Style{Font: "marck", Size: 1000, Color: "#abcdef", Align: "left", Bold: true, Italic: true}.Clamped()
	if m.Bold || m.Italic || m.Size != MaxSize || m.Color != "#abcdef" {
		t.Errorf("%+v", m)
	}
	if (Style{Size: -5}).Clamped().Size != MinSize {
		t.Error("нижний зажим")
	}
}
