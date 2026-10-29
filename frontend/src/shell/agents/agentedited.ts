import { store } from '../../base/core/store.svelte'
import { undoToast } from '../../base/core/toasts.svelte'
import type { AgentEdited, AgentShow } from './agents'

// Тосты правок агента (149-30, design §4). Текст приходит из Go («Собрано разворотов: 24»), здесь — имя клиента,
// «Вернуть» (механизм undoToast: только при той же ревизии) и переход к развороту.
export const GLUE_MS = 5000 // тосты одного клиента за это время склеиваются в один (М5)
const GLUE_MAX = 3 // в склейке не больше стольких последних правок: тост не должен разрастаться
const GO_TRIES = 10 // разворот мог ещё не дойти до интерфейса событием view: ждём до секунды
const GO_WAIT = 100

let last: { client: string; at: number; parts: string[]; text: string } | null = null

// Для тестов: забыть прошлый тост.
export function resetEdited() {
  last = null
}

// Перейти к листу, когда он появится в виде (view приходит событием, обгоняющим или догоняющим agents:*).
function goWhen(id: string, tries = GO_TRIES) {
  if (!id) return
  if (store.sheet(id)) return store.goId(id)
  if (tries > 0) setTimeout(() => goWhen(id, tries - 1), GO_WAIT)
}

// Агент изменил альбом: тост «Клиент · что сделано» с «Вернуть» и переход к `go`. rev — ревизия после правки: на «Вернуть»
// сверяется нынешняя (иначе «Уже были другие изменения»), другая правка гасит тост сама (Store.apply по toast.rev).
export function agentEdited(e: AgentEdited) {
  const now = Date.now()
  // Go шлёт view с новой ревизией раньше agents:edited, и Store.apply гасит прежний тост агента (toast.rev старый).
  // Поэтому склеиваем и когда тост погашен сменой ревизии (toast == null), и когда он ещё тот же; чужой тост склейку рвёт.
  const same = !!last && last.client === e.client && now - last.at < GLUE_MS && (store.toast == null || store.toast.text === last.text)
  const parts = same && last ? [...last.parts, e.text].slice(-GLUE_MAX) : [e.text]
  const text = `${e.client} · ${parts.join('. ')}`
  last = { client: e.client, at: now, parts, text }
  if (store.view.rev > e.rev) {
    store.notify(text) // после правки агента уже были другие: «Вернуть» откатило бы не её
  } else {
    undoToast(store, text, undefined, e.rev) // ревизия — правки агента: view мог ещё не дойти, тогда сверка при нажатии
  }
  goWhen(e.go)
}

// Агент просит показать лист: альбом не меняется, холст переходит к нему.
export function agentShow(s: AgentShow) {
  goWhen(s.spread)
}
