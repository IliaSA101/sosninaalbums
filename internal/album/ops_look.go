package album

import (
	"errors"
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
)

// Макет альбома (131 design §4): ядро задаёт порядок — пустые, список шагов, сопоставление
// разворотов позициям, вставка недостающих, шаг за шагом постановка; тела — grid/look.go,
// spreads/look.go, vigsheet. Режим определяет Go: есть фото из съёмки — собранный (G4b: add, bg),
// иначе серия. Постановка и сухой прогон — одна функция applyLook.

// ErrLookMode — у собранного альбома не выбран или ещё не поддержан режим постановки: серией
// такой альбом не ставится (фото в местах).
var ErrLookMode = errors.New("в альбоме есть фото: выберите «Добавить развороты» или «Фоны по серии»")

var errLookRef = errors.New("у макета нет ключа")

// LookFinder — находит макет по записи альбома (Project.Look): реестр библиотеки, у App. nil или
// nil в ответе — макета нет на этом ПК, все его развороты считаются тронутыми. Зовётся под замком
// Editor по фактической записи проекта: макет не отстаёт от правки (0.28-131-81).
type LookFinder func(model.LookRef) *model.AlbumTpl

// ApplyLook ставит макет t на альбом одной правкой (один шаг отмены, отказ — откат без шага,
// повтор того же — холостая правка). ref — запись макета (ключ my:, имя, набор), find — поиск
// макета, которым оформлен альбом сейчас, libs — свои из библиотеки для обоих макетов. Договор: ref и t зажимаются
// здесь; пустой макет — ErrTplEmpty; предел MaxSpreads и отказ конвейера — ошибка без правки;
// режим собранного альбома — LookOpts.Mode (G4b), иначе игнорируется.
func (e *Editor) ApplyLook(ref model.LookRef, t model.AlbumTpl, find LookFinder, o model.LookOpts, libs map[string]model.PhotoInfo) (v model.View, plan model.LookPlan, err error) {
	r, c, err := checkLook(ref, t)
	if err != nil {
		return e.View(), plan, err
	}
	v, err = e.try("", func(p *model.Project) (err error) {
		plan, err = applyLook(p, r, c, find, o, libs)
		return err
	})
	if err != nil {
		plan = model.LookPlan{}
	}
	return v, plan, err
}

// PlanLook — сухой прогон ApplyLook той же функцией на копии (вопрос до правки): под замком
// только снимок, rev, отмена и события не трогаются. У собранного альбома без выбора режима
// ответ — Full = true без чисел (ошибки нет): окно показывает «Что сделать».
func (e *Editor) PlanLook(ref model.LookRef, t model.AlbumTpl, find LookFinder, o model.LookOpts, libs map[string]model.PhotoInfo) (model.LookPlan, error) {
	r, c, err := checkLook(ref, t)
	if err != nil {
		return model.LookPlan{}, err
	}
	var plan model.LookPlan
	err = e.dryRun(func(cp *model.Project) (err error) {
		plan, err = applyLook(cp, r, c, find, o, libs)
		return err
	})
	if errors.Is(err, ErrLookMode) && plan.Full {
		return plan, nil
	}
	if err != nil {
		return model.LookPlan{}, err
	}
	return plan, nil
}

// dryRun — fn на копии проекта и хвост конвейера правки (settle): итог сухого прогона равен
// итогу правки. Под замком только снимок; rev, отмена и события не трогаются.
func (e *Editor) dryRun(fn func(cp *model.Project) error) error {
	var data []byte
	e.read(func(p *model.Project) { data = model.Marshal(p) })
	before, cp := model.Unmarshal(data), model.Unmarshal(data)
	err := fn(cp)
	if err == nil {
		err = settle(cp, before, false)
	}
	return err
}

