import { describe, expect, it } from 'vitest'
import { coveredBelow, nextBelow, pressPlan, sameItem, stackAt, type PickItem, type PressCtx } from './pickunder'
import type { CaptionView, FrameView, PlaceView, SpreadView, TextView } from '../../base/core/types'

const SW = 400
const PH = 300
const all = () => true

const style = { font: 'f', size: 12, color: '#000', align: 'left' as const }
const text = (id: string, x: number, y: number, w: number, h: number, z: number, rot = 0): TextView =>
  ({ id, x, y, w, h, rot, z, style, lines: [] }) as unknown as TextView
const caption = (photo: string, x: number, y: number, w: number, h: number, z: number): CaptionView =>
  ({ ...text('', x, y, w, h, z), photo }) as unknown as CaptionView
const place = (id: string, photo: string, x: number, y: number, w: number, h: number, z: number, rot = 0): PlaceView =>
  ({ id, photo, x, y, w, h, z, rot, fit: '', spine: false }) as PlaceView

function grid(places: PlaceView[], texts: TextView[] = [], captions: CaptionView[] = []): SpreadView {
  return { id: 's', grid: true, places, texts, captions, photos: [], rects: [], frames: [] } as unknown as SpreadView
}

const refs = (st: PickItem[]) => st.map((i) => `${i.kind}:${i.ref}`)

describe('stackAt', () => {
  it('ручная — места, подписи, тексты по z сверху вниз', () => {
    const s = grid(
      [place('p1', 'a', 0, 0, 200, 200, 1), place('p2', '', 50, 50, 100, 100, 4)],
      [text('t1', 60, 60, 50, 20, 3)],
      [caption('a', 10, 55, 100, 20, 2)],
    )
    expect(refs(stackAt(s, { x: 70, y: 70 }, SW, PH, all))).toEqual(['place:p2', 'text:t1', 'caption:a', 'photo:a'])
    // Снаружи текста и подписи — только места.
    expect(refs(stackAt(s, { x: 140, y: 140 }, SW, PH, all))).toEqual(['place:p2', 'photo:a'])
  })

  it('повёрнутое место — по повёрнутой рамке', () => {
    // 100×20 вокруг центра (100, 100) на 90°: занимает x 90..110, y 50..150.
    const s = grid([place('p', '', 50, 90, 100, 20, 1, 90)])
    expect(stackAt(s, { x: 100, y: 60 }, SW, PH, all)).toHaveLength(1)
    // Точка внутри неповёрнутой рамки, но вне повёрнутой.
    expect(stackAt(s, { x: 60, y: 100 }, SW, PH, all)).toHaveLength(0)
  })

  it('стикер (048) — в общем стеке по z, рядом с местами и текстом', () => {
    const stk = (id: string, x: number, y: number, w: number, h: number, z: number, rot = 0) => ({ id, x, y, w, h, z, rot })
    const s = {
      ...grid([place('p1', 'a', 0, 0, 200, 200, 1)], [text('t1', 60, 60, 50, 20, 4)]),
      stickers: [stk('k1', 50, 50, 100, 100, 3)],
    } as unknown as SpreadView
    expect(refs(stackAt(s, { x: 70, y: 70 }, SW, PH, all))).toEqual(['text:t1', 'sticker:k1', 'photo:a'])
    // Стикер под фото («Слой ▸ Назад»): выше лежит фото
    const under = { ...s, stickers: [stk('k1', 50, 50, 100, 100, 0)] } as unknown as SpreadView
    expect(refs(stackAt(under, { x: 140, y: 140 }, SW, PH, all))).toEqual(['photo:a', 'sticker:k1'])
  })

  it('повёрнутый стикер — поворот вокруг центра', () => {
    // 100×20 вокруг центра (100, 100) на 90°: занимает x 90..110, y 50..150
    const s = { ...grid([]), stickers: [{ id: 'k', x: 50, y: 90, w: 100, h: 20, z: 1, rot: 90 }] } as unknown as SpreadView
    expect(refs(stackAt(s, { x: 100, y: 60 }, SW, PH, all))).toEqual(['sticker:k'])
    expect(stackAt(s, { x: 60, y: 100 }, SW, PH, all)).toHaveLength(0)
  })

  it('уровень не даёт выделить стикер — can отсекает его', () => {
    const s = { ...grid([]), stickers: [{ id: 'k', x: 0, y: 0, w: 50, h: 50, z: 1, rot: 0 }] } as unknown as SpreadView
    expect(stackAt(s, { x: 10, y: 10 }, SW, PH, (k) => k !== 'sticker')).toHaveLength(0)
  })

  it('повёрнутый текст — поворот вокруг угла x, y', () => {
    // Угол (100, 100), 100×20 на 90° по часовой: занимает x 80..100, y 100..200.
    const s = grid([], [text('t', 100, 100, 100, 20, 1, 90)])
    expect(stackAt(s, { x: 90, y: 150 }, SW, PH, all)).toHaveLength(1)
    expect(stackAt(s, { x: 150, y: 110 }, SW, PH, all)).toHaveLength(0)
  })

  it('авто — ячейки по rects, тексты выше', () => {
    const s = {
      id: 's',
      grid: false,
      photos: ['a', 'b'],
      rects: [
        { x: 0, y: 0, w: 200, h: 300 },
        { x: 200, y: 0, w: 200, h: 300 },
      ],
      frames: [{ z: 1, cellRot: 0 } as FrameView, { z: 2, cellRot: 0 } as FrameView],
      texts: [text('t', 150, 100, 100, 20, 3)],
      captions: [],
      places: null,
    } as unknown as SpreadView
    expect(refs(stackAt(s, { x: 100, y: 110 }, SW, PH, all))).toEqual(['photo:a'])
    expect(refs(stackAt(s, { x: 160, y: 110 }, SW, PH, all))).toEqual(['text:t', 'photo:a'])
    expect(refs(stackAt(s, { x: 210, y: 110 }, SW, PH, all))).toEqual(['text:t', 'photo:b'])
  })

  it('недоступное уровню выпадает', () => {
    const s = grid([place('p1', 'a', 0, 0, 200, 200, 1), place('p2', '', 0, 0, 100, 100, 2)], [text('t', 0, 0, 50, 20, 3)])
    const can = (k: string) => k === 'photo'
    expect(refs(stackAt(s, { x: 10, y: 10 }, SW, PH, can))).toEqual(['photo:a'])
  })

  it('точка вне разворота — пусто', () => {
    // Повёрнутое место вылетает за край, но .content обрезает вылет.
    const s = grid([place('p', '', 0, 0, 200, 200, 1, 45)])
    expect(stackAt(s, { x: -5, y: 10 }, SW, PH, all)).toEqual([])
    expect(stackAt(s, { x: 10, y: PH + 1 }, SW, PH, all)).toEqual([])
    expect(stackAt(s, { x: SW + 1, y: 10 }, SW, PH, all)).toEqual([])
  })
})

