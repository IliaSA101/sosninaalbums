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
	v, look, _, err := e.createCoverLook(preset, ph, libs, want, nil)
	return v, look, err
}

// createCoverLook — тело CreateCoverLook и CreateCoverLookIf (x nil — без сверки). place — куда
// встало фото: CoverPlaceSeat | CoverPlaceBackground ("" — фото нет или отказ).
func (e *Editor) createCoverLook(preset string, ph *model.PhotoInfo, libs map[string]model.PhotoInfo, want *model.LookRef, x *Expect) (v model.View, look *model.LookRef, place string, err error) {
	v, err = e.editWith("", false, func(p *model.Project) (err error) {
		look, place = nil, ""
		// Точная запись, не SameLook: libs собраны по ней, и обновлённый набор — уже другие файлы.
		if (p.Look == nil) != (want == nil) || (want != nil && *p.Look != *want) {
			return ErrLookChanged
		}
		place, err = createCover(p, preset, ph, func() (err error) {
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
		return err
	}, nil, editOpts{expect: x})
	if err != nil {
		look, place = nil, ""
	}
	return v, look, place, err
}

// createCover — тело CreateCover и CreateCoverLook: обложка, затем (если задано) оформление
// макетом, затем фото броска: после макета — в первое пустое место лицевой (145 Ч2), иначе фоном
// лицевой. Копия фону не нужна (098-3): фон фото с места не снимает — запрос копии ставит само
// фото, а не запись «-new». place — куда встало фото.
func createCover(p *model.Project, preset string, ph *model.PhotoInfo, style func() error) (string, error) {
	err := cover.Create(p, preset)
	if err == nil && style != nil {
		err = style()
	}
	if err != nil || ph == nil {
		return "", err
	}
	return coverPhoto(p, ph, style != nil, false)
}

// Куда встало фото обложки (CoverPhotoIf, CreateCoverLookIf).
const (
	CoverPlaceSeat       = "seat"       // место лицевой
	CoverPlaceBackground = "background" // фон лицевой
)

// coverPhoto — фото ph на лицевую обложки: в первое пустое место (seats — места ищутся, у броска
// это только после макета), иначе фоном. Свое из библиотеки, уже стоящее в месте разворота, в
// место не идёт: PlacePhotos снял бы его с разворота (копией оно не ставится — ID записи и есть
// ключ библиотеки); встаёт фоном, как в 0.28. replace — только у агента (151, 0.34-151-16): пустых
// мест нет — кадр заменяется в наибольшем (копией, прежний кадр уходит в ленту); мест нет совсем —
// фоном лицевой, а в раскладке wrap — фото через корешок (иначе ScopeRight сняло бы его, и задняя
// с корешком опустели бы).
func coverPhoto(p *model.Project, ph *model.PhotoInfo, seats, replace bool) (string, error) {
	if seats && (ph.Lib == "" || !p.Placed(model.ReqBase(ph.ID))) {
		seat := frontSeat(p)
		if seat == nil && replace {
			seat = frontSeatAny(p)
		}
		if seat != nil {
			return CoverPlaceSeat, seatCopy(p, seat, *ph)
		}
	}
	in := *ph
	in.ID = model.ReqBase(in.ID)
	return CoverPlaceBackground, backdrop.SetBackdrop(p, model.CoverID, frontScope(p, replace), in)
}

// coverPhotoOnce — тело CoverPhotoIf: тот же файл уже на лицевой — холостая (place — где он стоит),
// иначе coverPhoto с заменой. Обложки нет — cover.ErrNone.
func coverPhotoOnce(p *model.Project, ph *model.PhotoInfo) (string, error) {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return "", cover.ErrNone
	}
	if place := onFront(p, ph); place != "" {
		return place, nil
	}
	return coverPhoto(p, ph, true, true)
}

// frontScope — область фона лицевой: у агента в wrap — весь лист (одно фото через корешок);
// у броска человека и в прочих раскладках — правая сторонка.
func frontScope(p *model.Project, agent bool) string {
	if agent && p.Cover != nil && p.Cover.Layout == model.CoverWrap {
		return model.ScopeSpread
	}
	return model.ScopeRight
}

// onFront — фото ph (по файлу, BaseID) уже на лицевой: в каком-нибудь месте или на фоне её
// области. place — где; нет — "".
func onFront(p *model.Project, ph *model.PhotoInfo) string {
	base := model.FileOf(ph.ID)
	s := p.Cover.Sheet
	for _, pl := range p.CoverSeats() {
		if pl.Photo != "" && model.BaseID(pl.Photo) == base && grid.InArea(p, s, pl.X, pl.W, model.ScopeRight) {
			return CoverPlaceSeat
		}
	}
	if b := *s.Slot(frontScope(p, true)); b != nil && model.BaseID(b.Photo) == base {
		return CoverPlaceBackground
	}
	return ""
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

// frontSeatAny — наибольшее место лицевой по площади (при равенстве — первое), занятое или нет:
// кадр обложки, когда пустых мест нет (151). Мест нет — nil.
func frontSeatAny(p *model.Project) *model.Place {
	var best *model.Place
	for _, pl := range p.CoverSeats() {
		if grid.InArea(p, p.Cover.Sheet, pl.X, pl.W, model.ScopeRight) && (best == nil || pl.W*pl.H > best.W*best.H) {
			best = pl
		}
	}
	return best
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
