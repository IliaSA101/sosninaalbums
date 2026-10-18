import { describe, expect, it } from 'vitest'
import type { Choice, SpreadView } from '../../base/core/types'
import { FAMILY_IDS } from '../../base/prefs/families'
import {
  activeFamily,
  anyVaries,
  FAMILIES,
  familiesIn,
  galleryCount,
  galleryLines,
  galleryOrders,
  galleryRow,
  shownRow,
  startOrder,
  stillSpread,
  tileTitle,
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
const ch = (key: string, family: Choice['family'], twin = -1): Choice => ({ key, rects: [], family, twin, formName: '' })
// Go шлёт все поля всегда; «без разметки» — пустые family и formName, twin −1.
const bare = (key: string): Choice => ch(key, '')

describe('familiesIn', () => {
  it('только семейства, что есть, в порядке сегментов', () => {
    const rows = [[ch('a', 'air'), ch('b', 'dense')], [ch('c', 'air')], [ch('d', 'bleed')]]
    expect(familiesIn(rows).map((f) => f.label)).toEqual(['Плотно', 'Воздух', 'В край'])
  })
  it('Go не размечает (пусто или нет поля) — список пуст, фильтра нет', () => {
    expect(familiesIn([[bare('a'), ch('b', '')], []])).toEqual([])
  })
})

describe('051: «Ленты» и «Фигуры»', () => {
  it('сегменты — после «В край», пустые не показываются', () => {
    const rows = [[ch('a', 'figure'), ch('b', 'strip'), ch('c', 'bleed'), ch('d', 'dense')]]
    expect(familiesIn(rows).map((f) => f.label)).toEqual(['Плотно', 'В край', 'Ленты', 'Фигуры'])
    expect(familiesIn([[ch('a', 'strip')]]).map((f) => f.id)).toEqual(['strip'])
    expect(familiesIn([[ch('a', 'dense')]]).map((f) => f.id)).toEqual(['dense'])
  })
  it('фильтр «Ленты» оставляет ленты и текущую; «Фигуры» без фигур — пусто, текущая видна', () => {
    const list = [ch('a', 'strip'), ch('b', 'figure'), ch('c', 'dense')]
    expect(shownRow(list, { family: 'strip', open: false }).shown.map((c) => c.key)).toEqual(['a'])
    expect(shownRow(list, { family: 'figure', current: 'c', open: false }).shown.map((c) => c.key)).toEqual(['b', 'c'])
    expect(shownRow([ch('a', 'strip')], { family: 'figure', open: false }).shown).toEqual([])
  })
  it('фильтр выбран, а лент в подборке не осталось — «Все»', () => {
    expect(activeFamily('strip', familiesIn([[ch('a', 'dense')]]))).toBe('all')
  })
  it('под фильтром без плиток — «Нет раскладок этого семейства»', () => {
    const lines = galleryLines('spread', { inOrder: [ch('a', 'dense')], rearranged: [], extra: [] }, { why: '', canMore: false })
    expect(galleryRow(lines, 'inOrder', { family: 'strip', openTwins: [] }).empty).toBe('Нет раскладок этого семейства')
  })
  it('подсказка плитки — название формы; у мозаики и без поля — нет', () => {
    expect(tileTitle({ formName: 'Лесенка' })).toBe('Лесенка')
    expect(tileTitle({ formName: '' })).toBeUndefined()
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

describe('FAMILIES', () => {
  it('065: порядок и имена те же, что у FAMILY_IDS основы (память окна автосборки)', () => {
    expect(FAMILIES.map((f) => f.id)).toEqual([...FAMILY_IDS])
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
  it('текущая раскладка видна и под фильтром другого семейства, в счётчик не входит (050)', () => {
    const r = shownRow(list, { family: 'air', current: 'a', open: false })
    expect(r.shown.map((c) => c.key)).toEqual(['a', 'c'])
    expect(r.hidden).toBe(2)
  })
  it('фильтр семейства отбирает до свёртки', () => {
    const r = shownRow(list, { family: 'air', open: false })
    expect(r.shown.map((c) => c.key)).toEqual(['c'])
    expect(r.hidden).toBe(2)
    expect(shownRow(list, { family: 'hero', open: false })).toEqual({ shown: [], hidden: 0 })
  })
  it('065: набор семейств — предикат; те же правила свёртки и текущей', () => {
    const set = (c: { family: string }) => c.family === 'air' || c.family === 'dense'
    expect(shownRow(list, { family: set, open: false })).toEqual(shownRow(list, { family: 'all', open: false }))
    const onlyAir = (c: { family: string }) => c.family === 'air'
    expect(shownRow(list, { family: onlyAir, open: false })).toEqual(shownRow(list, { family: 'air', open: false }))
    expect(shownRow(list, { family: onlyAir, current: 'a', open: false })).toEqual(
      shownRow(list, { family: 'air', current: 'a', open: false }),
    )
    expect(shownRow(list, { family: () => false, open: false })).toEqual({ shown: [], hidden: 0 })
  })
  it('близнец другого семейства не сворачивается: фильтр семейства не прячет чужое', () => {
    const mixed = [ch('a', 'dense'), ch('b', 'air', 0)]
    expect(shownRow(mixed, { family: 'all', open: false })).toEqual({ shown: mixed, hidden: 0 })
    expect(shownRow(mixed, { family: 'air', open: false }).shown.map((c) => c.key)).toEqual(['b'])
  })
  it('без разметки Go (нет twin) ряд как есть', () => {
    const plain: Choice[] = [bare('a'), bare('b')]
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

describe('startOrder (050)', () => {
  const page = (rearranged: boolean) => ({ photos: 2, variant: 0, variants: 3, rearranged })
  it('текущая из перестановки — «С перестановкой» (разворот)', () => {
    expect(startOrder({ rearranged: true, pages: null }, 'spread')).toBe('rearranged')
  })
  it('текущая из перестановки — «С перестановкой» (страница своей стороны)', () => {
    const s = { rearranged: false, pages: [page(true), page(false)] }
    expect(startOrder(s, 'left')).toBe('rearranged')
    expect(startOrder(s, 'right')).toBe('inOrder')
  })
  it('иначе и без листа — «В порядке ленты»', () => {
    expect(startOrder({ rearranged: false, pages: null }, 'spread')).toBe('inOrder')
    expect(startOrder({ rearranged: false, pages: null }, 'left')).toBe('inOrder')
    expect(startOrder(undefined, 'spread')).toBe('inOrder')
  })
})

describe('galleryRow (050)', () => {
  const nine = (n: number, fam: Choice['family'], pre: string) => Array.from({ length: n }, (_, i) => ch(`${pre}${i}`, fam))
  const rows = (o: Partial<{ inOrder: Choice[]; rearranged: Choice[]; extra: Choice[] }> = {}) => ({
    inOrder: nine(2, 'dense', 'a'),
    rearranged: [],
    extra: nine(2, 'air', 'x'),
    ...o,
  })
  const lines = (r = rows(), scope: 'spread' | 'left' = 'spread', canMore = false) => galleryLines(scope, r, { why: 'почему', canMore })
  const all = { family: 'all' as const, openTwins: [] }

  it('по ленте: «Авто», затем «Другие варианты» за чертой', () => {
    const r = galleryRow(lines(), 'inOrder', all)
    expect(r.parts.map((p) => [p.line, p.sep])).toEqual([
      ['В порядке ленты', ''],
      ['Другие варианты', 'другие'],
    ])
    expect(r.empty).toBe('')
  })
  it('черта только между непустыми частями', () => {
    const onlyExtra = galleryRow(lines(rows({ inOrder: [] })), 'inOrder', all)
    expect(onlyExtra.parts.map((p) => [p.line, p.sep])).toEqual([['Другие варианты', '']])
    const onlyMain = galleryRow(lines(rows({ extra: [] })), 'inOrder', all)
    expect(onlyMain.parts.map((p) => p.line)).toEqual(['В порядке ленты'])
    // фильтр оставил только «Других»
    const fam = galleryRow(lines(), 'inOrder', { ...all, family: 'air' })
    expect(fam.parts.map((p) => [p.line, p.sep])).toEqual([['Другие варианты', '']])
  })
  it('«Ещё» — в конце части «Авто», до черты', () => {
    const r = galleryRow(lines(rows(), 'spread', true), 'inOrder', all)
    expect(r.parts.map((p) => p.more)).toEqual([true, false])
    // часть без раскладок, но с «Ещё» — не пустая
    const empty = galleryRow(lines(rows({ inOrder: [] }), 'spread', true), 'inOrder', all)
    expect(empty.parts.map((p) => [p.line, p.sep, p.more])).toEqual([
      ['В порядке ленты', '', true],
      ['Другие варианты', 'другие', false],
    ])
  })
  it('с перестановкой — только свой ряд, пустой — причина', () => {
    const r = galleryRow(lines(rows({ rearranged: nine(3, 'hero', 'r') })), 'rearranged', all)
    expect(r.parts.map((p) => [p.line, p.shown.length])).toEqual([['С перестановкой порядка', 3]])
    const none = galleryRow(lines(), 'rearranged', all)
    expect(none).toEqual({ parts: [], empty: 'почему' })
  })
  it('фильтр скрыл всё — «Нет раскладок этого семейства»', () => {
    const r = galleryRow(lines(), 'inOrder', { ...all, family: 'bleed' })
    expect(r).toEqual({ parts: [], empty: 'Нет раскладок этого семейства' })
  })
  it('текущая видна под фильтром другого семейства', () => {
    const r = galleryRow(lines(), 'inOrder', { ...all, family: 'bleed', current: 'a1' })
    expect(r.parts.map((p) => p.shown.map((c) => c.key))).toEqual([['a1']])
  })
  it('нет раскладок совсем — текст ряда', () => {
    expect(galleryRow(lines(rows({ inOrder: [], extra: [] })), 'inOrder', all)).toEqual({ parts: [], empty: 'Нет раскладок' })
  })
  it('у страницы «Других» нет', () => {
    const r = galleryRow(lines(rows(), 'left'), 'inOrder', all)
    expect(r.parts.map((p) => p.line)).toEqual(['В порядке ленты'])
  })
  it('«Похожие» раскрываются по подписи линии', () => {
    const twins = [ch('a', 'dense'), ch('b', 'dense', 0)]
    const closed = galleryRow(lines(rows({ inOrder: twins, extra: [] })), 'inOrder', all)
    expect([closed.parts[0].shown.length, closed.parts[0].hidden]).toEqual([1, 1])
    const open = galleryRow(lines(rows({ inOrder: twins, extra: [] })), 'inOrder', { ...all, openTwins: ['В порядке ленты'] })
    expect([open.parts[0].shown.length, open.parts[0].hidden]).toEqual([2, 1])
  })
})

describe('galleryOrders (050)', () => {
  const mk = (r: { inOrder: Choice[]; rearranged: Choice[]; extra: Choice[] }) =>
    galleryLines('spread', r, { why: 'Перестановка доступна для разворотов до 7 фото', canMore: false })
  const a = (k: string, f: Choice['family']) => ch(k, f)
  it('числа с учётом фильтра, «В порядке ленты» считает и «Другие»', () => {
    const lines = mk({ inOrder: [a('1', 'dense'), a('2', 'air')], rearranged: [a('3', 'air')], extra: [a('4', 'air'), a('5', 'hero')] })
    expect(galleryOrders(lines, 'all').map((i) => [i.id, i.count])).toEqual([
      ['inOrder', 4],
      ['rearranged', 1],
    ])
    expect(galleryOrders(lines, 'air').map((i) => [i.id, i.count])).toEqual([
      ['inOrder', 2],
      ['rearranged', 1],
    ])
  })
  it('пустая перестановка — пункт выключен, причина в подсказке', () => {
    const items = galleryOrders(mk({ inOrder: [a('1', 'dense')], rearranged: [], extra: [] }), 'all')
    expect(items[1]).toMatchObject({ id: 'rearranged', count: 0, off: true, title: 'Перестановка доступна для разворотов до 7 фото' })
    expect(items[0].off).toBeFalsy()
  })
  it('фильтр обнулил перестановку — пункт не серый: в ряду будет «Нет раскладок этого семейства»', () => {
    const items = galleryOrders(mk({ inOrder: [], rearranged: [a('3', 'air')], extra: [] }), 'dense')
    expect(items[1]).toMatchObject({ count: 0 })
    expect(items[1].off).toBeFalsy()
  })
})
