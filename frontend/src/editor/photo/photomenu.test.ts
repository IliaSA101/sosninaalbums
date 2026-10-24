import { afterEach, describe, expect, it } from 'vitest'
import { setOsForTest } from '../../base/core/platform'
import { fileGroup, menuContext, photoMenu as photoMenuFull, type MenuCtx, type MenuItem } from './photomenu'
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
  copyable: true,
  pasteable: false,
  allFlipH: false,
  allFlipV: false,
  tiltOk: true,
  scopes: [
    { scope: 'spread', label: 'Весь разворот' },
    { scope: 'left', label: 'Левая страница' },
    { scope: 'right', label: 'Правая страница' },
  ],
  uses: false,
}
// Группа буфера (099) — в photomenu.clip.test.ts; здесь меню без неё, чтобы порядок остальных пунктов не заслонялся.
const photoMenu = (c: MenuCtx, can: (f: FeatureId) => boolean = () => true) => photoMenuFull(c, (f) => f !== 'clipboard' && can(f))
const menu = (c: Partial<MenuCtx> = {}, can?: (f: FeatureId) => boolean) => photoMenu({ ...base, ...c }, can)
// Все пункты вместе с вложенными (id подменю — turn, mirror, frame, shape, file).
const flat = (m: MenuItem[]): MenuItem[] => m.flatMap((i) => [i, ...(i.sub ? flat(i.sub) : [])])
const ids = (c: Partial<MenuCtx>, can?: (f: FeatureId) => boolean) => flat(menu(c, can)).map((i) => i.id)
const top = (c: Partial<MenuCtx>, can?: (f: FeatureId) => boolean) => menu(c, can).map((i) => i.id)
const subIds = (c: Partial<MenuCtx>, group: string, can?: (f: FeatureId) => boolean) =>
  menu(c, can)
    .find((i) => i.id === group)
    ?.sub?.map((i) => i.id)
const find = (c: Partial<MenuCtx>, id: string, can?: (f: FeatureId) => boolean) => flat(menu(c, can)).find((i) => i.id === id)

describe('photoMenu: дерево М1 (спека 108)', () => {
  it('М1: одно фото, «Ручная» — 13 строк в порядке групп (владелец 2026-10-23)', () => {
    const m = photoMenuFull({ ...base, grid: true, pasteable: true })
    expect(m.map((i) => i.id)).toEqual([
      'preview',
      'clipCopy',
      'clipPaste',
      'turn',
      'mirror',
      'frame',
      'shape',
      'gray',
      'backdrop',
      'layer',
      'caption',
      'file',
      'delete',
    ])
    expect(m.filter((i) => i.sub).map((i) => i.id)).toEqual(['turn', 'mirror', 'frame', 'shape', 'backdrop', 'layer', 'file'])
    expect(m.find((i) => i.id === 'clipCopy')?.sep).toBe(true)
  })
  it('линии: перед группой Повернуть…, перед «На фон», перед «Файл», перед «Удалить»; первой нет', () => {
    const m = menu({ grid: true })
    expect(m[0].sep).toBeFalsy()
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['turn', 'backdrop', 'file', 'delete'])
  })
  it('«Авто»: слоя нет; «Слой ▸» на «Ручной» — четыре операции с параметром', () => {
    expect(top({})).not.toContain('layer')
    const layer = menu({ grid: true }).find((i) => i.id === 'layer')
    expect(layer?.sub?.map((i) => [i.id, i.arg])).toEqual([
      ['layer', 'front'],
      ['layer', 'up'],
      ['layer', 'down'],
      ['layer', 'back'],
    ])
  })
  it('«На фон ▸» — только у одного найденного фото, области из контекста', () => {
    expect(
      menu()
        .find((i) => i.id === 'backdrop')
        ?.sub?.map((i) => [i.arg, i.label]),
    ).toEqual([
      ['spread', 'Весь разворот'],
      ['left', 'Левая страница'],
      ['right', 'Правая страница'],
    ])
    expect(top({ places: 2, photos: 2 })).not.toContain('backdrop')
    expect(top({ missing: 1 })).not.toContain('backdrop')
    expect(top({ grid: true, photos: 0 })).not.toContain('backdrop')
    expect(top({ scopes: [] })).not.toContain('backdrop')
  })
  it('«Подпись» — одному фото, в том числе пропавшему', () => {
    expect(top({})).toContain('caption')
    expect(top({ missing: 1 })).toContain('caption')
    expect(top({ places: 2, photos: 2 })).not.toContain('caption')
  })
  it('ничего не выделено — меню нет', () => {
    expect(top({ places: 0, photos: 0 })).toEqual([])
  })
})

