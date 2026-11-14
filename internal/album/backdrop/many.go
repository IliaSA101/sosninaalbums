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

// PlanTo — ApplyTo с полным планом: сколько листов изменилось и сколько фото фона ушло. Тело
// Editor.PlanBackgroundTo зовёт его над копией проекта c (её снимает обёртка под замком);
// ApplyToAgent с replace — над самим проектом. Ошибка — пустой план и та же ошибка, что у ApplyTo.
func PlanTo(c *model.Project, targets []model.GridTarget, src model.BgSource) (model.BgPlan, error) {
	return applyTo(c, targets, src)
}

// applyTo — общий ход ApplyTo и PlanTo: правка и сравнение фона каждого листа до и после.
// Лист с несколькими целями считается один раз; фото, поставленное и снятое той же правкой
// (левая, затем весь лист), в число ушедших не входит — сравниваются начало и конец.
func applyTo(p *model.Project, targets []model.GridTarget, src model.BgSource) (model.BgPlan, error) {
	plan, _, _, err := applyKeep(p, targets, src, false)
	return plan, err
}

// Outcome — итог ApplyToAgent: Changed — листов изменено; Skipped — ID листов, где фон области —
// фото клиента (без replace не тронуты); Replaced — заменено фото клиента на фоне (только при replace;
// фото фона из каталога и наборов не в счёт: человек соглашается на фото клиента).
// Cleared — ID листов, с которых SetAlbumBgAgent снял свой фон (по порядку листов); у
// ApplyToAgent пусто.
type Outcome struct {
	Changed  int
	Skipped  []string
	Replaced int
	Cleared  []string
}

// ApplyToAgent — тело Editor.ApplyBackgroundToIf (151, 0.34-151-19). С replace — как ApplyTo с
// планом (PlanTo): заменённые фото клиента на фоне — в Replaced. Без replace цель, которая сняла бы фото
// клиента (фото фона без ключа библиотеки; фоны наборов и свои заменяются), не применяется — её
// лист возвращается в состояние до цели, ID листа идёт в Skipped (по разу, в порядке целей);
// Replaced остаётся 0. Прогон один и на самом проекте: откат цели — восстановление её листа,
// копия проекта не нужна; ошибка откатывает всю правку (try), итог при ней пуст.
func ApplyToAgent(p *model.Project, targets []model.GridTarget, src model.BgSource, replace bool) (Outcome, error) {
	if replace {
		plan, _, client, err := applyKeep(p, targets, src, false)
		return Outcome{Changed: plan.Spreads, Replaced: client}, err
	}
	plan, skipped, _, err := applyKeep(p, targets, src, true)
	return Outcome{Changed: plan.Spreads, Skipped: skipped}, err
}

// applyKeep — тело applyTo и ApplyToAgent; keep — цели, снимающие фото клиента, пропускаются.
// Четвёртый результат — сколько фото клиента (без ключа библиотеки) ушло с листов.
func applyKeep(p *model.Project, targets []model.GridTarget, src model.BgSource, keep bool) (model.BgPlan, []string, int, error) {
	var plan model.BgPlan
	client := 0
	if len(targets) > 3*model.MaxSpreads {
		return plan, nil, 0, errTargets
	}
	if (src.Pattern == nil) == (src.Photo.ID == "") {
		return plan, nil, 0, model.ErrPattern
	}
	byID := make(map[string]*model.Spread, len(p.Spreads))
	for _, s := range p.Spreads {
		byID[s.ID] = s
	}
	before := map[*model.Spread]bgState{}
	var order []*model.Spread
	var skipped []string
	for _, t := range targets {
		if !model.ValidScope(t.Scope) { // корешка у разворотов нет; проверка до Find — отказ не зависит от ID
			return plan, nil, 0, model.ErrScope
		}
		s := byID[t.Spread]
		if s == nil {
			continue
		}
		was := stateOf(s)
		if err := applyOne(p, s, t.Scope, src); err != nil {
			return model.BgPlan{}, nil, 0, err
		}
		if keep && was.dropsClient(p, stateOf(s)) {
			was.restore(s)
			if !slices.Contains(skipped, s.ID) {
				skipped = append(skipped, s.ID)
			}
			continue
		}
		if _, ok := before[s]; !ok {
			before[s] = was
			order = append(order, s)
		}
	}
	for _, s := range order {
		was, now := before[s], stateOf(s)
		if !was.equal(now) {
			plan.Spreads++
		}
		all, cl := was.goneFrom(p, now)
		plan.Photos += all
		client += cl
	}
	return plan, skipped, client, nil
}

// applyOne — одна цель: узор src.Pattern с цветом или фото src.Photo на области scope листа s.
func applyOne(p *model.Project, s *model.Spread, scope string, src model.BgSource) error {
	if src.Pattern != nil {
		return ApplyPattern(p, s.ID, scope, *src.Pattern, src.Color)
	}
	return ApplyBackdrop(p, s.ID, scope, src.Photo)
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

// restore возвращает листу s фон из состояния a (откат цели, ApplyToAgent). Копии a уже свои.
func (a bgState) restore(s *model.Spread) {
	s.Pat, s.PatL, s.PatR = a.pat[0], a.pat[1], a.pat[2]
	s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.BgPhotoS = a.ph[0], a.ph[1], a.ph[2], a.ph[3]
	s.BgL, s.BgR = a.bgL, a.bgR
}

// goneFrom — сколько фото фона ушло при переходе a → b и сколько из них клиентских (без ключа библиотеки).
func (a bgState) goneFrom(p *model.Project, b bgState) (all, client int) {
	for i, was := range a.ph {
		if was != nil && (b.ph[i] == nil || b.ph[i].Photo != was.Photo) {
			all++
			if p.Photos[was.Photo].Lib == "" {
				client++
			}
		}
	}
	return all, client
}

// dropsClient — переход a → b снял или заменил фото клиента: фото фона без ключа библиотеки
// (запись Photos без Lib; записи нет — тоже клиентское, осторожнее).
func (a bgState) dropsClient(p *model.Project, b bgState) bool {
	for i, was := range a.ph {
		if was != nil && p.Photos[was.Photo].Lib == "" && (b.ph[i] == nil || b.ph[i].Photo != was.Photo) {
			return true
		}
	}
	return false
}
