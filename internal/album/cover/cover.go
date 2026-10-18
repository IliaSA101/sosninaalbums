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
	pr, ok := model.CoverPresetFor(preset, p.Settings)
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

// ApplyPreset — числа пресета id для страницы альбома (CoverPresetFor). Сторонку шаблона
// (Фотосфера на формате из таблицы, 047 решение 52) берёт из пресета, иначе сторонку
// пользователя (W, H) пресет не трогает. Сторонка шаблона прежнего пресета — не своя:
// уходит в 0 (страница), иначе Нетпринт унёс бы 286×286 Фотосферы как «свою».
// Им же работает «Исправить» старой обложки.
func ApplyPreset(p *model.Project, id string) error {
	if p.Cover == nil {
		return ErrNone
	}
	pr, ok := model.CoverPresetFor(id, p.Settings)
	if !ok {
		return ErrPreset
	}
	spec := pr.Spec
	if spec.W <= 0 && spec.H <= 0 {
		cur := p.Cover.Spec
		spec.W, spec.H = cur.W, cur.H
		if was, ok := model.CoverPresetFor(cur.Preset, p.Settings); ok && (was.Spec.W > 0 || was.Spec.H > 0) &&
			was.Spec.W == cur.W && was.Spec.H == cur.H {
			spec.W, spec.H = 0, 0
		}
	}
	p.Cover.Spec = spec
	return nil
}

// SetSpec — свои числа обложки (зажим ClampCoverSpec). Пресет остаётся, только если числа
// совпали с его шаблоном для страницы альбома (samePreset); иначе — «Свои размеры» с теми же
// числами. Безопасные линии в карточке не правятся: остаются прежние (047).
func SetSpec(p *model.Project, s model.CoverSpec) error {
	if p.Cover == nil {
		return ErrNone
	}
	s = model.ClampCoverSpec(s)
	s.Safe, s.SpineSafe = p.Cover.Spec.Safe, p.Cover.Spec.SpineSafe
	if !samePreset(p, s) {
		s.Preset = model.CoverCustom
	}
	p.Cover.Spec = s
	return nil
}

// samePreset — числа s — ровно пресет s.Preset для страницы альбома (CoverPresetFor). Сторонка
// в счёт, только если её задаёт шаблон (Фотосфера): сравнивается действующая, 0 — страница.
// Безопасные линии — нет: это направляющие пресета.
func samePreset(p *model.Project, s model.CoverSpec) bool {
	pr, ok := model.CoverPresetFor(s.Preset, p.Settings)
	if !ok {
		return false
	}
	d := pr.Spec
	if d.W > 0 || d.H > 0 {
		page := p.Geo(nil).Pages[0]
		side := func(v, pg float64) float64 {
			if v <= 0 {
				return pg
			}
			return v
		}
		if side(s.W, page.W) != side(d.W, page.W) || side(s.H, page.H) != side(d.H, page.H) {
			return false
		}
	}
	return s.Wrap == d.Wrap && s.Hinge == d.Hinge && s.A == d.A && s.B == d.B && slices.Equal(s.Steps, d.Steps)
}

// FollowFormat — смена формата страницы (old — прежние настройки): обложка ровно по шаблону
// прежнего формата переходит на шаблон нового (047 решение 53); свои числа не трогает.
// Безопасные линии в сравнении не участвуют (070 Е): обложка v10 без линий тоже переходит
// и получает линии шаблона. Обложка «из страницы» (W/H 0) шаблоном не считается — не трогаем.
// Зовёт обёртка Editor.SetSettings тем же шагом отмены (spreads не импортирует cover).
func FollowFormat(p *model.Project, old model.Settings) {
	c := p.Cover
	if c == nil {
		return
	}
	was, ok := model.CoverPresetFor(c.Spec.Preset, old)
	if !ok || !sameSpec(c.Spec, was.Spec) {
		return
	}
	now, _ := model.CoverPresetFor(c.Spec.Preset, p.Settings)
	c.Spec = now.Spec
}

// sameSpec — те же числа обложки (id и безопасные линии не в счёт: это направляющие).
func sameSpec(a, b model.CoverSpec) bool {
	return a.W == b.W && a.H == b.H && a.Wrap == b.Wrap && a.Hinge == b.Hinge && a.A == b.A && a.B == b.B &&
		slices.Equal(a.Steps, b.Steps)
}

// SetLayout — раскладка обложки kind (front | both | wrap; "" — front): фото фона переходят
// в новые зоны (design 032 §2). Кадр при переносе — по умолчанию: пропорции зоны другие; наклон остаётся (045, FitArea).
// Снятое с фона фото остаётся в альбоме, если стоит в месте. Фото корешка (061) бывает только
// в both: при уходе из неё снимается, как задняя в front; отмена вернёт его тем же шагом.
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
	s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.BgPhotoS = nil, nil, nil, nil // в both приходят с пустым корешком
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
// текст — контрастный; на фото корешка плашка не ставится (061): она ушла бы под фото
// и всплыла бы, когда фото уберут. Повёрнутый текст в корешке уже есть — холостая правка.
func SpineText(p *model.Project) error {
	c := p.Cover
	if c == nil {
		return ErrNone
	}
	if p.SpineTextID() != "" {
		return nil
	}
	s, cg := c.Sheet, p.CoverGeo()
	if len(s.Texts) >= model.MaxTexts {
		return fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
	}
	if c.SpineBg == "" && s.BgPhotoS == nil {
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