describe('Повернуть ▸', () => {
  it('↻ ↺ выпрямить наклон — у одного фото; «Выпрямить по линии» с текстом в группе нет', () => {
    expect(subIds({}, 'turn')).toEqual(['turnCw', 'turnCcw', 'straighten', 'tilt'])
    expect(subIds({ texts: 1, tiltOk: false }, 'turn')).toEqual(['turnCw', 'turnCcw'])
  })
  it('у группы — ↻ ↺', () => {
    expect(subIds({ places: 3, photos: 3, tiltOk: false }, 'turn')).toEqual(['turnCw', 'turnCcw'])
  })
  it('у пропавшего — без наклона в рамке', () => {
    expect(subIds({ missing: 1, tiltOk: false }, 'turn')).toEqual(['turnCw', 'turnCcw', 'straighten'])
  })
  it('нет у пустого места', () => {
    expect(top({ grid: true, photos: 0 })).not.toContain('turn')
  })
  it('на уровне без turn остаются выпрямить и наклон; без tilt — только повороты; без обоих группы нет', () => {
    expect(subIds({}, 'turn', (f) => f !== 'turn')).toEqual(['straighten', 'tilt'])
    expect(subIds({}, 'turn', (f) => f !== 'tilt')).toEqual(['turnCw', 'turnCcw'])
    expect(top({}, (f) => f !== 'tilt' && f !== 'turn')).not.toContain('turn')
  })
  it('«Наклон в рамке» стоит последним и зовёт tiltmode.toggle (id tilt)', () => {
    expect(find({}, 'tilt')).toMatchObject({ id: 'tilt', label: 'Наклон в рамке' })
  })
})

describe('Зеркало ▸', () => {
  it('галочка только если у всех', () => {
    const f = (c: Partial<MenuCtx>) =>
      menu(c)
        .find((i) => i.id === 'mirror')
        ?.sub?.map((i) => [i.id, i.mark, i.checked])
    expect(f({})).toEqual([
      ['flipH', 'check', false],
      ['flipV', 'check', false],
    ])
    expect(f({ allFlipH: true })).toEqual([
      ['flipH', 'check', true],
      ['flipV', 'check', false],
    ])
  })
  it('только у фото, пустое место его не получает', () => {
    expect(top({ grid: true, photos: 0 })).not.toContain('mirror')
    expect(top({ places: 3, photos: 2 })).toContain('mirror')
  })
  it('без функции flip группы нет', () => {
    expect(top({}, (f) => f !== 'flip')).not.toContain('mirror')
  })
})

describe('Кадр ▸', () => {
  const frame = (c: Partial<MenuCtx>) => subIds(c, 'frame')
  it('Целиком · С обрезкой; точка у текущего', () => {
    expect(frame({})).toEqual(['contain', 'cover'])
    expect(find({}, 'cover')).toMatchObject({ mark: 'radio', checked: true })
    expect(find({}, 'contain')).toMatchObject({ mark: 'radio', checked: false })
    expect(find({ allContain: true, allCover: false }, 'contain')).toMatchObject({ checked: true })
  })
  it('смешанное — без точки', () => {
    const c = { allContain: false, allCover: false }
    expect(find(c, 'contain')?.checked).toBe(false)
    expect(find(c, 'cover')?.checked).toBe(false)
  })
  it('«Подогнать место под фото» — при разных пропорциях, после черты', () => {
    expect(frame({})).not.toContain('fitPhoto')
    expect(frame({ differ: true })).toEqual(['contain', 'cover', 'fitPhoto'])
    expect(find({ differ: true }, 'fitPhoto')).toMatchObject({ label: 'Подогнать место под фото', sep: true })
  })
  it('«Сбросить кадр» — когда кадр изменён', () => {
    expect(frame({})).not.toContain('resetFrame')
    expect(frame({ framed: true })).toEqual(['contain', 'cover', 'resetFrame'])
    expect(frame({ framed: true, differ: true })).toEqual(['contain', 'cover', 'fitPhoto', 'resetFrame'])
  })
  it('без функции frame группы нет', () => {
    expect(top({ framed: true }, (f) => f !== 'frame')).not.toContain('frame')
  })
})

