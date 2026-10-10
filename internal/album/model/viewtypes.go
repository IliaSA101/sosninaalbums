package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

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

// Choice — вариант раскладки для галереи.
type Choice struct {
	Key   string        `json:"key"`
	Rects []layout.Rect `json:"rects"` // весь разворот, в порядке Spread.Photos
}

type ChoiceRows struct {
	InOrder    []Choice `json:"inOrder"`
	Rearranged []Choice `json:"rearranged"`
	Block      string   `json:"block"` // почему ряд с перестановкой пуст: "same" | "many" | "" (нет лучших)
}

// Choices — всё для галереи раскладок одного разворота. В View не кладётся:
// на сотне разворотов это мегабайты на каждое событие.
type Choices struct {
	Spread string      `json:"spread"`
	Rev    int         `json:"rev"` // для какого rev посчитано
	Whole  ChoiceRows  `json:"whole"`
	Left   *ChoiceRows `json:"left"` // nil — у раскладки нет страниц или страница пуста
	Right  *ChoiceRows `json:"right"`
}

// MaxSpreadPhotos — больше фото на развороте не собираем: совпадает с полным
// перебором деревьев в layout и верхом Group. Одна константа на AutoBuild и MergeSpreads.
const MaxSpreadPhotos = 7

// PhotoInfo — что редактору нужно знать о добавляемом фото.
type PhotoInfo struct {
	ID   string
	Path string
	W, H int
	Size int64    // байты файла; 0 — неизвестен
	Xmp  XmpMarks // отметки из файла; заполняет App (их ждёт SetMarks)
}

// PlaceBox — новое положение и размер места, мм разворота.
type PlaceBox struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
}

// GridTarget — куда ставить сетку: разворот и область (spread | left | right).
type GridTarget struct {
	Spread string `json:"spread"`
	Scope  string `json:"scope"`
}

// GridPlan — итог копирования заранее (сухой прогон): вопрос до правки, чтобы
// один клик «на все развороты» не заморозил альбом и не выкинул сотни фото.
type GridPlan struct {
	Frozen int `json:"frozen"` // «Авто» станут сеткой (с источником)
	Left   int `json:"left"`   // фото уйдут в ленту
	Texts  int `json:"texts"`  // блоков текста заменится (с design)
}

// PathPair — путь фото до и после перепривязки. BgOnly — фото стоит только на фоне:
// его своя папка — не папка съёмки, папки проекта по нему не двигаются и не добавляются.
type PathPair struct {
	Old, New string
	BgOnly   bool
}

// Области сетки: страница или весь разворот (scope из интерфейса).
const (
	ScopeLeft   = "left"
	ScopeRight  = "right"
	ScopeSpread = "spread"
	// Области шаблона: доли страницы или разворота.
	TplPage   = "page"
	TplSpread = "spread"
)

// Box — место в долях области (шаблон, копирование) или в мм разворота (applyGrid):
// неповёрнутый прямоугольник и угол по часовой вокруг центра. Угол — внутри коробки,
// а не параллельным массивом: ClampBoxes выбрасывает и обрезает коробки, угол
// уходит со своей. Поля явные (не layout.Rect): один плоский тип в TS.
type Box struct {
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
	W   float64 `json:"w"`
	H   float64 `json:"h"`
	Rot float64 `json:"rot,omitempty"`
}

// GridChoice — одна карточка галереи «Сетки».
type GridChoice struct {
	ID    string `json:"id"`    // шаблон: его ID; встроенная: ключ раскладки
	Name  string `json:"name"`  // "" у встроенных
	Scope string `json:"scope"` // page | spread
	Boxes []Box  `json:"boxes"` // доли области, порядок = порядок мест; rot — у шаблонов
	Drop  int    `json:"drop"`  // сколько фото уйдёт в ленту при применении на scope: значок «−N»
}

// GridChoices — содержимое вкладки «Сетки» для n мест.
type GridChoices struct {
	N       int          `json:"n"`
	Builtin []GridChoice `json:"builtin"` // «Из раскладок»
	Mine    []GridChoice `json:"mine"`    // «Мои шаблоны» с числом мест n (7 — от 7)
}