describe('форма места (спека 044)', () => {
  const shaped = (id: string, photo: string, shape: string, radius: number, z: number, x = 0, y = 0, w = 100, h = 100): PlaceView => ({
    ...place(id, photo, x, y, w, h, z),
    shape,
    radius,
  })
  it('угол габарита круга не берёт место: берёт то, что под ним', () => {
    // Круг 100×100 лежит поверх прямоугольного места 200×200.
    const s = grid([place('under', 'u', 0, 0, 200, 200, 1), shaped('circle', 'c', 'oval', 0, 2)])
    expect(refs(stackAt(s, { x: 50, y: 50 }, SW, PH, all))).toEqual(['photo:c', 'photo:u'])
    expect(refs(stackAt(s, { x: 3, y: 3 }, SW, PH, all))).toEqual(['photo:u']) // угол габарита: круг не берёт
    expect(refs(stackAt(s, { x: 85, y: 85 }, SW, PH, all))).toEqual(['photo:c', 'photo:u']) // 35;35 от центра — внутри
  })
  it('пустое место-круг: угол — мимо, край — внутри', () => {
    const s = grid([shaped('c', '', 'oval', 0, 1)])
    expect(stackAt(s, { x: 2, y: 2 }, SW, PH, all)).toHaveLength(0)
    expect(stackAt(s, { x: 50, y: 1 }, SW, PH, all)).toHaveLength(1)
  })
  it('скругление: самый угол — мимо, дальше — внутри', () => {
    const s = grid([shaped('r', '', '', 20, 1)])
    expect(stackAt(s, { x: 1, y: 1 }, SW, PH, all)).toHaveLength(0)
    expect(stackAt(s, { x: 8, y: 8 }, SW, PH, all)).toHaveLength(1)
  })
  it('повёрнутый круг: форма в осях места', () => {
    // Овал 100×40 вокруг центра (100, 100), поворот 90°: на экране вытянут по вертикали.
    const s = grid([{ ...shaped('o', '', 'oval', 0, 1, 50, 80, 100, 40), rot: 90 }])
    expect(stackAt(s, { x: 100, y: 60 }, SW, PH, all)).toHaveLength(1)
    expect(stackAt(s, { x: 118, y: 52 }, SW, PH, all)).toHaveLength(0) // угол повёрнутой рамки (x 80..120, y 50..150), вне формы
  })
  it('«Авто»: форма из кадра', () => {
    const s = {
      id: 's',
      grid: false,
      photos: ['a'],
      rects: [{ x: 0, y: 0, w: 100, h: 100 }],
      frames: [{ z: 1, cellRot: 0, shape: 'oval', radius: 0 } as FrameView],
      texts: [],
      captions: [],
      places: null,
    } as unknown as SpreadView
    expect(stackAt(s, { x: 2, y: 2 }, SW, PH, all)).toHaveLength(0)
    expect(stackAt(s, { x: 50, y: 50 }, SW, PH, all)).toHaveLength(1)
  })
  it('coveredBelow: угол нижнего вне круга сверху — не закрыт', () => {
    const circle: PickItem = { kind: 'photo', ref: 'c', z: 2, box: { x: 0, y: 0, w: 100, h: 100, rot: 0 }, shape: 'oval' }
    const low = item('text', 'low', 1, 0, 0, 100, 100)
    expect(coveredBelow([circle, low])).toBe(false)
    expect(coveredBelow([circle, item('text', 'in', 1, 40, 40, 20, 20)])).toBe(true)
  })
})

