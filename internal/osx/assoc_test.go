package osx

import (
	"errors"
	"strings"
	"testing"
)

// fakeReg — реестр в памяти: ключи без учёта регистра, как в Windows.
type fakeReg struct {
	vals    map[string]map[string]fakeVal
	notices int
	failSet string // подстрока ключа, на которой set падает
	failDel bool   // deleteTree падает (UserChoice под защитой)
}

type fakeVal struct {
	s    string
	none bool
}

func newFake() *fakeReg { return &fakeReg{vals: map[string]map[string]fakeVal{}} }

func (f *fakeReg) put(key, name string, v fakeVal) {
	key = strings.ToLower(key)
	if f.vals[key] == nil {
		f.vals[key] = map[string]fakeVal{}
	}
	f.vals[key][strings.ToLower(name)] = v
}

func (f *fakeReg) get(key, name string) (string, bool) {
	v, ok := f.vals[strings.ToLower(key)][strings.ToLower(name)]
	return v.s, ok && !v.none
}

func (f *fakeReg) set(key, name, val string) error {
	if f.failSet != "" && strings.Contains(key, f.failSet) {
		return errors.New("отказ")
	}
	f.put(key, name, fakeVal{s: val})
	return nil
}

func (f *fakeReg) setNone(key, name string) error {
	f.put(key, name, fakeVal{none: true})
	return nil
}

