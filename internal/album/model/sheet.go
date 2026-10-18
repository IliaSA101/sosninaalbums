package model

import (
	"cmp"
	"log"
	"math"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Геометрия листа (спека 032 §1, решение 62). Settings.PageW в коде листа читают только
// здесь, ClampSettings и Params (страж — TestPageWReaders): разворот, обложка и лист
// виньетки 031 различаются только Geo, остальное берёт размеры и страницы отсюда.

// Geo — геометрия листа, мм.
type Geo struct {
	W, H    float64       // размер листа
	Mid     float64       // «шов»: разворот — корешок, обложка — центр корешка; 0 — шва нет
	Margin  float64       // поле и промежуток раскладки — из настроек
	Gap     float64       //
	Pages   []layout.Rect // страницы: разворот — 2, обложка — сторонки (AreaOf), лист — 1
	Layflat bool          // место может идти через шов
}

// Geo — геометрия листа s; nil — обычный разворот (для обходов «по разворотам»).
func (p *Project) Geo(s *Spread) Geo {
	st := p.Settings
	if p.IsCover(s) {
		cg := p.CoverGeo()
		w, h := cg.X[7], cg.Y[3]
		return Geo{W: w, H: h, Mid: w / 2, Margin: st.Margin, Gap: st.Gap, Layflat: true,
			Pages: []layout.Rect{{X: 0, Y: 0, W: cg.X[3], H: h}, {X: cg.X[4], Y: 0, W: w - cg.X[4], H: h}}}
	}
	w, h := st.PageW, st.PageH
	if st.Sheet {
		// Лист 031 — одна страница во весь лист, шва нет (решение 62).
		return Geo{W: w, H: h, Margin: st.Margin, Gap: st.Gap, Layflat: true, Pages: []layout.Rect{{W: w, H: h}}}
	}
	return Geo{W: 2 * w, H: h, Mid: w, Margin: st.Margin, Gap: st.Gap, Layflat: st.Layflat,
		Pages: []layout.Rect{{X: 0, Y: 0, W: w, H: h}, {X: w, Y: 0, W: w, H: h}}}
}

// Sheet — лист по ID: обложка или разворот; nil — нет. Им пользуются операции, разрешённые
// на обложке (места, тексты, фоны, цвет, слои, выравнивание, группа); операции над порядком
// альбома и «Авто» берут Find — ID обложки там «не найден» (design 032 §1).
func (p *Project) Sheet(id string) *Spread {
	if id == CoverID {
		if p.Cover != nil {
			return p.Cover.Sheet
		}
		return nil
	}
	_, s := p.Find(id)
	return s
}

// Sheets — все листы: обложка первой, затем развороты. Для обходов по ID места, текста
// и фото и для инвариантов. Порядок решает, где остаётся фото-повтор при Load (dedupe):
// на обложке. Срез только для чтения: без обложки это сам p.Spreads.
func (p *Project) Sheets() []*Spread {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return p.Spreads
	}
	out := make([]*Spread, 0, len(p.Spreads)+1)
	return append(append(out, p.Cover.Sheet), p.Spreads...)
}

// CoverGeo — зоны обложки по X и Y и корешок при n = len(Spreads) (design 032 §1).
type CoverGeo struct {
	Spine float64 // корешок, мм
	N     int     // разворотов
	Over  bool    // n больше таблицы пресета
	// X — границы зон: 0, заворот, задняя, расстав (SpineL), корешок (SpineR), расстав,
	// лицевая, заворот (= W). Y — 0, заворот, сторонка, заворот (= H).
	X [8]float64
	Y [4]float64
}

// CoverGeo — зоны обложки; без обложки — нулевые.
func (p *Project) CoverGeo() CoverGeo {
	if p.Cover == nil {
		return CoverGeo{}
	}
	c := p.Cover.Spec
	w, h := c.W, c.H
	if w <= 0 {
		w = p.Settings.PageW
	}
	if h <= 0 {
		h = p.Settings.PageH
	}
	n := len(p.Spreads)
	sp, over := c.Spine(n)
	g := CoverGeo{Spine: sp, N: n, Over: over}
	step := [7]float64{c.Wrap, w, c.Hinge, sp, c.Hinge, w, c.Wrap}
	for i, d := range step {
		g.X[i+1] = geom.Round2(g.X[i] + d)
	}
	g.Y = [4]float64{0, c.Wrap, geom.Round2(c.Wrap + h), geom.Round2(2*c.Wrap + h)}
	return g
}

