import { describe, expect, it } from 'vitest'
import type { BorderView, PlaceView } from '../../base/core/types'
import { MIXED_NAME, borderRows, fadeRow, inkPatch, targetsOf } from './borderrows'

const bv = (over: Partial<BorderView> = {}): BorderView => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: 'sa:fr-white-line',
  name: 'Белая линия',
  inks: ['#ffffff'],
  width: 1.5,
  defWidth: 1.5,
  scale: 0,
  ins: [1.5, 1.5, 1.5, 1.5],
  url: '/media/gen/frame?k=line',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})
const pic = (over: Partial<BorderView> = {}) =>
  bv({
    kind: '',
    photo: 'f1',
    lib: 'my:0123456789abcdef',
    preset: '',
    name: 'Резная',
    inks: [],
    width: 0,
    defWidth: undefined,
    scale: 1,
    pic: true,
    ...over,
  })
const place = (id: string, photo: string, border?: BorderView): PlaceView =>
  ({ id, photo, fit: '', spine: false, z: 0, x: 0, y: 0, w: 50, h: 50, rot: 0, border }) as PlaceView

describe('targetsOf: рамочки выделенных', () => {
  const sp = { places: [place('p1', 'a', bv()), place('p2', '')] }
  it('по ID места и по ID фото, пустое место — без рамочки', () => {
    expect(targetsOf(sp, ['p1', 'p2', 'a']).map((t) => !!t.border)).toEqual([true, false, true])
  })
  it('«Авто» (мест нет) и нет листа — рамочек нет', () => {
    expect(targetsOf({ places: null }, ['a'])).toEqual([{ ref: 'a', border: undefined }])
    expect(targetsOf(undefined, ['a'])).toEqual([{ ref: 'a', border: undefined }])
  })
  it('пустая строка фото у места не совпадает с пустым ref', () => {
    expect(targetsOf(sp, ['']).map((t) => !!t.border)).toEqual([false])
  })
})

describe('borderRows: строки раздела', () => {
  it('ни у кого нет рамочки — null («Выбрать…»)', () => {
    expect(borderRows([])).toBeNull()
    expect(borderRows([{ ref: 'p1' }, { ref: 'p2' }])).toBeNull()
  })

  it('одна встроенная: имя, плитка, цвета, толщина, толщина по умолчанию', () => {
    const r = borderRows([{ ref: 'p1', border: bv() }])!
    expect(r).toMatchObject({
      name: 'Белая линия',
      tile: 'sa:fr-white-line',
      own: false,
      inks: ['#ffffff'],
      inkIds: [['p1']],
      widthIds: ['p1'],
      width: 1.5,
      widthMixed: false,
      defWidth: 1.5,
      sizeIds: [],
      all: ['p1'],
    })
  })

  it('правили руками (preset снят): имя вида, плитки нет', () => {
    const r = borderRows([{ ref: 'p1', border: bv({ preset: '', name: 'Линия' }) }])!
    expect(r.name).toBe('Линия')
    expect(r.tile).toBe('')
  })

  it('вид без цветов (мягкий край) — пикеров нет, толщина есть', () => {
    const r = borderRows([{ ref: 'p1', border: bv({ kind: 'soft', inks: [], width: 8 }) }])!
    expect(r.inks).toEqual([])
    expect(r.widthIds).toEqual(['p1'])
  })

  it('два цвета (паспарту) — два пикера', () => {
    const r = borderRows([{ ref: 'p1', border: bv({ kind: 'mat', inks: ['#fbf8f1', '#c8a24a'] }) }])!
    expect(r.inks).toEqual(['#fbf8f1', '#c8a24a'])
    expect(r.inkIds).toEqual([['p1'], ['p1']])
  })

  it('своя картинка: «Размер, %» вместо толщины, подложка-шашечка, плитка — ключ библиотеки', () => {
    const r = borderRows([{ ref: 'p1', border: pic({ scale: 1.25 }) }])!
    expect(r).toMatchObject({
      own: true,
      tile: 'my:0123456789abcdef',
      widthIds: [],
      sizeIds: ['p1'],
      size: 125,
      sizeMixed: false,
      inks: [],
    })
    expect(r.defWidth).toBeNull()
  })

  it('группа с одним значением — общее, без «—»', () => {
    const r = borderRows([
      { ref: 'p1', border: bv() },
      { ref: 'p2', border: bv() },
    ])!
    expect(r.widthMixed).toBe(false)
    expect(r.inks).toEqual(['#ffffff'])
    expect(r.name).toBe('Белая линия')
  })

  it('группа с разной толщиной и цветом — «—» (mixed, пустой цвет); значение — первого', () => {
    const r = borderRows([
      { ref: 'p1', border: bv({ width: 1.5, inks: ['#ffffff'] }) },
      { ref: 'p2', border: bv({ width: 4, inks: ['#000000'] }) },
    ])!
    expect(r.widthMixed).toBe(true)
    expect(r.width).toBe(1.5)
    expect(r.inks).toEqual([''])
    expect(r.inksFirst).toEqual(['#ffffff'])
  })

  it('группа с разными рамочками — «Разные рамочки», плитки нет, толщина по умолчанию неизвестна', () => {
    const r = borderRows([
      { ref: 'p1', border: bv() },
      { ref: 'p2', border: bv({ preset: 'sa:fr-gold', name: 'Золотая рамка', defWidth: 4 }) },
    ])!
    expect(r.name).toBe(MIXED_NAME)
    expect(r.tile).toBe('')
    expect(r.defWidth).toBeNull()
  })

  it('часть группы без рамочки: строки по тем, у кого она есть; «Убрать» — только им', () => {
    const r = borderRows([{ ref: 'p1', border: bv() }, { ref: 'p2' }, { ref: 'p3', border: bv() }])!
    expect(r.all).toEqual(['p1', 'p3'])
    expect(r.widthIds).toEqual(['p1', 'p3'])
  })

  it('встроенная и картинка вместе: обе строки, каждая правит своих', () => {
    const r = borderRows([
      { ref: 'p1', border: bv() },
      { ref: 'p2', border: pic() },
    ])!
    expect(r.widthIds).toEqual(['p1'])
    expect(r.sizeIds).toEqual(['p2'])
    expect(r.own).toBe(false)
  })

  it('вид новее программы (known = false): править нечего, отмечено для U5', () => {
    const r = borderRows([{ ref: 'p1', border: bv({ known: false, kind: 'future' }) }])!
    expect(r.newer).toBe(true)
    expect(r.widthIds).toEqual([])
    expect(r.inkIds).toEqual([])
  })

  it('у разных видов цветов разное число: пикер 2 правит только тех, у кого он есть', () => {
    const r = borderRows([
      { ref: 'p1', border: bv() },
      { ref: 'p2', border: bv({ kind: 'mat', inks: ['#fbf8f1', '#c8a24a'] }) },
    ])!
    expect(r.inkIds).toEqual([['p1', 'p2'], ['p2']])
  })
})

