package model

import (
	"cmp"
	"crypto/sha1" //nolint:gosec // ID записи, не криптография (как library.IDFor)
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Фоны и стикеры из библиотеки (спека 048, design §1–2).

var (
	ErrPattern = errors.New("узор не применён: неверные параметры")
	ErrLibKey  = errors.New("свой фон не поставлен: неверный ключ библиотеки")
)

// Pattern — узор на фоне области: параметры генератора decor/gen, не файл (решение №75).
// Слоты Spread.Pat/PatL/PatR взаимоисключающие, как фото фона; узор лежит под фото фона.
type Pattern struct {
	Kind  string   `json:"kind"`            // ключ генератора [a-z0-9-]{1,32}, заморожен (решение №90)
	Inks  []string `json:"inks,omitempty"`  // 0–4 цвета #rrggbb; пусто — цвета генератора
	Scale float64  `json:"scale,omitempty"` // 0.25..4, шаг 0.01; 0 — 1
	Seed  uint32   `json:"seed,omitempty"`
	Fade  float64  `json:"fade,omitempty"` // 0..1, шаг 0.01, как Backdrop.Fade
	// Preset — плитка каталога sa:<пресет>, поставившая узор (решение №201): имя и отметка в
	// карточке и галерее. Ручная правка цветов, масштаба или seed его снимает (SetPattern).
	Preset string `json:"preset,omitempty"`
}

// presetPrefix — ID встроенной плитки каталога: "sa:" + ключ [a-z0-9-]{1,32}.
const presetPrefix = "sa:"

// ValidPreset — грамматика Pattern.Preset: sa:<ключ>. Неизвестный, но правильный хранится
// (каталог новой версии), UI тогда угадывает плитку сам.
func ValidPreset(id string) bool {
	rest, ok := strings.CutPrefix(id, presetPrefix)
	return ok && gen.ValidKind(rest)
}

// SameParams — у узоров одни цвета, масштаб и seed (после зажима): отметка плитки остаётся.
func (pt *Pattern) SameParams(o *Pattern) bool {
	return slices.Equal(pt.Inks, o.Inks) && pt.Scale == o.Scale && pt.Seed == o.Seed
}

// Clone — копия узора (Inks не общий).
func (pt *Pattern) Clone() *Pattern {
	if pt == nil {
		return nil
	}
	c := *pt
	c.Inks = slices.Clone(pt.Inks)
	return &c
}

// PatSlot — где лежит узор области; nil — область неизвестна.
func (s *Spread) PatSlot(scope string) **Pattern {
	switch scope {
	case ScopeSpread:
		return &s.Pat
	case ScopeLeft:
		return &s.PatL
	case ScopeRight:
		return &s.PatR
	}
	return nil
}

// Patterns — узоры листа по порядку: весь лист, левая, правая.
func (s *Spread) Patterns() []*Pattern {
	var out []*Pattern
	for _, pt := range []*Pattern{s.Pat, s.PatL, s.PatR} {
		if pt != nil {
			out = append(out, pt)
		}
	}
	return out
}

// SplitPat — узор всего листа уходит странице напротив scope (left | right): scope готовят под
// свой узор или без узора, вид другой страницы не меняется (фото так не разрезать). Узора листа
// нет или scope не страница — ничего.
func (s *Spread) SplitPat(scope string) {
	if s.Pat == nil {
		return
	}
	switch scope {
	case ScopeLeft:
		s.PatR = s.Pat
	case ScopeRight:
		s.PatL = s.Pat
	default:
		return
	}
	s.Pat = nil
}

// ClampPattern — узор к допустимому; возвращает копию. Ключ, цвета, масштаб и seed — gen.Clamp
// (один зажим с /media/gen, решение №91): ключ не по грамматике → nil (узор отбрасывается),
// правильный, но неизвестный — хранится (файл из новой версии). Прозрачность 0..1, шаг 0.01.
// Preset не по грамматике — снимается (узор остаётся).
func ClampPattern(pt *Pattern) *Pattern {
	if pt == nil {
		return nil
	}
	s := gen.Clamp(gen.Spec{Kind: pt.Kind, Inks: pt.Inks, Scale: pt.Scale, Seed: pt.Seed})
	if s.Kind == "" {
		return nil
	}
	out := &Pattern{Kind: s.Kind, Inks: s.Inks, Scale: s.Scale, Seed: s.Seed,
		Fade: geom.Round2(geom.ClampF(Finite(pt.Fade, 0), 0, 1))}
	if ValidPreset(pt.Preset) {
		out.Preset = pt.Preset
	}
	return out
}

// clampPatterns — узоры листа при чтении файла: при pat и patL/R остаётся pat (как фон).
func clampPatterns(s *Spread) {
	for _, scope := range patScopes {
		slot := s.PatSlot(scope)
		*slot = ClampPattern(*slot)
	}
	if s.Pat != nil {
		s.PatL, s.PatR = nil, nil
	}
}

// foldPatterns — лист 031 без страниц: узор страницы переходит на весь лист (левая важнее).
func foldPatterns(s *Spread) {
	if s.Pat == nil {
		s.Pat = cmp.Or(s.PatL, s.PatR)
	}
	s.PatL, s.PatR = nil, nil
}

// libKeyPrefix — ключ своего элемента библиотеки: "my:" + 16 hex (8 байт sha256 исходника).
const libKeyPrefix = "my:"

// ValidLibKey — грамматика ключа PhotoRef.Lib: ^my:[0-9a-f]{16}$.
func ValidLibKey(key string) bool {
	if len(key) != len(libKeyPrefix)+16 || key[:len(libKeyPrefix)] != libKeyPrefix {
		return false
	}
	for i := len(libKeyPrefix); i < len(key); i++ {
		c := key[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// LibPhotoID — ID записи Photos своего элемента: от ключа, а не от пути (путь к библиотеке
// на каждом компьютере свой). Префикс "lib\x00" не даёт совпасть с library.IDFor.
func LibPhotoID(key string) string {
	sum := sha1.Sum([]byte("lib\x00" + key)) //nolint:gosec // ID записи, не криптография
	return hex.EncodeToString(sum[:8])
}

// Стикеры (Ф2, design §1, §3): картинка на листе — своя (фото из Photos с ключом библиотеки)
// или процедурная (параметры генератора). Слой между местами и текстами (order.go).

const (
	// MaxStickers — стикеров на лист. Load лишние отбрасывает: поднимать — только со сменой
	// FormatVersion (правило пределов).
	MaxStickers = 64
	// MaxTotalStickers — на весь проект: Load отбрасывает лишние, правка сверх — отказ (CheckTotals).
	MaxTotalStickers = 2000
	// MinSticker — меньшая сторона рамки стикера, мм.
	MinSticker = 3.0
)

var (
	ErrStickers  = fmt.Errorf("на развороте не может быть больше %d стикеров", MaxStickers)
	ErrSticker   = errors.New("стикер не поставлен: неверный рисунок")
	errStickerTo = fmt.Errorf("в альбоме не может быть больше %d стикеров", MaxTotalStickers)
)

// Art — процедурный стикер: параметры генератора decor/gen (как Pattern, без масштаба и
// прозрачности — прозрачность у самого стикера).
type Art struct {
	Kind string   `json:"kind"` // ключ генератора [a-z0-9-]{1,32}, заморожен (решение №90)
	Inks []string `json:"inks,omitempty"`
	Seed uint32   `json:"seed,omitempty"`
}

// Sticker — картинка на листе: своя (Photo — ID в Project.Photos) либо процедурная (Art);
// ровно одно. Рамка — как у места: угол до поворота, поворот вокруг центра (geom.RotRect).
type Sticker struct {
	ID    string  `json:"id"` // NewID, уникален в проекте
	Photo string  `json:"photo,omitempty"`
	Art   *Art    `json:"art,omitempty"`
	X     float64 `json:"x"` // мм листа
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Rot   float64 `json:"rot,omitempty"` // градусы по часовой вокруг центра, (−180, 180]
	FlipH bool    `json:"flipH,omitempty"`
	FlipV bool    `json:"flipV,omitempty"`
	Fade  float64 `json:"fade,omitempty"` // 0..1, шаг 0.01, как Backdrop.Fade
}

// StickerSrc — что ставится стикером: процедурный рисунок Art или своё фото Photo (ID —
// LibPhotoID ключа). Art != nil — рисунок, иначе фото.
type StickerSrc struct {
	Art   *Art
	Photo PhotoInfo
}

// Rect — рамка стикера для geom (группа, слои, выравнивание).
func (st *Sticker) Rect() geom.RotRect {
	return geom.RotRect{X: st.X, Y: st.Y, W: st.W, H: st.H, Rot: st.Rot}
}

// Clone — копия стикера (Art не общий).
func (st *Sticker) Clone() *Sticker {
	c := *st
	if st.Art != nil {
		a := *st.Art
		a.Inks = slices.Clone(st.Art.Inks)
		c.Art = &a
	}
	return &c
}

// ClampArt — рисунок к допустимому (gen.Clamp, как узор): ключ не по грамматике или ключ узора
// → nil; правильный, но неизвестный — хранится (файл из новой версии, решение №91).
func ClampArt(a *Art) *Art {
	if a == nil {
		return nil
	}
	s := gen.Clamp(gen.Spec{Kind: a.Kind, Inks: a.Inks, Seed: a.Seed})
	if s.Kind == "" || gen.Known(s.Kind) && !gen.IsArt(s.Kind) {
		return nil
	}
	return &Art{Kind: s.Kind, Inks: s.Inks, Seed: s.Seed}
}

// StickerAspect — своя пропорция стикера (ширина/высота); ok = false — пропорция свободна:
// растяжимая фигура (gen.Meta.Stretch, решение №136), неизвестный генератор или фото без
// размеров — рамка остаётся как есть.
func (p *Project) StickerAspect(st *Sticker) (float64, bool) {
	if st.Art != nil {
		m, ok := gen.Lookup(st.Art.Kind)
		if !ok || m.Stretch || !(m.Aspect > 0) {
			return 0, false
		}
		return m.Aspect, true
	}
	r, ok := p.Photos[st.Photo]
	if !ok || r.W <= 0 || r.H <= 0 {
		return 0, false
	}
	return float64(r.W) / float64(r.H), true
}

// StartAspect — пропорция рамки нового стикера: своя у рисунка и фото; у растяжимой фигуры —
// Meta.Aspect (только для первой вставки, решение №136); неизвестно — квадрат.
func (p *Project) StartAspect(st *Sticker) float64 {
	if a, ok := p.StickerAspect(st); ok {
		return a
	}
	if st.Art != nil {
		if m, ok := gen.Lookup(st.Art.Kind); ok && m.Aspect > 0 {
			return m.Aspect
		}
	}
	return 1
}

// stickerLim — большая сторона рамки стикера на листе g: два листа (за край можно).
func stickerLim(g Geo) float64 { return 2 * math.Max(g.W, g.H) }

// ClampSticker — стикер листа s к допустимому; возвращает копию или nil (отбросить): ровно одно
// из Photo/Art, фото есть в Photos, рисунок — ClampArt. Стороны MinSticker..2·лист, поворот
// NormDeg, прозрачность 0..1 (шаг 0.01), центр рамки — на листе, шаг 0.01 мм. ID не трогает.
func (p *Project) ClampSticker(s *Spread, st *Sticker) *Sticker {
	if st == nil || (st.Photo == "") == (st.Art == nil) {
		return nil
	}
	c := *st
	if st.Art != nil {
		if c.Art = ClampArt(st.Art); c.Art == nil {
			return nil
		}
	} else if _, ok := p.Photos[st.Photo]; !ok {
		return nil
	}
	g := p.Geo(s)
	lim := stickerLim(g)
	c.W = geom.Round2(geom.ClampF(Finite(c.W, MinSticker), MinSticker, lim))
	c.H = geom.Round2(geom.ClampF(Finite(c.H, MinSticker), MinSticker, lim))
	c.Rot = geom.NormDeg(c.Rot)
	c.Fade = geom.Round2(geom.ClampF(Finite(c.Fade, 0), 0, 1))
	cx := geom.ClampF(Finite(c.X, 0)+c.W/2, 0, g.W)
	cy := geom.ClampF(Finite(c.Y, 0)+c.H/2, 0, g.H)
	c.X, c.Y = geom.Round2(cx-c.W/2), geom.Round2(cy-c.H/2)
	return &c
}

// clampStickers — стикеры при чтении файла (ClampDesign): зажим каждого, сверх MaxStickers на
// лист и MaxTotalStickers на проект — отбросить; плохой или повторный ID (и с местами и
// текстами: стек слоёв различает их по ID) — новый.
func (p *Project) clampStickers() {
	taken := p.PlaceIDs()
	for _, s := range p.Sheets() {
		for _, t := range s.Texts {
			taken[t.ID] = true
		}
	}
	left := MaxTotalStickers
	for _, s := range p.Sheets() {
		var out []*Sticker
		for _, st := range s.Stickers {
			c := p.ClampSticker(s, st)
			if c == nil || len(out) >= MaxStickers || left == 0 {
				continue
			}
			if !IsTextID(c.ID) || taken[c.ID] {
				c.ID = NewID()
			}
			taken[c.ID] = true
			left--
			out = append(out, c)
		}
		s.Stickers = out
	}
}

// FitStickers — шаг конвейера после FitTexts (и в Load): стикер со своей пропорцией держит её —
// H = W / пропорция вокруг центра (не влезло в MinSticker..2·лист — ширина уступает);
// свободная пропорция (StickerAspect) — рамка как есть, стороны в MinSticker..2·лист. Центр —
// на листе. Идемпотентна: рамку с верной высотой и центром не трогает.
func (p *Project) FitStickers() {
	for _, s := range p.Sheets() {
		g := p.Geo(s)
		for _, st := range s.Stickers {
			p.fitSticker(g, st)
		}
	}
}

func (p *Project) fitSticker(g Geo, st *Sticker) {
	cx, cy := st.X+st.W/2, st.Y+st.H/2
	w, h := st.W, st.H
	lim := stickerLim(g)
	if a, ok := p.StickerAspect(st); ok {
		w = geom.ClampF(w, MinSticker, lim)
		if w/a < MinSticker {
			w = math.Min(MinSticker*a, lim)
		}
		if w/a > lim {
			w = lim * a
		}
		w, h = geom.Round2(w), geom.Round2(w/a)
	} else if w < MinSticker || w > lim || h < MinSticker || h > lim {
		// Свободная фигура после смены формата (Rescale по осям) могла выйти из MinSticker..lim:
		// иначе Load потом молча поменяет рамку (ClampSticker).
		w = geom.Round2(geom.ClampF(w, MinSticker, lim))
		h = geom.Round2(geom.ClampF(h, MinSticker, lim))
	}
	if w == st.W && h == st.H && cx >= 0 && cx <= g.W && cy >= 0 && cy <= g.H {
		return
	}
	cx, cy = geom.ClampF(cx, 0, g.W), geom.ClampF(cy, 0, g.H)
	st.W, st.H = w, h
	st.X, st.Y = geom.Round2(cx-w/2), geom.Round2(cy-h/2)
}

// FindSticker — стикер по ID и лист (с обложкой), где он стоит.
func (p *Project) FindSticker(id string) (*Spread, int) {
	for _, s := range p.Sheets() {
		for i, st := range s.Stickers {
			if st.ID == id {
				return s, i
			}
		}
	}
	return nil, -1
}

// DeleteSticker — снять стикер (DeleteSelection); фото без дела уходит само (PruneRefs).
func (p *Project) DeleteSticker(id string) {
	if s, i := p.FindSticker(id); s != nil {
		s.Stickers = slices.Delete(s.Stickers, i, i+1)
		if len(s.Stickers) == 0 {
			s.Stickers = nil
		}
	}
}

// stickerCount — стикеров в проекте (с обложкой).
func (p *Project) stickerCount() int {
	n := 0
	for _, s := range p.Sheets() {
		n += len(s.Stickers)
	}
	return n
}

// checkStickers — итог правки не больше того, что Load оставит: на лист MaxStickers, на проект
// MaxTotalStickers (только если стало больше, чем в before: переполненный файл не «замерзает»).
func (p *Project) checkStickers(before *Project) error {
	for _, s := range p.Sheets() {
		if len(s.Stickers) > MaxStickers {
			return ErrStickers
		}
	}
	if n := p.stickerCount(); n > MaxTotalStickers && n > before.stickerCount() {
		return errStickerTo
	}
	return nil
}

// Виды узора и стикера (View, 048 design §6) — здесь, а не в viewtypes.go: тот у предела размера.

// PatternView — узор области для холста и экспорта (048 design §6). Pat — после зажима, цвета
// дополнены цветами генератора, Scale 0 → 1 (что рисуется, то и в карточке). Inks — сколько
// цветов у генератора (пикеров), Random — есть «Перемешать». Known = false — генератора нет в
// этой сборке (файл из новой версии): рисуется пусто, Name и URL пустые. URL — превью
// /media/gen без px (UI дописывает &px=).
type PatternView struct {
	Scope  string      `json:"scope"` // spread | left | right
	Area   layout.Rect `json:"area"`  // область, мм листа (как у фото фона)
	Pat    Pattern     `json:"pat"`
	Name   string      `json:"name"`
	Inks   int         `json:"inks"`
	Random bool        `json:"random"`
	Known  bool        `json:"known"`
	URL    string      `json:"url"`
	// Preset — плитка каталога, поставившая узор (решение №201; в Pat его нет); "" — узор
	// правили руками или альбом старше поля: UI угадывает плитку по цветам сам.
	Preset string `json:"preset,omitempty"`
}

// StickerView — стикер для холста и экспорта (048 design §6): рамка — угол до поворота, мм
// листа, поворот вокруг центра (как место); зеркало и прозрачность — картинки. Свой: Photo,
// Lib (имя UI берёт из каталога), URL /media/p/<id>, DPI по ширине рамки. Процедурный (Gen):
// Art — после зажима, цвета дополнены цветами генератора; Name, Inks — пикеров; Stretch —
// растяжимая фигура, рисуется в пропорции рамки (URL с a=W/H). URL — без px. Known = false —
// генератора нет в этой сборке: рисуется пусто, Name и URL пустые. Aspect — своя пропорция
// (ширина/высота), 0 — свободная.
type StickerView struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	Rot     float64 `json:"rot"`
	FlipH   bool    `json:"flipH"`
	FlipV   bool    `json:"flipV"`
	Fade    float64 `json:"fade"`
	Z       int     `json:"z"` // слой в общем стеке листа (zView)
	Photo   string  `json:"photo"`
	Lib     string  `json:"lib,omitempty"`
	Art     *Art    `json:"art,omitempty"`
	Name    string  `json:"name"`
	Inks    int     `json:"inks"`
	URL     string  `json:"url"`
	Gen     bool    `json:"gen"`
	Stretch bool    `json:"stretch"`
	Aspect  float64 `json:"aspect"`
	DPI     int     `json:"dpi"` // свой: эффективный; 0 — процедурный или размер неизвестен
	Known   bool    `json:"known"`
}
