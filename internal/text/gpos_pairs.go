package text

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"golang.org/x/image/font/sfnt"
)

// Запрос пар кернинга по разобранной таблице (gpos.go). Читаем только XAdvance обоих глифов
// PairPos 1 и 2; всё чтение — по срезу GPOS с проверкой границ, вне среза «пары нет».

// gposStepsPerPair — бюджет шагов поиска на пару соседних знаков. Реальные шрифты (61
// встроенных с GPOS, 323 системных) — до 83 шагов на пару, запас больше ×12 (страж —
// TestGPOSBuiltinsParse); злая таблица с промахами кэша на 5000 знаках стоит 11–16 нс на шаг,
// поэтому предел невысок (161-26).
const gposStepsPerPair = 1024

// gu16 — u16 по смещению off в b или ok=false.
func gu16(b []byte, off int) (int, bool) {
	if off < 0 || off > len(b)-2 {
		return 0, false
	}
	return int(binary.BigEndian.Uint16(b[off:])), true
}

// gposQuery — состояние одного вызова gaps: счётчик шагов поиска (проба подтаблицы и каждая
// итерация двоичного поиска) и признак превышения бюджета.
type gposQuery struct {
	t            *gposTable
	steps, limit int
	over         bool
}

// spend списывает шаг; false — бюджет исчерпан (над ним работа не делается).
func (q *gposQuery) spend() bool {
	if q.steps >= q.limit {
		q.over = true
		return false
	}
	q.steps++
	return true
}

// find — двоичный поиск g среди n записей по stride байт от base, ключ — первые u16
// записи. Границы сужаются, поэтому на неотсортированных данных поиск не зацикливается.
func (q *gposQuery) find(base, n, stride, g int) (rec int, ok bool) {
	lo, hi := 0, n
	for lo < hi {
		if !q.spend() {
			return 0, false
		}
		m := (lo + hi) / 2
		v, got := gu16(q.t.b, base+stride*m)
		if !got {
			return 0, false
		}
		switch {
		case v < g:
			lo = m + 1
		case v > g:
			hi = m
		default:
			return m, true
		}
	}
	return 0, false
}

// findRange — двоичный поиск диапазона (start, end, value) формата 2 среди n записей по 6 байт.
func (q *gposQuery) findRange(base, n, g int) (start, val int, ok bool) {
	lo, hi := 0, n
	for lo < hi {
		if !q.spend() {
			return 0, 0, false
		}
		m := (lo + hi) / 2
		r := base + 6*m
		s, ok1 := gu16(q.t.b, r)
		e, ok2 := gu16(q.t.b, r+2)
		v, ok3 := gu16(q.t.b, r+4)
		if !ok1 || !ok2 || !ok3 {
			return 0, 0, false
		}
		switch {
		case e < g:
			lo = m + 1
		case s > g:
			hi = m
		default:
			return s, v, true
		}
	}
	return 0, 0, false
}

// coverage — индекс глифа в покрытии или -1.
func (q *gposQuery) coverage(off, g int) int {
	f, ok1 := gu16(q.t.b, off)
	n, ok2 := gu16(q.t.b, off+2)
	if !ok1 || !ok2 {
		return -1
	}
	switch f {
	case 1:
		if m, ok := q.find(off+4, n, 2, g); ok {
			return m
		}
	case 2:
		if s, si, ok := q.findRange(off+4, n, g); ok {
			return si + g - s
		}
	}
	return -1
}

// class — класс глифа по ClassDef (нет определения или глифа — 0).
func (q *gposQuery) class(off, g int) int {
	if off == 0 || !q.spend() {
		return 0
	}
	f, ok := gu16(q.t.b, off)
	if !ok {
		return 0
	}
	switch f {
	case 1:
		start, ok1 := gu16(q.t.b, off+2)
		n, ok2 := gu16(q.t.b, off+4)
		if ok1 && ok2 && g >= start && g < start+n {
			c, _ := gu16(q.t.b, off+6+2*(g-start))
			return c
		}
	case 2:
		n, ok := gu16(q.t.b, off+2)
		if !ok {
			return 0
		}
		_, c, _ := q.findRange(off+4, n, g)
		return c
	}
	return 0
}

// xadv — XAdvance из ValueRecord формата vf по адресу off. Нет поля, чтение вне таблицы и
// значение вне [-upem, upem] — 0 (не учитывается).
func (q *gposQuery) xadv(vf, off int) int {
	if vf&4 == 0 {
		return 0
	}
	v, ok := gu16(q.t.b, off+2*bits.OnesCount8(uint8(vf&3)))
	if v >= 0x8000 { // int16 без преобразования типов
		v -= 0x10000
	}
	if ok && v <= q.t.upem && v >= -q.t.upem {
		return v
	}
	return 0
}

