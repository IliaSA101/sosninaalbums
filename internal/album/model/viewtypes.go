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
	// Family — сегмент фильтра по форме: dense | hero | air | bleed (041), strip | figure (051);
	// "" — не размечено.
	Family string `json:"family"`
	Twin   int    `json:"twin"` // индекс более раннего с тем же видом в этом ряду; −1 — нет
	// FormName — название формы для подсказки плитки («Лента», «Лесенка»…); "" у мозаики (051).
	FormName string `json:"formName"`
}

type ChoiceRows struct {
	InOrder    []Choice `json:"inOrder"`
	Rearranged []Choice `json:"rearranged"`
	Block      string   `json:"block"` // почему ряд с перестановкой пуст: "same" | "many" | "" (нет лучших)
	// More — ряд «Авто» на многих фото можно досчитать (MoreLayouts); у страниц всегда false.
	More bool `json:"more"`
	// Extra — «Другие варианты» (041): пул вне ряда «Авто»; только у разворота, у страниц пусто.
	Extra []Choice `json:"extra"`
}

// Variety — ручка «Разнообразие раскладок» (041). Нулевое значение — «Меньше», ровно
// поведение 0.10: вызов без поля безопасен. UI по умолчанию шлёт «Обычное».
type Variety int

const (
	VarietyLess   Variety = iota // лучшая раскладка каждому развороту, как 0.10
	VarietyNormal                // соседи не повторяются, иногда воздух и главное фото
	VarietyMore                  // чаще воздух, крупные фото и смена вида
)

// Clamp — уровень из UI в 0…2: чужое число не должно включать несуществующий режим.
func (v Variety) Clamp() Variety { return min(max(v, VarietyLess), VarietyMore) }

// AutoOpts — настройки окна автосборки (AutoPlan/AutoBuild). Новые настройки (группы по
// папкам) ложатся полями сюда: сигнатура методов App второй раз не меняется (041 №16).
// Нули новых полей (MinPer, Auto, Series) — план 0.12 побитово (старый фронт, тесты).
type AutoOpts struct {
	Spreads int     `json:"spreads"` // N разворотов; зажим — в плане
	MaxPer  int     `json:"maxPer"`  // «до»: не больше фото на разворот, 1…MaxPhotos
	MinPer  int     `json:"minPer"`  // «от» (051): 1…MaxPer, развороты нарочно разные; 0 — ровно, как 0.12
	Auto    bool    `json:"auto"`    // «Авто» (051): диапазон от среднего по N; MinPer, MaxPer не действуют
	Series  bool    `json:"series"`  // не разрывать серии съёмки (папки, время; 051); только с диапазоном
	Variety Variety `json:"variety"` // в AutoPlan не влияет: группы те же
}