// SpineTextID — ID надписи на корешке: первый повёрнутый текст обложки с центром в зоне корешка;
// "" — нет. Одно условие на «надпись уже есть» у «Надписи на корешке» (cover.SpineText) и у
// кнопки «Выделить надпись» (CoverView.SpineText, 058 №19).
func (p *Project) SpineTextID() string {
	if p.Cover == nil || p.Cover.Sheet == nil {
		return ""
	}
	cg := p.CoverGeo()
	for _, t := range p.Cover.Sheet.Texts {
		if x, _ := BoxOf(t).Center(); t.Rot != 0 && x >= cg.X[3] && x <= cg.X[4] {
			return t.ID
		}
	}
	return ""
}

// FsferaFormatOf — формат Фотосферы по точному размеру страницы (047; ClampSettings округляет
// до 0,01, размеры таблицы целые). Лист 031 — не фотокнига: false. Здесь, а не в cover.go:
// страницу читает только геометрия листа (TestPageWReaders).
func FsferaFormatOf(s Settings) (PrintFormat, bool) {
	if s.Sheet {
		return PrintFormat{}, false
	}
	for _, f := range FsferaFormats() {
		if f.PageW == s.PageW && f.PageH == s.PageH {
			return f, true
		}
	}
	return PrintFormat{}, false
}

// FsferaFormatNear — формат Фотосферы, по которому считается шаблон обложки страницы s (070 Е):
// точный из таблицы, иначе ближайший по |ΔW|+|ΔH| страницы (при равенстве — раньше в таблице).
// Страница лист-виньетки 031 тоже приводится к ближайшему: обложки у неё нет, но пресет определён.
func FsferaFormatNear(s Settings) PrintFormat {
	if f, ok := FsferaFormatOf(s); ok {
		return f
	}
	all := FsferaFormats()
	best, bestD := all[0], math.Inf(1)
	for _, f := range all {
		if d := math.Abs(f.PageW-s.PageW) + math.Abs(f.PageH-s.PageH); d < bestD {
			best, bestD = f, d
		}
	}
	return best
}

// AreaOf — область scope листа s (left | right | spread | spine), мм; ok=false — неизвестная.
// Страницы — Geo.Pages: у обложки left — задняя с заворотом и расставом, right — лицевая.
// У листа 031 страниц нет: left и right — неизвестные (места, фон, шаблон страницы — ErrScope).
// spine — только у обложки (061): корешок с расставами и заворотом [X2, X5] × [0, H];
// области сторонок с ним перекрываются на расставах — их вид у старых альбомов не меняется.
func (p *Project) AreaOf(s *Spread, scope string) (layout.Rect, bool) {
	if scope == ScopeSpine {
		if !p.IsCover(s) {
			return layout.Rect{}, false
		}
		cg := p.CoverGeo()
		return layout.Rect{X: cg.X[2], Y: 0, W: geom.Round2(cg.X[5] - cg.X[2]), H: cg.Y[3]}, true
	}
	g := p.Geo(s)
	if len(g.Pages) < 2 && scope != ScopeSpread {
		return layout.Rect{}, false
	}
	switch scope {
	case ScopeLeft:
		return g.Pages[0], true
	case ScopeRight:
		return g.Pages[len(g.Pages)-1], true
	case ScopeSpread:
		return layout.Rect{W: g.W, H: g.H}, true
	}
	return layout.Rect{}, false
}

// BdArea — область фона scope листа s, мм (неизвестная — пустая).
func (p *Project) BdArea(s *Spread, scope string) layout.Rect {
	a, _ := p.AreaOf(s, scope)
	return a
}

// PageOf — номер страницы листа, где лежит x (правый край — уже следующая); вне страниц
// (корешок обложки, край листа) — ближайшая.
func (g Geo) PageOf(x float64) int { return geom.Sheet{Pages: g.Pages}.PageAt(x) }

// Page — страница i листа (зажимается в число страниц).
func (g Geo) Page(i int) layout.Rect { return g.Pages[max(0, min(i, len(g.Pages)-1))] }

// TextMaxW — предел ширины «по тексту» на листе s: страница без безопасных полей;
// дальше перенос по словам. nil — разворот.
func (p *Project) TextMaxW(s *Spread) float64 {
	return max(MinTextW, p.Geo(s).Pages[0].W-2*p.Settings.Safe)
}