// checkLook — зажим входа: ссылка (ключ по ValidLibKey) и макет.
func checkLook(ref model.LookRef, t model.AlbumTpl) (*model.LookRef, model.AlbumTpl, error) {
	r := model.ClampLookRef(&ref)
	if r == nil {
		return nil, model.AlbumTpl{}, errLookRef
	}
	c, ok := model.ClampLook(t)
	if !ok {
		return nil, model.AlbumTpl{}, model.ErrTplEmpty
	}
	return r, c, nil
}

// prevLook — макет, которым оформлен проект p сейчас: тот же, что ref (ключ или запись набора — набор
// мог обновиться), — новый макет t, иначе поиск find по записи проекта (негодный ответ — nil).
func prevLook(p *model.Project, ref *model.LookRef, t model.AlbumTpl, find LookFinder) *model.AlbumTpl {
	switch {
	case p.Look == nil:
		return nil
	case model.SameLook(p.Look, ref):
		return &t
	case find == nil:
		return nil
	}
	f := find(*p.Look)
	if f == nil {
		return nil
	}
	if c, ok := model.ClampLook(*f); ok {
		return &c
	}
	return nil
}

// applyLook — тело ApplyLook и PlanLook над проектом p. Режим: фото из съёмки в альбоме —
// собранный (applyLookFull), иначе серия.
func applyLook(p *model.Project, ref *model.LookRef, t model.AlbumTpl, find LookFinder, o model.LookOpts, libs map[string]model.PhotoInfo) (model.LookPlan, error) {
	plan := model.LookPlan{Full: lookBuilt(p)}
	if p.Look != nil {
		plan.Prev = p.Look.Name
	}
	prev := prevLook(p, ref, t, find)
	if plan.Full {
		return plan, applyLookFull(p, &plan, ref, t, prev, o, libs)
	}
	return plan, applyLookSeries(p, &plan, ref, t, prev, o, libs)
}

// lookBuilt — собранный альбом: есть фото из съёмки — в местах и на «Авто» любого листа
// (обложки тоже) или на фоне без ключа библиотеки. Надписи, шапка виньетки и пустые места не
// считаются.
func lookBuilt(p *model.Project) bool {
	for _, s := range p.Sheets() {
		if len(s.Photos) > 0 {
			return true
		}
		for _, bd := range []*model.Backdrop{s.BgPhoto, s.BgPhotoL, s.BgPhotoR, s.BgPhotoS} {
			if bd != nil && p.Photos[bd.Photo].Lib == "" {
				return true
			}
		}
	}
	return false
}

// lookSlot — развороту-цели серии назначен шаг; sp == nil — разворот создаётся.
type lookSlot struct {
	step int
	sp   *model.Spread
	old  *model.Spread // разворот другого вида на этой позиции: новый встаёт перед ним, старый уходит
}

// applyLookSeries — режим series: пустые развороты уходят, макетные сопоставляются шагам новой
// серии по позициям, лишние удаляются, недостающие создаются, затем каждый получает свой шаг.
func applyLookSeries(p *model.Project, plan *model.LookPlan, ref *model.LookRef, t model.AlbumTpl, prev *model.AlbumTpl, o model.LookOpts, libs map[string]model.PhotoInfo) error {
	plan.Mode = model.LookSeries
	// Обложка есть — её оформление макета (нет — не создаётся, design §5). Первой, до смены числа
	// разворотов: конвейер правки (cover.Reflow) сдвинет содержимое обложки по новому корешку, и
	// поставленное на новой геометрии уехало бы второй раз.
	missing := map[string]bool{}
	if err := lookCover(p, plan, missing, t, prev, libs); err != nil {
		return err
	}
	spreads.DropEmpty(p)
	idx := model.Series(t, orDefault(o.Spreads, len(t.Steps)))
	var macro []*model.Spread
	for _, s := range p.Spreads {
		if s.LookStep > 0 {
			macro = append(macro, s)
		}
	}
	if total := len(p.Spreads) - len(macro) + len(idx); total > model.MaxSpreads {
		return fmt.Errorf("в альбоме не может быть больше %d разворотов", model.MaxSpreads)
	}
	if plan.Trial = len(macro) > 0; plan.Trial {
		plan.Touched += lookTouched(p, macro, prev, libs) // плюс обложка (lookCover)
	}
	fin, drop := lookSlots(macro, t, idx)
	spreads.DeleteSpreads(p, drop)
	err := lookInsert(p, fin, t)
	for j := 0; j < len(fin) && err == nil; j++ { // первая ошибка останавливает постановку (откат — в try)
		f := fin[j]
		if f.old != nil {
			spreads.DeleteSpread(p, f.old.ID)
		}
		var gp model.GridPlan
		var gone []string
		if gp, gone, err = lookPut(p, f.sp, t.Steps[f.step], f.step+1, libs); err == nil {
			plan.Left += gp.Left
		}
		lookTally(plan, missing, gone, f.sp)
	}
	if err != nil {
		return err
	}
	plan.Spreads, plan.Missing = len(fin), len(missing)
	p.Look = ref
	return nil
}

