package text

import (
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"golang.org/x/image/font/sfnt"
)

// Сквозной тест времени Layout у бюджета шагов GPOS (161-30): злой шрифт плюс текст до MaxRunes
// знаков. Бюджет считается на отрезок, а не на блок, но текст блока ограничен MaxRunes, так что
// суммарно шагов не больше gposStepsPerPair × MaxRunes. Отказ в конце длинного блока стоит ещё
// одного прохода без кернинга (пересчёт всех абзацев) — тоже в пределе.

// budgetText — n знаков кириллицы с большим числом разных глифов (один скрипт — один отрезок).
func budgetText(n int) string {
	al := []rune("абвгдеёжзийклмнопрстуфхцчшщъыьэюяАБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЭЮЯ")
	rs := make([]rune, n)
	for i := range rs {
		rs[i] = al[(i*7+i/3)%len(al)]
	}
	return string(rs)
}

// budgetFont — Lora с GPOS gpos как своя гарнитура; возвращает стиль и данные начертания.
func budgetFont(t *testing.T, gpos []byte) (Style, *faceData) {
	t.Helper()
	b := withTables(t, builtinBytes(t, "Lora-Regular.ttf"), map[string][]byte{"GPOS": gpos})
	fc := CustomFace{Key: keyOf(b), Ext: "ttf"}
	name := "Бюджет " + fc.Key
	setCustom(t, []CustomFamily{{ID: FamilyID(name), Name: name, Faces: []CustomFace{fc}}}, loader(b))
	st := Style{Font: FamilyID(name), Size: 14, Color: "#000000", Align: "left"}.Clamped()
	_, _, d := fontData(st)
	if d == nil || d.kern == nil {
		t.Fatal("у шрифта нет разобранного GPOS")
	}
	return st, d
}

// perPairSteps — шагов запроса на пару для текста s по таблице gpos (скрипт cyrl берётся из DFLT).
func perPairSteps(t *testing.T, gpos []byte, s string) (int, bool) {
	t.Helper()
	sf, err := sfnt.Parse(builtinBytes(t, "Lora-Regular.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	var buf sfnt.Buffer
	var gs []sfnt.GlyphIndex
	for _, r := range s {
		g, _ := sf.GlyphIndex(&buf, r)
		gs = append(gs, g)
	}
	tab := mustParse(t, gpos, int(sf.UnitsPerEm()))
	steps, ok := tab.gapsRaw(gposTag("cyrl"), gs, make([]int32, len(gs)))
	return steps / (len(gs) - 1), ok
}

func TestLayoutBudgetTime(t *testing.T) {
	const n = MaxRunes
	txt := budgetText(n)
	type fam struct {
		name string
		gen  func(ns int) []byte
		max  int // наибольшее число подтаблиц, которое помещается в файл
	}
	fams := []fam{{"в кэше (ф. 2)", func(ns int) []byte { return evilFmt2(1, ns).build().b }, 512}}
	if !testkit.Lite() { // 28 МБ таблицы: подтаблицы не помещаются в кэш процессора, шаг в 6 раз дороже
		fams = append(fams, fam{"с промахами кэша", func(ns int) []byte { return evilCache(ns, true) }, 240})
	}
	for _, f := range fams {
		c1, ok := perPairSteps(t, f.gen(1), txt)
		if !ok || c1 < 8 {
			t.Fatalf("%s: одна подтаблица — %d шагов на пару, ok=%v", f.name, c1, ok)
		}
		// Ниже бюджета с запасом 30 %: у коротких абзацев среднее число шагов на пару другое, чем у
		// длинного, а у одной подтаблицы есть постоянная часть, которой нет в расчёте.
		near := min(gposStepsPerPair*7/10/c1, f.max/2)
		for _, c := range []struct {
			name     string
			ns       int
			paras    int  // абзацев в блоке: бюджет считается на каждый отдельно
			wantKern bool // кернинг после Layout цел
		}{
			{"у бюджета, один абзац", near, 1, true},
			{"у бюджета, 50 абзацев", near, 50, true},
			{"за бюджетом, один абзац", min(2*near, f.max), 1, false},
			{"за бюджетом, 50 абзацев", min(2*near, f.max), 50, false},
		} {
			t.Run(f.name+"/"+c.name, func(t *testing.T) {
				gpos := f.gen(c.ns)
				if _, ok := perPairSteps(t, gpos, txt); ok != c.wantKern {
					t.Fatalf("%d подтаблиц: ok=%v при бюджете %d, ждали %v", c.ns, ok, gposStepsPerPair, c.wantKern)
				}
				st, d := budgetFont(t, gpos)
				// блок ровно в MaxRunes знаков вместе с переводами строки; абзацы сдвинуты друг от друга
				per := (n - c.paras + 1) / c.paras
				rs := []rune(txt)
				var parts []string
				for i := range c.paras {
					parts = append(parts, string(rs[i%7:i%7+per]))
				}
				block := strings.Join(parts, "\n")
				start := time.Now()
				lines := len(Layout(block, st, 180).Lines)
				el := time.Since(start) // один прогон: отказ взводит kernOff, повтор был бы уже без кернинга
				limit := testkit.Limit(500 * time.Millisecond)
				t.Logf("%d подтаблиц, %d абзацев: %s, строк %d, kernOff=%v", c.ns, c.paras, el, lines, d.kernOff.Load())
				if el > limit {
					t.Errorf("Layout шёл %s (> %s)", el, limit)
				}
				if d.kernOff.Load() == c.wantKern {
					t.Errorf("kernOff=%v, ждали %v", d.kernOff.Load(), !c.wantKern)
				}
			})
		}
	}
}

// Худший порядок: 49 абзацев у самого бюджета (кернинг цел), отказ — в последнем. Потрачено
// почти всё на кернинг, потом ещё проход без него: оба в пределе, а все абзацы — без кернинга.
func TestLayoutBudgetRefusalLast(t *testing.T) {
	st, ra, rb, rc := mixedFont(t, 40)
	_, _, d := fontData(st)
	const paras = 50
	per := (MaxRunes - paras + 1) / paras / 2 * 2 // чётное число знаков: пары ra rb целиком
	cheap := strings.Repeat(string([]rune{ra, rb}), per/2)
	dear := strings.Repeat(string(rc), per)
	block := strings.Repeat(cheap+"\n", paras-1) + dear
	start := time.Now()
	box := Layout(block, st, 180)
	el := time.Since(start)
	t.Logf("%d абзацев, отказ в последнем: %s, строк %d", paras, el, len(box.Lines))
	if limit := testkit.Limit(500 * time.Millisecond); el > limit {
		t.Errorf("Layout шёл %s (> %s)", el, limit)
	}
	if !d.kernOff.Load() {
		t.Fatal("отказа по бюджету не было: тест ничего не проверил")
	}
	for i, l := range box.Lines {
		for _, v := range l.DX {
			if v != 0 {
				t.Fatalf("строка %d: кернинг остался после отказа, DX %v", i+1, l.DX)
			}
		}
	}
}