// SheetAutoW — ширина «по тексту» блока t на листе s: единая точка для FitTexts и правок.
// Меряется вид холста с подстановками (AutoW → Shown, 030), предел — страница листа
// (TextMaxW по Geo, 032): обложка и лист 031 получают свой предел без особых случаев.
func (p *Project) SheetAutoW(s *Spread, t *TextBlock) float64 {
	var v *Vars
	if HasFields(t.Text) { // поиск полей листа — только когда есть что подставлять
		v = p.VarsOf(s)
	}
	return AutoW(v, t, p.TextMaxW(s))
}

// AlignSheet — лист s для опор выравнивания page/spread.
func (p *Project) AlignSheet(s *Spread) geom.Sheet {
	g := p.Geo(s)
	return geom.Sheet{W: g.W, H: g.H, Margin: g.Margin, Pages: g.Pages}
}

// NewSheet — проект-лист (031): лист w×h мм без корешка (Settings.Sheet), поля 8, промежуток 3,
// безопасная зона 5, 300 dpi; размеры зажимаются как у страницы. Один пустой лист.
func NewSheet(w, h float64) *Project {
	p := New()
	p.Settings = ClampSettings(Settings{PageW: w, PageH: h, Margin: 8, Gap: 3, Safe: 5, DPI: 300, Sheet: true})
	p.Spreads = []*Spread{{ID: NewID(), Photos: []string{}}}
	return p
}

// FoldPages — Load листа: страниц у него нет, поэтому свой цвет и фото фона страниц
// и узор (048) переходят на весь лист (левая важнее), а не теряются. Не лист — ничего.
func (p *Project) FoldPages() {
	if !p.Settings.Sheet {
		return
	}
	for _, s := range p.Spreads {
		if s.BgL == "" {
			s.BgL = s.BgR
		}
		s.BgR = s.BgL
		if s.BgPhoto == nil {
			s.BgPhoto = cmp.Or(s.BgPhotoL, s.BgPhotoR)
		}
		s.BgPhotoL, s.BgPhotoR = nil, nil
		foldPatterns(s)
	}
}

// Виньетка (031): разворот особого вида — обычная сетка, у мест роли (Place.Role), у листа
// параметры генератора (Spread.Vig). План мест строит пакет album/vignette, ставит grid.

// KindVignette — Spread.Kind виньетки; "" — обычный разворот.
const KindVignette = "vignette"

// Роли мест виньетки; "" — тоже ученик (место, добавленное руками).
const (
	RolePupil   = "pupil"
	RoleTeacher = "teacher"
	RoleGroup   = "group"
)

// Пределы и умолчания виньетки (решения 46, 47).
const (
	MaxTeachers = 32 // страница предметников 9–11 класса (049, решение 119)
	MaxPupils   = 64
	VigRatio    = "3:4"
	maxTplID    = 32
	// PupilCaption, TeacherCaption — шаблоны подписей, которые ставит «Применить» (031 §5).
	PupilCaption   = "{Фамилия}\n{Имя}"
	TeacherCaption = "{Фамилия}\n{Имя} {Отчество}"
	// StaffCaption — подпись учителя у шаблонов «Руководитель…» (049 §3.1); равна
	// vignette.StaffText (страж TestStaffCaption в ядре).
	StaffCaption = "{Фамилия}\n{Имя} {Отчество}\n{Предмет}"
	// VigHeadText — текст новой шапки; «класс» — в значении подзаголовка (решение 124).
	VigHeadText = "{Заголовок}\n{Подзаголовок} · {Год}"
)

// Vignette — параметры виньетки; места — обычная сетка с ролями (Place.Role).
type Vignette struct {
	Tpl      string `json:"tpl"`             // ID шаблона из каталога; чужой — умолчание
	Teachers int    `json:"teachers"`        // 0..MaxTeachers
	Pupils   int    `json:"pupils"`          // 0..MaxPupils
	Ratio    string `json:"ratio,omitempty"` // "2:3" | "3:4" | "4:5" | "1:1"
	TText    string `json:"ttext,omitempty"` // шаблон подписи учителя; "" — как у листа
	Head     string `json:"head,omitempty"`  // ID надписи-шапки этого листа; "" — нет
	// Own — копия геометрии своего шаблона (038, v7): лист самодостаточен на другом
	// компьютере и после удаления шаблона. nil — шаблон каталога. Только при Tpl с
	// VigOwnPrefix (FixVignettes). Не меняется на месте: копии разворота делят указатель.
	Own *VigShape `json:"own,omitempty"`
	// Shape, Round — форма мест учеников и учителей (044, как Place.Shape/Round);
	// групповое фото — всегда прямоугольник.
	Shape string  `json:"shape,omitempty"`
	Round float64 `json:"round,omitempty"`
}

