package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// Choice — вариант раскладки для галереи.
type Choice struct {
	Key   string        `json:"key"`
	Rects []layout.Rect `json:"rects"` // весь разворот, в порядке Spread.Photos
	// Family — сегмент фильтра по форме: dense | hero | air | bleed (041), strip | figure (051);
	// "" — не размечено.
	Family string `json:"family"`
	Twin   int    `json:"twin"` // индекс более раннего с тем же видом в этом ряду; −1 — нет
	// FormName — название формы для подписи плитки («Лента», «Лесенка», «Плотно»…); непусто у каждой.
	FormName string `json:"formName"`
	// Crop — подрезка раскладки (104): целый процент вверх по стороне, 0 — фото целиком.
	Crop int `json:"crop"`
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
	// Families — разрешённые семейства раскладок (065, variety.Family: dense, hero, air, bleed,
	// strip, figure); nil, пусто или все — «Все», как 0.14. В AutoPlan не влияет.
	Families []string `json:"families"`
	// Похожие, ч/б, вариант и пересборка (078); нули — сборка 0.19 побитово.
	Similar int      `json:"similar"` // убирать похожие кадры: 0 нет, 1 дубли, 2 очень похожие, 3 похожие
	Keep    []string `json:"keep"`    // ID возвращённых пользователем кадров: не убирать
	Gray    int      `json:"gray"`    // ч/б разворотов: 0 нет, 1 — 10 %, 2 — 20 %, 3 — 30 %
	Variant int      `json:"variant"` // номер варианта: 0 и 1 — как 0.19, 2… — другие границы и раскладки
	Again   bool     `json:"again"`   // пересобрать последнюю сборку: от альбома до неё (Editor, 078 §8)
	// Album — «Весь альбом заново» (107): пул — фото голых разворотов «Авто», якоря (ручные,
	// виньетки, оформленные, пустые) остаются на местах; at не действует. Ноль — сборка 0.23.
	Album bool `json:"album"`
}

// MaxAutoLevel — старший уровень «Похожих» и «Ч/б»; MaxVariant — старший номер варианта.
const (
	MaxAutoLevel = 3
	MaxVariant   = 9999
)

// maxKeepArgs — сколько ID возвращённых кадров App берёт от UI: огромный массив не должен
// разбираться целиком, а в окне больше не вернуть.
const maxKeepArgs = 10_000

// maxFamilyArgs — сколько строк семейств App берёт от UI: их шесть, остальное — мусор, а
// большой массив не должен разбираться целиком.
const maxFamilyArgs = 16

// Clamp — настройки из UI: Variety в 0…2, семейств не больше maxFamilyArgs строк (имена и
// повторы разбирает сборка).
func (o AutoOpts) Clamp() AutoOpts {
	o.Variety = o.Variety.Clamp()
	o.Families = ClampFamilies(o.Families)
	o.Similar = min(max(o.Similar, 0), MaxAutoLevel)
	o.Gray = min(max(o.Gray, 0), MaxAutoLevel)
	o.Variant = min(max(o.Variant, 1), MaxVariant)
	o.Keep = o.Keep[:min(len(o.Keep), maxKeepArgs)]
	return o
}

// ClampFamilies — первые maxFamilyArgs строк семейств от UI (nil остаётся nil).
func ClampFamilies(fs []string) []string { return fs[:min(len(fs), maxFamilyArgs)] }

// AutoPreview — ряд блока «Раскладки» окна автосборки (065): возможные варианты и то, что
// соберёт AutoBuild с теми же аргументами; проект не меняется.
type AutoPreview struct {
	W        float64         `json:"w"` // лист нового разворота, мм (как SpreadView.w)
	H        float64         `json:"h"`
	Samples  []SampleRow     `json:"samples"`  // «Варианты»: по одному на число фото плана, по возрастанию
	Spreads  []PreviewSpread `json:"spreads"`  // «Что соберётся»: по порядку сборки
	Fallback int             `json:"fallback"` // разворотов без раскладки разрешённых семейств
}

// SampleRow — варианты первой группы плана из Photos фото (её настоящие пропорции): то, что
// покажет полоса «Варианты» на этом развороте. Семейства не фильтруются — фильтрует UI, как в
// полосе: снятие фильтра не требует запроса.
type SampleRow struct {
	Photos  int      `json:"photos"`
	Group   int      `json:"group"`   // номер разворота в сборке, 0-based
	InOrder []Choice `json:"inOrder"` // ряд «Авто» (первая порция, без «Ещё»)
	Extra   []Choice `json:"extra"`   // «Другие варианты»
}

