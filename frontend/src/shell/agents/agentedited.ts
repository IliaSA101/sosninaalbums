import { store } from '../../base/core/store.svelte'
import { undoAction, undoToast, type ToastAction } from '../../base/core/toasts.svelte'
import { capFirst, errText } from '../../base/words/errtext'
import { newDoc } from '../newdoc/newdoc.svelte'
import { bridge, type AgentDenied, type AgentEdited, type AgentShow } from './agents'
import { OPEN_PREV_LABEL, SETUP_LABEL, deniedText } from './agentstext'

// Тосты правок агента (149-30, design §4; 150, design §7.3). Текст приходит из Go («Собрано разворотов: 24»), здесь — имя клиента,
// «Вернуть» (механизм undoToast: только при той же ревизии) и переход к развороту или обложке (go = "cover", 151).
// Правка без вопроса (quiet / prev) и первый отказ «Запретить» дают тост «до фокуса» (Toasts.hold): человек в это время в чате.
// Один тост — одно событие: обычная правка заменяет прежний тост агента. Дописывается только своё: правки к тихой правке (путь и
// «Открыть прежний» не должны пропасть, когда автосборка идёт следом) и отказы к отказам (все названия в одной строке).
const GLUE_MAX = 3 // в тихой склейке не больше стольких частей: первая (путь, prev) закреплена
const GO_TRIES = 10 // разворот мог ещё не дойти до интерфейса событием view: ждём до секунды
const GO_WAIT = 100

interface Last {
  client: string
  lead: string // «Клиент · » (у отказа не нужен — текст собирает deniedText)
  kind: 'edit' | 'quiet' | 'denied' // чем начат тост: обычной правкой, тихой правкой или отказом
  parts: string[] // части текста правки; у отказа — названия инструментов
  text: string
  held: boolean // тост «до фокуса»
  prev: boolean // в склейке есть «Открыть прежний»
  denied: string // инструмент последнего отказа в склейке; "" — нет
  setup: ((tool: string) => void) | null // «Настроить»: открыть окно на строке инструмента
  paths: string[]
}
let last: Last | null = null

// Для тестов: забыть прошлый тост.
export function resetEdited() {
  last = null
}

// Перейти к листу (разворот или `cover` — обложка), когда он появится в виде (view приходит событием, обгоняющим или догоняющим agents:*).
function goWhen(id: string, tries = GO_TRIES) {
  if (!id) return
  if (store.sheet(id)) return store.goId(id)
  if (tries > 0) setTimeout(() => goWhen(id, tries - 1), GO_WAIT)
}

// «Открыть прежний»: Go закрепляет несохранённый текущий копией и открывает прежний файл или копию (AgentOpenPrev).
async function openPrev() {
  try {
    await newDoc.replace(() => bridge.openPrev())
  } catch (e) {
    store.notify(capFirst(errText(e)), 'error')
  }
}

// Части тихой склейки: первая закреплена, из прочих остаются последние GLUE_MAX - 1; повтор уже названного не дублируется.
function join(parts: string[], part: string): string[] {
  if (parts.includes(part)) return parts
  return [parts[0], ...[...parts.slice(1), part].slice(-(GLUE_MAX - 1))]
}

// Тост склейки может ждать фокуса сколько угодно, поэтому «тот же» — по тексту на экране, а не по часам.
const onScreen = (l: Last) => store.toast?.text === l.text && !!store.toast.hold

// Тост, в который дописать новую часть: тот же клиент, он ещё на экране, и пара подходит (см. шапку файла).
// Обычный тост (правка с «Вернуть») не склеивается никогда: view новой ревизии гасит его раньше agents:edited, а дописывать
// к погасшему — значит копить старые строки.
function glueTarget(client: string, kind: Last['kind']): Last | null {
  if (!last || last.client !== client || !onScreen(last)) return null
  if (kind === 'denied') return last.kind === 'denied' ? last : null
  return last.kind === 'quiet' ? last : null
}

const fresh = (client: string, lead: string, kind: Last['kind']): Last => ({
  client,
  lead,
  kind,
  parts: [],
  text: '',
  held: kind !== 'edit',
  prev: false,
  denied: '',
  setup: null,
  paths: [],
})

// Показать тост склейки. Действие по старшинству: «Открыть прежний» → «Настроить» → «Вернуть» последней правки.
function show(l: Last, rev: number | null) {
  l.text = l.kind === 'denied' ? deniedText(l.client, l.parts) : l.lead + l.parts.join('. ')
  last = l
  if (!l.held) {
    if (rev !== null && store.view.rev <= rev)
      undoToast(store, l.text, undefined, rev) // view мог ещё не дойти — сверка при нажатии
    else store.notify(l.text) // после правки агента уже были другие: «Вернуть» откатило бы не её
    return
  }
  let action: ToastAction | undefined
  let at: number | undefined
  if (l.prev) action = { label: OPEN_PREV_LABEL, run: () => void openPrev() }
  else if (l.denied && l.setup) action = { label: SETUP_LABEL, run: () => l.setup?.(l.denied) }
  else if (rev !== null && store.view.rev <= rev) {
    at = rev
    action = undoAction(store, rev)
  }
  store.hold(l.text, { action, rev: at, paths: l.paths })
}

// Агент изменил альбом: тост «Клиент · что сделано» с «Вернуть» и переход к `go`. rev — ревизия после правки: на «Вернуть»
// сверяется нынешняя (иначе «Уже были другие изменения»), другая правка гасит тост сама (Store.apply по toast.rev).
export function agentEdited(e: AgentEdited) {
  const part = e.path ? `${e.text} ${e.path}` : e.text
  const starts = !!e.quiet || !!e.prev
  const kind = starts ? 'quiet' : 'edit'
  const l = glueTarget(e.client, kind) ?? fresh(e.client, `${e.client} · `, kind)
  l.parts = l.parts.length === 0 ? [part] : join(l.parts, part)
  l.prev ||= !!e.prev
  if (e.path && !l.paths.includes(e.path)) l.paths = [...l.paths, e.path]
  show(l, e.rev)
  goWhen(e.go)
}

// Первый отказ «Запретить» по инструменту: «Клиент: «Создать альбом», «Автосборка» запрещено · Настроить» (все названия одной строкой). setup открывает окно «ИИ-агенты» на строке
// инструмента (передаёт agents.svelte: прямой импорт дал бы цикл).
export function agentDenied(d: AgentDenied, setup: (tool: string) => void) {
  const l = glueTarget(d.client, 'denied') ?? fresh(d.client, '', 'denied')
  if (!l.parts.includes(d.title)) l.parts = [...l.parts, d.title]
  l.denied = d.tool
  l.setup = setup
  show(l, null)
}

// Агент просит показать лист: альбом не меняется, холст переходит к нему.
export function agentShow(s: AgentShow) {
  goWhen(s.spread)
}
