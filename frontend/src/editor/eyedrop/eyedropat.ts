import type { Pt } from '../../base/geom/textbox'

// Чистое для пипетки на холсте (109, design §7). Геометрию листа считает Go; здесь только перевод точки экрана
// в мм листа (k = spread.w / rect.width, как в StraightenLayer) и очередь запросов лупы.

type Box = { left: number; top: number; width: number; height: number }

// Точка экрана → мм листа; null — точка вне слоя (в Go такой запрос дал бы «точка вне листа»).
export function sheetPoint(clientX: number, clientY: number, r: Box, sheetW: number): Pt | null {
  if (r.width <= 0 || r.height <= 0) return null
  const x = clientX - r.left
  const y = clientY - r.top
  if (x < 0 || y < 0 || x >= r.width || y >= r.height) return null
  const k = sheetW / r.width
  return { x: x * k, y: y * k }
}

export const LOUPE = 46 // диаметр круга, px
const GAP = 14 // зазор до курсора
const HEIGHT = 70 // круг + подпись HEX под ним
const PAD = 4 // не ближе к краю окна

// Где стоит лупа (fixed, px экрана): справа-сверху от курсора, как в макете; у правого края — слева,
// у верхнего — под курсором, у нижнего — прижата к низу окна.
export function loupeAt(x: number, y: number, vw: number, vh: number): { left: number; top: number } {
  let left = x + GAP
  if (left + LOUPE + PAD > vw) left = x - GAP - LOUPE
  let top = y - 60
  if (top < PAD) top = y + GAP
  if (top + HEIGHT + PAD > vh) top = Math.max(PAD, vh - HEIGHT - PAD)
  return { left: Math.max(PAD, left), top }
}

// Очередь запросов лупы: в пути один запрос; пока он идёт, новые точки вытесняют друг друга — следующим уходит последняя.
export function pointQueue<T>(ask: (p: Pt) => Promise<T>, done: (p: Pt, r: T) => void, fail: (e: unknown) => void) {
  let busy = false
  let next: Pt | null = null
  let dead = false
  async function run(p: Pt): Promise<void> {
    busy = true
    try {
      const r = await ask(p)
      if (!dead) done(p, r)
    } catch (e) {
      if (!dead) fail(e)
    }
    busy = false
    const n = next
    next = null
    if (n && !dead) await run(n)
  }
  return {
    push(p: Pt): void {
      if (dead) return
      if (busy) next = p
      else void run(p)
    },
    // Слой закрыт: ответы запросов в пути отбрасываются, очередь пуста.
    stop(): void {
      dead = true
      next = null
    },
  }
}
