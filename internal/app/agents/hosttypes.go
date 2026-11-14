package agents

// Типы, которыми Host отдаёт службе оформление и наборы: свои, узкие, без адресов картинок и сети.
// Фасад App (Г5) переводит в них DecorCatalog и LibraryPacks; лишнее в них попасть не может.

// DecorItemLite — плитка каталога оформления.
type DecorItemLite struct {
	ID     string
	Theme  string
	Name   string
	Own    bool   // своя (загружена на этом компьютере)
	PackID string // чей предмет: "" или "sa:builtin" — встроенный, иначе id набора
	Pack   string // название набора
	W, H   int
	Pages  int  // у разворота: 1 — страница, 2 — разворот
	Places int  // мест в шаблоне
	Steps  int  // у макета альбома: разворотов
	Vig    bool // у макета: есть шаг-виньетка
	Cover  bool // у макета: есть оформление обложки
}

// DecorThemeLite — тема каталога.
type DecorThemeLite struct {
	ID    string
	Name  string
	Count int
}

// DecorCatalogLite — каталог одного вида. Get и Invite — «Можно скачать» (144): образцы нескачанных
// наборов; служба их не читает (агенту нечего ставить из нескачанного), поля есть, чтобы тест
// проверил: даже переданные, наружу они не идут.
type DecorCatalogLite struct {
	Themes []DecorThemeLite
	Items  []DecorItemLite
	New    []string // ID плиток «Новые»
	Get    []DecorItemLite
	Invite bool
}

// PackLite — стоящий набор библиотеки.
type PackLite struct {
	ID      string
	Title   string
	Author  string
	License string
	Origin  string // builtin | file | net | demo
	Version int
	Bg      int
	Sticker int
	Spread  int
	Frame   int
	Print   int
	Bytes   int64
}

// PackAvailLite — набор из сохранённого списка «Можно скачать».
type PackAvailLite struct {
	ID      string
	Title   string
	Version int
	Bytes   int64
	New     bool // появился в последнем списке
}

// PacksLite — наборы без сети: то, что уже сохранено на этом компьютере.
type PacksLite struct {
	Installed []PackLite
	Available []PackAvailLite
	AvailAt   int64 // когда сохранён список, unix мс; 0 — списка нет
	Allowed   bool  // согласие на сеть
	AtStart   bool  // проверка при запуске
}
