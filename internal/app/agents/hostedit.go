package agents

// Правки альбома агентом (149, ступень 3). Служба Editor не получает: всё идёт замыканиями Host,
// которые заполняет фасад App (ключ и rev сверяются там под замком Editor — design §2.1).
// Типы запросов и ответов — только числа, bool, строки и срезы строк: через них наружу не уйдёт
// ни путь, ни замыкание, ни карта (TestHostTypesScalar). Полей с Dir и Path в имени нет.

// Виды вопроса (AskReq.Kind).
const (
	AskCreate = "create" // «Создать альбом по просьбе агента?»
	AskAdd    = "add"    // «Добавить папку по просьбе агента?»
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
	Kind   string // AskCreate | AskAdd
	Client string // имя клиента, очищено; "" — неизвестный клиент
	Hint   string // folder_hint как прислал агент (≤ 4096 байт): подсказка окну выбора, не путь для применения
	Key    string // AskAdd: ключ альбома, куда просят добавить
	Rev    int    // AskAdd: ревизия, которую видел агент
}

// AskRes — итог вопроса. Why — одно из Why*; при WhyAnswered остальные поля заполнены.
type AskRes struct {
	Why      string
	Album    AlbumRef // ключ и ревизия после действия
	Folder   int      // номер папки в альбоме с 1
	Merged   bool     // папка уже была в альбоме: шага отмены нет
	Scanning bool     // лента ещё читает папку
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
)

// AlbumKey — ключ альбома по номеру Editor (0 — альбома нет, ключ пуст): фасад сверяет с ним
// AlbumRef.Key агента.
func AlbumKey(ed uint64) string { return albumKey(ed) }

// canEdit — Host умеет правки: заполнены все замыкания ступени 3. Иначе инструментов правки нет.
func (h Host) canEdit() bool {
	return h.EditState != nil && h.Scanning != nil && h.Busy != nil && h.Backup != nil && h.Ask != nil &&
		h.Auto != nil && h.Vary != nil && h.Show != nil && h.Undo != nil
}
