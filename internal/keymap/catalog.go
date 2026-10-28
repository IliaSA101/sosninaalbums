// Package keymap — каталог команд с горячими клавишами и правила своих сочетаний (спека 074):
// разбор сочетания (Canon), служебные клавиши, действующие клавиши с учётом своих (Resolve),
// назначение, снятие и сброс. Чистая логика, только stdlib: хранение — prefs, методы окна — app.
package keymap

import "slices"

// Scope — область действия команды. Одна клавиша в одной области — конфликт, в пересекающихся
// областях разного ранга — перекрытие: действует старшая (design §2).
type Scope string

// Области (074-4).
const (
	ScopeApp     Scope = "app"     // везде
	ScopeCanvas  Scope = "canvas"  // холст и лента разворотов (зона последнего щелчка)
	ScopeLibrary Scope = "library" // лента фото
	ScopeBg      Scope = "bg"      // режим фона
)

// Command — строка каталога. Порядок каталога — порядок показа и приоритет внутри области.
type Command struct {
	ID       string   `json:"id"`             // совпадает с командой интерфейса (commands.ts)
	Label    string   `json:"label"`          // название в окне и справке
	Short    string   `json:"short"`          // на клавише карты, ≤ 10 знаков
	Hint     string   `json:"hint,omitempty"` // пояснение строкой ниже
	Group    string   `json:"group"`
	Scope    Scope    `json:"scope"`
	Feature  string   `json:"feature,omitempty"` // функция уровня интерфейса; Go её не толкует
	Defaults []string `json:"defaults"`
	Fixed    bool     `json:"fixed,omitempty"`   // служебная: клавиши не меняются
	Repeat   bool     `json:"repeat,omitempty"`  // срабатывает и автоповтором удержания
	Dialogs  bool     `json:"dialogs,omitempty"` // действует и под открытым окном (074-18)
}

// Named — группа: id и заголовок.
type Named struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ScopeInfo — область с заголовком и рангом (кто главнее при одной клавише).
type ScopeInfo struct {
	ID    Scope  `json:"id"`
	Title string `json:"title"`
	Rank  int    `json:"rank"`
}

var groups = []Named{
	{"album", "Альбом"},
	{"view", "Вид"},
	{"canvas", "Холст"},
	{"spreads", "Развороты и раскладки"},
	{"photos", "Фото"},
	{"marks", "Отметки фото"},
	{"bins", "Подборки фото"},
	{"text", "Текст"},
	{"grid", "Ручная раскладка"},
	{"decor", "Оформление"},
	{"backdrop", "Режим фона"},
}

var scopes = []ScopeInfo{
	{ScopeBg, "В режиме фона", 3},
	{ScopeCanvas, "На холсте и в ленте разворотов", 2},
	{ScopeLibrary, "В ленте фото", 2},
	{ScopeApp, "Везде", 1},
}

// Сборка строки каталога: область «везде», флаги — цепочкой.
func c(group, id, label, short, feature string, keys ...string) Command {
	if keys == nil {
		keys = []string{} // в JSON — [], а не null
	}
	return Command{ID: id, Label: label, Short: short, Group: group, Scope: ScopeApp, Feature: feature, Defaults: keys}
}

func (x Command) in(s Scope) Command    { x.Scope = s; return x }
func (x Command) hint(h string) Command { x.Hint = h; return x }
func (x Command) fixed() Command        { x.Fixed = true; return x }
func (x Command) repeat() Command       { x.Repeat = true; return x }
func (x Command) dialogs() Command      { x.Dialogs = true; return x }
func (x Command) marks() Command        { x.Scope = ScopeLibrary; x.Feature = "marks"; return x }
func (x Command) bins() Command         { x.Scope = ScopeLibrary; x.Feature = "bins"; return x }
func (x Command) bg() Command           { x.Scope = ScopeBg; x.Feature = "bgPhoto"; return x }

