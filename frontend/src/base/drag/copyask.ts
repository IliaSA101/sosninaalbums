// Окно «Фото уже в альбоме» (098): чистые решения — тексты, строки «где стоит», ID для Go по ответу.
// Состояние и вызовы Go — copyask.svelte.ts; окно — shell/ask/CopyAskDialog.svelte.
import { baseOf, copyReq } from '../core/copies'
import type { UsedAt } from '../core/types'

export type CopyAnswer = 'cancel' | 'move' | 'copy'

export const WHERE_MAX = 5 // сколько мест перечислять в окне

// Строка «где стоит»: лист (индекс в ряду холста, как у PhotoUse.sheet), ссылка и вид «место».
export interface Where {
  sheet: number
  link: string // «Разворот 3»
  kind: string // «место»
  ref: string // запись места — её выделяем при переходе
}

// Стоит ли файл в месте листа: «где стоит» от Go считает места всех копий (Places).
const placed = (u: UsedAt) => u.places > 0

// ID файлов без повторов, в порядке ids.
export function filesOf(ids: readonly string[]): string[] {
  return [...new Set(ids.map(baseOf))]
}

// Места (не фоны, стикеры и рамочки: окно про «стоит в месте») всех брошенных фото, по порядку.
export function whereOf(used: readonly UsedAt[]): Where[] {
  const out: Where[] = []
  for (const u of used.filter(placed)) {
    for (const x of u.uses ?? []) {
      if (x.sheet < 0) continue
      const i = x.label.indexOf(' — ')
      const kind = i < 0 ? '' : x.label.slice(i + 3)
      if (kind !== 'место') continue
      out.push({ sheet: x.sheet, link: x.label.slice(0, i), kind, ref: x.ref || u.id })
    }
  }
  return out
}

export interface CopyAskText {
  title: string
  lead: string
  where: Where[]
  more: number // сколько мест не показано
  rest: string // про свободные фото; '' — все стоят
  move: boolean // «Перенести сюда» доступно
  hint: string // почему нет «Перенести сюда»; '' — кнопка есть
}

// Тексты окна: ids — что бросили, used — Go (Api.WhereUsed) по файлам этих ids.
export function copyAskText(ids: readonly string[], used: readonly UsedAt[]): CopyAskText {
  const files = filesOf(ids).length
  const some = used.filter(placed)
  const all = whereOf(used)
  // Перенос — только если у каждого стоящего фото ровно одно место: иначе неясно, какое переносить.
  const move = some.length > 0 && some.every((u) => u.move !== '')
  return {
    title: 'Фото уже в альбоме',
    lead: files <= 1 ? 'Это фото уже стоит:' : `${some.length} из ${files} фото уже ${some.length === 1 ? 'стоит' : 'стоят'} в альбоме`,
    where: all.slice(0, WHERE_MAX),
    more: Math.max(0, all.length - WHERE_MAX),
    rest: files > some.length ? 'Остальные фото встанут в любом случае.' : '',
    move,
    hint: !move && some.length > 0 ? 'Перенести одно из мест — перетащите его на холсте.' : '',
  }
}

// ID для Go по ответу: стоящее фото — запрос копии «<файл>-new» или запись его единственного места,
// свободное — как есть. «Отмена» сюда не доходит (null — правки не будет).
export function sentIds(ids: readonly string[], used: readonly UsedAt[], answer: CopyAnswer): string[] | null {
  if (answer === 'cancel') return null
  const at = new Map(used.filter(placed).map((u) => [u.id, u]))
  return ids.map((id) => {
    const u = at.get(baseOf(id))
    if (!u) return id
    return answer === 'copy' ? copyReq(u.id) : u.move || id
  })
}
