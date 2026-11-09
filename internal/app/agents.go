package app

// ИИ-агенты (MCP, спека 0.31/143, Г5): окно «Файл ▸ ИИ-агенты (MCP)…» — включить доступ, уровень,
// тексты настройки клиентов, подключённые и журнал. Тело — internal/app/agents (сокет, инструменты
// чтения); здесь тонкие методы для интерфейса, события и запуск/остановка службы. Служба только
// читает альбом, поэтому правок проекта, rev и шагов отмены здесь нет. Настройки (config.json →
// agents) пишет только этот файл: служба писать их не вправе.

import (
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// AgentSetup — один клиент в «Как подключить» (agents.Setup).
type AgentSetup struct {
	ID    string `json:"id"`    // claude-desktop | claude-code | opencode | other
	Title string `json:"title"` // «Claude Desktop и Cowork»
	Text  string `json:"text"`  // готово к копированию; путь программы — из os.Executable
	Note  string `json:"note"`  // где лежит файл настройки
	File  bool   `json:"file"`  // есть «Сохранить файл для Claude Desktop…»
}

// AgentClient — подключённый сейчас клиент.
type AgentClient struct {
	Name  string `json:"name"`  // clientInfo.name, очищено, ≤ 40 знаков
	Since int64  `json:"since"` // unix мс
}

// AgentCall — вызов агента в журнале; событие agents:call несёт его же и today.
type AgentCall struct {
	At     int64  `json:"at"`     // unix мс
	Client string `json:"client"` // имя клиента
	Tool   string `json:"tool"`   // «Сводка альбома»
	OK     bool   `json:"ok"`
	Ms     int    `json:"ms"`
	Edit   bool   `json:"edit"` // правящий инструмент (149): в журнале ✎
	Note   string `json:"note"` // отказано | без ответа | занят | копия | поздно | ""
}

// AgentCallEvent — событие agents:call: вызов и счётчик «за сегодня».
type AgentCallEvent struct {
	AgentCall
	Today int `json:"today"`
}

// AgentsInfo — всё, что нужно окну и значку; событие agents:state несёт его же.
type AgentsInfo struct {
	Status   string        `json:"status"` // off | on | error
	Hint     string        `json:"hint"`   // "" или предупреждение над «Как подключить»
	Error    string        `json:"error"`  // словами, без путей; бывает только при попытке включить
	Level    int           `json:"level"`  // 1…3
	Portable bool          `json:"portable"`
	Setups   []AgentSetup  `json:"setups"`  // не nil; пусто при мёртвом пути программы
	Clients  []AgentClient `json:"clients"` // не nil
	Calls    []AgentCall   `json:"calls"`   // не nil; новые первыми, ≤ 50
	Today    int           `json:"today"`
	// Правки (149): флажок «Разрешить агентам менять альбом», причина, по которой его нельзя
	// включить, и «вопросы отключены после трёх отказов».
	Edit        bool   `json:"edit"`
	EditBlocked string `json:"editBlocked"` // "" | elevated (программа запущена от администратора)
	AsksMuted   bool   `json:"asksMuted"`
	// Разрешения (150): строки по инструментам и места съёмок. Perms заполняет фасад Т3.
	Perms  []AgentPerm  `json:"perms"`  // не nil
	Places []AgentPlace `json:"places"` // не nil; новые первыми
	// Lifted — места 0.34, поднятые до корня архива при обновлении: заметка до «Понятно» (0.35/153).
	Lifted []AgentPlaceLift `json:"lifted"` // не nil
}

// AgentsSaved — итог SaveClaudeExtension (как TplSave, 086).
type AgentsSaved struct {
	Canceled bool   `json:"canceled"`
	Text     string `json:"text"` // текст тоста
}

// agentsCallGap — не чаще 4 событий agents:call в секунду; лишние склеиваются в одно agents:state.
const agentsCallGap = 250 * time.Millisecond

// agentsPub — частота событий журнала.
type agentsPub struct {
	mu    sync.Mutex
	last  time.Time
	timer *time.Timer
}

// AgentsInfo — состояние для окна и значка при открытии.
func (a *App) AgentsInfo() AgentsInfo {
	defer guard("AgentsInfo")
	return a.agentsInfo()
}

// SetAgents включает или выключает доступ агентов. Включить не вышло — ответ со status error и
// причиной словами, On в config.json не пишется; выключение принимается и в error (оно снимает
// причину и пишет On=false, 0.31-143-42).
func (a *App) SetAgents(on bool) (info AgentsInfo, err error) {
	defer guardErr("SetAgents", &err)
	a.agentsMu.Lock()
	defer a.agentsMu.Unlock()
	if on {
		if a.stopped.Load() { // программа закрывается: сокет уже не открывать
			return a.agentsInfo(), nil
		}
		if a.agents.Start() != nil {
			return a.agentsInfo(), nil
		}
		if a.stopped.Load() { // выход начался, пока открывался сокет: stopAgents видел off и не закрыл
			a.agents.Close()
			return a.agentsInfo(), nil // после выхода App не трогает диск: On=true не пишем
		}
	} else {
		a.agents.Close()
	}
	prefs.UpdateAgents(a.cfgDir, func(c *prefs.Agents) { c.On = on })
	// agents:state для окон уже ушёл из Start/Close (Host.Changed); второй раз не шлём.
	return a.agentsInfo(), nil
}

// SetAgentsLevel ставит уровень доступа: 1 — устройство альбома, 2 — + имена файлов и тексты,
// 3 — + картинки. Вне 1…3 — зажим. Действует на следующий вызов агента.
func (a *App) SetAgentsLevel(level int) AgentsInfo {
	defer guard("SetAgentsLevel")
	lv := min(max(level, prefs.AgentsMinLevel), prefs.AgentsMaxLevel)
	prefs.UpdateAgents(a.cfgDir, func(c *prefs.Agents) { c.Level = lv })
	a.agentsChanged()
	return a.agentsInfo()
}

// ClearAgentsLog очищает журнал вызовов; счётчик «за сегодня» остаётся.
func (a *App) ClearAgentsLog() AgentsInfo {
	defer guard("ClearAgentsLog")
	a.agents.ClearCalls()
	a.agentsChanged()
	return a.agentsInfo()
}

// agentsInfo собирает AgentsInfo из службы, config.json и пути программы.
func (a *App) agentsInfo() AgentsInfo {
	st, errText := a.agents.State()
	exe, _ := a.agentsExe()
	hint := osx.ExeHint(exe, runtime.GOOS, os.TempDir(), isRegular)
	setups := []AgentSetup{}
	if hint == "" {
		for _, s := range agents.Setups(exe, runtime.GOOS) {
			setups = append(setups, AgentSetup(s))
		}
		hint = agents.XDGHint(runtime.GOOS, os.Getenv("XDG_CONFIG_HOME"), userHome())
	}
	cfg := prefs.AgentsOf(a.cfgDir)
	info := AgentsInfo{Status: st, Hint: hint, Error: errText, Level: cfg.Level, Edit: a.editGroupOn(), EditBlocked: a.editBlocked(), AsksMuted: a.agents != nil && a.agents.AsksMuted(),
		Portable: runtime.GOOS == "windows" && update.ModeOf(runtime.GOOS, exe) == update.ModeSetup,
		Setups:   setups, Clients: []AgentClient{}, Calls: []AgentCall{}, Perms: a.agentPermRows(), Places: a.agentPlaces(), Lifted: a.agentLifted()}
	for _, c := range a.agents.Clients() {
		info.Clients = append(info.Clients, AgentClient{Name: c.Name, Since: c.Since.UnixMilli()})
	}
	calls, today := a.agents.Calls()
	info.Today = today
	for _, c := range calls {
		info.Calls = append(info.Calls, agentCall(c))
	}
	return info
}

// editBlocked — почему флажок правок нельзя включить ("" — можно): процесс от администратора.
func (a *App) editBlocked() string {
	if a.askM().elevated() {
		return agentBlockedElevated
	}
	return ""
}

func agentCall(c agents.Call) AgentCall {
	return AgentCall{At: c.At.UnixMilli(), Client: c.Client, Tool: c.Tool, OK: c.OK, Ms: c.Ms, Edit: c.Edit, Note: c.Note}
}

// isRegular — на месте обычный файл (не ссылка, не каталог).
func isRegular(path string) bool {
	st, err := os.Lstat(path)
	return err == nil && st.Mode().IsRegular()
}

// userHome — домашняя папка; "" — неизвестна.
func userHome() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// agentsChanged — состояние службы или список подключённых сменились: окно и значок получают
// agents:state целиком (в нём же журнал и счётчик).
func (a *App) agentsChanged() {
	a.emit("agents:state", a.agentsInfo())
}

// agentsCalled — служба записала вызов агента: agents:call, но не чаще раза в agentsCallGap; вызовы,
// пришедшие чаще, окно получит одним agents:state в конце окна (в нём весь журнал).
func (a *App) agentsCalled(c agents.Call, today int) {
	p := &a.agentsPub
	p.mu.Lock()
	now := time.Now()
	if wait := agentsCallGap - now.Sub(p.last); wait > 0 {
		if p.timer == nil && !a.stopped.Load() {
			p.timer = time.AfterFunc(wait, func() {
				defer guard("agentsCalled")
				p.mu.Lock()
				p.timer, p.last = nil, time.Now()
				p.mu.Unlock()
				a.agentsChanged()
			})
		}
		p.mu.Unlock()
		return
	}
	p.last = now
	p.mu.Unlock()
	a.emit("agents:call", AgentCallEvent{AgentCall: agentCall(c), Today: today})
}

// startAgents — запуск программы: доступ открывается, если он включён в config.json. В фоне, окно
// не ждёт сокет; отказ — состояние error (причина словами в окне), программа работает.
func (a *App) startAgents() {
	a.goBG(func() {
		defer guard("startAgents")
		a.agentsMu.Lock()
		defer a.agentsMu.Unlock()
		if a.stopped.Load() {
			return
		}
		_ = a.agents.StartIfOn() // причина — в State и в log.txt (без путей)
		if a.stopped.Load() {    // выход начался, пока открывался сокет: stopAgents его уже не застал
			a.agents.Close()
		}
	})
}

// stopAgents — выход: сокет закрыт, подключения оборваны, таймер журнала снят. Идёт до flushSave:
// рендер агента не должен держать выход (Close ждёт горутины не дольше секунды).
func (a *App) stopAgents() {
	a.closeAgentAsk() // открытый вопрос снимается первым: done{closed}, окно закрывается
	a.agentsPub.mu.Lock()
	if a.agentsPub.timer != nil {
		a.agentsPub.timer.Stop()
		a.agentsPub.timer = nil
	}
	a.agentsPub.mu.Unlock()
	if st, _ := a.agents.State(); st != agents.StateOff {
		a.agents.Close()
	}
}
