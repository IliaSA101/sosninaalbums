import { describe, expect, it } from 'vitest'
import { tiles, type Cell } from './tiles'

// Сетка ячеек окна сравнения (138-6): ряд, столбец, 2×2; ячейки не перекрываются и лежат в окне.
const GAP = 8
const CAP = 36
const H = 3 / 2 // горизонтальный кадр 3:2
const V = 2 / 3 // вертикальный
const wide = { w: 1280, h: 700 }
const narrow = { w: 1024, h: 700 }
const tall = { w: 500, h: 900 }

const cols = (c: Cell[]) => new Set(c.map((x) => Math.round(x.left))).size
const rows = (c: Cell[]) => new Set(c.map((x) => Math.round(x.top))).size
// Прямоугольник ячейки с подписью (высота +CAP) — он и не должен перекрываться.
const boxes = (c: Cell[]) => c.map((x) => ({ l: x.left, t: x.top, r: x.left + x.w, b: x.top + x.h + CAP }))

function inside(c: Cell[], box: { w: number; h: number }) {
  for (const b of boxes(c)) {
    expect(b.l).toBeGreaterThanOrEqual(-1e-6)
    expect(b.t).toBeGreaterThanOrEqual(-1e-6)
    expect(b.r).toBeLessThanOrEqual(box.w + 1e-6)
    expect(b.b).toBeLessThanOrEqual(box.h + 1e-6)
  }
}
function apart(c: Cell[]) {
  const b = boxes(c)
  for (let i = 0; i < b.length; i++)
    for (let j = i + 1; j < b.length; j++) {
      const sep = b[i].r <= b[j].l + 1e-6 || b[j].r <= b[i].l + 1e-6 || b[i].b <= b[j].t + 1e-6 || b[j].b <= b[i].t + 1e-6
      expect(sep, `${i} и ${j} перекрываются`).toBe(true)
    }
}
function check(aspects: number[], box: { w: number; h: number }): Cell[] {
  const c = tiles(aspects, box, GAP, CAP)
  expect(c).toHaveLength(aspects.length)
  inside(c, box)
  apart(c)
  return c
}

describe('1 кадр', () => {
  it('занимает всё окно за вычетом подписи', () => {
    const [c] = check([H], wide)
    expect(c).toEqual({ left: 0, top: 0, w: 1280, h: 700 - CAP })
  })
  it('нет кадров — нет ячеек', () => {
    expect(tiles([], wide, GAP, CAP)).toEqual([])
  })
})

describe('2 кадра', () => {
  it('два горизонтальных в широком окне — ряд', () => {
    const c = check([H, H], wide)
    expect(rows(c)).toBe(1)
    expect(cols(c)).toBe(2)
  })
  it('два вертикальных — ряд', () => {
    const c = check([V, V], wide)
    expect(rows(c)).toBe(1)
    expect(cols(c)).toBe(2)
  })
  it('два горизонтальных в узком высоком окне — столбец', () => {
    const c = check([H, H], tall)
    expect(rows(c)).toBe(2)
    expect(cols(c)).toBe(1)
  })
  it('1024×700: два горизонтальных — всё ещё ряд, порядок слева направо', () => {
    const c = check([H, H], narrow)
    expect(rows(c)).toBe(1)
    expect(c[0].left).toBeLessThan(c[1].left)
  })
})

describe('3 кадра', () => {
  it('смешанные пропорции: размеры ячеек равны, кадры в порядке чтения', () => {
    const c = check([H, V, H], wide)
    for (const x of c) {
      expect(x.w).toBeCloseTo(c[0].w, 6)
      expect(x.h).toBeCloseTo(c[0].h, 6)
    }
    const order = c.map((x) => x.top * 10000 + x.left)
    expect(order).toEqual([...order].sort((a, b) => a - b))
  })
  it('в высоком окне 2+1: последний ряд по центру окна', () => {
    const c = check([H, H, H], { w: 800, h: 900 })
    expect(rows(c)).toBe(2)
    expect(c[0].top).toBe(c[1].top)
    expect(c[2].left + c[2].w / 2).toBeCloseTo(400, 6)
  })
  it('три вертикальных в широком окне — один ряд', () => {
    const c = check([V, V, V], wide)
    expect(rows(c)).toBe(1)
    expect(cols(c)).toBe(3)
  })
})

describe('4 кадра', () => {
  it('горизонтальные в 1280×700 — 2×2', () => {
    const c = check([H, H, H, H], wide)
    expect(rows(c)).toBe(2)
    expect(cols(c)).toBe(2)
  })
  it('1024×700 — 2×2, ячейки лежат в окне', () => {
    const c = check([H, V, H, V], narrow)
    expect(rows(c)).toBe(2)
    expect(cols(c)).toBe(2)
  })
  it('четыре вертикальных в очень широком окне — один ряд', () => {
    const c = check([V, V, V, V], { w: 2400, h: 600 })
    expect(rows(c)).toBe(1)
    expect(cols(c)).toBe(4)
  })
  it('четыре горизонтальных в узкой высокой — столбец', () => {
    const c = check([H, H, H, H], { w: 420, h: 1400 })
    expect(cols(c)).toBe(1)
    expect(rows(c)).toBe(4)
  })
})

describe('края', () => {
  it('неизвестная или мусорная пропорция — как квадрат, ячейки есть', () => {
    const c = check([0, NaN, -1, Infinity], wide)
    expect(c.every((x) => x.w > 0 && x.h > 0)).toBe(true)
  })
  it('окно меньше подписи и отступов: нулевые размеры, не отрицательные', () => {
    const c = tiles([H, H, H, H], { w: 10, h: 20 }, GAP, CAP)
    expect(c).toHaveLength(4)
    expect(c.every((x) => x.w >= 0 && x.h >= 0)).toBe(true)
  })
  it('самый мелкий кадр получает наибольшую площадь: ряд против столбца для вертикального и горизонтального', () => {
    const area = (a: number, c: Cell) => (a > c.w / c.h ? c.w * (c.w / a) : c.h * a * c.h)
    const c = tiles([H, V], wide, GAP, CAP)
    const best = Math.min(area(H, c[0]), area(V, c[1]))
    // Столбец из тех же двух кадров не лучше.
    const col = { w: wide.w, h: (wide.h - GAP) / 2 - CAP }
    expect(best).toBeGreaterThanOrEqual(Math.min(area(H, { ...col, left: 0, top: 0 }), area(V, { ...col, left: 0, top: 0 })) - 1e-6)
  })
})