// lookTally — разворот sp в итог: места, заготовки, ключи без файла на этом ПК (gone).
func lookTally(plan *model.LookPlan, missing map[string]bool, gone []string, sp *model.Spread) {
	for _, k := range gone {
		missing[k] = true
	}
	plan.Places += len(sp.Grid)
	for _, tx := range sp.Texts {
		if tx.Draft {
			plan.Drafts++
		}
	}
}

// orDefault — v, а при v ≤ 0 — умолчание def.
func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// lookPut — шаг st на разворот sp: у виньетки сначала её настройка, затем оформление шага.
// left — фото, ушедшие в ленту; gone — ключи библиотеки без файла (по тому, что реально встало).
func lookPut(p *model.Project, sp *model.Spread, st model.LookStep, n int, libs map[string]model.PhotoInfo) (plan model.GridPlan, gone []string, err error) {
	left := 0
	if st.Role == model.RoleVignette {
		left, err = lookVignette(p, sp, st)
	}
	if err == nil {
		plan, gone, err = grid.PutLookStep(p, sp.ID, st, n, libs)
	}
	plan.Left += left
	return plan, gone, err
}

// lookTouched — сколько макетных разворотов тронуто: повтор прежнего шага на них не холостой,
// есть имена, чужие надписи, настроенная виньетка. Прежнего макета нет или в нём нет шага —
// все тронуты. Сверка идёт на копии: проект p не меняется.
func lookTouched(p *model.Project, macro []*model.Spread, prev *model.AlbumTpl, libs map[string]model.PhotoInfo) int {
	var c *model.Project
	if prev != nil {
		c = p.Clone()
	}
	n := 0
	for _, s := range macro {
		if prev == nil || s.LookStep > len(prev.Steps) || !lookUntouched(c, s.ID, prev.Steps[s.LookStep-1], s.LookStep, libs) {
			n++
		}
	}
	return n
}

// lookUntouched — разворот копии c нетронут относительно шага st: вид (виньетка или обычный)
// и параметры виньетки те же, места виньетки по плану, остальное — grid.LookUntouched.
func lookUntouched(c *model.Project, id string, st model.LookStep, n int, libs map[string]model.PhotoInfo) bool {
	_, s := c.Find(id)
	if s.IsVignette() != (st.Role == model.RoleVignette) {
		return false
	}
	if s.IsVignette() {
		want := lookVig(st)
		got := *s.Vig
		if got.Tpl != want.Tpl || got.Teachers != want.Teachers || got.Pupils != want.Pupils || got.Ratio != want.Ratio ||
			!vigsheet.Untouched(c, s, vigPlan) {
			return false
		}
	}
	return grid.LookUntouched(c, id, st, n, libs)
}

// lookVig — параметры виньетки шага-виньетки (st.Vig не nil), зажатые как у «Применить» (пропорция по умолчанию).
func lookVig(st model.LookStep) model.Vignette {
	v := st.Vig // у шага-виньетки после ClampLook он есть
	return model.ClampVignette(model.Vignette{Tpl: v.Tpl, Teachers: v.Teachers, Pupils: v.Pupils, Ratio: v.Ratio})
}

