package spreads

// Разнообразие раскладок (спека 041, design §7): выбор раскладки собранных разворотов с учётом
// соседей, «Разнообразить раскладки» и поля подборки (семейство, похожие, «Другие варианты»).
// Ключи строит и выбирает internal/layout/variety; здесь — соседи из альбома и запись ключа.

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/plan"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// Rating — действующая оценка фото по ID (своя отметка поверх XMP); nil — оценок нет.
type Rating func(id string) int

const (
	// sinceCap — дальше этого ритм воздуха назад не считаем: самый длинный период — 5.
	sinceCap = 8
)

// level — уровень ручки для variety (зажим 0…2: чужое число не включает несуществующий режим).
func level(v model.Variety) variety.Level { return variety.Level(v.Clamp()) }

// Prep — автосборка до замка Editor (design §7): снимок входа, план и пулы групп.
// Под замком план и пулы берутся отсюда, если вход тот же (иначе — заново).
type Prep struct {
	photos []model.PhotoInfo
	o      model.AutoOpts
	job    PlanJob
	pl     plan.Plan
	err    error
	pools  [][]variety.Cand // по группам плана; nil — считать под замком
}

// Prepare — план и пулы групп параллельно вне замка; job снят под замком (PlanOf).
// «Меньше» пулов не считает (выбора нет, как в 0.10), но «Другие варианты» греет: счётчик «i / N»
// в View берёт их под замком, а холодный ряд — десятки мс на разворот.
func Prepare(job PlanJob, photos []model.PhotoInfo, o model.AutoOpts) Prep {
	w := Prep{photos: photos, o: o, job: job}
	_, w.pl, w.err = job.plan(o)
	if w.err == nil {
		asps := make([][]float64, len(w.pl.Groups))
		for i, g := range w.pl.Groups {
			asps[i] = job.asp[g[0]:g[1]]
		}
		w.pools = pools(asps, job.prm, poolLv(level(o.Variety), allowOf(o.Families)))
	}
	return w
}

// Photos — сколько фото пришло в сборку (журнал медленных операций).
func (w Prep) Photos() int { return len(w.photos) }

// planFor — план для входа j и пулы его групп: посчитанные до замка, если вход тот же, иначе
// план заново, а пулы (nil) — при выборе.
func (w Prep) planFor(j PlanJob) (plan.Plan, [][]variety.Cand, error) {
	if j.same(w.job) {
		return w.pl, w.pools, w.err
	}
	_, pl, err := j.plan(w.o)
	return pl, nil, err
}

// same — тот же вход плана: пропорции, геометрия, разворотов и пустой хвост, а для серий
// (051) — папки и время съёмки тех же фото.
func (j PlanJob) same(k PlanJob) bool {
	return j.prm == k.prm && j.have == k.have && j.tail == k.tail && slices.Equal(j.asp, k.asp) &&
		slices.EqualFunc(j.infos, k.infos, func(a, b model.PhotoInfo) bool {
			return a.Path == b.Path && a.Lib == b.Lib && a.Taken == b.Taken
		})
}

// poolLevel — пул разворота из n фото: с ManyFrom — только ряд «Авто», как у ExtraRow: выбранный
// ключ всегда виден в листании и счётчике.
func poolLevel(n int, lv variety.Level) variety.Level {
	if n >= layout.ManyFrom {
		return variety.Less
	}
	return lv
}

// poolOf — пул выбора: ряд «Авто» и «Другие варианты» (model.ExtraRow) — ровно то, что видят
// подборка, листание и счётчик: выбранный ключ всегда есть в ряду «i / N».
func poolOf(asp []float64, prm layout.Params, lv variety.Level) []variety.Cand {
	extra := model.ExtraRow(asp, prm)
	return slices.DeleteFunc(variety.Pool(asp, prm, model.RowLen(len(asp)), poolLevel(len(asp), lv)), func(c variety.Cand) bool {
		return !c.InRow && model.IndexOf(extra, c.Key) < 0
	})
}

// pools считает пулы параллельно вне замка (nil — выключено тестом). На «Меньше» пулов нет —
// только прогрев «Других вариантов» (ExtraRow), результат nil. Паника — только в лог, пул остаётся
// nil: тот же счёт повторится под замком и поведёт себя как раньше.
func pools(asps [][]float64, prm layout.Params, lv variety.Level) [][]variety.Cand {
	if !model.ParallelLayouts() || len(asps) == 0 {
		return nil
	}
	out := make([][]variety.Cand, len(asps))
	var pl model.PanicLog
	model.Each(len(asps), func(i int) {
		pl.Safely("", func() {
			if lv > variety.Less {
				out[i] = poolOf(asps[i], prm, lv)
			} else {
				model.ExtraRow(asps[i], prm)
			}
		})
	})
	pl.Summary()
	if lv == variety.Less {
		return nil
	}
	return out
}

