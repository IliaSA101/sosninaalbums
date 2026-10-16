import { describe, expect, it } from 'vitest'
import type { NamesPlan, NamesRow } from './nametypes'
import { columnsLine, fileInfo, leftList, leftLine, summaryLine, vigHint } from './plantext'

const row = (tag: string, no = 1): NamesRow => ({ photo: tag + no, tag, no, line: no, who: { last: 'Иванов' } })
const plan = (over: Partial<NamesPlan> = {}): NamesPlan => ({
  rows: [],
  left: [],
  extra: 0,
  seats: 0,
  named: 0,
  missing: 0,
  cols: null,
  ignored: [],
  byFile: false,
  order: true,
  teachers: 0,
  pupils: 0,
  teacherSeats: 0,
  pupilSeats: 0,
  needCaption: false,
  ...over,
})

describe('fileInfo', () => {
  it('xlsx: лист и число листов, csv: только строки', () => {
    expect(fileInfo({ file: 'Класс.xlsx', sheet: 'Класс', sheets: 2, rows: 26 })).toBe('Класс.xlsx, лист «Класс» (листов: 2), строк: 26')
    expect(fileInfo({ file: 'Класс.xlsx', sheet: 'Класс', sheets: 1, rows: 3 })).toBe('Класс.xlsx, лист «Класс», строк: 3')
    expect(fileInfo({ file: 'список.csv', sheet: '', sheets: 0, rows: 12 })).toBe('список.csv, строк: 12')
  })
})

describe('columnsLine', () => {
  it('без заголовка — пусто', () => {
    expect(columnsLine(plan())).toBe('')
    expect(columnsLine(null)).toBe('')
  })

  it('роли, предмет → учитель, неиспользуемые', () => {
    expect(columnsLine(plan({ cols: ['', 'Фамилия', 'Имя', 'Предмет'], ignored: ['№'] }))).toBe(
      'Столбцы: Фамилия, Имя · Предмет → учитель · не используются: №',
    )
  })

  it('«Файл» — по имени файла', () => {
    expect(columnsLine(plan({ cols: ['Файл', 'ФИО'] }))).toBe('Столбцы: ФИО · Файл → по имени файла')
  })
})

describe('summaryLine и строки без фото', () => {
  it('итог по плану', () => {
    expect(summaryLine({ named: 23, seats: 25, extra: 1, missing: 2 })).toBe('Подписано: 23 из 25 · строк лишних: 1 · фото без имени: 2')
  })

  it('причины со своим текстом', () => {
    const who = { last: 'Орлов', first: 'Олег' }
    expect(leftLine({ line: 3, who, reason: 'nofile', file: '99' })).toBe('стр. 3: Орлов Олег — нет фото «99»')
    expect(leftLine({ line: 4, who, reason: 'many', file: '12' })).toBe('стр. 4: Орлов Олег — несколько фото «12»')
    expect(leftLine({ line: 5, who, reason: 'dup', file: 'a.jpg' })).toBe('стр. 5: Орлов Олег — фото «a.jpg» уже занято')
    expect(leftLine({ line: 6, who, reason: 'extra', file: '' })).toBe('стр. 6: Орлов Олег — нет места на листе')
    expect(leftLine({ line: 7, who, reason: 'noseat', file: '' })).toBe('стр. 7: Орлов Олег — нет места учителя')
  })

  it('показаны первые 5, остальные — «ещё N» по полному числу', () => {
    const left = Array.from({ length: 8 }, (_, i) => ({ line: i + 2, who: { last: 'Л' }, reason: 'extra', file: '' }))
    const r = leftList({ left, extra: 60 })
    expect(r.items).toHaveLength(5)
    expect(r.more).toBe(55)
    expect(leftList({ left: left.slice(0, 2), extra: 2 }).more).toBe(0)
  })
})

describe('vigHint', () => {
  const vigRows = [row('ученик 1'), row('ученик 2', 2), row('групповое', 0), row('учитель')]
  const seats = { teacherSeats: 1, pupilSeats: 2 }

  it('расхождение с учителем — оба числа', () => {
    expect(vigHint(plan({ rows: vigRows, ...seats, teachers: 1, pupils: 25 }))).toBe(
      'В таблице: учителей 1, учеников 25; на листе мест: 1 и 2.',
    )
  })

  it('числа сошлись — подсказки нет', () => {
    expect(vigHint(plan({ rows: vigRows, ...seats, teachers: 1, pupils: 2 }))).toBe('')
  })

  it('без учителей сравниваются все места', () => {
    expect(vigHint(plan({ rows: vigRows, ...seats, pupils: 5 }))).toBe('В таблице: строк 5; на листе мест: 3.')
    expect(vigHint(plan({ rows: vigRows, ...seats, pupils: 3 }))).toBe('')
  })

  it('пустое место ученика считается (1+3 → 3→4 без фото: мест 1 и 4)', () => {
    const rows = [row('учитель'), row('ученик 1'), row('ученик 2', 2), row('ученик 3', 3), row('групповое', 0)]
    expect(vigHint(plan({ rows, teacherSeats: 1, pupilSeats: 4, teachers: 1, pupils: 3 }))).toBe(
      'В таблице: учителей 1, учеников 3; на листе мест: 1 и 4.',
    )
  })

  it('не виньетка, сопоставление по файлу и пустая таблица — подсказки нет', () => {
    expect(vigHint(plan({ rows: [row('')], pupils: 9 }))).toBe('')
    expect(vigHint(plan({ rows: vigRows, ...seats, teachers: 1, pupils: 25, byFile: true }))).toBe('')
    expect(vigHint(plan({ rows: vigRows }))).toBe('')
    expect(vigHint(null)).toBe('')
  })
})
