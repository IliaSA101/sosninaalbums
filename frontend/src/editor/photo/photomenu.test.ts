import { describe, expect, it } from 'vitest'
import { menuContext, photoMenu, type MenuCtx } from './photomenu'
import type { SelBox } from '../../base/geom/gridedit'
import { can, type FeatureId, type Level } from '../../base/prefs/levels'
import type { SpreadView } from '../../base/core/types'

const base: MenuCtx = {
  grid: false,
  places: 1,
  texts: 0,
  stickers: 0,
  photos: 1,
  missing: 0,
  borderLost: 0,
  differ: false,
  allContain: false,
  allCover: true,
  allRect: true,
  allRounded: false,
  allOval: false,
  allGray: false,
  framed: false,
}
const ids = (c: Partial<MenuCtx>) => photoMenu({ ...base, ...c }).map((i) => i.id)

describe('photoMenu', () => {
  it('«Авто», одно фото: просмотр, имя, Проводник, поворот, зеркало, Целиком, подпись, удалить', () => {
    expect(ids({})).toEqual([
      'preview',
      'copy',
      'explorer',
      'backdrop',
      'turnCw',
      'turnCcw',
      'straighten',
      'flipH',
      'flipV',
      'gray',
      'contain',
      'rounded',
      'oval',
      'border',
      'caption',
      'delete',
    ])
  })
  it('«Слой ▸» только на сетке и раскрывает подменю', () => {
    expect(ids({})).not.toContain('layer')
    expect(ids({ grid: true })).toContain('layer')
    expect(photoMenu({ ...base, grid: true }).find((i) => i.id === 'layer')?.sub).toBe(true)
  })
  it('«На фон ▸» только у одного найденного фото и с подменю', () => {
    expect(photoMenu(base).find((i) => i.id === 'backdrop')?.sub).toBe(true)
    expect(ids({ places: 2, photos: 2 })).not.toContain('backdrop')
    expect(ids({ missing: 1 })).not.toContain('backdrop')
    expect(ids({ grid: true, photos: 0 })).not.toContain('backdrop')
  })
  it('«Зеркало» только у фото, пустое место его не получает', () => {
    expect(ids({ grid: true, photos: 0 })).not.toContain('flipH')
    expect(ids({ grid: true, photos: 0 })).not.toContain('flipV')
    expect(ids({ places: 3, photos: 2 })).toEqual(expect.arrayContaining(['flipH', 'flipV']))
  })
  it('«Пропорции под фото» только при разных пропорциях', () => {
    expect(ids({})).not.toContain('fitPhoto')
    expect(ids({ differ: true })).toContain('fitPhoto')
  })
  it('«Сбросить кадр» только при изменённом кадре', () => {
    expect(ids({})).not.toContain('resetFrame')
    expect(ids({ framed: true })).toContain('resetFrame')
  })
  it('«Перепривязать» только у пропавшего, без просмотра и Проводника', () => {
    expect(ids({})).not.toContain('relink')
    const m = ids({ missing: 1 })
    expect(m).toContain('relink')
    expect(m).not.toContain('preview')
    expect(m).not.toContain('explorer')
    expect(m).not.toContain('copy')
  })
  it('«Перепривязать» и при пропавшей рамочке без библиотеки, даже у пустого места (079 U5)', () => {
    expect(ids({ borderLost: 1, photos: 0 })).toContain('relink')
    expect(ids({ borderLost: 0 })).not.toContain('relink')
  })
  it('menuContext: borderLost считает только картинки без lib с пропавшим файлом', () => {
    const b = (over: object) => ({ kind: '', photo: 'f1', lib: '', pic: true, known: true, ...over })
    const sp = {
      grid: true,
      photos: [],
      frames: [],
      places: [
        { id: 'p1', photo: '', border: b({}) },
        { id: 'p2', photo: '', border: b({ lib: 'my:1' }) },
        { id: 'p3', photo: '', border: b({ photo: 'f2' }) },
        { id: 'p4', photo: '' },
      ],
    } as unknown as SpreadView
    const box = (id: string): SelBox => ({ x: 0, y: 0, w: 50, h: 50, ref: id, id, photo: '', fit: '', rot: 0 })
    const sel = ['p1', 'p2', 'p3', 'p4'].map(box)
    expect(menuContext(sp, sel, ['f1']).borderLost).toBe(1)
    expect(menuContext(sp, sel, []).borderLost).toBe(0)
    expect(menuContext(undefined, sel, ['f1']).borderLost).toBe(0)
  })
  it('«Копировать имена» считает только найденные фото', () => {
    const m = photoMenu({ ...base, places: 2, photos: 2, missing: 1 })
    expect(m.find((i) => i.id === 'copy')?.label).toBe('Копировать имя файла')
    expect(photoMenu({ ...base, places: 2, photos: 2, missing: 2 }).map((i) => i.id)).not.toContain('copy')
  })
  it('contain: предлагается «С обрезкой», не «Целиком»', () => {
    const m = ids({ allContain: true, allCover: false })
    expect(m).toContain('cover')
    expect(m).not.toContain('contain')
  })
  it('вперемешку — оба пункта', () => {
    const m = ids({ allContain: false, allCover: false })
    expect(m).toEqual(expect.arrayContaining(['contain', 'cover']))
  })
  it('несколько: имена и «Удалить (N)», без просмотра, подписи и Проводника', () => {
    const m = photoMenu({ ...base, places: 3, photos: 3 })
    expect(m.map((i) => i.id)).toEqual([
      'copy',
      'turnCw',
      'turnCcw',
      'flipH',
      'flipV',
      'gray',
      'contain',
      'rounded',
      'oval',
      'border',
      'delete',
    ])
    expect(m[0].label).toBe('Копировать имена файлов')
    expect(m[10].label).toBe('Удалить (3)')
  })
  it('пустое место: без файловых пунктов и «Целиком»', () => {
    expect(ids({ grid: true, photos: 0, allCover: true })).toEqual(['rounded', 'oval', 'border', 'layer', 'delete'])
  })
  it('ничего не выделено — меню нет', () => {
    expect(ids({ places: 0, photos: 0 })).toEqual([])
  })
  it('линия разделяет группы, не стоит первой', () => {
    const m = photoMenu({ ...base, grid: true, differ: true, framed: true })
    expect(m[0].sep).toBeFalsy()
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['turnCw', 'caption'])
  })
})

