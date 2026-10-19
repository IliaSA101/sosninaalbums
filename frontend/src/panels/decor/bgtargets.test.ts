import { describe, expect, it } from 'vitest'
import { unitOf } from '../../base/geom/sheet'
import { bgAskText, bgDoneText, bgHint, bgNums, bgPickDefault, bgPickShown, bgSelShown, bgTargets, isMany } from './bgtargets'

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
