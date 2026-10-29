package album

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Обложка по макету альбома (131 design §5, G5): серия и «Создать обложку» ставят оформление
// обложки макета тем же шагом, «Поставить на обложку» — шаг 0 ApplyLookStep. Тело — grid/covertpl.go.

// lookCover — оформление обложки макета t для режима series: обложки нет или у макета её нет —
// ничего (обложка не создаётся). Фон, который поставил прежний макет prev, «своим» не считается
// (Replaced); свои места, надписи и стикеры сторонок, которые заменятся, а обложка не от prev, —
// тронутое (Touched, решение 136). Итог — в plan (Cover), ключи без файла — в missing.
func lookCover(p *model.Project, plan *model.LookPlan, missing map[string]bool, t model.AlbumTpl, prev *model.AlbumTpl, libs map[string]model.PhotoInfo) error {
	if t.Cover == nil || p.Cover == nil || p.Cover.Sheet == nil {
		return nil
	}
	var ownBg, ownAll bool // до правки: после неё сверять нечего
	if prev != nil && prev.Cover != nil {
		ownBg = prev.Cover.HasBg() && grid.LookCoverBgSame(p, prev.Cover, libs) // нет фона у прежнего — любой фон свой
		ownAll = lookCoverIs(p, prev.Cover, libs) && !grid.CoverExtras(p, prev.Cover)
	}
	res, err := grid.ApplyLookCover(p, t.Cover, libs)
	if err != nil {
		return err
	}
	if res.Replaced && !ownBg {
		plan.Replaced++
		plan.CoverReplaced = true
	}
	if res.Own > 0 && !ownAll {
		plan.Touched++
		plan.CoverTouched = true
	}
	plan.Cover = res.Changed
	// Фото обложки в серии нет по определению режима (с фото в местах альбом собранный): Left = 0.
	for _, k := range res.Missing {
		missing[k] = true
	}
	return nil
}

// lookCoverIs — обложка проекта p уже такая, как её ставит c (сверка на копии: проект не меняется).
func lookCoverIs(p *model.Project, c *model.CoverTpl, libs map[string]model.PhotoInfo) bool {
	res, err := grid.ApplyLookCover(p.Clone(), c, libs)
	return err == nil && !res.Changed
}

// applyLookCoverStep — «Поставить на обложку» (ApplyLookStep, шаг 0): оформление обложки макета t.
// Обложки нет — холостая; у макета нет оформления обложки — ErrLookStep. Макет ref становится
// макетом альбома, если у альбома макета ещё нет (как у шага на лист). В плане — фото в ленту,
// заменённые надписи и стикеры сторонок.
func applyLookCoverStep(p *model.Project, ref *model.LookRef, t model.AlbumTpl, libs map[string]model.PhotoInfo) (plan model.GridPlan, missing int, err error) {
	if t.Cover == nil {
		return plan, 0, ErrLookStep
	}
	if p.Cover == nil || p.Cover.Sheet == nil {
		return plan, 0, nil
	}
	res, err := grid.ApplyLookCover(p, t.Cover, libs)
	if err != nil {
		return model.GridPlan{}, 0, err
	}
	if ref != nil && (p.Look == nil || model.SameLook(p.Look, ref) && p.Look.Key != ref.Key) {
		p.Look = ref // нет макета или набор обновился (тот же макет с новым ключом)
	}
	plan.Left, plan.Texts, plan.Stickers = res.Left, res.Texts, res.Stickers
	return plan, len(res.Missing), nil
}

