package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/colorpick"
	"github.com/IliaSA101/sosninaalbums/internal/app/edit"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/keymap"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	"github.com/IliaSA101/sosninaalbums/internal/text/fontlib"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

const projectExt = ".salbum"

// App — всё, что вызывает интерфейс. Каждый публичный метод доступен из JS.
type App struct {
	// Methods — методы правок без своих данных (пакет edit): Wails видит их как методы App.
	*edit.Methods

	ctx    context.Context
	cfgDir string
	cache  *library.Cache
	// uilog — журнал ошибок интерфейса (LogUI, 092): окно в минуту, повторы счётчиком.
	uilog uiLog
	// data — имя папок данных (osx.DataName): у стенда QA (SA_STAND) своё, задаёт Run до окна.
	data string

	// ed подменяется при «Новый»/«Открыть», а читается из обработчиков /media,
	// автосохранения и методов UI одновременно — поэтому только через editor().
	ed atomic.Pointer[album.Editor]
	// clip — буфер «Копировать и вставить» (099, clip.go): снимок привязан к Editor.
	clip clipBuf
	// markups — кэш превью PSD разметок обложки и разворотов (markup.go), по записи на вид.
	markups [2]markupCache

	mu     sync.Mutex
	photos []library.Photo
	byID   map[string]library.Photo
	warm   context.CancelFunc
	// layouts — идущие прогревы раскладок (warmEditor); миниатюры ждут их (warmThumbs).
	layouts busyGate
	// wire — цепочка видов для интерфейса (075, wire.go): последний отданный вид и номер.
	wire wireState
	// sig — подписи кадров для автосборки и их фоновая подготовка (078, sigs.go), под mu.
	sig sigState

	// missing — ID фото альбома, чьих файлов нет на диске (см. refreshMissing), по порядку; под mu,
	// как и missGen — поколение полного пересчёта, и missRefs — ID → путь фото альбома, за которым
	// следит missing. missPending — сколько пересчётов в фоне (ждут тесты).
	// missSyncMu — снимок refs и его запись в missRefs одним куском (порядок: missSyncMu → emitMu → mu):
	// снимок старше не ляжет поверх нового. missEd и missRev — последняя правка, которую видел
	// syncMissing, под missSyncMu; редактор слабой ссылкой — не держит прежний альбом со стеком отмены.
	missing     []string
	missGen     int
	missPending atomic.Int32
	missRefs    map[string]string
	missSyncMu  sync.Mutex
	missEd      weak.Pointer[album.Editor]
	missRev     int
	exists      func(path string) bool // файл фото на месте; подменяется в тестах
	// snapRefs — ID → путь фото редактора; подменяется в тестах (порядок снимков).
	snapRefs func(ed *album.Editor) map[string]string

	// unread — отдельные файлы, найденные, но не прочитанные: ID фото → причина (117, unread.go); под mu.
	unread map[string]string
	// oursSeen — ключи файлов выгрузки программы прошлого законченного чтения папок (121, ours.go); под mu.
	oursSeen map[string]bool

	// Сканирование папок (см. rescan): поколение растёт с каждым запуском, отчёты устаревших
	// поколений отбрасываются. scan и scanGen — под mu.
	// rescanMu — снимок папок редактора и новое поколение одним куском (порядок: rescanMu →
	// замок Editor → mu): Folders ждёт замка редактора (правка) не под mu.
	rescanMu   sync.Mutex
	scanner    *library.Scanner
	scanCancel context.CancelFunc
	scanGen    int
	scan       library.ScanProgress
	// scanning — чтение папок поколения scanGen ещё не закончило Scan (под mu); пока оно идёт,
	// сверка папок (foldercheck.go) и тихое чтение не начинаются.
	scanning bool
	check    folderCheck // ворота сверки папок (088)
	// emitMu сериализует события library/scan: порядок отправки совпадает с порядком обновления состояния.
	emitMu sync.Mutex
	emit   func(name string, data ...any) // отправка события в интерфейс; в тестах подменяется

	startNotice string   // см. State.Notice
	startInfo   string   // см. State.Info
	startFresh  []string // см. State.FreshFonts
	// startFromFile — см. State.FromFile; один раз, как startNotice (GetState сбрасывает).
	startFromFile bool

	// Открытие альбома извне (launch.go, спека 112), под mu: argFile — альбом из аргументов запуска
	// до startup (argTaken — startup его уже забрал), ext — очередь внешнего открытия (State.Pending).
	argFile  string
	argTaken bool
	ext      string
	// launchLog — строки лога initLaunch до открытия log.txt (flushLaunchLog в startup).
	launchLog []string

	// ready закрывает startup, когда редактор, startNotice и первое сканирование готовы.
	// Wails зовёт методы App параллельно с OnStartup: без него GetState отдал бы пустой
	// альбом раньше загрузки проекта.
	ready chan struct{}

	tplMu sync.Mutex // шаблоны сеток и templates.json (api.go, internal/prefs)
	tpls  prefs.Templates

	colorPick *colorpick.Picker // пипетка и «С фото» окна выбора цвета (colorpick, 109)

	// draftLocked — черновик не удалось убрать в .bad: писать в него нельзя, иначе затрём.
	draftLocked atomic.Bool
	// draftNewer — черновик записан более новой версией программы: в этой сессии
	// его не пишем, не переносим и не удаляем, чтобы новая версия смогла его открыть.
	draftNewer atomic.Bool
	rename     func(from, to string) error // os.Rename; подменяется в тестах

	saveTimer *time.Timer
	// saveMu сериализует автосохранение и даёт stopBackground дождаться уже идущей записи.
	saveMu sync.Mutex
	// stopCtx — родитель сканирования, прогрева и экспорта; stopped — отбой автосохранению.
	// Нужны остановке (stopBackground): ни одна фоновая горутина не живёт дольше App.
	stopCtx    context.Context
	stopCancel context.CancelFunc
	stopped    atomic.Bool
	bg         sync.WaitGroup // сканирование (с прогревом) и экспорт
	bgMu       sync.Mutex     // порядок goBG и closeBackground: Add не идёт вместе с Wait
	bgClosed   bool           // closeBackground начат: новый фон не стартует
	exporting  bool
	exportDir  string // папка последнего экспорта: её открывает ShowInExplorer
	// exportCancel — «Остановить» идущего экспорта (073), под mu; nil — выгрузки нет.
	exportCancel context.CancelFunc
	// Сбор в папку (069, collect.go), под mu: идёт ли, отмена идущего, папка последнего (ShowCollected).
	collecting    bool
	collectCancel context.CancelFunc
	collectDir    string

	mediaMiss atomic.Int64 // промахи /media, чтобы не писать в лог каждый
	mediaSt   mediaState   // снятые запросы /media и прежний реестр фото (media.go)
	about     aboutCache   // сведения о файлах для карточки фото (photoabout.go)

	backups *backup.Store // резервные копии альбома (files.go); nil — без копий

	// decorLib — библиотека фонов и стикеров (decor.go); nil до startup — методы отвечают ошибкой.
	decorLib atomic.Pointer[decor.Store]
	// fontLib — библиотека своих шрифтов (fonts.go, 082); nil до startup — методы отвечают ошибкой.
	fontLib atomic.Pointer[fontlib.Store]
	// fontsMu — набор библиотеки → регистрация в text → событие `fonts` одним шагом: два
	// параллельных publishFonts иначе отдали бы окну старый список последним.
	fontsMu sync.Mutex
	// packs — одна операция наборов за раз и набор, ждущий «Заменить» (packs.go).
	packs packState
	// net — «Обновить библиотеку»: отмена, проверка при запуске, источник (packs_net.go).
	net libNet
	// upd — обновление программы: состояние, проверка при запуске (update.go, спека 111).
	upd updState
	// assoc и newer — открытие .salbum двойным щелчком и файл новее программы (112).
	assoc assocState
	newer newerState
}