// VigOwnPrefix — Vignette.Tpl своего шаблона: "my:" + ID шаблона в templates.json.
const VigOwnPrefix = "my:"

// VigShape — свой шаблон виньетки в листе: имя на момент выбора, места в долях листа, роли.
type VigShape struct {
	Name  string `json:"name"`
	Boxes []Box  `json:"boxes"`
	VigGeo
}

// Equal — та же форма; nil равен только nil.
func (s *VigShape) Equal(o *VigShape) bool {
	if s == nil || o == nil || s == o {
		return s == o
	}
	return s.Name == o.Name && slices.Equal(s.Boxes, o.Boxes) && s.equal(o.VigGeo)
}

// Same — те же параметры виньетки: Own сравнивается по содержимому, а не по указателю
// (после снимка правки или Load указатель другой).
func (v Vignette) Same(o Vignette) bool {
	a, b := v.Own, o.Own
	v.Own, o.Own = nil, nil
	return v == o && a.Equal(b)
}

// clampOwn — Own из файла или правки: зажим clampVigGeo, имя — CleanTemplateName. nil —
// строить нечего. Менять нечего — прежний указатель (копии разворота его делят).
func clampOwn(o *VigShape) *VigShape {
	boxes, g, ok := clampVigGeo(o.Boxes, o.VigGeo)
	if !ok {
		return nil
	}
	c := &VigShape{Name: CleanTemplateName(o.Name, 1), Boxes: boxes, VigGeo: g}
	if c.Equal(o) {
		return o
	}
	return c
}

// clampRatio — пропорция портрета виньетки: известная, иначе VigRatio.
func clampRatio(r string) string {
	switch r {
	case "2:3", "3:4", "4:5", "1:1":
		return r
	}
	return VigRatio
}

// DefaultVignette — «Шапка, учителя слева», 1 учитель, 24 ученика, 3:4 (031, сценарий 1).
func DefaultVignette() Vignette {
	return Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 24, Ratio: VigRatio}
}

// VigGroup — у шаблона tpl есть групповое фото. Каталог живёт в пакете vignette ниже ядра,
// а предел мест модели (Load) нужен с групповым: список сверяет TestVigGroupTpls.
func VigGroup(tpl string) bool {
	switch tpl {
	case "g-top", "head-g-top", "g-bottom", "head-g-bottom", "head-t-left-g", "t-top-g", "split-g", "split-tg":
		return true
	}
	return false
}

// VigPlaces — мест у виньетки v: учителя, ученики и групповые (у своего шаблона их число
// задаёт шаблон, у каталога — одно или нет).
func VigPlaces(v Vignette) int {
	n := v.Teachers + v.Pupils
	switch {
	case v.Own != nil:
		n += v.Own.Groups()
	case VigGroup(v.Tpl):
		n++
	}
	return n
}

// ClampVignette — параметры из файла или интерфейса: числа в пределах, пропорция известная
// (иначе 3:4), шаблон — одна строка, подпись учителя — CleanText, Own — clampOwn (строить
// нечего — снят, шаблон по умолчанию). Сумму мест не трогает: «Применить» её отвергает,
// Load уменьшает учеников (FixVignettes).
func ClampVignette(v Vignette) Vignette {
	v.Teachers = max(0, min(v.Teachers, MaxTeachers))
	v.Pupils = max(0, min(v.Pupils, MaxPupils))
	v.Ratio = clampRatio(v.Ratio)
	v.Shape, v.Round = ClampShape(v.Shape, v.Round)
	v.Tpl = cleanLine(v.Tpl, maxTplID)
	v.TText = CleanText(v.TText, MaxCaptionRunes)
	if strings.TrimSpace(v.TText) == "" {
		v.TText = ""
	}
	if v.Own != nil {
		if v.Own = clampOwn(v.Own); v.Own == nil {
			v.Tpl = DefaultVignette().Tpl
		}
	}
	return v
}

// VigCaps — стиль подписей, который «Применить» включает у листа без него (031 §5):
// подпись альбома ×0,8 (не меньше 8 пт), отступ 1 мм, шаблон «Фамилия / Имя».
func VigCaps(pageH float64) SheetCaps {
	st := NewCaptionStyle(pageH)
	st.Size = max(CaptionMinSize, st.Size*0.8)
	return SheetCaps{Style: ClampStyle(st), Pad: DefaultPad, Text: PupilCaption}
}

