// Окно «Фото уже в альбоме» (098): чистые решения — тексты, строки «где стоит», ID для Go по ответу.
// Состояние и вызовы Go — copyask.svelte.ts; окно — shell/ask/CopyAskDialog.svelte.
import { baseOf, copyReq } from '../core/copies'
import type { UsedAt } from '../core/types'
import { lineOf, type UseLine } from '../words/whereuse'

export type CopyAnswer = 'cancel' | 'move' | 'copy'

export const WHERE_MAX = 5 // сколько мест перечислять в окне
export const ALSO_MAX = 3 // сколько прочих вхождений (фон, стикер…) в строке «Ещё» — окно не растёт

// Стоит ли файл в месте листа: «где стоит» от Go считает места всех копий (Places).
const placed = (u: UsedAt) => u.places > 0

// ID файлов без повторов, в порядке ids.
export function filesOf(ids: readonly string[]): string[] {
  return [...new Set(ids.map(baseOf))]
}

// Вхождения всех стоящих брошенных фото, по порядку Go; keep — какие брать. Вид — по kind, подпись не разбираем.
function usesOf(used: readonly UsedAt[], keep: (kind: string) => boolean): UseLine[] {
  return used.filter(placed).flatMap((u) => (u.uses ?? []).filter((x) => keep(x.kind)).map(lineOf))
}

// Места (не фоны, стикеры и рамочки: окно про «стоит в месте») всех брошенных фото, по порядку.
export const whereOf = (used: readonly UsedAt[]): UseLine[] => usesOf(used, (k) => k === 'place')

// Остальное, что тоже держит эти фото: фон, стикер, рамочка, фон альбома — для строки «Ещё».
export const alsoOf = (used: readonly UsedAt[]): UseLine[] => usesOf(used, (k) => k !== 'place')

export interface CopyAskText {
  title: string
  lead: string
  where: UseLine[]
  more: number // сколько мест не показано
  also: UseLine[] // ссылки «Ещё: …» (не места), до ALSO_MAX
  alsoMore: number // сколько из них не показано
  rest: string // про свободные фото; '' — все стоят
  move: boolean // «Перенести сюда» доступно
  hint: string // почему нет «Перенести сюда»; '' — кнопка есть
}

// Тексты окна: ids — что бросили, used — Go (Api.WhereUsed) по файлам этих ids.
export function copyAskText(ids: readonly string[], used: readonly UsedAt[]): CopyAskText {
  const files = filesOf(ids).length
  const some = used.filter(placed)
  const all = whereOf(used)
  const also = alsoOf(used)
  // Перенос — только если у каждого стоящего фото ровно одно место: иначе неясно, какое переносить.
  const move = some.length > 0 && some.every((u) => u.move !== '')
  return {
    title: 'Фото уже в альбоме',
    lead: files <= 1 ? 'Это фото уже стоит:' : `${some.length} из ${files} фото уже ${some.length === 1 ? 'стоит' : 'стоят'} в альбоме`,
    where: all.slice(0, WHERE_MAX),
    more: Math.max(0, all.length - WHERE_MAX),
    also: also.slice(0, ALSO_MAX),
    alsoMore: Math.max(0, also.length - ALSO_MAX),
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
