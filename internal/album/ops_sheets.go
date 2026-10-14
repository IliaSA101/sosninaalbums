package album

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/album/vignette"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Обёртки Editor над листами особого вида: обложка (032, пакет cover) и виньетка (031 G5).
// Один файл на обе спеки: в корне album 20 файлов (decisions 63). Места, тексты, фоны и слои
// обложки — общие методы с spreadID = model.CoverID.

// CreateCover — новая обложка: preset "" — пресет по умолчанию, «Фото на лицевой». ph не nil —
// фото ставится фоном лицевой (бросок на плитку «Обложка») тем же шагом отмены. Обложка уже
// есть, проект-лист 031, неизвестный пресет — ошибка без правки.
func (e *Editor) CreateCover(preset string, ph *model.PhotoInfo) (model.View, error) {
	return e.try("", func(p *model.Project) error { return createCover(p, preset, ph) })
}

// createCover — тело CreateCover: обложка и (если есть) фото фона лицевой.
func createCover(p *model.Project, preset string, ph *model.PhotoInfo) error {
	if err := cover.Create(p, preset); err != nil || ph == nil {
		return err
	}
	return backdrop.SetBackdrop(p, model.CoverID, model.ScopeRight, *ph)
}

// DeleteCover снимает обложку одним шагом отмены; нет обложки — холостая правка.
func (e *Editor) DeleteCover() model.View { return e.change(cover.Delete) }

// ApplyCoverPreset — числа пресета типографии; сторонка пользователя остаётся.
func (e *Editor) ApplyCoverPreset(id string) (model.View, error) {
	return e.try("", func(p *model.Project) error { return cover.ApplyPreset(p, id) })
}

// SetCoverSpec — свои числа обложки (зажим); не совпали с пресетом — «Свои размеры».
func (e *Editor) SetCoverSpec(s model.CoverSpec) (model.View, error) {
	return e.try("", func(p *model.Project) error { return cover.SetSpec(p, s) })
}

// SetCoverLayout — раскладка обложки front | both | wrap: фото фона переходят в новые зоны.
func (e *Editor) SetCoverLayout(kind string) (model.View, error) {
	return e.try("", func(p *model.Project) error { return cover.SetLayout(p, kind) })
}

// CoverSpineText — надпись на корешке с плашкой; уже есть — холостая правка.
func (e *Editor) CoverSpineText() (model.View, error) {
	return e.try("", cover.SpineText)
}

// SetCoverSpineBg — цвет плашки корешка #rrggbb; "" — без плашки.
// Договор: неверный цвет — холостая правка.
func (e *Editor) SetCoverSpineBg(color string) model.View {
	c, ok := model.ClampColor(color)
	if !ok && color != "" {
		return e.View()
	}
	return e.change(func(p *model.Project) { cover.SetSpineBg(p, c) })
}

// SetCoverGuide — файл-разметка типографии (абсолютный путь JPEG/PNG; проверка путей — как
// у фото при Load: без UNC и устройств). Подложка только для экрана.
func (e *Editor) SetCoverGuide(path string) (model.View, error) {
	if err := salbum.CheckGuide(path); err != nil {
		return e.View(), err
	}
	return e.change(func(p *model.Project) {
		e.guide = guideState{} // тот же путь выбрали заново — файл могли заменить
		cover.SetGuide(p, path)
	}), nil
}

// ClearCoverGuide убирает разметку обложки.
func (e *Editor) ClearCoverGuide() model.View {
	return e.change(func(p *model.Project) { cover.SetGuide(p, "") })
}

// guideState — что Editor знает о файле-разметке: размер, когда читали, идёт ли перечитывание.
type guideState struct {
	view.Guide
	at   time.Time
	busy bool
}

// guideRetry — не чаще этого файл, которого не было (диск не подключён, том не отвечал),
// перечитывается: иначе разметка не появилась бы до смены пути.
const guideRetry = 10 * time.Second

