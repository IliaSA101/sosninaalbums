package agents

// Правки альбома агентом (149, ступень 3). Служба Editor не получает: всё идёт замыканиями Host,
// которые заполняет фасад App (ключ и rev сверяются там под замком Editor — design §2.1).
// Типы запросов и ответов — только числа, bool, строки и срезы строк: через них наружу не уйдёт
// ни путь, ни замыкание, ни карта (TestHostTypesScalar). Полей с Dir и Path в имени нет.

// Виды вопроса (AskReq.Kind).
const (
	AskCreate = "create" // «Создать альбом по просьбе агента?»
	AskAdd    = "add"    // «Добавить папку по просьбе агента?»
	AskExport = "export" // «Выгрузить альбом по просьбе агента?» (0.35/152)
	AskOpen   = "open"   // «Открыть альбом по просьбе агента?» (0.35/152)
	AskEdit   = "edit"   // «Спрашивать» у обратимой правки или показа (150): вопрос из шаблона Go по входу, окна ОС нет
)

// Итоги вопроса (AskRes.Why): чем кончился вопрос, когда окно было показано.
const (
	WhyAnswered    = "answered"     // «да», действие выполнено
	WhyDeclined    = "declined"     // «Отмена», ✕, Esc или отмена окна ОС
	WhyTimeout     = "timeout"      // срок вопроса вышел
	WhyGone        = "gone"         // агент ушёл или отменил вызов
	WhyClosed      = "closed"       // программа закрывается
	WhyOff         = "off"          // флажок правок выключили
	WhyTooWide     = "too_wide"     // выбранная папка не годится (корень, домашняя, системная)
	WhyOurs        = "ours"         // выбрана папка самой программы
	WhyDraftLocked = "draft_locked" // прежний черновик занят — альбом не создан
	WhyStopped     = "stopped"      // человек остановил действие после «да» (отмена окна ОС, 152): в счёт отказов не идёт
)

// AlbumRef — ключ альбома и его ревизия. Key — AlbumKey(номер Editor): по нему фасад сверяет
// альбом под замком.
type AlbumRef struct {
	Key string `json:"key" jsonschema:"ключ альбома из get_album; меняется при открытии другого альбома"`
	Rev int    `json:"rev" jsonschema:"ревизия альбома из get_album; растёт с каждой правкой"`
}

// AskReq — вопрос человеку в окне программы. Текст вопроса фасад берёт из своих шаблонов:
// своего текста агент не вносит.
type AskReq struct {
	Kind   string // AskCreate | AskAdd | AskEdit | AskOpen | AskExport
	Client string // имя клиента, очищено; "" — неизвестный клиент
	Hint   string // folder_hint как прислал агент (≤ 4096 байт): подсказка окну выбора, не путь для применения; AskOpen: name_hint, нашедший Key
	Key    string // AskAdd, AskEdit: ключ альбома, куда просят; AskOpen: ключ недавнего из list_albums ("" — окно выбора файла)
	Rev    int    // AskAdd, AskEdit: ревизия, которую видел агент
	// Always — у человека «Всегда» (150): фасад решает, можно ли без вопроса (design §4.1). Можно — AskRes{Why: WhyQuiet}
	// без окна; нельзя — обычный вопрос (служба вернёт тихий слот, увидев итог не WhyQuiet).
	Always bool
	// AskEdit: инструмент и числа входа для шаблона вопроса (Tool — имя MCP). Текст агента сюда не идёт.
	Tool     string
	Spreads  []string // ID разворотов входа (vary_layouts, show_spread, set_background, add_sticker)
	Families []string // семейства раскладок входа
	Build    int      // autobuild: сколько разворотов просит агент; 0 — по умолчанию окна
	Photos   []string // ID фото входа автосборки: фасад считает по ним фактически свободные (Count — число агента)
	Count    int      // сколько фото (autobuild, set_marks, set_cover_photo) или стикеров; 0 — не названо (все свободные)
	Variety  int      // vary_layouts: 1…2
	Item     string   // set_background, apply_print: id фона или типографии (фасад называет по каталогу)
	Format   string   // apply_print: имя формата
	// AskExport: окно «Экспорт» из входа агента (Key, Rev — альбом; Hint — folder_hint). Print: "" — как окно
	// открывается, "plain" — обычная выгрузка, иначе id типографии из list_print_profiles (маска уровня 1
	// служба уже сняла).
	Range     string
	Pages     bool
	Cover     bool
	CoverOnly bool
	Quality   int // 100 | 95 | 80; 0 — 95
	DPI       int // 72…1200; 0 — dpi альбома
	Print     string
}

// AskRes — итог вопроса. Why — одно из Why*; при WhyAnswered остальные поля заполнены.
type AskRes struct {
	Why      string
	Album    AlbumRef // ключ и ревизия после действия
	Folder   int      // номер папки в альбоме с 1
	Merged   bool     // папка уже была в альбоме: шага отмены нет
	Scanning bool     // лента ещё читает папку
	Files    int      // AskExport: сколько файлов выгрузка пишет
}