var arrows = []string{
	"ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown",
	"Shift+ArrowLeft", "Shift+ArrowRight", "Shift+ArrowUp", "Shift+ArrowDown",
}

// Каталог (design §2). Сдвиг мест (nudgePlace) без функции уровня: стрелки двигают и стикеры
// там, где ручной раскладки нет, — условие решает интерфейс (WHEN).
var catalog = []Command{
	c("album", "new", "Новый альбом", "Новый", "", "Mod+KeyN"),
	c("album", "newSheet", "Новый проект: виньетка-лист…", "Лист", "vignette"),
	c("album", "open", "Открыть альбом…", "Открыть", "", "Mod+KeyO"),
	c("album", "save", "Сохранить", "Сохранить", "", "Mod+KeyS"),
	c("album", "saveAs", "Сохранить как…", "Сохр. как", "", "Mod+Shift+KeyS"),
	c("album", "collect", "Собрать проект в папку…", "Собрать", ""),
	c("album", "backups", "Резервные копии…", "Копии", ""),
	c("album", "packs", "Наборы библиотеки…", "Наборы", "decor"),
	// Макеты альбома (131): окно с «Простого» уровня (0.27-131 вопрос 20 а), поэтому без функции уровня;
	// сохранение — из того же окна и тоже без неё (0.28-131-90).
	c("album", "looks", "Макеты альбома…", "Макеты", ""),
	c("album", "saveLook", "Сохранить альбом как макет…", "Как макет", ""),
	// Без функции уровня: подменю «Мои шаблоны» видно при сетках или виньетке — решает меню (086).
	c("album", "tplExport", "Сохранить мои шаблоны в файл…", "Сохр. шабл", ""),
	c("album", "tplImport", "Загрузить шаблоны из файла…", "Загр. шабл", ""),
	c("album", "showData", "Открыть папку данных", "К данным", ""),
	// Только Windows и выпуск: галочка, связь .salbum с этой программой (112); показ решает меню по State.Assoc.Can.
	c("album", "toggleAssoc", "Открывать .salbum этой программой", "Связь", ""),
	c("album", "addFolder", "Добавить папку с фото…", "Папка фото", ""),
	c("album", "relink", "Найти пропавшие фото…", "Пропавшие", ""),
	c("album", "export", "Экспорт…", "Экспорт", "", "Mod+KeyE"),
	c("album", "showExport", "Открыть папку экспорта", "К экспорту", ""),
	c("album", "settings", "Настройки альбома…", "Настройки", ""),
	c("album", "undo", "Отменить", "Отменить", "", "Mod+KeyZ").repeat().dialogs(),
	c("album", "redo", "Повторить", "Повторить", "", "Mod+KeyY", "Mod+Shift+KeyZ").repeat().dialogs(),

	// «Во весь экран» (режим F) и «Крупный просмотр фото» (окно, Z) — разные вещи (0.26-128-15).
	c("view", "toggleBig", "Во весь экран", "Весь экран", "timeline", "KeyF"),
	c("view", "photoPreview", "Крупный просмотр фото", "Крупно", "", "KeyZ"),
	// Сравнение 2–4 кадров (138): только из ленты; без функции уровня, как Z. Compare в Lightroom — C.
	c("view", "photoCompare", "Сравнить фото", "Сравнить", "", "KeyC"),
	c("view", "photoSimilar", "Похожие к этому", "Похожие", "", "Shift+KeyC"),
	c("view", "toggleDetails", "Панель деталей", "Детали", "", "KeyI"),
	c("view", "toggleSafe", "Безопасная зона", "Зона", "", "KeyG"),
	c("view", "toggleMarkup", "Разметка типографии", "Разметка", ""),
	c("view", "toggleRulers", "Линейки", "Линейки", "rulers", "Mod+KeyR"),
	c("view", "toggleSnap", "Привязка", "Привязка", "manual"),
	c("view", "toggleFaceGuide", "Силуэт под лицо", "Силуэт", "vignette"),
	c("view", "allSpreads", "Все развороты", "Все разв.", "allSpreads"),
	c("view", "compareLevels", "Уровни и функции…", "Уровни", ""),
	c("view", "levelSimple", "Уровень: Простой", "Простой", ""),
	c("view", "levelStandard", "Уровень: Стандартный", "Стандарт", ""),
	c("view", "levelFull", "Уровень: Полный", "Полный", ""),
	c("view", "levelCustom", "Уровень: Свой", "Свой", ""),
	c("view", "startWindow", "Стартовое окно", "Старт", ""),
	c("view", "whatsNew", "Что нового", "Новое", ""),
	c("view", "about", "О программе", "О прогр.", ""),
	c("view", "guide", "Справочник функций", "Справка", "", "F1").fixed(),
	c("view", "tour", "Первые шаги", "Шаги", ""),
	c("view", "videos", "Видео о программе", "Видео", ""),
	c("view", "help", "Горячие клавиши…", "Клавиши", "", "Shift+Slash", "Shift+Digit7"),

	c("canvas", "fitZoom", "Вписать разворот в окно", "Вписать", "", "Digit0", "Mod+Digit0"),
	c("canvas", "zoomIn", "Увеличить", "Увеличить", "zoom", "Mod+Equal", "Mod+Shift+Equal").repeat(),
	c("canvas", "zoomOut", "Уменьшить", "Уменьшить", "zoom", "Mod+Minus").repeat(),
	c("canvas", "spaceDown", "Пробел: коротко — раскладка, зажат + мышь — двигать", "Пробел", "", "Space", "Shift+Space").
		hint("Shift+Пробел коротко — предыдущая раскладка").fixed(),

	c("spreads", "prevSpread", "Предыдущий разворот", "Назад", "", "ArrowLeft").fixed().repeat(),
	c("spreads", "nextSpread", "Следующий разворот", "Вперёд", "", "ArrowRight").fixed().repeat(),
	c("spreads", "closeGallery", "Закрыть галерею раскладок", "Закрыть", "", "Escape").fixed(),
	c("spreads", "varyLayouts", "Разнообразить раскладки", "Разнообр.", "variants"),
	c("spreads", "makeVignette", "Сделать разворот виньеткой", "Виньетка", "vignette"),
	c("spreads", "vigSetup", "Настроить виньетку…", "Виньетка…", "vignette"),
	c("spreads", "sortVignette", "Сортировать учеников по фамилии", "По фамилии", "vignette"),
	c("spreads", "clearVignette", "Обычный разворот (снять виньетку)", "Обычный", "vignette"),
	c("spreads", "renameSpread", "Название разворота…", "Название", "names", "F2"),
	// 0.30-141: команды правого щелчка по группе ленты — в меню «Разворот»; клавиш нет, «можно ли» решает пункт меню (074-8).
	c("spreads", "insertSpread", "Вставить разворот после текущего", "Вставить", ""),
	c("spreads", "splitSpread", "Разрезать разворот перед выделенным фото", "Разрезать", "timeline"),
	c("spreads", "joinSpread", "Склеить разворот со следующим", "Склеить", "timeline"),

	c("photos", "selectSpread", "Выделить всё на развороте", "Выделить", "groupSelect", "Mod+KeyA").
		hint("Все фото, места и тексты разворота").in(ScopeCanvas),
	c("photos", "selectAll", "Выделить все фото в ленте", "Все фото", "", "Mod+KeyA").in(ScopeLibrary),
	// Поиск по имени в ленте (0.26-124-10): без пункта меню — Wails гасит поиск браузера (0.26-124-24).
	c("photos", "findPhotos", "Найти фото по имени", "Найти", "", "Mod+KeyF"),
	c("photos", "clearSelection", "Снять выделение", "Снять", "", "Escape").fixed(),
	c("photos", "removeCell", "Удалить выбранное", "Удалить", "", "Delete", "Backspace").
		hint("Фото — в ленту (в ручной раскладке место остаётся, второй раз — убрать место); " +
			"блок текста — удалить; группа — все сразу. Из библиотеки не удаляет").in(ScopeCanvas),
	// Копировать и вставить (099): Mod+C/V/X/D заняты не были; лента фото своих Mod+клавиш не имеет.
	c("photos", "cutItems", "Вырезать", "Вырезать", "clipboard", "Mod+KeyX").in(ScopeCanvas),
	c("photos", "copyItems", "Копировать", "Копировать", "clipboard", "Mod+KeyC").
		hint("Фото, места, надписи и стикеры; вставка — на любом листе").in(ScopeCanvas),
	c("photos", "pasteItems", "Вставить", "Вставить", "clipboard", "Mod+KeyV").in(ScopeCanvas),
	c("photos", "duplicateItems", "Дублировать", "Дубль", "clipboard", "Mod+KeyD").in(ScopeCanvas),
	c("photos", "addSelected", "Выделенные фото — на разворот", "На разв.", "", "Enter"),
	c("photos", "flipH", "Зеркало по горизонтали", "Зерк. ↔", "flip"),
	c("photos", "flipV", "Зеркало по вертикали", "Зерк. ↕", "flip"),
	c("photos", "gray", "Ч/б или цвет", "Ч/б", "adjust"),
	c("photos", "turnCw", "Повернуть ↻ 90°", "↻ 90°", "turn"),
	c("photos", "turnCcw", "Повернуть ↺ 90°", "↺ 90°", "turn"),
	c("photos", "straighten", "Выпрямить по линии", "Выпрямить", "tilt"),

	c("marks", "markRating0", "Звёзды: убрать", "Без звёзд", "", "Digit0").marks(),
	c("marks", "markRating1", "Звёзды: 1", "1 звезда", "", "Digit1").marks(),
	c("marks", "markRating2", "Звёзды: 2", "2 звезды", "", "Digit2").marks(),
	c("marks", "markRating3", "Звёзды: 3", "3 звезды", "", "Digit3").marks(),
	c("marks", "markRating4", "Звёзды: 4", "4 звезды", "", "Digit4").marks(),
	c("marks", "markRating5", "Звёзды: 5", "5 звёзд", "", "Digit5").marks(),
	c("marks", "markUp", "Звёзд больше", "Звёзды +", "", "BracketRight").marks(),
	c("marks", "markDown", "Звёзд меньше", "Звёзды −", "", "BracketLeft").marks(),
	c("marks", "markRed", "Метка: красная", "Красная", "", "Digit6").marks().hint("Повтор у всех выделенных — снять"),
	c("marks", "markYellow", "Метка: жёлтая", "Жёлтая", "", "Digit7").marks().hint("Повтор у всех выделенных — снять"),
	c("marks", "markGreen", "Метка: зелёная", "Зелёная", "", "Digit8").marks().hint("Повтор у всех выделенных — снять"),
	c("marks", "markBlue", "Метка: синяя", "Синяя", "", "Digit9").marks().hint("Повтор у всех выделенных — снять"),
	c("marks", "markPick", "Флажок «отобрано»", "Отобрано", "", "KeyP").marks(),
	c("marks", "markReject", "Флажок «отклонено»", "Отклонено", "", "KeyX").marks(),
	c("marks", "markUnflag", "Снять флажок", "Без флажка", "", "KeyU").marks(),

	c("bins", "binToggle", "Подборка: добавить или убрать выделенные", "Подборка", "", "KeyB").bins().
		hint("В показанную подборку, иначе — в последнюю, куда добавляли; повтор у всех выделенных — убрать"),
	c("bins", "binNew", "Новая подборка из выделенных…", "Новая", "", "Shift+KeyB").bins(),
	c("bins", "binRemove", "Убрать выделенные из подборки", "Из подб.", "", "Delete", "Backspace").bins().
		hint("Когда лента показывает подборку. С диска и из альбома ничего не удаляется"),
	c("bins", "binAll", "Показать все фото (без подборки)", "Все фото", "").bins(),

	c("text", "addText", "Добавить текст", "Текст", "text"),
	c("text", "pasteNames", "Имена из списка…", "Имена", "captions"),

	c("grid", "nudgePlace", "Сдвиг выделенного на 1 мм (Shift — 10 мм)", "Сдвиг", "", arrows...).fixed().repeat(),
	c("grid", "layerFront", "Слой: на передний план", "Передний", "layers", "Mod+Shift+BracketRight"),
	c("grid", "layerUp", "Слой: выше", "Выше", "layers", "Mod+BracketRight").repeat(),
	c("grid", "layerDown", "Слой: ниже", "Ниже", "layers", "Mod+BracketLeft").repeat(),
	c("grid", "layerBack", "Слой: на задний план", "Задний", "layers", "Mod+Shift+BracketLeft"),
	c("grid", "alignLeft", "Выровнять: по левому краю", "Влево", "align"),
	c("grid", "alignHcenter", "Выровнять: по центру по горизонтали", "Центр ↔", "align"),
	c("grid", "alignRight", "Выровнять: по правому краю", "Вправо", "align"),
	c("grid", "alignTop", "Выровнять: по верхнему краю", "Вверх", "align"),
	c("grid", "alignVcenter", "Выровнять: по центру по вертикали", "Центр ↕", "align"),
	c("grid", "alignBottom", "Выровнять: по нижнему краю", "Вниз", "align"),
	c("grid", "alignDistH", "Распределить по горизонтали", "Распр. ↔", "align"),
	c("grid", "alignDistV", "Распределить по вертикали", "Распр. ↕", "align"),

	c("decor", "bgGallery", "Галерея фонов…", "Фоны", "decor"),
	c("decor", "borderGallery", "Рамочки…", "Рамочки", "border"),
	c("decor", "addSticker", "Добавить стикер…", "Стикер", "decor"),
	c("decor", "copyDecor", "Копировать оформление…", "Оформление", "decor"),
	c("decor", "spreadTpl", "Готовые развороты…", "Готовые", "decor"),

	c("backdrop", "bgNudge", "Сдвиг рамки фона (Shift — 10 мм)", "Сдвиг", "", arrows...).bg().fixed().repeat(),
	c("backdrop", "bgRemove", "Убрать фото фона", "Убрать", "", "Delete", "Backspace").bg().hint("И выйти из режима фона"),
	c("backdrop", "bgExit", "Выйти из режима фона", "Выйти", "", "Escape").bg().fixed().
		hint("Во время жеста — отменить жест"),
}

