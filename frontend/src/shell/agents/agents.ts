// Окно «ИИ-агенты (MCP)» (143, У1): типы контракта с Go (design §5), слой вызовов и чистые подписи.
// Что и кому видно, пределы и уровни решает Go; здесь только форма данных и строки журнала.

import * as Api from '../../../wailsjs/go/app/App'
import type { app } from '../../../wailsjs/go/models'
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
  edit: boolean // группа правок не вся «Запретить» (150): первое включение спрашивает согласие 149
  editBlocked: '' | 'elevated' // правки нельзя включить: программа от администратора
  asksMuted: boolean // вопросы агентов отключены после трёх отказов подряд
  perms: AgentPerm[] // разрешения по инструментам (150); порядок — как в окне
  places: AgentPlace[] // места съёмок: где «Всегда» у папки берёт без вопроса
  lifted: AgentPlaceLift[] // места 0.34, поднятые до корня архива: заметка в окне висит до «Понятно» (153)
}
// Строка разрешений (150, design §6). group: read | edit | other (запуск и показ — вне группового щелчка).
export interface AgentPerm {
  tool: string // имя MCP
  title: string
  group: string
  mode: string // allow | ask | deny
  can: string[] // допустимые значения
  why: string // почему «Спрашивать» нет в can; показывается по нажатию
  note: string // подпись под названием
}
export interface AgentPlace {
  key: string // непрозрачный ключ из Go: по нему убирают и привязывают
  dir: string
  label: string // метка диска, серым справа; "" — без метки
  chosen: boolean // выбрана человеком («Папка со съёмками…»); иначе родитель папки, добавленной им
  bound: boolean // привязано к диску (153); на macOS всегда false
  narrow: boolean // место узкое (год): новый год спросит снова
  narrowRoot: boolean // узкое место лежит на корне диска (E:\2026)
  state: string // только из AgentPlacesLive: nodisk | nofolder | here (непривязанное, диск на месте) | ""
  here: string // только из AgentPlacesLive: метка диска, который сейчас по пути непривязанного места
}
export interface AgentPlaceLift {
  from: string
  to: string
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
  kind: 'create' | 'add' | 'edit' | 'export' | 'open'
  title: string
  lines: string[]
  hint: string // имя папки-подсказки агента; "" — нет
  ok: string // подпись действия: «Выбрать папку…», «Добавить», «Создать», «Собрать»
  path: string // настоящий путь папки (150), только окну; "" — вопрос 149 без пути
  link: boolean // подсказка шла через ссылку на другую папку
  alt: string // подпись «Выбрать другую…»; "" — кнопки нет
  tool: string // ссылка «Не спрашивать» ведёт к строке этого инструмента; у export и open пуст — у «только с вопросом» «Всегда» нет
  files?: number // export: сколько файлов запишет выгрузка (текст уже в lines)
  create?: boolean // export: папка будет создана (текст уже в lines; флаг — для вида пути)
  why: string // «Всегда», а тихо нельзя: place | remote | big | cloud | shots | link | rate | draft | bad; "" — «Спрашивать»
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
  prev?: boolean // у тоста «Открыть прежний» после тихого нового альбома (150)
  quiet?: boolean // правка без вопроса («Всегда»): тост держится до фокуса окна
  path?: string // полный путь тихой папки, только окну
}
// Событие agents:denied (150): первый отказ «Запретить» по инструменту за запуск программы.
export interface AgentDenied {
  client: string
  tool: string
  title: string
}
// Событие agents:placesLifted (153): места 0.34 подняты при запуске; тост-указатель на заметку в окне.
export interface AgentPlacesLifted {
  from: string[]
  to: string[]
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
  perms: [],
  places: [],
  lifted: [],
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
    perms: i.perms ?? [],
    places: i.places ?? [],
    lifted: i.lifted ?? [],
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
  setPerm(tool: string, mode: string): Promise<AgentsInfo> // mode вне can Go не принимает
  setPermGroup(group: string, mode: string): Promise<AgentsInfo> // read | edit
  forgetPlaces(): Promise<AgentsInfo>
  forgetPlace(key: string): Promise<AgentsInfo> // ✕ у места в списке
  bindPlace(key: string): Promise<AgentsInfo> // «Привязать к диску»; не узнали диск — отказ с текстом Go
  placesLive(): Promise<AgentPlace[]> // по раскрытию списка: состояние дисков и папок (Go опрашивает их только здесь)
  dismissLifted(): Promise<AgentsInfo> // «Понятно» в заметке о подъёме мест
  choosePlace(): Promise<AgentsInfo> // окно ОС «Папка со съёмками»; отмена — без изменений
  openPrev(): Promise<unknown> // State: «Открыть прежний» из тоста тихого нового альбома
  allowAsks(): Promise<void>
  answer(id: string, yes: boolean): Promise<unknown> // View: «Выбрать папку…» у add, отказ любого вида
  answerCreate(id: string, other?: boolean): Promise<unknown> // State: «Выбрать папку…» у create; other — «Выбрать другую…»
  answerOther(id: string): Promise<unknown> // View: «Выбрать другую…» у add
  answerExport(id: string, other?: boolean): Promise<app.ExportStarted> // «Выгрузить» / «Выбрать папку…»; other — «Выбрать другую…» (152)
  answerOpen(id: string, other?: boolean): Promise<unknown> // State: «Открыть» / «Выбрать файл…»; other — «Выбрать другой…» (152)
  busy(on: boolean): Promise<void> // человек занят: правка агента подождёт (agentbusy.ts)
}

export const bridge: AgentsBridge = {
  info: () => Api.AgentsInfo().then(normInfo),
  set: (on) => Api.SetAgents(on).then(normInfo),
  setLevel: (n) => Api.SetAgentsLevel(n).then(normInfo),
  clearLog: () => Api.ClearAgentsLog().then(normInfo),
  saveExtension: () => Api.SaveClaudeExtension(),
  setPerm: (tool, mode) => Api.SetAgentPerm(tool, mode).then(normInfo),
  setPermGroup: (group, mode) => Api.SetAgentPermGroup(group, mode).then(normInfo),
  forgetPlaces: () => Api.ForgetAgentPlaces().then(normInfo),
  forgetPlace: (key) => Api.ForgetAgentPlace(key).then(normInfo),
  bindPlace: (key) => Api.BindAgentPlace(key).then(normInfo),
  placesLive: () => Api.AgentPlacesLive().then((l) => l ?? []),
  dismissLifted: () => Api.DismissPlacesLifted().then(normInfo),
  choosePlace: () => Api.ChooseAgentPlace().then(normInfo),
  openPrev: () => Api.AgentOpenPrev(),
  allowAsks: () => Api.AllowAgentAsks(),
  answer: (id, yes) => Api.AnswerAgent(id, yes),
  answerCreate: (id, other = false) => Api.AgentAnswerCreate(id, other),
  answerOther: (id) => Api.AnswerAgentOther(id),
  answerExport: (id, other = false) => Api.AgentAnswerExport(id, other),
  answerOpen: (id, other = false) => Api.AgentAnswerOpen(id, other),
  busy: (on) => Api.AgentsBusy(on),
}
