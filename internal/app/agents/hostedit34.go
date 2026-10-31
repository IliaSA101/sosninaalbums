package agents

// Ступень 3б (151, 0.34): лист-сетка и правки отметок, обложки, фона, стикеров и формата типографии.
// Те же правила, что у hostedit.go: только числа, bool, строки и срезы строк (TestHostTypesScalar),
// ключ и rev сверяет фасад под замком Editor, пути и имена файлов в типы не попадают.

// SheetReq — лист-сетка (get_contact_sheet). IDs — ID фото в порядке клеток; пустой ID — клетка-«×»:
// файл пропал или не читается (по Status служба решила сама, фасад файла не касается).
type SheetReq struct {
	IDs   []string
	First int // номер первой клетки на плашке: offset+1; у листа по ids — 1
	Grid  int // клеток в стороне, 3…8
	Side  int // сторона листа, px, 768…1568
}

// Состояния клетки листа (второй результат Host.Sheet, по одному на ID запроса).
const (
	CellOK      = "ok"
	CellPending = "pending" // миниатюра не успела к сроку — повторный вызов возьмёт готовое
	CellBroken  = "broken"  // файла нет или он не читается
)

// MarkReq — отметки фото (set_marks). IDs — ID фото; Flag: "" не менять, pick | reject | none;
// Label: "" не менять, цвет или none.
type MarkReq struct {
	Key       string
	Rev       int
	Client    string
	IDs       []string
	SetRating bool // Rating задан (0 — снять звёзды)
	Rating    int
	Flag      string
	Label     string
}

// MarkRes — итог set_marks. Changed — у скольких фото отметка в самом деле сменилась; Skipped — ID не из ленты.
type MarkRes struct {
	Album   AlbumRef
	Changed int
	Skipped []string
}

// CoverReq — кадр обложки (set_cover_photo).
type CoverReq struct {
	Key    string
	Rev    int
	Client string
	Photo  string
}

// CoverRes — итог set_cover_photo. Place: seat (место лицевой) | background (фон лицевой).
type CoverRes struct {
	Album   AlbumRef
	Created bool   // обложки не было — создана
	Product string // изделие созданной обложки; "" — обложка уже была
	Place   string
	Changed bool // false — этот кадр уже на месте
}

// BgReq — фон из каталога на набор разворотов (set_background). Spreads — ID листов.
type BgReq struct {
	Key     string
	Rev     int
	Client  string
	Item    string
	Spreads []string
	Scope   string // spread | left | right
	Replace bool   // заменять и фото клиента на фоне
	// Album — фон альбома (154): Spreads — листы согласия (ID), где можно снять фото клиента (с Replace);
	// Scope не передаётся. Pages — на каждую страницу (служба берёт текущую, когда агент не сказал).
	Album bool
	Pages bool
}

// BgRes — итог set_background. Skipped — ID листов, где фон области — фото клиента (без Replace).
// Фон альбома (154): Cleared — ID листов со снятым своим фоном; Hidden — ID листов, где человек скрыл
// фон альбома («Не показывать здесь»), он там не виден; AlbumBg — фон альбома сменился.
type BgRes struct {
	Album    AlbumRef
	Changed  int
	Skipped  []string
	Replaced bool
	Cleared  []string
	Hidden   []string
	AlbumBg  bool
}

// VaryRes — итог vary_layouts (154): Spreads — ID листов запроса; Before и After — семейство каждого
// до и после ("" — нет); пусто, если человек правил альбом сразу после (пара недостоверна).
type VaryRes struct {
	Album   AlbumRef
	Changed int
	Spreads []string
	Before  []string
	After   []string
}

// StickerReq — пачка стикеров (add_sticker): параллельные срезы одной длины, по элементу на стикер.
// Срез структур TestHostTypesScalar не пускает (0.34-151-24); длины сверяет служба, фасад — ещё раз.
type StickerReq struct {
	Key     string
	Rev     int
	Client  string
	Spreads []string // ID листов
	Pages   []string // left | right
	Items   []string // id плитки каталога
	Places  []string // center | top-left | top-right | bottom-left | bottom-right
	Sizes   []string // s | m | l
}

// StickerRes — итог add_sticker.
type StickerRes struct {
	Album AlbumRef
	Added int
	IDs   []string // ID добавленных стикеров
}

// PrintReq — формат типографии (apply_print). Profile — настоящий ID профиля (маску own-<хэш> служба уже
// перевела), Format — имя формата; "" — первый.
type PrintReq struct {
	Key     string
	Rev     int
	Client  string
	Profile string
	Format  string
}

// PrintRes — итог apply_print.
type PrintRes struct {
	Album        AlbumRef
	PageW, PageH float64 // страница, мм
	Cover        string  // изделие обложки в итоге; "" — обложки нет
	CoverChanged bool
	Changed      bool
	OverLimit    bool
}

// Ошибки, которые фасад вправе вернуть из замыканий 3б: служба скажет их агенту словами (как ErrAskBusy и др.
// в hostedit.go).
var (
	ErrNoCover      error = errNoCover      // у альбома-листа обложки нет
	ErrNoDecor      error = errNoDecor      // нет такого фона или стикера в каталоге
	ErrNoProfile    error = errNoProfile    // нет такой типографии или формата
	ErrStickerLimit error = errStickerLimit // на листе или в альбоме нет места для стикеров
	ErrNoPhoto      error = errNoPhoto      // фото нет в ленте
)

// canEdit34 — Host умеет правки 0.34: заполнены правки 149 и все семь замыканий 3б. Иначе правящих
// инструментов 0.34 нет (get_contact_sheet читает и ждёт только Sheet).
func (h Host) canEdit34() bool {
	return h.canEdit() && h.Sheet != nil && h.Families != nil && h.Mark != nil && h.Cover != nil && h.Bg != nil &&
		h.Stickers != nil && h.Print != nil
}
