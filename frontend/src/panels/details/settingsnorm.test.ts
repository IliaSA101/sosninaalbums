import { describe, expect, it } from 'vitest'
import type { Settings } from '../../base/core/types'
import { norm, same } from './settingsnorm'

const s: Settings = { pageW: 281, pageH: 284, margin: 5, gap: 3, layflat: true, foldAny: false, dpi: 300, bg: '', safe: 10, bleed: 2 }

describe('norm: что уходит в SetSettings', () => {
  it('обрез проходит в Go (иначе «Применить» стёр бы его)', () => {
    expect(norm(s).bleed).toBe(2)
    expect(norm({ ...s, bleed: 0.5 }).bleed).toBe(0.5)
  })
  it('нет поля, пустой ввод и мусор — 0', () => {
    expect(norm({ ...s, bleed: undefined }).bleed).toBe(0)
    expect(norm({ ...s, bleed: null as never }).bleed).toBe(0)
    expect(norm({ ...s, bleed: NaN }).bleed).toBe(0)
  })
  it('строки из полей number становятся числами', () => {
    expect(norm({ ...s, bleed: '3' as never, safe: '8' as never })).toMatchObject({ bleed: 3, safe: 8 })
  })
  it('режим листа сохраняется', () => {
    expect(norm({ ...s, sheet: true }).sheet).toBe(true)
  })
})

describe('same: форма отличается от настроек', () => {
  it('другой обрез — отличается, нет поля и 0 — одно и то же', () => {
    expect(same(s, { ...s, bleed: 3 })).toBe(false)
    expect(same({ ...s, bleed: 0 }, { ...s, bleed: undefined })).toBe(true)
    expect(same(s, { ...s })).toBe(true)
  })
  it('«любые фото» без layflat не считается отличием', () => {
    expect(same({ ...s, layflat: false, foldAny: true }, { ...s, layflat: false, foldAny: false })).toBe(true)
  })
})
