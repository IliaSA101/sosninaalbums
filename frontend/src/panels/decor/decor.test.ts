import { describe, expect, it } from 'vitest'
import {
  ALL,
  MINE,
  RECENT,
  defaultScope,
  deleteText,
  emptyText,
  firstTheme,
  importSummary,
  importTheme,
  presetOf,
  presetOfView,
  scopeIn,
  sideItems,
  standingId,
  standingSticker,
  themeItems,
  tilePx,
  tileSrc,
  uploadHint,
  whereItems,
  type DecorCatalog,
  type DecorItem,
} from './decor'

const item = (id: string, theme: string, over: Partial<DecorItem> = {}): DecorItem => ({
  id,
  theme,
  name: id,
  url: `/media/gen/pat?k=${id}&v=1`,
  own: id.startsWith('my:'),
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})

const cat = (over: Partial<DecorCatalog> = {}): DecorCatalog => ({
  kind: 'bg',
  themes: [
    { id: RECENT, name: 'Недавние', count: 2 },
    { id: ALL, name: 'Все', count: 4 },
    { id: 'basic', name: 'Универсальные', count: 2 },
    { id: 'wedding', name: 'Свадьба', count: 1 },
    { id: MINE, name: 'Мои', count: 1 },
  ],
  items: [item('sa:a', 'basic'), item('sa:b', 'basic'), item('sa:c', 'wedding'), item('my:0123456789abcdef', MINE, { name: 'Моё' })],
  recent: ['sa:c', 'my:0123456789abcdef', 'sa:gone'],
  readOnly: false,
  ...over,
})

describe('темы окна', () => {
  it('в списке слева числа тем из Go, порядок Go', () => {
    expect(sideItems(cat())).toEqual([
      { id: 'recent', label: 'Недавние', count: 2 },
      { id: 'all', label: 'Все', count: 4 },
      { id: 'basic', label: 'Универсальные', count: 2 },
      { id: 'wedding', label: 'Свадьба', count: 1 },
      { id: 'mine', label: 'Мои', count: 1 },
    ])
    expect(sideItems(null)).toEqual([])
  })

  it('первая тема: «Недавние», если не пусты, иначе «Все»; прежняя остаётся', () => {
    expect(firstTheme(cat())).toBe(RECENT)
    const empty = cat({
      themes: [
        { id: RECENT, name: 'Недавние', count: 0 },
        { id: ALL, name: 'Все', count: 4 },
      ],
    })
    expect(firstTheme(empty)).toBe(ALL)
    expect(firstTheme(cat(), 'wedding')).toBe('wedding')
    expect(firstTheme(cat(), 'исчезла')).toBe(RECENT)
    expect(firstTheme(null)).toBe(ALL)
  })

  it('плитки темы: «Все» — каталог, «Недавние» — в порядке поставленных без пропавших, остальные — по теме', () => {
    const c = cat()
    expect(themeItems(c, ALL)).toHaveLength(4)
    expect(themeItems(c, RECENT).map((i) => i.id)).toEqual(['sa:c', 'my:0123456789abcdef'])
    expect(themeItems(c, 'basic').map((i) => i.id)).toEqual(['sa:a', 'sa:b'])
    expect(themeItems(c, 'kids')).toEqual([])
    expect(themeItems(null, ALL)).toEqual([])
  })

  it('загрузка из «Все» и «Недавних» кладёт в «Мои», из темы — в эту тему', () => {
    expect(importTheme(ALL)).toBe(MINE)
    expect(importTheme(RECENT)).toBe(MINE)
    expect(importTheme('wedding')).toBe('wedding')
    expect(importTheme(MINE)).toBe(MINE)
  })

  it('пустая тема: «Мои» зовёт загрузить, а при чтении без загрузки — нет', () => {
    expect(emptyText(MINE, false)).toBe('Загрузите свои кнопкой выше')
    expect(emptyText(MINE, true)).not.toContain('кнопкой')
    expect(emptyText('basic', false)).toBe('В этой теме пока пусто')
  })
})

describe('плитки', () => {
  it('px: ширина × dpr, шаг 64, не больше 4096', () => {
    expect(tilePx(168, 1)).toBe(192)
    expect(tilePx(168, 2)).toBe(384)
    expect(tilePx(168, 1.5)).toBe(256)
    expect(tilePx(10000, 2)).toBe(4096)
    expect(tilePx(0, 1)).toBe(64)
    expect(tilePx(NaN, 1)).toBe(64)
    expect(tilePx(100, 0)).toBe(128)
  })
  it('адрес: px дописывается к адресу Go', () => {
    expect(tileSrc('/media/lib/t/abc?v=1', 192)).toBe('/media/lib/t/abc?v=1&px=192')
    expect(tileSrc('/x', 64)).toBe('/x?px=64')
  })
})

