package model

import (
	"encoding/hex"
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Пределы сетки (спека 007, design §1–2).
const (
	// MaxPlaces — мест на разворот, столько же, сколько фото: иначе бывает «место
	// есть, а фото нельзя», и «Вернуть авто» при > MaxPhotos не посчитать.
	// Load лишние места отбрасывает, поэтому поднимать — только с новой FormatVersion.
	MaxPlaces = MaxPhotos
)

// FinitePlace — у места нет NaN/Inf: такое место не починить, только выбросить.
func FinitePlace(pl *Place) bool {
	for _, v := range [...]float64{pl.X, pl.Y, pl.W, pl.H} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// ClampPlace держит место внутри листа s (Geo): W, H — geom.MinPlace..ширина / высота листа,
// Rot — geom.NormDeg; без поворота X, Y внутрь, с поворотом — внутри hold (вписанный
// круг: от угла не зависит, поэтому поворот место не сдвигает). Шаг 0.01 мм.
// Идемпотентна. Значения должны быть конечными.
func (p *Project) ClampPlace(s *Spread, pl *Place) {
	g := p.Geo(s)
	clampPlaceIn(pl, g.W, g.H)
}

// ClampGrid — зажим мест прочитанного проекта (design 007 §2); файл не
// отвергается. Место с NaN/Inf выбрасывается (его фото остаётся в Photos и
// получит новое место), плохой или повторный ID — новый, неизвестный Fit — "",
// фото не из этого разворота или повтор — пустое место, лишние сверх MaxPlaces
// отбрасываются (сначала пустые с конца). Сетка без мест — «Авто».
func (p *Project) ClampGrid() {
	// ID мест и текстов — общие (clampDesign раньше): совпавший с текстом ID места
	// новый, иначе Order и стек слоёв неоднозначны.
	seenID, seenPhoto := map[string]bool{}, map[string]bool{}
	for _, s := range p.Sheets() {
		for _, t := range s.Texts {
			seenID[t.ID] = true
		}
	}
	for _, s := range p.Sheets() {
		if len(s.Grid) == 0 {
			s.Grid = nil
			continue
		}
		in := make(map[string]bool, len(s.Photos))
		for _, id := range s.Photos {
			in[id] = true
		}
		out := s.Grid[:0]
		for _, pl := range s.Grid {
			if pl == nil || !FinitePlace(pl) {
				continue
			}
			if !IsTextID(pl.ID) || seenID[pl.ID] { // формат тот же, что у блоков текста
				pl.ID = NewPlaceID(seenID)
			}
			seenID[pl.ID] = true
			if pl.Fit != FitContain {
				pl.Fit = ""
			}
			pl.Shape, pl.Round = ClampShape(pl.Shape, pl.Round)
			pl.Border = ClampBorder(p, pl.Border)
			if _, ok := p.Photos[pl.Photo]; pl.Photo != "" && (!ok || !in[pl.Photo] || seenPhoto[pl.Photo]) {
				pl.Photo = ""
			}
			if pl.Photo != "" {
				seenPhoto[pl.Photo] = true
			}
			p.ClampPlace(s, pl)
			out = append(out, pl)
		}
		clear(s.Grid[len(out):])
		s.Grid = trimPlaces(out)
		if len(s.Grid) == 0 {
			s.Grid = nil
		}
	}
	p.FixVignettes() // роли и числа виньетки — по уже зажатым местам и текстам (031)
}

// trimPlaces оставляет не больше MaxPlaces мест: сначала уходят пустые с конца,
// потом занятые с конца (их фото остаются в Photos разворота).
func trimPlaces(grid []*Place) []*Place {
	extra := len(grid) - MaxPlaces
	if extra <= 0 {
		return grid
	}
	drop := make([]bool, len(grid))
	for i := len(grid) - 1; i >= 0 && extra > 0; i-- {
		if grid[i].Photo == "" {
			drop[i] = true
			extra--
		}
	}
	out := grid[:0]
	for i, pl := range grid {
		if !drop[i] && len(out) < MaxPlaces {
			out = append(out, pl)
		}
	}
	return out
}

// ClampBoxesStrict — доли мест от интерфейса или шаблона (недоверенный ввод): NaN/Inf —
// отказ всей правки (место «примерно» не поставить), остальное — ClampBoxes (одно
// правило с шаблонами: w, h ≥ minBoxFrac, всё внутри 0..1, не больше MaxPlaces,
// лишние с конца отбрасываются; угол — в коробке). Пусто — ErrNoPlaces.
func ClampBoxesStrict(frac []Box) ([]Box, error) {
	if len(frac) == 0 {
		return nil, ErrNoPlaces
	}
	frac = frac[:min(len(frac), MaxPlaces)]
	if !slices.ContainsFunc(frac, func(f Box) bool { return !finiteBox(f) }) {
		return ClampBoxes(frac), nil
	}
	return nil, errBadBoxes
}

var errBadBoxes = errors.New("раскладка не применена: неверные размеры мест")

// Пределы оформления (спека 006, design §2.1). Файл с лишним не отвергается:
// значения зажимаются, лишнее отбрасывается.
const (
	// MaxScale — предел увеличения кадра: при 4× кадр 24 Мп в ячейке 100 мм
	// даёт ≈ 150 dpi, дальше только мыло. Один с geom (правило наклона 057).
	MaxScale = geom.MaxScale
	// MaxTexts — блоков текста на разворот. Load лишние отбрасывает: поднимать —
	// только с новой FormatVersion (64 — с v6, спека 026).
	MaxTexts = 64
	// MaxTotalTexts и MaxTotalRunes — общий запас на все тексты и подписи проекта при открытии файла.
	MaxTotalTexts   = 2000
	MaxTotalRunes   = 200_000
	MaxTextRunes    = 1000
	MaxCaptionRunes = 300
	MaxTextLines    = 50
	MinTextW        = 10.0 // мм
	// MaxNameRunes — длина названия разворота (лента и плитки, не печать).
	MaxNameRunes = 60
)

// round3 — до 0.001: кадр, отличающийся на шум, — холостая правка. Минус ноль не
// оставляет: сдвиг после наклона (057) бывает −0,0004, а «-0» в JSON — лишняя правка.
func round3(v float64) float64 {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return 0
	}
	return v
}

func Finite(v, def float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return def
	}
	return v
}

