package album

import (
	"fmt"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
)

// Собранный альбом (131 design §4, G4b): режимы add («Добавить развороты» — повторяемые шаги
// перед хвостом серии, места пустые) и bg («Фоны по серии» — только фон шага). Серией собранный
// альбом не ставится никогда: фото из мест ушли бы в ленту. Остальные режимы — ErrLookMode.

// applyLookFull — точка входа собранного альбома; prev — прежний макет (nil — его нет на этом ПК).
func applyLookFull(p *model.Project, plan *model.LookPlan, ref *model.LookRef, t model.AlbumTpl, prev *model.AlbumTpl, o model.LookOpts, libs map[string]model.PhotoInfo) error {
	switch o.Mode {
	case model.LookAdd:
		return lookAdd(p, plan, ref, t, prev, o, libs)
	case model.LookBg:
		return lookBg(p, plan, ref, t, prev, libs)
	}
	return ErrLookMode
}

// lookAdd — режим add: o.Spreads (по умолчанию 1) повторяемых шагов перед хвостом серии.
// Макет альбома другой (не SameLook) — альбом остаётся на своём: новые развороты встают перед
// хвостом серии макета альбома prev (нет его на ПК — в конец), шаг их не запоминается (0), как у
// «Поставить на разворот» чужим макетом; номера прежних разворотов целы.
func lookAdd(p *model.Project, plan *model.LookPlan, ref *model.LookRef, t model.AlbumTpl, prev *model.AlbumTpl, o model.LookOpts, libs map[string]model.PhotoInfo) error {
	plan.Mode = model.LookAdd
	n := orDefault(o.Spreads, 1)
	if p.Look != nil && !model.SameLook(p.Look, ref) {
		at := len(p.Spreads)
		if prev != nil {
			at = lookTailAt(p, *prev)
		}
		return lookAddAt(p, plan, t, n, at, -1, false, libs)
	}
	if err := lookAddSteps(p, plan, t, n, libs); err != nil {
		return err
	}
	p.Look = ref
	return nil
}

// lookAddSteps — n новых разворотов макета t: на месте хвоста серии (lookTailAt), шаги по кругу
// повторяемых от шага развороту перед ними. Итог — в plan.
func lookAddSteps(p *model.Project, plan *model.LookPlan, t model.AlbumTpl, n int, libs map[string]model.PhotoInfo) error {
	at, after := lookTailAt(p, t), -1
	_, rep, _ := model.LookBlocks(t)
	for i := at - 1; i >= 0 && after < 0; i-- { // последний повторяемый до места вставки: свои развороты между не в счёт
		if st := p.Spreads[i].LookStep - 1; st >= 0 && slices.Contains(rep, st) {
			after = st
		}
	}
	return lookAddAt(p, plan, t, n, at, after, true, libs)
}

// lookAddAt — n разворотов макета t с позиции at, шаги по кругу повторяемых после after; mark —
// запомнить номер шага (макет альбома).
func lookAddAt(p *model.Project, plan *model.LookPlan, t model.AlbumTpl, n, at, after int, mark bool, libs map[string]model.PhotoInfo) error {
	if len(p.Spreads)+n > model.MaxSpreads {
		return fmt.Errorf("в альбоме не может быть больше %d разворотов", model.MaxSpreads)
	}
	missing := map[string]bool{}
	for i, st := range model.AddSteps(t, after, n) {
		if err := spreads.NewSpread(p, at+i, nil); err != nil {
			return err
		}
		sp := p.Spreads[at+i]
		num := 0
		if mark {
			num = st + 1
		}
		gp, gone, err := lookPut(p, sp, t.Steps[st], num, libs)
		if err != nil {
			return err
		}
		plan.Left += gp.Left
		lookTally(plan, missing, gone, sp)
	}
	plan.Spreads, plan.Missing = n, len(missing)
	return nil
}

