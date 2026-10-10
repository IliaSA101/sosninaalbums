import { describe, expect, it } from 'vitest'
import { albumPhotos, driveLabel, missingText, relinkSummary, showMissing } from './missing'

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
