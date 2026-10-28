import { describe, expect, it } from 'vitest'
import cases from '../../../../testdata/parity/ranges.json'
import { parseExportRange, rangeSummary, type ExportUnit } from './exportrange'

type Case = { in: string; n: number; want?: [number, number][]; err?: true }

// Общие примеры с Go (TestParseRangeMatchesUI): одни и те же входы и ответы.
describe('parseExportRange по testdata/parity/ranges.json', () => {
  for (const c of cases as Case[]) {
    it(`«${c.in.length > 40 ? c.in.slice(0, 20) + '…' : c.in}» при ${c.n}`, () => {
      const r = parseExportRange(c.in, c.n)
      if (c.err) {
        expect(r.error).toBeTruthy()
        expect(r.units).toEqual([])
      } else {
        expect(r.error).toBeNull()
        expect(r.units).toEqual(c.want)
      }
    })
  }
})

describe('parseExportRange: тексты ошибок', () => {
  it('страница слитно', () => {
    expect(parseExportRange('7 п', 10).error).toMatch(/слитно/)
  })
  it('в примере — введённый номер', () => {
    expect(parseExportRange('6 п', 10).error).toBe('Номер страницы пишется слитно: 6п')
    expect(parseExportRange('1, 4 Л', 10).error).toBe('Номер страницы пишется слитно: 4л')
    expect(parseExportRange('3-5п', 10).error).toBe('Страницу можно указать только у одного разворота: 5п')
    expect(parseExportRange('7п-9', 10).error).toBe('Страницу можно указать только у одного разворота: 7п')
  })
  it('страница только у одного разворота', () => {
    expect(parseExportRange('3-5п', 10).error).toMatch(/только у одного/)
    expect(parseExportRange('7п-9', 10).error).toMatch(/только у одного/)
  })
  it('нет такого разворота', () => {
    expect(parseExportRange('12п', 10).error).toMatch(/нет/)
  })
  it('одни запятые — ошибка, а не пустой экспорт', () => {
    expect(parseExportRange(', ,', 10).error).toBeTruthy()
  })
})

describe('parseExportRange: проект-лист (031)', () => {
  it('слова «лист», альбом не меняется', () => {
    expect(parseExportRange('12', 10, true).error).toBe('Листа 12 нет (всего 10)')
    expect(parseExportRange('12', 10).error).toBe('Разворота 12 нет (всего 10)')
    expect(parseExportRange('12', 10, false).error).toBe('Разворота 12 нет (всего 10)')
    expect(parseExportRange('1', 0, true).error).toBe('Нет листов')
    expect(parseExportRange('', 5, true).error).toBe('Укажите лист')
    expect(parseExportRange('', 5).error).toBe('Укажите разворот или страницу')
    expect(parseExportRange(', ,', 5, true).error).toBe('Укажите лист')
  })
  it('страниц в листе нет (как Go CheckSheet)', () => {
    expect(parseExportRange('7п', 10, true).error).toMatch(/нет страниц/)
    expect(parseExportRange('3-5п', 10, true).error).toMatch(/нет страниц/)
    expect(parseExportRange('6 п', 10, true).error).toMatch(/нет страниц/)
  })
  it('верные номера листов разбираются', () => {
    expect(parseExportRange('3-4, 8', 10, true).units).toEqual([
      [3, 0],
      [4, 0],
      [8, 0],
    ])
  })
})

describe('rangeSummary (073)', () => {
  const whole = (n: number): ExportUnit[] => Array.from({ length: n }, (_, i) => [i + 1, 0])
  it('развороты: 1 / 2 / 5 / 11 / 21', () => {
    expect(rangeSummary(whole(1), false, false)).toBe('Выгрузится 1 разворот')
    expect(rangeSummary(whole(2), false, false)).toBe('Выгрузится 2 разворота')
    expect(rangeSummary(whole(5), false, false)).toBe('Выгрузится 5 разворотов')
    expect(rangeSummary(whole(11), false, false)).toBe('Выгрузится 11 разворотов')
    expect(rangeSummary(whole(21), false, false)).toBe('Выгрузится 21 разворот')
  })
  it('развороты и страницы вместе', () => {
    expect(rangeSummary([...whole(3), [7, 2]], false, false)).toBe('Выгрузится 3 разворота и 1 страница')
  })
  it('только страницы', () => {
    const u: ExportUnit[] = [
      [2, 1],
      [3, 2],
    ]
    expect(rangeSummary(u, false, false)).toBe('Выгрузится 2 страницы')
    expect(rangeSummary([[2, 1]], false, false)).toBe('Выгрузится 1 страница')
  })
  it('лист виньетки (031)', () => {
    expect(rangeSummary(whole(4), true, false)).toBe('Выгрузится 4 листа')
    expect(rangeSummary(whole(1), true, false)).toBe('Выгрузится 1 лист')
    expect(rangeSummary(whole(5), true, false)).toBe('Выгрузится 5 листов')
  })
  it('обложка — «… и обложка»', () => {
    expect(rangeSummary(whole(4), false, true)).toBe('Выгрузится 4 разворота и обложка')
    expect(rangeSummary(whole(4), true, true)).toBe('Выгрузится 4 листа и обложка')
    expect(rangeSummary([...whole(3), [7, 2]], false, true)).toBe('Выгрузится 3 разворота, 1 страница и обложка')
  })
  it('из разбора «3-5, 7п» — 3 разворота и 1 страница', () => {
    expect(rangeSummary(parseExportRange('3-5, 7п', 10).units, false, false)).toBe('Выгрузится 3 разворота и 1 страница')
  })
})
