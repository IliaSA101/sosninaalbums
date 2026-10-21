import { afterEach, describe, expect, it } from 'vitest'
import { HOLD_MS, LIMIT, SHOWN, afterNav, asSheet, cutHeldNext, isHidden, sheetThumbW, shownW, visibleChips, widthsKey } from './chips'
import { setMacForTest } from '../../base/core/platform'
import { GAP, PAD, cellW, chipW, groupW } from './timeline'
import type { PlaceView, SpreadView } from '../../base/core/types'

const place = (id: string): PlaceView => ({ id, x: 0, y: 0, w: 100, h: 100, rot: 0, photo: '', fit: '', spine: false, z: 1 })

// n фото и e пустых мест сетки.
const sp = (n: number, e = 0): SpreadView =>
  ({
    id: 's1',
    photos: Array.from({ length: n }, (_, i) => 'p' + i),
    rects: Array.from({ length: n }, () => ({ x: 0, y: 0, w: 1, h: 1 })),
    places: Array.from({ length: e }, (_, i) => place('e' + i)),
  }) as SpreadView

describe('visibleChips', () => {
  it('константы: 11 + «+N» = 12', () => {
    expect(SHOWN + 1).toBe(LIMIT)
    expect(HOLD_MS).toBe(600)
  })
  it('12 плиток — все видны, «+N» нет', () => {
    expect(visibleChips(sp(12), null)).toEqual({ photos: 12, places: 0, more: 0, fold: false })
  })
  it('13 фото — 11 и «+2»', () => {
    expect(visibleChips(sp(13), null)).toEqual({ photos: 11, places: 0, more: 2, fold: false })
  })
  it('пустые места входят в счёт: 8 фото + 5 мест — 8 фото, 3 места, «+2»', () => {
    expect(visibleChips(sp(8, 5), null)).toEqual({ photos: 8, places: 3, more: 2, fold: false })
  })
  it('больше 11 фото и есть места: места скрыты целиком', () => {
    expect(visibleChips(sp(14, 3), null)).toEqual({ photos: 11, places: 0, more: 6, fold: false })
  })
  it('раскрытая группа — все плитки и «‹»', () => {
    expect(visibleChips(sp(13), 's1')).toEqual({ photos: 13, places: 0, more: 0, fold: true })
  })
  it('раскрыта другая группа — эта свёрнута', () => {
    expect(visibleChips(sp(13), 's2').more).toBe(2)
  })
  it('короткая группа «раскрытой» не бывает: «‹» нет', () => {
    expect(visibleChips(sp(5), 's1').fold).toBe(false)
  })
})

describe('shownW', () => {
  it('короткая группа — как groupW', () => {
    const s = sp(4, 2)
    expect(shownW(s, null)).toBe(groupW(s))
  })
  it('пустой разворот — как groupW', () => {
    expect(shownW(sp(0), null)).toBe(groupW(sp(0)))
  })
  it('свёрнутая: 11 плиток и квадрат «+N»', () => {
    const s = sp(13)
    expect(shownW(s, null)).toBe(11 * cellW(s.rects[0]) + chipW(1) + GAP * 11 + 2 * PAD)
  })
  it('раскрытая: все и квадрат «‹»', () => {
    const s = sp(13)
    expect(shownW(s, 's1')).toBe(13 * cellW(s.rects[0]) + chipW(1) + GAP * 13 + 2 * PAD)
  })
})

describe('widthsKey', () => {
  it('те же ширины — та же подпись, даже у нового объекта разворота', () => {
    const a = sp(4)
    const b = { ...sp(2), id: 's2' }
    expect(widthsKey([a, b], null)).toBe(widthsKey([{ ...a, name: 'Новое' }, b], null))
  })
  it('сменилось число плиток или раскрылась длинная группа — подпись другая', () => {
    const a = sp(4)
    const long = { ...sp(13), id: 'long' }
    const k = widthsKey([a, long], null)
    expect(widthsKey([sp(5), long], null)).not.toBe(k)
    expect(widthsKey([a, long], 'long')).not.toBe(k)
    expect(widthsKey([a, long], null)).toBe(k) // раскрытие не отравило кэш свёрнутой
  })
  it('пустая лента', () => {
    expect(widthsKey([], null)).toBe('')
  })
})

