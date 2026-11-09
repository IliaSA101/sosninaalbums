import { describe, expect, it } from 'vitest'
import { KEY_CODES } from '../../base/keys/chord'
import { BOARD_H, BOARD_W, boardCodes, CMD_MIN, fitCmd, MAIN, NAV, RU, rowWidth } from './board'

describe('карта клавиатуры', () => {
  it('каждый код белого списка есть на карте, кроме лишней клавиши ISO (цифровой блок приводится к верхнему ряду)', () => {
    const on = new Set(boardCodes())
    // IntlBackslash — лишняя клавиша ISO-клавиатуры, на ANSI-карте её нет; сочетание с ней всё же назначается.
    expect(KEY_CODES.filter((c) => !on.has(c) && c !== 'IntlBackslash')).toEqual([])
  })
  it('на карте нет повторов и лишних кодов', () => {
    const codes = boardCodes()
    expect(new Set(codes).size).toBe(codes.length)
    expect(codes.filter((c) => !KEY_CODES.includes(c))).toEqual([])
  })
  it('ряды основного блока по 15 единиц, блок навигации — не шире 3, плата 18,25 × 6,05', () => {
    for (const r of MAIN) expect(rowWidth(r), JSON.stringify(r.cells[0])).toBeCloseTo(15, 5)
    for (const r of NAV) expect(rowWidth(r)).toBeLessThanOrEqual(3)
    const rows = (rs: typeof MAIN) => rs.reduce((s, r) => s + (r.fn ? 1.05 : 1), 0)
    expect(rows(MAIN)).toBeCloseTo(BOARD_H, 5)
    expect(rows(NAV)).toBeCloseTo(BOARD_H, 5)
    expect(BOARD_W).toBeCloseTo(15 + 0.25 + 3, 5)
  })
  it('русские буквы: одна заглавная кириллическая на клавише, подпись есть у всех буквенных клавиш ЙЦУКЕН', () => {
    for (const [code, ru] of Object.entries(RU)) {
      expect(ru, code).toMatch(/^[А-ЯЁ]$/)
      expect(KEY_CODES, code).toContain(code)
    }
    const cells = [...MAIN, ...NAV].flatMap((r) => r.cells)
    const key = (code: string) => cells.find((c) => c.kind === 'key' && c.code === code)
    expect(key('KeyD')).toMatchObject({ en: 'D', ru: 'В' })
    expect(key('Backquote')).toMatchObject({ ru: 'Ё' })
    expect(key('Digit1')).toMatchObject({ ru: '' })
    for (const l of 'QWERTYUIOPASDFGHJKLZXCVBNM') expect(RU['Key' + l], l).toBeTruthy()
  })
})

describe('подпись команды на клавише', () => {
  it('влезает — без изменений, шрифт базовый', () => {
    expect(fitCmd('Дублировать', 1, 100)).toEqual({ text: 'Дублировать', font: 11 })
  })
  it('длинное слово на узкой клавише — мельче шрифт, текст цел (без переносов внутри слова)', () => {
    const r = fitCmd('Отклонено', 1, 50)
    expect(r.text).toBe('Отклонено')
    expect(r.font).toBeLessThan(8)
    expect(r.font).toBeGreaterThanOrEqual(CMD_MIN)
  })
  it('совсем не влезает — слово обрезается «…», слова разделены пробелом', () => {
    const r = fitCmd('Отклонено ещё', 1, 38)
    expect(r.font).toBe(CMD_MIN)
    expect(r.text).toBe('Откло… ещё')
  })
  it('пустая подпись и нулевая ширина не падают', () => {
    expect(fitCmd('', 1, 40).text).toBe('')
    expect(fitCmd('Да', 1, 0).text).toBe('Да')
  })
})
