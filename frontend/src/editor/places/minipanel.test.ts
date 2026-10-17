import { describe, expect, it } from 'vitest'
import { LAYER_OPS, miniContext, miniItems, miniPlace, passThrough, ROT_KEEP, visibleArea, type MiniCtx } from './minipanel'
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
const sp = (frames: FrameView[], photos: string[], grid = true) => ({ grid, photos, frames, captions: [] }) as unknown as SpreadView

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

describe('имя под фото (037)', () => {
  it('подпись по шаблону с полями имени — «Имя под фото» вместо «Подпись к фото»', () => {
    const m = miniItems({ ...base, name: true })
    expect(m.map((i) => i.id)).toEqual(['flipH', 'flipV', 'fit', 'name', 'turn', 'delete'])
    expect(m.find((i) => i.id === 'name')?.label).toBe('Имя под фото')
  })
  it('miniContext: name — у единственного фото с who в подписи или заглушке', () => {
    const cap = { photo: 'a', who: ['last'] }
    const stub = { photo: 'b', place: 'p', who: ['last', 'first'] }
    const s = { ...sp([fv({}), fv({}), fv({})], ['a', 'b', 'c']), captions: [cap], stubs: [stub] } as unknown as SpreadView
    expect(miniContext(s, [sel('a')]).name).toBe(true)
    expect(miniContext(s, [sel('b')]).name).toBe(true)
    expect(miniContext(s, [sel('c')]).name).toBe(false)
    expect(miniContext(s, [sel('a'), sel('b')]).name).toBe(false)
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

describe('miniPlace и подпись под фото', () => {
  const box = { x: 10, y: 10, w: 40, h: 30 } // низ рамки 40 мм
  it('полоса не закрывает подпись под рамкой', () => {
    // Подпись под рамкой до 52 мм: верх полосы не выше низа подписи (раньше — низ рамки + 8).
    const p = miniPlace(box, 2, 600, 300, 100, undefined, 52)
    expect(p.top).toBeGreaterThanOrEqual(52 * 2)
    expect(p.top).toBe(52 * 2 + 8)
  })
  it('подписи нет или она внутри рамки — как раньше', () => {
    expect(miniPlace(box, 2, 600, 300, 100, undefined, 0).top).toBe(20 + 60 + 8)
    expect(miniPlace(box, 2, 600, 300, 100, undefined, 35).top).toBe(20 + 60 + 8)
  })
  it('под подписью места нет — над рамкой', () => {
    expect(miniPlace({ ...box, y: 40 }, 2, 600, 160, 100, undefined, 100).top).toBe(80 - 8 - 30)
  })
})

describe('miniPlace обходит ⟳', () => {
  // Фото почти во весь разворот: снизу места нет, ⟳ стоит над серединой верхней стороны.
  const big = { x: 12, y: 30, w: 400, h: 264 }
  const by = 30 * 4
  const rot = { x: (12 + 200) * 4, y: by - 24 } // середина верхней стороны, 24 px выше
  it('над рамкой полоса встаёт выше ⟳, по центру, не сдвигаясь вбок', () => {
    const p = miniPlace(big, 4, 1700, 1120, 100, rot)
    expect(p.left).toBe(rot.x)
    expect(p.top).toBe(rot.y - ROT_KEEP - 30)
    expect(p.top + 30).toBeLessThanOrEqual(rot.y - ROT_KEEP) // ⟳ не закрыт
  })
  it('без ⟳ — над рамкой впритык, по центру', () => {
    expect(miniPlace(big, 4, 1700, 1120, 100)).toEqual({ top: by - 8 - 30, left: rot.x })
  })
  it('⟳ не рядом — полоса над рамкой впритык', () => {
    expect(miniPlace(big, 4, 1700, 1120, 100, { x: 10, y: 10 })).toEqual({ top: by - 8 - 30, left: rot.x })
  })
  it('⟳ внутри рамки (у фото во всю высоту), сдвигаться некуда — полоса внутри у верха', () => {
    const p = miniPlace({ x: 0, y: 17.8, w: 70, h: 264 }, 2, 140, 600, 120, { x: 70, y: 17.8 * 2 + 24 })
    expect(p.top).toBe(17.8 * 2 + 8)
  })
  it('⟳ внизу (повёрнутое на 180°) — под рамкой не встаёт на него: уходит вверх', () => {
    const box = { x: 10, y: 100, w: 40, h: 30 } // низ 260 px при ppm 2
    const p = miniPlace(box, 2, 600, 600, 100, { x: 60, y: 260 + 24 })
    expect(p.top).toBeLessThan(200)
  })
})

describe('miniPlace и видимая область холста (0.13.1)', () => {
  // Лист 300×200 мм при ppm 2 (600×400 px) стоит в окне 800×500: серое поле 100 px слева/справа, 50 сверху/снизу.
  const view = { left: -100, top: -50, right: 700, bottom: 450 }
  const rot = (b: { x: number; y: number; w: number }) => ({ x: (b.x + b.w / 2) * 2, y: b.y * 2 - 24 })
  const at = (b: { x: number; y: number; w: number; h: number }) => miniPlace(b, 2, 600, 400, 100, rot(b), 0, view)

  it('малое верхнее — под фото, по центру', () => {
    const b = { x: 20, y: 10, w: 60, h: 40 }
    expect(at(b)).toEqual({ top: 10 * 2 + 40 * 2 + 8, left: 100 })
  })
  it('малое нижнее у края листа, но место в сером поле — под фото, по центру', () => {
    const b = { x: 200, y: 140, w: 60, h: 50 } // низ 380 px, лист кончается на 400
    expect(at(b)).toEqual({ top: 380 + 8, left: 460 })
  })
  it('большое во всю высоту листа — под фото, в сером поле', () => {
    const b = { x: 20, y: 0, w: 120, h: 200 }
    expect(at(b)).toEqual({ top: 400 + 8, left: 160 })
  })
  it('фото у реального низа видимой области — над фото, выше ⟳, по центру', () => {
    const b = { x: 20, y: 150, w: 100, h: 50 } // низ 400 px; окно кончается на 450 — под не влезает (408+30 > 450 - нет)
    const p = miniPlace(b, 2, 600, 400, 100, rot(b), 0, { ...view, bottom: 420 })
    expect(p.left).toBe(140)
    expect(p.top).toBe(rot(b).y - ROT_KEEP - 30)
  })
  it('зум: фото больше экрана — внутри у верха видимой части', () => {
    const b = { x: -50, y: -40, w: 500, h: 300 } // 1000×600 px, окно 800×500
    const p = miniPlace(b, 2, 600, 400, 100, undefined, 0, view)
    expect(p.top).toBe(-50 + 8)
    expect(p.left).toBe(400) // центр фото, внутри окна
  })
  it('зум: верх фото за окном, низ виден — под фото', () => {
    const b = { x: 20, y: -100, w: 60, h: 150 } // низ 100 px
    expect(at(b).top).toBe(100 + 8)
  })
  it('по горизонтали зажим краями видимой области, а не листа', () => {
    expect(miniPlace({ x: -80, y: 0, w: 4, h: 4 }, 2, 600, 400, 100, undefined, 0, view).left).toBe(-50)
    expect(miniPlace({ x: 296, y: 0, w: 4, h: 4 }, 2, 600, 400, 100, undefined, 0, view).left).toBe(596) // за лист — можно
    expect(miniPlace({ x: 340, y: 0, w: 10, h: 4 }, 2, 600, 400, 100, undefined, 0, view).left).toBe(650)
  })
  it('соседние фото одного ряда — полоса на одной высоте и по центру каждого', () => {
    const a = at({ x: 20, y: 10, w: 60, h: 40 })
    const b = at({ x: 100, y: 10, w: 60, h: 40 })
    expect(a.top).toBe(b.top)
    expect(b.left - a.left).toBe(160)
  })
})

describe('visibleArea', () => {
  it('переводит окно холста в оси разворота', () => {
    const v = visibleArea({ left: 200, top: 100, width: 600, height: 400 }, { left: 100, top: 50, width: 800, height: 500 }, 600)
    expect(v).toEqual({ left: -100, top: -50, right: 700, bottom: 450 })
  })
  it('учитывает масштаб: прямоугольник вдвое меньше ширины раскладки', () => {
    const v = visibleArea({ left: 0, top: 0, width: 300, height: 200 }, { left: 0, top: 0, width: 400, height: 300 }, 600)
    expect(v).toEqual({ left: 0, top: 0, right: 800, bottom: 600 })
  })
  it('нет размеров (ещё не раскладано) — undefined, остаётся лист', () => {
    expect(visibleArea({ left: 0, top: 0, width: 0, height: 0 }, { left: 0, top: 0, width: 800, height: 500 }, 600)).toBeUndefined()
    expect(visibleArea({ left: 0, top: 0, width: 600, height: 400 }, { left: 0, top: 0, width: 0, height: 0 }, 600)).toBeUndefined()
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
