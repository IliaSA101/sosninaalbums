import { describe, expect, it } from 'vitest'
import {
  axisLock,
  canFitPhoto,
  fieldBox,
  fmtMM,
  gridSession,
  moveSel,
  pickPlace,
  refBox,
  refsBoxes,
  resizeSel,
  sameBox,
  sampleFirst,
  type SnapCtx,
} from './gridedit'
import type { Rect, SpreadView } from '../core/types'

const B = { w: 560, h: 280 }
const params = { pageW: 280, pageH: 280, margin: 12, rects: [], gap: 4 }
const ctx = (others: Rect[], thr = 3): SnapCtx => ({ others, params, thr })

describe('axisLock', () => {
  it('без Shift сдвиг свободный', () => {
    expect(axisLock(7, -3, false)).toEqual({ dx: 7, dy: -3, axis: null })
  })
  it('с Shift остаётся больший сдвиг, меньший обнуляется', () => {
    expect(axisLock(7, -3, true)).toEqual({ dx: 7, dy: 0, axis: 'x' })
    expect(axisLock(-2, 9, true)).toEqual({ dx: 0, dy: 9, axis: 'y' })
  })
  it('ничья — по X; нулевой сдвиг не ломается', () => {
    expect(axisLock(4, -4, true)).toEqual({ dx: 4, dy: 0, axis: 'x' })
    expect(axisLock(0, 0, true)).toEqual({ dx: 0, dy: 0, axis: 'x' })
  })
})

describe('moveSel', () => {
  const a: Rect = { x: 50, y: 50, w: 100, h: 60 }
  it('без прилипания — обычный перенос', () => {
    const r = moveSel([a], 10, 5, B, null)
    expect(r.boxes[0]).toEqual({ x: 60, y: 55, w: 100, h: 60 })
    expect(r.guideX).toBeNull()
  })
  it('не выходит за разворот', () => {
    expect(moveSel([a], -500, 900, B, null).boxes[0]).toEqual({ x: 0, y: 220, w: 100, h: 60 })
  })
  it('прилипает к краю соседа и даёт направляющую', () => {
    const other: Rect = { x: 300, y: 0, w: 50, h: 40 }
    const r = moveSel([a], 148, 0, B, ctx([other])) // левый край 198 → не рядом; правый 298 → к 300
    expect(r.boxes[0].x + 100).toBeCloseTo(300)
    expect(r.guideX).toBe(300)
  })
  it('прилипает образец, остальные едут с ним', () => {
    const b: Rect = { x: 200, y: 50, w: 40, h: 40 }
    const r = moveSel([a, b], 131, 0, B, ctx([])) // правый край образца 281 → к 280
    expect(r.boxes[1].x - r.boxes[0].x).toBe(150)
    expect(r.boxes[0].x + 100).toBeCloseTo(280)
  })
  it('удержанная ось: по другой оси прилипание не уводит', () => {
    // Верх места 52 прилип бы к линии соседа (50), но ось X удержана — y остаётся.
    const other: Rect = { x: 300, y: 50, w: 50, h: 40 }
    const r = moveSel([a], 5, 2, B, ctx([other], 3), 'x')
    expect(r.boxes[0].y).toBe(52)
    expect(r.guideY).toBeNull()
    const free = moveSel([a], 5, 2, B, ctx([other], 3))
    expect(free.boxes[0].y).toBe(50)
  })
  it('зажим у края гасит направляющую', () => {
    const r = moveSel([{ x: 400, y: 50, w: 150, h: 60 }], 8, 0, B, ctx([{ x: 600, y: 0, w: 5, h: 5 }]))
    expect(r.boxes[0].x + 150).toBeLessThanOrEqual(560)
  })
})

describe('resizeSel', () => {
  const a: Rect = { x: 100, y: 100, w: 100, h: 50 }
  it('обычная ручка прилипает к линии', () => {
    const r = resizeSel(a, 'e', 79, 0, false, B, ctx([])) // правый край 279 → к 280
    expect(r.box.x + r.box.w).toBeCloseTo(280)
    expect(r.guideX).toBe(280)
  })
  it('замок — пропорции и без прилипания', () => {
    const r = resizeSel(a, 'se', 79, 0, true, B, ctx([]))
    expect(r.box.w / r.box.h).toBeCloseTo(2)
    expect(r.guideX).toBeNull()
  })
  it('не меньше минимума', () => {
    expect(resizeSel(a, 'e', -500, 0, false, B, null).box.w).toBe(5)
  })
})