describe('Форма ▸', () => {
  const shape = (c: Partial<MenuCtx>) =>
    menu(c)
      .find((i) => i.id === 'shape')
      ?.sub?.map((i) => [i.id, i.checked])
  it('точка у текущей, «Рамочка…» после черты', () => {
    expect(shape({})).toEqual([
      ['rect', true],
      ['rounded', false],
      ['oval', false],
      ['border', undefined],
    ])
    expect(find({}, 'border')).toMatchObject({ label: 'Рамочка…', sep: true })
    expect(shape({ allRect: false, allOval: true })?.slice(0, 3)).toEqual([
      ['rect', false],
      ['rounded', false],
      ['oval', true],
    ])
  })
  it('смесь — все три без точки', () => {
    expect(
      shape({ allRect: false })
        ?.slice(0, 3)
        .map((x) => x[1]),
    ).toEqual([false, false, false])
  })
  it('у пустого места тоже', () => {
    expect(subIds({ grid: true, photos: 0 }, 'shape')).toEqual(['rect', 'rounded', 'oval', 'border'])
  })
  it('у группы мест — рамочка ставится «Куда: выделенные»', () => {
    expect(subIds({ places: 3, photos: 3 }, 'shape')).toContain('border')
  })
  it('функции независимы: без frame остаётся «Рамочка…», без border — три формы; «Простой» меню не имеет', () => {
    expect(subIds({}, 'shape', (f) => f !== 'frame')).toEqual(['border'])
    expect(find({}, 'border', (f) => f !== 'frame')?.sep).toBeFalsy() // черта не первая
    expect(subIds({}, 'shape', (f) => f !== 'border')).toEqual(['rect', 'rounded', 'oval'])
    expect(photoMenu(base, (f) => can(f, 'simple'))).toEqual([])
  })
  it('у одних стикеров меню стикера без рамочки', () => {
    expect(ids({ places: 0, photos: 0, stickers: 1 })).not.toContain('border')
  })
})

describe('Файл ▸', () => {
  afterEach(() => setOsForTest(null))
  const file = (c: Partial<MenuCtx>) => subIds(c, 'file')
  it('Показать в…, Копировать имя файла; «Где ещё стоит…» выключено', () => {
    expect(file({})).toEqual(['explorer', 'copy'])
  })
  it('«Где ещё стоит…» — одному фото, когда подключено; без него «Файл ▸» не пустеет', () => {
    expect(file({ uses: true })).toEqual(['explorer', 'copy', 'uses'])
    expect(file({ uses: true, places: 2, photos: 2 })).toEqual(['copy'])
  })
  it('у группы — «Копировать имена файлов», считает только найденные', () => {
    expect(find({ places: 3, photos: 3 }, 'copy')?.label).toBe('Копировать имена файлов')
    expect(find({ places: 2, photos: 2, missing: 1 }, 'copy')?.label).toBe('Копировать имя файла')
    expect(top({ places: 2, photos: 2, missing: 2 })).not.toContain('file')
  })
  it('пропавшее фото: «Файл ▸» из одного пункта «Где ещё стоит…», «Перепривязать…» строкой после него', () => {
    expect(file({ missing: 1, uses: true })).toEqual(['uses'])
    const m = top({ missing: 1, uses: true })
    expect(m.slice(m.indexOf('file'), m.indexOf('file') + 2)).toEqual(['file', 'relink'])
  })
  it('пропавшее фото без «Где ещё стоит…»: «Файл ▸» нет, «Перепривязать…» остаётся', () => {
    const m = top({ missing: 1 })
    expect(m).not.toContain('file')
    expect(m).toContain('relink')
    expect(ids({ missing: 1 })).not.toContain('preview')
    expect(ids({ missing: 1 })).not.toContain('explorer')
  })
  it('«Перепривязать…» и при пропавшей рамочке без библиотеки, даже у пустого места (079 U5)', () => {
    expect(top({ borderLost: 1, photos: 0 })).toContain('relink')
    expect(top({ borderLost: 0 })).not.toContain('relink')
  })
  it('слово «Показать…» по платформе (101): Windows, Mac, Linux', () => {
    const label = () => find({}, 'explorer')?.label
    setOsForTest('windows')
    expect(label()).toBe('Показать в Проводнике')
    setOsForTest('mac')
    expect(label()).toBe('Показать в Finder')
    setOsForTest('linux')
    expect(label()).toBe('Показать в папке')
  })
  it('fileGroup — те же пункты, что «Файл ▸» меню (одно описание с 106)', () => {
    const c = { photos: 1, missing: 0, one: true, uses: true }
    expect(fileGroup(c)).toEqual(menu({ uses: true }).find((i) => i.id === 'file')?.sub)
    expect(fileGroup({ ...c, uses: false }).map((i) => i.id)).toEqual(['explorer', 'copy'])
    expect(fileGroup({ photos: 0, missing: 0, one: false, uses: true })).toEqual([])
  })
})

