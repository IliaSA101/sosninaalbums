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
	Origin    string `json:"origin"` // builtin | file | net
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
	Checked int64 `json:"checked"` // unix мс последней удачной проверки
	Skipped int   `json:"skipped"` // удалённых пользователем: «Обновить» их не ставит
}

// Remove — числа вопроса «Удалить набор?» (decor.PackRemove).
type Remove struct {
	Files int   `json:"files"`
	Kept  int   `json:"kept"`
	Bytes int64 `json:"bytes"`
}

// Skip — пропущенный предмет или набор и причина (в App — DecorSkip).
type Skip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// List — содержимое окна «Наборы»: Busy — идёт операция (кнопки неактивны). Вложенный DTO: в App
// он написан заново (PackList), пару стережёт TestDTOPairs.
type List struct {
	Packs    []Info `json:"packs"`
	Net      Net    `json:"net"`
	Busy     bool   `json:"busy"`
	ReadOnly bool   `json:"readOnly"`
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

// Update — итог «Обновить библиотеку»: новых и обновлённых наборов, пропущенное с причиной (нужна
// новая программа, предметы набора), байты скачанного. В App — LibraryUpdate (без titles); в
// событие library:updated уходит как есть.
type Update struct {
	Added   int      `json:"added"`
	Updated int      `json:"updated"`
	Skipped []Skip   `json:"skipped"`
	Bytes   int64    `json:"bytes"`
	titles  []string // названия поставленных и обновлённых — в library:changed; в JSON не идут
}

// Progress — событие library:progress: этап catalog | download | install, набор и байты.
type Progress struct {
	Stage string `json:"stage"`
	Title string `json:"title"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// Offer — событие library:offer: при запуске новое больше порога — тост «Скачать».
type Offer struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
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
	Now           func() time.Time       // часы вопроса «Заменить?»; тесты уводят их вперёд
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
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и colorpick.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("libpacks.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h}
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
