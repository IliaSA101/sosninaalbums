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
	"slices"
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
	return &placeEnv{rules: r, cloud: cloudRoots(runtime.GOOS, home, os.Getenv), system: system, vol: placeVolOf, volPath: placeVolPath}
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
		// У места с диском Dir — путь на момент привязки, буква могла смениться: строковые правила
		// для него применяет placeWhy к текущему пути (volPlaceSelf).
		if p.Vol != nil || env.static(p.Dir, p.How) == nil {
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
	dir, _, _, err := a.placeEnv().lift(last)
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

// pushPlaces ставит наверх списка родителей (how = parent, поднятых до корня архива — lift) или саму
// папку (chosen) — те, что прошли admit; одна запись на все. Места внутри других убираются
// (PushPlace). Не прошло или список не меняется — записи нет, добавление папки идёт как обычно.
func (a *App) pushPlaces(how string, dirs ...string) {
	a.seedPlaces()
	env := a.placeEnv()
	var add []prefs.Place
	seen, placed := map[string]bool{}, map[string]bool{}
	for _, d := range dirs {
		if len(add) == prefs.MaxPlaces {
			break // дальше всё равно не влезло бы в список
		}
		k := paths.DirKey(d)
		if seen[k] {
			continue
		}
		seen[k] = true
		var dir string
		var err error
		if how == prefs.PlaceParent {
			dir, _, _, err = env.lift(d)
		} else {
			dir, err = env.admit(d, how)
		}
		if err != nil || placed[paths.DirKey(dir)] {
			continue // не прошло, или соседний родитель поднялся в то же место
		}
		placed[paths.DirKey(dir)] = true
		// Диск пишет это действие человека (design 153 §4.4); не узнали — место по пути, как в 0.34.
		add = append(add, prefs.Place{Dir: dir, How: how, Vol: env.volOfDir(dir)})
	}
	cur := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(add) == 0 {
		return
	}
	binds := env.bindInside(cur, add)
	if sameOrder(cur, pushAll(cur, binds, add)) {
		return
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Places = pushAll(p.Places, binds, add) })
	a.agentsChanged()
}

// pushAll — add поверх list, первое из add — наверху; места без диска, оказавшиеся на диске новых
// (binds, см. bindInside), сначала получают его. list не меняется.
func pushAll(list []prefs.Place, binds map[string]prefs.PlaceVol, add []prefs.Place) []prefs.Place {
	tmp := prefs.AgentPerms{Places: list}
	tmp.BindPlaces(binds)
	list = tmp.Places
	for i := len(add) - 1; i >= 0; i-- {
		list = prefs.PushPlace(list, add[i])
	}
	return list
}

// sameOrder — списки мест одинаковы (диск или путь, способ — по порядку): писать нечего.
func sameOrder(a, b []prefs.Place) bool {
	return slices.EqualFunc(a, b, func(x, y prefs.Place) bool { return x.How == y.How && x.Key() == y.Key() })
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
	env := a.placeEnv()
	place, err := env.admit(dir, prefs.PlaceChosen)
	if err != nil {
		return a.agentsInfo(), err
	}
	// Диск пишет это действие человека; другой диск по тому же пути — новое место, не перепривязка.
	np := prefs.Place{Dir: place, How: prefs.PlaceChosen, Vol: env.volOfDir(place)}
	binds := env.bindInside(prefs.LoadAgentPerms(a.cfgDir).Places, []prefs.Place{np})
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Places = pushAll(p.Places, binds, []prefs.Place{np}) })
	a.agentsChanged()
	return a.agentsInfo(), nil
}
