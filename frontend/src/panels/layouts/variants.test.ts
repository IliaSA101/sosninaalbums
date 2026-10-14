import { describe, expect, it } from 'vitest'
import type { Choice, SpreadView } from '../../base/core/types'
import {
  activeFamily,
  anyVaries,
  familiesIn,
  galleryCount,
  galleryLines,
  shownRow,
  stillSpread,
  twinsLabel,
  variantCounter,
  varies,
  varyTargets,
} from './variants'

describe('variantCounter', () => {
  it('без продолжения — как в 0.9', () => {
    expect(variantCounter({ variant: 2, variants: 24, more: false })).toBe('3 / 24')
  })
  it('«+», когда ряд можно досчитать', () => {
    expect(variantCounter({ variant: 0, variants: 12, more: true })).toBe('1 / 12+')
    expect(variantCounter({ variant: 12, variants: 24, more: true })).toBe('13 / 24+')
  })
  it('раскладки нет в ряду — тире', () => {
    expect(variantCounter({ variant: -1, variants: 12, more: true })).toBe('— / 12+')
  })
})

describe('stillSpread', () => {
  it('один вариант без продолжения — листать нечего', () => {
    expect(stillSpread({ variant: 0, variants: 1, more: false })).toBe(true)
  })
  it('с продолжением листать можно', () => {
    expect(stillSpread({ variant: 0, variants: 1, more: true })).toBe(false)
  })
  it('вне ряда листание возвращает в ряд', () => {
    expect(stillSpread({ variant: -1, variants: 1, more: false })).toBe(false)
  })
})

// ---------- 041 ----------
const ch = (key: string, family: Choice['family'], twin = -1): Choice => ({ key, rects: [], family, twin })

describe('familiesIn', () => {
  it('только семейства, что есть, в порядке сегментов', () => {
    const rows = [[ch('a', 'air'), ch('b', 'dense')], [ch('c', 'air')], [ch('d', 'bleed')]]
    expect(familiesIn(rows).map((f) => f.label)).toEqual(['Плотно', 'Воздух', 'В край'])
  })
  it('Go не размечает (пусто или нет поля) — список пуст, фильтра нет', () => {
    expect(familiesIn([[{ key: 'a', rects: [] }, ch('b', '')], []])).toEqual([])
  })
})

describe('activeFamily', () => {
  it('исчезнувшее семейство — «Все»', () => {
    const present = [{ id: 'dense' as const }]
    expect(activeFamily('dense', present)).toBe('dense')
    expect(activeFamily('air', present)).toBe('all')
    expect(activeFamily('all', present)).toBe('all')
  })
})

describe('shownRow', () => {
  const list = [ch('a', 'dense'), ch('b', 'dense', 0), ch('c', 'air'), ch('d', 'air', 2), ch('e', 'air', 2)]
  it('«Все»: похожие свёрнуты, счётчик — сколько скрыто', () => {
    const r = shownRow(list, { family: 'all', open: false })
    expect(r.shown.map((c) => c.key)).toEqual(['a', 'c'])
    expect(r.hidden).toBe(3)
  })
  it('раскрыты — все на своих местах, счётчик тот же', () => {
    const r = shownRow(list, { family: 'all', open: true })
    expect(r.shown.map((c) => c.key)).toEqual(['a', 'b', 'c', 'd', 'e'])
    expect(r.hidden).toBe(3)
  })
  it('текущая раскладка видна всегда, даже если похожая, и в счётчик не входит', () => {
    const r = shownRow(list, { family: 'all', current: 'd', open: false })
    expect(r.shown.map((c) => c.key)).toEqual(['a', 'c', 'd'])
    expect(r.hidden).toBe(2)
  })
  it('фильтр семейства отбирает до свёртки', () => {
    const r = shownRow(list, { family: 'air', open: false })
    expect(r.shown.map((c) => c.key)).toEqual(['c'])
    expect(r.hidden).toBe(2)
    expect(shownRow(list, { family: 'hero', open: false })).toEqual({ shown: [], hidden: 0 })
  })
  it('близнец другого семейства не сворачивается: фильтр семейства не прячет чужое', () => {
    const mixed = [ch('a', 'dense'), ch('b', 'air', 0)]
    expect(shownRow(mixed, { family: 'all', open: false })).toEqual({ shown: mixed, hidden: 0 })
    expect(shownRow(mixed, { family: 'air', open: false }).shown.map((c) => c.key)).toEqual(['b'])
  })
  it('без разметки Go (нет twin) ряд как есть', () => {
    const plain: Choice[] = [
      { key: 'a', rects: [] },
      { key: 'b', rects: [] },
    ]
    expect(shownRow(plain, { family: 'all', open: false })).toEqual({ shown: plain, hidden: 0 })
  })
})