// IsVignette — лист — виньетка с параметрами.
func (s *Spread) IsVignette() bool { return s.Kind == KindVignette && s.Vig != nil }

// VigHead — надпись-шапка листа по ID; "" или нет такой — nil.
func (s *Spread) VigHead(id string) *TextBlock {
	if id == "" {
		return nil
	}
	for _, t := range s.Texts {
		if t.ID == id {
			return t
		}
	}
	return nil
}

// RoleOf — роль места виньетки: "" и чужая — ученик.
func RoleOf(pl *Place) string {
	if pl.Role == RoleTeacher || pl.Role == RoleGroup {
		return pl.Role
	}
	return RolePupil
}

// TeacherSeat — фото стоит на месте учителя какой-нибудь виньетки (049: карточка показывает
// «Предмет»). Фото — не больше чем в одном месте проекта; обложка виньеткой не бывает.
func (p *Project) TeacherSeat(photoID string) bool {
	for _, s := range p.Spreads {
		if !s.IsVignette() {
			continue
		}
		for _, pl := range s.Grid {
			if pl.Photo == photoID && photoID != "" {
				return pl.Role == RoleTeacher
			}
		}
	}
	return false
}

// FixVignettes — инварианты виньеток всех листов (Load и конвейер правки): Kind — "" или
// vignette, Vig — только у виньетки (нет — умолчание с числами по ролям мест), роли — только
// известные и только у виньетки, мест ≤ MaxPlaces (иначе меньше учеников), Head — надпись
// этого листа, Own — ровно при Tpl своего шаблона и с числом страниц листа (иначе снят,
// шаблон по умолчанию). Vig заменяется, а не правится на месте: копии разворота делят указатель.
func (p *Project) FixVignettes() {
	for _, s := range p.Sheets() {
		if s.Kind != "" && s.Kind != KindVignette {
			log.Printf("неизвестный вид разворота %s, открыт как обычный", paths.LogQ(s.Kind))
			s.Kind = ""
		}
		if s.Kind == "" {
			s.Vig = nil
			for _, pl := range s.Grid {
				pl.Role = ""
			}
			continue
		}
		v := vigByRoles(s)
		if s.Vig != nil {
			v = ClampVignette(*s.Vig)
		}
		own := strings.HasPrefix(v.Tpl, VigOwnPrefix)
		if v.Own != nil && (!own || v.Own.Pages != len(p.Geo(s).Pages)) {
			v.Own = nil // форма не к этому листу или не к этому шаблону
		}
		if v.Own == nil && own {
			v.Tpl = DefaultVignette().Tpl // своего шаблона без формы не построить
		}
		// Групповых своего шаблона может быть много: сверх предела уходят ученики, затем учителя.
		v.Pupils = max(0, v.Pupils-max(0, VigPlaces(v)-MaxPlaces))
		v.Teachers = max(0, v.Teachers-max(0, VigPlaces(v)-MaxPlaces))
		if v.Head != "" && !slices.ContainsFunc(s.Texts, func(t *TextBlock) bool { return t.ID == v.Head }) {
			v.Head = ""
		}
		for _, pl := range s.Grid {
			if pl.Role != RolePupil && pl.Role != RoleTeacher && pl.Role != RoleGroup {
				pl.Role = ""
			}
		}
		if s.Vig == nil || !v.Same(*s.Vig) {
			s.Vig = &v
		}
	}
}

// vigByRoles — параметры виньетки из файла без vig: числа — по ролям мест.
func vigByRoles(s *Spread) Vignette {
	v := DefaultVignette()
	v.Teachers, v.Pupils = 0, 0
	for _, pl := range s.Grid {
		switch RoleOf(pl) {
		case RoleTeacher:
			v.Teachers++
		case RolePupil:
			v.Pupils++
		}
	}
	return ClampVignette(v)
}

// clampVigIn — Load (spendTexts): подпись учителя списывается из запаса знаков только у
// виньетки, по cost — на каждое место учителя (tplCost), как в CheckTotals; без запаса
// теряется только она.
func clampVigIn(s *Spread, cost func(string) int, b *textBudget) *Vignette {
	if s.Vig == nil || s.Kind != KindVignette {
		return s.Vig
	}
	v := *s.Vig
	v.TText = CleanText(v.TText, MaxCaptionRunes)
	if strings.TrimSpace(v.TText) == "" || !b.takeRunes(cost(v.TText)) {
		v.TText = ""
	}
	return &v
}
