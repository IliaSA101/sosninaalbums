import { describe, expect, it } from 'vitest'
import { LAYER_OPS, miniContext, miniItems, miniPlace, passThrough, ROT_KEEP, type MiniCtx } from './minipanel'
import type { SelBox } from '../../base/geom/gridedit'
import { can, type FeatureId, type Level } from '../../base/prefs/levels'
import type { FrameView, SpreadView } from '../../base/core/types'

const base: MiniCtx = { grid: false, places: 1, photos: 1, differ: false, allContain: false, flipH: false, flipV: false }
const ids = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c }).map((i) => i.id)

describe('miniItems', () => {
  it('ничего не выделено — панели нет', () => {
    expect(ids({ places: 0, photos: 0 })).toEqual([])
  })
  it('фото, «Авто»: зеркала, переключатель вида, подпись, ↻ 90°, удалить; без слоя и ⇄', () => {
    expect(ids({})).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'delete'])
  })
  it('фото, «Ручная»: плюс «Слой» и ⇄ перед «Удалить»', () => {
    expect(ids({ grid: true })).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'layer', 'swap', 'delete'])
  })
  it('«Под фото» только при разных пропорциях, сразу после переключателя', () => {
    expect(ids({})).not.toContain('fitPhoto')
    expect(ids({ differ: true })).toEqual(['flipH', 'flipV', 'fit', 'fitPhoto', 'caption', 'turn', 'delete'])
  })
  it('пустое место: слой и удалить', () => {
    expect(ids({ grid: true, photos: 0 })).toEqual(['layer', 'delete'])
  })
  it('группа: зеркала и ↻ 90°, одинаковый размер, слой («Ручная»), удалить (N); без подписи и ⇄', () => {
    expect(ids({ grid: true, places: 3, photos: 3 })).toEqual(['flipH', 'flipV', 'turn', 'same', 'layer', 'delete'])
    expect(ids({ grid: false, places: 2, photos: 2 })).toEqual(['flipH', 'flipV', 'turn', 'same', 'delete'])
  })
  it('группа из пустых мест: зеркал нет', () => {
    expect(ids({ grid: true, places: 2, photos: 0 })).toEqual(['same', 'layer', 'delete'])
  })
  it('группа с текстами: число в «Удалить (N)» считает и тексты; одно фото с текстом — уже группа', () => {
    const del = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c }).find((i) => i.id === 'delete')!
    expect(del({ places: 2, photos: 2, texts: 1 }).badge).toBe('3')
    expect(ids({ grid: true, places: 1, photos: 1, texts: 1 })).toEqual(['flipH', 'flipV', 'turn', 'layer', 'delete'])
  })
  it('только тексты (2+): слой («Ручная») и удалить, без зеркал и «Одинакового размера»', () => {
    expect(ids({ grid: true, places: 0, photos: 0, texts: 2 })).toEqual(['layer', 'delete'])
    expect(ids({ grid: false, places: 0, photos: 0, texts: 3 })).toEqual(['delete'])
  })
  it('«Удалить (N)» у группы несёт число, у одиночного — нет', () => {
    const del = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c }).find((i) => i.id === 'delete')!
    expect(del({ places: 3, photos: 3 }).badge).toBe('3')
    expect(del({}).badge).toBeUndefined()
  })
  it('зеркало и «Целиком» подсвечены по контексту', () => {
    const m = miniItems({ ...base, flipH: true, allContain: true })
    expect(m.find((i) => i.id === 'flipH')?.on).toBe(true)
    expect(m.find((i) => i.id === 'flipV')?.on).toBe(false)
    expect(m.find((i) => i.id === 'fit')?.on).toBe(true)
  })
  it('линия между группами значков, не перед первым; ↻ стоит в группе с подписью', () => {
    const m = miniItems({ ...base, grid: true })
    expect(m[0].sep).toBeUndefined()
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['fit', 'caption', 'layer', 'delete'])
  })
  it('подсказки у всех значков', () => {
    for (const it of miniItems({ ...base, grid: true, differ: true })) expect(it.label).not.toBe('')
  })
})

const fv = (o: Partial<FrameView>): FrameView => ({
  s: 1,
  x: 0,
  y: 0,
  r: 0,
  turn: 0,
  tilt: 0,
  img: { x: 0, y: 0, w: 100, h: 50 },
  dpi: 300,
  flipH: false,
  flipV: false,
  cellRot: 0,
  z: 1,
  ...o,
})
const sel = (photo: string, o: Partial<SelBox> = {}): SelBox => ({
  ref: photo,
  id: photo,
  photo,
  fit: '',
  rot: 0,
  x: 0,
  y: 0,
  w: 100,
  h: 50,
  ...o,
})
const sp = (frames: FrameView[], photos: string[], grid = true) => ({ grid, photos, frames }) as unknown as SpreadView

describe('miniContext', () => {
  it('зеркало «у всех» — только если включено у каждого фото', () => {
    const s = sp([fv({ flipH: true }), fv({ flipH: false, flipV: true })], ['a', 'b'])
    expect(miniContext(s, [sel('a')]).flipH).toBe(true)
    const both = miniContext(s, [sel('a'), sel('b')])
    expect(both.flipH).toBe(false)
    expect(both.flipV).toBe(false)
  })
  it('пустое место зеркало не гасит и не включает', () => {
    const c = miniContext(sp([], []), [sel('', { id: 'p1' })])
    expect(c.photos).toBe(0)
    expect(c.flipH).toBe(false)
  })
  it('пропорции места и фото разные — differ', () => {
    const s = sp([fv({})], ['a'])
    expect(miniContext(s, [sel('a', { w: 100, h: 50 })]).differ).toBe(false)
    expect(miniContext(s, [sel('a', { w: 60, h: 60 })]).differ).toBe(true)
  })
  it('allContain — у всех фото «целиком»', () => {
    const s = sp([fv({}), fv({})], ['a', 'b'])
    expect(miniContext(s, [sel('a', { fit: 'contain' }), sel('b', { fit: 'contain' })]).allContain).toBe(true)
    expect(miniContext(s, [sel('a', { fit: 'contain' }), sel('b')]).allContain).toBe(false)
  })
})

