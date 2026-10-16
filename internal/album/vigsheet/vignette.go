package vigsheet

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Виньетка (031 design §5): тела правок над проектом. План мест строит чистый пакет
// album/vignette, но области его не импортируют (depguard: grid — только model и нижние
// пакеты), поэтому ядро передаёт план функцией VigPlanner.

// VigSlot — место плана: рамка портрета в мм листа и роль.
type VigSlot struct {
	Box  layout.Rect
	Role string
}

// VigPlan — план виньетки: места в порядке ученики → учителя → групповое, полоса шапки
// (nil — у шаблона её нет), портрет ученика в мм. HeadCell — шапка в ячейках последнего
// ряда (049): кегль считается и по ширине, а не только по высоте (headStyle).
type VigPlan struct {
	Slots    []VigSlot
	Head     *layout.Rect
	HeadCell bool
	// HeadFit — кегль шапки в ряду по замеру ядра (текст пакету не виден — depguard), пт; 0 — нет.
	HeadFit float64
	PupilW  float64
	// HeadStyle — стиль новой шапки (своего шаблона, 038); nil — по высоте полосы (headStyle).
	HeadStyle *model.TextStyle
}

// TplText — подпись учителя каталожного шаблона по ID (vignette.Tpl.TText; "" — своей нет).
// Ядро передаёт её функцией по той же причине, что и VigPlanner.
type TplText func(tpl string) string

// VigPlanner — план листа s проекта p по параметрам v: поля, промежуток и страницы — из
// p.Geo(s), полоса подписи — из s.Caps (nil — подписей нет).
type VigPlanner func(p *model.Project, s *model.Spread, v model.Vignette) VigPlan

// errVigSpread — разворота нет (удалили, пока открыта панель).
var errVigSpread = errors.New("разворот не найден")

func errVigLimit() error {
	return fmt.Errorf("не больше %d мест на развороте", model.MaxPlaces)
}

func errVigHead() error {
	return fmt.Errorf("на развороте уже %d надписей: шапке нет места", model.MaxTexts)
}

// Отклонение места от плана, при котором виньетка ещё нетронута, мм (зажим округляет до 0,01).
const vigTol = 0.05

// SetVignette — «Сделать виньеткой» и «Применить»: v уже зажат (ClampVignette). Мест больше
// MaxPlaces — ошибка без правки. У листа без стиля подписей он включается (VigCaps); подпись
// учителя по умолчанию — TeacherCaption; v.Head не читается — шапка та, что у листа.
// own — свой шаблон (ResolveOwn; nil — каталог: v.Own из аргумента не берётся). Оформление
// шаблона (стиль подписей, подпись учителя, стиль шапки) встаёт только при смене шаблона:
// повторное «Применить» с теми же числами меняет одни места (038, решение 146). Форма мест
// (044) — из v, у своего шаблона с формой — из его стиля (как подпись учителя); местам людей
// она встаёт вся при новой виньетке, смене шаблона или формы, иначе — только новым местам.
// tplText (nil — подписей у шаблонов нет) — подпись учителя шаблона v.Tpl: при смене шаблона
// подменяет подпись по умолчанию (SwapTText, 049 §3.1); у своего шаблона — "". Кегль прежней шапки
// при смене шаблона встаёт под новую полосу (refitHead); шаблон без шапки снимает её, если текст
// шаблонный (решение №44 0.10), правленую оставляет. left — сколько фото ушло в ленту.
func SetVignette(p *model.Project, spreadID string, v model.Vignette, plan VigPlanner, own *OwnTpl, tplText TplText) (int, error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return 0, errVigSpread
	}
	v.Own = nil
	if own != nil {
		v.Own = own.Shape
	}
	if model.VigPlaces(v) > model.MaxPlaces {
		return 0, errVigLimit()
	}
	fresh := !s.IsVignette() || s.Vig.Tpl != v.Tpl
	var was *VigPlan // план прежнего шаблона — кегль шапки по его полосе (один Build)
	if fresh && s.IsVignette() && s.VigHead(s.Vig.Head) != nil {
		w := plan(p, s, *s.Vig)
		was = &w
	}
	if fresh && tplText != nil { // до своего шаблона: его подпись (если есть) важнее
		tt := ""
		if own == nil {
			tt = tplText(v.Tpl)
		}
		v.TText = SwapTText(v.TText, tt)
	}
	p.Freeze(s)
	var headSt *model.TextStyle
	if fresh && own != nil && own.Style != nil {
		if c := own.Style.Caps; c != nil {
			cc := *c
			s.Caps = &cc
		}
		if own.Style.TText != "" {
			v.TText = own.Style.TText
		}
		if own.Style.Shape != "" || own.Style.Round != 0 {
			v.Shape, v.Round = own.Style.Shape, own.Style.Round
		}
		headSt = own.Style.Head
	}
	reshape := fresh || s.Vig.Shape != v.Shape || s.Vig.Round != v.Round
	if s.Caps == nil {
		c := model.VigCaps(p.Geo(s).H)
		s.Caps = &c
	}
	if v.TText == "" {
		v.TText = model.TeacherCaption
	}
	v.Head = "" // шапку знает лист, а не панель
	if s.IsVignette() {
		v.Head = s.Vig.Head
	} else if s.Vars == nil {
		s.Vars = inheritVars(p, s)
	}
	if h := s.VigHead(v.Head); h != nil && headSt != nil {
		h.Style = *headSt
	}
	pl := plan(p, s, v)
	pl.HeadStyle = headSt
	dropHead(s, &v, pl, fresh)
	if h := s.VigHead(v.Head); h != nil && was != nil && headSt == nil {
		refitHead(h, *was, pl)
	}
	return placeVignette(p, s, v, pl, true, reshape)
}

