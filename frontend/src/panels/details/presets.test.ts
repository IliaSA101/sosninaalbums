import { describe, expect, it } from 'vitest'
import { exportPx, presetActive, presetFields, presets } from './presets'

const fsfera = presets[0]

describe('пресет Фотосфера 30×30', () => {
  it('даёт 6638×3354 px при экспорте', () => {
    expect(exportPx(fsfera.w * 2, fsfera.dpi!)).toBe(6638)
    expect(exportPx(fsfera.h, fsfera.dpi!)).toBe(3354)
  })
  it('заполняет размер, dpi и layflat, поля не трогает', () => {
    expect(presetFields(fsfera)).toEqual({ pageW: 281, pageH: 284, dpi: 300, layflat: true })
  })
  it('подсвечен только при совпадении всех заданных полей', () => {
    const s = { pageW: 281, pageH: 284, dpi: 300, layflat: true }
    expect(presetActive(fsfera, s)).toBe(true)
    expect(presetActive(fsfera, { ...s, dpi: 250 })).toBe(false)
    expect(presetActive(fsfera, { ...s, layflat: false })).toBe(false)
    expect(presetActive(fsfera, { ...s, pageW: 300 })).toBe(false)
  })
  it('значения из полей number (строки) тоже сравниваются', () => {
    expect(presetActive(fsfera, { pageW: '281', pageH: '284', dpi: '300', layflat: true } as never)).toBe(true)
  })
})

describe('обычные пресеты', () => {
  it('не задают dpi и layflat и подсвечены по размеру', () => {
    const sq = presets.find((p) => p.label === '25×25')!
    expect(presetFields(sq)).toEqual({ pageW: 250, pageH: 250 })
    expect(presetActive(sq, { pageW: 250, pageH: 250, dpi: 600, layflat: false })).toBe(true)
  })
})