describe('fieldBox', () => {
  const a: Rect = { x: 10, y: 20, w: 100, h: 50 }
  it('X и Y двигают', () => expect(fieldBox(a, 'x', 40, false, B)).toEqual({ x: 40, y: 20, w: 100, h: 50 }))
  it('X зажимается в разворот', () => expect(fieldBox(a, 'x', 999, false, B).x).toBe(460))
  it('Ш без замка', () => expect(fieldBox(a, 'w', 80, false, B)).toEqual({ x: 10, y: 20, w: 80, h: 50 }))
  it('Ш с замком масштабирует В', () => expect(fieldBox(a, 'w', 200, true, B)).toEqual({ x: 10, y: 20, w: 200, h: 100 }))
  it('В с замком масштабирует Ш', () => expect(fieldBox(a, 'h', 25, true, B)).toEqual({ x: 10, y: 20, w: 50, h: 25 }))
  it('минимум 5 мм', () => expect(fieldBox(a, 'w', 1, false, B).w).toBe(5))
})

describe('pickPlace', () => {
  it('клик — одно место', () => expect(pickPlace(['a', 'b'], 'c', false)).toEqual(['c']))
  it('Shift добавляет в конец', () => expect(pickPlace(['a'], 'b', true)).toEqual(['a', 'b']))
  it('Shift по выделенному снимает', () => expect(pickPlace(['a', 'b'], 'a', true)).toEqual(['b']))
})

describe('мелочи', () => {
  it('sameBox терпит округление', () => expect(sameBox({ x: 1, y: 1, w: 5, h: 5 }, { x: 1.004, y: 1, w: 5, h: 5 })).toBe(true))
  it('sameBox видит сдвиг', () => expect(sameBox({ x: 1, y: 1, w: 5, h: 5 }, { x: 1.1, y: 1, w: 5, h: 5 })).toBe(false))
  it('fmtMM — один знак', () => expect(fmtMM(12.345)).toBe('12.3'))
  it('сессия подходит Go', () => expect(gridSession(1700000000000)).toMatch(/^[A-Za-z0-9-]{1,32}$/))
})

const auto = {
  grid: false,
  photos: ['pa', 'pb'],
  rects: [
    { x: 0, y: 0, w: 100, h: 80 },
    { x: 110, y: 0, w: 60, h: 80 },
  ],
  places: null,
} as unknown as SpreadView
const gridSp = {
  grid: true,
  photos: ['pb'],
  rects: [{ x: 5, y: 5, w: 30, h: 20 }],
  places: [
    { id: 'plA', x: 0, y: 0, w: 50, h: 40, photo: '', fit: '', spine: false },
    { id: 'plB', x: 60, y: 10, w: 70, h: 50, photo: 'pb', fit: 'contain', spine: false },
  ],
} as unknown as SpreadView

describe('refBox', () => {
  it('«Авто»: ячейка по индексу фото', () => {
    expect(refBox(auto, 'pb')).toMatchObject({ x: 110, y: 0, w: 60, h: 80, ref: 'pb', id: 'pb', photo: 'pb', fit: '' })
  })
  it('«Авто»: чужой ref и ID места — нет', () => {
    expect(refBox(auto, 'zz')).toBeNull()
    expect(refBox(auto, '')).toBeNull()
  })
  it('сетка: место по ID, в том числе пустое', () => {
    expect(refBox(gridSp, 'plA')).toMatchObject({ x: 0, w: 50, id: 'plA', photo: '' })
  })
  it('сетка: место по ID фото; rects не используются', () => {
    expect(refBox(gridSp, 'pb')).toMatchObject({ x: 60, y: 10, w: 70, h: 50, id: 'plB', ref: 'pb', fit: 'contain' })
  })
  it('нет разворота — null', () => expect(refBox(undefined, 'pb')).toBeNull())
})

describe('refsBoxes', () => {
  it('порядок выделения, исчезнувшие выпадают', () => {
    expect(refsBoxes(gridSp, ['plB', 'xx', 'plA']).map((b) => b.id)).toEqual(['plB', 'plA'])
  })
  it('фото и его место — одно выделение', () => {
    expect(refsBoxes(gridSp, ['pb', 'plB']).map((b) => b.ref)).toEqual(['pb'])
  })
})

describe('canFitPhoto', () => {
  it('те же пропорции — да', () => expect(canFitPhoto({ x: 0, y: 0, w: 150, h: 100 }, 1.5)).toBe(true))
  it('в пределах 0.5 % — да, дальше — нет', () => {
    expect(canFitPhoto({ x: 0, y: 0, w: 150.5, h: 100 }, 1.5)).toBe(true)
    expect(canFitPhoto({ x: 0, y: 0, w: 152, h: 100 }, 1.5)).toBe(false)
  })
  it('размер фото неизвестен — нечего предлагать', () => expect(canFitPhoto({ x: 0, y: 0, w: 5, h: 9 }, 0)).toBe(true))
})

describe('sampleFirst', () => {
  it('последнее щёлкнутое выходит вперёд, остальной порядок сохраняется', () => {
    expect(sampleFirst(['a', 'b', 'c'], 'c')).toEqual(['c', 'a', 'b'])
  })
  it('нет образца или его нет в списке — порядок как был', () => {
    const ids = ['a', 'b']
    expect(sampleFirst(ids, null)).toBe(ids)
    expect(sampleFirst(ids, 'x')).toBe(ids)
    expect(sampleFirst(ids, 'a')).toBe(ids)
  })
})