describe('«Куда»', () => {
  const sp = { id: 's1', mid: 300 }
  it('разворот — весь, левая, правая; подпись «весь» от слов листа', () => {
    expect(whereItems(sp, undefined, 'Весь разворот').map((i) => [i.id, i.label])).toEqual([
      ['spread', 'Весь разворот'],
      ['left', 'Левая'],
      ['right', 'Правая'],
    ])
  })
  it('лист без шва (виньетка-лист) — одна область: переключатель прячется', () => {
    expect(whereItems({ id: 's2', mid: 0 }, undefined, 'Весь лист')).toEqual([{ id: 'spread', label: 'Весь лист' }])
  })
  // Области фото фона по раскладкам — как их отдаёт Go (CoverView.scopes, 061)
  const front = { scopes: [{ scope: 'right' as const, label: 'Лицевая' }] }
  const wrap = { scopes: [{ scope: 'spread' as const, label: 'Вся обложка' }] }
  const both = {
    scopes: [
      { scope: 'left' as const, label: 'Задняя' },
      { scope: 'spine' as const, label: 'Корешок' },
      { scope: 'right' as const, label: 'Лицевая' },
    ],
  }
  const cover = { id: 'cover', mid: 300 }
  it('обложка — сторонки по раскладке', () => {
    expect(whereItems(cover, front, 'Весь разворот').map((i) => [i.id, i.label])).toEqual([['right', 'Лицевая']])
    expect(whereItems(cover, wrap, 'Весь разворот').map((i) => [i.id, i.label])).toEqual([['spread', 'Вся обложка']])
  })
  it('узор и цвет корешок не принимают: в «Куда» его нет', () => {
    expect(whereItems(cover, both, 'Весь разворот').map((i) => [i.id, i.label])).toEqual([
      ['left', 'Задняя'],
      ['right', 'Лицевая'],
    ])
  })
  it('листа нет — безопасная одна область', () => {
    expect(whereItems(undefined, undefined, 'Весь разворот')).toHaveLength(1)
  })
  it('область по умолчанию: разворот — весь, обложка — первая сторонка (не корешок)', () => {
    expect(defaultScope(sp, undefined)).toBe('spread')
    expect(defaultScope(cover, front)).toBe('right')
    expect(defaultScope(cover, both)).toBe('left')
    expect(defaultScope(cover, wrap)).toBe('spread')
    expect(defaultScope(undefined, undefined)).toBe('spread')
  })
  it('пропавшая из списка область заменяется первой доступной', () => {
    const items = whereItems(cover, front, 'x')
    expect(scopeIn(items, 'left')).toBe('right')
    expect(scopeIn(items, 'spine')).toBe('right')
    expect(scopeIn(items, 'right')).toBe('right')
    expect(scopeIn([], 'left')).toBe('spread')
  })
})

describe('итог «Загрузить своё…»', () => {
  const add = (n: number) => Array.from({ length: n }, (_, i) => item(`my:${i}`, MINE))
  it('диалог закрыли ничего не выбрав — тоста нет', () => {
    expect(importSummary({ added: [], skipped: [] })).toBeNull()
  })
  it('только добавлено', () => {
    expect(importSummary({ added: add(3), skipped: [] })).toEqual({ text: 'Добавлено: 3', error: false })
  })
  it('добавлено и пропущено — с причинами', () => {
    expect(importSummary({ added: add(3), skipped: [{ name: 'a.png', why: 'уже есть' }] })).toEqual({
      text: 'Добавлено: 3, пропущено: 1 (a.png — уже есть)',
      error: false,
    })
  })
  it('причин больше двух — «и ещё N»; ничего не добавлено — ошибка', () => {
    const skipped = [
      { name: 'a', why: 'х' },
      { name: 'b', why: 'у' },
      { name: 'c', why: 'з' },
      { name: 'd', why: 'ф' },
    ]
    expect(importSummary({ added: [], skipped })).toEqual({ text: 'Добавлено: 0, пропущено: 4 (a — х; b — у и ещё 2)', error: true })
  })
})

describe('тексты', () => {
  it('вопрос об убирании: имя, «из библиотеки» и «где стоит, не изменится»', () => {
    const t = deleteText('Мой фон')
    expect(t).toContain('«Мой фон»')
    expect(t).toContain('из библиотеки')
    expect(t).toContain('не изменится')
  })

  it('подсказка загрузки: SVG называется только у стикеров', () => {
    expect(uploadHint('sticker')).toContain('SVG')
    expect(uploadHint('bg')).not.toContain('SVG')
  })
})

