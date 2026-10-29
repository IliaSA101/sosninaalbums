package agents

import (
	"context"
	"log"
	"net"
	"reflect"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/peercred"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Состояния службы (State).
const (
	StateOff   = "off"
	StateOn    = "on"
	StateError = "error"
)

// Пределы канала (design §2). Срок записи, KeepAlive и прочие — поля Options: тесты ставят малые.
const (
	maxLine      = 1 << 20 // строка запроса агента, байт
	maxConns     = 16      // одновременных подключений
	writeTimeout = 15 * time.Second
	keepAlive    = 30 * time.Second
	initTimeout  = 10 * time.Second // от подключения до initialize
	closeWait    = time.Second      // сколько Close ждёт горутины
)

// Host — что службе нужно от App. Все поля обязательны (New паникует с именем пустого, как
// updater.New). Редактора здесь нет и не будет: служба только читает (design §5); список полей —
// замок TestHostFields, новое поле — правка теста и ревью безопасности. Поля чтения — копии или
// снимки: ничего из них не правит проект и не пишет на диск.
type Host struct {
	CfgDir  func() string // папка config.json
	Version func() string // версия программы
	Sock    func() string // путь сокета (osx.MCPSock); "" — папки данных нет
	Changed func()        // сменилось состояние или список подключённых (App шлёт agents:state)

	View         func() (v model.View, edID uint64)                  // последний отданный окну вид или Editor.View(); edID 0 — альбома нет
	Photos       func() []library.Photo                              // фото ленты, копия под замком App
	Marks        func() map[string]model.MarkView                    // свои отметки и подборки фото
	Status       func() (missing []string, unread map[string]string) // пропавшие и нечитаемые фото
	Decor        func(kind string) (DecorCatalogLite, error)         // каталог оформления: bg, sticker, frame, spread, look, template
	Packs        func() (PacksLite, error)                           // наборы библиотеки, без сети
	Prints       func() []printprof.Info                             // профили типографий
	Recent       func() []prefs.RecentProject                        // «Последние» альбомы
	FontsMissing func(used []model.FontUse) []string                 // названия своих гарнитур альбома, которых нет на этом компьютере
	// Preview — фото photoID, уменьшенное до px по длинной стороне: JPEG не больше maxJPEG (EncodePreview);
	// источник — как у /media/p (CheckLocalPath, том, пропавшее). Файл недоступен — ErrUnavailable.
	// Служба перекодирует ответ сама (метаданные в нём не нужны и не пройдут), ответ больше px — отказ,
	// после срока ctx — отбрасывается. Ошибки — без путей и имён файлов в тексте: `bare` чистит PathError,
	// но не fmt.Errorf("… %s", path), а текст идёт в log.txt (TestAgentsLogClean, Г5).
	Preview func(ctx context.Context, photoID string, px int) ([]byte, error)
	// Render — разворот spreadID (id из вида; model.CoverID — обложка) шириной не больше px: JPEG вида печати,
	// недоступные фото — пустые места. Тяжёлое: службе нужен ctx, а App не держит под a.mu ни рендер, ни
	// чтение файлов (design §5); рендеры службы идут по одному. Ошибки render пишут имя файла фото
	// (render.go, rotated.go) — Host заменяет их на безымянные (имена фото в log.txt не нужны).
	Render func(ctx context.Context, spreadID string, px int) ([]byte, error)
}

// Options — пределы канала; нулевое значение — рабочее.
type Options struct {
	WriteTimeout time.Duration // срок одной записи в подключение (15 с)
	KeepAlive    time.Duration // пинг клиента (30 с)
	InitTimeout  time.Duration // до initialize (10 с)
	CloseWait    time.Duration // ожидание горутин в Close (1 с)
	MaxConns     int           // подключений (16)
	MaxCalls     int           // одновременных вызовов инструментов (4)
	CallTimeout  time.Duration // срок одного вызова (10 с)
	// OnCall — вызов агента записан в журнал (today — вызовов за сегодня); App шлёт agents:call.
	// Зовётся из горутины обработчика, не под замками службы.
	OnCall func(c Call, today int)
	// Peer — сверка собеседника подключения (148): true — учётная запись ОС та же, что у программы.
	// По умолчанию peercred.Same; тесты подменяют (глобальной переменной нет — гонок в -race тоже).
	Peer func(net.Conn) (bool, error)
	// Setup — доработка сервера до первого подключения: тесты и таблица инструментов (Г2).
	Setup func(*mcp.Server)
}

func (o Options) withDefaults() Options {
	def := func(v *time.Duration, d time.Duration) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&o.WriteTimeout, writeTimeout)
	def(&o.KeepAlive, keepAlive)
	def(&o.InitTimeout, initTimeout)
	def(&o.CloseWait, closeWait)
	def(&o.CallTimeout, callTimeout)
	if o.Peer == nil {
		o.Peer = peercred.Same
	}
	if o.MaxCalls <= 0 {
		o.MaxCalls = maxCalls
	}
	if o.MaxConns <= 0 {
		o.MaxConns = maxConns
	}
	return o
}

