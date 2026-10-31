package album

// Правка «если всё ещё то же» (149, design §2.1): агент правит по виду, который читал, а правка
// идёт после плана вне замка и, бывает, после «время вышло». Сверка rev и срока — под e.mu в
// editWith, в одном месте с записью: между сверкой и правкой человек не успевает нажать Ctrl+Z
// или поправить альбом. Сама сверка проекта не трогает и шага отмены не ставит.

import (
	"context"
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
)

var (
	// ErrRevChanged — альбом правили после вида, по которому идёт правка (или он закрыт).
	ErrRevChanged = errors.New("альбом изменился после чтения — перечитайте")
	// ErrLate — срок вызова вышел: правка не применена.
	ErrLate = errors.New("срок вышел — правка не применена")
)

// Expect — условие правки под замком: rev вида (View.Rev) и срок вызова. Ctx nil — срока нет.
type Expect struct {
	Rev int
	Ctx context.Context
}

// check — сверка под e.mu: сначала срок (отказ «поздно» точнее «изменился»), затем rev. Закрытый
// Editor (альбом сменился) — как изменившийся: shut молча отдал бы вид, а агент принял бы это за успех.
func (x *Expect) check(e *Editor) error {
	if x == nil {
		return nil
	}
	if x.Ctx != nil && x.Ctx.Err() != nil {
		return ErrLate
	}
	if e.closed || e.rev != x.Rev {
		return ErrRevChanged
	}
	return nil
}

// AutoBuildIf — AutoBuild, который под замком Editor сверяет x: не совпал rev или вышел срок —
// ошибка, проект и отмена прежние. План и пулы считаются вне замка, как у AutoBuild.
func (e *Editor) AutoBuildIf(photos []model.PhotoInfo, at int, o model.AutoOpts, x Expect) (model.View, error) {
	return e.autoBuild(photos, at, o, &x)
}

// VaryLayoutsIf — VaryLayouts со сверкой x под замком; нечего менять — холостая (rev тот же,
// ошибки нет). Пулы греются вне замка.
func (e *Editor) VaryLayoutsIf(ids []string, v model.Variety, families []string, infos []model.PhotoInfo, x Expect) (model.View, error) {
	e.outside(func(p *model.Project) func() { return spreads.VaryWarm(p, ids, v) })
	return e.editWith("", false, func(p *model.Project) error {
		spreads.VaryLayouts(p, ids, v, families, e.rating(infos))
		return nil
	}, nil, editOpts{expect: &x})
}

// UndoIfRev — Undo, если rev всё ещё x.Rev (альбом после правки агента не менялся) и срок не
// вышел; иначе ошибка и ничего не отменено. Один замок на сверку и отмену.
func (e *Editor) UndoIfRev(x Expect) (model.View, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err := x.check(e); err != nil {
		return e.view(), err
	}
	return e.undoLocked(), nil
}