// dropHead — смена шаблона (fresh) на шаблон без шапки (у плана pl её нет) снимает шапку листа,
// если её текст шаблонный (решение №44 0.10: «убрать насовсем — шаблон без шапки»), и v.Head;
// правленая пользователем остаётся. Поля шапки (Vars) не трогаются.
func dropHead(s *model.Spread, v *model.Vignette, pl VigPlan, fresh bool) {
	if !fresh || pl.Head != nil {
		return
	}
	if h := s.VigHead(v.Head); h != nil && h.Text == model.VigHeadText {
		s.Texts = slices.DeleteFunc(s.Texts, func(t *model.TextBlock) bool { return t == h })
		v.Head = ""
	}
}

// SwapTText — подпись учителя при смене шаблона (049 §3.1): шаблон со своей подписью tpl
// («Руководитель…») ставит её вместо умолчания, шаблон без неё возвращает умолчание вместо
// подписи предметника. Своя подпись не трогается. Превью окна (vigChoices) считает так же.
func SwapTText(cur, tpl string) string {
	switch {
	case tpl != "" && (cur == "" || cur == model.TeacherCaption):
		return tpl
	case tpl == "" && cur == model.StaffCaption:
		return model.TeacherCaption
	}
	return cur
}

// refitHead — кегль шапки h при смене шаблона (049 §3.2): кегль «по полосе» прежнего плана
// was встаёт по полосе нового cur; свой только уменьшается — до кегля новой полосы, если она
// мельче прежней (на такой же полосе свой кегль влезал, решение №177). Прежней полосы нет —
// свой не больше кегля новой. Гарнитура, цвет, выравнивание и текст — прежние.
func refitHead(h *model.TextBlock, was, cur VigPlan) {
	if cur.Head == nil {
		return
	}
	size, old := headStyle(*cur.Head, cur.HeadCell, cur.HeadFit).Size, math.Inf(1)
	if was.Head != nil {
		old = headStyle(*was.Head, was.HeadCell, was.HeadFit).Size
	}
	switch {
	case h.Style.Size == old:
		h.Style.Size = size
	case size < old:
		h.Style.Size = min(h.Style.Size, size)
	}
}

// SetupVignette — «Применить» окна «Настроить виньетку» (031 dialog §5): поля шапки листа
// (зажим ClampVars; все пусты — снять) и параметры (зажим ClampVignette) одной правкой.
// Параметры виньетки прежние — пишутся только поля: иначе правка шапки сбросила бы сдвинутые
// места и вернула удалённую шапку. Неизвестный лист и мест больше MaxPlaces — ошибка до
// любых изменений. Свой шаблон ("my:<id>") берётся из tpls (ResolveOwn), а не из st.Vig.Own:
// геометрию от интерфейса не принимаем. tplText — как у SetVignette.
func SetupVignette(p *model.Project, spreadID string, st model.VigSetup, plan VigPlanner, tpls []model.Template, tplText TplText) (int, error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return 0, errVigSpread
	}
	st.Vig.Own = nil
	v := model.ClampVignette(st.Vig)
	own, err := ResolveOwn(p, spreadID, v.Tpl, tpls)
	if err != nil {
		return 0, err
	}
	if own != nil {
		v.Own = own.Shape
	}
	if model.VigPlaces(v) > model.MaxPlaces {
		return 0, errVigLimit()
	}
	s.Vars = model.ClampVars(st.Vars)
	if same := v; s.IsVignette() {
		same.Shape, same.Round = s.Vig.Shape, s.Vig.Round
		if sameVig(s, same) {
			keepVig(s, v)
			return 0, nil
		}
	}
	return SetVignette(p, spreadID, v, plan, own, tplText)
}

