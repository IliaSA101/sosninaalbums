import { describe, expect, it } from 'vitest'
import { cardClasses, cardWidth, cellBox } from './layoutcard'

const sheet = { w: 600, h: 300 }

describe('LayoutCard: размеры', () => {
  it('ширина плитки — пропорции листа', () => {
    expect(cardWidth(80, sheet)).toBe(160)
    expect(cardWidth(80, { w: 300, h: 400 })).toBe(60)
  })
  it('ячейка — проценты листа (как в полосе «Варианты» до выноса плитки)', () => {
    expect(cellBox({ x: 60, y: 30, w: 240, h: 120 }, sheet)).toEqual({ left: 10, top: 10, width: 40, height: 40 })
  })
})

describe('LayoutCard: классы', () => {
  it('картинка без щелчка — только card', () => {
    expect(cardClasses({})).toEqual(['card'])
  })
  it('нажимаемая — hov (рамка при наведении), текущая — current', () => {
    expect(cardClasses({ clickable: true })).toEqual(['card', 'hov'])
    expect(cardClasses({ clickable: true, current: true })).toEqual(['card', 'current', 'hov'])
  })
  it('запасная (065) — fallback, пунктир', () => {
    expect(cardClasses({ fallback: true })).toEqual(['card', 'fallback'])
  })
})