const file0 = (c: Partial<MenuCtx>) => subIds(c, 'file')

describe('Файл ▸: вхождения (108 U3)', () => {
  const rows = [{ id: 'use', arg: '0', label: 'Разворот 2 — место' }]
  const sub = (c: Partial<MenuCtx>) =>
    menu({ uses: true, usesSub: rows, ...c })
      .find((i) => i.id === 'file')
      ?.sub?.map((i) => i.id)
  it('menuContext включает «Где ещё стоит» для одного фото', () => {
    expect(menuContext(undefined, [], []).uses).toBe(true)
  })
  it('строки идут в «Файл ▸» под серой подписью: глубже второго уровня MenuList не рисует', () => {
    expect(sub({})).toEqual(['explorer', 'copy', 'usesHead', 'use'])
    const head = menu({ uses: true, usesSub: rows })
      .find((i) => i.id === 'file')
      ?.sub?.find((i) => i.id === 'usesHead')
    expect(head).toMatchObject({ label: 'Где ещё стоит', kind: 'note', sep: true })
  })
  it('пропавшее фото: «Файл ▸» — подпись и строки; группа — без вхождений', () => {
    expect(sub({ missing: 1 })).toEqual(['usesHead', 'use'])
    expect(sub({ places: 2, photos: 2 })).toEqual(['copy'])
  })
  it('нет строк у ctx — прежний пункт без подменю', () => {
    expect(file0({ uses: true })).toEqual(['explorer', 'copy', 'uses'])
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
  it('свой кадр (098): на «Авто» нетронутый масштаб тоже даёт «Сбросить кадр»; на сетке — нет', () => {
    const own = {
      ...spread,
      frames: [{ s: 1, x: 0, y: 0, own: 1.78, img: { x: 0, y: 0, w: 200, h: 100 }, dpi: 300 }],
    } as unknown as SpreadView
    expect(menuContext(own, [box('a', 200, 100)], []).framed).toBe(true)
    expect(menuContext({ ...own, grid: true }, [box('a', 200, 100)], []).framed).toBe(false)
    const none = { ...own, frames: [{ ...own.frames![0], own: 0 }] } as unknown as SpreadView
    expect(menuContext(none, [box('a', 200, 100)], []).framed).toBe(false)
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
  it('зеркало: allFlipH / allFlipV — у всех выделенных фото; без фото — нет', () => {
    const s = {
      ...spread,
      frames: [
        { flipH: true, flipV: false, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
        { flipH: true, flipV: true, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
      ],
    } as unknown as SpreadView
    const flags = (sel: SelBox[]) => {
      const c = menuContext(s, sel, [])
      return [c.allFlipH, c.allFlipV]
    }
    expect(flags([box('a', 100, 100)])).toEqual([true, false])
    expect(flags([box('a', 100, 100), box('b', 100, 100)])).toEqual([true, false])
    expect(flags([box('b', 100, 100)])).toEqual([true, true])
    expect(flags([])).toEqual([false, false])
  })
  it('tiltOk: одно фото на месте, не пропавшее, без текстов и стикеров', () => {
    const one = [box('a', 200, 100)]
    expect(menuContext(spread, one, []).tiltOk).toBe(true)
    expect(menuContext(spread, one, ['a']).tiltOk).toBe(false)
    expect(menuContext(spread, one, [], 1).tiltOk).toBe(false)
    expect(menuContext(spread, one, [], 0, 1).tiltOk).toBe(false)
    expect(menuContext(spread, [...one, box('b', 100, 100)], []).tiltOk).toBe(false)
    expect(menuContext(spread, [], []).tiltOk).toBe(false)
  })
  it('borderLost считает только картинки без lib с пропавшим файлом', () => {
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
    const bx = (id: string): SelBox => ({ x: 0, y: 0, w: 50, h: 50, ref: id, id, photo: '', fit: '', rot: 0 })
    const sel = ['p1', 'p2', 'p3', 'p4'].map(bx)
    expect(menuContext(sp, sel, ['f1']).borderLost).toBe(1)
    expect(menuContext(sp, sel, []).borderLost).toBe(0)
    expect(menuContext(undefined, sel, ['f1']).borderLost).toBe(0)
  })
  it('allRect / allRounded / allOval по форме выделенных', () => {
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
  it('allGray — у всех выделенных фото ч/б', () => {
    const s = {
      id: 's1',
      grid: false,
      photos: ['a', 'b'],
      frames: [
        { gray: true, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
        { gray: false, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 300 },
      ],
    } as unknown as SpreadView
    expect(menuContext(s, [box('a', 100, 100)], []).allGray).toBe(true)
    expect(menuContext(s, [box('a', 100, 100), box('b', 100, 100)], []).allGray).toBe(false)
    expect(menuContext(s, [], []).allGray).toBe(false)
  })
  it('принимает число текстов и стикеров', () => {
    expect(menuContext(undefined, [], [], 2).texts).toBe(2)
    expect(menuContext(undefined, [], []).texts).toBe(0)
    expect(menuContext(undefined, [], [], 0, 2).stickers).toBe(2)
    expect(menuContext(undefined, [], []).stickers).toBe(0)
  })
})

describe('Удалить', () => {
  const label = (c: Partial<MenuCtx>) => find(c, 'delete')
  it('считает места и тексты вместе; Del справа, красный', () => {
    expect(label({ places: 1, texts: 2 })?.label).toBe('Удалить (3)')
    expect(label({ places: 2, texts: 0, photos: 2 })?.label).toBe('Удалить (2)')
    expect(label({ places: 1, texts: 0 })).toMatchObject({ label: 'Удалить', hint: 'Del', danger: true, sep: true })
  })
  it('стоит последней', () => {
    expect(top({ grid: true }).at(-1)).toBe('delete')
  })
})

describe('«Сделать ч/б / цветным» (068)', () => {
  it('стоит после «Формы», слово по состоянию', () => {
    const m = top({})
    expect(m[m.indexOf('shape') + 1]).toBe('gray')
    expect(find({}, 'gray')?.label).toBe('Сделать ч/б')
    expect(find({ allGray: true }, 'gray')?.label).toBe('Сделать цветным')
  })
  it('есть у группы фото, нет у пустого места', () => {
    expect(top({ places: 3, photos: 3 })).toContain('gray')
    expect(top({ grid: true, photos: 0 })).not.toContain('gray')
  })
  it('только при can(adjust)', () => {
    expect(top({}, (f) => f !== 'adjust')).not.toContain('gray')
    expect(top({}, (f) => can(f, 'standard'))).toContain('gray')
  })
})

describe('меню стикера (048)', () => {
  const one = { places: 0, photos: 0, stickers: 1 }
  const items = (c: Partial<MenuCtx>, can?: (f: FeatureId) => boolean) => photoMenu({ ...base, ...c }, can)
  it('«Авто», один стикер: заменить, зеркало, слой, удалить', () => {
    expect(items(one).map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'layer', 'delete'])
    expect(items(one)[0]).toMatchObject({ label: 'Заменить…' })
    expect(items(one).find((i) => i.id === 'layer')?.sub).toHaveLength(4)
  })
  it('«Авто», стикер + текст: «Слой ▸» и «Удалить (2)»', () => {
    expect(items({ ...one, texts: 1 }).map((i) => i.id)).toEqual(['layer', 'delete'])
  })
  it('«Ручная»: тот же состав', () => {
    expect(items({ ...one, grid: true }).map((i) => i.id)).toEqual(['replace', 'flipH', 'flipV', 'layer', 'delete'])
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
    expect(find({ stickers: 1 }, 'delete')?.label).toBe('Удалить (2)')
    expect(ids({ stickers: 1 })).not.toContain('straighten')
    expect(ids({ stickers: 1 })).not.toContain('replace')
  })
})

describe('photoMenu по уровням (спека 010 U1)', () => {
  const at = (l: Level) => (f: FeatureId) => can(f, l)
  it('«Простой»: меню правой кнопки нет', () => {
    expect(top({}, at('simple'))).toEqual([])
  })
  it('«Стандартный»: без слоя, с «На фон», зеркалом и кадром', () => {
    expect(top({ grid: true }, at('standard'))).toEqual([
      'preview',
      'turn',
      'mirror',
      'frame',
      'shape',
      'gray',
      'backdrop',
      'caption',
      'file',
      'delete',
    ])
  })
  it('«Полный», «Ручная»: слой на месте', () => {
    expect(top({ grid: true }, at('full'))).toContain('layer')
  })
  it('подменю без видимых пунктов не показывается, черта не первая', () => {
    // Скрыты поворот, зеркало, кадр, форма, ч/б: «На фон» берёт на себя линию и не стоит первой.
    const hidden = (f: FeatureId) => !['turn', 'tilt', 'flip', 'frame', 'border', 'adjust'].includes(f)
    const m = menu({ grid: true }, hidden)
    expect(m.map((i) => i.id)).toEqual(['preview', 'backdrop', 'layer', 'caption', 'file', 'delete'])
    expect(m.filter((i) => i.sep).map((i) => i.id)).toEqual(['backdrop', 'file', 'delete'])
    const bare = menu({ photos: 0, grid: true }, (f) => f !== 'frame' && f !== 'border' && f !== 'layers')
    expect(bare.map((i) => i.id)).toEqual(['delete'])
    expect(bare[0].sep).toBeFalsy()
  })
  it('подменю с одним пунктом остаётся подменю', () => {
    expect(subIds({ photos: 0, grid: true }, 'shape', (f) => f !== 'frame')).toEqual(['border'])
    expect(top({ photos: 0, grid: true }, (f) => f !== 'frame')).toContain('shape')
  })
  it('без can меню прежнее', () => {
    expect(photoMenu(base).map((i) => i.id)).toEqual(top({}))
  })
})

describe('пустое место и группа', () => {
  it('пустое место (с группой буфера): Форма ▸ · Слой ▸ · Удалить', () => {
    expect(top({ grid: true, photos: 0 })).toEqual(['shape', 'layer', 'delete'])
    expect(top({ photos: 0 })).toEqual(['shape', 'delete'])
  })
  it('группа фото: без просмотра, подписи и «На фон»; «Удалить (N)» внизу', () => {
    const m = top({ places: 3, photos: 3, tiltOk: false })
    expect(m).toEqual(['turn', 'mirror', 'frame', 'shape', 'gray', 'file', 'delete'])
    expect(find({ places: 3, photos: 3 }, 'delete')?.label).toBe('Удалить (3)')
  })
})

describe('клавиши из реестра', () => {
  it('подпись справа — из hint(cmd) каталога; нет клавиши — нет подписи', () => {
    const keys: Record<string, string> = { flipH: 'Ctrl+H' }
    const m = photoMenuFull(
      base,
      (f) => f !== 'clipboard',
      (cmd) => keys[cmd] ?? '',
    )
    const mirror = m.find((i) => i.id === 'mirror')?.sub
    expect(mirror?.[0].hint).toBe('Ctrl+H')
    expect(mirror?.[1].hint).toBeUndefined()
  })
})