// keepVig — «Применить» без пересборки: параметры те же с точностью до имени своего шаблона
// и формы (044). Новое имя пишется в копию Vig; новая форма — в Vig и местам учеников и
// учителей (групповое — прямоугольник), места, сдвиги и фото не трогаются.
func keepVig(s *model.Spread, v model.Vignette) {
	nv := *s.Vig
	if v.Own != nil && v.Own.Name != nv.Own.Name { // шаблон переименован: новое имя, места прежние
		nv.Own = v.Own
	}
	if nv.Shape != v.Shape || nv.Round != v.Round {
		nv.Shape, nv.Round = v.Shape, v.Round
		for _, pl := range s.Grid {
			if model.RoleOf(pl) != model.RoleGroup {
				pl.Shape, pl.Round = v.Shape, v.Round
			}
		}
	}
	s.Vig = &nv
}

// sameVig — у виньетки s (s.IsVignette — проверяет вызывающий) уже параметры v (зажатые);
// пустая подпись учителя из окна равна умолчанию, которое ставит SetVignette. Head не
// сравнивается — шапку знает лист; имя своего шаблона тоже: после переименования «Применить»
// иначе пересобрал бы места и потерял сдвиги.
func sameVig(s *model.Spread, v model.Vignette) bool {
	if v.TText == "" && s.Vig.TText == model.TeacherCaption {
		v.TText = model.TeacherCaption
	}
	v.Head = s.Vig.Head
	if v.Own != nil && s.Vig.Own != nil && v.Own.Name != s.Vig.Own.Name {
		o := *v.Own
		o.Name = s.Vig.Own.Name
		v.Own = &o
	}
	return v.Same(*s.Vig)
}

// inheritVars — поля шапки новой виньетки s: Заголовок и Год ближайшей предыдущей виньетки,
// у которой есть хоть одно из них (школа и год у классов одни); подзаголовок и руководитель
// у каждого класса свои. Такой нет — nil.
func inheritVars(p *model.Project, s *model.Spread) *model.Vars {
	i, _ := p.Find(s.ID)
	for k := i - 1; k >= 0; k-- {
		o := p.Spreads[k]
		if !o.IsVignette() || o.Vars == nil {
			continue
		}
		if v := (model.Vars{Title: o.Vars.Title, Year: o.Vars.Year}); v != (model.Vars{}) {
			return &v
		}
	}
	return nil
}

// placeVignette ставит места плана: ID — прежние ID мест той же роли по порядку (с ними и
// режим кадра, и форма — 044), сверх них — новые; фото садятся по своей роли в прежнем
// порядке, остаток уходит из листа. Новые места людей получают форму v, групповое —
// прямоугольник; reshape (новая виньетка, смена шаблона или формы) — форму v всем местам
// людей. Шапка встаёт в свою полосу; нет надписи — создаётся, только если mkHead («Применить»):
// пересборка Relay удалённую пользователем шапку не возвращает. Тот же план даёт тот же
// проект: повторное «Применить» — холостая правка. Ошибка — до любых изменений.
func placeVignette(p *model.Project, s *model.Spread, v model.Vignette, plan VigPlan, mkHead, reshape bool) (int, error) {
	if len(plan.Slots) > model.MaxPlaces {
		return 0, errVigLimit()
	}
	head := s.VigHead(v.Head)
	if plan.Head != nil && head == nil && mkHead && len(s.Texts) >= model.MaxTexts {
		return 0, errVigHead()
	}
	had := len(s.Photos)
	ids, photos, fits := map[string][]string{}, map[string][]string{}, map[string]string{}
	shapes := map[string]model.Place{} // форма прежнего места по ID
	if len(s.Grid) == 0 {              // «Авто», которое не закрепилось: фото ленты — ученики по порядку
		photos[model.RolePupil] = slices.Clone(s.Photos)
	}
	for _, pl := range s.Grid {
		r := model.RoleOf(pl)
		ids[r] = append(ids[r], pl.ID)
		fits[pl.ID] = pl.Fit
		shapes[pl.ID] = model.Place{Shape: pl.Shape, Round: pl.Round}
		if pl.Photo != "" {
			photos[r] = append(photos[r], pl.Photo)
		}
	}
	taken := p.PlaceIDs()
	next := func(m map[string][]string, r string) string {
		if len(m[r]) == 0 {
			return ""
		}
		x := m[r][0]
		m[r] = m[r][1:]
		return x
	}
	grid := make([]*model.Place, len(plan.Slots))
	for i, sl := range plan.Slots {
		b := sl.Box
		pl := &model.Place{ID: next(ids, sl.Role), X: b.X, Y: b.Y, W: b.W, H: b.H, Role: sl.Role}
		if pl.ID == "" {
			pl.ID = model.NewPlaceID(taken)
		}
		pl.Fit, pl.Photo = fits[pl.ID], next(photos, sl.Role)
		old, kept := shapes[pl.ID]
		vigShape(pl, old, kept && (!reshape || sl.Role == model.RoleGroup), v)
		p.ClampPlace(s, pl)
		grid[i] = pl
	}
	s.Grid = grid
	s.Photos = model.GridPhotos(s)
	if plan.Head != nil && (head != nil || mkHead) {
		if head == nil {
			head = &model.TextBlock{ID: model.NewID(), Text: model.VigHeadText, Style: plan.headStyleOr()}
			s.Texts = append(s.Texts, head)
		}
		putHead(head, *plan.Head)
	}
	v.Head = ""
	if head != nil {
		v.Head = head.ID
	}
	s.Kind, s.Vig = model.KindVignette, &v
	return had - len(s.Photos), nil
}

