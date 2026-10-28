import { describe, expect, it } from 'vitest'
import { gapAt, inside, landOf, offsetOf, shiftOf, type Mode, type Slot } from './reorder'

const SP = 14 // черта склейки между группами ленты

// Лента: группы подряд с зазором SP, начало — x0.
function line(widths: number[], x0 = 30): Slot[] {
  let x = x0
  return widths.map((w) => {
    const s = { x, y: 8, w, h: 80 }
    x += w + SP
    return s
  })
}

// Сетка: cols в ряд, ячейка w×h, зазоры gx/gy.
function grid(n: number, cols = 3, w = 112, h = 90, gx = 8, gy = 4): Slot[] {
  return Array.from({ length: n }, (_, k) => ({ x: (k % cols) * (w + gx), y: Math.floor(k / cols) * (h + gy), w, h }))
}

// Порядок после MoveSpread(from → to): удалить и вставить.
function moved(n: number, from: number, to: number): number[] {
  const order = Array.from({ length: n }, (_, k) => k)
  const [x] = order.splice(from, 1)
  order.splice(to, 0, x)
  return order
}

describe('gapAt', () => {
  it('gapAt: лента — по серединам, разной ширины', () => {
    const s = line([100, 40, 200, 60]) // середины: 80, 164, 298, 442
    expect(gapAt(s, 0, 160, 40, 'line')).toBe(0)
    expect(gapAt(s, 0, 170, 40, 'line')).toBe(1)
    expect(gapAt(s, 0, 300, 40, 'line')).toBe(2)
    expect(gapAt(s, 0, 450, 40, 'line')).toBe(3)
    expect(gapAt(s, 3, 79, 40, 'line')).toBe(0)
    expect(gapAt(s, 3, 81, 40, 'line')).toBe(1)
    // высота точки в ленте не важна
    expect(gapAt(s, 3, 81, -500, 'line')).toBe(1)
  })

  it('gapAt: сетка — порядок чтения, между рядами — следующий ряд', () => {
    const s = grid(7) // ряды y 0, 94, 188; середины x 56, 176, 296
    expect(gapAt(s, 0, 180, 40, 'grid')).toBe(1) // ряд 0 правее середины слота 1: до точки только он
    expect(gapAt(s, 6, 180, 40, 'grid')).toBe(2)
    expect(gapAt(s, 6, 10, 120, 'grid')).toBe(3) // начало второго ряда
    expect(gapAt(s, 6, 200, 92, 'grid')).toBe(3) // в зазоре между рядами 0 и 1 — первый слот ряда 1
    expect(gapAt(s, 6, 350, 92, 'grid')).toBe(3)
    expect(gapAt(s, 0, 350, 150, 'grid')).toBe(5) // правее всего ряда 1
  })

  it('gapAt: своё место', () => {
    const l = line([100, 40, 200])
    expect(gapAt(l, 1, l[1].x + 20, 40, 'line')).toBe(1)
    const g = grid(6)
    expect(gapAt(g, 4, g[4].x + 60, g[4].y + 10, 'grid')).toBe(4)
  })

  it('gapAt: выше всех — 0, ниже всех — n − 1', () => {
    const g = grid(8)
    expect(gapAt(g, 3, 300, -50, 'grid')).toBe(0)
    expect(gapAt(g, 3, 0, 1000, 'grid')).toBe(7)
    const l = line([50, 50, 50])
    expect(gapAt(l, 1, -100, 0, 'line')).toBe(0)
    expect(gapAt(l, 1, 5000, 0, 'line')).toBe(2)
  })

  it('один слот — всегда своё место', () => {
    for (const mode of ['line', 'grid'] as Mode[]) {
      const s = mode === 'line' ? line([120]) : grid(1)
      for (const [x, y] of [
        [-100, -100],
        [60, 40],
        [900, 900],
      ])
        expect(gapAt(s, 0, x, y, mode)).toBe(0)
      expect(offsetOf(s, 0, 0, 0, mode)).toEqual({ dx: 0, dy: 0 })
      expect(landOf(s, 0, 0, mode)).toEqual({ dx: 0, dy: 0 })
    }
  })
})

it('shiftOf', () => {
  // from 1 → to 4: 2..4 на позицию назад
  expect([0, 1, 2, 3, 4, 5].map((k) => shiftOf(k, 1, 4))).toEqual([0, 0, -1, -1, -1, 0])
  // from 4 → to 1: 1..3 на позицию вперёд
  expect([0, 1, 2, 3, 4, 5].map((k) => shiftOf(k, 4, 1))).toEqual([0, 1, 1, 1, 0, 0])
  expect([0, 1, 2].map((k) => shiftOf(k, 1, 1))).toEqual([0, 0, 0])
})

describe('раскладка', () => {
  // Детерминированный генератор: тест повторяем.
  let seed = 7
  const rnd = () => {
    seed = (seed * 1103515245 + 12345) % 2147483648
    return seed / 2147483648
  }

  it('offsetOf и landOf совпадают с раскладкой после MoveSpread (лента и сетка)', () => {
    for (let c = 0; c < 400; c++) {
      const n = 1 + Math.floor(rnd() * 12)
      const from = Math.floor(rnd() * n)
      const to = Math.floor(rnd() * n)
      const order = moved(n, from, to)
      // лента: новая раскладка — заново суммой ширин и зазоров
      const ws = Array.from({ length: n }, () => 20 + Math.round(rnd() * 300))
      const s = line(ws)
      const after = line(order.map((k) => ws[k]))
      order.forEach((k, pos) => {
        const off = k === from ? landOf(s, from, to, 'line') : offsetOf(s, k, from, to, 'line')
        expect(s[k].x + off.dx).toBe(after[pos].x)
        expect(off.dy).toBe(0)
      })
      // сетка: позиция = индекс
      const g = grid(n)
      order.forEach((k, pos) => {
        const off = k === from ? landOf(g, from, to, 'grid') : offsetOf(g, k, from, to, 'grid')
        expect([g[k].x + off.dx, g[k].y + off.dy]).toEqual([g[pos].x, g[pos].y])
      })
    }
  })

  it('gapAt у середины целевого слота даёт ожидаемый to', () => {
    const ws = [100, 40, 200, 60, 90]
    const s = line(ws)
    for (let from = 0; from < ws.length; from++)
      for (let to = 0; to < ws.length; to++) {
        if (to === from) continue
        // за серединой слота to в сторону движения
        const t = s[to]
        const px = to > from ? t.x + t.w / 2 + 1 : t.x + t.w / 2 - 1
        expect(gapAt(s, from, px, 40, 'line')).toBe(to)
      }
  })
})

it('inside: 60 px от видимой части', () => {
  const a = { left: 100, top: 500, right: 1100, bottom: 600 }
  expect(inside(a, 500, 550)).toBe(true)
  expect(inside(a, 500, 440)).toBe(true) // 60 px выше — ещё внутри
  expect(inside(a, 500, 439)).toBe(false)
  expect(inside(a, 500, 660)).toBe(true)
  expect(inside(a, 500, 661)).toBe(false)
  expect(inside(a, 39, 550)).toBe(false)
  expect(inside(a, 1160, 550)).toBe(true)
  expect(inside(a, 1161, 550)).toBe(false)
  expect(inside(a, 500, 400, 200)).toBe(true)
})
