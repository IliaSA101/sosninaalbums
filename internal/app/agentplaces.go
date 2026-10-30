package app

// Правила мест съёмок (150, design §4.2): где «Всегда» у папки берёт папку без вопроса. Строковые
// правила — при каждом чтении мест (дёшево, диск не трогают); дисковые (ссылки, st_dev, mountinfo)
// — только при пополнении и засеве, под within и одной пробой. prefs.clean() проверяет лишь
// строение (0.34-150-30): правила 149 живут здесь, prefs их не импортирует. Агент мест не видит
// и не меняет; источники — agentplaces_app.go.

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// errPlaceBad — папка не годится как место съёмок (причина не уточняется: путь в ошибку не идёт).
var errPlaceBad = errors.New("эту папку нельзя сделать местом съёмок: слишком широкая, системная или недоступная")

// Подмена в тестах: таблица монтирования и st_dev — диск машины.
var (
	placeMounts = osx.MountPoints
	placeDev    = devOf
)

// placeEnv — правила 149 и то, что нужно местам сверх них: корни облачных папок и системный диск.
type placeEnv struct {
	rules  *folderRules
	cloud  []string // корни облачных папок: сами и прямые подпапки не место из родителя
	system string   // системный диск Windows («C:»); его корень — не место даже явное
}

// cloudRoots — корни облачных папок ОС goos (Windows — из окружения, macOS — iCloud Drive и
// CloudStorage). Linux-клиенты лежат прямо под домашней, их закрывает правило «не подпапка домашней».
func cloudRoots(goos, home string, env func(string) string) []string {
	var out []string
	switch goos {
	case "windows":
		for _, k := range []string{"OneDrive", "OneDriveConsumer", "OneDriveCommercial"} {
			if v := env(k); v != "" {
				out = append(out, v)
			}
		}
	case "darwin":
		if home != "" {
			// CloudStorage — папка провайдеров: её дети сами корни облаков (OneDrive-…, Dropbox).
			out = append(out, path.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs"),
				path.Join(home, "Library", "CloudStorage"))
		}
	}
	return out
}

// parent — родитель очищенного пути (корень остаётся корнем).
func (r *folderRules) parent(p string) string {
	i := strings.LastIndex(p, r.sep())
	switch {
	case r.windows && i <= 2:
		return p[:min(3, len(p))]
	case i <= 0:
		return "/"
	}
	return p[:i]
}

// unixMountShape — путь вида точек монтирования съёмных носителей Unix: userRoot — /media/<u>,
// /run/media/<u> (в нём все карты разом); volume — /media/<u>/X, /run/media/<u>/X, /Volumes/X,
// /mnt/X (корень одного тома). Правило 149 отсекает /media и /run/media только целиком, а st_dev
// у /media/<u> тот же, что у /media: сравнением устройств это не поймать.
func unixMountShape(p string) (userRoot, volume bool) {
	el := strings.Split(strings.Trim(p, "/"), "/")
	switch {
	case el[0] == "media":
		el = el[1:]
	case len(el) > 1 && el[0] == "run" && el[1] == "media":
		el = el[2:]
	case el[0] == "Volumes" || el[0] == "mnt":
		return false, len(el) == 2
	default:
		return false, false
	}
	return len(el) == 1, len(el) == 2
}

// base — строка и путь без диска: размер, символы, локальность, очистка и правило 149. rootOK —
// корень несистемного тома Windows допустим (явное место).
func (e *placeEnv) base(raw string, rootOK bool) (string, error) {
	r := e.rules
	if len(raw) > agentPathMax || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", errPlaceBad
	}
	p := r.clean(raw)
	if rootOK && r.windows && len(p) == 3 && r.fold(p[:2]) != r.fold(e.system) {
		return p, r.judgeBelowRoot(p)
	}
	return p, r.judge(p)
}

// static — строковые правила места для способа how (шаг без диска, годится при каждом чтении).
func (e *placeEnv) static(dir, how string) error {
	if e == nil || e.rules == nil {
		return errPlaceBad
	}
	var err error
	switch how {
	case prefs.PlaceChosen:
		err = e.staticChosen(dir)
	case prefs.PlaceParent:
		err = e.staticParent(dir)
	default:
		err = errPlaceBad
	}
	if err != nil {
		return errPlaceBad // причина правил 149 тут не нужна: окно скажет одно
	}
	return nil
}

