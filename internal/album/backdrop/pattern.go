package backdrop

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Узор на фоне области (спека 048, design §1, решения №75, №76): параметры генератора
// decor/gen в слотах Spread.Pat/PatL/PatR. Узор листа и узоры страниц взаимоисключающие, как
// фото фона; узор лежит под фото фона. Своё из библиотеки — фото фона с ключом (SetBackdrop).

// ApplyPattern — тело Editor.ApplyPattern: щелчок по плитке программы (решение №76) — узор pt
// в область scope и цвет её страниц color ("" — цвет не трогать; неверный — ErrPattern);
// прежний фон области (узоры и фото на ней) снимается. Узор с ключом не по грамматике —
// ErrPattern, неизвестная область или страница листа 031 — ErrScope; проект не тронут.
// Тот же узор повторно — холостая (сравнение JSON в Editor).
func ApplyPattern(p *model.Project, spreadID, scope string, pt model.Pattern, color string) error {
	np := model.ClampPattern(&pt)
	c, ok := model.ClampColor(color)
	if np == nil || !ok && color != "" {
		return model.ErrPattern
	}
	s := p.Sheet(spreadID)
	if s == nil {
		return nil
	}
	if _, ok := p.AreaOf(s, scope); !ok {
		return model.ErrScope
	}
	dropPatterns(s, scope)
	dropPhotos(s, scope)
	*s.PatSlot(scope) = np
	if c != "" {
		setColor(s, scope, c)
	}
	return nil
}

// ApplyBackdrop — тело Editor.ApplyBackdrop: своё из библиотеки — фото фона, как SetBackdrop,
// и узоры области снимаются (решение №76). Бросок фото и «Свой файл…» узор не снимают.
func ApplyBackdrop(p *model.Project, spreadID, scope string, ph model.PhotoInfo) error {
	if err := SetBackdrop(p, spreadID, scope, ph); err != nil {
		return err
	}
	if s := p.Sheet(spreadID); s != nil {
		dropPatterns(s, scope)
	}
	return nil
}

// SetPattern — тело Editor.SetPattern: цвета, масштаб, seed и прозрачность узора области.
// Ключ генератора меняет только ApplyPattern (другой узор — другая плитка). Отметка плитки
// (Preset, решение №201) — не от UI: остаётся, пока цвета, масштаб и seed те же (прозрачность
// плитку не меняет), иначе снимается. Узора нет или область неизвестна — холостая.
func SetPattern(p *model.Project, spreadID, scope string, pt model.Pattern) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	slot := s.PatSlot(scope)
	if slot == nil || *slot == nil {
		return
	}
	pt.Kind, pt.Preset = (*slot).Kind, ""
	np := model.ClampPattern(&pt)
	if np == nil {
		return
	}
	if np.SameParams(*slot) {
		np.Preset = (*slot).Preset
	}
	*slot = np
}

// ClearPattern — тело Editor.ClearPattern: снять узор области; цвет страниц и фото фона
// остаются. Неизвестная область или узора нет — холостая.
func ClearPattern(p *model.Project, spreadID, scope string) {
	if s := p.Sheet(spreadID); s != nil {
		if slot := s.PatSlot(scope); slot != nil {
			*slot = nil
		}
	}
}

// dropPatterns снимает узоры, лежащие на области scope. Узор всего листа под страницей
// остаётся другой странице (SplitPat): её вид не меняется.
func dropPatterns(s *model.Spread, scope string) {
	if scope == model.ScopeSpread {
		s.Pat, s.PatL, s.PatR = nil, nil, nil
		return
	}
	s.SplitPat(scope)
	*s.PatSlot(scope) = nil
}

// dropPhotos снимает фото фона, лежащие на области scope: у страницы — её и фото всего
// листа (как SetBackdrop на страницу), у всего листа — все.
func dropPhotos(s *model.Spread, scope string) {
	s.BgPhoto = nil
	if scope == model.ScopeSpread {
		s.BgPhotoL, s.BgPhotoR = nil, nil
		return
	}
	*s.Slot(scope) = nil
}

// setColor — цвет страниц области (как design.SetBackground: область пакетов не импортирует).
func setColor(s *model.Spread, scope, c string) {
	switch scope {
	case model.ScopeSpread:
		s.BgL, s.BgR = c, c
	case model.ScopeLeft:
		s.BgL = c
	case model.ScopeRight:
		s.BgR = c
	}
}
