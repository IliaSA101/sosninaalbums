import type { SpreadView } from './types'

// Какой разворот текущий после правки (store.apply). Исчезнувший текущий запоминаем в lost:
// если Undo его вернёт, текущим становится он, а не сосед, на котором мы остались.
export interface CurrentPick {
  index: number // индекс текущего разворота
  id: string
  lost: string // всё ещё ожидаемый вернувшийся id ('' — ждать нечего)
  back: number // индекс вернувшегося разворота, -1 — не вернулся
}

export function pickCurrent(ids: readonly string[], currentId: string, index: number, lost: string): CurrentPick {
  const i = ids.indexOf(currentId)
  const wait = i < 0 && currentId ? currentId : lost
  // Разворот остался — идём за ним; удалён — остаёмся на том же индексе.
  const cur = i >= 0 ? i : Math.max(0, Math.min(index, ids.length - 1))
  const back = wait ? ids.indexOf(wait) : -1
  return { index: cur, id: ids[cur] ?? '', lost: back >= 0 ? '' : wait, back }
}

// Склейка двух разворотов оставляет фоны первого, а без них — второго (Go, MergeSpreads): фон второго
// пропадает, когда у обоих есть фоны. Тогда UI предлагает «Вернуть».
export function backdropLost(first: SpreadView | undefined, second: SpreadView | undefined): boolean {
  return !!first?.backdrops?.length && !!second?.backdrops?.length
}