// guideFor — размер файла-разметки path: читается один раз на путь (смена пути — выбор,
// Undo, Load), а не под каждый вид. Неудача перечитывается в фоне не чаще guideRetry: проба
// мёртвого тома не держит e.mu, найденный размер покажет следующий вид. Договор: e.mu взят.
func (e *Editor) guideFor(path string) view.Guide {
	switch {
	case path == "":
	case e.guide.Path != path:
		w, h, ok := salbum.GuideSize(path)
		e.guide = guideState{Guide: view.Guide{Path: path, W: w, H: h, OK: ok}, at: time.Now()}
	case !e.guide.OK && !e.guide.busy && time.Since(e.guide.at) >= guideRetry:
		e.guide.busy, e.guide.at = true, time.Now()
		go e.reprobeGuide(path)
	}
	return e.guide.Guide
}

// reprobeGuide — фоновое перечитывание разметки path без замка; результат кладётся, только
// если путь тот же и файла ещё не было (выбор нового пути за это время не затирается).
func (e *Editor) reprobeGuide(path string) {
	w, h, ok := salbum.GuideSize(path)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.guide.Path != path || e.guide.OK {
		return
	}
	e.guide.busy, e.guide.at = false, time.Now()
	if ok {
		e.guide.Guide = view.Guide{Path: path, W: w, H: h, OK: true}
	}
}

// CoverGuidePath — абсолютный путь файла-разметки обложки ("" — нет); для раздачи /media/cover-guide.
func (e *Editor) CoverGuidePath() (path string) {
	e.read(func(p *model.Project) {
		if p.Cover != nil {
			path = p.Cover.Guide
		}
	})
	return path
}

// Виньетка (031 G5): тела правок — grid/vignette.go, план мест — пакет vignette (vigPlan).

// SetVignette — «Сделать виньеткой» и «Применить»; left — фото, ушедшие в ленту.
// Договор: v зажимается здесь (ClampVignette); мест больше MaxPlaces — ошибка try без правки.
// Свой шаблон ("my:<id>") берётся из tpls (виньеточные шаблоны templates.json) или с самого
// листа; поле v.Own от интерфейса отбрасывается, нет шаблона — «Шаблон не найден» без правки.
func (e *Editor) SetVignette(spreadID string, v model.Vignette, tpls []model.Template) (model.View, int, error) {
	v.Own = nil
	v = model.ClampVignette(v)
	return e.gridTry(func(p *model.Project) (int, error) {
		own, err := grid.ResolveOwn(p, spreadID, v.Tpl, tpls)
		if err != nil {
			return 0, err
		}
		return grid.SetVignette(p, spreadID, v, vigPlan, own)
	})
}

// SetupVignette — «Применить» окна «Настроить виньетку»: параметры и поля шапки одним шагом
// отмены; left — фото, ушедшие в ленту. Договор: зажим и разбор "my:" внутри
// grid.SetupVignette; мест больше MaxPlaces — ошибка try без правки.
func (e *Editor) SetupVignette(spreadID string, s model.VigSetup, tpls []model.Template) (model.View, int, error) {
	return e.gridTry(func(p *model.Project) (int, error) { return grid.SetupVignette(p, spreadID, s, vigPlan, tpls) })
}

// ResetVignette — «Вернуть авто» на виньетке (038, решение 14): места по плану текущего
// шаблона одним шагом отмены, фото и имена остаются; left — фото, ушедшие в ленту. Не виньетка
// и нетронутая — холостая правка.
func (e *Editor) ResetVignette(spreadID string) (model.View, int, error) {
	return e.gridTry(func(p *model.Project) (int, error) { return grid.ResetVignette(p, spreadID, vigPlan) })
}

