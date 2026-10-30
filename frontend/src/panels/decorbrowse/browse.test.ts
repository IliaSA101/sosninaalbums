import { describe, expect, it } from 'vitest'
import { type DecorCatalog, type DecorItem } from '../decor/decor'
import { tplSideItems } from '../decor/spreadtpl'
import { BUILTIN_PACK, NO_SEL, browse, formOpts, packOpts, sideCounts, styleOpts, tileLabel, type BrowseSel } from './browse'

// 133-9: отбор набор → вид → стиль, пропавший выбор, стоящая плитка, свои, стили, подпись без приставки, числа слева.
const it_ = (id: string, over: Partial<DecorItem> = {}): DecorItem => ({
  id,
  theme: 'a',
  name: id,
  url: `/media/lib/t/${id}`,
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})
const pk = (id: string, pack: string, packId: string, over: Partial<DecorItem> = {}) => it_(id, { pack, packId, ...over })
const sel = (over: Partial<BrowseSel> = {}): BrowseSel => ({ ...NO_SEL, ...over })

// Каталог: школа (значки ×3, рисунки ×3, уголок, разделитель), свадьба (уголок, разделитель, 2 стикера), встроенные, свои.
const ITEMS: DecorItem[] = [
  pk('s1', 'Школа', 'school', { name: 'Значки: Колокольчик', style: 'Значки' }),
  pk('s2', 'Школа', 'school', { name: 'Значки: Ромашка', style: 'Значки' }),
  pk('s3', 'Школа', 'school', { name: 'Значки: Звонок', style: 'Значки' }),
  pk('s4', 'Школа', 'school', { name: 'Рисунки: Рюкзак', style: 'Рисунки' }),
  pk('s5', 'Школа', 'school', { name: 'Рисунки: Мел', style: 'Рисунки' }),
  pk('s6', 'Школа', 'school', { name: 'Рисунки: Глобус', style: 'Рисунки' }),
  pk('sc', 'Школа', 'school', { name: 'Уголок школьный', form: 'corner' }),
  pk('sd', 'Школа', 'school', { name: 'Разделитель: яблоко', form: 'divider' }),
  pk('w1', 'Свадьба', 'wed', { name: 'Кольца' }),
  pk('w2', 'Свадьба', 'wed', { name: 'Голуби' }),
  pk('wc', 'Свадьба', 'wed', { name: 'Золотой уголок', form: 'corner' }),
  pk('wd', 'Свадьба', 'wed', { name: 'Гирлянда', form: 'divider' }),
  it_('b1', { name: 'Колокольчик', packId: BUILTIN_PACK }),
  it_('b2', { name: 'Звезда', packId: BUILTIN_PACK }),
  it_('my:1', { name: 'Логотип', own: true, theme: 'mine' }),
]
const ids = (l: { id: string }[]) => l.map((i) => i.id)

