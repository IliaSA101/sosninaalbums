package app

// Вопросы агента в окне программы и события правок (149, контракт Т1). Здесь типы событий и
// методы для интерфейса; тело вопроса (одно окно за раз, счёт отказов, выбор папки) — задача Т3,
// до неё методы честно отвечают «ещё не готово» и ничего не меняют.

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// errAgentEditSoon — методы правок агента ещё без тела (Т3).
var errAgentEditSoon = errors.New("правки агентов ещё не готовы")

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
func (a *App) AnswerAgent(id string, yes bool) (v model.View, err error) {
	defer guardErr("AnswerAgent", &err)
	return model.View{}, errAgentEditSoon
}

// AgentAnswerCreate — «Выбрать папку…» в вопросе «Создать альбом»: новый альбом из выбранной папки.
func (a *App) AgentAnswerCreate(id string) (st State, err error) {
	defer guardErr("AgentAnswerCreate", &err)
	return State{}, errAgentEditSoon
}

// AgentsBusy — интерфейс сообщает, что человек занят (перенос, набор текста, задача, окна
// «Автосборка» и «Настройки альбома»): правка агента подождёт.
func (a *App) AgentsBusy(on bool) {
	defer guard("AgentsBusy")
	_ = on // состояние занятости — Т3
}

// AllowAgentAsks снимает тишину после третьего отказа подряд («Разрешить вопросы»).
func (a *App) AllowAgentAsks() {
	defer guard("AllowAgentAsks")
}
