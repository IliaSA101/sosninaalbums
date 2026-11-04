import { describe, expect, it } from 'vitest'
import { recentDate, recentName, recentRows } from './recent'

// Местное время: тест не зависит от пояса, даты собираются конструктором.
const at = (y: number, m: number, d: number, h = 0, min = 0) => new Date(y, m - 1, d, h, min).getTime()
const now = at(2026, 10, 8, 15, 30)

describe('recentDate', () => {
  it('сегодня — со временем', () => expect(recentDate(at(2026, 10, 8, 9, 5), now)).toBe('сегодня, 09:05'))
  it('вчера — словом, даже ночью', () => expect(recentDate(at(2026, 10, 7, 23, 59), now)).toBe('вчера'))
  it('в этом году — день и месяц', () => expect(recentDate(at(2026, 3, 1, 12), now)).toBe('1 марта'))
  it('в прошлых годах — с годом', () => expect(recentDate(at(2025, 12, 31, 12), now)).toBe('31 декабря 2025'))
  it('нет времени — пусто', () => {
    expect(recentDate(0, now)).toBe('')
    expect(recentDate(-5, now)).toBe('')
  })
})

describe('recentRows', () => {
  it('пропавший файл: бледный, без даты, подсказка «Файл не найден»', () => {
    const [a, b] = recentRows(
      [
        { path: 'D:\\a\\Свадьба.salbum', name: 'Свадьба', exists: true, modified: at(2026, 10, 8, 10, 0) },
        { path: 'D:\\a\\Old.salbum', name: 'Old', exists: false, modified: 0 },
      ],
      now,
    )
    expect(a).toEqual({
      path: 'D:\\a\\Свадьба.salbum',
      name: 'Свадьба',
      date: 'сегодня, 10:00',
      missing: false,
      tip: 'D:\\a\\Свадьба.salbum',
    })
    expect(b.missing).toBe(true)
    expect(b.date).toBe('')
    expect(b.tip).toBe('Файл не найден')
  })
  it('пустой список — пустой результат', () => expect(recentRows([], now)).toEqual([]))
  it('пустое имя заменяется', () => expect(recentName({ name: '  ', path: 'x' })).toBe('Без имени'))
})
