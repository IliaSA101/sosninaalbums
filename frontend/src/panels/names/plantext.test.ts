import { describe, expect, it } from 'vitest'
import type { NamesPlan, NamesRow } from './nametypes'
import {
  columnsLine,
  EMPTY_SEAT,
  fileInfo,
  hasSeats,
  leftList,
  leftLine,
  noSeatsAction,
  noSeatsLine,
  rowKey,
  summaryLine,
  vigHint,
  whoText,
} from './plantext'

const row = (tag: string, no = 1): NamesRow => ({ place: '', photo: tag + no, seat: false, tag, no, line: no, who: { last: 'Иванов' } })
// Место листа с местами: photo '' — пустое.
const seat = (id: string, photo = '', over: Partial<NamesRow> = {}): NamesRow => ({
  place: id,
  photo,
  seat: true,
  tag: 'ученик 1',
  no: 1,
  line: 2,
  who: { last: 'Иванов', first: 'Петя' },
  ...over,
})
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
  it('«Авто»: итог по фото', () => {
    const rows = [row('')]
    expect(summaryLine({ named: 23, seats: 25, extra: 1, missing: 2, rows })).toBe(
      'Подписано: 23 из 25 · строк лишних: 1 · фото без имени: 2',
    )
  })

  it('лист с местами (хоть одна строка с place): «мест без имени»', () => {
    const rows = [seat('p1'), seat('p2', 'a')]
    expect(summaryLine({ named: 10, seats: 12, extra: 0, missing: 2, rows })).toBe(
      'Подписано: 10 из 12 · строк лишних: 0 · мест без имени: 2',
    )
  })

  it('нет строк — как «Авто»', () => {
    expect(summaryLine({ named: 0, seats: 0, extra: 3, missing: 0, rows: [] })).toBe(
      'Подписано: 0 из 0 · строк лишних: 3 · фото без имени: 0',
    )
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

describe('нет мест и фото (071-11)', () => {
  it('фраза с подсказкой, слово — по единице', () => {
    expect(noSeatsLine('На листе')).toBe('На листе нет мест. Сделайте виньетку или ручную сетку — имена лягут на места по порядку')
    expect(noSeatsLine('На развороте')).toMatch(/^На развороте нет мест./)
  })
  it('обложка: совета про виньетку нет', () => {
    expect(noSeatsLine('На обложке', true)).toBe('На обложке нет мест для имён')
  })
})

describe('noSeatsAction: кнопка пустого предпросмотра', () => {
  const all = () => true
  const sp = (over: object = {}) => ({ id: 's1', kind: 'spread', ...over }) as never
  it('«Авто» без фото: сначала сделать виньеткой', () => {
    expect(noSeatsAction(sp(), all)).toBe('make')
  })
  it('виньетка: сразу окно', () => {
    expect(noSeatsAction(sp({ kind: 'vignette', vig: {} }), all)).toBe('setup')
  })
  it('обложка, нет листа, уровень без виньетки: кнопки нет', () => {
    expect(noSeatsAction(sp({ id: 'cover' }), all)).toBe('none')
    expect(noSeatsAction(null, all)).toBe('none')
    expect(noSeatsAction(sp(), () => false)).toBe('none')
  })
})

describe('строки по месту', () => {
  it('ключ — место, на «Авто» — фото; пустые места не сливаются', () => {
    expect(rowKey(seat('p1'))).toBe('p1')
    expect(rowKey(seat('p2', 'a'))).toBe('p2')
    expect(rowKey(row(''))).toBe('1')
    const keys = [seat('p1'), seat('p2'), seat('p3', 'a')].map(rowKey)
    expect(new Set(keys).size).toBe(3)
  })

  it('лист с местами определяется по place', () => {
    expect(hasSeats({ rows: [seat('p1')] })).toBe(true)
    expect(hasSeats({ rows: [row('')] })).toBe(false)
    expect(hasSeats({ rows: [] })).toBe(false)
  })

  it('справа от стрелки: имя, «без имени» у группового на виньетке, прочерк', () => {
    expect(whoText(seat('p1'), true)).toBe('Иванов Петя')
    expect(whoText(seat('p2', '', { no: 0, tag: 'групповое', who: null }), true)).toBe('без имени')
    expect(whoText(seat('p3', 'a', { who: null }), true)).toBe('—')
    expect(whoText(row('', 1), false)).toBe('Иванов')
    expect(whoText(row('', 0), false)).toBe('Иванов')
  })

  it('подпись файла у пустого места', () => {
    expect(EMPTY_SEAT).toBe('место без фото')
  })
})
