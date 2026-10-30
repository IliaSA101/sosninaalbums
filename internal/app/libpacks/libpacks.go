// Package libpacks — наборы библиотеки (спеки 067, 073, 079, 123, 129; разрез 126 Ш4): «Добавить
// из файла» с вопросом о замене, список и удаление наборов, «Обновить библиотеку» из сети и
// проверка при запуске. Разбор архива — decor/pack, содержимое и запись — decor.Store, сеть —
// decor/fetch; служба переносит манифест между ними (decor не импортирует pack — depguard) и
// держит одну операцию наборов за раз.
//
// Зовёт её internal/app (packs.go — фасад с методами для интерфейса). Wails и App службе
// недоступны: диалог файла, события, фон, проект открытого альбома и реестр типографий приходят
// через Host (поля-функции, замыкания над App — позднее связывание, подмены тестов действуют и
// после NewApp).
package libpacks

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Info — строка окна «Наборы библиотеки» (decor.PackInfo). App объявляет её определённым типом
// (`type PackInfo libpacks.Info`): Wails берёт пространство имён models.ts из имени пакета, и
// тип службы в сигнатуре метода App переехал бы в другое пространство (126-2).
type Info struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	About     string `json:"about"`
	Author    string `json:"author"`
	License   string `json:"license"`
	Source    string `json:"source"` // текст, не ссылка (решение 14)
	Origin    string `json:"origin"` // builtin | file | net | demo
	Version   int    `json:"version"`
	Bg        int    `json:"bg"`
	Sticker   int    `json:"sticker"`
	Spread    int    `json:"spread"`
	Frame     int    `json:"frame"` // 079: рамочки-картинки
	Print     int    `json:"print"` // 123: профили типографий
	Bytes     int64  `json:"bytes"`
	Removable bool   `json:"removable"`
}

// Net — согласие на сеть и проверка при запуске (prefs.LibraryNet без списка удалённых: наружу —
// только сколько их, для «Вернуть удалённые»).
type Net struct {
	Allowed bool  `json:"allowed"`
	AtStart bool  `json:"atStart"`
	Checked int64 `json:"checked"` // unix мс последней проверки обновлений (не даты списка «Можно скачать»: она — List.AvailAt)
	Skipped int   `json:"skipped"` // удалённых пользователем: «Обновить» и «Скачать все» их не ставят
	// Пример набора в программе (спека 142). DemoSkipped — пример удалён вами и не покрыт стоящим
	// полным набором (входит в Skipped); DemoBack — только в ответе «Вернуть»: пример встал этим
	// вызовом; DemoOffer — есть строка «Скачать» примера (указатель в пустом окне макетов).
	DemoSkipped bool `json:"demoSkipped"`
	DemoBack    bool `json:"demoBack"`
	DemoOffer   bool `json:"demoOffer"`
}

// Remove — числа вопроса «Удалить набор?» (decor.PackRemove). Addons — названия стоящих
// дополнений донора по сохранённому списку: останутся без обновлений (137-10); пусто — не
// знаем или нет, не nil.
type Remove struct {
	Files  int      `json:"files"`
	Kept   int      `json:"kept"`
	Bytes  int64    `json:"bytes"`
	Addons []string `json:"addons"`
}

// Avail — строка раздела «Можно скачать»: набор сохранённого списка, которого нет в библиотеке
// (137). Тексты — из подписанного каталога, прошли cleanText; адреса в строке нет.
type Avail struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	About       string   `json:"about"`
	License     string   `json:"license"`
	Version     int      `json:"version"`
	Bg          int      `json:"bg"`
	Sticker     int      `json:"sticker"`
	Spread      int      `json:"spread"`
	Frame       int      `json:"frame"`
	Print       int      `json:"print"`
	Bytes       int64    `json:"bytes"`       // вместе с дополнениями, которые придут с набором
	AddonTitles []string `json:"addonTitles"` // что придёт вместе (дополнения); не nil
	Donor       string   `json:"donor"`       // название стоящего донора у строки-дополнения, иначе ""
	Why         string   `json:"why"`         // непусто — «Скачать» серая, текст — подсказка
	Removed     bool     `json:"removed"`     // в пропуске: «удалён вами»
}