// VigTemplate — шаблон виньетки листа для «Сохранить как шаблон» (ID и имя — на вызывающем).
// Не виньетка и лист без мест людей — ошибка. Только чтение.
func (e *Editor) VigTemplate(spreadID string) (t model.Template, err error) {
	e.read(func(p *model.Project) {
		var boxes []model.Box
		var geo model.VigGeo
		var st model.VigStyle
		if boxes, geo, st, err = grid.VigTemplateOf(p, spreadID); err == nil {
			t = model.Template{Scope: model.TplSpread, Boxes: boxes, Kind: model.TplVignette, Vig: &model.VigTpl{VigGeo: geo, Style: st}}
		}
	})
	return t, err
}

// PreviewVignette — сухой прогон SetupVignette для предпросмотра окна: то же тело и тот же
// хвост конвейера (settle) над копией, поэтому лист равен итогу «Применить». Договор: под
// замком только Marshal — счёт не держит View и автосохранение; rev, dirty, отмена и
// события не трогаются. Ошибки — те же, что у SetupVignette.
func (e *Editor) PreviewVignette(spreadID string, s model.VigSetup, tpls []model.Template) (model.VigPreview, error) {
	var data []byte
	e.read(func(p *model.Project) { data = model.Marshal(p) })
	before, c := model.Unmarshal(data), model.Unmarshal(data)
	left, err := grid.SetupVignette(c, spreadID, s, vigPlan, tpls)
	if err == nil {
		err = settle(c, before, false)
	}
	var sv model.SpreadView
	if err == nil { // лист — из копии после settle; не нашёлся — ошибка, а не пустой вид
		sv, err = view.SpreadOf(c, spreadID)
	}
	if err != nil {
		return model.VigPreview{}, err
	}
	markVig(c, &sv)
	return model.VigPreview{Spread: sv, Left: left}, nil
}

// AddVignette — новый разворот-виньетка по умолчанию на позиции at (-1 — в конец).
func (e *Editor) AddVignette(at int) (model.View, error) {
	return e.try("", func(p *model.Project) error { return grid.AddVignette(p, at, vigPlan) })
}

// ClearVignette — «Обычный разворот»: места, фото и подписи остаются. Не виньетка — холостая.
func (e *Editor) ClearVignette(spreadID string) model.View {
	return e.change(func(p *model.Project) { grid.ClearVignette(p, spreadID) })
}

// SortVignette — «Сортировать по фамилии» (ученики). Не виньетка — холостая.
func (e *Editor) SortVignette(spreadID string) model.View {
	return e.change(func(p *model.Project) { grid.SortVignette(p, spreadID) })
}

// VignetteChoices — превью шаблонов каталога (на листе 031 — без Split), затем своих из tpls
// (виньеточные шаблоны templates.json с числом страниц листа, 038) для чисел и пропорции
// панели (у своих — портрет шаблона), без правки. Свой шаблон, стоящий на листе, но удалённый из файла, идёт последним с
// Gone. Стиль подписей — листа, нет — тот, что включит «Применить». Неизвестный разворот — nil.
func (e *Editor) VignetteChoices(spreadID string, teachers, pupils int, ratio string, tpls []model.Template) []model.VigChoice {
	var out []model.VigChoice
	e.read(func(p *model.Project) { out = vigChoices(p, spreadID, teachers, pupils, ratio, tpls) })
	return out
}

