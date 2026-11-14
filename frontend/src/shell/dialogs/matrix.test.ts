import { describe, expect, it } from 'vitest'
import { resetText, rowTip, togetherText } from './matrix'

describe('подписи матрицы', () => {
  it('основа: «Без этого программа не работает»', () => {
    expect(rowTip('base')).toBe('Без этого программа не работает')
    expect(rowTip('menu')).toBe('Без этого программа не работает')
  })

  it('подсказка называет опоры и зависящие', () => {
    expect(rowTip('rotate')).toBe('Нужна: Ручная раскладка')
    const tip = rowTip('manual')
    expect(tip).toMatch(/^От неё зависят: /)
    expect(tip).toContain('Поворот мест')
    expect(rowTip('variants')).toBe('')
  })

  it('«Включено вместе» не называет саму строку', () => {
    expect(togetherText('rotate', ['manual', 'rotate'], [])).toBe('Включено вместе: Ручная раскладка')
    expect(togetherText('rotate', ['rotate'], [])).toBe('')
  })

  it('«Выключено вместе»', () => {
    const t = togetherText('manual', [], ['manual', 'rotate', 'layers'])
    expect(t).toMatch(/^Выключено вместе: /)
    expect(t).not.toContain('Ручная раскладка')
    expect(t).toContain('Слои')
  })

  it('сброс основы — в дательном падеже', () => {
    expect(resetText('full')).toBe('Набор сброшен к «Полному»')
    expect(resetText('simple')).toBe('Набор сброшен к «Простому»')
  })
})