describe('какая плитка стоит сейчас (отметка в окне)', () => {
  const pat = (k: string, i = '') => `/media/gen/pat?k=${k}${i ? `&i=${i}` : ''}&w=600&h=300&v=1`
  const items = [
    item('sa:thin-stripes', 'basic', { name: 'Тонкие полосы', url: pat('stripes') }),
    item('sa:blue-stripes', 'school', { name: 'Голубые полосы', url: pat('stripes', 'c9d9ec') }),
    item('sa:gold-dots', 'wedding', { name: 'Золотой горох', url: pat('dots', 'd4b26a') }),
    item('my:0123456789abcdef', MINE, { url: '/media/lib/t/0123456789abcdef?v=1' }),
  ]
  const sheet = (over: Record<string, unknown>) => ({ backdrops: null, patterns: [], ...over }) as never
  const pv = (kind: string, inks: string[]) => ({
    scope: 'spread',
    area: { x: 0, y: 0, w: 600, h: 300 },
    pat: { kind, inks },
    name: kind,
    inks: inks.length,
    random: false,
    known: true,
    url: '',
  })

  it('presetOf: по генератору; плитка с большим числом заданных цветов важнее', () => {
    expect(presetOf({ kind: 'stripes', inks: ['#eadccb'] }, items)?.id).toBe('sa:thin-stripes') // «Полосы» — но плитка «Тонкие полосы»
    expect(presetOf({ kind: 'stripes', inks: ['#C9D9EC'] }, items)?.id).toBe('sa:blue-stripes')
    expect(presetOf({ kind: 'dots', inks: ['#d4b26a'] }, items)?.id).toBe('sa:gold-dots')
  })
  it('presetOf: цвета узора не начинаются с цветов плитки или генератора нет — undefined', () => {
    expect(presetOf({ kind: 'dots', inks: ['#d9b779'] }, items)).toBeUndefined()
    expect(presetOf({ kind: 'dots' }, items)).toBeUndefined()
    expect(presetOf({ kind: 'waves', inks: [] }, items)).toBeUndefined()
    expect(presetOf({ kind: 'stripes', inks: [] }, [])).toBeUndefined()
  })
  it('своё из библиотеки — по ключу в области; пропало из каталога — пусто', () => {
    const lib = 'my:0123456789abcdef'
    const sp = sheet({ backdrops: [{ scope: 'spread', photo: 'p', lib }] })
    expect(standingId(sp, 'spread', items)).toBe(lib)
    expect(standingId(sp, 'left', items)).toBe('') // в другой области ничего
    expect(standingId(sp, 'spread', items.slice(0, 3))).toBe('')
  })
  it('узор — по presetOf в своей области; узор новее программы и пустой лист — пусто', () => {
    const sp = sheet({ patterns: [pv('stripes', ['#c9d9ec'])] })
    expect(standingId(sp, 'spread', items)).toBe('sa:blue-stripes')
    expect(standingId(sp, 'right', items)).toBe('')
    expect(standingId(sheet({ patterns: [{ ...pv('stripes', []), known: false }] }), 'spread', items)).toBe('')
    expect(standingId(sheet({}), 'spread', items)).toBe('')
    expect(standingId(undefined, 'spread', items)).toBe('')
  })
  it('preset от Go важнее догадки по цветам (№201): правили цвета у плитки с заданными цветами', () => {
    const sp = sheet({ patterns: [{ ...pv('stripes', ['#c9d9ec']), preset: 'sa:thin-stripes' }] })
    expect(standingId(sp, 'spread', items)).toBe('sa:thin-stripes')
    expect(presetOfView({ pat: { kind: 'stripes' }, preset: 'sa:thin-stripes' }, items)?.id).toBe('sa:thin-stripes')
  })
  it('preset есть, а плитки нет в каталоге (скрыта) — пусто, без догадки; preset пуст — запасной presetOf', () => {
    const sp = sheet({ patterns: [{ ...pv('stripes', ['#c9d9ec']), preset: 'sa:скрытая' }] })
    expect(standingId(sp, 'spread', items)).toBe('')
    expect(presetOfView({ pat: { kind: 'stripes', inks: ['#c9d9ec'] }, preset: '' }, items)?.id).toBe('sa:blue-stripes')
  })
})

describe('какая плитка стоит у стикера (отметка при «Заменить…»)', () => {
  const items = [
    item('sa:heart', 'wedding', { url: '/media/gen/art?k=heart&v=1' }),
    item('my:0123456789abcdef', MINE, { url: '/media/lib/t/0123456789abcdef?v=1' }),
  ]
  const sheet = (st: object) => ({ stickers: [{ id: 'k1', ...st }] }) as never
  it('своё — по ключу; пропало из каталога — пусто', () => {
    expect(standingSticker(sheet({ lib: 'my:0123456789abcdef' }), 'k1', items)).toBe('my:0123456789abcdef')
    expect(standingSticker(sheet({ lib: 'my:ffffffffffffffff' }), 'k1', items)).toBe('')
  })
  it('процедурный — по presetOf; неизвестный генератор и нет стикера — пусто', () => {
    expect(standingSticker(sheet({ known: true, art: { kind: 'heart', inks: [] } }), 'k1', items)).toBe('sa:heart')
    expect(standingSticker(sheet({ known: false, art: { kind: 'heart' } }), 'k1', items)).toBe('')
    expect(standingSticker(sheet({}), 'нет', items)).toBe('')
    expect(standingSticker(sheet({}), undefined, items)).toBe('')
    expect(standingSticker(undefined, 'k1', items)).toBe('')
  })
})
