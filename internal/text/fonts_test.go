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
	if n != 43 {
		t.Errorf("начертаний %d, ждали 43", n)
	}
	if len(Families()) != 18 {
		t.Errorf("гарнитур %d, ждали 18", len(Families()))
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
	// Начертания, которых нет у гарнитуры, сбрасываются; те, что есть, остаются.
	if g := (Style{Font: "greatvibes", Bold: true}).Clamped(); g.Bold || g.Font != "greatvibes" {
		t.Errorf("Great Vibes: %+v", g)
	}
	if c := (Style{Font: "caveat", Bold: true, Italic: true}).Clamped(); !c.Bold || c.Italic {
		t.Errorf("Caveat: %+v", c)
	}
	if n := (Style{Font: "nunito", Bold: true, Italic: true}).Clamped(); !n.Bold || !n.Italic {
		t.Errorf("Nunito: %+v", n)
	}
	if x := (Style{Font: "xx", Bold: true}).Clamped(); x.Font != DefaultFont || !x.Bold {
		t.Errorf("неизвестный id: %+v", x)
	}
	if (Style{Size: -5}).Clamped().Size != MinSize {
		t.Error("нижний зажим")
	}
}
