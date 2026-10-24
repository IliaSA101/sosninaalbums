package app

import (
	"log"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Открытие .salbum двойным щелчком (спека 112, design §2). Здесь решения: кому принадлежит
// связь (assocPlan), когда её писать (syncAssoc), вопрос и пункт меню (AssocView). Сам реестр —
// osx, через интерфейс assocReg (assocreg.go).

// Значения config.assoc.
const (
	assocOn  = "on"
	assocOff = "off"
)

// AssocView — связь .salbum для интерфейса (State.Assoc, ответы SetAssoc и AssocLater).
type AssocView struct {
	// Can — выпуск на Windows (стартовый вопрос и пункт меню есть); иначе false.
	Can bool `json:"can"`
	// On — .salbum открывается нашей программой (какой-либо версией или установленной).
	On bool `json:"on"`
	// Ask — показать вопрос «Открывать альбомы .salbum двойным щелчком?».
	Ask bool `json:"ask"`
}

// assocState — зависимости и память связи в App. Нулевое значение — «не поддерживается».
type assocState struct {
	reg   assocReg
	place func() osx.Place // откуда запущен exe; nil — launchPlace
	// devBuild — сборка для разработки; тест ставит true, настоящий признак — osx.DevBuild.
	devBuild bool

	mu       sync.Mutex
	fromFile bool // запуск двойным щелчком по альбому: вопрос в этот раз не нужен
}

// assocIn — вход assocPlan: что в реестре и кто этот запуск.
type assocIn struct {
	St     osx.AssocState
	Exe    string // exe этого запуска (osx.SelfExe)
	Ver    string // версия этого запуска (productVersion)
	Exists func(path string) bool
}

// assocAct — решение assocPlan.
type assocAct struct {
	Write   bool // записать связь на этот exe
	Foreign bool // расширение у чужой программы: не трогаем (в лог)
	Drop    bool // удалить наш прежний выбор «Открыть с помощью → всегда»
}

// assocPlan — чистое решение для связи, уже ставшей нашей (согласие дано или связь наша).
//
//	ProgID пуст или наш: пусто или exe из команды пропал → писать; моя версия новее → писать;
//	  равны: я установлена, они нет → писать; иначе не писать.
//	ProgID чужой → не писать.
//	UserChoice наш (userChoiceOurs) → удалить.
func assocPlan(in assocIn) assocAct {
	st := in.St
	act := assocAct{Drop: userChoiceOurs(st, in.Exists)}
	if st.ProgID != "" && !st.ProgIDIsOurs() {
		act.Foreign = true
		return act
	}
	theirs := theirVersion(st)
	switch {
	case st.ProgID == "" || st.Exe == "" || !in.Exists(st.Exe):
		act.Write = true
	case verKnown(in.Ver) && !verKnown(theirs):
		// update.Newer для неразборной версии даёт false, а здесь неизвестная версия ниже любой:
		// связь без внятной версии (запись чужим инструментом, «dev») забирает любая настоящая.
		act.Write = true
	case cmpVer(in.Ver, theirs) > 0:
		act.Write = true
	case cmpVer(in.Ver, theirs) == 0 && isInstalled(in.Exe, st) && !isInstalled(st.Exe, st):
		act.Write = true
	}
	return act
}

// userChoiceOurs — прежний выбор пользователя ведёт в нашу программу и мешает связи HKCU
// (Windows слушает UserChoice раньше). Applications\sosninaalbums*.exe — всегда наш (случай
// «Открыть с помощью → всегда» на старой версии). Наш ProgID Windows разбирает по команде
// связи, поэтому он мешает, только когда команда мертва: exe пропал. Чужой выбор — не наш.
func userChoiceOurs(st osx.AssocState, exists func(string) bool) bool {
	if st.UserChoice == "" || !st.UserChoiceIsOurs() {
		return false
	}
	if strings.EqualFold(st.UserChoice, osx.AssocProgID) {
		return st.Exe == "" || !exists(st.Exe)
	}
	return true
}

// isInstalled — exe — установленная копия (InstallDir\SosninaAlbums.exe, раздел пишет установщик).
func isInstalled(exe string, st osx.AssocState) bool {
	inst := st.InstalledExe()
	return inst != "" && exe != "" && paths.SamePath(exe, inst)
}

// theirVersion — версия записавшего связь: AppVersion; нет её, а exe установленный — версия
// установки; иначе "" (неизвестна).
func theirVersion(st osx.AssocState) string {
	if st.AppVersion != "" {
		return st.AppVersion
	}
	if isInstalled(st.Exe, st) {
		return st.InstallVersion
	}
	return ""
}

// splitVer — «0.25.1-rc2+abc» → версия 0.25.1, пред-выпуск true.
func splitVer(s string) (v update.Version, pre string, ok bool) {
	s, _, _ = strings.Cut(s, "+")
	base, pre, _ := strings.Cut(s, "-")
	v, ok = update.ParseVersion(base)
	return v, pre, ok
}

func verKnown(s string) bool { _, _, ok := splitVer(s); return ok }

// cmpVer — -1, 0, 1: a ниже, равна, выше b. Пред-выпуск (-rc2) ниже выпуска; не версия (dev,
// пусто) ниже любой, две не-версии равны.
func cmpVer(a, b string) int {
	va, pa, oka := splitVer(a)
	vb, pb, okb := splitVer(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	if c := va.Compare(vb); c != 0 {
		return c
	}
	switch {
	case pa == pb:
		return 0
	case pa == "":
		return 1
	case pb == "":
		return -1
	}
	return strings.Compare(pa, pb)
}

// assocCan — связь можно записывать: Windows-выпуск, не стенд QA, не запуск из архива.
func (a *App) assocCan() bool {
	s := &a.assoc
	if s.reg == nil || !s.reg.Supported() || osx.DevBuild || s.devBuild || a.data != osx.AppName {
		return false
	}
	place := s.place
	if place == nil {
		place = launchPlace
	}
	return place() == osx.PlaceNormal
}

// assocView — связь для интерфейса. Читает реестр сразу (6–8 чтений), а не из запомненного:
// GetState может прийти раньше фонового syncAssoc, и вопрос иначе не появился бы.
func (a *App) assocView() AssocView {
	if !a.assocCan() {
		return AssocView{}
	}
	cfg := a.loadConfig()
	on := a.assocOurs(a.assoc.reg.Read())
	a.assoc.mu.Lock()
	fromFile := a.assoc.fromFile
	a.assoc.mu.Unlock()
	ask := cfg.Assoc == "" && !on && !fromFile && cfg.AssocLater != shortVer(appVersion(wailsJSON))
	return AssocView{Can: true, On: on, Ask: ask}
}

// assocOurs — .salbum ведёт в нашу программу: ProgID наш и exe из команды на месте.
func (a *App) assocOurs(st osx.AssocState) bool {
	return st.ProgIDIsOurs() && st.Exe != "" && a.exists(st.Exe)
}

// syncAssoc — при запуске (после загрузки альбома, до закрытия ready): запоминает, что запуск был
// с файлом (вопрос не нужен), и в фоне приводит связь к плану. Не задерживает окно.
func (a *App) syncAssoc(fromFile bool) {
	a.assoc.mu.Lock()
	a.assoc.fromFile = fromFile
	a.assoc.mu.Unlock()
	a.goBG(a.runAssoc)
}

// runAssoc — тело syncAssoc (тесты зовут его без фона). Вопроса не задаёт: он в assocView.
//
//	нельзя (assocCan) или config.assoc = off → ничего;
//	config.assoc = on или связь уже наша → assocPlan и запись по плану, config.assoc = on;
//	иначе ничего (спросит стартовое окно).
func (a *App) runAssoc() {
	defer guard("assoc")
	if !a.assocCan() {
		return
	}
	reg := a.assoc.reg
	cfg := a.loadConfig()
	if cfg.Assoc == assocOff {
		return
	}
	st := reg.Read()
	if cfg.Assoc != assocOn && !a.assocOurs(st) {
		return
	}
	exe, err := reg.SelfExe()
	if err != nil {
		log.Printf("связь .salbum: путь программы: %v", paths.NoPath(err))
		return
	}
	mine := appVersion(wailsJSON)
	act := assocPlan(assocIn{St: st, Exe: exe, Ver: mine, Exists: a.exists})
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
		a.updateConfig(func(c *prefs.Config) { c.Assoc = assocOn })
	}
}

// SetAssoc — «Да» стартового окна, галочка меню (on) и «Не спрашивать» (off).
// on: связь пишется на эту программу сразу (согласие явное — и поверх чужого ProgID), прежний
// наш выбор «всегда» снимается, config.assoc = on. off: config.assoc = off, связь с нашей
// программой (любой версии и установщика) удаляется. Ошибка записи — тост, config не меняется
// на on. Вне Windows и в сборках без связи ничего не делает.
func (a *App) SetAssoc(on bool) AssocView {
	defer guard("SetAssoc")
	if !a.assocCan() {
		return a.assocView()
	}
	reg := a.assoc.reg
	if !on {
		a.updateConfig(func(c *prefs.Config) { c.Assoc = assocOff })
		if err := reg.Remove(); err != nil {
			log.Printf("связь .salbum: %v", paths.NoPath(err))
			a.notice("Не удалось убрать связь .salbum с программой. Подробности — в log.txt.")
		}
		return a.assocView()
	}
	exe, err := reg.SelfExe()
	if err == nil {
		err = reg.Write(exe, appVersion(wailsJSON))
	}
	if err != nil {
		log.Printf("связь .salbum: %v", paths.NoPath(err))
		a.notice("Не удалось включить открытие .salbum этой программой. Подробности — в log.txt.")
		return a.assocView()
	}
	if _, err := reg.DropUserChoice(); err != nil {
		log.Printf("связь .salbum: %v", paths.NoPath(err))
	}
	a.updateConfig(func(c *prefs.Config) { c.Assoc = assocOn })
	return a.assocView()
}

// AssocLater — «Не сейчас»: вопрос не возвращается до следующей версии (major.minor).
func (a *App) AssocLater() AssocView {
	defer guard("AssocLater")
	if a.assocCan() {
		a.updateConfig(func(c *prefs.Config) { c.AssocLater = shortVer(appVersion(wailsJSON)) })
	}
	return a.assocView()
}
