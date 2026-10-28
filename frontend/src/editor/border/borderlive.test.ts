import { describe, expect, it } from 'vitest'
import type { BorderView, FrameView, Rect } from '../../base/core/types'
import { BorderLive, scaledIns } from './borderlive.svelte'
import { windowOf } from './border'

const bv = (over: Partial<BorderView> = {}): BorderView => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: '',
  name: 'Линия',
  inks: ['#fff'],
  width: 2,
  scale: 0,
  ins: [2, 2, 2, 7],
  url: '/u',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})

describe('scaledIns: отступы вместе с толщиной и размером', () => {
  it('встроенная: отступы пропорциональны толщине', () => {
    expect(scaledIns(bv(), 4, undefined)).toEqual([4, 4, 4, 14])
    expect(scaledIns(bv(), 1, undefined)).toEqual([1, 1, 1, 3.5])
  })
  it('картинка: отступы пропорциональны размеру; толщина её не касается', () => {
    const p = bv({ pic: true, kind: '', width: 0, scale: 1, ins: [4, 5, 4, 5] })
    expect(scaledIns(p, undefined, 1.5)).toEqual([6, 7.5, 6, 7.5])
    expect(scaledIns(p, 9, undefined)).toEqual(p.ins)
  })
  it('толщина встроенной не берётся от размера, размер картинки не берётся от толщины', () => {
    expect(scaledIns(bv(), undefined, 2)).toEqual([2, 2, 2, 7])
  })
  it('нулевая или мусорная толщина — отступы как есть (деление на ноль не рвёт окно)', () => {
    expect(scaledIns(bv({ width: 0 }), 3, undefined)).toEqual([2, 2, 2, 7])
    expect(scaledIns(bv(), 0, undefined)).toEqual([2, 2, 2, 7])
    expect(scaledIns(bv(), NaN, undefined)).toEqual([2, 2, 2, 7])
  })
})

describe('BorderLive: просмотр до отпускания', () => {
  it('нет жеста — значения из View, отступов нет', () => {
    const l = new BorderLive()
    expect(l.fadeOf('s1', 'a', 0.3)).toBe(0.3)
    expect(l.insOf('s1', ['p1'], bv())).toBeUndefined()
  })

  it('прозрачность: только у фото из жеста и только на своём листе', () => {
    const l = new BorderLive()
    l.set({ spread: 's1', ids: ['a', 'b'], fade: 0.6 })
    expect(l.fadeOf('s1', 'a', 0)).toBe(0.6)
    expect(l.fadeOf('s1', 'b', 0.1)).toBe(0.6)
    expect(l.fadeOf('s1', 'c', 0.1)).toBe(0.1)
    expect(l.fadeOf('s2', 'a', 0.1)).toBe(0.1)
  })

  it('прозрачность ноль на ходу — это значение, а не «нет жеста»', () => {
    const l = new BorderLive()
    l.set({ spread: 's1', ids: ['a'], fade: 0 })
    expect(l.fadeOf('s1', 'a', 0.5)).toBe(0)
  })

  it('жест толщины не меняет прозрачность, жест прозрачности не меняет отступы', () => {
    const l = new BorderLive()
    l.set({ spread: 's1', ids: ['p1'], width: 4 })
    expect(l.fadeOf('s1', 'p1', 0.2)).toBe(0.2)
    l.set({ spread: 's1', ids: ['p1'], fade: 0.5 })
    expect(l.insOf('s1', ['p1'], bv())).toBeUndefined()
  })

  it('отступы: место находится по ID места или фото', () => {
    const l = new BorderLive()
    l.set({ spread: 's1', ids: ['p1'], width: 4 })
    expect(l.insOf('s1', ['p1', 'a'], bv())).toEqual([4, 4, 4, 14])
    expect(l.insOf('s1', ['p2', 'a'], bv())).toBeUndefined()
    expect(l.insOf('s2', ['p1'], bv())).toBeUndefined()
    expect(l.insOf('s1', [undefined, 'p1'], bv())).toEqual([4, 4, 4, 14])
  })

  it('release снимает просмотр, пока нет вызовов Go; с вызовом в пути — держит до ответа', async () => {
    const l = new BorderLive()
    l.set({ spread: 's1', ids: ['a'], fade: 0.5 })
    let done!: () => void
    const p = l.commit(() => new Promise<void>((r) => (done = r)))
    await Promise.resolve()
    l.release()
    expect(l.fadeOf('s1', 'a', 0)).toBe(0.5)
    done()
    await p
    expect(l.fadeOf('s1', 'a', 0)).toBe(0)
  })

  it('вызовы идут по очереди: второй ждёт первый; ошибка первого очередь не ломает', async () => {
    const l = new BorderLive()
    const log: string[] = []
    let first!: () => void
    const a = l.commit(
      () =>
        new Promise<void>((r) => {
          first = () => {
            log.push('a')
            r()
          }
        }),
    )
    const b = l.commit(async () => {
      log.push('b')
    })
    await Promise.resolve()
    expect(log).toEqual([])
    first()
    await Promise.all([a, b])
    expect(log).toEqual(['a', 'b'])

    const c = l.commit(() => Promise.reject(new Error('x')))
    await c // ошибку Go показывает store.run; очередь её глотает
    await l.commit(async () => {
      log.push('d')
    })
    expect(log).toEqual(['a', 'b', 'd'])
  })
})

describe('окно фото на ходу ползунка толщины', () => {
  const cell: Rect = { x: 10, y: 20, w: 100, h: 150 }
  const fv = {
    s: 1,
    x: 0,
    y: 0,
    r: 0,
    img: cell,
    win: { x: 12, y: 22, w: 96, h: 146 },
    winShape: '',
    winRadius: 0,
    shape: '',
    radius: 0,
  } as unknown as FrameView

  it('отступы с ходу растут — окно меньше, чем в View; без жеста берётся окно Go', () => {
    const base = windowOf(cell, fv, 0, [2, 2, 2, 2])
    expect(base.win).toEqual(fv.win)
    const ins = scaledIns(bv({ ins: [2, 2, 2, 2] }), 4, undefined)
    const live = windowOf(cell, fv, 0, ins, false, true)
    expect(live.win).toEqual({ x: 14, y: 24, w: 92, h: 142 })
    expect(live.windowed).toBe(true)
  })

  it('круг остаётся кругом, скругление уменьшается на минимальный отступ', () => {
    const oval = windowOf(cell, { ...fv, shape: 'oval', winShape: 'oval' } as FrameView, 0, [4, 4, 4, 4], false, true)
    expect(oval).toMatchObject({ shape: 'oval', radius: 0, windowed: true })
    const round = windowOf(cell, { ...fv, shape: '', radius: 10 } as FrameView, 10, [4, 4, 4, 4], false, true)
    expect(round.radius).toBe(6)
  })

  it('толщина до нуля — окно возвращается к месту, .clip занимает ячейку', () => {
    const w = windowOf(cell, fv, 0, [0, 0, 0, 0], false, true)
    expect(w).toMatchObject({ win: cell, windowed: false })
  })

  it('слишком толстая — пара сжата до предела 70 % стороны, окно не меньше 30 %', () => {
    const w = windowOf(cell, fv, 0, [80, 80, 80, 80], false, true)
    expect(w.win.w).toBeCloseTo(30)
    expect(w.win.h).toBeCloseTo(45)
  })

  it('галерея (preview) жеста не знает', () => {
    expect(windowOf(cell, fv, 0, [9, 9, 9, 9], true, true).windowed).toBe(false)
  })
})
