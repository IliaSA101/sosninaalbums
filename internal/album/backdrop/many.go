package backdrop

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Фон на набор листов (спека 077, design §2): тот же щелчок по плитке «Фоны», что ApplyPattern
// и ApplyBackdrop, на области многих разворотов одной правкой (образец целей — grid.CopyDecor).

// errTargets — целей больше, чем бывает областей у разворотов (как у «Копировать оформление»).
var errTargets = errors.New("слишком много листов для фона")

// ApplyTo — тело Editor.ApplyBackgroundTo: фон src на области targets; итог — число изменённых
// листов (тост). Обложки и неизвестные ID пропускаются (только развороты); повтор цели —
// холостой, как второй щелчок; уже стоящий фон заменяется, как при одном щелчке. Область не left/right/spread, у листа
// 031 страница, неверный узор или ключ — ошибка всей правки (try её откатывает).
func ApplyTo(p *model.Project, targets []model.GridTarget, src model.BgSource) (int, error) {
	plan, err := applyTo(p, targets, src)
	return plan.Spreads, err
}

// PlanTo — тело Editor.PlanBackgroundTo над копией проекта c (её снимает обёртка под замком):
// сколько листов изменится и сколько фото фона уйдёт. Ошибка — пустой план и та же ошибка,
// что даст ApplyTo.
func PlanTo(c *model.Project, targets []model.GridTarget, src model.BgSource) (model.BgPlan, error) {
	return applyTo(c, targets, src)
}

// applyTo — общий ход ApplyTo и PlanTo: правка и сравнение фона каждого листа до и после.
// Лист с несколькими целями считается один раз; фото, поставленное и снятое той же правкой
// (левая, затем весь лист), в число ушедших не входит — сравниваются начало и конец.
func applyTo(p *model.Project, targets []model.GridTarget, src model.BgSource) (model.BgPlan, error) {
	var plan model.BgPlan
	if len(targets) > 3*model.MaxSpreads {
		return plan, errTargets
	}
	if (src.Pattern == nil) == (src.Photo.ID == "") {
		return plan, model.ErrPattern
	}
	byID := make(map[string]*model.Spread, len(p.Spreads))
	for _, s := range p.Spreads {
		byID[s.ID] = s
	}
	before := map[*model.Spread]bgState{}
	var order []*model.Spread
	for _, t := range targets {
		if !model.ValidScope(t.Scope) { // корешка у разворотов нет; проверка до Find — отказ не зависит от ID
			return plan, model.ErrScope
		}
		s := byID[t.Spread]
		if s == nil {
			continue
		}
		if _, ok := before[s]; !ok {
			before[s] = stateOf(s)
			order = append(order, s)
		}
		var err error
		if src.Pattern != nil {
			err = ApplyPattern(p, s.ID, t.Scope, *src.Pattern, src.Color)
		} else {
			err = ApplyBackdrop(p, s.ID, t.Scope, src.Photo)
		}
		if err != nil {
			return model.BgPlan{}, err
		}
	}
	for _, s := range order {
		was, now := before[s], stateOf(s)
		if !was.equal(now) {
			plan.Spreads++
		}
		for i, b := range was.ph {
			if b != nil && (now.ph[i] == nil || now.ph[i].Photo != b.Photo) {
				plan.Photos++
			}
		}
	}
	return plan, nil
}

// bgState — фон листа для сравнения: узоры, цвет страниц и фото фона (копии — правка их не
// тронет). Равенство — как у сравнения JSON в Editor: «изменилось» ⇔ правка не холостая.
type bgState struct {
	pat      [3]*model.Pattern  // весь лист, левая, правая
	ph       [4]*model.Backdrop // весь лист, левая, правая, корешок
	bgL, bgR string
}

func stateOf(s *model.Spread) bgState {
	return bgState{
		pat: [3]*model.Pattern{s.Pat.Clone(), s.PatL.Clone(), s.PatR.Clone()},
		ph:  [4]*model.Backdrop{s.BgPhoto.Clone(), s.BgPhotoL.Clone(), s.BgPhotoR.Clone(), s.BgPhotoS.Clone()},
		bgL: s.BgL, bgR: s.BgR,
	}
}

func (a bgState) equal(b bgState) bool {
	return a.bgL == b.bgL && a.bgR == b.bgR &&
		slices.EqualFunc(a.pat[:], b.pat[:], samePattern) && slices.EqualFunc(a.ph[:], b.ph[:], sameBackdrop)
}

// samePattern — узоры равны по значению; Inks nil и пустой — одно (в JSON оба не пишутся).
func samePattern(a, b *model.Pattern) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Kind == b.Kind && slices.Equal(a.Inks, b.Inks) && a.Scale == b.Scale && a.Seed == b.Seed &&
		a.Fade == b.Fade && a.Preset == b.Preset
}

// sameBackdrop — фото фона равны по значению, кадр — тоже по значению.
func sameBackdrop(a, b *model.Backdrop) bool {
	if a == nil || b == nil {
		return a == b
	}
	if (a.Frame == nil) != (b.Frame == nil) || a.Frame != nil && *a.Frame != *b.Frame {
		return false
	}
	ca, cb := *a, *b
	ca.Frame, cb.Frame = nil, nil
	return ca == cb
}
