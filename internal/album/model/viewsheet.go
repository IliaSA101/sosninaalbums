package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Виды элементов листа для интерфейса и экспорта: места и рамочки, кадры фото, фоны, тексты,
// подписи и заглушки (вынесены из viewtypes.go: там — вид альбома, галереи и планы правок).

// PlaceView — место сетки для интерфейса, мм разворота. Порядок в
// SpreadView.Places = порядок мест в слоях; Z — слой в общем стеке с текстами:
// и пустое место, и ячейка с фото получают его, бросок попадает в верхнее.
type PlaceView struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Photo string  `json:"photo"` // "" — пусто
	Fit   string  `json:"fit"`
	Spine bool    `json:"spine"` // пересекает корешок при выключенном layflat
	Rot   float64 `json:"rot"`   // градусы по часовой вокруг центра; x, y, w, h — до поворота
	Z     int     `json:"z"`     // слой в общем стеке разворота (zView)
	Role  string  `json:"role"`  // роль места виньетки (031): pupil | teacher | group; "" — ученик
	// Shape, Round — форма места (044): "" | "oval" и скругление, % меньшей стороны;
	// Radius — оно же в мм (считает Go, UI только рисует).
	Shape  string  `json:"shape"`
	Round  float64 `json:"round"`
	Radius float64 `json:"radius"`
	// Border — рамочка места (079), и у пустого; nil — нет.
	Border *BorderView `json:"border,omitempty"`
}

// BorderView — рамочка места для холста и экспорта (079 design §7). Kind, Photo, Lib, Preset — как
// в альбоме; Name — имя вида (своей UI берёт имя из каталога по Lib). Inks — действующие цвета (с
// цветами вида), Width — действующая толщина (встроенная), Scale — размер картинки (1 — как
// задумано). DefWidth — толщина вида по умолчанию (двойной щелчок по «Толщине», 079-36; у картинки
// и неизвестного вида 0 — не пишется). Ins — отступы окна, мм (UI держит их при живом изменении
// размера места). URL и MaskURL — без px (UI дописывает &px=); URL "" — накладки нет (мягкий край,
// неизвестный вид), MaskURL "" — маски нет. Known = false — вида нет в этой сборке (файл новее):
// окно = место.
type BorderView struct {
	Kind     string     `json:"kind"`
	Photo    string     `json:"photo"`
	Lib      string     `json:"lib"`
	Preset   string     `json:"preset"`
	Name     string     `json:"name"`
	Inks     []string   `json:"inks"`
	Width    float64    `json:"width"`
	DefWidth float64    `json:"defWidth,omitempty"`
	Scale    float64    `json:"scale"`
	Ins      [4]float64 `json:"ins"`
	URL      string     `json:"url"`
	MaskURL  string     `json:"maskUrl"`
	Pic      bool       `json:"pic"`
	Open     bool       `json:"open"`
	Known    bool       `json:"known"`
	// File — файл картинки для печати (079-40): только в виде печати (Editor.Snapshot), копия из
	// «Рамочек», а не путь общей с фоном записи. Интерфейсу не нужен.
	File string `json:"-"`
}

// StubView — заглушка подписи: серый текст «[Фамилия]» под местом; выделить нельзя.
// Place — место сетки ("" в «Авто»), Photo — фото без имени ("" у пустого места).
type StubView struct {
	Place string `json:"place"`
	Photo string `json:"photo"`
	TextView
	// Who — поля имени шаблона по порядку (last, first, middle, subject) у заглушки фото (037)
	// и пустого места (071: окошко «Имя» пишет табличку места).
	Who []string `json:"who,omitempty"`
}

// BackdropBox — рамка фона: угол до поворота и поворот вокруг него, мм разворота.
type BackdropBox struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	W   float64 `json:"w"`
	H   float64 `json:"h"`
	Rot float64 `json:"rot"`
}

// BackdropView — фото на фоне для интерфейса и экспорта: Img — фото целиком в осях
// рамки (от её левого верхнего угла до поворота); зеркало внутри Img.
type BackdropView struct {
	Scope string      `json:"scope"` // spread | left | right | spine (корешок обложки, 061)
	Photo string      `json:"photo"`
	Area  layout.Rect `json:"area"` // область, мм разворота
	Box   BackdropBox `json:"box"`
	Img   layout.Rect `json:"img"`
	Frame Frame       `json:"frame"` // R — наклон фото в рамке ±44,9 (045), четвертей нет
	FlipH bool        `json:"flipH"`
	FlipV bool        `json:"flipV"`
	DPI   int         `json:"dpi"`  // 0 — размер фото неизвестен
	Fade  float64     `json:"fade"` // прозрачность 0..1, всегда есть
	// Lib — ключ своего из библиотеки (048): имя UI берёт из каталога; "" — обычное фото.
	Lib string `json:"lib,omitempty"`
}

type PageView struct {
	Photos     int  `json:"photos"`  // фото на странице; ряд страницы есть при ≥ 1
	Variant    int  `json:"variant"` // позиция блока в ряду страницы; -1 — нет
	Variants   int  `json:"variants"`
	Rearranged bool `json:"rearranged"`
}

