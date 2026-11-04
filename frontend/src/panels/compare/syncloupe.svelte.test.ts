// @vitest-environment happy-dom
import { describe, expect, it, vi } from 'vitest'
import { SyncLoupe, type Src } from './syncloupe.svelte'

// Жесты общей лупы окна сравнения (138-7): щелчок и Z — 1:1 у всех, перетаскивание двигает все,
// Shift-перетаскивание — одну ячейку, «вписать» снимает сдвиги, не готовая ячейка не зумится.
const NAT = { w: 2000, h: 1000 }
const BOX = { w: 500, h: 300 } // вписано 0.25
const mk = (over: Partial<Src> = {}) => {
  const picked: number[] = []
  const lp = new SyncLoupe(() => ({
    ids: ['a', 'b'],
    active: 0,
    boxes: [BOX, BOX],
    nat: () => NAT,
    ready: () => true,
    max: () => 1,
    pick: (i) => picked.push(i),
    ...over,
  }))
  return { lp, picked }
}
const el = () => ({ getBoundingClientRect: () => ({ left: 0, top: 0 }), setPointerCapture: vi.fn() })
const ev = (x: number, y: number, o: Partial<PointerEvent> = {}) =>
  ({
    clientX: x,
    clientY: y,
    button: 0,
    pointerId: 1,
    shiftKey: false,
    ctrlKey: false,
    metaKey: false,
    currentTarget: el(),
    ...o,
  }) as unknown as PointerEvent

describe('щелчок и Z', () => {
  it('щелчок по ячейке 1:1 у всех и она становится активной; второй — вписано', () => {
    const { lp, picked } = mk()
    lp.down(1, ev(100, 100))
    lp.up(1, ev(100, 100))
    expect(picked).toEqual([1])
    expect(lp.scale(0)).toBeCloseTo(1)
    expect(lp.scale(1)).toBeCloseTo(1)
    expect(lp.zoomed(0)).toBe(true)
    lp.down(1, ev(100, 100))
    lp.up(1, ev(100, 100))
    expect(lp.sync.z).toBe(0)
    expect(lp.zoomed(0)).toBe(false)
  })

  it('Z (toggleAt) — то же в активной ячейке без мыши, в центре', () => {
    const { lp } = mk()
    lp.toggleAt()
    expect(lp.scale(0)).toBeCloseTo(1)
    expect(lp.label).toBe('1:1')
    lp.toggleAt()
    expect(lp.label).toMatch(/^Вписано/)
  })

  it('картинка не загружена (или заглушка) — щелчок и зум ничего не делают', () => {
    const { lp } = mk({ ready: () => false })
    expect(lp.down(0, ev(1, 1))).toBe(false)
    lp.toggleAt()
    lp.zoomBy(2)
    expect(lp.sync.z).toBe(0)
    expect(lp.label).toBe('')
  })

  it('кадр, который и так не меньше 1:1, остаётся вписанным (canZoom ложно)', () => {
    const { lp } = mk({ nat: () => ({ w: 400, h: 200 }) })
    expect(lp.canZoom(0)).toBe(false)
    lp.toggleAt()
    expect(lp.zoomed(0)).toBe(false)
  })
})

describe('перетаскивание', () => {
  it('вписанный кадр не двигается и щелчком не становится', () => {
    const { lp } = mk()
    lp.down(0, ev(100, 100))
    lp.move(0, ev(160, 100))
    expect(lp.pressed).toBe(-1)
    lp.cancel()
    expect(lp.sync.z).toBe(0)
  })

  it('увеличенные двигаются все сразу: общая точка уходит против сдвига', () => {
    const { lp } = mk()
    lp.toggleAt()
    const before = lp.sync.cx
    lp.down(0, ev(100, 100))
    lp.move(0, ev(160, 100))
    expect(lp.pressed).toBe(0)
    expect(lp.sync.cx).toBeLessThan(before)
    lp.up(0, ev(160, 100))
    expect(lp.sync.z).toBeGreaterThan(0) // сдвиг — не щелчок
    expect(lp.pressed).toBe(-1)
  })

  it('Shift: только эта ячейка, общая точка прежняя; сдвиг держится при зуме и снят «вписать»', () => {
    const { lp } = mk()
    lp.toggleAt()
    const before = lp.sync
    lp.down(1, ev(100, 100, { shiftKey: true }))
    lp.move(1, ev(150, 100, { shiftKey: true }))
    lp.up(1, ev(150, 100, { shiftKey: true }))
    expect(lp.sync).toEqual(before)
    expect(lp.offs.b?.dx).not.toBe(0)
    expect(lp.offs.a).toBeUndefined()
    lp.zoomBy(1.25, 0)
    expect(lp.offs.b?.dx).not.toBe(0)
    lp.fit()
    expect(lp.offs).toEqual({})
    expect(lp.sync.z).toBe(0)
  })
})

describe('сброс и листание', () => {
  it('reset снимает всё; forget — свой сдвиг одного кадра', () => {
    const { lp } = mk()
    lp.toggleAt()
    lp.offs = { a: { dx: 0.1, dy: 0 }, b: { dx: 0.2, dy: 0 } }
    lp.forget('a')
    expect(Object.keys(lp.offs)).toEqual(['b'])
    lp.reset()
    expect(lp.sync.z).toBe(0)
    expect(lp.offs).toEqual({})
  })

  it('carry: те же пропорции — зум остаётся, свой сдвиг нового кадра снят; другие — вписано у всех', () => {
    const { lp } = mk()
    lp.toggleAt()
    lp.offs = { c: { dx: 0.1, dy: 0 } }
    lp.carry('c', { w: 2000, h: 1000 }, { w: 1000, h: 500 })
    expect(lp.sync.z).toBeGreaterThan(0)
    expect(lp.offs.c).toBeUndefined()
    lp.carry('c', { w: 2000, h: 1000 }, { w: 1000, h: 1000 })
    expect(lp.sync.z).toBe(0)
    expect(lp.offs).toEqual({})
  })
})
