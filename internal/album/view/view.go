package view

import (
	"errors"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// lowDPILimit — ниже этого эффективного dpi фото печатается мыльно.
const lowDPILimit = 200

// defaultBg — цвет страницы, если не задан ни у страницы, ни у альбома.
const defaultBg = "#ffffff"

// safeZone: при layflat сгиб не ограничен — один контур по внешнему краю листа g;
// без layflat шов режется, поэтому у каждой страницы свой контур с отступом m и от шва.
func safeZone(g model.Geo, m float64) []layout.Rect {
	inset := func(r layout.Rect) layout.Rect {
		return layout.Rect{X: r.X + m, Y: r.Y + m, W: max(r.W-2*m, 0), H: max(r.H-2*m, 0)}
	}
	if g.Layflat {
		return []layout.Rect{inset(layout.Rect{W: g.W, H: g.H})}
	}
	out := make([]layout.Rect, len(g.Pages))
	for i, r := range g.Pages {
		out[i] = inset(r)
	}
	return out
}

// State — поля Editor, которые попадают в View (проект их не хранит).
type State struct {
	Path             string
	Dirty            bool
	CanUndo, CanRedo bool
	Rev, MarksRev    int
	// Print — вид печати (Editor.Snapshot, экспорт): пустые поля — пусто, заглушек нет (030).
	Print bool
	// Guide — размер файла-разметки обложки (Editor читает его один раз на путь).
	Guide Guide
}

// BuildView — вид проекта p с состоянием редактора st (вызов под замком Editor).
func BuildView(p *model.Project, st State) model.View {
	v := model.View{
		Product:  p.Product,
		Settings: p.Settings,
		Folders:  append([]string{}, p.Folders...),
		Path:     st.Path,
		Dirty:    st.Dirty,
		CanUndo:  st.CanUndo,
		CanRedo:  st.CanRedo,
		Rev:      st.Rev,
		Spreads:  make([]model.SpreadView, len(p.Spreads)),

		MaxSpreadPhotos: model.MaxPhotos,
		Safe:            safeZone(p.Geo(nil), p.Settings.Safe),
		MinDPI:          min(lowDPILimit, p.Settings.DPI),
		TextMaxW:        p.TextMaxW(nil),
		BackdropUsed:    []string{},
		MarksRev:        st.MarksRev,
	}
	f := newFields(p, st.Print)
	ls := layAll(p, p.Spreads)
	for i, s := range p.Spreads {
		v.Spreads[i] = sheetView(f, s, ls[i], v.MinDPI)
		for _, b := range v.Spreads[i].Backdrops {
			v.BackdropUsed = append(v.BackdropUsed, b.Photo)
		}
		for _, pl := range v.Spreads[i].Places {
			if pl.Photo == "" {
				v.EmptyPlaces++
			}
		}
		v.LowDPI += v.Spreads[i].LowDPI
	}
	// Обложка — тем же f до result: её поля и имена попадают в Unfilled/Unnamed.
	v.Cover = coverView(f, st, &v)
	v.Unfilled, v.Unnamed = f.result()
	slices.Sort(v.BackdropUsed)
	v.BackdropUsed = slices.Compact(v.BackdropUsed)
	return v
}

// SpreadOf — вид одного листа id, как в BuildView для холста (заглушки видны): сухой прогон
// окна виньетки отдаёт его без сборки всего альбома. Нет такого листа — errNoSheet.
func SpreadOf(p *model.Project, id string) (model.SpreadView, error) {
	_, s := p.Find(id)
	if s == nil {
		return model.SpreadView{}, errNoSheet
	}
	return sheetView(newFields(p, false), s, Laid{}, min(lowDPILimit, p.Settings.DPI)), nil
}

// errNoSheet — листа нет (удалили, пока открыто окно).
var errNoSheet = errors.New("разворот не найден")

// sheetView — spreadView с отметкой мыльных кадров: общий шаг BuildView и SpreadOf.
func sheetView(f *fields, s *model.Spread, l Laid, minDPI int) model.SpreadView {
	sv := spreadView(f, s, l)
	sv.LowDPI = lowDPI(&sv, minDPI)
	return sv
}

// lowDPI — кадры и фото фона разворота ниже порога min: мыльный фон печатается так же
// мыльно, а найти его по ленте без значка нельзя (028 В). dpi 0 — размер неизвестен.
func lowDPI(sv *model.SpreadView, minDPI int) int {
	n := 0
	for _, f := range sv.Frames {
		if f.DPI > 0 && f.DPI < minDPI {
			n++
		}
	}
	for _, b := range sv.Backdrops {
		if b.DPI > 0 && b.DPI < minDPI {
			n++
		}
	}
	return n
}

// spreadView: всё берётся из кэша layout и text, поэтому повторные View дешёвые.
// l — раскладка, посчитанная заранее в layAll; не посчитана — считается здесь.
// f — подстановка полей и счёт незаполненного (один на весь вид).
func spreadView(f *fields, s *model.Spread, l Laid) (sv model.SpreadView) {
	p := f.p
	defer f.sheetStart(&sv)() // последним: итог листа — после всех подстановок
	f.vars = f.varsOf(s)      // до textViews и подписей: подстановки этого листа
	g := p.Geo(s)
	sv = model.SpreadView{ID: s.ID, W: g.W, H: g.H, Mid: g.Mid, Photos: append([]string{}, s.Photos...), Rects: []layout.Rect{},
		Frames: []model.FrameView{}, Captions: []model.CaptionView{}, Texts: textViews(f, s),
		Bg: pageBg(p, s), BgSet: [2]string{s.BgL, s.BgR}, Name: s.Name, Backdrops: backdropViews(p, s),
		Stubs: []model.StubView{}, Kind: s.Kind, Fields: model.FieldsOf(s, f.vars)}
	if s.Vars != nil {
		sv.Vars = *s.Vars
	}
	if s.Caps != nil {
		c := *s.Caps
		sv.Caps = &c
	}
	if s.IsVignette() {
		vv := model.NewVignetteView(*s.Vig)
		sv.Vig = &vv
	}
	defer zView(&sv, s) // после всех веток: z — по готовому виду
	if len(s.Grid) > 0 {
		gridView(f, &sv, s)
		return sv
	}
	if !l.done {
		l = lay(p, s)
	}
	if !l.ok {
		return sv
	}
	sv.Rects, sv.Layout = l.cur.Rects, l.cur.Key
	sv.Frames = frameViews(p, s.Photos, l.cur.Rects)
	sv.Captions, sv.Stubs = captionViews(f, s, s.Photos, l.cur.Rects)
	sv.Variant, sv.Variants, sv.More, sv.Rearranged, sv.Pages = l.variant, l.variants, l.more, l.rearranged, l.pages
	return sv
}
