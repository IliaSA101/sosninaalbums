import { describe, expect, it } from 'vitest'
import { needScroll, reachableTop, revealDelta } from './reveal'

describe('needScroll', () => {
  it('карточка выше видимой части', () => {
    expect(needScroll(0, 120)).toBe(true)
    expect(needScroll(0, 0)).toBe(false)
    expect(needScroll(40, 40)).toBe(false)
  })
})

describe('revealDelta', () => {
  it('в поле зрения — не двигаем', () => {
    expect(revealDelta(100, 160, 600)).toBe(0)
  })
  it('ниже края — подтянуть вверх', () => {
    expect(revealDelta(560, 640, 600)).toBe(48)
  })
  it('выше края — вернуть вниз', () => {
    expect(revealDelta(-30, 20, 600)).toBe(-38)
  })
  it('не влезает целиком: верх важнее низа', () => {
    // Не влезает целиком: приоритет верху элемента.
    expect(revealDelta(100, 900, 600)).toBe(92)
  })
})

describe('reachableTop', () => {
  it('цель в пределах содержимого', () => {
    expect(reachableTop(500, 2000, 600)).toBe(500)
  })
  it('цель дальше конца — упирается в конец и растёт вместе с содержимым', () => {
    expect(reachableTop(2056, 2175, 600)).toBe(1575)
    expect(reachableTop(2056, 2700, 600)).toBe(2056)
  })
  it('короткое содержимое — ноль', () => {
    expect(reachableTop(300, 400, 600)).toBe(0)
  })
})
