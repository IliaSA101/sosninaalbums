package view

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Подписи к фото и заглушки (030): текст — своя подпись или шаблон листа, оформление —
// своё или листа (Spread.Caps), поля подставляет model.Shown. Длинная строка в стиле
// листа ужимается, не хватило — перенос и «!».

const (
	// CaptionPad — зазор между фото и подписью по умолчанию, мм (Caption.Pad = nil).
	CaptionPad = model.DefaultPad
	// trimZone — обрезная зона типографии у нижнего края страницы (fsfera 2–3 мм):
	// подпись в неё не заходит.
	trimZone = 3.0
	// overLift — подпись «на фото» стоит на pad + overLift от нижнего края: при pad 1 мм
	// это прежние 3 мм (геометрия старых альбомов и эталоны выгрузки не сдвигаются).
	overLift = 2.0
	// fitStep, fitMin — шаг и нижний предел ужимания кегля, пт (decisions 0.10, 36).
	fitStep = 0.5
	fitMin  = 6.0
)

// Look — действующее оформление подписи: своё или листа.
type Look struct {
	Style model.TextStyle
	Over  bool
	Pad   float64 // мм от фото (на фото — от нижнего края, плюс overLift)
	Fit   bool    // ужимать строку шире ячейки (стиль листа без Wrap)
	Sheet bool    // оформление от листа
}

// LookOf — оформление подписи: при стиле листа caps — его (своё не действует, но
// хранится), иначе своё own (отступ nil — 1 мм, без ужимания).
func LookOf(own *model.Caption, caps *model.SheetCaps) Look {
	if caps != nil {
		return Look{Style: caps.Style, Over: caps.Over, Pad: caps.Pad, Fit: !caps.Wrap, Sheet: true}
	}
	if own == nil {
		return Look{Pad: CaptionPad}
	}
	return Look{Style: own.Style, Over: own.Over, Pad: own.PadOf()}
}

// CaptionIn — вход CaptionViewOf: подпись в ячейке Rects[I]. Структура, а не список
// аргументов: виньетка (031) и обложка (032) добавят поля без смены сигнатуры.
type CaptionIn struct {
	Photo string        // ID фото; у заглушки пустого места ""
	Text  string        // текст для показа (поля уже подставлены)
	Src   string        // исходный текст, если отличается от Text
	Look  Look          // оформление
	Rects []layout.Rect // ячейки листа: Rects[I] — своя, остальные — препятствия снизу
	I     int
	PageH float64 // низ листа, мм: у разворота — высота страницы, у обложки — линия обреза снизу
	// Outlines — формы ячеек (044, рядом с Rects; у «Авто» nil): подпись «на фото» стоит во
	// вписанном прямоугольнике формы, «под фото» и препятствия — по рамкам.
	Outlines []geom.Outline
}

// CaptionViewOf — место подписи к фото в ячейке Rects[I] шириной в ячейку.
// Под фото: на Pad ниже, до низа листа PageH без обрезной зоны и до ячейки, стоящей
// ниже с перекрытием по горизонтали (тоже с Pad). На фото: у нижнего края ячейки
// внутри неё. Свободные блоки текста препятствием не считаются.
func CaptionViewOf(in CaptionIn) model.CaptionView {
	r := in.Rects[in.I]
	if in.Look.Over && in.I < len(in.Outlines) {
		r = geom.InnerBox(r, in.Outlines[in.I])
	}
	st, tooWide := in.Look.Style, false
	if in.Look.Fit {
		st, tooWide = fitSize(in.Text, st, r.W)
	}
	tv := placeText("", in.Text, st, r.X, 0, r.W) // по вертикали сдвигается ниже
	tv.Src = in.Src
	pad, h := in.Look.Pad, tv.H
	var top, over float64
	if in.Look.Over {
		top = r.Y + r.H - pad - overLift - h
		over = r.Y + pad - top
	} else {
		top = r.Y + r.H + pad
		limit := in.PageH - trimZone
		for j, o := range in.Rects {
			below := o.Y >= r.Y+r.H-1e-6
			if j != in.I && below && o.X < r.X+r.W && o.X+o.W > r.X {
				limit = math.Min(limit, o.Y-pad)
			}
		}
		over = top + h - limit
	}
	tv.Y = top
	for k := range tv.Lines {
		tv.Lines[k].Y += top
	}
	over = math.Max(0, math.Round(over*10)/10)
	if tooWide { // и fitMin не хватило: строка перенеслась — «!»
		over = math.Max(over, 0.1)
	}
	return model.CaptionView{Photo: in.Photo, TextView: tv, Over: in.Look.Over, Overflow: over, Sheet: in.Look.Sheet, Pad: pad}
}