// shown — свойства действующей раскладки разворота; ok=false — не «Авто» с фото (сетка,
// виньетка, пустой): у такого соседа вида нет.
func shown(p *model.Project, s *model.Spread) (variety.Info, bool) {
	if !varies(s) {
		return variety.Info{}, false
	}
	r, _, ok := p.Layout(s)
	if !ok {
		return variety.Info{}, false
	}
	return variety.Describe(r, p.Params()), true
}

// varies — раскладку разворота выбирает «Авто»: фото есть, сетки и виньетки нет.
func varies(s *model.Spread) bool { return len(s.Photos) > 0 && len(s.Grid) == 0 && s.Kind == "" }

// ctxOf — соседи цепочки p.Spreads[first..last]: виды двух перед ней, семейство ближнего и есть ли
// у него главное фото, вид следующего и сколько разворотов перед ней без воздуха и без главного
// (не «Авто» ритм прерывает); формы до шести перед ней и следующего, среднее фото на разворот (051),
// направление ближайшей лесенки (055).
// dead — развороты, которые правка уже опустошила и Normalize удалит: соседями они не считаются.
func ctxOf(p *model.Project, first, last int, dead func(*model.Spread) bool) variety.Ctx {
	var c variety.Ctx
	alive := func(s *model.Spread) bool { return dead == nil || !dead(s) }
	var before []*model.Spread // живые перед цепочкой, ближний первым
	for i := first - 1; i >= 0; i-- {
		if s := p.Spreads[i]; alive(s) {
			before = append(before, s)
		}
	}
	for k := min(2, len(before)) - 1; k >= 0; k-- {
		in, _ := shown(p, before[k])
		c.Prev = append(c.Prev, in.Look)
		c.Family, c.Hero = in.Family, in.Heroes > 0
	}
	if len(before) > 0 && varies(before[0]) {
		if r, _, ok := p.Layout(before[0]); ok {
			c.PrevKey = r.Key
		}
	}
	for _, s := range p.Spreads[last+1:] {
		if alive(s) {
			in, _ := shown(p, s)
			c.Next, c.NextForm = in.Look, in.Form
			break
		}
	}
	c.Avg = avgPhotos(p, alive)
	// Ритмы и формы — одним проходом: вид соседа (Describe) считается раз, счёт каждого рвётся сам;
	// формы — не «Авто» (сетка, виньетка, пустой) их обрывает, как ритмы, окно — variety.FormsWindow.
	air, hero, form := true, true, true
	for i, s := range before {
		in, ok := shown(p, s)
		air = air && ok && in.Family != variety.Air && c.Since < sinceCap
		hero = hero && ok && in.Heroes == 0 && c.SinceHero < sinceCap
		form = form && ok && i < variety.FormsWindow
		if !air && !hero && !form {
			break
		}
		if form {
			c.Forms = append(c.Forms, in.Form)
		}
		if air {
			c.Since++
		}
		if hero {
			c.SinceHero++
		}
	}
	slices.Reverse(c.Forms) // последняя — ближняя
	c.Ladder = lastLadder(p, before)
	return c
}

// lastLadder — направление ближайшей лесенки среди before (ближний первым) до начала альбома,
// 0 — нет (055: «Разнообразить» части продолжает чередование). Лучшая (Layout "") лесенкой не
// бывает, так что Build — только у ключей с L(…)/A(…).
func lastLadder(p *model.Project, before []*model.Spread) byte {
	prm := p.Params()
	for _, s := range before {
		d := forms.LadderDir(s.Layout)
		if d == 0 || !varies(s) {
			continue
		}
		if r, ok := layout.Build(s.Layout, p.Aspects(s.Photos), prm); ok && forms.Of(r.Key, r.Rects, prm) == forms.Ladder {
			return d
		}
	}
	return 0
}

// avgPhotos — среднее фото на живой разворот «Авто» с фото (0 — таких нет): «редкая» и «плотная»
// группа выбора форм меряются от него.
func avgPhotos(p *model.Project, alive func(*model.Spread) bool) float64 {
	n, k := 0, 0
	for _, s := range p.Spreads {
		if varies(s) && alive(s) {
			n, k = n+len(s.Photos), k+1
		}
	}
	if k == 0 {
		return 0
	}
	return float64(n) / float64(k)
}

// starOf — звезда разворота: единственное фото с наибольшей оценкой ≥ 4 (variety.Star).
func starOf(photos []string, rate Rating) int {
	if rate == nil {
		return -1
	}
	r := make([]int, len(photos))
	for i, id := range photos {
		r[i] = rate(id)
	}
	return variety.Star(r)
}

