import { describe, expect, it } from 'vitest'
import { setMacForTest } from '../../base/core/platform'
import { miniItems, swapTip, type MiniCtx } from './minipanel'
import { can, type FeatureId, type Level } from '../../base/prefs/levels'

// Состав столбца мини-панели (спека 108): miniItems по контексту, уровням, наклону.
const base: MiniCtx = { grid: false, places: 1, photos: 1, differ: false, allContain: false, flipH: false, flipV: false }
const ids = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c }).map((i) => i.id)

describe('miniItems', () => {
  const at = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c })
  const moreOf = (c: Partial<MiniCtx>, can?: (f: FeatureId) => boolean) =>
    miniItems({ ...base, ...c }, can)
      .find((i) => i.id === 'more')
      ?.more?.map((n) => n.label)
  it('ничего не выделено — панели нет', () => {
    expect(ids({ places: 0, photos: 0 })).toEqual([])
  })
  it('Г: ↔ ↕ | целиком | ↻ наклон | ⇄ | ⋯ | корзина последней (владелец 2026-10-23)', () => {
    expect(ids({ grid: true })).toEqual(['flipH', 'flipV', 'fit', 'turnCw', 'tilt', 'swap', 'more', 'delete'])
    expect(at({ grid: true }).at(-1)?.id).toBe('delete')
  })
  it('«Авто»: без ⇄, в ⋯ только Подпись и заметка', () => {
    expect(ids({})).toEqual(['flipH', 'flipV', 'fit', 'turnCw', 'tilt', 'more', 'delete'])
    expect(moreOf({})).toEqual(['Подпись', 'Всё остальное — правым щелчком по фото'])
  })
  it('«Ручная»: в ⋯ Подпись, Слой ▸ из четырёх пунктов и заметка', () => {
    expect(moreOf({ grid: true })).toEqual(['Подпись', 'Слой', 'Всё остальное — правым щелчком по фото'])
    const layer = at({ grid: true })
      .find((i) => i.id === 'more')
      ?.more?.find((n) => n.id === 'layer')
    expect(layer?.sub?.map((n) => [n.id, n.arg])).toEqual([
      ['layer', 'front'],
      ['layer', 'up'],
      ['layer', 'down'],
      ['layer', 'back'],
    ])
  })
  it('подсказка «⋯» называет то, что в списке: Авто — подпись, Ручная — подпись и слой, группа — слой', () => {
    const tip = (c: Partial<MiniCtx>) => at(c).find((i) => i.id === 'more')?.label
    expect(tip({})).toBe('Ещё: подпись')
    expect(tip({ grid: true })).toBe('Ещё: подпись, слой')
    expect(tip({ grid: true, places: 3, photos: 3 })).toBe('Ещё: слой')
    expect(tip({ name: true })).toBe('Ещё: имя') // шаблон с именем: вместо подписи
  })
  it('⋯: «Имя под фото» вместо «Подписи» при шаблоне с именем (037)', () => {
    expect(moreOf({ name: true })?.[0]).toBe('Имя под фото')
    expect(at({ name: true }).find((i) => i.id === 'more')?.more?.[0].id).toBe('name')
  })
  it('заметка «правым щелчком» — только если меню фото есть на уровне', () => {
    expect(moreOf({}, (f) => f !== 'photoMenu')).toEqual(['Подпись'])
    expect(moreOf({}, (f) => can(f, 'simple'))).toEqual(['Подпись']) // в «Простом» меню правой кнопки нет
  })
  it('«Под фото» только при разных пропорциях, сразу после переключателя', () => {
    expect(ids({})).not.toContain('fitPhoto')
    expect(ids({ differ: true })).toEqual(['flipH', 'flipV', 'fit', 'fitPhoto', 'turnCw', 'tilt', 'more', 'delete'])
  })
  it('пустое место: ⋯ (Слой) | корзина; «Авто» — только корзина', () => {
    expect(ids({ grid: true, photos: 0 })).toEqual(['more', 'delete'])
    expect(moreOf({ grid: true, photos: 0 })).toEqual(['Слой', 'Всё остальное — правым щелчком по фото'])
    expect(ids({ grid: false, photos: 0 })).toEqual(['delete'])
  })
  it('группа: ↔ ↕ | ↻ | одинаковый размер | ⋯ (Слой) | корзина с числом; без подписи и ⇄', () => {
    expect(ids({ grid: true, places: 3, photos: 3 })).toEqual(['flipH', 'flipV', 'turnCw', 'same', 'more', 'delete'])
    expect(moreOf({ grid: true, places: 3, photos: 3 })?.[0]).toBe('Слой')
    expect(ids({ grid: false, places: 2, photos: 2 })).toEqual(['flipH', 'flipV', 'turnCw', 'same', 'delete'])
  })
  it('группа из пустых мест: зеркал нет', () => {
    expect(ids({ grid: true, places: 2, photos: 0 })).toEqual(['same', 'more', 'delete'])
  })
  it('группа с текстами: число в «Удалить (N)» считает и тексты; одно фото с текстом — уже группа', () => {
    const del = (c: Partial<MiniCtx>) => miniItems({ ...base, ...c }).find((i) => i.id === 'delete')!
    expect(del({ places: 2, photos: 2, texts: 1 }).badge).toBe('3')
    expect(ids({ grid: true, places: 1, photos: 1, texts: 1 })).toEqual(['flipH', 'flipV', 'turnCw', 'more', 'delete'])
  })
  it('только тексты (2+): ⋯ (Слой, «Ручная») и корзина, без зеркал и «Одинакового размера»', () => {
    expect(ids({ grid: true, places: 0, photos: 0, texts: 2 })).toEqual(['more', 'delete'])
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
  it('линия между группами значков, не перед первым; ↻ и наклон в одной группе', () => {
    const m = miniItems({ ...base, grid: true })
    expect(m[0].sep).toBeUndefined()
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['fit', 'turnCw', 'swap', 'more', 'delete'])
  })
  it('подсказки у всех значков', () => {
    for (const it of miniItems({ ...base, grid: true, differ: true })) expect(it.label).not.toBe('')
  })
})