describe('нечётный turn: пропорции фото меняются местами', () => {
  it('место 50×100 при фото 100×50 с turn=1 — differ нет, с turn=0 — есть', () => {
    const box = sel('a', { w: 50, h: 100 })
    expect(miniContext(sp([fv({ turn: 1, r: 90 })], ['a']), [box]).differ).toBe(false)
    expect(miniContext(sp([fv({ turn: 0 })], ['a']), [box]).differ).toBe(true)
    expect(miniContext(sp([fv({ turn: 2, r: 180 })], ['a']), [box]).differ).toBe(true)
  })
})

describe('miniPlace', () => {
  const box = { x: 10, y: 10, w: 40, h: 30 }
  it('под рамкой, по её центру', () => {
    expect(miniPlace(box, 2, 600, 300, 100)).toEqual({ top: 20 + 60 + 8, left: 60 })
  })
  it('снизу нет места — над рамкой', () => {
    expect(miniPlace({ ...box, y: 40 }, 2, 600, 150, 100).top).toBe(80 - 8 - 30)
  })
  it('нет и сверху — внутри у верхнего края', () => {
    expect(miniPlace({ x: 0, y: 0, w: 300, h: 150 }, 2, 600, 300, 100).top).toBe(8)
  })
  it('полоса не выходит за разворот по бокам', () => {
    expect(miniPlace({ x: 0, y: 0, w: 4, h: 4 }, 2, 600, 300, 100).left).toBe(50)
    expect(miniPlace({ x: 290, y: 0, w: 10, h: 4 }, 2, 600, 300, 100).left).toBe(550)
  })
})

describe('miniPlace обходит ⟳', () => {
  // Фото почти во весь разворот: снизу места нет, над рамкой ⟳ стоит ровно под полосой.
  const big = { x: 12, y: 17.8, w: 400, h: 264 }
  const by = 17.8 * 4
  const rot = { x: (12 + 200) * 4, y: by - 24 } // середина верхней стороны, 24 px выше
  it('над рамкой полоса сдвигается от ⟳ и не накрывает его', () => {
    const p = miniPlace(big, 4, 1700, 1120, 100, rot)
    expect(p.top).toBe(by - 8 - 30)
    expect(Math.abs(p.left - rot.x)).toBeGreaterThanOrEqual(50 + ROT_KEEP)
  })
  it('без ⟳ — как раньше, по центру', () => {
    expect(miniPlace(big, 4, 1700, 1120, 100).left).toBe(rot.x)
  })
  it('⟳ не рядом — полоса остаётся по центру', () => {
    expect(miniPlace(big, 4, 1700, 1120, 100, { x: 10, y: 10 }).left).toBe(rot.x)
  })
  it('⟳ внутри рамки, сдвигаться некуда — полоса внутри у верхнего края', () => {
    const p = miniPlace({ x: 0, y: 17.8, w: 70, h: 264 }, 2, 140, 600, 120, { x: 70, y: 17.8 * 2 + 24 })
    expect(p.top).toBe(17.8 * 2 + 8)
  })
})

describe('LAYER_OPS', () => {
  it('четыре действия в порядке меню', () => {
    expect(LAYER_OPS.map((o) => o.op)).toEqual(['front', 'up', 'down', 'back'])
  })
})

describe('miniItems по уровням (спека 010 U1)', () => {
  const at = (l: Level) => (f: FeatureId) => can(f, l)
  const lv = (l: Level, c: Partial<MiniCtx> = {}) => miniItems({ ...base, ...c }, at(l)).map((i) => i.id)

  it('«Простой»: подпись, ↻ 90° и «Удалить»', () => {
    expect(lv('simple')).toEqual(['caption', 'turn', 'delete'])
  })
  it('«Простой», «Ручная»: слой, ⇄ и зеркала скрыты', () => {
    expect(lv('simple', { grid: true })).toEqual(['caption', 'turn', 'delete'])
  })
  it('«Стандартный»: зеркала, «Целиком / С обрезкой», подпись; слоя и ⇄ нет', () => {
    expect(lv('standard', { grid: true })).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'delete'])
  })
  it('«Полный» — как без уровня', () => {
    expect(lv('full', { grid: true })).toEqual(ids({ grid: true }))
  })
  it('черта не стоит первой, даже если первая группа скрыта', () => {
    const items = miniItems({ ...base }, at('simple'))
    expect(items[0].sep).toBeUndefined()
    expect(items[1].sep).toBeUndefined() // ↻ в одной группе с подписью
    expect(items[2].sep).toBe(true)
  })
  it('группа в «Простом»: ↻ 90° и «Удалить (N)»', () => {
    expect(lv('simple', { places: 3, photos: 3 })).toEqual(['turn', 'delete'])
  })
})

describe('passThrough', () => {
  const none = { shiftKey: false, ctrlKey: false, altKey: false, metaKey: false }
  it('без модификаторов панель ловит мышь', () => {
    expect(passThrough(none)).toBe(false)
  })
  it('Shift, Ctrl, Alt или Meta — пропускает к фото под панелью', () => {
    for (const k of ['shiftKey', 'ctrlKey', 'altKey', 'metaKey'] as const) expect(passThrough({ ...none, [k]: true })).toBe(true)
  })
})
