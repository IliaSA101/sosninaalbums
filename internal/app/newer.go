package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/update"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Версия программы в файле (098 design §6): текст «файл новее программы» с версией записавшей
// программы и тост о копии старого формата. Отдельно от files.go: тот у предела строк.

// newerText — текст ошибки открытия err для what (первое слово как есть: «альбом» в ошибку для
// фронта — со строчной, как ошибки Go; «Черновик» в начало уведомления): файл новее программы с
// версией из поля app — «альбом сохранён в версии 0.25. Эта программа — 0.23: обновите её»;
// версии в файле нет — «… сохранён более новой версией программы»; не ErrNewer — err.Error().
// Без точки в конце.
func newerText(err error, what string) string { return newerTextFor(err, what, appVersion(wailsJSON)) }

// newerTextFor — newerText с версией этой программы mine (тесты подставляют свою).
func newerTextFor(err error, what, mine string) string {
	var ne *salbum.NewerError
	if !errors.As(err, &ne) {
		if errors.Is(err, salbum.ErrNewer) {
			return what + " сохранён более новой версией программы"
		}
		return err.Error()
	}
	theirs, mineShown := shownVers(ne.App, mine)
	switch {
	case theirs == "":
		return what + " сохранён более новой версией программы"
	case mineShown == "":
		return what + " сохранён в версии " + theirs + ": обновите программу"
	}
	return what + " сохранён в версии " + theirs + ". Эта программа — " + mineShown + ": обновите её"
}

// shownVers — версии файла и программы для текста: major.minor; если они совпали (файл 0.25.1,
// программа 0.25.0) — три цифры: «сохранён в 0.25, а программа — 0.25» ничего не объясняет.
// Полной версии нет (в файле «0.25») — остаются две.
func shownVers(theirs, mine string) (t, m string) {
	t, m = shortVer(theirs), shortVer(mine)
	if t == "" || t != m {
		return t, m
	}
	if tf, mf := fullVer(theirs), fullVer(mine); tf != "" && mf != "" {
		return tf, mf
	}
	return t, m
}

// fullVer — «0.25.1-rc2+abc» → «0.25.1»; не версия (dev, «0.25») — "".
func fullVer(s string) string {
	v, _, ok := splitVer(s)
	if !ok {
		return ""
	}
	return strconv.Itoa(v[0]) + "." + strconv.Itoa(v[1]) + "." + strconv.Itoa(v[2])
}

// shortVer — «0.25.1-rc2» → «0.25»: в тексте важны major.minor. Не версия (dev, пусто) — "".
func shortVer(v string) string {
	major, rest, ok := strings.Cut(v, ".")
	if !ok || !digits(major) {
		return ""
	}
	minor := rest
	if i := strings.IndexFunc(rest, func(c rune) bool { return c < '0' || c > '9' }); i >= 0 {
		minor = rest[:i]
	}
	if minor == "" {
		return ""
	}
	return major + "." + minor
}

// digits — непустая строка цифр.
func digits(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }

// openErr — ошибка открытия альбома для ответа фронту: файл новее программы — с версией.
func openErr(err error) error {
	if errors.Is(err, salbum.ErrNewer) {
		return errors.New(newerText(err, "альбом"))
	}
	return err
}

// noticeBackup — тост о копии старого формата, сделанной последней записью ed (Save, WriteDraft):
// «Сохранена копия для версии 0.22: «Свадьба (для 0.22).salbum»», один раз (Editor.TakeBackup
// забывает имя). Звать после успешной записи. В log.txt — без имени: в нём имя альбома.
func (a *App) noticeBackup(ed *album.Editor) {
	bak := ed.TakeBackup()
	if bak == "" {
		return
	}
	log.Print("сохранена копия файла для прежней версии программы")
	a.emit("notice", backupText(bak))
}

