// Package agentplaces — правила папок и мест съёмок агента (150, 153, 155): чистая библиотека без
// Host, событий и состояния. Строки и диск (только чтение; запись и запуск закрыты forbidigo 143/149):
// правила 149 для папки съёмки и выбранной папки, места из родителя, подъём до корня архива, привязка
// к диску. Продукт берёт правила только через Default; New собирает мягкие правила для тестов
// (forbidigo 155).
package agentplaces

// Папка от агента и папка, которую выбрал человек (149, design §2.3): одно правило для подсказки
// окну ОС и для выбранного. Агент папку не получает вовсе — только предлагает, где открыть окно;
// Шаг 1 — строка без диска (Static), шаг 2 — диск в своей горутине с пределом (disk), затем
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

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

var (
	// ErrTooWide — папка не подходит: широкая, системная, служебная, недоступная или не папка.
	ErrTooWide = errors.New("папка не подходит: слишком широкая, системная или недоступная")
	// ErrOurs — папка данных или кэша самой программы.
	ErrOurs = errors.New("папка не подходит: это папка данных или кэша программы")
)

const (
	agentPathMax = 4096 // байт в подсказке
	hintShownMax = 60   // рун в показанном имени папки
)

// Rules — правила папки и всё, что от среды (ОС, корни, срок, диск). Поля закрыты: продукт берёт
// Default, тесты — New. Нулевое значение отказывает (built не поставлен), а не разрешает всё.
type Rules struct {
	built   bool // собрано newRules: без признака Rules{} и new(Rules) отказывают
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

// ok — правила собраны (не nil и не нулевое значение).
func (r *Rules) ok() bool { return r != nil && r.built }

// refusal — причина отказа несобранных правил: nil — прежняя ошибка (fail-closed у Default), нулевое
// значение — ErrPlaceBad.
func (r *Rules) refusal() error {
	if r == nil {
		return ErrTooWide
	}
	return ErrPlaceBad
}

// Timeout — предел одной дисковой пробы.
func (r *Rules) Timeout() time.Duration { return r.timeout }

// Disk — проба диска (шаг 2): настоящий путь после ссылок.
func (r *Rules) Disk() func(string) (string, error) { return r.disk }

// Hint — подсказка агента годна для окна ОС: dir — очищенный путь. Не годна — окно открывается в
// обычной папке, причина агенту не сообщается. Правила те же, что у выбранной папки (design §2.3).
func (r *Rules) Hint(s string) (string, bool) {
	if !r.ok() || r.Folder(s) != nil {
		return "", false
	}
	return r.Clean(s), true
}

// Folder — папка годится как папка съёмки: строка, диск под пределом, строка ещё раз для настоящего пути.
func (r *Rules) Folder(raw string) error {
	if !r.ok() {
		return r.refusal()
	}
	p, err := r.Static(raw)
	if err != nil {
		return err
	}
	type probe struct {
		resolved string
		err      error
	}
	disk := r.disk // до Within: зависшая проба доживает в своей горутине, поле читать позже нельзя
	res, ok := appkit.Within(r.timeout, func() probe {
		resolved, err := disk(p)
		return probe{resolved, err}
	})
	if !ok || res.err != nil || res.resolved == "" {
		return ErrTooWide
	}
	// Ссылка могла вести в корень или в чужое: /Volumes/Macintosh HD → /, Wine z: → /.
	_, err = r.Static(res.resolved)
	return err
}

// Static — шаг 1, без диска: строка, затем путь. Возвращает очищенный путь.
func (r *Rules) Static(raw string) (string, error) {
	if !r.ok() {
		return "", r.refusal()
	}
	if len(raw) > agentPathMax || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", ErrTooWide
	}
	p := r.Clean(raw)
	return p, r.judge(p)
}