describe('inkPatch: массив цветов для SetBorder', () => {
  it('заменяет один, остальные держат общее значение', () => {
    const rows = { inks: ['#fbf8f1', '#c8a24a'], inksFirst: ['#fbf8f1', '#c8a24a'] }
    expect(inkPatch(rows, 1, '#112233')).toEqual(['#fbf8f1', '#112233'])
    expect(inkPatch(rows, 0, '#112233')).toEqual(['#112233', '#c8a24a'])
  })
  it('у разных значений («—») остальные пикеры берут цвет первого места', () => {
    const rows = { inks: ['', '#c8a24a'], inksFirst: ['#fbf8f1', '#c8a24a'] }
    expect(inkPatch(rows, 1, '#112233')).toEqual(['#fbf8f1', '#112233'])
  })
})

describe('borderRows: пропавшие, неизвестные и без имени (079 U5)', () => {
  it('своя без файла: считается в lostLib; без записи в библиотеке — в lostFile', () => {
    const r = borderRows(
      [
        { ref: 'p1', border: pic({ photo: 'f1' }) },
        { ref: 'p2', border: pic({ lib: '', photo: 'f2' }) },
        { ref: 'p3', border: pic({ photo: 'f3' }) },
      ],
      { missing: ['f1', 'f2'] },
    )!
    expect(r.lostLib).toBe(1)
    expect(r.lostFile).toBe(1)
    expect(r.newer).toBe(false)
  })
  it('без списка пропавших ничего не потеряно; размер правится и у пропавшей', () => {
    expect(borderRows([{ ref: 'p1', border: pic() }])).toMatchObject({ lostLib: 0, lostFile: 0 })
    expect(borderRows([{ ref: 'p1', border: pic() }], { missing: ['f1'] })!.sizeIds).toEqual(['p1'])
  })
  it('вид новее: newer, имя — «Нужна новая версия программы», править нечего, «Убрать» работает', () => {
    const r = borderRows([{ ref: 'p1', border: bv({ known: false, name: '' }) }])!
    expect(r.newer).toBe(true)
    expect(r.name).toBe('Нужна новая версия программы')
    expect(r.widthIds).toEqual([])
    expect(r.inkIds).toEqual([])
    expect(r.sizeIds).toEqual([])
    expect(r.all).toEqual(['p1'])
  })
  it('вид новее вместе с обычной — «Разные рамочки»', () => {
    const r = borderRows([
      { ref: 'p1', border: bv({ known: false }) },
      { ref: 'p2', border: bv() },
    ])!
    expect(r.name).toBe(MIXED_NAME)
    expect(r.newer).toBe(true)
  })
  it('своя без имени в View: из каталога, иначе «Своя рамочка»', () => {
    const t = [{ ref: 'p1', border: pic({ name: '' }) }]
    expect(borderRows(t, { nameOf: (k) => (k === 'my:0123456789abcdef' ? 'Лоза' : undefined) })!.name).toBe('Лоза')
    expect(borderRows(t, { nameOf: () => undefined })!.name).toBe('Своя рамочка')
    expect(borderRows(t)!.name).toBe('Своя рамочка')
  })
  it('имя из View главнее каталога', () => {
    expect(borderRows([{ ref: 'p1', border: pic() }], { nameOf: () => 'Другое' })!.name).toBe('Резная')
  })
})

describe('fadeRow: прозрачность фото', () => {
  const fr = (fade?: number) => ({ fade }) as never
  const sp = { photos: ['a', 'b', 'c'], frames: [fr(0.5), fr(0.5), fr(undefined)] }
  it('одно фото — проценты', () => expect(fadeRow(sp, ['a'])).toEqual({ pct: 50, mixed: false }))
  it('у всех то же — без «—»', () => expect(fadeRow(sp, ['a', 'b'])).toEqual({ pct: 50, mixed: false }))
  it('разное — «—», значение первого', () => expect(fadeRow(sp, ['a', 'c'])).toEqual({ pct: 50, mixed: true }))
  it('нет поля fade — 0 %; нет листа и фото — 0', () => {
    expect(fadeRow(sp, ['c'])).toEqual({ pct: 0, mixed: false })
    expect(fadeRow(undefined, [])).toEqual({ pct: 0, mixed: false })
    expect(fadeRow(sp, ['нет'])).toEqual({ pct: 0, mixed: false })
  })
})
