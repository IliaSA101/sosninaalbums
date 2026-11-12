import { describe, expect, it } from 'vitest'
import { CROP_ITEMS, cropChangeNote, cropNote, cropOf, type CropPct } from './crop'

const PCTS: CropPct[] = [0, 5, 10, 15]

describe('положение из поля settings.crop', () => {
  it('нет поля, 0, чужое число и мусор — «Нет»; шаги — как есть', () => {
    expect(cropOf({})).toBe(0)
    expect(cropOf({ crop: 0 })).toBe(0)
    expect(cropOf({ crop: 7 })).toBe(0)
    expect(cropOf({ crop: NaN })).toBe(0)
    expect(cropOf({ crop: null as never })).toBe(0)
    for (const p of PCTS) expect(cropOf({ crop: p })).toBe(p)
  })
})

describe('переключатель и строки', () => {
  it('четыре пункта «Нет · до 5 % · до 10 % · до 15 %», у каждого подсказка', () => {
    expect(CROP_ITEMS.map((i) => [i.id, i.label])).toEqual([
      [0, 'Нет'],
      [5, 'до 5 %'],
      [10, 'до 10 %'],
      [15, 'до 15 %'],
    ])
    expect(CROP_ITEMS.every((i) => !!i.title)).toBe(true)
  })
  it('четыре разные строки-пояснения; число — из выбора', () => {
    const notes = PCTS.map(cropNote)
    expect(new Set(notes).size).toBe(4)
    expect(notes[0]).toContain('белые полосы')
    expect(notes[2]).toContain('не больше 10 % по стороне')
    expect(notes[3]).toContain('Свой кадр фото не подрезается')
  })
})

describe('вторая строка «что будет после „Применить“»', () => {
  it('выбрано то же, что в альбоме — строки нет (в том числе «Нет» → «Нет»)', () => {
    for (const p of PCTS) expect(cropChangeNote(p, p)).toBe('')
  })
  it('выключение — фото снова целиком', () => {
    expect(cropChangeNote(10, 0)).toContain('снова встанут целиком')
  })
  it('включение и смена предела — раскладки те же, ручные не меняются', () => {
    for (const [a, b] of [
      [0, 5],
      [5, 15],
      [15, 5],
    ] as const)
      expect(cropChangeNote(a, b)).toContain('Ручные развороты не меняются')
  })
})