// AutoPlan — подсказка окна автосборки: сколько фото и как они лягут на N разворотов.
// AutoBuild с теми же фото и AutoOpts делает ровно эти развороты.
type AutoPlan struct {
	Photos     int     `json:"photos"`     // фото в сборку (без сеток и фонов)
	Spreads    int     `json:"spreads"`    // N после зажима
	MinSpreads int     `json:"minSpreads"` // низ ползунка: ⌈photos/до⌉
	MaxSpreads int     `json:"maxSpreads"` // верх: min(⌊photos/от⌋ или photos, MaxSpreads − непустых; пустой хвост — свободен)
	Avg        float64 `json:"avg"`
	Min        int     `json:"min"`   // фото в самом маленьком развороте
	Max        int     `json:"max"`   // и в самом большом
	Reuse      int     `json:"reuse"` // сколько пустых разворотов из хвоста займёт сборка
	From       int     `json:"from"`  // действующее «от» (в «Авто» — посчитанное по N); 0 — ровно, как 0.12
	To         int     `json:"to"`    // действующее «до»
	Cuts       int     `json:"cuts"`  // границ серий силы ≥ 0.5; 0 — без Series или сигналов нет
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

// PhotoInfo — что редактору нужно знать о добавляемом фото.
type PhotoInfo struct {
	ID   string
	Path string
	W, H int
	Size int64    // байты файла; 0 — неизвестен
	Xmp  XmpMarks // отметки из файла; заполняет App (их ждёт SetMarks)
	// Lib — ключ своего из библиотеки my:<16 hex> (048); тогда ID = LibPhotoID(Lib).
	Lib string
	// Taken — время съёмки (EXIF), секунды Unix, местное время как UTC; 0 — нет. Только для серий автосборки, в .salbum не пишется.
	Taken int64
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
	Frozen   int `json:"frozen"`   // «Авто» станут сеткой (с источником)
	Left     int `json:"left"`     // фото уйдут в ленту
	Texts    int `json:"texts"`    // блоков текста заменится (с design; у оформления — с надписями)
	Stickers int `json:"stickers"` // стикеров цели заменится (с оформлением, 049 Ф2)
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
	// Shape, Round — форма места (044, Place.Shape/Round): шаблон, копия сетки и
	// «Мои шаблоны» переносят и рисуют её.
	Shape string  `json:"shape,omitempty"`
	Round float64 `json:"round,omitempty"`
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
	ID     string   `json:"id"`
	Photos []string `json:"photos"`
	// W, H, Mid — лист из Geo, мм (032 §6): UI считает проценты от них, а не от settings.
	// Разворот — 2·pageW × pageH, шов pageW; обложка — шов в центре корешка; лист 031 — Mid 0.
	W          float64       `json:"w"`
	H          float64       `json:"h"`
	Mid        float64       `json:"mid"`
	Rects      []layout.Rect `json:"rects"`      // по действующему ключу
	Layout     string        `json:"layout"`     // действующий ключ (не пустой, если есть фото)
	Variant    int           `json:"variant"`    // позиция в ряду листания; -1 — нет в ряду
	Variants   int           `json:"variants"`   // длина этого ряда
	More       bool          `json:"more"`       // ряд «Авто» можно досчитать (17+ фото): «+» в счётчике
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
	// Patterns — узоры на фоне (0–2, 048): весь лист, левая, правая; под фото фона.
	Patterns []PatternView `json:"patterns"`
	// Stickers — стикеры листа (048) в порядке Spread.Stickers; слой — Z в общем стеке.
	Stickers []StickerView `json:"stickers"`
	// LowDPI — кадров, фото фона и своих стикеров с 0 < dpi < View.MinDPI: «!» на плитке разворота.
	LowDPI int `json:"lowDPI"`
	// Stubs — заглушки подписей (030): пустое место листа с шаблоном и фото без имени.
	// Только холст: в виде печати пусто, в Captions их нет — экспорт их не видит.
	Stubs []StubView `json:"stubs"`
	// Caps — копия общего стиля подписей листа для панели; nil — у подписей свой стиль.
	Caps *SheetCaps `json:"caps,omitempty"`
	// Kind — вид разворота ("" | KindVignette); Vig — параметры виньетки для панели (031).
	Kind string        `json:"kind"`
	Vig  *VignetteView `json:"vig,omitempty"`
	// Vars — свои поля шапки листа (Spread.Vars) для формы окна виньетки; пустые — нет ключа.
	Vars Vars `json:"vars"`
	// Fields — имена для меню «Вставить поле» на этом листе (model.FieldsOf): 3 или 7.
	Fields []string `json:"fields"`
	// Unfilled, Unnamed — то же, что у View, но по одному листу: вопрос перед экспортом
	// считается по листам плана выгрузки (диапазон, «Только обложка»). Интерфейсу не нужны.
	Unfilled []string `json:"-"`
	Unnamed  int      `json:"-"`
}

// VignetteView — виньетка для панели: параметры, сколько мест они дают (с групповым)
// и предел на лист. Group — у шаблона есть групповое фото (UI считает «Всего мест»).
// Геометрию своего шаблона вид не таскает: Own всегда nil, вместо него OwnName (038).
type VignetteView struct {
	Vignette
	Places  int    `json:"places"`
	Max     int    `json:"max"`
	Group   bool   `json:"group"`
	OwnName string `json:"ownName,omitempty"`
	// TplName — имя текущего шаблона для строки раскладки: каталога или своего; Manual — места
	// отличаются от плана шаблона (сдвинуты руками, 038 №14: «Ручная · Вернуть авто»). Оба
	// ставит Editor при сборке вида (план знает он), в NewVignetteView их нет.
	TplName string `json:"tplName"`
	Manual  bool   `json:"manual"`
}

// NewVignetteView — вид параметров v для панели.
func NewVignetteView(v Vignette) VignetteView {
	vv := VignetteView{Vignette: v, Places: VigPlaces(v), Max: MaxPlaces, Group: VigGroup(v.Tpl)}
	if v.Own != nil {
		vv.OwnName, vv.Group, vv.Own = v.Own.Name, v.Own.Groups() > 0, nil
	}
	return vv
}

// VigSetup — форма окна «Настроить виньетку» (031 dialog §5): параметры и поля шапки листа.
type VigSetup struct {
	Vig  Vignette `json:"vig"`
	Vars Vars     `json:"vars"`
}

// VigPreview — сухой прогон окна (PreviewVignette): лист после «Применить» с тем же
// VigSetup (ID прежний) и сколько фото уйдёт в ленту.
type VigPreview struct {
	Spread SpreadView `json:"spread"`
	Left   int        `json:"left"`
}

// VigChoice — превью шаблона виньетки (VignetteChoices): места в долях листа и их роли,
// портрет ученика в мм, Small — мельче 15 мм, Over — мест больше MaxPlaces.
type VigChoice struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Boxes  []Box    `json:"boxes"`
	Roles  []string `json:"roles"`
	PupilW float64  `json:"pupilW"`
	Small  bool     `json:"small"`
	Over   bool     `json:"over"`
	// Own — свой шаблон (id "my:<id>"); Ratio — его портрет (у каталога — подсказка окну:
	// "1:1" у кругов, иначе ""); Gone — шаблона нет в templates.json, он только на этом листе:
	// выбрать можно, переименовать и удалить нет.
	Own   bool   `json:"own"`
	Ratio string `json:"ratio"`
	Gone  bool   `json:"gone"`
	// Shape, Round — форма мест людей (044): у своего шаблона — из его стиля (выбор ставит её
	// в окне, как Ratio), у удалённого — та, что на листе; у каталога — подсказка шаблона
	// ("oval" у кругов, 049: выбор в окне и SetVignette ставят Круг и 1:1). Коробки людей в Boxes — с
	// этой формой, групповое — прямоугольник.
	Shape string  `json:"shape,omitempty"`
	Round float64 `json:"round,omitempty"`
	// Family — вид каталожного шаблона для фильтра (side, big, inrow, group, round, staff; 049);
	// у своих "". Head — шапка в долях листа, как Boxes (полоса или ячейки ряда); nil — нет.
	Family string `json:"family"`
	Head   *Box   `json:"head,omitempty"`
}

