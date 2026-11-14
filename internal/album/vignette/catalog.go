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
	PosCenter = "center" // Teach: учителя в середине нижнего ряда (049)
	PosLead   = "lead"   // Teach: первый учитель крупно, остальные — первыми в общей сетке (049)
	PosCell   = "cell"   // Head: в свободных ячейках последнего ряда (049)
)

// Виды шаблонов для фильтра окна и галереи (049 §1.4): у каждого шаблона ровно один.
const (
	FamSide  = "side"  // учителя у края листа
	FamBig   = "big"   // учитель крупно
	FamInrow = "inrow" // учителя в общих рядах или в центре ряда
	FamGroup = "group" // с групповым фото
	FamRound = "round" // круглые портреты шахматкой
	FamStaff = "staff" // руководитель и учителя-предметники
)

// StaffText — подпись учителя у шаблонов «Руководитель…»; та же строка, что model.StaffCaption
// (пакет ниже модели её не видит; сверяет тест G3).
const StaffText = "{Фамилия}\n{Имя} {Отчество}\n{Предмет}"

// Tpl — шаблон виньетки: сочетание зон одного генератора (031, развилка 3). Каталог в файл не
// пишется, в .salbum — только ID.
type Tpl struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Head  string `json:"head"`  // "" | top | bottom | cell
	Teach string `json:"teach"` // inline | left | right | top | bottom | page | center | lead
	Group string `json:"group"` // "" | top | bottom | page
	Split bool   `json:"split"` // только разворот: ученики (у lead — остальные учителя) на правой странице
	// Big — учитель крупно: множитель teachBig вместо teachMore, доли зоны bigParts (049).
	Big     bool   `json:"big,omitempty"`
	Stagger bool   `json:"stagger,omitempty"` // ученики шахматкой (049 §1.3.5)
	Family  string `json:"family"`            // вид для фильтра: Fam*
	TText   string `json:"ttext,omitempty"`   // подпись учителя, которую приносит смена шаблона
}

// DefaultTpl — «Шапка, учителя слева» (031, сценарий 1).
const DefaultTpl = "head-t-left"

// Catalog — шаблоны в порядке галереи; sheet — лист: без шаблонов Split (29 из 33). Первые 20
// (031) — те же ID, имена и порядок: ID лежат в файлах, порядок помнят руки.
func Catalog(sheet bool) []Tpl {
	all := []Tpl{
		{ID: "grid", Name: "Сетка", Teach: PosInline, Family: FamInrow},
		{ID: "head", Name: "Шапка и сетка", Head: PosTop, Teach: PosInline, Family: FamInrow},
		{ID: "t-left", Name: "Учителя слева", Teach: PosLeft, Family: FamSide},
		{ID: DefaultTpl, Name: "Шапка, учителя слева", Head: PosTop, Teach: PosLeft, Family: FamSide},
		{ID: "t-right", Name: "Учителя справа", Teach: PosRight, Family: FamSide},
		{ID: "head-t-right", Name: "Шапка, учителя справа", Head: PosTop, Teach: PosRight, Family: FamSide},
		{ID: "t-top", Name: "Учителя сверху", Teach: PosTop, Family: FamSide},
		{ID: "head-t-top", Name: "Шапка, учителя сверху", Head: PosTop, Teach: PosTop, Family: FamSide},
		{ID: "t-bottom", Name: "Учителя снизу", Teach: PosBottom, Family: FamSide},
		{ID: "head-t-bottom", Name: "Шапка, учителя снизу", Head: PosTop, Teach: PosBottom, Family: FamSide},
		{ID: "head-low", Name: "Сетка, шапка снизу", Head: PosBottom, Teach: PosInline, Family: FamInrow},
		{ID: "g-top", Name: "Групповое сверху", Teach: PosInline, Group: PosTop, Family: FamGroup},
		{ID: "head-g-top", Name: "Шапка, групповое сверху", Head: PosTop, Teach: PosInline, Group: PosTop, Family: FamGroup},
		{ID: "g-bottom", Name: "Групповое снизу", Teach: PosInline, Group: PosBottom, Family: FamGroup},
		{ID: "head-g-bottom", Name: "Шапка, групповое снизу", Head: PosTop, Teach: PosInline, Group: PosBottom, Family: FamGroup},
		{ID: "head-t-left-g", Name: "Шапка, учителя слева, групповое снизу", Head: PosTop, Teach: PosLeft, Group: PosBottom,
			Family: FamGroup},
		{ID: "t-top-g", Name: "Учителя сверху, групповое снизу", Teach: PosTop, Group: PosBottom, Family: FamGroup},
		{ID: "split-t", Name: "Учителя и шапка слева, класс справа", Head: PosTop, Teach: PosPage, Split: true, Family: FamSide},
		{ID: "split-g", Name: "Групповое слева, класс справа", Head: PosTop, Teach: PosInline, Group: PosPage, Split: true,
			Family: FamGroup},
		{ID: "split-tg", Name: "Шапка, учителя и групповое слева", Head: PosTop, Teach: PosPage, Group: PosPage, Split: true,
			Family: FamGroup},
		// 049: ID заморожены так же.
		{ID: "t-left-big", Name: "Учитель крупно слева", Teach: PosLeft, Big: true, Family: FamBig},
		{ID: "head-t-left-big", Name: "Шапка, учитель крупно слева", Head: PosTop, Teach: PosLeft, Big: true, Family: FamBig},
		{ID: "t-left-big-cell", Name: "Учитель крупно слева, шапка в ряду", Head: PosCell, Teach: PosLeft, Big: true,
			Family: FamBig},
		{ID: "t-center", Name: "Учитель в центре", Teach: PosCenter, Family: FamInrow},
		{ID: "head-t-center", Name: "Шапка, учитель в центре", Head: PosTop, Teach: PosCenter, Family: FamInrow},
		{ID: "cell-head", Name: "Сетка, шапка в ряду", Head: PosCell, Teach: PosInline, Family: FamInrow},
		{ID: "t-left-cell", Name: "Учителя слева, шапка в ряду", Head: PosCell, Teach: PosLeft, Family: FamSide},
		{ID: "lead-left", Name: "Руководитель крупно, учителя сеткой", Teach: PosLead, Family: FamStaff, TText: StaffText},
		{ID: "head-lead-left", Name: "Шапка, руководитель крупно", Head: PosTop, Teach: PosLead, Family: FamStaff,
			TText: StaffText},
		{ID: "split-lead", Name: "Руководитель слева, учителя справа", Head: PosTop, Teach: PosLead, Split: true,
			Family: FamStaff, TText: StaffText},
		// 056 №5, №7: имена «…россыпью», формы нет — круги или иную форму выбирает плитка.
		{ID: "round", Name: "Россыпью", Teach: PosInline, Stagger: true, Family: FamRound},
		{ID: "head-round", Name: "Шапка, россыпью", Head: PosTop, Teach: PosInline, Stagger: true, Family: FamRound},
		{ID: "head-t-left-round", Name: "Шапка, учителя слева, россыпью", Head: PosTop, Teach: PosLeft, Stagger: true,
			Family: FamRound},
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
// листа — учителя слева, групповое сверху (с учителями — снизу); руководитель — зона слева
// долями bigParts, как у lead-left.
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
