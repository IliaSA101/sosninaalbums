import type { SpreadView } from './types'

// Набор из ids; тот же объект prev, если состав не изменился (вид приходит новыми массивами на каждую правку).
export function keepSet(prev: Set<string>, ids: readonly string[]): Set<string> {
  if (prev.size === ids.length && ids.every((id) => prev.has(id))) return prev
  return new Set(ids)
}

// Фото «в альбоме» (075-16): набор id по листам. Пересчёт идёт по разворотам, чьи объекты сменились
// (дельта вида сохраняет неизменённые — viewdelta.mergeView), а не по всем 3200 фото на каждую правку.
// Набор неизменяемый: пока состав не изменился, возвращается тот же объект — зависимые списки не пересчитываются.
export class UsedIndex {
  private counts = new Map<string, number>()
  private prev: readonly SpreadView[] = []
  private out = new Set<string>()

  update(sheets: readonly SpreadView[]): Set<string> {
    if (sheets === this.prev) return this.out
    const now = new Set(sheets)
    const was = new Set(this.prev)
    // gone — ушедшие из набора; вернувшиеся в той же правке (разворот заменён на свою копию) из него вычёркиваются.
    const gone = new Set<string>()
    let added = false
    for (const s of was) if (!now.has(s)) this.drop(s, gone)
    for (const s of now) if (!was.has(s) && this.take(s, gone)) added = true
    this.prev = sheets
    if (added || gone.size > 0) this.out = new Set(this.counts.keys())
    return this.out
  }

  // photos от Go бывает null (пустой срез).
  private drop(s: SpreadView, gone: Set<string>) {
    for (const id of s.photos ?? []) {
      const c = (this.counts.get(id) ?? 0) - 1
      if (c > 0) {
        this.counts.set(id, c)
      } else {
        this.counts.delete(id)
        gone.add(id)
      }
    }
  }

  // true — в набор пришло фото, которого не было.
  private take(s: SpreadView, gone: Set<string>): boolean {
    let added = false
    for (const id of s.photos ?? []) {
      const c = this.counts.get(id) ?? 0
      this.counts.set(id, c + 1)
      if (c === 0 && !gone.delete(id)) added = true
    }
    return added
  }
}
