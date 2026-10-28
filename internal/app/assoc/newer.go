package assoc

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// AfterPIDFlag — флаг «Открыть в 0.X»: новая версия ждёт выхода старой (по pid) и открывает файл.
// Контракт между версиями (0.25 запускает 0.26+): менять нельзя, сторож — TestAfterPidContract
// (разбор флага — launch.go в app).
const AfterPIDFlag = "--after-pid"

// NewerInfo — окно «Альбом из новой версии» (State.Newer, 112 design §5). App объявляет его
// определённым типом (`type NewerInfo assoc.NewerInfo`): в models.ts он остаётся типом app.
type NewerInfo struct {
	Name  string `json:"name"`  // «Свадьба.salbum»; у черновика пусто
	Draft bool   `json:"draft"` // черновик при старте
	Need  string `json:"need"`  // «0.26» (ShortVer поля app); "" — неизвестна
	Mine  string `json:"mine"`  // «0.25»
	Have  string `json:"have"`  // найденная новая версия «0.26»; "" — нет или не Windows
}

// ErrNoNewer — OpenInNewer без найденной новой версии (или её exe пропал).
var ErrNoNewer = errors.New("новая версия не найдена")

// SetNewer запоминает отказ открыть path (draft — черновик, path пуст) из-за ошибки err и
// возвращает данные окна. Общий путь openPath и старта.
func (s *Service) SetNewer(path string, draft bool, err error) NewerInfo {
	mine := s.h.Version()
	info := NewerInfo{Draft: draft, Mine: ShortVer(mine)}
	var ne *salbum.NewerError
	theirs := ""
	if errors.As(err, &ne) {
		info.Need, theirs = ShortVer(ne.App), ne.App
	}
	if !draft {
		info.Name = filepath.Base(path)
	}
	exe, ver := s.findNewer(info.Need)
	if exe != "" {
		info.Have = ShortVer(ver)
	}
	// Поиск выше шёл по major.minor; в окне при совпадении двух цифр — три
	if info.Need != "" && info.Need == info.Mine {
		info.Need, info.Mine = shownVers(theirs, mine)
		if info.Have != "" && ShortVer(ver) == ShortVer(theirs) {
			if full := fullVer(ver); full != "" {
				info.Have = full
			}
		}
	}
	s.nmu.Lock()
	s.path, s.exe = path, exe
	s.nmu.Unlock()
	return info
}

// StartNewer — отказ открыть кандидата запуска (альбом path или черновик): окно покажет первый
// GetState. Первый отказ побеждает — альбом из аргумента важнее последнего проекта и черновика.
func (s *Service) StartNewer(path string, draft bool, err error) {
	s.nmu.Lock()
	has := s.startAt != nil
	s.nmu.Unlock()
	if !has {
		info := s.SetNewer(path, draft, err)
		s.nmu.Lock()
		s.startAt = &info
		s.nmu.Unlock()
	}
}

// TakeStartNewer — отказ при старте, один раз (первый GetState кладёт его в State.Newer).
func (s *Service) TakeStartNewer() *NewerInfo {
	s.nmu.Lock()
	defer s.nmu.Unlock()
	info := s.startAt
	s.startAt = nil
	return info
}

// findNewer — самая новая версия программы на компьютере, которая откроет файл версии need
// («0.26»; пусто — неизвестна, тогда любая новее этой). Ищет в связи .salbum (exe команды) и в
// установке 111; берёт только существующий exe «sosninaalbums*.exe» вне временной папки и не
// эту программу. Не Windows — ничего не найдёт: реестр пуст.
func (s *Service) findNewer(need string) (exe, ver string) {
	reg := s.h.Reg()
	if reg == nil || !reg.Supported() {
		return "", ""
	}
	st := reg.Read()
	self, _ := reg.SelfExe()
	mine := s.h.Version()
	cands := []struct{ exe, ver string }{{st.Exe, theirVersion(st)}, {st.InstalledExe(), st.InstallVersion}}
	for _, c := range cands {
		if c.exe == "" || !verKnown(c.ver) || cmpVer(c.ver, mine) <= 0 || (self != "" && paths.SamePath(c.exe, self)) {
			continue
		}
		if !s.exeOK(c.exe) || !minorAtLeast(c.ver, need) {
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

// exeOK — exe годится для запуска «Открыть в»: локальный абсолютный путь, имя
// sosninaalbums*.exe, файл на месте, лежит не во временной папке (запуск из архива).
func (s *Service) exeOK(exe string) bool {
	name := strings.ToLower(filepath.Base(exe))
	return filepath.IsAbs(exe) && paths.CheckLocalPath(exe) == nil &&
		strings.HasPrefix(name, "sosninaalbums") && strings.HasSuffix(name, ".exe") &&
		s.h.Exists(exe) && osx.LaunchPlace(exe, s.h.Temp()) == osx.PlaceNormal
}

// OpenInNewer запускает найденную новую версию с файлом из окна «Альбом из новой версии»
// (без файла у черновика) и закрывает эту программу: альбом сохраняется, новая ждёт выхода этого
// процесса (AfterPIDFlag). Ошибка запуска — вернуть, программа работает. Путь от интерфейса не
// принимается: берётся запомненный SetNewer.
func (s *Service) OpenInNewer() (err error) {
	defer appkit.GuardErr("OpenInNewer", &err)
	s.nmu.Lock()
	path, exe := s.path, s.exe
	s.nmu.Unlock()
	if exe == "" || !s.exeOK(exe) {
		return ErrNoNewer
	}
	s.h.FlushSave()
	args := []string{AfterPIDFlag, strconv.Itoa(os.Getpid())}
	if path != "" {
		args = append(args, path)
	}
	if err := s.h.Start(exe, args...); err != nil {
		log.Printf("запуск новой версии: %v", paths.NoPath(err))
		return errors.New("не удалось запустить новую версию")
	}
	s.h.Quit()
	return nil
}
