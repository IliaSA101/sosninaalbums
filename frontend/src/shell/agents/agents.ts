// Окно «ИИ-агенты (MCP)» (143, У1): типы контракта с Go (design §5), слой вызовов и чистые подписи.
// Что и кому видно, пределы и уровни решает Go; здесь только форма данных и строки журнала.

import * as Api from '../../../wailsjs/go/app/App'
import { EDIT_MARK } from './agentstext'

export type AgentsStatus = 'off' | 'on' | 'error'

export interface AgentSetup {
  id: string // claude-desktop | claude-code | opencode | other
  title: string
  text: string // готово к копированию — собрано в Go
  note: string
  file: boolean // есть «Сохранить файл для Claude Desktop…» (только Windows и Mac)
}
export interface AgentClient {
  name: string
  since: number // unix мс
}
export interface AgentCall {
  at: number // unix мс
  client: string
  tool: string
  ok: boolean
  ms: number
  edit?: boolean // правящий инструмент (149): в журнале ✎
  note?: string // итог словами Go: отказано | без ответа | занят | копия | поздно | ""
}
export interface AgentsInfo {
  status: AgentsStatus
  hint: string // "" или предупреждение над «Как подключить» (мёртвый путь exe, XDG_CONFIG_HOME)
  error: string
  level: number // 1…3
  portable: boolean
  setups: AgentSetup[]
  clients: AgentClient[]
  calls: AgentCall[] // новые первыми, ≤ 50
  today: number
  edit: boolean // агентам разрешено менять альбом (149)
  editBlocked: '' | 'elevated' // правки нельзя включить: программа от администратора
  asksMuted: boolean // вопросы агентов отключены после трёх отказов подряд
}
export interface AgentsSaved {
  canceled: boolean
  text: string
}
// Событие agents:call: вызов и счётчик «за сегодня».
export type AgentCallEvent = AgentCall & { today: number }

// События правок (149; Go — AgentAsk, AgentAskDone, AgentEdited, AgentShow, docs/architecture/events.md).
// Тексты вопроса и тостов целиком из Go: агент своих слов не вносит.
export interface AgentAsk {
  id: string
  kind: 'create' | 'add'
  title: string
  lines: string[]
  hint: string // имя папки-подсказки агента; "" — нет
  ok: string // подпись действия: «Выбрать папку…»
}
export type AgentAskWhy = 'answered' | 'declined' | 'timeout' | 'gone' | 'closed' | 'off' | 'too_wide' | 'ours'
export interface AgentAskDone {
  id: string
  why: AgentAskWhy
}
export interface AgentEdited {
  client: string
  text: string
  rev: number
  go: string // ID разворота для перехода; "" — без перехода
}
export interface AgentShow {
  spread: string
}

export const CALLS_MAX = 50

export const emptyInfo = (): AgentsInfo => ({
  status: 'off',
  hint: '',
  error: '',
  level: 1,
  portable: false,
  setups: [],
  clients: [],
  calls: [],
  today: 0,
  edit: false,
  editBlocked: '',
  asksMuted: false,
})

// Ответ Go с пропусками (null вместо списка) приводим к форме: шаблон не должен проверять каждое поле.
// Тип Wails даёт `status: string` — здесь он сужается сам, без приведения типов в вызывающем коде.
export type AgentsInfoRaw = Omit<Partial<AgentsInfo>, 'status' | 'editBlocked'> & { status?: string; editBlocked?: string }
export function normInfo(i: AgentsInfoRaw | null | undefined): AgentsInfo {
  const e = emptyInfo()
  if (!i) return e
  return {
    ...e,
    ...i,
    status: i.status === 'on' || i.status === 'error' ? i.status : 'off',
    editBlocked: i.editBlocked === 'elevated' ? 'elevated' : '',
    level: Math.min(3, Math.max(1, Math.round(i.level ?? 1))),
    setups: i.setups ?? [],
    clients: i.clients ?? [],
    calls: i.calls ?? [],
  }
}

// Новый вызов — наверх журнала; журнал не длиннее CALLS_MAX.
export const pushCall = (calls: AgentCall[], c: AgentCall): AgentCall[] => [c, ...calls].slice(0, CALLS_MAX)

const two = (n: number): string => String(n).padStart(2, '0')

// «12:03 · Claude Desktop · Сводка альбома · 4 мс»; неудача — «· ошибка». Правка — «✎ » перед инструментом,
// итог правки словами Go (note: «отказано», «без ответа»…) вместо «ошибка». Имя клиента и инструмента —
// чужой текст (Go чистит и обрезает), показываем только текстом.
export function callLine(c: AgentCall): string {
  const d = new Date(c.at)
  const t = `${two(d.getHours())}:${two(d.getMinutes())}`
  const tool = c.edit ? `${EDIT_MARK} ${c.tool}` : c.tool
  const tail = c.note ? [c.note] : c.ok ? [] : ['ошибка']
  return [t, c.client, tool, `${c.ms} мс`, ...tail].join(' · ')
}

// Имена подключённых без повторов (два окна Claude Code — одна строка).
export const clientNames = (clients: AgentClient[]): string[] => [...new Set(clients.map((c) => c.name))]

// Флажки уровней: уровень 2 — имена и тексты, 3 — ещё картинки (картинки включают первый флажок).
export const hasNames = (level: number): boolean => level >= 2
export const hasPics = (level: number): boolean => level >= 3
// Уровень после щелчка по флажку; зажим 1…3 остаётся за Go.
export const levelAfterNames = (on: boolean): number => (on ? 2 : 1)
export const levelAfterPics = (on: boolean): number => (on ? 3 : 2)

// ───────────── Слой вызовов Go ─────────────
// Весь Go только здесь: UI зовёт `bridge`, а не `wailsjs`. Тип события agents:call в models.ts не описан
// (Go шлёт AgentCall + today), поэтому AgentCallEvent выше — локальный.
// Включить не вышло — Go отвечает не ошибкой, а info со status error и причиной словами (SetAgents(true));
// SetAgents(false) принимается и из error. Эти ответы normInfo пропускает как есть.
export interface AgentsBridge {
  info(): Promise<AgentsInfo>
  set(on: boolean): Promise<AgentsInfo>
  setLevel(level: number): Promise<AgentsInfo>
  clearLog(): Promise<AgentsInfo>
  saveExtension(): Promise<AgentsSaved>
  setEdit(on: boolean): Promise<AgentsInfo>
  allowAsks(): Promise<void>
  answer(id: string, yes: boolean): Promise<unknown> // View: «Выбрать папку…» у add, отказ любого вида
  answerCreate(id: string): Promise<unknown> // State: «Выбрать папку…» у create
  busy(on: boolean): Promise<void> // человек занят: правка агента подождёт (agentbusy.ts)
}

export const bridge: AgentsBridge = {
  info: () => Api.AgentsInfo().then(normInfo),
  set: (on) => Api.SetAgents(on).then(normInfo),
  setLevel: (n) => Api.SetAgentsLevel(n).then(normInfo),
  clearLog: () => Api.ClearAgentsLog().then(normInfo),
  saveExtension: () => Api.SaveClaudeExtension(),
  setEdit: (on) => Api.SetAgentsEdit(on).then(normInfo),
  allowAsks: () => Api.AllowAgentAsks(),
  answer: (id, yes) => Api.AnswerAgent(id, yes),
  answerCreate: (id) => Api.AgentAnswerCreate(id),
  busy: (on) => Api.AgentsBusy(on),
}
