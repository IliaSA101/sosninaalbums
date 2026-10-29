package agents

import (
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// journalMax — сколько вызовов помнит журнал окна; старые вытесняются.
const journalMax = 50

// Call — запись журнала: один вызов инструмента агентом. Ни аргументов, ни ответа в ней нет —
// только кто, что, удачно ли и за сколько (журнал без данных альбома).
type Call struct {
	At     time.Time
	Client string // clientInfo.name, очищено
	Tool   string // название инструмента для человека («Сводка альбома»)
	OK     bool
	Ms     int
}

// journal — последние вызовы (новые первыми) и число вызовов за сегодня. Живёт в памяти: на диск
// не пишется, при выходе пропадает.
type journal struct {
	mu    sync.Mutex
	calls []Call
	day   string // день счётчика today: "2006-01-02" по местному времени
	today int
	now   func() time.Time // подмена часов в тестах
}

func (j *journal) clock() time.Time {
	if j.now != nil {
		return j.now()
	}
	return time.Now()
}

// add кладёт вызов первым; возвращает счётчик «за сегодня» вместе с этим вызовом.
func (j *journal) add(c Call) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.calls = append([]Call{c}, j.calls...)
	if len(j.calls) > journalMax {
		j.calls = j.calls[:journalMax]
	}
	j.rollLocked(c.At)
	j.today++
	return j.today
}

// rollLocked обнуляет счётчик, когда день at не тот, что у счётчика.
func (j *journal) rollLocked(at time.Time) {
	if d := at.Format(time.DateOnly); d != j.day {
		j.day, j.today = d, 0
	}
}

// Calls — журнал (новые первыми, копия) и счётчик «за сегодня».
func (s *Service) Calls() (calls []Call, today int) {
	j := &s.jr
	j.mu.Lock()
	defer j.mu.Unlock()
	j.rollLocked(j.clock())
	return append([]Call{}, j.calls...), j.today
}

// ClearCalls очищает список вызовов; счётчик «за сегодня» остаётся (это число запросов дня, а не
// содержимое журнала).
func (s *Service) ClearCalls() {
	s.jr.mu.Lock()
	s.jr.calls = nil
	s.jr.mu.Unlock()
}

// record — запись вызова после ответа; обратный вызов приложения — после отпускания замка.
func (s *Service) record(spec toolSpec, req *mcp.CallToolRequest, res *mcp.CallToolResult, d time.Duration) {
	defer guardPanic("journal")
	c := Call{At: s.jr.clock(), Client: clientOf(req), Tool: spec.title, OK: res != nil && !res.IsError, Ms: int(d.Milliseconds())}
	today := s.jr.add(c)
	if s.o.OnCall != nil {
		s.o.OnCall(c, today)
	}
}

// clientOf — имя клиента сессии вызова (после initialize); нет — пусто.
func clientOf(req *mcp.CallToolRequest) string {
	if req == nil || req.Session == nil {
		return ""
	}
	if ip := req.Session.InitializeParams(); ip != nil && ip.ClientInfo != nil {
		return cleanName(ip.ClientInfo.Name)
	}
	return ""
}
