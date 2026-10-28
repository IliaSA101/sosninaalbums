import { describe, expect, it } from 'vitest'
import type { SnapCtx } from '../../base/geom/gridedit'
import type { SpreadView, StickerView } from '../../base/core/types'
import {
  MIN_SIDE,
  STICKER_NEWER,
  artSrc,
  bounds,
  boxOf,
  centerIn,
  inksOf,
  isLost,
  longSidePx,
  maxSide,
  moveBox,
  nudgeBox,
  resizeBox,
  sameBox,
  stKey,
  stickerById,
  stickerName,
  stickerPx,
  stickerOverflow,
  stickersOf,
} from './sticker'

const SW = 600
const PH = 300
const box = (over: Partial<ReturnType<typeof boxOf>> = {}) => ({ x: 100, y: 100, w: 40, h: 20, rot: 0, ...over })

const st = (over: Partial<StickerView> = {}): StickerView => ({
  id: 'k1',
  x: 100,
  y: 100,
  w: 40,
  h: 20,
  rot: 0,
  flipH: false,
  flipV: false,
  fade: 0,
  z: 5,
  photo: '', // Go шлёт всегда, у процедурного — пусто
  stretch: false,
  name: 'Сердце',
  inks: 2,
  url: '/media/gen/art?k=heart&v=1',
  gen: true,
  aspect: 2,
  dpi: 0,
  known: true,
  art: { kind: 'heart', inks: ['#aa0000', '#00aa00', '#0000aa'] },
  ...over,
})

describe('рамка: центр и описанный прямоугольник', () => {
  it('центр зажат в лист, сама рамка может вылезать за край', () => {
    const r = centerIn(box({ x: 700 }), SW, PH)
    expect(r.x + r.w / 2).toBe(SW)
    expect(r.x + r.w).toBeGreaterThan(SW)
    const l = centerIn(box({ x: -500, y: -500 }), SW, PH)
    expect(l.x + l.w / 2).toBe(0)
    expect(l.y + l.h / 2).toBe(0)
  })
  it('внутри листа рамка не меняется', () => {
    expect(centerIn(box(), SW, PH)).toEqual(box())
  })
  it('bounds: без поворота сама рамка, на 90° — стороны меняются местами вокруг центра', () => {
    expect(bounds(box())).toEqual({ x: 100, y: 100, w: 40, h: 20 })
    const b = bounds(box({ rot: 90 }))
    expect(b.w).toBeCloseTo(20, 6)
    expect(b.h).toBeCloseTo(40, 6)
    expect(b.x).toBeCloseTo(110, 6)
    expect(b.y).toBeCloseTo(90, 6)
  })
  it('stickerOverflow: внутри листа и впритык — нет; вылет любой стороной или поворотом — да', () => {
    expect(stickerOverflow([], SW, PH)).toBe(false)
    expect(stickerOverflow([box()], SW, PH)).toBe(false)
    expect(stickerOverflow([box({ x: 0, y: 0, w: SW, h: PH })], SW, PH)).toBe(false)
    expect(stickerOverflow([box({ x: -10 })], SW, PH)).toBe(true)
    expect(stickerOverflow([box({ y: PH - 10 })], SW, PH)).toBe(true)
    expect(stickerOverflow([box({ x: SW - 30 })], SW, PH)).toBe(true)
    // 40×20 у верхнего края, на 90° встаёт 20×40 вокруг центра и вылезает вверх
    expect(stickerOverflow([box({ y: 0, rot: 90 })], SW, PH)).toBe(true)
    expect(stickerOverflow([box(), box({ x: SW })], SW, PH)).toBe(true)
  })
  it('sameBox: допуск на округление', () => {
    expect(sameBox(box(), box({ x: 100.004 }))).toBe(true)
    expect(sameBox(box(), box({ x: 100.5 }))).toBe(false)
    expect(sameBox(box(), box({ rot: 1 }))).toBe(false)
  })
  it('предел стороны — вдвое больше большей стороны листа', () => {
    expect(maxSide(SW, PH)).toBe(1200)
    expect(MIN_SIDE).toBe(3)
  })
})

describe('перенос', () => {
  const snap: SnapCtx = {
    others: [],
    params: { w: SW, h: PH, mid: 300, margin: 10, gap: 5, rects: [] },
    thr: 3,
  }
  it('без привязки — ровно на сдвиг', () => {
    const m = moveBox(box(), 30, -20, null, null, SW, PH)
    expect(m.box).toEqual(box({ x: 130, y: 80 }))
    expect(m.guideX).toBeNull()
    expect(m.guideY).toBeNull()
  })
  it('правый край прилипает к шву, направляющая показана', () => {
    const m = moveBox(box({ x: 100, w: 50, h: 50 }), 149, 0, snap, null, SW, PH)
    expect(m.box.x).toBe(250)
    expect(m.guideX).toBe(300)
  })
  it('Shift держит ось: по другой оси ни сдвига, ни прилипания', () => {
    const m = moveBox(box({ x: 100, w: 50, h: 50 }), 149, 0, snap, 'x', SW, PH)
    expect(m.box.y).toBe(100)
    expect(m.guideY).toBeNull()
    expect(m.box.x).toBe(250)
  })
  it('далеко за край: центр остаётся на листе', () => {
    const m = moveBox(box({ w: 50, h: 50 }), 10000, 10000, null, null, SW, PH)
    expect(m.box.x + 25).toBe(SW)
    expect(m.box.y + 25).toBe(PH)
  })
  it('зажатый центр уводит от линии — направляющей нет', () => {
    const m = moveBox(box({ x: 560, w: 50, h: 50 }), 100, 0, snap, null, SW, PH)
    expect(m.box.x + 25).toBe(SW)
    expect(m.guideX).toBeNull()
  })
})

