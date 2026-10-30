package app

// Места съёмок на стороне App (150, design §4.2): источники пополнения, засев при переходе на 0.34,
// окно «Папка со съёмками…». Источники — только действия, начатые человеком: ChooseFolder и бросок
// папки (dropPaths) пополняют места родителем добавленной папки; «Папка со съёмками…» — самой
// папкой. Ответ на вопрос агента (Добавить / Создать / Выбрать другую…) места не пополняет:
// приманка «Pictures\Свадьба» одним щелчком сделала бы местом всю «Изображения» (0.34-150-16).

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// openDirDialog — окно ОС «Папка»; тест подменяет (нативные диалоги в тестах не открываются).
var openDirDialog = wr.OpenDirectoryDialog

// seedMu держит засев одним: два потока, увидевшие «ключа нет», засеяли бы дважды.
var seedMu sync.Mutex

// placeEnv — правила мест этой машины поверх правил выбранной папки (askM().rules — в тестах мягкие).
func (a *App) placeEnv() *placeEnv {
	r := a.askM().rules()
	if r == nil {
		return nil // fail-closed: правил нет — места не пополняются и не читаются
	}
	home, _ := os.UserHomeDir()
	if !filepath.IsAbs(home) {
		home = ""
	}
	system := os.Getenv("SystemDrive")
	if system == "" {
		system = "C:"
	}
	return &placeEnv{rules: r, cloud: cloudRoots(runtime.GOOS, home, os.Getenv), system: system}
}

// usablePlaces — места из config.json, прошедшие строковые правила сейчас (место, ставшее негодным
// после смены правил, пропускается: тихо в нём нельзя). Диск не трогает.
func (a *App) usablePlaces() []prefs.Place {
	places := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(places) == 0 {
		return nil
	}
	env := a.placeEnv()
	var out []prefs.Place
	for _, p := range places {
		if env.static(p.Dir, p.How) == nil {
			out = append(out, p)
		}
	}
	return out
}

// seedCandidate — каким будет засев при переходе на 0.34: место из lastDirs.photos как «parent», если
// проходит правила; Dir == "" — засевать нечего. Ничего не пишет: общий отбор для seedPlaces и для
// показа в окне (человек видит место в согласии до первой записи).
func (a *App) seedCandidate(cfg prefs.Config) prefs.Place {
	last := cfg.LastDirs[prefs.DirPhotos]
	if last == "" {
		return prefs.Place{}
	}
	dir, err := a.placeEnv().admit(last, prefs.PlaceParent)
	if err != nil {
		return prefs.Place{}
	}
	return prefs.Place{Dir: dir, How: prefs.PlaceParent}
}

// seedPlaces — переход на 0.34 (нет ключа agentPerms): место из lastDirs.photos как «parent», если
// проходит правила, и сам ключ пишется — засев один раз. Зовётся перед первой записью разрешений и
// перед перезаписью lastDirs.photos (её пишет и ответ на вопрос агента — файл должен быть засеян
// до этого, иначе в место попала бы папка, названная агентом).
func (a *App) seedPlaces() {
	seedMu.Lock()
	defer seedMu.Unlock()
	cfg := a.loadConfig()
	if cfg.AgentPerms != nil {
		return
	}
	seed := a.seedCandidate(cfg)
	// Запись от сверенного вида (столбец миграции по agents.edit) закрепляет и ключ, и умолчания.
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		if seed.Dir != "" && len(p.Places) == 0 {
			p.Places = prefs.PushPlace(p.Places, seed)
		}
	})
}

// pushPlaces ставит наверх списка родителей (how = parent) или саму папку (chosen) — те, что прошли
// admit; одна запись на все. Не прошло — места не меняются, добавление папки идёт как обычно.
func (a *App) pushPlaces(how string, dirs ...string) {
	a.seedPlaces()
	env := a.placeEnv()
	var add []prefs.Place
	seen := map[string]bool{}
	for _, d := range dirs {
		if len(add) == prefs.MaxPlaces {
			break // дальше всё равно не влезло бы в список
		}
		k := paths.DirKey(d)
		if seen[k] {
			continue
		}
		seen[k] = true
		if dir, err := env.admit(d, how); err == nil {
			add = append(add, prefs.Place{Dir: dir, How: how})
		}
	}
	if len(add) == 0 || placesOnTop(prefs.LoadAgentPerms(a.cfgDir).Places, add) {
		return
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		for i := len(add) - 1; i >= 0; i-- { // первая брошенная — наверху
			p.Places = prefs.PushPlace(p.Places, add[i])
		}
	})
	a.agentsChanged()
}

// placesOnTop — add уже стоит в начале списка в том же порядке и с тем же способом: писать нечего.
func placesOnTop(list, add []prefs.Place) bool {
	if len(list) < len(add) {
		return false
	}
	for i, p := range add {
		if paths.DirKey(list[i].Dir) != paths.DirKey(p.Dir) || list[i].How != p.How {
			return false
		}
	}
	return true
}

// addChosenFolder — «Добавить папку…» после окна ОС: папка в проект, запомнить родителя для следующего
// окна и пополнить места родителем. Одна запись мест на выбор.
func (a *App) addChosenFolder(dir string) (model.View, error) {
	v, err := a.AddFolderPath(dir)
	if err == nil {
		parent := filepath.Dir(dir)
		a.rememberDir(prefs.DirPhotos, parent) // соседняя съёмка обычно лежит рядом
		a.pushPlaces(prefs.PlaceParent, parent)
	}
	return v, err
}

// placeStart — где открыть окно «Папка со съёмками…»: первое место или «Изображения», но не
// lastDirs.photos (его пишет и ответ на вопрос агента — щелчок «Выбрать» по приманке дал бы агенту
// целую ветку). "" — пусть решает система.
func (a *App) placeStart() string {
	for _, p := range a.usablePlaces() {
		if isDirLocal(p.Dir) {
			return p.Dir
		}
	}
	if d := osx.PicturesDir(); d != "" && isDirLocal(d) {
		return d
	}
	return ""
}

// isDirLocal — локальная папка есть: CheckLocalPath, затем StatLocal под пределом (отключённый
// сетевой диск не вешает окно).
func isDirLocal(p string) bool {
	if paths.CheckLocalPath(p) != nil {
		return false
	}
	ok, done := within(osx.ProbeTimeout, func() bool {
		st, err := library.StatLocal(p)
		return err == nil && st.IsDir()
	})
	return done && ok
}

// ChooseAgentPlace — «Папка со съёмками…»: окно ОС, выбранная папка становится местом (явным).
// Отмена окна — без изменений. Папка, которой нельзя быть местом, — ошибка, места прежние.
func (a *App) ChooseAgentPlace() (info AgentsInfo, err error) {
	defer guardErr("ChooseAgentPlace", &err)
	a.seedPlaces()
	dir, err := openDirDialog(a.ctx, wr.OpenDialogOptions{Title: "Папка со съёмками", DefaultDirectory: a.placeStart()})
	if err != nil {
		return a.agentsInfo(), err
	}
	if dir == "" {
		return a.agentsInfo(), nil
	}
	place, err := a.placeEnv().admit(dir, prefs.PlaceChosen)
	if err != nil {
		return a.agentsInfo(), err
	}
	prefs.AddPlace(a.cfgDir, prefs.Place{Dir: place, How: prefs.PlaceChosen})
	a.agentsChanged()
	return a.agentsInfo(), nil
}
