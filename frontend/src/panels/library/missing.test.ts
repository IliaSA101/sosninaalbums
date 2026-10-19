import { describe, expect, it } from 'vitest'
import {
  albumPhotos,
  driveLabel,
  LIB_MISSING_TEXT,
  isHidden,
  libMissingText,
  missTip,
  missingIn,
  missingKey,
  missingSpots,
  missingText,
  missingWhere,
  relinkSummary,
  showMissing,
  splitMissing,
  spotLabel,
} from './missing'

describe('missing', () => {
  it('полоса видна, только если есть недостающие', () => {
    expect(showMissing(0)).toBe(false)
    expect(showMissing(3)).toBe(true)
  })

  it('считает фото альбома', () => {
    expect(albumPhotos([{ photos: ['a', 'b'] }, { photos: ['c'] }, { photos: undefined as unknown as string[] }])).toBe(3)
    expect(albumPhotos([])).toBe(0)
  })

  it('текст полосы', () => {
    expect(missingText(3, 40)).toBe('Не найдено 3 фото из 40')
    expect(missingText(5, 2)).toBe('Не найдено 5 фото из 5')
  })

  it('кнопка диска', () => {
    expect(driveLabel({ from: 'E', to: 'F', found: 12, total: 15 })).toBe('Диск E: → F: (найдено 12)')
    expect(driveLabel({ from: '', to: '', found: 0, total: 0 })).toBe('')
    expect(driveLabel({ from: 'E', to: 'F', found: 0, total: 4 })).toBe('')
    expect(driveLabel(null)).toBe('')
  })

  it('итоговый тост', () => {
    expect(relinkSummary({ found: 5, missing: 2, ambiguous: 1 })).toBe('Найдено 5, не найдено 2, неоднозначно 1')
    expect(relinkSummary({ found: 7, missing: 0, ambiguous: 0 })).toBe('Найдено 7')
    expect(relinkSummary({ found: 0, missing: 4, ambiguous: 0 })).toBe('Найдено 0, не найдено 4')
    expect(relinkSummary({ found: 0, missing: 0, ambiguous: 0 })).toBe('')
  })
})

const U = { one: 'разворот', many: 'развороты', cap: 'Разворот' }
const SHEETS = [
  { id: 'cover', photos: ['c1'] },
  { id: 's1', photos: ['a', 'b'] },
  { id: 's2', photos: ['c'], backdrops: [{ photo: 'bg' }] },
  { id: 's3', photos: ['d', 'e'], backdrops: null },
  { id: 's4', photos: ['f'] },
]

describe('missing: где фото', () => {
  it('места по порядку листов, обложка — 0, фон тоже ищется', () => {
    expect(missingSpots(SHEETS, ['f', 'a', 'bg', 'zz'])).toEqual([
      { sheet: 's1', no: 1, photo: 'a' },
      { sheet: 's2', no: 2, photo: 'bg' },
      { sheet: 's4', no: 4, photo: 'f' },
    ])
    expect(missingSpots(SHEETS, [])).toEqual([])
  })

  it('текст «где»: один, несколько, обложка, никуда', () => {
    expect(missingWhere(missingSpots(SHEETS, ['f']), U)).toBe('разворот 4')
    expect(missingWhere(missingSpots(SHEETS, ['b', 'a', 'f']), U)).toBe('развороты 1, 4')
    expect(missingWhere(missingSpots(SHEETS, ['c1']), U)).toBe('обложка')
    expect(missingWhere(missingSpots(SHEETS, ['c1', 'd']), U)).toBe('обложка, разворот 3')
    expect(missingWhere(missingSpots(SHEETS, ['zz']), U)).toBe('')
    expect(missingWhere(missingSpots(SHEETS, ['d']), { one: 'лист', many: 'листы' })).toBe('лист 3')
  })

  it('длинный список обрезается', () => {
    const many = Array.from({ length: 12 }, (_, i) => ({ id: 's' + i, photos: ['p' + i] }))
    const ids = many.map((s) => s.photos[0])
    expect(missingWhere(missingSpots(many, ids), U)).toBe('развороты 1, 2, 3, 4, 5, 6, 7, 8 и ещё 4')
  })

  it('текст полосы с местом', () => {
    expect(missingText(1, 35, 'разворот 4')).toBe('Не найдено 1 фото из 35 — разворот 4')
    expect(missingText(1, 35, '')).toBe('Не найдено 1 фото из 35')
  })

  it('метка на листе считает пропавшие, включая фон', () => {
    expect(missingIn(SHEETS[2], new Set(['c', 'bg']))).toBe(2)
    expect(missingIn(SHEETS[1], new Set(['c']))).toBe(0)
    expect(missingIn(SHEETS[1], new Set())).toBe(0)
  })

  it('строка списка и подсказка заглушки', () => {
    const names = { f: 'IMG_1.jpg', c1: 'cover.jpg' }
    expect(spotLabel({ sheet: 's4', no: 4, photo: 'f' }, U, names)).toBe('Разворот 4 · IMG_1.jpg')
    expect(spotLabel({ sheet: 'cover', no: 0, photo: 'c1' }, U, names)).toBe('Обложка · cover.jpg')
    expect(spotLabel({ sheet: 's1', no: 1, photo: 'a' }, U, names)).toBe('Разворот 1')
    expect(missTip('f', names)).toBe('Файл не найден: IMG_1.jpg')
    expect(missTip('zz', names)).toBe('Файл фото не найден')
    expect(missTip('f', {})).toBe('Файл фото не найден')
  })

  it('скрытие держится, пока набор и открытие те же', () => {
    const k = missingKey(['b', 'a'], 1)
    expect(missingKey(['a', 'b'], 1)).toBe(k)
    expect(isHidden(k, k)).toBe(true)
    expect(isHidden(k, missingKey(['a', 'b', 'c'], 1))).toBe(false)
    expect(isHidden(k, missingKey(['a', 'b'], 2))).toBe(false)
    expect(isHidden(k, missingKey([], 1))).toBe(false)
  })
})