// fitSize — кегль, при котором самая длинная строка s встаёт в ширину w: вниз шагом
// fitStep, не ниже fitMin (и не выше своего). tooWide — не встала и при fitMin.
func fitSize(s string, st model.TextStyle, w float64) (model.TextStyle, bool) {
	nat := text.NaturalWidth(s, st)
	if nat <= w {
		return st, false
	}
	if st.Size > fitMin {
		st.Size = math.Min(st.Size, math.Max(fitMin, math.Floor(st.Size*w/nat/fitStep)*fitStep))
		// Ширина строки округляется в кэше: впритык слово ушло бы на новую строку.
		for st.Size > fitMin && text.NaturalWidth(s, st) > w-0.01 {
			st.Size = math.Max(fitMin, st.Size-fitStep)
		}
	}
	return st, text.NaturalWidth(s, st) > w-0.01
}

// fields — подстановка полей при сборке вида: режим (холст или печать) и что осталось
// незаполненным. Один на BuildView: все листы проходят через него, поэтому Unfilled и
// Unnamed считаются по всем листам (обложка 032 — тем же spreadView); sheet — то же
// по листу, который сейчас собирается (SpreadView.Unfilled/Unnamed).
type fields struct {
	p        *model.Project
	print    bool
	unfilled map[string]bool
	unnamed  int
	sheet    map[string]bool
	// vars — поля шапки листа, который сейчас собирает spreadView (VarsOf): весь лист
	// собирается подряд, поэтому одно значение на лист, а не поиск у каждой подписи.
	vars   *model.Vars
	varsOf func(*model.Spread) *model.Vars // VarsOf с полями первой виньетки, найденными один раз
}

func newFields(p *model.Project, forPrint bool) *fields {
	return &fields{p: p, print: forPrint, unfilled: map[string]bool{}, varsOf: p.SheetVars()}
}

func (f *fields) mode() model.FieldMode {
	if f.print {
		return model.FieldPrint
	}
	return model.FieldCanvas
}

// text — надпись для показа и исходник (если отличается); пустые поля — в Unfilled
// (имени у надписи нет, поэтому и {Фамилия} в ней — пустое поле).
func (f *fields) text(s string) (out, src string) {
	out, miss := model.Shown(f.vars, s, nil, f.mode())
	for _, m := range miss {
		f.miss(m)
	}
	if out != s {
		src = s
	}
	return out, src
}

// miss — пустое поле m: в итог вида и листа.
func (f *fields) miss(m string) {
	f.unfilled[m] = true
	if f.sheet != nil {
		f.sheet[m] = true
	}
}

// sheetStart начинает счёт листа; вызванная функция кладёт итог листа в sv.
func (f *fields) sheetStart(sv *model.SpreadView) func() {
	f.sheet = map[string]bool{}
	u0 := f.unnamed
	return func() {
		sv.Unfilled, sv.Unnamed = sortedKeys(f.sheet), f.unnamed-u0
		f.sheet = nil
	}
}

// result — Unfilled по алфавиту и Unnamed.
func (f *fields) result() ([]string, int) { return sortedKeys(f.unfilled), f.unnamed }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// capKind — что показать у фото: ничего, подпись или заглушку.
type capKind int

const (
	capNone capKind = iota
	capReal
	capStub
)

// capTemplate — шаблон подписи листа s для места роли role: у учителя виньетки — свой
// (Vig.TText, 031 §6), иначе шаблон листа; "" — шаблона нет.
func capTemplate(s *model.Spread, role string) string {
	switch {
	case s.Caps == nil:
		return ""
	case role == model.RoleTeacher && s.IsVignette() && s.Vig.TText != "":
		return s.Vig.TText
	}
	return s.Caps.Text
}

// photoRoles — роль места каждого фото виньетки s; у прочих листов nil (роль "").
func photoRoles(s *model.Spread) map[string]string {
	if !s.IsVignette() {
		return nil
	}
	out := make(map[string]string, len(s.Grid))
	for _, pl := range s.Grid {
		if pl.Photo != "" {
			out[pl.Photo] = pl.Role
		}
	}
	return out
}

