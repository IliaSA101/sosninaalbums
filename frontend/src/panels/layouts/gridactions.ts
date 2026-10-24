// Вызовы Go для галереи сеток и копирования (спека 007, U9): ответ — в store.take, ошибка — тост.
import { store, Api } from '../../base/core/store.svelte'
import { errText } from '../../base/words/errtext'
import { plural } from '../../base/words/plural'
import { unitOf } from '../../base/geom/sheet'
import { tplError } from '../gallery/gridgallery'
import type { GridChoice, GridPlan, GridResult, GridTarget, LayoutScope } from '../../base/core/types'

// Тост с «Вернуть» (= Undo): безопасен, только пока после правки ничего не менялось.
export function notifyUndo(text: string) {
  store.undoToast(text)
}

const leftText = (n: number) => `${n} ${plural(n, ['фото вернулось', 'фото вернулись', 'фото вернулись'])} в ленту`

// Клик по карточке: сетка встаёт на область scope.
export async function applyChoice(spread: string, scope: LayoutScope, ch: GridChoice) {
  try {
    const r = (await Api.ApplyGrid(spread, scope, ch.scope, ch.boxes)) as unknown as GridResult
    if (!(await store.take(r.view))) return // сбой GetView: красный тост цепочки уже показан, синий поверх не нужен (087)
    if (r.left > 0) notifyUndo(leftText(r.left))
  } catch (e) {
    store.notify(errText(e), 'error')
  }
}

// Правка шаблонов: true — выполнено. Ошибка «новее программы» объясняется понятным текстом.
export async function tplCall(p: Promise<unknown>): Promise<boolean> {
  try {
    await p
    return true
  } catch (e) {
    store.notify(tplError(errText(e)), 'error')
    return false
  }
}

export const planCopy = (src: string, from: LayoutScope, to: GridTarget[], design: boolean, mirror: boolean) =>
  Api.PlanCopyGrid(src, from, to, design, mirror) as unknown as Promise<GridPlan>

// Копирование: итог — тост; left > 0 — «N фото вернулись в ленту» с «Вернуть».
export async function runCopy(
  src: string,
  from: LayoutScope,
  to: GridTarget[],
  design: boolean,
  mirror: boolean,
  n: number,
): Promise<boolean> {
  try {
    const r = (await Api.CopyGrid(src, from, to, design, mirror)) as unknown as GridResult
    if (!(await store.take(r.view))) return false
    const head = `Раскладка скопирована на ${n} ${plural(n, unitOf(store.view.settings).plural)}`
    if (r.left > 0) notifyUndo(`${head}. ${leftText(r.left)}`)
    else store.notify(head)
    return true
  } catch (e) {
    store.notify(errText(e), 'error')
    return false
  }
}

// Новые пустые развороты с местами источника после него.
export async function runDuplicate(src: string, count: number, design: boolean): Promise<boolean> {
  const ok = await store.run(Api.DuplicateGrid(src, count, design))
  if (ok) store.notify(`Добавлено ${count} ${plural(count, ['разворот', 'разворота', 'разворотов'])} с такой раскладкой`)
  return ok
}

// «Копировать оформление» (049 Ф2, design §8): Api.PlanCopyDecor / Api.CopyDecor, последние аргументы — texts, mirror.
export const planCopyDecor = (src: string, from: LayoutScope, to: GridTarget[], texts: boolean, mirror: boolean) =>
  Api.PlanCopyDecor(src, from, to, texts, mirror) as unknown as Promise<GridPlan>

// Один вызов окна «Копировать раскладку» / «Копировать оформление»: decor выбирает метод Go, остальное общее.
export interface CopyCall {
  decor: boolean
  src: string
  from: LayoutScope
  to: GridTarget[]
  design: boolean // «С текстом и фоном» (раскладка)
  texts: boolean // «С надписями» (оформление)
  mirror: boolean // уже с учётом «источник — страница»
}

export const planFor = (c: CopyCall) =>
  c.decor ? planCopyDecor(c.src, c.from, c.to, c.texts, c.mirror) : planCopy(c.src, c.from, c.to, c.design, c.mirror)

export const runFor = (c: CopyCall, n: number) =>
  c.decor ? runCopyDecor(c.src, c.from, c.to, c.texts, c.mirror, n) : runCopy(c.src, c.from, c.to, c.design, c.mirror, n)

// Итог — тост. Версия не выросла — Go ничего не менял (у всех целей уже то же оформление): шага отмены нет.
export async function runCopyDecor(
  src: string,
  from: LayoutScope,
  to: GridTarget[],
  texts: boolean,
  mirror: boolean,
  n: number,
): Promise<boolean> {
  try {
    const before = store.view.rev
    const r = (await Api.CopyDecor(src, from, to, texts, mirror)) as unknown as GridResult
    if (!(await store.take(r.view))) return false
    if (store.view.rev === before) store.notify('У выбранных уже такое же оформление: копировать нечего')
    else store.notify(`Оформление скопировано на ${n} ${plural(n, unitOf(store.view.settings).plural)}`)
    return true
  } catch (e) {
    store.notify(errText(e), 'error')
    return false
  }
}
