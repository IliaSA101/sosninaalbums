package mcpbridge

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// start_program (спека 149, design §5, решение 0.33-149-22): заглушка умеет запустить программу.
// Только эта логика решает, можно ли; сам запуск — launch_*.go (линтер пускает exec только туда).
// Тексты — как у остальных: без путей и без текста системных ошибок.
const (
	startTool = "start_program"

	startToolDesc = "Запустить Sosnina Albums, если она закрыта: тот же exe без аргументов, откроется последний альбом. " +
		"Работает только при включённом доступе агентам (Файл ▸ ИИ-агенты); альбом не меняется."

	textStarting     = "Программа запускается; инструменты появятся через несколько секунд."
	textStartAlready = "Программа уже запущена; инструменты появятся через несколько секунд."
	textStartOwn     = "Запустите программу сами: "

	reasonElevated  = "агент запущен с повышенными правами"
	reasonPackaged  = "агент работает в пакете MSIX"
	reasonSandbox   = "агент работает в песочнице (Flatpak или Snap)"
	reasonNoDisplay = "в сеансе агента нет экрана"
	reasonAlive     = "программа уже запущена"
	reasonRecent    = "программа запускалась меньше минуты назад"
	reasonBadExe    = "файл программы не годится для запуска"
	reasonNoMark    = "не удалось записать отметку запуска"
	reasonBreakaway = "система не даёт отделить программу от агента"
	reasonFailed    = "программа не запустилась"
)

// Пределы и сроки start_program.
const (
	maxConfigRead = 64 << 10         // config.json читается не больше: настоящий — килобайты
	startEvery    = 60 * time.Second // не чаще раза в минуту на все посредники (отметка в кэше)
)

// startDeps — всё, что start_program берёт у ОС; тесты подменяют. Нулевое значение (spawn == nil) —
// запуск выключен: инструмента в списке нет.
type startDeps struct {
	goos        string
	dataName    string // папка данных: у стенда своя
	instanceKey string // UniqueId замка экземпляра (internal/app), для проверки «уже запущена»

	environ  func() []string
	cfgDir   func() string // osx.ConfigDir
	cacheDir func() string // osx.CacheDir
	tempDir  func() string
	exe      func() (string, error) // путь своей программы, ссылки раскрыты
	regular  func(path string) bool
	exists   func(path string) bool
	elevated func() bool
	packaged func() bool // Windows: пакетная идентичность MSIX
	instance func() bool // замок экземпляра программы занят
	spawn    func(exe string, env []string) error
	now      func() time.Time
}

func (d *startDeps) enabled() bool { return d.spawn != nil }

// defaultStart — настоящие зависимости посредника. dataName — DataName стенда; instanceKey —
// UniqueId замка экземпляра программы.
func defaultStart(dataName, instanceKey string) startDeps {
	return startDeps{
		goos: runtime.GOOS, dataName: dataName, instanceKey: instanceKey,
		environ: os.Environ, cfgDir: osx.ConfigDir, cacheDir: osx.CacheDir, tempDir: os.TempDir,
		exe: ownExe, regular: isRegular, exists: pathExists,
		elevated: osx.Elevated, packaged: platformPackaged,
		instance: func() bool { return platformInstanceAlive(instanceKey) },
		spawn:    spawnProgram, now: time.Now,
	}
}

// ownExe — своя программа: путь из ОС с раскрытыми ссылками (запуск не должен идти по подмене).
func ownExe() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func isRegular(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode().IsRegular()
}

func pathExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// accessOn — в config.json agents.on == true и start_program не запрещён (agentPerms.tools, 150).
// Файл читается здесь, только чтение: не prefs (она писала бы и тянет всю программу). Путь локальный,
// размер проверяется до чтения, файл не обычный, битый, усечённый, пустой или больше предела —
// «выключено». «Запретить» у start_program посредник не показывает вовсе (0.34-150-2): программа
// закрыта, ответить отказом словами некому; незнакомое значение — тоже запрет, как в prefs.clean().
func (d *startDeps) accessOn() bool {
	dir := d.cfgDir()
	if dir == "" || d.dataName == "" {
		return false
	}
	path := filepath.Join(dir, d.dataName, "config.json")
	if !filepath.IsAbs(path) || paths.CheckLocalPath(path) != nil {
		return false
	}
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxConfigRead {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, maxConfigRead+1)) // файл мог вырасти после Lstat
	if err != nil || len(b) > maxConfigRead {
		return false
	}
	var c struct {
		Agents *struct {
			On bool `json:"on"`
		} `json:"agents"`
		AgentPerms *struct {
			Tools map[string]any `json:"tools"`
		} `json:"agentPerms"`
	}
	if json.Unmarshal(b, &c) != nil || c.Agents == nil || !c.Agents.On {
		return false
	}
	if c.AgentPerms != nil {
		if v, ok := c.AgentPerms.Tools[startTool]; ok && v != "allow" {
			return false
		}
	}
	return true
}

// startOffered — инструмент start_program есть в списке: запуск включён, программа не отвечает по
// причине «закрыта» (не «небезопасно», не «много подключений») и доступ агентам включён.
func (b *bridge) startOffered() bool {
	d := &b.cfg.start
	return d.enabled() && b.why == textClosed && d.accessOn()
}

// startToolSpec — описание инструмента для tools/list. Аннотации явно (аудит М6): запуск процесса —
// не чтение, не удаление, не повторяется безвредно, во внешний мир не ходит.
func startToolSpec() map[string]any {
	return map[string]any{
		"name":        startTool,
		"title":       "Запустить Sosnina Albums",
		"description": startToolDesc,
		"inputSchema": map[string]any{"type": "object", "additionalProperties": false},
		"annotations": map[string]any{
			"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": false,
		},
	}
}

// startProgram — вызов start_program: текст ответа и признак успеха. Аргументы вызова не
// читаются вовсе: процессу они не уходят ни в каком виде.
func (b *bridge) startProgram() (text string, ok bool) {
	d := &b.cfg.start
	if !d.enabled() || !d.accessOn() { // как при вызове неизвестного инструмента
		return b.why, false
	}
	b.maybeProbe(true) // программа могла запуститься сама, пока клиент собирался звать
	if b.live() {
		return textStartAlready, true
	}
	if b.why != textClosed {
		return b.why, false // небезопасно, чужой слушатель, много подключений: запуском не лечится
	}
	exe, why := d.checkStart()
	if why != "" {
		return textStartOwn + why, false
	}
	why = d.launch(exe)
	if why != "" {
		return textStartOwn + why, false
	}
	b.last = time.Time{} // следующая проба — сразу: программа вот-вот откроет сокет
	return textStarting, true
}

// checkStart — все отказы, кроме самого запуска. Порядок: от дешёвых к дорогим; отметка времени
// ставится последней, чтобы отказ по другой причине не расходовал минуту.
func (d *startDeps) checkStart() (exe, why string) {
	switch {
	case d.elevated():
		return "", reasonElevated
	case d.packaged():
		return "", reasonPackaged
	case d.sandboxed():
		return "", reasonSandbox
	case d.goos == "linux" && d.noDisplay():
		return "", reasonNoDisplay
	case d.instance():
		return "", reasonAlive
	}
	exe, err := d.exe()
	if err != nil || osx.ExeHint(exe, d.goos, d.tempDir(), d.regular) != "" {
		return "", reasonBadExe
	}
	md := d.markDir()
	if md == "" {
		return "", reasonNoMark
	}
	claimed, err := osx.ClaimStart(md, d.now(), startEvery)
	if err != nil {
		return "", reasonNoMark
	}
	if !claimed {
		return "", reasonRecent
	}
	return exe, ""
}

// launch запускает программу с окружением из белого списка; причина отказа или "".
func (d *startDeps) launch(exe string) string {
	err := d.spawn(exe, childEnv(d.environ(), d.goos))
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errBreakaway):
		return reasonBreakaway
	}
	return reasonFailed
}

// markDir — папка отметки запуска: кэш программы (у стенда свой).
func (d *startDeps) markDir() string {
	c := d.cacheDir()
	if c == "" || d.dataName == "" {
		return ""
	}
	return filepath.Join(c, d.dataName, "mcp")
}
