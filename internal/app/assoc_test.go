package app

import (
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/assoc"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeReg — реестр в памяти: тесты app настоящий не трогают никогда. Write и Remove меняют
// состояние так, как это видит Read.
type fakeReg struct {
	unsupported bool
	st          osx.AssocState
	self        string
	writeErr    error
	removeErr   error
	writes      []string // «exe|версия»
	removes     int
	drops       int // сколько раз звали DropUserChoice
}

func (f *fakeReg) Supported() bool { return !f.unsupported }
func (f *fakeReg) Read() osx.AssocState {
	return f.st
}
func (f *fakeReg) Write(exe, version string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes = append(f.writes, exe+"|"+version)
	f.st.ProgID, f.st.Exe, f.st.AppVersion = osx.AssocProgID, exe, version
	return nil
}
func (f *fakeReg) Remove() error {
	f.removes++
	if f.removeErr != nil {
		return f.removeErr
	}
	f.st.ProgID, f.st.Exe, f.st.AppVersion = "", "", ""
	return nil
}
func (f *fakeReg) DropUserChoice() (bool, error) {
	f.drops++
	if f.st.UserChoice == "" || !f.st.UserChoiceIsOurs() {
		return false, nil
	}
	f.st.UserChoice = ""
	return true, nil
}
func (f *fakeReg) SelfExe() (string, error) { return f.self, nil }

// Значения config.assoc записаны в config.json: тест держит их литералами, а не константами службы.
const (
	assocOn  = "on"
	assocOff = "off"
)

const (
	myExe       = "/prog/me/SosninaAlbums-0.25.0.exe"
	oldExe      = "/prog/old/SosninaAlbums-0.24.0.exe"
	instDir     = "/prog/inst"
	instExe     = "/prog/inst/SosninaAlbums.exe"
	foreignProg = "Other.Album"
	appsOld     = `Applications\SosninaAlbums-0.24.0.exe`
	appsEtc     = `Applications\notepad.exe`
	progOurs    = osx.AssocProgID
)

// assocRig — App с пересобранной службой связи: подмены Host (реестр, место запуска, сборка dev,
// временная папка, запуск и выход) — поля рига, Host читает их при каждом вызове. Служба
// собирается в newAssocRig, до первого вызова и до фона (126-19); поля App доступны через встраивание.
type assocRig struct {
	*App
	reg     assoc.Registry // nil — связь не поддерживается
	placeFn func() osx.Place
	devFlag bool
	tempFn  func() string
	startFn func(exe string, args ...string) error
	quitFn  func()
}

func newAssocRig(t *testing.T, reg assoc.Registry) *assocRig {
	t.Helper()
	withVersion(t, "0.25.0")
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	r := &assocRig{App: a, reg: reg, placeFn: a.assocHost().Place, tempFn: os.TempDir,
		startFn: osx.Start, quitFn: a.quitWindow}
	h := a.assocHost()
	h.Reg = func() assoc.Registry { return r.reg }
	h.Place = func() osx.Place { return r.placeFn() }
	h.DevBuild = func() bool { return r.devFlag }
	h.Temp = func() string { return r.tempFn() }
	h.Start = func(exe string, args ...string) error { return r.startFn(exe, args...) }
	h.Quit = func() { r.quitFn() }
	a.assoc = assoc.New(h)
	return r
}

// run — тело фоновой синхронизации связи, дождавшись фона.
func (r *assocRig) run() {
	r.syncAssoc(false)
	r.bg.Wait()
}

// assocApp — App версии 0.25.0 с реестром в памяти; live — какие exe «лежат на диске».
func assocApp(t *testing.T, reg *fakeReg, live ...string) *assocRig {
	t.Helper()
	if reg.self == "" {
		reg.self = myExe
	}
	r := newAssocRig(t, reg)
	r.placeFn = func() osx.Place { return osx.PlaceNormal }
	r.exists = func(p string) bool { return slices.Contains(live, p) }
	return r
}

// Без регистрации: нет реестра, не Windows, dev, стенд, временная папка, assoc: off — связь не
// пишется и не снимается; кроме off вопроса и пункта меню тоже нет (Can=false).
func TestAssocSkip(t *testing.T) {
	cases := []struct {
		name string
		set  func(a *assocRig, r *fakeReg)
		view AssocView
	}{
		{"нет реестра", func(a *assocRig, _ *fakeReg) { a.reg = nil }, AssocView{}},
		{"не Windows", func(_ *assocRig, r *fakeReg) { r.unsupported = true }, AssocView{}},
		{"сборка dev", func(a *assocRig, _ *fakeReg) { a.devFlag = true }, AssocView{}},
		{"стенд QA", func(a *assocRig, _ *fakeReg) { a.data = "SosninaAlbums-stand-b" }, AssocView{}},
		{"запуск из архива", func(a *assocRig, _ *fakeReg) {
			a.placeFn = func() osx.Place { return osx.PlaceTemp }
		}, AssocView{}},
		// off: Can есть (пункт меню включит), вопроса нет, связь не пишется и не возвращается.
		{"assoc off", func(a *assocRig, _ *fakeReg) {
			a.updateConfig(func(c *prefs.Config) { c.Assoc = assocOff })
		}, AssocView{Can: true, On: true}},
	}
	for _, c := range cases {
		reg := &fakeReg{}
		a := assocApp(t, reg, oldExe)
		// Состояние, которое план иначе переписал бы: наша старая связь и выбор «всегда».
		reg.st = osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsOld}
		a.updateConfig(func(cf *prefs.Config) { cf.Assoc = assocOn })
		c.set(a, reg)
		a.syncAssoc(false)
		a.bg.Wait()
		if len(reg.writes) != 0 || reg.removes != 0 || reg.drops != 0 {
			t.Errorf("%s: тронут реестр: записей %d, снятий %d, выборов %d", c.name, len(reg.writes), reg.removes, reg.drops)
		}
		if got := a.assocView(); got != c.view {
			t.Errorf("%s: вид %+v, ждали %+v", c.name, got, c.view)
		}
		// SetAssoc в таком запуске тоже ничего не пишет (off: «Да» — отдельный случай ниже).
		if c.name != "assoc off" {
			a.SetAssoc(true)
			a.SetAssoc(false)
			if len(reg.writes) != 0 || reg.removes != 0 {
				t.Errorf("%s: SetAssoc тронул реестр", c.name)
			}
		}
	}
	// Контроль: тот же сценарий без помех пишет (тест не пустой).
	reg := &fakeReg{}
	a := assocApp(t, reg, oldExe)
	reg.st = osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsOld}
	a.updateConfig(func(cf *prefs.Config) { cf.Assoc = assocOn })
	a.run()
	if len(reg.writes) != 1 || reg.drops != 1 {
		t.Errorf("контроль: записей %d, выборов %d", len(reg.writes), reg.drops)
	}
}

