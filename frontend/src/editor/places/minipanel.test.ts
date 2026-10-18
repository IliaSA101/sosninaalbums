import { describe, expect, it } from 'vitest'
import {
  ARC_IN_H,
  ARC_IN_W,
  ARC_OUT_H,
  arcRoom,
  arcWhere,
  chromeInset,
  LAYER_OPS,
  MINI_GAP,
  MINI_H,
  MINI_IN_GAP,
  miniContext,
  miniItems,
  miniPlace,
  passThrough,
  ROT_KEEP,
  visibleArea,
  zoomLift,
  type MiniCtx,
} from './minipanel'
import { ARC_CMIN, ARC_OUT_GAP, ARC_PHI, KNOB } from '../tilt/tiltdial'
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
    expect(ids({})).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'tilt', 'delete'])
  })
  it('фото, «Ручная»: плюс «Слой» и ⇄ перед «Удалить»', () => {
    expect(ids({ grid: true })).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'tilt', 'layer', 'swap', 'delete'])
  })
  it('«Под фото» только при разных пропорциях, сразу после переключателя', () => {
    expect(ids({})).not.toContain('fitPhoto')
    expect(ids({ differ: true })).toEqual(['flipH', 'flipV', 'fit', 'fitPhoto', 'caption', 'turn', 'tilt', 'delete'])
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
    expect(m.map((i) => i.id)).toEqual(['flipH', 'flipV', 'fit', 'name', 'turn', 'tilt', 'delete'])
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
    expect(miniPlace(box, 2, 600, 300, 100)).toEqual({ top: 20 + 60 + 8, left: 60, where: 'below' })
  })
  it('снизу нет места — над рамкой', () => {
    expect(miniPlace({ ...box, y: 40 }, 2, 600, 150, 100).top).toBe(80 - 8 - 30)
  })
  it('нет ни под, ни над — внутри у нижнего края, по центру (владелец 2026-10-17)', () => {
    const p = miniPlace({ x: 0, y: 0, w: 300, h: 150 }, 2, 600, 300, 100)
    expect(p).toEqual({ top: 300 - MINI_IN_GAP - MINI_H, left: 300, where: 'inside' })
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
    expect(miniPlace(big, 4, 1700, 1120, 100)).toEqual({ top: by - 8 - 30, left: rot.x, where: 'above' })
  })
  it('⟳ не рядом — полоса над рамкой впритык', () => {
    expect(miniPlace(big, 4, 1700, 1120, 100, { x: 10, y: 10 })).toEqual({ top: by - 8 - 30, left: rot.x, where: 'above' })
  })
  it('фото во всю высоту, ⟳ внутри — полоса внутри у низа, ⟳ свободен (владелец 2026-10-17)', () => {
    const rotIn = { x: 70, y: 10 * 2 + 24 } // ⟳ внутри рамки у верхней стороны
    const p = miniPlace({ x: 0, y: 10, w: 70, h: 264 }, 2, 140, 560, 120, rotIn)
    expect(p).toEqual({ top: 548 - MINI_IN_GAP - MINI_H, left: 70, where: 'inside' })
    expect(rotIn.y).toBeLessThan(p.top - ROT_KEEP) // ⟳ далеко от полосы
  })
  it('внутри полоса задевает ⟳ (повёрнутое на 180°, ⟳ внизу) — прежний запасной у верха, вбок от ⟳', () => {
    const p = miniPlace({ x: 0, y: 10, w: 70, h: 264 }, 2, 140, 560, 100, { x: 70, y: 548 - 24 })
    expect(p.where).toBe('top')
    expect(p.top).toBe(20 + 8)
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
    expect(at(b)).toEqual({ top: 10 * 2 + 40 * 2 + 8, left: 100, where: 'below' })
  })
  it('малое нижнее у края листа, но место в сером поле — под фото, по центру', () => {
    const b = { x: 200, y: 140, w: 60, h: 50 } // низ 380 px, лист кончается на 400
    expect(at(b)).toEqual({ top: 380 + 8, left: 460, where: 'below' })
  })
  it('большое во всю высоту листа — под фото, в сером поле', () => {
    const b = { x: 20, y: 0, w: 120, h: 200 }
    expect(at(b)).toEqual({ top: 400 + 8, left: 160, where: 'below' })
  })
  it('фото у реального низа видимой области — над фото, выше ⟳, по центру', () => {
    const b = { x: 20, y: 150, w: 100, h: 50 } // низ 400 px; окно кончается на 450 — под не влезает (408+30 > 450 - нет)
    const p = miniPlace(b, 2, 600, 400, 100, rot(b), 0, { ...view, bottom: 420 })
    expect(p.left).toBe(140)
    expect(p.top).toBe(rot(b).y - ROT_KEEP - 30)
  })
  it('зум: фото больше экрана — внутри у низа видимой части (владелец 2026-10-17)', () => {
    const b = { x: -50, y: -40, w: 500, h: 300 } // 1000×600 px, окно 800×500
    const p = miniPlace(b, 2, 600, 400, 100, undefined, 0, view)
    expect(p.where).toBe('inside')
    expect(p.top).toBe(450 - MINI_IN_GAP - MINI_H) // низ окна, а не низ фото (520)
    expect(p.left).toBe(400) // центр фото, внутри окна
    // центр фото за окном справа — полоса зажата краем видимой части
    expect(miniPlace({ x: 100, y: -40, w: 500, h: 300 }, 2, 600, 400, 100, undefined, 0, view).left).toBe(650)
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

const cap = (x: number, y: number, w: number, h: number) => ({ x, y, w, h }) // бокс подписи, мм

describe('miniPlace внутри у низа (владелец 2026-10-17, owner-ux §1)', () => {
  const box = { x: 0, y: 0, w: 300, h: 150 } // ppm 2: 600×300 px, лист 600×300
  it('порядок: под → над → внутри у низа (владелец 2026-10-17)', () => {
    const small = { x: 100, y: 40, w: 60, h: 40 } // 120×80 px
    expect(miniPlace(small, 2, 600, 400, 100).where).toBe('below')
    expect(miniPlace(small, 2, 600, 400, 100, undefined, 0, { left: 0, top: 0, right: 600, bottom: 150 }).where).toBe('above')
    expect(miniPlace(box, 2, 600, 300, 100).where).toBe('inside')
  })
  it('внутри: зазор 14 px — средний маркер низа свободен (владелец 2026-10-17)', () => {
    const p = miniPlace(box, 2, 600, 300, 100)
    expect(MINI_IN_GAP).toBe(14) // полоса переноса места 10 + полручки 4
    expect(p.top + MINI_H + MINI_IN_GAP).toBe(300)
  })
  it('внутри: подпись у низа — полоса над подписью (владелец 2026-10-17)', () => {
    // Подпись внутри рамки: верх 120 мм = 240 px; полоса кончается на 8 px выше.
    const p = miniPlace(box, 2, 600, 300, 100, undefined, 0, undefined, [cap(100, 120, 100, 12)])
    expect(p.where).toBe('inside')
    expect(p.top + MINI_H).toBe(240 - MINI_GAP)
    // подпись под рамкой (ниже низа) полосу внутри не двигает
    expect(miniPlace(box, 2, 600, 300, 100, undefined, 0, undefined, [cap(100, 200, 100, 12)]).top).toBe(300 - MINI_IN_GAP - MINI_H)
  })
  it('подпись внутри у верха полосу у низа не трогает (ревью 0.14.1, решение 21)', () => {
    const p = miniPlace(box, 2, 600, 300, 100, undefined, 0, undefined, [cap(100, 5, 100, 12)])
    expect(p).toEqual(miniPlace(box, 2, 600, 300, 100))
    expect(p.where).toBe('inside')
  })
  it('подпись снаружи над рамкой + заглушка внутри у низа: полоса над заглушкой, а не на ней', () => {
    const outside = cap(100, -20, 100, 12) // над рамкой
    const stub = cap(100, 125, 100, 12) // y 250..274 px: пересекает полосу у низа
    const p = miniPlace(box, 2, 600, 300, 100, undefined, 0, undefined, [outside, stub])
    expect(p.where).toBe('inside')
    expect(p.top + MINI_H).toBe(250 - MINI_GAP)
  })
  it('подпись сбоку от полосы по горизонтали её не сдвигает', () => {
    const side = cap(0, 120, 40, 12) // x 0..80 px, полоса 250..350
    expect(miniPlace(box, 2, 600, 300, 100, undefined, 0, undefined, [side]).top).toBe(300 - MINI_IN_GAP - MINI_H)
  })
  it('zoomLift: ползунок масштаба над полосой (владелец 2026-10-17)', () => {
    const p = miniPlace(box, 2, 600, 300, 100)
    expect(zoomLift(p, 300)).toBe(300 - p.top + 8)
    expect(zoomLift({ top: 10, where: 'below' }, 300)).toBe(0)
    expect(zoomLift({ top: 10, where: 'above' }, 300)).toBe(0)
    expect(zoomLift({ top: 10, where: 'top' }, 300)).toBe(0)
    // у повёрнутого места оси чужие: вне 'inside' — 0, на 'inside' — постоянный подъём над полосой (ревью 0.14.1)
    expect(zoomLift({ top: 10, where: 'below' }, 300, 15)).toBe(0)
    expect(zoomLift(p, 300, 15)).toBe(MINI_IN_GAP + MINI_H + 8)
  })
  it('зум: повёрнутое место с полосой у низа — ползунок выше полосы', () => {
    const v = { left: 0, top: 0, right: 1000, bottom: 400 }
    const r = (3 * Math.PI) / 180
    const [W, H] = [200, 220] // мм, ppm 2 → 400×440 px; низ рамки в 10 px над низом окна
    const aw = W * Math.cos(r) + H * Math.sin(r)
    const ah = W * Math.sin(r) + H * Math.cos(r)
    const b = { x: 250 - aw / 2, y: 195 - ah, w: aw, h: ah }
    const p = miniPlace(b, 2, 1000, 1000, 290, undefined, 0, v)
    expect(p.where).toBe('inside')
    const lift = zoomLift(p, (b.y + b.h) * 2, 3)
    expect(lift).toBeGreaterThanOrEqual((b.y + b.h) * 2 - p.top) // выше верха полосы
  })
  it('внутри не помещается — прежний запасной (владелец 2026-10-17)', () => {
    const cut = { left: 0, top: 0, right: 600, bottom: 150 }
    // видимая часть ниже 160 px
    const low = miniPlace({ x: 0, y: 0, w: 300, h: 70 }, 2, 600, 300, 100, undefined, 0, cut)
    expect(low).toEqual({ top: 8, left: 300, where: 'top' })
    // видимая часть уже полосы + 16
    const narrow = miniPlace({ x: 0, y: 0, w: 55, h: 200 }, 2, 600, 300, 100, undefined, 0, { left: 0, top: 0, right: 600, bottom: 200 })
    expect(narrow.where).toBe('top')
    // ровно полоса + 16 — ещё помещается
    expect(
      miniPlace({ x: 0, y: 0, w: 58, h: 200 }, 2, 600, 300, 100, undefined, 0, { left: 0, top: 0, right: 600, bottom: 200 }).where,
    ).toBe('inside')
  })
  it('подпись у самого верха фото: места над ней нет — запасной', () => {
    // подпись пересекает полосу и стоит у самого верха — над ней места нет
    const tall = { x: 0, y: 0, w: 300, h: 100 }
    const p = miniPlace(tall, 2, 600, 200, 100, undefined, 0, { left: 0, top: 0, right: 600, bottom: 200 }, [cap(100, 8, 100, 80)])
    expect(p.where).toBe('top')
  })
})

describe('visibleArea без линеек и панелей (владелец 2026-10-17)', () => {
  const win = { left: 100, top: 50, width: 800, height: 500 }
  const sheet = { left: 200, top: 100, width: 600, height: 400 }
  it('visibleArea: без линеек, ленты «Добавить» и карточки (владелец 2026-10-17)', () => {
    const inset = chromeInset(win, [
      { edge: 'top', box: { left: 100, top: 50, width: 800, height: 18 } }, // линейка сверху
      { edge: 'left', box: { left: 100, top: 50, width: 18, height: 500 } }, // линейка слева
      { edge: 'left', box: { left: 108, top: 90, width: 62, height: 300 } }, // лента: правый край 170
      { edge: 'right', box: { left: 692, top: 90, width: 200, height: 300 } }, // карточка: левый край 692
    ])
    expect(inset).toEqual({ left: 70, top: 18, right: 208, bottom: 0 })
    expect(visibleArea(sheet, win, 600, inset)).toEqual({ left: -30, top: -32, right: 492, bottom: 450 })
  })
  it('срезы не больше окна: съели всё — по целому окну', () => {
    expect(visibleArea(sheet, win, 600, { left: 500, top: 0, right: 400, bottom: 0 })).toEqual(visibleArea(sheet, win, 600))
  })
})

describe('chromeInset', () => {
  const win = { left: 100, top: 50, width: 800, height: 500 }
  it('ничего нет — нули', () => {
    expect(chromeInset(win, [])).toEqual({ left: 0, top: 0, right: 0, bottom: 0 })
  })
  it('части вне окна и пустые не считаются', () => {
    const out = chromeInset(win, [
      { edge: 'left', box: { left: -50, top: 60, width: 100, height: 100 } }, // левее окна
      { edge: 'right', box: { left: 950, top: 60, width: 100, height: 100 } }, // правее
      { edge: 'top', box: { left: 100, top: 60, width: 0, height: 18 } }, // скрыта
    ])
    expect(out).toEqual({ left: 0, top: 0, right: 0, bottom: 0 })
  })
  it('из нескольких берётся самая широкая', () => {
    const out = chromeInset(win, [
      { edge: 'left', box: { left: 100, top: 50, width: 18, height: 500 } },
      { edge: 'left', box: { left: 108, top: 90, width: 62, height: 100 } },
    ])
    expect(out.left).toBe(70)
  })
})

describe('arcRoom', () => {
  it('arc room constants match the arc', () => {
    // Пороги места считаются здесь, а дуга — в tiltdial.ts: правка одной стороны без другой ловится тут.
    const phi = (ARC_PHI * Math.PI) / 180
    const sagitta = (ARC_CMIN / (2 * Math.sin(phi))) * (1 - Math.cos(phi))
    expect(ARC_IN_W).toBe(ARC_CMIN + 2 * 12)
    expect(ARC_OUT_H).toBeGreaterThanOrEqual(ARC_OUT_GAP + sagitta + KNOB)
  })
  const view = { left: 0, top: 0, right: 800, bottom: 500 }
  it('крупное фото — дуга внутри, по центру видимой части', () => {
    // 400×300 px у верха видимой области
    expect(arcRoom({ x: 10, y: 0, w: 200, h: 150 }, 2, 0, view, 'below')).toEqual({ mode: 'in', vx: 200, vy: 0, vw: 400 })
  })
  it('зум: верх и левый край за окном — центр и верх видимой части', () => {
    const v = { left: 100, top: 50, right: 700, bottom: 450 }
    // рамка x −200..900, y −100..700 px; видимая часть 100..700 × 50..450
    const r = arcRoom({ x: -100, y: -50, w: 550, h: 400 }, 2, 0, v, 'inside')
    expect(r).toEqual({ mode: 'in', vx: 600, vy: 150, vw: 600 })
  })
  it('меньше ARC_IN_W × ARC_IN_H, а над рамкой ≥ ARC_OUT_H — снаружи', () => {
    const small = { x: 100, y: 40, w: ARC_IN_W / 2 - 1, h: 70 } // ppm 2: ширина 162, высота 140
    expect(arcRoom(small, 2, 0, view, 'below')).toEqual({ mode: 'out', vx: (ARC_IN_W - 2) / 2, vy: 0, vw: ARC_IN_W - 2 })
    expect(arcRoom({ ...small, y: ARC_OUT_H / 2 }, 2, 0, view, 'below').mode).toBe('out') // ровно 48 px
  })
  it('мелкое у верха страницы — места нет; мини-панель над рамкой — снаружи нельзя', () => {
    const small = { x: 100, y: 10, w: 60, h: 40 }
    expect(arcRoom(small, 2, 0, view, 'below').mode).toBe('none')
    expect(arcRoom({ ...small, y: 100 }, 2, 0, view, 'above').mode).toBe('none')
  })
  it('граница внутри: 164 × 160 px', () => {
    expect(arcRoom({ x: 0, y: 0, w: ARC_IN_W / 2, h: ARC_IN_H / 2 }, 2, 0, view, 'below').mode).toBe('in')
    expect(arcRoom({ x: 0, y: 0, w: ARC_IN_W / 2 - 0.5, h: ARC_IN_H / 2 }, 2, 0, view, 'below').mode).toBe('none')
    expect(arcRoom({ x: 0, y: 0, w: ARC_IN_W / 2, h: ARC_IN_H / 2 - 0.5 }, 2, 0, view, 'below').mode).toBe('none')
  })
  it('зум: узкое высокое фото, полоса в запасном top — дуга под полосой (решение 20)', () => {
    const v = { left: 0, top: 0, right: 1000, bottom: 400 }
    const tall = { x: 100, y: -50, w: 125, h: 400 } // 250×800 px, окно 400
    const p = miniPlace(tall, 2, 1000, 600, 290, undefined, 0, v)
    expect(p).toMatchObject({ top: 8, where: 'top' })
    const a = arcRoom(tall, 2, 0, v, p.where)
    expect(a.mode).toBe('in')
    expect(a.vy).toBe(100 + MINI_GAP + MINI_H + 4) // видимая часть начинается на 100 px ниже верха рамки
    expect(a.vy).toBeGreaterThanOrEqual(p.top - 100 + MINI_H) // ниже низа полосы
  })
  it('полоса сверху внутри и под ней не хватает ARC_IN_H — внутри нельзя, снаружи или нигде', () => {
    const v = { left: 0, top: 0, right: 800, bottom: 500 }
    const b = { x: 0, y: 0, w: 200, h: ARC_IN_H / 2 + 10 } // высота 180 px: хватает без полосы, не хватает с ней
    expect(arcRoom(b, 2, 0, v, 'below').mode).toBe('in')
    expect(arcRoom(b, 2, 0, v, 'top').mode).toBe('none')
  })
  it('мелкое фото: место дуги не зависит от ⟳ — out → none, без мигания (ревью 0.14.1)', () => {
    // 200×120 px, верх в 60 px от верха области, снизу места нет
    const view = { left: 0, top: 0, right: 800, bottom: 200 }
    const b = { x: 100, y: 30, w: 100, h: 60 }
    const rot = { x: 300, y: 60 - 24 } // ⟳ снаружи над серединой
    const off = miniPlace(b, 2, 800, 600, 280, rot, 0, view) // режим выключен
    const on = miniPlace(b, 2, 800, 600, 280, undefined, 0, view) // режим включён: ⟳ спрятан
    expect(off.where).not.toBe(on.where) // полоса действительно переезжает
    // MiniPanel берёт where для дуги из размещения без ⟳ — решение не меняется от включения режима
    expect(arcRoom(b, 2, 0, view, on.where).mode).toBe('none')
  })
  it('повёрнутое место, полоса сверху внутри — дуга под ней (ревью 0.14.1)', () => {
    const v = { left: 0, top: 0, right: 1000, bottom: 400 }
    const r = (3 * Math.PI) / 180
    const [W, H] = [125, 400]
    const aw = W * Math.cos(r) + H * Math.sin(r)
    const ah = W * Math.sin(r) + H * Math.cos(r)
    const place = { x: 100, y: 5, w: W, h: H }
    const b = { x: 100 + W / 2 - aw / 2, y: 5 + H / 2 - ah / 2, w: aw, h: ah }
    const p = miniPlace(b, 2, 1000, 1000, 290, undefined, 0, v)
    expect(p.where).toBe('top')
    const a = arcRoom(place, 2, 3, v, p.where)
    expect(a.mode).toBe('in')
    expect(a.vy).toBe(MINI_GAP + MINI_H + 4)
    // высоты под полосой не хватает — места нет, а без полосы было бы
    const low = { x: 0, y: 0, w: 200, h: ARC_IN_H / 2 + 10 }
    expect(arcRoom(low, 2, 3, v, 'below').mode).toBe('in')
    expect(arcRoom(low, 2, 3, v, 'top').mode).toBe('none')
  })
  it('arcWhere: полоса как без ⟳, при ⟳ и без — один ответ', () => {
    const view = { left: 0, top: 0, right: 800, bottom: 200 }
    const b = { x: 100, y: 30, w: 100, h: 60 }
    const rot = { x: 300, y: 60 - 24 }
    const off = miniPlace(b, 2, 800, 600, 280, rot, 0, view)
    const on = miniPlace(b, 2, 800, 600, 280, undefined, 0, view)
    expect(off.where).not.toBe(on.where)
    expect(arcWhere(b, 2, 800, 600, 280, 0, view, [])).toBe(on.where)
  })
  it('повёрнутое место — по его осям, видимая область не считается', () => {
    expect(arcRoom({ x: -500, y: -500, w: 200, h: 150 }, 2, 30, view, 'below')).toEqual({ mode: 'in', vx: 200, vy: 0, vw: 400 })
    expect(arcRoom({ x: 100, y: 100, w: 40, h: 150 }, 2, 90, view, 'below').mode).toBe('none')
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
    expect(lv('standard', { grid: true })).toEqual(['flipH', 'flipV', 'fit', 'caption', 'turn', 'tilt', 'delete'])
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

describe('tilt', () => {
  const tilt = (c: Partial<MiniCtx>, can?: (f: FeatureId) => boolean) => miniItems({ ...base, ...c }, can).find((i) => i.id === 'tilt')
  it('кнопка стоит сразу после ↻ 90°, в группе с подписью', () => {
    const m = miniItems(base)
    const i = m.findIndex((x) => x.id === 'turn')
    expect(m[i + 1].id).toBe('tilt')
    expect(m[i + 1].sep).toBeUndefined()
    expect(tilt({})?.icon).toBe('tilt')
  })
  it('работает в «Авто» и в «Ручной»', () => {
    expect(tilt({ grid: false })).toBeDefined()
    expect(tilt({ grid: true })).toBeDefined()
  })
  it('нужен уровень с «Наклоном»: в «Простом» нет, в «Стандартном» есть, в «Своём» — по флажку', () => {
    expect(tilt({}, (f) => can(f, 'simple'))).toBeUndefined()
    expect(tilt({}, (f) => can(f, 'standard'))).toBeDefined()
    expect(tilt({}, (f) => f !== 'tilt')).toBeUndefined()
  })
  it('нет у группы, пустого места, текста, стикера и пропавшего фото', () => {
    expect(tilt({ places: 2, photos: 2 })).toBeUndefined() // группа
    expect(tilt({ places: 1, photos: 1, texts: 1 })).toBeUndefined() // фото и текст
    expect(tilt({ places: 0, photos: 0, texts: 1 })).toBeUndefined() // текст
    expect(tilt({ grid: true, photos: 0 })).toBeUndefined() // пустое место
    expect(tilt({ missing: true })).toBeUndefined() // фото пропало
  })
  it('tilt: кнопка не гаснет (нет disabled) — как быть без места, решает tiltmode (владелец 2026-10-17)', () => {
    // Мелкое фото: дуги нет, но нажатие ведёт к строке «Наклон, °» в карточке (tiltmode) — кнопка не гаснет.
    const t = tilt({})
    expect(t).not.toBeUndefined()
    expect(t).not.toHaveProperty('disabled')
    expect(t?.label).toBe('Наклон фото в рамке: выпрямить горизонт')
  })
  it('on — когда режим включён, и aria-pressed у выключенного ложен, а не пропущен', () => {
    expect(tilt({ tilt: true })?.on).toBe(true)
    expect(tilt({ tilt: false })?.on).toBe(false)
    expect(tilt({})?.on).toBe(false)
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
