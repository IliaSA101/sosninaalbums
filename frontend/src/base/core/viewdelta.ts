import type { SpreadView, View } from './types'

// Склейка дельты вида (075): Go шлёт шапку целиком и только изменённые развороты, порядок — в order.
// Неизменённые развороты берутся из прежнего вида теми же объектами (ключевые {#each} их не перерисуют).
// Разворота нет ни в дельте, ни в прежнем виде — null: дельта пришла не к тому виду, нужен полный (GetView).
// Паритет с Go — testdata/parity/viewdelta.json.
export function mergeView(prev: View, d: View): View | null {
  const order = d.order
  if (!order) return null
  const fresh = new Map<string, SpreadView>()
  for (const s of d.spreads ?? []) fresh.set(s.id, s)
  const old = new Map<string, SpreadView>()
  for (const s of prev.spreads) old.set(s.id, s)
  const spreads: SpreadView[] = []
  for (const id of order) {
    const s = fresh.get(id) ?? old.get(id)
    if (!s) return null
    spreads.push(s)
  }
  const out: View = { ...d, spreads }
  delete out.base
  delete out.order
  return out
}

// Вид — $state.raw (075-16): вложенное на месте не меняют, правка вида — только заменой целиком. В dev и тестах
// вид замораживается (уже замороженные неизменённые развороты пропускаются), и запись «на месте» падает сразу, а не
// молча теряет реактивность.
export function deepFreeze<T>(o: T): T {
  if (o && typeof o === 'object' && !Object.isFrozen(o)) {
    Object.freeze(o)
    for (const v of Object.values(o)) deepFreeze(v)
  }
  return o
}

// Что делать с пришедшим видом, если наш последний номер цепочки — seq (design 075 §1.4).
export type Admit = { kind: 'skip' } | { kind: 'resync'; seq: number } | { kind: 'set'; view: View }

export function admit(cur: View, seq: number, v: View): Admit {
  const n = v.seq ?? 0
  if (v.base && v.base > 0) {
    if (n <= seq) return { kind: 'skip' } // дубль: событие и ответ — один вид
    if (v.base !== seq) return { kind: 'resync', seq: n }
    const m = mergeView(cur, v)
    return m ? { kind: 'set', view: m } : { kind: 'resync', seq: n }
  }
  if (n > 0 && n <= seq) return { kind: 'skip' }
  return { kind: 'set', view: v }
}