const item = (kind: PickItem['kind'], ref: string, z: number, x = 0, y = 0, w = 100, h = 100): PickItem => ({
  kind,
  ref,
  z,
  box: { x, y, w, h, rot: 0 },
})

describe('nextBelow', () => {
  const a = item('text', 'a', 3)
  const b = item('photo', 'b', 2)
  const c = item('place', 'c', 1)
  it('по кругу; текущего нет под курсором — второй сверху; меньше двух — null', () => {
    const st = [a, b, c]
    expect(nextBelow(st, a)).toBe(b)
    expect(nextBelow(st, b)).toBe(c)
    expect(nextBelow(st, c)).toBe(a)
    expect(nextBelow(st, null)).toBe(b)
    expect(nextBelow(st, item('text', 'zzz', 0))).toBe(b)
    expect(nextBelow([a], a)).toBeNull()
    expect(nextBelow([], null)).toBeNull()
  })
  it('sameItem сравнивает вид и ref', () => {
    expect(sameItem(a, { ...a })).toBe(true)
    expect(sameItem(a, item('photo', 'a', 3))).toBe(false)
    expect(sameItem(a, null)).toBe(false)
  })
})

describe('coveredBelow', () => {
  it('целиком закрытый — да, частично — нет', () => {
    const top = item('photo', 'top', 2, 0, 0, 200, 200)
    expect(coveredBelow([top, item('text', 'in', 1, 50, 50, 50, 20)])).toBe(true)
    expect(coveredBelow([top, item('text', 'edge', 1, 150, 150, 100, 20)])).toBe(false)
    expect(coveredBelow([top])).toBe(false)
  })
  it('рамка во всю ширину верхней (край в край) — закрыта', () => {
    expect(coveredBelow([item('photo', 'top', 2, 0, 0, 200, 200), item('place', 'same', 1, 0, 0, 200, 200)])).toBe(true)
  })
  it('повёрнутый верхний: углы нижнего проверяются по его повёрнутой рамке', () => {
    // Верхний 200×20 на 90° вокруг угла (100, 0): x 80..100, y 0..200.
    const top: PickItem = { kind: 'photo', ref: 'top', z: 2, box: { x: 100, y: 0, w: 200, h: 20, rot: 90 } }
    expect(coveredBelow([top, item('text', 'in', 1, 82, 10, 10, 10)])).toBe(true)
    expect(coveredBelow([top, item('text', 'out', 1, 60, 10, 30, 10)])).toBe(false)
  })
})

