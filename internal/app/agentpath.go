package app

// Папка от агента и папка, которую выбрал человек (149, design §2.3): одно правило для подсказки
// окну ОС и для выбранного. Агент папку не получает вовсе — только предлагает, где открыть окно;
// Шаг 1 — строка без диска (static), шаг 2 — диск в своей горутине с пределом (disk), затем
// шаг 1 ещё раз для настоящего пути после ссылок. Пути в ошибки и лог не попадают.

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

var (
	// errFolderTooWide — папка не подходит: широкая, системная, служебная, недоступная или не папка.
	errFolderTooWide = errors.New("папка не подходит: слишком широкая, системная или недоступная")
	// errFolderOurs — папка данных или кэша самой программы.
	errFolderOurs = errors.New("папка не подходит: это папка данных или кэша программы")
)

const (
	agentPathMax = 4096 // байт в подсказке
	hintShownMax = 60   // рун в показанном имени папки
)

// folderRules — правила папки и всё, что от среды: тесты задают свои (ОС, корни, срок, диск).
type folderRules struct {
	windows bool
	fold    func(string) string // ключ сравнения: регистр Windows и macOS не важен
	homes   []string            // домашняя папка и её настоящий путь
	whole   []string            // «Документы», «Рабочий стол», «Загрузки»: целиком нельзя, подпапки можно
	ours    []string            // папки данных и кэша
	trees   []string            // системные деревья: сама папка и всё внутри
	allow   []string            // исключения из trees (/run/media)
	exact   []string            // широкие папки: только сама (/media, /Volumes, C:\Users)
	elems   []string            // служебные элементы пути в любом месте (в нижнем регистре)
	system  string              // системный том Windows («C:»); пусто — «C:»
	timeout time.Duration
	disk    func(p string) (resolved string, err error)
}

// hint — подсказка агента годна для окна ОС: dir — очищенный путь. Не годна — окно открывается в
// обычной папке, причина агенту не сообщается. Правила те же, что у выбранной папки (design §2.3).
func (r *folderRules) hint(s string) (string, bool) {
	if r == nil || r.folder(s) != nil {
		return "", false
	}
	return r.clean(s), true
}

func (r *folderRules) folder(raw string) error {
	if r == nil {
		return errFolderTooWide
	}
	p, err := r.static(raw)
	if err != nil {
		return err
	}
	type probe struct {
		resolved string
		err      error
	}
	disk := r.disk // до within: зависшая проба доживает в своей горутине, поле читать позже нельзя
	res, ok := within(r.timeout, func() probe {
		resolved, err := disk(p)
		return probe{resolved, err}
	})
	if !ok || res.err != nil || res.resolved == "" {
		return errFolderTooWide
	}
	// Ссылка могла вести в корень или в чужое: /Volumes/Macintosh HD → /, Wine z: → /.
	_, err = r.static(res.resolved)
	return err
}

// static — шаг 1, без диска: строка, затем путь. Возвращает очищенный путь.
func (r *folderRules) static(raw string) (string, error) {
	if len(raw) > agentPathMax || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", errFolderTooWide
	}
	p := r.clean(raw)
	return p, r.judge(p)
}