// Вопрос: пока не решено и связи нет — Ask; «Не сейчас» до следующей версии (major.minor);
// «Не спрашивать» навсегда; запуск с файлом и уже наша связь — без вопроса. Согласия нет —
// связь не пишется.
func TestAssocAsk(t *testing.T) {
	reg := &fakeReg{}
	a := assocApp(t, reg, oldExe)
	if v := a.assocView(); v != (AssocView{Can: true, Ask: true}) {
		t.Fatalf("первый запуск: %+v", v)
	}
	if st := a.GetState(); st.Assoc != a.assocView() || !st.Assoc.Ask {
		t.Errorf("State.Assoc: %+v", st.Assoc)
	}
	a.run()
	if len(reg.writes) != 0 {
		t.Fatalf("без согласия записано: %v", reg.writes)
	}
	if c := a.loadConfig(); c.Assoc != "" {
		t.Errorf("config.assoc = %q без согласия", c.Assoc)
	}
	// Запуск двойным щелчком по альбому: вопроса нет, но и согласия тоже.
	a.syncAssoc(true)
	a.bg.Wait()
	if v := a.assocView(); v.Ask {
		t.Errorf("вопрос при запуске с файлом: %+v", v)
	}
	a.syncAssoc(false)
	a.bg.Wait()
	if v := a.assocView(); !v.Ask {
		t.Errorf("после обычного запуска вопроса нет: %+v", v)
	}
	// «Не сейчас»: в этой версии больше нет, в следующей (0.26) — снова; в патче (0.25.3) — нет.
	if v := a.AssocLater(); v.Ask || !v.Can {
		t.Errorf("AssocLater: %+v", v)
	}
	if c := a.loadConfig(); c.AssocLater != "0.25" || c.Assoc != "" {
		t.Errorf("config после «Не сейчас»: %+v", c)
	}
	withVersion(t, "0.25.3")
	if v := a.assocView(); v.Ask {
		t.Errorf("вопрос вернулся в патче: %+v", v)
	}
	withVersion(t, "0.26.0")
	if v := a.assocView(); !v.Ask {
		t.Errorf("вопрос не вернулся в 0.26: %+v", v)
	}
	// «Не спрашивать» — навсегда, связь снята.
	if v := a.SetAssoc(false); v.Ask || !v.Can || v.On {
		t.Errorf("«Не спрашивать»: %+v", v)
	}
	withVersion(t, "0.27.0")
	if v := a.assocView(); v.Ask {
		t.Errorf("вопрос после off: %+v", v)
	}
	// Связь уже наша (установщик или прошлая версия): вопроса нет, Service.run принимает её как согласие.
	reg2 := &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0"}}
	b := assocApp(t, reg2, oldExe)
	if v := b.assocView(); v != (AssocView{Can: true, On: true}) {
		t.Errorf("связь уже наша: %+v", v)
	}
	b.run()
	if c := b.loadConfig(); c.Assoc != assocOn {
		t.Errorf("согласие не записано: %q", c.Assoc)
	}
	if len(reg2.writes) != 1 || reg2.writes[0] != myExe+"|0.25.0" {
		t.Errorf("новая версия не забрала связь: %v", reg2.writes)
	}
	// Связь наша, но exe пропал: связи фактически нет — вопрос.
	reg3 := &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0"}}
	c3 := assocApp(t, reg3)
	if v := c3.assocView(); v != (AssocView{Can: true, Ask: true}) {
		t.Errorf("связь с пропавшим exe: %+v", v)
	}
}

