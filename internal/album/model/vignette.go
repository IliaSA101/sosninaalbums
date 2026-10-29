package model

import (
	"log"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

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