// Round1 — до 0.1 (шаг угла); минус ноль не оставляет.
func Round1(v float64) float64 {
	v = math.Round(v*10) / 10
	if v == 0 {
		return 0
	}
	return v
}

// MaxTilt — предел наклона фото в рамке, градусы. Не 45: при ±45 и ±135 round(R/90)
// даёт ничью (Go и TS округляют −0,5 по-разному), и пропорции прыгали бы.
const MaxTilt = 44.9

// SplitR раскладывает поворот фото на четверти (0…3, по часовой) и наклон
// (−44,9…44,9; у неприведённого R может выйти за предел — зажимает clampR).
// Единственное место, где R делится на 90: UI получает turn и tilt готовыми.
func SplitR(r float64) (turn int, tilt float64) {
	r = math.Mod(Finite(r, 0), 360) // огромное R не переполнит int
	q := math.Round(r / 90)
	return ((int(q) % 4) + 4) % 4, Round1(r - 90*q)
}

// ClampR приводит поворот к допустимому: четверть как есть, наклон ±maxTilt, шаг 0.1.
// Идемпотентна.
func ClampR(r float64) float64 {
	r = math.Mod(Finite(r, 0), 360)
	q := math.Round(r / 90)
	return geom.NormDeg(90*q + geom.ClampF(Round1(r-90*q), -MaxTilt, MaxTilt))
}

// ClampFrame приводит кадр к допустимому: S 1..maxScale, X/Y −1..1, R — clampR,
// NaN/Inf — по умолчанию, шаг 0.001. Кадр по умолчанию — nil (сброс не оставляет
// следа в файле).
func ClampFrame(f *Frame) *Frame {
	if f == nil {
		return nil
	}
	c := Frame{
		S: round3(geom.ClampF(Finite(f.S, 1), 1, MaxScale)),
		X: round3(geom.ClampF(Finite(f.X, 0), -1, 1)),
		Y: round3(geom.ClampF(Finite(f.Y, 0), -1, 1)),
		R: ClampR(f.R),
	}
	if c == (Frame{S: 1}) {
		return nil
	}
	return &c
}

// ClampColor — «#rrggbb» в нижнем регистре; иначе "" и false.
func ClampColor(c string) (string, bool) {
	if len(c) != 7 || c[0] != '#' {
		return "", false
	}
	if _, err := hex.DecodeString(c[1:]); err != nil {
		return "", false
	}
	return strings.ToLower(c), true
}

// BgColor — цвет фона: неверный — "" (наследовать).
func BgColor(c string) string {
	c, _ = ClampColor(c)
	return c
}

// ClampStyle — стиль текста из файла или интерфейса: цвет в нижнем регистре,
// остальное — text.Style.Clamped (гарнитура из таблицы, кегль, выравнивание).
func ClampStyle(st TextStyle) TextStyle {
	st.Color = BgColor(st.Color)
	return st.Clamped()
}

// CleanText — текст блока или подписи: переводы строк к LF, без управляющих и
// bidi-знаков (text.CleanText), не больше maxRunes знаков и maxTextLines строк.
func CleanText(s string, maxRunes int) string {
	s = strings.NewReplacer("\r\n", "\n", "\r", "\n", " ", "\n", " ", "\n").Replace(s)
	s = text.CleanText(s)
	if utf8.RuneCountInString(s) > maxRunes {
		s = string([]rune(s)[:maxRunes])
	}
	if lines := strings.SplitN(s, "\n", MaxTextLines+1); len(lines) > MaxTextLines {
		s = strings.Join(lines[:MaxTextLines], "\n")
	}
	return s
}

