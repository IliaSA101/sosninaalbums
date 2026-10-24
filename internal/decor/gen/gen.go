// Package gen — процедурные узоры фона и стикеры программы (спека 048, design §4): параметры
// генератора → пиксели в точном dpi. Без зависимостей от проекта: модель, View, рендер и
// /media/gen зовут одни и те же функции, поэтому экран и печать совпадают.
//
// Правила (решение 0.12 №90): геометрия в мм, мотив поштучно по своим мм-координатам; свой ГСЧ
// (paint/rand.go), порядок вызовов не зависит от dpi; выпущенный генератор не меняется — эталоны в
// testdata/golden.json, правка = новый ключ (dots2).
package gen

import (
	"math"
	"strings"
	"sync"
)

// Spec — параметры одного узора или стикера; в модели — Pattern и Art.
type Spec struct {
	Kind   string   // ключ генератора [a-z0-9-]{1,32}, заморожен
	Inks   []string // #rrggbb; пусто или короче Meta.Inks — дополняется цветами генератора
	Scale  float64  // 0.25..4, 0 — 1 (у узоров)
	Seed   uint32   // у Random; у остальных зажим обнуляет
	Aspect float64  // ширина/высота рамки растяжимого стикера (Meta.Stretch, спека 049); иначе 0
}

// Meta — описание генератора для каталога, карточки и FitStickers.
type Meta struct {
	Kind, Name string
	Inks       []string // цвета по умолчанию; len — сколько пикеров
	Aspect     float64  // стикер: ширина/высота (у растяжимого — для первой вставки); узор — 0
	Random     bool     // есть «Перемешать»
	Paper      bool     // текстура CC0: затемняет цвет страницы
	Stretch    bool     // стикер без своей пропорции: рисуется заново в рамке (плашка, волна-лента)
	Image      bool     // стикер — готовая картинка (спека 116): без цветов, не перекрашивается
}

// Пределы Spec (design §1).
const (
	MaxInks  = 4
	MinScale = 0.25
	MaxScale = 4
	// MinAspect, MaxAspect — растяжимый стикер: 3 мм против двух листов 031 (решение №136).
	MinAspect = 1.0 / 500
	MaxAspect = 500
)

// gen — генератор: Meta и рисование в координатах canvas (узор — мм области, стикер — доли
// рамки 0..1).
type gen struct {
	meta  Meta
	draw  func(c *canvas, s Spec)
	ready func() bool // nil — всегда; текстура — есть ли её файл в сборке
	art   bool        // стикер (DrawArt), а не узор
}

func (g *gen) available() bool { return g.ready == nil || g.ready() }

var index = sync.OnceValue(func() map[string]*gen {
	m := make(map[string]*gen, len(patternGens)+len(artGens))
	for _, g := range append(append([]*gen(nil), patternGens...), artGens...) {
		if _, dup := m[g.meta.Kind]; dup || !ValidKind(g.meta.Kind) {
			panic("gen: ключ повторён или не по грамматике: " + g.meta.Kind)
		}
		m[g.meta.Kind] = g
	}
	return m
})

// find — генератор, доступный в этой сборке.
func find(kind string) (*gen, bool) {
	g, ok := index()[kind]
	if !ok || !g.available() {
		return nil, false
	}
	return g, true
}

func metas(gs []*gen) []Meta {
	out := make([]Meta, 0, len(gs))
	for _, g := range gs {
		if g.available() {
			m := g.meta
			m.Inks = append([]string(nil), m.Inks...)
			out = append(out, m)
		}
	}
	return out
}

// Patterns — узоры фона в порядке каталога (копии: правка снаружи реестр не трогает).
func Patterns() []Meta { return metas(patternGens) }

// Arts — процедурные стикеры в порядке каталога.
func Arts() []Meta { return metas(artGens) }

