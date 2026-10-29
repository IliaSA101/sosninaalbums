package app

// Автосборка по просьбе агента (149, Т3): те же умолчания, фото и число разворотов, что у окна
// «Автосборка». Умолчания (Авто, серии, «Обычное», семейства «Все», похожие 0, вариант 1) подставляет
// служба; здесь — то, что окно считает во фронте: свободные фото ленты (autoPick, freephotos.ts) и
// N по умолчанию (defaultSpreads, autobuild.ts). Паритет — testdata/parity/autopick.json.

import (
	"context"
	"math"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// agentAutoReq — вход автосборки агента: только числа, bool, строки и срезы строк.
type agentAutoReq struct {
	Key     string
	Rev     int
	Spreads int // 0 — по умолчанию окна: round(фото / среднее), в пределах плана
	// PerAuto — «Авто»; иначе PerMin…PerMax фото на разворот (1…64).
	PerAuto          bool
	PerMin, PerMax   int
	Series           bool
	Similar, Variant int
	Variety          int
	Families         []string
	Photos           []string // пусто — все свободные фото ленты; иначе они же, отфильтрованные тем же правилом
	ScanWaitMs       int      // сколько ждать конца чтения ленты, мс; 0 — не ждать
}

// agentAutoRes — итог автосборки.
type agentAutoRes struct {
	Rev      int    // ревизия после правки
	Added    int    // новых разворотов
	Photos   int    // фото в сборке (план)
	PlanFrom int    // действующее «от»
	PlanTo   int    // и «до»
	Similar  int    // убрано похожих
	First    int    // номер первого нового разворота с 1; 0 — нет
	Go       string // его ID для перехода; "" — без перехода
}

// maxPer — предел «до» на разворот (model.MaxPhotos).
const maxPer = model.MaxPhotos

// agentAuto собирает свободные фото в конец альбома одним шагом отмены. Ключ — по редактору, rev и
// срок — под замком его правки (AutoBuildIf); Album и Again всегда false (М10), позиция — конец.
func (a *App) agentAuto(ctx context.Context, in agentAutoReq) (res agentAutoRes, err error) {
	defer guardErr("agentAuto", &err)
	ed, err := a.agentEditor(in.Key)
	if err != nil {
		return res, err
	}
	if err := a.agentWaitScan(ctx, time.Duration(in.ScanWaitMs)*time.Millisecond); err != nil {
		return res, err
	}
	if err := a.agentCheckOn(); err != nil { // флажок могли снять, пока ждали ленту
		return res, err
	}
	before := ed.View()
	if before.Rev != in.Rev {
		return res, errAgentAlbumEdited
	}
	ids := a.agentFree(ed, before, in.Photos)
	if len(ids) == 0 {
		return res, errAgentNoFree
	}
	infos := a.infos(ids)
	o := model.AutoOpts{Auto: in.PerAuto, MinPer: in.PerMin, MaxPer: in.PerMax, Series: in.Series,
		Variety: model.Variety(in.Variety), Families: in.Families, Similar: in.Similar, Variant: in.Variant}
	if in.PerAuto {
		o.MinPer, o.MaxPer = 1, maxPer
	}
	o = o.Clamp()
	plan, err := a.agentPlan(ed, infos, &o, in.Spreads)
	if err != nil {
		return res, err
	}
	had := make(map[string]bool, len(before.Spreads))
	for _, sv := range before.Spreads {
		had[sv.ID] = true
	}
	v, err := ed.AutoBuildIf(infos, -1, o, album.Expect{Rev: in.Rev, Ctx: ctx})
	if err != nil {
		return res, agentErr(err)
	}
	album.LogMem("автосборка")
	res = agentAutoRes{Rev: v.Rev, Photos: plan.Photos, PlanFrom: plan.From, PlanTo: plan.To, Similar: plan.Similar}
	for i, sv := range v.Spreads {
		if had[sv.ID] {
			continue
		}
		if res.Added == 0 {
			res.First, res.Go = i+1, sv.ID
		}
		res.Added++
	}
	a.changed(v)
	return res, nil
}

// defaultSpreads — Go-двойник defaultSpreads (autobuild.ts): round(фото / среднее), среднее 4 в
// «Авто» или середина диапазона «Свой»; не меньше 1. Округление — как Math.round (половина вверх).
func defaultSpreads(photos int, rng [2]int) int {
	avg := 4.0
	if rng != [2]int{} {
		avg = float64(rng[0]+rng[1]) / 2
	}
	return max(1, int(math.Floor(float64(photos)/max(1, avg)+0.5)))
}

// agentWaitScan ждёт конца чтения ленты не дольше wait (и не дольше ctx); не дождались —
// errAgentScanning. Чтение, которого нет, не ждётся.
func (a *App) agentWaitScan(ctx context.Context, wait time.Duration) error {
	s := a.agentEd()
	end := time.Now().Add(wait)
	for {
		a.mu.Lock()
		busy := a.scanning || a.scan.Active
		a.mu.Unlock()
		if !busy {
			return nil
		}
		if ctx.Err() != nil || !time.Now().Before(end) {
			return errAgentScanning
		}
		select {
		case <-ctx.Done():
		case <-time.After(s.opts.Poll):
		}
	}
}

// agentFree — ID фото, которые берёт автосборка агента, по порядку ленты: свободные (не в альбоме,
// не на фоне), без отклонённых; фильтр и выделение ленты не учитываются — агент их не видит.
// only не пуст — только эти фото. Замок редактора берётся до a.mu (порядок замков).
func (a *App) agentFree(ed *album.Editor, v model.View, only []string) []string {
	marks := ed.Marks()
	a.mu.Lock()
	photos := slices.Clone(a.photos)
	a.mu.Unlock()
	used := []string{}
	add := func(sv model.SpreadView) { used = append(used, sv.Photos...) }
	for _, sv := range v.Spreads {
		add(sv)
	}
	if v.Cover != nil && !v.Settings.Sheet {
		add(v.Cover.Sheet)
	}
	ids, _ := freePick(photos, marks, used, v.BackdropUsed)
	if len(only) == 0 {
		return ids
	}
	want := make(map[string]bool, len(only))
	for _, id := range only {
		want[id] = true
	}
	return slices.DeleteFunc(ids, func(id string) bool { return !want[id] })
}

// freePick — Go-двойник autoPick без выделения (freephotos.ts): фото ленты, которых нет среди
// мест (placed — записи мест, копия «ab-2» занимает и файл «ab»), нет на фоне (backdrop) и не
// отклонены (действующий флаг −1: своя отметка главнее файла). skipped — сколько отклонённых
// пропущено.
func freePick(photos []library.Photo, own map[string]model.MarkView, placed, backdrop []string) (ids []string, skipped int) {
	used := make(map[string]bool, 2*len(placed))
	for _, id := range placed {
		used[id], used[model.BaseID(id)] = true, true
	}
	bd := make(map[string]bool, len(backdrop))
	for _, id := range backdrop {
		bd[id] = true
	}
	ids = []string{}
	for _, p := range photos {
		if used[p.ID] || bd[p.ID] {
			continue
		}
		flag := 0
		if p.Xmp != nil {
			flag = p.Xmp.Flag
		}
		if m, ok := own[p.ID]; ok && m.Flag != nil {
			flag = *m.Flag
		}
		if flag == -1 {
			skipped++
			continue
		}
		ids = append(ids, p.ID)
	}
	return ids, skipped
}

// agentPlan считает план и N: окно открывается с round(n / среднее), n — фото плана (без похожих),
// зажим — пределы плана. want > 0 — N назвал агент (зажим тот же). N записывается в o.
func (a *App) agentPlan(ed *album.Editor, infos []model.PhotoInfo, o *model.AutoOpts, want int) (model.AutoPlan, error) {
	var avg [2]int
	if !o.Auto {
		avg = [2]int{o.MinPer, o.MaxPer}
	}
	o.Spreads = want
	if o.Spreads <= 0 {
		o.Spreads = defaultSpreads(len(infos), avg) // первое приближение; точное — по фото плана
	}
	plan, err := ed.AutoPlan(infos, *o)
	if err != nil {
		return plan, agentErr(err)
	}
	n := want
	if n <= 0 {
		n = defaultSpreads(plan.Photos, avg)
	}
	if plan.MaxSpreads >= 1 {
		n = min(max(n, max(1, plan.MinSpreads)), plan.MaxSpreads)
	}
	if n != o.Spreads {
		o.Spreads = n
		plan, err = ed.AutoPlan(infos, *o)
	}
	return plan, agentErr(err)
}