// lookSlots — позиции новой серии idx для макетных разворотов macro: начало серии — с головы,
// конец — с хвоста, середина (повторяемые) — по порядку; лишние середины (с её конца) идут в drop.
// Разворот другого вида (виньетка вместо обычного и наоборот) заменяется: sp = nil, old — прежний.
// Позиции без разворота — sp = nil.
func lookSlots(macro []*model.Spread, t model.AlbumTpl, idx []int) (fin []lookSlot, drop []string) {
	a, _, c := model.LookBlockSizes(t)
	m, k := len(idx), len(macro)
	head := min(a, k)
	tail := min(c, k-head)
	midOld := k - head - tail
	keep := min(midOld, max(0, m-a-c))
	fin = make([]lookSlot, m)
	for j := range fin {
		fin[j].step = idx[j]
	}
	for j := range head {
		fin[j].sp = macro[j]
	}
	for j := range keep {
		fin[a+j].sp = macro[head+j]
	}
	for j := range tail {
		fin[m-tail+j].sp = macro[k-tail+j]
	}
	for _, s := range macro[head+keep : head+midOld] {
		drop = append(drop, s.ID)
	}
	for j, f := range fin {
		if f.sp != nil && f.sp.IsVignette() != (t.Steps[f.step].Role == model.RoleVignette) {
			fin[j].sp, fin[j].old = nil, f.sp
		}
	}
	return fin, drop
}

// lookInsert — создаёт недостающие развороты: перед ближайшим следующим по серии (перед хвостом
// post), нет такого — следом за предыдущим, серии ещё нет — в конец альбома. Виньетка — через
// AddVignette, обычный — NewSpread.
func lookInsert(p *model.Project, fin []lookSlot, t model.AlbumTpl) (err error) {
	for j := 0; j < len(fin) && err == nil; j++ {
		if fin[j].sp != nil {
			continue
		}
		at := lookAt(p, fin, j)
		if t.Steps[fin[j].step].Role == model.RoleVignette {
			err = vigsheet.AddVignette(p, at, vigPlan)
		} else {
			err = spreads.NewSpread(p, at, nil)
		}
		if err == nil {
			fin[j].sp = p.Spreads[at]
		}
	}
	return err
}

// lookAt — куда вставить разворот позиции j: на месте заменяемого, иначе перед ближайшим следующим
// по серии (перед хвостом post), нет такого — следом за предыдущим, серии ещё нет — в конец.
func lookAt(p *model.Project, fin []lookSlot, j int) int {
	if fin[j].old != nil {
		i, _ := p.Find(fin[j].old.ID)
		return i
	}
	for _, f := range fin[j+1:] {
		if f.sp != nil {
			i, _ := p.Find(f.sp.ID)
			return i
		}
	}
	if j > 0 {
		i, _ := p.Find(fin[j-1].sp.ID)
		return i + 1
	}
	return len(p.Spreads)
}

// lookVignette — настройка листа 031 по шагу vignette до оформления: параметры SetVignette;
// форма, подпись учителя и шапка листа остаются своими. Те же параметры и места по плану — ничего,
// сдвинутые места — «Вернуть авто». left — фото, ушедшие в ленту.
func lookVignette(p *model.Project, s *model.Spread, st model.LookStep) (int, error) {
	v := lookVig(st)
	if s.IsVignette() {
		cur := *s.Vig
		v.TText, v.Shape, v.Round, v.Head = cur.TText, cur.Shape, cur.Round, cur.Head
		if v.Same(cur) {
			if vigsheet.Untouched(p, s, vigPlan) {
				return 0, nil
			}
			return vigsheet.ResetVignette(p, s.ID, vigPlan)
		}
	}
	return vigsheet.SetVignette(p, s.ID, v, vigPlan, nil, tplText)
}