describe('pressPlan', () => {
  const top = item('photo', 'top', 3, 0, 0, 200, 200)
  const mid = item('text', 'mid', 2, 50, 50, 50, 20)
  const low = item('place', 'low', 1, 60, 60, 20, 20)
  const stack = [top, mid, low]
  const base: PressCtx = { button: 0, alt: false, additive: false, stack, cur: null, picked: null }
  const pass = { press: 'pass', click: 'pass', hint: false }

  it('1: не левая кнопка или Shift/Ctrl — как обычно', () => {
    expect(pressPlan({ ...base, alt: true, button: 2 })).toEqual(pass)
    expect(pressPlan({ ...base, alt: true, additive: true })).toEqual(pass)
  })
  it('2: Alt и меньше двух под курсором — как обычный щелчок', () => {
    expect(pressPlan({ ...base, alt: true, stack: [top] })).toEqual(pass)
    expect(pressPlan({ ...base, alt: true, stack: [] })).toEqual(pass)
  })
  it('3: Alt, текущий закрыт — двигаем текущий, щелчок выбирает следующий', () => {
    expect(pressPlan({ ...base, alt: true, cur: mid })).toEqual({ press: 'move', move: mid, click: 'select', select: low, hint: false })
  })
  it('4: Alt иначе — перенос верхнего как сейчас, щелчок по кругу', () => {
    // Текущий — верхний: следующий второй.
    expect(pressPlan({ ...base, alt: true, cur: top })).toEqual({ press: 'pass', click: 'select', select: mid, hint: false })
    // Текущего под курсором нет: второй сверху.
    expect(pressPlan({ ...base, alt: true, cur: item('text', 'far', 9) })).toEqual({
      press: 'pass',
      click: 'select',
      select: mid,
      hint: false,
    })
    // Группа (cur = null).
    expect(pressPlan({ ...base, alt: true })).toEqual({ press: 'pass', click: 'select', select: mid, hint: false })
  })
  it('5: выбранное Alt+щелчком двигается обычным нажатием, щелчок выбирает верхний', () => {
    expect(pressPlan({ ...base, cur: mid, picked: mid })).toEqual({ press: 'move', move: mid, click: 'select', select: top, hint: false })
    // Выделен не через Alt — привилегии нет.
    expect(pressPlan({ ...base, cur: mid, picked: low })).toMatchObject({ press: 'pass', click: 'pass' })
    // Верхний сам — ничего особого.
    expect(pressPlan({ ...base, cur: top, picked: top })).toMatchObject({ press: 'pass', click: 'pass' })
  })
  it('стикер (048) в стеке ведёт себя как остальные объекты: Alt+щелчок по кругу, перенос закрытого', () => {
    const stk = item('sticker', 'k', 2, 50, 50, 50, 20)
    const s3 = [top, stk, low]
    expect(pressPlan({ ...base, stack: s3, alt: true })).toEqual({ press: 'pass', click: 'select', select: stk, hint: false })
    expect(pressPlan({ ...base, stack: s3, alt: true, cur: stk })).toEqual({
      press: 'move',
      move: stk,
      click: 'select',
      select: low,
      hint: false,
    })
    expect(pressPlan({ ...base, stack: s3, cur: stk, picked: stk })).toMatchObject({ press: 'move', move: stk, select: top })
    expect(nextBelow(s3, stk)).toBe(low)
    expect(sameItem(stk, item('sticker', 'k', 9))).toBe(true)
    expect(sameItem(stk, item('text', 'k', 2))).toBe(false)
  })
  it('6: обычный щелчок; подсказка, если под верхним скрыт объект', () => {
    expect(pressPlan(base).hint).toBe(true) // mid целиком под top
    expect(pressPlan({ ...base, stack: [top] }).hint).toBe(false)
    const partly = item('text', 'part', 1, 150, 150, 100, 20)
    expect(pressPlan({ ...base, stack: [top, partly] }).hint).toBe(false)
  })
})