func (f *fakeReg) deleteTree(key string) error {
	if f.failDel {
		return errors.New("доступ запрещён")
	}
	key = strings.ToLower(key)
	for k := range f.vals {
		if k == key || strings.HasPrefix(k, key+`\`) {
			delete(f.vals, k)
		}
	}
	return nil
}

func (f *fakeReg) deleteValue(key, name string) error {
	delete(f.vals[strings.ToLower(key)], strings.ToLower(name))
	return nil
}

// deleteIfEmpty — ключ без значений и без подключей (подключ — любой ключ с префиксом key\).
func (f *fakeReg) deleteIfEmpty(key string) error {
	key = strings.ToLower(key)
	if len(f.vals[key]) > 0 {
		return nil
	}
	for k := range f.vals {
		if strings.HasPrefix(k, key+`\`) {
			return nil
		}
	}
	delete(f.vals, key)
	return nil
}

func (f *fakeReg) notify() { f.notices++ }

// hasKey — ключ есть (даже пустой).
func (f *fakeReg) hasKey(key string) bool {
	_, ok := f.vals[strings.ToLower(key)]
	return ok
}

func (f *fakeReg) has(key, name string) bool {
	_, ok := f.vals[strings.ToLower(key)][strings.ToLower(name)]
	return ok
}

// seed кладёт строку, минуя сбои (подготовка состояния теста).
func (f *fakeReg) seed(key, name, val string) { f.put(key, name, fakeVal{s: val}) }

const (
	testExe   = `C:\Users\Мария\Альбомы 0.25\SosninaAlbums.exe`
	choiceKey = `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\.salbum\UserChoice`
)

// Круг: запись → чтение возвращает то же; ключи — как у установщика (installer.nsi).
func TestAssocRoundTripFake(t *testing.T) {
	f := newFake()
	r := newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	got := r.Read()
	want := AssocState{ProgID: AssocProgID, Exe: testExe, AppVersion: "0.25.0"}
	if got != want {
		t.Fatalf("Read = %+v, ждали %+v", got, want)
	}
	if v, _ := f.get(`Software\Classes\SosninaAlbums.Album\shell\open\command`, ""); v != `"`+testExe+`" "%1"` {
		t.Errorf("команда = %q", v)
	}
	if v, _ := f.get(`Software\Classes\SosninaAlbums.Album\DefaultIcon`, ""); v != `"`+testExe+`",0` {
		t.Errorf("значок = %q", v)
	}
	if v, _ := f.get(`Software\Classes\SosninaAlbums.Album`, ""); v != "Альбом Sosnina Albums" {
		t.Errorf("описание = %q", v)
	}
	if !f.has(`Software\Classes\.salbum\OpenWithProgids`, AssocProgID) {
		t.Error("нет OpenWithProgids")
	}
	if f.notices != 1 {
		t.Errorf("SHChangeNotify вызван %d раз, ждали 1", f.notices)
	}
	// Повторная запись другой версией — перезапись, а не дубль.
	if err := r.Write(`D:\Новая\SosninaAlbums.exe`, "0.26.0"); err != nil {
		t.Fatal(err)
	}
	if got := r.Read(); got.Exe != `D:\Новая\SosninaAlbums.exe` || got.AppVersion != "0.26.0" {
		t.Errorf("после перезаписи: %+v", got)
	}
}

// Края Write: пустой путь и кавычка в пути — отказ без единой записи.
func TestAssocWriteRejectsBadExe(t *testing.T) {
	for _, exe := range []string{"", `C:\a"b\x.exe`} {
		f := newFake()
		if err := newReg("", f).Write(exe, "0.25.0"); err == nil {
			t.Errorf("Write(%q) без ошибки", exe)
		}
		if len(f.vals) != 0 || f.notices != 0 {
			t.Errorf("Write(%q) оставил записи: %v", exe, f.vals)
		}
	}
}

// Сбой посреди записи: расширение (последний шаг) не указывает на недописанный ProgID.
func TestAssocWriteFailureKeepsExtUntouched(t *testing.T) {
	f := newFake()
	f.failSet = `shell\open\command`
	r := newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err == nil {
		t.Fatal("ждали ошибку")
	}
	if f.has(`Software\Classes\.salbum`, "") {
		t.Error("расширение записано при сбое команды")
	}
	if f.notices != 0 {
		t.Error("уведомление при неудаче")
	}
}

// Чтение: чужая связь и установка 111 разбираются, пустой реестр — пустое состояние.
func TestAssocReadForeignAndInstall(t *testing.T) {
	f := newFake()
	r := newReg("", f)
	if got := r.Read(); got != (AssocState{}) {
		t.Fatalf("пустой реестр: %+v", got)
	}
	f.seed(`Software\Classes\.salbum`, "", "Other.Album")
	f.seed(`Software\SosninaAlbums`, "InstallDir", `C:\Users\I\AppData\Local\Programs`)
	f.seed(`Software\SosninaAlbums`, "Version", "0.24.1")
	f.seed(`Software\Classes\SosninaAlbums.Album\shell\open\command`, "", `C:\x\SosninaAlbums.exe %1`)
	got := r.Read()
	if got.ProgID != "Other.Album" || got.ProgIDIsOurs() {
		t.Errorf("ProgID = %q", got.ProgID)
	}
	if got.Exe != `C:\x\SosninaAlbums.exe` { // команда без кавычек
		t.Errorf("Exe = %q", got.Exe)
	}
	if got.InstallVersion != "0.24.1" || got.InstalledExe() == "" {
		t.Errorf("установка не прочитана: %+v", got)
	}
	if (AssocState{}).InstalledExe() != "" {
		t.Error("InstalledExe без InstallDir должен быть пуст")
	}
	if !(AssocState{ProgID: "sosninaalbums.album"}).ProgIDIsOurs() {
		t.Error("регистр ProgID не должен влиять")
	}
}

// DropUserChoice: наш выбор удаляется, чужой и отсутствующий — нет; отказ реестра — ошибка.
func TestAssocDropUserChoice(t *testing.T) {
	cases := []struct {
		id      string
		present bool
		dropped bool
	}{
		{AssocProgID, true, true},
		{`Applications\SosninaAlbums-0.24.exe`, true, true},
		{`Applications\sosninaalbums.exe`, true, true},
		{`Applications\notepad.exe`, true, false},
		{`Applications\sosninaalbums.txt`, true, false},
		{`Other.Album`, true, false},
		{"", false, false},
	}
	for _, c := range cases {
		f := newFake()
		r := newReg("", f)
		if c.present {
			f.seed(choiceKey, "ProgId", c.id)
			f.seed(choiceKey, "Hash", "xyz")
		}
		dropped, err := r.DropUserChoice()
		if err != nil || dropped != c.dropped {
			t.Errorf("%q: dropped=%v err=%v, ждали %v", c.id, dropped, err, c.dropped)
		}
		if want := c.present && !c.dropped; want != f.has(choiceKey, "ProgId") {
			t.Errorf("%q: ключ после вызова: есть=%v", c.id, f.has(choiceKey, "ProgId"))
		}
		if (f.notices == 1) != c.dropped {
			t.Errorf("%q: уведомлений %d", c.id, f.notices)
		}
	}
	f := newFake()
	f.seed(choiceKey, "ProgId", AssocProgID)
	f.failDel = true
	if dropped, err := newReg("", f).DropUserChoice(); err == nil || dropped {
		t.Errorf("отказ реестра: dropped=%v err=%v", dropped, err)
	}
}

// Remove: свои ключи уходят, чужое расширение и Applications\<exe> остаются.
func TestAssocRemove(t *testing.T) {
	f := newFake()
	r := newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	f.seed(choiceKey, "ProgId", AssocProgID)
	f.seed(`Software\Classes\Applications\SosninaAlbums.exe\shell\open\command`, "", "x")
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if got := r.Read(); got != (AssocState{}) {
		t.Errorf("после Remove: %+v", got)
	}
	if !f.has(`Software\Classes\Applications\SosninaAlbums.exe\shell\open\command`, "") {
		t.Error("след «Открыть с помощью» удалён")
	}
	if err := r.Remove(); err != nil { // повтор на пустом реестре
		t.Errorf("повторный Remove: %v", err)
	}

	// Расширение уже у чужой программы: его значение остаётся, наш ProgID уходит.
	f = newFake()
	r = newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	f.seed(`Software\Classes\.salbum`, "", "Other.Album")
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if got := r.Read(); got.ProgID != "Other.Album" || got.Exe != "" {
		t.Errorf("чужое расширение: %+v", got)
	}
}

// Remove не оставляет пустых .salbum и .salbum\OpenWithProgids; чужие значения держат ключи на месте.
func TestAssocRemoveLeavesNoEmptyKeys(t *testing.T) {
	const ext = `Software\Classes\.salbum`
	f := newFake()
	r := newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{ext, ext + `\OpenWithProgids`, `Software\Classes\` + AssocProgID} {
		if f.hasKey(k) {
			t.Errorf("ключ %q остался после Remove", k)
		}
	}
	if err := r.Remove(); err != nil { // повтор на пустом реестре
		t.Errorf("повторный Remove: %v", err)
	}

	// Чужой ProgID в OpenWithProgids: он и оба ключа остаются, наш ProgID уходит.
	f = newFake()
	r = newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	f.put(ext+`\OpenWithProgids`, "Other.Album", fakeVal{none: true})
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if !f.has(ext+`\OpenWithProgids`, "Other.Album") || f.has(ext+`\OpenWithProgids`, AssocProgID) {
		t.Errorf("OpenWithProgids после Remove: %+v", f.vals[strings.ToLower(ext+`\OpenWithProgids`)])
	}
	if !f.hasKey(ext) {
		t.Error(".salbum с чужим подключом удалён")
	}

	// Чужое значение по умолчанию держит .salbum, пустой OpenWithProgids уходит.
	f = newFake()
	r = newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	f.seed(ext, "", "Other.Album")
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if f.hasKey(ext+`\OpenWithProgids`) || !f.has(ext, "") {
		t.Errorf("чужое расширение: vals=%v", f.vals)
	}
}

const installKey = `Software\SosninaAlbums`

// Отметка AssocOff (установщик по ней не возвращает связь при тихом обновлении): ставит Remove,
// снимает Write; раздел установки (InstallDir, Version) не страдает; повторы безвредны.
func TestAssocOffMark(t *testing.T) {
	f := newFake()
	r := newReg("", f)
	if r.AssocOff() {
		t.Fatal("в пустом реестре отметка стоит")
	}
	f.seed(installKey, "InstallDir", `C:\Programs\Sosnina Albums`)
	f.seed(installKey, "Version", "0.25.0")
	for i := range 3 { // несколько кругов выкл → вкл
		if err := r.Write(testExe, "0.25.0"); err != nil {
			t.Fatal(err)
		}
		if r.AssocOff() || f.has(installKey, "AssocOff") {
			t.Fatalf("круг %d: после Write отметка стоит", i)
		}
		if err := r.Remove(); err != nil {
			t.Fatal(err)
		}
		if err := r.Remove(); err != nil { // холостой повтор
			t.Fatal(err)
		}
		if !r.AssocOff() {
			t.Fatalf("круг %d: после Remove отметки нет", i)
		}
		if got := r.Read(); got.InstallDir != `C:\Programs\Sosnina Albums` || got.InstallVersion != "0.25.0" {
			t.Fatalf("круг %d: раздел установки задет: %+v", i, got)
		}
	}
	// Значение не «1» (чужая запись) — не отметка.
	f.seed(installKey, "AssocOff", "0")
	if r.AssocOff() {
		t.Error(`AssocOff = "0" принят за отметку`)
	}
}

// Remove при отказе реестра: отметка уже стоит, остальное снимается, ошибка возвращается;
// Write при сбое не снимает отметку, пока связь не записана.
func TestAssocOffMarkFailures(t *testing.T) {
	f := newFake()
	r := newReg("", f)
	if err := r.Write(testExe, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	f.seed(choiceKey, "ProgId", AssocProgID)
	f.failDel = true // UserChoice под защитой
	if err := r.Remove(); err == nil {
		t.Fatal("ждали ошибку")
	}
	if !r.AssocOff() {
		t.Error("при сбое Remove отметка не поставлена")
	}

	f = newFake()
	r = newReg("", f)
	f.seed(installKey, "AssocOff", "1")
	f.failSet = `shell\open\command`
	if err := r.Write(testExe, "0.25.0"); err == nil {
		t.Fatal("ждали ошибку")
	}
	if !r.AssocOff() {
		t.Error("неудачный Write снял отметку")
	}
	// Reg без реестра отметки не знает.
	if newReg("", nil).AssocOff() {
		t.Error("nil-реестр: отметка")
	}
}

// commandExe: кавычки, пробелы в пути, пустое.
func TestCommandExe(t *testing.T) {
	for in, want := range map[string]string{
		`"C:\A B\x.exe" "%1"`: `C:\A B\x.exe`,
		`C:\x\y.exe "%1"`:     `C:\x\y.exe`,
		`  "C:\z.exe"`:        `C:\z.exe`,
		``:                    ``,
		`"`:                   ``,
	} {
		if got := commandExe(in); got != want {
			t.Errorf("commandExe(%q) = %q, ждали %q", in, got, want)
		}
	}
}

func TestSelfExe(t *testing.T) {
	exe, err := SelfExe()
	if err != nil || exe == "" {
		t.Fatalf("SelfExe = %q, %v", exe, err)
	}
}

// Reg без реестра (be == nil, не Windows) — всё no-op.
func TestAssocNilBackend(t *testing.T) {
	r := newReg("", nil)
	if r.Read() != (AssocState{}) || r.Write(testExe, "1") != nil || r.Remove() != nil {
		t.Error("nil-реестр должен молчать")
	}
	if d, err := r.DropUserChoice(); d || err != nil {
		t.Errorf("DropUserChoice: %v %v", d, err)
	}
}