// caption — подпись фото id в месте роли role на листе s: текст для показа, исходник и вид.
// Своя подпись заменяет шаблон листа текстом; names — поля имени шаблона (037), у своей
// подписи nil. Фото без своей подписи и без имени под шаблоном с полями имени — заглушка
// (на холсте; в печати ничего).
func (f *fields) caption(s *model.Spread, id, role string) (out, src string, look Look, kind capKind, names []string) {
	ref := f.p.Photos[id]
	own, caps := ref.Caption, s.Caps
	switch tpl := capTemplate(s, role); {
	case own != nil:
		src = own.Text
	case tpl != "":
		src = tpl
	default:
		return "", "", Look{}, capNone, nil
	}
	look = LookOf(own, caps)
	if own == nil {
		names = model.NameFieldsOf(src)
	}
	who, _ := ref.Name()
	out, miss := model.ShownCaption(f.vars, src, who, f.mode())
	unnamed := f.noName(src, who, miss)
	if unnamed {
		f.unnamed++
	}
	if unnamed && own == nil {
		if f.print {
			return "", "", look, capNone, nil
		}
		out, _ = model.ShownCaption(f.vars, src, nil, model.FieldStub)
		return out, "", look, capStub, names
	}
	for _, m := range miss {
		if !model.IsNameField(m) {
			f.miss(m)
		}
	}
	if out == src {
		src = ""
	}
	return out, src, look, capReal, names
}

// noName — в тексте src есть поля имени, но ни одно не заполнено (miss — пустые поля
// подстановки с именем who).
func (f *fields) noName(src string, who *model.Person, miss []string) bool {
	n := countNames(miss)
	if n == 0 {
		return false
	}
	if who == nil {
		return true
	}
	_, all := model.Shown(f.vars, src, nil, model.FieldPrint)
	return countNames(all) == n
}

func countNames(miss []string) int {
	n := 0
	for _, m := range miss {
		if model.IsNameField(m) {
			n++
		}
	}
	return n
}

// sheetBottom — низ листа s для подписей: у обложки — линия обреза снизу (ниже — заворот),
// у разворота и листа — высота страницы.
func sheetBottom(p *model.Project, s *model.Spread) float64 {
	if p.IsCover(s) {
		return p.CoverGeo().Y[2]
	}
	return p.Geo(s).H
}

// captionViews — подписи фото листа s в порядке ленты (ids[i] — в ячейке rects[i] формы
// outlines[i]; у «Авто» outlines nil) и заглушки фото без имени. Раскладку подпись не
// раздвигает: не хватило места — Overflow > 0, интерфейс показывает «!».
func captionViews(f *fields, s *model.Spread, ids []string, rects []layout.Rect, outlines []geom.Outline) (caps []model.CaptionView, stubs []model.StubView) {
	caps, stubs = []model.CaptionView{}, []model.StubView{}
	roles, bottom := photoRoles(s), sheetBottom(f.p, s)
	for i := range rects {
		out, src, look, kind, who := f.caption(s, ids[i], roles[ids[i]])
		if kind == capNone {
			continue
		}
		cv := CaptionViewOf(CaptionIn{Photo: ids[i], Text: out, Src: src, Look: look, Rects: rects, I: i, PageH: bottom, Outlines: outlines})
		if kind == capStub {
			stubs = append(stubs, model.StubView{Photo: ids[i], TextView: cv.TextView, Who: who})
			continue
		}
		cv.Who = who
		caps = append(caps, cv)
	}
	return caps, stubs
}

// placeStubs — заглушки пустых мест сетки под шаблоном листа: текст шаблона в режиме
// заглушки («[Фамилия]», переменные — значения); у учителя виньетки — его шаблон.
// Препятствия снизу — все места листа.
func placeStubs(f *fields, sv *model.SpreadView, s *model.Spread) {
	if f.print || s.Caps == nil {
		return
	}
	rects, outlines := make([]layout.Rect, len(s.Grid)), make([]geom.Outline, len(s.Grid))
	for i, pl := range s.Grid {
		rects[i], outlines[i] = layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}, pl.Outline()
	}
	look, bottom := LookOf(nil, s.Caps), sheetBottom(f.p, s)
	for i, pl := range s.Grid {
		tpl := capTemplate(s, pl.Role)
		if pl.Photo != "" || tpl == "" {
			continue
		}
		out, _ := model.ShownCaption(f.vars, tpl, nil, model.FieldStub)
		cv := CaptionViewOf(CaptionIn{Text: out, Look: look, Rects: rects, I: i, PageH: bottom, Outlines: outlines})
		turnCaption(&cv, pl)
		sv.Stubs = append(sv.Stubs, model.StubView{Place: pl.ID, TextView: cv.TextView})
	}
}
