import { describe, expect, it } from 'vitest'
import cases from '../../../../testdata/parity/marks.json'
import type { Label, MarkView, Photo, XmpMarks } from '../../base/core/types'
import { autoPick, emptyReason, withSkipped } from '../library/freephotos'
import { filterPhotos, filterSummary, isActive, markOf, matches, type FlagFilter, type MarkFilter } from './marks'

const ph = (id: string, xmp?: XmpMarks): Photo => ({ id, path: id, name: id, w: 100, h: 100, xmp })
const flt = (p: Partial<{ minRating: number; colors: Label[]; flag: FlagFilter }> = {}): MarkFilter => ({
  minRating: p.minRating ?? 0,
  colors: new Set(p.colors ?? []),
  flag: p.flag ?? 'all',
})

describe('markOf по testdata/parity/marks.json', () => {
  for (const c of cases) {
    it(c.name, () => {
      const m = markOf(ph('a', c.xmp as XmpMarks), c.own as MarkView | null)
      delete m.lr // lr — не часть паритета (в Go его нет)
      expect(m).toEqual({ ...c.want, labelText: c.want.labelText || undefined })
    })
  }
})

describe('markOf: lr и «не задано» ≠ 0', () => {
  const file = ph('a', { rating: 4, label: 'red', flag: 1 })
  it('без своей — как в файле, без lr', () => {
    expect(markOf(file)).toEqual({ rating: 4, label: 'red', flag: 1 })
    expect(markOf(file, {})).toEqual({ rating: 4, label: 'red', flag: 1 })
  })
  it('своя 0 и пустая метка заданы и главнее файла', () => {
    const m = markOf(file, { rating: 0, label: '' })
    expect(m).toMatchObject({ rating: 0, label: '', flag: 1 })
    expect(m.lr).toEqual({ rating: 4, label: 'red', flag: 1 })
  })
  it('своя равна файлу — lr нет', () => {
    expect(markOf(file, { rating: 4 }).lr).toBeUndefined()
  })
  it('фото без xmp — нули', () => {
    expect(markOf(ph('a'))).toEqual({ rating: 0, label: '', flag: 0 })
  })
  it('текст незнакомой метки — только пока метка из файла', () => {
    const other = ph('a', { label: 'other', labelText: 'Клиенту' })
    expect(markOf(other).labelText).toBe('Клиенту')
    expect(markOf(other, { label: 'red' }).labelText).toBeUndefined()
  })
})

describe('matches', () => {
  const m = (rating: number, label: Label | 'other', flag: -1 | 0 | 1) => ({ rating, label, flag })
  it('оценка: не ниже, без оценки', () => {
    expect(matches(m(3, '', 0), flt({ minRating: 3 }))).toBe(true)
    expect(matches(m(2, '', 0), flt({ minRating: 3 }))).toBe(false)
    expect(matches(m(0, '', 0), flt({ minRating: -1 }))).toBe(true)
    expect(matches(m(1, '', 0), flt({ minRating: -1 }))).toBe(false)
  })
  it('цвет: любой из набора; «без метки»; незнакомая — ни туда ни сюда', () => {
    expect(matches(m(0, 'red', 0), flt({ colors: ['red', 'blue'] }))).toBe(true)
    expect(matches(m(0, 'green', 0), flt({ colors: ['red', 'blue'] }))).toBe(false)
    expect(matches(m(0, '', 0), flt({ colors: [''] }))).toBe(true)
    expect(matches(m(0, 'red', 0), flt({ colors: [''] }))).toBe(false)
    expect(matches(m(0, 'other', 0), flt({ colors: [''] }))).toBe(false)
    expect(matches(m(0, 'other', 0), flt({ colors: ['red'] }))).toBe(false)
    expect(matches(m(0, 'other', 0), flt())).toBe(true)
  })
  it('флажок', () => {
    const cases: [FlagFilter, boolean, boolean, boolean][] = [
      // [фильтр, отобрано, нет флажка, отклонено]
      ['all', true, true, true],
      ['picked', true, false, false],
      ['notRejected', true, true, false],
      ['rejected', false, false, true],
    ]
    for (const [f, picked, none, rej] of cases) {
      expect(matches(m(0, '', 1), flt({ flag: f }))).toBe(picked)
      expect(matches(m(0, '', 0), flt({ flag: f }))).toBe(none)
      expect(matches(m(0, '', -1), flt({ flag: f }))).toBe(rej)
    }
  })
  it('условия через «и»', () => {
    const f = flt({ minRating: 3, colors: ['green'], flag: 'notRejected' })
    expect(matches(m(4, 'green', 0), f)).toBe(true)
    expect(matches(m(4, 'green', -1), f)).toBe(false)
    expect(matches(m(2, 'green', 0), f)).toBe(false)
    expect(matches(m(4, 'red', 0), f)).toBe(false)
  })
})

