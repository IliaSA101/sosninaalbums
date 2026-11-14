import { describe, expect, it } from 'vitest'
import { EMPTY } from '../places/groupsel'
import { marqueeHits, marqueePick, rectOf, toSpreadMM, touches } from './marquee'
import type { PlaceView, SpreadView, TextView } from '../../base/core/types'

const text = (id: string, x: number, y: number, w: number, h: number, rot = 0) => ({ id, x, y, w, h, rot }) as TextView
const place = (id: string, photo: string, x: number, y: number, w: number, h: number) =>
  ({ id, photo, x, y, w, h, fit: '', spine: false }) as PlaceView

function spread(over: Partial<SpreadView>): SpreadView {
  return { id: 's', photos: [], rects: [], texts: [], places: null, grid: false, ...over } as unknown as SpreadView
}

describe('rectOf и toSpreadMM', () => {
  it('углы в любом порядке', () => {
    expect(rectOf({ x: 50, y: 40 }, { x: 10, y: 10 })).toEqual({ x: 10, y: 10, w: 40, h: 30 })
  })
  it('px → мм по ширине разворота; серое поле даёт отрицательные мм', () => {
    const box = { left: 100, top: 50, width: 500 }
    expect(toSpreadMM({ x: 50, y: 50, w: 100, h: 50 }, box, 250)).toEqual({ x: -25, y: 0, w: 50, h: 25 })
  })
})

describe('touches', () => {
  const a = { x: 0, y: 0, w: 10, h: 10 }
  it('пересечение', () => expect(touches(a, { x: 9, y: 9, w: 5, h: 5 })).toBe(true))
  it('касание краем — нет', () => expect(touches(a, { x: 10, y: 0, w: 5, h: 5 })).toBe(false))
  it('вложение', () => expect(touches(a, { x: 2, y: 2, w: 1, h: 1 })).toBe(true))
})

describe('marqueeHits', () => {
  it('«Авто»: ячейки фото', () => {
    const s = spread({
      photos: ['a', 'b'],
      rects: [
        { x: 0, y: 0, w: 100, h: 100 },
        { x: 120, y: 0, w: 100, h: 100 },
      ],
    })
    expect(marqueeHits(s, { x: 90, y: 10, w: 40, h: 10 })).toEqual({ refs: ['a', 'b'], texts: [] })
    expect(marqueeHits(s, { x: 130, y: 10, w: 10, h: 10 })).toEqual({ refs: ['b'], texts: [] })
    expect(marqueeHits(s, { x: 300, y: 10, w: 10, h: 10 })).toEqual({ refs: [], texts: [] })
  })
  it('«Ручная»: занятое место — по фото, пустое — по ID, в порядке мест', () => {
    const s = spread({
      grid: true,
      photos: ['a'],
      rects: [{ x: 0, y: 0, w: 50, h: 50 }],
      places: [place('p1', 'a', 0, 0, 50, 50), place('p2', '', 60, 0, 50, 50), place('p3', '', 200, 0, 50, 50)],
    })
    expect(marqueeHits(s, { x: 40, y: 10, w: 30, h: 10 })).toEqual({ refs: ['a', 'p2'], texts: [] })
  })
  it('повёрнутый текст — по описанному прямоугольнику', () => {
    // 100×10, повёрнут на 90° вокруг (200, 100): занимает x 190..200, y 100..200
    const s = spread({ texts: [text('t', 200, 100, 100, 10, 90)] })
    expect(marqueeHits(s, { x: 192, y: 190, w: 5, h: 5 }).texts).toEqual(['t'])
    expect(marqueeHits(s, { x: 220, y: 105, w: 20, h: 5 }).texts).toEqual([])
  })
  it('нет разворота — пусто', () => expect(marqueeHits(undefined, { x: 0, y: 0, w: 1, h: 1 })).toEqual({ refs: [], texts: [] }))
})

describe('стикеры (048)', () => {
  const sticker = (id: string, x: number, y: number, w: number, h: number, rot = 0) => ({ id, x, y, w, h, rot })
  it('рамка берёт стикер по описанному прямоугольнику; без стикеров поля stickers нет', () => {
    const s = spread({ stickers: [sticker('k1', 100, 100, 40, 20), sticker('k2', 300, 100, 40, 20)] as never })
    expect(marqueeHits(s, { x: 90, y: 90, w: 30, h: 30 })).toEqual({ refs: [], texts: [], stickers: ['k1'] })
    expect(marqueeHits(s, { x: 0, y: 0, w: 10, h: 10 })).toEqual({ refs: [], texts: [] })
  })
  it('повёрнутый стикер ловится по описанному прямоугольнику', () => {
    // 40×20 на 90° вокруг центра (120, 110): занимает x 110..130, y 90..130
    const s = spread({ stickers: [sticker('k1', 100, 100, 40, 20, 90)] as never })
    expect(marqueeHits(s, { x: 111, y: 92, w: 4, h: 4 }).stickers).toEqual(['k1'])
    expect(marqueeHits(s, { x: 101, y: 101, w: 4, h: 4 }).stickers).toBeUndefined()
  })
  it('marqueePick: заменяет или добавляет, один стикер — выделение из одного объекта', () => {
    expect(marqueePick(EMPTY, { refs: [], texts: [], stickers: ['k1'] }, false)).toEqual({
      refs: [],
      texts: [],
      text: null,
      stickers: ['k1'],
    })
    const cur = { refs: ['a'], texts: [], text: null, stickers: ['k1'] }
    expect(marqueePick(cur, { refs: [], texts: [], stickers: ['k1', 'k2'] }, true).stickers).toEqual(['k1', 'k2'])
    expect(marqueePick(cur, { refs: [], texts: [] }, false)).toEqual(EMPTY)
    expect(marqueePick(cur, { refs: [], texts: [] }, true).stickers).toEqual(['k1'])
  })
  it('текст и стикер вместе — группа: текст не одиночный', () => {
    const r = marqueePick(EMPTY, { refs: [], texts: ['t'], stickers: ['k1'] }, false)
    expect(r.text).toBeNull()
    expect(r.texts).toEqual(['t'])
  })
})

describe('marqueePick', () => {
  const hit = { refs: ['b'], texts: ['t2'] }
  it('без Shift заменяет выделение', () => {
    expect(marqueePick({ refs: ['a'], texts: ['t1'], text: null }, hit, false)).toEqual({ refs: ['b'], texts: ['t2'], text: null })
  })
  it('с Shift добавляет, без повторов', () => {
    const r = marqueePick({ refs: ['a', 'b'], texts: [], text: null }, hit, true)
    expect(r.refs).toEqual(['a', 'b'])
    expect(r.texts).toEqual(['t2'])
  })
  it('с Shift одиночный текст уходит в группу', () => {
    expect(marqueePick({ refs: [], texts: [], text: 't1' }, { refs: ['b'], texts: [] }, true)).toEqual({
      refs: ['b'],
      texts: ['t1'],
      text: null,
    })
  })
  it('нашли один текст без фото — он одиночный', () => {
    expect(marqueePick(EMPTY, { refs: [], texts: ['t'] }, false)).toEqual({ refs: [], texts: [], text: 't' })
  })
  it('пустая рамка без Shift снимает выделение, с Shift — оставляет', () => {
    expect(marqueePick({ refs: ['a'], texts: [], text: null }, { refs: [], texts: [] }, false)).toEqual(EMPTY)
    expect(marqueePick({ refs: ['a'], texts: [], text: null }, { refs: [], texts: [] }, true).refs).toEqual(['a'])
  })
})