// PreviewSpread — будущий разворот «Что соберётся»: ключ и rects (весь разворот, мм), семейство
// и форма; Twin = −1 (похожие в этом ряду не сворачиваются).
type PreviewSpread struct {
	Choice
	Photos   int  `json:"photos"`
	Fallback bool `json:"fallback"` // разрешённых семейств нет — взята лучшая
	Gray     bool `json:"gray"`     // сборка сделает разворот ч/б (078)
}

// AutoPlan — подсказка окна автосборки: сколько фото и как они лягут на N разворотов.
// AutoBuild с теми же фото и AutoOpts делает ровно эти развороты.
type AutoPlan struct {
	Photos     int     `json:"photos"`     // фото в сборку (без сеток и фонов)
	Spreads    int     `json:"spreads"`    // N после зажима
	MinSpreads int     `json:"minSpreads"` // низ ползунка: ⌈photos/до⌉
	MaxSpreads int     `json:"maxSpreads"` // верх: min(⌊photos/от⌋ или photos, MaxSpreads − непустых; пустой хвост — свободен)
	Avg        float64 `json:"avg"`
	Min        int     `json:"min"`               // фото в самом маленьком развороте
	Max        int     `json:"max"`               // и в самом большом
	Reuse      int     `json:"reuse"`             // сколько пустых разворотов из хвоста займёт сборка
	From       int     `json:"from"`              // действующее «от» (в «Авто» — посчитанное по N); 0 — ровно, как 0.12
	To         int     `json:"to"`                // действующее «до»
	Cuts       int     `json:"cuts"`              // границ серий силы ≥ 0.5; 0 — без Series или сигналов нет
	Similar    int     `json:"similar,omitempty"` // похожих не взято (078), Photos — уже без них; 0 — поля нет: прежний фронт и тесты собираются
	Gray       int     `json:"gray,omitempty"`    // разворотов станет ч/б (078); 0 — поля нет
	// Album — состав альбома в режиме «Весь альбом заново» (107); только при AutoOpts.Album.
	// Spreads в этом режиме — сколько новых разворотов соберётся, якоря в него не входят.
	Album *AlbumPlan `json:"album,omitempty"`
}

// AlbumPlan — что режим «Весь альбом заново» (107) пересобирает и что не трогает: числа для
// строки источника и вопроса окна. Считается по альбому до похожих кадров.
type AlbumPlan struct {
	Spreads   int `json:"spreads"`   // голых разворотов сейчас — N по умолчанию в окне
	Photos    int `json:"photos"`    // фото на них
	Free      int `json:"free"`      // свободных взято в пул
	Manual    int `json:"manual"`    // якоря: ручные (сетка)
	Vignettes int `json:"vignettes"` // виньетки
	Decor     int `json:"decor"`     // оформленные «Авто» (тексты, стикеры, свой фон, название…)
	Blank     int `json:"blank"`     // пустые
}