// vigShape — форма места плана pl (044): прежняя old, если keep, иначе у ученика и учителя —
// форма виньетки v, у группового — прямоугольник.
func vigShape(pl *model.Place, old model.Place, keep bool, v model.Vignette) {
	switch {
	case keep:
		pl.Shape, pl.Round = old.Shape, old.Round
	case pl.Role != model.RoleGroup:
		pl.Shape, pl.Round = v.Shape, v.Round
	}
}

// headStyleOr — стиль новой шапки: шаблона, иначе по высоте полосы.
func (pl VigPlan) headStyleOr() model.TextStyle {
	if pl.HeadStyle != nil {
		return *pl.HeadStyle
	}
	return headStyle(*pl.Head, pl.HeadCell, pl.HeadFit)
}

// headStyle — стиль новой шапки: шрифт по умолчанию, по центру, кегль от высоты полосы band
// (две строки в полосе), 12…48 пт. Шапка в ряду (cell, 049 §3.2) высокая и узкая: кегль — fit,
// замер ядра (VigPlan.HeadFit); без замера (0) — ещё и не больше ширины/9.
func headStyle(band layout.Rect, cell bool, fit float64) model.TextStyle {
	mm := band.H / 2.6
	if cell {
		mm = min(mm, band.W/9)
	}
	st := model.PlainStyle()
	st.Size = math.Round(geom.ClampF(mm/0.3528, 12, 48))
	if cell && fit > 0 {
		st.Size = fit
	}
	return model.ClampStyle(st)
}

// putHead ставит рамку шапки в полосу band (угол и ширина полосы); текст и стиль не
// трогаются (031 §5, шаг 6). Кегль новой шапки — от высоты полосы (две строки), поэтому
// текст полосу и заполняет; правленый текст «Применить» не сдвигает.
func putHead(t *model.TextBlock, band layout.Rect) {
	t.X, t.Y, t.W, t.Auto = geom.Round2(band.X), geom.Round2(band.Y), geom.Round2(band.W), false
}

// Relay — пересборка нетронутых виньеток при смене настроек листа или стиля подписей
// (сценарий 9): шаг конвейера Editor.edit после Rescale и Reflow, иначе пересобранные места
// масштабировались бы второй раз. Нетронутая по before (Untouched) — места по новому плану,
// ID прежние, тем же шагом отмены; тронутая остаётся как есть («!» покажет подпись). Удалённую
// шапку пересборка не создаёт — только «Применить».
func Relay(p, before *model.Project, plan VigPlanner) {
	settings := p.Settings != before.Settings
	for _, s := range p.Spreads {
		if !s.IsVignette() {
			continue
		}
		_, old := before.Find(s.ID)
		if old == nil || !settings && sameCaps(old.Caps, s.Caps) || !Untouched(before, old, plan) {
			continue
		}
		_, _ = placeVignette(p, s, *s.Vig, plan(p, s, *s.Vig), false, false) // ошибка — места как есть
	}
}

func sameCaps(a, b *model.SheetCaps) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

