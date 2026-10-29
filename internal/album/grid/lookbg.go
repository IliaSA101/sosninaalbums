package grid

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Фоны по серии» (131 design §4, режим bg): на лист встаёт только фон шага — цвет, узор, фото
// фона набора. Места, фото, стикеры (и свои), надписи и название не трогаются, поэтому тело своё,
// без putDesign: он снял бы стикеры фотографа.

// BgRes — итог ApplyLookBg.
type BgRes struct {
	Changed  bool     // фон листа изменился (повтор — нет)
	Replaced bool     // изменился свой фон: он у листа был
	Missing  []string // ключи библиотеки без файла на этом ПК
}

// ApplyLookBg ставит на лист s фон шага tpl. Повтор того же — холостая правка (проект не
// меняется). Шаг без фона снимает фон листа: лист «идёт по серии». Лист 031 берёт страницу
// шага (vigPage). Шаг уже зажат ClampLook, поэтому отказа нет: негодный шаг или область — холостая правка. Свои из библиотеки — из libs; ключа нет — слой пропускается и попадает в Missing.
func ApplyLookBg(p *model.Project, s *model.Spread, tpl model.SpreadTpl, libs map[string]model.PhotoInfo) BgRes {
	return lookBg(p, s, tpl, libs, false)
}

// LookBgSame — фон листа s уже тот, что даёт шаг tpl (сверка без записи, проект не меняется).
func LookBgSame(p *model.Project, s *model.Spread, tpl model.SpreadTpl, libs map[string]model.PhotoInfo) bool {
	return !lookBg(p, s, tpl, libs, true).Changed
}

// ApplyLookCoverBg ставит на обложку фон макета c на весь лист (design §5: только фон — Bg и цвет,
// узор, фото сторон; корешок и места не трогаются; фото корешка уходит, если фон — фото всего
// листа). Replaced — свой фон обложки (цвет, узор, фото из съёмки или фото корешка) изменился.
// Обложки нет или макет фона не задаёт — холостая правка.
func ApplyLookCoverBg(p *model.Project, c *model.CoverTpl, libs map[string]model.PhotoInfo) BgRes {
	if p.Cover == nil || p.Cover.Sheet == nil || !c.HasBg() {
		return BgRes{}
	}
	return lookBg(p, p.Cover.Sheet, c.SheetBgTpl(), libs, false)
}

// LookCoverBgSame — фон обложки уже тот, что даёт c (сверка без записи; обложки нет или фона у
// макета нет — true).
func LookCoverBgSame(p *model.Project, c *model.CoverTpl, libs map[string]model.PhotoInfo) bool {
	return p.Cover == nil || p.Cover.Sheet == nil || !c.HasBg() || !lookBg(p, p.Cover.Sheet, c.SheetBgTpl(), libs, true).Changed
}

// HasBg — у листа свой фон: цвет страницы, узор или фото фона.
func HasBg(s *model.Spread) bool {
	return s.BgL != "" || s.BgR != "" || s.Pat != nil || s.PatL != nil || s.PatR != nil ||
		s.BgPhoto != nil || s.BgPhotoL != nil || s.BgPhotoR != nil
}

// lookBg — тело ApplyLookBg и LookBgSame; dry — без записи (и без записей Photos).
func lookBg(p *model.Project, s *model.Spread, tpl model.SpreadTpl, libs map[string]model.PhotoInfo, dry bool) (res BgRes) {
	if s.IsVignette() {
		tpl = vigPage(tpl)
	}
	t, _ := model.ClampSpreadTpl(tpl) // шаг без мест и фона тоже ставится: лист получает «без фона»
	scope := model.ScopeSpread
	parts, err := tplParts(p, s, scope, t)
	if t.Pages == 0 || err != nil {
		return res
	}
	r := &tplRes{p: p, s: s, libs: libs, gone: map[string]bool{}, dry: dry}
	sd := r.bgDesign(t, scope, parts)
	for k := range r.gone {
		res.Missing = append(res.Missing, k)
	}
	slices.Sort(res.Missing)
	o := putOpt{decor: true}
	m := placed(p, s, scope, sd, o)
	cur := designOf(p, s, scope, m.x0)
	cur.texts, cur.stickers = nil, nil          // стикеры и надписи в сверку не входят
	lost := s.BgPhotoS != nil && m.photo != nil // фото всего листа исключает фото корешка (putBg)
	if decorKey(cur, scope) == decorKey(m, scope) && !lost {
		return res
	}
	res.Changed, res.Replaced = true, HasBg(s) || lost
	if !dry {
		putBg(s, scope, m, o)
	}
	return res
}
