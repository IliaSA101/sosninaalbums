package app

import (
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

const projectExt = ".salbum"

// App — всё, что вызывает интерфейс. Каждый публичный метод доступен из JS.
type App struct {
	ctx    context.Context
	cfgDir string
	cache  *library.Cache

	// ed подменяется при «Новый»/«Открыть», а читается из обработчиков /media,
	// автосохранения и методов UI одновременно — поэтому только через editor().
	ed atomic.Pointer[album.Editor]

	mu     sync.Mutex
	photos []library.Photo
	byID   map[string]library.Photo
	warm   context.CancelFunc

	// missing — ID фото альбома, чьих файлов нет на диске (см. refreshMissing); под mu, как и
	// missGen — поколение пересчёта. missPending — сколько пересчётов в фоне (ждут тесты).
	missing     []string
	missGen     int
	missPending atomic.Int32
	exists      func(path string) bool // файл фото на месте; подменяется в тестах

	// Сканирование папок (см. rescan): поколение растёт с каждым запуском, отчёты устаревших
	// поколений отбрасываются. scan и scanGen — под mu.
	scanner    *library.Scanner
	scanCancel context.CancelFunc
	scanGen    int
	scan       library.ScanProgress
	// emitMu сериализует события library/scan: порядок отправки совпадает с порядком обновления состояния.
	emitMu sync.Mutex
	emit   func(name string, data ...any) // отправка события в интерфейс; в тестах подменяется

	startNotice string // см. State.Notice

	// ready закрывает startup, когда редактор, startNotice и первое сканирование готовы.
	// Wails зовёт методы App параллельно с OnStartup: без него GetState отдал бы пустой
	// альбом раньше загрузки проекта.
	ready chan struct{}

	tplMu sync.Mutex // шаблоны сеток и templates.json (templates.go, internal/prefs)
	tpls  prefs.Templates

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
	exporting  bool
	exportDir  string // папка последнего экспорта: её открывает ShowInExplorer

	mediaMiss atomic.Int64 // промахи /media, чтобы не писать в лог каждый

	backups *backup.Store // резервные копии альбома (backup.go); nil — без копий
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
	// Missing — ID недостающих фото (файла нет на диске); обновляется событием `missing`.
	Missing []string `json:"missing"`
	// Version — версия программы из wails.json.
	Version string `json:"version"`
	// Fonts — встроенные гарнитуры: интерфейс подключает их по /media/font/<file>.
	Fonts []text.Family `json:"fonts"`
	// Prefs — уровень интерфейса и стартовое окно (config.json); Recent — последние альбомы.
	Prefs  prefs.Prefs           `json:"prefs"`
	Recent []prefs.RecentProject `json:"recent"`
}

func NewApp() *App {
	a := &App{byID: map[string]library.Photo{}, rename: os.Rename, scanner: library.NewScanner(),
		ready: make(chan struct{}), exists: fileExists}
	a.stopCtx, a.stopCancel = context.WithCancel(context.Background())
	a.emit = func(name string, data ...any) {
		if a.ctx != nil {
			wr.EventsEmit(a.ctx, name, data...)
		}
	}
	a.ed.Store(blankEditor(""))
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
	cfg, _ := os.UserConfigDir()
	a.cfgDir = filepath.Join(cfg, "SosninaAlbums")
	if err := os.MkdirAll(a.cfgDir, 0o755); err != nil {
		log.Printf("папка настроек: %v", paths.NoPath(err)) // без неё лог и конфиг не пишутся, но программа работает
	}
	setupLog(filepath.Join(a.cfgDir, "log.txt"))
	log.Printf("запуск, версия %s", appVersion(wailsJSON))
	a.loadTemplates()
	a.startBackups()

	cacheDir, _ := os.UserCacheDir()
	c, err := library.NewCache(filepath.Join(cacheDir, "SosninaAlbums", "thumbs"))
	if err != nil {
		log.Printf("кэш миниатюр: %v", paths.NoPath(err))
	}
	a.cache = c

	// Открываем последний проект, а если его нет — черновик.
	path, notice := a.startEditor(a.loadStartup(a.loadConfig().LastProject))
	a.rememberRecent(path)
	a.mu.Lock()
	a.startNotice = notice
	a.mu.Unlock()
	a.rescan()

	wr.OnFileDrop(ctx, func(x, y int, paths []string) {
		for _, p := range paths {
			if st, err := os.Stat(p); err == nil && st.IsDir() {
				v, err := a.editor().AddFolder(p)
				if err != nil {
					a.notice(err.Error())
					continue
				}
				a.changed(v)
			}
		}
		a.rescan()
	})
}

func (a *App) shutdown(ctx context.Context) {
	// Сначала барьер, потом запись: правка между ними иначе не попала бы на диск.
	a.stopBackground()
	a.flushSave()
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
	a.bg.Wait()
	if a.cache != nil {
		a.cache.Close()
	}
}

// guard превращает панику в ошибку в логе, чтобы окно не падало молча.
func guard(name string) {
	if r := recover(); r != nil {
		log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
	}
}

// guardErr — guard для методов с ошибкой: паника уходит в лог, а вызывающий получает
// errInternal, а не nil (иначе интерфейс счёл бы операцию успешной).
func guardErr(name string, err *error) {
	if r := recover(); r != nil {
		log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
		*err = errInternal
	}
}

// fitScreen: если обычный (не развёрнутый) размер окна больше экрана —
// уменьшаем его, иначе после «Свернуть в окно» окно выглядит развёрнутым
// и его не за что взять, чтобы перетащить на другой монитор.
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

// Run открывает окно и держит его до закрытия. Это функция, а не метод App:
// Wails отдаёт интерфейсу все экспортируемые методы App.
// assets — собранный фронт, wailsCfg — wails.json (оба встраивает main).
func Run(assets fs.FS, wailsCfg []byte) error {
	wailsJSON = wailsCfg
	app := NewApp()

	return wails.Run(&options.App{
		Title:            "Sosnina Albums",
		Width:            1280,
		Height:           760,
		MinWidth:         1200,
		MinHeight:        640,
		WindowStartState: options.Maximised,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: app.media,
		},
		BackgroundColour: &options.RGBA{R: 38, G: 38, B: 40, A: 1},
		// Данные WebView2 (localStorage) не привязаны к имени exe: у каждой версии оно своё.
		Windows:     &windows.Options{WebviewUserDataPath: osx.WebViewDir()},
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		OnStartup:   app.startup,
		OnDomReady:  app.fitScreen,
		OnShutdown:  app.shutdown,
		Bind:        []any{app},
		// Два окна пишут в один черновик и конфиг: второй запуск лишь поднимает первое окно.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID,
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if app.ctx == nil {
					return
				}
				wr.WindowUnminimise(app.ctx)
				wr.WindowShow(app.ctx)
			},
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

const maxLogSize = 2 << 20

// setupLog пишет лог в файл и, если есть консоль, в stderr; файл больше 2 МБ уходит в .old.
func setupLog(path string) {
	rotateLog(path, maxLogSize)
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(logWriter{file: f, console: os.Stderr})
	}
}

// logWriter пишет сначала в файл, потом в консоль, и ошибку консоли не возвращает.
// io.MultiWriter(os.Stderr, f) не годится: у собранного exe (GUI без консоли) запись
// в stderr падает, MultiWriter на этом останавливается — и log.txt оставался пустым
// (так было в 0.2.0 у Марины).
type logWriter struct {
	file    io.Writer
	console io.Writer
}

func (w logWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	_, _ = w.console.Write(p)
	return n, err
}

// rotateLog переименовывает log.txt в log.old.txt, если он больше limit байт.
func rotateLog(path string, limit int64) {
	st, err := os.Stat(path)
	if err != nil || st.Size() <= limit {
		return
	}
	old := strings.TrimSuffix(path, ".txt") + ".old.txt"
	// Ротация — удобство: при сбое лог просто продолжит расти до следующего запуска.
	_ = os.Remove(old)
	_ = os.Rename(path, old)
}

// logRare пишет в лог первое событие счётчика и затем каждое сотое:
// на повторяющихся ошибках (например, 404 от /media) файл не раздувается.
func logRare(n int64, format string, args ...any) {
	if n == 1 || n%100 == 0 {
		log.Printf("[x%d] "+format, append([]any{n}, args...)...) //nolint:gosec // G706: текст ошибок ОС и имя файла без путей, в лог-файл разработчику
	}
}
