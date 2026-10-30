package agents

import (
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Разрешения агентов по инструментам (спека 0.34/150, design §3): проверка в serve, тихий слот,
// событие «запрещено», перечитывание настроек на каждом вызове. Хранение значений — prefs, решение
// «тихо или нет» — у фасада (Host.Ask с AskReq.Always).

// WhyQuiet — итог вопроса: папка или альбом приняты без вопроса человеку («Всегда» у папки,
// design §4.1). Это успех: без yes() и strike(), запись отмены пишет afterAsk.
const WhyQuiet = "quiet"

// errDenied — инструмент запрещён человеком (режим deny). Не отказ человека в вопросе: в счёт
// отказов не идёт. Путей и данных альбома в тексте нет.
var errDenied = newAgentErr("Запрещено человеком в программе: Файл ▸ ИИ-агенты (MCP)… ▸ Разрешения")

// Итоги вызова для журнала (Call.Note), кроме отказов из noteOf.
const (
	noteDenied = "запрещено"
	noteQuiet  = "без вопроса" // «Всегда» у папки: вопроса не было
	noteYes    = "да"          // человек ответил «да»
)

// Тихий слот (design §3): не больше трёх тихих папок и альбомов за десять минут на программу.
const (
	quietWindow = 10 * time.Minute
	quietMax    = 3
)

// ToolInfo — инструмент службы для окна «Разрешения»: имя MCP, название и класс (prefs.Class*).
type ToolInfo struct {
	Name  string
	Title string
	Class string
}

// permOf — разрешение на этот вызов: читается из настроек каждый раз (смена в окне действует сразу).
// Незнакомое значение и «Спрашивать» там, где его нет (чтение), — «Запретить»: закрыто по умолчанию.
// Host без Perm (только чтение) разрешает всё. Служебный инструмент (await_answer) вне разрешений.
func (s *Service) permOf(spec toolSpec) (mode, blocked string) {
	mode, blocked, _ = s.permLevel(spec)
	return mode, blocked
}

// permLevel — permOf и уровень доступа: с Host.PermLevel оба из одного чтения config.json (design §2,
// «одно чтение на вызов»); без него level < 0 — уровень читает вызывающий.
func (s *Service) permLevel(spec toolSpec) (mode, blocked string, level int) {
	level = -1
	if spec.perm == "" || s.h.Perm == nil {
		return prefs.PermAllow, "", level
	}
	if s.h.PermLevel != nil {
		mode, blocked, level = s.h.PermLevel(spec.name)
	} else {
		mode, blocked = s.h.Perm(spec.name)
	}
	switch {
	case mode == prefs.PermAllow:
	case mode == prefs.PermAsk && spec.perm != prefs.ClassRead && spec.perm != prefs.ClassStart:
	default:
		mode = prefs.PermDeny
	}
	return mode, blocked, level
}

// firstDenied — true, если этот инструмент ещё не называли в agents:denied за запуск программы.
func (e *editState) firstDenied(tool string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.denied[tool] {
		return false
	}
	e.denied[tool] = true
	return true
}

// deny — вызов отклонён разрешением: тост «Настроить» раз на инструмент за запуск.
func (c *call) deny() *agentErr {
	s := c.s
	if s.o.OnDenied != nil && s.ed.firstDenied(c.spec.name) {
		func() {
			defer guardPanic("OnDenied")
			s.o.OnDenied(c.client(), c.spec.name, c.spec.title)
		}()
	}
	return errDenied
}

// editAllowed — есть ли у агента хоть одна правка (get_album.edits): не все правки «Запретить».
// blocked — "elevated" для процесса от администратора.
func (s *Service) editAllowed() (allowed bool, blocked string) {
	for _, sp := range s.Specs() {
		if !sp.edit {
			continue
		}
		mode, b := s.permOf(sp)
		if b != "" {
			return false, b
		}
		allowed = allowed || mode != prefs.PermDeny
	}
	return allowed, ""
}

// takeQuiet резервирует тихий слот; нет места в окне — false. Вернуть: returnQuiet.
func (e *editState) takeQuiet() (stamp time.Time, ok bool) {
	now := e.now()
	e.mu.Lock()
	defer e.mu.Unlock()
	keep := e.quiet[:0]
	for _, t := range e.quiet {
		if now.Sub(t) < quietWindow {
			keep = append(keep, t)
		}
	}
	e.quiet = keep
	if len(e.quiet) >= quietMax {
		return time.Time{}, false
	}
	e.quiet = append(e.quiet, now)
	return now, true
}

// returnQuiet отдаёт слот: фасад вернул вопрос, а не тишину, или вопрос не дошёл до итога.
func (e *editState) returnQuiet(stamp time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.quiet) - 1; i >= 0; i-- {
		if e.quiet[i].Equal(stamp) {
			e.quiet = append(e.quiet[:i], e.quiet[i+1:]...)
			return
		}
	}
}

// PermsChanged — разрешения сменились в окне: открытый вопрос, чей инструмент стал «Запретить»,
// закрывается итогом off (в счёт отказов не идёт); следующий вызов и так читает настройки заново.
func (s *Service) PermsChanged() {
	if s.h.Perm == nil {
		return
	}
	e := s.ed
	e.mu.Lock()
	var open []*pend
	for _, set := range e.asks {
		for p := range set {
			if p.tool != "" && !p.finished() {
				open = append(open, p)
			}
		}
	}
	e.mu.Unlock()
	for _, p := range open {
		if mode, _ := s.h.Perm(p.tool); mode != prefs.PermAllow && mode != prefs.PermAsk {
			e.mu.Lock()
			p.off = true
			e.mu.Unlock()
			p.cancel()
		}
	}
}

// Specs — таблица инструментов службы по этому Host (какие замыкания заполнены); собирается один раз
// на отдельном сервере. От настроек не зависит.
func (s *Service) Specs() []toolSpec {
	s.specsOnce.Do(func() {
		e := &toolEnv{s: s, srv: mcp.NewServer(&mcp.Implementation{Name: "sosnina-albums"}, nil)}
		registerTools(e)
		s.specs = e.specs
	})
	return s.specs
}

// Tools — инструменты службы с классами для окна «Разрешения» (названия и группы).
func (s *Service) Tools() []ToolInfo {
	var out []ToolInfo
	for _, sp := range s.Specs() {
		if sp.perm != "" {
			out = append(out, ToolInfo{Name: sp.name, Title: sp.title, Class: sp.perm})
		}
	}
	return out
}
