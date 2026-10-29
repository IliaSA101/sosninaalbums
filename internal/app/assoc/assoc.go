// Package assoc — служба связи .salbum и окна «файл новее программы» (спеки 112 и 098, разрез 126
// Ш2): кому принадлежит связь в реестре (план), когда её писать (Sync), вопрос и пункт меню (View),
// отказ открыть файл новее программы и запуск найденной новой версии (OpenInNewer). Реестр — osx,
// через интерфейс Registry. Правка проекта не участвует.
//
// Зовёт её internal/app (assoc.go — фасад с методами для интерфейса). Wails и App службе
// недоступны: тост, фон, выход и запуск идут через Host (поля-функции, замыкания над App —
// позднее связывание, подмены тестов действуют и после NewApp).
package assoc

import (
	"log"
	"reflect"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Значения config.assoc.
const (
	assocOn  = "on"
	assocOff = "off"
)

// View — связь .salbum для интерфейса (State.Assoc, ответы SetAssoc и AssocLater). App объявляет
// его определённым типом (`type AssocView assoc.View`), поэтому в models.ts он остаётся типом app.
type View struct {
	// Can — выпуск на Windows (стартовый вопрос и пункт меню есть); иначе false.
	Can bool `json:"can"`
	// On — .salbum открывается нашей программой (какой-либо версией или установленной).
	On bool `json:"on"`
	// Ask — показать вопрос «Открывать альбомы .salbum двойным щелчком?».
	Ask bool `json:"ask"`
}

// Registry — всё, что связь .salbum берёт у системы: реестр и путь запущенной программы.
// Интерфейс, чтобы тесты app не трогали настоящий реестр: настоящий отдаёт System (Run подключает
// его до окна), NewApp оставляет nil — связь «не поддерживается».
type Registry interface {
	Supported() bool
	Read() osx.AssocState
	Write(exe, version string) error
	Remove() error
	DropUserChoice() (bool, error)
	SelfExe() (string, error)
}

// System — настоящая связь: функции osx (HKCU на Windows; на других ОС — пустые).
type System struct{}

func (System) Supported() bool                 { return osx.Supported() }
func (System) Read() osx.AssocState            { return osx.ReadAssoc() }
func (System) Write(exe, version string) error { return osx.WriteAssoc(exe, version) }
func (System) Remove() error                   { return osx.RemoveAssoc() }
func (System) DropUserChoice() (bool, error)   { return osx.DropUserChoice() }
func (System) SelfExe() (string, error)        { return osx.SelfExe() }

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем
// пустого). App отдаёт настоящие значения; тест пересобирает службу с подменой до первого вызова.
type Host struct {
	CfgDir    func() string                          // папка config.json
	Data      func() string                          // имя папки данных; не osx.AppName — стенд QA, связь не пишется
	Version   func() string                          // версия этой программы (wails.json)
	Notice    func(text string)                      // тост об ошибке (событие notice)
	GoBG      func(fn func()) bool                   // фон под WaitGroup App; false — App закрывается
	Exists    func(path string) bool                 // файл на месте
	FlushSave func()                                 // сохранить альбом перед выходом
	Quit      func()                                 // закрыть программу
	Reg       func() Registry                        // реестр; nil — связь не поддерживается
	Place     func() osx.Place                       // откуда запущен exe
	Temp      func() string                          // временная папка (запуск из архива)
	Start     func(exe string, args ...string) error // запуск другой программы
	DevBuild  func() bool                            // сборка для разработки: связь не пишется
}

// Service — связь .salbum и отказ «файл новее», под своими замками.
type Service struct {
	h Host

	mu       sync.Mutex // fromFile
	fromFile bool       // запуск двойным щелчком по альбому: вопрос в этот раз не нужен

	nmu     sync.Mutex // path, exe, start: пишут открытие (UI) и старт, читает OpenInNewer
	path    string     // альбом, который откроет новая версия; у черновика пусто
	exe     string     // найденная новая версия; "" — нет
	startAt *NewerInfo // отказ при старте: первый GetState отдаёт один раз
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и updater.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("assoc.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h}
}

// can — связь можно записывать: Windows-выпуск, не стенд QA, не запуск из архива.
func (s *Service) can() bool {
	reg := s.h.Reg()
	if reg == nil || !reg.Supported() || s.h.DevBuild() || s.h.Data() != osx.AppName {
		return false
	}
	return s.h.Place() == osx.PlaceNormal
}

// View — связь для интерфейса. Читает реестр сразу (6–8 чтений), а не из запомненного:
// GetState может прийти раньше фонового Sync, и вопрос иначе не появился бы.
func (s *Service) View() View {
	if !s.can() {
		return View{}
	}
	cfg := prefs.LoadConfig(s.h.CfgDir())
	on := s.ours(s.h.Reg().Read())
	s.mu.Lock()
	fromFile := s.fromFile
	s.mu.Unlock()
	ask := cfg.Assoc == "" && !on && !fromFile && cfg.AssocLater != ShortVer(s.h.Version())
	return View{Can: true, On: on, Ask: ask}
}

// ours — .salbum ведёт в нашу программу: ProgID наш и exe из команды на месте.
func (s *Service) ours(st osx.AssocState) bool {
	return st.ProgIDIsOurs() && st.Exe != "" && s.h.Exists(st.Exe)
}

// Sync — при запуске (после загрузки альбома, до закрытия ready): запоминает, что запуск был
// с файлом (вопрос не нужен), и в фоне приводит связь к плану. Не задерживает окно.
func (s *Service) Sync(fromFile bool) {
	s.mu.Lock()
	s.fromFile = fromFile
	s.mu.Unlock()
	s.h.GoBG(s.run)
}

// run — тело Sync. Вопроса не задаёт: он в View.
//
//	нельзя (can) или config.assoc = off → ничего;
//	config.assoc = on или связь уже наша → план и запись по плану, config.assoc = on;
//	иначе ничего (спросит стартовое окно).
func (s *Service) run() {
	defer appkit.Guard("assoc")
	if !s.can() {
		return
	}
	reg := s.h.Reg()
	cfgDir := s.h.CfgDir()
	cfg := prefs.LoadConfig(cfgDir)
	if cfg.Assoc == assocOff {
		return
	}
	st := reg.Read()
	if cfg.Assoc != assocOn && !s.ours(st) {
		return
	}
	exe, err := reg.SelfExe()
	if err != nil {
		log.Printf("связь .salbum: путь программы: %v", paths.NoPath(err))
		return
	}
	mine := s.h.Version()
	act := plan(planIn{St: st, Exe: exe, Ver: mine, Exists: s.h.Exists})
	if act.Foreign {
		log.Print("связь .salbum: расширение у другой программы, не трогаем")
	}
	if act.Write {
		if err := reg.Write(exe, mine); err != nil {
			log.Printf("связь .salbum: %v", paths.NoPath(err))
		}
	}
	if act.Drop {
		if _, err := reg.DropUserChoice(); err != nil {
			log.Printf("связь .salbum: %v", paths.NoPath(err))
		}
	}
	if cfg.Assoc != assocOn {
		prefs.UpdateConfig(cfgDir, func(c *prefs.Config) { c.Assoc = assocOn })
	}
}

// Set — «Да» стартового окна, галочка меню (on) и «Не спрашивать» (off).
// on: связь пишется на эту программу сразу (согласие явное — и поверх чужого ProgID), прежний
// наш выбор «всегда» снимается, config.assoc = on. off: config.assoc = off, связь с нашей
// программой (любой версии и установщика) удаляется. Ошибка записи — тост, config не меняется
// на on. Вне Windows и в сборках без связи ничего не делает.
func (s *Service) Set(on bool) (v View) {
	defer appkit.Guard("SetAssoc")
	if !s.can() {
		return s.View()
	}
	reg := s.h.Reg()
	cfgDir := s.h.CfgDir()
	if !on {
		prefs.UpdateConfig(cfgDir, func(c *prefs.Config) { c.Assoc = assocOff })
		if err := reg.Remove(); err != nil {
			log.Printf("связь .salbum: %v", paths.NoPath(err))
			s.h.Notice("Не удалось убрать связь .salbum с программой. Подробности — в log.txt.")
		}
		return s.View()
	}
	exe, err := reg.SelfExe()
	if err == nil {
		err = reg.Write(exe, s.h.Version())
	}
	if err != nil {
		log.Printf("связь .salbum: %v", paths.NoPath(err))
		s.h.Notice("Не удалось включить открытие .salbum этой программой. Подробности — в log.txt.")
		return s.View()
	}
	if _, err := reg.DropUserChoice(); err != nil {
		log.Printf("связь .salbum: %v", paths.NoPath(err))
	}
	prefs.UpdateConfig(cfgDir, func(c *prefs.Config) { c.Assoc = assocOn })
	return s.View()
}

// Later — «Не сейчас»: вопрос не возвращается до следующей версии (major.minor).
func (s *Service) Later() (v View) {
	defer appkit.Guard("AssocLater")
	if s.can() {
		short := ShortVer(s.h.Version())
		prefs.UpdateConfig(s.h.CfgDir(), func(c *prefs.Config) { c.AssocLater = short })
	}
	return s.View()
}
