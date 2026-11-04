import { describe, expect, it } from 'vitest'
import { fromLatin, norm, toLatin } from './kbdlayout'

describe('kbdlayout: раскладка ЙЦУКЕН ↔ QWERTY', () => {
  it('fromLatin: набрано в английской вместо русской', () => {
    expect(fromLatin('j,kj;rf')).toBe('обложка')
    expect(fromLatin('ghbdtn')).toBe('привет')
    expect(fromLatin('GHBDTN')).toBe('привет')
  })

  it('toLatin: набрано в русской вместо английской', () => {
    expect(toLatin('шьп')).toBe('img')
    expect(toLatin('ШЬП_0798')).toBe('img_0798')
  })

  it('таблица взаимно обратна на всех клавишах', () => {
    const cyr = 'йцукенгшщзхъфывапролджэячсмитьбюё'
    expect(fromLatin(toLatin(cyr))).toBe(cyr)
  })

  it('цифры, подчёркивание и чужие знаки остаются', () => {
    expect(toLatin('№798 - a_1')).toBe('№798 - a_1')
    expect(fromLatin('798_')).toBe('798_')
  })

  it('norm: нижний регистр и ё → е', () => {
    expect(norm('ЁЖик')).toBe('ежик')
  })
})
