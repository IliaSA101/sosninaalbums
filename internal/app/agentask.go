package app

// Вопросы агента в окне программы и события правок (149, контракт Т1). Здесь типы событий и методы
// для интерфейса; машина вопроса (одно окно за раз, состояния asked → choosing → done, выбор
// папки, выход) — agentask_flow.go, применение ответа — agentask_apply.go. Счёт отказов, пауза и
// тишина — в службе агентов, фасад их не повторяет.

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// AgentAsk — событие agents:ask: агент просит человека о действии, текст целиком из шаблонов Go.
type AgentAsk struct {
	ID    string   `json:"id"`    // 16 байт crypto/rand, hex
	Kind  string   `json:"kind"`  // create | add | edit (150: обратимая правка и показ при «Спрашивать»)
	Title string   `json:"title"` // «Создать альбом по просьбе агента?»
	Lines []string `json:"lines"` // абзацы: кто просит, что станет с открытым альбомом
	Hint  string   `json:"hint"`  // последний элемент годной подсказки ≤ 60 знаков; "" — нет
	OK    string   `json:"ok"`    // подпись кнопки справа: «Выбрать папку…»
	// 150: вопрос с полным путём. Path — настоящий путь после ссылок, только окну программы;
	// "" — вопрос 149 без пути.
	Path string `json:"path"`
	Link bool   `json:"link"` // подсказка шла через ссылку: «(по ссылке)»
	Alt  string `json:"alt"`  // подпись «Выбрать другую…»; "" — кнопки нет
	Tool string `json:"tool"` // ссылка «Не спрашивать — …» открывает окно на строке этого инструмента
	// Why — «Всегда», а тихо нельзя: place | remote | big | cloud | shots | link | rate | draft;
	// "" — «Спрашивать». Тексты — во фронте (agentstext.ts), агенту причина не сообщается.
	Why string `json:"why"`
}

// AgentAskDone — событие agents:askdone: вопрос закончен, окно закрыть.
type AgentAskDone struct {
	ID  string `json:"id"`
	Why string `json:"why"` // answered | declined | timeout | gone | closed | off
}

// AgentEdited — событие agents:edited: агент изменил альбом; интерфейс показывает тост с «Вернуть».
type AgentEdited struct {
	Client string `json:"client"` // имя клиента
	Text   string `json:"text"`   // «Собрано разворотов: 24»
	Rev    int    `json:"rev"`    // ревизия после правки: «Вернуть» только при совпадении
	Go     string `json:"go"`     // ID разворота для перехода; "" — без перехода
	// 150: Prev — у тоста «Открыть прежний» (AgentOpenPrev) после тихого нового альбома;
	// Quiet — правка без вопроса («Всегда»); Path — полный путь тихой папки, только окну.
	Prev  bool   `json:"prev"`
	Quiet bool   `json:"quiet"`
	Path  string `json:"path"`
}

// AgentDenied — событие agents:denied: первый отказ «Запретить» по инструменту за запуск программы
// (150, 0.34-150-24); интерфейс показывает тост «Настроить».
type AgentDenied struct {
	Client string `json:"client"` // имя клиента
	Tool   string `json:"tool"`   // имя MCP
	Title  string `json:"title"`  // «Создать альбом»
}

// AgentShow — событие agents:show: агент просит показать разворот (альбом не меняется).
type AgentShow struct {
	Spread string `json:"spread"` // ID разворота
}

// Отправка событий правок (зовёт фасад Т3).
func (a *App) agentAsked(v AgentAsk)       { a.emit("agents:ask", v) }
func (a *App) agentAskDone(v AgentAskDone) { a.emit("agents:askdone", v) }
func (a *App) agentEdited(v AgentEdited)   { a.emit("agents:edited", v) }
func (a *App) agentShown(v AgentShow)      { a.emit("agents:show", v) }
func (a *App) agentDenied(v AgentDenied)   { a.emit("agents:denied", v) }

// AnswerAgent — ответ на вопрос «Выбрать папку…» (add), «да» на вопрос о правке (edit) или отказ
// любого вида (yes = false). Вопрос закрыт, чужой или другого вида — ошибка, ничего не делается.
func (a *App) AnswerAgent(id string, yes bool) (v model.View, err error) {
	defer guardErr("AnswerAgent", &err)
	if kind, ok := a.askKindOf(id); yes && ok && kind == askKindEdit {
		return a.answerEdit(id)
	}
	return a.answerAdd(id, yes, false)
}

// AnswerAgentOther — «Выбрать другую…» в вопросе о папке: окно ОС в родителе показанного пути
// (150). Места не пополняет.
func (a *App) AnswerAgentOther(id string) (v model.View, err error) {
	defer guardErr("AnswerAgentOther", &err)
	return a.answerAdd(id, true, true)
}

// answerAdd — ответ на вопрос «Добавить папку»: yes — «Добавить»/«Выбрать папку…», other — окно ОС.
func (a *App) answerAdd(id string, yes, other bool) (model.View, error) {
	rec, err := a.askClaim(id, askKindAdd, yes)
	if err != nil || rec == nil {
		return a.askView(), err
	}
	defer close(rec.idle)
	dir, ok := a.askPick(rec, other)
	if !ok {
		return a.askView(), nil
	}
	if v, done := a.askApplyAdd(rec, dir); done {
		return v, nil
	}
	return a.askView(), nil
}

// AgentAnswerCreate — «Создать» / «Выбрать папку…» в вопросе «Создать альбом»: новый альбом из
// показанной (other = false) или выбранной в окне ОС (other, «Выбрать другую…») папки. Интерфейс
// зовёт внутри newDoc.replace; отказ идёт через AnswerAgent(id, false).
func (a *App) AgentAnswerCreate(id string, other bool) (st State, err error) {
	defer guardErr("AgentAnswerCreate", &err)
	rec, err := a.askClaim(id, askKindCreate, true)
	if err != nil || rec == nil {
		return a.cancelledState(), err
	}
	defer close(rec.idle)
	dir, ok := a.askPick(rec, other)
	if !ok {
		return a.cancelledState(), nil
	}
	if !a.askApplyCreate(rec, dir) {
		return a.cancelledState(), nil
	}
	return a.GetState(), nil
}

// cancelledState — состояние прежнего альбома с Cancelled (как в OpenProject): новый альбом не
// создан, интерфейс не должен принимать ответ за новый проект.
func (a *App) cancelledState() State {
	st := a.GetState()
	st.Cancelled = true
	return st
}

// AllowAgentAsks снимает тишину после третьего отказа подряд («Разрешить вопросы»).
func (a *App) AllowAgentAsks() {
	defer guard("AllowAgentAsks")
	if a.agents != nil {
		a.agents.AllowAsks()
	}
}

// askView — вид текущего альбома для ответа AnswerAgent (без изменений, если правки не было).
func (a *App) askView() model.View {
	if ed := a.editor(); ed != nil {
		return a.out(ed.View())
	}
	return model.View{}
}
