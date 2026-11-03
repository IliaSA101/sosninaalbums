package text

import (
	"unicode"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Кернинг строк своих шрифтов (161): Go сам считает сдвиги по GPOS (gpos.go) и кладёт их в
// Line.DX; экран и JPEG рисуют по одной таблице. Скрипт знака решает, какие lookups берутся,
// как у браузера (HarfBuzz): цифры и знаки берут скрипт соседней буквы.

const (
	scriptCommon uint32 = 0 // цифры, пробел, знаки, комбинирующие: берут скрипт соседа
	scriptNone   uint32 = 1 // письменность справа налево: без кернинга (не тег)
)

// scriptTag — тег скрипта GPOS знака; scriptCommon — «как сосед», scriptNone — без кернинга.
// Прочие письменности — DFLT.
func scriptTag(r rune) uint32 {
	switch {
	case r < 0x80 && !unicode.IsLetter(r):
		return scriptCommon // быстрый путь: цифры, пробел, знаки ASCII
	case unicode.Is(unicode.Common, r) || unicode.Is(unicode.Inherited, r):
		return scriptCommon
	case unicode.Is(unicode.Cyrillic, r):
		return gposTag("cyrl")
	case unicode.Is(unicode.Latin, r):
		return tagLatn
	case unicode.Is(unicode.Greek, r):
		return gposTag("grek")
	case unicode.Is(unicode.Armenian, r):
		return gposTag("armn")
	case unicode.Is(unicode.Georgian, r):
		return gposTag("geor")
	case isRTL(r):
		return scriptNone
	}
	return tagDFLT
}

// isRTL — письменности справа налево: Go кернит в логическом порядке, HarfBuzz — после
// переворота, пары были бы не те, поэтому кернинга у них нет (161-17).
func isRTL(r rune) bool {
	return unicode.In(r, unicode.Hebrew, unicode.Arabic, unicode.Syriac, unicode.Thaana,
		unicode.Nko, unicode.Samaritan, unicode.Mandaic, unicode.Adlam)
}

// paraScripts — скрипт каждого знака абзаца: scriptCommon берёт скрипт ближайшей буквы слева,
// в начале абзаца — первой справа, букв нет совсем — DFLT. Парные скобки не учитываются (161-17).
func paraScripts(rs []rune) []uint32 {
	cur := tagDFLT
	for _, r := range rs {
		if t := scriptTag(r); t != scriptCommon {
			cur = t
			break
		}
	}
	out := make([]uint32, len(rs))
	for i, r := range rs {
		if t := scriptTag(r); t != scriptCommon {
			cur = t
		}
		out[i] = cur
	}
	return out
}

// kernMode — чем кернятся знаки основного шрифта.
type kernMode uint8

const (
	kernSfnt kernMode = iota // нет фичи kern в GPOS: sfnt.Kern (таблица kern), как до 161
	kernGPOS                 // сдвиги из разобранного GPOS
	kernNone                 // GPOS признан негодным: без кернинга, тем же шрифтом
)

// kernState — режим кернинга начертания и таблица (для kernGPOS).
func (d *faceData) kernState() (kernMode, *gposTable) {
	switch {
	case d.kernOff.Load():
		return kernNone, nil
	case d.kern != nil:
		return kernGPOS, d.kern
	}
	return kernSfnt, nil
}

// glyphItem — знак абзаца с глифом; brk — перед ним кернинга нет (знак без глифа, ошибка ширины).
type glyphItem struct {
	g      sfnt.GlyphIndex
	src    glyphSrc
	script uint32
	brk    bool
}

// measureCustom — measure для своего шрифта: те же глифы и ширины, кернинг по режиму
// начертания d. Кернинг GPOS — внутри отрезка подряд идущих знаков основного шрифта одного
// скрипта; отрезок рвут знак запасного шрифта, знак без глифа и смена скрипта (браузер
// шейпит по отрезкам шрифта и скрипта). Между знаками запасного шрифта — sfnt.Kern, как раньше.
// Отказ GPOS: кернинг у начертания выключается (один раз, в журнал), абзац считается заново.
func measureCustom(sf, fb *sfnt.Font, d *faceData, s string) *para {
	mode, t := d.kernState()
	p, err := measureKerned(sf, fb, mode, t, s)
	if err != nil {
		d.disableKern(err)
		p, _ = measureKerned(sf, fb, kernNone, nil, s)
	}
	return p
}

func measureKerned(sf, fb *sfnt.Font, mode kernMode, t *gposTable, s string) (*para, error) {
	var buf, fbBuf sfnt.Buffer
	main, alt := glyphSrc{sf, &buf}, glyphSrc{fb, &fbBuf}
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6
	rs := []rune(s)
	scripts := paraScripts(rs)
	p := &para{custom: true}
	var items []glyphItem
	brk := true // начало абзаца
	for i, r := range rs {
		src, g, ok := pickGlyph(main, alt, r)
		if !ok {
			brk = true
			continue
		}
		adv, err := src.f.GlyphAdvance(src.b, g, unit, xfont.HintingNone)
		if err != nil {
			brk = true
			continue
		}
		items = append(items, glyphItem{g, src, scripts[i], brk})
		p.r, p.adv = append(p.r, r), append(p.adv, adv)
		brk = false
	}
	gaps := make([]fixed.Int26_6, len(items))
	if err := fillGaps(gaps, items, main.f, mode, t, unit); err != nil {
		return nil, err
	}
	var pos fixed.Int26_6
	p.x = make([]fixed.Int26_6, len(items))
	for i := range items {
		p.x[i] = pos
		pos += p.adv[i] + gaps[i]
	}
	return p, nil
}

// fillGaps — сдвиг после каждого знака: gaps[i] между items[i] и items[i+1], единицы шрифта ×64.
func fillGaps(gaps []fixed.Int26_6, items []glyphItem, mainFont *sfnt.Font, mode kernMode, t *gposTable, unit fixed.Int26_6) error {
	for a := 0; a < len(items); {
		e := a + 1 // отрезок [a,e): один шрифт, без разрывов; для основного — и один скрипт
		for e < len(items) && !items[e].brk && items[e].src.f == items[a].src.f &&
			(items[a].src.f != mainFont || mode != kernGPOS || items[e].script == items[a].script) {
			e++
		}
		seg := items[a:e]
		switch {
		case len(seg) < 2:
		case seg[0].src.f != mainFont: // запасной шрифт: sfnt.Kern, как до 161
			sfntGaps(gaps[a:e], seg, unit)
		case mode == kernSfnt:
			sfntGaps(gaps[a:e], seg, unit)
		case mode == kernGPOS && seg[0].script != scriptNone:
			if err := gposGaps(gaps[a:e], seg, t); err != nil {
				return err
			}
		}
		a = e
	}
	return nil
}

// sfntGaps — кернинг sfnt между соседними знаками отрезка.
func sfntGaps(gaps []fixed.Int26_6, seg []glyphItem, unit fixed.Int26_6) {
	for i := 0; i+1 < len(seg); i++ {
		if k, err := seg[0].src.f.Kern(seg[0].src.b, seg[i].g, seg[i+1].g, unit, xfont.HintingNone); err == nil {
			gaps[i] = k
		}
	}
}

// gposGaps — сдвиги GPOS отрезка одного скрипта; сдвиг в единицах шрифта (×64 — единицы Int26_6).
func gposGaps(gaps []fixed.Int26_6, seg []glyphItem, t *gposTable) error {
	gs := make([]sfnt.GlyphIndex, len(seg))
	for i, it := range seg {
		gs[i] = it.g
	}
	out := make([]int32, len(seg))
	if err := t.gaps(seg[0].script, gs, out); err != nil {
		return err
	}
	for i, v := range out {
		gaps[i] = fixed.Int26_6(v) << 6
	}
	return nil
}