describe('twinsLabel', () => {
  it('«Похожие: N» и «Свернуть похожие»', () => {
    expect(twinsLabel(3, false)).toBe('Похожие: 3')
    expect(twinsLabel(3, true)).toBe('Свернуть похожие')
  })
})

describe('varyTargets', () => {
  it('выделенные — как есть, нет выделения — текущий, нет и его — пусто', () => {
    expect(varyTargets(['b', 'd'], 'a')).toEqual(['b', 'd'])
    expect(varyTargets([], 'a')).toEqual(['a'])
    expect(varyTargets([], undefined)).toEqual([])
    expect(varyTargets([], '')).toEqual([])
  })
})

describe('varies / anyVaries', () => {
  const sp = (id: string, o: Partial<SpreadView> = {}) => ({ id, photos: ['p'], grid: false, kind: '', ...o }) as unknown as SpreadView
  const all = [sp('a'), sp('grid', { grid: true }), sp('vig', { kind: 'vignette' }), sp('empty', { photos: [] })]
  it('«Авто» с фото — да; сетка («Ручная»), виньетка и пустой — нет', () => {
    expect(all.map(varies)).toEqual([true, false, false, false])
  })
  it('есть хоть одна подходящая цель — да; все сетки/виньетки или одна «Ручная» — нет', () => {
    expect(anyVaries(all, ['grid', 'a'])).toBe(true)
    expect(anyVaries(all, ['grid', 'vig'])).toBe(false)
    expect(anyVaries(all, ['grid'])).toBe(false)
    expect(anyVaries(all, ['nope'])).toBe(false)
    expect(anyVaries(all, [])).toBe(false)
  })
})

describe('galleryLines', () => {
  const rows = { inOrder: [ch('a', 'dense')], rearranged: [], extra: [] as Choice[] }
  it('у разворота «Другие варианты» есть всегда, пустой — с текстом в ряду', () => {
    const l = galleryLines('spread', rows, { why: 'почему', canMore: true })
    expect(l.map((x) => x.label)).toEqual(['С перестановкой порядка', 'В порядке ленты', 'Другие варианты'])
    expect(l[0].empty).toBe('почему')
    expect(l[1].more).toBe(true)
    expect(l[2]).toMatchObject({ list: [], empty: 'Нет других вариантов', more: false })
  })
  it('у страницы третьего ряда нет', () => {
    expect(galleryLines('left', rows, { why: '', canMore: false }).map((x) => x.label)).toEqual([
      'С перестановкой порядка',
      'В порядке ленты',
    ])
  })
})

describe('galleryCount', () => {
  const rows = { inOrder: [1, 2, 3], rearranged: [4, 5], extra: [6, 7, 8, 9] }
  it('разворот: по порядку + «Другие», перестановка не входит', () => {
    expect(galleryCount(true, rows)).toBe(7)
  })
  it('разворот без «Других» — только ряд по порядку', () => {
    expect(galleryCount(true, { ...rows, extra: [] })).toBe(3)
  })
  it('страница: по порядку + перестановка, как раньше', () => {
    expect(galleryCount(false, { ...rows, extra: [] })).toBe(5)
  })
})