// StubView — заглушка подписи: серый текст «[Фамилия]» под местом; выделить нельзя.
// Place — место сетки ("" в «Авто»), Photo — фото без имени ("" у пустого места).
type StubView struct {
	Place string `json:"place"`
	Photo string `json:"photo"`
	TextView
	// Who — поля имени шаблона по порядку (last, first, middle, subject) у заглушки фото (037);
	// у заглушки пустого места пусто.
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
	Scope string      `json:"scope"` // spread | left | right
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
	// CellRot — угол ячейки (= места, вокруг его центра; в «Авто» 0): Rects и Img
	// неповёрнутые, UI крутит ячейку целиком, не ища место.
	CellRot float64 `json:"cellRot"`
	// Z — слой ячейки в общем стеке разворота (zView), как у её места.
	Z int `json:"z"`
	// Shape, Radius — форма места (044): "" | "oval" и радиус скругления, мм; у «Авто»
	// "" и 0. Экспорт и Cell обрезают ими фото.
	Shape  string  `json:"shape"`
	Radius float64 `json:"radius"`
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
	// MaxSpreadPhotos — интерфейс по нему заранее отключает склейку (= MaxPhotos;
	// имя прежнее — контракт UI).
	MaxSpreadPhotos int `json:"maxSpreadPhotos"`
	// Safe — безопасная зона (мм, в координатах разворота): значимое не ближе Settings.Safe к краю.
	// Считается всегда; показывать или нет, решает интерфейс.
	Safe []layout.Rect `json:"safe"`
	// MinDPI — порог «!» у фото: min(lowDPILimit, settings.dpi). LowDPI — сколько фото ниже него
	// (кадры и фоны, сумма SpreadView.LowDPI).
	MinDPI int `json:"minDPI"`
	LowDPI int `json:"lowDPI"`
	// EmptyPlaces — пустых мест сетки во всём альбоме.
	EmptyPlaces int `json:"emptyPlaces"`
	// TextMaxW — предел ширины блока «по тексту», мм (страница без безопасных полей).
	TextMaxW float64 `json:"textMaxW"`
	// BackdropUsed — ID фото, стоящих на фонах и в своих стикерах (по возрастанию): автосборка,
	// «Заполнить пустые» и фильтр «Неиспользованные» их не берут.
	BackdropUsed []string `json:"backdropUsed"`
	// LibPhotos — ID записей Photos со своим из библиотеки (PhotoRef.Lib, 048), по возрастанию:
	// пропавшие среди них UI считает отдельно («Нет в библиотеке этого компьютера», без «Найти…»).
	LibPhotos []string `json:"libPhotos"`
	// MarksRev — номер состояния своих отметок: сменился — UI перечитывает Marks (сами отметки
	// в View не кладутся: на тысячах фото это сотни КБ на каждое событие).
	MarksRev int `json:"marksRev"`
	// Unfilled — пустые переменные и поля, встреченные в печатаемом тексте всех листов (по
	// алфавиту); Unnamed — фото, у подписи которых поля имени есть, а имени нет. По ним —
	// вопрос перед экспортом (export.FieldsWarning).
	Unfilled []string `json:"unfilled"`
	Unnamed  int      `json:"unnamed"`
	// Cover — обложка (032); nil — её нет. В Spreads её нет: в счёт и нумерацию не входит.
	Cover *CoverView `json:"cover"`
}

// DefaultSafe — безопасная зона по умолчанию, мм (требование fsfera: значимое не ближе 8 мм).
const DefaultSafe = 8.0

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
}