// backupText — текст тоста о копии bak: версия — из хвоста имени « (для 0.22)» (BackupName).
func backupText(bak string) string {
	name := filepath.Base(bak)
	if ver := tailVer(name); ver != "" {
		return "Сохранена копия для версии " + ver + ": «" + name + "»"
	}
	return "Сохранена копия для прежней версии: «" + name + "»"
}

// tailVer — версия из последнего « (для X.Y)» в имени; нет — "".
func tailVer(name string) string {
	const head = " (для "
	i := strings.LastIndex(name, head)
	if i < 0 {
		return ""
	}
	ver, _, ok := strings.Cut(name[i+len(head):], ")")
	if !ok || shortVer(ver) != ver {
		return ""
	}
	return ver
}

// NewerInfo — окно «Альбом из новой версии» (State.Newer, 112 design §5).
type NewerInfo struct {
	Name  string `json:"name"`  // «Свадьба.salbum»; у черновика пусто
	Draft bool   `json:"draft"` // черновик при старте
	Need  string `json:"need"`  // «0.26» (shortVer поля app); "" — неизвестна
	Mine  string `json:"mine"`  // «0.25»
	Have  string `json:"have"`  // найденная новая версия «0.26»; "" — нет или не Windows
}

// newerState — последний отказ «файл новее» и что нужно OpenInNewer. Под своим замком: пишут
// открытие (UI) и старт, читает OpenInNewer.
type newerState struct {
	mu    sync.Mutex
	path  string     // альбом, который откроет новая версия; у черновика пусто
	exe   string     // найденная новая версия; "" — нет
	start *NewerInfo // отказ при старте: первый GetState отдаёт один раз
	// Подмены тестов; nil — настоящие.
	startFn func(exe string, args ...string) error
	quitFn  func()
	temp    func() string
}

// setNewer запоминает отказ открыть path (draft — черновик, path пуст) из-за ошибки err и
// возвращает данные окна. Общий путь openPath и старта.
func (a *App) setNewer(path string, draft bool, err error) NewerInfo {
	mine := appVersion(wailsJSON)
	info := NewerInfo{Draft: draft, Mine: shortVer(mine)}
	var ne *salbum.NewerError
	theirs := ""
	if errors.As(err, &ne) {
		info.Need, theirs = shortVer(ne.App), ne.App
	}
	if !draft {
		info.Name = filepath.Base(path)
	}
	exe, ver := a.findNewer(info.Need)
	if exe != "" {
		info.Have = shortVer(ver)
	}
	// Поиск выше шёл по major.minor; в окне при совпадении двух цифр — три
	if info.Need != "" && info.Need == info.Mine {
		info.Need, info.Mine = shownVers(theirs, mine)
		if info.Have != "" && shortVer(ver) == shortVer(theirs) {
			if full := fullVer(ver); full != "" {
				info.Have = full
			}
		}
	}
	a.newer.mu.Lock()
	a.newer.path, a.newer.exe = path, exe
	a.newer.mu.Unlock()
	return info
}

// startNewer — отказ открыть кандидата запуска (альбом path или черновик): окно покажет первый
// GetState. Первый отказ побеждает — альбом из аргумента важнее последнего проекта и черновика.
func (a *App) startNewer(path string, draft bool, err error) {
	a.newer.mu.Lock()
	has := a.newer.start != nil
	a.newer.mu.Unlock()
	if !has {
		a.holdStartNewer(a.setNewer(path, draft, err))
	}
}

// holdStartNewer — отказ при старте: первый GetState отдаёт его в State.Newer один раз.
func (a *App) holdStartNewer(info NewerInfo) {
	a.newer.mu.Lock()
	a.newer.start = &info
	a.newer.mu.Unlock()
}

// takeStartNewer — отказ при старте, один раз.
func (a *App) takeStartNewer() *NewerInfo {
	a.newer.mu.Lock()
	defer a.newer.mu.Unlock()
	info := a.newer.start
	a.newer.start = nil
	return info
}

