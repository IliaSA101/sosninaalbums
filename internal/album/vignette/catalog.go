package vignette

// Положение зоны в шаблоне.
const (
	PosNone   = ""
	PosTop    = "top"
	PosBottom = "bottom"
	PosLeft   = "left"
	PosRight  = "right"
	PosInline = "inline" // учителя первыми в общей сетке того же размера
	PosPage   = "page"   // вся левая страница разворота (шаблоны Split)
)

// Tpl — шаблон виньетки: сочетание зон одного генератора (031, развилка 3).
type Tpl struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Head  string `json:"head"`  // "" | top | bottom
	Teach string `json:"teach"` // inline | left | right | top | bottom | page
	Group string `json:"group"` // "" | top | bottom | page
	Split bool   `json:"split"` // только разворот: ученики на правой странице
}

// DefaultTpl — «Шапка, учителя слева» (031, сценарий 1).
const DefaultTpl = "head-t-left"

// Catalog — шаблоны в порядке галереи; sheet — лист: без шаблонов Split (17).
func Catalog(sheet bool) []Tpl {
	all := []Tpl{
		{"grid", "Сетка", PosNone, PosInline, PosNone, false},
		{"head", "Шапка и сетка", PosTop, PosInline, PosNone, false},
		{"t-left", "Учителя слева", PosNone, PosLeft, PosNone, false},
		{DefaultTpl, "Шапка, учителя слева", PosTop, PosLeft, PosNone, false},
		{"t-right", "Учителя справа", PosNone, PosRight, PosNone, false},
		{"head-t-right", "Шапка, учителя справа", PosTop, PosRight, PosNone, false},
		{"t-top", "Учителя сверху", PosNone, PosTop, PosNone, false},
		{"head-t-top", "Шапка, учителя сверху", PosTop, PosTop, PosNone, false},
		{"t-bottom", "Учителя снизу", PosNone, PosBottom, PosNone, false},
		{"head-t-bottom", "Шапка, учителя снизу", PosTop, PosBottom, PosNone, false},
		{"head-low", "Сетка, шапка снизу", PosBottom, PosInline, PosNone, false},
		{"g-top", "Групповое сверху", PosNone, PosInline, PosTop, false},
		{"head-g-top", "Шапка, групповое сверху", PosTop, PosInline, PosTop, false},
		{"g-bottom", "Групповое снизу", PosNone, PosInline, PosBottom, false},
		{"head-g-bottom", "Шапка, групповое снизу", PosTop, PosInline, PosBottom, false},
		{"head-t-left-g", "Шапка, учителя слева, групповое снизу", PosTop, PosLeft, PosBottom, false},
		{"t-top-g", "Учителя сверху, групповое снизу", PosNone, PosTop, PosBottom, false},
		{"split-t", "Учителя и шапка слева, класс справа", PosTop, PosPage, PosNone, true},
		{"split-g", "Групповое слева, класс справа", PosTop, PosInline, PosPage, true},
		{"split-tg", "Шапка, учителя и групповое слева", PosTop, PosPage, PosPage, true},
	}
	if !sheet {
		return all
	}
	out := all[:0]
	for _, t := range all {
		if !t.Split {
			out = append(out, t)
		}
	}
	return out
}

// Find — шаблон по ID; чужой ID — умолчание (файл мог прийти из новой версии).
func Find(id string) Tpl {
	var def Tpl
	for _, t := range Catalog(false) {
		if t.ID == id {
			return t
		}
		if t.ID == DefaultTpl {
			def = t
		}
	}
	return def
}

// adapt — шаблон Split на листе (одна область): левая страница становится зонами того же
// листа — учителя слева, групповое сверху (с учителями — снизу).
func adapt(t Tpl, regions int) Tpl {
	if regions >= 2 {
		return t
	}
	if t.Teach == PosPage {
		t.Teach = PosLeft
	}
	if t.Group == PosPage {
		t.Group = PosTop
		if t.Teach == PosLeft {
			t.Group = PosBottom
		}
	}
	t.Split = false
	return t
}