// staticChosen — явный выбор («Папка со съёмками…»): правило 149; корень несистемного тома,
// ~/Pictures и ~/Videos можно, сам /media/<u> — нет (все съёмные носители разом).
func (e *placeEnv) staticChosen(dir string) error {
	p, err := e.base(dir, true)
	if err != nil {
		return err
	}
	if !e.rules.windows {
		if userRoot, _ := unixMountShape(p); userRoot {
			return errPlaceBad
		}
	}
	return nil
}

// staticParent — место из родителя добавленной папки (строгие правила, аудит В2).
func (e *placeEnv) staticParent(dir string) error {
	p, err := e.base(dir, false)
	if err != nil {
		return err
	}
	r := e.rules
	par := r.parent(p)
	for _, h := range r.homes {
		if r.key(par) == r.key(h) {
			return errPlaceBad // прямая подпапка домашней: ~/Pictures, ~/OneDrive, ~/Работа
		}
	}
	for _, w := range r.whole {
		if r.inside(p, w) {
			return errPlaceBad // «Документы», «Рабочий стол», «Загрузки» на любой глубине
		}
	}
	for _, c := range e.cloud {
		if r.key(p) == r.key(c) || r.key(par) == r.key(c) {
			return errPlaceBad // корень облачной папки и её прямая подпапка
		}
	}
	if !r.windows {
		if userRoot, volume := unixMountShape(p); userRoot || volume {
			return errPlaceBad
		}
	}
	return nil
}

// admit — полная проверка при пополнении и засеве: строка, затем диск одной пробой под within —
// настоящий путь после ссылок (строка ещё раз), ни одного звена-ссылки (junction Go не раскрывает),
// для «parent» ещё не точка монтирования и не предок точки монтирования. Возвращает настоящий путь:
// место хранится им, чтобы «разрешается в себя же» (design §4.1 п. 4) держалось.
func (e *placeEnv) admit(dir, how string) (string, error) {
	if err := e.static(dir, how); err != nil {
		return "", err
	}
	type probe struct {
		resolved string
		ok       bool
	}
	r := e.rules
	// Подмены читаем здесь, а не в горутине: горутина, пережившая предел, не должна видеть
	// глобал уже после того, как тест вернул его (гонка с cleanup).
	dev, mounts := placeDev, placeMounts
	res, ok := within(r.timeout, func() probe {
		resolved, err := r.disk(dir)
		if err != nil || resolved == "" || e.static(resolved, how) != nil || hasLinkPart(resolved) {
			return probe{}
		}
		return probe{resolved, how != prefs.PlaceParent || e.notMount(resolved, dev, mounts)}
	})
	if !ok || !res.ok {
		return "", errPlaceBad
	}
	return res.resolved, nil
}

// notMount — папка не корень тома (st_dev отличается от родителя или она в таблице монтирования)
// и не накрывает точку монтирования. Не смогли узнать — нельзя.
func (e *placeEnv) notMount(resolved string, devFn func(string) (uint64, bool), mountsFn func() ([]string, error)) bool {
	r := e.rules
	dev, ok1 := devFn(resolved)
	pdev, ok2 := devFn(r.parent(resolved))
	if !ok1 || !ok2 || dev != pdev {
		return false
	}
	mounts, err := mountsFn()
	if err != nil {
		return false
	}
	for _, m := range mounts {
		if r.inside(m, resolved) { // сама точка или внутри: том смонтирован в этом месте
			return false
		}
	}
	return true
}

// hasLinkPart — среди элементов пути ниже корня тома есть ссылка или точка повторной обработки
// (junction, облачная папка): сканер из корня-ссылки ничего не берёт, а подмена ссылки между
// пробой и применением — угроза У40. Не прочитали звено — считаем ссылкой.
func hasLinkPart(p string) bool {
	vol := filepath.VolumeName(p)
	cur := vol + string(filepath.Separator)
	rest := strings.TrimLeft(p[len(vol):], `/\`)
	for _, el := range strings.FieldsFunc(rest, func(c rune) bool { return c == '/' || c == filepath.Separator }) {
		cur = filepath.Join(cur, el)
		st, err := os.Lstat(cur) //nolint:gosec // G703: только метаданные звена; путь — resolved из folderRules.resolve (CheckLocalFor, очистка, настоящий путь после ссылок), ничего не открывается и не пишется
		if err != nil || st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return true
		}
	}
	return false
}