describe('размер углом', () => {
  it('пропорция держится: угол се тянет вправо — высота растёт вместе', () => {
    const r = resizeBox(box(), 'se', 20, 0, false, SW, PH)
    expect(r).toMatchObject({ x: 100, y: 100, w: 60, h: 30 })
  })
  it('противоположный угол стоит: угол нз', () => {
    const r = resizeBox(box(), 'nw', -20, 0, false, SW, PH)
    expect(r).toMatchObject({ x: 80, y: 90, w: 60, h: 30 })
    expect(r.x + r.w).toBe(140)
    expect(r.y + r.h).toBe(120)
  })
  it('по большему относительному сдвигу', () => {
    const r = resizeBox(box(), 'se', 2, 20, false, SW, PH)
    expect(r.h).toBe(40)
    expect(r.w).toBe(80)
  })
  it('растяжимый (плашка, волна-лента): стороны свободны', () => {
    const r = resizeBox(box(), 'se', 20, 10, true, SW, PH)
    expect(r).toMatchObject({ x: 100, y: 100, w: 60, h: 30 })
    const t = resizeBox(box(), 'se', 20, 0, true, SW, PH)
    expect(t).toMatchObject({ w: 60, h: 20 })
  })
  it('предел снизу: 3 мм по меньшей стороне', () => {
    const r = resizeBox(box(), 'se', -1000, -1000, false, SW, PH)
    expect(Math.min(r.w, r.h)).toBeCloseTo(MIN_SIDE, 6)
    expect(r.w / r.h).toBeCloseTo(2, 6)
    const f = resizeBox(box(), 'se', -1000, -1000, true, SW, PH)
    expect(f.w).toBe(MIN_SIDE)
    expect(f.h).toBe(MIN_SIDE)
  })
  it('предел сверху: вдвое больше большей стороны листа', () => {
    const r = resizeBox(box(), 'se', 100000, 100000, false, SW, PH)
    expect(Math.max(r.w, r.h)).toBeLessThanOrEqual(maxSide(SW, PH))
    expect(r.w / r.h).toBeCloseTo(2, 2)
  })
  it('повёрнутый: сдвиг считается в осях стикера, центр уезжает вдоль них', () => {
    const r = resizeBox(box({ rot: 90 }), 'se', 0, 20, false, SW, PH)
    expect(r.w).toBe(60)
    expect(r.h).toBe(30)
    expect(r.x + r.w / 2).toBeCloseTo(115, 6)
    expect(r.y + r.h / 2).toBeCloseTo(120, 6)
    expect(r.rot).toBe(90)
  })
})

describe('стрелки', () => {
  it('1 мм, с Shift — 10; центр в листе; не стрелка — рамка та же', () => {
    expect(nudgeBox(box(), 'ArrowRight', false, SW, PH).x).toBe(101)
    expect(nudgeBox(box(), 'ArrowUp', true, SW, PH).y).toBe(90)
    expect(nudgeBox(box(), 'a', false, SW, PH)).toEqual(box())
    expect(nudgeBox(box({ x: 580 }), 'ArrowRight', true, SW, PH).x + 20).toBe(SW)
  })
})

describe('картинка и подписи', () => {
  it('ширина запроса: длинная сторона на экране, шаг 256, не больше 4096', () => {
    expect(stickerPx(100, 1)).toBe(256)
    expect(stickerPx(300, 2)).toBe(768)
    expect(stickerPx(1e6, 2)).toBe(4096)
  })
  it('longSidePx: доля длинной стороны в ширине листа × ширина на экране', () => {
    expect(longSidePx({ w: 60, h: 30 }, 600, 1200)).toBe(120)
    expect(longSidePx({ w: 30, h: 90 }, 600, 1200)).toBe(180)
    expect(longSidePx({ w: 30, h: 90 }, 0, 1200)).toBe(0)
  })
  it('адрес: px дописывается к адресу Go', () => {
    expect(artSrc('/media/gen/art?k=heart&v=1', 512)).toBe('/media/gen/art?k=heart&v=1&px=512')
  })
  it('цвета для пикеров — столько, сколько у генератора', () => {
    expect(inksOf(st())).toEqual(['#aa0000', '#00aa00'])
    expect(inksOf(st({ inks: 0 }))).toEqual([])
    expect(inksOf(st({ art: undefined }))).toEqual([])
  })
  it('имя: пустое у нового генератора — «Нужна новая версия программы»', () => {
    expect(stickerName(st())).toBe('Сердце')
    expect(stickerName(st({ name: '' }))).toBe('Стикер')
    expect(stickerName(st({ known: false, name: '' }))).toBe(STICKER_NEWER)
  })
  it('пропавший свой: фото в missing; у процедурного фото нет', () => {
    expect(isLost(st({ photo: 'p1' }), ['p1'])).toBe(true)
    expect(isLost(st({ photo: 'p1' }), ['p2'])).toBe(false)
    expect(isLost(st(), ['p1'])).toBe(false)
  })
})

describe('поиск и сессии', () => {
  const sp = (stickers?: StickerView[]) => ({ id: 's1', stickers }) as unknown as SpreadView
  it('нет поля — пусто; по ID', () => {
    expect(stickersOf(undefined)).toEqual([])
    expect(stickersOf(sp())).toEqual([])
    expect(stickerById(sp([st(), st({ id: 'k2' })]), 'k2')?.id).toBe('k2')
    expect(stickerById(sp([st()]), 'нет')).toBeUndefined()
  })
  it('ключ — разворот и стикер', () => {
    expect(stKey('s1', 'k1')).toBe('s1|k1')
  })
})
