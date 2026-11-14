import { describe, expect, it } from 'vitest'
import { baseOf, copyNo, copyReq, isCopyReq, MAX_COPY } from './copies'
import vectors from '../../../../testdata/parity/copyid.json'

describe('ID копии (паритет с Go)', () => {
  it('в общем файле есть случаи', () => {
    expect(vectors.length).toBeGreaterThan(10)
  })
  for (const v of vectors) {
    it(`«${v.id}»`, () => {
      expect(baseOf(v.id)).toBe(v.base)
      expect(copyNo(v.id)).toBe(v.no)
      expect(isCopyReq(v.id)).toBe(v.req)
    })
  }
})

describe('copyReq', () => {
  it('«<ID файла>-new» и обратно', () => {
    const q = copyReq('0123456789abcdef')
    expect(q).toBe('0123456789abcdef-new')
    expect(isCopyReq(q)).toBe(true)
  })
  it('запрос — не копия: baseOf не режет «-new»', () => {
    expect(copyNo('0123456789abcdef-new')).toBe(0)
    expect(baseOf('0123456789abcdef-new')).toBe('0123456789abcdef-new')
  })
  it('пустой ID файла — не запрос', () => {
    expect(isCopyReq('-new')).toBe(false)
    expect(isCopyReq('')).toBe(false)
  })
})

describe('границы номера', () => {
  it('2 и 99 — копии, 1 и 100 — нет', () => {
    expect(copyNo('a-2')).toBe(2)
    expect(copyNo(`a-${MAX_COPY}`)).toBe(MAX_COPY)
    expect(copyNo('a-1')).toBe(0)
    expect(copyNo('a-100')).toBe(0)
  })
  it('ведущий ноль, знак и пробел — не номер', () => {
    for (const t of ['02', '+3', '3 ', '3.0', '']) expect(copyNo(`a-${t}`)).toBe(0)
  })
})
