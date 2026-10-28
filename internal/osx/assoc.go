package osx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Связь .salbum с программой (спека 112, design §2). Ключи — те же, что пишет установщик 111
// (scripts/win/installer.nsi): ProgID и раздел установки общие, поэтому zip-копия и установленная
// не воюют за расширение. Только HKCU\Software\Classes; в HKLM не пишем и не читаем. Решений
// (кому принадлежит связь, когда писать) здесь нет — они в internal/app/assoc (assoc.plan), osx лишь
// читает и пишет реестр и версий не сравнивает.

const (
	// AssocExt — расширение альбома.
	AssocExt = ".salbum"
	// AssocProgID — ProgID программы; общий с установщиком, менять нельзя.
	AssocProgID = "SosninaAlbums.Album"
	// InstallExeName — имя exe установленной копии: InstallDir\SosninaAlbums.exe.
	InstallExeName = "SosninaAlbums.exe"

	assocTitle = "Альбом Sosnina Albums"
	// assocOffName — отметка «пользователь выключил связь» в разделе установки: значение
	// AssocOff=1 ставит Remove (SetAssoc(false)), снимает Write; установщик 111 читает её и при
	// тихом обновлении не возвращает связь. Удаление программы убирает её вместе с разделом.
	assocOffName = "AssocOff"
)

// AssocState — что сейчас в реестре; пустая строка — значения нет (или не Windows).
type AssocState struct {
	ProgID         string // (по умолчанию) у .salbum: кому принадлежит расширение
	Exe            string // exe из shell\open\command нашего ProgID
	AppVersion     string // версия того, кто записал связь (ProgID\AppVersion)
	InstallDir     string // Software\SosninaAlbums\InstallDir: где стоит установленная копия
	InstallVersion string // Software\SosninaAlbums\Version
	UserChoice     string // FileExts\.salbum\UserChoice\ProgId: «Открыть с помощью → всегда»
}

// ProgIDIsOurs — ProgID у расширения наш (без учёта регистра: реестр регистр не различает).
func (s AssocState) ProgIDIsOurs() bool { return strings.EqualFold(s.ProgID, AssocProgID) }

// UserChoiceIsOurs — выбор пользователя ведёт в нашу программу: наш ProgID или
// Applications\sosninaalbums*.exe (так Windows называет exe, выбранный через «Открыть с
// помощью»). Чужой выбор (Applications\other.exe, чужой ProgID) — не наш.
func (s AssocState) UserChoiceIsOurs() bool { return ownChoice(s.UserChoice) }

// InstalledExe — exe установленной копии; пусто, если установщик не писал InstallDir.
func (s AssocState) InstalledExe() string {
	if s.InstallDir == "" {
		return ""
	}
	return filepath.Join(s.InstallDir, InstallExeName)
}