// State — то, что нужно интерфейсу целиком: альбом и фото из папок.
type State struct {
	View   model.View      `json:"view"`
	Photos []library.Photo `json:"photos"`
	// Scan — ход чтения папок: интерфейс, открывшийся посреди сканирования, не ждёт события.
	Scan library.ScanProgress `json:"scan"`
	// Notice — ошибка запуска (не открылся проект): интерфейс ещё не подписан на
	// событие `notice`, поэтому она ждёт первого GetState и отдаётся один раз.
	Notice string `json:"notice,omitempty"`
	// Info — разовое сообщение (тост вида info), не ошибка: «Добавлены шрифты из папки альбома»
	// (082) при запуске и в ответе открытия альбома; отдаётся один раз.
	Info string `json:"info,omitempty"`
	// FreshFonts — «<16 hex>.<ttf|otf>» начертаний, которые только что добавила папка «Шрифты»
	// альбома (082-8): окно проверяет, что открывает их, и откатывает отказавшие DropFonts, как
	// после ImportFonts. Разово, вместе с Info.
	FreshFonts []string `json:"freshFonts,omitempty"`
	// Missing — ID недостающих фото (файла нет на диске); обновляется событием `missing`.
	Missing []string `json:"missing"`
	// Unread — отдельные файлы, которые найдены, но не читаются: ID фото → готовая причина (117);
	// обновляется событием `unread` с тем же видом. Нет ключа — файл читается или чтение ещё идёт.
	Unread map[string]string `json:"unread,omitempty"`
	// Version — версия программы из wails.json.
	Version string `json:"version"`
	// Fonts — гарнитуры (встроенные и свои, 082): интерфейс подключает их по /media/font/<file>;
	// смена набора своих — событие `fonts`.
	Fonts []text.Family `json:"fonts"`
	// Prefs — уровень интерфейса и стартовое окно (config.json); Recent — последние альбомы.
	Prefs  prefs.Prefs           `json:"prefs"`
	Recent []prefs.RecentProject `json:"recent"`
	// Keymap — действующие горячие клавиши (074-25): известны с первого GetState, без
	// отдельного вызова, когда клавиши ещё молчат. Окно «Горячие клавиши» берёт Keymap().
	Keymap keymap.View `json:"keymap"`
	// Cancelled — пользователь закрыл нативный диалог, проект прежний. Сравнение path+rev
	// на фронте отмену не отличит: удачное открытие шлёт `view` раньше ответа метода.
	Cancelled bool `json:"cancelled,omitempty"`
	// Pending — имя альбома в очереди внешнего открытия (двойной щелчок, 112): событие `open:external`
	// могло прийти до подписки окна. Не очищается — очередь забирает OpenExternal.
	Pending string `json:"pending,omitempty"`
	// Newer — файл (или черновик) сохранён более новой версией: окно «Альбом из новой версии».
	// В ответах открытия идёт с Cancelled; при старте — в первом GetState, один раз (112).
	Newer *NewerInfo `json:"newer,omitempty"`
	// Assoc — открытие .salbum двойным щелчком: вопрос и пункт меню (112).
	Assoc AssocView `json:"assoc"`
	// FromFile — запуск двойным щелчком, и альбом из аргумента действительно открылся: стартовое окно
	// «Продолжить» поверх него лишнее (0.25-q112-start). Берётся из startup, а не из Assoc.FromFile:
	// тот на не-Windows пуст и ставится в фоне после первого GetState. Только в первом GetState.
	FromFile bool `json:"fromFile,omitempty"`
	// Stand — стенд QA (сборка dev/qa + SA_STAND): Go не открывает браузер и проводник, поэтому
	// интерфейс вместо ссылок показывает тост. В выпуске всегда false.
	Stand bool `json:"stand,omitempty"`
}