// Service.run (internal/app/assoc) по согласию: план применяется (моя версия новее — пишем, старее — нет, чужой ProgID —
// нет), «всегда» от нашей старой версии снимается, чужой выбор нет.
func TestAssocSync(t *testing.T) {
	cases := []struct {
		name    string
		st      osx.AssocState
		writes  int
		drops   int
		choiceL string // UserChoice после
	}{
		{"старая связь — забираем", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0"}, 1, 0, ""},
		{"новая связь — не отбираем", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.26.0"}, 0, 0, ""},
		{"чужой ProgID", osx.AssocState{ProgID: foreignProg}, 0, 0, ""},
		{"пусто — пишем", osx.AssocState{}, 1, 0, ""},
		{"случай Марины", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsOld}, 1, 1, ""},
		{"чужой выбор цел", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsEtc}, 1, 0, appsEtc},
	}
	for _, c := range cases {
		reg := &fakeReg{st: c.st}
		a := assocApp(t, reg, oldExe)
		a.updateConfig(func(cf *prefs.Config) { cf.Assoc = assocOn })
		a.run()
		if len(reg.writes) != c.writes || reg.drops != c.drops || reg.st.UserChoice != c.choiceL {
			t.Errorf("%s: записей %d (ждали %d), выборов %d (ждали %d), выбор %q", c.name, len(reg.writes), c.writes, reg.drops, c.drops, reg.st.UserChoice)
		}
	}
	// Ошибка записи не роняет: связь не меняется, config остаётся.
	reg := &fakeReg{writeErr: errors.New("отказ")}
	a := assocApp(t, reg)
	a.updateConfig(func(cf *prefs.Config) { cf.Assoc = assocOn })
	a.run()
	if c := a.loadConfig(); c.Assoc != assocOn {
		t.Errorf("config после отказа записи: %+v", c)
	}
}