// byID — номер строки каталога по id.
var byID = func() map[string]int {
	m := make(map[string]int, len(catalog))
	for i, x := range catalog {
		m[x.ID] = i
	}
	return m
}()

// Catalog — копия каталога по порядку (списки клавиш не делят память с пакетом).
func Catalog() []Command {
	out := make([]Command, len(catalog))
	for i, x := range catalog {
		x.Defaults = slices.Clone(x.Defaults)
		out[i] = x
	}
	return out
}

// Groups — группы по порядку показа.
func Groups() []Named { return slices.Clone(groups) }

// Scopes — области по убыванию ранга.
func Scopes() []ScopeInfo { return slices.Clone(scopes) }

func lookup(id string) (Command, bool) {
	i, ok := byID[id]
	if !ok {
		return Command{}, false
	}
	return catalog[i], true
}

func rank(s Scope) int {
	for _, x := range scopes {
		if x.ID == s {
			return x.Rank
		}
	}
	return 0
}

// overlaps — бывают ли области активны одновременно: «везде» — с любой, фон — с холстом и лентой;
// холст и лента фото не пересекаются (зона последнего щелчка одна).
func overlaps(a, b Scope) bool {
	if a == b || a == ScopeApp || b == ScopeApp {
		return true
	}
	return a == ScopeBg || b == ScopeBg
}