// standOn — признак стенда QA для State; подменяется в тестах (тег dev в обычном прогоне не задан).
var standOn = osx.StandOn

func NewApp() *App {
	a := &App{byID: map[string]library.Photo{}, rename: os.Rename, scanner: library.NewScanner(),
		ready: make(chan struct{}), exists: fileExists, snapRefs: editorPaths, data: osx.AppName}
	a.stopCtx, a.stopCancel = context.WithCancel(context.Background())
	a.emit = func(name string, data ...any) {
		if a.ctx != nil {
			wr.EventsEmit(a.ctx, name, data...)
		}
	}
	a.ed.Store(blankEditor(""))
	a.Methods = edit.New(a.editHost())
	a.colorPick = colorpick.New(a.colorHost())
	return a
}

// editor — текущий редактор; указатель держать недолго, не кэшировать в полях.
func (a *App) editor() *album.Editor { return a.ed.Load() }

func (a *App) startup(ctx context.Context) {
	// Wails зовёт OnStartup в горутине без recover: guard пишет панику в log.txt, затем
	// close (defer — в обратном порядке) отпускает окно, и оно получает пустой альбом.
	defer close(a.ready)
	defer guard("startup")
	a.ctx = ctx
	cfgDir, thumbsDir := dataDirs(a.data)
	a.cfgDir = cfgDir
	if err := makeDataDir(a.cfgDir); err != nil {
		log.Printf("папка настроек: %v", paths.NoPath(err)) // без неё лог и конфиг не пишутся, но программа работает
	}
	setupLog(filepath.Join(a.cfgDir, "log.txt"))
	log.Printf("запуск, версия %s", appVersion(wailsJSON))
	a.flushLaunchLog()                          // строки разбора аргументов, накопленные до открытия log.txt
	salbum.SetAppVersion(appVersion(wailsJSON)) // до первого Load и записи: поле app в файле (098)
	a.updateStartup()                           // итог прошлого запуска установщика — до первого UpdateInfo
	a.loadTemplates()
	a.startBackups()
	a.openDecor() // до первого Load: свои фоны альбома находят путь по ключу
	a.openFonts() // до первого Load: ширины «по тексту» — своими шрифтами

	c, err := library.NewCache(thumbsDir)
	if err != nil {
		log.Printf("кэш миниатюр: %v", paths.NoPath(err))
	}
	a.cache = c

	// Открываем последний проект, а если его нет — черновик.
	a.mu.Lock()
	fromFile := a.argFile != "" // двойной щелчок по альбому: вопрос о связи в этот раз не нужен
	arg := a.argFile
	a.mu.Unlock()
	path, notice := a.startEditor(a.loadStartup(a.loadConfig().LastProject))
	a.rememberRecent(path)
	notice = joinLaunchNotice(notice, launchPlace())
	a.mu.Lock()
	a.startNotice = notice
	a.startFromFile = openedArg(arg, path)
	a.mu.Unlock()
	a.logLaunchArg(arg, path)
	a.rescan()
	a.syncAssoc(fromFile) // связь .salbum (112): в фоне, окно не ждёт

	wr.OnFileDrop(ctx, func(x, y int, ps []string) { a.dropPaths(ps) })
}