// Skip — пропущенный предмет или набор и причина (в App — DecorSkip).
type Skip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// List — содержимое окна «Наборы»: Busy — идёт операция (кнопки неактивны). Available — раздел
// «Можно скачать» из сохранённого списка (сеть не трогается), AvailAt — когда список записан
// (unix мс, время файла; 0 — списка нет), All — «Скачать все» (число строк и байты): подпись и
// план DownloadAll считает одна функция. Вложенный DTO: в App он написан заново (PackList),
// пару стережёт TestDTOPairs.
type List struct {
	Packs     []Info  `json:"packs"`
	Net       Net     `json:"net"`
	Busy      bool    `json:"busy"`
	ReadOnly  bool    `json:"readOnly"`
	Available []Avail `json:"available"`
	All       Offer   `json:"all"`
	AvailAt   int64   `json:"availAt"`
}

// Add — итог «Добавить из файла». Ask: "" — поставлен; same — эта версия уже стоит; update —
// стоит FromVersion, замена — Confirm(Token); older — стоит новее, не ставится.
type Add struct {
	Pack        Info   `json:"pack"`
	Ask         string `json:"ask"`
	FromVersion int    `json:"fromVersion"`
	Token       string `json:"token"`
	Skipped     []Skip `json:"skipped"`
}

// Update — итог «Обновить библиотеку», «Скачать» и «Скачать все»: новых и обновлённых наборов,
// пропущенное с причиной (нужна новая программа, предметы набора), не поставленные наборы
// (Failed — «повторите» или причина; в «Скачать» сюда же попадает нажатый набор, если он не
// встал), байты скачанного, Available — сколько строк «Скачать все» в разделе после операции
// (без серых и «удалён вами»). В App — LibraryUpdate (без titles).
type Update struct {
	Added     int               `json:"added"`
	Updated   int               `json:"updated"`
	Skipped   []Skip            `json:"skipped"`
	Bytes     int64             `json:"bytes"`
	Failed    []Skip            `json:"failed"`
	Available int               `json:"available"`
	Fresh     int               `json:"fresh"`  // новых строк раздела сверх прошлого списка
	Picked    bool              `json:"picked"` // «Скачать»: нажатый набор стоит после хода
	Why       string            `json:"why"`    // «Скачать»: почему не встал
	titles    []string          // названия поставленных и обновлённых — в library:changed; в JSON не идут
	done      []string          // id поставленных и обновлённых: «удалён вами» снимается с них
	why       map[string]string // id → почему набор не встал (итог «Скачать»)
}

