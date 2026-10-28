package text

import (
	"container/list"
	"math"
	"strings"
	"sync"
	"unicode"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const (
	// MaxRunes — предел длины текста одного блока (защита от гигантских строк).
	MaxRunes = 5000
	// lineHeight — межстрочный интервал в долях кегля.
	lineHeight = 1.2
	// cacheSize — число записей LRU-кэша переносов: блоку «по тексту» нужны
	// два ключа (без переноса и с переносом), при 2000 блоках 2048 мало.
	cacheSize = 8192
	eps       = 1e-9
)

// Line — одна строка: текст без хвостовых пробелов, X и Y (базовая линия)
// относительно левого верхнего угла блока, W — ширина строки, мм.
type Line struct {
	Text string  `json:"text"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	W    float64 `json:"w"`
}

// Box — результат переноса: размеры блока и строки. Size — кегль в мм.
type Box struct {
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Size  float64 `json:"size"`
	Lines []Line  `json:"lines"`
}

// CleanText убирает из текста всё, что нельзя ни показать, ни измерить:
// нарушенный UTF-8, управляющие и форматирующие знаки (кроме \n), \r;
// табуляция становится пробелом; длина режется до MaxRunes.
func CleanText(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	n := 0
	for _, r := range s {
		switch {
		case r == '\n':
		case r == '\t':
			r = ' '
		case unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == unicode.ReplacementChar || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r):
			continue
		}
		if n >= MaxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// para — абзац в виде глифов: x[i] — начало знака i с учётом кернинга
// (в единицах шрифта ×64), adv[i] — его ширина.
type para struct {
	r   []rune
	x   []fixed.Int26_6
	adv []fixed.Int26_6
}

// width — ширина знаков [a,b) в единицах шрифта ×64.
func (p *para) width(a, b int) fixed.Int26_6 {
	if b <= a {
		return 0
	}
	return p.x[b-1] + p.adv[b-1] - p.x[a]
}

// measure строит para; знак без глифа в шрифте берётся из запасного fb (может
// быть nil), нет и там — выпадает. Ширина запасного знака — в единицах основного
// шрифта ×64 (ppem основного), кернинг только между знаками одного шрифта,
// как в браузере.
func measure(sf, fb *sfnt.Font, s string) *para {
	var buf, fbBuf sfnt.Buffer
	main, alt := glyphSrc{sf, &buf}, glyphSrc{fb, &fbBuf}
	p := &para{}
	walkGlyphs(main, alt, s, fixed.Int26_6(sf.UnitsPerEm())<<6, func(r rune, _ glyphSrc, _ sfnt.GlyphIndex, pos, adv fixed.Int26_6) {
		p.r = append(p.r, r)
		p.x = append(p.x, pos)
		p.adv = append(p.adv, adv)
	})
	return p
}

// Layout переносит текст по ширине w (мм) и возвращает строки с координатами.
// Стиль приводится к допустимому (Clamped), текст — к CleanText.
func Layout(s string, st Style, w float64) Box {
	st = st.Clamped()
	if c := usableCustom(st.Font); c != nil {
		sf, _ := font(st)
		fb, _ := fallback(st)
		var box Box
		if guarded(func() { box = layoutWith(s, st, w, sf, fb) }) {
			return box
		}
		c.markBroken("перенос строк") // дальше — Montserrat, как у пропавшего шрифта
	}
	sf, _ := font(st)
	fb, _ := fallback(st)
	return layoutWith(s, st, w, sf, fb)
}

// layoutWith — Layout заданным шрифтом sf и запасным fb (может быть nil); st уже Clamped.
func layoutWith(s string, st Style, w float64, sf, fb *sfnt.Font) Box {
	size := st.Size * 25.4 / 72
	box := Box{W: w, Size: size}
	if sf == nil || !(w > 0) || math.IsInf(w, 0) {
		return box
	}
	upem := float64(sf.UnitsPerEm())
	mm := func(f fixed.Int26_6) float64 { return float64(f) / 64 * size / upem }
	var buf sfnt.Buffer
	var asc, desc float64
	if m, err := sf.Metrics(&buf, fixed.Int26_6(sf.UnitsPerEm())<<6, xfont.HintingNone); err == nil {
		asc, desc = mm(m.Ascent), mm(m.Descent)
	}
	lh := lineHeight * size
	// Одиночный завершающий \n строки не даёт: так считает HTML/contenteditable
	// редактора, иначе рамка выше видимого текста на строку.
	paras := strings.Split(CleanText(s), "\n")
	if n := len(paras); n > 1 && paras[n-1] == "" {
		paras = paras[:n-1]
	}
	for _, par := range paras {
		box.Lines = append(box.Lines, wrap(measure(sf, fb, par), mm, w)...)
	}
	n := len(box.Lines)
	for i := range box.Lines {
		l := &box.Lines[i]
		l.Y = asc + float64(i)*lh
		switch st.Align {
		case "center":
			l.X = (w - l.W) / 2
		case "right":
			l.X = w - l.W
		}
	}
	box.H = asc + desc + float64(max(n-1, 0))*lh
	return box
}

// wrap — жадный перенос абзаца по пробелам; слово длиннее w рвётся по знакам.
// Пробелы на стыке строк пропадают, ведущие пробелы абзаца остаются.
func wrap(p *para, mm func(fixed.Int26_6) float64, w float64) []Line {
	n := len(p.r)
	if n == 0 {
		return []Line{{}}
	}
	var lines []Line
	emit := func(a, b int) {
		e := b
		for e > a && p.r[e-1] == ' ' {
			e--
		}
		lines = append(lines, Line{Text: string(p.r[a:e]), W: mm(p.width(a, e))})
	}
	fits := func(a, b int) bool { return mm(p.width(a, b)) <= w+eps }
	ls, le := 0, 0 // текущая строка [ls,le): le — конец последнего слова
	i := 0
	for i < n {
		ws := i
		for ws < n && p.r[ws] == ' ' {
			ws++
		}
		we := ws
		for we < n && p.r[we] != ' ' {
			we++
		}
		i = we
		if ws == we { // хвост из одних пробелов
			break
		}
		if le > ls {
			if fits(ls, we) {
				le = we
				continue
			}
			emit(ls, le)
			ls = ws
		}
		// слово в пустую строку; длинное рвём по знакам
		from := ws
		for !fits(ls, we) {
			k := from + 1
			for k < we && fits(ls, k+1) {
				k++
			}
			if k >= we {
				break
			}
			emit(ls, k)
			ls, from = k, k
		}
		le = we
	}
	if le > ls {
		emit(ls, le)
	}
	if len(lines) == 0 {
		lines = append(lines, Line{})
	}
	return lines
}

// Кэш переносов: ключ — текст, стиль, ширина с точностью 0.01 мм и поколение
// реестра своих шрифтов. Чистки кэша при смене набора мало: Layout, начатый
// до смены, положил бы старую строку уже после чистки.
type cacheKey struct {
	s   string
	st  Style
	w   int64
	gen uint64
}

type cacheEntry struct {
	key cacheKey
	box Box
}

var cache = struct {
	sync.Mutex
	m   map[cacheKey]*list.Element
	lru *list.List
}{m: map[cacheKey]*list.Element{}, lru: list.New()}

// Cached — Layout с LRU-кэшем на cacheSize записей. Ширина округляется до 0.01 мм
// и считается по округлённой, чтобы результат не зависел от того, кто спросил первым.
// Возвращает копию строк.
func Cached(s string, st Style, w float64) Box {
	st = st.Clamped()
	if math.IsNaN(w) || math.IsInf(w, 0) {
		return Layout(s, st, 0)
	}
	k := cacheKey{s, st, int64(math.Round(w * 100)), customGen.Load()}
	cache.Lock()
	if e, ok := cache.m[k]; ok {
		cache.lru.MoveToFront(e)
		b := e.Value.(*cacheEntry).box
		cache.Unlock()
		b.Lines = append([]Line(nil), b.Lines...)
		return b
	}
	cache.Unlock()
	b := Layout(s, st, float64(k.w)/100)
	cache.Lock()
	if _, ok := cache.m[k]; !ok {
		cache.m[k] = cache.lru.PushFront(&cacheEntry{k, b})
		for cache.lru.Len() > cacheSize {
			last := cache.lru.Back()
			cache.lru.Remove(last)
			delete(cache.m, last.Value.(*cacheEntry).key)
		}
	}
	cache.Unlock()
	b.Lines = append([]Line(nil), b.Lines...)
	return b
}
