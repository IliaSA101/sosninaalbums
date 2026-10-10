import { describe, expect, it } from 'vitest'
import { capFirst, errText } from './errtext'

describe('errText', () => {
  it('берёт message у Error без префикса', () => {
    expect(errText(new Error('нет файла'))).toBe('нет файла')
    expect(errText(new Error('Error: нет файла'))).toBe('нет файла')
  })
  it('строку отдаёт как есть, срезая «Error: »', () => {
    expect(errText('нельзя склеить')).toBe('нельзя склеить')
    expect(errText('Error: нельзя склеить')).toBe('нельзя склеить')
  })
  it('прочее приводит к строке', () => {
    expect(errText(undefined)).toBe('')
    expect(errText(42)).toBe('42')
  })
})

describe('capFirst', () => {
  it('поднимает первую букву', () => {
    expect(capFirst('мест нет')).toBe('Мест нет')
    expect(capFirst('Уже так')).toBe('Уже так')
    expect(capFirst('')).toBe('')
  })
})
