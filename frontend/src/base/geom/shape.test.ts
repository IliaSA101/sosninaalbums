import { describe, expect, it } from 'vitest'
import { inShape, isOval, liveRadius, outlineOf, shapeCss } from './shape'

describe('shapeCss', () => {
  it('прямоугольник — без стиля', () => {
    expect(shapeCss('', 0, 100, 60)).toBeUndefined()
    expect(shapeCss(undefined, undefined, 100, 60)).toBeUndefined()
  })
  it('овал и круг — 50 %', () => {
    expect(shapeCss('oval', 0, 100, 100)).toBe('50%')
    expect(shapeCss('oval', 0, 60, 80)).toBe('50%')
  })
  it('скругление ρ мм — проценты от Ш и В: углы круглые', () => {
    expect(shapeCss('', 10, 100, 50)).toBe('10% / 20%')
  })
  it('радиус не больше половины меньшей стороны', () => {
    expect(shapeCss('', 999, 100, 50)).toBe('25% / 50%')
  })
  it('нулевая сторона — без стиля, не NaN', () => {
    expect(shapeCss('', 5, 0, 50)).toBeUndefined()
  })
})

describe('liveRadius и outlineOf', () => {
  it('liveRadius: % меньшей стороны, зажим 0..50', () => {
    expect(liveRadius(10, 100, 60)).toBeCloseTo(6)
    expect(liveRadius(80, 100, 60)).toBeCloseTo(30)
    expect(liveRadius(-5, 100, 60)).toBe(0)
    expect(liveRadius(NaN, 100, 60)).toBe(0)
  })
  it('outlineOf', () => {
    expect(outlineOf('', 0)).toBeUndefined()
    expect(outlineOf(undefined, undefined)).toBeUndefined()
    expect(outlineOf('oval', 0)).toEqual({ oval: true })
    expect(outlineOf('', 7)).toEqual({ radius: 7 })
    expect(isOval('oval')).toBe(true)
    expect(isOval('')).toBe(false)
  })
})

describe('inShape', () => {
  it('круг: центр и край внутри, угол габарита — нет', () => {
    expect(inShape({ x: 50, y: 50 }, 100, 100, 'oval')).toBe(true)
    expect(inShape({ x: 100, y: 50 }, 100, 100, 'oval')).toBe(true)
    expect(inShape({ x: 2, y: 2 }, 100, 100, 'oval')).toBe(false)
    expect(inShape({ x: 85, y: 85 }, 100, 100, 'oval')).toBe(true) // (35; 35) от центра: 49,5 < 50
  })
  it('овал 60×80: вытянут по высоте', () => {
    expect(inShape({ x: 30, y: 2 }, 60, 80, 'oval')).toBe(true)
    expect(inShape({ x: 2, y: 8 }, 60, 80, 'oval')).toBe(false)
  })
  it('скруглённый угол: у самого угла вне, середина стороны внутри', () => {
    expect(inShape({ x: 1, y: 1 }, 100, 60, '', 10)).toBe(false)
    expect(inShape({ x: 50, y: 0 }, 100, 60, '', 10)).toBe(true)
    expect(inShape({ x: 2, y: 2 }, 100, 60, '', 10)).toBe(false) // 8√2 = 11,3 от центра дуги (10; 10) > 10
    expect(inShape({ x: 3, y: 3 }, 100, 60, '', 10)).toBe(true) // 7√2 = 9,9 < 10
  })
  it('прямоугольник: вся рамка, вне рамки — нет', () => {
    expect(inShape({ x: 0, y: 0 }, 100, 60, '')).toBe(true)
    expect(inShape({ x: 101, y: 10 }, 100, 60, '')).toBe(false)
  })
})