describe('варианты рядов', () => {
  it('наборы — в порядке каталога, «Встроенные» последними; свои не в счёт', () => {
    expect(packOpts(ITEMS)).toEqual([
      { id: 'school', label: 'Школа' },
      { id: 'wed', label: 'Свадьба' },
      { id: BUILTIN_PACK, label: 'Встроенные' },
    ])
  })

  it('в выборке порядок не появления, а каталога (новые наборы выше)', () => {
    const recent = [ITEMS[8], ITEMS[0], ITEMS[12]] // свадьба, школа, встроенное — как поставили
    expect(packOpts(recent, ITEMS).map((o) => o.id)).toEqual(['school', 'wed', BUILTIN_PACK])
    // встроенные последние, даже если в каталоге они стоят выше
    expect(packOpts([ITEMS[12], ITEMS[0]], [ITEMS[12], ...ITEMS]).map((o) => o.id)).toEqual(['school', BUILTIN_PACK])
  })

  it('один набор — выбирать не из чего; одинаковые названия различаются id', () => {
    expect(packOpts(ITEMS.slice(0, 8))).toEqual([])
    const twins = [pk('a', 'Свадьба', 'wed-1'), pk('b', 'Свадьба', 'wed-2'), pk('c', 'Школа', 'school')]
    expect(packOpts(twins).map((o) => o.label)).toEqual(['Свадьба (wed-1)', 'Свадьба (wed-2)', 'Школа'])
  })

  it('виды — только встреченные; один — ряда нет; свои считаются стикерами', () => {
    expect(formOpts(ITEMS).map((o) => o.label)).toEqual(['Стикеры', 'Уголки', 'Разделители'])
    expect(formOpts(ITEMS.filter((i) => !i.form)).map((o) => o.id)).toEqual([])
    expect(formOpts([ITEMS[8], ITEMS[10]]).map((o) => o.id)).toEqual(['sticker', 'corner'])
    expect(formOpts([ITEMS[14], ITEMS[11]]).map((o) => o.id)).toEqual(['sticker', 'divider'])
  })

  it('стили — от трёх предметов; один стиль — ряда нет', () => {
    expect(styleOpts(ITEMS).map((o) => o.id)).toEqual(['Значки', 'Рисунки'])
    expect(styleOpts(ITEMS.slice(0, 5)).map((o) => o.id)).toEqual([]) // у «Рисунков» лишь два
    expect(styleOpts(ITEMS.slice(0, 3))).toEqual([]) // один стиль
  })

  it('в выборке стили — в порядке каталога, а не появления', () => {
    const asked = [ITEMS[3], ITEMS[4], ITEMS[5], ITEMS[0], ITEMS[1], ITEMS[2]] // «Рисунки» поставлены раньше «Значков»
    expect(styleOpts(asked, ITEMS).map((o) => o.id)).toEqual(['Значки', 'Рисунки'])
    expect(styleOpts(asked).map((o) => o.id)).toEqual(['Рисунки', 'Значки']) // без каталога — по появлению
    expect(browse(asked, ITEMS, NO_SEL).styles.map((o) => o.id)).toEqual(['Значки', 'Рисунки']) // конвейер передаёт каталог
  })

  it('стилей больше шести — шесть самых многочисленных, ничья — порядок каталога, показ в порядке каталога', () => {
    const many: DecorItem[] = []
    // стили 1…7: в стиле k — (k ≤ 2 ? 5 : 3) предметов; ничья между 3…7 решается порядком
    for (let k = 1; k <= 7; k++)
      for (let j = 0; j < (k <= 2 ? 5 : 3); j++) many.push(it_(`m${k}-${j}`, { style: `Стиль${k}`, name: `Стиль${k}: х` }))
    expect(styleOpts(many).map((o) => o.id)).toEqual(['Стиль1', 'Стиль2', 'Стиль3', 'Стиль4', 'Стиль5', 'Стиль6']) // седьмой отрезан
    // самый многочисленный в конце списка попадает в шесть, а ничья отрезает самый поздний
    const tail = [...many.slice(0, -3), ...Array.from({ length: 6 }, (_, j) => it_(`z${j}`, { style: 'Стиль7' }))]
    const got = styleOpts(tail).map((o) => o.id)
    expect(got).toHaveLength(6)
    expect(got).toContain('Стиль7')
    expect(got).not.toContain('Стиль6')
    expect(got).toEqual(['Стиль1', 'Стиль2', 'Стиль3', 'Стиль4', 'Стиль5', 'Стиль7'])
  })
})

