package view

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// lowDPILimit — ниже этого эффективного dpi фото печатается мыльно.
const lowDPILimit = 200

// defaultBg — цвет страницы, если не задан ни у страницы, ни у альбома.
const defaultBg = "#ffffff"

// safeZone: при layflat сгиб не ограничен — один контур по внешнему краю разворота;
// без layflat корешок режется, поэтому у каждой страницы свой контур с отступом и от корешка.
func safeZone(s model.Settings) []layout.Rect {
	h := max(s.PageH-2*model.SafeMargin, 0)
	if s.Layflat {
		return []layout.Rect{{X: model.SafeMargin, Y: model.SafeMargin, W: max(2*s.PageW-2*model.SafeMargin, 0), H: h}}
	}
	w := max(s.PageW-2*model.SafeMargin, 0)
	return []layout.Rect{
		{X: model.SafeMargin, Y: model.SafeMargin, W: w, H: h},
		{X: s.PageW + model.SafeMargin, Y: model.SafeMargin, W: w, H: h},
	}
}

// State — поля Editor, которые попадают в View (проект их не хранит).
type State struct {
	Path             string
	Dirty            bool
	CanUndo, CanRedo bool
	Rev, MarksRev    int
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

		MaxSpreadPhotos: model.MaxSpreadPhotos,
		Safe:            safeZone(p.Settings),
		MinDPI:          min(lowDPILimit, p.Settings.DPI),
		TextMaxW:        model.TextMaxW(p.Settings),
		BackdropUsed:    []string{},
		MarksRev:        st.MarksRev,
	}
	ls := layAll(p, p.Spreads)
	for i, s := range p.Spreads {
		v.Spreads[i] = spreadView(p, s, ls[i])
		for _, b := range v.Spreads[i].Backdrops {
			v.BackdropUsed = append(v.BackdropUsed, b.Photo)
		}
		for _, pl := range v.Spreads[i].Places {
			if pl.Photo == "" {
				v.EmptyPlaces++
			}
		}
		for _, f := range v.Spreads[i].Frames {
			if f.DPI > 0 && f.DPI < v.MinDPI {
				v.LowDPI++
			}
		}
	}
	slices.Sort(v.BackdropUsed)
	v.BackdropUsed = slices.Compact(v.BackdropUsed)
	return v
}

// spreadView: всё берётся из кэша layout и text, поэтому повторные View дешёвые.
// l — раскладка, посчитанная заранее в layAll; не посчитана — считается здесь.
func spreadView(p *model.Project, s *model.Spread, l Laid) (sv model.SpreadView) {
	sv = model.SpreadView{ID: s.ID, Photos: append([]string{}, s.Photos...), Rects: []layout.Rect{},
		Frames: []model.FrameView{}, Captions: []model.CaptionView{}, Texts: textViews(p, s),
		Bg: pageBg(p, s), BgSet: [2]string{s.BgL, s.BgR}, Name: s.Name, Backdrops: backdropViews(p, s)}
	defer zView(&sv, s) // после всех веток: z — по готовому виду
	if len(s.Grid) > 0 {
		gridView(p, &sv, s)
		return sv
	}
	if !l.done {
		l = lay(p, s)
	}
	if !l.ok {
		return sv
	}
	sv.Rects, sv.Layout = l.cur.Rects, l.cur.Key
	sv.Frames, sv.Captions = frameViews(p, s.Photos, l.cur.Rects), captionViews(p, s.Photos, l.cur.Rects)
	sv.Variant, sv.Variants, sv.Rearranged, sv.Pages = l.variant, l.variants, l.rearranged, l.pages
	return sv
}