// vigChoices — тело VignetteChoices под замком.
func vigChoices(p *model.Project, spreadID string, teachers, pupils int, ratio string, tpls []model.Template) []model.VigChoice {
	_, s := p.Find(spreadID)
	if s == nil {
		return nil
	}
	sc := *s // копия заголовка листа: Caps подменяется только для превью
	if sc.Caps == nil {
		c := model.VigCaps(p.Geo(s).H)
		sc.Caps = &c
	}
	tt := model.TeacherCaption
	if s.IsVignette() && s.Vig.TText != "" {
		tt = s.Vig.TText
	}
	g := p.Geo(s)
	cat := vignette.Catalog(p.Settings.Sheet)
	out := make([]model.VigChoice, 0, len(cat)+len(tpls)+1)
	// choice — превью шаблона v на листе sp (стиль подписей — его).
	choice := func(sp *model.Spread, v model.Vignette, name string) model.VigChoice {
		pl := vigPlan(p, sp, v)
		c := model.VigChoice{ID: v.Tpl, Name: name, Boxes: make([]model.Box, len(pl.Slots)), Roles: make([]string, len(pl.Slots)),
			PupilW: geom.Round2(pl.PupilW), Small: pl.PupilW > 0 && pl.PupilW < vigSmallW, Over: model.VigPlaces(v) > model.MaxPlaces}
		for k, sl := range pl.Slots {
			b := sl.Box
			c.Boxes[k] = model.Box{X: b.X / g.W, Y: b.Y / g.H, W: b.W / g.W, H: b.H / g.H}
			c.Roles[k] = sl.Role
		}
		return c
	}
	for _, t := range cat {
		v := model.ClampVignette(model.Vignette{Tpl: t.ID, Teachers: teachers, Pupils: pupils, Ratio: ratio, TText: tt})
		out = append(out, choice(&sc, v, t.Name))
	}
	inFile := map[string]bool{}
	for _, t := range tpls {
		if t.Kind != model.TplVignette || t.Vig == nil || t.Vig.Pages != len(g.Pages) {
			continue
		}
		id := model.VigOwnPrefix + t.ID
		inFile[id] = true
		sp, ttext := &sc, tt
		if st := t.Vig.Style; !s.IsVignette() || s.Vig.Tpl != id { // выбор принесёт оформление
			if st.Caps != nil {
				c2 := sc
				cc := *st.Caps
				c2.Caps, sp = &cc, &c2
			}
			ttext = cmp.Or(st.TText, tt)
		}
		// Шаблоны файла уже зажаты (ClampTemplates): повторный зажим геометрии на 64 местах не нужен.
		// Портрет — шаблона: выбор своего шаблона ставит его (точный режим BuildOwn требует того же).
		v := model.ClampVignette(model.Vignette{Tpl: id, Teachers: teachers, Pupils: pupils, Ratio: t.Vig.Ratio, TText: ttext})
		v.Own = &model.VigShape{Name: t.Name, Boxes: t.Boxes, VigGeo: t.Vig.VigGeo}
		c := choice(sp, v, t.Name)
		c.Own, c.Ratio = true, t.Vig.Ratio
		out = append(out, c)
	}
	if s.IsVignette() && s.Vig.Own != nil && !inFile[s.Vig.Tpl] { // шаблон удалён или с другого компьютера
		v := model.ClampVignette(model.Vignette{Tpl: s.Vig.Tpl, Teachers: teachers, Pupils: pupils, Ratio: s.Vig.Own.Ratio, TText: tt})
		v.Own = s.Vig.Own
		c := choice(&sc, v, s.Vig.Own.Name)
		c.Own, c.Ratio, c.Gone = true, s.Vig.Own.Ratio, true
		out = append(out, c)
	}
	return out
}

// markVigs — поля вида виньеток, которые знает только Editor (план мест): имя шаблона и
// признак «Ручная» (места отличаются от плана, 038 решение 14). Вид печати их не читает.
func markVigs(p *model.Project, v *model.View) {
	for i := range v.Spreads {
		markVig(p, &v.Spreads[i])
	}
}

func markVig(p *model.Project, sv *model.SpreadView) {
	if sv.Vig == nil {
		return
	}
	_, s := p.Find(sv.ID)
	if s == nil || !s.IsVignette() {
		return
	}
	sv.Vig.TplName = cmp.Or(sv.Vig.OwnName, vignette.Find(s.Vig.Tpl).Name)
	sv.Vig.Manual = !grid.Untouched(p, s, vigPlan)
}

