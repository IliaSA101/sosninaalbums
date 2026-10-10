import { describe, expect, it } from 'vitest'
import {
  GAP,
  MAX_SLOTS,
  dur,
  easeOut,
  jumpFrom,
  nearSlot,
  pitch,
  renderRange,
  rowScroll,
  rowWheel,
  rubber,
  slotLeft,
  snapTarget,
  stepTarget,
} from './spreadrow'

const W = 600
const P = pitch(W)
const wh = (deltaX: number, deltaY: number, deltaMode = 0) => ({ deltaX, deltaY, deltaMode, ctrlKey: false })

describe('slotLeft', () => {
  it('соседи на шаге W+G от текущего', () => {
    expect(P).toBe(W + GAP)
    expect(slotLeft(4, 3, 100, P) - slotLeft(3, 3, 100, P)).toBe(W + GAP)
    expect(slotLeft(3, 3, 100, P) - slotLeft(2, 3, 100, P)).toBe(W + GAP)
  })
  it('при pos=cur текущий ровно на fitPlace.left', () => {
    expect(slotLeft(5, 5, 137, P)).toBe(137)
    expect(slotLeft(5, 5.5, 137, P)).toBe(137 - P / 2)
  })
})

describe('renderRange', () => {
  it('видимые плюс один с каждой стороны, в пределах 0..max, не больше 13', () => {
    // окно 1200: видны слоты с |i − 5| < (1200+600)/(2·624) ≈ 1,44 → 4..6, плюс 3 и 7
    expect(renderRange(5, 20, 20, 1200, W, P, false, 5)).toEqual([3, 4, 5, 6, 7])
    expect(renderRange(0, 20, 20, 1200, W, P, false, 0)).toEqual([0, 1, 2])
    expect(renderRange(20, 20, 20, 1200, W, P, false, 19)).toEqual([18, 19, 20])
    const wide = renderRange(10, 30, 30, 20000, 100, 124, false, 10)
    expect(wide.length).toBeLessThanOrEqual(MAX_SLOTS)
    expect(wide).toContain(10)
    expect(Math.min(...wide)).toBeGreaterThanOrEqual(0)
    expect(Math.max(...wide)).toBeLessThanOrEqual(30)
    // текущий входит всегда, даже когда ряд уехал далеко
    const far = renderRange(15, 30, 30, 1200, W, P, false, 2)
    expect(far).toContain(2)
    expect(far).toContain(15)
    expect(far.length).toBeLessThanOrEqual(MAX_SLOTS)
  })
  it('в зуме — только текущий', () => {
    expect(renderRange(5, 20, 20, 1200, W, P, true, 5)).toEqual([5])
    expect(renderRange(5.4, 20, 20, 1200, W, P, true, 5)).toEqual([5])
  })
})

describe('snapTarget', () => {
  it('меньше 0,2 — обратно', () => {
    expect(snapTarget(3, 3.19, 0, 10, P)).toBe(3)
    expect(snapTarget(3, 2.85, 0, 10, P)).toBe(3)
  })
  it('0,2 и больше — следующий по направлению', () => {
    expect(snapTarget(3, 3.25, 0, 10, P)).toBe(4)
    expect(snapTarget(3, 2.7, 0, 10, P)).toBe(2)
    expect(snapTarget(3, 4.1, 0, 10, P)).toBe(4) // почти на слот — на слот
    expect(snapTarget(3, 4.3, 0, 10, P)).toBe(5) // уехали дальше слота
    expect(snapTarget(3, 1.6, 0, 10, P)).toBe(1)
  })
  it('бросок по скорости — следующий', () => {
    const v = 0.5 / P // 0,5 px/мс
    expect(snapTarget(3, 3.05, v, 10, P)).toBe(4)
    expect(snapTarget(3, 2.95, -v, 10, P)).toBe(2)
    expect(snapTarget(3, 3.6, v, 10, P)).toBe(4)
  })
  it('не за 0 и не за max', () => {
    expect(snapTarget(0, -0.4, 0, 5, P)).toBe(0)
    expect(snapTarget(0, 0.1, -1, 5, P)).toBe(0)
    expect(snapTarget(5, 5.4, 0, 5, P)).toBe(5)
    expect(snapTarget(5, 4.9, 1, 5, P)).toBe(5)
  })
})

describe('rubber', () => {
  it('за краем сопротивление, не дальше 0,3 слота', () => {
    expect(rubber(2, 5)).toBe(2)
    expect(rubber(-0.2, 5)).toBeCloseTo(-0.07, 10)
    expect(rubber(5.2, 5)).toBeCloseTo(5.07, 10)
    expect(rubber(-3, 5)).toBe(-0.3)
    expect(rubber(9, 5)).toBe(5.3)
  })
})

