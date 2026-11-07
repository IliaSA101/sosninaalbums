import { describe, expect, it } from 'vitest'
import type { PagePreset } from '../../base/core/types'
import { foldOf } from './fold'
import {
  FILE_DPI,
  PAGE_TOL,
  exportPx,
  groupLabel,
  presetActive,
  presetFields,
  presetGroupOf,
  presetHint,
  presetRows,
  parsePx,
  sameMm,
} from './presets'

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
  it('group of active preset prefers saved among equals: одинаковые числа — группа, выбранная последней', () => {
    const f20: PagePreset = { ...np20, label: '20×20', group: 'Фабрика Фотокниги' }
    const both = [...list, f20]
    const form = { pageW: 200, pageH: 203, dpi: 300, layflat: true, safe: 10 }
    expect(presetGroupOf(both, form, 'Фабрика Фотокниги')).toBe('Фабрика Фотокниги')
    expect(presetGroupOf(both, form, 'Нетпринт')).toBe('Нетпринт')
    expect(presetGroupOf(both, form)).toBe('Нетпринт') // сохранённой нет — первая по порядку
    expect(presetGroupOf(both, form, 'Фотосфера')).toBe('Нетпринт') // сохранённая не среди подсвеченных
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

describe('подсказка пресета по выбранной типографии', () => {
  const nk: PagePreset = { label: '30×30', group: 'Нетпринт', w: 300, h: 305, dpi: 300, layflat: true, safe: 10, hint: 'Нетпринт: …' }
  const fk: PagePreset = { ...nk, group: 'Фабрика Фотокниги', hint: 'Фабрика: …' }
  const form = { pageW: 300, pageH: 305, dpi: 300, layflat: true, safe: 10 }
  it('одинаковые числа: Фабрика — подсказка Фабрики, Нетпринт — Нетпринта', () => {
    expect(presetHint([nk, fk], form, 'Фабрика Фотокниги')).toBe('Фабрика: …')
    expect(presetHint([nk, fk], form, 'Нетпринт')).toBe('Нетпринт: …')
  })
  it('нет подсвеченного пресета в группе или у него нет подсказки — undefined', () => {
    expect(presetHint([nk, fk], form, 'Фотосфера')).toBeUndefined()
    expect(presetHint([nk, fk], { ...form, pageW: 123 }, 'Нетпринт')).toBeUndefined()
    expect(presetHint([{ ...nk, hint: undefined }], form, 'Нетпринт')).toBeUndefined()
    expect(presetHint([], form, '')).toBeUndefined()
  })
})

// Допуск формата (123 §1.2): 303,95 × 202,95 — Фабрика 30×20; чужие форматы на 1 мм не сходят за свои.
describe('допуск формата PAGE_TOL (123)', () => {
  const f3020: PagePreset = { label: '30×20', group: 'Фабрика Фотокниги', w: 304, h: 203, dpi: 300, layflat: true, safe: 10, bleed: 3 }
  const form = { pageW: 303.95, pageH: 202.95, dpi: 300, layflat: true, safe: 10, bleed: 3 }
  it('страница на 1 px от шаблона подсвечивает кнопку; на краю допуска — да, за краем — нет', () => {
    expect(presetActive(f3020, form)).toBe(true)
    expect(presetActive(f3020, { ...form, pageW: 303.75, pageH: 203.25 })).toBe(true)
    expect(presetActive(f3020, { ...form, pageW: 303.74 })).toBe(false)
    expect(presetActive(f3020, { ...form, pageH: 203.26 })).toBe(false)
  })
  it('соседние форматы (1 мм): 201×203, 251×254, 200×200 — не 30×20 / Фотосфера', () => {
    expect(presetActive(f3020, { ...form, pageW: 303, pageH: 203 })).toBe(false)
    expect(presetActive(fs25, { pageW: 250, pageH: 254, dpi: 300, layflat: true, safe: 8 })).toBe(false)
    expect(presetActive(np20, { pageW: 200, pageH: 200, dpi: 300, layflat: true, safe: 10 })).toBe(false)
  })
  it('допуск только у размера: dpi, зона и обрез остаются точными', () => {
    expect(presetActive(f3020, { ...form, dpi: 250 })).toBe(false)
    expect(presetActive(f3020, { ...form, safe: 8 })).toBe(false)
    expect(presetActive(f3020, { ...form, bleed: 2 })).toBe(false)
  })
  it('значения из полей-строк тоже сравниваются с допуском', () => {
    expect(presetActive(f3020, { ...form, pageW: '303.95', pageH: '202.95' } as never)).toBe(true)
  })
})

// Паритет с Go (model.SameMm): вектор пишет Go-тест `-update`; нет файла — тест падает.
const vectors = import.meta.glob('../../../../testdata/parity/pagetol.json', { eager: true, import: 'default' }) as Record<
  string,
  { tol: number; cases: { a: number; b: number; same: boolean }[] }
>
const vec = Object.values(vectors)[0]
describe('паритет допуска с Go (testdata/parity/pagetol.json)', () => {
  it('PAGE_TOL равен допуску Go и sameMm даёт те же ответы на всех парах', () => {
    expect(PAGE_TOL).toBe(vec.tol)
    expect(vec.cases.length).toBeGreaterThan(0)
    for (const c of vec.cases) expect(sameMm(c.a, c.b), `${c.a} и ${c.b}`).toBe(c.same)
  })
})

describe('parsePx: ввод в поле «Файл … px»', () => {
  it('целое положительное число, пробелы по краям допустимы', () => {
    expect(parsePx('7181')).toBe(7181)
    expect(parsePx(' 2398 ')).toBe(2398)
    expect(parsePx('1')).toBe(1)
  })
  it('пусто, ноль, минус, дробь, буквы, экспонента — null', () => {
    for (const t of ['', ' ', '0', '-1', '12.5', '12,5', 'abc', '1e3', '+5', '7 181']) expect(parsePx(t), `«${t}»`).toBeNull()
  })
  it('число за пределами целого — null', () => {
    expect(parsePx('99999999999999999999')).toBeNull()
  })
  it('файл типографии — всегда 300 dpi', () => {
    expect(FILE_DPI).toBe(300)
  })
})