describe('missing: свои из библиотеки (048)', () => {
  it('свои — не в «Найти»: пропавшие делятся по View.libPhotos', () => {
    expect(splitMissing(['a', 'lib1', 'b', 'lib2'], ['lib1', 'lib2', 'lib3'])).toEqual({ photos: ['a', 'b'], own: ['lib1', 'lib2'] })
    expect(splitMissing(['a', 'b'], [])).toEqual({ photos: ['a', 'b'], own: [] })
    expect(splitMissing(['a'], undefined)).toEqual({ photos: ['a'], own: [] })
    expect(splitMissing(['a'], null)).toEqual({ photos: ['a'], own: [] })
    expect(splitMissing([], ['lib1'])).toEqual({ photos: [], own: [] })
  })

  it('строка полосы про свои: сколько, где и без слова «Найти»', () => {
    expect(libMissingText(2)).toBe('Нет в библиотеке этого компьютера: 2 — загрузите те же файлы, они найдутся сами')
    expect(libMissingText(1, 'разворот 4')).toBe(
      'Нет в библиотеке этого компьютера: 1 (разворот 4) — загрузите те же файлы, они найдутся сами',
    )
    expect(libMissingText(2)).not.toMatch(/найти/i)
  })

  it('текст карточки фона — как в спеке', () => {
    expect(LIB_MISSING_TEXT).toBe('Нет в библиотеке этого компьютера. Загрузите тот же файл — он найдётся сам')
  })

  it('стикер с фото из библиотеки считается и ищется по листу; процедурный (без фото) — нет', () => {
    const sheets = [
      { id: 's1', photos: ['a'], stickers: [{ photo: 'stk1' }, {}] },
      { id: 's2', photos: [], stickers: null },
    ]
    expect(missingIn(sheets[0], new Set(['stk1']))).toBe(1)
    expect(missingIn(sheets[0], new Set())).toBe(0)
    expect(missingWhere(missingSpots(sheets, ['stk1']), U)).toBe('разворот 1')
  })

  it('где стоят свои: ищется и среди фонов листа', () => {
    expect(missingWhere(missingSpots(SHEETS, ['bg']), U)).toBe('разворот 2')
  })
})

describe('missing: рамочки-картинки (079 U5)', () => {
  const sheets = [
    { id: 's1', photos: ['a'], places: [{ border: { photo: 'fr1' } }, {}, { border: { photo: '' } }] },
    { id: 's2', photos: [], places: null },
    { id: 's3', photos: ['b'], places: [{ border: {} }] },
  ]
  it('пустое место с рамочкой без файла — на ленте метка и в списке «Показать»', () => {
    expect(missingIn(sheets[0], new Set(['fr1']))).toBe(1)
    expect(missingIn(sheets[1], new Set(['fr1']))).toBe(0)
    expect(missingWhere(missingSpots(sheets, ['fr1']), U)).toBe('разворот 1')
  })
  it('нет photo у рамочки (встроенная) — не пропавшая; места без рамочки не мешают', () => {
    expect(missingIn(sheets[2], new Set(['b']))).toBe(1)
    expect(missingSpots(sheets, ['']).length).toBe(0)
  })
  it('своя рамочка делится по libPhotos так же, как фон: загрузкой, а не папкой', () => {
    expect(splitMissing(['fr1', 'a'], ['fr1'])).toEqual({ photos: ['a'], own: ['fr1'] })
  })
})