// Untouched — места виньетки s проекта before совпадают с её планом: число, порядок, роли,
// рамки ±vigTol (план — после того же зажима, что у мест), без поворота.
func Untouched(before *model.Project, s *model.Spread, plan VigPlanner) bool {
	if !s.IsVignette() || len(s.Grid) == 0 { // «Авто» (вернули руками) — уже не план
		return false
	}
	pl := plan(before, s, *s.Vig)
	if len(pl.Slots) != len(s.Grid) {
		return false
	}
	near := func(a, b float64) bool { return math.Abs(a-b) <= vigTol }
	for i, sl := range pl.Slots {
		g := s.Grid[i]
		w := model.Place{X: sl.Box.X, Y: sl.Box.Y, W: sl.Box.W, H: sl.Box.H}
		before.ClampPlace(s, &w)
		if model.RoleOf(g) != sl.Role || g.Rot != 0 || !near(g.X, w.X) || !near(g.Y, w.Y) || !near(g.W, w.W) || !near(g.H, w.H) {
			return false
		}
	}
	return true
}

// ResetVignette — «Вернуть авто» (038, решение 14): места снова по плану текущего шаблона и
// чисел, ID, фото, подписи и стиль остаются; шапка встаёт в свою полосу, удалённую не
// возвращает (как Relay). Нетронутая виньетка и не виньетка — холостая правка.
func ResetVignette(p *model.Project, spreadID string, plan VigPlanner) (int, error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return 0, errVigSpread
	}
	if !s.IsVignette() {
		return 0, nil
	}
	return placeVignette(p, s, *s.Vig, plan(p, s, *s.Vig), false, false)
}

// GrowVignette — нетронутая виньетка s принимает k новых учеников (042, решение 47): «Учеников»
// += k, места заново по плану того же шаблона (свой шаблон, оформление и шапка те же — как
// Relay), ID и фото по ролям прежние, новые места учеников пустые, в конце блока учеников.
// Сверх MaxPupils или MaxPlaces — ошибка до любых изменений. Не виньетка, k ≤ 0 — ничего.
func GrowVignette(p *model.Project, s *model.Spread, k int, plan VigPlanner) error {
	if !s.IsVignette() || k <= 0 {
		return nil
	}
	v := *s.Vig
	v.Pupils += k
	if v.Pupils > model.MaxPupils || model.VigPlaces(v) > model.MaxPlaces {
		return errVigLimit()
	}
	_, err := placeVignette(p, s, v, plan(p, s, v), false, false)
	return err
}

// ClearVignette — «Обычный разворот»: вид и параметры снимаются, роли мест — "";
// места, фото, стиль подписей и шапка остаются.
func ClearVignette(p *model.Project, spreadID string) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	s.Kind, s.Vig = "", nil
	for _, pl := range s.Grid {
		pl.Role = ""
	}
}

// SortVignette — «Сортировать по фамилии»: фото занятых мест учеников по (Фамилия, Имя) без
// регистра, «ё» как «е», по кодовым точкам; без имени — в конце в прежнем порядке. Учителя,
// групповое и пустые места не двигаются.
func SortVignette(p *model.Project, spreadID string) {
	_, s := p.Find(spreadID)
	if s == nil || !s.IsVignette() {
		return
	}
	type seat struct {
		photo       string
		named       bool
		last, first string
	}
	key := func(v string) string { return strings.ReplaceAll(strings.ToLower(v), "ё", "е") }
	var places []*model.Place
	var seats []seat
	for _, pl := range s.Grid {
		if pl.Photo == "" || model.RoleOf(pl) != model.RolePupil {
			continue
		}
		st := seat{photo: pl.Photo}
		if who, _ := p.Photos[pl.Photo].Name(); who != nil {
			st = seat{pl.Photo, true, key(who.Last), key(who.First)}
		}
		places, seats = append(places, pl), append(seats, st)
	}
	slices.SortStableFunc(seats, func(a, b seat) int {
		if a.named != b.named {
			if a.named {
				return -1
			}
			return 1
		}
		return cmp.Or(strings.Compare(a.last, b.last), strings.Compare(a.first, b.first))
	})
	for i, pl := range places {
		pl.Photo = seats[i].photo
	}
	s.Photos = model.GridPhotos(s)
}

// AddVignette — новый разворот-виньетка по умолчанию на позиции at (-1 — в конец).
func AddVignette(p *model.Project, at int, plan VigPlanner) error {
	s := &model.Spread{ID: model.NewID(), Photos: []string{}}
	if at < 0 || at > len(p.Spreads) {
		at = len(p.Spreads)
	}
	p.Spreads = slices.Insert(p.Spreads, at, s)
	_, err := SetVignette(p, s.ID, model.DefaultVignette(), plan, nil, nil)
	return err
}