// SpreadView. Z мест/ячеек, подписей и текстов уникальны в развороте и задают порядок
// наложения: на этом строится стек под курсором во фронте (спека 015, TestViewZUnique).
type SpreadView struct {
	ID         string        `json:"id"`
	Photos     []string      `json:"photos"`
	Rects      []layout.Rect `json:"rects"`      // по действующему ключу
	Layout     string        `json:"layout"`     // действующий ключ (не пустой, если есть фото)
	Variant    int           `json:"variant"`    // позиция в ряду листания; -1 — нет в ряду
	Variants   int           `json:"variants"`   // длина этого ряда
	Rearranged bool          `json:"rearranged"` // ряд листания — «С перестановкой порядка»
	Pages      []PageView    `json:"pages"`      // nil — у раскладки нет страниц (не layout.IsPaged); иначе [левая, правая]
	// Frames — кадр каждого фото, по одному на Rects (пусто, если раскладки нет).
	Frames   []FrameView   `json:"frames"`
	Texts    []TextView    `json:"texts"`
	Captions []CaptionView `json:"captions"`
	Bg       [2]string     `json:"bg"`    // действующий цвет левой и правой страницы
	BgSet    [2]string     `json:"bgSet"` // BgL, BgR как заданы ("" — наследуется)
	// Grid — режим «Сетка»: Photos/Rects/Frames — из мест, Layout = "", Variant = -1,
	// Pages = nil. Places — все места по порядку слоёв (nil в «Авто»).
	Grid   bool        `json:"grid"`
	Places []PlaceView `json:"places"`
	Name   string      `json:"name"` // название разворота; "" — без названия
	// Backdrops — фото на фоне (0–2): весь разворот, левая, правая. В Photos их нет.
	Backdrops []BackdropView `json:"backdrops"`
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
	Scope string      `json:"scope"` // spread | left | right
	Photo string      `json:"photo"`
	Area  layout.Rect `json:"area"` // область, мм разворота
	Box   BackdropBox `json:"box"`
	Img   layout.Rect `json:"img"`
	Frame Frame       `json:"frame"`
	FlipH bool        `json:"flipH"`
	FlipV bool        `json:"flipV"`
	DPI   int         `json:"dpi"`  // 0 — размер фото неизвестен
	Fade  float64     `json:"fade"` // прозрачность 0..1, всегда есть
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
	// CellRot — угол ячейки (= места, вокруг его центра; в «Авто» 0): Rects и Img
	// неповёрнутые, UI крутит ячейку целиком, не ища место.
	CellRot float64 `json:"cellRot"`
	// Z — слой ячейки в общем стеке разворота (zView), как у её места.
	Z int `json:"z"`
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
	// Z — слой в общем стеке разворота (zView); у подписи — сразу над её фото.
	Z int `json:"z"`
}

// CaptionView — подпись к фото: рамка — место подписи, ID пуст.
type CaptionView struct {
	Photo string `json:"photo"`
	TextView
	Over     bool    `json:"over"`
	Overflow float64 `json:"overflow"` // мм, которых не хватает; 0 — помещается
}

type View struct {
	// Product — тип изделия: по нему интерфейс выбирает режимы (в 0.5.0 только album).
	Product  string       `json:"product"`
	Spreads  []SpreadView `json:"spreads"`
	Settings Settings     `json:"settings"`
	Folders  []string     `json:"folders"`
	Path     string       `json:"path"`
	Dirty    bool         `json:"dirty"`
	CanUndo  bool         `json:"canUndo"`
	CanRedo  bool         `json:"canRedo"`
	Rev      int          `json:"rev"`
	// MaxSpreadPhotos — интерфейс по нему заранее отключает склейку.
	MaxSpreadPhotos int `json:"maxSpreadPhotos"`
	// Safe — безопасная зона (мм, в координатах разворота): значимое не ближе SafeMargin к краю.
	// Считается всегда; показывать или нет, решает интерфейс.
	Safe []layout.Rect `json:"safe"`
	// MinDPI — порог «!» у фото: min(lowDPILimit, settings.dpi). LowDPI — сколько фото ниже него.
	MinDPI int `json:"minDPI"`
	LowDPI int `json:"lowDPI"`
	// EmptyPlaces — пустых мест сетки во всём альбоме.
	EmptyPlaces int `json:"emptyPlaces"`
	// TextMaxW — предел ширины блока «по тексту», мм (страница без безопасных полей).
	TextMaxW float64 `json:"textMaxW"`
	// BackdropUsed — ID фото, стоящих на фонах (по возрастанию): автосборка,
	// «Заполнить пустые» и фильтр «Неиспользованные» их не берут.
	BackdropUsed []string `json:"backdropUsed"`
	// MarksRev — номер состояния своих отметок: сменился — UI перечитывает Marks (сами отметки
	// в View не кладутся: на тысячах фото это сотни КБ на каждое событие).
	MarksRev int `json:"marksRev"`
}

// SafeMargin — отступ безопасной зоны от края, мм (требование fsfera: значимое не ближе 8 мм).
const SafeMargin = 8.0

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
}
