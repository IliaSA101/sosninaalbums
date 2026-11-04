package text

import (
	"encoding/binary"
	"errors"
	"slices"
	"testing"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Гарнитуры, у которых кириллический кернинг подключён только к скрипту cyrl,
// а файл менять нельзя. Lora — зарезервированное имя (RFN): sfnt кириллицу не
// кернит, браузер кернит — строка на экране чуть короче, чем в экспорте.
var noCyrlKern = map[string]bool{"lora": true}

// gposKern — номера lookups фичи kern в DefaultLangSys скрипта; found=false —
// скрипта нет или у него пустой список фич (тогда sfnt берёт DFLT).
func gposKern(font []byte, script string) (lookups []int, found bool, err error) {
	errBad := errors.New("GPOS вне файла")
	u16 := func(off int) int {
		if off < 0 || off+2 > len(font) {
			err = errBad
			return 0
		}
		return int(binary.BigEndian.Uint16(font[off:]))
	}
	tag := func(off int) string {
		if off < 0 || off+4 > len(font) {
			err = errBad
			return ""
		}
		return string(font[off : off+4])
	}
	gpos := -1
	for i := range u16(4) {
		rec := 12 + 16*i
		if tag(rec) == "GPOS" && rec+12 <= len(font) {
			gpos = int(binary.BigEndian.Uint32(font[rec+8:]))
		}
	}
	if gpos < 0 || err != nil {
		return nil, false, err
	}
	scripts, features := gpos+u16(gpos+4), gpos+u16(gpos+6)
	for i := range u16(scripts) {
		if tag(scripts+2+6*i) != script {
			continue
		}
		sc := scripts + u16(scripts+2+6*i+4)
		if u16(sc) == 0 {
			return nil, false, err
		}
		ls := sc + u16(sc)
		n := u16(ls + 4)
		for j := range n {
			fi := u16(ls + 6 + 2*j)
			if tag(features+2+6*fi) != "kern" {
				continue
			}
			ft := features + u16(features+2+6*fi+4)
			for k := range u16(ft + 2) {
				lookups = append(lookups, u16(ft+4+2*k))
			}
		}
		return lookups, n > 0, err
	}
	return nil, false, err
}

// sfnt (gpos.go) берёт kern только из скрипта latn (нет его — DFLT), а браузер
// для кириллицы — из cyrl. Lookups кириллического кернинга должны быть и в
// latn/DFLT, иначе «То» в экспорте шире, чем на экране.
func TestFontsCyrillicKern(t *testing.T) {
	for _, f := range Families() {
		for _, fc := range f.Faces {
			b, ok := FontFile(fc.File)
			if !ok {
				t.Fatalf("нет %s", fc.File)
			}
			cyrl, _, err := gposKern(b, "cyrl")
			if err != nil {
				t.Fatalf("%s: %v", fc.File, err)
			}
			got, ok, err := gposKern(b, "latn")
			if !ok {
				got, _, err = gposKern(b, "DFLT")
			}
			if err != nil {
				t.Fatalf("%s: %v", fc.File, err)
			}
			var miss []int
			for _, l := range cyrl {
				if !slices.Contains(got, l) {
					miss = append(miss, l)
				}
			}
			switch {
			case noCyrlKern[f.ID] && len(miss) == 0:
				t.Errorf("%s: кириллический кернинг уже в latn, уберите %s из noCyrlKern", fc.File, f.ID)
			case !noCyrlKern[f.ID] && len(miss) > 0:
				t.Errorf("%s: lookups %v есть у cyrl, но не у latn/DFLT — sfnt их не видит (README «Кернинг кириллицы»)", fc.File, miss)
			}
		}
	}
}

// Кириллическая пара кернится в sfnt так же, как в браузере (значения — из GPOS
// исходных файлов, lookup скрипта cyrl).
func TestCyrillicKernPairs(t *testing.T) {
	cases := []struct {
		file string
		a, b rune
		want int // единицы шрифта
	}{
		{"RobotoCondensed-Regular.ttf", 'Т', 'о', -99},
		{"Rubik-Regular.ttf", 'Г', 'о', -124},
		{"GreatVibes-Regular.ttf", 'Г', 'о', -20},
	}
	m, err := loadFonts()
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	for _, c := range cases {
		sf := m[c.file]
		ga, _ := sf.GlyphIndex(&buf, c.a)
		gb, _ := sf.GlyphIndex(&buf, c.b)
		k, err := sf.Kern(&buf, ga, gb, fixed.Int26_6(sf.UnitsPerEm())<<6, xfont.HintingNone)
		if err != nil || k != fixed.Int26_6(c.want)<<6 {
			t.Errorf("%s «%c%c»: кернинг %v (%v), ждали %d", c.file, c.a, c.b, k, err, c.want)
		}
	}
}