// lookTailAt — куда встают новые развороты: перед последними подряд разворотами, чей номер шага
// из блока post макета t (конец серии), иначе — в конец альбома. Свои развороты после конца
// серии остаются после новых.
func lookTailAt(p *model.Project, t model.AlbumTpl) int {
	_, _, post := model.LookBlocks(t)
	tail := func(s *model.Spread) bool { return s.LookStep > 0 && slices.Contains(post, s.LookStep-1) }
	last := -1
	for i := len(p.Spreads) - 1; i >= 0 && last < 0; i-- {
		if tail(p.Spreads[i]) {
			last = i
		}
	}
	if last < 0 {
		return len(p.Spreads) // хвоста нет
	}
	for last > 0 && tail(p.Spreads[last-1]) {
		last--
	}
	return last
}

// lookBg — режим bg («Фоны по серии»): разворотам-не-виньеткам по порядку — шаги SeriesFit,
// виньеткам — первый шаг vignette (нет такого — не трогаются), обложке — Bg макета. Только фон
// (grid.ApplyLookBg), LookStep = шаг + 1. Фон, который поставил прежний макет prev, «своим» не
// считается; Anchors — голые развороты, ставшие якорями 107.
func lookBg(p *model.Project, plan *model.LookPlan, ref *model.LookRef, t model.AlbumTpl, prev *model.AlbumTpl, libs map[string]model.PhotoInfo) error {
	plan.Mode = model.LookBg
	var plain int
	for _, s := range p.Spreads {
		if !s.IsVignette() {
			plain++
		}
	}
	idx, vig := model.SeriesFit(t, plain), model.VigStep(t)
	bare := bareSet(p)
	same := model.SameLook(p.Look, ref)
	missing := map[string]bool{}
	k := 0
	for _, s := range p.Spreads {
		step := vig
		if !s.IsVignette() {
			step = idx[k]
			k++
		}
		old := s.LookStep
		if !same {
			s.LookStep = 0
		}
		if step < 0 {
			continue
		}
		owned := lookOwnedBg(p, s, old, prev, libs) // до правки: после неё сверять нечего
		res := grid.ApplyLookBg(p, s, t.Steps[step].Tpl, libs)
		if res.Replaced && !owned {
			plan.Replaced++
		}
		for _, key := range res.Missing {
			missing[key] = true
		}
		s.LookStep = step + 1
		plan.Spreads++
	}
	if t.Cover != nil && t.Cover.HasBg() {
		own := prev != nil && prev.Cover != nil && prev.Cover.HasBg() && grid.LookCoverBgSame(p, prev.Cover, libs) // до правки
		res := grid.ApplyLookCoverBg(p, t.Cover, libs)
		// Обложки ещё нет — её оформление поставит createCover (G5), здесь пропуск. Свой фон обложки
		// (цвет, узор, фото из съёмки, фото корешка) считается в Replaced наравне с разворотами:
		// вопрос «заменится свой фон» один на всё; фон прежнего макета «своим» не считается.
		if res.Replaced && !own {
			plan.Replaced++
			plan.CoverReplaced = true
		}
		plan.Cover = res.Changed
		for _, key := range res.Missing {
			missing[key] = true
		}
	}
	for id := range bare { // фон — единственное, что режим меняет: голый разворот с фоном — якорь
		if _, s := p.Find(id); grid.HasBg(s) {
			plan.Anchors++
		}
	}
	plan.Missing = len(missing)
	p.Look = ref
	return nil
}

// bareSet — ID голых разворотов 107 (без своего оформления: пересборка их заберёт).
func bareSet(p *model.Project) map[string]bool {
	out := map[string]bool{}
	for _, id := range spreads.BareIDs(p) {
		out[id] = true
	}
	return out
}

// lookOwnedBg — фон листа s поставил прежний макет prev (шаг old — номер до правки): он не «свой».
func lookOwnedBg(p *model.Project, s *model.Spread, old int, prev *model.AlbumTpl, libs map[string]model.PhotoInfo) bool {
	return prev != nil && old > 0 && old <= len(prev.Steps) && grid.LookBgSame(p, s, prev.Steps[old-1].Tpl, libs)
}
