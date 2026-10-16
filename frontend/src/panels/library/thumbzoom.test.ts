import { describe, expect, it } from 'vitest'
import { THUMB_MAX, THUMB_MIN } from '../../base/prefs/view.svelte'
import { anchorFrac, isThumbWheel, scrollFix, thumbStep, thumbWheelAcc, type ThumbWheel } from './thumbzoom'

const wheel = (deltaY: number, mod: Partial<ThumbWheel> = {}): ThumbWheel => ({
  deltaX: 0,
  deltaY,
  deltaMode: 0,
  ctrlKey: true,
  ...mod,
})

describe('thumbStep', () => {
  it('step clamps: вверх до 220, вниз до 70, дальше нет', () => {
    let n = 110
    for (let i = 0; i < 40; i++) n = thumbStep(n, wheel(-100))
    expect(n).toBe(THUMB_MAX)
    expect(thumbStep(n, wheel(-100))).toBe(THUMB_MAX)
    for (let i = 0; i < 40; i++) n = thumbStep(n, wheel(100))
    expect(n).toBe(THUMB_MIN)
    expect(thumbStep(n, wheel(100))).toBe(THUMB_MIN)
  })

  it('щелчок колеса ≈ ×1,13: от 70 до 220 примерно за 10 щелчков', () => {
    let n = 70
    let clicks = 0
    while (n < THUMB_MAX) {
      n = thumbStep(n, wheel(-100))
      clicks++
    }
    expect(clicks).toBeGreaterThanOrEqual(9)
    expect(clicks).toBeLessThanOrEqual(11)
  })

  it('accumulates: малые дельты тачпада не теряются на округлении', () => {
    let exact = 110
    for (let i = 0; i < 60; i++) exact = thumbStep(exact, wheel(-2))
    // каждый шаг по отдельности округлился бы обратно в 110
    expect(Math.round(thumbStep(110, wheel(-2)))).toBe(110)
    expect(Math.round(exact)).toBeGreaterThan(110)
  })

  it('нулевая и мусорная дельта размер не меняют', () => {
    expect(thumbStep(120, wheel(0))).toBe(120)
    expect(thumbStep(120, wheel(NaN))).toBe(120)
  })

  it('строчные дельты (deltaMode 1) тоже работают', () => {
    expect(thumbStep(110, wheel(-3, { deltaMode: 1 }))).toBeGreaterThan(110)
  })
})

describe('thumbWheelAcc', () => {
  it('два события до кадра — обе дельты учтены, кадр заказан один раз', () => {
    const acc = thumbWheelAcc()
    expect(acc.push(110, wheel(-100))).toBe(true)
    // ползунок всё ещё 110: прошлое событие кадр не применил
    expect(acc.push(110, wheel(-100))).toBe(false)
    const both = Math.round(thumbStep(thumbStep(110, wheel(-100)), wheel(-100)))
    expect(acc.take()).toBe(both)
    expect(both).toBeGreaterThan(Math.round(thumbStep(110, wheel(-100))))
  })

  it('после кадра ползунок главнее: копим от его значения', () => {
    const acc = thumbWheelAcc()
    acc.push(110, wheel(-100))
    acc.take()
    acc.push(90, wheel(0))
    expect(acc.take()).toBe(90)
  })

  it('reset сбрасывает ожидание кадра', () => {
    const acc = thumbWheelAcc()
    acc.push(110, wheel(-100))
    acc.reset()
    expect(acc.push(70, wheel(0))).toBe(true)
    expect(acc.take()).toBe(70)
  })
})

describe('isThumbWheel', () => {
  it('Ctrl, Cmd и Alt — да; без модификаторов — нет', () => {
    expect(isThumbWheel(wheel(-100))).toBe(true)
    expect(isThumbWheel(wheel(-100, { ctrlKey: false, metaKey: true }))).toBe(true)
    expect(isThumbWheel(wheel(-100, { ctrlKey: false, altKey: true }))).toBe(true)
    expect(isThumbWheel(wheel(-100, { ctrlKey: false }))).toBe(false)
    expect(isThumbWheel(wheel(-100, { ctrlKey: false, metaKey: false, altKey: false }))).toBe(false)
  })
})

describe('якорь', () => {
  it('anchorFrac: середина, края и вне ячейки', () => {
    expect(anchorFrac(100, 80, 140)).toBe(0.5)
    expect(anchorFrac(100, 80, 90)).toBe(0)
    expect(anchorFrac(100, 80, 500)).toBe(1)
    expect(anchorFrac(100, 0, 140)).toBe(0)
  })

  it('scrollFix: фото под курсором остаётся на месте (≤ 2 px)', () => {
    // курсор на 25 % высоты ячейки 100 px, верх ячейки 200; после смены на 150 px верх уехал на 230
    const cy = 225
    const fy = anchorFrac(200, 100, cy)
    const dy = scrollFix(230, cy, fy, 150)
    // после прокрутки верх = 230 − dy; точка ячейки под курсором
    expect(Math.abs(230 - dy + fy * 150 - cy)).toBeLessThanOrEqual(2)
  })

  it('scrollFix: без сдвига — ноль', () => {
    expect(scrollFix(200, 225, 0.25, 100)).toBe(0)
  })
})
