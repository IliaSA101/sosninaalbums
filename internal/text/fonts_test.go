package text

import (
	"math"
	"strings"
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
	if n != 62 {
		t.Errorf("начертаний %d, ждали 62", n)
	}
	if len(Families()) != 33 {
		t.Errorf("гарнитур %d, ждали 33", len(Families()))
	}
	if _, ok := FontFile("../fonts.go"); ok {
		t.Error("FontFile пропустил путь вне белого списка")
	}
}

// id гарнитур хранятся в .salbum навсегда: переименование молча заменит шрифт на Montserrat.
func TestFamilyIDsStable(t *testing.T) {
	want := "badscript caveat comforter greatvibes lobster marck pacifico " +
		"amaticsc comfortaa marmelad neucha nunito pangolin rubik " +
		"alice cormorant forum kurale lora oldstandard playfair prata ptserif yeseva " +
		"golos montserrat oswald raleway robotocondensed tenorsans " +
		"poiretone ruslan underdog"
	var got []string
	for _, f := range Families() {
		got = append(got, f.ID)
	}
	if g := strings.Join(got, " "); g != want {
		t.Errorf("порядок или id гарнитур изменились:\n%s\nждали:\n%s", g, want)
	}
}

// Вшитые файлы растут с каждой гарнитурой: предел папки 8 МБ (спека 046) — вес exe.
func TestFontsFolderSize(t *testing.T) {
	ents, err := fontFS.ReadDir("fonts")
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, e := range ents {
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += info.Size()
	}
	if total > 8_000_000 {
		t.Errorf("вшитых шрифтов %d байт, предел 8 000 000", total)
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
	// Новые гарнитуры: у Rubik есть жирный, но нет курсива; у Lobster нет ни того ни другого.
	if r := (Style{Font: "rubik", Bold: true, Italic: true}).Clamped(); !r.Bold || r.Italic {
		t.Errorf("Rubik: %+v", r)
	}
	if l := (Style{Font: "lobster", Bold: true, Italic: true}).Clamped(); l.Bold || l.Italic || l.Font != "lobster" {
		t.Errorf("Lobster: %+v", l)
	}
	if x := (Style{Font: "xx", Bold: true}).Clamped(); x.Font != DefaultFont || !x.Bold {
		t.Errorf("неизвестный id: %+v", x)
	}
	if (Style{Size: -5}).Clamped().Size != MinSize {
		t.Error("нижний зажим")
	}
}
