package cover

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Отказы правок обложки: текст — для уведомления в интерфейсе.
var (
	ErrSheet  = errors.New("у листа нет обложки")
	ErrExists = errors.New("обложка уже есть")
	ErrNone   = errors.New("обложки нет")
	ErrPreset = errors.New("неизвестная типография")
	ErrLayout = errors.New("неизвестная раскладка обложки")
)

// Create — новая пустая обложка: пресет preset ("" — CoverDefault), сторонка — страница
// альбома, «Фото на лицевой». В проекте-листе 031 обложки не бывает. Есть — ErrExists.
func Create(p *model.Project, preset string) error {
	switch {
	case p.Settings.Sheet:
		return ErrSheet
	case p.Cover != nil:
		return ErrExists
	}
	if preset == "" {
		preset = model.CoverDefault
	}
	pr, ok := model.CoverPresetOf(preset)
	if !ok {
		return ErrPreset
	}
	// Photos nil — как после Load (FixCoverSheet): иначе Save → Load давал бы другой JSON.
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec, Layout: model.CoverFront}
	return nil
}

// Delete снимает обложку; фото её мест уходят из альбома (PruneRefs), как при удалении
// разворота. Нет обложки — холостая правка.
func Delete(p *model.Project) { p.Cover = nil }

// ApplyPreset — числа пресета id; сторонку пользователя (W, H) пресет не трогает.
func ApplyPreset(p *model.Project, id string) error {
	if p.Cover == nil {
		return ErrNone
	}
	pr, ok := model.CoverPresetOf(id)
	if !ok {
		return ErrPreset
	}
	spec := pr.Spec
	spec.W, spec.H = p.Cover.Spec.W, p.Cover.Spec.H
	p.Cover.Spec = spec
	return nil
}

// SetSpec — свои числа обложки (зажим ClampCoverSpec). Пресет остаётся, только если числа
// совпали с его таблицей (сторонка не в счёт); иначе — «Свои размеры» с теми же числами.
func SetSpec(p *model.Project, s model.CoverSpec) error {
	if p.Cover == nil {
		return ErrNone
	}
	s = model.ClampCoverSpec(s)
	if !samePreset(s) {
		s.Preset = model.CoverCustom
	}
	p.Cover.Spec = s
	return nil
}

// samePreset — числа s (без сторонки) — ровно пресет s.Preset.
func samePreset(s model.CoverSpec) bool {
	pr, ok := model.CoverPresetOf(s.Preset)
	if !ok {
		return false
	}
	d := pr.Spec
	return s.Wrap == d.Wrap && s.Hinge == d.Hinge && s.A == d.A && s.B == d.B && slices.Equal(s.Steps, d.Steps)
}

// SetLayout — раскладка обложки kind (front | both | wrap; "" — front): фото фона переходят
// в новые зоны (design 032 §2). Кадр при переносе — по умолчанию: пропорции зоны другие.
// Снятое с фона фото остаётся в альбоме, если стоит в месте.
func SetLayout(p *model.Project, kind string) error {
	if kind == "front" {
		kind = model.CoverFront
	}
	if !model.ValidCoverLayout(kind) {
		return ErrLayout
	}
	c := p.Cover
	if c == nil {
		return ErrNone
	}
	if c.Layout == kind {
		return nil
	}
	s := c.Sheet
	had := map[string]*model.Backdrop{model.ScopeSpread: s.BgPhoto, model.ScopeLeft: s.BgPhotoL, model.ScopeRight: s.BgPhotoR}
	s.BgPhoto, s.BgPhotoL, s.BgPhotoR = nil, nil, nil
	// move — первый фон из областей from встаёт в область to; рамка по зоне — только при переносе.
	move := func(to string, from ...string) {
		for _, f := range from {
			if b := had[f]; b != nil {
				if f != to {
					b.FitArea(p.BdArea(s, to))
				}
				*s.Slot(to) = b
				return
			}
		}
	}
	switch kind {
	case model.CoverWrap:
		move(model.ScopeSpread, model.ScopeRight, model.ScopeSpread, model.ScopeLeft)
	case model.CoverBoth:
		move(model.ScopeRight, model.ScopeRight, model.ScopeSpread)
		move(model.ScopeLeft, model.ScopeLeft)
	default:
		move(model.ScopeRight, model.ScopeRight, model.ScopeSpread)
	}
	c.Layout = kind
	return nil
}

// SpineText — «Надпись на корешке»: текст по центру корешка, повёрнутый на −90° (снизу вверх,
// decisions 57), ширина переноса — высота сторонки без безопасных полей, кегль — от ширины
// корешка (строка ≈ 0,42·кегль мм ≤ 0,6 корешка). Плашки нет — ставится цвет лицевой,
// текст — контрастный. Повёрнутый текст в корешке уже есть — холостая правка.
func SpineText(p *model.Project) error {
	c := p.Cover
	if c == nil {
		return ErrNone
	}
	s, cg := c.Sheet, p.CoverGeo()
	for _, t := range s.Texts {
		if x, _ := model.BoxOf(t).Center(); t.Rot != 0 && x >= cg.X[3] && x <= cg.X[4] {
			return nil
		}
	}
	if len(s.Texts) >= model.MaxTexts {
		return fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
	}
	if c.SpineBg == "" {
		c.SpineBg = frontColor(p, s)
	}
	t := &model.TextBlock{ID: model.NewID(), Text: "Текст", Style: model.PlainStyle(), Rot: -90}
	t.Style.Size = math.Round(geom.ClampF(cg.Spine*1.4, 6, 36)*10) / 10 // шаг кегля 0,1, как у ручки
	if dark(c.SpineBg) {
		t.Style.Color = "#ffffff"
	}
	t.Style = model.ClampStyle(t.Style)
	g := p.Geo(s)
	t.W = geom.Round2(max(model.MinTextW, g.H-2*c.Spec.Wrap-2*p.Settings.Safe))
	h := text.Cached(t.Text, t.Style, t.W).H
	// Центр рамки — центр листа: угол до поворота = центр − Rotate(−90, W/2, h/2).
	dx, dy := geom.Rotate(t.Rot, t.W/2, h/2)
	t.X, t.Y = geom.Round2(g.W/2-dx), geom.Round2(g.H/2-dy)
	s.Texts = append(s.Texts, t)
	return nil
}

// frontColor — действующий цвет лицевой: свой, иначе альбома, иначе белый.
func frontColor(p *model.Project, s *model.Spread) string {
	for _, c := range []string{s.BgR, p.Settings.Bg} {
		if c != "" {
			return c
		}
	}
	return "#ffffff"
}

// dark — цвет #rrggbb тёмный (яркость < 0,5): на нём текст белый.
func dark(c string) bool {
	if len(c) != 7 {
		return false
	}
	v, err := strconv.ParseUint(c[1:], 16, 32)
	if err != nil {
		return false
	}
	r, g, b := float64(v>>16&0xff), float64(v>>8&0xff), float64(v&0xff)
	return (0.299*r+0.587*g+0.114*b)/255 < 0.5
}

// SetSpineBg — цвет плашки корешка (уже проверенный #rrggbb или "" — нет плашки).
func SetSpineBg(p *model.Project, color string) {
	if p.Cover != nil {
		p.Cover.SpineBg = color
	}
}

// SetGuide — файл-разметка (уже проверенный абсолютный путь; "" — убрать).
func SetGuide(p *model.Project, path string) {
	if p.Cover != nil && p.Cover.Guide != path {
		p.Cover.Guide, p.Cover.GuideAlt = path, "" // другой файл: запасной путь прежнего не годится
	}
}