// Service — служба агентов: слушает сокет ОС и ведёт сессии MCP. Пока не Start — ничего не
// делает и на диске не трогает. Wails и App ей недоступны: всё через Host.
type Service struct {
	h    Host
	o    Options
	life sync.Mutex // весь Start и весь Close: между закрытием и открытием сокета других нет
	mu   sync.Mutex // поля ниже
	run  *run       // нет — выключено
	st   string
	err  string
	jr   journal // журнал вызовов (journal.go)
}

// New собирает службу; пустое поле Host — паника с его именем.
func New(h Host, o Options) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("agents.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h, o: o.withDefaults(), st: StateOff}
}

// StartIfOn — запуск программы: открывает доступ, если он включён в config.json. Выключен или
// папки данных нет — не делает ничего (на диске тоже).
func (s *Service) StartIfOn() error {
	dir := s.h.CfgDir()
	if dir == "" || !prefs.AgentsOf(dir).On {
		return nil
	}
	return s.Start()
}

// Start открывает сокет (design §2 «Запуск»). Уже открыт — nil. Отказ — State «error» с причиной
// словами; On в config.json служба не пишет (это делает фасад по итогу Start).
func (s *Service) Start() error {
	s.life.Lock()
	err := s.start()
	s.life.Unlock()
	s.changed()
	return err
}

func (s *Service) start() error {
	s.mu.Lock()
	running := s.run != nil
	s.mu.Unlock()
	if running {
		return nil
	}
	ln, err := s.open()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.st, s.err = StateError, err.Error()
		return err
	}
	s.run, s.st, s.err = s.newRun(ln), StateOn, ""
	return nil
}

// Close закрывает доступ: слушатель (файл сокета исчезает), все подключения; горутины ждёт не
// дольше CloseWait — рендер или запись клиенту, который не читает, выход не держат.
func (s *Service) Close() {
	s.life.Lock()
	s.mu.Lock()
	r := s.run
	s.run, s.st, s.err = nil, StateOff, ""
	s.mu.Unlock()
	if r != nil {
		r.close()
	}
	s.life.Unlock()
	s.changed()
}

// fail — цикл приёма умер от паники: служба больше не слушает, состояние «error».
func (s *Service) fail(r *run) {
	s.mu.Lock()
	if s.run == r {
		s.run, s.st, s.err = nil, StateError, errOpen.Error()
	}
	s.mu.Unlock()
	go func() { // close ждёт и саму эту горутину
		defer guardPanic("fail")
		r.close()
		s.changed()
	}()
}

// State — состояние и причина отказа (для «error»).
func (s *Service) State() (state, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st, s.err
}

// Client — подключённый клиент: clientInfo.name из initialize и время.
type Client struct {
	Name  string
	Since time.Time
}

// Clients — подключённые сейчас (прошедшие initialize), в порядке подключения.
func (s *Service) Clients() []Client {
	s.mu.Lock()
	r := s.run
	s.mu.Unlock()
	if r == nil {
		return []Client{}
	}
	return r.clients()
}

// changed сообщает App; паника обратного вызова не должна ронять ни службу, ни SDK.
func (s *Service) changed() {
	defer guardPanic("changed")
	s.h.Changed()
}

// newRun запускает приём подключений на слушателе.
func (s *Service) newRun(ln net.Listener) *run {
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{s: s, o: s.o, ln: ln, ctx: ctx, cancel: cancel,
		conns: map[net.Conn]struct{}{}, seen: map[*mcp.ServerSession]Client{}, gone: map[*mcp.ServerSession]struct{}{}}
	r.srv = mcp.NewServer(&mcp.Implementation{Name: "sosnina-albums", Version: s.h.Version()},
		&mcp.ServerOptions{KeepAlive: s.o.KeepAlive})
	registerTools(newToolEnv(s, r.srv))
	if s.o.Setup != nil {
		s.o.Setup(r.srv)
	}
	r.srv.AddReceivingMiddleware(unknownTool)
	r.srv.AddReceivingMiddleware(r.noteClient) // последним: внешний слой ловит и панику чужих слоёв
	r.wg.Add(1)
	go r.serve()
	log.Printf("agents: доступ открыт")
	return r
}