// SimilarDrop — кадр, который автосборка не возьмёт как похожий (078): Like — оставленный кадр,
// на который он похож, Dist — расстояние подписей (0…128).
type SimilarDrop struct {
	ID   string `json:"id"`
	Like string `json:"like"`
	Dist int    `json:"dist"`
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
	// Sig — подпись кадра для отбора похожих (078); заполняет App из памяти, Ok = false — подписи нет.
	// Только вход автосборки, в .salbum не пишется.
	Sig imgsig.Sig
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
	// ScopeSpine — область фото корешка обложки (061): только фон-фото, у сетки и узоров её нет.
	ScopeSpine = "spine"
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
	// Border — рамочка места (079) только в памяти: копии внутри альбома («Копировать
	// раскладку»). В templates.json и от UI не идёт (079-10: запас файла шаблонов 1,9 %).
	Border *Border `json:"-"`
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
	// Waiting — пустые места листа с именем на табличке (071): человек из списка без фото,
	// в печати его нет — вопрос перед экспортом. Интерфейсу не нужно.
	Waiting int `json:"-"`
	// NoAlbumBg — фон альбома на листе скрыт (Spread.NoAlbumBg, 098): «Фон альбома скрыт · Показать».
	NoAlbumBg bool `json:"noAlbumBg,omitempty"`
	// FoldHidden — сколько раскладок «Разброса» скрыто из-за фото на сгибе при «Только широкие»
	// (105 В1): «Ещё N раскладок с фото на сгибе» в галерее; 0 — нечего показывать.
	FoldHidden int `json:"foldHidden,omitempty"`
	// LookStep — номер шага макета, которым поставлен лист (131, Spread.LookStep); 0 — не макетом.
	LookStep int `json:"lookStep,omitempty"`
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

// View — вид альбома для интерфейса. В JSON может уйти дельтой (075, viewwire.go): у вида с
// Base > 0 MarshalJSON кладёт в spreads только изменённые развороты. Go-код всегда видит полный
// список Spreads.
type View struct {
	// Product — тип изделия: по нему интерфейс выбирает режимы (в 0.5.0 только album).
	Product  string       `json:"product"`
	Spreads  []SpreadView `json:"spreads"`
	Settings Settings     `json:"settings"`
	Folders  []string     `json:"folders"`
	// Loose — отдельные фото по добавлениям (097, LooseView); всегда список, не null.
	Loose   []LooseView `json:"loose"`
	Path    string      `json:"path"`
	Dirty   bool        `json:"dirty"`
	CanUndo bool        `json:"canUndo"`
	CanRedo bool        `json:"canRedo"`
	Rev     int         `json:"rev"`
	// MaxSpreadPhotos — интерфейс по нему заранее отключает склейку (= MaxPhotos;
	// имя прежнее — контракт UI).
	MaxSpreadPhotos int `json:"maxSpreadPhotos"`
	// Safe — безопасная зона (мм, в координатах разворота): значимое не ближе Settings.Safe к краю.
	// Считается всегда; показывать или нет, решает интерфейс.
	Safe []layout.Rect `json:"safe"`
	// Bleed — полосы обреза (066) по внешним краям разворота, мм: верх, низ, лево, право; nil —
	// Settings.Bleed 0. Показываются вместе с Safe; у сгиба полос нет.
	Bleed []layout.Rect `json:"bleed"`
	// SpreadMarkup — разметка типографии разворотов (066, Project.SpreadGuide); nil — файла нет.
	SpreadMarkup *GuideView `json:"spreadMarkup"`
	// MinDPI — порог «!» у фото: min(lowDPILimit, settings.dpi). LowDPI — сколько фото ниже него
	// (кадры и фоны, сумма SpreadView.LowDPI).
	MinDPI int `json:"minDPI"`
	LowDPI int `json:"lowDPI"`
	// EmptyPlaces — пустых мест сетки во всём альбоме.
	EmptyPlaces int `json:"emptyPlaces"`
	// FillFrom — сколько мест займёт «Заполнить места» с разворота i (FillCounts), индекс как у
	// Spreads; всегда список (nil ушёл бы null), у вида печати — пустой. Не в SpreadView: дельта
	// (075) пересылала бы все развороты при каждой посадке. EmptyPlaces остаётся для «Сведений».
	FillFrom []int `json:"fillFrom"`
	// CoverFill — то же с обложки (обложка и все развороты); без обложки и в виде печати — 0.
	CoverFill int `json:"coverFill"`
	// TextMaxW — предел ширины блока «по тексту», мм (страница без безопасных полей).
	TextMaxW float64 `json:"textMaxW"`
	// BackdropUsed — ID фото, стоящих на фонах и в своих стикерах (по возрастанию): автосборка,
	// «Заполнить пустые» и фильтр «Неиспользованные» их не берут.
	BackdropUsed []string `json:"backdropUsed"`
	// LibPhotos — ID записей Photos со своим из библиотеки (PhotoRef.Lib, 048), по возрастанию:
	// пропавшие среди них UI считает отдельно («Нет в библиотеке этого компьютера», без «Найти…»).
	LibPhotos []string `json:"libPhotos"`
	// AlbumFonts — свои гарнитуры, стоящие в альбоме (Project.Fonts, 082), по названию: каких нет
	// на этом компьютере, UI считает сам — их нет в списке шрифтов (082-24). Без своих — поля нет.
	AlbumFonts []FontUse `json:"albumFonts,omitempty"`
	// MarksRev — номер состояния своих отметок: сменился — UI перечитывает Marks (сами отметки
	// в View не кладутся: на тысячах фото это сотни КБ на каждое событие).
	MarksRev int `json:"marksRev"`
	// Bins — подборки фото (083) в порядке создания; всегда список (nil и [] разошлись бы в шапке
	// дельты). Членство — в Marks (MarkView.Bins).
	Bins []Bin `json:"bins"`
	// Unfilled — пустые переменные и поля, встреченные в печатаемом тексте всех листов (по
	// алфавиту); Unnamed — фото, у подписи которых поля имени есть, а имени нет. По ним —
	// вопрос перед экспортом (export.FieldsWarning).
	Unfilled []string `json:"unfilled"`
	Unnamed  int      `json:"unnamed"`
	// Drafts — заготовок (131) на разворотах и обложке: вопрос перед экспортом и окно сохранения макета.
	Drafts int `json:"drafts"`
	// Look — макет альбома (131, Project.Look); nil — альбом не по макету.
	Look *LookView `json:"look,omitempty"`
	// Cover — обложка (032); nil — её нет. В Spreads её нет: в счёт и нумерацию не входит.
	Cover *CoverView `json:"cover"`
	// AlbumBg — фон альбома для строки настроек (098); nil — его нет.
	AlbumBg *AlbumBgView `json:"albumBg,omitempty"`
	// PageLimit — предел разворотов типографии альбома (085, LimitOf); nil — типография
	// неизвестна или у изделия нет предела. omitempty: у альбомов без типографии вид прежний.
	PageLimit *PageLimitView `json:"pageLimit,omitempty"`
	// Again — последнюю автосборку можно пересобрать (078 §8): после неё альбом не менялся
	// (звёзды и метки не в счёт). UI по нему показывает «Пересобрать…».
	Again bool `json:"again"`
	// Seq — номер вида в цепочке App (075); 0 — вид вне цепочки (Editor.View, экспорт, паника).
	Seq int `json:"seq"`
	// Base > 0 — дельта к виду с seq == Base: в JSON spreads — только изменённые развороты.
	Base int `json:"base,omitempty"`
	// Order — в дельте ID всех разворотов по порядку (удалённого нет, новый — и в spreads).
	Order []string `json:"order,omitempty"`
	// fresh — какие Spreads идут в дельту; ставит только Wire.
	fresh []bool
	// src — номер Editor, построившего вид (087-3): поздний вид прежнего альбома App отличает
	// от вида нынешнего. В JSON не идёт; 0 — вид не от Editor.
	src uint64
}

// SetSrc — пометить вид номером Editor (ставит только Editor.view).
func (v *View) SetSrc(id uint64) { v.src = id }

// Src — номер Editor, построившего вид; 0 — неизвестен.
func (v View) Src() uint64 { return v.src }

// PageLimitView — предел разворотов типографии альбома (085): строки готовы для интерфейса.
type PageLimitView struct {
	Who   string `json:"who"`   // «Нетпринт «Премиум»» — изделие из обложки | «Фотосфера»
	Range string `json:"range"` // «от 10 до 60 разворотов» | «до 57 разворотов»
	Count int    `json:"count"` // разворотов (страниц при PrintLimit.Pages)
	Min   int    `json:"min"`   // 0 — не задано
	Max   int    `json:"max"`   // 0 — не задано
	Warn  string `json:"warn"`  // "" — в пределах; иначе текст предупреждения
}

// Виды вхождения (106): фронт выбирает переход по ним, подпись не разбирает.
const (
	UsePlace    = "place"    // место листа (Авто или сетка)
	UseBackdrop = "backdrop" // фото фона области листа
	UseSticker  = "sticker"  // фото-стикер
	UseBorder   = "border"   // картинка рамочки места
	UseAlbumBg  = "albumBg"  // фон альбома (098)
)

// PhotoUse — одно вхождение фото в альбом для карточки (084): лист, подпись и вид перехода.
type PhotoUse struct {
	// Sheet — индекс в Project.Sheets() (он же store.sheets интерфейса); −1 — фон альбома (098).
	Sheet  int    `json:"sheet"`
	Label  string `json:"label"`          // «Разворот 3 — место»; = Page + " — " + What (фон альбома — Page)
	Kind   string `json:"kind"`           // Use*
	Spread string `json:"spread"`         // ID листа (обложка — CoverID); "" — фон альбома
	Page   string `json:"page"`           // «Разворот 3» | «Лист 2» | «Обложка» | «Фон альбома» — текст ссылки
	What   string `json:"what"`           // «место» | «фон» | «стикер» | «рамочка»; "" — фон альбома
	Pick   string `json:"pick,omitempty"` // что выделить: запись места, ID стикера, ref места с рамочкой, область фона
}

// UsedAt — «где стоит» фото для окна «Фото уже в альбоме» (098 design §2.4).
type UsedAt struct {
	ID     string     `json:"id"`     // ID файла (BaseID переданного)
	Uses   []PhotoUse `json:"uses"`   // вхождения всех записей семьи; не nil
	Places int        `json:"places"` // сколько мест у всех записей семьи
	Move   string     `json:"move"`   // запись единственного места; "" — мест 0 или больше одного
}

// DefaultSafe — безопасная зона по умолчанию, мм (требование fsfera: значимое не ближе 8 мм).
const DefaultSafe = 8.0