describe('конвейер набор → вид → стиль', () => {
  it('без выбора видно всё и все ряды есть', () => {
    const b = browse(ITEMS, ITEMS, NO_SEL)
    expect(b.items).toHaveLength(ITEMS.length)
    expect([b.packs.length, b.forms.length, b.styles.length]).toEqual([3, 3, 2])
  })

  it('набор прячет свои и чужие, варианты вида и стиля берутся из оставшегося', () => {
    const b = browse(ITEMS, ITEMS, sel({ pack: 'wed' }))
    expect(ids(b.items)).toEqual(['w1', 'w2', 'wc', 'wd'])
    expect(b.forms.map((o) => o.id)).toEqual(['sticker', 'corner', 'divider'])
    expect(b.styles).toEqual([]) // у свадьбы стилей нет
  })

  it('набор → вид → стиль друг за другом', () => {
    expect(ids(browse(ITEMS, ITEMS, sel({ pack: 'school', form: 'corner' })).items)).toEqual(['sc'])
    expect(ids(browse(ITEMS, ITEMS, sel({ form: 'divider' })).items)).toEqual(['sd', 'wd'])
    expect(ids(browse(ITEMS, ITEMS, sel({ pack: 'school', form: 'sticker', style: 'Рисунки' })).items)).toEqual(['s4', 's5', 's6'])
    expect(ids(browse(ITEMS, ITEMS, sel({ style: 'Значки' })).items)).toEqual(['s1', 's2', 's3'])
  })

  it('«Стикеры» показывает свои, «Уголки», «Разделители», стиль и набор — прячут', () => {
    expect(ids(browse(ITEMS, ITEMS, sel({ form: 'sticker' })).items)).toContain('my:1')
    for (const s of [sel({ form: 'corner' }), sel({ form: 'divider' }), sel({ style: 'Значки' }), sel({ pack: 'school' })])
      expect(ids(browse(ITEMS, ITEMS, s).items)).not.toContain('my:1')
  })

  it('выбранного в списке нет — в показе «Все», выбор в памяти не стирается', () => {
    const f = sel({ pack: 'wed', form: 'corner', style: 'Значки' })
    const mine = ITEMS.filter((i) => i.own)
    const b = browse(mine, ITEMS, f)
    expect(ids(b.items)).toEqual(['my:1'])
    expect(b.sel).toEqual(NO_SEL)
    expect(f).toEqual({ pack: 'wed', form: 'corner', style: 'Значки' }) // входной объект не тронут
    // в другой теме выбор снова действует
    expect(browse(ITEMS, ITEMS, f).sel).toEqual({ pack: 'wed', form: 'corner', style: '' })
  })

  it('выбранный набор есть, а вида в нём нет — вид «Все», набор действует', () => {
    const b = browse(ITEMS, ITEMS, sel({ pack: 'wed', style: 'Значки', form: 'sticker' }))
    expect(b.sel).toEqual({ pack: 'wed', form: 'sticker', style: '' })
    expect(ids(b.items)).toEqual(['w1', 'w2'])
  })

  it('стоящая плитка видна при любом отборе и стоит на своём месте', () => {
    const b = browse(ITEMS, ITEMS, sel({ pack: 'wed' }), 'b2')
    expect(ids(b.items)).toEqual(['w1', 'w2', 'wc', 'wd', 'b2'])
    expect(ids(browse(ITEMS, ITEMS, sel({ pack: 'school', form: 'corner' }), 's2').items)).toEqual(['s2', 'sc'])
    expect(ids(browse(ITEMS, ITEMS, NO_SEL, 'b2').items)).toHaveLength(ITEMS.length)
    expect(ids(browse(ITEMS, ITEMS, sel({ pack: 'wed' }), 'нет-такой').items)).toEqual(['w1', 'w2', 'wc', 'wd'])
  })

  it('пустого списка нет и отбор его не ломает', () => {
    const b = browse([], [], sel({ pack: 'x' }))
    expect(b.items).toEqual([])
    expect(b.packs).toEqual([])
  })
})

describe('подпись плитки', () => {
  const badge = ITEMS[0]
  it('без приставки — только при выбранном стиле', () => {
    expect(tileLabel(badge, 'Значки')).toBe('Колокольчик')
    expect(tileLabel(badge, '')).toBe('Значки: Колокольчик') // «Стиль: Все» и скрытый ряд — полное имя
  })
  it('чужой стиль, имя без приставки и пустой остаток не режутся', () => {
    expect(tileLabel(badge, 'Рисунки')).toBe('Значки: Колокольчик')
    expect(tileLabel(it_('x', { name: 'Колокольчик', style: 'Значки' }), 'Значки')).toBe('Колокольчик')
    expect(tileLabel(it_('y', { name: 'Значки: ', style: 'Значки' }), 'Значки')).toBe('Значки: ')
    expect(tileLabel(it_('z', { name: 'Значки: Звонок' }), 'Значки')).toBe('Значки: Звонок') // у плитки нет стиля
  })
  it('действующий стиль берётся из browse: пропавший выбор приставку не убирает', () => {
    const b = browse(ITEMS, ITEMS, sel({ style: 'Значки' }))
    expect(b.items.map((i) => tileLabel(i, b.sel.style))).toEqual(['Колокольчик', 'Ромашка', 'Звонок'])
    const lost = browse(ITEMS.slice(8), ITEMS, sel({ style: 'Значки' }))
    expect(tileLabel(badge, lost.sel.style)).toBe('Значки: Колокольчик')
  })
})