describe('Удалить у группы с текстами', () => {
  const label = (c: Partial<MenuCtx>) => photoMenu({ ...base, ...c }).find((i) => i.id === 'delete')?.label
  it('считает места и тексты вместе', () => {
    expect(label({ places: 1, texts: 2 })).toBe('Удалить (3)')
    expect(label({ places: 2, texts: 0 })).toBe('Удалить (2)')
    expect(label({ places: 1, texts: 0 })).toBe('Удалить')
  })
  it('menuContext принимает число текстов', () => {
    expect(menuContext(undefined, [], [], 2).texts).toBe(2)
    expect(menuContext(undefined, [], []).texts).toBe(0)
  })
})

describe('меню стикера (048)', () => {
  const one = { places: 0, photos: 0, stickers: 1 }
  const items = (c: Partial<MenuCtx>, can?: (f: FeatureId) => boolean) => photoMenu({ ...base, ...c }, can)
  it('«Авто», один стикер: заменить, зеркало, слой, удалить', () => {
    expect(items(one).map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'layer', 'delete'])
    expect(items(one)[0]).toMatchObject({ label: 'Заменить…' })
    expect(items(one).find((i) => i.id === 'layer')?.sub).toBe(true)
  })
  it('«Авто», стикер + текст: «Слой ▸» и «Удалить (2)»', () => {
    expect(items({ ...one, texts: 1 }).map((i) => i.id)).toEqual(['layer', 'delete'])
  })
  it('«Ручная»: плюс «Слой ▸»', () => {
    expect(items({ ...one, grid: true }).map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'layer', 'delete'])
    expect(items({ ...one, grid: true }).find((i) => i.id === 'layer')?.sub).toBe(true)
  })
  it('линии: первый пункт без неё, зеркало и удаление отделены', () => {
    const m = items({ ...one, grid: true })
    expect(m[0].sep).toBeFalsy()
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['flipH', 'layer', 'delete'])
  })
  it('несколько стикеров или стикер с текстом: без «Заменить…» и зеркала, «Удалить (N)»', () => {
    expect(items({ ...one, stickers: 3, grid: true }).map((i) => i.id)).toEqual(['layer', 'delete'])
    expect(items({ ...one, stickers: 3 }).find((i) => i.id === 'delete')?.label).toBe('Удалить (3)')
    expect(items({ ...one, stickers: 1, texts: 1 }).find((i) => i.id === 'delete')?.label).toBe('Удалить (2)')
    expect(items({ ...one, stickers: 2 })[0].sep).toBeFalsy()
  })
  it('уровень: без decor меню нет, без слоёв нет «Слоя»; photoMenu стикеру не нужна', () => {
    expect(items(one, (f) => f !== 'decor')).toEqual([])
    expect(items({ ...one, grid: true }, (f) => f !== 'layers').map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'delete'])
    expect(items(one, (f) => f !== 'layers').map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'delete'])
    expect(items(one, (f) => f !== 'photoMenu').map((i) => i.id)).toContain('replace')
    expect(items(one, (f) => can(f, 'simple'))).toEqual([])
  })
  it('стикер среди мест: обычное меню места, «Удалить (N)» считает и стикер, выпрямить нельзя', () => {
    expect(items({ stickers: 1 }).find((i) => i.id === 'delete')?.label).toBe('Удалить (2)')
    expect(ids({ stickers: 1 })).not.toContain('straighten')
    expect(ids({ stickers: 1 })).not.toContain('replace')
  })
  it('menuContext принимает число стикеров', () => {
    expect(menuContext(undefined, [], [], 0, 2).stickers).toBe(2)
    expect(menuContext(undefined, [], []).stickers).toBe(0)
  })
})