// Clean — очистка по правилам ОС правил, а не ОС процесса: Windows-путь чистится и на Linux, путь
// Unix — и на Windows (filepath.Clean там заменил бы «/» на «\»; тесты гоняют обе таблицы везде).
func (r *Rules) Clean(p string) string {
	if !r.windows {
		return path.Clean(p)
	}
	// CheckLocalFor уже гарантировал «X:» и разделитель следом; ".." не уходит выше корня.
	rest := path.Clean(strings.ReplaceAll(p[2:], `\`, "/"))
	return p[:2] + strings.ReplaceAll(rest, "/", `\`)
}

// Sep — разделитель путей по правилам.
func (r *Rules) Sep() string {
	if r.windows {
		return `\`
	}
	return "/"
}

// Key — ключ папки: свёрнутый регистр, разделитель в конце, чтобы «/a/b» не накрывала «/a/bc».
func (r *Rules) Key(p string) string {
	k := r.fold(r.Clean(p))
	if !strings.HasSuffix(k, r.Sep()) {
		k += r.Sep()
	}
	return k
}

// Inside — p совпадает с root или лежит внутри.
func (r *Rules) Inside(p, root string) bool {
	return root != "" && strings.HasPrefix(r.Key(p), r.Key(root))
}

// judge — p очищен и локален: подходит ли как папка съёмки.
func (r *Rules) judge(p string) error {
	if r.windows && len(p) == 3 || !r.windows && p == "/" {
		return ErrTooWide // корень тома
	}
	return r.judgeBelowRoot(p)
}

// judgeBelowRoot — judge без проверки «корень тома»: явное место съёмок (places.go) может быть
// корнем несистемного тома, остальное правило то же.
func (r *Rules) judgeBelowRoot(p string) error {
	for _, o := range r.ours {
		if r.Inside(p, o) {
			return ErrOurs
		}
		if r.Inside(o, p) {
			return ErrTooWide // накрывает нашу папку
		}
	}
	// Домашняя и предки, три целые папки пользователя и предки их.
	for _, w := range append(slices.Clone(r.homes), r.whole...) {
		if r.Inside(w, p) {
			return ErrTooWide
		}
	}
	for _, e := range r.exact {
		if r.Key(p) == r.Key(e) {
			return ErrTooWide
		}
	}
	if r.systemTree(p) || r.badElem(p) {
		return ErrTooWide
	}
	return nil
}

func (r *Rules) systemTree(p string) bool {
	for _, a := range r.allow {
		if r.Inside(p, a) {
			return false
		}
	}
	for _, t := range r.trees {
		if r.Inside(p, t) {
			return true
		}
	}
	return false
}

// badElem — служебный элемент: точка в начале на Unix, «System Volume Information» и корзина на Windows.
func (r *Rules) badElem(p string) bool {
	for _, el := range strings.Split(p, r.Sep()) {
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
// возвращает настоящий путь. Висящий NFS и SMB держит предел Within.
func diskProbe(p string) (string, error) {
	if !osx.Reachable(p) {
		return "", ErrTooWide
	}
	if _, err := os.Lstat(p); err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		// Lstat нашёл звено, а раскрыть его нельзя (висячая ссылка): это не «папки нет — создать».
		return "", ErrTooWide
	}
	// subst-диск и отображённая папка — не ссылки: настоящий путь даёт только ОС (аудит 0.34, М4).
	if resolved, err = osx.RealPath(resolved); err != nil {
		return "", err
	}
	if st, err := library.StatLocal(resolved); err != nil || !st.IsDir() {
		return "", ErrTooWide
	}
	return resolved, nil
}

// HintLabel — что показать в вопросе: только последний элемент подсказки, не больше hintShownMax
// рун (лишнее — «…»). Остальной путь человек видит в окне ОС. dir — результат Hint.
func HintLabel(dir string) string {
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

// HintShown — подсказка агента для вопроса по правилам r (теми же, что у выбранной папки):
// dir для окна ОС и label для AgentAsk.Hint. Не годная — оба пусты.
func HintShown(r *Rules, s string) (dir, label string) {
	dir, ok := r.Hint(s)
	if !ok {
		return "", ""
	}
	return dir, HintLabel(dir)
}

// JudgeWrite — правило места записи выгрузки по подсказке агента (152, design §3.3) для строки без диска:
// абсолютный локальный путь (UNC — нет), не системное дерево, не папка программы, не скрытый и служебный
// элемент (имя с точки — на всех ОС, `.ssh` на Windows тоже), не корень системного тома и не домашняя целиком.
// Можно подпапки домашней, «Документы», «Рабочий стол», «Загрузки» (и сами они) и корень несистемного тома
// (флешка `E:\`). Возвращает очищенный путь.
func (r *Rules) JudgeWrite(raw string) (string, error) {
	if !r.ok() {
		return "", r.refusal()
	}
	if len(raw) > agentPathMax || !utf8.ValidString(raw) || paths.BadRunes(raw) || paths.CheckLocalFor(r.windows, raw) != nil {
		return "", ErrTooWide
	}
	p := r.Clean(raw)
	if r.windows {
		sys := r.system
		if sys == "" {
			sys = "C:"
		}
		if len(p) == 3 && r.fold(p[:2]) == r.fold(sys) {
			return "", ErrTooWide // корень системного тома
		}
	} else if p == "/" {
		return "", ErrTooWide
	}
	for _, o := range r.ours {
		if r.Inside(p, o) {
			return "", ErrOurs
		}
		if r.Inside(o, p) {
			return "", ErrTooWide
		}
	}
	for _, h := range r.homes { // домашняя целиком и её предки: выгрузка россыпью в ~
		if r.Inside(h, p) {
			return "", ErrTooWide
		}
	}
	for _, w := range r.whole { // «Документы», «Рабочий стол», «Загрузки» можно, их предков — нет
		if r.Inside(w, p) && r.Key(w) != r.Key(p) {
			return "", ErrTooWide
		}
	}
	for _, e := range r.exact {
		if r.Key(p) == r.Key(e) {
			return "", ErrTooWide
		}
	}
	if r.systemTree(p) || r.hiddenWrite(p) {
		return "", ErrTooWide
	}
	return p, nil
}

// hiddenWrite — скрытый или служебный элемент пути: имя с точки на любой ОС (на Windows тоже, `.ssh`), а также
// служебные элементы ОС (badElem).
func (r *Rules) hiddenWrite(p string) bool {
	for el := range strings.SplitSeq(p, r.Sep()) {
		if strings.HasPrefix(el, ".") {
			return true
		}
	}
	return r.badElem(p)
}

// WriteDir — место записи выгрузки на диске: настоящий путь после ссылок и подмены тома. create — последнего
// элемента ещё нет (родитель есть и годен: глубже одного уровня не создаём). Путь после ссылок судится заново.
// Непрозрачный: проверенный путь не собрать литералом, читать — Path и Create; нулевое значение — «ничего».
type WriteDir struct {
	path   string
	create bool
}

// Path — настоящий путь места; "" у нулевого значения.
func (w WriteDir) Path() string { return w.path }

// Create — последней папки ещё нет: её нужно создать.
func (w WriteDir) Create() bool { return w.create }

// WriteDir проверяет место записи выгрузки по подсказке: строка, диск под пределом, том, строка ещё раз.
func (r *Rules) WriteDir(raw string) (WriteDir, error) {
	cleaned, err := r.JudgeWrite(raw)
	if err != nil {
		return WriteDir{}, err
	}
	type probe struct {
		w   WriteDir
		err error
	}
	disk := r.disk // до Within: зависшая проба доживает в своей горутине
	res, ok := appkit.Within(r.timeout, func() probe {
		p, err := disk(cleaned)
		if err == nil {
			return probe{w: WriteDir{path: p}}
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return probe{err: ErrTooWide}
		}
		par, err := disk(r.Parent(cleaned)) // нет последней папки: родитель есть, имя придёт к настоящему родителю
		if err != nil {
			return probe{err: ErrTooWide}
		}
		name := cleaned[strings.LastIndex(cleaned, r.Sep())+1:]
		if paths.FolderName(name, "") != name { // имя новой папки из подсказки-пути годно не любое: «Петровы.», «a<b»
			return probe{err: ErrTooWide}
		}
		return probe{w: WriteDir{path: strings.TrimRight(par, r.Sep()) + r.Sep() + name, create: true}}
	})
	if !ok || res.err != nil || res.w.path == "" {
		return WriteDir{}, ErrTooWide
	}
	if r.VolumeMoved(cleaned, res.w.path) { // subst, сетевая буква, ссылка на другой том
		return WriteDir{}, ErrTooWide
	}
	if _, err := r.JudgeWrite(res.w.path); err != nil {
		return WriteDir{}, err
	}
	return res.w, nil
}

// ExportPick — повтор правила при щелчке: место то же, что показано в вопросе (ключ настоящего пути). Папку,
// которой не было, успели создать обычной папкой — это существующая; подмена (ссылка, файл, другой том) — отказ.
func (r *Rules) ExportPick(shown string) (WriteDir, error) {
	w, err := r.WriteDir(shown)
	if err != nil {
		return WriteDir{}, err
	}
	if r.Key(w.path) != r.Key(shown) {
		return WriteDir{}, ErrTooWide
	}
	return w, nil
}
