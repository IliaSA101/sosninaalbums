import { beforeEach, describe, expect, it } from 'vitest'
import { framelive } from './framelive.svelte'

const F = { s: 1.2, x: 0, y: 0, r: 3 }

// Живой угол шкалы (059): карточка читает его на ходу (InspectorFrame: angleOf(id) ?? fv.tilt).
describe('framelive angle', () => {
  beforeEach(() => framelive.clear())

  it('while dragging the card sees the live angle', () => {
    framelive.set('a', F, true, false, 3.2)
    expect(framelive.angleOf('a')).toBe(3.2)
    framelive.set('a', { ...F, r: 4.5 }, true, false, 4.5)
    expect(framelive.angleOf('a')).toBe(4.5)
  })

  it('zero angle is a live value, not "none"', () => {
    framelive.set('a', F, true, false, 0)
    expect(framelive.angleOf('a')).toBe(0)
  })

  it('after cancel (Esc) the card falls back to the View value', () => {
    framelive.set('a', F, true, false, 3.2)
    framelive.clear()
    expect(framelive.angleOf('a')).toBeNull()
  })

  it('another photo does not see it', () => {
    framelive.set('a', F, true, false, 3.2)
    expect(framelive.angleOf('b')).toBeNull()
  })

  it('zoom/pan preview has no angle', () => {
    framelive.set('a', F)
    expect(framelive.angleOf('a')).toBeNull()
    framelive.set('a', F, false, true)
    expect(framelive.angleOf('a')).toBeNull()
  })

  it('frame kept until Go answers still carries the angle', () => {
    framelive.set('a', F, false, false, 3.2)
    expect(framelive.angleOf('a')).toBe(3.2)
    expect(framelive.tilting('a')).toBe(false)
  })
})

// Подгонка кадра (130): силуэт на виньетке виден, пока сдвигают кадр или тянут масштаб; наклон и отпускание его гасят.
describe('framelive fitting', () => {
  beforeEach(() => framelive.clear())

  it('без сессии подгонки нет', () => {
    expect(framelive.fitting('a')).toBe(false)
  })

  it('сдвиг за ✥ — подгонка, только у этого фото', () => {
    framelive.set('a', F, false, true)
    expect(framelive.fitting('a')).toBe(true)
    expect(framelive.fitting('b')).toBe(false)
  })

  it('масштаб (ползунок на фото и в карточке) — подгонка', () => {
    framelive.setZoom('a', F)
    expect(framelive.fitting('a')).toBe(true)
    expect(framelive.panning('a')).toBe(false)
    expect(framelive.of('a')).toEqual(F)
  })

  it('отпускание гасит: кадр остаётся до ответа Go, подгонки нет', () => {
    framelive.setZoom('a', F)
    framelive.set('a', F)
    expect(framelive.fitting('a')).toBe(false)
    expect(framelive.of('a')).toEqual(F)
    framelive.set('a', F, false, true)
    framelive.clear()
    expect(framelive.fitting('a')).toBe(false)
  })

  it('наклон — не подгонка (там сетка 9×9)', () => {
    framelive.set('a', F, true, false, 2)
    expect(framelive.fitting('a')).toBe(false)
    expect(framelive.tilting('a')).toBe(true)
  })

  it('смена нормы листа поднимает счётчик', () => {
    const n = framelive.normRev
    framelive.bumpNorm()
    expect(framelive.normRev).toBe(n + 1)
  })
})