// clean — очистка по правилам ОС правил, а не ОС процесса: Windows-путь чистится и на Linux, путь
// Unix — и на Windows (filepath.Clean там заменил бы «/» на «\»; тесты гоняют обе таблицы везде).
func (r *folderRules) clean(p string) string {
	if !r.windows {
		return path.Clean(p)
	}
	// CheckLocalFor уже гарантировал «X:» и разделитель следом; ".." не уходит выше корня.
	rest := path.Clean(strings.ReplaceAll(p[2:], `\`, "/"))
	return p[:2] + strings.ReplaceAll(rest, "/", `\`)
}

func (r *folderRules) sep() string {
	if r.windows {
		return `\`
	}
	return "/"
}

// key — ключ папки: свёрнутый регистр, разделитель в конце, чтобы «/a/b» не накрывала «/a/bc».
func (r *folderRules) key(p string) string {
	k := r.fold(r.clean(p))
	if !strings.HasSuffix(k, r.sep()) {
		k += r.sep()
	}
	return k
}

// inside — p совпадает с root или лежит внутри.
func (r *folderRules) inside(p, root string) bool {
	return root != "" && strings.HasPrefix(r.key(p), r.key(root))
}

// judge — p очищен и локален: подходит ли как папка съёмки.
func (r *folderRules) judge(p string) error {
	if r.windows && len(p) == 3 || !r.windows && p == "/" {
		return errFolderTooWide // корень тома
	}
	return r.judgeBelowRoot(p)
}

// judgeBelowRoot — judge без проверки «корень тома»: явное место съёмок (agentplaces.go) может быть
// корнем несистемного тома, остальное правило то же.
func (r *folderRules) judgeBelowRoot(p string) error {
	for _, o := range r.ours {
		if r.inside(p, o) {
			return errFolderOurs
		}
		if r.inside(o, p) {
			return errFolderTooWide // накрывает нашу папку
		}
	}
	// Домашняя и предки, три целые папки пользователя и предки их.
	for _, w := range append(slices.Clone(r.homes), r.whole...) {
		if r.inside(w, p) {
			return errFolderTooWide
		}
	}
	for _, e := range r.exact {
		if r.key(p) == r.key(e) {
			return errFolderTooWide
		}
	}
	if r.systemTree(p) || r.badElem(p) {
		return errFolderTooWide
	}
	return nil
}

func (r *folderRules) systemTree(p string) bool {
	for _, a := range r.allow {
		if r.inside(p, a) {
			return false
		}
	}
	for _, t := range r.trees {
		if r.inside(p, t) {
			return true
		}
	}
	return false
}

// badElem — служебный элемент: точка в начале на Unix, «System Volume Information» и корзина на Windows.
func (r *folderRules) badElem(p string) bool {
	for _, el := range strings.Split(p, r.sep()) {
		if !r.windows && strings.HasPrefix(el, ".") {
			return true
		}
		for _, bad := range r.elems {
			if r.fold(el) == bad {
				return true
			}
		}
	}
	return false
}

// diskProbe — шаг 2 на настоящем диске: том отвечает, путь есть и это каталог (после ссылок);
// возвращает настоящий путь. Висящий NFS и SMB держит предел within.
func diskProbe(p string) (string, error) {
	if !osx.Reachable(p) {
		return "", errFolderTooWide
	}
	if _, err := os.Lstat(p); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", err
	}
	// subst-диск и отображённая папка — не ссылки: настоящий путь даёт только ОС (аудит 0.34, М4).
	if resolved, err = osx.RealPath(resolved); err != nil {
		return "", err
	}
	if st, err := library.StatLocal(resolved); err != nil || !st.IsDir() {
		return "", errFolderTooWide
	}
	return resolved, nil
}

// within — f с пределом d: не уложилась или упала с паникой — ok = false. Зависшая f доживает в
// своей горутине (отменить системный вызов нельзя, как у osx.Volumes).
func within[T any](d time.Duration, f func() T) (v T, ok bool) {
	type result struct {
		v  T
		ok bool
	}
	ch := make(chan result, 1)
	go func() {
		var r result
		defer func() {
			_ = recover() // чужой диск не должен валить программу
			ch <- r
		}()
		r.v = f()
		r.ok = true
	}()
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case r := <-ch:
		return r.v, r.ok
	case <-t.C:
		return v, false
	}
}

// hintLabel — что показать в вопросе: только последний элемент подсказки, не больше hintShownMax
// рун (лишнее — «…»). Остальной путь человек видит в окне ОС. dir — результат agentHint.
func hintLabel(dir string) string {
	return lastElem(dir, filepath.Separator == '\\')
}

func lastElem(dir string, windows bool) string {
	i := strings.LastIndexByte(dir, '/')
	if windows {
		i = max(i, strings.LastIndexByte(dir, '\\'))
	}
	el := dir[i+1:]
	if utf8.RuneCountInString(el) <= hintShownMax {
		return el
	}
	return string([]rune(el)[:hintShownMax-1]) + "…"
}

// agentHintShown — подсказка агента для вопроса по правилам r (теми же, что у выбранной папки):
// dir для окна ОС и label для AgentAsk.Hint. Не годная — оба пусты.
func agentHintShown(r *folderRules, s string) (dir, label string) {
	dir, ok := r.hint(s)
	if !ok {
		return "", ""
	}
	return dir, hintLabel(dir)
}

// judgeWrite — правило места записи выгрузки по подсказке агента (152, design §3.3) для строки без диска:
// абсолютный локальный путь (UNC — нет), не системное дерево, не папка программы, не скрытый и служебный
// элемент (имя с точки — на всех ОС, `.ssh` на Windows тоже), не корень системного тома и не домашняя целиком.
// Можно подпапки домашней, «Документы», «Рабочий стол», «Загрузки» (и сами они) и корень несистемного тома
// (флешка `E:\`). Возвращает очищенный путь.
func (r *folderRules) judgeWrite(raw string) (string, error) {
	if r == nil || len(raw) > agentPathMax || !utf8.ValidString(raw) || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", errFolderTooWide
	}
	p := r.clean(raw)
	if r.windows {
		sys := r.system
		if sys == "" {
			sys = "C:"
		}
		if len(p) == 3 && r.fold(p[:2]) == r.fold(sys) {
			return "", errFolderTooWide // корень системного тома
		}
	} else if p == "/" {
		return "", errFolderTooWide
	}
	for _, o := range r.ours {
		if r.inside(p, o) {
			return "", errFolderOurs
		}
		if r.inside(o, p) {
			return "", errFolderTooWide
		}
	}
	for _, h := range r.homes { // домашняя целиком и её предки: выгрузка россыпью в ~
		if r.inside(h, p) {
			return "", errFolderTooWide
		}
	}
	for _, w := range r.whole { // «Документы», «Рабочий стол», «Загрузки» можно, их предков — нет
		if r.inside(w, p) && r.key(w) != r.key(p) {
			return "", errFolderTooWide
		}
	}
	for _, e := range r.exact {
		if r.key(p) == r.key(e) {
			return "", errFolderTooWide
		}
	}
	if r.systemTree(p) || r.hiddenWrite(p) {
		return "", errFolderTooWide
	}
	return p, nil
}

// hiddenWrite — скрытый или служебный элемент пути: имя с точки на любой ОС (на Windows тоже, `.ssh`), а также
// служебные элементы ОС (badElem).
func (r *folderRules) hiddenWrite(p string) bool {
	for el := range strings.SplitSeq(p, r.sep()) {
		if strings.HasPrefix(el, ".") {
			return true
		}
	}
	return r.badElem(p)
}

// writeDir — место записи выгрузки на диске: настоящий путь после ссылок и подмены тома. create — последнего
// элемента ещё нет (родитель есть и годен: глубже одного уровня не создаём). Путь после ссылок судится заново.
type writeDir struct {
	path   string
	create bool
}

func (r *folderRules) writeDir(raw string) (writeDir, error) {
	cleaned, err := r.judgeWrite(raw)
	if err != nil {
		return writeDir{}, err
	}
	type probe struct {
		w   writeDir
		err error
	}
	disk := r.disk // до within: зависшая проба доживает в своей горутине
	res, ok := within(r.timeout, func() probe {
		p, err := disk(cleaned)
		if err == nil {
			return probe{w: writeDir{path: p}}
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return probe{err: errFolderTooWide}
		}
		par, err := disk(r.parent(cleaned)) // нет последней папки: родитель есть, имя придёт к настоящему родителю
		if err != nil {
			return probe{err: errFolderTooWide}
		}
		name := cleaned[strings.LastIndex(cleaned, r.sep())+1:]
		if paths.FolderName(name, "") != name { // имя новой папки из подсказки-пути годно не любое: «Петровы.», «a<b»
			return probe{err: errFolderTooWide}
		}
		return probe{w: writeDir{path: strings.TrimRight(par, r.sep()) + r.sep() + name, create: true}}
	})
	if !ok || res.err != nil || res.w.path == "" {
		return writeDir{}, errFolderTooWide
	}
	if r.volumeMoved(cleaned, res.w.path) { // subst, сетевая буква, ссылка на другой том
		return writeDir{}, errFolderTooWide
	}
	if _, err := r.judgeWrite(res.w.path); err != nil {
		return writeDir{}, err
	}
	return res.w, nil
}

// exportPick — повтор правила при щелчке: место то же, что показано в вопросе (ключ настоящего пути). Папку,
// которой не было, успели создать обычной папкой — это существующая; подмена (ссылка, файл, другой том) — отказ.
func (r *folderRules) exportPick(shown string) (writeDir, error) {
	w, err := r.writeDir(shown)
	if err != nil {
		return writeDir{}, err
	}
	if r.key(w.path) != r.key(shown) {
		return writeDir{}, errFolderTooWide
	}
	return w, nil
}