// CreateCoverLook — CreateCover, который оформляет новую обложку макетом альбома (Project.Look):
// макет ищет e.findLook (LookHost) по фактической записи проекта под замком (0.28-131-81), затем
// встаёт фото броска ph (явное действие сильнее макета). Итог look — запись макета (с именем для
// тоста), если обложка изменилась от макета, иначе nil. want — запись макета, по которой App
// собрал libs: смена макета между чтением App и правкой — ErrLookChanged (как NewLookSpread).
// Макета нет на этом ПК или у него нет обложки — обычная CreateCover. Кнопка «Без оформления» —
// Undo и CreateCover. Одна правка, отказ — без шага.
func (e *Editor) CreateCoverLook(preset string, ph *model.PhotoInfo, libs map[string]model.PhotoInfo, want *model.LookRef) (model.View, *model.LookRef, error) {
	var look *model.LookRef
	v, err := e.try("", func(p *model.Project) (err error) {
		look = nil
		// Точная запись, не SameLook: libs собраны по ней, и обновлённый набор — уже другие файлы.
		if (p.Look == nil) != (want == nil) || (want != nil && *p.Look != *want) {
			return ErrLookChanged
		}
		return createCover(p, preset, ph, func() (err error) {
			if l := e.findLook(p.Look); l != nil && l.Cover != nil {
				var res grid.CoverRes
				res, err = grid.ApplyLookCover(p, l.Cover, libs)
				if res.Changed {
					c := *p.Look
					look = &c
				}
			}
			return err
		})
	})
	if err != nil {
		look = nil
	}
	return v, look, err
}

// createCover — тело CreateCover и CreateCoverLook: обложка, затем (если задано) оформление
// макетом, затем фото броска: после макета — в первое пустое место лицевой (145 Ч2), иначе фоном
// лицевой. Копия фону не нужна (098-3): фон фото с места не снимает — запрос копии ставит само
// фото, а не запись «-new».
func createCover(p *model.Project, preset string, ph *model.PhotoInfo, style func() error) error {
	err := cover.Create(p, preset)
	if err == nil && style != nil {
		err = style()
	}
	if err != nil || ph == nil {
		return err
	}
	// Своё из библиотеки, уже стоящее в месте разворота, в место не идёт: PlacePhotos снял бы его
	// с разворота (копией оно не ставится — ID записи и есть ключ библиотеки); встаёт фоном, как в 0.28.
	if style != nil && (ph.Lib == "" || !p.Placed(model.ReqBase(ph.ID))) {
		if seat := frontSeat(p); seat != nil {
			return seatCopy(p, seat, *ph)
		}
	}
	in := *ph
	in.ID = model.ReqBase(in.ID)
	return backdrop.SetBackdrop(p, model.CoverID, model.ScopeRight, in)
}

// frontSeat — первое пустое место лицевой по model.CoverSeats (задняя и корешок не годятся:
// бросок на плитку — «этот кадр на лицевую»); мест нет — nil, фото встанет фоном.
func frontSeat(p *model.Project) *model.Place {
	s := p.Cover.Sheet
	for _, pl := range p.CoverSeats() {
		if pl.Photo == "" && grid.InArea(p, s, pl.X, pl.W, model.ScopeRight) {
			return pl
		}
	}
	return nil
}

// seatCopy — фото ph в место pl обложки. Фото, уже стоящее в альбоме, встаёт копией (098-2,
// запрос «-new» разбирает ResolveCopies внутри PlacePhotos), без вопроса: кадр остаётся в
// развороте. Фото вне альбома и своё из библиотеки, которого нет в местах (createCover отсёк
// остальные), — как есть.
func seatCopy(p *model.Project, pl *model.Place, ph model.PhotoInfo) error {
	ph.ID = model.ReqBase(ph.ID)
	if ph.Lib != "" && (!model.ValidLibKey(ph.Lib) || ph.ID != model.LibPhotoID(ph.Lib)) {
		return model.ErrLibKey // как у фона: иначе Rekey при открытии разведёт запись и ключ
	}
	if ph.Lib == "" && p.Placed(ph.ID) {
		ph.ID = model.CopyReq(model.FileOf(ph.ID))
	}
	_, err := grid.PlacePhotos(p, model.CoverID, pl.ID, []model.PhotoInfo{ph})
	return err
}
