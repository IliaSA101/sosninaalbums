import { describe, expect, it } from 'vitest'
import { countLabel, sizeLabel, timeLabel, whenLabel } from './backups'

const at = (y: number, m: number, d: number, h = 0, min = 0) => new Date(y, m - 1, d, h, min).getTime()
const now = at(2026, 10, 9, 16, 0)

describe('whenLabel', () => {
  it('сегодня и вчера — со временем', () => {
    expect(whenLabel(at(2026, 10, 9, 15, 30), now)).toBe('Сегодня 15:30')
    expect(whenLabel(at(2026, 10, 8, 9, 12), now)).toBe('Вчера 09:12')
  })
  it('раньше в этом году — дата и время', () => {
    expect(whenLabel(at(2026, 10, 3, 18, 40), now)).toBe('3 окт 18:40')
  })
  it('другой год — дата с годом, без времени', () => {
    expect(whenLabel(at(2025, 10, 3, 18, 40), now)).toBe('3 окт 2025')
  })
  it('вчера считается по календарю, а не по 24 часам', () => {
    expect(whenLabel(at(2026, 10, 8, 23, 59), at(2026, 10, 9, 0, 1))).toBe('Вчера 23:59')
  })
})

describe('timeLabel', () => {
  it('часы и минуты с нулём', () => {
    expect(timeLabel(at(2026, 10, 9, 7, 5))).toBe('07:05')
  })
})

describe('sizeLabel', () => {
  it('байты, килобайты, мегабайты', () => {
    expect(sizeLabel(512)).toBe('512 Б')
    expect(sizeLabel(131_072)).toBe('128 КБ')
    expect(sizeLabel(1_258_291)).toBe('1,2 МБ')
  })
  it('граница КБ и МБ', () => {
    expect(sizeLabel(1_048_300)).toBe('1,0 МБ')
  })
})

describe('countLabel', () => {
  it('склоняет развороты', () => {
    expect(countLabel(1, 4)).toBe('1 разворот, 4 фото')
    expect(countLabel(2, 0)).toBe('2 разворота, 0 фото')
    expect(countLabel(11, 120)).toBe('11 разворотов, 120 фото')
    expect(countLabel(21, 1)).toBe('21 разворот, 1 фото')
  })
})