func (a *App) shutdown(ctx context.Context) {
	// Сначала барьер, потом запись: правка между ними иначе не попала бы на диск.
	a.stopBackground()
	a.flushSave()
	a.uilog.flush() // итог окна журнала интерфейса — до строки «выход»
	log.Printf("выход")
}

// stopBackground отменяет сканирование, прогрев и экспорт и гасит автосохранение; после него таймер
// ничего не пишет. Возвращается, когда уже идущая запись автосохранения закончилась; остальные
// горутины сами видят отмену — их дожидается closeBackground.
func (a *App) stopBackground() {
	a.stopped.Store(true)
	a.stopCancel()
	a.mu.Lock()
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.mu.Unlock()
	a.saveMu.Lock()   // барьер: запись, начатая до stopped, дописывается
	a.saveMu.Unlock() //nolint:staticcheck // пустая секция — это и есть ожидание
}

// closeBackground — stopBackground и ожидание всех фоновых горутин, в том числе чистки кэша.
// После него App не трогает диск: тесты удаляют свои папки сразу за этим.
func (a *App) closeBackground() {
	a.stopBackground()
	a.bgMu.Lock()
	a.bgClosed = true
	a.bgMu.Unlock()
	a.bg.Wait()
	if a.cache != nil {
		a.cache.Close()
	}
}

