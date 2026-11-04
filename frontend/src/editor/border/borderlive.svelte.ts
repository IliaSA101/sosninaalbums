// Живой показ рамочки и прозрачности фото (079 U3): пока тянут ползунок карточки, ячейка рисуется по этому значению,
// а не по View; Go зовётся на отпускании. Образец — fadeLive (017). Рамочка-накладка (PNG от Go) на ходу прежняя:
// живыми бывают окно фото (отступы растут и сжимаются вместе с толщиной) и прозрачность.
import type { BorderView } from '../../base/core/types'

interface Live {
  spread: string
  ids: string[] // ID мест (толщина, размер) или фото (прозрачность), которых касается жест
  width?: number // толщина встроенных, мм
  scale?: number // размер картинок, 1 — как задумано
  fade?: number // прозрачность фото, 0…1
}

// Отступы окна, пропорциональные толщине (встроенная) или размеру (картинка): те же числа у Go считаются как
// Meta.Ins × толщина и k × срез, так что отношение «новое/текущее» переносит отступы без знания вида.
// Зажим пары сторон делает insetBox при расчёте окна.
export function scaledIns(b: BorderView, width: number | undefined, scale: number | undefined): number[] {
  const to = b.pic ? scale : width
  const from = b.pic ? b.scale : b.width
  if (to === undefined || !(from > 0) || !(to > 0)) return b.ins
  return b.ins.map((v) => (v * to) / from)
}

export class BorderLive {
  cur = $state<Live | null>(null)
  // Очередь вызовов Go: удержанная стрелка шлёт много change, порядок ответов иначе не гарантирован.
  private chain: Promise<unknown> = Promise.resolve()
  private pending = 0

  set(live: Live) {
    this.cur = live
  }

  // Просмотр снимается, только когда нет вызовов в пути: иначе ячейка мигнёт старым значением.
  release() {
    if (this.pending === 0) this.cur = null
  }

  commit(run: () => Promise<unknown>): Promise<unknown> {
    this.pending++
    const p = this.chain.then(run, run)
    this.chain = p.catch(() => {})
    return this.chain.finally(() => {
      this.pending--
      this.release()
    })
  }

  private of(spread: string, refs: readonly (string | undefined)[]): Live | null {
    const c = this.cur
    return c && c.spread === spread && c.ids.some((x) => refs.includes(x)) ? c : null
  }

  // Прозрачность фото id: просмотр, если он про него, иначе значение из View.
  fadeOf(spread: string, photo: string, fade: number): number {
    const c = this.of(spread, [photo])
    return c?.fade !== undefined ? c.fade : fade
  }

  // Отступы места на ходу или undefined, если ползунок толщины/размера его не касается (окно из Go).
  insOf(spread: string, refs: readonly (string | undefined)[], b: BorderView): number[] | undefined {
    const c = this.of(spread, refs)
    if (!c || (c.width === undefined && c.scale === undefined)) return undefined
    return scaledIns(b, c.width, c.scale)
  }
}

export const borderlive = new BorderLive()