// FrameView — фото целиком в мм разворота; ячейка (Rects) его обрезает.
type FrameView struct {
	Img layout.Rect `json:"img"`
	S   float64     `json:"s"`
	X   float64     `json:"x"`
	Y   float64     `json:"y"`
	// R — поворот фото в рамке (градусы по часовой вокруг центра Img): для рисования
	// и placeImage; Turn — четверти 0…3, Tilt — наклон −44,9…44,9. UI на 90 не делит.
	R    float64 `json:"r"`
	Turn int     `json:"turn"`
	Tilt float64 `json:"tilt"`
	DPI  int     `json:"dpi"` // эффективный; 0 — размер фото неизвестен
	// FlipH/FlipV — зеркало картинки внутри Img (геометрия не меняется).
	FlipH bool `json:"flipH"`
	FlipV bool `json:"flipV"`
	// Gray, Bright, Contrast — тон фото (068) для карточки; Filter — готовая строка CSS
	// filter для <img> (Tone.CSS), "" — без тона.
	Gray     bool   `json:"gray"`
	Bright   int    `json:"bright"`
	Contrast int    `json:"contrast"`
	Filter   string `json:"filter"`
	// CellRot — угол ячейки (= места, вокруг его центра; в «Авто» 0): Rects и Img
	// неповёрнутые, UI крутит ячейку целиком, не ища место.
	CellRot float64 `json:"cellRot"`
	// Z — слой ячейки в общем стеке разворота (zView), как у её места.
	Z int `json:"z"`
	// Shape, Radius — форма места (044): "" | "oval" и радиус скругления, мм; у «Авто»
	// "" и 0. Экспорт и Cell обрезают ими фото.
	Shape  string  `json:"shape"`
	Radius float64 `json:"radius"`
	// Win — окно рамочки места (079), мм разворота до поворота: Img посчитан для него, клип фото —
	// по нему с формой WinShape/WinRadius; без рамочки — сама ячейка и её форма. Fade —
	// прозрачность фото 0…1 (PhotoRef.Fade).
	Win       layout.Rect `json:"win"`
	WinShape  string      `json:"winShape"`
	WinRadius float64     `json:"winRadius"`
	Fade      float64     `json:"fade"`
}

// TextView — блок текста: рамка и строки в мм разворота (у строки x — левый
// край, y — базовая линия). Интерфейс только рисует, переносы считает Go.
// X, Y — угол рамки до поворота; строки без поворота, весь блок поворачивается
// на Rot вокруг (X, Y). У подписей Rot = 0, Auto = false.
type TextView struct {
	ID    string      `json:"id"`
	X     float64     `json:"x"`
	Y     float64     `json:"y"`
	W     float64     `json:"w"`
	H     float64     `json:"h"`
	Text  string      `json:"text"`
	Style TextStyle   `json:"style"`
	Lines []text.Line `json:"lines"`
	Rot   float64     `json:"rot"`  // градусы по часовой
	Auto  bool        `json:"auto"` // ширина по тексту
	// Src — исходный текст с полями ({Школа}…), если он отличается от Text (030):
	// поле правки показывает его, холст — Text.
	Src string `json:"src,omitempty"`
	// Z — слой в общем стеке разворота (zView); у подписи — сразу над её фото.
	Z int `json:"z"`
}

// CaptionView — подпись к фото: рамка — место подписи, ID пуст.
type CaptionView struct {
	Photo string `json:"photo"`
	TextView
	Over     bool    `json:"over"`
	Overflow float64 `json:"overflow"` // мм, которых не хватает; 0 — помещается
	// Sheet — оформление от общего стиля листа (Spread.Caps), а не своё (030).
	Sheet bool `json:"sheet"`
	// Pad — действующий отступ от фото, мм: свой, по умолчанию 1, у подписи под стилем
	// листа — из Spread.Caps. UI шлёт его в каждой правке стиля, чтобы не сбросить.
	Pad float64 `json:"pad"`
	// Who — поля имени шаблона листа по порядку (last, first, middle, subject), если подпись идёт
	// по шаблону (037); своя подпись и шаблон без имени — пусто.
	Who []string `json:"who,omitempty"`
}

// Shown — видимая часть фона на развороте: описанный прямоугольник повёрнутой рамки,
// обрезанный областью (за область фон не рисуется). ok=false — рамка вся за областью.
func (b BackdropView) Shown() (r layout.Rect, ok bool) {
	box := geom.TextBox{X: b.Box.X, Y: b.Box.Y, W: b.Box.W, H: b.Box.H, Rot: b.Box.Rot}.AABB()
	x0, y0 := max(box.X, b.Area.X), max(box.Y, b.Area.Y)
	x1, y1 := min(box.X+box.W, b.Area.X+b.Area.W), min(box.Y+box.H, b.Area.Y+b.Area.H)
	if x1 <= x0 || y1 <= y0 {
		return layout.Rect{}, false
	}
	return layout.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, true
}
