import { describe, expect, it } from 'vitest'
import { unitOf } from '../../base/geom/sheet'
import {
  bgAskText,
  bgDoneText,
  bgHint,
  bgMissing,
  bgNums,
  bgOfferText,
  bgPickDefault,
  bgPickItems,
  bgPickShown,
  bgSelShown,
  bgStreak,
  bgTargets,
  isMany,
  type BgPut,
} from './bgtargets'
import type { DecorItem } from './decor'

const SPREAD = unitOf({ sheet: false })
const SHEET = unitOf({ sheet: true })
const all = ['s1', 's2', 's3', 's4', 's5']

describe('bgPickShown: где есть строка «на какие листы»', () => {
  it('разворот при двух и больше — да', () => expect(bgPickShown({ id: 's1' }, 2)).toBe(true))
  it('обложка — нет', () => expect(bgPickShown({ id: 'cover' }, 10)).toBe(false))
  it('альбом из одного разворота — нет', () => expect(bgPickShown({ id: 's1' }, 1)).toBe(false))
  it('нет листа — нет', () => expect(bgPickShown(undefined, 5)).toBe(false))
})

describe('выбор при открытии', () => {
  it('выделено 2 и больше — «выделенные», иначе «этот»', () => {
    expect(bgPickDefault(0)).toBe('this')
    expect(bgPickDefault(1)).toBe('this')
    expect(bgPickDefault(2)).toBe('sel')
    expect(bgPickDefault(7)).toBe('sel')
  })
  it('пункт «выделенные» виден только от двух', () => {
    expect(bgSelShown(1)).toBe(false)
    expect(bgSelShown(2)).toBe(true)
  })
})

describe('bgNums: номера -> ID', () => {
  it('«3, 5-8» по порядку альбома', () => {
    const ids = Array.from({ length: 9 }, (_, i) => `s${i + 1}`)
    expect(bgNums('3, 5-8', ids)).toEqual({ ids: ['s3', 's5', 's6', 's7', 's8'], error: null })
  })
  it('пусто, больше числа, мусор — ошибка образца и пустой список', () => {
    expect(bgNums('', all)).toEqual({ ids: [], error: 'Укажите номера разворотов' })
    expect(bgNums('99', all)).toEqual({ ids: [], error: 'Разворота 99 нет (всего 5)' })
    expect(bgNums('x', all).error).toBe('Не понимаю «x»')
  })
})

describe('bgTargets: цели × «Куда»', () => {
  const base = { cur: 's2', sel: ['s3', 's4'], nums: ['s1', 's5'], all }
  it.each([
    ['this', ['s2']],
    ['sel', ['s3', 's4']],
    ['nums', ['s1', 's5']],
    ['all', all],
  ] as const)('%s', (pick, ids) => {
    for (const scope of ['spread', 'left', 'right'] as const)
      expect(bgTargets({ ...base, pick, scope })).toEqual(ids.map((spread) => ({ spread, scope })))
  })
  it('обложки в «все» нет — её нет в all', () => {
    expect(bgTargets({ ...base, pick: 'all', scope: 'spread' }).map((t) => t.spread)).not.toContain('cover')
  })
})

describe('isMany: путь мимо ApplyBackground', () => {
  const t = (...ids: string[]) => ids.map((spread) => ({ spread, scope: 'spread' as const }))
  it('«этот» — нет', () => expect(isMany('this', t('s2'), 's2')).toBe(false))
  it('набор из нескольких — да', () => expect(isMany('all', t('s1', 's2'), 's2')).toBe(true))
  it('номер только текущего — один лист, прежний путь', () => expect(isMany('nums', t('s2'), 's2')).toBe(false))
  it('один другой лист — набор (ApplyBackground ставит на открытый)', () => expect(isMany('nums', t('s3'), 's2')).toBe(true))
})