// pair — сдвиги после первого и второго глифа; applied — подтаблица сработала (даже с
// нулевой записью): lookup для этой позиции закончен.
func (q *gposQuery) pair(s *pairSub, g1, g2 int) (a1, a2 int, applied bool) {
	if !q.spend() {
		return 0, 0, false
	}
	ci := q.coverage(s.cov, g1)
	if ci < 0 {
		return 0, 0, false
	}
	if s.format == 2 {
		c1, c2 := q.class(s.cd1, g1), q.class(s.cd2, g2)
		if c1 >= s.c1 || c2 >= s.c2 {
			return 0, 0, false
		}
		r := s.off + 16 + (c1*s.c2+c2)*(s.n1+s.n2)
		return q.xadv(s.vf1, r), q.xadv(s.vf2, r+s.n1), true
	}
	nps, ok := gu16(q.t.b, s.off+8)
	if !ok || ci >= nps {
		return 0, 0, false
	}
	po, ok := gu16(q.t.b, s.off+10+2*ci)
	if !ok {
		return 0, 0, false
	}
	ps := s.off + po
	n, ok := gu16(q.t.b, ps)
	if !ok {
		return 0, 0, false
	}
	rec := 2 + s.n1 + s.n2
	m, found := q.find(ps+2, n, rec, g2)
	if !found {
		return 0, 0, false
	}
	r := ps + 2 + rec*m
	return q.xadv(s.vf1, r+2), q.xadv(s.vf2, r+2+s.n1), true
}

// lookupsFor — lookups kern скрипта; скрипта нет в таблице — DFLT, dflt, latn (порядок HarfBuzz).
// Скрипт есть, но без фичи kern — кернинга нет (в браузере так же).
func (t *gposTable) lookupsFor(script uint32) []int {
	if lks, ok := t.scripts[script]; ok {
		return lks
	}
	for _, alt := range [...]uint32{tagDFLT, tagDflt, tagLatn} {
		if lks, ok := t.scripts[alt]; ok {
			return lks
		}
	}
	return nil
}

var (
	tagDFLT = gposTag("DFLT")
	tagDflt = gposTag("dflt")
	tagLatn = gposTag("latn")
)

// run проходит lookup по глифам: на каждой позиции подтаблицы по порядку, первая сработавшая
// закрывает позицию; ValueFormat2 ≠ 0 — второй глиф пары пропускается (как в HarfBuzz).
func (q *gposQuery) run(lk *kernLookup, gs []sfnt.GlyphIndex, out []int32) bool {
	for i := 0; i+1 < len(gs); {
		step := 1
		for si := range lk.subs {
			s := &lk.subs[si]
			a1, a2, ok := q.pair(s, int(gs[i]), int(gs[i+1]))
			if q.over {
				return false
			}
			if !ok {
				continue
			}
			out[i] += int32(a1)   //nolint:gosec // |a1| ≤ upem ≤ 16384
			out[i+1] += int32(a2) //nolint:gosec // |a2| ≤ upem ≤ 16384
			if s.vf2 != 0 {
				step = 2
			}
			break
		}
		i += step
	}
	return true
}

// gapsRaw считает сдвиги ПОСЛЕ каждого глифа отрезка одного скрипта, в единицах шрифта:
// сумма по lookups, итог на знак зажат в ±upem. out не короче gs. Без recover (сырое тело).
// steps — потрачено шагов поиска; ok=false — бюджет gposStepsPerPair × (n−1) исчерпан, out негоден.
func (t *gposTable) gapsRaw(script uint32, gs []sfnt.GlyphIndex, out []int32) (steps int, ok bool) {
	n := len(gs)
	clear(out[:n])
	if n < 2 {
		return 0, true
	}
	q := gposQuery{t: t, limit: gposStepsPerPair * (n - 1)}
	for _, li := range t.lookupsFor(script) {
		if !q.run(t.lookups[li], gs, out) {
			return q.steps, false
		}
	}
	for i := range out[:n] {
		out[i] = int32(max(-t.upem, min(t.upem, int(out[i])))) //nolint:gosec // зажато в ±upem ≤ 16384
	}
	return q.steps, true
}

// gaps — обёртка для продукта: nil — сдвиги посчитаны; errGPOSSteps — бюджет шагов; паника
// сырого тела (не sfnt) — errGPOS с текстом паники. Любая ошибка — кернинг GPOS у начертания
// выключает вызывающий. Короткий out — ошибка вызывающего, а не порча шрифта: паника без recover.
func (t *gposTable) gaps(script uint32, gs []sfnt.GlyphIndex, out []int32) error {
	if len(out) < len(gs) {
		panic("text: gaps: out короче gs")
	}
	var ok bool
	if v := panicked(func() { _, ok = t.gapsRaw(script, gs, out) }); v != nil {
		return fmt.Errorf("%w: %v", errGPOS, v)
	}
	if !ok {
		return errGPOSSteps
	}
	return nil
}
