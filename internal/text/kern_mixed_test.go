package text

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// mixedFont — Lora с таблицей GPOS: настоящая пара kern (ra, rb) = -200 и lookup из ns общих
// подтаблиц ф. 2 (evilFmt2): у пары с первым глифом вне покрытия (чётный) один поиск —
// 643 шага на пару при ns = 40, у пары с покрытым (нечётным) — ещё два поиска, бюджет 1024
// исчерпан.
// Возвращает стиль и знаки: ra, rb — дешёвая пара, rc rc — дорогая.
func mixedFont(t *testing.T, ns int) (st Style, ra, rb, rc rune) {
	t.Helper()
	lora := builtinBytes(t, "Lora-Regular.ttf")
	sf, err := sfnt.Parse(lora)
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	var even, odd []rune
	for r := 'a'; r <= 'z'; r++ {
		g, err := sf.GlyphIndex(&buf, r)
		if err != nil || g == 0 {
			continue
		}
		if g%2 == 0 {
			even = append(even, r)
		} else {
			odd = append(odd, r)
		}
	}
	if len(even) < 2 || len(odd) < 1 {
		t.Fatalf("в Lora чётных глифов %d, нечётных %d", len(even), len(odd))
	}
	ra, rb, rc = even[0], even[1], odd[0]
	ga, _ := sf.GlyphIndex(&buf, ra)
	gb, _ := sf.GlyphIndex(&buf, rb)
	pairKern := gbLk(gbKern([3]int{int(ga), int(gb), -200}))
	g := gbSimple("DFLT", pairKern, evilFmt2(1, ns).lookups[0])
	b := withTables(t, lora, map[string][]byte{"GPOS": g.build().b})
	fc := CustomFace{Key: keyOf(b), Ext: "ttf"}
	name := "Смешанный " + fc.Key
	setCustom(t, []CustomFamily{{ID: FamilyID(name), Name: name, Faces: []CustomFace{fc}}}, loader(b))
	return Style{Font: FamilyID(name), Size: 40, Color: "#000000", Align: "left"}.Clamped(), ra, rb, rc
}

// Смешанный блок (161-30а): бюджет шагов исчерпан в одном из абзацев — кернинг GPOS у начертания
// выключен, и абзацы, посчитанные раньше с кернингом, обязаны пересчитаться без него. Иначе блок
// несёт два режима, а повторный Layout (кернинг уже выключен) даёт другие ширины: экран и
// выгрузка разойдутся до следующей правки. Без дорогого абзаца кернинг остаётся (контроль).
func TestLayoutRefusalRecomputesAllParas(t *testing.T) {
	const ns = 40
	for _, c := range []struct {
		name  string
		paras string // c — дешёвый абзац (kern -200), d — дорогой (бюджет исчерпан)
		kern  bool   // отказа нет: кернинг в дешёвых абзацах остаётся
	}{
		{"cd", "cd", false},
		{"dc", "dc", false},
		{"ccd", "ccd", false},
		{"cdc", "cdc", false},
		{"cc", "cc", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			st, ra, rb, rc := mixedFont(t, ns)
			cheap, dear := string([]rune{ra, rb}), string([]rune{rc, rc})
			checkMixedBudget(t, st, ra, rb, rc)
			var texts []string
			for _, k := range c.paras {
				texts = append(texts, map[rune]string{'c': cheap, 'd': dear}[k])
			}
			text := strings.Join(texts, "\n")
			box := Layout(text, st, 5000)
			if len(box.Lines) != len(texts) {
				t.Fatalf("строк %d, ждали %d", len(box.Lines), len(texts))
			}
			for i, l := range box.Lines {
				kerned := slices.ContainsFunc(l.DX, func(v float64) bool { return v != 0 })
				if c.paras[i] == 'c' && kerned != c.kern {
					t.Errorf("абзац %d: кернинг %v, ждали %v (DX %v)", i+1, kerned, c.kern, l.DX)
				}
			}
			// повторная раскладка (кернинг у начертания уже выключен, если был отказ) совпадает
			again := Layout(text, st, 5000)
			for i := range box.Lines {
				if !slices.Equal(box.Lines[i].DX, again.Lines[i].DX) || box.Lines[i].W != again.Lines[i].W {
					t.Errorf("абзац %d: первый вызов W=%v DX=%v, повторный W=%v DX=%v", i+1,
						box.Lines[i].W, box.Lines[i].DX, again.Lines[i].W, again.Lines[i].DX)
				}
			}
		})
	}
}

// checkMixedBudget — предусловие: дешёвая пара укладывается в бюджет шагов, дорогая — нет,
// а настоящий кернинг дешёвой пары виден (иначе тест пересчёта ничего не докажет).
func checkMixedBudget(t *testing.T, st Style, ra, rb, rc rune) {
	t.Helper()
	sf, _, d := fontData(st)
	if d == nil || d.kern == nil {
		t.Fatal("у шрифта нет разобранного GPOS")
	}
	var buf sfnt.Buffer
	gl := func(r rune) sfnt.GlyphIndex { g, _ := sf.GlyphIndex(&buf, r); return g }
	if _, ok := d.kern.gapsRaw(tagDFLT, []sfnt.GlyphIndex{gl(ra), gl(rb)}, make([]int32, 2)); !ok {
		t.Fatal("дешёвая пара не укладывается в бюджет: подбери ns")
	}
	if _, ok := d.kern.gapsRaw(tagDFLT, []sfnt.GlyphIndex{gl(rc), gl(rc)}, make([]int32, 2)); ok {
		t.Fatal("дорогая пара укладывается в бюджет: подбери ns")
	}
}
