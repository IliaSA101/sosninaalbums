package app

import (
	"errors"
	"slices"
	"testing"

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

// assocApp — App версии 0.25.0 с реестром в памяти; live — какие exe «лежат на диске».
func assocApp(t *testing.T, reg *fakeReg, live ...string) *App {
	t.Helper()
	withVersion(t, "0.25.0")
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	if reg.self == "" {
		reg.self = myExe
	}
	a.assoc.reg = reg
	a.assoc.place = func() osx.Place { return osx.PlaceNormal }
	a.exists = func(p string) bool { return slices.Contains(live, p) }
	return a
}

func TestAssocPlan(t *testing.T) {
	live := func(ps ...string) func(string) bool {
		return func(p string) bool { return slices.Contains(ps, p) }
	}
	ours := func(exe, ver string) osx.AssocState {
		return osx.AssocState{ProgID: progOurs, Exe: exe, AppVersion: ver}
	}
	cases := []struct {
		name       string
		st         osx.AssocState
		exe, ver   string
		live       []string
		write      bool
		foreignAct bool
		drop       bool
	}{
		{"пусто", osx.AssocState{}, myExe, "0.25.0", nil, true, false, false},
		{"ProgID наш, команды нет", osx.AssocState{ProgID: progOurs}, myExe, "0.25.0", nil, true, false, false},
		{"exe из команды пропал", ours(oldExe, "0.26.0"), myExe, "0.25.0", nil, true, false, false},
		{"моя версия новее", ours(oldExe, "0.24.0"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"моя версия старее", ours(oldExe, "0.26.0"), myExe, "0.25.0", []string{oldExe}, false, false, false},
		{"равны, обе zip", ours(oldExe, "0.25.0"), myExe, "0.25.0", []string{oldExe}, false, false, false},
		{"равны, это же exe", ours(myExe, "0.25.0"), myExe, "0.25.0", []string{myExe}, false, false, false},
		{"пред-выпуск ниже выпуска", ours(oldExe, "0.25.0-rc2"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"я пред-выпуск, у них выпуск", ours(oldExe, "0.25.0"), myExe, "0.25.0-rc2", []string{oldExe}, false, false, false},
		{"0.9 против 0.10 числом", ours(oldExe, "0.9.0"), myExe, "0.10.0", []string{oldExe}, true, false, false},
		// Их версия не разбирается, моя — да: update.Newer тут дал бы false, а план пишет.
		{"их версия не разбирается", ours(oldExe, "dev"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"их версии нет вовсе", ours(oldExe, ""), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"моя не разбирается, их да", ours(oldExe, "0.1.0"), myExe, "dev", []string{oldExe}, false, false, false},
		{"обе не разбираются", ours(oldExe, "dev"), myExe, "dev", []string{oldExe}, false, false, false},
		{"чужой ProgID", osx.AssocState{ProgID: foreignProg}, myExe, "0.25.0", nil, false, true, false},
		{"чужой ProgID и наш выбор «всегда»", osx.AssocState{ProgID: foreignProg, UserChoice: appsOld}, myExe, "0.25.0", nil, false, true, true},
		{"равны: я установлена, они zip", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.25.0", InstallDir: instDir},
			instExe, "0.25.0", []string{oldExe, instExe}, true, false, false},
		{"равны: я zip, они установлены", osx.AssocState{ProgID: progOurs, Exe: instExe, AppVersion: "0.25.0", InstallDir: instDir},
			myExe, "0.25.0", []string{instExe, myExe}, false, false, false},
		{"равны: обе установлены", osx.AssocState{ProgID: progOurs, Exe: instExe, AppVersion: "0.25.0", InstallDir: instDir},
			instExe, "0.25.0", []string{instExe}, false, false, false},
		// AppVersion нет (записал установщик без неё) — версия установки.
		{"версия из установки: новее", osx.AssocState{ProgID: progOurs, Exe: instExe, InstallDir: instDir, InstallVersion: "0.26.0"},
			myExe, "0.25.0", []string{instExe}, false, false, false},
		{"версия из установки: старее", osx.AssocState{ProgID: progOurs, Exe: instExe, InstallDir: instDir, InstallVersion: "0.24.0"},
			myExe, "0.25.0", []string{instExe}, true, false, false},
		{"выбор «всегда» на старой версии", osx.AssocState{ProgID: progOurs, Exe: myExe, AppVersion: "0.25.0", UserChoice: appsOld},
			myExe, "0.25.0", []string{myExe}, false, false, true},
		{"чужой выбор не трогаем", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsEtc},
			myExe, "0.25.0", []string{oldExe}, true, false, false},
	}
	for _, c := range cases {
		got := assocPlan(assocIn{St: c.st, Exe: c.exe, Ver: c.ver, Exists: live(c.live...)})
		want := assocAct{Write: c.write, Foreign: c.foreignAct, Drop: c.drop}
		if got != want {
			t.Errorf("%s: %+v, ждали %+v", c.name, got, want)
		}
	}
}

func TestCmpVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.26.0", "0.25.3", 1},
		{"0.25.3", "0.26.0", -1},
		{"0.25.0", "0.25.0", 0},
		{"0.25.0", "0.25.0-rc2", 1},
		{"0.25.0-rc2", "0.25.0", -1},
		{"0.25.0-rc2", "0.25.0-rc10", 1}, // пред-выпуски — по строке: порядок rc важен лишь внутри одной версии
		{"0.25.0+abc", "0.25.0", 0},
		{"0.10.0", "0.9.9", 1},
		{"dev", "0.0.1", -1},
		{"0.0.1", "dev", 1},
		{"", "0.0.1", -1},
		{"dev", "", 0},
		{"1.0", "0.9.0", -1}, // «1.0» не x.y.z: не версия
	}
	for _, c := range cases {
		if got := cmpVer(c.a, c.b); got != c.want {
			t.Errorf("cmpVer(%q, %q) = %d, ждали %d", c.a, c.b, got, c.want)
		}
	}
}

func TestUserChoiceOurs(t *testing.T) {
	ex := func(ps ...string) func(string) bool {
		return func(p string) bool { return slices.Contains(ps, p) }
	}
	cases := []struct {
		name string
		st   osx.AssocState
		live []string
		want bool
	}{
		{"выбора нет", osx.AssocState{}, nil, false},
		{"exe старой версии", osx.AssocState{UserChoice: appsOld}, nil, true},
		{"exe без версии в имени, регистр", osx.AssocState{UserChoice: `applications\SOSNINAALBUMS.EXE`}, nil, true},
		{"чужой exe", osx.AssocState{UserChoice: appsEtc}, nil, false},
		{"чужой ProgID", osx.AssocState{UserChoice: "Other.Album"}, nil, false},
		{"имя похоже, но не наше", osx.AssocState{UserChoice: `Applications\notsosninaalbums.exe`}, nil, false},
		{"наш ProgID, команда жива", osx.AssocState{UserChoice: progOurs, Exe: myExe}, []string{myExe}, false},
		{"наш ProgID, exe пропал", osx.AssocState{UserChoice: progOurs, Exe: myExe}, nil, true},
		{"наш ProgID, команды нет", osx.AssocState{UserChoice: progOurs}, nil, true},
	}
	for _, c := range cases {
		if got := userChoiceOurs(c.st, ex(c.live...)); got != c.want {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}

// Без регистрации: нет реестра, не Windows, dev, стенд, временная папка, assoc: off — связь не
// пишется и не снимается; кроме off вопроса и пункта меню тоже нет (Can=false).
func TestAssocSkip(t *testing.T) {
	cases := []struct {
		name string
		set  func(a *App, r *fakeReg)
		view AssocView
	}{
		{"нет реестра", func(a *App, _ *fakeReg) { a.assoc.reg = nil }, AssocView{}},
		{"не Windows", func(_ *App, r *fakeReg) { r.unsupported = true }, AssocView{}},
		{"сборка dev", func(a *App, _ *fakeReg) { a.assoc.devBuild = true }, AssocView{}},
		{"стенд QA", func(a *App, _ *fakeReg) { a.data = "SosninaAlbums-stand-b" }, AssocView{}},
		{"запуск из архива", func(a *App, _ *fakeReg) {
			a.assoc.place = func() osx.Place { return osx.PlaceTemp }
		}, AssocView{}},
		// off: Can есть (пункт меню включит), вопроса нет, связь не пишется и не возвращается.
		{"assoc off", func(a *App, _ *fakeReg) {
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
	a.runAssoc()
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
	a.runAssoc()
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
	// Связь уже наша (установщик или прошлая версия): вопроса нет, runAssoc принимает её как согласие.
	reg2 := &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0"}}
	b := assocApp(t, reg2, oldExe)
	if v := b.assocView(); v != (AssocView{Can: true, On: true}) {
		t.Errorf("связь уже наша: %+v", v)
	}
	b.runAssoc()
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

// runAssoc по согласию: план применяется (моя версия новее — пишем, старее — нет, чужой ProgID —
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
		a.runAssoc()
		if len(reg.writes) != c.writes || reg.drops != c.drops || reg.st.UserChoice != c.choiceL {
			t.Errorf("%s: записей %d (ждали %d), выборов %d (ждали %d), выбор %q", c.name, len(reg.writes), c.writes, reg.drops, c.drops, reg.st.UserChoice)
		}
	}
	// Ошибка записи не роняет: связь не меняется, config остаётся.
	reg := &fakeReg{writeErr: errors.New("отказ")}
	a := assocApp(t, reg)
	a.updateConfig(func(cf *prefs.Config) { cf.Assoc = assocOn })
	a.runAssoc()
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