// ExportStatus — ход последней выгрузки для get_album.export: State idle | running | done | stopped | failed.
// Error без пути (Host чистит). Только скаляры (TestHostTypesScalar).
type ExportStatus struct {
	State string
	Done  int
	Total int
	Error string
}

// AutoReq — автосборка в конец альбома; умолчания окна «Автосборка» (0.33-149-30) служба уже
// подставила. Полей Album и Again нет: фасад всегда ставит их false (М10).
type AutoReq struct {
	Key        string   // ключ альбома
	Rev        int      // ревизия, которую видел агент
	Client     string   // имя клиента для тоста «Claude Code · Собрано разворотов: N»
	Spreads    int      // 0 — по умолчанию окна (photos/4), зажим — как у AutoPlan
	PerAuto    bool     // «Авто»: «от» и «до» считает программа
	PerMin     int      // «от» (при PerAuto не действует)
	PerMax     int      // «до»
	Series     bool     // не разрывать серии съёмки
	Similar    int      // 0…3
	Variant    int      // 0…9999
	Variety    int      // 0…2
	Families   []string // пусто — «Все»
	Photos     []string // ID из list_photos; пусто — все свободные
	ScanWaitMs int      // сколько ждать конец чтения ленты, мс; 0 — не ждать
	Asked      bool     // вызов шёл при «Спрашивать» (вопрос задан); при «Всегда» смена на «Спрашивать» в ожидании — отказ
}

// AutoRes — итог автосборки.
type AutoRes struct {
	Album    AlbumRef
	Added    int    // собрано разворотов
	Photos   int    // фото в сборке
	PlanFrom int    // «от» и «до» плана
	PlanTo   int    //
	Similar  int    // похожих кадров не взято
	First    int    // номер первого нового разворота с 1; 0 — нет
	Go       string // ID разворота для перехода окна; "" — без перехода
}

// VaryReq — перебор раскладок разворотов (vary_layouts). Spreads — ID разворотов.
type VaryReq struct {
	Key      string
	Rev      int
	Client   string // имя клиента для тоста
	Spreads  []string
	Variety  int      // 1…2
	Families []string // пусто — все
}

// Ошибки, которые фасад вправе вернуть из замыканий Host: служба скажет их агенту словами.
// Нет такого разворота — прежний ErrNoSpread (preview.go). Любая другая ошибка Host становится «внутренней ошибкой программы», подробности — в log.txt.
var (
	ErrAskBusy      error = errAskBusy      // открыт другой вопрос
	ErrNotReady     error = errNotReady     // окно ещё не готово
	ErrNoAlbum      error = errNoAlbum      // альбома нет
	ErrAlbumChanged error = errAlbumChanged // ключ не совпал: открыли другой альбом
	ErrAlbumEdited  error = errAlbumEdited  // ревизия не совпала: альбом правили
	ErrScanning     error = errScanning     // лента ещё читается, срок ожидания вышел
	ErrNoFree       error = errNoFree       // свободных фото нет
	ErrLate         error = errLate         // срок вызова вышел до записи: правка не применена
	ErrUndoNotOwn   error = errUndoNotOwn   // после правки агента альбом менялся
	ErrBackup       error = errBackup       // резервная копия не сделана
	ErrNoEdits      error = errNoEdits      // правки агентам выключены (флажок сняли, пока правка ждала)
	ErrBusy         error = errProgBusy     // программа занята выгрузкой по ссылке (152); выгрузка в JPEG — ErrExporting
	ErrOpenRemote   error = errOpenRemote   // альбом не на этом компьютере (UNC): агенту не открывать
	ErrOpenFailed   error = errOpenFailed   // файл не открылся (битый, чужой, недоступный)
	ErrOpenNewer    error = errOpenNewer    // файл новее программы
	ErrNoRecent     error = errNoRecent     // ключа нет среди недавних
	ErrExporting    error = errExporting    // идёт выгрузка: альбомы не создаются и не открываются
)

// ExportFail — отказ проверок выгрузки текстом окна «Экспорт»: Host отдаёт его агенту как есть, поэтому
// текст обязан быть без пути и имён файлов (TestExportCheckErrorsNoPath).
func ExportFail(text string) error { return newAgentErr(text) }

// AlbumKey — ключ альбома по номеру Editor (0 — альбома нет, ключ пуст): фасад сверяет с ним
// AlbumRef.Key агента.
func AlbumKey(ed uint64) string { return albumKey(ed) }

// canEdit — Host умеет правки: заполнены все замыкания ступени 3. Иначе инструментов правки нет.
func (h Host) canEdit() bool {
	return h.Perm != nil && h.Scanning != nil && h.Busy != nil && h.Backup != nil && h.Ask != nil &&
		h.Auto != nil && h.Vary != nil && h.Show != nil && h.Undo != nil
}
