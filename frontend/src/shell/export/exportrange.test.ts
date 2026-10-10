import { describe, expect, it } from 'vitest'
import cases from '../../../../testdata/parity/ranges.json'
import { parseExportRange } from './exportrange'

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