func ownChoice(progID string) bool {
	if strings.EqualFold(progID, AssocProgID) {
		return true
	}
	name, ok := strings.CutPrefix(strings.ToLower(progID), `applications\`)
	return ok && strings.HasPrefix(name, "sosninaalbums") && strings.HasSuffix(name, ".exe")
}

// regBackend — HKCU по путям от корня пользователя; пустое имя — значение «(по умолчанию)».
// Реализует Windows (advapi32), в тестах — память. Нет ключа или значения при удалении — не
// ошибка; при чтении — ok=false.
type regBackend interface {
	get(key, name string) (val string, ok bool)
	set(key, name, val string) error
	setNone(key, name string) error // REG_NONE без данных (OpenWithProgids)
	deleteTree(key string) error
	deleteValue(key, name string) error
	deleteIfEmpty(key string) error // удаляет ключ, только если в нём нет значений и подключей
	notify()                        // SHChangeNotify(ASSOCCHANGED): значки обновятся без перезапуска Проводника
}

// Reg — ветки реестра под связь .salbum. Корень — параметр, чтобы тест писал не в настоящую
// Software\Classes, а в свою ветку (newReg).
type Reg struct {
	classes string // аналог Software\Classes
	install string // аналог Software\SosninaAlbums
	choice  string // аналог …\Explorer\FileExts\.salbum\UserChoice
	be      regBackend
}

// newReg — root "" — настоящие ветки; иначе всё под HKCU\<root> (тест).
func newReg(root string, be regBackend) Reg {
	if root == "" {
		return Reg{
			classes: `Software\Classes`,
			install: `Software\SosninaAlbums`,
			choice:  `Software\Microsoft\Windows\CurrentVersion\Explorer\FileExts\` + AssocExt + `\UserChoice`,
			be:      be,
		}
	}
	return Reg{
		classes: root + `\Classes`,
		install: root + `\SosninaAlbums`,
		choice:  root + `\FileExts\` + AssocExt + `\UserChoice`,
		be:      be,
	}
}

// sys — настоящий реестр; на других ОС be пуст и всё — no-op.
var sys = newReg("", sysBackend())

// Supported — связь .salbum на этой ОС вообще можно записать (Windows).
func Supported() bool { return sys.be != nil }

// ReadAssoc читает состояние связи; не Windows — пустое.
func ReadAssoc() AssocState { return sys.Read() }

// WriteAssoc записывает связь на exe версии version (HKCU, без прав администратора).
func WriteAssoc(exe, version string) error { return sys.Write(exe, version) }

// RemoveAssoc снимает нашу связь: ProgID, ссылку на него и наш UserChoice.
func RemoveAssoc() error { return sys.Remove() }

// DropUserChoice удаляет UserChoice, если он наш (иначе Windows игнорирует связь HKCU и
// открывает прежним exe); dropped — ключ был и удалён. Чужой выбор не трогает.
func DropUserChoice() (dropped bool, err error) { return sys.DropUserChoice() }

// Read — состояние связи.
func (r Reg) Read() AssocState {
	if r.be == nil {
		return AssocState{}
	}
	var s AssocState
	s.ProgID, _ = r.be.get(r.classes+`\`+AssocExt, "")
	cmd, _ := r.be.get(r.progKey()+`\shell\open\command`, "")
	s.Exe = commandExe(cmd)
	s.AppVersion, _ = r.be.get(r.progKey(), "AppVersion")
	s.InstallDir, _ = r.be.get(r.install, "InstallDir")
	s.InstallVersion, _ = r.be.get(r.install, "Version")
	s.UserChoice, _ = r.be.get(r.choice, "ProgId")
	return s
}

func (r Reg) progKey() string { return r.classes + `\` + AssocProgID }

// Write пишет ProgID целиком, и лишь потом расширение на него: расширение никогда не ведёт
// на ProgID без команды открытия. Прежнее значение расширения (чужой ProgID) перезаписывается —
// решает вызывающий (assoc.plan).
func (r Reg) Write(exe, version string) error {
	if r.be == nil {
		return nil
	}
	if exe == "" || strings.ContainsRune(exe, '"') {
		return fmt.Errorf("assoc: недопустимый путь программы %q", exe)
	}
	pk := r.progKey()
	steps := []func() error{
		func() error { return r.be.set(pk, "", assocTitle) },
		func() error { return r.be.set(pk, "AppVersion", version) },
		func() error { return r.be.set(pk+`\DefaultIcon`, "", `"`+exe+`",0`) },
		func() error { return r.be.set(pk+`\shell\open\command`, "", `"`+exe+`" "%1"`) },
		func() error { return r.be.setNone(r.classes+`\`+AssocExt+`\OpenWithProgids`, AssocProgID) },
		func() error { return r.be.set(r.classes+`\`+AssocExt, "", AssocProgID) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return fmt.Errorf("assoc: запись связи: %w", err)
		}
	}
	// Явное включение снимает отметку; не снялась — установщик не вернёт связь, говорим об этом
	if err := r.be.deleteValue(r.install, assocOffName); err != nil {
		return fmt.Errorf("assoc: отметка выключения не снялась: %w", err)
	}
	r.be.notify()
	return nil
}

// AssocOff — пользователь выключил связь (отметка стоит). Не Windows — false.
func (r Reg) AssocOff() bool {
	if r.be == nil {
		return false
	}
	v, ok := r.be.get(r.install, assocOffName)
	return ok && v == "1"
}

// Remove — снятие связи (§2.4) и отметка «выключено» (AssocOff). Ключ Applications\<exe> («Открыть с помощью») не трогаем.
// Ошибки копятся: один отказ не должен оставить остальное.
func (r Reg) Remove() error {
	if r.be == nil {
		return nil
	}
	ext := r.classes + `\` + AssocExt
	// Отметка первой: даже при сбое остальных шагов установщик связь не вернёт
	errs := []error{r.be.set(r.install, assocOffName, "1")}
	if cur, _ := r.be.get(ext, ""); strings.EqualFold(cur, AssocProgID) {
		errs = append(errs, r.be.deleteValue(ext, ""))
	}
	errs = append(errs,
		r.be.deleteValue(ext+`\OpenWithProgids`, AssocProgID),
		r.be.deleteTree(r.progKey()),
		// Пустые ключи после себя не оставляем; чужие значения (другой ProgID, чужой
		// OpenWithProgids) держат ключ на месте. Порядок: сначала вложенный
		r.be.deleteIfEmpty(ext+`\OpenWithProgids`),
		r.be.deleteIfEmpty(ext),
	)
	_, err := r.dropChoice()
	errs = append(errs, err)
	r.be.notify()
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("assoc: снятие связи: %w", err)
	}
	return nil
}

// DropUserChoice — см. пакетную функцию.
func (r Reg) DropUserChoice() (bool, error) {
	dropped, err := r.dropChoice()
	if dropped {
		r.be.notify()
	}
	return dropped, err
}

func (r Reg) dropChoice() (bool, error) {
	if r.be == nil {
		return false, nil
	}
	id, ok := r.be.get(r.choice, "ProgId")
	if !ok || !ownChoice(id) {
		return false, nil
	}
	if err := r.be.deleteTree(r.choice); err != nil {
		return false, fmt.Errorf("assoc: UserChoice не удалился: %w", err)
	}
	return true, nil
}

// commandExe — путь программы из команды открытия: `"C:\A B\x.exe" "%1"` → `C:\A B\x.exe`;
// без кавычек — до первого пробела.
func commandExe(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if rest, ok := strings.CutPrefix(cmd, `"`); ok {
		exe, _, _ := strings.Cut(rest, `"`)
		return exe
	}
	exe, _, _ := strings.Cut(cmd, " ")
	return exe
}

// SelfExe — путь запущенной программы для связи: без символических ссылок и коротких имён 8.3
// (Проводник показывает и сравнивает длинные).
func SelfExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return longPath(exe), nil
}