// vigSmallW — портрет мельче, мм, превью помечает (031 design §1: на 200×200 при 64 — 13 мм).
const vigSmallW = 15

// vigPlan — план виньетки листа s (grid.VigPlanner): области — страницы Geo без max(поля,
// безопасная зона) со всех сторон, у корешка тоже; полоса подписи — отступ и строки шаблона
// в стиле листа (реальные метрики шрифта), подпись на фото — без полосы.
func vigPlan(p *model.Project, s *model.Spread, v model.Vignette) grid.VigPlan {
	g := p.Geo(s)
	m := max(g.Margin, p.Settings.Safe)
	rs := make([]vignette.Rect, len(g.Pages))
	for i, r := range g.Pages {
		rs[i] = vignette.Rect{X: r.X + m, Y: r.Y + m, W: max(r.W-2*m, 0), H: max(r.H-2*m, 0)}
	}
	in := vignette.Input{Regions: rs, Gap: g.Gap, Teachers: v.Teachers, Pupils: v.Pupils, Ratio: vignette.ParseRatio(v.Ratio)}
	if c := s.Caps; c != nil && !c.Over {
		in.Pad = c.Pad
		in.HS = c.Pad + capBand(c.Text, c.Style)
		in.HST = c.Pad + capBand(cmp.Or(v.TText, c.Text), c.Style)
	}
	var pl vignette.Plan
	if v.Own != nil { // свой шаблон — доли области листа (038)
		pl = vignette.BuildOwn(shapeOf(v.Own), vignette.Rect{W: g.W, H: g.H}, in)
	} else {
		pl = vignette.Build(vignette.Find(v.Tpl), in)
	}
	out := grid.VigPlan{Slots: make([]grid.VigSlot, len(pl.Slots)), PupilW: pl.PupilW}
	for i, sl := range pl.Slots {
		b := sl.Box
		out.Slots[i] = grid.VigSlot{Box: layout.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, Role: sl.Role}
	}
	if h := pl.Head; h != nil {
		out.Head = &layout.Rect{X: h.X, Y: h.Y, W: h.W, H: h.H}
	}
	return out
}

// shapeOf — геометрия своего шаблона для vignette.BuildOwn (доли, роли, шапка, пропорции).
func shapeOf(o *model.VigShape) vignette.Shape {
	sh := vignette.Shape{Boxes: make([]vignette.Rect, len(o.Boxes)), Roles: o.Roles, Aspect: o.Aspect, Ratio: vignette.ParseRatio(o.Ratio),
		W: o.W, H: o.H}
	for i, b := range o.Boxes {
		sh.Boxes[i] = vignette.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}
	}
	if h := o.Head; h != nil {
		sh.Head = &vignette.Rect{X: h.X, Y: h.Y, W: h.W, H: h.H}
	}
	return sh
}

// capBand — высота текста подписи по шаблону tpl: столько строк «Ёу», сколько в нём абзацев
// (не меньше одной), в стиле st. Метрики кэширует text.Cached.
func capBand(tpl string, st model.TextStyle) float64 {
	n := strings.Count(tpl, "\n") + 1
	return text.Cached(strings.TrimSuffix(strings.Repeat("Ёу\n", n), "\n"), st, 1e5).H
}

// NewVignetteSheet — проект-лист w×h мм (model.NewSheet) с одной виньеткой по умолчанию:
// «Виньетка (лист)» на стартовом окне. Собирается до Editor: шаг отмены создания не нужен.
func NewVignetteSheet(w, h float64) *model.Project {
	p := model.NewSheet(w, h)
	if _, err := grid.SetVignette(p, p.Spreads[0].ID, model.DefaultVignette(), vigPlan, nil); err != nil {
		panic(fmt.Sprintf("виньетка по умолчанию не встала: %v", err)) // 25 мест < MaxPlaces: ошибка кода
	}
	p.FitTexts()
	p.SyncOrder()
	return p
}