// SetAssoc: «Да»/галочка пишут связь на эту программу сразу (и поверх чужого ProgID, и снимая
// наш «всегда»), «Не спрашивать»/снять — удаляют; отказ записи — тост и config не меняется;
// круг: новый App на той же папке видит выбор.
func TestSetAssoc(t *testing.T) {
	reg := &fakeReg{st: osx.AssocState{ProgID: foreignProg, UserChoice: appsOld}}
	a := assocApp(t, reg)
	var notes []string
	a.emit = func(name string, data ...any) {
		if name == "notice" {
			notes = append(notes, data[0].(string))
		}
	}
	reg.st.ProgID = foreignProg
	reg.self = myExe
	a.exists = func(p string) bool { return p == myExe }
	v := a.SetAssoc(true)
	if v != (AssocView{Can: true, On: true}) {
		t.Errorf("включено: %+v", v)
	}
	if len(reg.writes) != 1 || reg.writes[0] != myExe+"|0.25.0" {
		t.Errorf("запись: %v", reg.writes)
	}
	if reg.st.ProgID != progOurs || reg.st.UserChoice != "" {
		t.Errorf("ProgID %q, выбор %q", reg.st.ProgID, reg.st.UserChoice)
	}
	if c := a.loadConfig(); c.Assoc != assocOn {
		t.Errorf("config: %+v", c)
	}
	// Круг: другой запуск видит согласие и не спрашивает.
	b := assocApp(t, reg, myExe)
	b.cfgDir = a.cfgDir
	if got := b.assocView(); got != (AssocView{Can: true, On: true}) {
		t.Errorf("после перезапуска: %+v", got)
	}
	// Снять.
	v = a.SetAssoc(false)
	if v != (AssocView{Can: true}) || reg.removes != 1 || reg.st.ProgID != "" {
		t.Errorf("снято: %+v, снятий %d, ProgID %q", v, reg.removes, reg.st.ProgID)
	}
	if c := a.loadConfig(); c.Assoc != assocOff {
		t.Errorf("config после снятия: %+v", c)
	}
	// И снова включить: off не залипает.
	if v = a.SetAssoc(true); !v.On || len(reg.writes) != 2 {
		t.Errorf("повторное включение: %+v, записей %d", v, len(reg.writes))
	}
	// Отказ записи: тост, config прежний, связи нет.
	reg2 := &fakeReg{writeErr: errors.New("нет доступа")}
	c := assocApp(t, reg2)
	notes = nil
	c.emit = func(name string, data ...any) {
		if name == "notice" {
			notes = append(notes, data[0].(string))
		}
	}
	if v = c.SetAssoc(true); v.On || len(notes) != 1 {
		t.Errorf("отказ записи: %+v, тосты %v", v, notes)
	}
	if cf := c.loadConfig(); cf.Assoc != "" {
		t.Errorf("config после отказа: %+v", cf)
	}
	// Отказ снятия: тост, но выбор «off» сохранён — вопрос не вернётся.
	reg3 := &fakeReg{removeErr: errors.New("нет доступа"), st: osx.AssocState{ProgID: progOurs, Exe: myExe}}
	d := assocApp(t, reg3, myExe)
	notes = nil
	d.emit = c.emit
	d.SetAssoc(false)
	if cf := d.loadConfig(); cf.Assoc != assocOff || len(notes) != 1 {
		t.Errorf("отказ снятия: %+v, тосты %v", cf, notes)
	}
}
