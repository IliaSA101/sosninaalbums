import { describe, expect, it } from 'vitest'
import { VIG_FAMILIES, activeFamily, familiesIn, segOn, shownTpls, visibleTpls, type FamilyItem } from './tplfilter'

const t = (id: string, family?: string, own?: boolean): FamilyItem => ({ id, family, own })
const list = [t('a', 'side'), t('b', 'side'), t('c', 'big'), t('d', 'group'), t('my:1', undefined, true)]

describe('VIG_FAMILIES', () => {
  it('шесть видов в порядке решения 123', () => {
    expect(VIG_FAMILIES.map((f) => f.label)).toEqual(['С края', 'Крупно', 'В ряду', 'С групповым', 'Россыпь', 'Предметники'])
  })
})

describe('familiesIn', () => {
  it('пустые виды скрыты, порядок — как в VIG_FAMILIES', () => {
    expect(familiesIn(list).map((f) => f.id)).toEqual(['side', 'big', 'group'])
  })
  it('один вид или свои шаблоны одни — фильтра нет', () => {
    expect(familiesIn([t('a', 'side'), t('b', 'side')])).toEqual([])
    expect(familiesIn([t('my:1', 'side', true), t('my:2', undefined, true)])).toEqual([])
    expect(familiesIn([])).toEqual([])
  })
  it('вид без разметки Go (до G3) не считается', () => {
    expect(familiesIn([t('a'), t('b'), t('c', 'big')])).toEqual([])
  })
})

describe('activeFamily', () => {
  const present = familiesIn(list)
  it('вид есть — остаётся, пропал — «Все»', () => {
    expect(activeFamily('big', present)).toBe('big')
    expect(activeFamily('round', present)).toBe('all')
    expect(activeFamily('all', present)).toBe('all')
  })
})

describe('visibleTpls (Пробел в ▦)', () => {
  it('«Все» — всё в порядке Go', () => {
    expect(visibleTpls(list, 'all').map((c) => c.id)).toEqual(['a', 'b', 'c', 'd', 'my:1'])
  })
  it('вид оставляет свой вид, текущий и свои шаблоны — как плитки в ▦', () => {
    expect(visibleTpls(list, 'big', 'a').map((c) => c.id)).toEqual(['a', 'c', 'my:1'])
  })
  it('вида нет среди каталожных (числа поменялись) — «Все»', () => {
    expect(visibleTpls(list, 'round').map((c) => c.id)).toEqual(['a', 'b', 'c', 'd', 'my:1'])
  })
  it('порядок исходного списка сохраняется, свои остаются на своих местах', () => {
    expect(visibleTpls([t('my:1', undefined, true), ...list.slice(0, 4)], 'side').map((c) => c.id)).toEqual(['my:1', 'a', 'b'])
  })
})

describe('shownTpls', () => {
  it('«Все» — всё как есть', () => {
    expect(shownTpls(list, 'all').map((c) => c.id)).toEqual(['a', 'b', 'c', 'd', 'my:1'])
  })
  it('вид оставляет свой вид и все свои шаблоны', () => {
    expect(shownTpls(list, 'big').map((c) => c.id)).toEqual(['c', 'my:1'])
  })
  it('текущий виден всегда, даже другого вида', () => {
    expect(shownTpls(list, 'big', 'a').map((c) => c.id)).toEqual(['a', 'c', 'my:1'])
  })
  it('стрелки по видимым: список после фильтра — то, по чему ходит gridStep', () => {
    expect(shownTpls(list, 'side', 'a').map((c) => c.id)).toEqual(['a', 'b', 'my:1'])
  })
  it('список не мутируется', () => {
    const before = list.length
    shownTpls(list, 'group').push(t('x'))
    expect(list).toHaveLength(before)
  })
})

describe('segOn', () => {
  it('одиночный выбор — по value', () => {
    expect(segOn('a', 'a')).toBe(true)
    expect(segOn('b', 'a')).toBe(false)
    expect(segOn('a', undefined)).toBe(false)
  })
  it('набор (065) главнее value: нажато несколько', () => {
    expect(segOn('a', undefined, ['a', 'c'])).toBe(true)
    expect(segOn('c', 'zzz', ['a', 'c'])).toBe(true)
    expect(segOn('b', 'b', ['a', 'c'])).toBe(false)
    expect(segOn('a', undefined, [])).toBe(false)
  })
})