describe('filterSummary / isActive', () => {
  it('пустой фильтр неактивен', () => {
    expect(isActive(flt())).toBe(false)
    expect(filterSummary(flt())).toBe('')
  })
  it('оценка, цвета, флажок', () => {
    expect(filterSummary(flt({ minRating: 3, colors: ['red', 'blue'] }))).toBe('★≥3 · ●●')
    expect(filterSummary(flt({ minRating: -1, colors: [''], flag: 'rejected' }))).toBe('без оценки · ○ · отклонённые')
    expect(isActive(flt({ flag: 'picked' }))).toBe(true)
  })
})

describe('filterPhotos', () => {
  const photos = [ph('a', { rating: 5 }), ph('b', { rating: 2, flag: -1 }), ph('c')]
  const own = { c: { rating: 4 }, a: { flag: -1 as const } }
  it('без условий — тот же массив', () => {
    expect(filterPhotos(photos, own, flt())).toBe(photos)
  })
  it('по действующей отметке (своя главнее файла)', () => {
    expect(filterPhotos(photos, own, flt({ minRating: 4 })).map((p) => p.id)).toEqual(['a', 'c'])
    expect(filterPhotos(photos, own, flt({ flag: 'notRejected' })).map((p) => p.id)).toEqual(['c'])
  })
})

describe('autoPick', () => {
  const photos = ['a', 'b', 'c', 'd'].map((id) => ph(id))
  const used = new Set(['a'])
  const bg = new Set<string>()
  const own = { b: { flag: -1 as const } }
  it('без выделения: свободные видимые без отклонённых, считает пропущенные', () => {
    expect(autoPick(photos, [], used, bg, own, true)).toEqual({ ids: ['c', 'd'], skipped: 1 })
  })
  it('только видимые: что вне списка — не берём', () => {
    expect(autoPick(photos.slice(2), [], used, bg, own, true)).toEqual({ ids: ['c', 'd'], skipped: 0 })
  })
  it('с выделением — как есть, отклонённые тоже; автосборка берёт и стоящие в альбоме', () => {
    expect(autoPick(photos, ['a', 'b', 'c'], used, bg, own, true)).toEqual({ ids: ['a', 'b', 'c'], skipped: 0 })
  })
  it('с выделением для заполнения мест — без стоящих в альбоме', () => {
    expect(autoPick(photos, ['a', 'b', 'c'], used, bg, own, false)).toEqual({ ids: ['b', 'c'], skipped: 0 })
  })
  it('тексты тоста', () => {
    expect(withSkipped('Собрано разворотов: 2', 3)).toBe('Собрано разворотов: 2. Отклонённые не взяты: 3')
    expect(withSkipped('Собрано разворотов: 2', 0)).toBe('Собрано разворотов: 2')
    expect(emptyReason(false, 2, false)).toBe('Отклонённые не взяты: 2')
    expect(emptyReason(true, 0, true)).toBe('Выделенные фото уже в альбоме')
    expect(emptyReason(false, 0, true)).toBe('Под фильтр не подходит ни одно свободное фото')
    expect(emptyReason(false, 0, false)).toBe('Все фото уже в альбоме')
  })
})