// Progress — событие library:progress: этап catalog | download | install, набор и байты.
type Progress struct {
	Stage string `json:"stage"`
	Title string `json:"title"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// Offer — число наборов и их байты: «Скачать все» в списке, обновления больше порога и новые
// наборы в событии library:start.
type Offer struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Start — событие library:start: итог проверки при запуске одним событием (тост один, следующий
// стирает прежний): Updated — встало тихо, Offer — обновления больше порога (не скачаны), Fresh —
// новые наборы списка. Шлётся, если хоть одна часть не нулевая; неудачи — только в log.txt.
type Start struct {
	Updated int   `json:"updated"`
	Offer   Offer `json:"offer"`
	Fresh   Offer `json:"fresh"`
}

// Changed — данные library:changed: названия (Info.Title) поставленных и обновлённых наборов,
// чтобы окно «Фоны/Стикеры/Развороты» встало на тему нового (решение 073-1). При удалении —
// пустой список (не null).
type Changed struct {
	Installed []string `json:"installed"`
}

// TTL — сколько ждёт ответа вопрос «Заменить набор?»: распакованное держит место на диске. Его
// же ждёт вопрос «Заменить типографию?» (printprof_file.go).
const TTL = 10 * time.Minute

// progressGap — library:progress не чаще 4 раз в секунду.
const progressGap = 250 * time.Millisecond

var (
	ErrBusy      = errors.New("библиотека уже обновляется")
	ErrStale     = errors.New("вопрос устарел — добавьте набор ещё раз")
	ErrFile      = errors.New("это не набор библиотеки (.zip)")
	ErrNoConsent = errors.New("обращение к сети не разрешено — нажмите «Обновить библиотеку» и разрешите")
	// ErrGone — «Скачать» набору, которого нет в свежем списке; список при этом уже обновлён.
	ErrGone = errors.New("набора больше нет в списке — список обновлён")
	// ErrRemote — без пути: ошибка CheckLocalPath несёт его целиком, а тост его не показывает.
	ErrRemote = errors.New("файл набора не на локальном диске")
)

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем
// пустого). App отдаёт настоящие значения; тест пересобирает службу с подменой до первого вызова.
type Host struct {
	CfgDir  func() string                  // папка настроек: config.json (согласие, пропуски, seq каталога)
	Version func() string                  // версия программы: minApp наборов
	Store   func() (*decor.Store, error)   // библиотека; ошибка — startup до неё не дошёл
	Emit    func(name string, data ...any) // событие окну: library:*
	GoBG    func(fn func()) bool           // фон под WaitGroup App; false — App закрывается
	StopCtx func() context.Context         // контекст выхода программы: родитель сети и таймеров
	// AlbumLibs — ключи библиотеки открытого альбома (и в шагах отмены): удаление и замена их не
	// уносят, иначе Ctrl+Y вернул бы фон или стикер без файла (ревью 067).
	AlbumLibs func() func(key string) bool
	// Relink находит пропавшие своего альбома после установки (RelinkLib — шаг, если нашлось).
	Relink func(s *decor.Store)
	// PrintsChanged — набор мог принести или унести типографию: реестр собирается заново до
	// события окна.
	PrintsChanged func()
	PickFile      func() (string, error) // «Набор библиотеки» *.zip; "" — отмена; последняя папка — у App
	RememberDir   func(dir string)       // последняя папка диалога «Добавить из файла»
	OpenDir       func(dir string) error // папка в проводнике
	Fetcher       func() *fetch.Client   // сетевой клиент; тесты подменяют своим сервером
	Demo          func() Demo            // пример набора в программе (спека 142); пустой ID — выключен
	Now           func() time.Time       // часы вопроса «Заменить?» и пауз образцов; тесты уводят их вперёд
	// Rename — переименование скачанного листа образцов (os.Rename; тест подменяет, чтобы проверить
	// уборку временного файла при отказе).
	Rename func(oldPath, newPath string) error
	// Start — проверка при запуске (решение 17): задержка после открытия окна, период и порог
	// размера, после которого вместо скачивания только предложение. Читается при каждом вызове.
	Start func() (delay, every time.Duration, offerOver int64)
	// Крючки тестов гонки отзыва: BeforeBegin — между первой проверкой согласия и begin;
	// AfterNetWrite — в SetNet между записью согласия и остановкой операции.
	BeforeBegin   func()
	AfterNetWrite func()
}

// Service — одна операция наборов за раз, вопрос «Заменить?», отмена идущего обновления и
// проверка при запуске раз за сеанс.
type Service struct {
	h Host

	mu   sync.Mutex // TryLock: вторая операция — ErrBusy, а не ожидание
	busy atomic.Bool

	pendMu  sync.Mutex
	pending *pendingPack

	netMu   sync.Mutex
	cancel  context.CancelFunc
	started atomic.Bool

	// Сохранённый список «Можно скачать» (cache.go). Чтение файла и запись с подменой cat идут под
	// одним catMu, loaded — отдельно от cat == nil (безопасность Д1): ленивое чтение не затирает
	// свежий список и не повторяется после неудачи.
	catMu  sync.Mutex
	cat    *fetch.Catalog // nil — списка нет; после записи Env обнулён
	catAt  time.Time      // время файла списка
	catBad bool           // файл есть, но не читается: проверка при запуске не объявляет «новые»
	loaded bool

	consMu sync.Mutex // согласие на сеть в памяти (consent.go)
	cons   consent

	demoAsset demoAsset // адрес ассета примера: проверяется один раз (demo.go)

	// Образцы наборов (спека 144): задание, пауза, плохие листы — prevjob.go; кэш клеток — cells.go.
	// prevMu держит задание и уборку (TryLock); отмена задания — prevCancel под netMu.
	prevMu     sync.Mutex
	prevCancel context.CancelFunc
	prev       prevState
	cells      *cellCache

	// Крючки внутренних тестов (в программе nil): между Lstat листа и его открытием; разборов JPEG.
	afterLstat func()
	decodes    atomic.Int64
	lpMiss     atomic.Int64 // промахи /media/lp/: журнал — первый и каждый сотый
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и colorpick.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("libpacks.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h, cells: newCellCache()}
}

// ResetSession — новый сеанс: проверка при запуске снова разрешена (тесты зовут вместо перезапуска
// программы).
func (s *Service) ResetSession() { s.started.Store(false) }

// Started — проверка при запуске уже занята в этом сеансе (флаг раз за сеанс).
func (s *Service) Started() bool { return s.started.Load() }

// Hold занимает службу как операция наборов (для сквозных тестов «занято»): ok == false — уже
// занята; release отпускает.
func (s *Service) Hold() (release func(), ok bool) {
	if !s.lock() {
		return nil, false
	}
	return s.unlock, true
}

// Cancelable — у обновления есть, что отменять: операция встала в begin и ещё не закончилась.
func (s *Service) Cancelable() bool {
	s.netMu.Lock()
	defer s.netMu.Unlock()
	return s.cancel != nil
}