// pickRun выбирает раскладки цепочки p.Spreads[first..last] (все varies) и пишет ключи.
// cur — текущий вид тоже повтор («Разнообразить»): разворот меняется, если есть другой.
// ready — пулы цепочки, посчитанные вне замка (nil или nil-пул — считать здесь); dead — см. ctxOf.
// allow — разрешённые семейства (065, nil — все): пул сужается до Pick, [0] выборки — «лучшая»
// для допусков. Возвращает по развороту цепочки: разрешённых нет, взята лучшая.
// Выбранный вид совпал с текущим — ключ не пишется: смена ключа без видимой перемены дала бы
// rev, шаг отмены и тост «Раскладки изменены» впустую.
func pickRun(p *model.Project, first, last int, lv variety.Level, rate Rating, cur bool, ready [][]variety.Cand,
	dead func(*model.Spread) bool, allow map[variety.Family]bool) []bool {
	c, prm := ctxOf(p, first, last, dead), p.Params()
	in := make([]variety.Spread, last-first+1)
	fb := make([]bool, len(in))
	for k := range in {
		s := p.Spreads[first+k]
		pool := []variety.Cand(nil)
		if k < len(ready) {
			pool = ready[k]
		}
		if pool == nil {
			pool = poolOf(p.Aspects(s.Photos), prm, poolLv(lv, allow))
		}
		pool, fb[k] = variety.ByFamily(pool, allow)
		in[k] = variety.Spread{Pool: pool, Star: starOf(s.Photos, rate)}
		if cur {
			info, _ := shown(p, s)
			c.Cur = append(c.Cur, info.Look)
		}
	}
	for k, x := range variety.Pick(in, prm, lv, c) {
		if x >= 0 && (!cur || in[k].Pool[x].Look != c.Cur[k]) {
			p.SetLayout(p.Spreads[first+k], in[k].Pool[x].Key)
		}
	}
	return fb
}

// varyBuilt — раскладки n собранных разворотов с позиции at по уровню ручки; «Меньше» без
// семейств — ничего (лучшая, Layout ""), ровно как 0.10; с семействами — первая разрешённая
// (SetLayout пишет "" за лучшую). Возвращает флаги запасных (nil — фильтра нет).
func varyBuilt(p *model.Project, at, n int, o model.AutoOpts, rate Rating, ready [][]variety.Cand, dead func(*model.Spread) bool) []bool {
	lv, allow := level(o.Variety), allowOf(o.Families)
	if (lv > variety.Less || allow != nil) && n > 0 {
		return pickRun(p, at, at+n-1, lv, rate, false, ready, dead, allow)
	}
	return nil
}

// runs — цепочки подряд идущих выделенных разворотов «Авто» с фото, по порядку альбома.
func runs(p *model.Project, ids []string) [][2]int {
	sel := make(map[string]bool, min(len(ids), model.MaxSpreads))
	for _, id := range ids[:min(len(ids), model.MaxSpreads)] {
		sel[id] = true
	}
	var out [][2]int
	for i := 0; i < len(p.Spreads); i++ {
		if s := p.Spreads[i]; !sel[s.ID] || !varies(s) {
			continue
		}
		j := i
		for j+1 < len(p.Spreads) && sel[p.Spreads[j+1].ID] && varies(p.Spreads[j+1]) {
			j++
		}
		out = append(out, [2]int{i, j})
		i = j
	}
	return out
}

// VaryWarm — снимок под замком для прогрева пулов «Разнообразить» вне его (как Prepare).
func VaryWarm(p *model.Project, ids []string, v model.Variety) func() {
	lv, prm := max(variety.Normal, level(v)), p.Params()
	var asps [][]float64
	for _, r := range runs(p, ids) {
		for _, s := range p.Spreads[r[0] : r[1]+1] {
			asps = append(asps, p.Aspects(s.Photos))
		}
	}
	return func() { pools(asps, prm, lv) } // только прогрев кэша: позиции под замком могут сдвинуться
}

// VaryLayouts — «Разнообразить раскладки» (041): выделенные развороты «Авто» с фото по
// порядку альбома, каждая цепочка подряд идущих — с соседями по краям и своим текущим видом.
// Сетки, виньетки и обложка не трогаются; «Меньше» считается «Обычным». families — тот же
// фильтр семейств, что у автосборки (065; пусто — все). Нечего менять — проект тот же
// (холостая правка у Editor).
func VaryLayouts(p *model.Project, ids []string, v model.Variety, families []string, rate Rating) {
	lv, allow := max(variety.Normal, level(v)), allowOf(families)
	for _, r := range runs(p, ids) {
		pickRun(p, r[0], r[1], lv, rate, true, nil, nil, allow)
	}
}

// describeRows — семейство формы, её название и похожие (Twin) у плиток ряда: подборка фильтрует,
// подписывает и сворачивает.
func describeRows(rows []model.Choice, prm layout.Params) {
	looks := make([]string, len(rows))
	for i := range rows {
		f := forms.Of(rows[i].Key, rows[i].Rects, prm)
		rows[i].Family, rows[i].FormName = string(variety.FamilyOf(f)), forms.Name(f)
		looks[i] = variety.Look(rows[i].Rects, prm)
	}
	for i, t := range variety.Twins(looks) {
		rows[i].Twin = t
	}
}

// extraChoices — «Другие варианты» (model.ExtraRow): тот же порядок, что у листания и счётчика.
func extraChoices(asp []float64, prm layout.Params) []model.Choice {
	rs := model.ExtraRow(asp, prm)
	out := make([]model.Choice, len(rs))
	for i, r := range rs {
		out[i] = model.Choice{Key: r.Key, Rects: r.Rects}
	}
	describeRows(out, prm)
	return out
}
