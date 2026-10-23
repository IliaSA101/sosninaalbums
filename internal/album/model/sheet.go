package model

import (
	"cmp"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
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

// FormatOf — формат шаблонной группы (Фотосфера, Фабрика Фотокниги) по точному размеру страницы
// (047, 081; ClampSettings округляет до 0,01, размеры таблиц целые). Лист 031 — не фотокнига:
// false. Здесь, а не в cover.go: страницу читает только геометрия листа (TestPageWReaders).
func FormatOf(group string, s Settings) (PrintFormat, bool) {
	tg, ok := templateGroupOf(group)
	if !ok || s.Sheet {
		return PrintFormat{}, false
	}
	for _, f := range tg.Formats() {
		if f.PageW == s.PageW && f.PageH == s.PageH {
			return f, true
		}
	}
	return PrintFormat{}, false
}

// FormatNear — формат группы, по которому считается шаблон обложки страницы s (070 Е): точный из
// таблицы, иначе ближайший по |ΔW|+|ΔH| страницы (при равенстве — раньше в таблице). Страница
// лист-виньетки 031 тоже приводится к ближайшему: обложки у неё нет, но пресет определён.
// Группа не шаблонная — нулевой формат.
func FormatNear(group string, s Settings) PrintFormat {
	tg, ok := templateGroupOf(group)
	if !ok {
		return PrintFormat{}
	}
	all := tg.Formats()
	for _, f := range all {
		if f.PageW == s.PageW && f.PageH == s.PageH {
			return f
		}
	}
	best, bestD := all[0], math.Inf(1)
	for _, f := range all {
		if d := math.Abs(f.PageW-s.PageW) + math.Abs(f.PageH-s.PageH); d < bestD {
			best, bestD = f, d
		}
	}
	return best
}

// FsferaFormatOf — формат Фотосферы по точному размеру страницы (047); лист 031 — false.
func FsferaFormatOf(s Settings) (PrintFormat, bool) { return FormatOf(CoverGroupFsfera, s) }

// FsferaFormatNear — ближайший формат Фотосферы для шаблона обложки страницы s (070 Е).
func FsferaFormatNear(s Settings) PrintFormat { return FormatNear(CoverGroupFsfera, s) }

// presetFits — страница s совпадает с кнопкой pr целиком: размер и все заданные в ней поля
// (правило подсветки presetActive в интерфейсе).
func presetFits(pr PagePreset, s Settings) bool {
	return pr.W == s.PageW && pr.H == s.PageH &&
		(pr.DPI == 0 || pr.DPI == s.DPI) && (!pr.Layflat || s.Layflat) &&
		(pr.Safe == 0 || pr.Safe == s.Safe) && (pr.Bleed == 0 || pr.Bleed == s.Bleed)
}

// PagePresetOf — кнопка ряда типографии, с которой совпадает страница s (085: типография альбома
// без обложки). Среди кнопок с этим размером первой берётся та, что совпала по всем заданным
// полям (081 С3: 21×30 Фабрики и Фотосферы различает зона и обрез), иначе первая по порядку
// PagePresets. Общий ряд и лист 031 — false. Здесь: страницу читает только геометрия листа
// (TestPageWReaders).
func PagePresetOf(s Settings) (PagePreset, bool) {
	if s.Sheet {
		return PagePreset{}, false
	}
	var first PagePreset
	found := false
	for _, pr := range PagePresets() {
		if pr.Group == "" || pr.W != s.PageW || pr.H != s.PageH {
			continue
		}
		if presetFits(pr, s) {
			return pr, true
		}
		if !found {
			first, found = pr, true
		}
	}
	return first, found
}

// PagePresetIn — кнопка ряда group с размером страницы s; лист 031, общий ряд и размер вне ряда —
// false. Формат строки предела (LimitIn) берётся из ряда типографии, а не из первой кнопки по
// размеру: 30×30 Фабрики и Нетпринта — один размер (081 С4).
func PagePresetIn(s Settings, group string) (PagePreset, bool) {
	if s.Sheet || group == "" {
		return PagePreset{}, false
	}
	for _, pr := range PagePresets() {
		if pr.Group == group && pr.W == s.PageW && pr.H == s.PageH {
			return pr, true
		}
	}
	return PagePreset{}, false
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