// goBG запускает фон под ожиданием closeBackground. Голый bg.Go из горутины, не входящей в bg
// (таймер автосохранения, обработчик окна), мог выполнить Add(1) из нуля в тот миг, когда Wait
// просыпается, — паника WaitGroup «Add called concurrently with Wait». После closeBackground фон
// не стартует (false): диск уже освобождён, тесты удаляют папки.
func (a *App) goBG(fn func()) bool {
	a.bgMu.Lock()
	defer a.bgMu.Unlock()
	if a.bgClosed {
		return false
	}
	a.bg.Go(fn)
	return true
}

// Общие с edit: паника в методе — в лог, а не в падение окна; ошибки правок.
var (
	guard            = edit.Guard
	guardErr         = edit.GuardErr
	errInternal      = edit.ErrInternal
	errNoPhoto       = edit.ErrNoPhoto
	errBackdropScope = edit.ErrBackdropScope
)

// guardEdit — guard для методов правок (см. edit.Fail): recover зовёт сам метод.
func (a *App) guardEdit(name string, v *model.View, err *error) {
	if r := recover(); r != nil {
		edit.Fail(a.editHost(), r, name, v, err)
	}
}

// editHost — что пакету edit нужно от App.
func (a *App) editHost() edit.Host {
	return edit.Host{Editor: a.editor, Changed: a.changed, Notice: a.notice, Infos: a.infos,
		InfosOrRefs: a.infosOrRefs, Templates: a.templateList}
}

// domReady — окно открыто: размер под экран и проверка библиотеки при запуске (067 Ф3).
func (a *App) domReady(ctx context.Context) {
	defer guard("domReady")
	a.fitScreen(ctx)
	a.scheduleLibraryCheck()
	a.scheduleUpdateCheck()
}

// Постоянный минимум окна (логические px); на малом экране его уменьшает minSize.
const (
	minWinW = 1200
	minWinH = 640
)

// minSize — минимум окна под экран sw×sh (логические px, 089-D16): Wails умножает минимум
// на масштаб, и при 125 % на 1366×768 постоянный 1200×640 шире и выше экрана — окно
// не сжать до экрана. Неизвестный размер экрана — постоянный минимум.
func minSize(sw, sh int) (w, h int) {
	if sw <= 0 || sh <= 0 {
		return minWinW, minWinH
	}
	return max(1, min(minWinW, sw-40)), max(1, min(minWinH, sh-120))
}

// fitScreen: если обычный (не развёрнутый) размер окна больше экрана —
// уменьшаем его, иначе после «Свернуть в окно» окно выглядит развёрнутым
// и его не за что взять, чтобы перетащить на другой монитор.
// Минимум ставим раньше: WindowSetSize не даёт окну стать меньше минимума.
func (a *App) fitScreen(ctx context.Context) {
	defer guard("fitScreen")
	screens, err := wr.ScreenGetAll(ctx)
	if err != nil {
		return
	}
	for _, s := range screens {
		if !s.IsCurrent {
			continue
		}
		mw, mh := minSize(s.Size.Width, s.Size.Height)
		wr.WindowSetMinSize(ctx, mw, mh)
		w, h := wr.WindowGetSize(ctx)
		maxW, maxH := s.Size.Width-80, s.Size.Height-120
		if w > maxW || h > maxH {
			wasMax := wr.WindowIsMaximised(ctx)
			if wasMax {
				wr.WindowUnmaximise(ctx)
			}
			wr.WindowSetSize(ctx, min(w, maxW), min(h, maxH))
			wr.WindowCenter(ctx)
			if wasMax {
				wr.WindowMaximise(ctx)
			}
		}
	}
}