describe('jumpFrom', () => {
  it('дальше 2 — старт за 1 слот до цели с нужной стороны', () => {
    expect(jumpFrom(0, 10)).toBe(9)
    expect(jumpFrom(10, 0)).toBe(1)
    expect(jumpFrom(2.5, 5.6)).toBeCloseTo(4.6, 10)
  })
  it('близко — с текущего места', () => {
    expect(jumpFrom(3, 5)).toBe(3)
    expect(jumpFrom(5, 3)).toBe(5)
    expect(jumpFrom(3.4, 3)).toBe(3.4)
  })
})

describe('rowWheel', () => {
  it('щелчок мыши по X или Y (целый, ≥ 40) — шаг', () => {
    expect(rowWheel(wh(0, 100))).toEqual({ kind: 'step', dir: 1 })
    expect(rowWheel(wh(0, -120))).toEqual({ kind: 'step', dir: -1 })
    expect(rowWheel(wh(100, 0))).toEqual({ kind: 'step', dir: 1 }) // Shift+колесо
    expect(rowWheel(wh(-100, 0))).toEqual({ kind: 'step', dir: -1 })
  })
  it('тачпад — px по главной оси', () => {
    expect(rowWheel(wh(12.5, 3))).toEqual({ kind: 'px', px: 12.5 })
    expect(rowWheel(wh(-2, 30.5))).toEqual({ kind: 'px', px: 30.5 })
    expect(rowWheel(wh(0, -7.5))).toEqual({ kind: 'px', px: -7.5 })
    expect(rowWheel(wh(0, 20))).toEqual({ kind: 'px', px: 20 }) // целый, но меньше 40
    expect(rowWheel(wh(0, 0))).toBeNull()
  })
  it('строки (deltaMode 1) — шаг', () => {
    expect(rowWheel(wh(0, 3, 1))).toEqual({ kind: 'step', dir: 1 })
    expect(rowWheel(wh(-1, 0, 1))).toEqual({ kind: 'step', dir: -1 })
    expect(rowWheel(wh(0, 1, 2))).toEqual({ kind: 'step', dir: 1 })
  })
})

describe('stepTarget', () => {
  it('щелчки колеса копят цель не больше 3 вперёд', () => {
    let t = 4
    for (let k = 0; k < 10; k++) t = stepTarget(t, 1, 4, 20)
    expect(t).toBe(7)
    t = 4
    for (let k = 0; k < 10; k++) t = stepTarget(t, -1, 4, 20)
    expect(t).toBe(1)
    expect(stepTarget(0, -1, 0, 20)).toBe(0)
    expect(stepTarget(20, 1, 20, 20)).toBe(20)
  })
})

describe('rowScroll', () => {
  const centers = [100, 300, 700, 800]
  it('лента между центрами групп по дробной позиции', () => {
    expect(rowScroll(centers, 0, 400)).toBe(100 - 200)
    expect(rowScroll(centers, 1, 400)).toBe(300 - 200)
    expect(rowScroll(centers, 1.5, 400)).toBe(500 - 200)
    expect(rowScroll(centers, 0.25, 400)).toBe(150 - 200)
  })
  it('на слоте n — центр «+»', () => {
    expect(rowScroll(centers, 3, 400)).toBe(800 - 200)
    expect(rowScroll(centers, 9, 400)).toBe(800 - 200) // за краем — тоже «+»
    expect(rowScroll([], 2, 400)).toBe(0)
  })
})

describe('nearSlot и easeOut', () => {
  it('ближайший слот в пределах 0..max', () => {
    expect(nearSlot(3.4, 3)).toBe(3)
    expect(nearSlot(-0.2, 3)).toBe(0)
    expect(nearSlot(1.6, 3)).toBe(2)
  })
  it('ease-out: 0 → 0, 1 → 1, быстро в начале', () => {
    expect(easeOut(0)).toBe(0)
    expect(easeOut(1)).toBe(1)
    expect(easeOut(0.5)).toBeGreaterThan(0.5)
  })
})

describe('dur', () => {
  it('от 160 до 450 мс по расстоянию', () => {
    expect(dur(0)).toBe(160)
    expect(dur(0.5)).toBe(270)
    expect(dur(1)).toBe(380)
    expect(dur(-1)).toBe(380)
    expect(dur(5)).toBe(450)
  })
})