describe('числа слева', () => {
  const cat: DecorCatalog = {
    kind: 'sticker',
    themes: [
      { id: 'new', name: 'Новые', count: 4 },
      { id: 'all', name: 'Все', count: ITEMS.length },
      { id: 'a', name: 'Тема', count: ITEMS.length - 1 },
      { id: 'mine', name: 'Мои', count: 1 },
    ],
    items: ITEMS,
    recent: [],
    new: ['w1', 'wc', 'w2', 'wd'],
    readOnly: false,
  }
  const counts = (f: BrowseSel, o?: Parameters<typeof sideCounts>[2]) => sideCounts(cat, f, o).map((t) => t.count)
  // Тема «a» — все, кроме своих; «Мои» — своё (my:1 лежит в теме mine)
  const themeA = ITEMS.filter((i) => i.theme === 'a')

  it('без отбора — полные числа тем, «Новые» по cat.new', () => {
    expect(counts(NO_SEL)).toEqual([4, ITEMS.length, themeA.length, 1])
  })

  it('числа по строкам при разных отборах (ожидания разобраны вручную, а не тем же конвейером)', () => {
    // строки: Новые (w1 wc w2 wd), Все, тема «a» (все, кроме своих), Мои (my:1)
    expect(counts(sel({ style: 'Значки' }))).toEqual([4, 3, 3, 1]) // в «Новых» и «Мои» стиля нет — «Все»
    expect(counts(sel({ form: 'corner' }))).toEqual([1, 2, 2, 1])
    expect(counts(sel({ pack: 'school', form: 'sticker', style: 'Рисунки' }))).toEqual([2, 3, 3, 1])
    expect(counts(sel({ form: 'divider', pack: 'wed' }))).toEqual([1, 1, 1, 1])
  })

  it('набор «Свадьба»: во «Все» 4, в «Новых» 4, в «Мои» полное 1 (выбранного там нет — «Все»)', () => {
    expect(counts(sel({ pack: 'wed' }))).toEqual([4, 4, 4, 1])
  })

  it('«Уголки»: во «Все» 2, в «Новых» 1 (wc), в «Мои» полное число — выбранного там нет', () => {
    expect(counts(sel({ form: 'corner' }))).toEqual([1, 2, 2, 1])
  })

  it('стоящая плитка входит в число своей темы', () => {
    expect(counts(sel({ pack: 'wed' }), { keep: 'b2' })).toEqual([4, 5, 5, 1])
  })

  it('«Куда» у разворотов: сначала tplVisible, потом отбор набора', () => {
    const pages = [
      pk('p1', 'Школа', 'school', { pages: 1 }),
      pk('p2', 'Свадьба', 'wed', { pages: 2 }),
      pk('p3', 'Свадьба', 'wed', { pages: 1 }),
    ]
    const sp: DecorCatalog = {
      kind: 'spread',
      themes: [{ id: 'all', name: 'Все', count: 3 }],
      items: pages,
      recent: [],
      new: [],
      readOnly: false,
    }
    const one = { mid: 0 } // лист без шва: разворот на две страницы не встанет
    expect(tplSideItems(sp, 'spread', one).map((t) => t.count)).toEqual([2])
    expect(tplSideItems(sp, 'spread', one, sel({ pack: 'wed' })).map((t) => t.count)).toEqual([1])
    expect(tplSideItems(sp, 'spread', { mid: 300 }, sel({ pack: 'wed' })).map((t) => t.count)).toEqual([2])
  })

  it('нет каталога — пусто', () => expect(sideCounts(null, NO_SEL)).toEqual([]))
})