describe('имя под фото (037)', () => {
  it('подпись по шаблону с полями имени — «Имя под фото» в «⋯» вместо «Подписи»', () => {
    const m = miniItems({ ...base, name: true })
    expect(m.map((i) => i.id)).toEqual(['flipH', 'flipV', 'fit', 'turnCw', 'tilt', 'more', 'delete'])
    expect(m.find((i) => i.id === 'more')?.more?.find((n) => n.id === 'name')?.label).toBe('Имя под фото')
  })
})

describe('miniItems по уровням (спека 010 U1)', () => {
  const at = (l: Level) => (f: FeatureId) => can(f, l)
  const lv = (l: Level, c: Partial<MiniCtx> = {}) => miniItems({ ...base, ...c }, at(l)).map((i) => i.id)

  it('«Простой»: ↻ 90° | ⋯ (Подпись) | корзина', () => {
    expect(lv('simple')).toEqual(['turnCw', 'more', 'delete'])
  })
  it('«Простой», «Ручная»: слой, ⇄ и зеркала скрыты', () => {
    expect(lv('simple', { grid: true })).toEqual(['turnCw', 'more', 'delete'])
  })
  it('«Стандартный»: зеркала, «Целиком / С обрезкой», ⋯ (подпись); слоя и ⇄ нет', () => {
    expect(lv('standard', { grid: true })).toEqual(['flipH', 'flipV', 'fit', 'turnCw', 'tilt', 'more', 'delete'])
  })
  it('нет ни подписи, ни слоя — «⋯» пропадает вместе с чертой', () => {
    const noMore = (f: FeatureId) => f !== 'captions' && f !== 'layers'
    expect(miniItems(base, noMore).map((i) => i.id)).toEqual(['flipH', 'flipV', 'fit', 'turnCw', 'tilt', 'delete'])
  })
  it('«Полный» — как без уровня', () => {
    expect(lv('full', { grid: true })).toEqual(ids({ grid: true }))
  })
  it('черта не стоит первой, даже если первая группа скрыта', () => {
    const items = miniItems({ ...base }, at('simple'))
    expect(items[0].sep).toBeUndefined()
    expect(items[1].sep).toBe(true) // ⋯ — своя группа
    expect(items[2].sep).toBe(true)
  })
  it('группа в «Простом»: ↻ 90° и «Удалить (N)»', () => {
    expect(lv('simple', { places: 3, photos: 3 })).toEqual(['turnCw', 'delete'])
  })
})

describe('tilt', () => {
  const tilt = (c: Partial<MiniCtx>, can?: (f: FeatureId) => boolean) => miniItems({ ...base, ...c }, can).find((i) => i.id === 'tilt')
  it('кнопка стоит сразу после ↻ 90°, в одной с ним группе', () => {
    const m = miniItems(base)
    const i = m.findIndex((x) => x.id === 'turnCw')
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

describe('подсказка ⇄ с Ctrl / ⌘ (спека 100 п. 6)', () => {
  const swapLabel = () => miniItems({ ...base, grid: true }).find((i) => i.id === 'swap')?.label
  it('текст: Ctrl вне Mac, ⌘ на Mac', () => {
    expect(swapTip(false)).toBe('Перетащите на другое место — фото поменяются местами (или тащите фото с Ctrl)')
    expect(swapTip(true)).toBe('Перетащите на другое место — фото поменяются местами (или тащите фото с ⌘)')
  })
  it('подпись значка берёт платформу из platform.ts', () => {
    setMacForTest(false)
    expect(swapLabel()).toBe(swapTip(false))
    setMacForTest(true)
    expect(swapLabel()).toBe(swapTip(true))
    setMacForTest(null)
  })
  it('на «Авто» значка ⇄ нет', () => {
    expect(ids({ grid: false })).not.toContain('swap')
  })
})
