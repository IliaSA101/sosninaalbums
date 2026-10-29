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
	Kind  string   `json:"kind"`  // create | add
	Title string   `json:"title"` // «Создать альбом по просьбе агента?»
	Lines []string `json:"lines"` // абзацы: кто просит, что станет с открытым альбомом
	Hint  string   `json:"hint"`  // последний элемент годной подсказки ≤ 60 знаков; "" — нет
	OK    string   `json:"ok"`    // подпись кнопки справа: «Выбрать папку…»
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

// AnswerAgent — ответ на вопрос «Выбрать папку…» (add) или отказ любого вида (yes = false).
// Вопрос закрыт, чужой или другого вида — ошибка, ничего не делается.
func (a *App) AnswerAgent(id string, yes bool) (v model.View, err error) {
	defer guardErr("AnswerAgent", &err)
	rec, err := a.askClaim(id, askKindAdd, yes)
	if err != nil || rec == nil {
		return a.askView(), err
	}
	defer close(rec.idle)
	dir, ok := a.askPick(rec)
	if !ok {
		return a.askView(), nil
	}
	if v, done := a.askApplyAdd(rec, dir); done {
		return v, nil
	}
	return a.askView(), nil
}

// AgentAnswerCreate — «Выбрать папку…» в вопросе «Создать альбом»: новый альбом из выбранной папки.
// Интерфейс зовёт внутри newDoc.replace; отказ идёт через AnswerAgent(id, false).
func (a *App) AgentAnswerCreate(id string) (st State, err error) {
	defer guardErr("AgentAnswerCreate", &err)
	rec, err := a.askClaim(id, askKindCreate, true)
	if err != nil || rec == nil {
		return a.cancelledState(), err
	}
	defer close(rec.idle)
	dir, ok := a.askPick(rec)
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