// Стабильный идентификатор приложения для блокировки второго экземпляра. Один во всех
// версиях: иначе 0.6 и 0.7 запустятся вместе и поделят config.json и черновик
// (межпроцессного замка нет сознательно, спека 019 §C). Сторож — TestInstanceIDStable.
const instanceID = "6f1c2b7e-3a4d-4e58-9b0a-5d2e8c7f1a93"

// instanceKey — замок второго экземпляра: у стенда QA свой, чтобы 2–3 копии wails dev
// открылись рядом (папки данных у них тоже свои, см. dataDirs).
func instanceKey(stand string) string {
	if stand == "" {
		return instanceID
	}
	return instanceID + "-stand-" + stand
}

// dataDirs — папка настроек (config.json, templates.json, Библиотека, черновик, log.txt) и кэша
// миниатюр; name — osx.DataName.
func dataDirs(name string) (cfg, thumbs string) {
	c, _ := os.UserConfigDir()
	k, _ := os.UserCacheDir()
	return filepath.Join(c, name), filepath.Join(k, name, "thumbs")
}

// quietTransport выключает keep-alive у транспорта. В wails dev обратный прокси к Vite идёт через
// http.DefaultTransport (в exe прокси нет; свой клиент библиотеки — decor/fetch, со своим транспортом). Vite пишет
// «400 Bad Request» в простаивающее keep-alive соединение, и net/http сыплет в log.txt
// «Unsolicited response received on idle HTTP channel». Без повторного использования
// соединений простаивающих нет; WebSocket HMR не страдает: Upgrade-запросы Transport
// и так не помечает «Connection: close».
func quietTransport(t *http.Transport) {
	t.DisableKeepAlives = true
}

// Run открывает окно и держит его до закрытия. Это функция, а не метод App:
// Wails отдаёт интерфейсу все экспортируемые методы App.
// assets — собранный фронт, wailsCfg — wails.json, icon — значок окна Linux (всё встраивает main).
func Run(assets fs.FS, wailsCfg, icon []byte) error {
	wailsJSON = wailsCfg
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		quietTransport(t)
	}
	// Неверное имя стенда — отказ, а не тихий запуск в настоящих настройках.
	stand, err := osx.Stand()
	if err != nil {
		return err
	}
	applyMemoryLimit()
	app := NewApp()
	app.data = osx.DataName(stand)
	app.initLaunch() // файл из аргументов и ожидание прежней версии — до окна
	app.useSystemAssoc()

	return wails.Run(&options.App{
		Title:            "Sosnina Albums",
		Width:            1280,
		Height:           760,
		MinWidth:         minWinW,
		MinHeight:        minWinH,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: app.media,
		},
		BackgroundColour: &options.RGBA{R: 38, G: 38, B: 40, A: 1},
		// Данные WebView2 (localStorage) не привязаны к имени exe: у каждой версии оно своё.
		Windows:     &windows.Options{WebviewUserDataPath: osx.WebViewDir(app.data)},
		Linux:       linuxOptions(app.data, icon),
		Mac:         &mac.Options{OnFileOpen: app.macOpen},
		Menu:        appMenu(runtime.GOOS),
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		OnStartup:   app.startup,
		OnDomReady:  app.domReady,
		OnShutdown:  app.shutdown,
		Bind:        []any{app},
		// Два окна пишут в один черновик и конфиг: второй запуск поднимает первое окно и отдаёт ему альбом.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               instanceKey(stand),
			OnSecondInstanceLaunch: app.secondLaunch,
		},
	})
}

// Единственный источник версии — wails.json (info.productVersion): его же читает wails build.
// Встраивает его main (embed не ходит вверх по папкам) и передаёт в Run.
var wailsJSON []byte

// appVersion достаёт версию из wails.json; при любой ошибке — "dev".
func appVersion(raw []byte) string {
	var cfg struct {
		Info struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil || cfg.Info.ProductVersion == "" {
		return "dev"
	}
	return cfg.Info.ProductVersion
}
