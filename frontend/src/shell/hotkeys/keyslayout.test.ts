import { describe, expect, it } from 'vitest'
import { DEF_KEYS_SPLIT } from '../../base/prefs/keyswin'
import { BOARD_H, BOARD_W } from './board'
import { dragSplit, fitBoard, GAP, MIN, resetSplit, tracks, tracksStyle, U_AUTO, U_MAX, type FitIn } from './keyslayout'

const body = { w: 1200, h: 600 }
const split = { ...DEF_KEYS_SPLIT }

describe('границы блоков', () => {
  it('по умолчанию «Карта крупно» берёт высоту под клавиши (px), список — остальное', () => {
    expect(tracks(split, 304).rA).toBe('304px 8px minmax(130px,1fr)')
    expect(tracks(split, null).rA).toBe('auto 8px minmax(130px,1fr)')
  })
  it('своя граница — доли в fr с пределами в px', () => {
    const t = tracks({ ra: 0.5, rw: 0.3, rh: 0.4 }, null)
    expect(t.rA).toBe('minmax(190px,50.00fr) 8px minmax(130px,50.00fr)')
    expect(t.cB).toBe('minmax(190px,30.00fr) 8px minmax(430px,70.00fr)')
    expect(t.rB).toBe('minmax(110px,60.00fr) 8px minmax(190px,40.00fr)')
    expect(tracksStyle(split, 300)).toBe(`--rA:${tracks(split, 300).rA};--cB:${tracks(split, 300).cB};--rB:${tracks(split, 300).rB}`)
  })
  it('перетаскивание держится в пределах: карта не меньше 190 px, список — 130 px, карточка — 110 px', () => {
    const w = body.w - GAP
    const h = body.h - GAP
    expect(dragSplit('ra', split, { x: 0, y: 0 }, body).ra).toBeCloseTo(MIN.map / h, 5)
    expect(dragSplit('ra', split, { x: 0, y: 9999 }, body).ra).toBeCloseTo(1 - MIN.lowA / h, 5)
    expect(dragSplit('rw', split, { x: 0, y: 0 }, body).rw).toBeCloseTo(MIN.listB / w, 5)
    expect(dragSplit('rw', split, { x: 9999, y: 0 }, body).rw).toBeCloseTo(1 - MIN.rightB / w, 5)
    expect(dragSplit('rh', split, { x: 0, y: 9999 }, body).rh).toBeCloseTo(MIN.map / h, 5)
    expect(dragSplit('rh', split, { x: 0, y: 0 }, body).rh).toBeCloseTo(1 - MIN.card / h, 5)
  })
  it('серединой границы доля считается от тела окна без толщины границы', () => {
    const r = dragSplit('rw', split, { x: 4 + 0.5 * (body.w - GAP), y: 0 }, body)
    expect(r.rw).toBeCloseTo(0.5, 5)
    expect(r.ra).toBeNull() // остальные доли не тронуты
  })
  it('тесное окно (пределы не сходятся) — граница посередине допустимого', () => {
    const r = dragSplit('rw', split, { x: 100, y: 0 }, { w: 500, h: 400 })
    expect(r.rw).toBeGreaterThan(0)
    expect(r.rw).toBeLessThan(1)
  })
  it('двойной щелчок — «авто»: доля по умолчанию, остальные остаются', () => {
    const own = { ra: 0.5, rw: 0.4, rh: 0.3 }
    expect(resetSplit('ra', own)).toEqual({ ra: null, rw: 0.4, rh: 0.3 })
    expect(resetSplit('rw', own)).toEqual({ ra: 0.5, rw: 0.27, rh: 0.3 })
    expect(resetSplit('rh', own)).toEqual({ ra: 0.5, rw: 0.4, rh: 0.62 })
  })
})

const base: FitIn = { mode: 'map', autoRows: true, paneW: 1180, paneH: 300, bodyH: 640, headH: 0, legendH: 40, legendW: 230 }

describe('размер клавиш', () => {
  it('«Карта крупно» по умолчанию: клавиши не крупнее 50 px, высота карты — под них', () => {
    const f = fitBoard(base)
    expect(f.u).toBe(U_AUTO)
    expect(f.below).toBe(false)
    expect(f.autoH).toBe(Math.ceil(BOARD_H * U_AUTO) + 2)
  })
  it('широкое окно: легенда справа, клавиши ровно 50 px', () => {
    expect(fitBoard({ ...base, paneW: 1184 })).toMatchObject({ u: 50, below: false })
  })
  it('узкое окно: справа клавиши мельче 50, а под картой вырастают — легенда уходит под карту, высота карты включает её', () => {
    const f = fitBoard({ ...base, paneW: 928 }) // ≈1024 px по ширине экрана
    expect(Math.floor((928 - 230 - 18) / BOARD_W)).toBeLessThan(U_AUTO)
    expect(f.below).toBe(true)
    expect(f.u).toBe(U_AUTO)
    expect(f.autoH).toBe(Math.ceil(BOARD_H * f.u) + 2 + base.legendH + 6)
  })
  it('совсем узкое окно: клавиши мельче 50 и при этом целиком по ширине', () => {
    const f = fitBoard({ ...base, paneW: 700 })
    expect(f.below).toBe(true)
    expect(f.u).toBe(Math.floor(700 / BOARD_W))
    expect(f.u * BOARD_W).toBeLessThanOrEqual(700)
  })
  it('своя граница: легенда уходит под карту, если так клавиши крупнее', () => {
    const own: FitIn = { ...base, autoRows: false, paneW: 760, paneH: 330, legendH: 24 }
    const f = fitBoard(own)
    expect(f.below).toBe(true)
    expect(f.autoH).toBeNull()
    expect(f.u).toBeLessThanOrEqual(U_MAX)
    expect(f.u).toBeGreaterThan(Math.floor((760 - 248) / BOARD_W))
  })
  it('своя граница, места много: легенда справа, клавиши до 64 px', () => {
    const f = fitBoard({ ...base, autoRows: false, paneW: 1500, paneH: 500 })
    expect(f.below).toBe(false)
    expect(f.u).toBe(Math.floor(500 / BOARD_H) > U_MAX ? U_MAX : Math.floor(500 / BOARD_H))
  })
  it('«Список крупно»: плата вписана в блок под шапкой, не крупнее 60 px и не мельче 16 px', () => {
    const f = fitBoard({ ...base, mode: 'list', paneW: 700, paneH: 300, headH: 30 })
    expect(f.below).toBe(false)
    expect(f.autoH).toBeNull()
    expect(f.u).toBe(Math.floor(Math.min(700 / BOARD_W, (300 - 30 - 6) / BOARD_H)))
    expect(fitBoard({ ...base, mode: 'list', paneW: 3000, paneH: 3000 }).u).toBe(60)
    expect(fitBoard({ ...base, mode: 'list', paneW: 10, paneH: 10 }).u).toBe(16)
  })
  it('до первого измерения (нули) размер клавиш — минимальный, а не NaN', () => {
    const f = fitBoard({ ...base, paneW: 0, paneH: 0, bodyH: 0, legendH: 0 })
    expect(Number.isFinite(f.u)).toBe(true)
    expect(f.u).toBe(16)
  })
})
