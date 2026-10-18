import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const store = vi.hoisted(() => ({
  current: 0,
  navAt: 0,
  navSeq: 0,
  view: { spreads: [] as { id: string; photos: string[] }[] },
  go: (_i: number) => {},
  // Ряд идёт по листам (обложка 032 + развороты); без обложки лист = разворот.
  get sheetIndex() {
    return this.current
  },
  get sheets() {
    return this.view.spreads
  },
  goSheet(i: number) {
    this.go(i)
  },
}))
vi.mock('../../base/core/store.svelte', () => ({ store }))
vi.mock('../../base/drag/drag.svelte', () => ({ drag: { active: false, rehit: () => {} } }))
vi.mock('../backdrop/bgmode.svelte', () => ({ bgmode: { cur: null } }))

import { bgmode } from '../backdrop/bgmode.svelte'
import { row } from './spreadrow.svelte'

const N = 10
const P = 100
let now = 1000
let frames: FrameRequestCallback[] = []
const go = vi.fn()

// Едем кадрами по 16 мс, пока не кончатся анимации.
function ride(limit = 1000) {
  for (let t = 0; t < limit && frames.length; t += 16) {
    now += 16
    const q = frames
    frames = []
    q.forEach((cb) => cb(now))
  }
}

// То же, что делает эффект Stage: go() меняет current и зовёт follow.
function setGo() {
  go.mockImplementation((i: number) => {
    store.current = Math.max(0, Math.min(i, N - 1))
    store.navAt = now
    store.navSeq++
    row.follow(store.current)
  })
  store.go = go
}

const wheelEv = (deltaX: number, deltaY = 0, extra: Partial<WheelEvent> = {}) =>
  ({ deltaX, deltaY, deltaMode: 0, ctrlKey: false, ...extra }) as WheelEvent

beforeEach(() => {
  now = 1000
  frames = []
  vi.useFakeTimers()
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => frames.push(cb))
  vi.stubGlobal('cancelAnimationFrame', () => {
    frames = []
  })
  store.current = 0
  store.navAt = 0
  store.view = { ...store.view, spreads: Array.from({ length: N }, (_, i) => ({ id: `s${i}`, photos: [] })) as never }
  go.mockReset()
  setGo()
  row.geo(P, N, false)
  row.follow(0)
})
afterEach(() => {
  row.geo(P, 0, false)
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('SpreadRow', () => {
  it('навигация (свежий navAt) — едет, не прыгает; в конце pos = цель', () => {
    store.go(2)
    expect(row.moving).toBe(true)
    expect(row.pos).toBe(0)
    ride()
    expect(row.pos).toBe(2)
    expect(row.moving).toBe(false)
    expect(go).toHaveBeenCalledTimes(1)
  })

  it('правка без навигации (navAt устарел) — встаёт на место мгновенно', () => {
    store.current = 4
    row.follow(4)
    expect(row.pos).toBe(4)
    expect(row.moving).toBe(false)
  })

  it('дальний переход — старт за 1 слот до цели', () => {
    store.go(8)
    expect(row.pos).toBe(7)
    ride()
    expect(row.pos).toBe(8)
  })

  it('shown: пока ряд едет — цель перехода, а не слот на кадре (счётчик не отстаёт)', () => {
    store.go(8)
    expect(row.near).toBe(7)
    expect(row.shown).toBe(8)
    ride()
    expect(row.shown).toBe(8)
    expect(row.near).toBe(8)
  })

  it('step до слота «новый» и обратно: current не меняется', () => {
    store.go(N - 1)
    ride()
    go.mockClear()
    row.step(1)
    ride()
    expect(row.pos).toBe(N)
    expect(row.atNew).toBe(true)
    expect(go).not.toHaveBeenCalled()
    row.step(-1)
    ride()
    expect(row.pos).toBe(N - 1)
    expect(go).not.toHaveBeenCalled()
  })

  it('go() на тот же разворот со слота «новый» возвращает ряд', () => {
    store.go(N - 1)
    ride()
    row.step(1)
    ride()
    store.go(N - 1)
    ride()
    expect(row.pos).toBe(N - 1)
  })

  it('щелчки колеса мыши подряд копятся, не дальше 3 вперёд', () => {
    const e = wheelEv(0, 100)
    for (let i = 0; i < 6; i++) row.wheel(e)
    expect(go).toHaveBeenLastCalledWith(3)
    ride()
    expect(row.pos).toBe(3)
  })

  it('тачпад: ряд едет за пальцами, после паузы доводка к следующему (сдвиг ≥ 0,2)', () => {
    expect(row.wheel(wheelEv(30.5))).toBe(true)
    expect(row.pos).toBeCloseTo(0.305, 3)
    expect(row.moving).toBe(true)
    expect(go).not.toHaveBeenCalled()
    vi.advanceTimersByTime(200) // IDLE
    ride()
    expect(row.pos).toBe(1)
    expect(go).toHaveBeenCalledWith(1) // жест фиксирует текущего в конце доводки
    expect(store.current).toBe(1)
  })

  it('тачпад: малый сдвиг возвращает, текущий и выделение целы', () => {
    row.wheel(wheelEv(10.5))
    vi.advanceTimersByTime(200)
    ride()
    expect(row.pos).toBe(0)
    expect(go).not.toHaveBeenCalled()
  })

  it('резинка за левым краем', () => {
    row.wheel(wheelEv(-500.5))
    expect(row.pos).toBeLessThan(0)
    expect(row.pos).toBeGreaterThanOrEqual(-0.3)
    vi.advanceTimersByTime(200)
    ride()
    expect(row.pos).toBe(0)
  })

  it('щелчок мыши Shift+колесо (целый ±100 по X) — шаг на слот', () => {
    row.wheel(wheelEv(100))
    ride()
    expect(row.pos).toBe(1)
    expect(go).toHaveBeenCalledWith(1)
  })

  it('в зуме ряд стоит: pos ≡ cur, колесо не разбирается, шаг — мгновенно', () => {
    row.geo(P, N, true)
    expect(row.wheel(wheelEv(30.5))).toBe(false)
    expect(row.wheel(wheelEv(0, 100))).toBe(false)
    row.step(1)
    expect(row.pos).toBe(1)
    expect(row.moving).toBe(false)
  })

  it('finish() доводит до цели сразу', () => {
    store.go(2)
    row.finish()
    expect(row.pos).toBe(2)
    expect(row.moving).toBe(false)
  })

  it('режим фона: на последнем развороте «→» не уводит ряд на слот «новый»', () => {
    store.go(N - 1)
    ride()
    ;(bgmode as { cur: unknown }).cur = {}
    try {
      row.step(1)
      ride()
      expect(row.pos).toBe(N - 1)
      expect(row.atNew).toBe(false)
    } finally {
      ;(bgmode as { cur: unknown }).cur = null
    }
  })

  it('grab по соседу посреди жеста тачпада: ряд доезжает до слота, не застревает', () => {
    vi.stubGlobal('window', { addEventListener: () => {}, removeEventListener: () => {} })
    row.wheel(wheelEv(30.5))
    expect(row.pos).toBeCloseTo(0.305)
    const peer = { closest: () => ({}) }
    expect(row.grab({ clientX: 0, clientY: 0, target: peer } as unknown as PointerEvent)).toBe(true)
    expect(row.moving).toBe(false)
    expect(Number.isInteger(row.pos)).toBe(true)
  })

  it('near и подсветка: округляет, на слоте n — atNew', () => {
    row.wheel(wheelEv(60.5))
    expect(row.near).toBe(1)
    expect(row.atNew).toBe(false)
  })
})
