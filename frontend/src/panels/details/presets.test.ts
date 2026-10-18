import { describe, expect, it } from 'vitest'
import type { PagePreset } from '../../base/core/types'
import { foldOf } from './fold'
import { exportPx, groupLabel, presetActive, presetFields, presetGroupOf, presetRows } from './presets'

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

// Нетпринт (058): 20×20 — страница 200×203, 300 dpi, layflat, важное 10 мм.
const np20: PagePreset = { label: '20×20', group: 'Нетпринт', w: 200, h: 203, dpi: 300, layflat: true, safe: 10 }
const list = [sq, wide, fs30, fs25, np20]
const blank = { pageW: 123, pageH: 321, dpi: 300, layflat: false, safe: 0 }

describe('типография списка форматов', () => {
  it('group of active preset: подсвеченный пресет главнее сохранённой группы', () => {
    expect(presetGroupOf(list, { pageW: 200, pageH: 203, dpi: 300, layflat: true, safe: 10 }, 'Фотосфера')).toBe('Нетпринт')
    expect(presetGroupOf(list, { pageW: 281, pageH: 284, dpi: 300, layflat: true, safe: 10 })).toBe('Фотосфера')
    expect(presetGroupOf(list, { pageW: 400, pageH: 300, dpi: 150, layflat: false, safe: 0 }, 'Нетпринт')).toBe('')
  })
  it('нет подсвеченного: сохранённая группа, если она есть в списке, иначе общие', () => {
    expect(presetGroupOf(list, blank, 'Нетпринт')).toBe('Нетпринт')
    expect(presetGroupOf(list, blank, 'Чужая')).toBe('')
    expect(presetGroupOf(list, blank)).toBe('')
    expect(presetGroupOf([], blank, 'Нетпринт')).toBe('')
  })
  it('group switch keeps form: смена группы — выбор списка, форму меняет только щелчок по кнопке', () => {
    const form = { ...blank }
    const before = { ...form }
    presetGroupOf(list, form, 'Фотосфера')
    presetRows(list).filter((r) => r.group === 'Нетпринт')
    expect(form).toEqual(before)
    expect(presetFields(np20)).toEqual({ pageW: 200, pageH: 203, dpi: 300, layflat: true, safe: 10 })
  })
  it('подпись группы: пустая — «Общие»', () => {
    expect(groupLabel('')).toBe('Общие')
    expect(groupLabel('Нетпринт')).toBe('Нетпринт')
  })
})

// 064: пресет с layflat только включает layflat, foldAny не трогает — так «Нет» превращается в «Только широкие».
describe('пресет и положение «через корешок» (064)', () => {
  const pickInto = (form: { layflat: boolean; foldAny: boolean }, p: PagePreset) => ({ ...blank, ...form, ...presetFields(p) })
  it('при «Нет» пресет Фотосферы и Нетпринта ставит «Только широкие»', () => {
    for (const p of [fs30, fs25, np20]) {
      expect(foldOf(pickInto({ layflat: false, foldAny: false }, p))).toBe('wide')
    }
  })
  it('«Только широкие» и «Любые фото» пресет оставляет как есть', () => {
    expect(foldOf(pickInto({ layflat: true, foldAny: false }, fs30))).toBe('wide')
    expect(foldOf(pickInto({ layflat: true, foldAny: true }, fs30))).toBe('any')
  })
  it('общий пресет положение не меняет', () => {
    expect(foldOf(pickInto({ layflat: false, foldAny: false }, sq))).toBe('none')
    expect(foldOf(pickInto({ layflat: true, foldAny: true }, sq))).toBe('any')
  })
  it('подсветка: при «Нет» пресет layflat не активен, при любом другом положении — активен', () => {
    const s = { pageW: 281, pageH: 284, dpi: 300, safe: 10 }
    expect(presetActive(fs30, { ...s, layflat: false })).toBe(false)
    expect(presetActive(fs30, { ...s, layflat: true })).toBe(true)
  })
})

// Обрез (066): Фотосфера 2 мм, Нетпринт 3 (гипотеза); общий ряд обрез не задаёт.
describe('обрез пресета (066)', () => {
  const fsb: PagePreset = { ...fs30, bleed: 2 }
  const s = { pageW: 281, pageH: 284, dpi: 300, layflat: true, safe: 10, bleed: 2 }
  it('presetFields подставляет обрез, у пресета без обреза поле не трогает', () => {
    expect(presetFields(fsb)).toEqual({ pageW: 281, pageH: 284, dpi: 300, layflat: true, safe: 10, bleed: 2 })
    expect('bleed' in presetFields(fs30)).toBe(false)
    expect('bleed' in presetFields(sq)).toBe(false)
  })
  it('presetActive: другой обрез в форме — не подсвечен; нет поля = 0', () => {
    expect(presetActive(fsb, s)).toBe(true)
    expect(presetActive(fsb, { ...s, bleed: 3 })).toBe(false)
    expect(presetActive(fsb, { ...s, bleed: undefined })).toBe(false)
    expect(presetActive(fsb, { ...s, bleed: '2' as never })).toBe(true)
  })
  it('пресет без обреза подсвечен при любом обрезе формы', () => {
    expect(presetActive(fs30, { ...s, bleed: 0 })).toBe(true)
    expect(presetActive(fs30, { ...s, bleed: 5 })).toBe(true)
  })
})