// Known — генератор есть в этой сборке; правильный, но неизвестный ключ (файл из новой
// версии) модель хранит и рисует пусто (решение №91).
func Known(kind string) bool {
	_, ok := find(kind)
	return ok
}

// Lookup — Meta генератора (копия); ok = Known.
func Lookup(kind string) (Meta, bool) {
	g, ok := find(kind)
	if !ok {
		return Meta{}, false
	}
	m := g.meta
	m.Inks = append([]string(nil), m.Inks...)
	return m, true
}

// IsArt — ключ процедурного стикера, а не узора.
func IsArt(kind string) bool {
	g, ok := find(kind)
	return ok && g.art
}

// ValidKind — грамматика ключа [a-z0-9-]{1,32}.
func ValidKind(kind string) bool {
	if len(kind) == 0 || len(kind) > 32 {
		return false
	}
	for i := range len(kind) {
		c := kind[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// Clamp — общий зажим модели и /media/gen: ключ не по грамматике → пустой Spec (модель его
// отбрасывает); цвета — #rrggbb нижним регистром, не больше, чем у генератора (неизвестного —
// MaxInks), плохой — цвет генератора на его месте (у неизвестного — конец списка); Scale — 0
// (по умолчанию, в том числе ровно 1: одна запись на один вид, холостая правка без шага) или
// 0.25..4 с шагом 0.01; Seed — только у Random; Aspect — только у Stretch.
func Clamp(s Spec) Spec {
	if !ValidKind(s.Kind) {
		return Spec{}
	}
	out := Spec{Kind: s.Kind, Seed: s.Seed}
	g, known := find(s.Kind)
	limit := MaxInks
	if known {
		limit = len(g.meta.Inks)
	}
	for i, v := range s.Inks {
		if i >= limit {
			break
		}
		c, ok := normInk(v)
		if !ok {
			if !known {
				break
			}
			c = g.meta.Inks[i]
		}
		out.Inks = append(out.Inks, c)
	}
	// Хвост из цветов по умолчанию не хранится: View отдаёт цвета дополненными (Resolve), и
	// та же палитра, присланная обратно без изменений, — холостая правка, а не шаг отмены.
	for known && len(out.Inks) > 0 && out.Inks[len(out.Inks)-1] == g.meta.Inks[len(out.Inks)-1] {
		out.Inks = out.Inks[:len(out.Inks)-1]
	}
	if len(out.Inks) == 0 {
		out.Inks = nil
	}
	out.Scale = clampScale(s.Scale)
	if known && !g.meta.Random {
		out.Seed = 0
	}
	if known && g.meta.Stretch && s.Aspect > 0 && !math.IsInf(s.Aspect, 0) {
		out.Aspect = min(max(s.Aspect, MinAspect), MaxAspect)
	}
	return out
}

func clampScale(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	v = math.Round(min(max(v, MinScale), MaxScale)*100) / 100
	if v == 1 {
		return 0
	}
	return v
}

// Resolve — Spec для рисования и карточки: Clamp, Scale 0 → 1, цвета дополнены цветами
// генератора, Aspect — рамки (у растяжимого без своего — Meta.Aspect). ok = Known.
func Resolve(s Spec) (Spec, bool) {
	s = Clamp(s)
	g, ok := find(s.Kind)
	if !ok {
		return s, false
	}
	if s.Scale == 0 {
		s.Scale = 1
	}
	for i := len(s.Inks); i < len(g.meta.Inks); i++ {
		s.Inks = append(s.Inks, g.meta.Inks[i])
	}
	if s.Aspect == 0 {
		s.Aspect = g.meta.Aspect
	}
	return s, true
}

// normInk — #rrggbb нижним регистром; ok = было семь знаков «#» + 6 hex.
func normInk(v string) (string, bool) {
	if len(v) != 7 || v[0] != '#' {
		return "", false
	}
	v = strings.ToLower(v)
	for i := 1; i < 7; i++ {
		c := v[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", false
		}
	}
	return v, true
}
