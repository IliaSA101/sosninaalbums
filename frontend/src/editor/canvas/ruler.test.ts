import { describe, expect, it } from 'vitest'
import { labelStep, selSpan, ticks, MIN_LABEL_PX } from './ruler'

describe('labelStep', () => {
  it('берёт наименьший шаг, при котором подписи не ближе 48 px', () => {
    expect(labelStep(1)).toBe(50) // 20 мм = 20 px < 48, 50 мм = 50 px
    expect(labelStep(5)).toBe(10) // 5 мм = 25 px, 10 мм = 50 px
    expect(labelStep(48)).toBe(1) // ровно 48 px подходит
    expect(labelStep(47.9)).toBe(2)
  })
  it('на крошечном масштабе — самый крупный шаг', () => {
    expect(labelStep(0.01)).toBe(200)
  })
  it('расстояние между подписями всегда >= MIN_LABEL_PX (кроме потолка)', () => {
    for (let k = 0.25; k <= 100; k *= 1.07) {
      const s = labelStep(k)
      if (s !== 200) expect(s * k).toBeGreaterThanOrEqual(MIN_LABEL_PX)
    }
  })
})

describe('ticks', () => {
  it('шаг 50: крупные 0, 50, 100, мелкие каждые 10', () => {
    const t = ticks(0, 100, 1)
    expect(t.filter((x) => x.major).map((x) => x.at)).toEqual([0, 50, 100])
    expect(t.map((x) => x.at)).toEqual([0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100])
    expect(t[0]).toEqual({ at: 0, major: true, label: '0' })
    expect(t[1]).toEqual({ at: 10, major: false, label: '' })
  })
  it('шаг 2 и 20 — мелкие вдвое', () => {
    expect(ticks(0, 4, 30).map((x) => x.at)).toEqual([0, 1, 2, 3, 4]) // шаг 2
    expect(ticks(0, 40, 2.5).map((x) => x.at)).toEqual([0, 10, 20, 30, 40]) // шаг 20
  })
  it('шаг 1: мелкие по 0,2 мм без накопления ошибки', () => {
    const t = ticks(0, 2, 100)
    expect(t.map((x) => x.at)).toEqual([0, 0.2, 0.4, 0.6, 0.8, 1, 1.2, 1.4, 1.6, 1.8, 2])
    expect(t.filter((x) => x.major).map((x) => x.label)).toEqual(['0', '1', '2'])
  })
  it('срез из середины: деления в пределах отрезка', () => {
    const t = ticks(123, 177, 1)
    expect(t[0].at).toBe(130)
    expect(t[t.length - 1].at).toBe(170)
    expect(t.filter((x) => x.major).map((x) => x.at)).toEqual([150])
  })
  it('зум ×4: подписи гуще, но не ближе 48 px', () => {
    const fit = ticks(0, 600, 1.7).filter((x) => x.major)
    const z4 = ticks(0, 150, 6.8).filter((x) => x.major)
    expect(z4[1].at - z4[0].at).toBeLessThan(fit[1].at - fit[0].at)
    expect((z4[1].at - z4[0].at) * 6.8).toBeGreaterThanOrEqual(MIN_LABEL_PX)
  })
  it('пустой или неверный отрезок и масштаб', () => {
    expect(ticks(10, 5, 1)).toEqual([])
    expect(ticks(0, 100, 0)).toEqual([])
    expect(ticks(0, 100, NaN)).toEqual([])
  })
})

describe('selSpan', () => {
  it('зажимает в разворот', () => {
    expect(selSpan(-5, 20, 100)).toEqual([0, 20])
    expect(selSpan(90, 130, 100)).toEqual([90, 100])
  })
  it('вне разворота — null', () => {
    expect(selSpan(110, 120, 100)).toBeNull()
    expect(selSpan(-20, -5, 100)).toBeNull()
  })
})
