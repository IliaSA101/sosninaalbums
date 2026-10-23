// Окно «Фото уже в альбоме» (098): состояние, вызов Go (Api.WhereUsed) и ворота броска из ленты.
// Образец — DropAskDialog (042): вопрос задаётся до правки, «Отмена» и Esc — Go не зовём. Окно: shell/ask/CopyAskDialog.svelte.
import * as Api from '../../../wailsjs/go/app/App'
import { errText } from '../words/errtext'
import { store } from '../core/store.svelte'
import type { UsedAt, View } from '../core/types'
import { filesOf, sentIds, type CopyAnswer, type Where } from './copyask'

interface Open {
  ids: string[]
  used: UsedAt[]
  rev: number // правка из другого места (Ctrl+Z) делает вопрос неактуальным
  done: (a: CopyAnswer) => void
}

class CopyAskState {
  cur = $state<Open | null>(null)
  // Идёт запрос «где стоит»: второй вопрос не открываем (окно ещё не показано).
  busy = false

  prune(v: View) {
    if (this.cur && v.rev !== this.cur.rev) this.cur.done('cancel')
  }
}

export const copyAsk = new CopyAskState()

// Открыть вопрос и дождаться ответа; used — «где стоит» от Go (для ID по ответу). Ошибка Go — тост и «Отмена».
async function ask(ids: readonly string[]): Promise<{ answer: CopyAnswer; used: UsedAt[] }> {
  if (copyAsk.busy || copyAsk.cur) return { answer: 'cancel', used: [] }
  copyAsk.busy = true
  try {
    const used = ((await Api.WhereUsed(filesOf(ids))) ?? []) as unknown as UsedAt[]
    return await new Promise((resolve) => {
      const done = (answer: CopyAnswer) => {
        copyAsk.cur = null
        resolve({ answer, used })
      }
      copyAsk.cur = { ids: [...ids], used, rev: store.view.rev, done }
    })
  } catch (e) {
    store.notify(errText(e), 'error')
    return { answer: 'cancel', used: [] }
  } finally {
    copyAsk.busy = false
  }
}

// Вопрос для всех, кому нужно окно (бросок 098, вставка 099): ids — фото (ID файла или записи). Ответ — кнопка.
export async function askCopy(ids: string[]): Promise<CopyAnswer> {
  return (await ask(ids)).answer
}

export function answerCopy(a: CopyAnswer) {
  copyAsk.cur?.done(a)
}

// Строка «где стоит»: бросок отменён, лист открыт, место выделено.
export function showWhere(w: Where) {
  answerCopy('cancel')
  store.goSheet(w.sheet)
  store.selectedCell = w.ref
}

// Ворота броска из ленты (StoreHooks.gate): ничего из брошенного не стоит — ids как есть, без вопроса (решает
// UsedIndex, синхронно); иначе окно и ID для Go — запрос копии или запись переносимого места; null — «Отмена».
export async function gateCopy(ids: string[]): Promise<string[] | null> {
  if (!ids.some((id) => store.used.has(id))) return ids
  const { answer, used } = await ask(ids)
  return sentIds(ids, used, answer)
}
