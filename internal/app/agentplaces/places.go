package agentplaces

// Правила мест съёмок (150, design §4.2): где «Всегда» у папки берёт папку без вопроса. Строковые
// правила — при каждом чтении мест (дёшево, диск не трогают); дисковые (ссылки, st_dev, mountinfo)
// — только при пополнении и засеве, под Within и одной пробой. prefs.clean() проверяет лишь
// строение (0.34-150-30): правила 149 живут здесь, prefs их не импортирует. Агент мест не видит
// и не меняет; источники — internal/app/agentplaces_app.go.

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// ErrPlaceBad — папка не годится как место съёмок (причина не уточняется: путь в ошибку не идёт).
var ErrPlaceBad = errors.New("эту папку нельзя сделать местом съёмок: слишком широкая, системная или недоступная")

// Env — правила 149 и то, что нужно местам сверх них: корни облачных папок и системный диск. Подмены
// диска (Vol, VolPath, Dev, Mounts) — nil означает настоящие; тесты ставят свои, App берёт свои
// переменные до Within.
type Env struct {
	Rules  *Rules
	Cloud  []string // корни облачных папок: сами и прямые подпапки не место из родителя
	System string   // системный диск Windows («C:»); его корень — не место даже явное
	// Vol и VolPath — номер тома пути и путь на томе по номеру (153); nil — osx.VolumeOf / osx.VolumePath.
	Vol     func(string) (osx.Vol, error)
	VolPath func(id, rel string) (string, string, error)
	Dev     func(string) (uint64, bool) // st_dev папки; nil — DevOf
	Mounts  func() ([]string, error)    // таблица монтирования; nil — osx.MountPoints
}

// ok — среда годна: правила собраны.
func (e *Env) ok() bool { return e != nil && e.Rules.ok() }