describe('тексты', () => {
  it('подсказка: падежи и страницы', () => {
    expect(bgHint(1, 'spread', SPREAD)).toBe('Затронет 1 разворот')
    expect(bgHint(3, 'spread', SPREAD)).toBe('Затронет 3 разворота')
    expect(bgHint(12, 'spread', SPREAD)).toBe('Затронет 12 разворотов')
    expect(bgHint(12, 'left', SPREAD)).toBe('Затронет 12 разворотов, левые страницы')
    expect(bgHint(4, 'right', SPREAD)).toBe('Затронет 4 разворота, правые страницы')
  })
  it('в альбоме-листе — «лист»', () => {
    expect(bgHint(5, 'spread', SHEET)).toBe('Затронет 5 листов')
    expect(bgDoneText(2, SHEET)).toBe('Фон поставлен на 2 листа')
  })
  it('вопрос: только если уходят фото фона', () => {
    expect(bgAskText({ spreads: 12, photos: 0 }, SPREAD)).toBeNull()
    expect(bgAskText({ spreads: 12, photos: 3 }, SPREAD)).toBe('3 фото фона заменятся. Поставить фон на 12 разворотов?')
    expect(bgAskText({ spreads: 4, photos: 1 }, SPREAD)).toBe('1 фото фона заменится. Поставить фон на 4 разворота?')
    expect(bgAskText({ spreads: 21, photos: 5 }, SPREAD)).toBe('5 фото фона заменятся. Поставить фон на 21 разворот?')
  })
  it('итог: число листов или «уже этот фон»', () => {
    expect(bgDoneText(12, SPREAD)).toBe('Фон поставлен на 12 разворотов')
    expect(bgDoneText(1, SPREAD)).toBe('Фон поставлен на 1 разворот')
    expect(bgDoneText(0, SPREAD)).toBe('У выбранных уже этот фон')
  })
})

describe('bgPickItems: строка «На»', () => {
  it('выделенные только от двух; порядок — этот, все, весь альбом (098), выделенные, номера', () => {
    expect(bgPickItems(0, 26, SPREAD).map((i) => i.label)).toEqual(['Этот разворот', 'Все развороты (26)', 'Весь альбом', 'Номера'])
    expect(bgPickItems(1, 26, SPREAD).map((i) => i.id)).toEqual(['this', 'all', 'album', 'nums'])
    expect(bgPickItems(3, 26, SPREAD).map((i) => i.label)).toEqual([
      'Этот разворот',
      'Все развороты (26)',
      'Весь альбом',
      'Выделенные (3)',
      'Номера',
    ])
  })
  it('в альбоме-листе — «лист»', () => {
    expect(bgPickItems(0, 4, SHEET).map((i) => i.label)).toEqual(['Этот лист', 'Все листы (4)', 'Весь альбом', 'Номера'])
  })
})

describe('bgStreak: серия постановок', () => {
  const put = (index: number, o: Partial<BgPut> = {}): BgPut => ({ item: 'my:a', scope: 'spread', index, ...o })
  const none = new Set<string>()
  it('сосед (±1), тот же фон и область — да', () => {
    expect(bgStreak(put(2), put(3), none)).toBe(true)
    expect(bgStreak(put(3), put(2), none)).toBe(true)
  })
  it('первая постановка, через один, тот же лист, другой фон, другая область — нет', () => {
    expect(bgStreak(null, put(3), none)).toBe(false)
    expect(bgStreak(put(1), put(3), none)).toBe(false)
    expect(bgStreak(put(3), put(3), none)).toBe(false)
    expect(bgStreak(put(2, { item: 'my:b' }), put(3), none)).toBe(false)
    expect(bgStreak(put(2, { scope: 'left' }), put(3), none)).toBe(false)
  })
  it('об этом фоне уже спрашивали — нет', () => expect(bgStreak(put(2), put(3), new Set(['my:a']))).toBe(false))
})

describe('bgMissing: листы без этого фона', () => {
  const lib = 'my:0123456789abcdef'
  const items = [{ id: lib, theme: 'mine', name: 'М', url: '', own: true, aspect: 1, w: 1, h: 1 } as DecorItem]
  const sp = (id: string, o: Record<string, unknown> = {}) => ({ id, mid: 300, backdrops: null, patterns: [], ...o }) as never
  const own = (scope: string) => [{ scope, photo: 'p', lib }]
  it('считает листы, где в области стоит другое или ничего', () => {
    const list = [sp('s1', { backdrops: own('spread') }), sp('s2'), sp('s3', { backdrops: own('spread') }), sp('s4')]
    expect(bgMissing(list, 'spread', lib, items)).toBe(2)
    expect(bgMissing(list, 'left', lib, items)).toBe(4)
  })
  it('обложка не в счёт; лист без шва не принимает страницу', () => {
    const list = [sp('cover'), sp('s1'), sp('s2', { mid: 0 })]
    expect(bgMissing(list, 'spread', lib, items)).toBe(2)
    expect(bgMissing(list, 'right', lib, items)).toBe(1)
  })
  it('на всех уже стоит — 0', () => expect(bgMissing([sp('s1', { backdrops: own('spread') })], 'spread', lib, items)).toBe(0))
  it('текст тоста', () => {
    expect(bgOfferText(22, SPREAD)).toBe('Поставить этот фон на все развороты? Без него ещё 22')
    expect(bgOfferText(3, SHEET)).toBe('Поставить этот фон на все листы? Без него ещё 3')
  })
})
