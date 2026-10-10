package model

import (
	"fmt"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// MaxPhotos — предел фото на разворот (определяется скоростью раскладки).
// MaxSpreadPhotos (склейка и автосборка) меньше и от него не зависит.
const MaxPhotos = layout.MaxPhotos

// MaxFileSize — больше файл проекта не читаем: нормальный проект на тысячи
// фото занимает единицы мегабайт.
const MaxFileSize = 50 << 20

// MaxSpreads — разворотов в проекте при открытии файла. Нормальный альбом — десятки;
// предел защищает от файла, который раздул бы раскладку и экспорт.
const MaxSpreads = 1000

// MaxMarks — своих отметок в проекте (004). Load обрезает лишние с потерей: поднимать только
// со сменой FormatVersion. marksLimit — то же, тест уменьшает.
const MaxMarks = 100_000

var marksLimit = MaxMarks

// MarksLimit — действующий предел своих отметок: MaxMarks, в тесте меньше.
func MarksLimit() int { return marksLimit }

// SetMarksLimit — подмена предела в тесте; рабочему коду линтер вызов запрещает (023).
func SetMarksLimit(n int) { marksLimit = n }

// MaxFolders — папок в проекте. Load отбрасывает лишние с потерей: поднимать только со сменой
// FormatVersion. Съёмка — единицы папок; 10 000 — запас без квадратичных проверок.
const MaxFolders = 10_000

func ErrTooMany() error {
	return fmt.Errorf("на развороте не может быть больше %d фото", MaxPhotos)
}

// Пределы настроек, мм и dpi.
const (
	minPage, maxPage = 50.0, 1000.0
	minDPI, maxDPI   = 72, 1200
)

// ClampSettings приводит настройки к допустимым: NaN/Inf — значение по умолчанию,
// остальное зажимается в пределы. Поля и промежуток не больше четверти меньшей стороны страницы.
func ClampSettings(s Settings) Settings {
	d := DefaultSettings()
	fix := func(v, def, lo, hi float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = def
		}
		return math.Max(lo, math.Min(hi, v))
	}
	s.PageW = fix(s.PageW, d.PageW, minPage, maxPage)
	s.PageH = fix(s.PageH, d.PageH, minPage, maxPage)
	top := min(s.PageW, s.PageH) / 4
	s.Margin = fix(s.Margin, d.Margin, 0, top)
	s.Gap = fix(s.Gap, d.Gap, 0, top)
	if s.DPI == 0 {
		s.DPI = d.DPI
	}
	s.DPI = max(minDPI, min(maxDPI, s.DPI))
	s.Bg = BgColor(s.Bg)
	return s
}

// SplitBig режет разворот больше MaxPhotos на подряд идущие по ≤ MaxPhotos фото:
// альбомы, собранные до появления лимита, должны открываться. Раскладка такого
// разворота (ключ строился под другое число фото) сбрасывается; сетка с таким
// числом фото (чужой или битый файл) снимается — части становятся «Авто».
func (p *Project) SplitBig() {
	var out []*Spread
	for _, s := range p.Spreads {
		if s == nil || len(s.Photos) <= MaxPhotos {
			out = append(out, s)
			continue
		}
		for i := 0; i < len(s.Photos); i += MaxPhotos {
			photos := slices.Clone(s.Photos[i:min(len(s.Photos), i+MaxPhotos)])
			part := &Spread{ID: NewID(), Photos: photos}
			if i == 0 {
				// Первая часть — копия целиком (тексты, фон, название и будущие
				// поля, как при SplitSpread); обнуляется только то, что не годится.
				first := *s
				first.Photos, first.Grid, first.Layout, first.Variant = photos, nil, "", 0
				part = &first
			}
			out = append(out, part)
		}
	}
	p.Spreads = out
}

// DedupePhotos убирает повторы id с сохранением порядка: иначе повторы
// обходили бы лимит или ставили одно фото на разворот дважды.
func DedupePhotos(photos []PhotoInfo) []PhotoInfo {
	seen := make(map[string]bool, len(photos))
	out := make([]PhotoInfo, 0, len(photos))
	for _, ph := range photos {
		if !seen[ph.ID] {
			seen[ph.ID] = true
			out = append(out, ph)
		}
	}
	return out
}