// tempDir — временная папка для проверки места запуска (тест подменяет).
func (a *App) tempDir() string {
	a.newer.mu.Lock()
	temp := a.newer.temp
	a.newer.mu.Unlock()
	if temp != nil {
		return temp()
	}
	return os.TempDir()
}

// findNewer — самая новая версия программы на компьютере, которая откроет файл версии need
// («0.26»; пусто — неизвестна, тогда любая новее этой). Ищет в связи .salbum (exe команды) и в
// установке 111; берёт только существующий exe «sosninaalbums*.exe» вне временной папки и не
// эту программу. Не Windows — ничего не найдёт: реестр пуст.
func (a *App) findNewer(need string) (exe, ver string) {
	reg := a.assoc.reg
	if reg == nil || !reg.Supported() {
		return "", ""
	}
	st := reg.Read()
	self, _ := reg.SelfExe()
	mine := appVersion(wailsJSON)
	cands := []struct{ exe, ver string }{{st.Exe, theirVersion(st)}, {st.InstalledExe(), st.InstallVersion}}
	for _, c := range cands {
		if c.exe == "" || !verKnown(c.ver) || cmpVer(c.ver, mine) <= 0 || (self != "" && paths.SamePath(c.exe, self)) {
			continue
		}
		if !a.newerExeOK(c.exe) || !minorAtLeast(c.ver, need) {
			continue
		}
		if exe == "" || cmpVer(c.ver, ver) > 0 {
			exe, ver = c.exe, c.ver
		}
	}
	return exe, ver
}

// minorAtLeast — major.minor версии ver не ниже need («0.26»); need пусто — подходит любая.
func minorAtLeast(ver, need string) bool {
	if need == "" {
		return true
	}
	v, _, ok := splitVer(ver)
	n, nok := update.ParseVersion(need + ".0")
	if !ok || !nok {
		return false
	}
	return v[0] > n[0] || v[0] == n[0] && v[1] >= n[1]
}

// newerExeOK — exe годится для запуска «Открыть в»: локальный абсолютный путь, имя
// sosninaalbums*.exe, файл на месте, лежит не во временной папке (запуск из архива).
func (a *App) newerExeOK(exe string) bool {
	name := strings.ToLower(filepath.Base(exe))
	return filepath.IsAbs(exe) && paths.CheckLocalPath(exe) == nil &&
		strings.HasPrefix(name, "sosninaalbums") && strings.HasSuffix(name, ".exe") &&
		a.exists(exe) && osx.LaunchPlace(exe, a.tempDir()) == osx.PlaceNormal
}

// errNoNewer — OpenInNewer без найденной новой версии (или её exe пропал).
var errNoNewer = errors.New("новая версия не найдена")

// OpenInNewer запускает найденную новую версию с файлом из окна «Альбом из новой версии»
// (без файла у черновика) и закрывает эту программу: альбом сохраняется, новая ждёт выхода этого
// процесса (afterPIDFlag). Ошибка запуска — вернуть, программа работает. Путь от интерфейса не
// принимается: берётся запомненный setNewer.
func (a *App) OpenInNewer() error {
	defer guard("OpenInNewer")
	a.newer.mu.Lock()
	path, exe := a.newer.path, a.newer.exe
	start, quit := a.newer.startFn, a.newer.quitFn
	a.newer.mu.Unlock()
	if exe == "" || !a.newerExeOK(exe) {
		return errNoNewer
	}
	if start == nil {
		start = osx.Start
	}
	a.flushSave()
	args := []string{afterPIDFlag, strconv.Itoa(os.Getpid())}
	if path != "" {
		args = append(args, path)
	}
	if err := start(exe, args...); err != nil {
		log.Printf("запуск новой версии: %v", paths.NoPath(err))
		return errors.New("не удалось запустить новую версию")
	}
	if quit == nil {
		quit = func() {
			if a.ctx != nil {
				wr.Quit(a.ctx)
			}
		}
	}
	quit()
	return nil
}