describe('«Повернуть» в меню', () => {
  it('есть только у фото, не у пустого места', () => {
    expect(ids({})).toEqual(expect.arrayContaining(['turnCw', 'turnCcw']))
    expect(ids({ grid: true, photos: 0 })).not.toContain('turnCw')
  })
  it('на уровне без turn пунктов нет', () => {
    expect(photoMenu(base, (f) => f !== 'turn').map((i) => i.id)).not.toContain('turnCw')
  })
})

describe('menuContext', () => {
  const spread = {
    id: 's1',
    grid: false,
    photos: ['a', 'b'],
    frames: [
      { s: 1, x: 0, y: 0, img: { x: 0, y: 0, w: 200, h: 100 }, dpi: 300 },
      { s: 1.5, x: 0.2, y: 0, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
    ],
  } as unknown as SpreadView
  const box = (photo: string, w: number, h: number): SelBox => ({ x: 0, y: 0, w, h, ref: photo, id: photo, photo, fit: '', rot: 0 })

  it('пропорции и кадр по фото', () => {
    const a = menuContext(spread, [box('a', 200, 100)], [])
    expect(a.differ).toBe(false)
    expect(a.framed).toBe(false)
    const b = menuContext(spread, [box('b', 200, 100)], [])
    expect(b.differ).toBe(true)
    expect(b.framed).toBe(true)
  })
  it('несколько: кадр не сбрасывается, пропавшие считаются', () => {
    const c = menuContext(spread, [box('a', 200, 100), box('b', 100, 100)], ['b'])
    expect(c.framed).toBe(false)
    expect(c.photos).toBe(2)
    expect(c.missing).toBe(1)
  })
  it('нечётный turn: пропорции фото меняются местами', () => {
    const s = {
      ...spread,
      photos: ['a'],
      frames: [{ s: 1, x: 0, y: 0, r: 90, turn: 1, tilt: 0, img: { x: 0, y: 0, w: 200, h: 100 }, dpi: 300 }],
    } as unknown as SpreadView
    expect(menuContext(s, [box('a', 100, 200)], []).differ).toBe(false)
    expect(menuContext(s, [box('a', 200, 100)], []).differ).toBe(true)
  })
  it('пустое место не фото', () => {
    const e: SelBox = { x: 0, y: 0, w: 50, h: 50, ref: 'p1', id: 'p1', photo: '', fit: '', rot: 0 }
    const c = menuContext(spread, [e], [])
    expect(c.places).toBe(1)
    expect(c.photos).toBe(0)
  })
})

describe('photoMenu по уровням (спека 010 U1)', () => {
  const at = (l: Level) => (f: FeatureId) => can(f, l)
  const lv = (l: Level, c: Partial<MenuCtx> = {}) => photoMenu({ ...base, ...c }, at(l)).map((i) => i.id)

  it('«Простой»: меню правой кнопки нет', () => {
    expect(lv('simple')).toEqual([])
  })
  it('«Стандартный»: без слоя, с «На фон», зеркалом и кадром', () => {
    expect(lv('standard', { grid: true })).toEqual([
      'preview',
      'copy',
      'explorer',
      'backdrop',
      'turnCw',
      'turnCcw',
      'straighten',
      'flipH',
      'flipV',
      'gray',
      'contain',
      'rounded',
      'oval',
      'border',
      'caption',
      'delete',
    ])
  })
  it('«Полный», «Ручная»: слой на месте', () => {
    expect(lv('full', { grid: true })).toContain('layer')
  })
  it('без can меню прежнее', () => {
    expect(photoMenu(base).map((i) => i.id)).toEqual(ids({}))
  })
  it('линия первой не стоит и переходит к следующему пункту, когда скрыт первый пункт группы', () => {
    const items = photoMenu({ ...base, photos: 0, grid: true }, (f) => f !== 'layers')
    expect(items.map((i) => i.id)).toEqual(['rounded', 'oval', 'border', 'delete'])
    expect(items[0].sep).toBeFalsy()
    const all = photoMenu(base, at('standard'))
    // Зеркала и кадр скрыты (в «Простом» нет меню совсем) — проверяем «Стандартный» без зеркала.
    const noFlip = photoMenu(base, (f) => f !== 'flip' && f !== 'turn' && f !== 'tilt' && f !== 'adjust')
    expect(noFlip.find((i) => i.id === 'contain')?.sep).toBe(true)
    expect(all[0].sep).toBeFalsy()
  })
})

describe('«Выпрямить по линии» в меню (спека 045)', () => {
  it('есть у одного фото сразу после «Повернуть ↺ 90°»', () => {
    const m = photoMenu(base)
    const at = m.findIndex((i) => i.id === 'turnCcw')
    expect(m[at + 1]).toMatchObject({ id: 'straighten', label: 'Выпрямить по линии' })
  })
  it('нет у нескольких фото, пустого места и группы с текстом', () => {
    expect(ids({ places: 3, photos: 3 })).not.toContain('straighten')
    expect(ids({ grid: true, photos: 0 })).not.toContain('straighten')
    expect(ids({ texts: 1 })).not.toContain('straighten')
    expect(ids({ places: 0, photos: 0 })).not.toContain('straighten')
  })
  it('только при can(tilt): «Простой» меню не имеет, без tilt пункта нет', () => {
    expect(photoMenu(base, (f) => f !== 'tilt').map((i) => i.id)).not.toContain('straighten')
    expect(photoMenu(base, (f) => can(f, 'standard')).map((i) => i.id)).toContain('straighten')
    expect(photoMenu(base, (f) => can(f, 'simple'))).toEqual([])
  })
})

describe('«Прямоугольное» / «Скруглённое» / «Круглое» в меню (спеки 044, 076)', () => {
  const none = { allRect: false, allRounded: false, allOval: false }
  const shapeIds = (c: Partial<MenuCtx>) => ids(c).filter((i) => ['rect', 'rounded', 'oval'].includes(i))
  it('как «Целиком» / «С обрезкой»: показан пункт, которого нет у всех выделенных', () => {
    expect(shapeIds({})).toEqual(['rounded', 'oval']) // base: у всех прямоугольное
    expect(shapeIds({ allRect: false, allRounded: true })).toEqual(['rect', 'oval'])
    expect(shapeIds({ allRect: false, allOval: true })).toEqual(['rect', 'rounded'])
  })
  it('смесь — все три, порядок ▭ ▢ ◯', () => {
    expect(shapeIds(none)).toEqual(['rect', 'rounded', 'oval'])
  })
  it('есть и у пустого места (не только у фото)', () => {
    expect(shapeIds({ grid: true, photos: 0 })).toEqual(['rounded', 'oval'])
    expect(shapeIds({ grid: true, photos: 0, allRect: false, allOval: true })).toEqual(['rect', 'rounded'])
  })
  it('стоят после «С обрезкой», подписи по-русски', () => {
    const m = photoMenu({ ...base, ...none, allContain: false, allCover: false })
    const at = m.findIndex((i) => i.id === 'cover')
    expect(m.slice(at + 1, at + 4).map((i) => [i.id, i.label])).toEqual([
      ['rect', 'Прямоугольное'],
      ['rounded', 'Скруглённое'],
      ['oval', 'Круглое'],
    ])
  })
  it('только при can(frame)', () => {
    const m = photoMenu(base, (f) => f !== 'frame').map((i) => i.id)
    for (const id of ['rect', 'rounded', 'oval']) expect(m).not.toContain(id)
  })
  it('menuContext: allRect / allRounded / allOval по форме выделенных', () => {
    const b = (shape: string, round = 0): SelBox => ({
      x: 0,
      y: 0,
      w: 50,
      h: 50,
      ref: 'p',
      id: 'p',
      photo: '',
      fit: '',
      rot: 0,
      shape,
      round,
    })
    const flags = (sel: SelBox[]) => {
      const c = menuContext(undefined, sel, [])
      return [c.allRect, c.allRounded, c.allOval]
    }
    expect(flags([b('oval'), b('oval')])).toEqual([false, false, true])
    expect(flags([b(''), b('oval')])).toEqual([false, false, false])
    expect(flags([b('')])).toEqual([true, false, false])
    expect(flags([b('', 25), b('', 10)])).toEqual([false, true, false])
    expect(flags([b('', 25), b('')])).toEqual([false, false, false])
    expect(flags([])).toEqual([false, false, false])
  })
})

describe('«Сделать ч/б / цветным» в меню (068)', () => {
  const gray = (c: Partial<MenuCtx> = {}, can?: (f: FeatureId) => boolean) => photoMenu({ ...base, ...c }, can).find((i) => i.id === 'gray')

  it('стоит после «Зеркало ↕», подпись по состоянию', () => {
    const m = photoMenu(base)
    expect(m[m.findIndex((i) => i.id === 'flipV') + 1]).toMatchObject({ id: 'gray', label: 'Сделать ч/б' })
    expect(gray({ allGray: true })?.label).toBe('Сделать цветным')
  })
  it('есть у группы фото, нет у пустого места', () => {
    expect(gray({ places: 3, photos: 3 })).toBeDefined()
    expect(gray({ grid: true, photos: 0 })).toBeUndefined()
  })
  it('только при can(adjust): «Простой» меню не имеет, без adjust пункта нет', () => {
    expect(gray({}, (f) => f !== 'adjust')).toBeUndefined()
    expect(gray({}, (f) => can(f, 'standard'))).toBeDefined()
  })
  it('линия группы переходит к «Сделать ч/б», когда скрыты поворот и зеркало', () => {
    const m = photoMenu(base, (f) => f !== 'flip' && f !== 'turn' && f !== 'tilt')
    expect(m.find((i) => i.id === 'gray')?.sep).toBe(true)
  })
  it('«Рамочка…» (079): у фото и у пустого места, после формы; на «Простом» меню нет, без функции — пункта нет', () => {
    const label = photoMenu(base).find((i) => i.id === 'border')?.label
    expect(label).toBe('Рамочка…')
    expect(ids({})).toContain('border')
    expect(ids({ photos: 0, grid: true })).toEqual(['rounded', 'oval', 'border', 'layer', 'delete'])
    expect(ids({ places: 3, photos: 3 })).toContain('border') // группа мест: окно ставит «Куда: выделенные»
    expect(photoMenu(base, (f) => f !== 'border').map((i) => i.id)).not.toContain('border')
    expect(photoMenu(base, (f) => f !== 'frame').map((i) => i.id)).toContain('border') // форма и рамочка — разные функции
    expect(photoMenu(base, (f) => can(f, 'simple'))).toEqual([])
    // порядок: форма · рамочка · «Сбросить кадр»
    const m = ids({ framed: true })
    expect(m.indexOf('border')).toBe(m.indexOf('oval') + 1)
    expect(m.indexOf('resetFrame')).toBe(m.indexOf('border') + 1)
  })
  it('«Рамочка…» (079): у одних стикеров меню стикера без неё', () => {
    expect(ids({ places: 0, photos: 0, stickers: 1 })).not.toContain('border')
  })
  it('menuContext: allGray — у всех выделенных фото ч/б', () => {
    const s = {
      id: 's1',
      grid: false,
      photos: ['a', 'b'],
      frames: [
        { gray: true, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
        { gray: false, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
      ],
    } as unknown as SpreadView
    const box = (photo: string): SelBox => ({ x: 0, y: 0, w: 100, h: 100, ref: photo, id: photo, photo, fit: '', rot: 0 })
    expect(menuContext(s, [box('a')], []).allGray).toBe(true)
    expect(menuContext(s, [box('a'), box('b')], []).allGray).toBe(false)
    expect(menuContext(s, [], []).allGray).toBe(false)
  })
})