// CleanName — название разворота: одна строка без управляющих и bidi-знаков,
// без пробелов по краям, не больше maxNameRunes знаков.
func CleanName(s string) string { return cleanLine(s, MaxNameRunes) }

// oneLine — переводы строк и Tab в пробел. Один на пакет: сборка Replacer дороже самой замены,
// а cleanLine зовётся на каждое имя (1000 строк таблицы в окне «Имена» — 3000 раз, 043).
var oneLine = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ", "\u2028", " ", "\u2029", " ")

// cleanLine — то же правило со своим пределом n (переменные и имена 030).
func cleanLine(s string, n int) string {
	s = oneLine.Replace(s)
	s = strings.TrimSpace(text.CleanText(s))
	if utf8.RuneCountInString(s) > n {
		s = strings.TrimSpace(string([]rune(s)[:n]))
	}
	return s
}

// IsTextID — 12 знаков в нижнем hex, как у NewID.
func IsTextID(id string) bool {
	if len(id) != 12 || strings.ToLower(id) != id {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

// Пределы фото на фоне (design-backdrop §2).
const (
	minBackdrop  = 10.0 // мм, меньше рамку не сжать
	maxBackdropK = 3.0  // во сколько раз рамка может быть больше области
)

// ClampBackdrop приводит фон области scope листа s к допустимому: неизвестное фото — фон
// выбрасывается; W или H не число / ≤ 0 — рамка по области без поворота (у нулевой области —
// дальше по пределам); иначе размер 10 мм..3× области (у корешка — от его ширины, если она
// меньше 10, но не меньше MinSpine, до 3× большей стороны по обеим осям), поворот
// geom.NormDeg, центр рамки внутри области (X, Y сдвигаются): так фон нельзя потерять за краем. Шаг 0.01 мм. Прозрачность 0..1, шаг 0.01. Кадр — ClampFrame,
// у Frame.R четверть отбрасывается: остаётся наклон ±44,9 (045). Возвращает копию.
func (p *Project) ClampBackdrop(s *Spread, b *Backdrop, scope string) *Backdrop {
	if b == nil {
		return nil
	}
	if _, ok := p.Photos[b.Photo]; !ok {
		return nil
	}
	a := p.BdArea(s, scope)
	c := *b
	c.Frame = ClampFrame(b.Frame)
	if c.Frame != nil && c.Frame.R != 0 { // у фона только наклон (045): 90° — «Поворот» рамки
		f := *c.Frame
		_, f.R = SplitR(f.R)
		c.Frame = ClampFrame(&f)
	}
	c.Fade = geom.Round2(geom.ClampF(Finite(c.Fade, 0), 0, 1))
	pos := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }
	if !pos(c.W) || !pos(c.H) {
		c.X, c.Y, c.W, c.H, c.Rot = a.X, a.Y, a.W, a.H, 0
		if pos(a.W) && pos(a.H) && (scope != ScopeSpine || a.W >= MinSpine) {
			return &c
		}
		// Область нулевая (корешок 0 мм): рамку по ней поднимает нижний предел ниже — иначе
		// следующий зажим снова сбросил бы её по области (decisions 31).
	}
	lo, mw, mh := minBackdrop, maxBackdropK*a.W, maxBackdropK*a.H
	if scope == ScopeSpine { // корешок — узкая полоса (061): «Поворот» 90° не сжимает рамку,
		mw = maxBackdropK * max(a.W, a.H)        // а рамка по области (корешок 6 мм) уже допустима;
		mh, lo = mw, max(min(lo, a.W), MinSpine) // не ниже MinSpine: нулевая рамка не зажата повторно (decisions 31)
	}
	c.W = geom.Round2(geom.ClampF(c.W, lo, mw))
	c.H = geom.Round2(geom.ClampF(c.H, lo, mh))
	c.Rot = geom.NormDeg(c.Rot)
	// Дикие X, Y зажимаем заранее: иначе сдвиг центра терял бы точность.
	lim := 10 * (a.W + a.H)
	c.X = geom.ClampF(Finite(c.X, a.X), a.X-lim, a.X+lim)
	c.Y = geom.ClampF(Finite(c.Y, a.Y), a.Y-lim, a.Y+lim)
	cx, cy := c.TBox().Center()
	c.X += geom.ClampF(cx, a.X, a.X+a.W) - cx
	c.Y += geom.ClampF(cy, a.Y, a.Y+a.H) - cy
	c.X, c.Y = geom.Round2(c.X), geom.Round2(c.Y)
	return &c
}