// CloudRoots — корни облачных папок ОС goos (Windows — из окружения, macOS — iCloud Drive и
// CloudStorage). Linux-клиенты лежат прямо под домашней, их закрывает правило «не подпапка домашней».
func CloudRoots(goos, home string, env func(string) string) []string {
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

// Parent — родитель очищенного пути (корень остаётся корнем).
func (r *Rules) Parent(p string) string {
	i := strings.LastIndex(p, r.Sep())
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
func (e *Env) base(raw string, rootOK bool) (string, error) {
	r := e.Rules
	if len(raw) > agentPathMax || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", ErrPlaceBad
	}
	p := r.Clean(raw)
	if rootOK && r.windows && len(p) == 3 && r.fold(p[:2]) != r.fold(e.System) {
		return p, r.judgeBelowRoot(p)
	}
	return p, r.judge(p)
}

// Static — строковые правила места для способа how (шаг без диска, годится при каждом чтении).
func (e *Env) Static(dir, how string) error {
	if !e.ok() {
		return ErrPlaceBad
	}
	var err error
	switch how {
	case prefs.PlaceChosen:
		err = e.staticChosen(dir)
	case prefs.PlaceParent:
		err = e.staticParent(dir)
	default:
		err = ErrPlaceBad
	}
	if err != nil {
		return ErrPlaceBad // причина правил 149 тут не нужна: окно скажет одно
	}
	return nil
}

// staticChosen — явный выбор («Папка со съёмками…»): правило 149; корень несистемного тома,
// ~/Pictures и ~/Videos можно, сам /media/<u> — нет (все съёмные носители разом).
func (e *Env) staticChosen(dir string) error {
	p, err := e.base(dir, true)
	if err != nil {
		return err
	}
	if !e.Rules.windows {
		if userRoot, _ := unixMountShape(p); userRoot {
			return ErrPlaceBad
		}
	}
	return nil
}

// staticParent — место из родителя добавленной папки (строгие правила, аудит В2).
func (e *Env) staticParent(dir string) error {
	p, err := e.base(dir, false)
	if err != nil {
		return err
	}
	r := e.Rules
	par := r.Parent(p)
	for _, h := range r.homes {
		if r.Key(par) == r.Key(h) {
			return ErrPlaceBad // прямая подпапка домашней: ~/Pictures, ~/OneDrive, ~/Работа
		}
	}
	for _, w := range r.whole {
		if r.Inside(p, w) {
			return ErrPlaceBad // «Документы», «Рабочий стол», «Загрузки» на любой глубине
		}
	}
	for _, c := range e.Cloud {
		if r.Key(p) == r.Key(c) || r.Key(par) == r.Key(c) {
			return ErrPlaceBad // корень облачной папки и её прямая подпапка
		}
	}
	if !r.windows {
		if userRoot, volume := unixMountShape(p); userRoot || volume {
			return ErrPlaceBad
		}
	}
	return nil
}

// Admit — полная проверка при пополнении и засеве: строка, затем диск одной пробой под Within —
// настоящий путь после ссылок (строка ещё раз), ни одного звена-ссылки (junction Go не раскрывает),
// для «parent» ещё не точка монтирования и не предок точки монтирования. Возвращает настоящий путь:
// место хранится им, чтобы «разрешается в себя же» (design §4.1 п. 4) держалось.
func (e *Env) Admit(dir, how string) (string, error) {
	if err := e.Static(dir, how); err != nil {
		return "", err
	}
	type probe struct {
		resolved string
		ok       bool
	}
	r := e.Rules
	// Подмены читаем здесь, а не в горутине: горутина, пережившая предел, не должна видеть
	// подмену уже после того, как тест вернул её (гонка с cleanup).
	dev, mounts := e.devFn(), e.mountsFn()
	res, ok := appkit.Within(r.timeout, func() probe {
		resolved, err := r.disk(dir)
		if err != nil || resolved == "" || e.Static(resolved, how) != nil || HasLinkPart(resolved) {
			return probe{}
		}
		return probe{resolved, how != prefs.PlaceParent || e.notMount(resolved, dev, mounts)}
	})
	if !ok || !res.ok {
		return "", ErrPlaceBad
	}
	return res.resolved, nil
}

// devFn — чтение st_dev: подмена Env.Dev или настоящее.
func (e *Env) devFn() func(string) (uint64, bool) {
	if e.Dev != nil {
		return e.Dev
	}
	return DevOf
}

// mountsFn — таблица монтирования: подмена Env.Mounts или настоящая.
func (e *Env) mountsFn() func() ([]string, error) {
	if e.Mounts != nil {
		return e.Mounts
	}
	return osx.MountPoints
}

// notMount — папка не корень тома (st_dev отличается от родителя или она в таблице монтирования)
// и не накрывает точку монтирования. Не смогли узнать — нельзя.
func (e *Env) notMount(resolved string, devFn func(string) (uint64, bool), mountsFn func() ([]string, error)) bool {
	r := e.Rules
	dev, ok1 := devFn(resolved)
	pdev, ok2 := devFn(r.Parent(resolved))
	if !ok1 || !ok2 || dev != pdev {
		return false
	}
	mounts, err := mountsFn()
	if err != nil {
		return false
	}
	for _, m := range mounts {
		if r.Inside(m, resolved) { // сама точка или внутри: том смонтирован в этом месте
			return false
		}
	}
	return true
}

// HasLinkPart — среди элементов пути ниже корня тома есть ссылка или точка повторной обработки
// (junction, облачная папка): сканер из корня-ссылки ничего не берёт, а подмена ссылки между
// пробой и применением — угроза У40. Не прочитали звено — считаем ссылкой. p — настоящий путь из
// Rules.Resolve (после ссылок), а не введённая строка.
func HasLinkPart(p string) bool {
	vol := filepath.VolumeName(p)
	cur := vol + string(filepath.Separator)
	rest := strings.TrimLeft(p[len(vol):], `/\`)
	for _, el := range strings.FieldsFunc(rest, func(c rune) bool { return c == '/' || c == filepath.Separator }) {
		cur = filepath.Join(cur, el)
		st, err := os.Lstat(cur) //nolint:gosec // G703: только метаданные звена; путь — resolved из Rules.Resolve (CheckLocalFor, очистка, настоящий путь после ссылок), ничего не открывается и не пишется
		if err != nil || st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			return true
		}
	}
	return false
}
