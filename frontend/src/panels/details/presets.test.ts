import { describe, expect, it } from 'vitest'
import type { PagePreset } from '../../base/core/types'
import { exportPx, presetActive, presetFields, presetRows } from './presets'

// Как отдаёт Go (App.PagePresets): общий ряд без dpi/layflat, ряд Фотосферы — с ними.
const sq: PagePreset = { label: '25×25', group: '', w: 250, h: 250 }
const wide: PagePreset = { label: '40×30', group: '', w: 400, h: 300 }
const fs30: PagePreset = { label: '30×30', group: 'Фотосфера', w: 281, h: 284, dpi: 300, layflat: true, safe: 10, hint: 'Фотосфера Pro: …' }
const fs25: PagePreset = { label: '25×25', group: 'Фотосфера', w: 251, h: 254, dpi: 300, layflat: true, safe: 8 }

describe('пресет Фотосферы из Go', () => {
  it('даёт 6638×3354 px при экспорте', () => {
    expect(exportPx(fs30.w * 2, fs30.dpi!)).toBe(6638)
    expect(exportPx(fs30.h, fs30.dpi!)).toBe(3354)
  })
  it('заполняет размер, dpi, layflat и безопасную зону, поля не трогает', () => {
    expect(presetFields(fs30)).toEqual({ pageW: 281, pageH: 284, dpi: 300, layflat: true, safe: 10 })
    expect(presetFields(fs25).safe).toBe(8)
  })
  it('подсвечен только при совпадении всех заданных полей', () => {
    const s = { pageW: 281, pageH: 284, dpi: 300, layflat: true, safe: 10 }
    expect(presetActive(fs30, s)).toBe(true)
    expect(presetActive(fs30, { ...s, dpi: 250 })).toBe(false)
    expect(presetActive(fs30, { ...s, layflat: false })).toBe(false)
    expect(presetActive(fs30, { ...s, safe: 8 })).toBe(false)
    expect(presetActive(fs30, { ...s, pageW: 300 })).toBe(false)
  })
  it('значения из полей number (строки) тоже сравниваются', () => {
    expect(presetActive(fs30, { pageW: '281', pageH: '284', dpi: '300', layflat: true, safe: '10' } as never)).toBe(true)
  })
})

describe('обычные пресеты', () => {
  it('не задают dpi, layflat и safe: форма их не меняет, подсветка — по размеру', () => {
    expect(presetFields(sq)).toEqual({ pageW: 250, pageH: 250 })
    expect(presetActive(sq, { pageW: 250, pageH: 250, dpi: 600, layflat: false, safe: 0 })).toBe(true)
    expect(presetActive(sq, { pageW: 250, pageH: 250, dpi: 300, layflat: true, safe: 10 })).toBe(true)
  })
  it('поле 0/false в пресете (не пришло из Go) тоже не сравнивается', () => {
    const p = { label: 'x', w: 100, h: 100, dpi: 0, layflat: false, safe: 0 }
    expect(presetFields(p)).toEqual({ pageW: 100, pageH: 100 })
    expect(presetActive(p, { pageW: 100, pageH: 100, dpi: 150, layflat: true, safe: 5 })).toBe(true)
  })
})

describe('ряды кнопок', () => {
  it('общий ряд первым, Фотосфера — отдельным; порядок внутри ряда из Go', () => {
    const rows = presetRows([fs30, sq, fs25, wide])
    expect(rows.map((r) => r.group)).toEqual(['', 'Фотосфера'])
    expect(rows[0].items.map((p) => p.label)).toEqual(['25×25', '40×30'])
    expect(rows[1].items.map((p) => p.label)).toEqual(['30×30', '25×25'])
  })
  it('пустой список — нет рядов', () => {
    expect(presetRows([])).toEqual([])
  })
  it('формат подсвечивается в своём ряду: 25×25 общий и Фотосферы не путаются', () => {
    const s = { pageW: 251, pageH: 254, dpi: 300, layflat: true, safe: 8 }
    expect(presetActive(sq, s)).toBe(false)
    expect(presetActive(fs25, s)).toBe(true)
  })
})