describe('afterNav', () => {
  it('тот же разворот — группа остаётся', () => {
    expect(afterNav('s1', 's1', 's1')).toBe('s1')
  })
  it('другой разворот — сворачивается', () => {
    expect(afterNav('s1', 's1', 's2')).toBeNull()
  })
  it('нечего сворачивать — null', () => {
    expect(afterNav(null, 's1', 's2')).toBeNull()
  })
})

describe('isHidden', () => {
  it('12-е фото из 13 скрыто, 11-е видно', () => {
    const s = sp(13)
    expect(isHidden(s, 'p10', null)).toBe(false)
    expect(isHidden(s, 'p11', null)).toBe(true)
  })
  it('раскрытая группа ничего не прячет', () => {
    expect(isHidden(sp(13), 'p12', 's1')).toBe(false)
  })
  it('чужое фото и короткая группа — не скрыто', () => {
    expect(isHidden(sp(13), 'zz', null)).toBe(false)
    expect(isHidden(sp(12), 'p11', null)).toBe(false)
  })
})

// Жалоба владельца 14.10: лист виньетки 20,4×30,4 с 25 местами рисовался полосой из 11 квадратиков и «+14» (~560 px).
describe('лист виньетки в ленте', () => {
  const vig = (n: number, e: number): SpreadView => ({ ...sp(n, e), kind: 'vignette', w: 204, h: 304 }) as SpreadView
  it('одна миниатюра в пропорциях листа, а не плитки мест', () => {
    const s = vig(0, 25)
    expect(asSheet(s)).toBe(true)
    expect(sheetThumbW(s)).toBe(Math.round((40 * 204) / 304)) // высота без рамки 40
    expect(shownW(s, null)).toBe(sheetThumbW(s) + 4 + 2 * PAD)
    expect(shownW(s, null)).toBeLessThan(chipW(1) * 2) // не полоса
  })
  it('ширина не зависит от числа мест и фото', () => {
    expect(shownW(vig(0, 60), null)).toBe(shownW(vig(30, 30), null))
  })
  it('ничего не прячет — раскрывать нечего', () => {
    expect(isHidden(vig(20, 5), 'p15', null)).toBe(false)
  })
  it('обычный разворот с местами — прежние плитки (027)', () => {
    expect(asSheet(sp(0, 25))).toBe(false)
    expect(asSheet({ kind: '' })).toBe(false)
  })
  it('нет размеров — квадрат', () => {
    expect(sheetThumbW({ w: 0, h: 0 })).toBe(40)
  })
})

describe('cutHeldNext (094 §1)', () => {
  afterEach(() => setMacForTest(null))
  const ev = (type: 'keydown' | 'keyup' | 'pointermove' | 'pointerdown', ctrlKey: boolean, metaKey = false) => ({ type, ctrlKey, metaKey })

  it('Ctrl на Windows и ⌘ на Mac — да, Ctrl на Mac — нет', () => {
    setMacForTest(false)
    expect(cutHeldNext(ev('keydown', true))).toBe(true)
    expect(cutHeldNext(ev('keyup', false))).toBe(false)
    setMacForTest(true)
    expect(cutHeldNext(ev('keydown', false, true))).toBe(true)
    expect(cutHeldNext(ev('keydown', true))).toBe(false) // Ctrl+щелчок на Mac — правая кнопка
  })
  it('blur и скрытие окна — нет', () => {
    expect(cutHeldNext({ type: 'blur' })).toBe(false)
    expect(cutHeldNext({ type: 'hidden' })).toBe(false)
  })
  it('pointermove без Ctrl снимает признак после потерянного keyup', () => {
    setMacForTest(false)
    let held = cutHeldNext(ev('keydown', true))
    expect(held).toBe(true)
    // keyup ушёл в другое окно (Alt+Tab) — первое же движение мыши несёт настоящее состояние
    held = cutHeldNext(ev('pointermove', false))
    expect(held).toBe(false)
    expect(cutHeldNext(ev('pointerdown', true))).toBe(true)
  })
})
