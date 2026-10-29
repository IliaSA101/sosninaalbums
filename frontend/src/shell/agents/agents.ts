// Окно «ИИ-агенты (MCP)» (143, У1): типы контракта с Go (design §5), слой вызовов и чистые подписи.
// Что и кому видно, пределы и уровни решает Go; здесь только форма данных и строки журнала.

import * as Api from '../../../wailsjs/go/app/App'

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
}
export interface AgentsSaved {
  canceled: boolean
  text: string
}
// Событие agents:call: вызов и счётчик «за сегодня».
export type AgentCallEvent = AgentCall & { today: number }

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
})

// Ответ Go с пропусками (null вместо списка) приводим к форме: шаблон не должен проверять каждое поле.
// Тип Wails даёт `status: string` — здесь он сужается сам, без приведения типов в вызывающем коде.
export type AgentsInfoRaw = Omit<Partial<AgentsInfo>, 'status'> & { status?: string }
export function normInfo(i: AgentsInfoRaw | null | undefined): AgentsInfo {
  const e = emptyInfo()
  if (!i) return e
  return {
    ...e,
    ...i,
    status: i.status === 'on' || i.status === 'error' ? i.status : 'off',
    level: Math.min(3, Math.max(1, Math.round(i.level ?? 1))),
    setups: i.setups ?? [],
    clients: i.clients ?? [],
    calls: i.calls ?? [],
  }
}

// Новый вызов — наверх журнала; журнал не длиннее CALLS_MAX.
export const pushCall = (calls: AgentCall[], c: AgentCall): AgentCall[] => [c, ...calls].slice(0, CALLS_MAX)

const two = (n: number): string => String(n).padStart(2, '0')

// «12:03 · Claude Desktop · Сводка альбома · 4 мс»; неудача — «· ошибка». Имя клиента и инструмента —
// чужой текст (Go чистит и обрезает), показываем только текстом.
export function callLine(c: AgentCall): string {
  const d = new Date(c.at)
  const t = `${two(d.getHours())}:${two(d.getMinutes())}`
  return [t, c.client, c.tool, `${c.ms} мс`, ...(c.ok ? [] : ['ошибка'])].join(' · ')
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
}

export const bridge: AgentsBridge = {
  info: () => Api.AgentsInfo().then(normInfo),
  set: (on) => Api.SetAgents(on).then(normInfo),
  setLevel: (n) => Api.SetAgentsLevel(n).then(normInfo),
  clearLog: () => Api.ClearAgentsLog().then(normInfo),
  saveExtension: () => Api.SaveClaudeExtension(),
}
